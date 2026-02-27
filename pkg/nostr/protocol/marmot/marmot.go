package marmot

import (
	"context"
	"fmt"
	"sync"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/lol/log"
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
	onDM   DMHandler
	kpp    *mls.KeyPairPackage // our current key pair package
	groups map[string]*GroupState
	mu     sync.RWMutex
}

// NewClient creates a Marmot client. The signer provides identity and
// signing. The store persists group state. The relay handles event transport.
func NewClient(sign signer.I, store GroupStore, relay RelayConnection) (*Client, error) {
	kpp, err := GenerateKeyPackage(sign)
	if err != nil {
		return nil, fmt.Errorf("generate key package: %w", err)
	}

	c := &Client{
		sign:   sign,
		store:  store,
		relay:  relay,
		kpp:    kpp,
		groups: make(map[string]*GroupState),
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
				GroupID:  gs.GroupID,
				PeerPub:  gs.PeerPub,
				mlsBytes: gs.MLSState,
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
	ev, err := KeyPackageToEvent(c.kpp, c.sign)
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

	ev, err := MessageToEvent(groupID, ciphertext, c.sign)
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

	gs, welcome, _, err := CreateDMGroup(c.kpp, peerKP, c.sign.Pub(), peerPub)
	if err != nil {
		return nil, fmt.Errorf("create DM group: %w", err)
	}

	// Send the welcome as a gift-wrapped event
	wrapEv, err := WelcomeToGiftWrap(welcome, peerPub, c.sign)
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
	welcome, err := UnwrapWelcome(ev, c.sign)
	if err != nil {
		return fmt.Errorf("unwrap welcome: %w", err)
	}

	senderPub := ev.Pubkey
	gs, err := JoinDMGroup(welcome, c.kpp, senderPub)
	if err != nil {
		return fmt.Errorf("join DM group: %w", err)
	}

	// Derive the group ID
	gs.GroupID = DMGroupID(c.sign.Pub(), senderPub)

	c.mu.Lock()
	c.groups[string(gs.GroupID)] = gs
	c.mu.Unlock()

	c.persistGroup(gs)

	log.I.F("joined DM group with %s", hex.Enc(senderPub))
	return nil
}

func (c *Client) handleGroupMessage(ctx context.Context, ev *event.E) error {
	groupID, ciphertext, err := EventToMessage(ev)
	if err != nil {
		return err
	}

	c.mu.RLock()
	gs, ok := c.groups[string(groupID)]
	c.mu.RUnlock()

	if !ok || gs.group == nil {
		return fmt.Errorf("unknown group %x", groupID)
	}

	plaintext, err := gs.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if c.onDM != nil {
		c.onDM(ev.Pubkey, plaintext)
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

// SubscriptionFilter returns the filter for subscribing to events relevant
// to this client (key packages, welcomes, and group messages addressed to us).
func (c *Client) SubscriptionFilter() *filter.S {
	f := filter.New()
	f.Kinds = kind.NewS(
		kind.New(KindGiftWrap),
		kind.New(KindGroupMessage),
	)
	f.Tags = tag.NewS(
		tag.NewFromAny("#p", hex.Enc(c.sign.Pub())),
	)
	return filter.NewS(f)
}
