package archive

import (
	"context"
	"errors"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

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

// connConnectReq asks the actor to establish a connection.
type connConnectReq struct {
	resp chan error
}

// connQueryReq asks the actor to execute a query.
type connQueryReq struct {
	ctx  context.Context
	f    *filter.F
	resp chan connQueryResp
}

type connQueryResp struct {
	events []*event.E
	err    error
}

// connIsConnectedReq asks whether the relay is currently connected.
type connIsConnectedReq struct {
	resp chan bool
}

// RelayConnection manages a single archive relay connection.
type RelayConnection struct {
	url string
	ctx context.Context
	cancel context.CancelFunc

	connectReq     chan connConnectReq
	queryReq       chan connQueryReq
	isConnectedReq chan connIsConnectedReq
	stop           chan struct{}
	done           chan struct{}
}

// NewRelayConnection creates a new relay connection.
func NewRelayConnection(parentCtx context.Context, url string) *RelayConnection {
	ctx, cancel := context.WithCancel(parentCtx)
	rc := &RelayConnection{
		url:            url,
		ctx:            ctx,
		cancel:         cancel,
		connectReq:     make(chan connConnectReq),
		queryReq:       make(chan connQueryReq),
		isConnectedReq: make(chan connIsConnectedReq),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
	go rc.actor()
	return rc
}

// actor owns the connection state and processes requests sequentially.
func (rc *RelayConnection) actor() {
	defer close(rc.done)

	var client *ws.Client
	var connected bool
	var reconnectDelay = initialReconnectDelay

	doConnect := func() error {
		if connected && client != nil {
			return nil
		}
		connectCtx, cancel := context.WithTimeout(rc.ctx, connectTimeout)
		defer cancel()

		c, err := ws.RelayConnect(connectCtx, rc.url)
		if err != nil {
			reconnectDelay = min(reconnectDelay*2, maxReconnectDelay)
			return err
		}
		client = c
		connected = true
		reconnectDelay = initialReconnectDelay
		log.D.F("archive: connected to %s", rc.url)
		return nil
	}

	handleDisconnection := func() {
		connected = false
		if client != nil {
			client.Close()
			client = nil
		}
	}

	for {
		select {
		case <-rc.stop:
			handleDisconnection()
			return
		case <-rc.ctx.Done():
			handleDisconnection()
			return

		case req := <-rc.connectReq:
			req.resp <- doConnect()

		case req := <-rc.queryReq:
			// Ensure connected
			if !connected || client == nil {
				if err := doConnect(); err != nil {
					req.resp <- connQueryResp{err: err}
					continue
				}
			}

			// Create query context with timeout
			queryCtx, cancel := context.WithTimeout(req.ctx, queryTimeout)

			// Subscribe to the filter
			sub, err := client.Subscribe(queryCtx, filter.NewS(req.f))
			if err != nil {
				cancel()
				handleDisconnection()
				req.resp <- connQueryResp{err: err}
				continue
			}

			// Collect events until EOSE or timeout
			var events []*event.E
			func() {
				defer sub.Unsub()
				defer cancel()
				for {
					select {
					case <-queryCtx.Done():
						return
					case <-sub.EndOfStoredEvents:
						return
					case ev := <-sub.Events:
						if ev == nil {
							return
						}
						events = append(events, ev)
					}
				}
			}()

			req.resp <- connQueryResp{events: events}

		case req := <-rc.isConnectedReq:
			if !connected || client == nil {
				req.resp <- false
				continue
			}
			req.resp <- client.IsConnected()
		}
	}
}

// Connect establishes a connection to the archive relay.
func (rc *RelayConnection) Connect() error {
	req := connConnectReq{resp: make(chan error, 1)}
	select {
	case rc.connectReq <- req:
		return <-req.resp
	case <-rc.stop:
		return errors.New("connection stopped")
	}
}

// Query executes a query against the archive relay.
// Returns a slice of events matching the filter.
func (rc *RelayConnection) Query(ctx context.Context, f *filter.F) ([]*event.E, error) {
	req := connQueryReq{
		ctx:  ctx,
		f:    f,
		resp: make(chan connQueryResp, 1),
	}
	select {
	case rc.queryReq <- req:
		r := <-req.resp
		return r.events, r.err
	case <-rc.stop:
		return nil, errors.New("connection stopped")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// IsConnected returns whether the relay is currently connected.
func (rc *RelayConnection) IsConnected() bool {
	req := connIsConnectedReq{resp: make(chan bool, 1)}
	select {
	case rc.isConnectedReq <- req:
		return <-req.resp
	case <-rc.stop:
		return false
	}
}

// Close closes the relay connection.
func (rc *RelayConnection) Close() {
	rc.cancel()
	close(rc.stop)
	<-rc.done
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
