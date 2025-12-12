# Rate Limiting Test Report: Neo4j Backend

**Test Date:** December 12, 2025
**Test Duration:** 73 minutes (4,409 seconds)
**Import File:** `wot_reference.jsonl` (2.7 GB, 2,158,366 events)

## Configuration

| Parameter | Value |
|-----------|-------|
| Database Backend | Neo4j 5-community (Docker) |
| Target Memory | 1,500 MB (relay process) |
| Emergency Threshold | 1,167 (target + 1/6) |
| Recovery Threshold | 833 (target - 1/6) |
| Max Write Delay | 1,000 ms (normal), 5,000 ms (emergency) |
| Neo4j Memory Limits | Heap: 512MB-1GB, Page Cache: 512MB |

## Results Summary

### Memory Management

| Component | Metric | Value |
|-----------|--------|-------|
| **Relay Process** | Peak RSS (VmHWM) | 148 MB |
| **Relay Process** | Final RSS | 35 MB |
| **Neo4j Container** | Memory Usage | 1.614 GB |
| **Neo4j Container** | Memory % | 10.83% of 14.91GB |
| **Rate Limiting** | Events Triggered | **0** |

### Key Finding: Architecture Difference

Unlike Badger (embedded database), Neo4j runs as a **separate process** in a Docker container. This means:

1. **Relay process memory stays low** (~35MB) because it's just a client
2. **Neo4j manages its own memory** within the container (1.6GB used)
3. **Rate limiter monitors relay RSS**, which doesn't reflect Neo4j's actual load
4. **No rate limiting triggered** because relay memory never approached the 1.5GB target

This is architecturally correct - the relay doesn't need memory-based rate limiting for Neo4j because it's not holding the data in process.

### Event Processing

| Event Type | Count | Rate |
|------------|-------|------|
| Contact Lists (kind 3) | 174,836 | 40 events/sec |
| Mute Lists (kind 10000) | 4,027 | 0.9 events/sec |
| **Total Social Events** | **178,863** | **41 events/sec** |

### Neo4j Performance

| Metric | Value |
|--------|-------|
| CPU Usage | 40-45% |
| Memory | Stable at 1.6GB |
| Disk Writes | 12.7 GB |
| Network In | 1.8 GB |
| Network Out | 583 MB |
| Process Count | 77-82 |

### Import Throughput Over Time

```
Time      Contact Lists  Delta/min  Neo4j Memory
------    -------------  ---------  ------------
08:28     0              -          1.57 GB
08:47     31,257         ~2,100     1.61 GB
08:52     42,403         ~2,200     1.61 GB
09:02     67,581         ~2,500     1.61 GB
09:12     97,316         ~3,000     1.60 GB
09:22     112,681        ~3,100     1.61 GB
09:27     163,252        ~10,000*   1.61 GB
09:41     174,836        ~2,400     1.61 GB
```
*Spike may be due to batch processing of cached events

### Memory Stability

Neo4j's memory usage remained remarkably stable throughout the test:

```
Sample      Memory     Delta
--------    --------   -----
08:47       1.605 GB   -
09:02       1.611 GB   +6 MB
09:12       1.603 GB   -8 MB
09:27       1.607 GB   +4 MB
09:41       1.614 GB   +7 MB
```

**Variance:** < 15 MB over 73 minutes - excellent stability.

## Architecture Comparison: Badger vs Neo4j

| Aspect | Badger | Neo4j |
|--------|--------|-------|
| Database Type | Embedded | External (Docker) |
| Memory Consumer | Relay process | Container process |
| Rate Limiter Target | Relay RSS | Relay RSS |
| Rate Limiting Effectiveness | High | Low* |
| Compaction Triggering | Yes | N/A |
| Emergency Mode | Yes | Not triggered |

*The current rate limiter design targets relay process memory, which doesn't reflect Neo4j's actual resource usage.

## Recommendations for Neo4j Rate Limiting

The current implementation monitors **relay process memory**, but for Neo4j this should be enhanced to monitor:

### 1. Query Latency-Based Throttling (Currently Implemented)
The Neo4j monitor already tracks query latency via `RecordQueryLatency()` and `RecordWriteLatency()`, using EMA smoothing. Latency > 500ms increases reported load.

### 2. Connection Pool Saturation (Currently Implemented)
The `querySem` semaphore limits concurrent queries (default 10). When full, the load metric increases.

### 3. Future Enhancement: Container Metrics
Consider monitoring Neo4j container metrics via:
- Docker stats API for memory/CPU
- Neo4j metrics endpoint for transaction counts, cache hit rates
- JMX metrics for heap usage and GC pressure

## Conclusion

The Neo4j import test demonstrated:

1. **Stable Memory Usage**: Neo4j maintained consistent 1.6GB memory throughout
2. **Consistent Throughput**: ~40 social events/second with no degradation
3. **Architectural Isolation**: Relay stays lightweight while Neo4j handles data
4. **Rate Limiter Design**: Current RSS-based limiting is appropriate for Badger but less relevant for Neo4j

**Recommendation:** The Neo4j rate limiter is correctly implemented but relies on latency and concurrency metrics rather than memory pressure. For production deployments with Neo4j, configure appropriate Neo4j memory limits in the container (heap_initial, heap_max, pagecache) rather than relying on relay-side rate limiting.

## Test Environment

- **OS:** Linux 6.8.0-87-generic
- **Architecture:** x86_64
- **Go Version:** 1.25.3
- **Neo4j Version:** 5.26.18 (community)
- **Container:** Docker with 14.91GB limit
- **Neo4j Settings:**
  - Heap Initial: 512MB
  - Heap Max: 1GB
  - Page Cache: 512MB
