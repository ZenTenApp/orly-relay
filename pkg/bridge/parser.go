package bridge

import (
	"strings"
)

// ParsedDM represents a parsed outbound email DM from a user.
type ParsedDM struct {
	// To is the list of recipient email addresses.
	To []string
	// Cc is the list of CC email addresses.
	Cc []string
	// Bcc is the list of BCC email addresses.
	Bcc []string
	// Subject is the email subject line.
	Subject string
	// Attachments is the list of Blossom fragment-key URLs.
	Attachments []string
	// Body is the email body text (everything after the blank line separator).
	Body string
}

// DMCommand represents a recognized command in a DM.
type DMCommand int

const (
	DMCommandNone      DMCommand = iota // Not a command — treat as email or contact
	DMCommandSubscribe                  // "subscribe" command
)

// ClassifyDM determines what kind of DM this is:
// - A subscribe command
// - An outbound email (has To: header)
// - A contact message (everything else → blind proxy)
func ClassifyDM(content string) DMCommand {
	trimmed := strings.TrimSpace(strings.ToLower(content))
	if trimmed == "subscribe" {
		return DMCommandSubscribe
	}
	return DMCommandNone
}

// IsOutboundEmail returns true if the DM content looks like an outbound email
// (starts with recognized headers like To:, Subject:, etc.)
func IsOutboundEmail(content string) bool {
	// Check first non-empty line for a header-like pattern
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return false
	}
	first := strings.TrimSpace(lines[0])
	firstLower := strings.ToLower(first)
	return strings.HasPrefix(firstLower, "to:") ||
		strings.HasPrefix(firstLower, "subject:") ||
		strings.HasPrefix(firstLower, "cc:") ||
		strings.HasPrefix(firstLower, "bcc:") ||
		strings.HasPrefix(firstLower, "attachment:")
}

// ParseDMContent parses a DM message in RFC 822-style format into structured
// email fields. The format is:
//
//	To: alice@example.com, bob@example.com
//	Cc: carol@example.com
//	Subject: Hello from Nostr
//	Attachment: https://blossom.example/abc123#key
//
//	Message body starts here.
//
// Headers are terminated by the first blank line. Everything after is the body.
// No From: header — the bridge derives the sender from the Nostr event pubkey.
func ParseDMContent(content string) (*ParsedDM, error) {
	dm := &ParsedDM{}

	lines := strings.Split(content, "\n")
	bodyStart := 0

headerLoop:
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Blank line terminates headers
		if trimmed == "" {
			bodyStart = i + 1
			break
		}

		// Parse header
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			// Not a header — treat everything from here as body
			bodyStart = i
			break
		}

		key := strings.TrimSpace(trimmed[:colonIdx])
		value := strings.TrimSpace(trimmed[colonIdx+1:])

		switch strings.ToLower(key) {
		case "to":
			dm.To = parseAddressList(value)
		case "cc":
			dm.Cc = parseAddressList(value)
		case "bcc":
			dm.Bcc = parseAddressList(value)
		case "subject":
			dm.Subject = value
		case "attachment":
			if value != "" {
				dm.Attachments = append(dm.Attachments, value)
			}
		default:
			// Unknown header — treat everything from here as body
			bodyStart = i
			break headerLoop
		}
	}

	// Extract body
	if bodyStart < len(lines) {
		dm.Body = strings.Join(lines[bodyStart:], "\n")
		dm.Body = strings.TrimSpace(dm.Body)
	}

	return dm, nil
}

// parseAddressList splits a comma-or-space-separated list of email addresses.
// Per the spec, spaces are the real delimiter; commas are decorative.
func parseAddressList(s string) []string {
	// Replace commas with spaces, then split on whitespace
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	var addrs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && strings.Contains(p, "@") {
			addrs = append(addrs, p)
		}
	}
	return addrs
}
