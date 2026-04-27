package app

import (
	"context"
	// std hex not used; use project hex encoder instead
	"fmt"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/protocol/nwc"
)

// PaymentProcessor handles NWC payment notifications and updates subscriptions
type PaymentProcessor struct {
	nwcClient    *nwc.Client
	db           *database.D
	config       *config.C
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	dashboardURL string
}

// NewPaymentProcessor creates a new payment processor
func NewPaymentProcessor(
	ctx context.Context, cfg *config.C, db *database.D,
) (pp *PaymentProcessor, err error) {
	if cfg.NWCUri == "" {
		return nil, fmt.Errorf("NWC URI not configured")
	}

	var nwcClient *nwc.Client
	if nwcClient, err = nwc.NewClient(cfg.NWCUri); chk.E(err) {
		return nil, fmt.Errorf("failed to create NWC client: %w", err)
	}

	c, cancel := context.WithCancel(ctx)

	pp = &PaymentProcessor{
		nwcClient: nwcClient,
		db:        db,
		config:    cfg,
		ctx:       c,
		cancel:    cancel,
	}

	return pp, nil
}

// Start begins listening for payment notifications
func (pp *PaymentProcessor) Start() error {
	// start NWC notifications listener
	pp.wg.Add(1)
	go func() {
		defer pp.wg.Done()
		if err := pp.listenForPayments(); err != nil {
			log.E.F("payment processor error: %v", err)
		}
	}()
	// start periodic follow-list sync if subscriptions are enabled
	if pp.config != nil && pp.config.SubscriptionEnabled {
		pp.wg.Add(1)
		go func() {
			defer pp.wg.Done()
			pp.runFollowSyncLoop()
		}()
		// start daily subscription checker
		pp.wg.Add(1)
		go func() {
			defer pp.wg.Done()
			pp.runDailySubscriptionChecker()
		}()
	}
	return nil
}

// Stop gracefully stops the payment processor
func (pp *PaymentProcessor) Stop() {
	if pp.cancel != nil {
		pp.cancel()
	}
	pp.wg.Wait()
}

// listenForPayments subscribes to NWC notifications and processes payments
func (pp *PaymentProcessor) listenForPayments() error {
	return pp.nwcClient.SubscribeNotifications(pp.ctx, pp.handleNotification)
}

// runFollowSyncLoop periodically syncs the relay identity follow list with active subscribers
func (pp *PaymentProcessor) runFollowSyncLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	// do an initial sync shortly after start
	_ = pp.syncFollowList()
	for {
		select {
		case <-pp.ctx.Done():
			return
		case <-t.C:
			if err := pp.syncFollowList(); err != nil {
				log.W.F("follow list sync failed: %v", err)
			}
		}
	}
}

// runDailySubscriptionChecker checks once daily for subscription expiry warnings and trial reminders
func (pp *PaymentProcessor) runDailySubscriptionChecker() {
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	// do an initial check shortly after start
	_ = pp.checkSubscriptionStatus()
	for {
		select {
		case <-pp.ctx.Done():
			return
		case <-t.C:
			if err := pp.checkSubscriptionStatus(); err != nil {
				log.W.F("subscription status check failed: %v", err)
			}
		}
	}
}

// syncFollowList builds a kind-3 event from the relay identity containing only active subscribers
func (pp *PaymentProcessor) syncFollowList() error {
	// ensure we have a relay identity secret
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return nil // nothing to do if no identity
	}
	// collect active subscribers
	actives, err := pp.getActiveSubscriberPubkeys()
	if err != nil {
		return err
	}
	// signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return err
	}
	// build follow list event
	ev := event.New()
	ev.Kind = kind.FollowList.K
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Tags = tag.NewS()
	for _, pk := range actives {
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("p", hex.Enc(pk)))
	}
	// sign and save
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return err
	}
	log.I.F(
		"updated relay follow list with %d active subscribers", len(actives),
	)
	return nil
}

// getActiveSubscriberPubkeys scans the subscription records and returns active ones
func (pp *PaymentProcessor) getActiveSubscriberPubkeys() ([][]byte, error) {
	prefix := []byte("sub:")
	now := time.Now()
	var out [][]byte
	err := pp.db.DB.View(
		func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				// key format: sub:<hexpub>
				hexpub := string(key[len(prefix):])
				var sub database.Subscription
				if err := item.Value(
					func(val []byte) error {
						return json.Unmarshal(val, &sub)
					},
				); err != nil {
					return err
				}
				if now.Before(sub.TrialEnd) || (!sub.PaidUntil.IsZero() && now.Before(sub.PaidUntil)) {
					if b, err := hex.Dec(hexpub); err == nil {
						out = append(out, b)
					}
				}
			}
			return nil
		},
	)
	return out, err
}

// checkSubscriptionStatus scans all subscriptions and creates warning/reminder notes
func (pp *PaymentProcessor) checkSubscriptionStatus() error {
	prefix := []byte("sub:")
	now := time.Now()
	sevenDaysFromNow := now.AddDate(0, 0, 7)

	return pp.db.DB.View(
		func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				// key format: sub:<hexpub>
				hexpub := string(key[len(prefix):])

				var sub database.Subscription
				if err := item.Value(
					func(val []byte) error {
						return json.Unmarshal(val, &sub)
					},
				); err != nil {
					continue // skip invalid subscription records
				}

				pubkey, err := hex.Dec(hexpub)
				if err != nil {
					continue // skip invalid pubkey
				}

				// Check if paid subscription is expiring in 7 days
				if !sub.PaidUntil.IsZero() {
					// Format dates for comparison (ignore time component)
					paidUntilDate := sub.PaidUntil.Truncate(24 * time.Hour)
					sevenDaysDate := sevenDaysFromNow.Truncate(24 * time.Hour)

					if paidUntilDate.Equal(sevenDaysDate) {
						go pp.createExpiryWarningNote(pubkey, sub.PaidUntil)
					}
				}

				// Check if user is on trial (no paid subscription, trial not expired)
				if sub.PaidUntil.IsZero() && now.Before(sub.TrialEnd) {
					go pp.createTrialReminderNote(pubkey, sub.TrialEnd)
				}
			}
			return nil
		},
	)
}

// createExpiryWarningNote creates a warning note for users whose paid subscription expires in 7 days
func (pp *PaymentProcessor) createExpiryWarningNote(
	userPubkey []byte, expiryTime time.Time,
) error {
	// Get relay identity secret to sign the note
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return fmt.Errorf("no relay identity configured")
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return fmt.Errorf("failed to initialize signer: %w", err)
	}

	monthlyPrice := pp.config.MonthlyPriceSats
	if monthlyPrice <= 0 {
		monthlyPrice = 6000
	}

	// Get relay npub for content link
	relayNpubForContent, err := bech32encoding.BinToNpub(sign.Pub())
	if err != nil {
		return fmt.Errorf("failed to encode relay npub: %w", err)
	}

	// Create the warning note content
	content := fmt.Sprintf(
		`⚠️ Subscription Expiring Soon ⚠️

Your paid subscription to this relay will expire in 7 days on %s.

💰 To extend your subscription:
- Monthly price: %d sats
- Zap this note with your payment amount
- Each %d sats = 30 days of access

⚡ Payment Instructions:
1. Use any Lightning wallet that supports zaps
2. Zap this note with your payment
3. Your subscription will be automatically extended

Don't lose access to your private relay! Extend your subscription today.

Relay: nostr:%s

Log in to the relay dashboard to access your configuration at: %s`,
		expiryTime.Format("2006-01-02 15:04:05 UTC"), monthlyPrice,
		monthlyPrice, string(relayNpubForContent), pp.getDashboardURL(),
	)

	// Build the event
	ev := event.New()
	ev.Kind = kind.TextNote.K // Kind 1 for text note
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Add "p" tag for the user
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("p", hex.Enc(userPubkey)))

	// Add expiration tag (5 days from creation)
	noteExpiry := time.Now().AddDate(0, 0, 5)
	*ev.Tags = append(
		*ev.Tags,
		tag.NewFromAny("expiration", fmt.Sprintf("%d", noteExpiry.Unix())),
	)

	// Add "private" tag with authorized npubs (user and relay)
	var authorizedNpubs []string

	// Add user npub
	userNpub, err := bech32encoding.BinToNpub(userPubkey)
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(userNpub))
	}

	// Add relay npub
	relayNpub, err := bech32encoding.BinToNpub(sign.Pub())
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(relayNpub))
	}

	// Create the private tag with comma-separated npubs
	if len(authorizedNpubs) > 0 {
		privateTagValue := strings.Join(authorizedNpubs, ",")
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("private", privateTagValue))
		// Add protected "-" tag to mark this event as protected
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("-", ""))
	}

	// Add a special tag to mark this as an expiry warning
	*ev.Tags = append(
		*ev.Tags, tag.NewFromAny("warning", "subscription-expiry"),
	)

	// Sign and save the event
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return fmt.Errorf("failed to save expiry warning note: %w", err)
	}

	log.I.F(
		"created expiry warning note for user %s (expires %s)",
		hex.Enc(userPubkey), expiryTime.Format("2006-01-02"),
	)
	return nil
}

// createTrialReminderNote creates a reminder note for users on trial to support the relay
func (pp *PaymentProcessor) createTrialReminderNote(
	userPubkey []byte, trialEnd time.Time,
) error {
	// Get relay identity secret to sign the note
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return fmt.Errorf("no relay identity configured")
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return fmt.Errorf("failed to initialize signer: %w", err)
	}

	monthlyPrice := pp.config.MonthlyPriceSats
	if monthlyPrice <= 0 {
		monthlyPrice = 6000
	}

	// Calculate daily rate
	dailyRate := monthlyPrice / 30

	// Get relay npub for content link
	relayNpubForContent, err := bech32encoding.BinToNpub(sign.Pub())
	if err != nil {
		return fmt.Errorf("failed to encode relay npub: %w", err)
	}

	// Create the reminder note content
	content := fmt.Sprintf(
		`🆓 Free Trial Reminder 🆓

You're currently using this relay for FREE! Your trial expires on %s.

🙏 Support Relay Operations:
This relay provides you with private, censorship-resistant communication. Please consider supporting its continued operation.

💰 Subscription Details:
- Monthly price: %d sats (%d sats/day)
- Fair pricing for premium service
- Helps keep the relay running 24/7

⚡ How to Subscribe:
Simply zap this note with your payment amount:
- Each %d sats = 30 days of access
- Payment is processed automatically
- No account setup required

Thank you for considering supporting decentralized communication!

Relay: nostr:%s

Log in to the relay dashboard to access your configuration at: %s`,
		trialEnd.Format("2006-01-02 15:04:05 UTC"), monthlyPrice, dailyRate,
		monthlyPrice, string(relayNpubForContent), pp.getDashboardURL(),
	)

	// Build the event
	ev := event.New()
	ev.Kind = kind.TextNote.K // Kind 1 for text note
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Add "p" tag for the user
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("p", hex.Enc(userPubkey)))

	// Add expiration tag (5 days from creation)
	noteExpiry := time.Now().AddDate(0, 0, 5)
	*ev.Tags = append(
		*ev.Tags,
		tag.NewFromAny("expiration", fmt.Sprintf("%d", noteExpiry.Unix())),
	)

	// Add "private" tag with authorized npubs (user and relay)
	var authorizedNpubs []string

	// Add user npub
	userNpub, err := bech32encoding.BinToNpub(userPubkey)
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(userNpub))
	}

	// Add relay npub
	relayNpub, err := bech32encoding.BinToNpub(sign.Pub())
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(relayNpub))
	}

	// Create the private tag with comma-separated npubs
	if len(authorizedNpubs) > 0 {
		privateTagValue := strings.Join(authorizedNpubs, ",")
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("private", privateTagValue))
		// Add protected "-" tag to mark this event as protected
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("-", ""))
	}

	// Add a special tag to mark this as a trial reminder
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("reminder", "trial-support"))

	// Sign and save the event
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return fmt.Errorf("failed to save trial reminder note: %w", err)
	}

	log.I.F(
		"created trial reminder note for user %s (trial ends %s)",
		hex.Enc(userPubkey), trialEnd.Format("2006-01-02"),
	)
	return nil
}

// handleNotification processes incoming payment notifications
func (pp *PaymentProcessor) handleNotification(
	notificationType string, notification map[string]any,
) error {
	// Only process payment_received notifications
	if notificationType != "payment_received" {
		return nil
	}

	amount, ok := notification["amount"].(float64)
	if !ok {
		return fmt.Errorf("invalid amount")
	}

	// Prefer explicit payer/relay pubkeys if provided in metadata
	var payerPubkey []byte
	var userNpub string
	var metadata map[string]any
	if md, ok := notification["metadata"].(map[string]any); ok {
		metadata = md
		if s, ok := metadata["payer_pubkey"].(string); ok && s != "" {
			if pk, err := decodeAnyPubkey(s); err == nil {
				payerPubkey = pk
			}
		}
		if payerPubkey == nil {
			if s, ok := metadata["sender_pubkey"].(string); ok && s != "" { // alias
				if pk, err := decodeAnyPubkey(s); err == nil {
					payerPubkey = pk
				}
			}
		}
		// Optional: the intended subscriber npub (for backwards compat)
		if userNpub == "" {
			if npubField, ok := metadata["npub"].(string); ok {
				userNpub = npubField
			}
		}
		// If relay identity pubkey is provided, verify it matches ours
		if s, ok := metadata["relay_pubkey"].(string); ok && s != "" {
			if rpk, err := decodeAnyPubkey(s); err == nil {
				if skb, err := pp.db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
					signer := p8k.MustNew()
					if err := signer.InitSec(skb); err == nil {
						if !strings.EqualFold(
							hex.Enc(rpk), hex.Enc(signer.Pub()),
						) {
							log.W.F(
								"relay_pubkey in payment metadata does not match this relay identity: got %s want %s",
								hex.Enc(rpk), hex.Enc(signer.Pub()),
							)
						}
					}
				}
			}
		}
	}

	// Fallback: extract npub from description or metadata
	description, _ := notification["description"].(string)
	if userNpub == "" {
		userNpub = pp.extractNpubFromDescription(description)
	}

	var pubkey []byte
	var err error
	if payerPubkey != nil {
		pubkey = payerPubkey
	} else {
		if userNpub == "" {
			return fmt.Errorf("no payer_pubkey or npub provided in payment notification")
		}
		pubkey, err = pp.npubToPubkey(userNpub)
		if err != nil {
			return fmt.Errorf("invalid npub: %w", err)
		}
	}

	satsReceived := int64(amount / 1000)
	
	// Parse zap memo for blossom service level
	blossomLevel := pp.parseBlossomServiceLevel(description, metadata)
	
	// Calculate subscription days (for relay access)
	monthlyPrice := pp.config.MonthlyPriceSats
	if monthlyPrice <= 0 {
		monthlyPrice = 6000
	}

	days := int((float64(satsReceived) / float64(monthlyPrice)) * 30)
	if days < 1 {
		return fmt.Errorf("payment amount too small")
	}

	// Extend relay subscription
	if err := pp.db.ExtendSubscription(pubkey, days); err != nil {
		return fmt.Errorf("failed to extend subscription: %w", err)
	}

	// If blossom service level specified, extend blossom subscription
	if blossomLevel != "" {
		if err := pp.extendBlossomSubscription(pubkey, satsReceived, blossomLevel, days); err != nil {
			log.W.F("failed to extend blossom subscription: %v", err)
			// Don't fail the payment if blossom subscription fails
		}
	}

	// Record payment history
	invoice, _ := notification["invoice"].(string)
	preimage, _ := notification["preimage"].(string)
	if err := pp.db.RecordPayment(
		pubkey, satsReceived, invoice, preimage,
	); err != nil {
		log.E.F("failed to record payment: %v", err)
	}

	// Log helpful identifiers
	var payerHex = hex.Enc(pubkey)
	if userNpub == "" {
		log.I.F(
			"payment processed: payer %s %d sats -> %d days", payerHex,
			satsReceived, days,
		)
	} else {
		log.I.F(
			"payment processed: %s (%s) %d sats -> %d days", userNpub, payerHex,
			satsReceived, days,
		)
	}

	// Update ACL follows cache and relay follow list immediately
	if pp.config != nil && pp.config.ACLMode == "follows" {
		acl.Registry.AddFollow(pubkey)
	}
	// Trigger an immediate follow-list sync in background (best-effort)
	go func() { _ = pp.syncFollowList() }()

	// Create a note with payment confirmation and private tag
	if err := pp.createPaymentNote(pubkey, satsReceived, days); err != nil {
		log.E.F("failed to create payment note: %v", err)
	}

	return nil
}

// createPaymentNote creates a note recording the payment with private tag for authorization
func (pp *PaymentProcessor) createPaymentNote(
	payerPubkey []byte, satsReceived int64, days int,
) error {
	// Get relay identity secret to sign the note
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return fmt.Errorf("no relay identity configured")
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return fmt.Errorf("failed to initialize signer: %w", err)
	}

	// Get subscription info to determine expiry
	sub, err := pp.db.GetSubscription(payerPubkey)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	var expiryTime time.Time
	if sub != nil && !sub.PaidUntil.IsZero() {
		expiryTime = sub.PaidUntil
	} else {
		expiryTime = time.Now().AddDate(0, 0, days)
	}

	// Get relay npub for content link
	relayNpubForContent, err := bech32encoding.BinToNpub(sign.Pub())
	if err != nil {
		return fmt.Errorf("failed to encode relay npub: %w", err)
	}

	// Create the note content with nostr:npub link and dashboard link
	content := fmt.Sprintf(
		"Payment received: %d sats for %d days. Subscription expires: %s\n\nRelay: nostr:%s\n\nLog in to the relay dashboard to access your configuration at: %s",
		satsReceived, days, expiryTime.Format("2006-01-02 15:04:05 UTC"),
		string(relayNpubForContent), pp.getDashboardURL(),
	)

	// Build the event
	ev := event.New()
	ev.Kind = kind.TextNote.K // Kind 1 for text note
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Add "p" tag for the payer
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("p", hex.Enc(payerPubkey)))

	// Add expiration tag (5 days from creation)
	noteExpiry := time.Now().AddDate(0, 0, 5)
	*ev.Tags = append(
		*ev.Tags,
		tag.NewFromAny("expiration", fmt.Sprintf("%d", noteExpiry.Unix())),
	)

	// Add "private" tag with authorized npubs (payer and relay)
	var authorizedNpubs []string

	// Add payer npub
	payerNpub, err := bech32encoding.BinToNpub(payerPubkey)
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(payerNpub))
	}

	// Add relay npub
	relayNpub, err := bech32encoding.BinToNpub(sign.Pub())
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(relayNpub))
	}

	// Create the private tag with comma-separated npubs
	if len(authorizedNpubs) > 0 {
		privateTagValue := strings.Join(authorizedNpubs, ",")
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("private", privateTagValue))
		// Add protected "-" tag to mark this event as protected
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("-", ""))
	}

	// Sign and save the event
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return fmt.Errorf("failed to save payment note: %w", err)
	}

	log.I.F(
		"created payment note for %s with private authorization",
		hex.Enc(payerPubkey),
	)
	return nil
}

// CreateWelcomeNote creates a welcome note for first-time users with private tag for authorization
func (pp *PaymentProcessor) CreateWelcomeNote(userPubkey []byte) error {
	// Get relay identity secret to sign the note
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return fmt.Errorf("no relay identity configured")
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return fmt.Errorf("failed to initialize signer: %w", err)
	}

	monthlyPrice := pp.config.MonthlyPriceSats
	if monthlyPrice <= 0 {
		monthlyPrice = 6000
	}

	// Get relay npub for content link
	relayNpubForContent, err := bech32encoding.BinToNpub(sign.Pub())
	if err != nil {
		return fmt.Errorf("failed to encode relay npub: %w", err)
	}

	// Get user npub for personalized greeting
	userNpub, err := bech32encoding.BinToNpub(userPubkey)
	if err != nil {
		return fmt.Errorf("failed to encode user npub: %w", err)
	}

	// Create the welcome note content with privacy notice and personalized greeting
	content := fmt.Sprintf(
		`This note is only visible to you

Hi nostr:%s

Welcome to the relay! 🎉

You have a FREE 30-day trial that started when you first logged in.

💰 Subscription Details:
- Monthly price: %d sats
- Trial period: 30 days from first login

💡 How to Subscribe:
To extend your subscription after the trial ends, simply zap this note with the amount you want to pay. Each %d sats = 30 days of access.

⚡ Payment Instructions:
1. Use any Lightning wallet that supports zaps
2. Zap this note with your payment
3. Your subscription will be automatically extended

Relay: nostr:%s

Log in to the relay dashboard to access your configuration at: %s

Enjoy your time on the relay!`, string(userNpub), monthlyPrice, monthlyPrice,
		string(relayNpubForContent), pp.getDashboardURL(),
	)

	// Build the event
	ev := event.New()
	ev.Kind = kind.TextNote.K // Kind 1 for text note
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Add "p" tag for the user with mention in third field
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("p", hex.Enc(userPubkey), "", "mention"))

	// Add expiration tag (5 days from creation)
	noteExpiry := time.Now().AddDate(0, 0, 5)
	*ev.Tags = append(
		*ev.Tags,
		tag.NewFromAny("expiration", fmt.Sprintf("%d", noteExpiry.Unix())),
	)

	// Add "private" tag with authorized npubs (user and relay)
	var authorizedNpubs []string

	// Add user npub (already encoded above)
	authorizedNpubs = append(authorizedNpubs, string(userNpub))

	// Add relay npub
	relayNpub, err := bech32encoding.BinToNpub(sign.Pub())
	if err == nil {
		authorizedNpubs = append(authorizedNpubs, string(relayNpub))
	}

	// Create the private tag with comma-separated npubs
	if len(authorizedNpubs) > 0 {
		privateTagValue := strings.Join(authorizedNpubs, ",")
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("private", privateTagValue))
		// Add protected "-" tag to mark this event as protected
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("-", ""))
	}

	// Add a special tag to mark this as a welcome note
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("welcome", "first-time-user"))

	// Sign and save the event
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return fmt.Errorf("failed to save welcome note: %w", err)
	}

	log.I.F("created welcome note for first-time user %s", hex.Enc(userPubkey))
	return nil
}

// SetDashboardURL sets the dynamic dashboard URL based on HTTP request
func (pp *PaymentProcessor) SetDashboardURL(url string) {
	pp.dashboardURL = url
}

// getDashboardURL returns the dashboard URL for the relay
func (pp *PaymentProcessor) getDashboardURL() string {
	// Use dynamic URL if available
	if pp.dashboardURL != "" {
		return pp.dashboardURL
	}
	// Fallback to static config
	if pp.config.RelayURL != "" {
		return pp.config.RelayURL
	}
	// Default fallback if no URL is configured
	return "https://your-relay.example.com"
}

// extractNpubFromDescription extracts an npub from the payment description
func (pp *PaymentProcessor) extractNpubFromDescription(description string) string {
	// check if the entire description is just an npub
	description = strings.TrimSpace(description)
	if strings.HasPrefix(description, "npub1") && len(description) == 63 {
		return description
	}

	// Look for npub1... pattern in the description
	parts := strings.Fields(description)
	for _, part := range parts {
		if strings.HasPrefix(part, "npub1") && len(part) == 63 {
			return part
		}
	}

	return ""
}

// npubToPubkey converts an npub string to pubkey bytes
func (pp *PaymentProcessor) npubToPubkey(npubStr string) ([]byte, error) {
	// Validate npub format
	if !strings.HasPrefix(npubStr, "npub1") || len(npubStr) != 63 {
		return nil, fmt.Errorf("invalid npub format")
	}

	// Decode using bech32encoding
	prefix, value, err := bech32encoding.Decode([]byte(npubStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode npub: %w", err)
	}

	if !strings.EqualFold(string(prefix), "npub") {
		return nil, fmt.Errorf("invalid prefix: %s", string(prefix))
	}

	pubkey, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("decoded value is not []byte")
	}

	return pubkey, nil
}

// parseBlossomServiceLevel parses the zap memo for a blossom service level specification
// Format: "blossom:level" or "blossom:level:storage_mb" in description or metadata memo field
func (pp *PaymentProcessor) parseBlossomServiceLevel(
	description string, metadata map[string]any,
) string {
	// Check metadata memo field first
	if metadata != nil {
		if memo, ok := metadata["memo"].(string); ok && memo != "" {
			if level := pp.extractBlossomLevelFromMemo(memo); level != "" {
				return level
			}
		}
	}

	// Check description
	if description != "" {
		if level := pp.extractBlossomLevelFromMemo(description); level != "" {
			return level
		}
	}

	return ""
}

// extractBlossomLevelFromMemo extracts blossom service level from memo text
// Supports formats: "blossom:basic", "blossom:premium", "blossom:basic:100"
func (pp *PaymentProcessor) extractBlossomLevelFromMemo(memo string) string {
	// Look for "blossom:" prefix
	parts := strings.Fields(memo)
	for _, part := range parts {
		if strings.HasPrefix(part, "blossom:") {
			// Extract level name (e.g., "basic", "premium")
			levelPart := strings.TrimPrefix(part, "blossom:")
			// Remove any storage specification (e.g., ":100")
			if colonIdx := strings.Index(levelPart, ":"); colonIdx > 0 {
				levelPart = levelPart[:colonIdx]
			}
			// Validate level exists in config
			if pp.isValidBlossomLevel(levelPart) {
				return levelPart
			}
		}
	}
	return ""
}

// isValidBlossomLevel checks if a service level is configured
func (pp *PaymentProcessor) isValidBlossomLevel(level string) bool {
	if pp.config == nil || pp.config.BlossomServiceLevels == "" {
		return false
	}

	// Parse service levels from config
	levels := strings.Split(pp.config.BlossomServiceLevels, ",")
	for _, l := range levels {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, level+":") {
			return true
		}
	}
	return false
}

// parseServiceLevelStorage parses storage quota in MB per sat per month for a service level
func (pp *PaymentProcessor) parseServiceLevelStorage(level string) (int64, error) {
	if pp.config == nil || pp.config.BlossomServiceLevels == "" {
		return 0, fmt.Errorf("blossom service levels not configured")
	}

	levels := strings.Split(pp.config.BlossomServiceLevels, ",")
	for _, l := range levels {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, level+":") {
			parts := strings.Split(l, ":")
			if len(parts) >= 2 {
				var storageMB float64
				if _, err := fmt.Sscanf(parts[1], "%f", &storageMB); err != nil {
					return 0, fmt.Errorf("invalid storage format: %w", err)
				}
				return int64(storageMB), nil
			}
		}
	}
	return 0, fmt.Errorf("service level %s not found", level)
}

// extendBlossomSubscription extends or creates a blossom subscription with service level
func (pp *PaymentProcessor) extendBlossomSubscription(
	pubkey []byte, satsReceived int64, level string, days int,
) error {
	// Get storage quota per sat per month for this level
	storageMBPerSatPerMonth, err := pp.parseServiceLevelStorage(level)
	if err != nil {
		return fmt.Errorf("failed to parse service level storage: %w", err)
	}

	// Calculate storage quota: sats * storage_mb_per_sat_per_month * (days / 30)
	storageMB := int64(float64(satsReceived) * float64(storageMBPerSatPerMonth) * (float64(days) / 30.0))

	// Extend blossom subscription
	if err := pp.db.ExtendBlossomSubscription(pubkey, level, storageMB, days); err != nil {
		return fmt.Errorf("failed to extend blossom subscription: %w", err)
	}

	log.I.F(
		"extended blossom subscription: level=%s, storage=%d MB, days=%d",
		level, storageMB, days,
	)

	return nil
}

// UpdateRelayProfile creates or updates the relay's kind 0 profile with subscription information
func (pp *PaymentProcessor) UpdateRelayProfile() error {
	// Get relay identity secret to sign the profile
	skb, err := pp.db.GetRelayIdentitySecret()
	if err != nil || len(skb) != 32 {
		return fmt.Errorf("no relay identity configured")
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(skb); err != nil {
		return fmt.Errorf("failed to initialize signer: %w", err)
	}

	monthlyPrice := pp.config.MonthlyPriceSats
	if monthlyPrice <= 0 {
		monthlyPrice = 6000
	}

	// Calculate daily rate
	dailyRate := monthlyPrice / 30

	// Get relay wss:// URL - use dashboard URL but with wss:// scheme
	relayURL := strings.Replace(pp.getDashboardURL(), "https://", "wss://", 1)

	// Create profile content as JSON
	profileContent := fmt.Sprintf(
		`{
	"name": "Relay Bot",
	"about": "This relay requires a subscription to access. Zap any of my notes to pay for access. Monthly price: %d sats (%d sats/day). Relay: %s",
	"lud16": "",
	"nip05": "",
	"website": "%s"
}`, monthlyPrice, dailyRate, relayURL, pp.getDashboardURL(),
	)

	// Build the profile event
	ev := event.New()
	ev.Kind = kind.ProfileMetadata.K // Kind 0 for profile metadata
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(profileContent)
	ev.Tags = tag.NewS()

	// Sign and save the event
	ev.Sign(sign)
	if _, err := pp.db.SaveEvent(pp.ctx, ev); err != nil {
		return fmt.Errorf("failed to save relay profile: %w", err)
	}

	log.I.F("updated relay profile with subscription information")
	return nil
}

// decodeAnyPubkey decodes a public key from either hex string or npub format
func decodeAnyPubkey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "npub1") {
		prefix, value, err := bech32encoding.Decode([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("failed to decode npub: %w", err)
		}
		if !strings.EqualFold(string(prefix), "npub") {
			return nil, fmt.Errorf("invalid prefix: %s", string(prefix))
		}
		b, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("decoded value is not []byte")
		}
		return b, nil
	}
	// assume hex-encoded public key
	return hex.Dec(s)
}
