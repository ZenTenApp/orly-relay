package archive

import (
	"context"
	"time"

	"git.smesh.lol/actor"
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

// queryArgs holds the arguments for a query request.
type queryArgs struct {
	ctx context.Context
	f   *filter.F
}

// connQueryResp wraps the query result.
type connQueryResp struct {
	events []*event.E
	err    error
}

// RelayConnection manages a single archive relay connection.
type RelayConnection struct {
	url    string
	ctx    context.Context
	cancel context.CancelFunc

	connectActor actor.Query[error]
	query        actor.Func[queryArgs, connQueryResp]
	isConnected  actor.Query[bool]
	lc           actor.Lifecycle
}

// NewRelayConnection creates a new relay connection.
func NewRelayConnection(parentCtx context.Context, url string) *RelayConnection {
	ctx, cancel := context.WithCancel(parentCtx)
	rc := &RelayConnection{
		url:          url,
		ctx:          ctx,
		cancel:       cancel,
		connectActor: actor.NewQuery[error](),
		query:        actor.NewFunc[queryArgs, connQueryResp](),
		isConnected:  actor.NewQuery[bool](),
		lc:           actor.NewLifecycle(),
	}
	actor.Go(rc.lc, rc.actorLoop)
	return rc
}

// actorLoop owns the connection state and processes requests sequentially.
func (rc *RelayConnection) actorLoop() {
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
		case <-rc.lc.Stopping():
			handleDisconnection()
			return
		case <-rc.ctx.Done():
			handleDisconnection()
			return

		case msg := <-rc.connectActor.Recv():
			msg.Reply(doConnect())

		case msg := <-rc.query.Recv():
			if !connected || client == nil {
				if err := doConnect(); err != nil {
					msg.Reply(connQueryResp{err: err})
					continue
				}
			}

			queryCtx, cancel := context.WithTimeout(msg.Req.ctx, queryTimeout)

			sub, err := client.Subscribe(queryCtx, filter.NewS(msg.Req.f))
			if err != nil {
				cancel()
				handleDisconnection()
				msg.Reply(connQueryResp{err: err})
				continue
			}

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

			msg.Reply(connQueryResp{events: events})

		case msg := <-rc.isConnected.Recv():
			if !connected || client == nil {
				msg.Reply(false)
				continue
			}
			msg.Reply(client.IsConnected())
		}
	}
}

// Connect establishes a connection to the archive relay.
func (rc *RelayConnection) Connect() error { return rc.connectActor.Call() }

// Query executes a query against the archive relay.
// Uses CallCtx so the caller can bail if ctx is cancelled.
func (rc *RelayConnection) Query(ctx context.Context, f *filter.F) ([]*event.E, error) {
	r, err := rc.query.CallCtx(ctx, queryArgs{ctx: ctx, f: f})
	if err != nil {
		return nil, err
	}
	return r.events, r.err
}

// IsConnected returns whether the relay is currently connected.
func (rc *RelayConnection) IsConnected() bool { return rc.isConnected.Call() }

// Close closes the relay connection.
func (rc *RelayConnection) Close() {
	rc.cancel()
	rc.lc.Stop()
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
