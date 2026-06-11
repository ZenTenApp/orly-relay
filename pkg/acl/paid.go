//go:build !(js && wasm)

package acl

import (
	"context"
	"encoding/hex"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/database"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
)

// paid actor request types

type paidGetAccessLevelReq struct {
	pub     []byte
	address string
	resp    chan string
}

type paidSubscribeReq struct {
	pubkeyHex string
	expiresAt time.Time
	resp      chan struct{}
}

type paidUnsubscribeReq struct {
	pubkeyHex string
	resp      chan struct{}
}

type paidIsSubscribedReq struct {
	pubkeyHex string
	resp      chan bool
}

type paidSetStateReq struct {
	owners    [][]byte
	admins    [][]byte
	ownerSet  map[string]struct{}
	adminSet  map[string]struct{}
	activeSet map[string]time.Time
	resp      chan struct{}
}

// Paid implements a Lightning payment-gated ACL.
// Active subscribers get "write" access to the relay and can send email
// through the bridge. Owners and admins bypass payment requirements.
type Paid struct {
	Ctx context.Context
	cfg *config.C
	db  database.Database

	getAccessLevelCh chan paidGetAccessLevelReq
	subscribeCh      chan paidSubscribeReq
	unsubscribeCh    chan paidUnsubscribeReq
	isSubscribedCh   chan paidIsSubscribedReq
	setStateCh       chan paidSetStateReq
	cleanExpiredCh   chan struct{}
	stop             chan struct{}
	done             chan struct{}
}

func (p *Paid) Configure(cfg ...any) (err error) {
	log.I.F("configuring paid ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			p.cfg = c
		case database.Database:
			p.db = c
		case context.Context:
			p.Ctx = c
		}
	}
	if p.cfg == nil || p.db == nil {
		return errorf.E("both config and database must be set")
	}

	// Build owner/admin sets
	newOwnerSet := make(map[string]struct{})
	var newOwners [][]byte
	for _, owner := range p.cfg.Owners {
		if own, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(e) {
			continue
		} else {
			newOwners = append(newOwners, own)
			newOwnerSet[hex.EncodeToString(own)] = struct{}{}
		}
	}

	newAdminSet := make(map[string]struct{})
	var newAdmins [][]byte
	for _, admin := range p.cfg.Admins {
		if adm, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(e) {
			continue
		} else {
			newAdmins = append(newAdmins, adm)
			newAdminSet[hex.EncodeToString(adm)] = struct{}{}
		}
	}

	// Load active subscriptions into memory
	newActiveSet := make(map[string]time.Time)
	subs, err := p.db.ListPaidSubscriptions()
	if err != nil {
		log.W.F("paid ACL: failed to load subscriptions: %v", err)
		err = nil
	}
	now := time.Now()
	for _, sub := range subs {
		if sub.ExpiresAt.After(now) {
			newActiveSet[sub.PubkeyHex] = sub.ExpiresAt
		}
	}

	// Start actor goroutine
	p.getAccessLevelCh = make(chan paidGetAccessLevelReq)
	p.subscribeCh = make(chan paidSubscribeReq)
	p.unsubscribeCh = make(chan paidUnsubscribeReq)
	p.isSubscribedCh = make(chan paidIsSubscribedReq)
	p.setStateCh = make(chan paidSetStateReq)
	p.cleanExpiredCh = make(chan struct{}, 16)
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	go p.actor(newOwners, newAdmins, newOwnerSet, newAdminSet, newActiveSet)

	log.I.F("paid ACL configured: %d owners, %d admins, %d active subscribers",
		len(newOwners), len(newAdmins), len(newActiveSet))

	return nil
}

func (p *Paid) actor(owners, admins [][]byte, ownerSet, adminSet map[string]struct{}, activeSet map[string]time.Time) {
	defer close(p.done)

	for {
		select {
		case <-p.stop:
			return

		case req := <-p.getAccessLevelCh:
			pubHex := hex.EncodeToString(req.pub)
			level := "read"
			if _, ok := ownerSet[pubHex]; ok {
				level = "owner"
			} else if _, ok := adminSet[pubHex]; ok {
				level = "admin"
			} else if expiry, ok := activeSet[pubHex]; ok {
				if time.Now().Before(expiry) {
					level = "write"
				}
			}
			req.resp <- level

		case req := <-p.subscribeCh:
			activeSet[req.pubkeyHex] = req.expiresAt
			req.resp <- struct{}{}

		case req := <-p.unsubscribeCh:
			delete(activeSet, req.pubkeyHex)
			req.resp <- struct{}{}

		case req := <-p.isSubscribedCh:
			expiry, ok := activeSet[req.pubkeyHex]
			req.resp <- ok && time.Now().Before(expiry)

		case req := <-p.setStateCh:
			owners = req.owners
			admins = req.admins
			ownerSet = req.ownerSet
			adminSet = req.adminSet
			activeSet = req.activeSet
			_ = owners
			_ = admins
			req.resp <- struct{}{}

		case <-p.cleanExpiredCh:
			now := time.Now()
			for pubkey, expiry := range activeSet {
				if now.After(expiry) {
					delete(activeSet, pubkey)
				}
			}
		}
	}
}

func (p *Paid) GetAccessLevel(pub []byte, address string) (level string) {
	resp := make(chan string, 1)
	p.getAccessLevelCh <- paidGetAccessLevelReq{pub: pub, address: address, resp: resp}
	return <-resp
}

func (p *Paid) GetACLInfo() (name, description, documentation string) {
	return "paid", "Lightning payment-gated access control",
		"This ACL mode grants write access to subscribers who pay via Lightning. " +
			"Users can also claim email aliases for a higher monthly rate."
}

func (p *Paid) Type() string { return "paid" }

// Syncer runs a periodic expiry cleanup goroutine.
func (p *Paid) Syncer() {
	if p.Ctx == nil {
		return
	}
	go p.expiryLoop()
}

func (p *Paid) expiryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.Ctx.Done():
			return
		case <-ticker.C:
			select {
			case p.cleanExpiredCh <- struct{}{}:
			default:
			}
		}
	}
}

// Subscribe activates a subscription for a pubkey.
func (p *Paid) Subscribe(pubkeyHex string, expiresAt time.Time, invoiceHash, alias string) error {
	sub := &database.PaidSubscription{
		PubkeyHex:   pubkeyHex,
		Alias:       alias,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		InvoiceHash: invoiceHash,
	}
	if err := p.db.SavePaidSubscription(sub); err != nil {
		return err
	}

	resp := make(chan struct{}, 1)
	p.subscribeCh <- paidSubscribeReq{pubkeyHex: pubkeyHex, expiresAt: expiresAt, resp: resp}
	<-resp

	log.I.F("paid ACL: subscription activated for %s (expires %s)", pubkeyHex, expiresAt.Format(time.RFC3339))
	return nil
}

// Unsubscribe removes a subscription.
func (p *Paid) Unsubscribe(pubkeyHex string) error {
	if err := p.db.DeletePaidSubscription(pubkeyHex); err != nil {
		return err
	}

	resp := make(chan struct{}, 1)
	p.unsubscribeCh <- paidUnsubscribeReq{pubkeyHex: pubkeyHex, resp: resp}
	<-resp

	return nil
}

// IsSubscribed returns true if the pubkey has an active (non-expired) subscription.
func (p *Paid) IsSubscribed(pubkeyHex string) bool {
	resp := make(chan bool, 1)
	p.isSubscribedCh <- paidIsSubscribedReq{pubkeyHex: pubkeyHex, resp: resp}
	return <-resp
}

// GetSubscription returns the subscription for a pubkey.
func (p *Paid) GetSubscription(pubkeyHex string) (*database.PaidSubscription, error) {
	return p.db.GetPaidSubscription(pubkeyHex)
}

// ClaimAlias claims an alias for a pubkey. Validates the alias and delegates to DB.
func (p *Paid) ClaimAlias(alias, pubkeyHex string) error {
	if err := ValidateAlias(alias); err != nil {
		return err
	}
	return p.db.ClaimAlias(alias, pubkeyHex)
}

// GetAliasByPubkey returns the alias for a pubkey, or "" if none.
func (p *Paid) GetAliasByPubkey(pubkeyHex string) (string, error) {
	return p.db.GetAliasByPubkey(pubkeyHex)
}

// GetAliasesByPubkey returns all aliases for a pubkey.
func (p *Paid) GetAliasesByPubkey(pubkeyHex string) ([]string, error) {
	return p.db.GetAliasesByPubkey(pubkeyHex)
}

// GetPubkeyByAlias returns the pubkey for an alias, or "" if not found.
func (p *Paid) GetPubkeyByAlias(alias string) (string, error) {
	return p.db.GetPubkeyByAlias(alias)
}

// IsAliasTaken returns true if the alias is claimed.
func (p *Paid) IsAliasTaken(alias string) (bool, error) {
	return p.db.IsAliasTaken(alias)
}

// GetDatabase returns the underlying database for direct access.
func (p *Paid) GetDatabase() database.Database {
	return p.db
}

func init() {
	Registry.Register(new(Paid))
}
