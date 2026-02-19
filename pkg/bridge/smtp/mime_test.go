package smtp

import (
	"strings"
	"testing"
)

func TestParseMIME_SimplePlainText(t *testing.T) {
	raw := "From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Hello\r\nContent-Type: text/plain\r\n\r\nHello Bob!"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.From, "alice@example.com") {
		t.Errorf("From = %q", parsed.From)
	}
	if parsed.Subject != "Hello" {
		t.Errorf("Subject = %q", parsed.Subject)
	}
	if parsed.TextPlain != "Hello Bob!" {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
}

func TestParseMIME_MultipartAlternative(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Multi\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"boundary1\"\r\n" +
		"\r\n" +
		"--boundary1\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Plain text version\r\n" +
		"--boundary1\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<html><body>HTML version</body></html>\r\n" +
		"--boundary1--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "Plain text version") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
	if !strings.Contains(parsed.TextHTML, "HTML version") {
		t.Errorf("TextHTML = %q", parsed.TextHTML)
	}
}

func TestParseMIME_WithAttachment(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: With file\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary2\"\r\n" +
		"\r\n" +
		"--boundary2\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"See attached.\r\n" +
		"--boundary2\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=\"test.pdf\"\r\n" +
		"\r\n" +
		"fake-pdf-content\r\n" +
		"--boundary2--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "See attached") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(parsed.Attachments))
	}

	att := parsed.Attachments[0]
	if att.Filename != "test.pdf" {
		t.Errorf("Filename = %q", att.Filename)
	}
	if att.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q", att.ContentType)
	}
	if !strings.Contains(string(att.Data), "fake-pdf-content") {
		t.Errorf("Data = %q", string(att.Data))
	}
}

func TestParseMIME_MessageID_InReplyTo(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Reply\r\n" +
		"Message-Id: <abc123@example.com>\r\n" +
		"In-Reply-To: <parent456@example.com>\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Replying."

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if parsed.MessageID != "<abc123@example.com>" {
		t.Errorf("MessageID = %q", parsed.MessageID)
	}
	if parsed.InReplyTo != "<parent456@example.com>" {
		t.Errorf("InReplyTo = %q", parsed.InReplyTo)
	}
}

func TestParseMIME_MultipleTo(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com, carol@example.com\r\n" +
		"Cc: dave@example.com\r\n" +
		"Subject: Group\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Group message."

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if len(parsed.To) != 2 {
		t.Errorf("To has %d addresses, want 2", len(parsed.To))
	}
	if len(parsed.Cc) != 1 {
		t.Errorf("Cc has %d addresses, want 1", len(parsed.Cc))
	}
}

func TestParseMIME_EmptyMessage(t *testing.T) {
	raw := "From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Empty\r\n\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if parsed.Subject != "Empty" {
		t.Errorf("Subject = %q", parsed.Subject)
	}
}

func TestParseMIME_NestedMultipart(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Nested\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"outer\"\r\n" +
		"\r\n" +
		"--outer\r\n" +
		"Content-Type: multipart/alternative; boundary=\"inner\"\r\n" +
		"\r\n" +
		"--inner\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Nested plain\r\n" +
		"--inner\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<p>Nested HTML</p>\r\n" +
		"--inner--\r\n" +
		"--outer\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Disposition: attachment; filename=\"img.png\"\r\n" +
		"\r\n" +
		"PNG-DATA\r\n" +
		"--outer--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "Nested plain") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
	if !strings.Contains(parsed.TextHTML, "Nested HTML") {
		t.Errorf("TextHTML = %q", parsed.TextHTML)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "img.png" {
		t.Errorf("attachment filename = %q", parsed.Attachments[0].Filename)
	}
}
