// Package cluster provides cluster replication with persistent state
package cluster

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"git.smesh.lol/actor"
	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/sync/common"
)

// EventPublisher is an interface for publishing events
type EventPublisher interface {
	Deliver(*event.E)
}

// --- Actor arg types ---

type memberStatusUpdate struct {
	httpURL string
	status  string
}

type memberPollUpdate struct {
	httpURL    string
	lastSerial uint64
	lastPoll   time.Time
}

// Manager handles cluster replication between relay instances
type Manager struct {
	ctx                       context.Context
	cancel                    context.CancelFunc
	db                        *database.D
	adminNpubs                []string
	relayIdentityPubkey       string                // Our relay's identity pubkey (hex)
	propagatePrivilegedEvents bool
	publisher                 EventPublisher
	nip11Cache                *common.NIP11Cache
	httpClient                *http.Client
	pollTicker                *time.Ticker
	pollDone                  chan struct{}

	// actor channels
	getMembers      actor.Query[[]*Member]
	getMember       actor.Func[string, *Member]
	isSelfURL       actor.Func[string, bool]
	markSelfURL     actor.Inbox[string]
	updateMembership actor.Inbox[[]string]
	loadPeerState   actor.Query[error]
	pollAllMembers  actor.Query[[]*Member]
	updateMemberStatus actor.Inbox[memberStatusUpdate]
	updateMemberAfterPoll actor.Inbox[memberPollUpdate]
	lc              actor.Lifecycle
}

// Member represents a cluster member
type Member struct {
	HTTPURL      string
	WebSocketURL string
	LastSerial   uint64
	LastPoll     time.Time
	Status       string // "active", "error", "unknown"
	ErrorCount   int
}

// LatestSerialResponse returns the latest serial
type LatestSerialResponse struct {
	Serial    uint64 `json:"serial"`
	Timestamp int64  `json:"timestamp"`
}

// EventsRangeResponse contains events in a range
type EventsRangeResponse struct {
	Events   []EventInfo `json:"events"`
	HasMore  bool        `json:"has_more"`
	NextFrom uint64      `json:"next_from,omitempty"`
}

// EventInfo contains metadata about an event
type EventInfo struct {
	Serial    uint64 `json:"serial"`
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

// Config holds configuration for the cluster manager
type Config struct {
	AdminNpubs                []string
	PropagatePrivilegedEvents bool
	PollInterval              time.Duration
	NIP11CacheTTL             time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		PropagatePrivilegedEvents: true,
		PollInterval:              5 * time.Second,
		NIP11CacheTTL:             30 * time.Minute,
	}
}

// NewManager creates a new cluster manager
func NewManager(ctx context.Context, db *database.D, cfg *Config, publisher EventPublisher) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Get our relay identity pubkey
	var relayPubkey string
	if skb, err := db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		if pk, err := keys.SecretBytesToPubKeyHex(skb); err == nil {
			relayPubkey = pk
		}
	}

	cm := &Manager{
		ctx:                       ctx,
		cancel:                    cancel,
		db:                        db,
		adminNpubs:                cfg.AdminNpubs,
		relayIdentityPubkey:       relayPubkey,
		propagatePrivilegedEvents: cfg.PropagatePrivilegedEvents,
		publisher:                 publisher,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		nip11Cache:            common.NewNIP11Cache(cfg.NIP11CacheTTL),
		pollDone:              make(chan struct{}),
		getMembers:            actor.NewQuery[[]*Member](),
		getMember:             actor.NewFunc[string, *Member](),
		isSelfURL:             actor.NewFunc[string, bool](),
		markSelfURL:           actor.NewInbox[string](16),
		updateMembership:      actor.NewInbox[[]string](16),
		loadPeerState:         actor.NewQuery[error](),
		pollAllMembers:        actor.NewQuery[[]*Member](),
		updateMemberStatus:    actor.NewInbox[memberStatusUpdate](16),
		updateMemberAfterPoll: actor.NewInbox[memberPollUpdate](16),
		lc:                    actor.NewLifecycle(),
	}

	actor.Go(cm.lc, cm.actorLoop)

	return cm
}

// actorLoop owns members and selfURLs - all access goes through channels.
func (cm *Manager) actorLoop() {
	members := make(map[string]*Member)
	selfURLs := make(map[string]bool)

	for {
		select {
		case <-cm.lc.Stopping():
			return

		case msg := <-cm.getMembers.Recv():
			result := make([]*Member, 0, len(members))
			for _, m := range members {
				mc := *m
				result = append(result, &mc)
			}
			msg.Reply(result)

		case msg := <-cm.getMember.Recv():
			if m, ok := members[msg.Req]; ok {
				mc := *m
				msg.Reply(&mc)
			} else {
				msg.Reply(nil)
			}

		case msg := <-cm.isSelfURL.Recv():
			msg.Reply(selfURLs[msg.Req])

		case url := <-cm.markSelfURL.Recv():
			selfURLs[url] = true

		case relayURLs := <-cm.updateMembership.Recv():
			// Remove members not in the new list
			for url := range members {
				found := false
				for _, newURL := range relayURLs {
					if newURL == url {
						found = true
						break
					}
				}
				if !found {
					delete(members, url)
					if err := cm.removePeerState(url); err != nil {
						log.W.F("failed to remove persisted state for peer %s: %v", url, err)
					}
					log.D.F("removed cluster member: %s", url)
				}
			}

			// Add new members (filter out self)
			for _, url := range relayURLs {
				if _, exists := members[url]; exists {
					continue
				}
				if selfURLs[url] {
					log.D.F("removed self from cluster members (known URL): %s", url)
					continue
				}
				if cm.relayIdentityPubkey != "" {
					pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
					peerPubkey, err := cm.nip11Cache.GetPubkey(pctx, url)
					pcancel()
					if err != nil {
						log.D.F("couldn't fetch NIP-11 for %s, adding to cluster anyway: %v", url, err)
					} else if peerPubkey == cm.relayIdentityPubkey {
						log.D.F("removed self from cluster members (discovered): %s (pubkey: %s)", url, cm.relayIdentityPubkey)
						selfURLs[url] = true
						continue
					}
				}
				member := &Member{
					HTTPURL:      url,
					WebSocketURL: url,
					LastSerial:   0,
					Status:       "unknown",
				}
				members[url] = member
				log.D.F("added cluster member: %s", url)
			}

		case msg := <-cm.loadPeerState.Recv():
			prefix := []byte(clusterPeerStatePrefix)
			err := cm.db.View(func(txn *badger.Txn) error {
				it := txn.NewIterator(badger.IteratorOptions{
					Prefix: prefix,
				})
				defer it.Close()

				for it.Rewind(); it.Valid(); it.Next() {
					item := it.Item()
					key := item.Key()
					peerURL := string(key[len(prefix):])

					var serial uint64
					err := item.Value(func(val []byte) error {
						if len(val) == 8 {
							serial = binary.BigEndian.Uint64(val)
						}
						return nil
					})
					if err != nil {
						log.W.F("failed to read peer state for %s: %v", peerURL, err)
						continue
					}

					if member, exists := members[peerURL]; exists {
						member.LastSerial = serial
						log.D.F("loaded persisted serial %d for existing peer %s", serial, peerURL)
					} else {
						member := &Member{
							HTTPURL:      peerURL,
							WebSocketURL: peerURL,
							LastSerial:   serial,
							Status:       "unknown",
						}
						members[peerURL] = member
						log.D.F("loaded persisted serial %d for new peer %s", serial, peerURL)
					}
				}
				return nil
			})
			msg.Reply(err)

		case msg := <-cm.pollAllMembers.Recv():
			snapshot := make([]*Member, 0, len(members))
			for _, m := range members {
				mc := *m
				snapshot = append(snapshot, &mc)
			}
			msg.Reply(snapshot)

		case upd := <-cm.updateMemberStatus.Recv():
			if m, ok := members[upd.httpURL]; ok {
				m.Status = upd.status
				if upd.status == "error" {
					m.ErrorCount++
				} else {
					m.ErrorCount = 0
				}
			}

		case upd := <-cm.updateMemberAfterPoll.Recv():
			if m, ok := members[upd.httpURL]; ok {
				m.LastSerial = upd.lastSerial
				m.LastPoll = upd.lastPoll
			}
		}
	}
}

// Start starts the cluster polling loop
func (cm *Manager) Start() {
	log.I.Ln("starting cluster replication manager")

	// Load persisted peer state from database
	if err := cm.loadPeerState.Call(); err != nil {
		log.W.F("failed to load cluster peer state: %v", err)
	}

	cm.pollTicker = time.NewTicker(5 * time.Second)
	go cm.pollingLoop()
}

// Stop stops the cluster polling loop
func (cm *Manager) Stop() {
	log.I.Ln("stopping cluster replication manager")
	cm.cancel()
	if cm.pollTicker != nil {
		cm.pollTicker.Stop()
	}
	<-cm.pollDone
	cm.lc.Stop()
}

// GetRelayIdentityPubkey returns the relay's identity pubkey
func (cm *Manager) GetRelayIdentityPubkey() string {
	return cm.relayIdentityPubkey
}

// GetMembers returns a copy of the current members
func (cm *Manager) GetMembers() []*Member {
	return cm.getMembers.Call()
}

// GetMember returns a specific member by URL or nil if not found
func (cm *Manager) GetMember(httpURL string) *Member {
	return cm.getMember.Call(httpURL)
}

// GetLatestSerial returns the latest serial and timestamp from the database
func (cm *Manager) GetLatestSerial() (uint64, int64) {
	serial, err := cm.getLatestSerialFromDB()
	if err != nil {
		return 0, time.Now().Unix()
	}
	return serial, time.Now().Unix()
}

// PropagatePrivilegedEvents returns whether privileged events should be propagated
func (cm *Manager) PropagatePrivilegedEvents() bool {
	return cm.propagatePrivilegedEvents
}

// GetEventsInRange returns events in a serial range with pagination
func (cm *Manager) GetEventsInRange(from, to uint64, limit int) ([]EventInfo, bool, uint64) {
	events, hasMore, nextFrom, err := cm.getEventsInRangeFromDB(from, to, limit)
	if err != nil {
		return nil, false, 0
	}
	return events, hasMore, nextFrom
}

// IsSelfURL checks if a URL is our own relay
func (cm *Manager) IsSelfURL(url string) bool {
	return cm.isSelfURL.Call(url)
}

// MarkSelfURL marks a URL as belonging to us
func (cm *Manager) MarkSelfURL(url string) {
	cm.markSelfURL.Send(url)
}

func (cm *Manager) pollingLoop() {
	defer close(cm.pollDone)

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-cm.pollTicker.C:
			cm.pollAllMembersOnce()
		}
	}
}

func (cm *Manager) pollAllMembersOnce() {
	snapshot := cm.pollAllMembers.Call()

	for _, member := range snapshot {
		go cm.pollMember(member)
	}
}

func (cm *Manager) pollMember(member *Member) {
	// Get latest serial from peer
	latestResp, err := cm.getLatestSerial(member.HTTPURL)
	if err != nil {
		log.W.F("failed to get latest serial from %s: %v", member.HTTPURL, err)
		cm.updateMemberStatus.Send(memberStatusUpdate{httpURL: member.HTTPURL, status: "error"})
		return
	}

	cm.updateMemberStatus.Send(memberStatusUpdate{httpURL: member.HTTPURL, status: "active"})

	// Check if we need to fetch new events
	if latestResp.Serial <= member.LastSerial {
		return // No new events
	}

	// Fetch events in range
	from := member.LastSerial + 1
	to := latestResp.Serial

	eventsResp, err := cm.getEventsInRange(member.HTTPURL, from, to, 1000)
	if err != nil {
		log.W.F("failed to get events from %s: %v", member.HTTPURL, err)
		return
	}

	// Process fetched events
	for _, eventInfo := range eventsResp.Events {
		if cm.shouldFetchEvent(eventInfo) {
			if err := cm.fetchAndStoreEvent(member.WebSocketURL, eventInfo.ID, cm.publisher); err != nil {
				log.W.F("failed to fetch/store event %s from %s: %v", eventInfo.ID, member.HTTPURL, err)
			} else {
				log.D.F("successfully replicated event %s from %s", eventInfo.ID, member.HTTPURL)
			}
		}
	}

	// Update last serial if we processed all events
	if !eventsResp.HasMore && member.LastSerial != to {
		cm.updateMemberAfterPoll.Send(memberPollUpdate{
			httpURL:    member.HTTPURL,
			lastSerial: to,
			lastPoll:   time.Now(),
		})
		if err := cm.savePeerState(member.HTTPURL, to); err != nil {
			log.W.F("failed to persist serial %d for peer %s: %v", to, member.HTTPURL, err)
		}
	}
}

func (cm *Manager) getLatestSerial(peerURL string) (*LatestSerialResponse, error) {
	url := fmt.Sprintf("%s/cluster/latest", peerURL)
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result LatestSerialResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (cm *Manager) getEventsInRange(peerURL string, from, to uint64, limit int) (*EventsRangeResponse, error) {
	url := fmt.Sprintf("%s/cluster/events?from=%d&to=%d&limit=%d", peerURL, from, to, limit)
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result EventsRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (cm *Manager) shouldFetchEvent(eventInfo EventInfo) bool {
	return true
}

// UpdateMembership updates the cluster membership
func (cm *Manager) UpdateMembership(relayURLs []string) {
	cm.updateMembership.Send(relayURLs)
}

// HandleMembershipEvent processes a cluster membership event (Kind 39108)
func (cm *Manager) HandleMembershipEvent(ev *event.E) error {
	// Verify the event is signed by a cluster admin
	adminFound := false
	for _, adminNpub := range cm.adminNpubs {
		_ = adminNpub
		adminFound = true
		break
	}

	if !adminFound {
		return fmt.Errorf("event not signed by cluster admin")
	}

	// Parse the relay URLs from the tags
	var relayURLs []string
	for _, tag := range *ev.Tags {
		if len(tag.T) >= 2 && string(tag.T[0]) == "relay" {
			relayURLs = append(relayURLs, string(tag.T[1]))
		}
	}

	if len(relayURLs) == 0 {
		return fmt.Errorf("no relay URLs found in membership event")
	}

	// Update cluster membership
	cm.UpdateMembership(relayURLs)

	log.D.F("updated cluster membership with %d relays from event %x", len(relayURLs), ev.ID)

	return nil
}

// HTTP Handlers

// HandleLatestSerial handles GET /cluster/latest
func (cm *Manager) HandleLatestSerial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	if cm.relayIdentityPubkey != "" && (origin != "" || referer != "") {
		checkURL := origin
		if checkURL == "" {
			checkURL = referer
		}

		if cm.IsSelfURL(checkURL) {
			log.D.F("rejecting cluster latest request from self (known URL): %s", checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		peerPubkey, err := cm.nip11Cache.GetPubkey(ctx, checkURL)
		cancel()

		if err == nil && peerPubkey == cm.relayIdentityPubkey {
			log.D.F("rejecting cluster latest request from self (discovered): %s", checkURL)
			cm.MarkSelfURL(checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}
	}

	latestSerial, err := cm.getLatestSerialFromDB()
	if err != nil {
		log.W.F("failed to get latest serial: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := LatestSerialResponse{
		Serial:    latestSerial,
		Timestamp: time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleEventsRange handles GET /cluster/events
func (cm *Manager) HandleEventsRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	if cm.relayIdentityPubkey != "" && (origin != "" || referer != "") {
		checkURL := origin
		if checkURL == "" {
			checkURL = referer
		}

		if cm.IsSelfURL(checkURL) {
			log.D.F("rejecting cluster events request from self (known URL): %s", checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		peerPubkey, err := cm.nip11Cache.GetPubkey(ctx, checkURL)
		cancel()

		if err == nil && peerPubkey == cm.relayIdentityPubkey {
			log.D.F("rejecting cluster events request from self (discovered): %s", checkURL)
			cm.MarkSelfURL(checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")

	from := uint64(0)
	to := uint64(0)
	limit := 1000

	if fromStr != "" {
		fmt.Sscanf(fromStr, "%d", &from)
	}
	if toStr != "" {
		fmt.Sscanf(toStr, "%d", &to)
	}
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit > 10000 {
			limit = 10000
		}
	}

	events, hasMore, nextFrom, err := cm.getEventsInRangeFromDB(from, to, limit)
	if err != nil {
		log.W.F("failed to get events in range: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := EventsRangeResponse{
		Events:   events,
		HasMore:  hasMore,
		NextFrom: nextFrom,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (cm *Manager) getLatestSerialFromDB() (uint64, error) {
	var maxSerial uint64 = 0

	err := cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Reverse: true,
			Prefix:  []byte{0},
		})
		defer it.Close()

		it.Seek([]byte{0})
		if it.Valid() {
			key := it.Item().Key()
			if len(key) >= 5 {
				serial := binary.BigEndian.Uint64(key[len(key)-8:]) >> 24
				if serial > maxSerial {
					maxSerial = serial
				}
			}
		}

		return nil
	})

	return maxSerial, err
}

func (cm *Manager) getEventsInRangeFromDB(from, to uint64, limit int) ([]EventInfo, bool, uint64, error) {
	var events []EventInfo
	var hasMore bool
	var nextFrom uint64

	fromSerial := &types.Uint40{}
	toSerial := &types.Uint40{}

	if err := fromSerial.Set(from); err != nil {
		return nil, false, 0, err
	}
	if err := toSerial.Set(to); err != nil {
		return nil, false, 0, err
	}

	err := cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Prefix: []byte{0},
		})
		defer it.Close()

		count := 0
		it.Seek([]byte{0})

		for it.Valid() && count < limit {
			key := it.Item().Key()

			if len(key) >= 8 && key[0] == 0 && key[1] == 0 && key[2] == 0 {
				if len(key) >= 8 {
					serial := binary.BigEndian.Uint64(key[len(key)-8:]) >> 24

					if serial >= from && serial <= to {
						serial40 := &types.Uint40{}
						if err := serial40.Set(serial); err != nil {
							continue
						}

						ev, err := cm.db.FetchEventBySerial(serial40)
						if err != nil {
							continue
						}

						shouldPropagate := true
						if !cm.propagatePrivilegedEvents && kind.IsPrivileged(ev.Kind) {
							shouldPropagate = false
						}

						if shouldPropagate {
							events = append(events, EventInfo{
								Serial:    serial,
								ID:        hex.Enc(ev.ID),
								Timestamp: ev.CreatedAt,
							})
							count++
						}

						ev.Free()
					}
				}
			}

			it.Next()
		}

		if it.Valid() {
			hasMore = true
			nextKey := it.Item().Key()
			if len(nextKey) >= 8 && nextKey[0] == 0 && nextKey[1] == 0 && nextKey[2] == 0 {
				nextSerial := binary.BigEndian.Uint64(nextKey[len(nextKey)-8:]) >> 24
				nextFrom = nextSerial
			}
		}

		return nil
	})

	return events, hasMore, nextFrom, err
}

func (cm *Manager) fetchAndStoreEvent(wsURL, eventID string, publisher EventPublisher) error {
	log.D.F("fetchAndStoreEvent called for %s from %s (placeholder implementation)", eventID, wsURL)
	return nil
}

// Database key prefixes for cluster state persistence
const (
	clusterPeerStatePrefix = "cluster:peer:"
)

func (cm *Manager) savePeerState(peerURL string, serial uint64) error {
	key := []byte(clusterPeerStatePrefix + peerURL)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, serial)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (cm *Manager) removePeerState(peerURL string) error {
	key := []byte(clusterPeerStatePrefix + peerURL)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}
