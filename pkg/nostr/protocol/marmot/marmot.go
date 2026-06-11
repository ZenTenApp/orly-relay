package marmot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emersion/go-mls"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
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

// --- Actor request/response types ---

type mcSetLastEventTSReq struct {
	ts   int64
	resp chan struct{}
}

type mcGetLastEventTSReq struct {
	resp chan int64
}

type mcGetGroupReq struct {
	groupID string
	resp    chan *GroupState
}

type mcSetGroupReq struct {
	groupID string
	gs      *GroupState
	resp    chan struct{}
}

type mcDeleteGroupReq struct {
	groupID string
	resp    chan *GroupState // returns old group state if existed
}

type mcFindByNostrGroupIDReq struct {
	nostrGroupID []byte
	resp         chan *GroupState
}

type mcCheckAndSetSeenReq struct {
	evKey     string
	createdAt int64
	resp      chan bool // true if already seen
}

type mcMarkSeenReq struct {
	evKey string
}

type mcActiveGroupIDsReq struct {
	resp chan []string
}

type mcGroupMessageFilterReq struct {
	resp chan *filter.F
}

type mcWelcomeFilterReq struct {
	resp chan *filter.F
}

type mcSubscriptionFiltersReq struct {
	resp chan *filter.S
}

type mcBackupSnapshotReq struct {
	resp chan *mcBackupSnapshot
}

type mcBackupSnapshot struct {
	skip        bool
	groups      []groupStateBackup
	lastEventTS int64
}

type mcSetBackupTimeReq struct {
	t    time.Time
	resp chan struct{}
}

type mcResetBackupTimeReq struct {
	resp chan struct{}
}

type mcRestoreGroupReq struct {
	groupID string
	gs      *GroupState
	epoch   uint64
	resp    chan bool // true if restored (not superseded by local)
}

type mcRestoreLastEventTSReq struct {
	ts   int64
	resp chan struct{}
}

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

	// Actor channels
	setLastEventTS     chan mcSetLastEventTSReq
	getLastEventTS     chan mcGetLastEventTSReq
	getGroup           chan mcGetGroupReq
	setGroup           chan mcSetGroupReq
	deleteGroup        chan mcDeleteGroupReq
	findByNostrGroupID chan mcFindByNostrGroupIDReq
	checkAndSetSeen    chan mcCheckAndSetSeenReq
	markSeen           chan mcMarkSeenReq
	activeGroupIDs     chan mcActiveGroupIDsReq
	groupMessageFilter chan mcGroupMessageFilterReq
	welcomeFilter      chan mcWelcomeFilterReq
	subscriptionFilter chan mcSubscriptionFiltersReq
	backupSnapshot     chan mcBackupSnapshotReq
	setBackupTime      chan mcSetBackupTimeReq
	resetBackupTime    chan mcResetBackupTimeReq
	restoreGroup       chan mcRestoreGroupReq
	restoreLastEventTS chan mcRestoreLastEventTSReq

	// groupsChanged is signalled when a new group is added so callers
	// can refresh subscription filters.
	groupsChanged chan struct{}

	stop chan struct{}
	done chan struct{}
}

func (c *Client) actor(initGroups map[string]*GroupState) {
	defer close(c.done)

	groups := initGroups
	seenIDs := make(map[string]struct{})
	var lastEventTS int64
	var lastBackupTime time.Time

	for {
		select {
		case <-c.stop:
			return

		case req := <-c.setLastEventTS:
			lastEventTS = req.ts
			req.resp <- struct{}{}

		case req := <-c.getLastEventTS:
			req.resp <- lastEventTS

		case req := <-c.getGroup:
			req.resp <- groups[req.groupID]

		case req := <-c.setGroup:
			groups[req.groupID] = req.gs
			req.resp <- struct{}{}

		case req := <-c.deleteGroup:
			old := groups[req.groupID]
			delete(groups, req.groupID)
			req.resp <- old

		case req := <-c.findByNostrGroupID:
			var found *GroupState
			for _, gs := range groups {
				if string(gs.NostrGroupID) == string(req.nostrGroupID) {
					found = gs
					break
				}
			}
			req.resp <- found

		case req := <-c.checkAndSetSeen:
			if _, seen := seenIDs[req.evKey]; seen {
				req.resp <- true
				continue
			}
			seenIDs[req.evKey] = struct{}{}
			if req.createdAt > lastEventTS {
				lastEventTS = req.createdAt
			}
			// Prune seenIDs to cap memory. 4096 is ~128KB of 32-byte event IDs.
			if len(seenIDs) > 4096 {
				seenIDs = make(map[string]struct{})
				seenIDs[req.evKey] = struct{}{}
			}
			req.resp <- false

		case req := <-c.markSeen:
			seenIDs[req.evKey] = struct{}{}

		case req := <-c.activeGroupIDs:
			ids := make([]string, 0, len(groups))
			for _, gs := range groups {
				if len(gs.NostrGroupID) > 0 {
					ids = append(ids, hex.Enc(gs.NostrGroupID))
				}
			}
			req.resp <- ids

		case req := <-c.groupMessageFilter:
			if len(groups) == 0 {
				req.resp <- nil
				continue
			}
			hValues := make([]any, 0, len(groups)+1)
			hValues = append(hValues, "h")
			for _, gs := range groups {
				if len(gs.NostrGroupID) > 0 {
					hValues = append(hValues, hex.Enc(gs.NostrGroupID))
				}
			}
			f := filter.New()
			f.Kinds = kind.NewS(kind.New(KindGroupMessage))
			f.Tags = tag.NewS(tag.NewFromAny(hValues...))
			if lastEventTS > 0 {
				f.Since = timestamp.FromUnix(lastEventTS - 172800)
			} else {
				f.Since = timestamp.FromUnix(time.Now().Unix() - 172800)
			}
			req.resp <- f

		case req := <-c.welcomeFilter:
			f := filter.New()
			f.Kinds = kind.NewS(kind.New(KindGiftWrap))
			f.Tags = tag.NewS(tag.NewFromAny("p", hex.Enc(c.crypto.Pub())))
			if lastEventTS > 0 {
				f.Since = timestamp.FromUnix(lastEventTS - 172800) // 2-day margin for NIP-59 timestamp randomization
			} else {
				f.Since = timestamp.FromUnix(time.Now().Unix() - 172800) // 2-day margin for NIP-59
			}
			req.resp <- f

		case req := <-c.subscriptionFilter:
			// Build welcome filter inline
			wf := filter.New()
			wf.Kinds = kind.NewS(kind.New(KindGiftWrap))
			wf.Tags = tag.NewS(tag.NewFromAny("p", hex.Enc(c.crypto.Pub())))
			if lastEventTS > 0 {
				wf.Since = timestamp.FromUnix(lastEventTS - 172800)
			} else {
				wf.Since = timestamp.FromUnix(time.Now().Unix() - 172800)
			}
			filters := []*filter.F{wf}

			// Build group message filter inline
			if len(groups) > 0 {
				hValues := make([]any, 0, len(groups)+1)
				hValues = append(hValues, "h")
				for _, gs := range groups {
					if len(gs.NostrGroupID) > 0 {
						hValues = append(hValues, hex.Enc(gs.NostrGroupID))
					}
				}
				gmf := filter.New()
				gmf.Kinds = kind.NewS(kind.New(KindGroupMessage))
				gmf.Tags = tag.NewS(tag.NewFromAny(hValues...))
				if lastEventTS > 0 {
					gmf.Since = timestamp.FromUnix(lastEventTS - 172800)
				} else {
					gmf.Since = timestamp.FromUnix(time.Now().Unix() - 172800)
				}
				filters = append(filters, gmf)
			}
			req.resp <- filter.NewS(filters...)

		case req := <-c.backupSnapshot:
			if time.Since(lastBackupTime) < 30*time.Second {
				req.resp <- &mcBackupSnapshot{skip: true}
				continue
			}
			gs := make([]groupStateBackup, 0, len(groups))
			for _, g := range groups {
				mlsBytes := g.mlsBytes
				var epoch uint64
				if g.group != nil {
					if b, err := g.group.Marshal(); err == nil {
						mlsBytes = b
					}
					epoch = g.group.Epoch()
				}
				gs = append(gs, groupStateBackup{
					GroupID:      hex.Enc(g.GroupID),
					NostrGroupID: hex.Enc(g.NostrGroupID),
					PeerPub:      hex.Enc(g.PeerPub),
					MLSState:     base64.StdEncoding.EncodeToString(mlsBytes),
					Epoch:        epoch,
				})
			}
			req.resp <- &mcBackupSnapshot{
				groups:      gs,
				lastEventTS: lastEventTS,
			}

		case req := <-c.setBackupTime:
			lastBackupTime = req.t
			req.resp <- struct{}{}

		case req := <-c.resetBackupTime:
			lastBackupTime = time.Time{}
			req.resp <- struct{}{}

		case req := <-c.restoreGroup:
			existing, hasLocal := groups[req.groupID]
			if hasLocal && existing.group != nil && existing.group.Epoch() >= req.epoch {
				req.resp <- false
				continue
			}
			groups[req.groupID] = req.gs
			req.resp <- true

		case req := <-c.restoreLastEventTS:
			if req.ts > lastEventTS {
				lastEventTS = req.ts
			}
			req.resp <- struct{}{}
		}
	}
}

// NewClient creates a Marmot client. The crypto provider handles identity,
// signing, and NIP-44 encryption. The store persists group state.
// The relay handles event transport.
func NewClient(crypto CryptoProvider, store GroupStore, relay RelayConnection, relays ...string) (*Client, error) {
	// Try loading persisted key package first. This ensures welcomes
	// created against the previous key package remain decryptable after restart.
	var kpp *mls.KeyPairPackage
	if data, err := store.LoadKeyPackage(); err == nil {
		if loaded, err := UnmarshalKeyPairPackage(data); err == nil {
			kpp = loaded
			log.I.F("loaded persisted MLS key package")
		}
	}
	if kpp == nil {
		var err error
		kpp, err = GenerateKeyPackage(crypto)
		if err != nil {
			return nil, fmt.Errorf("generate key package: %w", err)
		}
		// Persist the fresh key package for next restart.
		if data, err := MarshalKeyPairPackage(kpp); err == nil {
			_ = store.SaveKeyPackage(data)
		}
	}

	c := &Client{
		crypto:             crypto,
		store:              store,
		relay:              relay,
		relays:             relays,
		kpp:                kpp,
		setLastEventTS:     make(chan mcSetLastEventTSReq),
		getLastEventTS:     make(chan mcGetLastEventTSReq),
		getGroup:           make(chan mcGetGroupReq),
		setGroup:           make(chan mcSetGroupReq),
		deleteGroup:        make(chan mcDeleteGroupReq),
		findByNostrGroupID: make(chan mcFindByNostrGroupIDReq),
		checkAndSetSeen:    make(chan mcCheckAndSetSeenReq),
		markSeen:           make(chan mcMarkSeenReq, 16),
		activeGroupIDs:     make(chan mcActiveGroupIDsReq),
		groupMessageFilter: make(chan mcGroupMessageFilterReq),
		welcomeFilter:      make(chan mcWelcomeFilterReq),
		subscriptionFilter: make(chan mcSubscriptionFiltersReq),
		backupSnapshot:     make(chan mcBackupSnapshotReq),
		setBackupTime:      make(chan mcSetBackupTimeReq),
		resetBackupTime:    make(chan mcResetBackupTimeReq),
		restoreGroup:       make(chan mcRestoreGroupReq),
		restoreLastEventTS: make(chan mcRestoreLastEventTSReq),
		groupsChanged:      make(chan struct{}, 1),
		stop:               make(chan struct{}),
		done:               make(chan struct{}),
	}

	// Load persisted groups.
	initGroups := make(map[string]*GroupState)
	ids, err := store.ListGroups()
	if err == nil {
		for _, id := range ids {
			data, loadErr := store.LoadGroup(id)
			if loadErr != nil {
				_ = store.DeleteGroup(id)
				continue
			}
			ss, unmarshalErr := unmarshalGroupState(data)
			if unmarshalErr != nil {
				_ = store.DeleteGroup(id)
				continue
			}
			group, mlsErr := mls.UnmarshalGroup(ss.MLSState)
			if mlsErr != nil {
				log.I.F("discarding corrupted group %x: %v", id, mlsErr)
				_ = store.DeleteGroup(id)
				continue
			}
			initGroups[string(ss.GroupID)] = &GroupState{
				GroupID:      ss.GroupID,
				NostrGroupID: ss.NostrGroupID,
				PeerPub:      ss.PeerPub,
				group:        group,
				mlsBytes:     ss.MLSState,
			}
		}
		if len(initGroups) > 0 {
			log.I.F("restored %d persisted MLS groups", len(initGroups))
		}
	}

	go c.actor(initGroups)
	return c, nil
}

// Stop shuts down the actor goroutine.
func (c *Client) Stop() {
	close(c.stop)
	<-c.done
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
	req := mcSetLastEventTSReq{ts: ts, resp: make(chan struct{}, 1)}
	c.setLastEventTS <- req
	<-req.resp
}

// LastEventTS returns the highest event created_at seen so far.
// Persist this value to skip old events on next restart.
func (c *Client) LastEventTS() int64 {
	req := mcGetLastEventTSReq{resp: make(chan int64, 1)}
	c.getLastEventTS <- req
	return <-req.resp
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

// rotateKeyPackage generates a fresh KP, persists it, and publishes it.
// Called after processing a Welcome - the consumed init_key is destroyed
// per MLS spec, so the old KP on relays is now useless.
func (c *Client) rotateKeyPackage(ctx context.Context) {
	kpp, err := GenerateKeyPackage(c.crypto)
	if err != nil {
		log.W.F("rotate key package: generate: %v", err)
		return
	}
	c.kpp = kpp
	if data, err := MarshalKeyPairPackage(kpp); err == nil {
		_ = c.store.SaveKeyPackage(data)
	}
	if err := c.PublishKeyPackage(ctx); err != nil {
		log.W.F("rotate key package: publish: %v", err)
	}
}

// SendDM sends an encrypted DM to the given recipient. If no group exists,
// it fetches the recipient's key package, creates a group, and sends a
// welcome. Then it encrypts and publishes the message.
func (c *Client) SendDM(ctx context.Context, recipientPub []byte, plaintext []byte) error {
	groupID := DMGroupID(c.crypto.Pub(), recipientPub)

	req := mcGetGroupReq{groupID: string(groupID), resp: make(chan *GroupState, 1)}
	c.getGroup <- req
	gs := <-req.resp

	if gs == nil || gs.group == nil {
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

	// Re-persist after encrypt - MLS state may have advanced.
	c.persistGroup(gs)

	// Track this event ID so we skip it when it comes back via subscription.
	c.markSeen <- mcMarkSeenReq{evKey: string(ev.ID)}

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
	setReq := mcSetGroupReq{groupID: string(gs.GroupID), gs: gs, resp: make(chan struct{}, 1)}
	c.setGroup <- setReq
	<-setReq.resp

	c.persistGroup(gs)

	// Signal that filters need refreshing (so subscription includes kind 445 for this group).
	select {
	case c.groupsChanged <- struct{}{}:
	default:
	}

	c.backupAsync()
	return gs, nil
}

// HandleEvent processes an incoming event. Call this from the subscription loop.
func (c *Client) HandleEvent(ctx context.Context, ev *event.E) error {
	// Atomic check-and-set to avoid TOCTOU
	req := mcCheckAndSetSeenReq{
		evKey:     string(ev.ID),
		createdAt: ev.CreatedAt,
		resp:      make(chan bool, 1),
	}
	c.checkAndSetSeen <- req
	if <-req.resp {
		return nil // already seen
	}

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
		// Gift wraps we can't unwrap (wrong key, corrupt, etc.) are noise - skip.
		return nil
	}

	// Dispatch based on inner event kind.
	switch uw.Inner.Kind {
	case KindWelcome:
		return c.processWelcome(ctx, uw)
	case 14:
		// NIP-17 DM (kind 14 rumor inside gift wrap) - deliver as DM.
		if c.onDM != nil {
			c.onDM(uw.SenderPub, uw.Inner.Content)
		}
		return nil
	default:
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
		// Expected after restart - init_key from the consumed KeyPackage is
		// destroyed per MLS spec, so old welcomes are permanently undecryptable.
		log.I.F("skipping stale welcome from %s: %v", hex.Enc(senderPub), err)
		return nil
	}

	if len(gs.GroupID) == 0 {
		gs.GroupID = DMGroupID(c.crypto.Pub(), senderPub)
	}

	setReq := mcSetGroupReq{groupID: string(gs.GroupID), gs: gs, resp: make(chan struct{}, 1)}
	c.setGroup <- setReq
	<-setReq.resp

	c.persistGroup(gs)

	select {
	case c.groupsChanged <- struct{}{}:
	default:
	}

	log.I.F("joined DM group with %s (nostr_group_id: %s)", hex.Enc(senderPub), hex.Enc(gs.NostrGroupID))

	c.backupAsync()

	// MIP-00: rotate KeyPackage after Welcome - the consumed init_key is
	// dead, so peers fetching the old KP would fail to create a group.
	c.rotateKeyPackage(ctx)

	if c.onGroupJoined != nil {
		c.onGroupJoined(senderPub)
	}
	return nil
}

// findGroupByNostrGroupID looks up a group by its nostr_group_id (from "h" tag).
func (c *Client) findGroupByNostrGroupID(nostrGroupID []byte) *GroupState {
	req := mcFindByNostrGroupIDReq{nostrGroupID: nostrGroupID, resp: make(chan *GroupState, 1)}
	c.findByNostrGroupID <- req
	return <-req.resp
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

	plaintext, selfSent, err := gs.Decrypt(mlsCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// Re-persist after decrypt - MLS state may have advanced (epoch ratchet).
	c.persistGroup(gs)

	if selfSent {
		return nil
	}

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
	req := mcWelcomeFilterReq{resp: make(chan *filter.F, 1)}
	c.welcomeFilter <- req
	return <-req.resp
}

// GroupMessageFilter returns a filter for kind 445 events tagged with our
// active group IDs. Kind 445 events use ephemeral pubkeys (no "p" tag for
// the real recipient), so we subscribe via "#h" tags.
func (c *Client) GroupMessageFilter() *filter.F {
	req := mcGroupMessageFilterReq{resp: make(chan *filter.F, 1)}
	c.groupMessageFilter <- req
	return <-req.resp
}

// SubscriptionFilters returns filters for all events relevant to this client.
// Returns one or two filters depending on whether active groups exist.
func (c *Client) SubscriptionFilters() *filter.S {
	req := mcSubscriptionFiltersReq{resp: make(chan *filter.S, 1)}
	c.subscriptionFilter <- req
	return <-req.resp
}

// GroupsChanged returns a channel that is signalled whenever a new group
// is added (e.g. after processing a Welcome). Callers should use this to
// refresh subscription filters that include group-specific "#h" tags.
func (c *Client) GroupsChanged() <-chan struct{} {
	return c.groupsChanged
}

// ActiveGroupIDs returns hex-encoded nostr_group_ids of all active groups.
func (c *Client) ActiveGroupIDs() []string {
	req := mcActiveGroupIDsReq{resp: make(chan []string, 1)}
	c.activeGroupIDs <- req
	return <-req.resp
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

const KindAppSpecific = 30078

// BackupGroups serializes all active MLS groups, NIP-44 encrypts to self,
// and publishes as a kind 30078 event. Enables cross-device sync and
// recovery from IDB loss without full re-establishment.
func (c *Client) BackupGroups(ctx context.Context) error {
	snapReq := mcBackupSnapshotReq{resp: make(chan *mcBackupSnapshot, 1)}
	c.backupSnapshot <- snapReq
	snap := <-snapReq.resp
	if snap.skip {
		return nil
	}

	if len(snap.groups) == 0 {
		return nil
	}

	payload, err := json.Marshal(&backupPayload{Groups: snap.groups, LastEventTS: snap.lastEventTS})
	if err != nil {
		return fmt.Errorf("marshal backup: %w", err)
	}

	ciphertext, err := c.crypto.Nip44Encrypt(c.crypto.Pub(), payload)
	if err != nil {
		return fmt.Errorf("nip44 encrypt backup: %w", err)
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindAppSpecific
	ev.Content = []byte(ciphertext)
	ev.Tags = tag.NewS(tag.NewFromAny("d", "marmot-groups"))
	if err := c.crypto.SignEvent(ev); err != nil {
		return fmt.Errorf("sign backup event: %w", err)
	}
	if err := c.relay.Publish(ctx, ev); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}

	btReq := mcSetBackupTimeReq{t: time.Now(), resp: make(chan struct{}, 1)}
	c.setBackupTime <- btReq
	<-btReq.resp

	log.I.F("backed up %d MLS groups to relay", len(snap.groups))
	return nil
}

// RestoreGroups fetches the latest kind 30078 group backup from the relay,
// decrypts it, and restores MLS groups. Returns the number of groups restored.
func (c *Client) RestoreGroups(ctx context.Context) (int, error) {
	f := filter.New()
	f.Kinds = kind.NewS(kind.New(KindAppSpecific))
	f.Authors = &tag.T{T: [][]byte{c.crypto.Pub()}}
	f.Tags = tag.NewS(tag.NewFromAny("d", "marmot-groups"))
	limit := uint(1)
	f.Limit = &limit

	stream, err := c.relay.Subscribe(ctx, filter.NewS(f))
	if err != nil {
		return 0, fmt.Errorf("subscribe for backup: %w", err)
	}
	defer stream.Close()

	var backupEv *event.E
	select {
	case ev := <-stream.Events():
		backupEv = ev
	case <-time.After(10 * time.Second):
		return 0, nil // no backup found
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	if backupEv == nil {
		return 0, nil
	}

	plaintext, err := c.crypto.Nip44Decrypt(c.crypto.Pub(), string(backupEv.Content))
	if err != nil {
		return 0, fmt.Errorf("nip44 decrypt backup: %w", err)
	}

	var bp backupPayload
	if err := json.Unmarshal([]byte(plaintext), &bp); err != nil {
		return 0, fmt.Errorf("unmarshal backup: %w", err)
	}

	restored := 0
	for _, gsb := range bp.Groups {
		groupID, err := hex.Dec(gsb.GroupID)
		if err != nil {
			continue
		}
		nostrGroupID, err := hex.Dec(gsb.NostrGroupID)
		if err != nil {
			continue
		}
		peerPub, err := hex.Dec(gsb.PeerPub)
		if err != nil {
			continue
		}
		mlsBytes, err := base64.StdEncoding.DecodeString(gsb.MLSState)
		if err != nil {
			continue
		}
		group, err := mls.UnmarshalGroup(mlsBytes)
		if err != nil {
			log.I.F("backup: skipping corrupted group %s: %v", gsb.GroupID, err)
			continue
		}

		gs := &GroupState{
			GroupID:      groupID,
			NostrGroupID: nostrGroupID,
			PeerPub:      peerPub,
			group:        group,
			mlsBytes:     mlsBytes,
		}

		rReq := mcRestoreGroupReq{groupID: string(groupID), gs: gs, epoch: gsb.Epoch, resp: make(chan bool, 1)}
		c.restoreGroup <- rReq
		if <-rReq.resp {
			c.persistGroup(gs)
			restored++
		}
	}

	if bp.LastEventTS > 0 {
		tsReq := mcRestoreLastEventTSReq{ts: bp.LastEventTS, resp: make(chan struct{}, 1)}
		c.restoreLastEventTS <- tsReq
		<-tsReq.resp
	}

	if restored > 0 {
		select {
		case c.groupsChanged <- struct{}{}:
		default:
		}
		log.I.F("restored %d MLS groups from relay backup", restored)
	}

	return restored, nil
}

// RatchetGroup destroys the existing MLS group with a peer, publishes a
// delete event for the old kind 445 events, and creates a fresh group.
// The caller should also clear local DM history for the peer.
func (c *Client) RatchetGroup(ctx context.Context, peerPub []byte) error {
	groupID := DMGroupID(c.crypto.Pub(), peerPub)

	delReq := mcDeleteGroupReq{groupID: string(groupID), resp: make(chan *GroupState, 1)}
	c.deleteGroup <- delReq
	oldGS := <-delReq.resp

	_ = c.store.DeleteGroup(groupID)

	// Publish kind 5 to request deletion of old kind 445 events by h-tag.
	if oldGS != nil && len(oldGS.NostrGroupID) > 0 {
		delEv := event.New()
		delEv.CreatedAt = time.Now().Unix()
		delEv.Kind = 5
		delEv.Tags = tag.NewS(
			tag.NewFromAny("h", hex.Enc(oldGS.NostrGroupID)),
			tag.NewFromAny("k", "445"),
		)
		if err := c.crypto.SignEvent(delEv); err != nil {
			log.W.F("ratchet: sign delete event: %v", err)
		} else if err := c.relay.Publish(ctx, delEv); err != nil {
			log.W.F("ratchet: publish delete event: %v", err)
		} else {
			log.I.F("ratchet: published delete for group %s", hex.Enc(oldGS.NostrGroupID))
		}
	}

	// Create new group with the same peer.
	if _, err := c.establishGroup(ctx, peerPub); err != nil {
		return fmt.Errorf("ratchet: establish new group: %w", err)
	}

	// Backup new state to relay.
	rstReq := mcResetBackupTimeReq{resp: make(chan struct{}, 1)}
	c.resetBackupTime <- rstReq
	<-rstReq.resp

	go func() {
		if err := c.BackupGroups(context.Background()); err != nil {
			log.W.F("ratchet: backup: %v", err)
		}
	}()

	return nil
}

// backupAsync triggers a debounced backup in a fire-and-forget goroutine.
func (c *Client) backupAsync() {
	go func() {
		if err := c.BackupGroups(context.Background()); err != nil {
			log.W.F("async backup: %v", err)
		}
	}()
}
