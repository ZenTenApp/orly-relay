package marmot

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
)

// RelayConnection abstracts the relay interface so the Marmot client can be
// used with any relay transport (WebSocket, in-process channel, mock).
type RelayConnection interface {
	Publish(ctx context.Context, ev *event.E) error
	Subscribe(ctx context.Context, ff *filter.S) (EventStream, error)
}

// EventStream delivers events from a subscription. Close stops delivery.
type EventStream interface {
	Events() <-chan *event.E
	Close()
}

// DMHandler is called when an incoming DM is decrypted.
type DMHandler func(senderPub []byte, plaintext []byte)

// GroupJoinedHandler is called when a new DM group is established (Welcome processed).
type GroupJoinedHandler func(peerPub []byte)

// Client manages Marmot DM conversations. It holds MLS group state for
// active 1:1 conversations and handles the lifecycle of key packages,
// welcomes, and encrypted messages.
type Client struct {
	crypto CryptoProvider
	store  GroupStore
	relay  RelayConnection
	relays []string // relay URLs for key package discovery
	onDM          DMHandler
	onGroupJoined GroupJoinedHandler
	kpp    *mls.KeyPairPackage // our current key pair package
	groups map[string]*GroupState
	mu     sync.RWMutex

	// seenIDs tracks event IDs we published or processed, to skip duplicates
	// when they come back via the subscription.
	seenIDs map[string]struct{}

	// lastEventTS is the highest created_at seen across HandleEvent calls.
	// Used as "since" in subscription filters to skip already-processed events
	// on restart. Set from persisted storage via SetLastEventTS before Subscribe.
	lastEventTS int64

	// groupsChanged is signalled when a new group is added so callers
	// can refresh subscription filters.
	groupsChanged chan struct{}
}

// NewClient creates a Marmot client. The crypto provider handles identity,
// signing, and NIP-44 encryption. The store persists group state.
// The relay handles event transport.
func NewClient(crypto CryptoProvider, store GroupStore, relay RelayConnection, relays ...string) (*Client, error) {
	kpp, err := GenerateKeyPackage(crypto)
	if err != nil {
		return nil, fmt.Errorf("generate key package: %w", err)
	}

	c := &Client{
		crypto:        crypto,
		store:         store,
		relay:         relay,
		relays:        relays,
		kpp:           kpp,
		groups:        make(map[string]*GroupState),
		seenIDs:       make(map[string]struct{}),
		groupsChanged: make(chan struct{}, 1),
	}

	// Load persisted groups
	ids, err := store.ListGroups()
	if err == nil {
		for _, id := range ids {
			data, err := store.LoadGroup(id)
			if err != nil {
				log.W.F("failed to load group %x: %v", id, err)
				continue
			}
			gs, err := unmarshalGroupState(data)
			if err != nil {
				log.W.F("failed to unmarshal group %x: %v", id, err)
				continue
			}
			if len(gs.MLSState) == 0 {
				_ = store.DeleteGroup(id)
				continue
			}
			group, err := mls.UnmarshalGroup(gs.MLSState)
			if err != nil {
				log.W.F("failed to restore MLS group %x: %v", id, err)
				_ = store.DeleteGroup(id)
				continue
			}
			c.groups[string(gs.GroupID)] = &GroupState{
				GroupID:      gs.GroupID,
				NostrGroupID: gs.NostrGroupID,
				PeerPub:      gs.PeerPub,
				group:        group,
				mlsBytes:     gs.MLSState,
			}
		}
	}

	return c, nil
}

// OnDM registers a handler for incoming decrypted DMs.
func (c *Client) OnDM(handler DMHandler) {
	c.onDM = handler
}

// OnGroupJoined registers a handler called when a new DM group is established.
func (c *Client) OnGroupJoined(handler GroupJoinedHandler) {
	c.onGroupJoined = handler
}

// SetLastEventTS sets the high-water mark for processed events.
// Call this with a persisted value before Subscribe to skip old events.
func (c *Client) SetLastEventTS(ts int64) {
	c.mu.Lock()
	c.lastEventTS = ts
	c.mu.Unlock()
}

// LastEventTS returns the highest event created_at seen so far.
// Persist this value to skip old events on next restart.
func (c *Client) LastEventTS() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastEventTS
}

// PublishKeyPackage publishes our MLS key package as a kind 443 event so
// peers can create DM groups with us.
func (c *Client) PublishKeyPackage(ctx context.Context) error {
	ev, err := KeyPackageToEvent(c.kpp, c.crypto, c.relays)
	if err != nil {
		return err
	}
	return c.relay.Publish(ctx, ev)
}

// rotateKeyPackage generates a fresh KP and publishes it. Called after
// processing a Welcome — the consumed init_key is destroyed per MLS spec,
// so the old KP on relays is now useless.
func (c *Client) rotateKeyPackage(ctx context.Context) {
	kpp, err := GenerateKeyPackage(c.crypto)
	if err != nil {
		log.W.F("rotate key package: generate: %v", err)
		return
	}
	c.kpp = kpp
	if err := c.PublishKeyPackage(ctx); err != nil {
		log.W.F("rotate key package: publish: %v", err)
	}
}

// SendDM sends an encrypted DM to the given recipient. If no group exists,
// it fetches the recipient's key package, creates a group, and sends a
// welcome. Then it encrypts and publishes the message.
func (c *Client) SendDM(ctx context.Context, recipientPub []byte, plaintext []byte) error {
	groupID := DMGroupID(c.crypto.Pub(), recipientPub)

	c.mu.RLock()
	gs, ok := c.groups[string(groupID)]
	c.mu.RUnlock()

	if !ok || gs.group == nil {
		// Need to establish a new group
		var err error
		gs, err = c.establishGroup(ctx, recipientPub)
		if err != nil {
			return fmt.Errorf("establish group: %w", err)
		}
	}

	ciphertext, err := gs.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	exporterSecret, err := gs.DeriveExporterSecret()
	if err != nil {
		return fmt.Errorf("derive exporter secret: %w", err)
	}

	ev, err := MessageToEvent(gs.NostrGroupID, ciphertext, exporterSecret)
	if err != nil {
		return err
	}

	// Re-persist after encrypt — MLS state may have advanced.
	c.persistGroup(gs)

	// Track this event ID so we skip it when it comes back via subscription.
	c.mu.Lock()
	c.seenIDs[string(ev.ID)] = struct{}{}
	c.mu.Unlock()

	return c.relay.Publish(ctx, ev)
}

// establishGroup fetches the peer's key package and creates a DM group.
func (c *Client) establishGroup(ctx context.Context, peerPub []byte) (*GroupState, error) {
	// Fetch the peer's latest key package (kind 443)
	f := filter.New()
	f.Kinds = kind.NewS(kind.New(KindKeyPackage))
	f.Authors = &tag.T{T: [][]byte{peerPub}}
	limit := uint(1)
	f.Limit = &limit

	stream, err := c.relay.Subscribe(ctx, filter.NewS(f))
	if err != nil {
		return nil, fmt.Errorf("subscribe for key package: %w", err)
	}
	defer stream.Close()

	// Wait for one event
	var peerKPEvent *event.E
	select {
	case ev := <-stream.Events():
		peerKPEvent = ev
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if peerKPEvent == nil {
		return nil, fmt.Errorf("no key package found for %s", hex.Enc(peerPub))
	}

	peerKP, err := EventToKeyPackage(peerKPEvent)
	if err != nil {
		return nil, fmt.Errorf("parse peer key package: %w", err)
	}

	gs, welcome, _, err := CreateDMGroup(c.kpp, peerKP, c.crypto.Pub(), peerPub, c.relays)
	if err != nil {
		return nil, fmt.Errorf("create DM group: %w", err)
	}

	// Send the welcome as a gift-wrapped event with key package event ID
	wrapEv, err := WelcomeToGiftWrap(welcome, peerPub, c.crypto, peerKPEvent, c.relays)
	if err != nil {
		return nil, fmt.Errorf("gift wrap welcome: %w", err)
	}
	if err := c.relay.Publish(ctx, wrapEv); err != nil {
		return nil, fmt.Errorf("publish welcome: %w", err)
	}

	// Store the group
	c.mu.Lock()
	c.groups[string(gs.GroupID)] = gs
	c.mu.Unlock()

	c.persistGroup(gs)

	// Signal that filters need refreshing (so subscription includes kind 445 for this group).
	select {
	case c.groupsChanged <- struct{}{}:
	default:
	}

	return gs, nil
}

// HandleEvent processes an incoming event. Call this from the subscription loop.
func (c *Client) HandleEvent(ctx context.Context, ev *event.E) error {
	// Atomic check-and-set to avoid TOCTOU: a separate read-lock check
	// followed by write-lock set would let two goroutines both pass the
	// check for the same event.
	evKey := string(ev.ID)
	c.mu.Lock()
	if _, seen := c.seenIDs[evKey]; seen {
		c.mu.Unlock()
		return nil
	}
	c.seenIDs[evKey] = struct{}{}
	if ev.CreatedAt > c.lastEventTS {
		c.lastEventTS = ev.CreatedAt
	}
	// Prune seenIDs to cap memory. 4096 is ~128KB of 32-byte event IDs.
	if len(c.seenIDs) > 4096 {
		c.seenIDs = make(map[string]struct{})
		c.seenIDs[evKey] = struct{}{}
	}
	c.mu.Unlock()

	switch ev.Kind {
	case KindGiftWrap:
		return c.handleWelcome(ctx, ev)
	case KindGroupMessage:
		return c.handleGroupMessage(ctx, ev)
	default:
		return nil
	}
}

func (c *Client) handleWelcome(ctx context.Context, ev *event.E) error {
	uw, err := UnwrapGiftWrap(ev, c.crypto)
	if err != nil {
		// Gift wraps we can't unwrap (wrong key, corrupt, etc.) are noise — skip.
		log.D.F("skipping undecryptable gift wrap: %v", err)
		return nil
	}

	// Dispatch based on inner event kind.
	switch uw.Inner.Kind {
	case KindWelcome:
		return c.processWelcome(ctx, uw)
	case 14:
		// NIP-17 DM (kind 14 rumor inside gift wrap) — deliver as DM.
		if c.onDM != nil {
			c.onDM(uw.SenderPub, uw.Inner.Content)
		}
		return nil
	default:
		// Unknown inner kind — skip silently.
		return nil
	}
}

func (c *Client) processWelcome(ctx context.Context, uw *UnwrappedGiftWrap) error {
	// Decode content: base64 or raw binary (legacy)
	content := uw.Inner.Content
	encodingTag := uw.Inner.Tags.GetFirst([]byte("encoding"))
	if encodingTag != nil && string(encodingTag.Value()) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err != nil {
			return fmt.Errorf("base64 decode welcome: %w", err)
		}
		content = decoded
	}

	welcome, err := mls.UnmarshalWelcome(content)
	if err != nil {
		return fmt.Errorf("unmarshal welcome: %w", err)
	}

	senderPub := uw.SenderPub
	gs, err := JoinDMGroup(welcome, c.kpp, senderPub)
	if err != nil {
		// Expected after restart — init_key from the consumed KeyPackage is
		// destroyed per MLS spec, so old welcomes are permanently undecryptable.
		log.D.F("skipping stale welcome from %s: %v", hex.Enc(senderPub), err)
		return nil
	}

	if len(gs.GroupID) == 0 {
		gs.GroupID = DMGroupID(c.crypto.Pub(), senderPub)
	}

	c.mu.Lock()
	c.groups[string(gs.GroupID)] = gs
	c.mu.Unlock()

	c.persistGroup(gs)

	select {
	case c.groupsChanged <- struct{}{}:
	default:
	}

	log.I.F("joined DM group with %s (nostr_group_id: %s)", hex.Enc(senderPub), hex.Enc(gs.NostrGroupID))

	// MIP-00: rotate KeyPackage after Welcome — the consumed init_key is
	// dead, so peers fetching the old KP would fail to create a group.
	c.rotateKeyPackage(ctx)

	if c.onGroupJoined != nil {
		c.onGroupJoined(senderPub)
	}
	return nil
}

// findGroupByNostrGroupID looks up a group by its nostr_group_id (from "h" tag).
func (c *Client) findGroupByNostrGroupID(nostrGroupID []byte) *GroupState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, gs := range c.groups {
		if string(gs.NostrGroupID) == string(nostrGroupID) {
			return gs
		}
	}
	return nil
}

func (c *Client) handleGroupMessage(ctx context.Context, ev *event.E) error {
	hTag := ev.Tags.GetFirst([]byte("h"))
	if hTag == nil {
		return fmt.Errorf("missing 'h' tag on kind %d", KindGroupMessage)
	}
	nostrGroupID, err := hex.Dec(string(hTag.Value()))
	if err != nil {
		return fmt.Errorf("decode nostr group ID: %w", err)
	}

	gs := c.findGroupByNostrGroupID(nostrGroupID)
	if gs == nil || gs.group == nil {
		return fmt.Errorf("unknown nostr group %x", nostrGroupID)
	}

	exporterSecret, err := gs.DeriveExporterSecret()
	if err != nil {
		return fmt.Errorf("derive exporter secret: %w", err)
	}

	_, mlsCiphertext, err := EventToMessage(ev, exporterSecret)
	if err != nil {
		return err
	}

	plaintext, err := gs.Decrypt(mlsCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// Re-persist after decrypt — MLS state may have advanced (epoch ratchet).
	c.persistGroup(gs)

	if c.onDM != nil {
		c.onDM(gs.PeerPub, plaintext)
	}

	return nil
}

func (c *Client) persistGroup(gs *GroupState) {
	// Refresh mlsBytes from live group state (may have advanced epoch).
	if gs.group != nil {
		if b, err := gs.group.Marshal(); err == nil {
			gs.mlsBytes = b
		}
	}
	data, err := marshalGroupState(gs)
	if err != nil {
		log.W.F("failed to marshal group state: %v", err)
		return
	}
	if err := c.store.SaveGroup(gs.GroupID, data); err != nil {
		log.W.F("failed to persist group state: %v", err)
	}
}

// WelcomeFilter returns a filter for kind 1059 events addressed to us via
// "p" tag. These are Welcome messages from peers establishing new groups.
func (c *Client) WelcomeFilter() *filter.F {
	f := filter.New()
	f.Kinds = kind.NewS(kind.New(KindGiftWrap))
	f.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(c.crypto.Pub())),
	)
	c.mu.RLock()
	ts := c.lastEventTS
	c.mu.RUnlock()
	if ts > 0 {
		f.Since = timestamp.FromUnix(ts - 172800) // 2-day margin for NIP-59 timestamp randomization
	} else {
		// No persisted timestamp — first run. Only fetch recent events
		// to avoid flooding the bus with hundreds of historical DMs.
		f.Since = timestamp.FromUnix(time.Now().Unix() - 172800) // 2-day margin for NIP-59
	}
	return f
}

// GroupMessageFilter returns a filter for kind 445 events tagged with our
// active group IDs. Kind 445 events use ephemeral pubkeys (no "p" tag for
// the real recipient), so we subscribe via "#h" tags.
func (c *Client) GroupMessageFilter() *filter.F {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.groups) == 0 {
		return nil
	}

	hValues := make([]any, 0, len(c.groups)+1)
	hValues = append(hValues, "h")
	for _, gs := range c.groups {
		if len(gs.NostrGroupID) > 0 {
			hValues = append(hValues, hex.Enc(gs.NostrGroupID))
		}
	}

	f := filter.New()
	f.Kinds = kind.NewS(kind.New(KindGroupMessage))
	f.Tags = tag.NewS(
		tag.NewFromAny(hValues...),
	)
	if c.lastEventTS > 0 {
		f.Since = timestamp.FromUnix(c.lastEventTS - 172800)
	} else {
		f.Since = timestamp.FromUnix(time.Now().Unix() - 172800) // 2-day margin for NIP-59
	}
	return f
}

// SubscriptionFilters returns filters for all events relevant to this client.
// Returns one or two filters depending on whether active groups exist.
func (c *Client) SubscriptionFilters() *filter.S {
	filters := []*filter.F{c.WelcomeFilter()}
	if gmf := c.GroupMessageFilter(); gmf != nil {
		filters = append(filters, gmf)
	}
	return filter.NewS(filters...)
}

// GroupsChanged returns a channel that is signalled whenever a new group
// is added (e.g. after processing a Welcome). Callers should use this to
// refresh subscription filters that include group-specific "#h" tags.
func (c *Client) GroupsChanged() <-chan struct{} {
	return c.groupsChanged
}

// ActiveGroupIDs returns hex-encoded nostr_group_ids of all active groups.
func (c *Client) ActiveGroupIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.groups))
	for _, gs := range c.groups {
		if len(gs.NostrGroupID) > 0 {
			ids = append(ids, hex.Enc(gs.NostrGroupID))
		}
	}
	return ids
}

// KeyPackageEvent returns a signed kind 443 event containing our MLS key
// package, suitable for broadcasting to external relays.
func (c *Client) KeyPackageEvent() (*event.E, error) {
	return KeyPackageToEvent(c.kpp, c.crypto, c.relays)
}

// KeyPackageRelaysEvent returns a signed kind 10051 event listing relay URLs
// where our key packages can be found, suitable for broadcasting.
func (c *Client) KeyPackageRelaysEvent(relayURLs []string) (*event.E, error) {
	tags := make([]*tag.T, len(relayURLs))
	for i, u := range relayURLs {
		tags[i] = tag.NewFromAny("relay", u)
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindKeyPackageRelays
	ev.Tags = tag.NewS(tags...)
	if err := c.crypto.SignEvent(ev); err != nil {
		return nil, fmt.Errorf("sign key package relays: %w", err)
	}
	return ev, nil
}

// PublishKeyPackageRelays publishes a kind 10051 event listing relay URLs
// where our key packages can be found.
func (c *Client) PublishKeyPackageRelays(ctx context.Context, relayURLs []string) error {
	tags := make([]*tag.T, len(relayURLs))
	for i, u := range relayURLs {
		tags[i] = tag.NewFromAny("relay", u)
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindKeyPackageRelays
	ev.Tags = tag.NewS(tags...)
	if err := c.crypto.SignEvent(ev); err != nil {
		return fmt.Errorf("sign key package relays: %w", err)
	}
	return c.relay.Publish(ctx, ev)
}
