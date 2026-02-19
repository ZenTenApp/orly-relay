package smtp

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"

	// Enable full charset support for international emails
	_ "github.com/emersion/go-message/charset"
)

// ParsedMIME represents a parsed email message.
type ParsedMIME struct {
	From        string
	To          []string
	Cc          []string
	Subject     string
	MessageID   string
	InReplyTo   string
	TextPlain   string
	TextHTML    string
	Attachments []Attachment
}

// Attachment represents an email attachment.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

const maxTextBytes = 64 * 1024 // 64KB text limit per spec

// ParseMIME parses a raw email message into structured fields.
func ParseMIME(raw []byte) (*ParsedMIME, error) {
	entity, err := message.Read(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	parsed := &ParsedMIME{}

	// Extract headers
	h := entity.Header
	parsed.From, _ = h.Text("From")
	parsed.Subject, _ = h.Text("Subject")
	parsed.MessageID = h.Get("Message-Id")
	parsed.InReplyTo = h.Get("In-Reply-To")

	// Parse To addresses
	if to, _ := h.Text("To"); to != "" {
		parsed.To = splitAddresses(to)
	}
	if cc, _ := h.Text("Cc"); cc != "" {
		parsed.Cc = splitAddresses(cc)
	}

	// Parse body
	if mr := entity.MultipartReader(); mr != nil {
		// Multipart message
		if err := parseMultipart(mr, parsed); err != nil {
			return nil, err
		}
	} else {
		// Simple message
		ct, _, _ := h.ContentType()
		body, err := io.ReadAll(io.LimitReader(entity.Body, maxTextBytes))
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		switch {
		case strings.HasPrefix(ct, "text/plain"), ct == "":
			parsed.TextPlain = string(body)
		case strings.HasPrefix(ct, "text/html"):
			parsed.TextHTML = string(body)
		}
	}

	return parsed, nil
}

func parseMultipart(mr message.MultipartReader, parsed *ParsedMIME) error {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("next part: %w", err)
		}

		ct, params, _ := part.Header.ContentType()
		disp, dispParams, _ := part.Header.ContentDisposition()

		// Nested multipart (e.g., multipart/alternative inside multipart/mixed)
		if strings.HasPrefix(ct, "multipart/") {
			if nestedMR := part.MultipartReader(); nestedMR != nil {
				if err := parseMultipart(nestedMR, parsed); err != nil {
					return err
				}
				continue
			}
		}

		// Attachment (by disposition or non-text content type)
		if disp == "attachment" || (disp == "" && !strings.HasPrefix(ct, "text/")) {
			data, err := io.ReadAll(part.Body)
			if err != nil {
				return fmt.Errorf("read attachment: %w", err)
			}

			filename := dispParams["filename"]
			if filename == "" {
				filename = params["name"]
			}

			parsed.Attachments = append(parsed.Attachments, Attachment{
				Filename:    filename,
				ContentType: ct,
				Data:        data,
			})
			continue
		}

		// Text content
		body, err := io.ReadAll(io.LimitReader(part.Body, maxTextBytes))
		if err != nil {
			return fmt.Errorf("read text part: %w", err)
		}

		switch {
		case strings.HasPrefix(ct, "text/plain"):
			if parsed.TextPlain == "" {
				parsed.TextPlain = string(body)
			}
		case strings.HasPrefix(ct, "text/html"):
			if parsed.TextHTML == "" {
				parsed.TextHTML = string(body)
			}
		}
	}

	return nil
}

// splitAddresses splits a comma-separated address list.
func splitAddresses(s string) []string {
	parts := strings.Split(s, ",")
	var addrs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			addrs = append(addrs, p)
		}
	}
	return addrs
}
