package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// mockNWC implements NWCRequester for testing.
type mockNWC struct {
	responses map[string]any    // method -> response to marshal into result
	errors    map[string]error  // method -> error to return
	callCount map[string]int    // method -> number of calls
}

func newMockNWC() *mockNWC {
	return &mockNWC{
		responses: make(map[string]any),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
}

func (m *mockNWC) Request(ctx context.Context, method string, params, result any) error {
	m.callCount[method]++
	if err, ok := m.errors[method]; ok {
		return err
	}
	if resp, ok := m.responses[method]; ok {
		data, _ := json.Marshal(resp)
		return json.Unmarshal(data, result)
	}
	return nil
}

func TestNewPaymentProcessorWithClient(t *testing.T) {
	mock := newMockNWC()
	pp := NewPaymentProcessorWithClient(mock, 2100)
	if pp == nil {
		t.Fatal("expected non-nil PaymentProcessor")
	}
	if pp.monthlyPriceMSats != 2100000 {
		t.Errorf("monthlyPriceMSats = %d, want 2100000", pp.monthlyPriceMSats)
	}
}

func TestCreateSubscriptionInvoice(t *testing.T) {
	mock := newMockNWC()
	mock.responses["make_invoice"] = map[string]any{
		"invoice":      "lnbc21000n1...",
		"payment_hash": "abc123",
		"amount":       2100000,
		"description":  "test",
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	inv, err := pp.CreateSubscriptionInvoice(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Bolt11 != "lnbc21000n1..." {
		t.Errorf("Bolt11 = %q, want lnbc21000n1...", inv.Bolt11)
	}
	if inv.PaymentHash != "abc123" {
		t.Errorf("PaymentHash = %q, want abc123", inv.PaymentHash)
	}
	if mock.callCount["make_invoice"] != 1 {
		t.Errorf("callCount = %d, want 1", mock.callCount["make_invoice"])
	}
}

func TestCreateSubscriptionInvoice_Error(t *testing.T) {
	mock := newMockNWC()
	mock.errors["make_invoice"] = fmt.Errorf("wallet offline")

	pp := NewPaymentProcessorWithClient(mock, 2100)
	_, err := pp.CreateSubscriptionInvoice(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "make_invoice") {
		t.Errorf("error = %q, want to contain 'make_invoice'", err)
	}
}

func TestLookupInvoice_Paid(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"invoice":      "lnbc...",
		"payment_hash": "abc123",
		"amount":       2100000,
		"settled_at":   1700000000,
		"preimage":     "deadbeef",
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	status, err := pp.LookupInvoice(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsPaid {
		t.Error("expected IsPaid=true")
	}
	if status.Preimage != "deadbeef" {
		t.Errorf("Preimage = %q, want deadbeef", status.Preimage)
	}
}

func TestLookupInvoice_Unpaid(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"invoice":      "lnbc...",
		"payment_hash": "abc123",
		"amount":       2100000,
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	status, err := pp.LookupInvoice(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.IsPaid {
		t.Error("expected IsPaid=false")
	}
}

func TestLookupInvoice_Error(t *testing.T) {
	mock := newMockNWC()
	mock.errors["lookup_invoice"] = fmt.Errorf("not found")

	pp := NewPaymentProcessorWithClient(mock, 2100)
	_, err := pp.LookupInvoice(context.Background(), "abc123")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLookupInvoice_PaidByPreimageOnly(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"invoice":      "lnbc...",
		"payment_hash": "abc123",
		"preimage":     "some_preimage",
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	status, err := pp.LookupInvoice(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsPaid {
		t.Error("expected IsPaid=true when preimage present")
	}
}

func TestWaitForPayment_ImmediatelyPaid(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
		"settled_at":   1700000000,
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	status, err := pp.WaitForPayment(context.Background(), "abc123", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsPaid {
		t.Error("expected IsPaid=true")
	}
}

func TestWaitForPayment_Cancelled(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := pp.WaitForPayment(ctx, "abc123", 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestWaitForPayment_DefaultInterval(t *testing.T) {
	mock := newMockNWC()
	mock.responses["lookup_invoice"] = map[string]any{
		"payment_hash": "abc123",
		"settled_at":   1700000000,
	}

	pp := NewPaymentProcessorWithClient(mock, 2100)
	// Pass 0 interval to trigger default
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	status, err := pp.WaitForPayment(ctx, "abc123", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsPaid {
		t.Error("expected IsPaid=true")
	}
}

func TestWaitForPayment_TransientErrors(t *testing.T) {
	callCount := 0
	mock := &mockNWC{
		responses: make(map[string]any),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
	// Override Request to return errors first, then success
	originalRequest := mock.Request
	_ = originalRequest
	transientMock := &transientNWC{
		failCount: 2,
		successResp: map[string]any{
			"payment_hash": "abc123",
			"settled_at":   1700000000,
		},
		calls: &callCount,
	}

	pp := NewPaymentProcessorWithClient(transientMock, 2100)
	status, err := pp.WaitForPayment(context.Background(), "abc123", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsPaid {
		t.Error("expected IsPaid=true after transient errors")
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}

type transientNWC struct {
	failCount   int
	successResp map[string]any
	calls       *int
}

func (m *transientNWC) Request(ctx context.Context, method string, params, result any) error {
	*m.calls++
	if *m.calls <= m.failCount {
		return fmt.Errorf("transient error %d", *m.calls)
	}
	data, _ := json.Marshal(m.successResp)
	return json.Unmarshal(data, result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
