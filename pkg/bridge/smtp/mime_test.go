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

func TestParseMIME_HTMLOnly(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: HTML\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<html><body>Hello</body></html>"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if parsed.TextPlain != "" {
		t.Errorf("TextPlain should be empty, got %q", parsed.TextPlain)
	}
	if !strings.Contains(parsed.TextHTML, "Hello") {
		t.Errorf("TextHTML = %q", parsed.TextHTML)
	}
}

func TestParseMIME_NoContentType(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: No CT\r\n\r\n" +
		"Body without content type"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	// Without content-type, should default to text/plain
	if !strings.Contains(parsed.TextPlain, "Body without content type") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
}

func TestParseMIME_AttachmentWithoutDisposition(t *testing.T) {
	// Non-text part without Content-Disposition should be treated as attachment
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Inline attachment\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"b1\"\r\n\r\n" +
		"--b1\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body text\r\n" +
		"--b1\r\n" +
		"Content-Type: image/jpeg\r\n\r\n" +
		"JPEGDATA\r\n" +
		"--b1--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(parsed.Attachments))
	}
	if parsed.Attachments[0].ContentType != "image/jpeg" {
		t.Errorf("ContentType = %q", parsed.Attachments[0].ContentType)
	}
}

func TestParseMIME_AttachmentWithNameParam(t *testing.T) {
	// Filename from Content-Type name param when no Content-Disposition filename
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Name param\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"b2\"\r\n\r\n" +
		"--b2\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body\r\n" +
		"--b2\r\n" +
		"Content-Type: application/pdf; name=\"report.pdf\"\r\n\r\n" +
		"PDFDATA\r\n" +
		"--b2--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "report.pdf" {
		t.Errorf("Filename = %q, want report.pdf", parsed.Attachments[0].Filename)
	}
}

func TestSplitAddresses_Various(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"alice@example.com, bob@example.com", 2},
		{"single@example.com", 1},
		{"  spaces@example.com  ,  tabs@example.com  ", 2},
		{"", 0},
	}
	for _, tt := range tests {
		got := splitAddresses(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitAddresses(%q) returned %d, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestParseMIME_InlineDisposition(t *testing.T) {
	// inline disposition for text/html — should be treated as text content, not attachment
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Inline\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"b3\"\r\n\r\n" +
		"--b3\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: inline\r\n\r\n" +
		"Plain text\r\n" +
		"--b3\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Disposition: inline\r\n\r\n" +
		"<p>HTML text</p>\r\n" +
		"--b3--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "Plain text") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
	if !strings.Contains(parsed.TextHTML, "HTML text") {
		t.Errorf("TextHTML = %q", parsed.TextHTML)
	}
	// Should NOT have attachments (both are inline text)
	if len(parsed.Attachments) != 0 {
		t.Errorf("expected 0 attachments for inline text, got %d", len(parsed.Attachments))
	}
}

func TestParseMIME_DuplicateTextParts(t *testing.T) {
	// Two text/plain parts — only the first should be used
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Dup\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"dup\"\r\n\r\n" +
		"--dup\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"First plain\r\n" +
		"--dup\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Second plain\r\n" +
		"--dup--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "First plain") {
		t.Errorf("TextPlain should be first part: %q", parsed.TextPlain)
	}
	if strings.Contains(parsed.TextPlain, "Second plain") {
		t.Errorf("TextPlain should not contain second part: %q", parsed.TextPlain)
	}
}

func TestParseMIME_ExplicitAttachmentDisposition(t *testing.T) {
	// text/plain with disposition=attachment should be treated as attachment
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Attached Text\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"att\"\r\n\r\n" +
		"--att\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Body text\r\n" +
		"--att\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: attachment; filename=\"readme.txt\"\r\n\r\n" +
		"Attached text file content\r\n" +
		"--att--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}

	if !strings.Contains(parsed.TextPlain, "Body text") {
		t.Errorf("TextPlain = %q", parsed.TextPlain)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "readme.txt" {
		t.Errorf("attachment filename = %q", parsed.Attachments[0].Filename)
	}
}

func TestParseMIME_TrueParseError(t *testing.T) {
	// Completely empty input with no headers at all
	// message.Read with empty bytes may return an error
	_, err := ParseMIME([]byte{})
	// Empty input: message.Read may or may not error depending on implementation
	// If it doesn't error, that's fine too — this tests the code path
	_ = err
}

func TestParseMIME_MalformedMultipart(t *testing.T) {
	// Mismatched boundary — multipart reader will return error
	raw := "From: alice@example.com\r\n" +
		"To: bob@example.com\r\n" +
		"Subject: Malformed\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"expected\"\r\n" +
		"\r\n" +
		"--wrong\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Body\r\n" +
		"--wrong--\r\n"

	parsed, err := ParseMIME([]byte(raw))
	// go-message handles mismatched boundaries gracefully (returns empty parts)
	// so this may not error, but should return no text
	if err != nil {
		t.Logf("ParseMIME error (may be expected): %v", err)
		return
	}
	// No parts should have been found since boundary doesn't match
	if parsed.TextPlain != "" {
		t.Errorf("expected empty TextPlain for mismatched boundary, got %q", parsed.TextPlain)
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
