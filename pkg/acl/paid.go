//go:build !(js && wasm)

package acl

import (
	"context"
	"encoding/hex"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/database"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
)

type paidAccessArgs struct {
	Pub     []byte
	Address string
}

type paidSubscribeArgs struct {
	PubkeyHex string
	ExpiresAt time.Time
}

type paidSetStateArgs struct {
	Owners    [][]byte
	Admins    [][]byte
	OwnerSet  map[string]struct{}
	AdminSet  map[string]struct{}
	ActiveSet map[string]time.Time
}

// Paid implements a Lightning payment-gated ACL.
// Active subscribers get "write" access to the relay and can send email
// through the bridge. Owners and admins bypass payment requirements.
// All mutable state is owned by the actor goroutine.
type Paid struct {
	Ctx context.Context
	cfg *config.C
	db  database.Database

	getAccessLevel actor.Func[paidAccessArgs, string]
	subscribe      actor.Proc[paidSubscribeArgs]
	unsubscribe    actor.Proc[string]
	isSubscribed   actor.Func[string, bool]
	setState       actor.Proc[paidSetStateArgs]
	cleanExpired   actor.Inbox[struct{}]
	actor.Lifecycle
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

	// Initialize actor channels
	p.getAccessLevel = actor.NewFunc[paidAccessArgs, string]()
	p.subscribe = actor.NewProc[paidSubscribeArgs]()
	p.unsubscribe = actor.NewProc[string]()
	p.isSubscribed = actor.NewFunc[string, bool]()
	p.setState = actor.NewProc[paidSetStateArgs]()
	p.cleanExpired = actor.NewInbox[struct{}](16)
	p.Lifecycle = actor.NewLifecycle()
	actor.Go(p.Lifecycle, func() {
		p.actorLoop(newOwners, newAdmins, newOwnerSet, newAdminSet, newActiveSet)
	})

	log.I.F("paid ACL configured: %d owners, %d admins, %d active subscribers",
		len(newOwners), len(newAdmins), len(newActiveSet))

	return nil
}

func (p *Paid) actorLoop(owners, admins [][]byte, ownerSet, adminSet map[string]struct{}, activeSet map[string]time.Time) {
	for {
		select {
		case <-p.Stopping():
			return

		case msg := <-p.getAccessLevel.Recv():
			pubHex := hex.EncodeToString(msg.Req.Pub)
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
			msg.Reply(level)

		case msg := <-p.subscribe.Recv():
			activeSet[msg.Req.PubkeyHex] = msg.Req.ExpiresAt
			msg.Done()

		case msg := <-p.unsubscribe.Recv():
			delete(activeSet, msg.Req)
			msg.Done()

		case msg := <-p.isSubscribed.Recv():
			expiry, ok := activeSet[msg.Req]
			msg.Reply(ok && time.Now().Before(expiry))

		case msg := <-p.setState.Recv():
			owners = msg.Req.Owners
			admins = msg.Req.Admins
			ownerSet = msg.Req.OwnerSet
			adminSet = msg.Req.AdminSet
			activeSet = msg.Req.ActiveSet
			_ = owners
			_ = admins
			msg.Done()

		case <-p.cleanExpired.Recv():
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
	return p.getAccessLevel.Call(paidAccessArgs{Pub: pub, Address: address})
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
			p.cleanExpired.TrySend(struct{}{})
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

	p.subscribe.Call(paidSubscribeArgs{PubkeyHex: pubkeyHex, ExpiresAt: expiresAt})

	log.I.F("paid ACL: subscription activated for %s (expires %s)", pubkeyHex, expiresAt.Format(time.RFC3339))
	return nil
}

// Unsubscribe removes a subscription.
func (p *Paid) Unsubscribe(pubkeyHex string) error {
	if err := p.db.DeletePaidSubscription(pubkeyHex); err != nil {
		return err
	}

	p.unsubscribe.Call(pubkeyHex)

	return nil
}

// IsSubscribed returns true if the pubkey has an active (non-expired) subscription.
func (p *Paid) IsSubscribed(pubkeyHex string) bool {
	return p.isSubscribed.Call(pubkeyHex)
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
