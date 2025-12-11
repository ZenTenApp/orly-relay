// Package loadmonitor defines the interface for database load monitoring.
// This allows different database backends to provide their own load metrics
// while the rate limiter remains database-agnostic.
package loadmonitor

import "time"

// Metrics contains load metrics from a database backend.
// All values are normalized to 0.0-1.0 where 0 means no load and 1 means at capacity.
type Metrics struct {
	// MemoryPressure indicates memory usage relative to a target limit (0.0-1.0+).
	// Values above 1.0 indicate the target has been exceeded.
	MemoryPressure float64

	// WriteLoad indicates the write-side load level (0.0-1.0).
	// For Badger: L0 tables and compaction score
	// For Neo4j: active write transactions
	WriteLoad float64

	// ReadLoad indicates the read-side load level (0.0-1.0).
	// For Badger: cache hit ratio (inverted)
	// For Neo4j: active read transactions
	ReadLoad float64

	// QueryLatency is the recent average query latency.
	QueryLatency time.Duration

	// WriteLatency is the recent average write latency.
	WriteLatency time.Duration

	// Timestamp is when these metrics were collected.
	Timestamp time.Time
}

// Monitor defines the interface for database load monitoring.
// Implementations are database-specific (Badger, Neo4j, etc.).
type Monitor interface {
	// GetMetrics returns the current load metrics.
	// This should be efficient as it may be called frequently.
	GetMetrics() Metrics

	// RecordQueryLatency records a query latency sample for averaging.
	RecordQueryLatency(latency time.Duration)

	// RecordWriteLatency records a write latency sample for averaging.
	RecordWriteLatency(latency time.Duration)

	// SetMemoryTarget sets the target memory limit in bytes.
	// Memory pressure is calculated relative to this target.
	SetMemoryTarget(bytes uint64)

	// Start begins background metric collection.
	// Returns a channel that will be closed when the monitor is stopped.
	Start() <-chan struct{}

	// Stop halts background metric collection.
	Stop()
}
