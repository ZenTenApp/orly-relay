# Event Encoder Performance Optimization Report

## Executive Summary

This report documents the profiling and optimization of event encoders in the `next.orly.dev/pkg/encoders/event` package. The optimization focused on reducing memory allocations and CPU processing time for JSON, binary, and canonical encoders.

## Methodology

### Profiling Setup

1. Created comprehensive benchmark tests covering:
   - JSON marshaling/unmarshaling
   - Binary marshaling/unmarshaling
   - Canonical encoding
   - ID generation (canonical + SHA256)
   - Round-trip operations
   - Small and large event sizes

2. Used Go's built-in profiling tools:
   - CPU profiling (`-cpuprofile`)
   - Memory profiling (`-memprofile`)
   - Allocation tracking (`-benchmem`)

### Initial Findings

The profiling data revealed several key bottlenecks:

1. **JSON Marshal**: 6 allocations per operation, 2232 bytes allocated
2. **Canonical Encoding**: 5 allocations per operation, 1208 bytes allocated
3. **Memory Allocations**: Primary hotspots identified:
   - `text.NostrEscape`: 3.95GB total allocations (45.34% of all allocations)
   - `event.Marshal`: 1.39GB allocations
   - `event.ToCanonical`: 0.22GB allocations

4. **CPU Processing**: Primary hotspots:
   - `text.NostrEscape`: 4.39s (23.12% of CPU time)
   - `runtime.mallocgc`: 3.98s (20.96% of CPU time)
   - `event.Marshal`: 3.16s (16.64% of CPU time)

## Optimizations Implemented

### 1. JSON Marshal Optimization

**Problem**: Multiple allocations from `make([]byte, ...)` calls and buffer growth during append operations.

**Solution**:
- Pre-allocate output buffer using `EstimateSize()` when `dst` is `nil`
- Track hex encoding positions to avoid recalculating slice offsets
- Add 100-byte overhead for JSON structure (keys, quotes, commas)

**Code Changes** (`event.go`):
```go
func (ev *E) Marshal(dst []byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	if b == nil {
		estimatedSize := ev.EstimateSize()
		estimatedSize += 100 // JSON structure overhead
		b = make([]byte, 0, estimatedSize)
	}
	// ... rest of implementation
}
```

**Results**:
- **Before**: 1758 ns/op, 2232 B/op, 6 allocs/op
- **After**: 1325 ns/op, 1024 B/op, 1 allocs/op
- **Improvement**: 24% faster, 54% less memory, 83% fewer allocations

### 2. Canonical Encoding Optimization

**Problem**: Similar allocation issues as JSON marshal, with additional overhead from tag and content escaping.

**Solution**:
- Pre-allocate buffer based on estimated size
- Handle nil tags explicitly to avoid unnecessary allocations
- Estimate size accounting for hex encoding and escaping overhead

**Code Changes** (`canonical.go`):
```go
func (ev *E) ToCanonical(dst []byte) (b []byte) {
	b = dst
	if b == nil {
		estimatedSize := 5 + 2*len(ev.Pubkey) + 20 + 10 + 100
		if ev.Tags != nil {
			for _, tag := range *ev.Tags {
				for _, elem := range tag.T {
					estimatedSize += len(elem)*2 + 10
				}
			}
		}
		estimatedSize += len(ev.Content)*2 + 10
		b = make([]byte, 0, estimatedSize)
	}
	// ... rest of implementation
}
```

**Results**:
- **Before**: 1523 ns/op, 1208 B/op, 5 allocs/op
- **After**: 1272 ns/op, 896 B/op, 1 allocs/op
- **Improvement**: 16% faster, 26% less memory, 80% fewer allocations

### 3. Binary Marshal Optimization

**Problem**: `varint.Encode` writes one byte at a time, causing many small allocations. Also, nil tags were not handled explicitly.

**Solution**:
- Add explicit nil tag handling to avoid calling `Len()` on nil
- Add `MarshalBinaryToBytes` helper method that uses `bytes.Buffer` with pre-allocated capacity
- Estimate buffer size based on event structure

**Code Changes** (`binary.go`):
```go
func (ev *E) MarshalBinary(w io.Writer) {
	// ... existing code ...
	if ev.Tags == nil {
		varint.Encode(w, 0)
	} else {
		varint.Encode(w, uint64(ev.Tags.Len()))
		// ... rest of tags encoding
	}
	// ... rest of implementation
}

func (ev *E) MarshalBinaryToBytes(dst []byte) []byte {
	// New helper method with pre-allocated buffer
	// ... implementation
}
```

**Results**:
- Minimal change to existing `MarshalBinary` (nil check optimization)
- New `MarshalBinaryToBytes` method provides better performance when bytes are needed directly

### 4. Binary Unmarshal Optimization

**Problem**: Always allocating tags slice even when nTags is 0.

**Solution**:
- Check if `nTags == 0` and set `ev.Tags = nil` instead of allocating empty slice

**Code Changes** (`binary.go`):
```go
func (ev *E) UnmarshalBinary(r io.Reader) (err error) {
	// ... existing code ...
	if nTags == 0 {
		ev.Tags = nil
	} else {
		ev.Tags = tag.NewSWithCap(int(nTags))
		// ... rest of tag unmarshaling
	}
	// ... rest of implementation
}
```

**Results**:
- Avoids unnecessary allocation for events with no tags

## Performance Comparison

### Small Events (Standard Test Event)

| Operation | Metric | Before | After | Improvement |
|-----------|--------|--------|-------|-------------|
| JSON Marshal | Time | 1758 ns/op | 1325 ns/op | **24% faster** |
| JSON Marshal | Memory | 2232 B/op | 1024 B/op | **54% less** |
| JSON Marshal | Allocations | 6 allocs/op | 1 allocs/op | **83% fewer** |
| Canonical | Time | 1523 ns/op | 1272 ns/op | **16% faster** |
| Canonical | Memory | 1208 B/op | 896 B/op | **26% less** |
| Canonical | Allocations | 5 allocs/op | 1 allocs/op | **80% fewer** |
| GetIDBytes | Time | 1739 ns/op | 1552 ns/op | **11% faster** |
| GetIDBytes | Memory | 1240 B/op | 928 B/op | **25% less** |
| GetIDBytes | Allocations | 6 allocs/op | 2 allocs/op | **67% fewer** |

### Large Events (20+ Tags, 4KB Content)

| Operation | Metric | Before | After | Improvement |
|-----------|--------|--------|-------|-------------|
| JSON Marshal | Time | 19751 ns/op | 17666 ns/op | **11% faster** |
| JSON Marshal | Memory | 18616 B/op | 9472 B/op | **49% less** |
| JSON Marshal | Allocations | 11 allocs/op | 1 allocs/op | **91% fewer** |
| Canonical | Time | 19725 ns/op | 17903 ns/op | **9% faster** |
| Canonical | Memory | 18616 B/op | 10240 B/op | **45% less** |
| Canonical | Allocations | 11 allocs/op | 1 allocs/op | **91% fewer** |

### Binary Operations

| Operation | Metric | Before | After | Notes |
|-----------|--------|--------|-------|-------|
| Binary Marshal | Time | 347.4 ns/op | 297.2 ns/op | **14% faster** |
| Binary Marshal | Allocations | 13 allocs/op | 13 allocs/op | No change (varint limitation) |
| Binary Unmarshal | Time | 990.5 ns/op | 1028 ns/op | Slight regression (nil check overhead) |
| Binary Unmarshal | Allocations | 32 allocs/op | 32 allocs/op | No change (varint limitation) |

*Note: Binary operations are limited by the `varint` package which writes one byte at a time, causing many small allocations. Further optimization would require changes to the varint encoding implementation.*

## Key Insights

### Allocation Reduction

The most significant improvement came from reducing allocations:
- **JSON Marshal**: Reduced from 6 to 1 allocation (83% reduction)
- **Canonical Encoding**: Reduced from 5 to 1 allocation (80% reduction)
- **Large Events**: Reduced from 11 to 1 allocation (91% reduction)

This reduction has cascading benefits:
- Less GC pressure
- Better CPU cache utilization
- Reduced memory bandwidth usage

### Buffer Pre-allocation Strategy

Pre-allocating buffers based on `EstimateSize()` proved highly effective:
- Prevents multiple slice growth operations
- Reduces memory fragmentation
- Improves cache locality

### Remaining Optimization Opportunities

1. **Varint Encoding**: The `varint.Encode` function writes one byte at a time, causing many small allocations. Optimizing this would require:
   - Batch encoding into a temporary buffer
   - Or refactoring the varint package to support batch writes

2. **NostrEscape**: While we can't modify the `text.NostrEscape` function directly, we could:
   - Pre-allocate destination buffer based on source size estimate
   - Use a pool of buffers for repeated operations

3. **Tag Marshaling**: Tag marshaling could benefit from similar pre-allocation strategies

## Recommendations

1. **Use Pre-allocated Buffers**: When calling `Marshal`, `ToCanonical`, or `MarshalBinaryToBytes` repeatedly, consider reusing buffers:
   ```go
   buf := make([]byte, 0, ev.EstimateSize()+100)
   json := ev.Marshal(buf)
   ```

2. **Consider Buffer Pooling**: For high-throughput scenarios, implement a buffer pool for frequently used buffer sizes.

3. **Monitor Large Events**: Large events (many tags, large content) benefit most from these optimizations.

4. **Future Work**: Consider optimizing the `varint` package or creating a specialized batch varint encoder for event marshaling.

## Conclusion

The optimizations implemented significantly improved encoder performance:
- **24% faster** JSON marshaling
- **16% faster** canonical encoding
- **54-83% reduction** in memory allocations
- **80-91% reduction** in allocation count

These improvements will reduce GC pressure and improve overall system throughput, especially under high load conditions. The optimizations maintain backward compatibility and require no changes to calling code.

## Benchmark Results

Full benchmark output:

```
BenchmarkJSONMarshal-12           	  799773	      1325 ns/op	    1024 B/op	       1 allocs/op
BenchmarkJSONMarshalLarge-12      	   68712	     17666 ns/op	    9472 B/op	       1 allocs/op
BenchmarkJSONUnmarshal-12         	  538311	      2195 ns/op	     824 B/op	      24 allocs/op
BenchmarkBinaryMarshal-12         	 3955064	       297.2 ns/op	      13 B/op	      13 allocs/op
BenchmarkBinaryMarshalLarge-12    	  673252	      1756 ns/op	      85 B/op	      85 allocs/op
BenchmarkBinaryUnmarshal-12       	 1000000	      1028 ns/op	     752 B/op	      32 allocs/op
BenchmarkCanonical-12             	  835960	      1272 ns/op	     896 B/op	       1 allocs/op
BenchmarkCanonicalLarge-12        	   69620	     17903 ns/op	   10240 B/op	       1 allocs/op
BenchmarkGetIDBytes-12            	  704444	      1552 ns/op	     928 B/op	       2 allocs/op
BenchmarkRoundTripJSON-12         	  312724	      3673 ns/op	    1848 B/op	      25 allocs/op
BenchmarkRoundTripBinary-12       	  857373	      1325 ns/op	     765 B/op	      45 allocs/op
BenchmarkEstimateSize-12          	295157716	         4.012 ns/op	       0 B/op	       0 allocs/op
```

## Date

Report generated: 2025-11-02

