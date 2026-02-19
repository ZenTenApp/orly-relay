package bridge

import (
	"context"
	"fmt"
	"time"

	"lol.mleku.dev/log"
)

// SubscriptionHandler manages the subscription flow:
// user sends "subscribe" → create invoice → poll for payment → activate → confirm.
type SubscriptionHandler struct {
	store     SubscriptionStore
	payments  *PaymentProcessor
	sendDM    func(pubkeyHex string, content string) error
	priceSats int64
}

// NewSubscriptionHandler creates a handler for subscription DM commands.
// sendDM is a callback that sends a DM reply to the user.
func NewSubscriptionHandler(
	store SubscriptionStore,
	payments *PaymentProcessor,
	sendDM func(pubkeyHex string, content string) error,
	priceSats int64,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		store:     store,
		payments:  payments,
		sendDM:    sendDM,
		priceSats: priceSats,
	}
}

// HandleSubscribe processes a "subscribe" command from a user.
// It creates an invoice, sends it to the user, waits for payment,
// then activates the subscription and sends confirmation.
func (sh *SubscriptionHandler) HandleSubscribe(ctx context.Context, pubkeyHex string) {
	// Check for existing active subscription
	existing, err := sh.store.Get(pubkeyHex)
	if err == nil && existing.IsActive() {
		remaining := time.Until(existing.ExpiresAt).Round(time.Hour)
		sh.sendReply(pubkeyHex, fmt.Sprintf(
			"You already have an active subscription (%v remaining). "+
				"Send \"subscribe\" again after it expires to renew.",
			remaining,
		))
		return
	}

	// Create invoice
	if sh.payments == nil {
		log.E.F("subscription handler has no payment processor configured")
		sh.sendReply(pubkeyHex, "Subscriptions are not available — payment processor not configured.")
		return
	}

	invoice, err := sh.payments.CreateSubscriptionInvoice(ctx)
	if err != nil {
		log.E.F("failed to create subscription invoice for %s: %v", pubkeyHex, err)
		sh.sendReply(pubkeyHex, "Failed to create invoice. Please try again later.")
		return
	}

	// Send invoice to user
	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Marmot Email Bridge subscription: %d sats/month\n\n"+
			"Pay this Lightning invoice to activate:\n\n%s\n\n"+
			"The invoice expires in 10 minutes. "+
			"You'll receive a confirmation DM when payment is received.",
		sh.priceSats,
		invoice.Bolt11,
	))

	// Wait for payment in background (10 minute timeout)
	payCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	status, err := sh.payments.WaitForPayment(payCtx, invoice.PaymentHash, 5*time.Second)
	if err != nil {
		// Timed out or context cancelled — no action needed.
		// User can send "subscribe" again.
		log.D.F("subscription payment wait ended for %s: %v", pubkeyHex, err)
		return
	}

	// Payment received — activate subscription
	now := time.Now()
	sub := &Subscription{
		PubkeyHex:   pubkeyHex,
		ExpiresAt:   now.Add(30 * 24 * time.Hour), // 30 days
		CreatedAt:   now,
		InvoiceHash: status.PaymentHash,
	}

	if err := sh.store.Save(sub); err != nil {
		log.E.F("failed to save subscription for %s: %v", pubkeyHex, err)
		sh.sendReply(pubkeyHex, "Payment received but failed to activate subscription. Contact the relay operator.")
		return
	}

	log.I.F("subscription activated for %s (expires %s)", pubkeyHex, sub.ExpiresAt.Format(time.RFC3339))

	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Payment received! Your Marmot Email Bridge subscription is now active.\n\n"+
			"Expires: %s\n\n"+
			"You can now send emails by DMing this bridge with email headers:\n\n"+
			"To: recipient@example.com\n"+
			"Subject: Your subject\n\n"+
			"Your message here.",
		sub.ExpiresAt.Format("2006-01-02"),
	))
}

func (sh *SubscriptionHandler) sendReply(pubkeyHex, content string) {
	if err := sh.sendDM(pubkeyHex, content); err != nil {
		log.E.F("failed to send DM reply to %s: %v", pubkeyHex, err)
	}
}

// IsSubscribed checks whether a user has an active subscription.
func (sh *SubscriptionHandler) IsSubscribed(pubkeyHex string) bool {
	sub, err := sh.store.Get(pubkeyHex)
	if err != nil {
		return false
	}
	return sub.IsActive()
}
