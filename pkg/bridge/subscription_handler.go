package bridge

import (
	"context"
	"fmt"
	"time"

	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"

	"next.orly.dev/pkg/acl"
	aclgrpc "next.orly.dev/pkg/acl/grpc"
)

// SubscriptionHandler manages the subscription flow:
// user sends "subscribe" → create invoice → poll for payment → activate → confirm.
type SubscriptionHandler struct {
	store          SubscriptionStore
	payments       *PaymentProcessor
	sendDM         func(pubkeyHex string, content string) error
	priceSats      int64
	aclClient      *aclgrpc.Client
	aliasPriceSats int64
	domain         string
}

// NewSubscriptionHandler creates a handler for subscription DM commands.
// sendDM is a callback that sends a DM reply to the user.
func NewSubscriptionHandler(
	store SubscriptionStore,
	payments *PaymentProcessor,
	sendDM func(pubkeyHex string, content string) error,
	priceSats int64,
	aclClient *aclgrpc.Client,
	aliasPriceSats int64,
	domain string,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		store:          store,
		payments:       payments,
		sendDM:         sendDM,
		priceSats:      priceSats,
		aclClient:      aclClient,
		aliasPriceSats: aliasPriceSats,
		domain:         domain,
	}
}

// HandleSubscribe processes a "subscribe" or "subscribe <alias>" command.
// It creates an invoice, sends it to the user, waits for payment,
// then activates the subscription and sends confirmation.
func (sh *SubscriptionHandler) HandleSubscribe(ctx context.Context, pubkeyHex, alias string) {
	// Determine price based on alias request
	price := sh.priceSats
	if alias != "" {
		price = sh.aliasPriceSats
		if price == 0 {
			price = sh.priceSats * 2
		}
	}

	// If alias requested, validate and check availability BEFORE creating invoice
	if alias != "" {
		if err := acl.ValidateAlias(alias); err != nil {
			sh.sendReply(pubkeyHex, fmt.Sprintf("Invalid alias: %v", err))
			return
		}
		if sh.aclClient != nil {
			taken, err := sh.aclClient.IsAliasTaken(alias)
			if err != nil {
				log.E.F("alias check failed for %s: %v", alias, err)
				sh.sendReply(pubkeyHex, "Failed to check alias availability. Please try again.")
				return
			}
			if taken {
				// Check if this pubkey already owns it (re-subscribe is ok)
				existingPubkey, _ := sh.aclClient.GetPubkeyByAlias(alias)
				if existingPubkey != pubkeyHex {
					sh.sendReply(pubkeyHex, fmt.Sprintf("Alias %q is already taken. Try a different alias.", alias))
					return
				}
			}
		}
	}

	month := 30 * 24 * time.Hour
	now := time.Now()

	// Look up existing subscription expiry.
	var currentExpiry time.Time
	if sh.aclClient != nil {
		sub, err := sh.aclClient.GetSubscription(pubkeyHex)
		if err == nil {
			currentExpiry = sub.ExpiresAt
		}
	} else {
		existing, err := sh.store.Get(pubkeyHex)
		if err == nil {
			currentExpiry = existing.ExpiresAt
		}
	}

	var expiresAt time.Time
	var paymentHash string

	if sh.payments == nil {
		// Free tier: max 2 months from now, can't re-subscribe until halfway through.
		if !currentExpiry.IsZero() && currentExpiry.After(now) {
			remaining := currentExpiry.Sub(now)
			if remaining > month/2 {
				sh.sendReply(pubkeyHex, fmt.Sprintf(
					"You can renew after %s (halfway through your current period).",
					now.Add(remaining-month/2).Format("2006-01-02"),
				))
				return
			}
		}
		expiresAt = now.Add(month)
		if !currentExpiry.IsZero() && currentExpiry.After(now) {
			expiresAt = currentExpiry.Add(month)
			// Cap at 2 months from now.
			cap := now.Add(2 * month)
			if expiresAt.After(cap) {
				expiresAt = cap
			}
		}
		log.I.F("free subscription activated for %s (alias=%q)", pubkeyHex, alias)
	} else {
		// Paid tier: extend from current expiry (stacks).
		baseTime := now
		if !currentExpiry.IsZero() && currentExpiry.After(now) {
			baseTime = currentExpiry
		}
		expiresAt = baseTime.Add(month)
		// Paid mode — create invoice and wait for payment
		invoice, err := sh.payments.CreateInvoice(ctx, price)
		if err != nil {
			log.E.F("failed to create subscription invoice for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Failed to create invoice. Please try again later.")
			return
		}

		desc := fmt.Sprintf("Marmot Email Bridge subscription: %d sats/month", price)
		if alias != "" {
			desc = fmt.Sprintf("Marmot Email Bridge subscription with alias %q: %d sats/month", alias, price)
		}
		sh.sendReply(pubkeyHex, fmt.Sprintf(
			"%s\n\nPay this Lightning invoice to activate:\n\n%s\n\n"+
				"The invoice expires in 10 minutes. "+
				"You'll receive a confirmation DM when payment is received.",
			desc,
			invoice.Bolt11,
		))

		payCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		status, err := sh.payments.WaitForPayment(payCtx, invoice.PaymentHash, 5*time.Second)
		if err != nil {
			log.D.F("subscription payment wait ended for %s: %v", pubkeyHex, err)
			return
		}
		paymentHash = status.PaymentHash
	}

	// Activate subscription
	if sh.aclClient != nil {
		if err := sh.aclClient.SubscribePubkey(pubkeyHex, expiresAt, paymentHash, alias); err != nil {
			log.E.F("failed to activate ACL subscription for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Failed to activate subscription. Contact the relay operator.")
			return
		}
		if alias != "" {
			if err := sh.aclClient.ClaimAlias(alias, pubkeyHex); err != nil {
				log.W.F("alias claim failed for %s → %s: %v", alias, pubkeyHex, err)
				sh.sendReply(pubkeyHex, fmt.Sprintf(
					"Subscription active (expires %s).\n\n"+
						"However, alias %q could not be claimed: %v\n"+
						"You can still send email using your npub address.",
					expiresAt.Format("2006-01-02"), alias, err,
				))
				return
			}
		}
	} else {
		sub := &Subscription{
			PubkeyHex:   pubkeyHex,
			ExpiresAt:   expiresAt,
			CreatedAt:   time.Now(),
			InvoiceHash: paymentHash,
		}
		if err := sh.store.Save(sub); err != nil {
			log.E.F("failed to save subscription for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Failed to activate subscription. Contact the relay operator.")
			return
		}
	}

	log.I.F("subscription activated for %s (alias=%q, expires %s)", pubkeyHex, alias, expiresAt.Format(time.RFC3339))

	email := sh.userEmail(pubkeyHex)
	confirmMsg := fmt.Sprintf(
		"Payment received! Your subscription is now active.\n\n"+
			"Your email address: %s\n"+
			"Expires: %s\n",
		email, expiresAt.Format("2006-01-02"),
	)
	if alias != "" {
		confirmMsg += fmt.Sprintf("Alias: %s@%s\n", alias, sh.domain)
	}
	confirmMsg += "\nYou can now send emails by DMing this bridge with email headers:\n\n" +
		"To: recipient@example.com\n" +
		"Subject: Your subject\n\n" +
		"Your message here."

	sh.sendReply(pubkeyHex, confirmMsg)
}

// HandleStatus replies with the user's subscription info.
func (sh *SubscriptionHandler) HandleStatus(pubkeyHex string) {
	email := sh.userEmail(pubkeyHex)

	if sh.aclClient != nil {
		sub, err := sh.aclClient.GetSubscription(pubkeyHex)
		if err != nil {
			sh.sendReply(pubkeyHex, "No active subscription found.")
			return
		}
		msg := fmt.Sprintf("Subscription status:\n\nYour email: %s\nExpires: %s\n", email, sub.ExpiresAt.Format("2006-01-02"))
		if sub.HasAlias {
			msg += fmt.Sprintf("Alias: %s@%s\n", sub.Alias, sh.domain)
		}
		remaining := time.Until(sub.ExpiresAt).Round(time.Hour)
		if remaining > 0 {
			msg += fmt.Sprintf("Time remaining: %v\n", remaining)
		} else {
			msg += "Status: EXPIRED\n"
		}
		sh.sendReply(pubkeyHex, msg)
		return
	}

	// File-store fallback
	sub, err := sh.store.Get(pubkeyHex)
	if err != nil {
		sh.sendReply(pubkeyHex, "No active subscription found.")
		return
	}
	remaining := time.Until(sub.ExpiresAt).Round(time.Hour)
	status := "active"
	if remaining <= 0 {
		status = "EXPIRED"
	}
	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Subscription status: %s\nYour email: %s\nExpires: %s\nTime remaining: %v",
		status, email, sub.ExpiresAt.Format("2006-01-02"), remaining,
	))
}

// userEmail returns "npub1...@domain" for the given hex pubkey.
func (sh *SubscriptionHandler) userEmail(pubkeyHex string) string {
	npub, err := bech32encoding.HexToNpub([]byte(pubkeyHex))
	if err != nil {
		return pubkeyHex + "@" + sh.domain
	}
	return string(npub) + "@" + sh.domain
}

func (sh *SubscriptionHandler) sendReply(pubkeyHex, content string) {
	if err := sh.sendDM(pubkeyHex, content); err != nil {
		log.E.F("failed to send DM reply to %s: %v", pubkeyHex, err)
	}
}

// IsSubscribed checks whether a user has an active subscription.
func (sh *SubscriptionHandler) IsSubscribed(pubkeyHex string) bool {
	if sh.aclClient != nil {
		subscribed, err := sh.aclClient.IsSubscribedPaid(pubkeyHex)
		if err != nil {
			return false
		}
		return subscribed
	}
	sub, err := sh.store.Get(pubkeyHex)
	if err != nil {
		return false
	}
	return sub.IsActive()
}
