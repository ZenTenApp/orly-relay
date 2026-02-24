package bridge

import (
	"context"

	"lol.mleku.dev/log"
)

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
	log.D.F("unrecognized DM from %s", senderPubkeyHex)
	r.reply(senderPubkeyHex,
		"Marmot Email Bridge\n\n"+
			"Commands:\n"+
			"  subscribe — Subscribe with npub-only email\n"+
			"  subscribe <alias> — Subscribe with a custom email alias\n"+
			"  status — Check your subscription status\n\n"+
			"To send an email, format your DM like:\n\n"+
			"To: recipient@example.com\n"+
			"Subject: Your subject\n\n"+
			"Your message here.",
	)
}

func (r *Router) reply(pubkeyHex, content string) {
	if r.sendDM == nil {
		return
	}
	if err := r.sendDM(pubkeyHex, content); err != nil {
		log.E.F("failed to send reply DM to %s: %v", pubkeyHex, err)
	}
}
