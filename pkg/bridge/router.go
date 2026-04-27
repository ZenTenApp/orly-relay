package bridge

import (
	"context"

	"git.smesh.lol/orly/pkg/lol/log"
)

const helpText = "Marmot Email Bridge\n\n" +
	"Commands:\n" +
	"  help — Show this message\n" +
	"  subscribe — Free tier: npub-only email (20s send limit)\n" +
	"  subscribe <alias> — Custom alias + paid tier (5s send limit)\n" +
	"  status — Check your subscription status\n\n" +
	"To send an email, DM this bridge with:\n\n" +
	"To: recipient@example.com\n" +
	"Subject: Your subject\n\n" +
	"Your message here.\n\n" +
	"All standard email headers (Cc, Bcc, Attachment) will also be included if added."

// Router dispatches incoming DMs to the appropriate handler.
type Router struct {
	subHandler *SubscriptionHandler
	outbound   *OutboundProcessor
	sendDM     func(pubkeyHex string, content string) error
}

// NewRouter creates a DM router.
func NewRouter(
	subHandler *SubscriptionHandler,
	outbound *OutboundProcessor,
	sendDM func(pubkeyHex string, content string) error,
) *Router {
	return &Router{
		subHandler: subHandler,
		outbound:   outbound,
		sendDM:     sendDM,
	}
}

// RouteDM processes an incoming DM and routes it to the right handler.
func (r *Router) RouteDM(ctx context.Context, senderPubkeyHex, content string) {
	// Check for commands first
	result := ClassifyDMFull(content)

	switch result.Command {
	case DMCommandSubscribe:
		log.I.F("subscribe command from %s (alias=%q)", senderPubkeyHex, result.Alias)
		if r.subHandler != nil {
			// Run in goroutine — HandleSubscribe blocks for up to 10 minutes
			// waiting for payment, and must not block the event processing loop.
			go r.subHandler.HandleSubscribe(ctx, senderPubkeyHex, result.Alias)
		} else {
			r.reply(senderPubkeyHex, "Subscriptions are not configured on this bridge.")
		}
		return

	case DMCommandStatus:
		log.I.F("status command from %s", senderPubkeyHex)
		if r.subHandler != nil {
			r.subHandler.HandleStatus(senderPubkeyHex)
		} else {
			r.reply(senderPubkeyHex, "Subscriptions are not configured on this bridge.")
		}
		return

	case DMCommandHelp:
		log.I.F("help command from %s", senderPubkeyHex)
		r.reply(senderPubkeyHex, helpText)
		return
	}

	// Check if it's an outbound email
	if IsOutboundEmail(content) {
		log.I.F("outbound email from %s", senderPubkeyHex)
		if r.outbound != nil {
			r.outbound.ProcessOutbound(senderPubkeyHex, content)
		} else {
			r.reply(senderPubkeyHex, "Outbound email is not configured on this bridge.")
		}
		return
	}

	// Not a recognized command or email — auto-reply with help
	log.I.F("unrecognized DM from %s", senderPubkeyHex)
	r.reply(senderPubkeyHex, helpText)
}

// SendWelcome sends the help text to a new peer (triggered by group establishment).
func (r *Router) SendWelcome(pubkeyHex string) {
	r.reply(pubkeyHex, helpText)
}

func (r *Router) reply(pubkeyHex, content string) {
	if r.sendDM == nil {
		log.E.F("sendDM callback is nil, cannot reply to %s", pubkeyHex)
		return
	}
	if err := r.sendDM(pubkeyHex, content); err != nil {
		log.E.F("failed to send reply DM to %s: %v", pubkeyHex, err)
	} else {
		log.I.F("sent reply DM to %s (%d bytes)", pubkeyHex, len(content))
	}
}
