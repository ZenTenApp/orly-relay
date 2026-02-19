package bridge

import (
	"context"
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

	sh := NewSubscriptionHandler(store, nil, nil, 2100)

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
	sh := NewSubscriptionHandler(store, nil, dms.sendDM, 2100)

	ctx := context.Background()
	sh.HandleSubscribe(ctx, "abc123")

	msgs := dms.get("abc123")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(msgs))
	}
	if got := msgs[0]; len(got) == 0 {
		t.Error("expected non-empty already-active message")
	}
}

func TestSubscriptionHandler_HandleSubscribe_NoPaymentProcessor(t *testing.T) {
	// When payment processor is nil, HandleSubscribe should handle gracefully
	// by failing at the invoice creation step.
	// In production, this would be a configuration error.
	store := NewMemorySubscriptionStore()
	dms := newTestDMCollector()

	sh := NewSubscriptionHandler(store, nil, dms.sendDM, 2100)

	// This will panic if we don't guard against nil payments.
	// Let's verify the handler doesn't crash — it should send an error DM.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HandleSubscribe panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sh.HandleSubscribe(ctx, "newuser")
}
