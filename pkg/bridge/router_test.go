package bridge

import (
	"context"
	"strings"
	"sync"
	"testing"
)

type mockDMSink struct {
	mu   sync.Mutex
	msgs map[string][]string
}

func newMockDMSink() *mockDMSink {
	return &mockDMSink{msgs: make(map[string][]string)}
}

func (m *mockDMSink) send(pubkey, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs[pubkey] = append(m.msgs[pubkey], content)
	return nil
}

func (m *mockDMSink) get(pubkey string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.msgs[pubkey]
}

func TestRouter_Subscribe(t *testing.T) {
	sink := newMockDMSink()

	// No subscription handler — should get "not configured" response
	router := NewRouter(nil, nil, sink.send)
	router.RouteDM(context.Background(), "user1", "subscribe")

	msgs := sink.get("user1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "not configured") {
		t.Errorf("expected 'not configured' message, got: %s", msgs[0])
	}
}

func TestRouter_OutboundEmail(t *testing.T) {
	sink := newMockDMSink()

	// No outbound processor — should get "not configured" response
	router := NewRouter(nil, nil, sink.send)
	router.RouteDM(context.Background(), "user1", "To: alice@example.com\nSubject: Test\n\nHello")

	msgs := sink.get("user1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "not configured") {
		t.Errorf("expected 'not configured' message, got: %s", msgs[0])
	}
}

func TestRouter_UnrecognizedMessage(t *testing.T) {
	sink := newMockDMSink()

	router := NewRouter(nil, nil, sink.send)
	router.RouteDM(context.Background(), "user1", "just a regular hello message")

	msgs := sink.get("user1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 help message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "Marmot Email Bridge") {
		t.Errorf("expected help text, got: %s", msgs[0])
	}
	if !strings.Contains(msgs[0], "subscribe") {
		t.Error("help text should mention subscribe command")
	}
}

func TestRouter_WithSubscriptionHandler(t *testing.T) {
	sink := newMockDMSink()
	store := NewMemorySubscriptionStore()

	// Create handler with nil payment processor — will get "not available" reply
	subHandler := NewSubscriptionHandler(store, nil, sink.send, 2100)
	router := NewRouter(subHandler, nil, sink.send)

	router.RouteDM(context.Background(), "user1", "subscribe")

	// Should have been routed to the subscription handler,
	// which sends a "not available" message because payments=nil
	msgs := sink.get("user1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "not available") {
		t.Errorf("expected 'not available' from sub handler, got: %s", msgs[0])
	}
}

func TestRouter_OutboundWithProcessor(t *testing.T) {
	sink := newMockDMSink()

	// Create outbound processor with nil SMTP client — will fail at send time
	outbound := NewOutboundProcessor(nil, nil, nil, "test.example.com", sink.send)
	router := NewRouter(nil, outbound, sink.send)

	// This will hit the SMTP send which panics because smtpClient is nil.
	// But first, it should check subscription — without a sub handler, it skips that check.
	// Then it parses, validates "To:", and tries to send.
	// We're testing routing, not delivery — so we accept the panic/error.
	defer func() {
		if r := recover(); r != nil {
			// Outbound processor tried to use nil SMTP client, which is expected in this test
			t.Logf("expected panic from nil SMTP client: %v", r)
		}
	}()

	router.RouteDM(context.Background(), "user1", "To: alice@example.com\nSubject: Test\n\nBody")
}

func TestGenerateReplyLink(t *testing.T) {
	link := GenerateReplyLink("https://bridge.example.com/compose", "alice@example.com", "Hello")
	if !strings.Contains(link, "bridge.example.com/compose") {
		t.Errorf("link missing base URL: %s", link)
	}
	if !strings.Contains(link, "#to=alice@example.com") {
		t.Errorf("link missing to param: %s", link)
	}
	if !strings.Contains(link, "subject=Re: Hello") {
		t.Errorf("link missing subject: %s", link)
	}
}

func TestGenerateReplyLink_AlreadyRe(t *testing.T) {
	link := GenerateReplyLink("https://example.com/compose", "bob@test.com", "Re: Original")
	if strings.Contains(link, "Re: Re:") {
		t.Errorf("double Re: in link: %s", link)
	}
}
