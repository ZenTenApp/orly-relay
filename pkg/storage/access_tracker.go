//go:build !windows

package storage

import (
	"container/list"
	"context"
	"sync"

	"lol.mleku.dev/log"
)

// AccessTrackerDatabase defines the interface for the underlying database
// that stores access tracking information.
type AccessTrackerDatabase interface {
	RecordEventAccess(serial uint64, connectionID string) error
	GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error)
	GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error)
}

// accessKey is the composite key for deduplication: serial + connectionID
type accessKey struct {
	Serial       uint64
	ConnectionID string
}

// AccessTracker tracks event access patterns with session deduplication.
// It maintains an in-memory cache to deduplicate accesses from the same
// connection, reducing database writes while ensuring unique session counting.
type AccessTracker struct {
	db AccessTrackerDatabase

	// Deduplication cache: tracks which (serial, connectionID) pairs
	// have already been recorded in this session window
	mu           sync.RWMutex
	seen         map[accessKey]struct{}
	seenOrder    *list.List // LRU order for eviction
	seenElements map[accessKey]*list.Element
	maxSeen      int // Maximum entries in dedup cache

	// Flush interval for stats
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAccessTracker creates a new access tracker.
// maxSeenEntries controls the size of the deduplication cache.
func NewAccessTracker(db AccessTrackerDatabase, maxSeenEntries int) *AccessTracker {
	if maxSeenEntries <= 0 {
		maxSeenEntries = 100000 // Default: 100k entries
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AccessTracker{
		db:           db,
		seen:         make(map[accessKey]struct{}),
		seenOrder:    list.New(),
		seenElements: make(map[accessKey]*list.Element),
		maxSeen:      maxSeenEntries,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// RecordAccess records an access to an event by a connection.
// Deduplicates accesses from the same connection within the cache window.
// Returns true if this was a new access, false if deduplicated.
func (t *AccessTracker) RecordAccess(serial uint64, connectionID string) (bool, error) {
	key := accessKey{Serial: serial, ConnectionID: connectionID}

	t.mu.Lock()
	// Check if already seen
	if _, exists := t.seen[key]; exists {
		// Move to front (most recent)
		if elem, ok := t.seenElements[key]; ok {
			t.seenOrder.MoveToFront(elem)
		}
		t.mu.Unlock()
		return false, nil // Deduplicated
	}

	// Evict oldest if at capacity
	if len(t.seen) >= t.maxSeen {
		oldest := t.seenOrder.Back()
		if oldest != nil {
			oldKey := oldest.Value.(accessKey)
			delete(t.seen, oldKey)
			delete(t.seenElements, oldKey)
			t.seenOrder.Remove(oldest)
		}
	}

	// Add to cache
	t.seen[key] = struct{}{}
	elem := t.seenOrder.PushFront(key)
	t.seenElements[key] = elem
	t.mu.Unlock()

	// Record to database
	if err := t.db.RecordEventAccess(serial, connectionID); err != nil {
		return true, err
	}

	return true, nil
}

// GetAccessInfo returns the access information for an event.
func (t *AccessTracker) GetAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	return t.db.GetEventAccessInfo(serial)
}

// GetColdestEvents returns event serials sorted by coldness.
// limit: max events to return
// minAgeSec: minimum age in seconds since last access
func (t *AccessTracker) GetColdestEvents(limit int, minAgeSec int64) ([]uint64, error) {
	return t.db.GetLeastAccessedEvents(limit, minAgeSec)
}

// ClearConnection removes all dedup entries for a specific connection.
// Call this when a connection closes to free up cache space.
func (t *AccessTracker) ClearConnection(connectionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Find and remove all entries for this connection
	for key, elem := range t.seenElements {
		if key.ConnectionID == connectionID {
			delete(t.seen, key)
			delete(t.seenElements, key)
			t.seenOrder.Remove(elem)
		}
	}
}

// Stats returns current cache statistics.
func (t *AccessTracker) Stats() AccessTrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return AccessTrackerStats{
		CachedEntries: len(t.seen),
		MaxEntries:    t.maxSeen,
	}
}

// AccessTrackerStats holds access tracker statistics.
type AccessTrackerStats struct {
	CachedEntries int
	MaxEntries    int
}

// Start starts any background goroutines for the tracker.
// Currently a no-op but provided for future use.
func (t *AccessTracker) Start() {
	log.I.F("access tracker started with %d max dedup entries", t.maxSeen)
}

// Stop stops the access tracker and releases resources.
func (t *AccessTracker) Stop() {
	t.cancel()
	log.I.F("access tracker stopped")
}
