//go:build js && wasm

// Package wasmdb provides a WebAssembly-compatible database implementation
// using IndexedDB as the storage backend. It replicates the Badger database's
// index schema for full query compatibility.
//
// This implementation uses aperturerobotics/go-indexeddb (a fork of hack-pad/go-indexeddb)
// which provides full IndexedDB bindings with cursor/range support and transaction retry
// mechanisms to handle IndexedDB's transaction expiration issues in Go WASM.
//
// Architecture:
//   - Each index type (evt, eid, kc-, pc-, etc.) maps to an IndexedDB object store
//   - Keys are binary-encoded using the same format as the Badger implementation
//   - Range queries use IndexedDB cursors with KeyRange bounds
//   - Serial numbers are managed using a dedicated "meta" object store
package wasmdb

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"github.com/hack-pad/safejs"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/database/indexes"
)

const (
	// DatabaseName is the IndexedDB database name
	DatabaseName = "orly-nostr-relay"

	// DatabaseVersion is incremented when schema changes require migration
	DatabaseVersion = 1

	// MetaStoreName holds metadata like serial counters
	MetaStoreName = "meta"

	// EventSerialKey is the key for the event serial counter in meta store
	EventSerialKey = "event_serial"

	// PubkeySerialKey is the key for the pubkey serial counter in meta store
	PubkeySerialKey = "pubkey_serial"

	// RelayIdentityKey is the key for the relay identity secret
	RelayIdentityKey = "relay_identity"
)

// Object store names matching Badger index prefixes
var objectStoreNames = []string{
	MetaStoreName,
	string(indexes.EventPrefix),            // "evt" - full events
	string(indexes.SmallEventPrefix),       // "sev" - small events inline
	string(indexes.ReplaceableEventPrefix), // "rev" - replaceable events
	string(indexes.AddressableEventPrefix), // "aev" - addressable events
	string(indexes.IdPrefix),               // "eid" - event ID index
	string(indexes.FullIdPubkeyPrefix),     // "fpc" - full ID + pubkey + timestamp
	string(indexes.CreatedAtPrefix),        // "c--" - created_at index
	string(indexes.KindPrefix),             // "kc-" - kind index
	string(indexes.PubkeyPrefix),           // "pc-" - pubkey index
	string(indexes.KindPubkeyPrefix),       // "kpc" - kind + pubkey index
	string(indexes.TagPrefix),              // "tc-" - tag index
	string(indexes.TagKindPrefix),          // "tkc" - tag + kind index
	string(indexes.TagPubkeyPrefix),        // "tpc" - tag + pubkey index
	string(indexes.TagKindPubkeyPrefix),    // "tkp" - tag + kind + pubkey index
	string(indexes.WordPrefix),             // "wrd" - word search index
	string(indexes.ExpirationPrefix),       // "exp" - expiration index
	string(indexes.VersionPrefix),          // "ver" - schema version
	string(indexes.PubkeySerialPrefix),     // "pks" - pubkey serial index
	string(indexes.SerialPubkeyPrefix),     // "spk" - serial to pubkey
	string(indexes.EventPubkeyGraphPrefix), // "epg" - event-pubkey graph
	string(indexes.PubkeyEventGraphPrefix), // "peg" - pubkey-event graph
	"markers",                              // metadata key-value storage
	"subscriptions",                        // payment subscriptions
	"nip43",                                // NIP-43 membership
	"invites",                              // invite codes
}

// W implements the database.Database interface using IndexedDB
type W struct {
	ctx    context.Context
	cancel context.CancelFunc

	dataDir string // Not really used in WASM, but kept for interface compatibility
	Logger  *logger

	db    *idb.Database
	dbMu  sync.RWMutex
	ready chan struct{}

	// Serial counters (cached in memory, persisted to IndexedDB)
	eventSerial  uint64
	pubkeySerial uint64
	serialMu     sync.Mutex
}

// Ensure W implements database.Database interface at compile time
var _ database.Database = (*W)(nil)

// init registers the wasmdb database factory
func init() {
	database.RegisterWasmDBFactory(func(
		ctx context.Context,
		cancel context.CancelFunc,
		cfg *database.DatabaseConfig,
	) (database.Database, error) {
		return NewWithConfig(ctx, cancel, cfg)
	})
}

// NewWithConfig creates a new IndexedDB-based database instance
func NewWithConfig(
	ctx context.Context, cancel context.CancelFunc, cfg *database.DatabaseConfig,
) (*W, error) {
	w := &W{
		ctx:     ctx,
		cancel:  cancel,
		dataDir: cfg.DataDir,
		Logger:  NewLogger(lol.GetLogLevel(cfg.LogLevel)),
		ready:   make(chan struct{}),
	}

	// Open or create the IndexedDB database
	if err := w.openDatabase(); err != nil {
		return nil, fmt.Errorf("failed to open IndexedDB: %w", err)
	}

	// Load serial counters from storage
	if err := w.loadSerialCounters(); err != nil {
		return nil, fmt.Errorf("failed to load serial counters: %w", err)
	}

	// Start warmup goroutine
	go w.warmup()

	// Setup shutdown handler
	go func() {
		<-w.ctx.Done()
		w.cancel()
		w.Close()
	}()

	return w, nil
}

// New creates a new IndexedDB-based database instance with default configuration
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (*W, error) {
	cfg := &database.DatabaseConfig{
		DataDir:  dataDir,
		LogLevel: logLevel,
	}
	return NewWithConfig(ctx, cancel, cfg)
}

// openDatabase opens or creates the IndexedDB database with all required object stores
func (w *W) openDatabase() error {
	w.dbMu.Lock()
	defer w.dbMu.Unlock()

	// Get the IndexedDB factory (panics if not available)
	factory := idb.Global()

	// Open the database with upgrade handler
	openReq, err := factory.Open(w.ctx, DatabaseName, DatabaseVersion, func(db *idb.Database, oldVersion, newVersion uint) error {
		// This is called when the database needs to be created or upgraded
		w.Logger.Infof("IndexedDB upgrade: version %d -> %d", oldVersion, newVersion)

		// Create all object stores
		for _, storeName := range objectStoreNames {
			// Check if store already exists
			if !w.hasObjectStore(db, storeName) {
				// Create object store without auto-increment (we manage keys manually)
				opts := idb.ObjectStoreOptions{}
				if _, err := db.CreateObjectStore(storeName, opts); err != nil {
					return fmt.Errorf("failed to create object store %s: %w", storeName, err)
				}
				w.Logger.Debugf("created object store: %s", storeName)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to open IndexedDB: %w", err)
	}

	db, err := openReq.Await(w.ctx)
	if err != nil {
		return fmt.Errorf("failed to await IndexedDB open: %w", err)
	}

	w.db = db
	return nil
}

// hasObjectStore checks if an object store exists in the database
func (w *W) hasObjectStore(db *idb.Database, name string) bool {
	names, err := db.ObjectStoreNames()
	if err != nil {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// loadSerialCounters loads the event and pubkey serial counters from IndexedDB
func (w *W) loadSerialCounters() error {
	w.serialMu.Lock()
	defer w.serialMu.Unlock()

	// Load event serial
	eventSerialBytes, err := w.getMeta(EventSerialKey)
	if err != nil {
		return err
	}
	if eventSerialBytes != nil && len(eventSerialBytes) == 8 {
		w.eventSerial = binary.BigEndian.Uint64(eventSerialBytes)
	}

	// Load pubkey serial
	pubkeySerialBytes, err := w.getMeta(PubkeySerialKey)
	if err != nil {
		return err
	}
	if pubkeySerialBytes != nil && len(pubkeySerialBytes) == 8 {
		w.pubkeySerial = binary.BigEndian.Uint64(pubkeySerialBytes)
	}

	w.Logger.Infof("loaded serials: event=%d, pubkey=%d", w.eventSerial, w.pubkeySerial)
	return nil
}

// getMeta retrieves a value from the meta object store
func (w *W) getMeta(key string) ([]byte, error) {
	tx, err := w.db.Transaction(idb.TransactionReadOnly, MetaStoreName)
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(MetaStoreName)
	if err != nil {
		return nil, err
	}

	keyVal, err := safejs.ValueOf(key)
	if err != nil {
		return nil, err
	}

	req, err := store.Get(keyVal)
	if err != nil {
		return nil, err
	}

	val, err := req.Await(w.ctx)
	if err != nil {
		// Key not found is not an error
		return nil, nil
	}

	if val.IsUndefined() || val.IsNull() {
		return nil, nil
	}

	// Convert safejs.Value to []byte
	return safeValueToBytes(val), nil
}

// setMeta stores a value in the meta object store
func (w *W) setMeta(key string, value []byte) error {
	tx, err := w.db.Transaction(idb.TransactionReadWrite, MetaStoreName)
	if err != nil {
		return err
	}

	store, err := tx.ObjectStore(MetaStoreName)
	if err != nil {
		return err
	}

	// Convert value to Uint8Array for IndexedDB storage
	valueJS := bytesToSafeValue(value)

	// Put with key - using PutKey since we're managing keys
	keyVal, err := safejs.ValueOf(key)
	if err != nil {
		return err
	}

	_, err = store.PutKey(keyVal, valueJS)
	if err != nil {
		return err
	}

	return tx.Await(w.ctx)
}

// nextEventSerial returns the next event serial number and persists it
func (w *W) nextEventSerial() (uint64, error) {
	w.serialMu.Lock()
	defer w.serialMu.Unlock()

	w.eventSerial++
	serial := w.eventSerial

	// Persist to IndexedDB
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, serial)
	if err := w.setMeta(EventSerialKey, buf); err != nil {
		return 0, err
	}

	return serial, nil
}

// nextPubkeySerial returns the next pubkey serial number and persists it
func (w *W) nextPubkeySerial() (uint64, error) {
	w.serialMu.Lock()
	defer w.serialMu.Unlock()

	w.pubkeySerial++
	serial := w.pubkeySerial

	// Persist to IndexedDB
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, serial)
	if err := w.setMeta(PubkeySerialKey, buf); err != nil {
		return 0, err
	}

	return serial, nil
}

// warmup performs database warmup and closes the ready channel when complete
func (w *W) warmup() {
	defer close(w.ready)
	// IndexedDB is ready immediately after opening
	w.Logger.Infof("IndexedDB database warmup complete, ready to serve requests")
}

// Path returns the database path (not used in WASM)
func (w *W) Path() string { return w.dataDir }

// Init initializes the database (no-op, done in New)
func (w *W) Init(path string) error { return nil }

// Sync flushes pending writes (IndexedDB handles persistence automatically)
func (w *W) Sync() error { return nil }

// Close closes the database
func (w *W) Close() error {
	w.dbMu.Lock()
	defer w.dbMu.Unlock()

	if w.db != nil {
		w.db.Close()
		w.db = nil
	}
	return nil
}

// Wipe removes all data and recreates object stores
func (w *W) Wipe() error {
	w.dbMu.Lock()
	defer w.dbMu.Unlock()

	// Close the current database
	if w.db != nil {
		w.db.Close()
		w.db = nil
	}

	// Delete the database
	factory := idb.Global()
	delReq, err := factory.DeleteDatabase(DatabaseName)
	if err != nil {
		return fmt.Errorf("failed to delete IndexedDB: %w", err)
	}
	if err := delReq.Await(w.ctx); err != nil {
		return fmt.Errorf("failed to await IndexedDB delete: %w", err)
	}

	// Reset serial counters
	w.serialMu.Lock()
	w.eventSerial = 0
	w.pubkeySerial = 0
	w.serialMu.Unlock()

	// Reopen the database (this will recreate all object stores)
	w.dbMu.Unlock()
	err = w.openDatabase()
	w.dbMu.Lock()

	return err
}

// SetLogLevel sets the logging level
func (w *W) SetLogLevel(level string) {
	w.Logger.SetLogLevel(lol.GetLogLevel(level))
}

// Ready returns a channel that closes when the database is ready
func (w *W) Ready() <-chan struct{} { return w.ready }

// RunMigrations runs database migrations (handled by IndexedDB upgrade)
func (w *W) RunMigrations() {}

// EventIdsBySerial retrieves event IDs by serial range
func (w *W) EventIdsBySerial(start uint64, count int) ([]uint64, error) {
	return nil, errors.New("not implemented")
}

// Query cache methods (simplified for WASM - no caching)
func (w *W) GetCachedJSON(f *filter.F) ([][]byte, bool)             { return nil, false }
func (w *W) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {}
func (w *W) GetCachedEvents(f *filter.F) (event.S, bool)            { return nil, false }
func (w *W) CacheEvents(f *filter.F, events event.S)                {}
func (w *W) InvalidateQueryCache()                                  {}

// Placeholder implementations for remaining interface methods
// Query methods are implemented in query-events.go
// Delete methods are implemented in delete-event.go

// Import, Export, and ImportEvents methods are implemented in import-export.go

func (w *W) GetRelayIdentitySecret() (skb []byte, err error) {
	return w.getMeta(RelayIdentityKey)
}

func (w *W) SetRelayIdentitySecret(skb []byte) error {
	return w.setMeta(RelayIdentityKey, skb)
}

func (w *W) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	skb, err = w.GetRelayIdentitySecret()
	if err != nil {
		return nil, err
	}
	if skb != nil {
		return skb, nil
	}
	// Generate new secret key (32 random bytes)
	// In WASM, we use crypto.getRandomValues
	skb = make([]byte, 32)
	if err := cryptoRandom(skb); err != nil {
		return nil, err
	}
	if err := w.SetRelayIdentitySecret(skb); err != nil {
		return nil, err
	}
	return skb, nil
}

func (w *W) SetMarker(key string, value []byte) error {
	return w.setStoreValue("markers", key, value)
}

func (w *W) GetMarker(key string) (value []byte, err error) {
	return w.getStoreValue("markers", key)
}

func (w *W) HasMarker(key string) bool {
	val, err := w.GetMarker(key)
	return err == nil && val != nil
}

func (w *W) DeleteMarker(key string) error {
	return w.deleteStoreValue("markers", key)
}

// Subscription methods are implemented in subscriptions.go
// NIP-43 methods are implemented in nip43.go

// Helper methods for object store operations

func (w *W) setStoreValue(storeName, key string, value []byte) error {
	tx, err := w.db.Transaction(idb.TransactionReadWrite, storeName)
	if err != nil {
		return err
	}

	store, err := tx.ObjectStore(storeName)
	if err != nil {
		return err
	}

	keyVal, err := safejs.ValueOf(key)
	if err != nil {
		return err
	}

	valueJS := bytesToSafeValue(value)

	_, err = store.PutKey(keyVal, valueJS)
	if err != nil {
		return err
	}

	return tx.Await(w.ctx)
}

func (w *W) getStoreValue(storeName, key string) ([]byte, error) {
	tx, err := w.db.Transaction(idb.TransactionReadOnly, storeName)
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(storeName)
	if err != nil {
		return nil, err
	}

	keyVal, err := safejs.ValueOf(key)
	if err != nil {
		return nil, err
	}

	req, err := store.Get(keyVal)
	if err != nil {
		return nil, err
	}

	val, err := req.Await(w.ctx)
	if err != nil {
		return nil, nil
	}

	if val.IsUndefined() || val.IsNull() {
		return nil, nil
	}

	return safeValueToBytes(val), nil
}

func (w *W) deleteStoreValue(storeName, key string) error {
	tx, err := w.db.Transaction(idb.TransactionReadWrite, storeName)
	if err != nil {
		return err
	}

	store, err := tx.ObjectStore(storeName)
	if err != nil {
		return err
	}

	keyVal, err := safejs.ValueOf(key)
	if err != nil {
		return err
	}

	_, err = store.Delete(keyVal)
	if err != nil {
		return err
	}

	return tx.Await(w.ctx)
}

// Placeholder for unused variable
var _ = chk.E
