package events

import (
	"context"
	"sync"
	"sync/atomic"

	"next.orly.dev/pkg/lol/log"
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

// Dispatcher publishes domain events to subscribers.
type Dispatcher struct {
	subscribers []Subscriber
	mu          sync.RWMutex

	asyncChan chan DomainEvent
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Metrics
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

	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		asyncChan:      make(chan DomainEvent, cfg.AsyncBufferSize),
		ctx:            ctx,
		cancel:         cancel,
		asyncQueueSize: cfg.AsyncBufferSize,
	}

	// Start async processor
	d.wg.Add(1)
	go d.processAsync()

	return d
}

// Subscribe adds a subscriber to receive events.
func (d *Dispatcher) Subscribe(s Subscriber) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers = append(d.subscribers, s)
}

// Unsubscribe removes a subscriber.
func (d *Dispatcher) Unsubscribe(s Subscriber) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, sub := range d.subscribers {
		if sub == s {
			d.subscribers = append(d.subscribers[:i], d.subscribers[i+1:]...)
			return
		}
	}
}

// Publish sends an event to all matching subscribers synchronously.
// This blocks until all subscribers have processed the event.
func (d *Dispatcher) Publish(event DomainEvent) {
	d.mu.RLock()
	subscribers := d.subscribers
	d.mu.RUnlock()

	for _, s := range subscribers {
		if s.Supports(event.EventType()) {
			s.Handle(event)
		}
	}
	d.eventsPublished.Add(1)
}

// PublishAsync sends an event to be processed asynchronously.
// This returns immediately and the event is processed in a background goroutine.
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

// processAsync handles async event delivery.
func (d *Dispatcher) processAsync() {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			// Drain remaining events before exiting
			for {
				select {
				case event := <-d.asyncChan:
					d.Publish(event)
				default:
					return
				}
			}
		case event := <-d.asyncChan:
			d.Publish(event)
		}
	}
}

// Stop stops the dispatcher and waits for pending events to be processed.
func (d *Dispatcher) Stop() {
	d.cancel()
	d.wg.Wait()
}

// Stats returns dispatcher statistics.
func (d *Dispatcher) Stats() DispatcherStats {
	d.mu.RLock()
	subscriberCount := len(d.subscribers)
	d.mu.RUnlock()

	return DispatcherStats{
		EventsPublished: d.eventsPublished.Load(),
		EventsDropped:   d.eventsDropped.Load(),
		SubscriberCount: subscriberCount,
		QueueSize:       len(d.asyncChan),
		QueueCapacity:   d.asyncQueueSize,
	}
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
// Global Dispatcher (Optional Convenience)
// =============================================================================

var (
	globalDispatcher     *Dispatcher
	globalDispatcherOnce sync.Once
)

// Global returns the global dispatcher instance.
// Creates one with default config if not already created.
func Global() *Dispatcher {
	globalDispatcherOnce.Do(func() {
		globalDispatcher = NewDispatcher(DefaultDispatcherConfig())
	})
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
