# Go Memory Optimization Patterns

Detailed code examples and patterns for memory-efficient Go programming.

## Buffer Pool Implementations

### Tiered Buffer Pool

For workloads with varying buffer sizes:

```go
type TieredPool struct {
    small  sync.Pool  // 1KB
    medium sync.Pool  // 16KB
    large  sync.Pool  // 256KB
}

func NewTieredPool() *TieredPool {
    return &TieredPool{
        small:  sync.Pool{New: func() interface{} { return make([]byte, 1024) }},
        medium: sync.Pool{New: func() interface{} { return make([]byte, 16384) }},
        large:  sync.Pool{New: func() interface{} { return make([]byte, 262144) }},
    }
}

func (p *TieredPool) Get(size int) []byte {
    switch {
    case size <= 1024:
        return p.small.Get().([]byte)[:size]
    case size <= 16384:
        return p.medium.Get().([]byte)[:size]
    case size <= 262144:
        return p.large.Get().([]byte)[:size]
    default:
        return make([]byte, size)  // too large for pool
    }
}

func (p *TieredPool) Put(b []byte) {
    switch cap(b) {
    case 1024:
        p.small.Put(b[:cap(b)])
    case 16384:
        p.medium.Put(b[:cap(b)])
    case 262144:
        p.large.Put(b[:cap(b)])
    }
    // Non-standard sizes are not pooled
}
```

### bytes.Buffer Pool

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func GetBuffer() *bytes.Buffer {
    return bufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(b *bytes.Buffer) {
    b.Reset()
    bufferPool.Put(b)
}

// Usage
func processData(data []byte) string {
    buf := GetBuffer()
    defer PutBuffer(buf)

    buf.WriteString("prefix:")
    buf.Write(data)
    buf.WriteString(":suffix")

    return buf.String()  // allocates new string
}
```

## Zero-Allocation JSON Encoding

### Pre-allocated Encoder

```go
type JSONEncoder struct {
    buf    []byte
    scratch [64]byte  // for number formatting
}

func (e *JSONEncoder) Reset() {
    e.buf = e.buf[:0]
}

func (e *JSONEncoder) Bytes() []byte {
    return e.buf
}

func (e *JSONEncoder) WriteString(s string) {
    e.buf = append(e.buf, '"')
    for i := 0; i < len(s); i++ {
        c := s[i]
        switch c {
        case '"':
            e.buf = append(e.buf, '\\', '"')
        case '\\':
            e.buf = append(e.buf, '\\', '\\')
        case '\n':
            e.buf = append(e.buf, '\\', 'n')
        case '\r':
            e.buf = append(e.buf, '\\', 'r')
        case '\t':
            e.buf = append(e.buf, '\\', 't')
        default:
            if c < 0x20 {
                e.buf = append(e.buf, '\\', 'u', '0', '0',
                    hexDigits[c>>4], hexDigits[c&0xf])
            } else {
                e.buf = append(e.buf, c)
            }
        }
    }
    e.buf = append(e.buf, '"')
}

func (e *JSONEncoder) WriteInt(n int64) {
    e.buf = strconv.AppendInt(e.buf, n, 10)
}

func (e *JSONEncoder) WriteHex(b []byte) {
    e.buf = append(e.buf, '"')
    for _, v := range b {
        e.buf = append(e.buf, hexDigits[v>>4], hexDigits[v&0xf])
    }
    e.buf = append(e.buf, '"')
}

var hexDigits = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7',
                          '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}
```

### Append-Based Encoding

```go
// AppendJSON appends JSON representation to dst, returning extended slice
func (ev *Event) AppendJSON(dst []byte) []byte {
    dst = append(dst, `{"id":"`...)
    dst = appendHex(dst, ev.ID[:])
    dst = append(dst, `","pubkey":"`...)
    dst = appendHex(dst, ev.Pubkey[:])
    dst = append(dst, `","created_at":`...)
    dst = strconv.AppendInt(dst, ev.CreatedAt, 10)
    dst = append(dst, `,"kind":`...)
    dst = strconv.AppendInt(dst, int64(ev.Kind), 10)
    dst = append(dst, `,"content":`...)
    dst = appendJSONString(dst, ev.Content)
    dst = append(dst, '}')
    return dst
}

// Usage with pre-allocated buffer
func encodeEvents(events []Event) []byte {
    // Estimate size: ~500 bytes per event
    buf := make([]byte, 0, len(events)*500)
    buf = append(buf, '[')
    for i, ev := range events {
        if i > 0 {
            buf = append(buf, ',')
        }
        buf = ev.AppendJSON(buf)
    }
    buf = append(buf, ']')
    return buf
}
```

## Memory-Efficient String Building

### strings.Builder with Preallocation

```go
func buildQuery(parts []string) string {
    // Calculate total length
    total := len(parts) - 1  // for separators
    for _, p := range parts {
        total += len(p)
    }

    var b strings.Builder
    b.Grow(total)  // single allocation

    for i, p := range parts {
        if i > 0 {
            b.WriteByte(',')
        }
        b.WriteString(p)
    }
    return b.String()
}
```

### Avoiding String Concatenation

```go
// BAD: O(n^2) allocations
func buildPath(parts []string) string {
    result := ""
    for _, p := range parts {
        result += "/" + p  // new allocation each iteration
    }
    return result
}

// GOOD: O(n) with single allocation
func buildPath(parts []string) string {
    if len(parts) == 0 {
        return ""
    }
    n := len(parts)  // for slashes
    for _, p := range parts {
        n += len(p)
    }

    b := make([]byte, 0, n)
    for _, p := range parts {
        b = append(b, '/')
        b = append(b, p...)
    }
    return string(b)
}
```

### Unsafe String/Byte Conversion

```go
import "unsafe"

// Zero-allocation string to []byte (read-only!)
func unsafeBytes(s string) []byte {
    return unsafe.Slice(unsafe.StringData(s), len(s))
}

// Zero-allocation []byte to string (b must not be modified!)
func unsafeString(b []byte) string {
    return unsafe.String(unsafe.SliceData(b), len(b))
}

// Use when:
// 1. Converting string for read-only operations (hashing, comparison)
// 2. Returning []byte from buffer that won't be modified
// 3. Performance-critical paths with careful ownership management
```

## Slice Capacity Management

### Append Growth Patterns

```go
// Slice growth: 0 -> 1 -> 2 -> 4 -> 8 -> 16 -> 32 -> 64 -> ...
// After 1024: grows by 25% each time

// BAD: Unknown final size causes multiple reallocations
func collectItems() []Item {
    var items []Item
    for item := range source {
        items = append(items, item)  // may reallocate multiple times
    }
    return items
}

// GOOD: Preallocate when size is known
func collectItems(n int) []Item {
    items := make([]Item, 0, n)
    for item := range source {
        items = append(items, item)
    }
    return items
}

// GOOD: Use slice header trick for uncertain sizes
func collectItems() []Item {
    items := make([]Item, 0, 32)  // reasonable initial capacity
    for item := range source {
        items = append(items, item)
    }
    // Trim excess capacity if items will be long-lived
    return items[:len(items):len(items)]
}
```

### Slice Recycling

```go
// Reuse slice backing array
func processInBatches(items []Item, batchSize int) {
    batch := make([]Item, 0, batchSize)

    for i, item := range items {
        batch = append(batch, item)

        if len(batch) == batchSize || i == len(items)-1 {
            processBatch(batch)
            batch = batch[:0]  // reset length, keep capacity
        }
    }
}
```

### Preventing Slice Memory Leaks

```go
// BAD: Subslice keeps entire backing array alive
func getFirst10(data []byte) []byte {
    return data[:10]  // entire data array stays in memory
}

// GOOD: Copy to release original array
func getFirst10(data []byte) []byte {
    result := make([]byte, 10)
    copy(result, data[:10])
    return result
}

// Alternative: explicit capacity limit
func getFirst10(data []byte) []byte {
    return data[:10:10]  // cap=10, can't accidentally grow into original
}
```

## Struct Layout Optimization

### Field Ordering for Alignment

```go
// BAD: 32 bytes due to padding
type BadLayout struct {
    a bool      // 1 byte + 7 padding
    b int64     // 8 bytes
    c bool      // 1 byte + 7 padding
    d int64     // 8 bytes
}

// GOOD: 24 bytes with optimal ordering
type GoodLayout struct {
    b int64     // 8 bytes
    d int64     // 8 bytes
    a bool      // 1 byte
    c bool      // 1 byte + 6 padding
}

// Rule: Order fields from largest to smallest alignment
```

### Checking Struct Size

```go
func init() {
    // Compile-time size assertions
    var _ [24]byte = [unsafe.Sizeof(GoodLayout{})]byte{}

    // Or runtime check
    if unsafe.Sizeof(Event{}) > 256 {
        panic("Event struct too large")
    }
}
```

### Cache-Line Optimization

```go
const CacheLineSize = 64

// Pad struct to prevent false sharing in concurrent access
type PaddedCounter struct {
    value uint64
    _     [CacheLineSize - 8]byte  // padding
}

type Counters struct {
    reads  PaddedCounter
    writes PaddedCounter
    // Each counter on separate cache line
}
```

## Object Reuse Patterns

### Reset Methods

```go
type Request struct {
    Method  string
    Path    string
    Headers map[string]string
    Body    []byte
}

func (r *Request) Reset() {
    r.Method = ""
    r.Path = ""
    // Reuse map, just clear entries
    for k := range r.Headers {
        delete(r.Headers, k)
    }
    r.Body = r.Body[:0]
}

var requestPool = sync.Pool{
    New: func() interface{} {
        return &Request{
            Headers: make(map[string]string, 8),
            Body:    make([]byte, 0, 1024),
        }
    },
}
```

### Flyweight Pattern

```go
// Share immutable parts across many instances
type Event struct {
    kind    *Kind   // shared, immutable
    content string
}

type Kind struct {
    ID          int
    Name        string
    Description string
}

var kindRegistry = map[int]*Kind{
    0: {0, "set_metadata", "User metadata"},
    1: {1, "text_note", "Text note"},
    // ... pre-allocated, shared across all events
}

func NewEvent(kindID int, content string) Event {
    return Event{
        kind:    kindRegistry[kindID],  // no allocation
        content: content,
    }
}
```

## Channel Patterns for Memory Efficiency

### Buffered Channels as Object Pools

```go
type SimplePool struct {
    pool chan *Buffer
}

func NewSimplePool(size int) *SimplePool {
    p := &SimplePool{pool: make(chan *Buffer, size)}
    for i := 0; i < size; i++ {
        p.pool <- NewBuffer()
    }
    return p
}

func (p *SimplePool) Get() *Buffer {
    select {
    case b := <-p.pool:
        return b
    default:
        return NewBuffer()  // pool empty, allocate new
    }
}

func (p *SimplePool) Put(b *Buffer) {
    select {
    case p.pool <- b:
    default:
        // pool full, let GC collect
    }
}
```

### Batch Processing Channels

```go
// Reduce channel overhead by batching
func batchProcessor(input <-chan Item, batchSize int) <-chan []Item {
    output := make(chan []Item)
    go func() {
        defer close(output)
        batch := make([]Item, 0, batchSize)

        for item := range input {
            batch = append(batch, item)
            if len(batch) == batchSize {
                output <- batch
                batch = make([]Item, 0, batchSize)
            }
        }
        if len(batch) > 0 {
            output <- batch
        }
    }()
    return output
}
```

## Advanced Techniques

### Manual Memory Management with mmap

```go
import "golang.org/x/sys/unix"

// Allocate memory outside Go heap
func allocateMmap(size int) ([]byte, error) {
    data, err := unix.Mmap(-1, 0, size,
        unix.PROT_READ|unix.PROT_WRITE,
        unix.MAP_ANON|unix.MAP_PRIVATE)
    return data, err
}

func freeMmap(data []byte) error {
    return unix.Munmap(data)
}
```

### Inline Arrays in Structs

```go
// Small-size optimization: inline for small, pointer for large
type SmallVec struct {
    len   int
    small [8]int      // inline storage for ≤8 elements
    large []int       // heap storage for >8 elements
}

func (v *SmallVec) Append(x int) {
    if v.large != nil {
        v.large = append(v.large, x)
        v.len++
        return
    }
    if v.len < 8 {
        v.small[v.len] = x
        v.len++
        return
    }
    // Spill to heap
    v.large = make([]int, 9, 16)
    copy(v.large, v.small[:])
    v.large[8] = x
    v.len++
}
```

### Bump Allocator

```go
// Simple arena-style allocator for batch allocations
type BumpAllocator struct {
    buf []byte
    off int
}

func NewBumpAllocator(size int) *BumpAllocator {
    return &BumpAllocator{buf: make([]byte, size)}
}

func (a *BumpAllocator) Alloc(size int) []byte {
    if a.off+size > len(a.buf) {
        panic("bump allocator exhausted")
    }
    b := a.buf[a.off : a.off+size]
    a.off += size
    return b
}

func (a *BumpAllocator) Reset() {
    a.off = 0
}

// Usage: allocate many small objects, reset all at once
func processBatch(items []Item) {
    arena := NewBumpAllocator(1 << 20)  // 1MB
    defer arena.Reset()

    for _, item := range items {
        buf := arena.Alloc(item.Size())
        item.Serialize(buf)
    }
}
```
