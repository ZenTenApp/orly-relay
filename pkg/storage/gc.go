//go:build !windows

package storage

// TODO: IMPORTANT - This GC implementation is EXPERIMENTAL and may cause crashes under high load.
// The current implementation has the following issues that need to be addressed:
//
// 1. Badger race condition: DeleteEventBySerial runs transactions that can trigger
//    "assignment to entry in nil map" panics in Badger v4.8.0 under concurrent load.
//    This happens when GC deletes events while many REQ queries are being processed.
//
// 2. Batch transaction handling: On large datasets (14+ GB), deletions should be:
//    - Serialized or use a transaction pool to prevent concurrent txn issues
//    - Batched with proper delays between batches to avoid overwhelming Badger
//    - Rate-limited based on current system load
//
// 3. The current 10ms delay every 100 events (line ~237) is insufficient for busy relays.
//    Consider adaptive rate limiting based on pending transaction count.
//
// 4. Consider using Badger's WriteBatch API instead of individual Update transactions
//    for bulk deletions, which may be more efficient and avoid some race conditions.

import (
	"context"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// WoTProvider supplies WoT depth information for graph-weighted eviction.
// Returns the WoT depth tier for a pubkey serial:
//   - 1 = direct follow (depth 1), 2 = depth 2, 3 = depth 3
//   - 0 = outsider (not in WoT)
type WoTProvider interface {
	GetDepthForGC(pubkeySerial uint64) int
}

// GCDatabase defines the interface for database operations needed by the GC.
type GCDatabase interface {
	Path() string
	FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error)
	DeleteEventBySerial(ctx context.Context, ser *types.Uint40, ev *event.E) error
}

// AuthorLookup provides the ability to resolve event author pubkeys to serials.
type AuthorLookup interface {
	FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error)
	GetPubkeySerial(pubkey []byte) (*types.Uint40, error)
}

// gcStatsReq asks the actor for current stats.
type gcStatsReq struct {
	resp chan GCStats
}

// GarbageCollector manages continuous event eviction based on access patterns.
// It monitors storage usage and evicts the least accessed events when the
// storage limit is exceeded.
//
// When a WoTProvider is configured, eviction scoring incorporates WoT depth:
// events from socially proximate authors survive eviction longer than events
// from outsiders. Kind 0 (profile), kind 3 (follow list), and kind 10002
// (relay list) events are immune from eviction.
type GarbageCollector struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     GCDatabase
	tracker *AccessTracker

	// Configuration
	dataDir   string
	maxBytes  int64 // 0 = auto-calculate
	interval  time.Duration
	batchSize int
	minAgeSec int64 // Minimum age before considering for eviction

	// WoT integration (optional, nil = flat scoring)
	wotProvider  WoTProvider
	authorLookup AuthorLookup

	// Actor channels
	statsCh chan gcStatsReq
	stop    chan struct{}
	done    chan struct{}
}

// GCConfig holds configuration for the garbage collector.
type GCConfig struct {
	MaxStorageBytes int64         // 0 = auto-calculate (80% of filesystem)
	Interval        time.Duration // How often to check storage
	BatchSize       int           // Events to consider per GC run
	MinAgeSec       int64         // Minimum age before eviction (default: 1 hour)
	WoTProvider     WoTProvider   // Optional WoT depth provider for graph-weighted eviction
	AuthorLookup    AuthorLookup  // Required if WoTProvider is set
}

// DefaultGCConfig returns a default GC configuration.
func DefaultGCConfig() GCConfig {
	return GCConfig{
		MaxStorageBytes: 0,             // Auto-detect
		Interval:        time.Minute,   // Check every minute
		BatchSize:       1000,          // 1000 events per run
		MinAgeSec:       3600,          // 1 hour minimum age
	}
}

// NewGarbageCollector creates a new garbage collector.
func NewGarbageCollector(
	ctx context.Context,
	db GCDatabase,
	tracker *AccessTracker,
	cfg GCConfig,
) *GarbageCollector {
	gcCtx, cancel := context.WithCancel(ctx)

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Minute
	}
	if cfg.MinAgeSec <= 0 {
		cfg.MinAgeSec = 3600 // 1 hour
	}

	return &GarbageCollector{
		ctx:          gcCtx,
		cancel:       cancel,
		db:           db,
		tracker:      tracker,
		dataDir:      db.Path(),
		maxBytes:     cfg.MaxStorageBytes,
		interval:     cfg.Interval,
		batchSize:    cfg.BatchSize,
		minAgeSec:    cfg.MinAgeSec,
		wotProvider:  cfg.WoTProvider,
		authorLookup: cfg.AuthorLookup,
		statsCh:      make(chan gcStatsReq),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
	}
}

// Start begins the garbage collection loop.
func (gc *GarbageCollector) Start() {
	go gc.runLoop()
	log.I.F("garbage collector started (interval: %s, batch: %d)", gc.interval, gc.batchSize)
}

// Stop stops the garbage collector.
func (gc *GarbageCollector) Stop() {
	gc.cancel()
	close(gc.stop)
	<-gc.done
}

// runLoop is the main GC actor loop. It owns running, lastRun, and evictedCount.
func (gc *GarbageCollector) runLoop() {
	defer close(gc.done)

	running := true
	var lastRun time.Time
	var evictedCount uint64

	ticker := time.NewTicker(gc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-gc.stop:
			running = false
			log.I.F("garbage collector stopped (total evicted: %d)", evictedCount)
			return

		case <-gc.ctx.Done():
			running = false
			log.I.F("garbage collector stopped (total evicted: %d)", evictedCount)
			return

		case <-ticker.C:
			lastRun = time.Now()
			evicted, err := gc.runCycle()
			if err != nil {
				log.W.F("GC cycle error: %v", err)
			}
			evictedCount += uint64(evicted)
			_ = running
			_ = lastRun

		case req := <-gc.statsCh:
			// Get storage info
			currentBytes, _ := GetCurrentStorageUsage(gc.dataDir)
			maxBytes, _ := CalculateMaxStorage(gc.dataDir, gc.maxBytes)

			var percentage float64
			if maxBytes > 0 {
				percentage = float64(currentBytes) / float64(maxBytes) * 100
			}

			req.resp <- GCStats{
				Running:             running,
				LastRunTime:         lastRun,
				TotalEvicted:        evictedCount,
				CurrentStorageBytes: currentBytes,
				MaxStorageBytes:     maxBytes,
				StoragePercentage:   percentage,
			}
		}
	}
}

// runCycle executes one garbage collection cycle. Returns evicted count.
func (gc *GarbageCollector) runCycle() (int, error) {
	// Check if we need to run GC
	shouldRun, currentBytes, maxBytes, err := gc.shouldRunGC()
	if err != nil {
		return 0, err
	}

	if !shouldRun {
		return 0, nil
	}

	log.D.F("GC triggered: current=%d MB, max=%d MB (%.1f%%)",
		currentBytes/(1024*1024),
		maxBytes/(1024*1024),
		float64(currentBytes)/float64(maxBytes)*100)

	// Get coldest events, using WoT-weighted scoring if available
	var serials []uint64
	if gc.wotProvider != nil && gc.authorLookup != nil {
		serials, err = gc.tracker.GetColdestEventsWithWoT(
			gc.batchSize, 5, gc.minAgeSec, gc.wotProvider, gc.authorLookup)
	} else {
		serials, err = gc.tracker.GetColdestEvents(gc.batchSize, gc.minAgeSec)
	}
	if err != nil {
		return 0, err
	}

	if len(serials) == 0 {
		log.D.F("GC: no events eligible for eviction")
		return 0, nil
	}

	// Evict events
	evicted, err := gc.evictEvents(serials)
	if err != nil {
		return evicted, err
	}

	log.I.F("GC: evicted %d events", evicted)
	return evicted, nil
}

// shouldRunGC checks if storage limit is exceeded.
func (gc *GarbageCollector) shouldRunGC() (bool, int64, int64, error) {
	// Calculate max storage (dynamic based on filesystem)
	maxBytes, err := CalculateMaxStorage(gc.dataDir, gc.maxBytes)
	if err != nil {
		return false, 0, 0, err
	}

	// Get current usage
	currentBytes, err := GetCurrentStorageUsage(gc.dataDir)
	if err != nil {
		return false, 0, 0, err
	}

	return currentBytes > maxBytes, currentBytes, maxBytes, nil
}

// evictEvents evicts the specified events from the database.
func (gc *GarbageCollector) evictEvents(serials []uint64) (int, error) {
	evicted := 0

	for _, serial := range serials {
		// Check context for cancellation
		select {
		case <-gc.ctx.Done():
			return evicted, gc.ctx.Err()
		default:
		}

		// Convert serial to Uint40
		ser := &types.Uint40{}
		if err := ser.Set(serial); err != nil {
			log.D.F("GC: invalid serial %d: %v", serial, err)
			continue
		}

		// Fetch the event
		ev, err := gc.db.FetchEventBySerial(ser)
		if err != nil {
			log.D.F("GC: failed to fetch event %d: %v", serial, err)
			continue
		}
		if ev == nil {
			continue // Already deleted
		}

		// Never evict structural kinds that define the social graph
		if isImmuneKind(ev.Kind) {
			continue
		}

		// Delete the event
		if err := gc.db.DeleteEventBySerial(gc.ctx, ser, ev); err != nil {
			log.D.F("GC: failed to delete event %d: %v", serial, err)
			continue
		}

		evicted++

		// Rate limit to avoid overwhelming the database
		if evicted%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return evicted, nil
}

// Stats returns current GC statistics.
func (gc *GarbageCollector) Stats() GCStats {
	resp := make(chan GCStats, 1)
	gc.statsCh <- gcStatsReq{resp: resp}
	return <-resp
}

// isImmuneKind returns true for event kinds that must never be evicted.
// These structural kinds define the social graph and relay metadata.
func isImmuneKind(kind uint16) bool {
	switch kind {
	case 0:     // Profile metadata (NIP-01)
		return true
	case 3:     // Follow list (NIP-02)
		return true
	case 10002: // Relay list metadata (NIP-65)
		return true
	}
	return false
}

// wotBonus returns the coldness score bonus (in seconds) for a WoT depth tier.
// Higher bonus = harder to evict. Depth 1 gets 30 days of protection,
// depth 2 gets 7 days, depth 3 gets 1 day, outsiders get nothing.
func wotBonus(depth int) int64 {
	switch depth {
	case 1:
		return 2592000 // 30 days
	case 2:
		return 604800 // 7 days
	case 3:
		return 86400 // 1 day
	default:
		return 0
	}
}

// GCStats holds garbage collector statistics.
type GCStats struct {
	Running             bool
	LastRunTime         time.Time
	TotalEvicted        uint64
	CurrentStorageBytes int64
	MaxStorageBytes     int64
	StoragePercentage   float64
}
