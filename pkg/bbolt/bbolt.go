//go:build !(js && wasm)

package bbolt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/utils/apputil"
)

// Bucket names - map to existing index prefixes but without the 3-byte prefix in keys
var (
	bucketEvt  = []byte("evt")  // Event storage: serial -> compact event data
	bucketEid  = []byte("eid")  // Event ID index
	bucketFpc  = []byte("fpc")  // Full ID/pubkey index
	bucketC    = []byte("c--")  // Created at index
	bucketKc   = []byte("kc-")  // Kind + created index
	bucketPc   = []byte("pc-")  // Pubkey + created index
	bucketKpc  = []byte("kpc")  // Kind + pubkey + created
	bucketTc   = []byte("tc-")  // Tag + created
	bucketTkc  = []byte("tkc")  // Tag + kind + created
	bucketTpc  = []byte("tpc")  // Tag + pubkey + created
	bucketTkp  = []byte("tkp")  // Tag + kind + pubkey + created
	bucketWrd  = []byte("wrd")  // Word search index
	bucketExp  = []byte("exp")  // Expiration index
	bucketPks  = []byte("pks")  // Pubkey hash -> serial
	bucketSpk  = []byte("spk")  // Serial -> pubkey
	bucketSei  = []byte("sei")  // Serial -> event ID
	bucketCmp  = []byte("cmp")  // Compact event storage
	bucketEv   = []byte("ev")   // Event vertices (adjacency list)
	bucketPv   = []byte("pv")   // Pubkey vertices (adjacency list)
	bucketMeta = []byte("_meta") // Markers, version, serial counter, bloom filter
)

// All buckets that need to be created on init
var allBuckets = [][]byte{
	bucketEvt, bucketEid, bucketFpc, bucketC, bucketKc, bucketPc, bucketKpc,
	bucketTc, bucketTkc, bucketTpc, bucketTkp, bucketWrd, bucketExp,
	bucketPks, bucketSpk, bucketSei, bucketCmp, bucketEv, bucketPv, bucketMeta,
}

// B implements the database.Database interface using BBolt as the storage backend.
// Optimized for HDD with write batching and adjacency list graph storage.
type B struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *Logger

	db    *bolt.DB
	ready chan struct{}

	// Write batching
	batcher *WriteBatcher

	// Serial management
	serialMu       sync.Mutex
	nextSerial     uint64
	nextPubkeySeq  uint64

	// Edge bloom filter for fast negative lookups
	edgeBloom *EdgeBloomFilter

	// Configuration
	cfg *BboltConfig
}

// BboltConfig holds bbolt-specific configuration
type BboltConfig struct {
	DataDir  string
	LogLevel string

	// Batch settings (tuned for 7200rpm HDD)
	BatchMaxEvents    int           // Max events before flush (default: 5000)
	BatchMaxBytes     int64         // Max bytes before flush (default: 128MB)
	BatchFlushTimeout time.Duration // Max time before flush (default: 30s)

	// Bloom filter settings
	BloomSizeMB int // Bloom filter size in MB (default: 16)

	// BBolt settings
	NoSync          bool // Disable fsync for performance (DANGEROUS)
	InitialMmapSize int  // Initial mmap size in bytes
}

// Ensure B implements Database interface at compile time
var _ database.Database = (*B)(nil)

// New creates a new BBolt database instance with default configuration.
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (b *B, err error) {
	cfg := &BboltConfig{
		DataDir:           dataDir,
		LogLevel:          logLevel,
		BatchMaxEvents:    5000,
		BatchMaxBytes:     128 * 1024 * 1024, // 128MB
		BatchFlushTimeout: 30 * time.Second,
		BloomSizeMB:       16,
		InitialMmapSize:   8 * 1024 * 1024 * 1024, // 8GB
	}
	return NewWithConfig(ctx, cancel, cfg)
}

// NewWithConfig creates a new BBolt database instance with full configuration.
func NewWithConfig(
	ctx context.Context, cancel context.CancelFunc, cfg *BboltConfig,
) (b *B, err error) {
	// Apply defaults
	if cfg.BatchMaxEvents <= 0 {
		cfg.BatchMaxEvents = 5000
	}
	if cfg.BatchMaxBytes <= 0 {
		cfg.BatchMaxBytes = 128 * 1024 * 1024
	}
	if cfg.BatchFlushTimeout <= 0 {
		cfg.BatchFlushTimeout = 30 * time.Second
	}
	if cfg.BloomSizeMB <= 0 {
		cfg.BloomSizeMB = 16
	}
	if cfg.InitialMmapSize <= 0 {
		cfg.InitialMmapSize = 8 * 1024 * 1024 * 1024
	}

	b = &B{
		ctx:     ctx,
		cancel:  cancel,
		dataDir: cfg.DataDir,
		Logger:  NewLogger(lol.GetLogLevel(cfg.LogLevel), cfg.DataDir),
		ready:   make(chan struct{}),
		cfg:     cfg,
	}

	// Ensure the data directory exists
	if err = os.MkdirAll(cfg.DataDir, 0755); chk.E(err) {
		return
	}
	if err = apputil.EnsureDir(filepath.Join(cfg.DataDir, "dummy")); chk.E(err) {
		return
	}

	// Open BBolt database
	dbPath := filepath.Join(cfg.DataDir, "orly.db")
	opts := &bolt.Options{
		Timeout:         10 * time.Second,
		NoSync:          cfg.NoSync,
		InitialMmapSize: cfg.InitialMmapSize,
	}

	if b.db, err = bolt.Open(dbPath, 0600, opts); chk.E(err) {
		return
	}

	// Create all buckets
	if err = b.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	}); chk.E(err) {
		return
	}

	// Initialize serial counters
	if err = b.initSerialCounters(); chk.E(err) {
		return
	}

	// Initialize bloom filter
	b.edgeBloom, err = NewEdgeBloomFilter(cfg.BloomSizeMB, b.db)
	if chk.E(err) {
		return
	}

	// Initialize write batcher
	b.batcher = NewWriteBatcher(b.db, b.edgeBloom, cfg, b.Logger)

	// Run migrations
	b.RunMigrations()

	// Start warmup and mark ready
	go b.warmup()

	// Start background maintenance
	go b.backgroundLoop()

	return
}

// Path returns the path where the database files are stored.
func (b *B) Path() string { return b.dataDir }

// Init initializes the database with the given path.
func (b *B) Init(path string) error {
	b.dataDir = path
	return nil
}

// Sync flushes the database buffers to disk.
func (b *B) Sync() error {
	// Flush pending writes
	if err := b.batcher.Flush(); err != nil {
		return err
	}
	// Persist bloom filter
	if err := b.edgeBloom.Persist(b.db); err != nil {
		return err
	}
	// Persist serial counters
	if err := b.persistSerialCounters(); err != nil {
		return err
	}
	// Sync BBolt
	return b.db.Sync()
}

// Close releases resources and closes the database.
func (b *B) Close() (err error) {
	b.Logger.Infof("bbolt: closing database...")

	// Stop accepting new writes and flush pending
	if b.batcher != nil {
		if err = b.batcher.Shutdown(); chk.E(err) {
			// Log but continue cleanup
		}
	}

	// Persist bloom filter
	if b.edgeBloom != nil {
		if err = b.edgeBloom.Persist(b.db); chk.E(err) {
			// Log but continue cleanup
		}
	}

	// Persist serial counters
	if err = b.persistSerialCounters(); chk.E(err) {
		// Log but continue cleanup
	}

	// Close BBolt database
	if b.db != nil {
		if err = b.db.Close(); chk.E(err) {
			return
		}
	}

	b.Logger.Infof("bbolt: database closed")
	return
}

// Wipe deletes all data in the database.
func (b *B) Wipe() error {
	return b.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range allBuckets {
			if err := tx.DeleteBucket(bucket); err != nil && !errors.Is(err, bolt.ErrBucketNotFound) {
				return err
			}
			if _, err := tx.CreateBucket(bucket); err != nil {
				return err
			}
		}
		// Reset serial counters
		b.serialMu.Lock()
		b.nextSerial = 1
		b.nextPubkeySeq = 1
		b.serialMu.Unlock()
		// Reset bloom filter
		b.edgeBloom.Reset()
		return nil
	})
}

// SetLogLevel changes the logging level.
func (b *B) SetLogLevel(level string) {
	b.Logger.SetLogLevel(lol.GetLogLevel(level))
}

// Ready returns a channel that closes when the database is ready to serve requests.
func (b *B) Ready() <-chan struct{} {
	return b.ready
}

// warmup performs database warmup operations and closes the ready channel when complete.
func (b *B) warmup() {
	defer close(b.ready)

	// Give the database time to settle
	time.Sleep(1 * time.Second)

	b.Logger.Infof("bbolt: database warmup complete, ready to serve requests")
}

// backgroundLoop runs periodic maintenance tasks.
func (b *B) backgroundLoop() {
	expirationTicker := time.NewTicker(10 * time.Minute)
	bloomPersistTicker := time.NewTicker(5 * time.Minute)
	defer expirationTicker.Stop()
	defer bloomPersistTicker.Stop()

	for {
		select {
		case <-expirationTicker.C:
			b.DeleteExpired()
		case <-bloomPersistTicker.C:
			if err := b.edgeBloom.Persist(b.db); chk.E(err) {
				b.Logger.Warningf("bbolt: failed to persist bloom filter: %v", err)
			}
		case <-b.ctx.Done():
			b.cancel()
			return
		}
	}
}
