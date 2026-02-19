package smtp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	netsmtp "net/smtp"
)

func TestServer_StartStop(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }

	cfg := DefaultServerConfig("test.example.com")
	cfg.ListenAddr = "127.0.0.1:0" // random port

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	addr := srv.Addr()
	if addr == nil {
		t.Fatal("Addr is nil after start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestServer_ReceiveEmail(t *testing.T) {
	var mu sync.Mutex
	var received *InboundEmail

	handler := func(email *InboundEmail) error {
		mu.Lock()
		defer mu.Unlock()
		received = email
		return nil
	}

	cfg := DefaultServerConfig("test.example.com")
	cfg.ListenAddr = "127.0.0.1:0"

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop(context.Background())

	addr := srv.Addr().(*net.TCPAddr)

	// Send an email using net/smtp
	c, err := netsmtp.Dial(fmt.Sprintf("127.0.0.1:%d", addr.Port))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Quit()

	if err := c.Mail("sender@external.com"); err != nil {
		t.Fatalf("MAIL FROM: %v", err)
	}
	if err := c.Rcpt("npub1test@test.example.com"); err != nil {
		t.Fatalf("RCPT TO: %v", err)
	}

	wc, err := c.Data()
	if err != nil {
		t.Fatalf("DATA: %v", err)
	}

	msg := "From: sender@external.com\r\nTo: npub1test@test.example.com\r\nSubject: Test\r\n\r\nHello from email!"
	if _, err := wc.Write([]byte(msg)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Give handler time to process
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("no email received")
	}
	if received.From != "sender@external.com" {
		t.Errorf("From = %q", received.From)
	}
	if len(received.To) != 1 || received.To[0] != "npub1test@test.example.com" {
		t.Errorf("To = %v", received.To)
	}
	if !strings.Contains(string(received.RawMessage), "Hello from email!") {
		t.Error("message body not found in RawMessage")
	}
}

func TestServer_RejectsWrongDomain(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }

	cfg := DefaultServerConfig("bridge.example.com")
	cfg.ListenAddr = "127.0.0.1:0"

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop(context.Background())

	addr := srv.Addr().(*net.TCPAddr)

	c, err := netsmtp.Dial(fmt.Sprintf("127.0.0.1:%d", addr.Port))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Quit()

	if err := c.Mail("sender@external.com"); err != nil {
		t.Fatalf("MAIL FROM: %v", err)
	}

	// This should be rejected — wrong domain
	err = c.Rcpt("user@wrong-domain.com")
	if err == nil {
		t.Error("expected rejection for wrong domain recipient")
	}
}

func TestServer_MultipleRecipients(t *testing.T) {
	var mu sync.Mutex
	var received *InboundEmail

	handler := func(email *InboundEmail) error {
		mu.Lock()
		defer mu.Unlock()
		received = email
		return nil
	}

	cfg := DefaultServerConfig("test.example.com")
	cfg.ListenAddr = "127.0.0.1:0"

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop(context.Background())

	addr := srv.Addr().(*net.TCPAddr)

	c, err := netsmtp.Dial(fmt.Sprintf("127.0.0.1:%d", addr.Port))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Quit()

	c.Mail("sender@external.com")
	c.Rcpt("alice@test.example.com")
	c.Rcpt("bob@test.example.com")

	wc, _ := c.Data()
	wc.Write([]byte("From: sender@external.com\r\nTo: alice@test.example.com, bob@test.example.com\r\nSubject: Multi\r\n\r\nMulti test"))
	wc.Close()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("no email received")
	}
	if len(received.To) != 2 {
		t.Errorf("expected 2 recipients, got %d: %v", len(received.To), received.To)
	}
}
