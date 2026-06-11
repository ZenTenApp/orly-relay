package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/log"
)

// --- actor request types ---

type rcGetConnReq struct {
	resp chan *ws.Client
}

type rcSetConnReq struct {
	conn   *ws.Client
	authed bool
	resp   chan struct{}
}

type rcSetAuthedReq struct {
	resp chan struct{}
}

type rcPublishReq struct {
	ctx context.Context
	ev  *event.E
	resp chan error
}

type rcSubscribeReq struct {
	ctx  context.Context
	ff   *filter.S
	resp chan rcSubscribeResp
}

type rcSubscribeResp struct {
	stream *WsEventStream
	err    error
}

type rcFetchKind0Req struct {
	ctx    context.Context
	pubkey []byte
	resp   chan *event.E
}

type rcCloseReq struct {
	resp chan struct{}
}

// RelayConn wraps a WebSocket relay connection with auto-reconnect.
// It satisfies the RelayConnection interface used by the bridge's Marmot
// client for standalone mode (connecting to an external relay).
type RelayConn struct {
	url    string
	sign   signer.I
	conn   *ws.Client
	ctx    context.Context
	cancel context.CancelFunc
	authed bool

	getConnCh  chan rcGetConnReq
	setConnCh  chan rcSetConnReq
	setAuthedCh chan rcSetAuthedReq
	publishCh  chan rcPublishReq
	subscribeCh chan rcSubscribeReq
	fetchK0Ch  chan rcFetchKind0Req
	closeCh    chan rcCloseReq
	stop       chan struct{}
	done       chan struct{}
}

// NewRelayConn creates a new relay connection wrapper.
// The signer is used for NIP-42 authentication when the relay requires it.
func NewRelayConn(url string, sign signer.I) *RelayConn {
	rc := &RelayConn{
		url:         url,
		sign:        sign,
		getConnCh:   make(chan rcGetConnReq),
		setConnCh:   make(chan rcSetConnReq),
		setAuthedCh: make(chan rcSetAuthedReq),
		publishCh:   make(chan rcPublishReq),
		subscribeCh: make(chan rcSubscribeReq),
		fetchK0Ch:   make(chan rcFetchKind0Req),
		closeCh:     make(chan rcCloseReq),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	go rc.loop()
	return rc
}

func (rc *RelayConn) loop() {
	defer close(rc.done)
	for {
		select {
		case <-rc.stop:
			return
		case req := <-rc.getConnCh:
			req.resp <- rc.conn
		case req := <-rc.setConnCh:
			rc.conn = req.conn
			rc.authed = req.authed
			req.resp <- struct{}{}
		case req := <-rc.setAuthedCh:
			rc.authed = true
			req.resp <- struct{}{}
		case req := <-rc.publishCh:
			req.resp <- rc.doPublish(req.ctx, req.ev)
		case req := <-rc.subscribeCh:
			s, err := rc.doSubscribe(req.ctx, req.ff)
			req.resp <- rcSubscribeResp{s, err}
		case req := <-rc.fetchK0Ch:
			req.resp <- rc.doFetchKind0(req.ctx, req.pubkey)
		case req := <-rc.closeCh:
			rc.doClose()
			req.resp <- struct{}{}
		}
	}
}

// getConn retrieves the current connection from the actor.
func (rc *RelayConn) getConn() *ws.Client {
	req := rcGetConnReq{resp: make(chan *ws.Client, 1)}
	rc.getConnCh <- req
	return <-req.resp
}

// setConn sets the connection and authed state via the actor.
func (rc *RelayConn) setConn(conn *ws.Client, authed bool) {
	req := rcSetConnReq{conn: conn, authed: authed, resp: make(chan struct{}, 1)}
	rc.setConnCh <- req
	<-req.resp
}

// setAuthed marks the connection as authenticated via the actor.
func (rc *RelayConn) setAuthed() {
	req := rcSetAuthedReq{resp: make(chan struct{}, 1)}
	rc.setAuthedCh <- req
	<-req.resp
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

	rc.setConn(conn, false)

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

	rc.setAuthed()

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
			rc.setConn(conn, false)
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
	req := rcPublishReq{ctx: ctx, ev: ev, resp: make(chan error, 1)}
	rc.publishCh <- req
	return <-req.resp
}

func (rc *RelayConn) doPublish(ctx context.Context, ev *event.E) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected to relay")
	}

	err := rc.conn.Publish(ctx, ev)
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

	if authErr := rc.conn.Auth(ctx, rc.sign); authErr != nil {
		return fmt.Errorf("auth failed: %w", authErr)
	}

	rc.authed = true

	log.I.F("bridge authenticated with relay")

	// Retry the publish
	return rc.conn.Publish(ctx, ev)
}

// Subscribe creates a subscription on the relay and returns a stream of events.
func (rc *RelayConn) Subscribe(ctx context.Context, ff *filter.S) (*WsEventStream, error) {
	req := rcSubscribeReq{ctx: ctx, ff: ff, resp: make(chan rcSubscribeResp, 1)}
	rc.subscribeCh <- req
	r := <-req.resp
	return r.stream, r.err
}

func (rc *RelayConn) doSubscribe(ctx context.Context, ff *filter.S) (*WsEventStream, error) {
	if rc.conn == nil {
		return nil, fmt.Errorf("not connected to relay")
	}

	sub, err := rc.conn.Subscribe(ctx, ff)
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
	req := rcCloseReq{resp: make(chan struct{}, 1)}
	rc.closeCh <- req
	<-req.resp
	// Now stop the actor
	close(rc.stop)
	<-rc.done
}

func (rc *RelayConn) doClose() {
	if rc.conn != nil {
		rc.conn.Close()
		rc.conn = nil
	}
}

// FetchKind0 fetches the latest kind 0 profile event for a pubkey.
// Returns nil if not found or on error.
func (rc *RelayConn) FetchKind0(ctx context.Context, pubkey []byte) *event.E {
	req := rcFetchKind0Req{ctx: ctx, pubkey: pubkey, resp: make(chan *event.E, 1)}
	rc.fetchK0Ch <- req
	return <-req.resp
}

func (rc *RelayConn) doFetchKind0(ctx context.Context, pubkey []byte) *event.E {
	if rc.conn == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	f := filter.New()
	f.Authors = &tag.T{T: [][]byte{pubkey}}
	f.Kinds = kind.NewS(kind.New(0))
	one := uint(1)
	f.Limit = &one

	events, err := rc.conn.QuerySync(ctx, f)
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
