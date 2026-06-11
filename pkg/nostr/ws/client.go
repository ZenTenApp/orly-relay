package ws

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/authenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/closedenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eoseenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eventenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/noticeenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/okenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/codec"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/utils/normalize"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

var subscriptionIDCh = func() chan int64 {
	ch := make(chan int64, 1)
	go func() {
		var counter int64
		for {
			counter++
			ch <- counter
		}
	}()
	return ch
}()

// --- Actor request types for subscriptions map ---

type subLoadReq struct {
	key  string
	resp chan subLoadResp
}

type subLoadResp struct {
	sub *Subscription
	ok  bool
}

type subStoreReq struct {
	key string
	sub *Subscription
}

type subDeleteReq struct {
	key string
}

type subRangeReq struct {
	resp chan []*Subscription
}

// --- Actor request types for okCallbacks map ---

type okLoadReq struct {
	key  string
	resp chan okLoadResp
}

type okLoadResp struct {
	fn func(bool, string)
	ok bool
}

type okStoreReq struct {
	key string
	fn  func(bool, string)
}

type okDeleteReq struct {
	key string
}

// --- Actor request for close ---

type closeReq struct {
	reason error
	resp   chan error
}

// Client represents a connection to a Nostr relay.
type Client struct {
	URL           string
	requestHeader http.Header // e.g. for origin header

	Connection    *Connection

	ConnectionError         error
	connectionContext       context.Context // will be canceled when the connection closes
	connectionContextCancel context.CancelCauseFunc

	challenge                     []byte       // NIP-42 challenge, we only keep the last
	notices                       chan []byte  // NIP-01 NOTICEs
	customHandler                 func(string) // nonstandard unparseable messages
	writeQueue                    chan writeRequest
	subscriptionChannelCloseQueue chan []byte

	// Actor channels for subscriptions map
	subLoad   chan subLoadReq
	subStore  chan subStoreReq
	subDelete chan subDeleteReq
	subRange  chan subRangeReq

	// Actor channels for okCallbacks map
	okLoad   chan okLoadReq
	okStore  chan okStoreReq
	okDelete chan okDeleteReq

	// Actor channel for close
	closeCh chan closeReq

	// Actor lifecycle
	mapStop chan struct{}
	mapDone chan struct{}

	// custom things that aren't often used
	//
	AssumeValid bool // this will skip verifying signatures for events received from this relay
}

type writeRequest struct {
	msg    []byte
	answer chan error
}

// NewRelay returns a new relay. It takes a context that, when canceled, will close the relay connection.
func NewRelay(ctx context.Context, url string, opts ...RelayOption) *Client {
	ctx, cancel := context.WithCancelCause(ctx)
	r := &Client{
		URL:                     string(normalize.URL(url)),
		connectionContext:       ctx,
		connectionContextCancel: cancel,
		writeQueue:                    make(chan writeRequest),
		subscriptionChannelCloseQueue: make(chan []byte),
		requestHeader:                 nil,
		subLoad:                       make(chan subLoadReq),
		subStore:                      make(chan subStoreReq, 16),
		subDelete:                     make(chan subDeleteReq, 16),
		subRange:                      make(chan subRangeReq),
		okLoad:                        make(chan okLoadReq),
		okStore:                       make(chan okStoreReq, 16),
		okDelete:                      make(chan okDeleteReq, 16),
		closeCh:                       make(chan closeReq),
		mapStop:                       make(chan struct{}),
		mapDone:                       make(chan struct{}),
	}

	for _, opt := range opts {
		opt.ApplyRelayOption(r)
	}

	go r.mapActor()

	return r
}

// mapActor owns the subscriptions map, okCallbacks map, and close state.
func (r *Client) mapActor() {
	defer close(r.mapDone)

	subs := make(map[string]*Subscription)
	okCBs := make(map[string]func(bool, string))
	closed := false

	for {
		select {
		case <-r.mapStop:
			return

		case req := <-r.subLoad:
			sub, ok := subs[req.key]
			req.resp <- subLoadResp{sub, ok}

		case req := <-r.subStore:
			subs[req.key] = req.sub

		case req := <-r.subDelete:
			delete(subs, req.key)

		case req := <-r.subRange:
			all := make([]*Subscription, 0, len(subs))
			for _, sub := range subs {
				all = append(all, sub)
			}
			req.resp <- all

		case req := <-r.okLoad:
			fn, ok := okCBs[req.key]
			req.resp <- okLoadResp{fn, ok}

		case req := <-r.okStore:
			okCBs[req.key] = req.fn

		case req := <-r.okDelete:
			delete(okCBs, req.key)

		case req := <-r.closeCh:
			if closed {
				req.resp <- fmt.Errorf("relay already closed")
				continue
			}
			closed = true
			log.T.F(
				"WS.Client: closing connection to %s: reason=%v lastErr=%v", r.URL,
				req.reason, r.ConnectionError,
			)
			r.connectionContextCancel(req.reason)
			r.connectionContextCancel = nil

			if r.Connection == nil {
				req.resp <- fmt.Errorf("relay not connected")
				continue
			}
			req.resp <- r.Connection.Close()
		}
	}
}

// loadSub loads a subscription from the actor-owned map.
func (r *Client) loadSub(key string) (*Subscription, bool) {
	req := subLoadReq{key: key, resp: make(chan subLoadResp, 1)}
	r.subLoad <- req
	resp := <-req.resp
	return resp.sub, resp.ok
}

// storeSub stores a subscription in the actor-owned map.
func (r *Client) storeSub(key string, sub *Subscription) {
	r.subStore <- subStoreReq{key: key, sub: sub}
}

// deleteSub deletes a subscription from the actor-owned map.
func (r *Client) deleteSub(key string) {
	r.subDelete <- subDeleteReq{key: key}
}

// rangeSubs returns all subscriptions.
func (r *Client) rangeSubs() []*Subscription {
	req := subRangeReq{resp: make(chan []*Subscription, 1)}
	r.subRange <- req
	return <-req.resp
}

// loadOkCallback loads an ok callback from the actor-owned map.
func (r *Client) loadOkCallback(key string) (func(bool, string), bool) {
	req := okLoadReq{key: key, resp: make(chan okLoadResp, 1)}
	r.okLoad <- req
	resp := <-req.resp
	return resp.fn, resp.ok
}

// storeOkCallback stores an ok callback in the actor-owned map.
func (r *Client) storeOkCallback(key string, fn func(bool, string)) {
	r.okStore <- okStoreReq{key: key, fn: fn}
}

// deleteOkCallback deletes an ok callback from the actor-owned map.
func (r *Client) deleteOkCallback(key string) {
	r.okDelete <- okDeleteReq{key: key}
}

// RelayConnect returns a relay object connected to url.
//
// The given subscription is only used during the connection phase. Once successfully connected, cancelling ctx has no effect.
//
// The ongoing relay connection uses a background context. To close the connection, call r.Close().
// If you need fine grained long-term connection contexts, use NewRelay() instead.
func RelayConnect(ctx context.Context, url string, opts ...RelayOption) (
	*Client, error,
) {
	r := NewRelay(context.Background(), url, opts...)
	err := r.Connect(ctx)
	return r, err
}

// RelayOption is the type of the argument passed when instantiating relay connections.
type RelayOption interface {
	ApplyRelayOption(*Client)
}

var (
	_ RelayOption = (WithCustomHandler)(nil)
	_ RelayOption = (WithRequestHeader)(nil)
	_ RelayOption = (WithNoticeHandler)(nil)
)

// WithCustomHandler must be a function that handles any relay message that couldn't be
// parsed as a standard envelope.
type WithCustomHandler func(data string)

func (ch WithCustomHandler) ApplyRelayOption(r *Client) {
	r.customHandler = ch
}

// WithRequestHeader sets the HTTP request header of the websocket preflight request.
type WithRequestHeader http.Header

func (ch WithRequestHeader) ApplyRelayOption(r *Client) {
	r.requestHeader = http.Header(ch)
}

// WithNoticeHandler must be a function that handles NOTICE messages from the relay.
type WithNoticeHandler func(notice []byte)

func (nh WithNoticeHandler) ApplyRelayOption(r *Client) {
	r.notices = make(chan []byte, 8)
	go func() {
		for notice := range r.notices {
			nh(notice)
		}
	}()
}

// String just returns the relay URL.
func (r *Client) String() string {
	return r.URL
}

// Context retrieves the context that is associated with this relay connection.
// It will be closed when the relay is disconnected.
func (r *Client) Context() context.Context { return r.connectionContext }

// IsConnected returns true if the connection to this relay seems to be active.
func (r *Client) IsConnected() bool { return r.connectionContext.Err() == nil }

// ConnectionCause returns the cancel cause for the relay connection context.
func (r *Client) ConnectionCause() error { return context.Cause(r.connectionContext) }

// LastError returns the last connection error observed by the reader loop.
func (r *Client) LastError() error { return r.ConnectionError }

// Connect tries to establish a websocket connection to r.URL.
// If the context expires before the connection is complete, an error is returned.
// Once successfully connected, context expiration has no effect: call r.Close
// to close the connection.
//
// The given context here is only used during the connection phase. The long-living
// relay connection will be based on the context given to NewRelay().
func (r *Client) Connect(ctx context.Context) error {
	return r.ConnectWithTLS(ctx, nil)
}

func extractSubID(jsonStr string) string {
	// look for "EVENT" pattern
	start := strings.Index(jsonStr, `"EVENT"`)
	if start == -1 {
		return ""
	}

	// move to the next quote
	offset := strings.Index(jsonStr[start+7:], `"`)
	if offset == -1 {
		return ""
	}

	start += 7 + offset + 1

	// find the ending quote
	end := strings.Index(jsonStr[start:], `"`)

	// get the contents
	return jsonStr[start : start+end]
}

func subIdToSerial(subId string) int64 {
	n := strings.Index(subId, ":")
	if n < 0 || n > len(subId) {
		return -1
	}
	serialId, _ := strconv.ParseInt(subId[0:n], 10, 64)
	return serialId
}

// ConnectWithTLS is like Connect(), but takes a special tls.Config if you need that.
func (r *Client) ConnectWithTLS(
	ctx context.Context, tlsConfig *tls.Config,
) error {
	if r.connectionContext == nil || r.subLoad == nil {
		return fmt.Errorf("relay must be initialized with a call to NewRelay()")
	}

	if r.URL == "" {
		return fmt.Errorf("invalid relay URL '%s'", r.URL)
	}

	if _, ok := ctx.Deadline(); !ok {
		// if no timeout is set, force it to 7 seconds
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(
			ctx, 7*time.Second, errors.New("connection took too long"),
		)
		defer cancel()
	}

	conn, err := NewConnection(ctx, r.URL, r.requestHeader, tlsConfig)
	if err != nil {
		return fmt.Errorf("error opening websocket to '%s': %w", r.URL, err)
	}
	r.Connection = conn

	// ping every 29 seconds
	ticker := time.NewTicker(29 * time.Second)

	// queue all write operations here so we don't do mutex spaghetti
	go func() {
		var err error
		for {
			select {
			case <-r.connectionContext.Done():
				log.T.F(
					"WS.Client: connection context done for %s: cause=%v lastErr=%v",
					r.URL, context.Cause(r.connectionContext),
					r.ConnectionError,
				)
				ticker.Stop()
				r.Connection = nil

				for _, sub := range r.rangeSubs() {
					sub.unsub(
						fmt.Errorf(
							"relay connection closed: %w / %w",
							context.Cause(r.connectionContext),
							r.ConnectionError,
						),
					)
				}
				return

			case <-ticker.C:
				err = r.Connection.Ping(r.connectionContext)
				if err != nil && !strings.Contains(
					err.Error(), "failed to wait for pong",
				) {
					log.I.F(
						"{%s} error writing ping: %v; closing websocket", r.URL,
						err,
					)
					r.CloseWithReason(
						fmt.Errorf(
							"ping failed: %w", err,
						),
					) // this should trigger a context cancelation
					return
				}

			case wr := <-r.writeQueue:
				// all write requests will go through this to prevent races
				// log.D.F("{%s} sending %v\n", r.URL, string(wr.msg))
				log.T.F(
					"WS.Client: outbound message to %s: %s", r.URL,
					string(wr.msg),
				)
				if err = r.Connection.WriteMessage(
					r.connectionContext, wr.msg,
				); err != nil {
					wr.answer <- err
				}
				close(wr.answer)
			}
		}
	}()

	// general message reader loop
	go func() {

		for {
			buf := new(bytes.Buffer)
			for {
				buf.Reset()
				if err := conn.ReadMessage(
					r.connectionContext, buf,
				); err != nil {
					r.ConnectionError = err
					log.T.F(
						"WS.Client: reader loop error on %s: %v; closing connection",
						r.URL, err,
					)
					r.CloseWithReason(fmt.Errorf("reader loop error: %w", err))
					return
				}
				message := buf.Bytes()
				var t string
				if t, message, err = envelopes.Identify(message); chk.E(err) {
					continue
				}
				switch t {
				case noticeenvelope.L:
					env := noticeenvelope.New()
					if env, message, err = noticeenvelope.Parse(message); chk.E(err) {
						continue
					}
					// see WithNoticeHandler
					if r.notices != nil {
						r.notices <- env.Message
					} else {
						log.E.F("NOTICE from %s: '%s'", r.URL, env.Message)
					}
				case authenvelope.L:
					env := authenvelope.NewChallenge()
					if env, message, err = authenvelope.ParseChallenge(message); chk.E(err) {
						continue
					}
					if len(env.Challenge) == 0 {
						continue
					}
					r.challenge = env.Challenge
				case eventenvelope.L:
					env := eventenvelope.NewResult()
					if env, message, err = eventenvelope.ParseResult(message); chk.E(err) {
						continue
					}
					if len(env.Subscription) == 0 {
						continue
					}
					if sub, ok := r.loadSub(string(env.Subscription)); !ok {
						log.D.F(
							"{%s} no subscription with id '%s'\n", r.URL,
							env.Subscription,
						)
						continue
					} else {
						// check if the event matches the desired filter, ignore otherwise
						if !sub.Filters.Match(env.Event) {
							continue
						}
						// check signature, ignore invalid, except from trusted (AssumeValid) relays
						if !r.AssumeValid {
							if ok, err = env.Event.Verify(); !ok {
								log.E.F(
									"{%s} bad signature on %s\n", r.URL,
									env.Event,
								)
								continue
							}
						}
						// dispatch this to the internal .events channel of the subscription
						sub.dispatchEvent(env.Event)
					}
				case eoseenvelope.L:
					env := eoseenvelope.New()
					if env, message, err = eoseenvelope.Parse(message); chk.E(err) {
						continue
					}
					if subscription, ok := r.loadSub(string(env.Subscription)); ok {
						subscription.dispatchEose()
					}
				case closedenvelope.L:
					env := closedenvelope.New()
					if env, message, err = closedenvelope.Parse(message); chk.E(err) {
						continue
					}
					if subscription, ok := r.loadSub(string(env.Subscription)); ok {
						subscription.handleClosed(env.ReasonString())
					}
				case okenvelope.L:
					env := okenvelope.New()
					if env, message, err = okenvelope.Parse(message); chk.E(err) {
						continue
					}
					eventIDHex := hex.Enc(env.EventID)
					if okCallback, exist := r.loadOkCallback(eventIDHex); exist {
						okCallback(env.OK, env.ReasonString())
					}
				}
			}
		}
	}()

	return nil
}

// Write queues an arbitrary message to be sent to the relay.
func (r *Client) Write(msg []byte) <-chan error {
	ch := make(chan error)
	select {
	case r.writeQueue <- writeRequest{msg: msg, answer: ch}:
	case <-r.connectionContext.Done():
		go func() { ch <- fmt.Errorf("connection closed") }()
	}
	return ch
}

// Publish sends an "EVENT" command to the relay r as in NIP-01 and waits for an OK response.
func (r *Client) Publish(ctx context.Context, ev *event.E) error {
	return r.publish(
		ctx, hex.Enc(ev.ID), eventenvelope.NewSubmissionWith(ev),
	)
}

// Auth sends an "AUTH" command client->relay as in NIP-42 and waits for an OK response.
//
// You don't have to build the AUTH event yourself, this function takes a function to which the
// event that must be signed will be passed, so it's only necessary to sign that.
func (r *Client) Auth(
	ctx context.Context, sign signer.I,
) (err error) {
	authEvent := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.ClientAuthentication.K,
		Tags: tag.NewS(
			tag.NewFromAny("relay", r.URL),
			tag.NewFromAny("challenge", r.challenge),
		),
		Content: nil,
	}
	if err = authEvent.Sign(sign); err != nil {
		return fmt.Errorf("error signing auth event: %w", err)
	}

	return r.publish(
		ctx, hex.Enc(authEvent.ID), authenvelope.NewResponseWith(authEvent),
	)
}

func (r *Client) publish(
	ctx context.Context, id string, env codec.Envelope,
) error {
	var err error
	var cancel context.CancelFunc

	if _, ok := ctx.Deadline(); !ok {
		// if no timeout is set, force it to 7 seconds
		ctx, cancel = context.WithTimeoutCause(
			ctx, 7*time.Second, fmt.Errorf("given up waiting for an OK"),
		)
		defer cancel()
	} else {
		// otherwise make the context cancellable so we can stop everything upon receiving an "OK"
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	// listen for an OK callback
	gotOk := false
	r.storeOkCallback(
		id, func(ok bool, reason string) {
			gotOk = true
			if !ok {
				err = fmt.Errorf("msg: %s", reason)
			}
			cancel()
		},
	)
	defer r.deleteOkCallback(id)

	// publish event
	envb := env.Marshal(nil)
	if err = <-r.Write(envb); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			// this will be called when we get an OK or when the context has been canceled
			if gotOk {
				return err
			}
			return ctx.Err()
		case <-r.connectionContext.Done():
			// this is caused when we lose connectivity
			return err
		}
	}
}

// Subscribe sends a "REQ" command to the relay r as in NIP-01.
// Events are returned through the channel sub.Events.
// The subscription is closed when context ctx is cancelled ("CLOSE" in NIP-01).
//
// Remember to cancel subscriptions, either by calling `.Unsub()` on them or ensuring their `context.Context` will be canceled at some point.
// Failure to do that will result in a huge number of halted goroutines being created.
func (r *Client) Subscribe(
	ctx context.Context, ff *filter.S, opts ...SubscriptionOption,
) (*Subscription, error) {
	sub := r.PrepareSubscription(ctx, ff, opts...)

	if r.Connection == nil {
		log.T.F(
			"WS.Subscribe: not connected to %s; aborting sub id=%s", r.URL,
			sub.GetID(),
		)
		return nil, fmt.Errorf("not connected to %s", r.URL)
	}

	log.T.F(
		"WS.Subscribe: firing subscription id=%s to %s with %d filters",
		sub.GetID(), r.URL, len(*ff),
	)
	if err := sub.Fire(); err != nil {
		log.T.F(
			"WS.Subscribe: Fire failed id=%s to %s: %v", sub.GetID(), r.URL,
			err,
		)
		return nil, fmt.Errorf(
			"couldn't subscribe to %v at %s: %w", ff, r.URL, err,
		)
	}
	log.T.F("WS.Subscribe: Fire succeeded id=%s to %s", sub.GetID(), r.URL)

	return sub, nil
}

// PrepareSubscription creates a subscription, but doesn't fire it.
//
// Remember to cancel subscriptions, either by calling `.Unsub()` on them or ensuring their `context.Context` will be canceled at some point.
// Failure to do that will result in a huge number of halted goroutines being created.
func (r *Client) PrepareSubscription(
	ctx context.Context, ff *filter.S, opts ...SubscriptionOption,
) (sub *Subscription) {
	current := <-subscriptionIDCh
	ctx, cancel := context.WithCancelCause(ctx)
	sub = &Subscription{
		Client:            r,
		Context:           ctx,
		cancel:            cancel,
		counter:           current,
		Events:            make(event.C),
		EndOfStoredEvents: make(chan struct{}, 1),
		ClosedReason:      make(chan string, 1),
		Filters:           ff,
		match:             ff.Match,
		dispatchEventCh:   make(chan subDispatchEventReq, 128),
		dispatchEoseCh:    make(chan subDispatchEoseReq, 1),
		handleClosedCh:    make(chan subHandleClosedReq, 1),
		unsubCh:           make(chan subUnsubReq, 1),
		fireCh:            make(chan subFireReq),
		done:              make(chan struct{}),
	}
	label := ""
	for _, opt := range opts {
		switch o := opt.(type) {
		case WithLabel:
			label = string(o)
		case WithCheckDuplicate:
			sub.checkDuplicate = o
		case WithCheckDuplicateReplaceable:
			sub.checkDuplicateReplaceable = o
		}
	}
	// subscription id computation - must copy since sub.id outlives this function
	buf := subIdPool.Get().([]byte)[:0]
	buf = strconv.AppendInt(buf, sub.counter, 10)
	buf = append(buf, ':')
	buf = append(buf, label...)
	sub.id = make([]byte, len(buf))
	copy(sub.id, buf)
	subIdPool.Put(buf)
	r.storeSub(string(buf), sub)
	// start handling events, eose, unsub etc:
	go sub.start()
	return sub
}

// QueryEvents subscribes to events matching the given filter and returns a channel of events.
//
// In most cases it's better to use SimplePool instead of this method.
func (r *Client) QueryEvents(
	ctx context.Context, f *filter.F, opts ...SubscriptionOption,
) (
	evc event.C, err error,
) {
	var sub *Subscription
	if sub, err = r.Subscribe(ctx, filter.NewS(f), opts...); chk.E(err) {
		return
	}
	go func() {
		for {
			select {
			case <-sub.ClosedReason:
			case <-sub.EndOfStoredEvents:
			case <-ctx.Done():
			case <-r.Context().Done():
			}
			sub.unsub(errors.New("QueryEvents() ended"))
			return
		}
	}()
	evc = sub.Events
	return
}

// QuerySync subscribes to events matching the given filter and returns a slice of events.
// This method blocks until all events are received or the context is canceled.
//
// In most cases it's better to use SimplePool instead of this method.
func (r *Client) QuerySync(
	ctx context.Context, ff *filter.F, opts ...SubscriptionOption,
) (
	[]*event.E, error,
) {
	if _, ok := ctx.Deadline(); !ok {
		// if no timeout is set, force it to 7 seconds
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(
			ctx, 7*time.Second, errors.New("QuerySync() took too long"),
		)
		defer cancel()
	}
	var lim int
	if ff.Limit != nil {
		lim = int(*ff.Limit)
	}
	events := make([]*event.E, 0, max(lim, 250))
	ch, err := r.QueryEvents(ctx, ff, opts...)
	if err != nil {
		return nil, err
	}

	for evt := range ch {
		events = append(events, evt)
	}

	return events, nil
}

// Close closes the relay connection.
func (r *Client) Close() error { return r.CloseWithReason(errors.New("Close() called")) }

// CloseWithReason closes the relay connection with a specific reason that will be stored as the context cancel cause.
func (r *Client) CloseWithReason(reason error) error {
	req := closeReq{reason: reason, resp: make(chan error, 1)}
	r.closeCh <- req
	return <-req.resp
}

var subIdPool = sync.Pool{
	New: func() any { return make([]byte, 0, 15) },
}
