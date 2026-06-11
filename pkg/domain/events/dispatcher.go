package events

import (
	"sync/atomic"

	"git.smesh.lol/orly/pkg/lol/log"
)

// Subscriber handles domain events.
type Subscriber interface {
	// Handle processes the given domain event.
	Handle(event DomainEvent)
	// Supports returns true if this subscriber handles the given event type.
	Supports(eventType string) bool
}

// SubscriberFunc is a function that can be used as a Subscriber.
type SubscriberFunc struct {
	handleFunc   func(DomainEvent)
	supportsFunc func(string) bool
}

// Handle implements Subscriber.
func (s *SubscriberFunc) Handle(event DomainEvent) {
	s.handleFunc(event)
}

// Supports implements Subscriber.
func (s *SubscriberFunc) Supports(eventType string) bool {
	return s.supportsFunc(eventType)
}

// NewSubscriberFunc creates a new SubscriberFunc.
func NewSubscriberFunc(handle func(DomainEvent), supports func(string) bool) *SubscriberFunc {
	return &SubscriberFunc{
		handleFunc:   handle,
		supportsFunc: supports,
	}
}

// NewSubscriberForTypes creates a subscriber that handles specific event types.
func NewSubscriberForTypes(handle func(DomainEvent), types ...string) *SubscriberFunc {
	typeSet := make(map[string]struct{}, len(types))
	for _, t := range types {
		typeSet[t] = struct{}{}
	}
	return &SubscriberFunc{
		handleFunc: handle,
		supportsFunc: func(eventType string) bool {
			_, ok := typeSet[eventType]
			return ok
		},
	}
}

// NewSubscriberForAll creates a subscriber that handles all event types.
func NewSubscriberForAll(handle func(DomainEvent)) *SubscriberFunc {
	return &SubscriberFunc{
		handleFunc:   handle,
		supportsFunc: func(string) bool { return true },
	}
}

// --- Actor request/response types ---

type dispSubscribeReq struct {
	s    Subscriber
	done chan struct{}
}

type dispUnsubscribeReq struct {
	s    Subscriber
	done chan struct{}
}

type dispPublishReq struct {
	event DomainEvent
	done  chan struct{}
}

type dispStatsReq struct {
	resp chan DispatcherStats
}

// Dispatcher publishes domain events to subscribers.
// All mutable state is owned by the actor goroutine.
type Dispatcher struct {
	subscribeCh   chan dispSubscribeReq
	unsubscribeCh chan dispUnsubscribeReq
	publishCh     chan dispPublishReq
	asyncChan     chan DomainEvent
	statsCh       chan dispStatsReq
	stop          chan struct{}
	done          chan struct{}

	// Metrics (atomics are safe for concurrent reads from Stats while actor updates)
	eventsPublished atomic.Int64
	eventsDropped   atomic.Int64
	asyncQueueSize  int
}

// DispatcherConfig configures the dispatcher.
type DispatcherConfig struct {
	// AsyncBufferSize is the buffer size for async event delivery.
	// Default: 1000
	AsyncBufferSize int
}

// DefaultDispatcherConfig returns the default dispatcher configuration.
func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		AsyncBufferSize: 1000,
	}
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	if cfg.AsyncBufferSize <= 0 {
		cfg.AsyncBufferSize = 1000
	}

	d := &Dispatcher{
		subscribeCh:   make(chan dispSubscribeReq),
		unsubscribeCh: make(chan dispUnsubscribeReq),
		publishCh:     make(chan dispPublishReq),
		asyncChan:     make(chan DomainEvent, cfg.AsyncBufferSize),
		statsCh:       make(chan dispStatsReq),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		asyncQueueSize: cfg.AsyncBufferSize,
	}

	go d.actorLoop()

	return d
}

// actorLoop owns the subscribers list and processes all requests.
func (d *Dispatcher) actorLoop() {
	defer close(d.done)

	var subscribers []Subscriber

	deliver := func(event DomainEvent) {
		for _, s := range subscribers {
			if s.Supports(event.EventType()) {
				s.Handle(event)
			}
		}
		d.eventsPublished.Add(1)
	}

	for {
		select {
		case <-d.stop:
			// Drain remaining async events before exiting
			for {
				select {
				case event := <-d.asyncChan:
					deliver(event)
				default:
					return
				}
			}
		case req := <-d.subscribeCh:
			subscribers = append(subscribers, req.s)
			close(req.done)
		case req := <-d.unsubscribeCh:
			for i, sub := range subscribers {
				if sub == req.s {
					subscribers = append(subscribers[:i], subscribers[i+1:]...)
					break
				}
			}
			close(req.done)
		case req := <-d.publishCh:
			deliver(req.event)
			close(req.done)
		case event := <-d.asyncChan:
			deliver(event)
		case req := <-d.statsCh:
			req.resp <- DispatcherStats{
				EventsPublished: d.eventsPublished.Load(),
				EventsDropped:   d.eventsDropped.Load(),
				SubscriberCount: len(subscribers),
				QueueSize:       len(d.asyncChan),
				QueueCapacity:   d.asyncQueueSize,
			}
		}
	}
}

// Subscribe adds a subscriber to receive events.
func (d *Dispatcher) Subscribe(s Subscriber) {
	done := make(chan struct{})
	d.subscribeCh <- dispSubscribeReq{s: s, done: done}
	<-done
}

// Unsubscribe removes a subscriber.
func (d *Dispatcher) Unsubscribe(s Subscriber) {
	done := make(chan struct{})
	d.unsubscribeCh <- dispUnsubscribeReq{s: s, done: done}
	<-done
}

// Publish sends an event to all matching subscribers synchronously.
func (d *Dispatcher) Publish(event DomainEvent) {
	done := make(chan struct{})
	d.publishCh <- dispPublishReq{event: event, done: done}
	<-done
}

// PublishAsync sends an event to be processed asynchronously.
// Returns true if the event was queued, false if the queue is full.
func (d *Dispatcher) PublishAsync(event DomainEvent) bool {
	select {
	case d.asyncChan <- event:
		return true
	default:
		d.eventsDropped.Add(1)
		log.W.F("domain event dropped (queue full): %s", event.EventType())
		return false
	}
}

// Stop stops the dispatcher and waits for pending events to be processed.
func (d *Dispatcher) Stop() {
	close(d.stop)
	<-d.done
}

// Stats returns dispatcher statistics.
func (d *Dispatcher) Stats() DispatcherStats {
	req := dispStatsReq{resp: make(chan DispatcherStats, 1)}
	d.statsCh <- req
	return <-req.resp
}

// DispatcherStats contains dispatcher statistics.
type DispatcherStats struct {
	EventsPublished int64
	EventsDropped   int64
	SubscriberCount int
	QueueSize       int
	QueueCapacity   int
}

// =============================================================================
// Global Dispatcher
// =============================================================================

var globalDispatcher *Dispatcher

func init() {
	globalDispatcher = NewDispatcher(DefaultDispatcherConfig())
}

// Global returns the global dispatcher instance.
func Global() *Dispatcher {
	return globalDispatcher
}

// SetGlobal sets the global dispatcher instance.
// This should be called early in application startup if custom config is needed.
func SetGlobal(d *Dispatcher) {
	globalDispatcher = d
}

// =============================================================================
// Typed Subscription Helpers
// =============================================================================

// OnEventSaved subscribes to EventSaved events.
func (d *Dispatcher) OnEventSaved(handler func(*EventSaved)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if es, ok := e.(*EventSaved); ok {
			handler(es)
		}
	}, EventSavedType))
}

// OnEventDeleted subscribes to EventDeleted events.
func (d *Dispatcher) OnEventDeleted(handler func(*EventDeleted)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if ed, ok := e.(*EventDeleted); ok {
			handler(ed)
		}
	}, EventDeletedType))
}

// OnFollowListUpdated subscribes to FollowListUpdated events.
func (d *Dispatcher) OnFollowListUpdated(handler func(*FollowListUpdated)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if flu, ok := e.(*FollowListUpdated); ok {
			handler(flu)
		}
	}, FollowListUpdatedType))
}

// OnACLMembershipChanged subscribes to ACLMembershipChanged events.
func (d *Dispatcher) OnACLMembershipChanged(handler func(*ACLMembershipChanged)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if amc, ok := e.(*ACLMembershipChanged); ok {
			handler(amc)
		}
	}, ACLMembershipChangedType))
}

// OnPolicyConfigUpdated subscribes to PolicyConfigUpdated events.
func (d *Dispatcher) OnPolicyConfigUpdated(handler func(*PolicyConfigUpdated)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if pcu, ok := e.(*PolicyConfigUpdated); ok {
			handler(pcu)
		}
	}, PolicyConfigUpdatedType))
}

// OnUserAuthenticated subscribes to UserAuthenticated events.
func (d *Dispatcher) OnUserAuthenticated(handler func(*UserAuthenticated)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if ua, ok := e.(*UserAuthenticated); ok {
			handler(ua)
		}
	}, UserAuthenticatedType))
}

// OnMemberJoined subscribes to MemberJoined events.
func (d *Dispatcher) OnMemberJoined(handler func(*MemberJoined)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if mj, ok := e.(*MemberJoined); ok {
			handler(mj)
		}
	}, MemberJoinedType))
}

// OnMemberLeft subscribes to MemberLeft events.
func (d *Dispatcher) OnMemberLeft(handler func(*MemberLeft)) {
	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		if ml, ok := e.(*MemberLeft); ok {
			handler(ml)
		}
	}, MemberLeftType))
}
