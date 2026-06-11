// Package actor provides generic building blocks for CSP/actor-model
// concurrency in Go. Each type maps to a common inter-goroutine
// communication pattern, eliminating the per-actor boilerplate of
// defining request structs, response channels, and caller methods.
//
// The five channel types cover every pattern observed in production
// actor codebases:
//
//   - [Func]   synchronous request -> response  (function call)
//   - [Query]  synchronous void -> response      (getter)
//   - [Proc]   synchronous request -> done        (setter / command)
//   - [Signal] synchronous void -> done           (trigger / reset)
//   - [Inbox]  async fire-and-forget              (logging / notifications)
//
// [Lifecycle] manages the stop/done shutdown protocol.
//
// All synchronous types use unbuffered channels. The caller blocks
// until the actor processes the message. This eliminates the class of
// bugs where a buffered write appears to succeed but the actor hasn't
// seen it yet (the transport manager race pattern).
//
// # Usage
//
// Define an actor struct embedding the channel types it needs:
//
//	type Counter struct {
//	    inc   actor.Signal
//	    get   actor.Query[int]
//	    actor.Lifecycle
//	}
//
// Initialize in the constructor:
//
//	func NewCounter() *Counter {
//	    c := &Counter{
//	        inc:       actor.NewSignal(),
//	        get:       actor.NewQuery[int](),
//	        Lifecycle: actor.NewLifecycle(),
//	    }
//	    actor.Go(c.Lifecycle, c.run)
//	    return c
//	}
//
// Write the actor loop using Recv() in select:
//
//	func (c *Counter) run() {
//	    var n int
//	    for {
//	        select {
//	        case <-c.Stopping():
//	            return
//	        case msg := <-c.inc.Recv():
//	            n++
//	            msg.Done()
//	        case msg := <-c.get.Recv():
//	            msg.Reply(n)
//	        }
//	    }
//	}
//
// Public methods are one-liners:
//
//	func (c *Counter) Inc()    { c.inc.Call() }
//	func (c *Counter) Get() int { return c.get.Call() }
//
// # Design Principles
//
// Every interaction between peers requires independent bidirectional
// channels. Simplex+lock is strictly more complex and introduces
// deadlock risk.
//
// Backpressure is expressed by channel state, not by blocking the
// sender arbitrarily. Neither side should be able to freeze the other
// - use [Inbox] with a bounded buffer when the sender must not block.
//
// State ownership stays with the actor goroutine. Short-lived callers
// get copies (via response channels), not references to internal state.
// Death of a worker is reported via [Lifecycle], not hidden or
// auto-recovered.
package actor

import "context"

// ---------------------------------------------------------------------------
// Func: synchronous request -> response
// ---------------------------------------------------------------------------

// Func is a synchronous function call channel. The caller sends a
// request of type Req and blocks until the actor replies with Resp.
type Func[Req, Resp any] struct {
	ch chan FuncMsg[Req, Resp]
}

// FuncMsg is the message received by the actor from a [Func] channel.
// Access the request via the Req field; send the response via Reply.
type FuncMsg[Req, Resp any] struct {
	Req  Req
	resp chan Resp
}

// NewFunc creates a Func channel (unbuffered).
func NewFunc[Req, Resp any]() Func[Req, Resp] {
	return Func[Req, Resp]{ch: make(chan FuncMsg[Req, Resp])}
}

// Call sends req to the actor and blocks for the response.
func (f Func[Req, Resp]) Call(req Req) Resp {
	resp := make(chan Resp, 1)
	f.ch <- FuncMsg[Req, Resp]{Req: req, resp: resp}
	return <-resp
}

// CallCtx sends req to the actor and blocks for the response, but
// abandons the call if ctx is cancelled. Returns ctx.Err() on cancel.
// If the actor has already received the request when the caller bails,
// the actor's Reply writes to a buffered channel that gets GC'd.
func (f Func[Req, Resp]) CallCtx(ctx context.Context, req Req) (Resp, error) {
	resp := make(chan Resp, 1)
	select {
	case f.ch <- FuncMsg[Req, Resp]{Req: req, resp: resp}:
	case <-ctx.Done():
		var zero Resp
		return zero, ctx.Err()
	}
	select {
	case r := <-resp:
		return r, nil
	case <-ctx.Done():
		var zero Resp
		return zero, ctx.Err()
	}
}

// Recv returns the receive-only channel for use in select.
func (f Func[Req, Resp]) Recv() <-chan FuncMsg[Req, Resp] {
	return f.ch
}

// Reply sends the response back to the caller.
// Must be called exactly once per received message.
func (m FuncMsg[Req, Resp]) Reply(r Resp) {
	m.resp <- r
}

// ---------------------------------------------------------------------------
// Query: synchronous void -> response
// ---------------------------------------------------------------------------

// Query is a synchronous getter channel. The caller sends no data
// and blocks until the actor replies with Resp.
type Query[Resp any] struct {
	ch chan QueryMsg[Resp]
}

// QueryMsg is the message received by the actor from a [Query] channel.
type QueryMsg[Resp any] struct {
	resp chan Resp
}

// NewQuery creates a Query channel (unbuffered).
func NewQuery[Resp any]() Query[Resp] {
	return Query[Resp]{ch: make(chan QueryMsg[Resp])}
}

// Call sends a query and blocks for the response.
func (q Query[Resp]) Call() Resp {
	resp := make(chan Resp, 1)
	q.ch <- QueryMsg[Resp]{resp: resp}
	return <-resp
}

// CallCtx sends a query and blocks for the response, but abandons
// the call if ctx is cancelled.
func (q Query[Resp]) CallCtx(ctx context.Context) (Resp, error) {
	resp := make(chan Resp, 1)
	select {
	case q.ch <- QueryMsg[Resp]{resp: resp}:
	case <-ctx.Done():
		var zero Resp
		return zero, ctx.Err()
	}
	select {
	case r := <-resp:
		return r, nil
	case <-ctx.Done():
		var zero Resp
		return zero, ctx.Err()
	}
}

// Recv returns the receive-only channel for use in select.
func (q Query[Resp]) Recv() <-chan QueryMsg[Resp] {
	return q.ch
}

// Reply sends the response back to the caller.
func (m QueryMsg[Resp]) Reply(r Resp) {
	m.resp <- r
}

// ---------------------------------------------------------------------------
// Proc: synchronous request -> done
// ---------------------------------------------------------------------------

// Proc is a synchronous command channel. The caller sends a request
// and blocks until the actor confirms it has been processed.
type Proc[Req any] struct {
	ch chan ProcMsg[Req]
}

// ProcMsg is the message received by the actor from a [Proc] channel.
// Access the request via the Req field; signal completion via Done.
type ProcMsg[Req any] struct {
	Req  Req
	done chan struct{}
}

// NewProc creates a Proc channel (unbuffered).
func NewProc[Req any]() Proc[Req] {
	return Proc[Req]{ch: make(chan ProcMsg[Req])}
}

// Call sends req to the actor and blocks until it is processed.
func (p Proc[Req]) Call(req Req) {
	done := make(chan struct{})
	p.ch <- ProcMsg[Req]{Req: req, done: done}
	<-done
}

// CallCtx sends req to the actor and blocks until processed, but
// abandons the call if ctx is cancelled.
func (p Proc[Req]) CallCtx(ctx context.Context, req Req) error {
	done := make(chan struct{})
	select {
	case p.ch <- ProcMsg[Req]{Req: req, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Recv returns the receive-only channel for use in select.
func (p Proc[Req]) Recv() <-chan ProcMsg[Req] {
	return p.ch
}

// Done signals the caller that the request has been processed.
// Must be called exactly once per received message.
func (m ProcMsg[Req]) Done() {
	close(m.done)
}

// ---------------------------------------------------------------------------
// Signal: synchronous void -> done
// ---------------------------------------------------------------------------

// Signal is a synchronous trigger channel. The caller sends no data
// and blocks until the actor confirms it has handled the signal.
type Signal struct {
	ch chan SignalMsg
}

// SignalMsg is the message received by the actor from a [Signal] channel.
type SignalMsg struct {
	done chan struct{}
}

// NewSignal creates a Signal channel (unbuffered).
func NewSignal() Signal {
	return Signal{ch: make(chan SignalMsg)}
}

// Call sends the signal and blocks until it is processed.
func (s Signal) Call() {
	done := make(chan struct{})
	s.ch <- SignalMsg{done: done}
	<-done
}

// CallCtx sends the signal and blocks until processed, but abandons
// the call if ctx is cancelled.
func (s Signal) CallCtx(ctx context.Context) error {
	done := make(chan struct{})
	select {
	case s.ch <- SignalMsg{done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Recv returns the receive-only channel for use in select.
func (s Signal) Recv() <-chan SignalMsg {
	return s.ch
}

// Done signals the caller that the signal has been handled.
func (m SignalMsg) Done() {
	close(m.done)
}

// ---------------------------------------------------------------------------
// Inbox: async fire-and-forget
// ---------------------------------------------------------------------------

// Inbox is a buffered channel for fire-and-forget messages.
// Senders do not wait for the actor to process the message.
// Use for logging, metrics, and other non-critical notifications.
type Inbox[T any] struct {
	ch chan T
}

// NewInbox creates an Inbox with the given buffer size.
// A buffer of 0 creates an unbuffered (synchronous) inbox.
func NewInbox[T any](buffer int) Inbox[T] {
	if buffer < 0 {
		buffer = 0
	}
	return Inbox[T]{ch: make(chan T, buffer)}
}

// Send enqueues a message. Blocks if the buffer is full.
func (i Inbox[T]) Send(v T) {
	i.ch <- v
}

// TrySend attempts to enqueue a message without blocking.
// Returns false if the buffer is full (message dropped).
func (i Inbox[T]) TrySend(v T) bool {
	select {
	case i.ch <- v:
		return true
	default:
		return false
	}
}

// Recv returns the receive-only channel for use in select.
func (i Inbox[T]) Recv() <-chan T {
	return i.ch
}

// ---------------------------------------------------------------------------
// Lifecycle: stop/done shutdown protocol
// ---------------------------------------------------------------------------

// Lifecycle manages the shutdown handshake between an actor and its
// owner. The owner calls Stop to request shutdown and wait for
// completion. The actor calls MarkDone (typically via defer) when it
// has finished.
type Lifecycle struct {
	stop chan struct{}
	done chan struct{}
}

// NewLifecycle creates an initialized Lifecycle.
func NewLifecycle() Lifecycle {
	return Lifecycle{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Stop signals the actor to stop and blocks until it finishes.
// Must be called at most once.
func (l Lifecycle) Stop() {
	close(l.stop)
	<-l.done
}

// Stopping returns a channel that closes when Stop is called.
// Use in select to detect shutdown requests.
func (l Lifecycle) Stopping() <-chan struct{} {
	return l.stop
}

// Stopped returns a channel that closes when the actor finishes.
// Useful for external observers waiting on actor completion.
func (l Lifecycle) Stopped() <-chan struct{} {
	return l.done
}

// MarkDone signals that the actor goroutine has finished.
// Call via defer at the top of the actor function.
func (l Lifecycle) MarkDone() {
	close(l.done)
}

// Go starts fn in a new goroutine and automatically calls
// l.MarkDone when fn returns.
func Go(l Lifecycle, fn func()) {
	go func() {
		defer l.MarkDone()
		fn()
	}()
}
