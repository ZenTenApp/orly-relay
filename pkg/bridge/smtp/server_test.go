package smtp

import (
	"context"
	"fmt"
	"net"
	netsmtp "net/smtp"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestServer_StopWithoutStart(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }
	cfg := DefaultServerConfig("test.example.com")
	srv := NewServer(cfg, handler)

	// Stop without Start — server field is set but ln is nil
	err := srv.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop without start should not error: %v", err)
	}
}

func TestServer_AddrBeforeStart(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }
	cfg := DefaultServerConfig("test.example.com")
	srv := NewServer(cfg, handler)

	addr := srv.Addr()
	if addr != nil {
		t.Errorf("Addr before start should be nil, got %v", addr)
	}
}

func TestServer_DefaultConfig(t *testing.T) {
	cfg := DefaultServerConfig("example.com")
	if cfg.Domain != "example.com" {
		t.Errorf("Domain = %q", cfg.Domain)
	}
	if cfg.ListenAddr != "0.0.0.0:2525" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.MaxMessageBytes != 25*1024*1024 {
		t.Errorf("MaxMessageBytes = %d", cfg.MaxMessageBytes)
	}
	if cfg.MaxRecipients != 10 {
		t.Errorf("MaxRecipients = %d", cfg.MaxRecipients)
	}
}

func TestServer_HandlerError(t *testing.T) {
	handler := func(email *InboundEmail) error {
		return fmt.Errorf("handler error")
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
	c.Rcpt("npub1test@test.example.com")

	wc, err := c.Data()
	if err != nil {
		t.Fatalf("DATA: %v", err)
	}
	wc.Write([]byte("From: sender@external.com\r\nTo: npub1test@test.example.com\r\n\r\nBody"))
	err = wc.Close()
	// The handler error should propagate as an SMTP error
	if err == nil {
		t.Error("expected error from handler failure")
	}
}

func TestSession_RcptInvalidAddress(t *testing.T) {
	s := &session{domain: "test.example.com", handler: func(e *InboundEmail) error { return nil }}
	err := s.Rcpt("no-at-sign", nil)
	if err == nil {
		t.Error("expected error for address without @")
	}
}

func TestSession_RcptEmptyLocalPart(t *testing.T) {
	s := &session{domain: "test.example.com", handler: func(e *InboundEmail) error { return nil }}
	err := s.Rcpt("@test.example.com", nil)
	if err == nil {
		t.Error("expected error for empty local part")
	}
}

func TestSession_DataNoRecipients(t *testing.T) {
	s := &session{domain: "test.example.com", handler: func(e *InboundEmail) error { return nil }}
	err := s.Data(strings.NewReader("body"))
	if err == nil {
		t.Error("expected error for no recipients")
	}
}

func TestSession_ResetAndLogout(t *testing.T) {
	s := &session{
		domain:  "test.example.com",
		handler: func(e *InboundEmail) error { return nil },
		from:    "test@example.com",
		to:      []string{"rcpt@test.example.com"},
	}

	s.Reset()
	if s.from != "" {
		t.Errorf("from after reset = %q", s.from)
	}
	if s.to != nil {
		t.Errorf("to after reset = %v", s.to)
	}

	err := s.Logout()
	if err != nil {
		t.Errorf("Logout error: %v", err)
	}
}

func TestServer_StartListenError(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }
	cfg := DefaultServerConfig("test.example.com")
	// Use an address that will fail to listen (already-bound or invalid)
	cfg.ListenAddr = "999.999.999.999:99999"

	srv := NewServer(cfg, handler)
	err := srv.Start()
	if err == nil {
		srv.Stop(context.Background())
		t.Fatal("expected error from invalid listen address")
	}
}

func TestServer_CustomConfig(t *testing.T) {
	handler := func(email *InboundEmail) error { return nil }
	cfg := ServerConfig{
		Domain:          "test.example.com",
		ListenAddr:      "127.0.0.1:0",
		MaxMessageBytes: 1024,
		MaxRecipients:   5,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
	}

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop(context.Background())

	if srv.Addr() == nil {
		t.Error("Addr should not be nil after Start")
	}
}

func TestSession_DataReadError(t *testing.T) {
	s := &session{
		domain:  "test.example.com",
		handler: func(e *InboundEmail) error { return nil },
		to:      []string{"user@test.example.com"},
	}

	// errReader always returns an error
	err := s.Data(&errReader{})
	if err == nil {
		t.Error("expected error from failing reader")
	}
	if !strings.Contains(err.Error(), "read message") {
		t.Errorf("expected 'read message' error, got: %v", err)
	}
}

type errReader struct{}

func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func TestSession_MailStoresFrom(t *testing.T) {
	s := &session{domain: "test.example.com"}
	err := s.Mail("sender@example.com", nil)
	if err != nil {
		t.Fatalf("Mail error: %v", err)
	}
	if s.from != "sender@example.com" {
		t.Errorf("from = %q, want sender@example.com", s.from)
	}
}

func TestServer_StopReturnsShutdownError(t *testing.T) {
	// Start a server, then stop it — should succeed
	handler := func(email *InboundEmail) error { return nil }
	cfg := DefaultServerConfig("test.example.com")
	cfg.ListenAddr = "127.0.0.1:0"

	srv := NewServer(cfg, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Normal stop should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := srv.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should succeed: %v", err)
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
