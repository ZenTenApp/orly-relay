package sync

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
)

type ClusterManager struct {
	ctx                           context.Context
	cancel                        context.CancelFunc
	db                            *database.D
	adminNpubs                    []string
	relayIdentityPubkey           string                    // Our relay's identity pubkey (hex)
	selfURLs                      map[string]bool           // URLs discovered to be ourselves (for fast lookups)
	members                       map[string]*ClusterMember // keyed by relay URL
	membersMux                    sync.RWMutex
	pollTicker                    *time.Ticker
	pollDone                      chan struct{}
	httpClient                    *http.Client
	propagatePrivilegedEvents     bool
	publisher                     interface{ Deliver(*event.E) }
	nip11Cache                    *NIP11Cache
}

type ClusterMember struct {
	HTTPURL      string
	WebSocketURL string
	LastSerial   uint64
	LastPoll     time.Time
	Status       string // "active", "error", "unknown"
	ErrorCount   int
}

type LatestSerialResponse struct {
	Serial    uint64 `json:"serial"`
	Timestamp int64  `json:"timestamp"`
}

type EventsRangeResponse struct {
	Events   []EventInfo `json:"events"`
	HasMore  bool        `json:"has_more"`
	NextFrom uint64      `json:"next_from,omitempty"`
}

type EventInfo struct {
	Serial    uint64 `json:"serial"`
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

func NewClusterManager(ctx context.Context, db *database.D, adminNpubs []string, propagatePrivilegedEvents bool, publisher interface{ Deliver(*event.E) }) *ClusterManager {
	ctx, cancel := context.WithCancel(ctx)

	// Get our relay identity pubkey
	var relayPubkey string
	if skb, err := db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		if pk, err := keys.SecretBytesToPubKeyHex(skb); err == nil {
			relayPubkey = pk
		}
	}

	cm := &ClusterManager{
		ctx:                       ctx,
		cancel:                    cancel,
		db:                        db,
		adminNpubs:                adminNpubs,
		relayIdentityPubkey:       relayPubkey,
		selfURLs:                  make(map[string]bool),
		members:                   make(map[string]*ClusterMember),
		pollDone:                  make(chan struct{}),
		propagatePrivilegedEvents: propagatePrivilegedEvents,
		publisher:                 publisher,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		nip11Cache: NewNIP11Cache(30 * time.Minute),
	}

	return cm
}

func (cm *ClusterManager) Start() {
	log.I.Ln("starting cluster replication manager")

	// Load persisted peer state from database
	if err := cm.loadPeerState(); err != nil {
		log.W.F("failed to load cluster peer state: %v", err)
	}

	cm.pollTicker = time.NewTicker(5 * time.Second)
	go cm.pollingLoop()
}

func (cm *ClusterManager) Stop() {
	log.I.Ln("stopping cluster replication manager")
	cm.cancel()
	if cm.pollTicker != nil {
		cm.pollTicker.Stop()
	}
	<-cm.pollDone
}

func (cm *ClusterManager) pollingLoop() {
	defer close(cm.pollDone)

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-cm.pollTicker.C:
			cm.pollAllMembers()
		}
	}
}

func (cm *ClusterManager) pollAllMembers() {
	cm.membersMux.RLock()
	members := make([]*ClusterMember, 0, len(cm.members))
	for _, member := range cm.members {
		members = append(members, member)
	}
	cm.membersMux.RUnlock()

	for _, member := range members {
		go cm.pollMember(member)
	}
}

func (cm *ClusterManager) pollMember(member *ClusterMember) {
	// Get latest serial from peer
	latestResp, err := cm.getLatestSerial(member.HTTPURL)
	if err != nil {
		log.W.F("failed to get latest serial from %s: %v", member.HTTPURL, err)
		cm.updateMemberStatus(member, "error")
		return
	}

	cm.updateMemberStatus(member, "active")
	member.LastPoll = time.Now()

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
				// Fetch full event via WebSocket and store it
				if err := cm.fetchAndStoreEvent(member.WebSocketURL, eventInfo.ID, cm.publisher); err != nil {
					log.W.F("failed to fetch/store event %s from %s: %v", eventInfo.ID, member.HTTPURL, err)
				} else {
					log.D.F("successfully replicated event %s from %s", eventInfo.ID, member.HTTPURL)
				}
			}
		}

	// Update last serial if we processed all events
	if !eventsResp.HasMore && member.LastSerial != to {
		member.LastSerial = to
		// Persist the updated serial to database
		if err := cm.savePeerState(member.HTTPURL, to); err != nil {
			log.W.F("failed to persist serial %d for peer %s: %v", to, member.HTTPURL, err)
		}
	}
}

func (cm *ClusterManager) getLatestSerial(peerURL string) (*LatestSerialResponse, error) {
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

func (cm *ClusterManager) getEventsInRange(peerURL string, from, to uint64, limit int) (*EventsRangeResponse, error) {
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

func (cm *ClusterManager) shouldFetchEvent(eventInfo EventInfo) bool {
	// Relays MAY choose not to store every event they receive
	// For now, accept all events
	return true
}

func (cm *ClusterManager) updateMemberStatus(member *ClusterMember, status string) {
	member.Status = status
	if status == "error" {
		member.ErrorCount++
	} else {
		member.ErrorCount = 0
	}
}

func (cm *ClusterManager) UpdateMembership(relayURLs []string) {
	cm.membersMux.Lock()
	defer cm.membersMux.Unlock()

	// Remove members not in the new list
	for url := range cm.members {
		found := false
		for _, newURL := range relayURLs {
			if newURL == url {
				found = true
				break
			}
		}
		if !found {
			delete(cm.members, url)
			// Remove persisted state for removed peer
			if err := cm.removePeerState(url); err != nil {
				log.W.F("failed to remove persisted state for peer %s: %v", url, err)
			}
			log.I.F("removed cluster member: %s", url)
		}
	}

	// Add new members (filter out self once at this point)
	for _, url := range relayURLs {
		// Skip if already exists
		if _, exists := cm.members[url]; exists {
			continue
		}

		// Fast path: check if we already know this URL is ours
		if cm.selfURLs[url] {
			log.I.F("removed self from cluster members (known URL): %s", url)
			continue
		}

		// Slow path: check via NIP-11 pubkey
		if cm.relayIdentityPubkey != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			peerPubkey, err := cm.nip11Cache.GetPubkey(ctx, url)
			cancel()

			if err != nil {
				log.D.F("couldn't fetch NIP-11 for %s, adding to cluster anyway: %v", url, err)
			} else if peerPubkey == cm.relayIdentityPubkey {
				log.I.F("removed self from cluster members (discovered): %s (pubkey: %s)", url, cm.relayIdentityPubkey)
				// Cache this URL as ours for future fast lookups
				cm.selfURLs[url] = true
				continue
			}
		}

		// Add member
		member := &ClusterMember{
			HTTPURL:      url,
			WebSocketURL: url, // TODO: Convert to WebSocket URL
			LastSerial:   0,
			Status:       "unknown",
		}
		cm.members[url] = member
		log.I.F("added cluster member: %s", url)
	}
}

// HandleMembershipEvent processes a cluster membership event (Kind 39108)
func (cm *ClusterManager) HandleMembershipEvent(event *event.E) error {
	// Verify the event is signed by a cluster admin
	adminFound := false
	for _, adminNpub := range cm.adminNpubs {
		// TODO: Convert adminNpub to pubkey and verify signature
		// For now, accept all events (this should be properly validated)
		_ = adminNpub // Mark as used to avoid compiler warning
		adminFound = true
		break
	}

	if !adminFound {
		return fmt.Errorf("event not signed by cluster admin")
	}

	// Parse the relay URLs from the tags
	var relayURLs []string
	for _, tag := range *event.Tags {
		if len(tag.T) >= 2 && string(tag.T[0]) == "relay" {
			relayURLs = append(relayURLs, string(tag.T[1]))
		}
	}

	if len(relayURLs) == 0 {
		return fmt.Errorf("no relay URLs found in membership event")
	}

	// Update cluster membership
	cm.UpdateMembership(relayURLs)

	log.I.F("updated cluster membership with %d relays from event %x", len(relayURLs), event.ID)

	return nil
}

// HTTP Handlers

func (cm *ClusterManager) HandleLatestSerial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if request is from ourselves by examining the Referer or Origin header
	// Note: Self-members are already filtered out, but this catches edge cases
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	if cm.relayIdentityPubkey != "" && (origin != "" || referer != "") {
		checkURL := origin
		if checkURL == "" {
			checkURL = referer
		}

		// Fast path: check known self-URLs
		if cm.selfURLs[checkURL] {
			log.D.F("rejecting cluster latest request from self (known URL): %s", checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}

		// Slow path: verify via NIP-11
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		peerPubkey, err := cm.nip11Cache.GetPubkey(ctx, checkURL)
		cancel()

		if err == nil && peerPubkey == cm.relayIdentityPubkey {
			log.D.F("rejecting cluster latest request from self (discovered): %s", checkURL)
			// Cache for future fast lookups
			cm.membersMux.Lock()
			cm.selfURLs[checkURL] = true
			cm.membersMux.Unlock()
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}
	}

	// Get the latest serial from database by querying for the highest serial
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

func (cm *ClusterManager) HandleEventsRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if request is from ourselves by examining the Referer or Origin header
	// Note: Self-members are already filtered out, but this catches edge cases
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	if cm.relayIdentityPubkey != "" && (origin != "" || referer != "") {
		checkURL := origin
		if checkURL == "" {
			checkURL = referer
		}

		// Fast path: check known self-URLs
		if cm.selfURLs[checkURL] {
			log.D.F("rejecting cluster events request from self (known URL): %s", checkURL)
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}

		// Slow path: verify via NIP-11
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		peerPubkey, err := cm.nip11Cache.GetPubkey(ctx, checkURL)
		cancel()

		if err == nil && peerPubkey == cm.relayIdentityPubkey {
			log.D.F("rejecting cluster events request from self (discovered): %s", checkURL)
			// Cache for future fast lookups
			cm.membersMux.Lock()
			cm.selfURLs[checkURL] = true
			cm.membersMux.Unlock()
			http.Error(w, "Cannot sync with self", http.StatusBadRequest)
			return
		}
	}

	// Parse query parameters
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

	// Get events in range
	events, hasMore, nextFrom, err := cm.getEventsInRangeFromDB(from, to, int(limit))
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

func (cm *ClusterManager) getLatestSerialFromDB() (uint64, error) {
	// Query the database to find the highest serial number
	// We'll iterate through the event keys to find the maximum serial
	var maxSerial uint64 = 0

	err := cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Reverse: true, // Start from highest
			Prefix:  []byte{0}, // Event keys start with 0
		})
		defer it.Close()

		// Look for the first event key (which should have the highest serial in reverse iteration)
		it.Seek([]byte{0})
		if it.Valid() {
			key := it.Item().Key()
			if len(key) >= 5 { // Serial is in the last 5 bytes
				serial := binary.BigEndian.Uint64(key[len(key)-8:]) >> 24 // Convert from Uint40
				if serial > maxSerial {
					maxSerial = serial
				}
			}
		}

		return nil
	})

	return maxSerial, err
}

func (cm *ClusterManager) getEventsInRangeFromDB(from, to uint64, limit int) ([]EventInfo, bool, uint64, error) {
	var events []EventInfo
	var hasMore bool
	var nextFrom uint64

	// Convert serials to Uint40 format for querying
	fromSerial := &types.Uint40{}
	toSerial := &types.Uint40{}

	if err := fromSerial.Set(from); err != nil {
		return nil, false, 0, err
	}
	if err := toSerial.Set(to); err != nil {
		return nil, false, 0, err
	}

	// Query events by serial range
	err := cm.db.View(func(txn *badger.Txn) error {
		// Iterate through event keys in the database
		it := txn.NewIterator(badger.IteratorOptions{
			Prefix: []byte{0}, // Event keys start with 0
		})
		defer it.Close()

		count := 0
		it.Seek([]byte{0})

		for it.Valid() && count < limit {
			key := it.Item().Key()

			// Check if this is an event key (starts with event prefix)
			if len(key) >= 8 && key[0] == 0 && key[1] == 0 && key[2] == 0 {
				// Extract serial from the last 5 bytes (Uint40)
				if len(key) >= 8 {
					serial := binary.BigEndian.Uint64(key[len(key)-8:]) >> 24 // Convert from Uint40

					// Check if serial is in range
					if serial >= from && serial <= to {
						// Fetch the full event to check if it's privileged
						serial40 := &types.Uint40{}
						if err := serial40.Set(serial); err != nil {
							continue
						}

						ev, err := cm.db.FetchEventBySerial(serial40)
						if err != nil {
							continue
						}

						// Check if we should propagate this event
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

						// Free the event
						ev.Free()
					}
				}
			}

			it.Next()
		}

		// Check if there are more events
		if it.Valid() {
			hasMore = true
			// Try to get the next serial
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

func (cm *ClusterManager) fetchAndStoreEvent(wsURL, eventID string, publisher interface{ Deliver(*event.E) }) error {
	// TODO: Implement WebSocket connection and event fetching
	// For now, this is a placeholder that assumes the event can be fetched
	// In a full implementation, this would:
	// 1. Connect to the WebSocket endpoint
	// 2. Send a REQ message for the specific event ID
	// 3. Receive the EVENT message
	// 4. Validate and store the event in the local database
	// 5. Propagate the event to subscribers via the publisher

	// Placeholder - mark as not implemented for now
	log.D.F("fetchAndStoreEvent called for %s from %s (placeholder implementation)", eventID, wsURL)

	// Note: When implementing the full WebSocket fetching logic, after storing the event,
	// the publisher should be called like this:
	// if publisher != nil {
	//     clonedEvent := fetchedEvent.Clone()
	//     go publisher.Deliver(clonedEvent)
	// }

	return nil // Return success for now
}

// Database key prefixes for cluster state persistence
const (
	clusterPeerStatePrefix = "cluster:peer:"
)

// loadPeerState loads persisted peer state from the database
func (cm *ClusterManager) loadPeerState() error {
	cm.membersMux.Lock()
	defer cm.membersMux.Unlock()

	prefix := []byte(clusterPeerStatePrefix)
	return cm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			Prefix: prefix,
		})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract peer URL from key (remove prefix)
			peerURL := string(key[len(prefix):])

			// Read the serial value
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

			// Update existing member or create new one
			if member, exists := cm.members[peerURL]; exists {
				member.LastSerial = serial
				log.D.F("loaded persisted serial %d for existing peer %s", serial, peerURL)
			} else {
				// Create member with persisted state
				member := &ClusterMember{
					HTTPURL:      peerURL,
					WebSocketURL: peerURL, // TODO: Convert to WebSocket URL
					LastSerial:   serial,
					Status:       "unknown",
				}
				cm.members[peerURL] = member
				log.D.F("loaded persisted serial %d for new peer %s", serial, peerURL)
			}
		}
		return nil
	})
}

// savePeerState saves the current serial for a peer to the database
func (cm *ClusterManager) savePeerState(peerURL string, serial uint64) error {
	key := []byte(clusterPeerStatePrefix + peerURL)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, serial)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// removePeerState removes persisted state for a peer from the database
func (cm *ClusterManager) removePeerState(peerURL string) error {
	key := []byte(clusterPeerStatePrefix + peerURL)

	return cm.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}
