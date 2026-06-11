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
	"time"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/negentropy"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/ratelimit"
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
	Filter               *filter.F // Optional filter for selective sync
	MaxEvents            uint      // Max events to sync per cycle (0 = unlimited)
	MemoryTargetMB       int       // Memory target for backpressure (0 = disabled)
}

// -- actor request/response types --

type startReq struct {
	resp chan struct{} // signals done
}

type stopReq struct {
	resp chan struct{} // signals done
}

type isActiveReq struct {
	resp chan bool
}

type lastSyncReq struct {
	resp chan time.Time
}

type getPeersReq struct {
	resp chan []string
}

type getPeerStatesReq struct {
	resp chan []*PeerState
}

type getPeerStateReq struct {
	peerURL string
	resp    chan getPeerStateResp
}
type getPeerStateResp struct {
	state *PeerState
	ok    bool
}

type addPeerReq struct {
	peerURL string
}

type removePeerReq struct {
	peerURL string
}

type syncAllPeersReq struct {
	resp chan []string // returns snapshot of peer URLs
}

type updatePeerAfterSyncReq struct {
	peerURL      string
	lastSync     time.Time
	status       string
	lastError    string
	consFailures int32
	eventsDelta  int64
}

type updateLastSyncReq struct {
	t time.Time
}

type openSessionReq struct {
	connectionID   string
	subscriptionID string
	resp           chan *ClientSession
}

type getSessionReq struct {
	connectionID   string
	subscriptionID string
	resp           chan getSessionResp
}
type getSessionResp struct {
	session *ClientSession
	ok      bool
}

type updateSessionActivityReq struct {
	connectionID   string
	subscriptionID string
}

type closeSessionReq struct {
	connectionID   string
	subscriptionID string
}

type closeSessionsByConnReq struct {
	connectionID string
}

type listSessionsReq struct {
	resp chan []*ClientSession
}

type cleanupExpiredSessionsReq struct {
	resp chan int
}

// Manager handles negentropy sync operations.
type Manager struct {
	db     database.Database
	config *Config

	memoryMonitor *ratelimit.MemoryMonitor // nil if backpressure disabled

	// actor channels
	startCh                    chan startReq
	stopCh                     chan stopReq
	isActiveCh                 chan isActiveReq
	lastSyncCh                 chan lastSyncReq
	getPeersCh                 chan getPeersReq
	getPeerStatesCh            chan getPeerStatesReq
	getPeerStateCh             chan getPeerStateReq
	addPeerCh                  chan addPeerReq                  // fire-and-forget
	removePeerCh               chan removePeerReq               // fire-and-forget
	syncAllPeersCh             chan syncAllPeersReq
	updatePeerAfterSyncCh      chan updatePeerAfterSyncReq      // fire-and-forget
	updateLastSyncCh           chan updateLastSyncReq           // fire-and-forget
	openSessionCh              chan openSessionReq
	getSessionCh               chan getSessionReq
	updateSessionActivityCh    chan updateSessionActivityReq    // fire-and-forget
	closeSessionCh             chan closeSessionReq             // fire-and-forget
	closeSessionsByConnCh      chan closeSessionsByConnReq      // fire-and-forget
	listSessionsCh             chan listSessionsReq
	cleanupExpiredSessionsCh   chan cleanupExpiredSessionsReq
	actorDoneCh                chan struct{}
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
		db:     db,
		config: cfg,
		startCh:                  make(chan startReq),
		stopCh:                   make(chan stopReq),
		isActiveCh:               make(chan isActiveReq),
		lastSyncCh:               make(chan lastSyncReq),
		getPeersCh:               make(chan getPeersReq),
		getPeerStatesCh:          make(chan getPeerStatesReq),
		getPeerStateCh:           make(chan getPeerStateReq),
		addPeerCh:                make(chan addPeerReq, 16),
		removePeerCh:             make(chan removePeerReq, 16),
		syncAllPeersCh:           make(chan syncAllPeersReq),
		updatePeerAfterSyncCh:    make(chan updatePeerAfterSyncReq, 16),
		updateLastSyncCh:         make(chan updateLastSyncReq, 16),
		openSessionCh:            make(chan openSessionReq),
		getSessionCh:             make(chan getSessionReq),
		updateSessionActivityCh:  make(chan updateSessionActivityReq, 16),
		closeSessionCh:           make(chan closeSessionReq, 16),
		closeSessionsByConnCh:    make(chan closeSessionsByConnReq, 16),
		listSessionsCh:           make(chan listSessionsReq),
		cleanupExpiredSessionsCh: make(chan cleanupExpiredSessionsReq),
		actorDoneCh:              make(chan struct{}),
	}

	// Initialize memory monitor for backpressure if configured
	if cfg.MemoryTargetMB > 0 {
		m.memoryMonitor = ratelimit.NewMemoryMonitor(500 * time.Millisecond)
		m.memoryMonitor.SetMemoryTarget(uint64(cfg.MemoryTargetMB) * 1024 * 1024)
		m.memoryMonitor.Start()
		log.I.F("negentropy: backpressure enabled (target %dMB)", cfg.MemoryTargetMB)
	}

	go m.actorLoop(cfg.Peers)

	return m
}

// actorLoop owns peers, sessions, active, lastSync, stopChan.
func (m *Manager) actorLoop(initialPeers []string) {
	defer close(m.actorDoneCh)

	peers := make(map[string]*PeerState)
	for _, peerURL := range initialPeers {
		peers[peerURL] = &PeerState{
			URL:    peerURL,
			Status: "idle",
		}
	}

	sessions := make(map[string]*ClientSession)
	var active bool
	var lastSync time.Time
	var syncLoopDone chan struct{} // non-nil when sync loop is running

	for {
		select {
		case req := <-m.startCh:
			if !active {
				active = true
				syncLoopDone = make(chan struct{})
				log.I.F("negentropy manager starting background sync")
				go m.syncLoop(syncLoopDone)
			}
			req.resp <- struct{}{}

		case req := <-m.stopCh:
			if active {
				active = false
				// syncLoop detects inactive via IsActive() and exits
			}
			// Wait for sync loop to finish if it was running
			if syncLoopDone != nil {
				<-syncLoopDone
				syncLoopDone = nil
			}
			if m.memoryMonitor != nil {
				m.memoryMonitor.Stop()
			}
			log.I.F("negentropy manager stopped")
			req.resp <- struct{}{}

		case req := <-m.isActiveCh:
			req.resp <- active

		case req := <-m.lastSyncCh:
			req.resp <- lastSync

		case req := <-m.getPeersCh:
			result := make([]string, 0, len(peers))
			for url := range peers {
				result = append(result, url)
			}
			req.resp <- result

		case req := <-m.getPeerStatesCh:
			states := make([]*PeerState, 0, len(peers))
			for _, peer := range peers {
				states = append(states, &PeerState{
					URL:                 peer.URL,
					LastSync:            peer.LastSync,
					EventsSynced:        peer.EventsSynced,
					Status:              peer.Status,
					LastError:           peer.LastError,
					ConsecutiveFailures: peer.ConsecutiveFailures,
				})
			}
			req.resp <- states

		case req := <-m.getPeerStateCh:
			peer, ok := peers[req.peerURL]
			if !ok {
				req.resp <- getPeerStateResp{nil, false}
			} else {
				req.resp <- getPeerStateResp{
					state: &PeerState{
						URL:                 peer.URL,
						LastSync:            peer.LastSync,
						EventsSynced:        peer.EventsSynced,
						Status:              peer.Status,
						LastError:           peer.LastError,
						ConsecutiveFailures: peer.ConsecutiveFailures,
					},
					ok: true,
				}
			}

		case req := <-m.addPeerCh:
			if _, ok := peers[req.peerURL]; !ok {
				peers[req.peerURL] = &PeerState{
					URL:    req.peerURL,
					Status: "idle",
				}
			}

		case req := <-m.removePeerCh:
			delete(peers, req.peerURL)

		case req := <-m.syncAllPeersCh:
			// Return snapshot of URLs and mark peers as syncing
			urls := make([]string, 0, len(peers))
			for url := range peers {
				urls = append(urls, url)
			}
			req.resp <- urls

		case req := <-m.updatePeerAfterSyncCh:
			if peer, ok := peers[req.peerURL]; ok {
				peer.LastSync = req.lastSync
				peer.Status = req.status
				peer.LastError = req.lastError
				peer.ConsecutiveFailures = req.consFailures
				peer.EventsSynced += req.eventsDelta
			}

		case req := <-m.updateLastSyncCh:
			lastSync = req.t

		case req := <-m.openSessionCh:
			key := sessionKey(req.connectionID, req.subscriptionID)
			session := &ClientSession{
				SubscriptionID: req.subscriptionID,
				ConnectionID:   req.connectionID,
				CreatedAt:      time.Now(),
				LastActivity:   time.Now(),
				RoundCount:     0,
			}
			sessions[key] = session
			req.resp <- session

		case req := <-m.getSessionCh:
			key := sessionKey(req.connectionID, req.subscriptionID)
			session, ok := sessions[key]
			req.resp <- getSessionResp{session: session, ok: ok}

		case req := <-m.updateSessionActivityCh:
			key := sessionKey(req.connectionID, req.subscriptionID)
			if session, ok := sessions[key]; ok {
				session.LastActivity = time.Now()
				session.RoundCount++
			}

		case req := <-m.closeSessionCh:
			key := sessionKey(req.connectionID, req.subscriptionID)
			if session, ok := sessions[key]; ok {
				if session.neg != nil {
					session.neg.Close()
				}
			}
			delete(sessions, key)

		case req := <-m.closeSessionsByConnCh:
			for key, session := range sessions {
				if session.ConnectionID == req.connectionID {
					if session.neg != nil {
						session.neg.Close()
					}
					delete(sessions, key)
				}
			}

		case req := <-m.listSessionsCh:
			result := make([]*ClientSession, 0, len(sessions))
			for _, session := range sessions {
				result = append(result, &ClientSession{
					SubscriptionID: session.SubscriptionID,
					ConnectionID:   session.ConnectionID,
					CreatedAt:      session.CreatedAt,
					LastActivity:   session.LastActivity,
					RoundCount:     session.RoundCount,
				})
			}
			req.resp <- result

		case req := <-m.cleanupExpiredSessionsCh:
			cutoff := time.Now().Add(-m.config.ClientSessionTimeout)
			removed := 0
			for key, session := range sessions {
				if session.LastActivity.Before(cutoff) {
					if session.neg != nil {
						session.neg.Close()
					}
					delete(sessions, key)
					removed++
				}
			}
			req.resp <- removed
		}
	}
}

// Start starts the background sync loop.
func (m *Manager) Start() {
	req := startReq{resp: make(chan struct{}, 1)}
	m.startCh <- req
	<-req.resp
}

// Stop stops the background sync loop.
func (m *Manager) Stop() {
	req := stopReq{resp: make(chan struct{}, 1)}
	m.stopCh <- req
	<-req.resp
}

// checkBackpressure applies progressive delays when memory pressure is high.
func (m *Manager) checkBackpressure(ctx context.Context) error {
	if m.memoryMonitor == nil {
		return nil
	}
	metrics := m.memoryMonitor.GetMetrics()

	if metrics.InEmergencyMode {
		log.W.F("negentropy: pausing sync - emergency mode (memory pressure %.1f%%)",
			metrics.MemoryPressure*100)
		select {
		case <-time.After(10 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if metrics.MemoryPressure > 0.7 {
		fraction := (metrics.MemoryPressure - 0.7) / 0.3
		if fraction > 1.0 {
			fraction = 1.0
		}
		delay := time.Duration(fraction*5000) * time.Millisecond
		log.D.F("negentropy: backpressure %v (memory pressure %.1f%%)",
			delay, metrics.MemoryPressure*100)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (m *Manager) syncLoop(done chan struct{}) {
	defer close(done)

	// Do initial sync after a short delay
	time.Sleep(5 * time.Second)

	// Check if still active before initial sync
	if !m.IsActive() {
		return
	}
	m.syncAllPeers()

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		if !m.IsActive() {
			return
		}
		select {
		case <-ticker.C:
			if !m.IsActive() {
				return
			}
			m.syncAllPeers()
		}
	}
}

func (m *Manager) syncAllPeers() {
	req := syncAllPeersReq{resp: make(chan []string, 1)}
	m.syncAllPeersCh <- req
	peerURLs := <-req.resp

	for _, peerURL := range peerURLs {
		m.syncWithPeer(context.Background(), peerURL)
	}

	m.updateLastSyncCh <- updateLastSyncReq{t: time.Now()}
}

func (m *Manager) syncWithPeer(ctx context.Context, peerURL string) {
	// Mark peer as syncing
	m.updatePeerAfterSyncCh <- updatePeerAfterSyncReq{
		peerURL:  peerURL,
		lastSync: time.Time{}, // don't update yet
		status:   "syncing",
	}

	log.D.F("negentropy sync starting with %s", peerURL)

	eventsSynced, err := m.performNegentropy(ctx, peerURL)

	now := time.Now()
	if err != nil {
		// Get current consecutive failures to increment
		stateReq := getPeerStateReq{peerURL: peerURL, resp: make(chan getPeerStateResp, 1)}
		m.getPeerStateCh <- stateReq
		stateResp := <-stateReq.resp
		var consFailures int32
		if stateResp.ok {
			consFailures = stateResp.state.ConsecutiveFailures
		}
		m.updatePeerAfterSyncCh <- updatePeerAfterSyncReq{
			peerURL:      peerURL,
			lastSync:     now,
			status:       "error",
			lastError:    err.Error(),
			consFailures: consFailures + 1,
		}
		log.E.F("negentropy sync with %s failed: %v", peerURL, err)
	} else {
		m.updatePeerAfterSyncCh <- updatePeerAfterSyncReq{
			peerURL:      peerURL,
			lastSync:     now,
			status:       "idle",
			lastError:    "",
			consFailures: 0,
			eventsDelta:  eventsSynced,
		}
		log.D.F("negentropy sync with %s complete: %d events synced", peerURL, eventsSynced)
	}
}

// performNegentropy performs the actual NIP-77 negentropy sync with a peer.
func (m *Manager) performNegentropy(ctx context.Context, peerURL string) (int64, error) {
	// Build local storage from our events
	storage, err := m.buildStorage(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to build storage: %w", err)
	}

	log.D.F("built negentropy storage with %d events", storage.Size())

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
	negFilter := m.filterToMap()
	negOpen := []any{"NEG-OPEN", subID, negFilter, hex.EncodeToString(initialMsg)}
	if err := conn.WriteJSON(negOpen); err != nil {
		return 0, fmt.Errorf("failed to send NEG-OPEN: %w", err)
	}

	var eventsSynced int64
	var needIDs []string
	var haveIDs []string

	// Phase 1: Reconciliation - exchange NEG-MSG until complete
	for i := 0; i < 20; i++ {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return eventsSynced, fmt.Errorf("failed to read message during reconciliation: %w", err)
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

			needIDs = append(needIDs, neg.CollectHaveNots()...)
			haveIDs = append(haveIDs, neg.CollectHaves()...)

			if len(response) > 0 {
				negMsgResp := []any{"NEG-MSG", subID, hex.EncodeToString(response)}
				if err := conn.WriteJSON(negMsgResp); err != nil {
					return eventsSynced, fmt.Errorf("failed to send NEG-MSG: %w", err)
				}
			}

			if complete {
				log.D.F("negentropy: reconciliation complete, need %d events, have %d to push", len(needIDs), len(haveIDs))
				goto fetchAndPush
			}

		case "NEG-ERR":
			var errMsg string
			if len(msg) >= 3 {
				json.Unmarshal(msg[2], &errMsg)
			}
			return eventsSynced, fmt.Errorf("peer returned error: %s", errMsg)
		}
	}

fetchAndPush:
	// Send NEG-CLOSE to end the negentropy session
	{
		negClose := []any{"NEG-CLOSE", subID}
		conn.WriteJSON(negClose)
	}
	conn.SetReadDeadline(time.Time{})

	log.D.F("negentropy: need %d events, have %d events to send", len(needIDs), len(haveIDs))

	if len(needIDs) > 0 {
		fetched, err := m.fetchEventsFromPeer(ctx, conn, subID, needIDs)
		if err != nil {
			log.W.F("negentropy: failed to fetch events: %v", err)
		} else {
			log.D.F("negentropy: fetched %d events from peer", fetched)
			eventsSynced += int64(fetched)
		}
	}

	if len(haveIDs) > 0 {
		pushed, err := m.pushEventsToPeer(ctx, conn, haveIDs)
		if err != nil {
			log.W.F("failed to push events to peer: %v", err)
		} else {
			log.D.F("negentropy: pushed %d events to peer", pushed)
			eventsSynced += int64(pushed)
		}
	}

	return eventsSynced, nil
}

// buildStorage creates a negentropy Vector from local events.
func (m *Manager) buildStorage(ctx context.Context) (*negentropy.Vector, error) {
	storage := negentropy.NewVector()

	limit := m.config.MaxEvents
	if limit == 0 {
		limit = 1000000
	}
	var f *filter.F
	if m.config.Filter != nil {
		f = m.config.Filter
		f.Limit = &limit
	} else {
		f = &filter.F{
			Limit: &limit,
		}
	}

	idPkTs, err := m.db.QueryForIds(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	for _, item := range idPkTs {
		storage.Insert(item.Ts, item.IDHex())
	}

	storage.Seal()
	return storage, nil
}

// filterToMap converts the configured filter to a map for NEG-OPEN message.
func (m *Manager) filterToMap() map[string]any {
	result := map[string]any{}

	if m.config.Filter == nil {
		return result
	}

	f := m.config.Filter

	if f.Kinds != nil && f.Kinds.Len() > 0 {
		kinds := make([]int, 0, f.Kinds.Len())
		for _, k := range f.Kinds.K {
			kinds = append(kinds, k.ToInt())
		}
		result["kinds"] = kinds
	}

	if f.Authors != nil && f.Authors.Len() > 0 {
		authors := make([]string, 0, f.Authors.Len())
		for _, a := range f.Authors.T {
			authors = append(authors, hex.EncodeToString(a))
		}
		result["authors"] = authors
	}

	if f.Ids != nil && f.Ids.Len() > 0 {
		ids := make([]string, 0, f.Ids.Len())
		for _, id := range f.Ids.T {
			ids = append(ids, hex.EncodeToString(id))
		}
		result["ids"] = ids
	}

	if f.Since != nil && f.Since.V != 0 {
		result["since"] = f.Since.V
	}

	if f.Until != nil && f.Until.V != 0 {
		result["until"] = f.Until.V
	}

	if f.Limit != nil && *f.Limit > 0 {
		result["limit"] = *f.Limit
	}

	return result
}

// pushEventsToPeer sends events we have to the peer.
func (m *Manager) pushEventsToPeer(ctx context.Context, conn *websocket.Conn, truncatedIDs []string) (int, error) {
	if len(truncatedIDs) == 0 {
		return 0, nil
	}
	log.D.F("pushEventsToPeer: looking up %d events to push", len(truncatedIDs))

	pushed := 0
	for _, truncID := range truncatedIDs {
		if err := m.checkBackpressure(ctx); err != nil {
			return pushed, err
		}

		events, err := m.queryEventsByIDPrefix(ctx, truncID)
		if err != nil {
			log.D.F("failed to query event with prefix %s: %v", truncID, err)
			continue
		}

		for _, ev := range events {
			if kind.IsPrivileged(ev.Kind) {
				continue
			}
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
	limit := uint(10000)

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
			idBytes, err := hex.DecodeString(fullID)
			if err != nil {
				log.D.F("failed to decode ID %s: %v", fullID, err)
				continue
			}

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

// fetchEventsFromPeer fetches specific events from a peer by ID.
func (m *Manager) fetchEventsFromPeer(ctx context.Context, conn *websocket.Conn, baseSubID string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	log.D.F("fetchEventsFromPeer: fetching %d events with IDs (first 3): %v", len(ids), ids[:min(3, len(ids))])

	const batchSize = 100
	fetched := 0

	for i := 0; i < len(ids); i += batchSize {
		if err := m.checkBackpressure(ctx); err != nil {
			return fetched, err
		}

		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		subID := fmt.Sprintf("%s-fetch-%d", baseSubID, i/batchSize)
		log.D.F("fetchEventsFromPeer: sending REQ %s for batch of %d IDs", subID, len(batch))

		filterMap := map[string]any{
			"ids": batch,
		}
		req := []any{"REQ", subID, filterMap}
		reqJSON, _ := json.Marshal(req)
		log.D.F("fetchEventsFromPeer: REQ message: %s", string(reqJSON)[:min(500, len(reqJSON))])

		if err := conn.WriteJSON(req); err != nil {
			log.E.F("fetchEventsFromPeer: failed to send REQ: %v", err)
			return fetched, fmt.Errorf("failed to send REQ: %w", err)
		}

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
					if err := m.checkBackpressure(ctx); err != nil {
						return fetched, err
					}
					if err := m.storeEventFromJSON(ctx, msg[2]); err != nil {
						log.W.F("fetchEventsFromPeer: failed to store event: %v", err)
					} else {
						fetched++
						if fetched%10 == 0 {
							log.D.F("fetchEventsFromPeer: stored %d events so far", fetched)
						}
					}
				}
			case "EOSE":
				log.D.F("fetchEventsFromPeer: received EOSE for %s after %d messages, fetched %d events in batch", subID, messageCount, fetched)
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
		closeMsg := []any{"CLOSE", subID}
		conn.WriteJSON(closeMsg)
	}

	log.D.F("fetchEventsFromPeer: completed, total fetched: %d", fetched)
	return fetched, nil
}

// storeEventFromJSON stores an event from raw JSON.
func (m *Manager) storeEventFromJSON(ctx context.Context, eventJSON json.RawMessage) error {
	ev := &event.E{}
	if err := ev.UnmarshalJSON(eventJSON); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	if ok, err := ev.Verify(); err != nil || !ok {
		return fmt.Errorf("event verification failed")
	}

	_, err := m.db.SaveEvent(ctx, ev)
	return err
}

// IsActive returns whether background sync is running.
func (m *Manager) IsActive() bool {
	req := isActiveReq{resp: make(chan bool, 1)}
	m.isActiveCh <- req
	return <-req.resp
}

// LastSync returns the timestamp of the last sync cycle.
func (m *Manager) LastSync() time.Time {
	req := lastSyncReq{resp: make(chan time.Time, 1)}
	m.lastSyncCh <- req
	return <-req.resp
}

// GetPeers returns the list of peer URLs.
func (m *Manager) GetPeers() []string {
	req := getPeersReq{resp: make(chan []string, 1)}
	m.getPeersCh <- req
	return <-req.resp
}

// GetPeerStates returns the sync state for all peers.
func (m *Manager) GetPeerStates() []*PeerState {
	req := getPeerStatesReq{resp: make(chan []*PeerState, 1)}
	m.getPeerStatesCh <- req
	return <-req.resp
}

// GetPeerState returns the sync state for a specific peer.
func (m *Manager) GetPeerState(peerURL string) (*PeerState, bool) {
	req := getPeerStateReq{peerURL: peerURL, resp: make(chan getPeerStateResp, 1)}
	m.getPeerStateCh <- req
	resp := <-req.resp
	return resp.state, resp.ok
}

// AddPeer adds a peer for negentropy sync.
func (m *Manager) AddPeer(peerURL string) {
	m.addPeerCh <- addPeerReq{peerURL: peerURL}
}

// RemovePeer removes a peer from negentropy sync.
func (m *Manager) RemovePeer(peerURL string) {
	m.removePeerCh <- removePeerReq{peerURL: peerURL}
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
	req := openSessionReq{
		connectionID:   connectionID,
		subscriptionID: subscriptionID,
		resp:           make(chan *ClientSession, 1),
	}
	m.openSessionCh <- req
	return <-req.resp
}

// GetSession retrieves an existing session.
func (m *Manager) GetSession(connectionID, subscriptionID string) (*ClientSession, bool) {
	req := getSessionReq{
		connectionID:   connectionID,
		subscriptionID: subscriptionID,
		resp:           make(chan getSessionResp, 1),
	}
	m.getSessionCh <- req
	resp := <-req.resp
	return resp.session, resp.ok
}

// UpdateSessionActivity updates the last activity time for a session.
func (m *Manager) UpdateSessionActivity(connectionID, subscriptionID string) {
	m.updateSessionActivityCh <- updateSessionActivityReq{
		connectionID:   connectionID,
		subscriptionID: subscriptionID,
	}
}

// CloseSession closes a client session.
func (m *Manager) CloseSession(connectionID, subscriptionID string) {
	m.closeSessionCh <- closeSessionReq{
		connectionID:   connectionID,
		subscriptionID: subscriptionID,
	}
}

// CloseSessionsByConnection closes all sessions for a connection.
func (m *Manager) CloseSessionsByConnection(connectionID string) {
	m.closeSessionsByConnCh <- closeSessionsByConnReq{connectionID: connectionID}
}

// ListSessions returns all active sessions.
func (m *Manager) ListSessions() []*ClientSession {
	req := listSessionsReq{resp: make(chan []*ClientSession, 1)}
	m.listSessionsCh <- req
	return <-req.resp
}

// CleanupExpiredSessions removes sessions that have been inactive beyond timeout.
func (m *Manager) CleanupExpiredSessions() int {
	req := cleanupExpiredSessionsReq{resp: make(chan int, 1)}
	m.cleanupExpiredSessionsCh <- req
	return <-req.resp
}

// Ensure chk is used
var _ = chk.E
