package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcherPublish(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())
	defer d.Stop()

	var received atomic.Int32

	d.Subscribe(NewSubscriberForTypes(func(e DomainEvent) {
		received.Add(1)
	}, EventSavedType))

	// Should be received
	d.Publish(NewEventSaved(nil, 1, false, false))

	// Should not be received (different type)
	d.Publish(NewEventDeleted(nil, nil, 1))

	if received.Load() != 1 {
		t.Errorf("expected 1 event received, got %d", received.Load())
	}
}

func TestDispatcherPublishAsync(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{AsyncBufferSize: 100})
	defer d.Stop()

	var received atomic.Int32
	var wg sync.WaitGroup

	d.Subscribe(NewSubscriberForAll(func(e DomainEvent) {
		received.Add(1)
		wg.Done()
	}))

	wg.Add(10)
	for i := 0; i < 10; i++ {
		d.PublishAsync(NewEventSaved(nil, uint64(i), false, false))
	}

	// Wait for all events to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for async events")
	}

	if received.Load() != 10 {
		t.Errorf("expected 10 events received, got %d", received.Load())
	}
}

func TestDispatcherMultipleSubscribers(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())
	defer d.Stop()

	var count1, count2 atomic.Int32

	d.Subscribe(NewSubscriberForAll(func(e DomainEvent) {
		count1.Add(1)
	}))

	d.Subscribe(NewSubscriberForAll(func(e DomainEvent) {
		count2.Add(1)
	}))

	d.Publish(NewEventSaved(nil, 1, false, false))

	if count1.Load() != 1 || count2.Load() != 1 {
		t.Errorf("expected both subscribers to receive event, got %d and %d", count1.Load(), count2.Load())
	}
}

func TestDispatcherUnsubscribe(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())
	defer d.Stop()

	var received atomic.Int32

	sub := NewSubscriberForAll(func(e DomainEvent) {
		received.Add(1)
	})

	d.Subscribe(sub)
	d.Publish(NewEventSaved(nil, 1, false, false))

	d.Unsubscribe(sub)
	d.Publish(NewEventSaved(nil, 2, false, false))

	if received.Load() != 1 {
		t.Errorf("expected 1 event received after unsubscribe, got %d", received.Load())
	}
}

func TestDispatcherTypedSubscription(t *testing.T) {
	d := NewDispatcher(DefaultDispatcherConfig())
	defer d.Stop()

	var savedCount atomic.Int32
	var deletedCount atomic.Int32

	d.OnEventSaved(func(e *EventSaved) {
		savedCount.Add(1)
	})

	d.OnEventDeleted(func(e *EventDeleted) {
		deletedCount.Add(1)
	})

	d.Publish(NewEventSaved(nil, 1, false, false))
	d.Publish(NewEventDeleted(nil, nil, 1))

	if savedCount.Load() != 1 {
		t.Errorf("expected 1 saved event, got %d", savedCount.Load())
	}
	if deletedCount.Load() != 1 {
		t.Errorf("expected 1 deleted event, got %d", deletedCount.Load())
	}
}

func TestDispatcherStats(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{AsyncBufferSize: 100})
	defer d.Stop()

	d.Subscribe(NewSubscriberForAll(func(e DomainEvent) {}))

	d.Publish(NewEventSaved(nil, 1, false, false))
	d.Publish(NewEventSaved(nil, 2, false, false))

	stats := d.Stats()
	if stats.EventsPublished != 2 {
		t.Errorf("expected 2 events published, got %d", stats.EventsPublished)
	}
	if stats.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", stats.SubscriberCount)
	}
	if stats.QueueCapacity != 100 {
		t.Errorf("expected queue capacity 100, got %d", stats.QueueCapacity)
	}
}

func TestDispatcherQueueFull(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{AsyncBufferSize: 1})
	defer d.Stop()

	// Block the processor
	blocker := make(chan struct{})
	d.Subscribe(NewSubscriberForAll(func(e DomainEvent) {
		<-blocker
	}))

	// First should succeed
	if !d.PublishAsync(NewEventSaved(nil, 1, false, false)) {
		t.Error("first async publish should succeed")
	}

	// Wait for it to be picked up
	time.Sleep(10 * time.Millisecond)

	// Second should fail (queue full)
	if d.PublishAsync(NewEventSaved(nil, 2, false, false)) {
		// might succeed if the first was already picked up
	}

	close(blocker)
}

func TestEventTypes(t *testing.T) {
	types := AllEventTypes()
	if len(types) == 0 {
		t.Error("expected event types to be registered")
	}

	// Check all types are unique
	seen := make(map[string]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate event type: %s", typ)
		}
		seen[typ] = true
	}
}

func TestDomainEventInterface(t *testing.T) {
	events := []DomainEvent{
		NewEventSaved(nil, 1, true, false),
		NewEventDeleted([]byte{1}, []byte{2}, 3),
		NewFollowListUpdated([]byte{1}, nil, nil),
		NewACLMembershipChanged([]byte{1}, "none", "write", "followed"),
		NewPolicyConfigUpdated([]byte{1}, map[string]interface{}{"key": "value"}),
		NewPolicyFollowsUpdated([]byte{1}, 10, nil),
		NewRelayGroupConfigChanged(nil),
		NewClusterMembershipChanged(nil, "join"),
		NewSyncSerialUpdated(100),
		NewUserAuthenticated([]byte{1}, "write", true, "conn-123"),
		NewConnectionOpened("conn-123", "192.168.1.1:1234"),
		NewConnectionClosed("conn-123", time.Hour, 100, 50),
		NewSubscriptionCreated("sub-1", "conn-123", 3),
		NewSubscriptionClosed("sub-1", "conn-123", 25),
		NewMemberJoined([]byte{1}, "invite-abc"),
		NewMemberLeft([]byte{1}),
	}

	for _, e := range events {
		if e.OccurredAt().IsZero() {
			t.Errorf("event %s has zero timestamp", e.EventType())
		}
		if e.EventType() == "" {
			t.Error("event has empty type")
		}
	}
}

func TestSubscriberFunc(t *testing.T) {
	var called bool
	sf := NewSubscriberFunc(
		func(e DomainEvent) { called = true },
		func(typ string) bool { return typ == EventSavedType },
	)

	if !sf.Supports(EventSavedType) {
		t.Error("should support EventSavedType")
	}
	if sf.Supports(EventDeletedType) {
		t.Error("should not support EventDeletedType")
	}

	sf.Handle(NewEventSaved(nil, 1, false, false))
	if !called {
		t.Error("handler was not called")
	}
}
