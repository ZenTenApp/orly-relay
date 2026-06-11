// Package archive provides query augmentation from authoritative archive relays.
// It manages connections to archive relays and fetches events that match local
// queries, caching them locally for future access.
package archive

import (
	"context"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
)

// ArchiveDatabase defines the interface for storing fetched events.
type ArchiveDatabase interface {
	SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error)
}

// EventDeliveryChannel defines the interface for streaming results back to clients.
type EventDeliveryChannel interface {
	SendEvent(ev *event.E) error
	IsConnected() bool
}

// mgrGetOrCreateReq asks the actor to return or create a connection.
type mgrGetOrCreateReq struct {
	url  string
	resp chan mgrGetOrCreateResp
}

type mgrGetOrCreateResp struct {
	conn *RelayConnection
	err  error
}

// mgrStatsReq asks the actor for current stats.
type mgrStatsReq struct {
	resp chan ManagerStats
}

// Manager handles connections to archive relays for query augmentation.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	relays     []string
	timeout    time.Duration
	db         ArchiveDatabase
	queryCache *QueryCache

	getOrCreateReq chan mgrGetOrCreateReq
	statsReq       chan mgrStatsReq
	stop           chan struct{}
	done           chan struct{}

	// Configuration
	enabled bool
}

// Config holds the configuration for the archive manager.
type Config struct {
	Enabled     bool
	Relays      []string
	TimeoutSec  int
	CacheTTLHrs int
}

// New creates a new archive manager.
func New(ctx context.Context, db ArchiveDatabase, cfg Config) *Manager {
	mgrCtx, cancel := context.WithCancel(ctx)

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	cacheTTL := time.Duration(cfg.CacheTTLHrs) * time.Hour
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}

	enabled := cfg.Enabled && len(cfg.Relays) > 0

	m := &Manager{
		ctx:            mgrCtx,
		cancel:         cancel,
		relays:         cfg.Relays,
		timeout:        timeout,
		db:             db,
		queryCache:     NewQueryCache(cacheTTL, 100000), // 100k cached queries
		getOrCreateReq: make(chan mgrGetOrCreateReq),
		statsReq:       make(chan mgrStatsReq),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		enabled:        enabled,
	}

	go m.actor()

	if enabled {
		log.I.F("archive manager initialized with %d relays, %v timeout, %v cache TTL",
			len(cfg.Relays), timeout, cacheTTL)
	}

	return m
}

// actor owns the connections map and processes requests sequentially.
func (m *Manager) actor() {
	defer close(m.done)

	connections := make(map[string]*RelayConnection)

	for {
		select {
		case req := <-m.getOrCreateReq:
			conn, exists := connections[req.url]
			if exists && conn.IsConnected() {
				req.resp <- mgrGetOrCreateResp{conn: conn}
				continue
			}

			// Create new connection
			conn = NewRelayConnection(m.ctx, req.url)
			if err := conn.Connect(); err != nil {
				req.resp <- mgrGetOrCreateResp{err: err}
				continue
			}
			connections[req.url] = conn
			req.resp <- mgrGetOrCreateResp{conn: conn}

		case req := <-m.statsReq:
			connected := 0
			for _, conn := range connections {
				if conn.IsConnected() {
					connected++
				}
			}
			req.resp <- ManagerStats{
				Enabled:          m.enabled,
				TotalRelays:      len(m.relays),
				ConnectedRelays:  connected,
				CachedQueries:    m.queryCache.Len(),
				MaxCachedQueries: m.queryCache.MaxSize(),
			}

		case <-m.stop:
			for _, conn := range connections {
				conn.Close()
			}
			connections = make(map[string]*RelayConnection)
			return
		}
	}
}

// IsEnabled returns whether the archive manager is enabled.
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// getOrCreateConnection returns an existing connection or creates a new one.
func (m *Manager) getOrCreateConnection(url string) (*RelayConnection, error) {
	req := mgrGetOrCreateReq{
		url:  url,
		resp: make(chan mgrGetOrCreateResp, 1),
	}
	select {
	case m.getOrCreateReq <- req:
		r := <-req.resp
		return r.conn, r.err
	case <-m.stop:
		return nil, m.ctx.Err()
	}
}

// QueryArchive queries archive relays asynchronously and stores/streams results.
// This should be called in a goroutine after returning local results.
//
// Parameters:
//   - subID: the subscription ID for the query
//   - connID: the connection ID (for access tracking)
//   - f: the filter to query
//   - delivered: map of event IDs already delivered to the client
//   - listener: optional channel to stream results back (may be nil)
func (m *Manager) QueryArchive(
	subID string,
	connID string,
	f *filter.F,
	delivered map[string]struct{},
	listener EventDeliveryChannel,
) {
	if !m.enabled {
		return
	}

	// Check if this query was recently executed
	if m.queryCache.HasQueried(f) {
		log.D.F("archive: query cache hit, skipping archive query for sub %s", subID)
		return
	}

	// Mark query as executed
	m.queryCache.MarkQueried(f)

	// Create query context with timeout
	queryCtx, cancel := context.WithTimeout(m.ctx, m.timeout)
	defer cancel()

	// Query all relays in parallel
	results := make(chan *event.E, 1000)
	done := make([]chan struct{}, len(m.relays))

	for i, relayURL := range m.relays {
		done[i] = make(chan struct{})
		go func(d chan struct{}, url string) {
			defer close(d)
			m.queryRelay(queryCtx, url, f, results)
		}(done[i], relayURL)
	}

	// Close results channel when all relays are done
	go func() {
		for _, d := range done {
			<-d
		}
		close(results)
	}()

	// Process results
	stored := 0
	streamed := 0

	for ev := range results {
		// Skip if already delivered
		evIDStr := string(ev.ID[:])
		if _, exists := delivered[evIDStr]; exists {
			continue
		}

		// Store event
		exists, err := m.db.SaveEvent(queryCtx, ev)
		if err != nil {
			log.D.F("archive: failed to save event: %v", err)
			continue
		}
		if !exists {
			stored++
		}

		// Stream to client if still connected
		if listener != nil && listener.IsConnected() {
			if err := listener.SendEvent(ev); err == nil {
				streamed++
				delivered[evIDStr] = struct{}{}
			}
		}
	}

	if stored > 0 || streamed > 0 {
		log.D.F("archive: query %s completed - stored: %d, streamed: %d", subID, stored, streamed)
	}
}

// queryRelay queries a single archive relay and sends results to the channel.
func (m *Manager) queryRelay(ctx context.Context, url string, f *filter.F, results chan<- *event.E) {
	conn, err := m.getOrCreateConnection(url)
	if err != nil {
		log.D.F("archive: failed to connect to %s: %v", url, err)
		return
	}

	events, err := conn.Query(ctx, f)
	if err != nil {
		log.D.F("archive: query failed on %s: %v", url, err)
		return
	}

	for _, ev := range events {
		select {
		case <-ctx.Done():
			return
		case results <- ev:
		}
	}
}

// QueryRelays queries explicit relay URLs and stores/streams results.
// Unlike QueryArchive, this uses client-provided relay URLs and skips the query cache.
// Works even when archive mode is disabled - only needs a valid context and database.
func (m *Manager) QueryRelays(
	subID string,
	connID string,
	f *filter.F,
	relayURLs []string,
	timeout time.Duration,
	delivered map[string]struct{},
	listener EventDeliveryChannel,
) {
	if len(relayURLs) == 0 {
		return
	}

	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	queryCtx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	results := make(chan *event.E, 1000)
	done := make([]chan struct{}, len(relayURLs))

	for i, relayURL := range relayURLs {
		done[i] = make(chan struct{})
		go func(d chan struct{}, url string) {
			defer close(d)
			m.queryRelay(queryCtx, url, f, results)
		}(done[i], relayURL)
	}

	go func() {
		for _, d := range done {
			<-d
		}
		close(results)
	}()

	stored := 0
	streamed := 0

	for ev := range results {
		evIDStr := string(ev.ID[:])
		if _, exists := delivered[evIDStr]; exists {
			continue
		}

		exists, err := m.db.SaveEvent(queryCtx, ev)
		if err != nil {
			log.D.F("proxy: failed to save event: %v", err)
			continue
		}
		if !exists {
			stored++
		}

		if listener != nil && listener.IsConnected() {
			if err := listener.SendEvent(ev); err == nil {
				streamed++
				delivered[evIDStr] = struct{}{}
			}
		}
	}

	if stored > 0 || streamed > 0 {
		log.I.F("proxy: query %s completed - stored: %d, streamed: %d from %d relays",
			subID, stored, streamed, len(relayURLs))
	}
}

// Stop stops the archive manager and closes all connections.
func (m *Manager) Stop() {
	m.cancel()
	close(m.stop)
	<-m.done
	m.queryCache.Stop()

	if m.enabled {
		log.I.F("archive manager stopped")
	}
}

// Stats returns current archive manager statistics.
func (m *Manager) Stats() ManagerStats {
	if !m.enabled {
		return ManagerStats{}
	}

	req := mgrStatsReq{resp: make(chan ManagerStats, 1)}
	select {
	case m.statsReq <- req:
		return <-req.resp
	case <-m.stop:
		return ManagerStats{}
	}
}

// ManagerStats holds archive manager statistics.
type ManagerStats struct {
	Enabled          bool
	TotalRelays      int
	ConnectedRelays  int
	CachedQueries    int
	MaxCachedQueries int
}
