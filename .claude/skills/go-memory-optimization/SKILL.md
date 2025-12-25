---
name: go-memory-optimization
description: This skill should be used when optimizing Go code for memory efficiency, reducing GC pressure, implementing object pooling, analyzing escape behavior, choosing between fixed-size arrays and slices, designing worker pools, or profiling memory allocations. Provides comprehensive knowledge of Go's memory model, stack vs heap allocation, sync.Pool patterns, goroutine reuse, and GC tuning.
---

# Go Memory Optimization

## Overview

This skill provides guidance on optimizing Go programs for memory efficiency and reduced garbage collection overhead. Topics include stack allocation semantics, fixed-size types, escape analysis, object pooling, goroutine management, and GC tuning.

## Core Principles

### The Allocation Hierarchy

Prefer allocations in this order (fastest to slowest):

1. **Stack allocation** - Zero GC cost, automatic cleanup on function return
2. **Pooled objects** - Amortized allocation cost via sync.Pool
3. **Pre-allocated buffers** - Single allocation, reused across operations
4. **Heap allocation** - GC-managed, use when lifetime exceeds function scope

### When Optimization Matters

Focus memory optimization efforts on:
- Hot paths executed thousands/millions of times per second
- Large objects (>32KB) that stress the GC
- Long-running services where GC pauses affect latency
- Memory-constrained environments

Avoid premature optimization. Profile first with `go tool pprof` to identify actual bottlenecks.

## Fixed-Size Types vs Slices

### Stack Allocation with Arrays

Arrays with known compile-time size can be stack-allocated, avoiding heap entirely:

```go
// HEAP: slice header + backing array escape to heap
func processSlice() []byte {
    data := make([]byte, 32)
    // ... use data
    return data  // escapes
}

// STACK: fixed array stays on stack if doesn't escape
func processArray() {
    var data [32]byte  // stack-allocated
    // ... use data
}   // automatically cleaned up
```

### Fixed-Size Binary Types Pattern

Define types with explicit sizes for protocol fields, cryptographic values, and identifiers:

```go
// Binary types enforce length and enable stack allocation
type EventID [32]byte      // SHA256 hash
type Pubkey [32]byte       // Schnorr public key
type Signature [64]byte    // Schnorr signature

// Methods operate on value receivers when size permits
func (id EventID) Hex() string {
    return hex.EncodeToString(id[:])
}

func (id EventID) IsZero() bool {
    return id == EventID{}  // efficient zero-value comparison
}
```

### Size Thresholds

| Size | Recommendation |
|------|----------------|
| ≤64 bytes | Pass by value, stack-friendly |
| 65-128 bytes | Consider context; value for read-only, pointer for mutation |
| >128 bytes | Pass by pointer to avoid copy overhead |

### Array to Slice Conversion

Convert fixed arrays to slices only at API boundaries:

```go
type Hash [32]byte

func (h Hash) Bytes() []byte {
    return h[:]  // creates slice header, array stays on stack if h does
}

// Prefer methods that accept arrays directly
func VerifySignature(pubkey Pubkey, msg []byte, sig Signature) bool {
    // pubkey and sig are stack-allocated in caller
}
```

## Escape Analysis

### Understanding Escape

Variables "escape" to the heap when the compiler cannot prove their lifetime is bounded by the stack frame. Check escape behavior with:

```bash
go build -gcflags="-m -m" ./...
```

### Common Escape Causes

```go
// 1. Returning pointers to local variables
func escapes() *int {
    x := 42
    return &x  // x escapes
}

// 2. Storing in interface{}
func escapes(x int) interface{} {
    return x  // x escapes (boxed)
}

// 3. Closures capturing by reference
func escapes() func() int {
    x := 42
    return func() int { return x }  // x escapes
}

// 4. Slice/map with unknown capacity
func escapes(n int) []byte {
    return make([]byte, n)  // escapes (size unknown at compile time)
}

// 5. Sending pointers to channels
func escapes(ch chan *int) {
    x := 42
    ch <- &x  // x escapes
}
```

### Preventing Escape

```go
// 1. Accept pointers, don't return them
func noEscape(result *[32]byte) {
    // caller owns memory, function fills it
    copy(result[:], computeHash())
}

// 2. Use fixed-size arrays
func noEscape() {
    var buf [1024]byte  // known size, stack-allocated
    process(buf[:])
}

// 3. Preallocate with known capacity
func noEscape() {
    buf := make([]byte, 0, 1024)  // may stay on stack
    // ... append up to 1024 bytes
}

// 4. Avoid interface{} on hot paths
func noEscape(x int) int {
    return x * 2  // no boxing
}
```

## sync.Pool Usage

### Basic Pattern

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func processRequest(data []byte) {
    buf := bufferPool.Get().([]byte)
    buf = buf[:0]  // reset length, keep capacity
    defer bufferPool.Put(buf)

    // use buf...
}
```

### Typed Pool Wrapper

```go
type BufferPool struct {
    pool sync.Pool
    size int
}

func NewBufferPool(size int) *BufferPool {
    return &BufferPool{
        pool: sync.Pool{
            New: func() interface{} {
                b := make([]byte, size)
                return &b
            },
        },
        size: size,
    }
}

func (p *BufferPool) Get() *[]byte {
    return p.pool.Get().(*[]byte)
}

func (p *BufferPool) Put(b *[]byte) {
    if b == nil || cap(*b) < p.size {
        return  // don't pool undersized buffers
    }
    *b = (*b)[:p.size]  // reset to full size
    p.pool.Put(b)
}
```

### Pool Anti-Patterns

```go
// BAD: Pool of pointers to small values (overhead exceeds benefit)
var intPool = sync.Pool{New: func() interface{} { return new(int) }}

// BAD: Not resetting state before Put
bufPool.Put(buf)  // may contain sensitive data

// BAD: Pooling objects with goroutine-local state
var connPool = sync.Pool{...}  // connections are stateful

// BAD: Assuming pooled objects persist (GC clears pools)
obj := pool.Get()
// ... long delay
pool.Put(obj)  // obj may have been GC'd during delay
```

### When to Use sync.Pool

| Use Case | Pool? | Reason |
|----------|-------|--------|
| Buffers in HTTP handlers | Yes | High allocation rate, short lifetime |
| Encoder/decoder state | Yes | Expensive to initialize |
| Small values (<64 bytes) | No | Pointer overhead exceeds benefit |
| Long-lived objects | No | Pools are for short-lived reuse |
| Objects with cleanup needs | No | Pool provides no finalization |

## Goroutine Pooling

### Worker Pool Pattern

```go
type WorkerPool struct {
    jobs    chan func()
    workers int
    wg      sync.WaitGroup
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
    p := &WorkerPool{
        jobs:    make(chan func(), queueSize),
        workers: workers,
    }
    p.wg.Add(workers)
    for i := 0; i < workers; i++ {
        go p.worker()
    }
    return p
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()
    for job := range p.jobs {
        job()
    }
}

func (p *WorkerPool) Submit(job func()) {
    p.jobs <- job
}

func (p *WorkerPool) Shutdown() {
    close(p.jobs)
    p.wg.Wait()
}
```

### Bounded Concurrency with Semaphore

```go
type Semaphore struct {
    sem chan struct{}
}

func NewSemaphore(n int) *Semaphore {
    return &Semaphore{sem: make(chan struct{}, n)}
}

func (s *Semaphore) Acquire() { s.sem <- struct{}{} }
func (s *Semaphore) Release() { <-s.sem }

// Usage
sem := NewSemaphore(runtime.GOMAXPROCS(0))
for _, item := range items {
    sem.Acquire()
    go func(it Item) {
        defer sem.Release()
        process(it)
    }(item)
}
```

### Goroutine Reuse Benefits

| Metric | Spawn per request | Worker pool |
|--------|-------------------|-------------|
| Goroutine creation | O(n) | O(workers) |
| Stack allocation | 2KB × n | 2KB × workers |
| Scheduler overhead | Higher | Lower |
| GC pressure | Higher | Lower |

## Reducing GC Pressure

### Allocation Reduction Strategies

```go
// 1. Reuse buffers across iterations
buf := make([]byte, 0, 4096)
for _, item := range items {
    buf = buf[:0]  // reset without reallocation
    buf = processItem(buf, item)
}

// 2. Preallocate slices with known length
result := make([]Item, 0, len(input))  // avoid append reallocations
for _, in := range input {
    result = append(result, transform(in))
}

// 3. Struct embedding instead of pointer fields
type Event struct {
    ID        [32]byte    // embedded, not *[32]byte
    Pubkey    [32]byte    // single allocation for entire struct
    Signature [64]byte
    Content   string      // only string data on heap
}

// 4. String interning for repeated values
var kindStrings = map[int]string{
    0: "set_metadata",
    1: "text_note",
    // ...
}
```

### GC Tuning

```go
import "runtime/debug"

func init() {
    // GOGC: target heap growth percentage (default 100)
    // Lower = more frequent GC, less memory
    // Higher = less frequent GC, more memory
    debug.SetGCPercent(50)  // GC when heap grows 50%

    // GOMEMLIMIT: soft memory limit (Go 1.19+)
    // GC becomes more aggressive as limit approaches
    debug.SetMemoryLimit(512 << 20)  // 512MB limit
}
```

Environment variables:

```bash
GOGC=50              # More aggressive GC
GOMEMLIMIT=512MiB    # Soft memory limit
GODEBUG=gctrace=1    # GC trace output
```

### Arena Allocation (Go 1.20+, experimental)

```go
//go:build goexperiment.arenas

import "arena"

func processLargeDataset(data []byte) Result {
    a := arena.NewArena()
    defer a.Free()  // bulk free all allocations

    // All allocations from arena are freed together
    items := arena.MakeSlice[Item](a, 0, 1000)
    // ... process

    // Copy result out before Free
    return copyResult(result)
}
```

## Memory Profiling

### Heap Profile

```go
import "runtime/pprof"

func captureHeapProfile() {
    f, _ := os.Create("heap.prof")
    defer f.Close()
    runtime.GC()  // get accurate picture
    pprof.WriteHeapProfile(f)
}
```

```bash
go tool pprof -http=:8080 heap.prof
go tool pprof -alloc_space heap.prof  # total allocations
go tool pprof -inuse_space heap.prof  # current usage
```

### Allocation Benchmarks

```go
func BenchmarkAllocation(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        result := processData(input)
        _ = result
    }
}
```

Output interpretation:

```
BenchmarkAllocation-8  1000000  1234 ns/op  256 B/op  3 allocs/op
                                            ↑         ↑
                                   bytes/op   allocations/op
```

### Live Memory Monitoring

```go
func printMemStats() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    fmt.Printf("Alloc: %d MB\n", m.Alloc/1024/1024)
    fmt.Printf("TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
    fmt.Printf("Sys: %d MB\n", m.Sys/1024/1024)
    fmt.Printf("NumGC: %d\n", m.NumGC)
    fmt.Printf("GCPause: %v\n", time.Duration(m.PauseNs[(m.NumGC+255)%256]))
}
```

## Common Patterns Reference

For detailed code examples and patterns, see `references/patterns.md`:

- Buffer pool implementations
- Zero-allocation JSON encoding
- Memory-efficient string building
- Slice capacity management
- Struct layout optimization

## Checklist for Memory-Critical Code

1. [ ] Profile before optimizing (`go tool pprof`)
2. [ ] Check escape analysis output (`-gcflags="-m"`)
3. [ ] Use fixed-size arrays for known-size data
4. [ ] Implement sync.Pool for frequently allocated objects
5. [ ] Preallocate slices with known capacity
6. [ ] Reuse buffers instead of allocating new ones
7. [ ] Consider struct field ordering for alignment
8. [ ] Benchmark with `-benchmem` flag
9. [ ] Set appropriate GOGC/GOMEMLIMIT for production
10. [ ] Monitor GC behavior with GODEBUG=gctrace=1
