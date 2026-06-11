//go:build !(js && wasm)

package ratelimit

import (
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/interfaces/loadmonitor"
)

// MemoryMonitor actor request types
type (
	memGetMetricsReq struct {
		resp chan loadmonitor.Metrics
	}
	memRecordLatencyReq struct {
		latency time.Duration
		isWrite bool
	}
	memSetEmergencyThresholdReq struct {
		threshold float64
	}
	memGetEmergencyThresholdReq struct {
		resp chan float64
	}
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

	// Actor channels
	getMetricsCh             chan memGetMetricsReq
	recordLatencyCh          chan memRecordLatencyReq
	setEmergencyThresholdCh  chan memSetEmergencyThresholdReq
	getEmergencyThresholdCh  chan memGetEmergencyThresholdReq

	// Emergency mode
	inEmergency atomic.Bool
}

// NewMemoryMonitor creates a memory-only load monitor.
// pollInterval controls how often memory is sampled (recommended: 100ms).
func NewMemoryMonitor(pollInterval time.Duration) *MemoryMonitor {
	m := &MemoryMonitor{
		pollInterval:            pollInterval,
		stopChan:                make(chan struct{}),
		doneChan:                make(chan struct{}),
		getMetricsCh:            make(chan memGetMetricsReq, 1),
		recordLatencyCh:         make(chan memRecordLatencyReq, 16),
		setEmergencyThresholdCh: make(chan memSetEmergencyThresholdReq, 16),
		getEmergencyThresholdCh: make(chan memGetEmergencyThresholdReq, 1),
	}
	return m
}

// GetMetrics returns the current load metrics.
func (m *MemoryMonitor) GetMetrics() loadmonitor.Metrics {
	resp := make(chan loadmonitor.Metrics, 1)
	m.getMetricsCh <- memGetMetricsReq{resp: resp}
	return <-resp
}

// RecordQueryLatency records a query latency sample.
func (m *MemoryMonitor) RecordQueryLatency(latency time.Duration) {
	select {
	case m.recordLatencyCh <- memRecordLatencyReq{latency: latency, isWrite: false}:
	default:
		// Drop if buffer full
	}
}

// RecordWriteLatency records a write latency sample.
func (m *MemoryMonitor) RecordWriteLatency(latency time.Duration) {
	select {
	case m.recordLatencyCh <- memRecordLatencyReq{latency: latency, isWrite: true}:
	default:
		// Drop if buffer full
	}
}

// SetMemoryTarget sets the target memory limit in bytes.
func (m *MemoryMonitor) SetMemoryTarget(bytes uint64) {
	m.targetBytes.Store(bytes)
}

// SetEmergencyThreshold sets the memory threshold for emergency mode.
func (m *MemoryMonitor) SetEmergencyThreshold(threshold float64) {
	select {
	case m.setEmergencyThresholdCh <- memSetEmergencyThresholdReq{threshold: threshold}:
	default:
	}
}

// GetEmergencyThreshold returns the current emergency threshold.
func (m *MemoryMonitor) GetEmergencyThreshold() float64 {
	resp := make(chan float64, 1)
	m.getEmergencyThresholdCh <- memGetEmergencyThresholdReq{resp: resp}
	return <-resp
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

// pollLoop continuously samples memory, manages latencies, and serves actor requests.
func (m *MemoryMonitor) pollLoop() {
	defer close(m.doneChan)

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Actor-owned state
	var currentMetrics loadmonitor.Metrics
	queryLatencies := make([]time.Duration, 0, 100)
	writeLatencies := make([]time.Duration, 0, 100)
	emergencyThreshold := 1.167 // Default: target + 1/6
	recoveryThreshold := 0.833  // Default: target - 1/6

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			currentMetrics = m.computeMetrics(
				queryLatencies, writeLatencies,
				emergencyThreshold, recoveryThreshold,
			)
		case req := <-m.getMetricsCh:
			req.resp <- currentMetrics
		case req := <-m.recordLatencyCh:
			if req.isWrite {
				writeLatencies = append(writeLatencies, req.latency)
				if len(writeLatencies) > 100 {
					writeLatencies = writeLatencies[1:]
				}
			} else {
				queryLatencies = append(queryLatencies, req.latency)
				if len(queryLatencies) > 100 {
					queryLatencies = queryLatencies[1:]
				}
			}
		case req := <-m.setEmergencyThresholdCh:
			emergencyThreshold = req.threshold
		case req := <-m.getEmergencyThresholdCh:
			req.resp <- emergencyThreshold
		}
	}
}

// computeMetrics samples current memory and computes metrics.
// Called only from the actor goroutine.
func (m *MemoryMonitor) computeMetrics(
	queryLatencies, writeLatencies []time.Duration,
	emergencyThreshold, recoveryThreshold float64,
) loadmonitor.Metrics {
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
	wasEmergency := m.inEmergency.Load()
	if memPressure > emergencyThreshold {
		m.inEmergency.Store(true)
	} else if memPressure < recoveryThreshold && wasEmergency {
		m.inEmergency.Store(false)
	}

	// Calculate average latencies
	var avgQuery, avgWrite time.Duration
	if len(queryLatencies) > 0 {
		var total time.Duration
		for _, l := range queryLatencies {
			total += l
		}
		avgQuery = total / time.Duration(len(queryLatencies))
	}
	if len(writeLatencies) > 0 {
		var total time.Duration
		for _, l := range writeLatencies {
			total += l
		}
		avgWrite = total / time.Duration(len(writeLatencies))
	}

	return loadmonitor.Metrics{
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
}

// Ensure MemoryMonitor implements the required interfaces
var _ loadmonitor.Monitor = (*MemoryMonitor)(nil)
var _ loadmonitor.EmergencyModeMonitor = (*MemoryMonitor)(nil)
