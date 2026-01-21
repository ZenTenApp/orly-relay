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
	"sync"
	"sync/atomic"
	"time"

	"lol.mleku.dev/log"

	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// GCDatabase defines the interface for database operations needed by the GC.
type GCDatabase interface {
	Path() string
	FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error)
	DeleteEventBySerial(ctx context.Context, ser *types.Uint40, ev *event.E) error
}

// GarbageCollector manages continuous event eviction based on access patterns.
// It monitors storage usage and evicts the least accessed events when the
// storage limit is exceeded.
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

	// State
	mu           sync.Mutex
	running      bool
	evictedCount uint64
	lastRun      time.Time
}

// GCConfig holds configuration for the garbage collector.
type GCConfig struct {
	MaxStorageBytes int64         // 0 = auto-calculate (80% of filesystem)
	Interval        time.Duration // How often to check storage
	BatchSize       int           // Events to consider per GC run
	MinAgeSec       int64         // Minimum age before eviction (default: 1 hour)
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
		ctx:       gcCtx,
		cancel:    cancel,
		db:        db,
		tracker:   tracker,
		dataDir:   db.Path(),
		maxBytes:  cfg.MaxStorageBytes,
		interval:  cfg.Interval,
		batchSize: cfg.BatchSize,
		minAgeSec: cfg.MinAgeSec,
	}
}

// Start begins the garbage collection loop.
func (gc *GarbageCollector) Start() {
	gc.mu.Lock()
	if gc.running {
		gc.mu.Unlock()
		return
	}
	gc.running = true
	gc.mu.Unlock()

	go gc.runLoop()
	log.I.F("garbage collector started (interval: %s, batch: %d)", gc.interval, gc.batchSize)
}

// Stop stops the garbage collector.
func (gc *GarbageCollector) Stop() {
	gc.cancel()
	gc.mu.Lock()
	gc.running = false
	gc.mu.Unlock()
	log.I.F("garbage collector stopped (total evicted: %d)", atomic.LoadUint64(&gc.evictedCount))
}

// runLoop is the main GC loop.
func (gc *GarbageCollector) runLoop() {
	ticker := time.NewTicker(gc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-gc.ctx.Done():
			return
		case <-ticker.C:
			if err := gc.runCycle(); err != nil {
				log.W.F("GC cycle error: %v", err)
			}
		}
	}
}

// runCycle executes one garbage collection cycle.
func (gc *GarbageCollector) runCycle() error {
	gc.mu.Lock()
	gc.lastRun = time.Now()
	gc.mu.Unlock()

	// Check if we need to run GC
	shouldRun, currentBytes, maxBytes, err := gc.shouldRunGC()
	if err != nil {
		return err
	}

	if !shouldRun {
		return nil
	}

	log.D.F("GC triggered: current=%d MB, max=%d MB (%.1f%%)",
		currentBytes/(1024*1024),
		maxBytes/(1024*1024),
		float64(currentBytes)/float64(maxBytes)*100)

	// Get coldest events
	serials, err := gc.tracker.GetColdestEvents(gc.batchSize, gc.minAgeSec)
	if err != nil {
		return err
	}

	if len(serials) == 0 {
		log.D.F("GC: no events eligible for eviction")
		return nil
	}

	// Evict events
	evicted, err := gc.evictEvents(serials)
	if err != nil {
		return err
	}

	atomic.AddUint64(&gc.evictedCount, uint64(evicted))
	log.I.F("GC: evicted %d events (total: %d)", evicted, atomic.LoadUint64(&gc.evictedCount))

	return nil
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
	gc.mu.Lock()
	lastRun := gc.lastRun
	running := gc.running
	gc.mu.Unlock()

	// Get storage info
	currentBytes, _ := GetCurrentStorageUsage(gc.dataDir)
	maxBytes, _ := CalculateMaxStorage(gc.dataDir, gc.maxBytes)

	var percentage float64
	if maxBytes > 0 {
		percentage = float64(currentBytes) / float64(maxBytes) * 100
	}

	return GCStats{
		Running:             running,
		LastRunTime:         lastRun,
		TotalEvicted:        atomic.LoadUint64(&gc.evictedCount),
		CurrentStorageBytes: currentBytes,
		MaxStorageBytes:     maxBytes,
		StoragePercentage:   percentage,
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
