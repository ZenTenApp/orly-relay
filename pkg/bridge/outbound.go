package bridge

import (
	"fmt"
	"strings"

	"next.orly.dev/pkg/lol/log"

	aclgrpc "next.orly.dev/pkg/acl/grpc"
	bridgesmtp "next.orly.dev/pkg/bridge/smtp"
)

// OutboundProcessor handles converting DMs to outbound emails.
type OutboundProcessor struct {
	smtpClient  *bridgesmtp.Client
	rateLimiter *RateLimiter
	subHandler  *SubscriptionHandler
	domain      string
	sendDM      func(pubkeyHex string, content string) error
	aclClient   *aclgrpc.Client
}

// NewOutboundProcessor creates an outbound DM-to-email processor.
func NewOutboundProcessor(
	smtpClient *bridgesmtp.Client,
	rateLimiter *RateLimiter,
	subHandler *SubscriptionHandler,
	domain string,
	sendDM func(pubkeyHex string, content string) error,
	aclClient *aclgrpc.Client,
) *OutboundProcessor {
	return &OutboundProcessor{
		smtpClient:  smtpClient,
		rateLimiter: rateLimiter,
		subHandler:  subHandler,
		domain:      domain,
		sendDM:      sendDM,
		aclClient:   aclClient,
	}
}

// ProcessOutbound parses a DM as an outbound email and sends it.
// senderPubkeyHex is the Nostr pubkey of the DM author.
func (op *OutboundProcessor) ProcessOutbound(senderPubkeyHex, content string) error {
	// Check subscription
	if op.subHandler != nil && !op.subHandler.IsSubscribed(senderPubkeyHex) {
		op.reply(senderPubkeyHex, "You need an active subscription to send emails. Send \"subscribe\" to get started.")
		return nil
	}

	// Check rate limit
	if op.rateLimiter != nil {
		if err := op.rateLimiter.Check(senderPubkeyHex); err != nil {
			op.reply(senderPubkeyHex, fmt.Sprintf("Rate limit: %v", err))
			return nil
		}
	}

	// Parse DM content as email
	parsed, err := ParseDMContent(content)
	if err != nil {
		op.reply(senderPubkeyHex, fmt.Sprintf("Could not parse your message: %v", err))
		return nil
	}

	if len(parsed.To) == 0 {
		op.reply(senderPubkeyHex, "No recipients found. Start your message with:\nTo: recipient@example.com")
		return nil
	}

	// Build the from address: alias@domain or truncated-pubkey@domain
	fromLocal := senderPubkeyHex
	if op.aclClient != nil {
		if alias, err := op.aclClient.GetAliasByPubkey(senderPubkeyHex); err == nil && alias != "" {
			fromLocal = alias
		}
	}
	if fromLocal == senderPubkeyHex && len(fromLocal) > 16 {
		fromLocal = fromLocal[:16]
	}
	fromAddr := fromLocal + "@" + op.domain

	// Send via SMTP
	email := &bridgesmtp.OutboundEmail{
		From:    fromAddr,
		To:      parsed.To,
		Cc:      parsed.Cc,
		Bcc:     parsed.Bcc,
		Subject: parsed.Subject,
		Body:    parsed.Body,
	}

	if err := op.smtpClient.Send(email); err != nil {
		log.E.F("outbound email failed for %s: %v", senderPubkeyHex, err)
		op.reply(senderPubkeyHex, fmt.Sprintf("Email delivery failed: %v", err))
		return err
	}

	// Record for rate limiting
	if op.rateLimiter != nil {
		op.rateLimiter.Record(senderPubkeyHex)
	}

	// Confirm delivery
	var recips []string
	recips = append(recips, parsed.To...)
	recips = append(recips, parsed.Cc...)
	op.reply(senderPubkeyHex, fmt.Sprintf("Email sent to %s", strings.Join(recips, ", ")))

	log.I.F("outbound email from %s to %v sent", senderPubkeyHex, parsed.To)
	return nil
}

func (op *OutboundProcessor) reply(pubkeyHex, content string) {
	if op.sendDM == nil {
		return
	}
	if err := op.sendDM(pubkeyHex, content); err != nil {
		log.E.F("failed to send reply DM to %s: %v", pubkeyHex, err)
	}
}
