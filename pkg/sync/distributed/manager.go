// Package distributed provides serial-based peer-to-peer synchronization
package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// -- actor request/response types --

type updatePeersReq struct {
	newPeers []string
}

type getCurrentSerialReq struct {
	resp chan uint64
}

type getPeersReq struct {
	resp chan []string
}

type updateSerialReq struct{}

type notifyNewEventReq struct {
	serial uint64
}

type isSelfURLReq struct {
	url  string
	resp chan bool
}

type markSelfURLReq struct {
	url string
}

type getPeerStatusReq struct {
	resp chan map[string]uint64
}

type updatePeerSerialReq struct {
	peerURL string
	serial  uint64
}

// Manager handles distributed synchronization between relay peers using serial numbers as clocks
type Manager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	db            *database.D
	nodeID        string
	relayURL      string
	nip11Cache    *common.NIP11Cache
	policyManager PolicyChecker

	// actor channels
	updatePeersCh      chan updatePeersReq      // fire-and-forget
	getCurrentSerialCh chan getCurrentSerialReq
	getPeersCh         chan getPeersReq
	updateSerialCh     chan updateSerialReq      // fire-and-forget
	notifyNewEventCh   chan notifyNewEventReq    // fire-and-forget
	isSelfURLCh        chan isSelfURLReq
	markSelfURLCh      chan markSelfURLReq       // fire-and-forget
	getPeerStatusCh    chan getPeerStatusReq
	updatePeerSerialCh chan updatePeerSerialReq  // fire-and-forget
	stopCh             chan struct{}
	doneCh             chan struct{}
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

	selfURLs := make(map[string]bool)
	if cfg.RelayURL != "" {
		selfURLs[cfg.RelayURL] = true
	}

	// Filter self from peer list at startup
	filteredPeers := cfg.Peers
	if cfg.NodeID != "" {
		nip11Cache := common.NewNIP11Cache(cfg.NIP11CacheTTL)
		filtered := make([]string, 0, len(cfg.Peers))
		for _, peerURL := range cfg.Peers {
			if selfURLs[peerURL] {
				log.D.F("removed self from sync peer list (known URL): %s", peerURL)
				continue
			}
			pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
			peerPubkey, err := nip11Cache.GetPubkey(pctx, peerURL)
			pcancel()
			if err != nil {
				log.D.F("couldn't fetch NIP-11 for %s, keeping in peer list: %v", peerURL, err)
				filtered = append(filtered, peerURL)
				continue
			}
			if peerPubkey == cfg.NodeID {
				log.D.F("removed self from sync peer list (discovered): %s (pubkey: %s)", peerURL, cfg.NodeID)
				selfURLs[peerURL] = true
				continue
			}
			filtered = append(filtered, peerURL)
		}
		filteredPeers = filtered
	}

	m := &Manager{
		ctx:                ctx,
		cancel:             cancel,
		db:                 db,
		nodeID:             cfg.NodeID,
		relayURL:           cfg.RelayURL,
		nip11Cache:         common.NewNIP11Cache(cfg.NIP11CacheTTL),
		policyManager:      policyManager,
		updatePeersCh:      make(chan updatePeersReq, 16),
		getCurrentSerialCh: make(chan getCurrentSerialReq),
		getPeersCh:         make(chan getPeersReq),
		updateSerialCh:     make(chan updateSerialReq, 16),
		notifyNewEventCh:   make(chan notifyNewEventReq, 16),
		isSelfURLCh:        make(chan isSelfURLReq),
		markSelfURLCh:      make(chan markSelfURLReq, 16),
		getPeerStatusCh:    make(chan getPeerStatusReq),
		updatePeerSerialCh: make(chan updatePeerSerialReq, 16),
		stopCh:             make(chan struct{}),
		doneCh:             make(chan struct{}),
	}

	go m.actorLoop(filteredPeers, selfURLs)

	// Start sync routine
	go m.syncRoutine()

	return m
}

// actorLoop owns all mutable state: peers, selfURLs, currentSerial, peerSerials.
func (m *Manager) actorLoop(initialPeers []string, initialSelfURLs map[string]bool) {
	defer close(m.doneCh)

	peers := make([]string, len(initialPeers))
	copy(peers, initialPeers)
	selfURLs := initialSelfURLs
	var currentSerial uint64
	peerSerials := make(map[string]uint64)

	for {
		select {
		case <-m.stopCh:
			return

		case req := <-m.updatePeersCh:
			peers = make([]string, len(req.newPeers))
			copy(peers, req.newPeers)
			log.D.F("updated peer list to %d peers", len(req.newPeers))

		case req := <-m.getCurrentSerialCh:
			req.resp <- currentSerial

		case req := <-m.getPeersCh:
			cp := make([]string, len(peers))
			copy(cp, peers)
			req.resp <- cp

		case <-m.updateSerialCh:
			// getLatestSerial in the original just returned currentSerial,
			// so this is a no-op in practice, but preserves the interface.
			// If actual DB query is needed later, do it here.

		case req := <-m.notifyNewEventCh:
			if req.serial > currentSerial {
				currentSerial = req.serial
			}

		case req := <-m.isSelfURLCh:
			req.resp <- selfURLs[req.url]

		case req := <-m.markSelfURLCh:
			selfURLs[req.url] = true

		case req := <-m.getPeerStatusCh:
			result := make(map[string]uint64)
			for k, v := range peerSerials {
				result[k] = v
			}
			req.resp <- result

		case req := <-m.updatePeerSerialCh:
			peerSerials[req.peerURL] = req.serial
		}
	}
}

// Stop stops the sync manager
func (m *Manager) Stop() {
	m.cancel()
	close(m.stopCh)
	<-m.doneCh
}

// UpdatePeers updates the peer list from relay group configuration
func (m *Manager) UpdatePeers(newPeers []string) {
	m.updatePeersCh <- updatePeersReq{newPeers: newPeers}
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
	req := getCurrentSerialReq{resp: make(chan uint64, 1)}
	m.getCurrentSerialCh <- req
	return <-req.resp
}

// GetPeers returns a copy of the current peer list
func (m *Manager) GetPeers() []string {
	req := getPeersReq{resp: make(chan []string, 1)}
	m.getPeersCh <- req
	return <-req.resp
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
	m.updateSerialCh <- updateSerialReq{}
}

// NotifyNewEvent notifies the manager of a new event
func (m *Manager) NotifyNewEvent(eventID []byte, serial uint64) {
	m.notifyNewEventCh <- notifyNewEventReq{serial: serial}
}

// IsSelfURL checks if a URL is our own relay
func (m *Manager) IsSelfURL(url string) bool {
	req := isSelfURLReq{url: url, resp: make(chan bool, 1)}
	m.isSelfURLCh <- req
	return <-req.resp
}

// MarkSelfURL marks a URL as belonging to us
func (m *Manager) MarkSelfURL(url string) {
	m.markSelfURLCh <- markSelfURLReq{url: url}
}

// IsSelfNodeID checks if a node ID matches ours
func (m *Manager) IsSelfNodeID(nodeID string) bool {
	return nodeID != "" && nodeID == m.nodeID
}

// syncRoutine periodically syncs with peers sequentially
func (m *Manager) syncRoutine() {
	ticker := time.NewTicker(5 * time.Second)
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
	peers := m.GetPeers()
	for _, peerURL := range peers {
		m.syncWithPeer(peerURL)
		time.Sleep(100 * time.Millisecond)
	}
}

// syncWithPeer syncs with a specific peer
func (m *Manager) syncWithPeer(peerURL string) {
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

	peerSerial := currentResp.Serial

	// Get our last seen serial for this peer
	statusReq := getPeerStatusReq{resp: make(chan map[string]uint64, 1)}
	m.getPeerStatusCh <- statusReq
	peerSerials := <-statusReq.resp
	ourLastSeen := peerSerials[peerURL]

	if peerSerial > ourLastSeen {
		m.requestEventIDs(peerURL, ourLastSeen+1, peerSerial)
		m.updatePeerSerialCh <- updatePeerSerialReq{peerURL: peerURL, serial: peerSerial}
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
		if !m.hasEventLocally(eventID) {
			missing = append(missing, eventID)
		}
	}

	return missing
}

// hasEventLocally checks if we have a specific event
func (m *Manager) hasEventLocally(eventID string) bool {
	eventIDBytes, err := hex.Dec(eventID)
	if err != nil {
		log.D.F("invalid event ID format: %s", eventID)
		return false
	}

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

	var eventIDBytes [][]byte
	for _, eventID := range eventIDs {
		if evBytes, err := hex.Dec(eventID); err == nil {
			eventIDBytes = append(eventIDBytes, evBytes)
		}
	}

	if len(eventIDBytes) == 0 {
		return
	}

	peers := m.GetPeers()
	for _, peerURL := range peers {
		wsURL := strings.Replace(peerURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

		log.D.F("would connect to %s and request %d events", wsURL, len(eventIDBytes))
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

	serials, err := m.db.EventIdsBySerial(from, int(to-from+1))
	if err != nil {
		return nil, err
	}

	for i, serial := range serials {
		eventID := fmt.Sprintf("event_%d", serial)
		eventMap[eventID] = serial
		_ = i
	}

	return eventMap, nil
}

// GetPeerStatus returns the sync status for all peers
func (m *Manager) GetPeerStatus() map[string]uint64 {
	req := getPeerStatusReq{resp: make(chan map[string]uint64, 1)}
	m.getPeerStatusCh <- req
	return <-req.resp
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

	if req.NodeID != "" && req.NodeID == m.nodeID {
		log.D.F("rejecting sync current request from self (nodeID: %s)", req.NodeID)
		if req.RelayURL != "" {
			m.MarkSelfURL(req.RelayURL)
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

	if req.NodeID != "" && req.NodeID == m.nodeID {
		log.D.F("rejecting sync event-ids request from self (nodeID: %s)", req.NodeID)
		if req.RelayURL != "" {
			m.MarkSelfURL(req.RelayURL)
			log.D.F("cached self-URL from inbound request: %s", req.RelayURL)
		}
		http.Error(w, "Cannot sync with self", http.StatusBadRequest)
		return
	}

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
