//go:build !(js && wasm)

package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DatabaseConfig holds all database configuration options that can be passed
// to any database backend. Each backend uses the relevant fields for its type.
// This centralizes configuration instead of having each backend read env vars directly.
type DatabaseConfig struct {
	// Common settings for all backends
	DataDir  string
	LogLevel string

	// Badger-specific settings
	BlockCacheMB       int           // ORLY_DB_BLOCK_CACHE_MB
	IndexCacheMB       int           // ORLY_DB_INDEX_CACHE_MB
	QueryCacheDisabled bool          // ORLY_QUERY_CACHE_DISABLED - disable query cache to reduce memory usage
	QueryCacheSizeMB   int           // ORLY_QUERY_CACHE_SIZE_MB
	QueryCacheMaxAge   time.Duration // ORLY_QUERY_CACHE_MAX_AGE

	// Serial cache settings for compact event storage
	SerialCachePubkeys  int // ORLY_SERIAL_CACHE_PUBKEYS - max pubkeys to cache (default: 100000)
	SerialCacheEventIds int // ORLY_SERIAL_CACHE_EVENT_IDS - max event IDs to cache (default: 500000)

	// Compression settings
	ZSTDLevel int // ORLY_DB_ZSTD_LEVEL - ZSTD compression level (0=none, 1=fast, 3=default, 9=best)

	// Neo4j-specific settings
	Neo4jURI      string // ORLY_NEO4J_URI
	Neo4jUser     string // ORLY_NEO4J_USER
	Neo4jPassword string // ORLY_NEO4J_PASSWORD

	// Neo4j driver tuning (memory and connection management)
	Neo4jMaxConnPoolSize   int // ORLY_NEO4J_MAX_CONN_POOL - max connection pool size (default: 25)
	Neo4jFetchSize         int // ORLY_NEO4J_FETCH_SIZE - max records per fetch batch (default: 1000)
	Neo4jMaxTxRetrySeconds int // ORLY_NEO4J_MAX_TX_RETRY_SEC - max transaction retry time (default: 30)
	Neo4jQueryResultLimit  int // ORLY_NEO4J_QUERY_RESULT_LIMIT - max results per query (default: 10000, 0=unlimited)
}

// NewDatabase creates a database instance based on the specified type.
// Supported types: "badger", "neo4j"
func NewDatabase(
	ctx context.Context,
	cancel context.CancelFunc,
	dbType string,
	dataDir string,
	logLevel string,
) (Database, error) {
	// Create a default config for backward compatibility with existing callers
	cfg := &DatabaseConfig{
		DataDir:  dataDir,
		LogLevel: logLevel,
	}
	return NewDatabaseWithConfig(ctx, cancel, dbType, cfg)
}

// NewDatabaseWithConfig creates a database instance with full configuration.
// This is the preferred method when you have access to the app config.
func NewDatabaseWithConfig(
	ctx context.Context,
	cancel context.CancelFunc,
	dbType string,
	cfg *DatabaseConfig,
) (Database, error) {
	switch strings.ToLower(dbType) {
	case "badger", "":
		// Use the existing badger implementation
		return NewWithConfig(ctx, cancel, cfg)
	case "neo4j":
		// Use the neo4j implementation
		if newNeo4jDatabase == nil {
			return nil, fmt.Errorf("neo4j database backend not available (import _ \"next.orly.dev/pkg/neo4j\")")
		}
		return newNeo4jDatabase(ctx, cancel, cfg)
	case "wasmdb", "indexeddb", "wasm":
		// Use the wasmdb implementation (IndexedDB backend for WebAssembly)
		if newWasmDBDatabase == nil {
			return nil, fmt.Errorf("wasmdb database backend not available (import _ \"next.orly.dev/pkg/wasmdb\")")
		}
		return newWasmDBDatabase(ctx, cancel, cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: badger, neo4j, wasmdb)", dbType)
	}
}

// newNeo4jDatabase creates a neo4j database instance
// This is defined here to avoid import cycles
var newNeo4jDatabase func(context.Context, context.CancelFunc, *DatabaseConfig) (Database, error)

// RegisterNeo4jFactory registers the neo4j database factory
// This is called from the neo4j package's init() function
func RegisterNeo4jFactory(factory func(context.Context, context.CancelFunc, *DatabaseConfig) (Database, error)) {
	newNeo4jDatabase = factory
}

// newWasmDBDatabase creates a wasmdb database instance (IndexedDB backend for WebAssembly)
// This is defined here to avoid import cycles
var newWasmDBDatabase func(context.Context, context.CancelFunc, *DatabaseConfig) (Database, error)

// RegisterWasmDBFactory registers the wasmdb database factory
// This is called from the wasmdb package's init() function
func RegisterWasmDBFactory(factory func(context.Context, context.CancelFunc, *DatabaseConfig) (Database, error)) {
	newWasmDBDatabase = factory
}
