package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// testDMCollector records DMs sent by the subscription handler.
type testDMCollector struct {
	mu       sync.Mutex
	messages map[string][]string
}

func newTestDMCollector() *testDMCollector {
	return &testDMCollector{messages: make(map[string][]string)}
}

func (c *testDMCollector) sendDM(pubkeyHex, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages[pubkeyHex] = append(c.messages[pubkeyHex], content)
	return nil
}

func (c *testDMCollector) get(pubkeyHex string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.messages[pubkeyHex]
}

func TestSubscriptionHandler_IsSubscribed(t *testing.T) {
	store := NewMemorySubscriptionStore()

	sh := NewSubscriptionHandler(store, nil, nil, 2100, nil, 0, "test.example.com")

	if sh.IsSubscribed("abc123") {
		t.Error("should not be subscribed before saving")
	}

	store.Save(&Subscription{
		PubkeyHex: "abc123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	})

	if !sh.IsSubscribed("abc123") {
		t.Error("should be subscribed after saving")
	}

	// Expired subscription
	store.Save(&Subscription{
		PubkeyHex: "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-25 * time.Hour),
	})

	if sh.IsSubscribed("expired") {
		t.Error("expired subscription should not be active")
	}
}

func TestSubscriptionHandler_HandleSubscribe_AlreadyActive(t *testing.T) {
	store := NewMemorySubscriptionStore()
	store.Save(&Subscription{
		PubkeyHex: "abc123",
		ExpiresAt: time.Now().Add(15 * 24 * time.Hour),
		CreatedAt: time.Now().Add(-15 * 24 * time.Hour),
	})

	dms := newTestDMCollector()

	// payments=nil is fine because we shouldn't reach the payment code
	sh := NewSubscriptionHandler(store, nil, dms.sendDM, 2100, nil, 0, "test.example.com")

	ctx := context.Background()
	sh.HandleSubscribe(ctx, "abc123", "")

	msgs := dms.get("abc123")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(msgs))
	}
	if got := msgs[0]; len(got) == 0 {
		t.Error("expected non-empty already-active message")
	}
}

func TestSubscriptionHandler_HandleSubscribe_NoPaymentProcessor(t *testing.T) {
	store := NewMemorySubscriptionStore()
	dms := newTestDMCollector()

	sh := NewSubscriptionHandler(store, nil, dms.sendDM, 2100, nil, 0, "test.example.com")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HandleSubscribe panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sh.HandleSubscribe(ctx, "newuser", "")

	msgs := dms.get("newuser")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "not available") {
		t.Errorf("expected 'not available' message, got: %s", msgs[0])
	}
}

func TestSubscriptionHandler_HandleSubscribe_InvoiceCreationFails(t *testing.T) {
	store := NewMemorySubscriptionStore()
	dms := newTestDMCollector()

	mock := newMockNWC()
	mock.errors["make_invoice"] = fmt.Errorf("wallet offline")
	pp := NewPaymentProcessorWithClient(mock, 2100)

	sh := NewSubscriptionHandler(store, pp, dms.sendDM, 2100, nil, 0, "test.example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sh.HandleSubscribe(ctx, "user1", "")

	msgs := dms.get("user1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "Failed to create invoice") {
		t.Errorf("expected invoice failure message, got: %s", msgs[0])
	}
}

func TestSubscriptionHandler_HandleSubscribe_FullFlow(t *testing.T) {
	store := NewMemorySubscriptionStore()
	dms := newTestDMCollector()

	mock := newMockNWC()
	mock.responses["make_invoice"] = map[string]any{
		"invoice":      "lnbc21000n1...",
		"payment_hash": "abc123",
		"amount":       2100000,
	}
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
		"settled_at":   1700000000,
		"preimage":     "deadbeef",
	}
	pp := NewPaymentProcessorWithClient(mock, 2100)

	sh := NewSubscriptionHandler(store, pp, dms.sendDM, 2100, nil, 0, "test.example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sh.HandleSubscribe(ctx, "user1", "")

	msgs := dms.get("user1")
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 DMs (invoice + confirmation), got %d", len(msgs))
	}
	// First message: invoice
	if !strings.Contains(msgs[0], "lnbc21000n1") {
		t.Errorf("first DM should contain invoice, got: %s", msgs[0])
	}
	// Last message: confirmation
	last := msgs[len(msgs)-1]
	if !strings.Contains(last, "Payment received") {
		t.Errorf("last DM should confirm payment, got: %s", last)
	}

	// Verify subscription was saved
	if !sh.IsSubscribed("user1") {
		t.Error("user1 should be subscribed after payment")
	}
}

func TestSubscriptionHandler_HandleSubscribe_PaymentTimeout(t *testing.T) {
	store := NewMemorySubscriptionStore()
	dms := newTestDMCollector()

	mock := newMockNWC()
	mock.responses["make_invoice"] = map[string]any{
		"invoice":      "lnbc21000n1...",
		"payment_hash": "abc123",
		"amount":       2100000,
	}
	// lookup_invoice always returns unpaid
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
	}
	pp := NewPaymentProcessorWithClient(mock, 2100)

	sh := NewSubscriptionHandler(store, pp, dms.sendDM, 2100, nil, 0, "test.example.com")

	// Short timeout so the test doesn't take forever
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sh.HandleSubscribe(ctx, "user1", "")

	// Should NOT be subscribed — payment timed out
	if sh.IsSubscribed("user1") {
		t.Error("user1 should not be subscribed after timeout")
	}
}

func TestSubscriptionHandler_HandleSubscribe_SaveFails(t *testing.T) {
	dms := newTestDMCollector()

	mock := newMockNWC()
	mock.responses["make_invoice"] = map[string]any{
		"invoice":      "lnbc21000n1...",
		"payment_hash": "abc123",
	}
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
		"settled_at":   1700000000,
	}
	pp := NewPaymentProcessorWithClient(mock, 2100)

	// Use a store that fails on Save
	failStore := &failingSaveStore{}

	sh := NewSubscriptionHandler(failStore, pp, dms.sendDM, 2100, nil, 0, "test.example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sh.HandleSubscribe(ctx, "user1", "")

	msgs := dms.get("user1")
	// Should get invoice message + failure message
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "failed to activate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected save failure message, got: %v", msgs)
	}
}

func TestSubscriptionHandler_SendReply_Error(t *testing.T) {
	store := NewMemorySubscriptionStore()
	sh := NewSubscriptionHandler(store, nil, func(pk, c string) error {
		return fmt.Errorf("send error")
	}, 2100, nil, 0, "test.example.com")
	// Should not panic, just log
	sh.sendReply("user1", "test")
}

// failingSaveStore is a SubscriptionStore that always fails on Save.
type failingSaveStore struct {
	MemorySubscriptionStore
}

func (f *failingSaveStore) Save(sub *Subscription) error {
	return fmt.Errorf("save failed")
}

func (f *failingSaveStore) Get(pubkeyHex string) (*Subscription, error) {
	return nil, fmt.Errorf("not found")
}

func (f *failingSaveStore) List() ([]*Subscription, error) {
	return nil, nil
}

func (f *failingSaveStore) Delete(pubkeyHex string) error {
	return nil
}
