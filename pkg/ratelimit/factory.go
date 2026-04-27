//go:build !(js && wasm)

package ratelimit

import (
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"git.smesh.lol/orly/pkg/interfaces/loadmonitor"
)

// NewBadgerLimiter creates a rate limiter configured for a Badger database.
// It automatically creates a BadgerMonitor for the provided database.
func NewBadgerLimiter(config Config, db *badger.DB) *Limiter {
	monitor := NewBadgerMonitor(db, 100*time.Millisecond)
	return NewLimiter(config, monitor)
}

// NewNeo4jLimiter creates a rate limiter configured for a Neo4j database.
// It automatically creates a Neo4jMonitor for the provided driver.
// querySem should be the semaphore used to limit concurrent queries.
// maxConcurrency is typically 10 (matching the semaphore size).
func NewNeo4jLimiter(
	config Config,
	driver neo4j.DriverWithContext,
	querySem chan struct{},
	maxConcurrency int,
) *Limiter {
	monitor := NewNeo4jMonitor(driver, querySem, maxConcurrency, 100*time.Millisecond)
	return NewLimiter(config, monitor)
}

// NewDisabledLimiter creates a rate limiter that is disabled.
// This is useful when rate limiting is not configured.
func NewDisabledLimiter() *Limiter {
	config := DefaultConfig()
	config.Enabled = false
	return NewLimiter(config, nil)
}

// MonitorFromBadgerDB creates a BadgerMonitor from a Badger database.
// Exported for use when you need to create the monitor separately.
func MonitorFromBadgerDB(db *badger.DB) loadmonitor.Monitor {
	return NewBadgerMonitor(db, 100*time.Millisecond)
}

// MonitorFromNeo4jDriver creates a Neo4jMonitor from a Neo4j driver.
// Exported for use when you need to create the monitor separately.
func MonitorFromNeo4jDriver(
	driver neo4j.DriverWithContext,
	querySem chan struct{},
	maxConcurrency int,
) loadmonitor.Monitor {
	return NewNeo4jMonitor(driver, querySem, maxConcurrency, 100*time.Millisecond)
}

// NewMemoryOnlyLimiter creates a rate limiter that only monitors process memory.
// Useful for database backends that don't have their own load metrics.
func NewMemoryOnlyLimiter(config Config) *Limiter {
	monitor := NewMemoryMonitor(100 * time.Millisecond)
	return NewLimiter(config, monitor)
}
