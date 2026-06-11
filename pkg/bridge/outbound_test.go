package bridge

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	bridgesmtp "git.smesh.lol/orly/pkg/bridge/smtp"
)

func TestOutboundProcessor_NotSubscribed_NoSMTP(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	store := NewMemorySubscriptionStore()
	handler := NewSubscriptionHandler(store, nil, sendDM, 2100, nil, 0, "bridge.example.com")
	op := NewOutboundProcessor(nil, nil, handler, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "To: alice@example.com\n\nHello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected error reply")
	}
	if !strings.Contains(replies[0], "not configured") {
		t.Errorf("reply = %q, want 'not configured' message", replies[0])
	}
}

func TestOutboundProcessor_RateLimited(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	rl := NewRateLimiter(RateLimitConfig{
		PerUserPerHour: 1,
		MinInterval:    0,
	})
	rl.Record("user1")

	op := NewOutboundProcessor(nil, rl, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "To: alice@example.com\n\nHello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected rate limit reply")
	}
	if !strings.Contains(replies[0], "Rate limit") {
		t.Errorf("reply = %q, want rate limit message", replies[0])
	}
}

func TestOutboundProcessor_NoRecipients(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(nil, nil, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "Subject: No recipient\n\nBody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected no-recipient reply")
	}
	if !strings.Contains(replies[0], "No recipients") {
		t.Errorf("reply = %q, want no-recipients message", replies[0])
	}
}

func TestOutboundProcessor_EmptyContent(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(nil, nil, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected reply")
	}
}

func TestOutboundProcessor_WithSubscription_SMTPNil(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	store := NewMemorySubscriptionStore()
	store.Save(&Subscription{
		PubkeyHex: "user1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	handler := NewSubscriptionHandler(store, nil, sendDM, 2100, nil, 0, "bridge.example.com")

	op := NewOutboundProcessor(nil, nil, handler, "bridge.example.com", sendDM, nil)

	// Should return gracefully with "not configured" instead of panicking
	err := op.ProcessOutbound("user1", "To: alice@example.com\n\nHello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected not-configured reply")
	}
	if !strings.Contains(replies[0], "not configured") {
		t.Errorf("reply = %q, want 'not configured' message", replies[0])
	}
}

func TestOutboundProcessor_Reply_NilSendDM(t *testing.T) {
	op := &OutboundProcessor{sendDM: nil}
	op.reply("user1", "test") // should not panic
}

func TestOutboundProcessor_Reply_Error(t *testing.T) {
	op := &OutboundProcessor{
		sendDM: func(pubkey, content string) error {
			return fmt.Errorf("send failed")
		},
	}
	op.reply("user1", "test") // should not panic, just log
}

func TestOutboundProcessor_Reply_Success(t *testing.T) {
	var sent string
	op := &OutboundProcessor{
		sendDM: func(pubkey, content string) error {
			sent = content
			return nil
		},
	}
	op.reply("user1", "hello")
	if sent != "hello" {
		t.Errorf("sent = %q, want hello", sent)
	}
}

func TestOutboundProcessor_FullFlow(t *testing.T) {
	var received *bridgesmtp.InboundEmail
	server := bridgesmtp.NewServer(bridgesmtp.ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *bridgesmtp.InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	smtpClient := bridgesmtp.NewClient(bridgesmtp.ClientConfig{
		FromDomain: "bridge.example.com",
	})
	smtpClient.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	smtpClient.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial("127.0.0.1:" + port)
	})

	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	store := NewMemorySubscriptionStore()
	store.Save(&Subscription{
		PubkeyHex: "user1pubkeyhex0123456789abcdef0123456789abcdef0123456789abcdef01",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	handler := NewSubscriptionHandler(store, nil, sendDM, 2100, nil, 0, "bridge.example.com")

	op := NewOutboundProcessor(smtpClient, nil, handler, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound(
		"user1pubkeyhex0123456789abcdef0123456789abcdef0123456789abcdef01",
		"To: alice@test.example.com\nSubject: Hello\n\nThis is a test.",
	)
	if err != nil {
		t.Fatalf("ProcessOutbound failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}
	if !strings.Contains(received.From, "bridge.example.com") {
		t.Errorf("From = %q, want bridge.example.com domain", received.From)
	}
	if len(replies) == 0 {
		t.Fatal("expected confirmation reply")
	}
	if !strings.Contains(replies[len(replies)-1], "Email sent to") {
		t.Errorf("expected confirmation, got: %s", replies[len(replies)-1])
	}
}

func TestOutboundProcessor_SendFails(t *testing.T) {
	smtpClient := bridgesmtp.NewClient(bridgesmtp.ClientConfig{
		FromDomain: "bridge.example.com",
	})
	smtpClient.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	smtpClient.SetDialer(func(a string) (*gosmtp.Client, error) {
		return nil, net.ErrClosed
	})

	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(smtpClient, nil, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "To: alice@example.com\n\nHello")
	if err == nil {
		t.Fatal("expected error from SMTP failure")
	}
	if len(replies) == 0 {
		t.Fatal("expected failure reply")
	}
	if !strings.Contains(replies[0], "delivery failed") {
		t.Errorf("expected delivery failure message, got: %s", replies[0])
	}
}

func TestOutboundProcessor_ParseError(t *testing.T) {
	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(nil, nil, nil, "bridge.example.com", sendDM, nil)

	// Content without "To:" header but starting with something that looks like a header
	err := op.ProcessOutbound("user1", "Subject: Test\n\nNo recipient")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(replies) == 0 {
		t.Fatal("expected reply about no recipients")
	}
	if !strings.Contains(replies[0], "No recipients") {
		t.Errorf("expected no-recipients message, got: %s", replies[0])
	}
}

func TestOutboundProcessor_WithCcRecipients(t *testing.T) {
	var received *bridgesmtp.InboundEmail
	server := bridgesmtp.NewServer(bridgesmtp.ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *bridgesmtp.InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	smtpClient := bridgesmtp.NewClient(bridgesmtp.ClientConfig{
		FromDomain: "bridge.example.com",
	})
	smtpClient.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	smtpClient.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial("127.0.0.1:" + port)
	})

	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(smtpClient, nil, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "To: alice@test.example.com\nCc: bob@test.example.com\nSubject: CC Test\n\nHello with CC")
	if err != nil {
		t.Fatalf("ProcessOutbound failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}
	// Confirmation should include both To and Cc
	if len(replies) == 0 {
		t.Fatal("expected confirmation reply")
	}
	last := replies[len(replies)-1]
	if !strings.Contains(last, "alice@test.example.com") {
		t.Errorf("confirmation missing To recipient: %s", last)
	}
	if !strings.Contains(last, "bob@test.example.com") {
		t.Errorf("confirmation missing Cc recipient: %s", last)
	}
}

func TestOutboundProcessor_ShortPubkey(t *testing.T) {
	// Test with pubkey shorter than 16 chars
	var received *bridgesmtp.InboundEmail
	server := bridgesmtp.NewServer(bridgesmtp.ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *bridgesmtp.InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	smtpClient := bridgesmtp.NewClient(bridgesmtp.ClientConfig{
		FromDomain: "bridge.example.com",
	})
	smtpClient.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	smtpClient.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial("127.0.0.1:" + port)
	})

	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	op := NewOutboundProcessor(smtpClient, nil, nil, "bridge.example.com", sendDM, nil)

	// Use a short pubkey (< 16 chars)
	err := op.ProcessOutbound("abc", "To: alice@test.example.com\n\nHello")
	if err != nil {
		t.Fatalf("ProcessOutbound failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}
	// From address should use the full short pubkey
	if !strings.Contains(received.From, "abc@bridge.example.com") {
		t.Errorf("From = %q, expected abc@bridge.example.com", received.From)
	}
}

func TestOutboundProcessor_WithRateLimiter(t *testing.T) {
	var received *bridgesmtp.InboundEmail
	server := bridgesmtp.NewServer(bridgesmtp.ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *bridgesmtp.InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	smtpClient := bridgesmtp.NewClient(bridgesmtp.ClientConfig{
		FromDomain: "bridge.example.com",
	})
	smtpClient.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	smtpClient.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial("127.0.0.1:" + port)
	})

	var replies []string
	sendDM := func(pubkey, content string) error {
		replies = append(replies, content)
		return nil
	}

	rl := NewRateLimiter(RateLimitConfig{
		PerUserPerHour: 100,
		MinInterval:    0,
	})

	op := NewOutboundProcessor(smtpClient, rl, nil, "bridge.example.com", sendDM, nil)

	err := op.ProcessOutbound("user1", "To: alice@test.example.com\n\nHello")
	if err != nil {
		t.Fatalf("ProcessOutbound failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}

	_ = received
}
