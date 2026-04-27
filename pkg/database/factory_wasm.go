//go:build js && wasm

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

	// Badger-specific settings (not available in WASM)
	BlockCacheMB     int           // ORLY_DB_BLOCK_CACHE_MB
	IndexCacheMB     int           // ORLY_DB_INDEX_CACHE_MB
	QueryCacheSizeMB int           // ORLY_QUERY_CACHE_SIZE_MB
	QueryCacheMaxAge time.Duration // ORLY_QUERY_CACHE_MAX_AGE

	// Serial cache settings for compact event storage (Badger-specific)
	SerialCachePubkeys  int // ORLY_SERIAL_CACHE_PUBKEYS - max pubkeys to cache (default: 100000)
	SerialCacheEventIds int // ORLY_SERIAL_CACHE_EVENT_IDS - max event IDs to cache (default: 500000)

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
// Supported types in WASM: "wasmdb", "neo4j"
// Note: "badger" is not available in WASM builds due to filesystem dependencies
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
	case "wasmdb", "indexeddb", "wasm", "badger", "":
		// In WASM builds, default to wasmdb (IndexedDB backend)
		// "badger" is mapped to wasmdb since Badger is not available
		if newWasmDBDatabase == nil {
			return nil, fmt.Errorf("wasmdb database backend not available (import _ \"git.smesh.lol/orly/pkg/wasmdb\")")
		}
		return newWasmDBDatabase(ctx, cancel, cfg)
	case "neo4j":
		// Use the neo4j implementation (HTTP-based, works in WASM)
		if newNeo4jDatabase == nil {
			return nil, fmt.Errorf("neo4j database backend not available (import _ \"git.smesh.lol/orly/pkg/neo4j\")")
		}
		return newNeo4jDatabase(ctx, cancel, cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported in WASM: wasmdb, neo4j)", dbType)
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
