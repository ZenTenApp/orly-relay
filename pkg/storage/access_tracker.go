//go:build !windows

package storage

import (
	"container/list"
	"context"
	"sort"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database/indexes/types"
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

type recordAccessArgs struct {
	Serial       uint64
	ConnectionID string
}

type recordAccessResp struct {
	isNew bool
	err   error
}

// AccessTracker tracks event access patterns with session deduplication.
// It maintains an in-memory cache to deduplicate accesses from the same
// connection, reducing database writes while ensuring unique session counting.
// All mutable state is owned by the actor goroutine.
type AccessTracker struct {
	db AccessTrackerDatabase

	recordAccess    actor.Func[recordAccessArgs, recordAccessResp]
	clearConnection actor.Proc[string]
	stats           actor.Query[AccessTrackerStats]
	lc              actor.Lifecycle

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

	t := &AccessTracker{
		db:              db,
		recordAccess:    actor.NewFunc[recordAccessArgs, recordAccessResp](),
		clearConnection: actor.NewProc[string](),
		stats:           actor.NewQuery[AccessTrackerStats](),
		lc:              actor.NewLifecycle(),
		ctx:             ctx,
		cancel:          cancel,
	}

	actor.Go(t.lc, func() { t.actorLoop(maxSeenEntries) })
	return t
}

// actorLoop owns all dedup cache state.
func (t *AccessTracker) actorLoop(maxSeen int) {
	seen := make(map[accessKey]struct{})
	seenOrder := list.New()
	seenElements := make(map[accessKey]*list.Element)

	for {
		select {
		case <-t.lc.Stopping():
			return

		case msg := <-t.recordAccess.Recv():
			key := accessKey{Serial: msg.Req.Serial, ConnectionID: msg.Req.ConnectionID}

			// Check if already seen
			if _, exists := seen[key]; exists {
				if elem, ok := seenElements[key]; ok {
					seenOrder.MoveToFront(elem)
				}
				msg.Reply(recordAccessResp{isNew: false, err: nil})
				continue
			}

			// Evict oldest if at capacity
			if len(seen) >= maxSeen {
				oldest := seenOrder.Back()
				if oldest != nil {
					oldKey := oldest.Value.(accessKey)
					delete(seen, oldKey)
					delete(seenElements, oldKey)
					seenOrder.Remove(oldest)
				}
			}

			// Add to cache
			seen[key] = struct{}{}
			elem := seenOrder.PushFront(key)
			seenElements[key] = elem

			// Record to database (done inside actor to keep ordering)
			err := t.db.RecordEventAccess(msg.Req.Serial, msg.Req.ConnectionID)
			msg.Reply(recordAccessResp{isNew: true, err: err})

		case msg := <-t.clearConnection.Recv():
			for key, elem := range seenElements {
				if key.ConnectionID == msg.Req {
					delete(seen, key)
					delete(seenElements, key)
					seenOrder.Remove(elem)
				}
			}
			msg.Done()

		case msg := <-t.stats.Recv():
			msg.Reply(AccessTrackerStats{
				CachedEntries: len(seen),
				MaxEntries:    maxSeen,
			})
		}
	}
}

// RecordAccess records an access to an event by a connection.
// Deduplicates accesses from the same connection within the cache window.
// Returns true if this was a new access, false if deduplicated.
func (t *AccessTracker) RecordAccess(serial uint64, connectionID string) (bool, error) {
	r := t.recordAccess.Call(recordAccessArgs{Serial: serial, ConnectionID: connectionID})
	return r.isNew, r.err
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

// GetColdestEventsWithWoT returns event serials sorted by WoT-weighted coldness.
// It overfetches candidates from the database, resolves each event's author to a
// WoT depth, applies a depth bonus to the coldness score, filters out immune kinds,
// re-sorts, and returns the coldest `limit` events.
//
// candidateMultiplier controls overfetch ratio (e.g., 5 = fetch 5x limit candidates).
func (t *AccessTracker) GetColdestEventsWithWoT(
	limit, candidateMultiplier int,
	minAgeSec int64,
	wot WoTProvider,
	authorDB AuthorLookup,
) ([]uint64, error) {
	if candidateMultiplier <= 1 {
		candidateMultiplier = 5
	}

	// Overfetch raw candidates
	candidates, err := t.db.GetLeastAccessedEvents(limit*candidateMultiplier, minAgeSec)
	if err != nil {
		return nil, err
	}

	type scored struct {
		serial uint64
		score  int64
	}

	var results []scored
	for _, serial := range candidates {
		ser := &types.Uint40{}
		if err := ser.Set(serial); err != nil {
			continue
		}

		ev, err := authorDB.FetchEventBySerial(ser)
		if err != nil || ev == nil {
			continue
		}

		// Skip immune kinds
		if isImmuneKind(ev.Kind) {
			continue
		}

		// Get access info for scoring
		lastAccess, accessCount, err := t.db.GetEventAccessInfo(serial)
		if err != nil {
			continue
		}

		// Resolve author WoT depth
		depth := 0
		if pkSerial, err := authorDB.GetPubkeySerial(ev.Pubkey); err == nil {
			depth = wot.GetDepthForGC(pkSerial.Get())
		}

		// WoT-weighted coldness: higher score = warmer = keep longer
		score := lastAccess + int64(accessCount)*3600 + wotBonus(depth)
		results = append(results, scored{serial, score})
	}

	// Sort by score ascending (coldest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].score < results[j].score
	})

	// Return up to limit
	out := make([]uint64, 0, limit)
	for i := 0; i < len(results) && i < limit; i++ {
		out = append(out, results[i].serial)
	}
	return out, nil
}

// ClearConnection removes all dedup entries for a specific connection.
// Call this when a connection closes to free up cache space.
func (t *AccessTracker) ClearConnection(connectionID string) {
	t.clearConnection.Call(connectionID)
}

// Stats returns current cache statistics.
func (t *AccessTracker) Stats() AccessTrackerStats {
	return t.stats.Call()
}

// AccessTrackerStats holds access tracker statistics.
type AccessTrackerStats struct {
	CachedEntries int
	MaxEntries    int
}

// Start starts any background goroutines for the tracker.
// The actor goroutine is started in NewAccessTracker.
func (t *AccessTracker) Start() {
	log.I.F("access tracker started with dedup cache")
}

// Stop stops the access tracker and releases resources.
func (t *AccessTracker) Stop() {
	t.cancel()
	t.lc.Stop()
	log.I.F("access tracker stopped")
}
