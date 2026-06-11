package ratelimit

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/interfaces/loadmonitor"
	pidif "git.smesh.lol/orly/pkg/interfaces/pid"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/pid"
)

// ErrCompactionPause is returned by Wait() during STW compaction.
// The error string is the human-readable detail; callers prepend the
// NIP-01 machine-readable "rate-limited:" prefix.
var ErrCompactionPause = errors.New("pausing request processing to compact tables")

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

	// CompactionThresholdMB is the LSM size growth (in MB) since last compaction
	// that triggers a stop-the-world compaction. Default: 256MB.
	CompactionThresholdMB int

	// STWCheckInterval is how often to check LSM size delta for STW compaction.
	// Default: 30 seconds.
	STWCheckInterval time.Duration
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
		CompactionThresholdMB:   256,
		STWCheckInterval:        30 * time.Second,
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
		CompactionThresholdMB:   256,
		STWCheckInterval:        30 * time.Second,
	}
}

// Limiter actor request types
type (
	limiterGetMetricsReq struct {
		resp chan loadmonitor.Metrics
	}
	limiterSetMetricsReq struct {
		metrics loadmonitor.Metrics
	}
)

// Limiter implements adaptive rate limiting using PID control.
// It monitors database load metrics and computes appropriate delays
// to keep the system within its target operating range.
type Limiter struct {
	config  Config
	monitor loadmonitor.Monitor

	// PID controllers for reads and writes (using generic pid.Controller)
	writePID pidif.Controller
	readPID  pidif.Controller

	// Actor channels for metrics state
	getMetricsCh chan limiterGetMetricsReq
	setMetricsCh chan limiterSetMetricsReq

	// Emergency mode tracking with hysteresis
	inEmergencyMode    atomic.Bool
	lastEmergencyCheck atomic.Int64 // Unix nano timestamp

	// STW compaction state
	compacting              atomic.Bool  // true during stop-the-world compaction
	lsmSizeAtLastCompact    atomic.Int64 // LSM size right after last compaction
	lastCompactionDuration  atomic.Int64 // nanoseconds
	lastCompactionReclaimed atomic.Int64 // bytes reclaimed

	// Connection-level metrics for adaptive connection acceptance
	activeConnections atomic.Int64
	goroutineCount    atomic.Int64

	// Connection storm config (set via SetConnectionLimits)
	maxGlobalConns   int
	connDelayMaxMs   int
	goroutineWarning int
	goroutineMax     int

	// Statistics
	totalWriteDelayMs atomic.Int64
	totalReadDelayMs  atomic.Int64
	writeThrottles    atomic.Int64
	readThrottles     atomic.Int64
	emergencyEvents   atomic.Int64
	droppedConns      atomic.Int64

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}
	// done channels replace sync.WaitGroup - one per background goroutine
	updateLoopDone chan struct{}
	metricsActorDone chan struct{}
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
	if config.CompactionThresholdMB == 0 {
		config.CompactionThresholdMB = 256
	}
	if config.STWCheckInterval == 0 {
		config.STWCheckInterval = 30 * time.Second
	}

	l := &Limiter{
		config:           config,
		monitor:          monitor,
		ctx:              ctx,
		cancel:           cancel,
		stopped:          make(chan struct{}),
		getMetricsCh:     make(chan limiterGetMetricsReq, 1),
		setMetricsCh:     make(chan limiterSetMetricsReq, 16),
		updateLoopDone:   make(chan struct{}),
		metricsActorDone: make(chan struct{}),
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

	// Start the metrics actor goroutine
	go l.metricsActor()

	return l
}

// metricsActor owns the currentMetrics state. All reads/writes go through channels.
func (l *Limiter) metricsActor() {
	defer close(l.metricsActorDone)

	var currentMetrics loadmonitor.Metrics

	for {
		select {
		case req := <-l.getMetricsCh:
			req.resp <- currentMetrics
		case req := <-l.setMetricsCh:
			currentMetrics = req.metrics
		case <-l.ctx.Done():
			// Drain any pending gets before exit
			for {
				select {
				case req := <-l.getMetricsCh:
					req.resp <- currentMetrics
				default:
					return
				}
			}
		}
	}
}

// getMetrics retrieves the current metrics from the actor.
func (l *Limiter) getMetrics() loadmonitor.Metrics {
	resp := make(chan loadmonitor.Metrics, 1)
	l.getMetricsCh <- limiterGetMetricsReq{resp: resp}
	return <-resp
}

// setMetrics sends updated metrics to the actor.
func (l *Limiter) setMetrics(m loadmonitor.Metrics) {
	select {
	case l.setMetricsCh <- limiterSetMetricsReq{metrics: m}:
	default:
		// Drop if buffer full - next tick will update
	}
}

// Start begins the rate limiter's background metric collection.
func (l *Limiter) Start() {
	if l.monitor == nil || !l.config.Enabled {
		return
	}

	// Start the monitor
	l.monitor.Start()

	// Start metric update loop
	go l.updateLoop()
}

// updateLoop periodically fetches metrics from the monitor and checks STW triggers.
func (l *Limiter) updateLoop() {
	defer close(l.updateLoopDone)

	ticker := time.NewTicker(l.config.MetricUpdateInterval)
	defer ticker.Stop()

	stwCheck := time.NewTicker(l.config.STWCheckInterval)
	defer stwCheck.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			if l.monitor != nil {
				metrics := l.monitor.GetMetrics()
				l.setMetrics(metrics)

				// Re-evaluate emergency mode on every tick, not just during
				// event processing. Without this, ShouldAcceptConnection()
				// refuses connections during emergency, which means no events
				// arrive, which means checkEmergencyMode() never runs to
				// clear the flag - permanent lockup.
				l.checkEmergencyMode(metrics.MemoryPressure)
			}
			// Sample goroutine count for connection storm detection
			l.goroutineCount.Store(int64(runtime.NumGoroutine()))

		case <-stwCheck.C:
			l.checkSTWCompaction()
		}
	}
}

// checkSTWCompaction checks if LSM growth since last compaction exceeds
// the threshold and triggers a synchronous STW compaction if so.
func (l *Limiter) checkSTWCompaction() {
	compactMon, ok := l.monitor.(loadmonitor.CompactableMonitor)
	if !ok {
		return
	}

	currentSize := compactMon.LSMSize()
	lastSize := l.lsmSizeAtLastCompact.Load()

	// Initialize baseline on first check
	if lastSize == 0 {
		l.lsmSizeAtLastCompact.Store(currentSize)
		return
	}

	delta := currentSize - lastSize
	thresholdBytes := int64(l.config.CompactionThresholdMB) * 1024 * 1024

	if delta >= thresholdBytes {
		log.I.F("STW compaction triggered: LSM grew %dMB (threshold %dMB)",
			delta/(1024*1024), l.config.CompactionThresholdMB)
		l.triggerSTWCompaction()
	}
}

// Stop halts the rate limiter.
func (l *Limiter) Stop() {
	// CAS pattern replaces sync.Once: try to close stopCh; if already closed, return
	select {
	case <-l.stopped:
		// Already stopped
		return
	default:
	}

	l.cancel()
	if l.monitor != nil {
		l.monitor.Stop()
	}
	// Wait for background goroutines (replaces sync.WaitGroup)
	<-l.updateLoopDone
	<-l.metricsActorDone
	close(l.stopped)
}

// Stopped returns a channel that closes when the limiter has stopped.
func (l *Limiter) Stopped() <-chan struct{} {
	return l.stopped
}

// Wait blocks until the rate limiter permits the operation to proceed.
// Returns the delay applied and an error if the operation should be rejected.
// During STW compaction, writes are rejected immediately with ErrCompactionPause.
// opType accepts int for interface compatibility (0=Read, 1=Write)
func (l *Limiter) Wait(ctx context.Context, opType int) (time.Duration, error) {
	if !l.config.Enabled || l.monitor == nil {
		return 0, nil
	}

	// STW check - reject writes immediately during compaction
	if OperationType(opType) == Write && l.compacting.Load() {
		return 0, ErrCompactionPause
	}

	delay := l.ComputeDelay(OperationType(opType))
	if delay <= 0 {
		return 0, nil
	}

	// Apply the delay
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(delay):
		return delay, nil
	}
}

// ComputeDelay calculates the recommended delay for an operation.
// This can be used to check the delay without actually waiting.
func (l *Limiter) ComputeDelay(opType OperationType) time.Duration {
	if !l.config.Enabled || l.monitor == nil {
		return 0
	}

	// Get current metrics
	metrics := l.getMetrics()

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
			// Calculate how far above emergency threshold we are
			// Linear scaling: multiplier = 1 + (excess * 5)
			// At emergency threshold: 1x, at +20% above: 2x, at +40% above: 3x
			excessPressure := metrics.MemoryPressure - l.config.EmergencyThreshold
			if excessPressure < 0 {
				excessPressure = 0
			}
			multiplier := 1.0 + excessPressure*5.0

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
			log.I.F("exiting emergency mode: memory %.1f%% <= recovery threshold %.1f%%",
				memoryPressure*100, l.config.RecoveryThreshold*100)
			return false
		}
		return true
	}

	// To enter, must exceed emergency threshold
	if memoryPressure >= l.config.EmergencyThreshold {
		l.inEmergencyMode.Store(true)
		l.emergencyEvents.Add(1)
		log.W.F("entering emergency mode: memory %.1f%% >= threshold %.1f%%",
			memoryPressure*100, l.config.EmergencyThreshold*100)

		return true
	}

	return false
}

// triggerSTWCompaction runs a synchronous stop-the-world compaction.
// While compacting, l.compacting is true and Wait() rejects writes immediately.
func (l *Limiter) triggerSTWCompaction() {
	compactMon, ok := l.monitor.(loadmonitor.CompactableMonitor)
	if !ok || compactMon.IsCompacting() {
		return
	}

	l.compacting.Store(true)
	defer l.compacting.Store(false)

	sizeBefore := compactMon.LSMSize()
	start := time.Now()

	if err := compactMon.TriggerCompaction(); err != nil {
		log.E.F("STW compaction failed: %v", err)
		return
	}

	dur := time.Since(start)
	sizeAfter := compactMon.LSMSize()
	reclaimed := sizeBefore - sizeAfter

	l.lsmSizeAtLastCompact.Store(sizeAfter)
	l.lastCompactionDuration.Store(dur.Nanoseconds())
	l.lastCompactionReclaimed.Store(reclaimed)

	log.I.F("STW compaction: %v, reclaimed %dMB (%dMB -> %dMB)",
		dur, reclaimed/(1024*1024), sizeBefore/(1024*1024), sizeAfter/(1024*1024))
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
	metrics := l.getMetrics()

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

// SetConnectionLimits configures the connection storm mitigation parameters.
func (l *Limiter) SetConnectionLimits(maxGlobal, delayMaxMs, goroutineWarn, goroutineMax int) {
	l.maxGlobalConns = maxGlobal
	l.connDelayMaxMs = delayMaxMs
	l.goroutineWarning = goroutineWarn
	l.goroutineMax = goroutineMax
}

// SetActiveConnections updates the current connection count metric.
func (l *Limiter) SetActiveConnections(n int64) {
	l.activeConnections.Store(n)
}

// ActiveConnections returns the current connection count.
func (l *Limiter) ActiveConnections() int64 {
	return l.activeConnections.Load()
}

// DroppedConnections returns the total number of connections dropped due to overload.
func (l *Limiter) DroppedConnections() int64 {
	return l.droppedConns.Load()
}

// systemLoadScore computes a composite load score from 0.0 (idle) to 1.0+ (overloaded).
// It combines memory pressure, goroutine count, and connection count.
func (l *Limiter) systemLoadScore() float64 {
	resp := make(chan loadmonitor.Metrics, 1)
	l.getMetricsCh <- limiterGetMetricsReq{resp: resp}
	m := <-resp
	memPressure := m.MemoryPressure

	goroutines := l.goroutineCount.Load()
	conns := l.activeConnections.Load()

	// Memory component (0-1, already normalized)
	memScore := memPressure

	// Goroutine component: linear from warning to max
	var goroutineScore float64
	if l.goroutineWarning > 0 && goroutines > int64(l.goroutineWarning) {
		goroutineScore = float64(goroutines-int64(l.goroutineWarning)) /
			float64(l.goroutineMax-l.goroutineWarning)
		if goroutineScore > 1.0 {
			goroutineScore = 1.0
		}
	}

	// Connection component: linear from 50% to 100% of max
	var connScore float64
	if l.maxGlobalConns > 0 && conns > int64(l.maxGlobalConns/2) {
		connScore = float64(conns-int64(l.maxGlobalConns/2)) /
			float64(l.maxGlobalConns/2)
		if connScore > 1.0 {
			connScore = 1.0
		}
	}

	// Weighted combination: memory 50%, goroutines 30%, connections 20%
	return memScore*0.5 + goroutineScore*0.3 + connScore*0.2
}

// ShouldAcceptConnection returns false if the system is too overloaded to accept
// new connections. It checks memory pressure, goroutine count, and connection count.
func (l *Limiter) ShouldAcceptConnection() bool {
	if !l.config.Enabled || l.monitor == nil {
		return true
	}

	// Hard limits: refuse immediately
	goroutines := l.goroutineCount.Load()
	if l.goroutineMax > 0 && goroutines >= int64(l.goroutineMax) {
		l.droppedConns.Add(1)
		log.W.F("refusing connection: goroutine count %d >= max %d", goroutines, l.goroutineMax)
		return false
	}

	conns := l.activeConnections.Load()
	if l.maxGlobalConns > 0 && conns >= int64(l.maxGlobalConns) {
		l.droppedConns.Add(1)
		log.W.F("refusing connection: active connections %d >= max %d", conns, l.maxGlobalConns)
		return false
	}

	// Emergency mode: refuse
	if l.inEmergencyMode.Load() {
		l.droppedConns.Add(1)
		log.W.F("refusing connection: emergency mode active")
		return false
	}

	return true
}

// ConnectionDelay returns a delay to apply before accepting a new connection.
// Returns 0 if no delay is needed. The delay is proportional to system load.
func (l *Limiter) ConnectionDelay() time.Duration {
	if !l.config.Enabled || l.monitor == nil || l.connDelayMaxMs <= 0 {
		return 0
	}

	score := l.systemLoadScore()

	// No delay below 0.5 load
	if score < 0.5 {
		return 0
	}

	// Linear delay from 0.5 to 1.0 load
	fraction := (score - 0.5) * 2.0 // 0.0 at 0.5, 1.0 at 1.0
	if fraction > 1.0 {
		fraction = 1.0
	}

	delayMs := fraction * float64(l.connDelayMaxMs)
	return time.Duration(delayMs) * time.Millisecond
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
