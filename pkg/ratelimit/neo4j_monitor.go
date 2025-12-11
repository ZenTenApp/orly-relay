package ratelimit

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"next.orly.dev/pkg/interfaces/loadmonitor"
)

// Neo4jMonitor implements loadmonitor.Monitor for Neo4j database.
// Since Neo4j driver doesn't expose detailed metrics, we track:
// - Memory pressure via Go runtime
// - Query concurrency via the semaphore
// - Latency via recording
type Neo4jMonitor struct {
	driver   neo4j.DriverWithContext
	querySem chan struct{} // Reference to the query semaphore

	// Target memory for pressure calculation
	targetMemoryBytes atomic.Uint64

	// Latency tracking with exponential moving average
	queryLatencyNs atomic.Int64
	writeLatencyNs atomic.Int64
	latencyAlpha   float64 // EMA coefficient (default 0.1)

	// Concurrency tracking
	activeReads  atomic.Int32
	activeWrites atomic.Int32
	maxConcurrency int

	// Cached metrics (updated by background goroutine)
	metricsLock   sync.RWMutex
	cachedMetrics loadmonitor.Metrics

	// Background collection
	stopChan chan struct{}
	stopped  chan struct{}
	interval time.Duration
}

// Compile-time check that Neo4jMonitor implements loadmonitor.Monitor
var _ loadmonitor.Monitor = (*Neo4jMonitor)(nil)

// NewNeo4jMonitor creates a new Neo4j load monitor.
// The querySem should be the same semaphore used for limiting concurrent queries.
// maxConcurrency is the maximum concurrent query limit (typically 10).
func NewNeo4jMonitor(
	driver neo4j.DriverWithContext,
	querySem chan struct{},
	maxConcurrency int,
	updateInterval time.Duration,
) *Neo4jMonitor {
	if updateInterval <= 0 {
		updateInterval = 100 * time.Millisecond
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	m := &Neo4jMonitor{
		driver:         driver,
		querySem:       querySem,
		maxConcurrency: maxConcurrency,
		latencyAlpha:   0.1, // 10% new, 90% old for smooth EMA
		stopChan:       make(chan struct{}),
		stopped:        make(chan struct{}),
		interval:       updateInterval,
	}

	// Set a default target (1.5GB)
	m.targetMemoryBytes.Store(1500 * 1024 * 1024)

	return m
}

// GetMetrics returns the current load metrics.
func (m *Neo4jMonitor) GetMetrics() loadmonitor.Metrics {
	m.metricsLock.RLock()
	defer m.metricsLock.RUnlock()
	return m.cachedMetrics
}

// RecordQueryLatency records a query latency sample using exponential moving average.
func (m *Neo4jMonitor) RecordQueryLatency(latency time.Duration) {
	ns := latency.Nanoseconds()
	for {
		old := m.queryLatencyNs.Load()
		if old == 0 {
			if m.queryLatencyNs.CompareAndSwap(0, ns) {
				return
			}
			continue
		}
		// EMA: new = alpha * sample + (1-alpha) * old
		newVal := int64(m.latencyAlpha*float64(ns) + (1-m.latencyAlpha)*float64(old))
		if m.queryLatencyNs.CompareAndSwap(old, newVal) {
			return
		}
	}
}

// RecordWriteLatency records a write latency sample using exponential moving average.
func (m *Neo4jMonitor) RecordWriteLatency(latency time.Duration) {
	ns := latency.Nanoseconds()
	for {
		old := m.writeLatencyNs.Load()
		if old == 0 {
			if m.writeLatencyNs.CompareAndSwap(0, ns) {
				return
			}
			continue
		}
		// EMA: new = alpha * sample + (1-alpha) * old
		newVal := int64(m.latencyAlpha*float64(ns) + (1-m.latencyAlpha)*float64(old))
		if m.writeLatencyNs.CompareAndSwap(old, newVal) {
			return
		}
	}
}

// SetMemoryTarget sets the target memory limit in bytes.
func (m *Neo4jMonitor) SetMemoryTarget(bytes uint64) {
	m.targetMemoryBytes.Store(bytes)
}

// Start begins background metric collection.
func (m *Neo4jMonitor) Start() <-chan struct{} {
	go m.collectLoop()
	return m.stopped
}

// Stop halts background metric collection.
func (m *Neo4jMonitor) Stop() {
	close(m.stopChan)
	<-m.stopped
}

// collectLoop periodically collects metrics.
func (m *Neo4jMonitor) collectLoop() {
	defer close(m.stopped)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.updateMetrics()
		}
	}
}

// updateMetrics collects current metrics.
func (m *Neo4jMonitor) updateMetrics() {
	metrics := loadmonitor.Metrics{
		Timestamp: time.Now(),
	}

	// Calculate memory pressure from Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	targetBytes := m.targetMemoryBytes.Load()
	if targetBytes > 0 {
		// Use HeapAlloc as primary memory metric
		metrics.MemoryPressure = float64(memStats.HeapAlloc) / float64(targetBytes)
	}

	// Calculate load from semaphore usage
	// querySem is a buffered channel - count how many slots are taken
	if m.querySem != nil {
		usedSlots := len(m.querySem)
		concurrencyLoad := float64(usedSlots) / float64(m.maxConcurrency)
		if concurrencyLoad > 1.0 {
			concurrencyLoad = 1.0
		}
		// Both read and write use the same semaphore
		metrics.WriteLoad = concurrencyLoad
		metrics.ReadLoad = concurrencyLoad
	}

	// Add latency-based load adjustment
	// High latency indicates the database is struggling
	queryLatencyNs := m.queryLatencyNs.Load()
	writeLatencyNs := m.writeLatencyNs.Load()

	// Consider > 500ms query latency as concerning
	const latencyThresholdNs = 500 * 1e6 // 500ms
	if queryLatencyNs > 0 {
		latencyLoad := float64(queryLatencyNs) / float64(latencyThresholdNs)
		if latencyLoad > 1.0 {
			latencyLoad = 1.0
		}
		// Blend concurrency and latency for read load
		metrics.ReadLoad = 0.5*metrics.ReadLoad + 0.5*latencyLoad
	}

	if writeLatencyNs > 0 {
		latencyLoad := float64(writeLatencyNs) / float64(latencyThresholdNs)
		if latencyLoad > 1.0 {
			latencyLoad = 1.0
		}
		// Blend concurrency and latency for write load
		metrics.WriteLoad = 0.5*metrics.WriteLoad + 0.5*latencyLoad
	}

	// Store latencies
	metrics.QueryLatency = time.Duration(queryLatencyNs)
	metrics.WriteLatency = time.Duration(writeLatencyNs)

	// Update cached metrics
	m.metricsLock.Lock()
	m.cachedMetrics = metrics
	m.metricsLock.Unlock()
}

// IncrementActiveReads tracks an active read operation.
// Call this when starting a read, and call the returned function when done.
func (m *Neo4jMonitor) IncrementActiveReads() func() {
	m.activeReads.Add(1)
	return func() {
		m.activeReads.Add(-1)
	}
}

// IncrementActiveWrites tracks an active write operation.
// Call this when starting a write, and call the returned function when done.
func (m *Neo4jMonitor) IncrementActiveWrites() func() {
	m.activeWrites.Add(1)
	return func() {
		m.activeWrites.Add(-1)
	}
}

// GetConcurrencyStats returns current concurrency statistics for debugging.
func (m *Neo4jMonitor) GetConcurrencyStats() (reads, writes int32, semUsed int) {
	reads = m.activeReads.Load()
	writes = m.activeWrites.Load()
	if m.querySem != nil {
		semUsed = len(m.querySem)
	}
	return
}

// CheckConnectivity performs a connectivity check to Neo4j.
// This can be used to verify the database is responsive.
func (m *Neo4jMonitor) CheckConnectivity(ctx context.Context) error {
	if m.driver == nil {
		return nil
	}
	return m.driver.VerifyConnectivity(ctx)
}
