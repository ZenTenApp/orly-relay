//go:build !(js && wasm)

package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/interfaces/loadmonitor"
)

// MemoryMonitor is a simple load monitor that only tracks process memory.
// Used for database backends that don't have their own load metrics.
type MemoryMonitor struct {
	// Configuration
	pollInterval time.Duration
	targetBytes  atomic.Uint64

	// State
	running  atomic.Bool
	stopChan chan struct{}
	doneChan chan struct{}

	// Metrics (protected by mutex)
	mu             sync.RWMutex
	currentMetrics loadmonitor.Metrics

	// Latency tracking
	queryLatencies []time.Duration
	writeLatencies []time.Duration
	latencyMu      sync.Mutex

	// Emergency mode
	emergencyThreshold float64 // e.g., 1.167 (target + 1/6)
	recoveryThreshold  float64 // e.g., 0.833 (target - 1/6)
	inEmergency        atomic.Bool
}

// NewMemoryMonitor creates a memory-only load monitor.
// pollInterval controls how often memory is sampled (recommended: 100ms).
func NewMemoryMonitor(pollInterval time.Duration) *MemoryMonitor {
	m := &MemoryMonitor{
		pollInterval:       pollInterval,
		stopChan:           make(chan struct{}),
		doneChan:           make(chan struct{}),
		queryLatencies:     make([]time.Duration, 0, 100),
		writeLatencies:     make([]time.Duration, 0, 100),
		emergencyThreshold: 1.167, // Default: target + 1/6
		recoveryThreshold:  0.833, // Default: target - 1/6
	}
	return m
}

// GetMetrics returns the current load metrics.
func (m *MemoryMonitor) GetMetrics() loadmonitor.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMetrics
}

// RecordQueryLatency records a query latency sample.
func (m *MemoryMonitor) RecordQueryLatency(latency time.Duration) {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()

	m.queryLatencies = append(m.queryLatencies, latency)
	if len(m.queryLatencies) > 100 {
		m.queryLatencies = m.queryLatencies[1:]
	}
}

// RecordWriteLatency records a write latency sample.
func (m *MemoryMonitor) RecordWriteLatency(latency time.Duration) {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()

	m.writeLatencies = append(m.writeLatencies, latency)
	if len(m.writeLatencies) > 100 {
		m.writeLatencies = m.writeLatencies[1:]
	}
}

// SetMemoryTarget sets the target memory limit in bytes.
func (m *MemoryMonitor) SetMemoryTarget(bytes uint64) {
	m.targetBytes.Store(bytes)
}

// SetEmergencyThreshold sets the memory threshold for emergency mode.
func (m *MemoryMonitor) SetEmergencyThreshold(threshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emergencyThreshold = threshold
}

// GetEmergencyThreshold returns the current emergency threshold.
func (m *MemoryMonitor) GetEmergencyThreshold() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.emergencyThreshold
}

// ForceEmergencyMode manually triggers emergency mode for a duration.
func (m *MemoryMonitor) ForceEmergencyMode(duration time.Duration) {
	m.inEmergency.Store(true)
	go func() {
		time.Sleep(duration)
		m.inEmergency.Store(false)
	}()
}

// Start begins background metric collection.
func (m *MemoryMonitor) Start() <-chan struct{} {
	if m.running.Swap(true) {
		// Already running
		return m.doneChan
	}

	go m.pollLoop()
	return m.doneChan
}

// Stop halts background metric collection.
func (m *MemoryMonitor) Stop() {
	if !m.running.Swap(false) {
		return
	}
	close(m.stopChan)
	<-m.doneChan
}

// pollLoop continuously samples memory and updates metrics.
func (m *MemoryMonitor) pollLoop() {
	defer close(m.doneChan)

	ticker := time.NewTicker(m.pollInterval)
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

// updateMetrics samples current memory and updates the metrics.
func (m *MemoryMonitor) updateMetrics() {
	target := m.targetBytes.Load()
	if target == 0 {
		target = 1 // Avoid division by zero
	}

	// Get physical memory using the same method as other monitors
	procMem := ReadProcessMemoryStats()
	physicalMemBytes := procMem.PhysicalMemoryBytes()
	physicalMemMB := physicalMemBytes / (1024 * 1024)

	// Calculate memory pressure
	memPressure := float64(physicalMemBytes) / float64(target)

	// Check emergency mode thresholds
	m.mu.RLock()
	emergencyThreshold := m.emergencyThreshold
	recoveryThreshold := m.recoveryThreshold
	m.mu.RUnlock()

	wasEmergency := m.inEmergency.Load()
	if memPressure > emergencyThreshold {
		m.inEmergency.Store(true)
	} else if memPressure < recoveryThreshold && wasEmergency {
		m.inEmergency.Store(false)
	}

	// Calculate average latencies
	m.latencyMu.Lock()
	var avgQuery, avgWrite time.Duration
	if len(m.queryLatencies) > 0 {
		var total time.Duration
		for _, l := range m.queryLatencies {
			total += l
		}
		avgQuery = total / time.Duration(len(m.queryLatencies))
	}
	if len(m.writeLatencies) > 0 {
		var total time.Duration
		for _, l := range m.writeLatencies {
			total += l
		}
		avgWrite = total / time.Duration(len(m.writeLatencies))
	}
	m.latencyMu.Unlock()

	// Update metrics
	m.mu.Lock()
	m.currentMetrics = loadmonitor.Metrics{
		MemoryPressure:    memPressure,
		WriteLoad:         0,    // No database-specific load metric
		ReadLoad:          0,    // No database-specific load metric
		QueryLatency:      avgQuery,
		WriteLatency:      avgWrite,
		Timestamp:         time.Now(),
		InEmergencyMode:   m.inEmergency.Load(),
		CompactionPending: false, // memory-only monitor doesn't track compaction
		PhysicalMemoryMB:  physicalMemMB,
	}
	m.mu.Unlock()
}

// Ensure MemoryMonitor implements the required interfaces
var _ loadmonitor.Monitor = (*MemoryMonitor)(nil)
var _ loadmonitor.EmergencyModeMonitor = (*MemoryMonitor)(nil)
