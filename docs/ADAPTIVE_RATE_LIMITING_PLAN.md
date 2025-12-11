# Adaptive Rate Limiting Plan for Database Operations

## Overview

This plan describes an adaptive rate limiting strategy that responds to database load levels during all relay operations - both bulk imports and normal WebSocket traffic (EVENT and REQ messages).

The goals are:
1. Constrain memory usage to configurable targets (default ≤1.5GB for imports)
2. Maintain responsive query latency under sustained high load
3. Prevent database overload from write storms or complex queries
4. Gracefully degrade performance rather than crash or timeout

## Design Principles

1. **Database-Aware**: Monitor actual database load, not just Go heap memory
2. **Adaptive**: Dynamically adjust rates based on current conditions using PID control
3. **Non-Blocking**: Use async monitoring to minimize overhead
4. **Configurable**: Allow tuning via environment variables
5. **Backend-Agnostic Interface**: Common interface with backend-specific implementations
6. **Differentiated by Operation Type**: Different strategies for reads vs writes
7. **PID Control**: Use Proportional-Integral-Derivative control with filtered derivative

## Control Theory Background

The rate limiter uses a **PID controller** adapted from difficulty adjustment algorithms used in proof-of-work blockchains. This approach handles intermittent heavy spikes effectively.

### Why PID Control?

Traditional threshold-based rate limiting has problems:
- **Threshold hysteresis**: Oscillates around thresholds
- **Delayed response**: Only reacts after problems occur
- **No predictive capability**: Can't anticipate load increases

PID control provides:
- **Proportional (P)**: Immediate response proportional to current error
- **Integral (I)**: Eliminates steady-state error, handles sustained load
- **Derivative (D)**: Anticipates future error based on rate of change (with filtering)

### Filtered Derivative

Raw derivative amplifies high-frequency noise. We apply a **low-pass filter** before computing the derivative:

```
filtered_error = α * current_error + (1-α) * previous_filtered_error
derivative = (filtered_error - previous_filtered_error) / dt
```

This is equivalent to a band-pass filter that:
- Passes the signal of interest (load trends)
- Attenuates high-frequency noise (momentary spikes)
- Provides smoother derivative response

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         WebSocket Handler                                    │
│                                                                              │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────────┐   │
│  │ EVENT msg   │────▶│ Write Rate  │────▶│ SaveEvent (to database)     │   │
│  │ (writes)    │     │ Limiter     │     │                             │   │
│  └─────────────┘     └─────────────┘     └─────────────────────────────┘   │
│                             │                                               │
│  ┌─────────────┐     ┌─────┴───────┐     ┌─────────────────────────────┐   │
│  │ REQ msg     │────▶│ Read Rate   │────▶│ QueryEvents (from database) │   │
│  │ (reads)     │     │ Limiter     │     │                             │   │
│  └─────────────┘     └─────────────┘     └─────────────────────────────┘   │
│                             │                                               │
│                      ┌──────┴──────┐                                        │
│                      │ Load Monitor │ (async, periodic)                     │
│                      └──────┬──────┘                                        │
└─────────────────────────────┼───────────────────────────────────────────────┘
                              │
                 ┌────────────┴────────────┐
                 │                         │
           ┌─────▼─────┐            ┌──────▼──────┐
           │  Badger   │            │   Neo4j     │
           │  Monitor  │            │   Monitor   │
           └───────────┘            └─────────────┘
```

## Common Interface

```go
// LoadMonitor provides database-specific load metrics
type LoadMonitor interface {
    // GetLoadLevel returns a value from 0.0 (idle) to 1.0+ (overloaded)
    GetLoadLevel() float64

    // GetMemoryPressure returns estimated memory pressure (0.0-1.0)
    GetMemoryPressure() float64

    // GetReadLatency returns recent average read/query latency
    GetReadLatency() time.Duration

    // GetWriteLatency returns recent average write latency
    GetWriteLatency() time.Duration

    // ShouldThrottleWrites returns true if writes should slow down
    ShouldThrottleWrites() bool

    // ShouldThrottleReads returns true if reads should slow down
    ShouldThrottleReads() bool

    // RecommendedWriteDelay returns suggested delay before next write
    RecommendedWriteDelay() time.Duration

    // RecommendedReadDelay returns suggested delay before next read
    RecommendedReadDelay() time.Duration

    // ForceFlush triggers a flush/sync operation if supported
    ForceFlush() error

    // RecordReadLatency records a read operation's latency for tracking
    RecordReadLatency(d time.Duration)

    // RecordWriteLatency records a write operation's latency for tracking
    RecordWriteLatency(d time.Duration)

    // Start begins async monitoring
    Start(ctx context.Context)

    // Stop halts monitoring
    Stop()
}

// OperationType distinguishes read vs write operations
type OperationType int

const (
    OpRead  OperationType = iota  // REQ, COUNT queries
    OpWrite                        // EVENT storage
)

// AdaptiveRateLimiter controls operation rate based on load metrics
type AdaptiveRateLimiter struct {
    monitor       LoadMonitor
    targetMemMB   uint64        // Target max memory (e.g., 1400 MB)

    // Per-operation-type configuration
    writeConfig   RateLimitConfig
    readConfig    RateLimitConfig

    // Adaptive state (per operation type)
    writeState    rateLimitState
    readState     rateLimitState
}

type RateLimitConfig struct {
    MinBatchSize    int           // Min ops before checking (e.g., 10)
    MaxBatchSize    int           // Max ops before forced check (e.g., 500)
    MaxDelay        time.Duration // Maximum delay cap
    LatencyTarget   time.Duration // Target latency (trigger throttling above this)
}

type rateLimitState struct {
    mu              sync.Mutex
    currentDelay    time.Duration
    opCount         int
    smoothedLoad    float64       // Exponential moving average of load
    smoothedLatency time.Duration // EMA of operation latency

    // Stats
    totalThrottleTime time.Duration
    throttleCount     int
}
```

## PID Controller Implementation

The core of the adaptive rate limiting is a PID controller that computes delay based on the error between current memory/load and the target.

### PID Controller Structure

```go
// PIDController implements a PID controller with filtered derivative
// for adaptive rate limiting based on memory pressure and load metrics.
//
// The controller uses:
// - Proportional: Immediate response to current error
// - Integral: Accumulates error over time to eliminate steady-state offset
// - Derivative: Anticipates future error (with low-pass filtering to reduce noise)
type PIDController struct {
    // Gains (tunable parameters)
    Kp float64 // Proportional gain
    Ki float64 // Integral gain
    Kd float64 // Derivative gain

    // Target setpoint (e.g., 0.85 = 85% of memory target)
    Setpoint float64

    // Filter coefficient for derivative (0 < alpha < 1)
    // Lower alpha = more filtering (smoother but slower response)
    // Higher alpha = less filtering (faster but noisier)
    DerivativeFilterAlpha float64

    // Anti-windup: maximum integral accumulation
    IntegralMax float64
    IntegralMin float64

    // Output limits
    OutputMin float64 // Minimum delay (can be 0)
    OutputMax float64 // Maximum delay (e.g., 1.0 for 1 second)

    // Internal state
    mu               sync.Mutex
    integral         float64
    prevError        float64
    prevFilteredError float64
    lastUpdate       time.Time
    initialized      bool
}

// NewPIDController creates a PID controller with recommended defaults
// for memory-based rate limiting.
//
// The setpoint represents the target operating point as a fraction of
// the memory target (e.g., 0.85 means aim to stay at 85% of target).
func NewPIDController(setpoint float64) *PIDController {
    return &PIDController{
        // Tuned for memory/load control
        // These values are starting points - tune based on testing
        Kp: 0.5,  // Moderate proportional response
        Ki: 0.1,  // Slow integral to handle sustained load
        Kd: 0.05, // Light derivative (filtered)

        Setpoint: setpoint,

        // Low-pass filter for derivative
        // α = 0.2 gives good noise rejection while maintaining responsiveness
        DerivativeFilterAlpha: 0.2,

        // Anti-windup limits
        IntegralMax: 2.0,
        IntegralMin: -0.5, // Allow some negative to speed recovery

        // Output limits (in seconds)
        OutputMin: 0.0,
        OutputMax: 1.0,
    }
}

// NewPIDControllerForReads creates a PID controller tuned for read operations.
// Reads should be more responsive (lower gains, faster recovery).
func NewPIDControllerForReads(setpoint float64) *PIDController {
    return &PIDController{
        Kp: 0.3,  // Lower proportional - reads should stay responsive
        Ki: 0.05, // Lower integral - don't accumulate as much
        Kd: 0.02, // Minimal derivative

        Setpoint: setpoint,
        DerivativeFilterAlpha: 0.3, // More filtering for reads

        IntegralMax: 1.0,
        IntegralMin: -0.2,

        OutputMin: 0.0,
        OutputMax: 0.2, // Cap read delays at 200ms
    }
}

// Update computes the control output based on the current process variable.
//
// processVariable: current value (e.g., memory pressure 0.0-1.0+)
// Returns: recommended delay in seconds (0.0 to OutputMax)
func (p *PIDController) Update(processVariable float64) float64 {
    p.mu.Lock()
    defer p.mu.Unlock()

    now := time.Now()

    // Initialize on first call
    if !p.initialized {
        p.lastUpdate = now
        p.prevError = 0
        p.prevFilteredError = 0
        p.initialized = true
        return 0
    }

    // Calculate time delta
    dt := now.Sub(p.lastUpdate).Seconds()
    if dt <= 0 {
        dt = 0.001 // Minimum 1ms to avoid division by zero
    }
    p.lastUpdate = now

    // Calculate error (positive = over target, need to slow down)
    error := processVariable - p.Setpoint

    // ===== PROPORTIONAL TERM =====
    pTerm := p.Kp * error

    // ===== INTEGRAL TERM =====
    // Accumulate error over time
    p.integral += error * dt

    // Anti-windup: clamp integral
    if p.integral > p.IntegralMax {
        p.integral = p.IntegralMax
    } else if p.integral < p.IntegralMin {
        p.integral = p.IntegralMin
    }

    iTerm := p.Ki * p.integral

    // ===== DERIVATIVE TERM (with low-pass filter) =====
    // Apply low-pass filter to error before computing derivative
    // This is the key insight from PoW difficulty adjustment:
    // raw derivative amplifies noise, filtered derivative tracks trends
    α := p.DerivativeFilterAlpha
    filteredError := α*error + (1-α)*p.prevFilteredError

    // Compute derivative of filtered error
    var dTerm float64
    if dt > 0 {
        derivative := (filteredError - p.prevFilteredError) / dt
        dTerm = p.Kd * derivative
    }

    // Save state for next iteration
    p.prevError = error
    p.prevFilteredError = filteredError

    // ===== COMBINE TERMS =====
    output := pTerm + iTerm + dTerm

    // Clamp output to limits
    if output < p.OutputMin {
        output = p.OutputMin
    } else if output > p.OutputMax {
        output = p.OutputMax
    }

    return output
}

// Reset clears the controller state (use when conditions change dramatically)
func (p *PIDController) Reset() {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.integral = 0
    p.prevError = 0
    p.prevFilteredError = 0
    p.initialized = false
}

// GetState returns current internal state for debugging/monitoring
func (p *PIDController) GetState() (integral, prevError, filteredError float64) {
    p.mu.Lock()
    defer p.mu.Unlock()
    return p.integral, p.prevError, p.prevFilteredError
}

// SetGains allows runtime tuning of PID gains
func (p *PIDController) SetGains(kp, ki, kd float64) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.Kp = kp
    p.Ki = ki
    p.Kd = kd
}
```

### PID Tuning Guidelines

The PID gains control how aggressively the system responds:

| Parameter | Effect | Too High | Too Low |
|-----------|--------|----------|---------|
| **Kp** | Immediate response | Oscillation, overshoot | Slow response, steady-state error |
| **Ki** | Eliminates steady-state error | Overshoot, slow recovery | Persistent offset from target |
| **Kd** | Dampens oscillation, predicts | Noise amplification | Overshoot, oscillation |
| **α** (filter) | Derivative smoothing | Slow derivative response | Noisy derivative |

**Recommended starting values** (from PoW difficulty adjustment experience):

```go
// For writes (more aggressive control)
Kp = 0.5   // Moderate proportional
Ki = 0.1   // Slow integral accumulation
Kd = 0.05  // Light filtered derivative
α  = 0.2   // Strong low-pass filtering

// For reads (more responsive, lighter control)
Kp = 0.3   // Lower proportional
Ki = 0.05  // Minimal integral
Kd = 0.02  // Very light derivative
α  = 0.3   // Moderate filtering
```

### Control Loop Integration

The PID controller is called periodically with the current "process variable" (memory pressure + load):

```go
// Compute combined process variable from multiple signals
func computeProcessVariable(monitor LoadMonitor) float64 {
    memPressure := monitor.GetMemoryPressure()  // 0.0 - 1.0+
    loadLevel := monitor.GetLoadLevel()          // 0.0 - 1.0+

    // Weight memory more heavily as it's the primary constraint
    // Memory: 70%, Load: 30%
    return 0.7*memPressure + 0.3*loadLevel
}

// In the rate limiter
func (r *AdaptiveRateLimiter) computeDelay(opType OperationType) time.Duration {
    pv := computeProcessVariable(r.monitor)

    var controller *PIDController
    if opType == OpWrite {
        controller = r.writeController
    } else {
        controller = r.readController
    }

    // PID outputs delay in seconds (0.0 - 1.0)
    delaySec := controller.Update(pv)

    return time.Duration(delaySec * float64(time.Second))
}
```

### Setpoint Selection

The **setpoint** defines the target operating point:

- **Setpoint = 0.85**: Target 85% of memory limit
  - Leaves 15% headroom for spikes
  - Controller activates when approaching limit
  - Good for normal operation

- **Setpoint = 0.70**: Target 70% of memory limit
  - More conservative, leaves 30% headroom
  - Better for import operations with large bursts
  - Slower steady-state throughput

```go
// For imports (conservative)
importController := NewPIDController(0.70)

// For normal relay operation
relayController := NewPIDController(0.85)
```

## Badger Load Monitor

### Metrics Available

Badger exposes several useful metrics through its API:

| Method | Returns | Use Case |
|--------|---------|----------|
| `db.Levels()` | `[]LevelInfo` | LSM tree state, compaction pressure |
| `db.Size()` | `(lsm, vlog int64)` | Current database size |
| `db.BlockCacheMetrics()` | `*ristretto.Metrics` | Cache hit ratio, memory usage |
| `db.IndexCacheMetrics()` | `*ristretto.Metrics` | Index cache performance |

### LevelInfo Fields

```go
type LevelInfo struct {
    Level          int
    NumTables      int     // Number of SST files at this level
    Size           int64   // Current size of this level
    TargetSize     int64   // Target size before compaction
    Score          float64 // Compaction score (>1.0 = needs compaction)
    StaleDatSize   int64   // Stale data waiting to be cleaned
}
```

### Badger Load Calculation

```go
type BadgerLoadMonitor struct {
    db            *badger.DB
    targetMemMB   uint64
    checkInterval time.Duration

    // Cached metrics (updated async)
    mu            sync.RWMutex
    loadLevel     float64
    memPressure   float64
    l0Tables      int
    compactScore  float64

    // Latency tracking (sliding window)
    readLatencies  *LatencyTracker
    writeLatencies *LatencyTracker
}

// LatencyTracker maintains a sliding window of latency samples
type LatencyTracker struct {
    mu      sync.Mutex
    samples []time.Duration
    index   int
    size    int
    sum     time.Duration
}

func NewLatencyTracker(windowSize int) *LatencyTracker {
    return &LatencyTracker{
        samples: make([]time.Duration, windowSize),
        size:    windowSize,
    }
}

func (t *LatencyTracker) Record(d time.Duration) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Subtract old value, add new
    t.sum -= t.samples[t.index]
    t.samples[t.index] = d
    t.sum += d
    t.index = (t.index + 1) % t.size
}

func (t *LatencyTracker) Average() time.Duration {
    t.mu.Lock()
    defer t.mu.Unlock()

    if t.sum == 0 {
        return 0
    }
    return t.sum / time.Duration(t.size)
}

func (m *BadgerLoadMonitor) updateMetrics() {
    levels := m.db.Levels()

    // L0 tables are the primary indicator of write pressure
    // More L0 tables = writes outpacing compaction
    var l0Tables int
    var maxScore float64
    var totalStale int64

    for _, level := range levels {
        if level.Level == 0 {
            l0Tables = level.NumTables
        }
        if level.Score > maxScore {
            maxScore = level.Score
        }
        totalStale += level.StaleDatSize
    }

    // Calculate load level (0.0 - 1.0+)
    // L0 tables: each table adds ~0.1 to load (8 tables = 0.8)
    // Compaction score: directly indicates compaction pressure
    l0Load := float64(l0Tables) / 10.0
    compactLoad := maxScore / 2.0 // Score of 2.0 = full load

    m.mu.Lock()
    m.l0Tables = l0Tables
    m.compactScore = maxScore
    m.loadLevel = max(l0Load, compactLoad)

    // Memory pressure from Go runtime
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    heapMB := memStats.HeapAlloc / 1024 / 1024
    m.memPressure = float64(heapMB) / float64(m.targetMemMB)
    m.mu.Unlock()
}

func (m *BadgerLoadMonitor) GetLoadLevel() float64 {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.loadLevel
}

func (m *BadgerLoadMonitor) GetReadLatency() time.Duration {
    return m.readLatencies.Average()
}

func (m *BadgerLoadMonitor) GetWriteLatency() time.Duration {
    return m.writeLatencies.Average()
}

func (m *BadgerLoadMonitor) RecordReadLatency(d time.Duration) {
    m.readLatencies.Record(d)
}

func (m *BadgerLoadMonitor) RecordWriteLatency(d time.Duration) {
    m.writeLatencies.Record(d)
}

func (m *BadgerLoadMonitor) ShouldThrottleWrites() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Throttle writes if:
    // - L0 tables > 6 (approaching stall at 8)
    // - Compaction score > 1.5
    // - Memory pressure > 90%
    // - Write latency > 50ms (indicates backlog)
    return m.l0Tables > 6 ||
           m.compactScore > 1.5 ||
           m.memPressure > 0.9 ||
           m.writeLatencies.Average() > 50*time.Millisecond
}

func (m *BadgerLoadMonitor) ShouldThrottleReads() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Throttle reads if:
    // - Memory pressure > 95% (critical)
    // - Read latency > 100ms (queries taking too long)
    // - Compaction score > 2.0 (severe compaction backlog affects reads)
    return m.memPressure > 0.95 ||
           m.readLatencies.Average() > 100*time.Millisecond ||
           m.compactScore > 2.0
}

func (m *BadgerLoadMonitor) RecommendedWriteDelay() time.Duration {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // No delay if load is low
    if m.loadLevel < 0.5 && m.memPressure < 0.7 {
        return 0
    }

    // Linear scaling: load 0.5-1.0 maps to 0-100ms
    // Above 1.0: exponential backoff
    if m.loadLevel < 1.0 {
        return time.Duration((m.loadLevel - 0.5) * 200) * time.Millisecond
    }

    // Exponential backoff for overload
    excess := m.loadLevel - 1.0
    delay := 100 * time.Millisecond * time.Duration(1<<int(excess*3))
    if delay > 1*time.Second {
        delay = 1 * time.Second
    }
    return delay
}

func (m *BadgerLoadMonitor) RecommendedReadDelay() time.Duration {
    // Reads get lighter throttling than writes
    // We want to stay responsive to queries
    avgLatency := m.readLatencies.Average()

    if avgLatency < 50*time.Millisecond {
        return 0
    }

    // Scale delay based on how far over target we are
    // Target: 50ms, at 200ms = 4x over = 150ms delay
    excess := float64(avgLatency) / float64(50*time.Millisecond)
    if excess > 1.0 {
        delay := time.Duration((excess - 1.0) * 50) * time.Millisecond
        if delay > 200*time.Millisecond {
            delay = 200 * time.Millisecond
        }
        return delay
    }
    return 0
}

func (m *BadgerLoadMonitor) ForceFlush() error {
    return m.db.Sync()
}
```

## Neo4j Load Monitor

### Metrics Available

Neo4j provides metrics through Cypher procedures:

| Procedure | Purpose |
|-----------|---------|
| `CALL dbms.listTransactions()` | Active transaction count |
| `CALL dbms.listQueries()` | Running queries with timing |
| `CALL dbms.listConnections()` | Active connections |
| `SHOW SETTINGS` | Configuration values |

### Neo4j Load Calculation

```go
type Neo4jLoadMonitor struct {
    driver        neo4j.DriverWithContext
    targetMemMB   uint64
    checkInterval time.Duration

    // Cached metrics
    mu                 sync.RWMutex
    loadLevel          float64
    memPressure        float64
    activeTransactions int
    avgQueryTime       time.Duration

    // Latency tracking
    readLatencies  *LatencyTracker
    writeLatencies *LatencyTracker
}

func (m *Neo4jLoadMonitor) updateMetrics() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    session := m.driver.NewSession(ctx, neo4j.SessionConfig{
        AccessMode: neo4j.AccessModeRead,
    })
    defer session.Close(ctx)

    // Get active transaction count and query stats
    result, err := session.Run(ctx, `
        CALL dbms.listTransactions()
        YIELD transactionId, currentQuery, status, elapsedTime
        WHERE status = 'Running'
        RETURN count(*) AS activeCount,
               avg(elapsedTime.milliseconds) AS avgTime
    `, nil)

    if err != nil {
        return
    }

    if result.Next(ctx) {
        record := result.Record()
        activeCount, _ := record.Get("activeCount")
        avgTime, _ := record.Get("avgTime")

        m.mu.Lock()
        m.activeTransactions = int(activeCount.(int64))
        if avgTime != nil {
            m.avgQueryTime = time.Duration(avgTime.(float64)) * time.Millisecond
        }

        // Calculate load based on active transactions
        // Neo4j typically handles 10-50 concurrent transactions well
        m.loadLevel = float64(m.activeTransactions) / 20.0

        // Factor in query latency (>100ms = warning, >500ms = high load)
        if m.avgQueryTime > 500*time.Millisecond {
            m.loadLevel += 0.5
        } else if m.avgQueryTime > 100*time.Millisecond {
            m.loadLevel += 0.2
        }
        m.mu.Unlock()
    }

    // Memory pressure from Go runtime (client-side)
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    heapMB := memStats.HeapAlloc / 1024 / 1024

    m.mu.Lock()
    m.memPressure = float64(heapMB) / float64(m.targetMemMB)
    m.mu.Unlock()
}

func (m *Neo4jLoadMonitor) ShouldThrottleWrites() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Throttle writes if:
    // - Active transactions > 15
    // - Average query time > 200ms
    // - Memory pressure > 90%
    // - Write latency > 100ms
    return m.activeTransactions > 15 ||
           m.avgQueryTime > 200*time.Millisecond ||
           m.memPressure > 0.9 ||
           m.writeLatencies.Average() > 100*time.Millisecond
}

func (m *Neo4jLoadMonitor) ShouldThrottleReads() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Throttle reads if:
    // - Active transactions > 25 (higher threshold for reads)
    // - Read latency > 200ms
    // - Memory pressure > 95%
    return m.activeTransactions > 25 ||
           m.readLatencies.Average() > 200*time.Millisecond ||
           m.memPressure > 0.95
}

func (m *Neo4jLoadMonitor) ForceFlush() error {
    // Neo4j doesn't have a direct flush command
    // We can only wait for transactions to complete
    // Return nil as a no-op
    return nil
}
```

## Adaptive Rate Limiter Implementation (PID-Based)

```go
// AdaptiveRateLimiter controls operation rate using PID controllers
// for both read and write operations.
type AdaptiveRateLimiter struct {
    monitor LoadMonitor

    // Configuration
    targetMemMB uint64
    setpoint    float64 // Target operating point (e.g., 0.85)

    // PID controllers (separate for reads and writes)
    writeController *PIDController
    readController  *PIDController

    // Per-operation state
    writeState rateLimitState
    readState  rateLimitState

    // Batching configuration
    writeMinBatch int // Check every N writes
    readMinBatch  int // Check every N reads
}

// NewAdaptiveRateLimiter creates a rate limiter with PID control.
// targetMemMB: memory target in megabytes (e.g., 1400)
// setpoint: target operating point as fraction (e.g., 0.85 = 85% of target)
func NewAdaptiveRateLimiter(monitor LoadMonitor, targetMemMB uint64, setpoint float64) *AdaptiveRateLimiter {
    return &AdaptiveRateLimiter{
        monitor:     monitor,
        targetMemMB: targetMemMB,
        setpoint:    setpoint,

        // Create PID controllers with appropriate tuning
        writeController: NewPIDController(setpoint),
        readController:  NewPIDControllerForReads(setpoint),

        // Batching: check every N operations to reduce overhead
        writeMinBatch: 10,
        readMinBatch:  5,
    }
}

// NewAdaptiveRateLimiterForImport creates a rate limiter tuned for bulk imports.
// Uses more conservative setpoint and tighter control.
func NewAdaptiveRateLimiterForImport(monitor LoadMonitor, targetMemMB uint64) *AdaptiveRateLimiter {
    limiter := &AdaptiveRateLimiter{
        monitor:     monitor,
        targetMemMB: targetMemMB,
        setpoint:    0.70, // More conservative for imports

        writeController: NewPIDController(0.70),
        readController:  NewPIDControllerForReads(0.85), // Reads less critical during import

        writeMinBatch: 5,  // Check more frequently during import
        readMinBatch:  10,
    }

    // Tune write controller for import workload
    limiter.writeController.Ki = 0.15  // Slightly higher integral for sustained load
    limiter.writeController.OutputMax = 1.0  // Allow up to 1s delays

    return limiter
}

// computeProcessVariable combines memory pressure and load into a single metric
func (r *AdaptiveRateLimiter) computeProcessVariable() float64 {
    memPressure := r.monitor.GetMemoryPressure()  // 0.0 - 1.0+
    loadLevel := r.monitor.GetLoadLevel()          // 0.0 - 1.0+

    // Weight memory more heavily as it's the primary constraint
    // Memory: 70%, Load: 30%
    return 0.7*memPressure + 0.3*loadLevel
}

// MaybeThrottle is called after each operation.
// Returns the delay that was applied.
func (r *AdaptiveRateLimiter) MaybeThrottle(opType OperationType) time.Duration {
    var state *rateLimitState
    var controller *PIDController
    var minBatch int

    if opType == OpWrite {
        state = &r.writeState
        controller = r.writeController
        minBatch = r.writeMinBatch
    } else {
        state = &r.readState
        controller = r.readController
        minBatch = r.readMinBatch
    }

    // Increment operation count
    state.mu.Lock()
    state.opCount++
    opCount := state.opCount
    state.mu.Unlock()

    // Only check every minBatch operations to reduce overhead
    if opCount%minBatch != 0 {
        return 0
    }

    // Get current process variable (combined memory + load metric)
    pv := r.computeProcessVariable()

    // Run PID controller to compute delay
    delaySec := controller.Update(pv)
    delay := time.Duration(delaySec * float64(time.Second))

    // Additional actions when under significant pressure
    if delaySec > 0.1 { // More than 100ms delay recommended
        // Force flush for writes to help database catch up
        if opType == OpWrite {
            r.monitor.ForceFlush()
        }

        // Trigger GC if memory pressure is high
        memPressure := r.monitor.GetMemoryPressure()
        if memPressure > 0.90 {
            runtime.GC()
            debug.FreeOSMemory()
        }
    }

    // Apply delay if needed
    if delay > 0 {
        state.mu.Lock()
        state.throttleCount++
        state.totalThrottleTime += delay
        state.currentDelay = delay
        state.mu.Unlock()

        time.Sleep(delay)
    }

    return delay
}

// RecordLatency records operation latency for monitoring
func (r *AdaptiveRateLimiter) RecordLatency(opType OperationType, d time.Duration) {
    if opType == OpWrite {
        r.monitor.RecordWriteLatency(d)
    } else {
        r.monitor.RecordReadLatency(d)
    }
}

// Stats returns throttling statistics for an operation type
func (r *AdaptiveRateLimiter) Stats(opType OperationType) (throttleCount int, totalTime time.Duration) {
    var state *rateLimitState
    if opType == OpWrite {
        state = &r.writeState
    } else {
        state = &r.readState
    }

    state.mu.Lock()
    defer state.mu.Unlock()
    return state.throttleCount, state.totalThrottleTime
}

// GetControllerState returns PID controller state for debugging
func (r *AdaptiveRateLimiter) GetControllerState(opType OperationType) (integral, error, filtered float64) {
    if opType == OpWrite {
        return r.writeController.GetState()
    }
    return r.readController.GetState()
}

// ResetControllers resets both PID controllers (use after major state changes)
func (r *AdaptiveRateLimiter) ResetControllers() {
    r.writeController.Reset()
    r.readController.Reset()
}

// SetWriteGains allows runtime tuning of write controller
func (r *AdaptiveRateLimiter) SetWriteGains(kp, ki, kd float64) {
    r.writeController.SetGains(kp, ki, kd)
}

// SetReadGains allows runtime tuning of read controller
func (r *AdaptiveRateLimiter) SetReadGains(kp, ki, kd float64) {
    r.readController.SetGains(kp, ki, kd)
}
```

## Integration with WebSocket Handlers

### EVENT Handler (Writes)

```go
// In app/handle-event.go

func (s *Server) handleEvent(ctx context.Context, conn *websocket.Conn, ev *event.E) {
    // Record start time for latency tracking
    start := time.Now()

    // ... validation ...

    // Save event
    replaced, err := s.DB.SaveEvent(ctx, ev)

    // Record write latency
    s.rateLimiter.RecordLatency(OpWrite, time.Since(start))

    if err != nil {
        // ... error handling ...
        return
    }

    // Apply adaptive rate limiting after successful write
    delay := s.rateLimiter.MaybeThrottle(OpWrite)
    if delay > 0 {
        log.D.F("write throttled for %v (load=%.2f)", delay, s.loadMonitor.GetLoadLevel())
    }

    // ... send OK response ...
}
```

### REQ Handler (Reads)

```go
// In app/handle-req.go

func (s *Server) handleReq(ctx context.Context, conn *websocket.Conn, subID string, filters []*filter.F) {
    // Record start time for latency tracking
    start := time.Now()

    // Execute query
    events, err := s.DB.QueryEvents(ctx, filters)

    // Record read latency
    queryTime := time.Since(start)
    s.rateLimiter.RecordLatency(OpRead, queryTime)

    if err != nil {
        // ... error handling ...
        return
    }

    // Apply adaptive rate limiting if query was slow or system under load
    delay := s.rateLimiter.MaybeThrottle(OpRead)
    if delay > 0 {
        log.D.F("read throttled for %v after %v query (load=%.2f)",
            delay, queryTime, s.loadMonitor.GetLoadLevel())
    }

    // ... send events ...
}
```

### Integration with Import Loop

```go
// In import_utils.go

func (d *D) processJSONLEventsWithPolicy(
    ctx context.Context,
    rr io.Reader,
    policyManager PolicyChecker,
) error {
    // Create load monitor based on database type
    var monitor LoadMonitor
    switch db := d.getUnderlyingDB().(type) {
    case *badger.DB:
        monitor = NewBadgerLoadMonitor(db, 1400) // 1.4GB target
    case neo4j.DriverWithContext:
        monitor = NewNeo4jLoadMonitor(db, 1400)
    default:
        // Fallback to memory-only monitoring
        monitor = NewMemoryOnlyMonitor(1400)
    }

    // Start async monitoring
    monitor.Start(ctx)
    defer monitor.Stop()

    // Create rate limiter with import-specific config
    limiter := NewAdaptiveRateLimiter(monitor, 1400)
    // Use more aggressive write throttling for imports
    limiter.writeConfig.MaxDelay = 1 * time.Second
    limiter.writeConfig.LatencyTarget = 30 * time.Millisecond

    scan := bufio.NewScanner(rr)
    // ... scanner setup ...

    var count int
    for scan.Scan() {
        // ... event parsing and validation ...

        start := time.Now()
        if _, err := d.SaveEvent(ctx, ev); err != nil {
            // ... error handling ...
            continue
        }
        limiter.RecordLatency(OpWrite, time.Since(start))

        ev.Free()
        count++

        // Apply adaptive rate limiting
        limiter.MaybeThrottle(OpWrite)

        // Progress logging (less frequent to reduce overhead)
        if count%5000 == 0 {
            throttles, throttleTime := limiter.Stats(OpWrite)
            log.I.F("import: %d events, load=%.2f, latency=%v, throttled %d times (%.1fs total)",
                count, monitor.GetLoadLevel(), monitor.GetWriteLatency(),
                throttles, throttleTime.Seconds())
        }
    }

    // Final stats
    throttles, throttleTime := limiter.Stats(OpWrite)
    log.I.F("import: complete - %d events, throttled %d times (%.1fs total)",
        count, throttles, throttleTime.Seconds())

    return scan.Err()
}
```

## Configuration

```bash
# Global adaptive rate limiting settings
ORLY_RATE_LIMIT_ENABLED=true           # Enable/disable rate limiting
ORLY_RATE_LIMIT_TARGET_MEMORY_MB=1400  # Target max memory in MB
ORLY_RATE_LIMIT_SETPOINT=0.85          # Target operating point (0.0-1.0)

# PID Controller - Write operations
ORLY_RATE_LIMIT_WRITE_KP=0.5           # Proportional gain
ORLY_RATE_LIMIT_WRITE_KI=0.1           # Integral gain
ORLY_RATE_LIMIT_WRITE_KD=0.05          # Derivative gain
ORLY_RATE_LIMIT_WRITE_FILTER_ALPHA=0.2 # Low-pass filter coefficient
ORLY_RATE_LIMIT_WRITE_MAX_DELAY_MS=1000  # Maximum delay cap
ORLY_RATE_LIMIT_WRITE_MIN_BATCH=10     # Ops between PID updates

# PID Controller - Read operations
ORLY_RATE_LIMIT_READ_KP=0.3            # Proportional gain (lower for responsiveness)
ORLY_RATE_LIMIT_READ_KI=0.05           # Integral gain
ORLY_RATE_LIMIT_READ_KD=0.02           # Derivative gain
ORLY_RATE_LIMIT_READ_FILTER_ALPHA=0.3  # Low-pass filter coefficient
ORLY_RATE_LIMIT_READ_MAX_DELAY_MS=200  # Maximum delay cap (lower for reads)
ORLY_RATE_LIMIT_READ_MIN_BATCH=5       # Ops between PID updates

# Import-specific overrides
ORLY_IMPORT_SETPOINT=0.70              # More conservative for imports
ORLY_IMPORT_WRITE_KI=0.15              # Higher integral for sustained load
ORLY_IMPORT_MIN_BATCH=5                # Check more frequently

# Badger-specific thresholds (used in load calculation)
ORLY_RATE_LIMIT_BADGER_L0_THRESHOLD=6      # L0 tables contributing to load
ORLY_RATE_LIMIT_BADGER_SCORE_THRESHOLD=1.5 # Compaction score threshold

# Neo4j-specific thresholds (used in load calculation)
ORLY_RATE_LIMIT_NEO4J_TXN_THRESHOLD=15     # Active transactions threshold
ORLY_RATE_LIMIT_NEO4J_LATENCY_THRESHOLD_MS=200  # Query latency threshold
```

### PID Tuning Quick Reference

| Symptom | Adjustment |
|---------|------------|
| Slow to respond to load spikes | Increase Kp |
| Oscillating around target | Decrease Kp, increase Kd |
| Steady-state error (never reaches target) | Increase Ki |
| Overshoots then slowly recovers | Decrease Ki |
| Noisy/jittery delays | Decrease filter alpha (more smoothing) |
| Slow to track rapid changes | Increase filter alpha (less smoothing) |

## Expected Behavior

### Normal Operation (load < 0.5, latency < target)
- No delays applied
- Full throughput for both reads and writes
- Memory well under target

### Moderate Load (0.5 < load < 0.8)
- Light delays (10-50ms) applied occasionally
- Throughput slightly reduced
- Latency stays near target
- Memory hovers around 60-80% of target

### High Write Load
- Write delays increase (50-200ms)
- Force flush operations triggered
- Read delays remain light (preserve query responsiveness)
- Import speed drops to ~200-400 events/sec

### High Read Load
- Read delays increase (20-100ms)
- Complex queries get throttled more
- Write delays remain moderate
- Query latency stays bounded

### Critical Load (load > 1.0, memory > 90%)
- Maximum delays applied
- Aggressive GC triggered
- System stabilizes before continuing
- Graceful degradation, not failure

## Metrics and Observability

The rate limiter exposes metrics for monitoring:

```go
// Metrics available via /api/metrics or logging
type RateLimitMetrics struct {
    // Current state
    LoadLevel       float64
    MemoryPressure  float64
    ReadLatencyAvg  time.Duration
    WriteLatencyAvg time.Duration

    // Throttling stats
    WriteThrottleCount int
    WriteThrottleTime  time.Duration
    ReadThrottleCount  int
    ReadThrottleTime   time.Duration

    // Per-second rates
    WritesPerSecond float64
    ReadsPerSecond  float64
}
```

## Files to Create/Modify

1. **New**: `pkg/database/load_monitor.go` - LoadMonitor interface and LatencyTracker
2. **New**: `pkg/database/badger_load_monitor.go` - Badger-specific implementation
3. **New**: `pkg/neo4j/load_monitor.go` - Neo4j-specific implementation
4. **New**: `pkg/database/adaptive_rate_limiter.go` - Rate limiter implementation
5. **Modify**: `pkg/database/import_utils.go` - Integrate rate limiter for imports
6. **Modify**: `app/handle-event.go` - Integrate rate limiter for EVENT handling
7. **Modify**: `app/handle-req.go` - Integrate rate limiter for REQ handling
8. **Modify**: `app/server.go` - Initialize and manage rate limiter lifecycle
9. **Modify**: `app/config/config.go` - Add configuration options

## Testing Strategy

1. **Unit Tests**: Test load calculation and throttling logic with mocked metrics
2. **Integration Tests**: Test with real database under controlled load
3. **Latency Tests**: Verify query latency stays bounded under load
4. **Stress Tests**: Import large files while running concurrent queries
5. **Memory Tests**: Verify memory stays under target during sustained load
6. **Comparison Tests**: Compare throughput/latency with and without rate limiting

## References

- [Neo4j Metrics Reference](https://neo4j.com/docs/operations-manual/current/monitoring/metrics/reference/)
- [Neo4j Memory Configuration](https://neo4j.com/docs/operations-manual/current/performance/memory-configuration/)
- [Neo4j Query Monitoring](https://neo4j.com/graphacademy/training-cqt-40/05-cqt-40-monitoring-queries/)
- [Badger DB Documentation](https://dgraph.io/docs/badger/)
