package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/interfaces/loadmonitor"
)

// Neo4jMonitor implements loadmonitor.Monitor for Neo4j database.
// Since Neo4j driver doesn't expose detailed metrics, we track:
// - Memory pressure via actual RSS (not Go runtime)
// - Query concurrency via the semaphore
// - Latency via recording
//
// This monitor implements aggressive memory-based limiting:
// When memory exceeds the target, it applies 50% more aggressive throttling.
// It rechecks every 10 seconds and doubles the throttling multiplier until
// memory returns under target.
type Neo4jMonitor struct {
	driver   neo4j.DriverWithContext
	querySem chan struct{} // Reference to the query semaphore

	// Target memory for pressure calculation
	targetMemoryBytes atomic.Uint64

	// Emergency mode configuration
	emergencyThreshold atomic.Uint64 // stored as threshold * 1000 (e.g., 1500 = 1.5)
	emergencyModeUntil atomic.Int64  // Unix nano when forced emergency mode ends
	inEmergencyMode    atomic.Bool

	// Aggressive throttling multiplier for Neo4j
	// Starts at 1.5 (50% more aggressive), doubles every 10 seconds while over limit
	throttleMultiplier atomic.Uint64 // stored as multiplier * 100 (e.g., 150 = 1.5x)
	lastThrottleCheck  atomic.Int64  // Unix nano timestamp

	// Latency tracking with exponential moving average
	queryLatencyNs atomic.Int64
	writeLatencyNs atomic.Int64
	latencyAlpha   float64 // EMA coefficient (default 0.1)

	// Concurrency tracking
	activeReads    atomic.Int32
	activeWrites   atomic.Int32
	maxConcurrency int

	// Cached metrics (updated by background goroutine)
	metricsLock   sync.RWMutex
	cachedMetrics loadmonitor.Metrics

	// Background collection
	stopChan chan struct{}
	stopped  chan struct{}
	interval time.Duration
}

// Compile-time checks for interface implementation
var _ loadmonitor.Monitor = (*Neo4jMonitor)(nil)
var _ loadmonitor.EmergencyModeMonitor = (*Neo4jMonitor)(nil)

// ThrottleCheckInterval is how often to recheck memory and adjust throttling
const ThrottleCheckInterval = 10 * time.Second

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

	// Default emergency threshold: 100% of target (same as target for Neo4j)
	m.emergencyThreshold.Store(1000)

	// Start with 1.0x multiplier (no throttling)
	m.throttleMultiplier.Store(100)

	return m
}

// SetEmergencyThreshold sets the memory threshold above which emergency mode is triggered.
// threshold is a fraction, e.g., 1.0 = 100% of target memory.
func (m *Neo4jMonitor) SetEmergencyThreshold(threshold float64) {
	m.emergencyThreshold.Store(uint64(threshold * 1000))
}

// GetEmergencyThreshold returns the current emergency threshold as a fraction.
func (m *Neo4jMonitor) GetEmergencyThreshold() float64 {
	return float64(m.emergencyThreshold.Load()) / 1000.0
}

// ForceEmergencyMode manually triggers emergency mode for a duration.
func (m *Neo4jMonitor) ForceEmergencyMode(duration time.Duration) {
	m.emergencyModeUntil.Store(time.Now().Add(duration).UnixNano())
	m.inEmergencyMode.Store(true)
	m.throttleMultiplier.Store(150) // Start at 1.5x
	log.W.F("⚠️  Neo4j emergency mode forced for %v", duration)
}

// GetThrottleMultiplier returns the current throttle multiplier.
// Returns a value >= 1.0, where 1.0 = no extra throttling, 1.5 = 50% more aggressive, etc.
func (m *Neo4jMonitor) GetThrottleMultiplier() float64 {
	return float64(m.throttleMultiplier.Load()) / 100.0
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

// updateMetrics collects current metrics and manages aggressive throttling.
func (m *Neo4jMonitor) updateMetrics() {
	metrics := loadmonitor.Metrics{
		Timestamp: time.Now(),
	}

	// Use RSS-based memory pressure (actual physical memory, not Go runtime)
	procMem := ReadProcessMemoryStats()
	physicalMemBytes := procMem.PhysicalMemoryBytes()
	metrics.PhysicalMemoryMB = physicalMemBytes / (1024 * 1024)

	targetBytes := m.targetMemoryBytes.Load()
	if targetBytes > 0 {
		// Use actual physical memory (RSS - shared) for pressure calculation
		metrics.MemoryPressure = float64(physicalMemBytes) / float64(targetBytes)
	}

	// Check and update emergency mode with aggressive throttling
	m.updateEmergencyMode(metrics.MemoryPressure)
	metrics.InEmergencyMode = m.inEmergencyMode.Load()

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

	// Apply throttle multiplier to loads when in emergency mode
	// This makes the PID controller think load is higher, causing more throttling
	if metrics.InEmergencyMode {
		multiplier := m.GetThrottleMultiplier()
		metrics.WriteLoad = metrics.WriteLoad * multiplier
		if metrics.WriteLoad > 1.0 {
			metrics.WriteLoad = 1.0
		}
		metrics.ReadLoad = metrics.ReadLoad * multiplier
		if metrics.ReadLoad > 1.0 {
			metrics.ReadLoad = 1.0
		}
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

// updateEmergencyMode manages the emergency mode state and throttle multiplier.
// When memory exceeds the target:
// - Enters emergency mode with 1.5x throttle multiplier (50% more aggressive)
// - Every 10 seconds while still over limit, doubles the multiplier
// - When memory returns under target, resets to normal
func (m *Neo4jMonitor) updateEmergencyMode(memoryPressure float64) {
	threshold := float64(m.emergencyThreshold.Load()) / 1000.0
	forcedUntil := m.emergencyModeUntil.Load()
	now := time.Now().UnixNano()

	// Check if in forced emergency mode
	if forcedUntil > now {
		return // Stay in forced mode
	}

	// Check if memory exceeds threshold
	if memoryPressure >= threshold {
		if !m.inEmergencyMode.Load() {
			// Entering emergency mode - start at 1.5x (50% more aggressive)
			m.inEmergencyMode.Store(true)
			m.throttleMultiplier.Store(150)
			m.lastThrottleCheck.Store(now)
			log.W.F("⚠️  Neo4j entering emergency mode: memory %.1f%% >= threshold %.1f%%, throttle 1.5x",
				memoryPressure*100, threshold*100)
			return
		}

		// Already in emergency mode - check if it's time to double throttling
		lastCheck := m.lastThrottleCheck.Load()
		elapsed := time.Duration(now - lastCheck)

		if elapsed >= ThrottleCheckInterval {
			// Double the throttle multiplier
			currentMult := m.throttleMultiplier.Load()
			newMult := currentMult * 2
			if newMult > 1600 { // Cap at 16x to prevent overflow
				newMult = 1600
			}
			m.throttleMultiplier.Store(newMult)
			m.lastThrottleCheck.Store(now)
			log.W.F("⚠️  Neo4j still over memory limit: %.1f%%, doubling throttle to %.1fx",
				memoryPressure*100, float64(newMult)/100.0)
		}
	} else {
		// Memory is under threshold
		if m.inEmergencyMode.Load() {
			m.inEmergencyMode.Store(false)
			m.throttleMultiplier.Store(100) // Reset to 1.0x
			log.I.F("✅ Neo4j exiting emergency mode: memory %.1f%% < threshold %.1f%%",
				memoryPressure*100, threshold*100)
		}
	}
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
