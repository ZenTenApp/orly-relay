package bridge

import (
	"context"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"

	"git.smesh.lol/orly/pkg/acl"
	aclgrpc "git.smesh.lol/orly/pkg/acl/grpc"
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
// Plain subscribe is free and instant. Alias subscribe requires payment.
func (sh *SubscriptionHandler) HandleSubscribe(ctx context.Context, pubkeyHex, alias string) {
	if alias == "" {
		sh.handleFreeSubscribe(pubkeyHex)
	} else {
		sh.handleAliasSubscribe(ctx, pubkeyHex, alias)
	}
}

// handleFreeSubscribe activates npub-only email instantly, no payment.
func (sh *SubscriptionHandler) handleFreeSubscribe(pubkeyHex string) {
	month := 30 * 24 * time.Hour
	now := time.Now()

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

	// Can't renew until halfway through current period.
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

	expiresAt := now.Add(month)
	if !currentExpiry.IsZero() && currentExpiry.After(now) {
		expiresAt = currentExpiry.Add(month)
		cap := now.Add(2 * month)
		if expiresAt.After(cap) {
			expiresAt = cap
		}
	}

	if sh.aclClient != nil {
		if err := sh.aclClient.SubscribePubkey(pubkeyHex, expiresAt, "", ""); err != nil {
			log.E.F("failed to activate subscription for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Failed to activate subscription. Contact the relay operator.")
			return
		}
	} else {
		sub := &Subscription{PubkeyHex: pubkeyHex, ExpiresAt: expiresAt, CreatedAt: time.Now()}
		if err := sh.store.Save(sub); err != nil {
			log.E.F("failed to save subscription for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Failed to activate subscription. Contact the relay operator.")
			return
		}
	}

	email := sh.userEmail(pubkeyHex)
	log.I.F("free subscription activated for %s, email: %s", pubkeyHex, email)
	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Subscription active!\n\nYour email address: %s\nExpires: %s\nSend rate: 1 per 20 seconds\n\n"+
			"To send an email, DM this bridge with:\n\n"+
			"To: recipient@example.com\nSubject: Your subject\n\nYour message here.",
		email, expiresAt.Format("2006-01-02"),
	))
}

// handleAliasSubscribe validates alias, creates invoice, waits for payment, claims alias.
// One pubkey can have multiple aliases — each costs the same.
func (sh *SubscriptionHandler) handleAliasSubscribe(ctx context.Context, pubkeyHex, alias string) {
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
			existingPubkey, _ := sh.aclClient.GetPubkeyByAlias(alias)
			if existingPubkey != pubkeyHex {
				sh.sendReply(pubkeyHex, fmt.Sprintf("Alias %q is already taken. Try a different alias.", alias))
				return
			}
			sh.sendReply(pubkeyHex, fmt.Sprintf("You already own alias %q.", alias))
			return
		}
	}

	price := sh.aliasPriceSats
	if price == 0 {
		price = sh.priceSats
	}

	if sh.payments == nil {
		sh.sendReply(pubkeyHex, "Alias subscriptions require Lightning payment, which is not configured.")
		return
	}

	invoice, err := sh.payments.CreateInvoice(ctx, price)
	if err != nil {
		log.E.F("failed to create alias invoice for %s: %v", pubkeyHex, err)
		sh.sendReply(pubkeyHex, "Failed to create invoice. Please try again later.")
		return
	}

	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Alias %q: %d sats\n\nPay this Lightning invoice:\n\n%s\n\n"+
			"Expires in 10 minutes.",
		alias, price, invoice.Bolt11,
	))

	payCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	status, err := sh.payments.WaitForPayment(payCtx, invoice.PaymentHash, 5*time.Second)
	if err != nil {
		log.D.F("alias payment wait ended for %s: %v", pubkeyHex, err)
		return
	}

	// Ensure base subscription exists (extend or create).
	month := 30 * 24 * time.Hour
	now := time.Now()
	expiresAt := now.Add(month)
	if sh.aclClient != nil {
		sub, err := sh.aclClient.GetSubscription(pubkeyHex)
		if err == nil && sub.ExpiresAt.After(now) {
			expiresAt = sub.ExpiresAt.Add(month)
		}
		if err := sh.aclClient.SubscribePubkey(pubkeyHex, expiresAt, status.PaymentHash, alias); err != nil {
			log.E.F("failed to activate subscription for %s: %v", pubkeyHex, err)
			sh.sendReply(pubkeyHex, "Payment received but subscription failed. Contact the relay operator.")
			return
		}
		if err := sh.aclClient.ClaimAlias(alias, pubkeyHex); err != nil {
			log.W.F("alias claim failed for %s → %s: %v", alias, pubkeyHex, err)
			sh.sendReply(pubkeyHex, fmt.Sprintf("Payment received but alias %q could not be claimed: %v", alias, err))
			return
		}
	} else {
		sub := &Subscription{PubkeyHex: pubkeyHex, ExpiresAt: expiresAt, CreatedAt: now, InvoiceHash: status.PaymentHash}
		if err := sh.store.Save(sub); err != nil {
			log.E.F("failed to save subscription for %s: %v", pubkeyHex, err)
		}
	}

	aliasEmail := fmt.Sprintf("%s@%s", alias, sh.domain)
	npubEmail := sh.userEmail(pubkeyHex)
	log.I.F("alias %q claimed by %s, email: %s", alias, pubkeyHex, aliasEmail)
	sh.sendReply(pubkeyHex, fmt.Sprintf(
		"Payment received! Alias claimed.\n\nAlias email: %s\nnpub email: %s\nExpires: %s\nSend rate: 1 per 5 seconds",
		aliasEmail, npubEmail, expiresAt.Format("2006-01-02"),
	))
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
		msg := fmt.Sprintf("Subscription status:\n\nYour npub email: %s\nExpires: %s\n", email, sub.ExpiresAt.Format("2006-01-02"))
		if len(sub.Aliases) > 0 {
			for _, a := range sub.Aliases {
				msg += fmt.Sprintf("Alias email: %s@%s\n", a, sh.domain)
			}
		} else if sub.HasAlias {
			msg += fmt.Sprintf("Alias email: %s@%s\n", sub.Alias, sh.domain)
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
