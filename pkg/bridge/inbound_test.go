package bridge

import (
	crand "crypto/rand"
	"fmt"
	"strings"
	"testing"

	bridgesmtp "next.orly.dev/pkg/bridge/smtp"
)

type mockBlossom struct {
	uploadedData []byte
	returnURL    string
	returnErr    error
}

func (m *mockBlossom) Upload(data []byte, contentType string) (string, error) {
	m.uploadedData = data
	return m.returnURL, m.returnErr
}

func TestNewInboundProcessor(t *testing.T) {
	ip := NewInboundProcessor(nil, "https://example.com/compose", func(string, string) error { return nil })
	if ip == nil {
		t.Fatal("expected non-nil InboundProcessor")
	}
}

func TestProcessInbound_PlainText(t *testing.T) {
	var sentDM string
	var sentTo string
	sendDM := func(pubkey, content string) error {
		sentTo = pubkey
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(nil, "https://relay.example.com/compose", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: Hello World\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"This is the body."

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentTo != "abcd1234" {
		t.Errorf("sentTo = %q, want abcd1234", sentTo)
	}
	if !strings.Contains(sentDM, "From: alice@example.com") {
		t.Errorf("DM missing From header: %q", sentDM)
	}
	if !strings.Contains(sentDM, "Subject: Hello World") {
		t.Errorf("DM missing Subject header: %q", sentDM)
	}
	if !strings.Contains(sentDM, "This is the body.") {
		t.Errorf("DM missing body: %q", sentDM)
	}
	if !strings.Contains(sentDM, "Reply: https://relay.example.com/compose#to=alice@example.com") {
		t.Errorf("DM missing reply link: %q", sentDM)
	}
}

func TestProcessInbound_WithAttachments(t *testing.T) {
	blossom := &mockBlossom{returnURL: "https://blossom.example.com/abc123"}
	var sentDM string
	sendDM := func(pubkey, content string) error {
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(blossom, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: With Attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary123\r\n\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body text here.\r\n" +
		"--boundary123\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=\"doc.pdf\"\r\n\r\n" +
		"PDFCONTENT\r\n" +
		"--boundary123--\r\n"

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sentDM, "Attachment: https://blossom.example.com/abc123#") {
		t.Errorf("DM missing attachment URL with fragment key: %q", sentDM)
	}
	if blossom.uploadedData == nil {
		t.Error("expected blossom upload to be called")
	}
}

func TestProcessInbound_NoBlossom(t *testing.T) {
	var sentDM string
	sendDM := func(pubkey, content string) error {
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(nil, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: With HTML\r\n" +
		"Content-Type: multipart/alternative; boundary=b1\r\n\r\n" +
		"--b1\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Plain text.\r\n" +
		"--b1\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<h1>HTML</h1>\r\n" +
		"--b1--\r\n"

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No blossom = no attachment in DM
	if strings.Contains(sentDM, "Attachment:") {
		t.Errorf("should not have attachment when no Blossom: %q", sentDM)
	}
}

func TestProcessInbound_HTMLOnly(t *testing.T) {
	blossom := &mockBlossom{returnURL: "https://blossom.example.com/xyz"}
	var sentDM string
	sendDM := func(pubkey, content string) error {
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(blossom, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: HTML Only\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<h1>Hello</h1>"

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sentDM, "[HTML-only email") {
		t.Errorf("expected HTML-only fallback text: %q", sentDM)
	}
}

func TestProcessInbound_BlossomUploadError(t *testing.T) {
	blossom := &mockBlossom{returnErr: fmt.Errorf("upload failed")}
	var sentDM string
	sendDM := func(pubkey, content string) error {
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(blossom, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: Error\r\n" +
		"Content-Type: multipart/mixed; boundary=err1\r\n\r\n" +
		"--err1\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body.\r\n" +
		"--err1\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=\"doc.pdf\"\r\n\r\n" +
		"PDF\r\n" +
		"--err1--\r\n"

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sentDM, "[processing failed]") {
		t.Errorf("expected processing failed message: %q", sentDM)
	}
}

func TestProcessInbound_SendDMError(t *testing.T) {
	sendDM := func(pubkey, content string) error {
		return fmt.Errorf("DM send failed")
	}

	ip := NewInboundProcessor(nil, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body."

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessInbound_InvalidMIME(t *testing.T) {
	sendDM := func(pubkey, content string) error { return nil }
	ip := NewInboundProcessor(nil, "", sendDM)

	// Truly malformed input that go-message cannot parse at all
	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte("Content-Type: multipart/mixed; boundary=xyz\r\n\r\n--abc\r\n"),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err == nil {
		t.Fatal("expected error from invalid MIME")
	}
}

func TestProcessInbound_NoComposeURL(t *testing.T) {
	var sentDM string
	sendDM := func(pubkey, content string) error {
		sentDM = content
		return nil
	}

	ip := NewInboundProcessor(nil, "", sendDM)

	raw := "From: alice@example.com\r\nTo: npub1test@bridge.example.com\r\n" +
		"Subject: No Reply Link\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body."

	email := &bridgesmtp.InboundEmail{
		From:       "alice@example.com",
		To:         []string{"npub1test@bridge.example.com"},
		RawMessage: []byte(raw),
	}

	err := ip.ProcessInbound(email, "abcd1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sentDM, "Reply:") {
		t.Errorf("should not have Reply link without composeURL: %q", sentDM)
	}
}

func TestProcessAttachments_ZipError(t *testing.T) {
	// Feed incompressible data exceeding the 25MB zip limit
	blossom := &mockBlossom{returnURL: "https://blossom.example.com/test"}
	ip := &InboundProcessor{blossom: blossom}

	bigData := make([]byte, 26*1024*1024)
	// crypto/rand data is incompressible — zip output will be >= input
	crand.Read(bigData)

	url, err := ip.processAttachments("", []bridgesmtp.Attachment{
		{Filename: "huge.bin", ContentType: "application/octet-stream", Data: bigData},
	})
	if err == nil {
		t.Fatal("expected error from zip size exceeding limit")
	}
	if url != "" {
		t.Errorf("expected empty URL on error, got %q", url)
	}
}

func TestProcessAttachments_Empty(t *testing.T) {
	blossom := &mockBlossom{returnURL: "https://blossom.example.com/test"}
	ip := &InboundProcessor{blossom: blossom}

	url, err := ip.processAttachments("", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for no content, got %q", url)
	}
}
