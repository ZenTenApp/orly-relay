# ORLY Relay Memory Optimization Analysis

This document analyzes ORLY's current memory optimization patterns against Go best practices for high-performance systems. The analysis covers buffer management, caching strategies, allocation patterns, and identifies optimization opportunities.

## Executive Summary

ORLY implements several sophisticated memory optimization strategies:
- **Compact event storage** achieving ~87% space savings via serial references
- **Two-level caching** for serial lookups and query results
- **ZSTD compression** for query cache with LRU eviction
- **Atomic operations** for lock-free statistics tracking
- **Pre-allocation patterns** for slice capacity management

However, several opportunities exist to further reduce GC pressure:
- Implement `sync.Pool` for frequently allocated buffers
- Use fixed-size arrays for cryptographic values
- Pool `bytes.Buffer` instances in hot paths
- Optimize escape behavior in serialization code

---

## Current Memory Patterns

### 1. Compact Event Storage

**Location**: `pkg/database/compact_event.go`

ORLY's most significant memory optimization is the compact binary format for event storage:

```
Original event:  32 (ID) + 32 (pubkey) + 32*4 (tags) = 192+ bytes
Compact format:   5 (pubkey serial) + 5*4 (tag serials) = 25 bytes
Savings: ~87% compression per event
```

**Key techniques:**
- 5-byte serial references replace 32-byte IDs/pubkeys
- Varint encoding for variable-length integers (CreatedAt, tag counts)
- Type flags for efficient deserialization
- Separate `SerialEventId` index for ID reconstruction

**Assessment**: Excellent storage optimization. This dramatically reduces database size and I/O costs.

### 2. Serial Cache System

**Location**: `pkg/database/serial_cache.go`

Two-way lookup cache for serial ↔ ID/pubkey mappings:

```go
type SerialCache struct {
    pubkeyBySerial     map[uint64][]byte      // For decoding
    serialByPubkeyHash map[string]uint64      // For encoding
    eventIdBySerial    map[uint64][]byte      // For decoding
    serialByEventIdHash map[string]uint64     // For encoding
}
```

**Memory footprint:**
- Pubkey cache: 100k entries × 32 bytes ≈ 3.2MB
- Event ID cache: 500k entries × 32 bytes ≈ 16MB
- Total: ~19-20MB overhead

**Strengths:**
- Fine-grained `RWMutex` locking per direction/type
- Configurable cache limits
- Defensive copying prevents external mutations

**Improvement opportunity:** The eviction strategy (clear 50% when full) is simple but not LRU. Consider ring buffers or generational caching for better hit rates.

### 3. Query Cache with ZSTD Compression

**Location**: `pkg/database/querycache/event_cache.go`

```go
type EventCache struct {
    entries   map[string]*EventCacheEntry
    lruList   *list.List
    encoder   *zstd.Encoder  // Reused encoder (level 9)
    decoder   *zstd.Decoder  // Reused decoder
    maxSize   int64          // Default 512MB compressed
}
```

**Strengths:**
- ZSTD level 9 compression (best ratio)
- Encoder/decoder reuse avoids repeated initialization
- LRU eviction with proper size tracking
- Background cleanup of expired entries
- Tracks compression ratio with exponential moving average

**Memory pattern:** Stores compressed data in cache, decompresses on-demand. This trades CPU for memory.

### 4. Buffer Allocation Patterns

**Current approach:** Uses `new(bytes.Buffer)` throughout serialization code:

```go
// pkg/database/save-event.go, compact_event.go, serial_cache.go
buf := new(bytes.Buffer)
// ... encode data
return buf.Bytes()
```

**Assessment:** Each call allocates a new buffer on the heap. For high-throughput scenarios (thousands of events/second), this creates significant GC pressure.

---

## Optimization Opportunities

### 1. Implement sync.Pool for Buffer Reuse

**Priority: High**

Currently, ORLY creates new `bytes.Buffer` instances for every serialization operation. A buffer pool would amortize allocation costs:

```go
// Recommended implementation
var bufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, 4096))
    },
}

func getBuffer() *bytes.Buffer {
    return bufferPool.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
    buf.Reset()
    bufferPool.Put(buf)
}
```

**Impact areas:**
- `pkg/database/compact_event.go` - MarshalCompactEvent, encodeCompactTag
- `pkg/database/save-event.go` - index key generation
- `pkg/database/serial_cache.go` - GetEventIdBySerial, StoreEventIdSerial

**Expected benefit:** 50-80% reduction in buffer allocations on hot paths.

### 2. Fixed-Size Array Types for Cryptographic Values

**Priority: Medium**

The external nostr library uses `[]byte` slices for IDs, pubkeys, and signatures. However, these are always fixed sizes:

| Type | Size | Current | Recommended |
|------|------|---------|-------------|
| Event ID | 32 bytes | `[]byte` | `[32]byte` |
| Pubkey | 32 bytes | `[]byte` | `[32]byte` |
| Signature | 64 bytes | `[]byte` | `[64]byte` |

Internal types like `Uint40` already follow this pattern but use struct wrapping:

```go
// Current (pkg/database/indexes/types/uint40.go)
type Uint40 struct{ value uint64 }

// Already efficient - no slice allocation
```

For cryptographic values, consider wrapper types:

```go
type EventID [32]byte
type Pubkey [32]byte
type Signature [64]byte

func (id EventID) IsZero() bool { return id == EventID{} }
func (id EventID) Hex() string  { return hex.Enc(id[:]) }
```

**Benefit:** Stack allocation for local variables, zero-value comparison efficiency.

### 3. Pre-allocated Slice Patterns

**Current usage is good:**

```go
// pkg/database/save-event.go:51-54
sers = make(types.Uint40s, 0, len(idxs)*100) // Estimate 100 serials per index

// pkg/database/compact_event.go:283
ev.Tags = tag.NewSWithCap(int(nTags)) // Pre-allocate tag slice
```

**Improvement:** Apply consistently to:
- `Uint40s.Union/Intersection/Difference` methods (currently use `append` without capacity hints)
- Query result accumulation in `query-events.go`

### 4. Escape Analysis Optimization

**Priority: Medium**

Several patterns cause unnecessary heap escapes. Check with:

```bash
go build -gcflags="-m -m" ./pkg/database/...
```

**Common escape causes in codebase:**

```go
// compact_event.go:224 - Small slice escapes
buf := make([]byte, 5)  // Could be [5]byte on stack

// compact_event.go:335 - Single-byte slice escapes
typeBuf := make([]byte, 1)  // Could be var typeBuf [1]byte
```

**Fix:**
```go
func readUint40(r io.Reader) (value uint64, err error) {
    var buf [5]byte  // Stack-allocated
    if _, err = io.ReadFull(r, buf[:]); err != nil {
        return 0, err
    }
    // ...
}
```

### 5. Atomic Bytes Wrapper Optimization

**Location**: `pkg/utils/atomic/bytes.go`

Current implementation copies on both Load and Store:

```go
func (x *Bytes) Load() (b []byte) {
    vb := x.v.Load().([]byte)
    b = make([]byte, len(vb))  // Allocation on every Load
    copy(b, vb)
    return
}
```

This is safe but expensive for high-frequency access. Consider:
- Read-copy-update (RCU) pattern for read-heavy workloads
- `sync.RWMutex` with direct access for controlled use cases

### 6. Goroutine Management

**Current patterns:**
- Worker goroutines for message processing (`app/listener.go`)
- Background cleanup goroutines (`querycache/event_cache.go`)
- Pinger goroutines per connection (`app/handle-websocket.go`)

**Assessment:** Good use of bounded channels and `sync.WaitGroup` for lifecycle management.

**Improvement:** Consider a worker pool for subscription handlers to limit peak goroutine count:

```go
type WorkerPool struct {
    jobs    chan func()
    workers int
    wg      sync.WaitGroup
}
```

---

## Memory Budget Analysis

### Runtime Memory Breakdown

| Component | Estimated Size | Notes |
|-----------|---------------|-------|
| Serial Cache (pubkeys) | 3.2 MB | 100k × 32 bytes |
| Serial Cache (event IDs) | 16 MB | 500k × 32 bytes |
| Query Cache | 512 MB | Configurable, compressed |
| Per-connection state | ~10 KB | Channels, buffers, maps |
| Badger DB caches | Variable | Controlled by Badger config |

### GC Tuning Recommendations

For a relay handling 1000+ events/second:

```go
// main.go or init
import "runtime/debug"

func init() {
    // More aggressive GC to limit heap growth
    debug.SetGCPercent(50)  // GC at 50% heap growth (default 100)

    // Set soft memory limit based on available RAM
    debug.SetMemoryLimit(2 << 30)  // 2GB limit
}
```

Or via environment:
```bash
GOGC=50 GOMEMLIMIT=2GiB ./orly
```

---

## Profiling Commands

### Heap Profile

```bash
# Enable pprof (already supported)
ORLY_PPROF_HTTP=true ./orly

# Capture heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Analyze allocations
go tool pprof -alloc_space heap.prof
go tool pprof -inuse_space heap.prof
```

### Escape Analysis

```bash
# Check which variables escape to heap
go build -gcflags="-m -m" ./pkg/database/... 2>&1 | grep "escapes to heap"
```

### Allocation Benchmarks

Add to existing benchmarks:

```go
func BenchmarkCompactMarshal(b *testing.B) {
    b.ReportAllocs()
    ev := createTestEvent()
    resolver := &testResolver{}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        data, _ := MarshalCompactEvent(ev, resolver)
        _ = data
    }
}
```

---

## Implementation Priority

1. **High Priority (Immediate Impact)**
   - Implement `sync.Pool` for `bytes.Buffer` in serialization paths
   - Replace small `make([]byte, n)` with fixed arrays in decode functions

2. **Medium Priority (Significant Improvement)**
   - Add pre-allocation hints to set operation methods
   - Optimize escape behavior in compact event encoding
   - Consider worker pool for subscription handlers

3. **Low Priority (Refinement)**
   - LRU-based serial cache eviction
   - Fixed-size types for cryptographic values (requires nostr library changes)
   - RCU pattern for atomic bytes in high-frequency paths

---

## Conclusion

ORLY demonstrates thoughtful memory optimization in its storage layer, particularly the compact event format achieving 87% space savings. The dual-cache architecture (serial cache + query cache) balances memory usage with lookup performance.

The primary opportunity for improvement is in the serialization hot path, where buffer pooling could significantly reduce GC pressure. The recommended `sync.Pool` implementation would have immediate benefits for high-throughput deployments without requiring architectural changes.

Secondary improvements around escape analysis and fixed-size types would provide incremental gains and should be prioritized based on profiling data from production workloads.
