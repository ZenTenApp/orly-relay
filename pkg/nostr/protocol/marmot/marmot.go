package marmot

import (
	"context"
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
	"next.orly.dev/pkg/nostr/interfaces/signer"
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

// Client manages Marmot DM conversations. It holds MLS group state for
// active 1:1 conversations and handles the lifecycle of key packages,
// welcomes, and encrypted messages.
type Client struct {
	sign   signer.I
	store  GroupStore
	relay  RelayConnection
	relays []string // relay URLs for key package discovery
	onDM   DMHandler
	kpp    *mls.KeyPairPackage // our current key pair package
	groups map[string]*GroupState
	mu     sync.RWMutex

	// groupsChanged is signalled when a new group is added so callers
	// can refresh subscription filters.
	groupsChanged chan struct{}
}

// NewClient creates a Marmot client. The signer provides identity and
// signing. The store persists group state. The relay handles event transport.
// Relays are the WebSocket URLs advertised in key package events.
func NewClient(sign signer.I, store GroupStore, relay RelayConnection, relays ...string) (*Client, error) {
	kpp, err := GenerateKeyPackage(sign)
	if err != nil {
		return nil, fmt.Errorf("generate key package: %w", err)
	}

	c := &Client{
		sign:          sign,
		store:         store,
		relay:         relay,
		relays:        relays,
		kpp:           kpp,
		groups:        make(map[string]*GroupState),
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
			// We store the serialized state but can't re-hydrate the
			// mls.Group from bytes with the current go-mls API.
			// For now, groups are re-established on restart via welcome
			// re-exchange. Store the metadata so we know about them.
			c.groups[string(gs.GroupID)] = &GroupState{
				GroupID:      gs.GroupID,
				NostrGroupID: gs.NostrGroupID,
				PeerPub:      gs.PeerPub,
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

// PublishKeyPackage publishes our MLS key package as a kind 443 event so
// peers can create DM groups with us.
func (c *Client) PublishKeyPackage(ctx context.Context) error {
	ev, err := KeyPackageToEvent(c.kpp, c.sign, c.relays)
	if err != nil {
		return err
	}
	return c.relay.Publish(ctx, ev)
}

// SendDM sends an encrypted DM to the given recipient. If no group exists,
// it fetches the recipient's key package, creates a group, and sends a
// welcome. Then it encrypts and publishes the message.
func (c *Client) SendDM(ctx context.Context, recipientPub []byte, plaintext []byte) error {
	groupID := DMGroupID(c.sign.Pub(), recipientPub)

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

	gs, welcome, _, err := CreateDMGroup(c.kpp, peerKP, c.sign.Pub(), peerPub, c.relays)
	if err != nil {
		return nil, fmt.Errorf("create DM group: %w", err)
	}

	// Send the welcome as a gift-wrapped event with key package event ID
	wrapEv, err := WelcomeToGiftWrap(welcome, peerPub, c.sign, peerKPEvent, c.relays)
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

	return gs, nil
}

// HandleEvent processes an incoming event. Call this from the subscription loop.
func (c *Client) HandleEvent(ctx context.Context, ev *event.E) error {
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
	unwrapped, err := UnwrapWelcome(ev, c.sign)
	if err != nil {
		return fmt.Errorf("unwrap welcome: %w", err)
	}

	// SenderPub comes from the seal layer (real identity, not ephemeral).
	senderPub := unwrapped.SenderPub
	gs, err := JoinDMGroup(unwrapped.Welcome, c.kpp, senderPub)
	if err != nil {
		return fmt.Errorf("join DM group: %w", err)
	}

	// Ensure the MLS GroupID is set (should match from the Welcome)
	if len(gs.GroupID) == 0 {
		gs.GroupID = DMGroupID(c.sign.Pub(), senderPub)
	}

	c.mu.Lock()
	c.groups[string(gs.GroupID)] = gs
	c.mu.Unlock()

	c.persistGroup(gs)

	// Signal that filters need refreshing
	select {
	case c.groupsChanged <- struct{}{}:
	default:
	}

	log.I.F("joined DM group with %s (nostr_group_id: %s)", hex.Enc(senderPub), hex.Enc(gs.NostrGroupID))
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

	if c.onDM != nil {
		c.onDM(gs.PeerPub, plaintext)
	}

	return nil
}

func (c *Client) persistGroup(gs *GroupState) {
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
		tag.NewFromAny("p", hex.Enc(c.sign.Pub())),
	)
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
	return KeyPackageToEvent(c.kpp, c.sign, c.relays)
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
	if err := ev.Sign(c.sign); err != nil {
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
	if err := ev.Sign(c.sign); err != nil {
		return fmt.Errorf("sign key package relays: %w", err)
	}
	return c.relay.Publish(ctx, ev)
}
