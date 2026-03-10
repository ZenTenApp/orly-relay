package bridge

import (
	"fmt"
	"net/url"
	"strings"

	bridgesmtp "next.orly.dev/pkg/bridge/smtp"

	"next.orly.dev/pkg/lol/log"
)

// BlossomUploader uploads encrypted data to a Blossom server and returns the URL.
type BlossomUploader interface {
	Upload(data []byte, contentType string) (url string, err error)
}

// InboundProcessor handles converting inbound emails to DMs.
type InboundProcessor struct {
	blossom    BlossomUploader
	composeURL string
	sendDM     func(pubkeyHex string, content string) error
}

// NewInboundProcessor creates an inbound email processor.
func NewInboundProcessor(
	blossom BlossomUploader,
	composeURL string,
	sendDM func(pubkeyHex string, content string) error,
) *InboundProcessor {
	return &InboundProcessor{
		blossom:    blossom,
		composeURL: composeURL,
		sendDM:     sendDM,
	}
}

// ProcessInbound converts an inbound email to a DM and sends it to the
// Nostr recipient. The recipientPubkeyHex is resolved from the local part
// of the recipient email address (npub or hex).
func (ip *InboundProcessor) ProcessInbound(email *bridgesmtp.InboundEmail, recipientPubkeyHex string) error {
	// Parse MIME
	parsed, err := bridgesmtp.ParseMIME(email.RawMessage)
	if err != nil {
		return fmt.Errorf("parse MIME: %w", err)
	}

	// Build DM content
	var dm strings.Builder

	dm.WriteString(fmt.Sprintf("From: %s\n", parsed.From))
	dm.WriteString(fmt.Sprintf("Subject: %s\n", parsed.Subject))

	// Handle attachments: zip non-text parts, encrypt, upload to Blossom
	if len(parsed.Attachments) > 0 || parsed.TextHTML != "" {
		if ip.blossom != nil {
			url, err := ip.processAttachments(parsed.TextHTML, parsed.Attachments)
			if err != nil {
				log.W.F("attachment processing failed: %v", err)
				dm.WriteString("Attachment: [processing failed]\n")
			} else if url != "" {
				dm.WriteString(fmt.Sprintf("Attachment: %s\n", url))
			}
		} else {
			log.W.F("attachments present but no Blossom uploader configured")
		}
	}

	// Add reply link if compose URL is configured
	if ip.composeURL != "" {
		replyLink := GenerateReplyLink(ip.composeURL, parsed.From, parsed.Subject)
		dm.WriteString(fmt.Sprintf("Reply: %s\n", replyLink))
	}

	dm.WriteString("\n")

	// Body
	body := parsed.TextPlain
	if body == "" && parsed.TextHTML != "" {
		body = "[HTML-only email — see attachment]"
	}
	dm.WriteString(body)

	// Send DM to recipient
	if err := ip.sendDM(recipientPubkeyHex, dm.String()); err != nil {
		return fmt.Errorf("send DM: %w", err)
	}

	log.I.F("inbound email from %s to %s forwarded as DM", parsed.From, recipientPubkeyHex)
	return nil
}

func (ip *InboundProcessor) processAttachments(htmlBody string, attachments []bridgesmtp.Attachment) (string, error) {
	// Only process if there's something to zip
	if htmlBody == "" && len(attachments) == 0 {
		return "", nil
	}

	// Zip non-text parts
	zipData, err := ZipParts(htmlBody, attachments)
	if err != nil {
		return "", fmt.Errorf("zip parts: %w", err)
	}

	// Encrypt
	encrypted, keyHex, err := EncryptAttachment(zipData)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	// Upload to Blossom
	url, err := ip.blossom.Upload(encrypted, "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("blossom upload: %w", err)
	}

	// Append key as fragment (never sent to server)
	return url + "#" + keyHex, nil
}

// GenerateReplyLink creates a compose form URL pre-populated with reply fields.
func GenerateReplyLink(baseURL, replyTo, subject string) string {
	// Fragment-only params so the server never sees the data
	if !strings.HasPrefix(subject, "Re: ") {
		subject = "Re: " + subject
	}
	return fmt.Sprintf("%s#to=%s&subject=%s", baseURL, url.QueryEscape(replyTo), url.QueryEscape(subject))
}
