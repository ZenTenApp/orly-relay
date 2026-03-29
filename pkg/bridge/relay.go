package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/nostr/ws"
	"next.orly.dev/pkg/lol/log"
)

// RelayConn wraps a WebSocket relay connection with auto-reconnect.
// It satisfies the RelayConnection interface used by the bridge's Marmot
// client for standalone mode (connecting to an external relay).
type RelayConn struct {
	url    string
	sign   signer.I
	conn   *ws.Client
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	authed bool
}

// NewRelayConn creates a new relay connection wrapper.
// The signer is used for NIP-42 authentication when the relay requires it.
func NewRelayConn(url string, sign signer.I) *RelayConn {
	return &RelayConn{url: url, sign: sign}
}

// Connect establishes the WebSocket connection to the relay and
// pre-authenticates via NIP-42 so that subscriptions have proper access.
// In monolithic mode, the relay may not be listening yet, so Connect
// retries with exponential backoff for up to 30 seconds.
func (rc *RelayConn) Connect(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	delay := time.Second
	maxDelay := 10 * time.Second
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	var conn *ws.Client
	var err error

	for {
		conn, err = ws.RelayConnect(rc.ctx, rc.url)
		if err == nil {
			break
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("connect to relay %s after %v: %w", rc.url, timeout, err)
		}

		log.D.F("bridge waiting for relay %s: %v (retrying in %v)", rc.url, err, delay)

		select {
		case <-time.After(delay):
			if delay < maxDelay {
				delay *= 2
			}
		case <-rc.ctx.Done():
			return fmt.Errorf("connect to relay %s: %w", rc.url, rc.ctx.Err())
		}
	}

	conn.AssumeValid = true // trust our own relay's signature validation

	rc.mu.Lock()
	rc.conn = conn
	rc.authed = false
	rc.mu.Unlock()

	log.I.F("bridge connected to relay: %s", rc.url)

	// Pre-authenticate so subscriptions get proper access level
	if rc.sign != nil {
		if err := rc.preAuth(conn); err != nil {
			log.W.F("bridge pre-auth failed: %v (will retry on publish)", err)
		}
	}

	return nil
}

// preAuth waits briefly for the relay's AUTH challenge, then authenticates.
func (rc *RelayConn) preAuth(conn *ws.Client) error {
	// Give the relay time to send the AUTH challenge
	time.Sleep(200 * time.Millisecond)

	if err := conn.Auth(rc.ctx, rc.sign); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	rc.mu.Lock()
	rc.authed = true
	rc.mu.Unlock()

	log.I.F("bridge pre-authenticated with relay")
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
			rc.authed = false
			rc.mu.Unlock()
			log.I.F("bridge reconnected to relay: %s", rc.url)

			// Pre-authenticate after reconnect
			if rc.sign != nil {
				if err := rc.preAuth(conn); err != nil {
					log.W.F("bridge pre-auth after reconnect failed: %v", err)
				}
			}

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

// Publish sends an event to the relay. If the relay responds with
// auth-required, the bridge authenticates via NIP-42 and retries once.
func (rc *RelayConn) Publish(ctx context.Context, ev *event.E) error {
	rc.mu.RLock()
	conn := rc.conn
	rc.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to relay")
	}

	err := conn.Publish(ctx, ev)
	if err == nil {
		return nil
	}

	// Check if the error is auth-required
	if !strings.Contains(err.Error(), "auth-required") {
		return err
	}

	// Authenticate and retry
	if rc.sign == nil {
		return fmt.Errorf("auth required but no signer configured")
	}

	log.D.F("relay requires auth, authenticating...")

	// Give the relay a moment to send the challenge
	time.Sleep(100 * time.Millisecond)

	if authErr := conn.Auth(ctx, rc.sign); authErr != nil {
		return fmt.Errorf("auth failed: %w", authErr)
	}

	rc.mu.Lock()
	rc.authed = true
	rc.mu.Unlock()

	log.I.F("bridge authenticated with relay")

	// Retry the publish
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

// FetchKind0 fetches the latest kind 0 profile event for a pubkey.
// Returns nil if not found or on error.
func (rc *RelayConn) FetchKind0(ctx context.Context, pubkey []byte) *event.E {
	rc.mu.RLock()
	conn := rc.conn
	rc.mu.RUnlock()
	if conn == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	f := filter.New()
	f.Authors = &tag.T{T: [][]byte{pubkey}}
	f.Kinds = kind.NewS(kind.New(0))
	one := uint(1)
	f.Limit = &one

	events, err := conn.QuerySync(ctx, f)
	if err != nil || len(events) == 0 {
		return nil
	}
	return events[0]
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
