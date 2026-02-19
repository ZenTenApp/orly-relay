package bridge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/ws"
	"lol.mleku.dev/log"
)

// RelayConn wraps a WebSocket relay connection with auto-reconnect.
// It satisfies the RelayConnection interface used by the bridge's Marmot
// client for standalone mode (connecting to an external relay).
type RelayConn struct {
	url    string
	conn   *ws.Client
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRelayConn creates a new relay connection wrapper.
func NewRelayConn(url string) *RelayConn {
	return &RelayConn{url: url}
}

// Connect establishes the WebSocket connection to the relay.
func (rc *RelayConn) Connect(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	conn, err := ws.RelayConnect(rc.ctx, rc.url)
	if err != nil {
		return fmt.Errorf("connect to relay %s: %w", rc.url, err)
	}

	rc.mu.Lock()
	rc.conn = conn
	rc.mu.Unlock()

	log.I.F("bridge connected to relay: %s", rc.url)
	return nil
}

// Reconnect attempts to reconnect with exponential backoff.
func (rc *RelayConn) Reconnect() error {
	delay := time.Second
	maxDelay := 30 * time.Second

	for {
		select {
		case <-rc.ctx.Done():
			return rc.ctx.Err()
		default:
		}

		conn, err := ws.RelayConnect(rc.ctx, rc.url)
		if err == nil {
			rc.mu.Lock()
			rc.conn = conn
			rc.mu.Unlock()
			log.I.F("bridge reconnected to relay: %s", rc.url)
			return nil
		}

		log.W.F("bridge relay reconnect failed: %v, retrying in %v", err, delay)
		select {
		case <-time.After(delay):
			if delay < maxDelay {
				delay *= 2
			}
		case <-rc.ctx.Done():
			return rc.ctx.Err()
		}
	}
}

// Publish sends an event to the relay.
func (rc *RelayConn) Publish(ctx context.Context, ev *event.E) error {
	rc.mu.RLock()
	conn := rc.conn
	rc.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to relay")
	}

	return conn.Publish(ctx, ev)
}

// Subscribe creates a subscription on the relay and returns a stream of events.
func (rc *RelayConn) Subscribe(ctx context.Context, ff *filter.S) (*WsEventStream, error) {
	rc.mu.RLock()
	conn := rc.conn
	rc.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected to relay")
	}

	sub, err := conn.Subscribe(ctx, ff)
	if err != nil {
		return nil, err
	}

	return &WsEventStream{sub: sub}, nil
}

// Close closes the relay connection.
func (rc *RelayConn) Close() {
	if rc.cancel != nil {
		rc.cancel()
	}
	rc.mu.Lock()
	if rc.conn != nil {
		rc.conn.Close()
		rc.conn = nil
	}
	rc.mu.Unlock()
}

// WsEventStream wraps a ws.Subscription to deliver events.
type WsEventStream struct {
	sub *ws.Subscription
}

// Events returns the channel of events.
func (s *WsEventStream) Events() <-chan *event.E {
	return s.sub.Events
}

// Close unsubscribes from the relay.
func (s *WsEventStream) Close() {
	s.sub.Unsub()
}
