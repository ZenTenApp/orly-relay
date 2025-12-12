package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/interfaces/loadmonitor"
	pidif "next.orly.dev/pkg/interfaces/pid"
	"next.orly.dev/pkg/pid"
)

// OperationType distinguishes between read and write operations
// for applying different rate limiting strategies.
type OperationType int

const (
	// Read operations (REQ queries)
	Read OperationType = iota
	// Write operations (EVENT saves, imports)
	Write
)

// String returns a human-readable name for the operation type.
func (o OperationType) String() string {
	switch o {
	case Read:
		return "read"
	case Write:
		return "write"
	default:
		return "unknown"
	}
}

// Config holds configuration for the adaptive rate limiter.
type Config struct {
	// Enabled controls whether rate limiting is active.
	Enabled bool

	// TargetMemoryMB is the target memory limit in megabytes.
	// Memory pressure is calculated relative to this target.
	TargetMemoryMB int

	// WriteSetpoint is the target process variable for writes (0.0-1.0).
	// Default: 0.85 (throttle when load exceeds 85%)
	WriteSetpoint float64

	// ReadSetpoint is the target process variable for reads (0.0-1.0).
	// Default: 0.90 (more tolerant for reads)
	ReadSetpoint float64

	// PID gains for writes
	WriteKp float64
	WriteKi float64
	WriteKd float64

	// PID gains for reads
	ReadKp float64
	ReadKi float64
	ReadKd float64

	// MaxWriteDelayMs is the maximum delay for write operations in milliseconds.
	MaxWriteDelayMs int

	// MaxReadDelayMs is the maximum delay for read operations in milliseconds.
	MaxReadDelayMs int

	// MetricUpdateInterval is how often to poll the load monitor.
	MetricUpdateInterval time.Duration

	// MemoryWeight is the weight given to memory pressure in process variable (0.0-1.0).
	// The remaining weight is given to the load metric.
	// Default: 0.7 (70% memory, 30% load)
	MemoryWeight float64

	// EmergencyThreshold is the memory pressure level (fraction of target) that triggers emergency mode.
	// Default: 1.167 (116.7% = target + 1/6th)
	// When exceeded, writes are aggressively throttled until memory drops below RecoveryThreshold.
	EmergencyThreshold float64

	// RecoveryThreshold is the memory pressure level below which we exit emergency mode.
	// Default: 0.833 (83.3% = target - 1/6th)
	// Hysteresis prevents rapid oscillation between normal and emergency modes.
	RecoveryThreshold float64

	// EmergencyMaxDelayMs is the maximum delay for writes during emergency mode.
	// Default: 5000 (5 seconds) - much longer than normal MaxWriteDelayMs
	EmergencyMaxDelayMs int

	// CompactionCheckInterval controls how often to check if compaction should be triggered.
	// Default: 10 seconds
	CompactionCheckInterval time.Duration
}

// DefaultConfig returns a default configuration for the rate limiter.
func DefaultConfig() Config {
	return Config{
		Enabled:                 true,
		TargetMemoryMB:          1500, // 1.5GB target
		WriteSetpoint:           0.85,
		ReadSetpoint:            0.90,
		WriteKp:                 0.5,
		WriteKi:                 0.1,
		WriteKd:                 0.05,
		ReadKp:                  0.3,
		ReadKi:                  0.05,
		ReadKd:                  0.02,
		MaxWriteDelayMs:         1000, // 1 second max
		MaxReadDelayMs:          500,  // 500ms max
		MetricUpdateInterval:    100 * time.Millisecond,
		MemoryWeight:            0.7,
		EmergencyThreshold:      1.167, // Target + 1/6th (~1.75GB for 1.5GB target)
		RecoveryThreshold:       0.833, // Target - 1/6th (~1.25GB for 1.5GB target)
		EmergencyMaxDelayMs:     5000,  // 5 seconds max in emergency mode
		CompactionCheckInterval: 10 * time.Second,
	}
}

// NewConfigFromValues creates a Config from individual configuration values.
// This is useful when loading configuration from environment variables.
func NewConfigFromValues(
	enabled bool,
	targetMB int,
	writeKp, writeKi, writeKd float64,
	readKp, readKi, readKd float64,
	maxWriteMs, maxReadMs int,
	writeTarget, readTarget float64,
	emergencyThreshold, recoveryThreshold float64,
	emergencyMaxMs int,
) Config {
	// Apply defaults for zero values
	if emergencyThreshold == 0 {
		emergencyThreshold = 1.167 // Target + 1/6th
	}
	if recoveryThreshold == 0 {
		recoveryThreshold = 0.833 // Target - 1/6th
	}
	if emergencyMaxMs == 0 {
		emergencyMaxMs = 5000 // 5 seconds
	}

	return Config{
		Enabled:                 enabled,
		TargetMemoryMB:          targetMB,
		WriteSetpoint:           writeTarget,
		ReadSetpoint:            readTarget,
		WriteKp:                 writeKp,
		WriteKi:                 writeKi,
		WriteKd:                 writeKd,
		ReadKp:                  readKp,
		ReadKi:                  readKi,
		ReadKd:                  readKd,
		MaxWriteDelayMs:         maxWriteMs,
		MaxReadDelayMs:          maxReadMs,
		MetricUpdateInterval:    100 * time.Millisecond,
		MemoryWeight:            0.7,
		EmergencyThreshold:      emergencyThreshold,
		RecoveryThreshold:       recoveryThreshold,
		EmergencyMaxDelayMs:     emergencyMaxMs,
		CompactionCheckInterval: 10 * time.Second,
	}
}

// Limiter implements adaptive rate limiting using PID control.
// It monitors database load metrics and computes appropriate delays
// to keep the system within its target operating range.
type Limiter struct {
	config  Config
	monitor loadmonitor.Monitor

	// PID controllers for reads and writes (using generic pid.Controller)
	writePID pidif.Controller
	readPID  pidif.Controller

	// Cached metrics (updated periodically)
	metricsLock    sync.RWMutex
	currentMetrics loadmonitor.Metrics

	// Emergency mode tracking with hysteresis
	inEmergencyMode     atomic.Bool
	lastEmergencyCheck  atomic.Int64 // Unix nano timestamp
	compactionTriggered atomic.Bool

	// Statistics
	totalWriteDelayMs atomic.Int64
	totalReadDelayMs  atomic.Int64
	writeThrottles    atomic.Int64
	readThrottles     atomic.Int64
	emergencyEvents   atomic.Int64

	// Lifecycle
	ctx       context.Context
	cancel    context.CancelFunc
	stopOnce  sync.Once
	stopped   chan struct{}
	wg        sync.WaitGroup
}

// NewLimiter creates a new adaptive rate limiter.
// If monitor is nil, the limiter will be disabled.
func NewLimiter(config Config, monitor loadmonitor.Monitor) *Limiter {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults for zero values
	if config.EmergencyThreshold == 0 {
		config.EmergencyThreshold = 1.167 // Target + 1/6th
	}
	if config.RecoveryThreshold == 0 {
		config.RecoveryThreshold = 0.833 // Target - 1/6th
	}
	if config.EmergencyMaxDelayMs == 0 {
		config.EmergencyMaxDelayMs = 5000 // 5 seconds
	}
	if config.CompactionCheckInterval == 0 {
		config.CompactionCheckInterval = 10 * time.Second
	}

	l := &Limiter{
		config:  config,
		monitor: monitor,
		ctx:     ctx,
		cancel:  cancel,
		stopped: make(chan struct{}),
	}

	// Create PID controllers with configured gains using the generic pid package
	l.writePID = pid.New(pidif.Tuning{
		Kp:                    config.WriteKp,
		Ki:                    config.WriteKi,
		Kd:                    config.WriteKd,
		Setpoint:              config.WriteSetpoint,
		DerivativeFilterAlpha: 0.2, // Strong filtering for writes
		IntegralMin:           -2.0,
		IntegralMax:           float64(config.MaxWriteDelayMs) / 1000.0 * 2, // Anti-windup limits
		OutputMin:             0,
		OutputMax:             float64(config.MaxWriteDelayMs) / 1000.0,
	})

	l.readPID = pid.New(pidif.Tuning{
		Kp:                    config.ReadKp,
		Ki:                    config.ReadKi,
		Kd:                    config.ReadKd,
		Setpoint:              config.ReadSetpoint,
		DerivativeFilterAlpha: 0.15, // Very strong filtering for reads
		IntegralMin:           -1.0,
		IntegralMax:           float64(config.MaxReadDelayMs) / 1000.0 * 2,
		OutputMin:             0,
		OutputMax:             float64(config.MaxReadDelayMs) / 1000.0,
	})

	// Set memory target on monitor
	if monitor != nil && config.TargetMemoryMB > 0 {
		monitor.SetMemoryTarget(uint64(config.TargetMemoryMB) * 1024 * 1024)
	}

	// Configure emergency threshold if monitor supports it
	if emMon, ok := monitor.(loadmonitor.EmergencyModeMonitor); ok {
		emMon.SetEmergencyThreshold(config.EmergencyThreshold)
	}

	return l
}

// Start begins the rate limiter's background metric collection.
func (l *Limiter) Start() {
	if l.monitor == nil || !l.config.Enabled {
		return
	}

	// Start the monitor
	l.monitor.Start()

	// Start metric update loop
	l.wg.Add(1)
	go l.updateLoop()
}

// updateLoop periodically fetches metrics from the monitor.
func (l *Limiter) updateLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.MetricUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			if l.monitor != nil {
				metrics := l.monitor.GetMetrics()
				l.metricsLock.Lock()
				l.currentMetrics = metrics
				l.metricsLock.Unlock()
			}
		}
	}
}

// Stop halts the rate limiter.
func (l *Limiter) Stop() {
	l.stopOnce.Do(func() {
		l.cancel()
		if l.monitor != nil {
			l.monitor.Stop()
		}
		l.wg.Wait()
		close(l.stopped)
	})
}

// Stopped returns a channel that closes when the limiter has stopped.
func (l *Limiter) Stopped() <-chan struct{} {
	return l.stopped
}

// Wait blocks until the rate limiter permits the operation to proceed.
// It returns the delay that was applied, or 0 if no delay was needed.
// If the context is cancelled, it returns immediately.
// opType accepts int for interface compatibility (0=Read, 1=Write)
func (l *Limiter) Wait(ctx context.Context, opType int) time.Duration {
	if !l.config.Enabled || l.monitor == nil {
		return 0
	}

	delay := l.ComputeDelay(OperationType(opType))
	if delay <= 0 {
		return 0
	}

	// Apply the delay
	select {
	case <-ctx.Done():
		return 0
	case <-time.After(delay):
		return delay
	}
}

// ComputeDelay calculates the recommended delay for an operation.
// This can be used to check the delay without actually waiting.
func (l *Limiter) ComputeDelay(opType OperationType) time.Duration {
	if !l.config.Enabled || l.monitor == nil {
		return 0
	}

	// Get current metrics
	l.metricsLock.RLock()
	metrics := l.currentMetrics
	l.metricsLock.RUnlock()

	// Check emergency mode with hysteresis
	inEmergency := l.checkEmergencyMode(metrics.MemoryPressure)

	// Compute process variable as weighted combination of memory and load
	var loadMetric float64
	switch opType {
	case Write:
		loadMetric = metrics.WriteLoad
	case Read:
		loadMetric = metrics.ReadLoad
	}

	// Combine memory pressure and load
	// Process variable = memoryWeight * memoryPressure + (1-memoryWeight) * loadMetric
	pv := l.config.MemoryWeight*metrics.MemoryPressure + (1-l.config.MemoryWeight)*loadMetric

	// Select the appropriate PID controller
	var delaySec float64
	switch opType {
	case Write:
		out := l.writePID.UpdateValue(pv)
		delaySec = out.Value()

		// In emergency mode, apply progressive throttling for writes
		if inEmergency {
			// Calculate how far above recovery threshold we are
			// At emergency threshold, add 1x normal delay
			// For every additional 10% above emergency, double the delay
			excessPressure := metrics.MemoryPressure - l.config.RecoveryThreshold
			if excessPressure > 0 {
				// Progressive multiplier: starts at 2x, doubles every 10% excess
				multiplier := 2.0
				for excess := excessPressure; excess > 0.1; excess -= 0.1 {
					multiplier *= 2
				}

				emergencyDelaySec := delaySec * multiplier
				maxEmergencySec := float64(l.config.EmergencyMaxDelayMs) / 1000.0

				if emergencyDelaySec > maxEmergencySec {
					emergencyDelaySec = maxEmergencySec
				}
				// Minimum emergency delay of 100ms to allow other operations
				if emergencyDelaySec < 0.1 {
					emergencyDelaySec = 0.1
				}
				delaySec = emergencyDelaySec
			}
		}

		if delaySec > 0 {
			l.writeThrottles.Add(1)
			l.totalWriteDelayMs.Add(int64(delaySec * 1000))
		}
	case Read:
		out := l.readPID.UpdateValue(pv)
		delaySec = out.Value()
		if delaySec > 0 {
			l.readThrottles.Add(1)
			l.totalReadDelayMs.Add(int64(delaySec * 1000))
		}
	}

	if delaySec <= 0 {
		return 0
	}

	return time.Duration(delaySec * float64(time.Second))
}

// checkEmergencyMode implements hysteresis-based emergency mode detection.
// Enters emergency mode when memory pressure >= EmergencyThreshold.
// Exits emergency mode when memory pressure <= RecoveryThreshold.
func (l *Limiter) checkEmergencyMode(memoryPressure float64) bool {
	wasInEmergency := l.inEmergencyMode.Load()

	if wasInEmergency {
		// To exit, must drop below recovery threshold
		if memoryPressure <= l.config.RecoveryThreshold {
			l.inEmergencyMode.Store(false)
			log.I.F("✅ exiting emergency mode: memory %.1f%% <= recovery threshold %.1f%%",
				memoryPressure*100, l.config.RecoveryThreshold*100)
			return false
		}
		return true
	}

	// To enter, must exceed emergency threshold
	if memoryPressure >= l.config.EmergencyThreshold {
		l.inEmergencyMode.Store(true)
		l.emergencyEvents.Add(1)
		log.W.F("⚠️  entering emergency mode: memory %.1f%% >= threshold %.1f%%",
			memoryPressure*100, l.config.EmergencyThreshold*100)

		// Trigger compaction if supported
		l.triggerCompactionIfNeeded()
		return true
	}

	return false
}

// triggerCompactionIfNeeded triggers database compaction if the monitor supports it
// and compaction isn't already in progress.
func (l *Limiter) triggerCompactionIfNeeded() {
	if l.compactionTriggered.Load() {
		return // Already triggered
	}

	compactMon, ok := l.monitor.(loadmonitor.CompactableMonitor)
	if !ok {
		return // Monitor doesn't support compaction
	}

	if compactMon.IsCompacting() {
		return // Already compacting
	}

	l.compactionTriggered.Store(true)
	go func() {
		defer l.compactionTriggered.Store(false)
		if err := compactMon.TriggerCompaction(); err != nil {
			log.E.F("compaction failed: %v", err)
		}
	}()
}

// InEmergencyMode returns true if the limiter is currently in emergency mode.
func (l *Limiter) InEmergencyMode() bool {
	return l.inEmergencyMode.Load()
}

// RecordLatency records an operation latency for the monitor.
func (l *Limiter) RecordLatency(opType OperationType, latency time.Duration) {
	if l.monitor == nil {
		return
	}

	switch opType {
	case Write:
		l.monitor.RecordWriteLatency(latency)
	case Read:
		l.monitor.RecordQueryLatency(latency)
	}
}

// Stats returns rate limiter statistics.
type Stats struct {
	WriteThrottles    int64
	ReadThrottles     int64
	TotalWriteDelayMs int64
	TotalReadDelayMs  int64
	EmergencyEvents   int64
	InEmergencyMode   bool
	CurrentMetrics    loadmonitor.Metrics
	WritePIDState     PIDState
	ReadPIDState      PIDState
}

// PIDState contains the internal state of a PID controller.
type PIDState struct {
	Integral          float64
	PrevError         float64
	PrevFilteredError float64
}

// GetStats returns current rate limiter statistics.
func (l *Limiter) GetStats() Stats {
	l.metricsLock.RLock()
	metrics := l.currentMetrics
	l.metricsLock.RUnlock()

	stats := Stats{
		WriteThrottles:    l.writeThrottles.Load(),
		ReadThrottles:     l.readThrottles.Load(),
		TotalWriteDelayMs: l.totalWriteDelayMs.Load(),
		TotalReadDelayMs:  l.totalReadDelayMs.Load(),
		EmergencyEvents:   l.emergencyEvents.Load(),
		InEmergencyMode:   l.inEmergencyMode.Load(),
		CurrentMetrics:    metrics,
	}

	// Type assert to concrete pid.Controller to access State() method
	// This is for monitoring/debugging only
	if wCtrl, ok := l.writePID.(*pid.Controller); ok {
		integral, prevErr, prevFiltered, _ := wCtrl.State()
		stats.WritePIDState = PIDState{
			Integral:          integral,
			PrevError:         prevErr,
			PrevFilteredError: prevFiltered,
		}
	}
	if rCtrl, ok := l.readPID.(*pid.Controller); ok {
		integral, prevErr, prevFiltered, _ := rCtrl.State()
		stats.ReadPIDState = PIDState{
			Integral:          integral,
			PrevError:         prevErr,
			PrevFilteredError: prevFiltered,
		}
	}

	return stats
}

// Reset clears all PID controller state and statistics.
func (l *Limiter) Reset() {
	l.writePID.Reset()
	l.readPID.Reset()
	l.writeThrottles.Store(0)
	l.readThrottles.Store(0)
	l.totalWriteDelayMs.Store(0)
	l.totalReadDelayMs.Store(0)
}

// IsEnabled returns whether rate limiting is active.
func (l *Limiter) IsEnabled() bool {
	return l.config.Enabled && l.monitor != nil
}

// UpdateConfig updates the rate limiter configuration.
// This is useful for dynamic tuning.
func (l *Limiter) UpdateConfig(config Config) {
	l.config = config

	// Update PID controllers - use interface methods for setpoint and gains
	l.writePID.SetSetpoint(config.WriteSetpoint)
	l.writePID.SetGains(config.WriteKp, config.WriteKi, config.WriteKd)
	// Type assert to set output limits (not part of base interface)
	if wCtrl, ok := l.writePID.(*pid.Controller); ok {
		wCtrl.SetOutputLimits(0, float64(config.MaxWriteDelayMs)/1000.0)
	}

	l.readPID.SetSetpoint(config.ReadSetpoint)
	l.readPID.SetGains(config.ReadKp, config.ReadKi, config.ReadKd)
	if rCtrl, ok := l.readPID.(*pid.Controller); ok {
		rCtrl.SetOutputLimits(0, float64(config.MaxReadDelayMs)/1000.0)
	}

	// Update memory target
	if l.monitor != nil && config.TargetMemoryMB > 0 {
		l.monitor.SetMemoryTarget(uint64(config.TargetMemoryMB) * 1024 * 1024)
	}
}
