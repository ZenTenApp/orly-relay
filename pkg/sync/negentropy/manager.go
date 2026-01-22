// Package negentropy provides NIP-77 negentropy-based set reconciliation
// for both relay-to-relay sync and client-facing WebSocket operations.
package negentropy

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/negentropy"
	"next.orly.dev/pkg/database"
)

// PeerState represents the sync state for a peer relay.
type PeerState struct {
	URL                 string
	LastSync            time.Time
	EventsSynced        int64
	Status              string // "idle", "syncing", "error"
	LastError           string
	ConsecutiveFailures int32
}

// ClientSession represents an active client negentropy session.
type ClientSession struct {
	SubscriptionID string
	ConnectionID   string
	CreatedAt      time.Time
	LastActivity   time.Time
	RoundCount     int32
	neg            *negentropy.Negentropy
	storage        *negentropy.Vector
}

// SetNegentropy sets the negentropy instance and storage for this session.
func (s *ClientSession) SetNegentropy(neg *negentropy.Negentropy, storage *negentropy.Vector) {
	s.neg = neg
	s.storage = storage
}

// GetNegentropy returns the negentropy instance for this session.
func (s *ClientSession) GetNegentropy() *negentropy.Negentropy {
	return s.neg
}

// Config holds configuration for the negentropy manager.
type Config struct {
	Peers                []string
	SyncInterval         time.Duration
	FrameSize            int
	IDSize               int
	ClientSessionTimeout time.Duration
}

// Manager handles negentropy sync operations.
type Manager struct {
	db     database.Database
	config *Config

	mu         gosync.RWMutex
	peers      map[string]*PeerState
	sessions   map[string]*ClientSession // keyed by connectionID:subscriptionID
	active     bool
	lastSync   time.Time
	stopChan   chan struct{}
	syncWg     gosync.WaitGroup
}

// NewManager creates a new negentropy manager.
func NewManager(db database.Database, cfg *Config) *Manager {
	if cfg == nil {
		cfg = &Config{
			SyncInterval:         60 * time.Second,
			FrameSize:            128 * 1024,
			IDSize:               16,
			ClientSessionTimeout: 5 * time.Minute,
		}
	}

	m := &Manager{
		db:       db,
		config:   cfg,
		peers:    make(map[string]*PeerState),
		sessions: make(map[string]*ClientSession),
	}

	// Initialize peers from config
	for _, peerURL := range cfg.Peers {
		m.peers[peerURL] = &PeerState{
			URL:    peerURL,
			Status: "idle",
		}
	}

	return m
}

// Start starts the background sync loop.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.active {
		m.mu.Unlock()
		return
	}
	m.active = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	log.I.F("negentropy manager starting background sync")

	m.syncWg.Add(1)
	go m.syncLoop()
}

// Stop stops the background sync loop.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.active {
		m.mu.Unlock()
		return
	}
	m.active = false
	close(m.stopChan)
	m.mu.Unlock()

	m.syncWg.Wait()
	log.I.F("negentropy manager stopped")
}

func (m *Manager) syncLoop() {
	defer m.syncWg.Done()

	// Do initial sync after a short delay
	time.Sleep(5 * time.Second)
	m.syncAllPeers()

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.syncAllPeers()
		}
	}
}

func (m *Manager) syncAllPeers() {
	m.mu.RLock()
	peers := make([]string, 0, len(m.peers))
	for url := range m.peers {
		peers = append(peers, url)
	}
	m.mu.RUnlock()

	for _, peerURL := range peers {
		m.syncWithPeer(context.Background(), peerURL)
	}

	m.mu.Lock()
	m.lastSync = time.Now()
	m.mu.Unlock()
}

func (m *Manager) syncWithPeer(ctx context.Context, peerURL string) {
	m.mu.Lock()
	peer, ok := m.peers[peerURL]
	if !ok {
		m.mu.Unlock()
		return
	}
	peer.Status = "syncing"
	m.mu.Unlock()

	log.I.F("negentropy sync starting with %s", peerURL)

	eventsSynced, err := m.performNegentropy(ctx, peerURL)

	m.mu.Lock()
	peer.LastSync = time.Now()
	if err != nil {
		peer.Status = "error"
		peer.LastError = err.Error()
		peer.ConsecutiveFailures++
		log.E.F("negentropy sync with %s failed: %v", peerURL, err)
	} else {
		peer.Status = "idle"
		peer.LastError = ""
		peer.ConsecutiveFailures = 0
		peer.EventsSynced += eventsSynced
		log.I.F("negentropy sync with %s complete: %d events synced", peerURL, eventsSynced)
	}
	m.mu.Unlock()
}

// performNegentropy performs the actual NIP-77 negentropy sync with a peer.
func (m *Manager) performNegentropy(ctx context.Context, peerURL string) (int64, error) {
	// Build local storage from our events
	storage, err := m.buildStorage(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to build storage: %w", err)
	}

	log.I.F("built negentropy storage with %d events", storage.Size())

	// Create negentropy instance
	neg := negentropy.New(storage, m.config.FrameSize)
	defer neg.Close()

	// Connect to peer WebSocket
	wsURL := strings.Replace(peerURL, "wss://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "ws://", "ws://", 1)
	if !strings.HasPrefix(wsURL, "ws") {
		wsURL = "wss://" + wsURL
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, http.Header{})
	if err != nil {
		return 0, fmt.Errorf("failed to connect to peer: %w", err)
	}
	defer conn.Close()

	// Generate subscription ID
	subID := fmt.Sprintf("neg-%d", time.Now().UnixNano())

	// Start negentropy protocol
	initialMsg, err := neg.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start negentropy: %w", err)
	}

	// Send NEG-OPEN: ["NEG-OPEN", subscription_id, filter, initial_message]
	filter := map[string]any{} // Empty filter = all events
	negOpen := []any{"NEG-OPEN", subID, filter, hex.EncodeToString(initialMsg)}
	if err := conn.WriteJSON(negOpen); err != nil {
		return 0, fmt.Errorf("failed to send NEG-OPEN: %w", err)
	}

	var eventsSynced int64
	var needIDs []string
	var haveIDs []string

	// Exchange messages until complete
	for i := 0; i < 20; i++ { // Max 20 rounds
		// Read response
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return eventsSynced, fmt.Errorf("failed to read message: %w", err)
		}

		var msg []json.RawMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			return eventsSynced, fmt.Errorf("failed to parse message: %w", err)
		}

		if len(msg) < 2 {
			continue
		}

		var msgType string
		if err := json.Unmarshal(msg[0], &msgType); err != nil {
			continue
		}

		switch msgType {
		case "NEG-MSG":
			if len(msg) < 3 {
				continue
			}
			var hexMsg string
			if err := json.Unmarshal(msg[2], &hexMsg); err != nil {
				continue
			}

			negMsg, err := hex.DecodeString(hexMsg)
			if err != nil {
				continue
			}

			response, complete, err := neg.Reconcile(negMsg)
			if err != nil {
				return eventsSynced, fmt.Errorf("reconcile failed: %w", err)
			}

			// Collect IDs we need and IDs we have
			needIDs = append(needIDs, neg.CollectHaveNots()...)
			haveIDs = append(haveIDs, neg.CollectHaves()...)

			if complete {
				// Send NEG-CLOSE
				negClose := []any{"NEG-CLOSE", subID}
				conn.WriteJSON(negClose)
				goto done
			}

			// Send NEG-MSG response
			negMsgResp := []any{"NEG-MSG", subID, hex.EncodeToString(response)}
			if err := conn.WriteJSON(negMsgResp); err != nil {
				return eventsSynced, fmt.Errorf("failed to send NEG-MSG: %w", err)
			}

		case "NEG-ERR":
			var errMsg string
			if len(msg) >= 3 {
				json.Unmarshal(msg[2], &errMsg)
			}
			return eventsSynced, fmt.Errorf("peer returned error: %s", errMsg)

		case "EVENT":
			// Peer is sending us an event
			eventsSynced++
		}
	}

done:
	log.I.F("negentropy: need %d events, have %d events to send", len(needIDs), len(haveIDs))
	if len(needIDs) > 0 {
		log.I.F("negentropy: first few need IDs: %v", needIDs[:min(len(needIDs), 3)])
	}

	// PUSH events we have to the peer (haveIDs)
	// This is more reliable than trying to PULL events using ID prefixes
	if len(haveIDs) > 0 {
		pushed, err := m.pushEventsToPeer(ctx, conn, haveIDs)
		if err != nil {
			log.W.F("failed to push events to peer: %v", err)
		} else {
			log.I.F("negentropy: pushed %d events to peer", pushed)
			eventsSynced += int64(pushed)
		}
	}

	// NOTE: Events we need (needIDs) will be pushed to us by the peer's sync process
	// The peer runs the same negentropy sync and will identify these events as "haves"
	// to push to us. We don't need to explicitly fetch them.

	return eventsSynced, nil
}

// buildStorage creates a negentropy Vector from local events.
func (m *Manager) buildStorage(ctx context.Context) (*negentropy.Vector, error) {
	storage := negentropy.NewVector()

	// Query all events from database using an empty filter
	// This returns IdPkTs structs with Id, Pub, Ts, Ser fields
	limit := uint(100000)
	f := &filter.F{
		Limit: &limit, // Limit to 100k events for now
	}

	idPkTs, err := m.db.QueryForIds(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	for _, item := range idPkTs {
		// IDHex() returns lowercase hex string of the event ID
		storage.Insert(item.Ts, item.IDHex())
	}

	storage.Seal()
	return storage, nil
}

// pushEventsToPeer sends events we have to the peer.
// The truncated IDs are 32-char hex prefixes, so we query our local DB and push matching events.
func (m *Manager) pushEventsToPeer(ctx context.Context, conn *websocket.Conn, truncatedIDs []string) (int, error) {
	if len(truncatedIDs) == 0 {
		return 0, nil
	}
	log.I.F("pushEventsToPeer: looking up %d events to push", len(truncatedIDs))

	pushed := 0
	for _, truncID := range truncatedIDs {
		// Query local database for events matching this ID prefix
		// Use QueryByIDPrefix if available, otherwise fall back to broader query
		events, err := m.queryEventsByIDPrefix(ctx, truncID)
		if err != nil {
			log.D.F("failed to query event with prefix %s: %v", truncID, err)
			continue
		}

		for _, ev := range events {
			// Send event to peer
			eventMsg := []any{"EVENT", ev}
			if err := conn.WriteJSON(eventMsg); err != nil {
				log.W.F("failed to push event %s: %v", truncID, err)
				continue
			}
			pushed++
		}
	}

	return pushed, nil
}

// queryEventsByIDPrefix queries local database for events matching an ID prefix.
func (m *Manager) queryEventsByIDPrefix(ctx context.Context, idPrefix string) ([]*event.E, error) {
	// For now, query by the prefix - Badger supports prefix iteration
	// The ID prefix is 32 hex chars = 16 bytes
	limit := uint(10000) // Get enough events to find our prefix matches

	// Query IDs and filter by prefix
	f := &filter.F{
		Limit: &limit,
	}

	idPkTs, err := m.db.QueryForIds(ctx, f)
	if err != nil {
		return nil, err
	}

	var results []*event.E
	for _, item := range idPkTs {
		fullID := item.IDHex()
		if len(fullID) >= len(idPrefix) && fullID[:len(idPrefix)] == idPrefix {
			// Found a match - decode the full ID and fetch the event
			idBytes, err := hex.DecodeString(fullID)
			if err != nil {
				log.D.F("failed to decode ID %s: %v", fullID, err)
				continue
			}

			// Create filter with the full ID
			idTag := tag.NewFromBytesSlice(idBytes)
			evs, err := m.db.QueryEvents(ctx, &filter.F{
				Ids: idTag,
			})
			if err != nil {
				log.D.F("failed to fetch event %s: %v", fullID, err)
				continue
			}
			if len(evs) > 0 {
				results = append(results, evs[0])
			}
		}
	}

	return results, nil
}

// fetchEventsFromPeer fetches specific events from a peer by ID (can be prefixes).
// NOTE: This is deprecated in favor of push-based sync, but kept for reference.
func (m *Manager) fetchEventsFromPeer(ctx context.Context, conn *websocket.Conn, baseSubID string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	log.I.F("fetchEventsFromPeer: fetching %d events with IDs (first 3): %v", len(ids), ids[:min(3, len(ids))])

	// Batch IDs into chunks of 100
	const batchSize = 100
	fetched := 0

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		subID := fmt.Sprintf("%s-fetch-%d", baseSubID, i/batchSize)
		log.I.F("fetchEventsFromPeer: sending REQ %s for batch of %d IDs", subID, len(batch))

		// Send REQ for these IDs
		filter := map[string]any{
			"ids": batch,
		}
		req := []any{"REQ", subID, filter}
		reqJSON, _ := json.Marshal(req)
		log.D.F("fetchEventsFromPeer: REQ message: %s", string(reqJSON)[:min(500, len(reqJSON))])

		if err := conn.WriteJSON(req); err != nil {
			log.E.F("fetchEventsFromPeer: failed to send REQ: %v", err)
			return fetched, fmt.Errorf("failed to send REQ: %w", err)
		}

		// Read events until EOSE
		messageCount := 0
		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				log.E.F("fetchEventsFromPeer: failed to read after %d messages: %v", messageCount, err)
				return fetched, fmt.Errorf("failed to read: %w", err)
			}
			messageCount++

			var msg []json.RawMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				log.D.F("fetchEventsFromPeer: failed to unmarshal message: %v", err)
				continue
			}

			if len(msg) < 2 {
				log.D.F("fetchEventsFromPeer: message too short: %d elements", len(msg))
				continue
			}

			var msgType string
			if err := json.Unmarshal(msg[0], &msgType); err != nil {
				log.D.F("fetchEventsFromPeer: failed to unmarshal message type: %v", err)
				continue
			}

			switch msgType {
			case "EVENT":
				if len(msg) >= 3 {
					// Store the event
					if err := m.storeEventFromJSON(ctx, msg[2]); err != nil {
						log.W.F("fetchEventsFromPeer: failed to store event: %v", err)
					} else {
						fetched++
						if fetched%10 == 0 {
							log.I.F("fetchEventsFromPeer: stored %d events so far", fetched)
						}
					}
				}
			case "EOSE":
				log.I.F("fetchEventsFromPeer: received EOSE for %s after %d messages, fetched %d events in batch", subID, messageCount, fetched)
				goto nextBatch
			case "CLOSED":
				var reason string
				if len(msg) >= 3 {
					json.Unmarshal(msg[2], &reason)
				}
				log.W.F("fetchEventsFromPeer: subscription %s closed: %s", subID, reason)
				goto nextBatch
			case "NOTICE":
				var notice string
				if len(msg) >= 2 {
					json.Unmarshal(msg[1], &notice)
				}
				log.W.F("fetchEventsFromPeer: NOTICE from peer: %s", notice)
			default:
				log.D.F("fetchEventsFromPeer: unknown message type: %s", msgType)
			}
		}
	nextBatch:
		// Send CLOSE for this subscription
		closeMsg := []any{"CLOSE", subID}
		conn.WriteJSON(closeMsg)
	}

	log.I.F("fetchEventsFromPeer: completed, total fetched: %d", fetched)
	return fetched, nil
}

// storeEventFromJSON stores an event from raw JSON.
func (m *Manager) storeEventFromJSON(ctx context.Context, eventJSON json.RawMessage) error {
	// Parse the event using the nostr event encoder
	ev := &event.E{}
	if err := ev.UnmarshalJSON(eventJSON); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Verify the event signature
	if ok, err := ev.Verify(); err != nil || !ok {
		return fmt.Errorf("event verification failed")
	}

	// Store via database using the standard SaveEvent method
	_, err := m.db.SaveEvent(ctx, ev)
	return err
}

// IsActive returns whether background sync is running.
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// LastSync returns the timestamp of the last sync cycle.
func (m *Manager) LastSync() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSync
}

// GetPeers returns the list of peer URLs.
func (m *Manager) GetPeers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peers := make([]string, 0, len(m.peers))
	for url := range m.peers {
		peers = append(peers, url)
	}
	return peers
}

// GetPeerStates returns the sync state for all peers.
func (m *Manager) GetPeerStates() []*PeerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	states := make([]*PeerState, 0, len(m.peers))
	for _, peer := range m.peers {
		states = append(states, &PeerState{
			URL:                 peer.URL,
			LastSync:            peer.LastSync,
			EventsSynced:        peer.EventsSynced,
			Status:              peer.Status,
			LastError:           peer.LastError,
			ConsecutiveFailures: peer.ConsecutiveFailures,
		})
	}
	return states
}

// GetPeerState returns the sync state for a specific peer.
func (m *Manager) GetPeerState(peerURL string) (*PeerState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, ok := m.peers[peerURL]
	if !ok {
		return nil, false
	}
	return &PeerState{
		URL:                 peer.URL,
		LastSync:            peer.LastSync,
		EventsSynced:        peer.EventsSynced,
		Status:              peer.Status,
		LastError:           peer.LastError,
		ConsecutiveFailures: peer.ConsecutiveFailures,
	}, true
}

// AddPeer adds a peer for negentropy sync.
func (m *Manager) AddPeer(peerURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.peers[peerURL]; !ok {
		m.peers[peerURL] = &PeerState{
			URL:    peerURL,
			Status: "idle",
		}
	}
}

// RemovePeer removes a peer from negentropy sync.
func (m *Manager) RemovePeer(peerURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, peerURL)
}

// TriggerSync manually triggers sync with a specific peer or all peers.
func (m *Manager) TriggerSync(ctx context.Context, peerURL string) {
	if peerURL == "" {
		m.syncAllPeers()
	} else {
		m.syncWithPeer(ctx, peerURL)
	}
}

// sessionKey creates a unique key for a session.
func sessionKey(connectionID, subscriptionID string) string {
	return connectionID + ":" + subscriptionID
}

// OpenSession opens a new client negentropy session.
func (m *Manager) OpenSession(connectionID, subscriptionID string) *ClientSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := sessionKey(connectionID, subscriptionID)
	session := &ClientSession{
		SubscriptionID: subscriptionID,
		ConnectionID:   connectionID,
		CreatedAt:      time.Now(),
		LastActivity:   time.Now(),
		RoundCount:     0,
	}
	m.sessions[key] = session
	return session
}

// GetSession retrieves an existing session.
func (m *Manager) GetSession(connectionID, subscriptionID string) (*ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := sessionKey(connectionID, subscriptionID)
	session, ok := m.sessions[key]
	return session, ok
}

// UpdateSessionActivity updates the last activity time for a session.
func (m *Manager) UpdateSessionActivity(connectionID, subscriptionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey(connectionID, subscriptionID)
	if session, ok := m.sessions[key]; ok {
		session.LastActivity = time.Now()
		session.RoundCount++
	}
}

// CloseSession closes a client session.
func (m *Manager) CloseSession(connectionID, subscriptionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := sessionKey(connectionID, subscriptionID)
	if session, ok := m.sessions[key]; ok {
		if session.neg != nil {
			session.neg.Close()
		}
	}
	delete(m.sessions, key)
}

// CloseSessionsByConnection closes all sessions for a connection.
func (m *Manager) CloseSessionsByConnection(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session.ConnectionID == connectionID {
			if session.neg != nil {
				session.neg.Close()
			}
			delete(m.sessions, key)
		}
	}
}

// ListSessions returns all active sessions.
func (m *Manager) ListSessions() []*ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]*ClientSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, &ClientSession{
			SubscriptionID: session.SubscriptionID,
			ConnectionID:   session.ConnectionID,
			CreatedAt:      session.CreatedAt,
			LastActivity:   session.LastActivity,
			RoundCount:     session.RoundCount,
		})
	}
	return sessions
}

// CleanupExpiredSessions removes sessions that have been inactive beyond timeout.
func (m *Manager) CleanupExpiredSessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.ClientSessionTimeout)
	removed := 0
	for key, session := range m.sessions {
		if session.LastActivity.Before(cutoff) {
			if session.neg != nil {
				session.neg.Close()
			}
			delete(m.sessions, key)
			removed++
		}
	}
	return removed
}

// Ensure chk is used
var _ = chk.E
