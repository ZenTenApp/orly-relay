// Package distributed provides serial-based peer-to-peer synchronization
package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/sync/common"
)

// PolicyChecker is an interface for checking event policies
type PolicyChecker interface {
	CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
}

// RelayGroupConfigProvider provides relay group configuration
type RelayGroupConfigProvider interface {
	FindAuthoritativeConfig(ctx context.Context) ([]string, error)
}

// Manager handles distributed synchronization between relay peers using serial numbers as clocks
type Manager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	db            *database.D
	nodeID        string
	relayURL      string
	peers         []string
	selfURLs      map[string]bool   // URLs discovered to be ourselves (for fast lookups)
	currentSerial uint64
	peerSerials   map[string]uint64 // peer URL -> latest serial seen
	nip11Cache    *common.NIP11Cache
	policyManager PolicyChecker
	mutex         gosync.RWMutex
}

// CurrentRequest represents a request for the current serial number
type CurrentRequest struct {
	NodeID   string `json:"node_id"`
	RelayURL string `json:"relay_url"`
}

// CurrentResponse returns the current serial number
type CurrentResponse struct {
	NodeID   string `json:"node_id"`
	RelayURL string `json:"relay_url"`
	Serial   uint64 `json:"serial"`
}

// EventIDsRequest represents a request for event IDs with serials
type EventIDsRequest struct {
	NodeID   string `json:"node_id"`
	RelayURL string `json:"relay_url"`
	From     uint64 `json:"from"`
	To       uint64 `json:"to"`
}

// EventIDsResponse contains event IDs mapped to their serial numbers
type EventIDsResponse struct {
	EventMap map[string]uint64 `json:"event_map"` // event_id -> serial
}

// Config holds configuration for the distributed sync manager
type Config struct {
	NodeID        string
	RelayURL      string
	Peers         []string
	SyncInterval  time.Duration
	NIP11CacheTTL time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		SyncInterval:  5 * time.Second,
		NIP11CacheTTL: 30 * time.Minute,
	}
}

// NewManager creates a new sync manager
func NewManager(ctx context.Context, db *database.D, cfg *Config, policyManager PolicyChecker) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	if cfg == nil {
		cfg = DefaultConfig()
	}

	m := &Manager{
		ctx:           ctx,
		cancel:        cancel,
		db:            db,
		nodeID:        cfg.NodeID,
		relayURL:      cfg.RelayURL,
		peers:         cfg.Peers,
		selfURLs:      make(map[string]bool),
		currentSerial: 0,
		peerSerials:   make(map[string]uint64),
		nip11Cache:    common.NewNIP11Cache(cfg.NIP11CacheTTL),
		policyManager: policyManager,
	}

	// Add our configured relay URL to self-URLs cache if provided
	if m.relayURL != "" {
		m.selfURLs[m.relayURL] = true
	}

	// Remove self from peer list once at startup if we have a nodeID
	if m.nodeID != "" {
		filteredPeers := make([]string, 0, len(m.peers))
		for _, peerURL := range m.peers {
			// Fast path: check if we already know this URL is ours
			if m.selfURLs[peerURL] {
				log.D.F("removed self from sync peer list (known URL): %s", peerURL)
				continue
			}

			// Slow path: check via NIP-11 pubkey
			pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
			peerPubkey, err := m.nip11Cache.GetPubkey(pctx, peerURL)
			pcancel()

			if err != nil {
				log.D.F("couldn't fetch NIP-11 for %s, keeping in peer list: %v", peerURL, err)
				filteredPeers = append(filteredPeers, peerURL)
				continue
			}

			if peerPubkey == m.nodeID {
				log.D.F("removed self from sync peer list (discovered): %s (pubkey: %s)", peerURL, m.nodeID)
				// Cache this URL as ours for future fast lookups
				m.selfURLs[peerURL] = true
				continue
			}

			filteredPeers = append(filteredPeers, peerURL)
		}
		m.peers = filteredPeers
	}

	// Start sync routine
	go m.syncRoutine()

	return m
}

// Stop stops the sync manager
func (m *Manager) Stop() {
	m.cancel()
}

// UpdatePeers updates the peer list from relay group configuration
func (m *Manager) UpdatePeers(newPeers []string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.peers = newPeers
	log.D.F("updated peer list to %d peers", len(newPeers))
}

// IsAuthorizedPeer checks if a peer is authorized by validating its NIP-11 pubkey
func (m *Manager) IsAuthorizedPeer(peerURL string, expectedPubkey string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	peerPubkey, err := m.nip11Cache.GetPubkey(ctx, peerURL)
	if err != nil {
		log.D.F("failed to fetch NIP-11 pubkey for %s: %v", peerURL, err)
		return false
	}

	return peerPubkey == expectedPubkey
}

// GetPeerPubkey fetches and caches the pubkey for a peer relay
func (m *Manager) GetPeerPubkey(peerURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return m.nip11Cache.GetPubkey(ctx, peerURL)
}

// GetCurrentSerial returns the current serial number
func (m *Manager) GetCurrentSerial() uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentSerial
}

// GetPeers returns a copy of the current peer list
func (m *Manager) GetPeers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	peers := make([]string, len(m.peers))
	copy(peers, m.peers)
	return peers
}

// GetNodeID returns the node's identity
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// GetRelayURL returns the relay's URL
func (m *Manager) GetRelayURL() string {
	return m.relayURL
}

// UpdateSerial updates the current serial number when a new event is stored
func (m *Manager) UpdateSerial() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the latest serial from database
	if latest, err := m.getLatestSerial(); err == nil {
		m.currentSerial = latest
	}
}

// NotifyNewEvent notifies the manager of a new event
func (m *Manager) NotifyNewEvent(eventID []byte, serial uint64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if serial > m.currentSerial {
		m.currentSerial = serial
	}
}

// IsSelfURL checks if a URL is our own relay
func (m *Manager) IsSelfURL(url string) bool {
	m.mutex.RLock()
	if m.selfURLs[url] {
		m.mutex.RUnlock()
		return true
	}
	m.mutex.RUnlock()
	return false
}

// MarkSelfURL marks a URL as belonging to us
func (m *Manager) MarkSelfURL(url string) {
	m.mutex.Lock()
	m.selfURLs[url] = true
	m.mutex.Unlock()
}

// IsSelfNodeID checks if a node ID matches ours
func (m *Manager) IsSelfNodeID(nodeID string) bool {
	return nodeID != "" && nodeID == m.nodeID
}

// getLatestSerial gets the latest serial number from the database
func (m *Manager) getLatestSerial() (uint64, error) {
	// This is a simplified implementation
	// In practice, you'd want to track the highest serial number
	// For now, return the current serial
	return m.currentSerial, nil
}

// syncRoutine periodically syncs with peers sequentially
func (m *Manager) syncRoutine() {
	ticker := time.NewTicker(5 * time.Second) // Sync every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.syncWithPeersSequentially()
		}
	}
}

// syncWithPeersSequentially syncs with all configured peers one at a time
func (m *Manager) syncWithPeersSequentially() {
	for _, peerURL := range m.peers {
		// Self-peers are already filtered out during initialization/update
		m.syncWithPeer(peerURL)
		// Small delay between peers to avoid overwhelming
		time.Sleep(100 * time.Millisecond)
	}
}

// syncWithPeer syncs with a specific peer
func (m *Manager) syncWithPeer(peerURL string) {
	// Get the peer's current serial
	currentReq := CurrentRequest{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
	}

	jsonData, err := json.Marshal(currentReq)
	if err != nil {
		log.E.F("failed to marshal current request: %v", err)
		return
	}

	resp, err := http.Post(peerURL+"/api/sync/current", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.D.F("failed to get current serial from %s: %v", peerURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.D.F("current request failed with %s: status %d", peerURL, resp.StatusCode)
		return
	}

	var currentResp CurrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&currentResp); err != nil {
		log.E.F("failed to decode current response from %s: %v", peerURL, err)
		return
	}

	// Check if we need to sync
	peerSerial := currentResp.Serial
	ourLastSeen := m.peerSerials[peerURL]

	if peerSerial > ourLastSeen {
		// Request event IDs for the missing range
		m.requestEventIDs(peerURL, ourLastSeen+1, peerSerial)
		// Update our knowledge of peer's serial
		m.mutex.Lock()
		m.peerSerials[peerURL] = peerSerial
		m.mutex.Unlock()
	}
}

// requestEventIDs requests event IDs for a serial range from a peer
func (m *Manager) requestEventIDs(peerURL string, from, to uint64) {
	req := EventIDsRequest{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
		From:     from,
		To:       to,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		log.E.F("failed to marshal event-ids request: %v", err)
		return
	}

	resp, err := http.Post(peerURL+"/api/sync/event-ids", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.E.F("failed to request event IDs from %s: %v", peerURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.E.F("event-ids request failed with %s: status %d", peerURL, resp.StatusCode)
		return
	}

	var eventIDsResp EventIDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventIDsResp); err != nil {
		log.E.F("failed to decode event-ids response from %s: %v", peerURL, err)
		return
	}

	// Check which events we don't have and request them via websocket
	missingEventIDs := m.findMissingEventIDs(eventIDsResp.EventMap)
	if len(missingEventIDs) > 0 {
		m.requestEventsViaWebsocket(missingEventIDs)
		log.D.F("requested %d missing events from peer %s", len(missingEventIDs), peerURL)
	}
}

// findMissingEventIDs checks which event IDs we don't have locally
func (m *Manager) findMissingEventIDs(eventMap map[string]uint64) []string {
	var missing []string

	for eventID := range eventMap {
		// Check if we have this event locally
		// This is a simplified check - in practice you'd query the database
		if !m.hasEventLocally(eventID) {
			missing = append(missing, eventID)
		}
	}

	return missing
}

// hasEventLocally checks if we have a specific event
func (m *Manager) hasEventLocally(eventID string) bool {
	// Convert hex event ID to bytes
	eventIDBytes, err := hex.Dec(eventID)
	if err != nil {
		log.D.F("invalid event ID format: %s", eventID)
		return false
	}

	// Query for the event
	f := &filter.F{
		Ids: tag.NewFromBytesSlice(eventIDBytes),
	}

	events, err := m.db.QueryEvents(context.Background(), f)
	if err != nil {
		log.D.F("error querying for event %s: %v", eventID, err)
		return false
	}

	return len(events) > 0
}

// requestEventsViaWebsocket requests specific events via websocket from peers
func (m *Manager) requestEventsViaWebsocket(eventIDs []string) {
	if len(eventIDs) == 0 {
		return
	}

	// Convert hex event IDs to bytes for websocket requests
	var eventIDBytes [][]byte
	for _, eventID := range eventIDs {
		if evBytes, err := hex.Dec(eventID); err == nil {
			eventIDBytes = append(eventIDBytes, evBytes)
		}
	}

	if len(eventIDBytes) == 0 {
		return
	}

	// TODO: Implement websocket connection and REQ message sending
	// For now, try to request from our peers via their websocket endpoints
	for _, peerURL := range m.peers {
		// Convert HTTP URL to WebSocket URL
		wsURL := strings.Replace(peerURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

		log.D.F("would connect to %s and request %d events", wsURL, len(eventIDBytes))
		// Here we would:
		// 1. Establish websocket connection to peer
		// 2. Send NIP-98 auth if required
		// 3. Send REQ message with the filter for specific event IDs
		// 4. Receive and process EVENT messages
		// 5. Import received events
	}

	limit := 5
	if len(eventIDs) < limit {
		limit = len(eventIDs)
	}
	log.D.F("requested %d events via websocket: %v", len(eventIDs), eventIDs[:limit])
}

// GetEventsWithIDs retrieves events with their IDs by serial range
func (m *Manager) GetEventsWithIDs(from, to uint64) (map[string]uint64, error) {
	eventMap := make(map[string]uint64)

	// Get event serials by serial range
	serials, err := m.db.EventIdsBySerial(from, int(to-from+1))
	if err != nil {
		return nil, err
	}

	// For each serial, we need to map it to an event ID
	// This is a simplified implementation - in practice we'd need to query events by serial
	for i, serial := range serials {
		// TODO: Implement actual event ID retrieval by serial
		// For now, create placeholder event IDs based on serial
		eventID := fmt.Sprintf("event_%d", serial)
		eventMap[eventID] = serial
		_ = i // avoid unused variable warning
	}

	return eventMap, nil
}

// GetPeerStatus returns the sync status for all peers
func (m *Manager) GetPeerStatus() map[string]uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make(map[string]uint64)
	for k, v := range m.peerSerials {
		result[k] = v
	}
	return result
}

// HandleCurrentRequest handles requests for current serial number
func (m *Manager) HandleCurrentRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CurrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Reject requests from ourselves (same nodeID)
	if req.NodeID != "" && req.NodeID == m.nodeID {
		log.D.F("rejecting sync current request from self (nodeID: %s)", req.NodeID)
		// Cache the requesting relay URL as ours for future fast lookups
		if req.RelayURL != "" {
			m.mutex.Lock()
			m.selfURLs[req.RelayURL] = true
			m.mutex.Unlock()
			log.D.F("cached self-URL from inbound request: %s", req.RelayURL)
		}
		http.Error(w, "Cannot sync with self", http.StatusBadRequest)
		return
	}

	resp := CurrentResponse{
		NodeID:   m.nodeID,
		RelayURL: m.relayURL,
		Serial:   m.GetCurrentSerial(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleEventIDsRequest handles requests for event IDs with their serial numbers
func (m *Manager) HandleEventIDsRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EventIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Reject requests from ourselves (same nodeID)
	if req.NodeID != "" && req.NodeID == m.nodeID {
		log.D.F("rejecting sync event-ids request from self (nodeID: %s)", req.NodeID)
		// Cache the requesting relay URL as ours for future fast lookups
		if req.RelayURL != "" {
			m.mutex.Lock()
			m.selfURLs[req.RelayURL] = true
			m.mutex.Unlock()
			log.D.F("cached self-URL from inbound request: %s", req.RelayURL)
		}
		http.Error(w, "Cannot sync with self", http.StatusBadRequest)
		return
	}

	// Get events with IDs in the requested range
	eventMap, err := m.GetEventsWithIDs(req.From, req.To)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get event IDs: %v", err), http.StatusInternalServerError)
		return
	}

	resp := EventIDsResponse{
		EventMap: eventMap,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
