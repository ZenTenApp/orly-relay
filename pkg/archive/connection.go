package archive

import (
	"context"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/log"
)

// RelayConnection manages a single archive relay connection.
type RelayConnection struct {
	url    string
	client *ws.Client
	ctx    context.Context
	cancel context.CancelFunc

	// Connection state
	mu             sync.RWMutex
	lastConnect    time.Time
	reconnectDelay time.Duration
	connected      bool
}

const (
	// Initial delay between reconnection attempts
	initialReconnectDelay = 5 * time.Second
	// Maximum delay between reconnection attempts
	maxReconnectDelay = 5 * time.Minute
	// Connection timeout
	connectTimeout = 10 * time.Second
	// Query timeout (per query, not global)
	queryTimeout = 30 * time.Second
)

// NewRelayConnection creates a new relay connection.
func NewRelayConnection(parentCtx context.Context, url string) *RelayConnection {
	ctx, cancel := context.WithCancel(parentCtx)
	return &RelayConnection{
		url:            url,
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: initialReconnectDelay,
	}
}

// Connect establishes a connection to the archive relay.
func (rc *RelayConnection) Connect() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.connected && rc.client != nil {
		return nil
	}

	connectCtx, cancel := context.WithTimeout(rc.ctx, connectTimeout)
	defer cancel()

	client, err := ws.RelayConnect(connectCtx, rc.url)
	if err != nil {
		rc.reconnectDelay = min(rc.reconnectDelay*2, maxReconnectDelay)
		return err
	}

	rc.client = client
	rc.connected = true
	rc.lastConnect = time.Now()
	rc.reconnectDelay = initialReconnectDelay

	log.D.F("archive: connected to %s", rc.url)

	return nil
}

// Query executes a query against the archive relay.
// Returns a slice of events matching the filter.
func (rc *RelayConnection) Query(ctx context.Context, f *filter.F) ([]*event.E, error) {
	rc.mu.RLock()
	client := rc.client
	connected := rc.connected
	rc.mu.RUnlock()

	if !connected || client == nil {
		if err := rc.Connect(); err != nil {
			return nil, err
		}
		rc.mu.RLock()
		client = rc.client
		rc.mu.RUnlock()
	}

	// Create query context with timeout
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Subscribe to the filter
	sub, err := client.Subscribe(queryCtx, filter.NewS(f))
	if err != nil {
		rc.handleDisconnection()
		return nil, err
	}
	defer sub.Unsub()

	// Collect events until EOSE or timeout
	var events []*event.E

	for {
		select {
		case <-queryCtx.Done():
			return events, nil
		case <-sub.EndOfStoredEvents:
			return events, nil
		case ev := <-sub.Events:
			if ev == nil {
				return events, nil
			}
			events = append(events, ev)
		}
	}
}

// handleDisconnection marks the connection as disconnected.
func (rc *RelayConnection) handleDisconnection() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.connected = false
	if rc.client != nil {
		rc.client.Close()
		rc.client = nil
	}
}

// IsConnected returns whether the relay is currently connected.
func (rc *RelayConnection) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if !rc.connected || rc.client == nil {
		return false
	}

	// Check if client is still connected
	return rc.client.IsConnected()
}

// Close closes the relay connection.
func (rc *RelayConnection) Close() {
	rc.cancel()

	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.connected = false
	if rc.client != nil {
		rc.client.Close()
		rc.client = nil
	}
}

// URL returns the relay URL.
func (rc *RelayConnection) URL() string {
	return rc.url
}

// min returns the smaller of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
