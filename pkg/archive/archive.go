// Package archive provides query augmentation from authoritative archive relays.
// It manages connections to archive relays and fetches events that match local
// queries, caching them locally for future access.
package archive

import (
	"context"
	"sync"
	"time"

	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
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

// Manager handles connections to archive relays for query augmentation.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	relays     []string
	timeout    time.Duration
	db         ArchiveDatabase
	queryCache *QueryCache

	// Connection pool
	mu          sync.RWMutex
	connections map[string]*RelayConnection

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
	if !cfg.Enabled || len(cfg.Relays) == 0 {
		return &Manager{enabled: false}
	}

	mgrCtx, cancel := context.WithCancel(ctx)

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	cacheTTL := time.Duration(cfg.CacheTTLHrs) * time.Hour
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}

	m := &Manager{
		ctx:         mgrCtx,
		cancel:      cancel,
		relays:      cfg.Relays,
		timeout:     timeout,
		db:          db,
		queryCache:  NewQueryCache(cacheTTL, 100000), // 100k cached queries
		connections: make(map[string]*RelayConnection),
		enabled:     true,
	}

	log.I.F("archive manager initialized with %d relays, %v timeout, %v cache TTL",
		len(cfg.Relays), timeout, cacheTTL)

	return m
}

// IsEnabled returns whether the archive manager is enabled.
func (m *Manager) IsEnabled() bool {
	return m.enabled
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
	var wg sync.WaitGroup
	results := make(chan *event.E, 1000)

	for _, relayURL := range m.relays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			m.queryRelay(queryCtx, url, f, results)
		}(relayURL)
	}

	// Close results channel when all relays are done
	go func() {
		wg.Wait()
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

// getOrCreateConnection returns an existing connection or creates a new one.
func (m *Manager) getOrCreateConnection(url string) (*RelayConnection, error) {
	m.mu.RLock()
	conn, exists := m.connections[url]
	m.mu.RUnlock()

	if exists && conn.IsConnected() {
		return conn, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	conn, exists = m.connections[url]
	if exists && conn.IsConnected() {
		return conn, nil
	}

	// Create new connection
	conn = NewRelayConnection(m.ctx, url)
	if err := conn.Connect(); err != nil {
		return nil, err
	}

	m.connections[url] = conn
	return conn, nil
}

// Stop stops the archive manager and closes all connections.
func (m *Manager) Stop() {
	if !m.enabled {
		return
	}

	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		conn.Close()
	}
	m.connections = make(map[string]*RelayConnection)

	log.I.F("archive manager stopped")
}

// Stats returns current archive manager statistics.
func (m *Manager) Stats() ManagerStats {
	if !m.enabled {
		return ManagerStats{}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	connected := 0
	for _, conn := range m.connections {
		if conn.IsConnected() {
			connected++
		}
	}

	return ManagerStats{
		Enabled:          m.enabled,
		TotalRelays:      len(m.relays),
		ConnectedRelays:  connected,
		CachedQueries:    m.queryCache.Len(),
		MaxCachedQueries: m.queryCache.MaxSize(),
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
