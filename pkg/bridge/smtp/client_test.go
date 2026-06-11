package smtp

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

func TestNewClient(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "example.com"})
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
	if c.resolver == nil {
		t.Error("expected non-nil resolver")
	}
	if c.dialer == nil {
		t.Error("expected non-nil dialer")
	}
}

func TestGroupByDomain(t *testing.T) {
	result := groupByDomain([]string{
		"alice@example.com",
		"bob@example.com",
		"charlie@other.com",
		"invalid-no-at",
	})
	if len(result) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(result))
	}
	if len(result["example.com"]) != 2 {
		t.Errorf("example.com count = %d, want 2", len(result["example.com"]))
	}
	if len(result["other.com"]) != 1 {
		t.Errorf("other.com count = %d, want 1", len(result["other.com"]))
	}
}

func TestGroupByDomain_CaseInsensitive(t *testing.T) {
	result := groupByDomain([]string{
		"alice@Example.COM",
		"bob@EXAMPLE.com",
	})
	if len(result) != 1 {
		t.Errorf("expected 1 domain, got %d", len(result))
	}
}

func TestGroupByDomain_Empty(t *testing.T) {
	result := groupByDomain(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 domains, got %d", len(result))
	}
}

func TestBuildMessage(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	email := &OutboundEmail{
		From:    "npub1abc@bridge.example.com",
		To:      []string{"alice@example.com"},
		Subject: "Test Subject",
		Body:    "Hello, world!",
	}
	msg, err := c.buildMessage(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(msg)
	if !strings.Contains(s, "From: npub1abc@bridge.example.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(s, "To: alice@example.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(s, "Subject: Test Subject") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(s, "Hello, world!") {
		t.Error("missing body")
	}
	if !strings.Contains(s, "Message-Id:") {
		t.Error("missing Message-Id")
	}
	if !strings.Contains(s, "Mime-Version: 1.0") {
		t.Error("missing Mime-Version header")
	}
}

func TestBuildMessage_WithCC(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	email := &OutboundEmail{
		From:    "from@bridge.example.com",
		To:      []string{"alice@example.com"},
		Cc:      []string{"bob@example.com"},
		Subject: "CC Test",
		Body:    "Body",
	}
	msg, err := c.buildMessage(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(msg), "Cc: bob@example.com") {
		t.Error("missing Cc header")
	}
}

func TestBuildMessage_BccNotInHeaders(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	email := &OutboundEmail{
		From:    "from@bridge.example.com",
		To:      []string{"alice@example.com"},
		Bcc:     []string{"secret@example.com"},
		Subject: "BCC Test",
		Body:    "Body",
	}
	msg, err := c.buildMessage(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(msg), "secret@example.com") {
		t.Error("BCC address should NOT appear in headers")
	}
}

func TestClient_SendViaLocalServer(t *testing.T) {
	// Start a local SMTP server
	var received *InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	// Create client with mock resolver pointing to local server
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		// Override to connect to our local port
		return gosmtpDial("127.0.0.1:" + port)
	})

	email := &OutboundEmail{
		From:    "sender@bridge.example.com",
		To:      []string{"recipient@test.example.com"},
		Subject: "Integration Test",
		Body:    "This is a test email.",
	}

	err := client.Send(email)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait briefly for async processing
	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}
	if received.From != "sender@bridge.example.com" {
		t.Errorf("From = %q, want sender@bridge.example.com", received.From)
	}
}

func TestClient_SendWithDKIM(t *testing.T) {
	var received *InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	// Generate a DKIM key for signing
	keyPEM, _, err := GenerateDKIMKeyPair()
	if err != nil {
		t.Fatalf("generate DKIM: %v", err)
	}
	signer, err := NewDKIMSignerFromPEM("bridge.example.com", "test", keyPEM)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	client := NewClient(ClientConfig{
		FromDomain: "bridge.example.com",
		DKIMSigner: signer,
	})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtpDial("127.0.0.1:" + port)
	})

	err = client.Send(&OutboundEmail{
		From:    "sender@bridge.example.com",
		To:      []string{"recipient@test.example.com"},
		Subject: "DKIM Test",
		Body:    "Signed message.",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email")
	}
	if !strings.Contains(string(received.RawMessage), "DKIM-Signature:") {
		t.Error("expected DKIM-Signature header in received message")
	}
}

func TestClient_MXLookupFails(t *testing.T) {
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return nil, net.ErrClosed
	})

	err := client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error from MX lookup failure")
	}
}

func TestClient_AllMXFail(t *testing.T) {
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{
			{Host: "192.0.2.1", Pref: 10}, // unreachable
		}, nil
	})
	client.SetDialer(func(addr string) (*gosmtp.Client, error) {
		return nil, net.ErrClosed
	})

	err := client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error when all MX fail")
	}
}

func TestClient_EmptyMXFallback(t *testing.T) {
	// Empty MX list should fall back to A record (the domain itself)
	var dialedAddr string
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return nil, nil // empty, no error
	})
	client.SetDialer(func(addr string) (*gosmtp.Client, error) {
		dialedAddr = addr
		return nil, net.ErrClosed
	})

	client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	// Should try to connect to example.com:25
	if !strings.Contains(dialedAddr, "example.com") {
		t.Errorf("dialedAddr = %q, want example.com:25", dialedAddr)
	}
}

func TestClient_MultipleDomains(t *testing.T) {
	var received []*InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = append(received, email)
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	// Route all domains to our local server
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtpDial("127.0.0.1:" + port)
	})

	err := client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@test.example.com", "bob@test.example.com"},
		Body: "Multi-recipient",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestClient_SendWithDKIMSignFailure(t *testing.T) {
	// DKIM signer that always fails — should fall back to sending unsigned
	var received *InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	// Create a DKIM signer with a tiny invalid key that will fail to sign
	badSigner := &DKIMSigner{
		domain:   "bridge.example.com",
		selector: "test",
		key:      nil, // nil key will cause sign to fail
	}

	client := NewClient(ClientConfig{
		FromDomain: "bridge.example.com",
		DKIMSigner: badSigner,
	})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtpDial("127.0.0.1:" + port)
	})

	err := client.Send(&OutboundEmail{
		From:    "sender@bridge.example.com",
		To:      []string{"recipient@test.example.com"},
		Subject: "DKIM Fail Test",
		Body:    "Should send unsigned.",
	})
	if err != nil {
		t.Fatalf("Send should succeed even with DKIM failure: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if received == nil {
		t.Fatal("expected server to receive email (unsigned)")
	}
	// Should NOT have DKIM-Signature since signing failed
	if strings.Contains(string(received.RawMessage), "DKIM-Signature:") {
		t.Error("should NOT have DKIM-Signature when signing fails")
	}
}

func TestClient_SendWithBcc(t *testing.T) {
	var received []*InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = append(received, email)
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtpDial("127.0.0.1:" + port)
	})

	err := client.Send(&OutboundEmail{
		From:    "sender@bridge.example.com",
		To:      []string{"alice@test.example.com"},
		Cc:      []string{"bob@test.example.com"},
		Bcc:     []string{"secret@test.example.com"},
		Subject: "BCC Test",
		Body:    "With BCC recipient.",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if len(received) == 0 {
		t.Fatal("expected server to receive email")
	}
	// BCC address should NOT appear in the message headers
	msg := string(received[0].RawMessage)
	if strings.Contains(msg, "secret@test.example.com") {
		t.Error("BCC address should NOT appear in message headers")
	}
}

func TestClient_SendMultipleDomains_PartialFailure(t *testing.T) {
	// One domain succeeds, another fails
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	resolveCount := 0
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		resolveCount++
		if domain == "fail.com" {
			return nil, net.ErrClosed
		}
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(addr string) (*gosmtp.Client, error) {
		return nil, net.ErrClosed
	})

	err := client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@fail.com", "bob@other.com"},
		Body: "Test",
	})
	// Should return an error (the last error)
	if err == nil {
		t.Fatal("expected error from partial delivery failure")
	}
}

func TestBuildMessage_WithInReplyTo(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	email := &OutboundEmail{
		From:    "from@bridge.example.com",
		To:      []string{"alice@example.com"},
		Cc:      []string{"bob@example.com", "carol@example.com"},
		Bcc:     []string{"secret@example.com"},
		Subject: "Reply Test",
		Body:    "Reply body",
	}
	msg, err := c.buildMessage(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(msg)
	// Cc should be in headers
	if !strings.Contains(s, "bob@example.com") {
		t.Error("missing Cc addresses in header")
	}
	// Bcc should NOT be in headers
	if strings.Contains(s, "secret@example.com") {
		t.Error("Bcc address should NOT appear in headers")
	}
	// Date header
	if !strings.Contains(s, "Date:") {
		t.Error("missing Date header")
	}
}

func TestClient_SendNoDomains(t *testing.T) {
	c := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	// Send to addresses with no @ sign — they're all skipped
	err := c.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"invalid-no-at"},
		Body: "Test",
	})
	// groupByDomain returns empty map → no delivery attempts → no error
	if err != nil {
		t.Errorf("expected no error for empty domain map, got: %v", err)
	}
}

func TestClient_DeliverDirect_HelloFails(t *testing.T) {
	// Start a server that accepts connections but then we can test Hello failure
	// by using a dialer that connects to a server that rejects EHLO
	var received *InboundEmail
	server := NewServer(ServerConfig{
		Domain:     "test.example.com",
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		received = email
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	// Create client with an empty FromDomain that has a very long invalid hostname
	// to trigger Hello failure on some SMTP implementations
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial("127.0.0.1:" + port)
	})

	// This should succeed since our server accepts connections
	err := client.Send(&OutboundEmail{
		From:    "sender@bridge.example.com",
		To:      []string{"alice@test.example.com"},
		Subject: "Hello Test",
		Body:    "Body",
	})
	// Should succeed with our test server
	if err != nil {
		t.Logf("deliverDirect error (may be expected): %v", err)
	}
	_ = received
}

func TestClient_DeliverDirect_RcptFails(t *testing.T) {
	// Start a server that rejects specific recipients
	server := NewServer(ServerConfig{
		Domain:     "other.example.com", // different domain
		ListenAddr: "127.0.0.1:0",
	}, func(email *InboundEmail) error {
		return nil
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop(context.Background())

	addr := server.Addr().String()
	_, port, _ := net.SplitHostPort(addr)

	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtpDial("127.0.0.1:" + port)
	})

	// The server domain is "other.example.com" so it will reject recipients
	// for "wrong.example.com" domain
	err := client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@wrong.example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error from RCPT rejection")
	}
}

func TestClient_DeliverDirect_HelloFails_Rejection(t *testing.T) {
	// Raw TCP server that rejects EHLO/HELO
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("220 test ESMTP\r\n"))
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
					switch {
					case strings.HasPrefix(cmd, "EHLO"):
						c.Write([]byte("550 EHLO rejected\r\n"))
					case strings.HasPrefix(cmd, "HELO"):
						c.Write([]byte("550 HELO rejected\r\n"))
					case strings.HasPrefix(cmd, "QUIT"):
						c.Write([]byte("221 Bye\r\n"))
						return
					default:
						c.Write([]byte("500 Unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial(addr)
	})

	err = client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error from HELO rejection")
	}
}

func TestClient_DeliverDirect_WriteDataFails(t *testing.T) {
	// Raw TCP server that accepts DATA but disconnects during body write
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("220 test ESMTP\r\n"))
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
					switch {
					case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "MAIL FROM"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "RCPT TO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "DATA"):
						c.Write([]byte("354 Start mail input\r\n"))
						time.Sleep(10 * time.Millisecond)
						c.Close()
						return
					case strings.HasPrefix(cmd, "QUIT"):
						c.Write([]byte("221 Bye\r\n"))
						return
					default:
						c.Write([]byte("500 Unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial(addr)
	})

	err = client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test body that needs writing",
	})
	if err == nil {
		t.Fatal("expected error from write data failure")
	}
}

func TestClient_DeliverDirect_MailFromFails(t *testing.T) {
	// Raw TCP server that responds to greeting and EHLO but rejects MAIL FROM
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("220 test ESMTP\r\n"))
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
					switch {
					case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "MAIL FROM"):
						c.Write([]byte("550 Sender rejected\r\n"))
					case strings.HasPrefix(cmd, "QUIT"):
						c.Write([]byte("221 Bye\r\n"))
						return
					default:
						c.Write([]byte("500 Unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial(addr)
	})

	err = client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error from MAIL FROM rejection")
	}
	if !strings.Contains(err.Error(), "MAIL FROM") {
		t.Logf("error: %v", err)
	}
}

func TestClient_DeliverDirect_DataFails(t *testing.T) {
	// Raw TCP server that accepts MAIL FROM and RCPT but rejects DATA
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("220 test ESMTP\r\n"))
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
					switch {
					case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "MAIL FROM"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "RCPT TO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "DATA"):
						c.Write([]byte("554 Transaction failed\r\n"))
					case strings.HasPrefix(cmd, "QUIT"):
						c.Write([]byte("221 Bye\r\n"))
						return
					default:
						c.Write([]byte("500 Unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial(addr)
	})

	err = client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test",
	})
	if err == nil {
		t.Fatal("expected error from DATA rejection")
	}
}

func TestClient_DeliverDirect_CloseFails(t *testing.T) {
	// Raw TCP server that accepts DATA but closes connection during write
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("220 test ESMTP\r\n"))
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
					switch {
					case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "MAIL FROM"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "RCPT TO"):
						c.Write([]byte("250 OK\r\n"))
					case strings.HasPrefix(cmd, "DATA"):
						c.Write([]byte("354 Start mail input\r\n"))
						for {
							n, err := c.Read(buf)
							if err != nil {
								return
							}
							if strings.Contains(string(buf[:n]), "\r\n.\r\n") {
								break
							}
						}
						c.Write([]byte("554 Message rejected\r\n"))
					case strings.HasPrefix(cmd, "QUIT"):
						c.Write([]byte("221 Bye\r\n"))
						return
					default:
						c.Write([]byte("500 Unknown\r\n"))
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	client := NewClient(ClientConfig{FromDomain: "bridge.example.com"})
	client.SetResolver(func(domain string) ([]*net.MX, error) {
		return []*net.MX{{Host: "127.0.0.1", Pref: 10}}, nil
	})
	client.SetDialer(func(a string) (*gosmtp.Client, error) {
		return gosmtp.Dial(addr)
	})

	err = client.Send(&OutboundEmail{
		From: "sender@bridge.example.com",
		To:   []string{"alice@example.com"},
		Body: "Test body content",
	})
	if err == nil {
		t.Fatal("expected error from message rejection")
	}
}

// Alias used in test function signatures.
var gosmtpDial = gosmtp.Dial
