package bridge

import (
	"testing"
)

func TestClassifyDM_Subscribe(t *testing.T) {
	tests := []struct {
		input string
		want  DMCommand
	}{
		{"subscribe", DMCommandSubscribe},
		{"Subscribe", DMCommandSubscribe},
		{"SUBSCRIBE", DMCommandSubscribe},
		{"  subscribe  ", DMCommandSubscribe},
		{"\n subscribe \n", DMCommandSubscribe},
	}

	for _, tt := range tests {
		got := ClassifyDM(tt.input)
		if got != tt.want {
			t.Errorf("ClassifyDM(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestClassifyDM_None(t *testing.T) {
	tests := []string{
		"To: alice@example.com\n\nHello",
		"just a regular message",
		"subscriber",
		"subscribe please",
		"unsubscribe",
		"",
	}

	for _, input := range tests {
		got := ClassifyDM(input)
		if got != DMCommandNone {
			t.Errorf("ClassifyDM(%q) = %d, want DMCommandNone", input, got)
		}
	}
}

func TestIsOutboundEmail(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"To: alice@example.com\n\nHello", true},
		{"to: alice@example.com\n\nHello", true},
		{"TO: alice@example.com\n\nHello", true},
		{"Subject: Hello\n\nBody", true},
		{"Cc: bob@example.com\n\nBody", true},
		{"Bcc: carol@example.com\n\nBody", true},
		{"Attachment: https://blossom.example/abc#key\n\nBody", true},
		{"just a regular message", false},
		{"From: alice@example.com\n\nHello", false},
		{"", false},
		{"subscribe", false},
	}

	for _, tt := range tests {
		got := IsOutboundEmail(tt.input)
		if got != tt.want {
			t.Errorf("IsOutboundEmail(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseDMContent_FullMessage(t *testing.T) {
	content := "To: alice@example.com, bob@example.com\nCc: carol@example.com\nSubject: Hello from Nostr\nAttachment: https://blossom.example/abc123#key\n\nMessage body starts here.\nSecond line."

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.To) != 2 {
		t.Fatalf("expected 2 To addresses, got %d: %v", len(dm.To), dm.To)
	}
	if dm.To[0] != "alice@example.com" || dm.To[1] != "bob@example.com" {
		t.Errorf("To = %v, want [alice@example.com bob@example.com]", dm.To)
	}

	if len(dm.Cc) != 1 || dm.Cc[0] != "carol@example.com" {
		t.Errorf("Cc = %v, want [carol@example.com]", dm.Cc)
	}

	if len(dm.Bcc) != 0 {
		t.Errorf("Bcc = %v, want empty", dm.Bcc)
	}

	if dm.Subject != "Hello from Nostr" {
		t.Errorf("Subject = %q, want %q", dm.Subject, "Hello from Nostr")
	}

	if len(dm.Attachments) != 1 || dm.Attachments[0] != "https://blossom.example/abc123#key" {
		t.Errorf("Attachments = %v, want [https://blossom.example/abc123#key]", dm.Attachments)
	}

	expected := "Message body starts here.\nSecond line."
	if dm.Body != expected {
		t.Errorf("Body = %q, want %q", dm.Body, expected)
	}
}

func TestParseDMContent_ToOnly(t *testing.T) {
	content := "To: alice@example.com\n\nHello"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.To) != 1 || dm.To[0] != "alice@example.com" {
		t.Errorf("To = %v", dm.To)
	}
	if dm.Body != "Hello" {
		t.Errorf("Body = %q", dm.Body)
	}
}

func TestParseDMContent_MultipleAttachments(t *testing.T) {
	content := "To: alice@example.com\nAttachment: https://blossom.example/a#k1\nAttachment: https://blossom.example/b#k2\n\nBody"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(dm.Attachments))
	}
	if dm.Attachments[0] != "https://blossom.example/a#k1" {
		t.Errorf("Attachments[0] = %q", dm.Attachments[0])
	}
	if dm.Attachments[1] != "https://blossom.example/b#k2" {
		t.Errorf("Attachments[1] = %q", dm.Attachments[1])
	}
}

func TestParseDMContent_SpaceSeparatedAddresses(t *testing.T) {
	// Per spec: spaces are the real delimiter, commas are decorative
	content := "To: alice@example.com bob@example.com carol@example.com\n\nBody"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.To) != 3 {
		t.Fatalf("expected 3 To addresses, got %d: %v", len(dm.To), dm.To)
	}
}

func TestParseDMContent_BccHeader(t *testing.T) {
	content := "To: alice@example.com\nBcc: secret@example.com\n\nPrivate"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.Bcc) != 1 || dm.Bcc[0] != "secret@example.com" {
		t.Errorf("Bcc = %v, want [secret@example.com]", dm.Bcc)
	}
}

func TestParseDMContent_NoBlankLine(t *testing.T) {
	// If there's no blank line and an unrecognized line appears,
	// it becomes the body
	content := "To: alice@example.com\nThis is not a header line"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.To) != 1 {
		t.Errorf("To = %v", dm.To)
	}
	if dm.Body != "This is not a header line" {
		t.Errorf("Body = %q, want %q", dm.Body, "This is not a header line")
	}
}

func TestParseDMContent_EmptyBody(t *testing.T) {
	content := "To: alice@example.com\nSubject: No body\n\n"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if dm.Subject != "No body" {
		t.Errorf("Subject = %q", dm.Subject)
	}
	if dm.Body != "" {
		t.Errorf("Body = %q, want empty", dm.Body)
	}
}

func TestParseDMContent_BodyOnly(t *testing.T) {
	// No headers at all — everything is body
	content := "Just a plain message with no headers"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if dm.Body != "Just a plain message with no headers" {
		t.Errorf("Body = %q", dm.Body)
	}
	if len(dm.To) != 0 {
		t.Errorf("To = %v, want empty", dm.To)
	}
}

func TestParseDMContent_EmptyString(t *testing.T) {
	dm, err := ParseDMContent("")
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if dm.Body != "" {
		t.Errorf("Body = %q, want empty", dm.Body)
	}
}

func TestParseAddressList(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"alice@example.com", 1},
		{"alice@example.com, bob@example.com", 2},
		{"alice@example.com bob@example.com", 2},
		{"alice@example.com,bob@example.com,carol@example.com", 3},
		{"alice@example.com , bob@example.com , carol@example.com", 3},
		{"", 0},
		{"not-an-email", 0},
		{"alice@example.com not-an-email bob@example.com", 2},
	}

	for _, tt := range tests {
		got := parseAddressList(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseAddressList(%q) returned %d addresses, want %d: %v",
				tt.input, len(got), tt.want, got)
		}
	}
}

func TestIsOutboundEmail_EmptyFirstLine(t *testing.T) {
	// Content starts with a newline — first line is empty
	got := IsOutboundEmail("\nTo: alice@example.com")
	if got {
		t.Error("expected false when first line is empty")
	}
}

func TestParseDMContent_EmptyAttachmentValue(t *testing.T) {
	content := "To: alice@example.com\nAttachment:\n\nBody"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}
	if len(dm.Attachments) != 0 {
		t.Errorf("expected 0 attachments for empty value, got %d", len(dm.Attachments))
	}
}

func TestParseDMContent_UnknownHeaderBecomesBody(t *testing.T) {
	// An unknown header key should cause parser to treat rest as body
	content := "To: alice@example.com\nX-Custom: value\nMore text"

	dm, err := ParseDMContent(content)
	if err != nil {
		t.Fatalf("ParseDMContent error: %v", err)
	}

	if len(dm.To) != 1 {
		t.Errorf("To = %v", dm.To)
	}
	// X-Custom line and everything after should be body
	if dm.Body != "X-Custom: value\nMore text" {
		t.Errorf("Body = %q, want %q", dm.Body, "X-Custom: value\nMore text")
	}
}
