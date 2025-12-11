//go:build !(js && wasm)

package ratelimit

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/interfaces/loadmonitor"
)

// BadgerMonitor implements loadmonitor.Monitor for the Badger database.
// It collects metrics from Badger's LSM tree, caches, and Go runtime.
type BadgerMonitor struct {
	db *badger.DB

	// Target memory for pressure calculation
	targetMemoryBytes atomic.Uint64

	// Latency tracking with exponential moving average
	queryLatencyNs atomic.Int64
	writeLatencyNs atomic.Int64
	latencyAlpha   float64 // EMA coefficient (default 0.1)

	// Cached metrics (updated by background goroutine)
	metricsLock    sync.RWMutex
	cachedMetrics  loadmonitor.Metrics
	lastL0Tables   int
	lastL0Score    float64

	// Background collection
	stopChan chan struct{}
	stopped  chan struct{}
	interval time.Duration
}

// Compile-time check that BadgerMonitor implements loadmonitor.Monitor
var _ loadmonitor.Monitor = (*BadgerMonitor)(nil)

// NewBadgerMonitor creates a new Badger load monitor.
// The updateInterval controls how often metrics are collected (default 100ms).
func NewBadgerMonitor(db *badger.DB, updateInterval time.Duration) *BadgerMonitor {
	if updateInterval <= 0 {
		updateInterval = 100 * time.Millisecond
	}

	m := &BadgerMonitor{
		db:           db,
		latencyAlpha: 0.1, // 10% new, 90% old for smooth EMA
		stopChan:     make(chan struct{}),
		stopped:      make(chan struct{}),
		interval:     updateInterval,
	}

	// Set a default target (1.5GB)
	m.targetMemoryBytes.Store(1500 * 1024 * 1024)

	return m
}

// GetMetrics returns the current load metrics.
func (m *BadgerMonitor) GetMetrics() loadmonitor.Metrics {
	m.metricsLock.RLock()
	defer m.metricsLock.RUnlock()
	return m.cachedMetrics
}

// RecordQueryLatency records a query latency sample using exponential moving average.
func (m *BadgerMonitor) RecordQueryLatency(latency time.Duration) {
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
func (m *BadgerMonitor) RecordWriteLatency(latency time.Duration) {
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
func (m *BadgerMonitor) SetMemoryTarget(bytes uint64) {
	m.targetMemoryBytes.Store(bytes)
}

// Start begins background metric collection.
func (m *BadgerMonitor) Start() <-chan struct{} {
	go m.collectLoop()
	return m.stopped
}

// Stop halts background metric collection.
func (m *BadgerMonitor) Stop() {
	close(m.stopChan)
	<-m.stopped
}

// collectLoop periodically collects metrics from Badger.
func (m *BadgerMonitor) collectLoop() {
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

// updateMetrics collects current metrics from Badger and runtime.
func (m *BadgerMonitor) updateMetrics() {
	if m.db == nil || m.db.IsClosed() {
		return
	}

	metrics := loadmonitor.Metrics{
		Timestamp: time.Now(),
	}

	// Calculate memory pressure from Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	targetBytes := m.targetMemoryBytes.Load()
	if targetBytes > 0 {
		// Use HeapAlloc as primary memory metric
		// This represents the actual live heap objects
		metrics.MemoryPressure = float64(memStats.HeapAlloc) / float64(targetBytes)
	}

	// Get Badger LSM tree information for write load
	levels := m.db.Levels()
	var l0Tables int
	var maxScore float64

	for _, level := range levels {
		if level.Level == 0 {
			l0Tables = level.NumTables
		}
		if level.Score > maxScore {
			maxScore = level.Score
		}
	}

	// Calculate write load based on L0 tables and compaction score
	// L0 tables stall at NumLevelZeroTablesStall (default 16)
	// We consider write pressure high when approaching that limit
	const l0StallThreshold = 16
	l0Load := float64(l0Tables) / float64(l0StallThreshold)
	if l0Load > 1.0 {
		l0Load = 1.0
	}

	// Compaction score > 1.0 means compaction is needed
	// We blend L0 tables and compaction score for write load
	compactionLoad := maxScore / 2.0 // Score of 2.0 = fully loaded
	if compactionLoad > 1.0 {
		compactionLoad = 1.0
	}

	// Blend: 60% L0 (immediate backpressure), 40% compaction score
	metrics.WriteLoad = 0.6*l0Load + 0.4*compactionLoad

	// Calculate read load from cache metrics
	blockMetrics := m.db.BlockCacheMetrics()
	indexMetrics := m.db.IndexCacheMetrics()

	var blockHitRatio, indexHitRatio float64
	if blockMetrics != nil {
		blockHitRatio = blockMetrics.Ratio()
	}
	if indexMetrics != nil {
		indexHitRatio = indexMetrics.Ratio()
	}

	// Average cache hit ratio (0 = no hits = high load, 1 = all hits = low load)
	avgHitRatio := (blockHitRatio + indexHitRatio) / 2.0

	// Invert: low hit ratio = high read load
	// Use 0.5 as the threshold (below 50% hit ratio is concerning)
	if avgHitRatio < 0.5 {
		metrics.ReadLoad = 1.0 - avgHitRatio*2 // 0% hits = 1.0 load, 50% hits = 0.0 load
	} else {
		metrics.ReadLoad = 0 // Above 50% hit ratio = minimal load
	}

	// Store latencies
	metrics.QueryLatency = time.Duration(m.queryLatencyNs.Load())
	metrics.WriteLatency = time.Duration(m.writeLatencyNs.Load())

	// Update cached metrics
	m.metricsLock.Lock()
	m.cachedMetrics = metrics
	m.lastL0Tables = l0Tables
	m.lastL0Score = maxScore
	m.metricsLock.Unlock()
}

// GetL0Stats returns L0-specific statistics for debugging.
func (m *BadgerMonitor) GetL0Stats() (tables int, score float64) {
	m.metricsLock.RLock()
	defer m.metricsLock.RUnlock()
	return m.lastL0Tables, m.lastL0Score
}
