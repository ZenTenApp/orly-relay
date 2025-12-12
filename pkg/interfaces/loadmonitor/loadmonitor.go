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

	// InEmergencyMode indicates that memory pressure is critical
	// and aggressive throttling should be applied.
	InEmergencyMode bool

	// CompactionPending indicates that the database needs compaction
	// and writes should be throttled to allow it to catch up.
	CompactionPending bool

	// PhysicalMemoryMB is the actual physical memory (RSS - shared) in MB
	PhysicalMemoryMB uint64
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

// CompactableMonitor extends Monitor with compaction-triggering capability.
// Implemented by database backends that support manual compaction (e.g., Badger).
type CompactableMonitor interface {
	Monitor

	// TriggerCompaction initiates a database compaction operation.
	// This may take significant time; callers should run this in a goroutine.
	// Returns an error if compaction fails or is not supported.
	TriggerCompaction() error

	// IsCompacting returns true if a compaction is currently in progress.
	IsCompacting() bool
}

// EmergencyModeMonitor extends Monitor with emergency mode detection.
// Implemented by monitors that can detect critical memory pressure.
type EmergencyModeMonitor interface {
	Monitor

	// SetEmergencyThreshold sets the memory threshold (as a fraction, e.g., 1.5 = 150% of target)
	// above which emergency mode is triggered.
	SetEmergencyThreshold(threshold float64)

	// GetEmergencyThreshold returns the current emergency threshold.
	GetEmergencyThreshold() float64

	// ForceEmergencyMode manually triggers emergency mode for a duration.
	ForceEmergencyMode(duration time.Duration)
}
