# Benchmark CPU Usage Optimization

This document describes the CPU optimization settings for the ORLY benchmark suite, specifically tuned for systems with limited CPU resources (6-core/12-thread and lower).

## Problem Statement

The original benchmark implementation was designed for maximum throughput testing, which caused:
- **CPU saturation**: 95-100% sustained CPU usage across all cores
- **System instability**: Other services unable to run alongside benchmarks
- **Thermal throttling**: Long benchmark runs causing CPU frequency reduction
- **Unrealistic load**: Tight loops not representative of real-world relay usage

## Solution: Aggressive Rate Limiting

The benchmark now implements multi-layered CPU usage controls:

### 1. Reduced Worker Concurrency

**Default Worker Count**: `NumCPU() / 4` (minimum 2)

For a 6-core/12-thread system:
- Previous: 12 workers
- **Current: 3 workers**

This 4x reduction dramatically lowers:
- Goroutine context switching overhead
- Lock contention on shared resources
- CPU cache thrashing

### 2. Per-Operation Delays

All benchmark operations now include mandatory delays to prevent CPU saturation:

| Operation Type | Delay | Rationale |
|---------------|-------|-----------|
| Event writes | 500µs | Simulates network latency and client pacing |
| Queries | 1ms | Queries are CPU-intensive, need more spacing |
| Concurrent writes | 500µs | Balanced for mixed workloads |
| Burst writes | 500µs | Prevents CPU spikes during bursts |

### 3. Implementation Locations

#### Main Benchmark (Badger backend)

**Peak Throughput Test** ([main.go:471-473](main.go#L471-L473)):
```go
const eventDelay = 500 * time.Microsecond
time.Sleep(eventDelay) // After each event save
```

**Burst Pattern Test** ([main.go:599-600](main.go#L599-L600)):
```go
const eventDelay = 500 * time.Microsecond
time.Sleep(eventDelay) // In worker loop
```

**Query Test** ([main.go:899](main.go#L899)):
```go
time.Sleep(1 * time.Millisecond) // After each query
```

**Concurrent Query/Store** ([main.go:900, 1068](main.go#L900)):
```go
time.Sleep(1 * time.Millisecond)  // Readers
time.Sleep(500 * time.Microsecond) // Writers
```

#### BenchmarkAdapter (DGraph/Neo4j backends)

**Peak Throughput** ([benchmark_adapter.go:58](benchmark_adapter.go#L58)):
```go
const eventDelay = 500 * time.Microsecond
```

**Burst Pattern** ([benchmark_adapter.go:142](benchmark_adapter.go#L142)):
```go
const eventDelay = 500 * time.Microsecond
```

## Expected CPU Usage

### Before Optimization
- **Workers**: 12 (on 12-thread system)
- **Delays**: None or minimal
- **CPU Usage**: 95-100% sustained
- **System Impact**: Severe - other processes starved

### After Optimization
- **Workers**: 3 (on 12-thread system)
- **Delays**: 500µs-1ms per operation
- **Expected CPU Usage**: 40-60% average, 70% peak
- **System Impact**: Minimal - plenty of headroom for other processes

## Performance Impact

### Throughput Reduction
The aggressive rate limiting will reduce benchmark throughput:

**Before** (unrealistic, CPU-bound):
- ~50,000 events/second with 12 workers

**After** (realistic, rate-limited):
- ~5,000-10,000 events/second with 3 workers
- More representative of real-world relay load
- Network latency and client pacing simulated

### Latency Accuracy
**Improved**: With lower CPU contention, latency measurements are more accurate:
- Less queueing delay in database operations
- More consistent response times
- Better P95/P99 metric reliability

## Tuning Guide

If you need to adjust CPU usage further:

### Further Reduce CPU (< 40%)

1. **Reduce workers**:
   ```bash
   ./benchmark --workers 2  # Half of default
   ```

2. **Increase delays** in code:
   ```go
   // Change from 500µs to 1ms for writes
   const eventDelay = 1 * time.Millisecond

   // Change from 1ms to 2ms for queries
   time.Sleep(2 * time.Millisecond)
   ```

3. **Reduce event count**:
   ```bash
   ./benchmark --events 5000  # Shorter test runs
   ```

### Increase CPU (for faster testing)

1. **Increase workers**:
   ```bash
   ./benchmark --workers 6  # More concurrency
   ```

2. **Decrease delays** in code:
   ```go
   // Change from 500µs to 100µs
   const eventDelay = 100 * time.Microsecond

   // Change from 1ms to 500µs
   time.Sleep(500 * time.Microsecond)
   ```

## Monitoring CPU Usage

### Real-time Monitoring

```bash
# Terminal 1: Run benchmark
cd cmd/benchmark
./benchmark --workers 3 --events 10000

# Terminal 2: Monitor CPU
watch -n 1 'ps aux | grep benchmark | grep -v grep | awk "{print \$3\" %CPU\"}"'
```

### With htop (recommended)

```bash
# Install htop if needed
sudo apt install htop

# Run htop and filter for benchmark process
htop -p $(pgrep -f benchmark)
```

### System-wide CPU Usage

```bash
# Check overall system load
mpstat 1

# Or with sar
sar -u 1
```

## Docker Compose Considerations

When running the full benchmark suite in Docker Compose:

### Resource Limits

The compose file should limit CPU allocation:

```yaml
services:
  benchmark-runner:
    deploy:
      resources:
        limits:
          cpus: '4'  # Limit to 4 CPU cores
```

### Sequential vs Parallel

Current implementation runs benchmarks **sequentially** to avoid overwhelming the system.
Each relay is tested one at a time, ensuring:
- Consistent baseline for comparisons
- No CPU competition between tests
- Reliable latency measurements

## Best Practices

1. **Always monitor CPU during first run** to verify settings work for your system
2. **Close other applications** during benchmarking for consistent results
3. **Use consistent worker counts** across test runs for fair comparisons
4. **Document your settings** if you modify delay constants
5. **Test with small event counts first** (--events 1000) to verify CPU usage

## Realistic Workload Simulation

The delays aren't just for CPU management - they simulate real-world conditions:

- **500µs write delay**: Typical network round-trip time for local clients
- **1ms query delay**: Client thinking time between queries
- **3 workers**: Simulates 3 concurrent users/clients
- **Burst patterns**: Models social media posting patterns (busy hours vs quiet periods)

This makes benchmark results more applicable to production relay deployment planning.

## System Requirements

### Minimum
- 4 CPU cores (2 physical cores with hyperthreading)
- 8GB RAM
- SSD storage for database

### Recommended
- 6+ CPU cores
- 16GB RAM
- NVMe SSD

### For Full Suite (Docker Compose)
- 8+ CPU cores (allows multiple relays + benchmark runner)
- 32GB RAM (Neo4j, DGraph are memory-hungry)
- Fast SSD with 100GB+ free space

## Conclusion

These aggressive CPU optimizations ensure the benchmark suite:
- ✅ Runs reliably on modest hardware
- ✅ Doesn't interfere with other system processes
- ✅ Produces realistic, production-relevant metrics
- ✅ Completes without thermal throttling
- ✅ Allows fair comparison across different relay implementations

The trade-off is longer test duration, but the results are far more valuable for actual relay deployment planning.
