//go:build !(js && wasm)

package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/querycache"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"next.orly.dev/pkg/utils/apputil"
	"git.mleku.dev/mleku/nostr/utils/units"
)

// D implements the Database interface using Badger as the storage backend
type D struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *logger
	*badger.DB
	seq        *badger.Sequence
	pubkeySeq  *badger.Sequence // Sequence for pubkey serials
	ready      chan struct{}    // Closed when database is ready to serve requests
	queryCache *querycache.EventCache

	// Serial cache for compact event storage
	// Caches pubkey and event ID serial mappings for fast compact event decoding
	serialCache *SerialCache
}

// Ensure D implements Database interface at compile time
var _ Database = (*D)(nil)

// New creates a new Badger database instance with default configuration.
// This is provided for backward compatibility with existing callers.
// For full configuration control, use NewWithConfig instead.
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (
	d *D, err error,
) {
	// Create a default config for backward compatibility
	cfg := &DatabaseConfig{
		DataDir:          dataDir,
		LogLevel:         logLevel,
		BlockCacheMB:     1024,            // Default 1024 MB
		IndexCacheMB:     512,             // Default 512 MB
		QueryCacheSizeMB: 512,             // Default 512 MB
		QueryCacheMaxAge: 5 * time.Minute, // Default 5 minutes
	}
	return NewWithConfig(ctx, cancel, cfg)
}

// NewWithConfig creates a new Badger database instance with full configuration.
// This is the preferred method when you have access to DatabaseConfig.
func NewWithConfig(
	ctx context.Context, cancel context.CancelFunc, cfg *DatabaseConfig,
) (
	d *D, err error,
) {
	// Apply defaults for zero values (backward compatibility)
	blockCacheMB := cfg.BlockCacheMB
	if blockCacheMB == 0 {
		blockCacheMB = 1024 // Default 1024 MB
	}
	indexCacheMB := cfg.IndexCacheMB
	if indexCacheMB == 0 {
		indexCacheMB = 512 // Default 512 MB
	}
	queryCacheSizeMB := cfg.QueryCacheSizeMB
	if queryCacheSizeMB == 0 {
		queryCacheSizeMB = 512 // Default 512 MB
	}
	queryCacheMaxAge := cfg.QueryCacheMaxAge
	if queryCacheMaxAge == 0 {
		queryCacheMaxAge = 5 * time.Minute // Default 5 minutes
	}

	// Serial cache configuration for compact event storage
	serialCachePubkeys := cfg.SerialCachePubkeys
	if serialCachePubkeys == 0 {
		serialCachePubkeys = 100000 // Default 100k pubkeys (~3.2MB memory)
	}
	serialCacheEventIds := cfg.SerialCacheEventIds
	if serialCacheEventIds == 0 {
		serialCacheEventIds = 500000 // Default 500k event IDs (~16MB memory)
	}

	// ZSTD compression level configuration
	// Level 0 = disabled, 1 = fast (~500 MB/s), 3 = default, 9 = best ratio
	zstdLevel := cfg.ZSTDLevel
	if zstdLevel < 0 {
		zstdLevel = 0
	} else if zstdLevel > 19 {
		zstdLevel = 19 // ZSTD maximum level
	}

	queryCacheSize := int64(queryCacheSizeMB * 1024 * 1024)

	d = &D{
		ctx:         ctx,
		cancel:      cancel,
		dataDir:     cfg.DataDir,
		Logger:      NewLogger(lol.GetLogLevel(cfg.LogLevel), cfg.DataDir),
		DB:          nil,
		seq:         nil,
		ready:       make(chan struct{}),
		queryCache:  querycache.NewEventCache(queryCacheSize, queryCacheMaxAge),
		serialCache: NewSerialCache(serialCachePubkeys, serialCacheEventIds),
	}

	// Ensure the data directory exists
	if err = os.MkdirAll(cfg.DataDir, 0755); chk.E(err) {
		return
	}

	// Also ensure the directory exists using apputil.EnsureDir for any
	// potential subdirectories
	dummyFile := filepath.Join(cfg.DataDir, "dummy.sst")
	if err = apputil.EnsureDir(dummyFile); chk.E(err) {
		return
	}

	opts := badger.DefaultOptions(d.dataDir)
	// Configure caches based on config to better match workload.
	// Defaults aim for higher hit ratios under read-heavy workloads while remaining safe.
	opts.BlockCacheSize = int64(blockCacheMB * units.Mb)
	opts.IndexCacheSize = int64(indexCacheMB * units.Mb)
	opts.BlockSize = 4 * units.Kb // 4 KB block size

	// Reduce table sizes to lower cost-per-key in cache
	// Smaller tables mean lower cache cost metric per entry
	opts.BaseTableSize = 8 * units.Mb // 8 MB per table (reduced from 64 MB to lower cache cost)
	opts.MemTableSize = 16 * units.Mb // 16 MB memtable (reduced from 64 MB)

	// Keep value log files to a moderate size
	opts.ValueLogFileSize = 128 * units.Mb // 128 MB value log files (reduced from 256 MB)

	// CRITICAL: Keep small inline events in LSM tree, not value log
	// VLogPercentile 0.99 means 99% of values stay in LSM (our optimized inline events!)
	// This dramatically improves read performance for small events
	opts.VLogPercentile = 0.99

	// Optimize LSM tree structure
	opts.BaseLevelSize = 64 * units.Mb // Increased from default 10 MB for fewer levels
	opts.LevelSizeMultiplier = 10      // Default, good balance

	opts.CompactL0OnClose = true
	opts.LmaxCompaction = true

	// Enable compression to reduce cache cost
	// Level 0 disables compression, 1 = fast (~500 MB/s), 3 = default, 9 = best ratio
	if zstdLevel == 0 {
		opts.Compression = options.None
	} else {
		opts.Compression = options.ZSTD
		opts.ZSTDCompressionLevel = zstdLevel
	}

	// Disable conflict detection for write-heavy relay workloads
	// Nostr events are immutable, no need for transaction conflict checks
	opts.DetectConflicts = false

	// Performance tuning for high-throughput workloads
	opts.NumCompactors = 8            // Increase from default 4 for faster compaction
	opts.NumLevelZeroTables = 8       // Increase from default 5 to allow more L0 tables before compaction
	opts.NumLevelZeroTablesStall = 16 // Increase from default 15 to reduce write stalls
	opts.NumMemtables = 8             // Increase from default 5 to buffer more writes
	opts.MaxLevels = 7                // Default is 7, keep it

	opts.Logger = d.Logger
	if d.DB, err = badger.Open(opts); chk.E(err) {
		return
	}
	if d.seq, err = d.DB.GetSequence([]byte("EVENTS"), 1000); chk.E(err) {
		return
	}
	if d.pubkeySeq, err = d.DB.GetSequence([]byte("PUBKEYS"), 1000); chk.E(err) {
		return
	}
	// run code that updates indexes when new indexes have been added and bumps
	// the version so they aren't run again.
	d.RunMigrations()

	// Start warmup goroutine to signal when database is ready
	go d.warmup()

	// start up the expiration tag processing and shut down and clean up the
	// database after the context is canceled.
	go func() {
		expirationTicker := time.NewTicker(time.Minute * 10)
		select {
		case <-expirationTicker.C:
			d.DeleteExpired()
			return
		case <-d.ctx.Done():
		}
		d.cancel()
		// d.seq.Release()
		// d.DB.Close()
	}()
	return
}

// Path returns the path where the database files are stored.
func (d *D) Path() string { return d.dataDir }

// Ready returns a channel that closes when the database is ready to serve requests.
// This allows callers to wait for database warmup to complete.
func (d *D) Ready() <-chan struct{} {
	return d.ready
}

// warmup performs database warmup operations and closes the ready channel when complete.
// Warmup criteria:
// - Wait at least 2 seconds for initial compactions to settle
// - Ensure cache hit ratio is reasonable (if we have metrics available)
func (d *D) warmup() {
	defer close(d.ready)

	// Give the database time to settle after opening
	// This allows:
	// - Initial compactions to complete
	// - Memory allocations to stabilize
	// - Cache to start warming up
	time.Sleep(2 * time.Second)

	d.Logger.Infof("database warmup complete, ready to serve requests")
}

func (d *D) Wipe() (err error) {
	err = errors.New("not implemented")
	return
}

func (d *D) SetLogLevel(level string) {
	d.Logger.SetLogLevel(lol.GetLogLevel(level))
}

func (d *D) EventIdsBySerial(start uint64, count int) (
	evs []uint64, err error,
) {
	err = errors.New("not implemented")
	return
}

// Init initializes the database with the given path.
func (d *D) Init(path string) (err error) {
	// The database is already initialized in the New function,
	// so we just need to ensure the path is set correctly.
	d.dataDir = path
	return nil
}

// Sync flushes the database buffers to disk.
func (d *D) Sync() (err error) {
	d.DB.RunValueLogGC(0.5)
	return d.DB.Sync()
}

// QueryCacheStats returns statistics about the query cache
func (d *D) QueryCacheStats() querycache.CacheStats {
	if d.queryCache == nil {
		return querycache.CacheStats{}
	}
	return d.queryCache.Stats()
}

// InvalidateQueryCache clears all entries from the query cache
func (d *D) InvalidateQueryCache() {
	if d.queryCache != nil {
		d.queryCache.Invalidate()
	}
}

// GetCachedJSON retrieves cached marshaled JSON for a filter
// Returns nil, false if not found
func (d *D) GetCachedJSON(f *filter.F) ([][]byte, bool) {
	if d.queryCache == nil {
		return nil, false
	}
	return d.queryCache.Get(f)
}

// CacheMarshaledJSON stores marshaled JSON event envelopes for a filter
func (d *D) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {
	if d.queryCache != nil && len(marshaledJSON) > 0 {
		// Store the serialized JSON directly - this is already in envelope format
		// We create a wrapper to store it with the right structure
		d.queryCache.PutJSON(f, marshaledJSON)
	}
}

// GetCachedEvents retrieves cached events for a filter (without subscription ID)
// Returns nil, false if not found
func (d *D) GetCachedEvents(f *filter.F) (event.S, bool) {
	if d.queryCache == nil {
		return nil, false
	}
	return d.queryCache.GetEvents(f)
}

// CacheEvents stores events for a filter (without subscription ID)
func (d *D) CacheEvents(f *filter.F, events event.S) {
	if d.queryCache != nil && len(events) > 0 {
		d.queryCache.PutEvents(f, events)
	}
}

// Close releases resources and closes the database.
func (d *D) Close() (err error) {
	if d.seq != nil {
		if err = d.seq.Release(); chk.E(err) {
			return
		}
	}
	if d.DB != nil {
		if err = d.DB.Close(); chk.E(err) {
			return
		}
	}
	return
}
