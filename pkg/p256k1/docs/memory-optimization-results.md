# Memory Optimization Results

This document summarizes the memory optimization work performed on the p256k1 pure Go secp256k1 implementation.

## Executive Summary

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| ECDSA Verify allocations | 8 allocs, 5,632 B | **0 allocs, 0 B** | 100% reduction |
| ECDSA Sign allocations | 39 allocs, 2,386 B | 11 allocs, 546 B | 72% fewer allocs, 77% less memory |
| Schnorr Verify allocations | 11 allocs, 5,730 B | 3 allocs, 98 B | 73% fewer allocs, 98% less memory |
| Schnorr Sign allocations | ~40 allocs | 6 allocs, 320 B | 85% fewer allocs |
| BatchNormalize memory | 80 MB cumulative | 0 B | 100% reduction |

## Detailed Benchmark Comparison

### ECDSA Operations

| Operation | Before | After | Change |
|-----------|--------|-------|--------|
| **ECDSA Verify** | | | |
| - Time | ~180 μs | 193 μs | - |
| - Allocations | 8 | **0** | -100% |
| - Bytes/op | 5,632 B | **0 B** | -100% |
| **ECDSA Sign** | | | |
| - Time | ~77 μs | 77 μs | - |
| - Allocations | 39 | 11 | -72% |
| - Bytes/op | 2,386 B | 546 B | -77% |
| **ECDSA PubkeyDerivation** | | | |
| - Time | ~58 μs | 58 μs | - |
| - Allocations | 0 | 0 | - |
| - Bytes/op | 0 B | 0 B | - |

### Schnorr Operations

| Operation | Before | After | Change |
|-----------|--------|-------|--------|
| **Schnorr Verify** | | | |
| - Time | ~185 μs | 181 μs | -2% |
| - Allocations | 11 | 3 | -73% |
| - Bytes/op | 5,730 B | 98 B | -98% |
| **Schnorr Sign** | | | |
| - Time | ~138 μs | 110 μs | -20% |
| - Allocations | ~40 | 6 | -85% |
| - Bytes/op | ~2,500 B | 320 B | -87% |
| **Schnorr PubkeyDerivation** | | | |
| - Time | ~61 μs | 58 μs | -5% |
| - Allocations | 2 | 2 | - |
| - Bytes/op | 128 B | 128 B | - |

## Comparison vs Competitors

### vs btcec (Pure Go)

| Operation | p256k1 | btcec | p256k1 Advantage |
|-----------|--------|-------|------------------|
| Schnorr Sign (time) | 110 μs | 235 μs | **2.1x faster** |
| Schnorr Sign (allocs) | 6 | 37 | **84% fewer** |
| Schnorr Verify (allocs) | 3 | 16 | **81% fewer** |
| ECDSA Verify (allocs) | 0 | 23 | **100% fewer** |
| ECDSA Sign (allocs) | 11 | 28 | **61% fewer** |

### vs libsecp256k1 (C library via purego)

| Operation | p256k1 (Pure Go) | libsecp256k1 (C) | Notes |
|-----------|------------------|------------------|-------|
| ECDSA Verify | 193 μs, 0 allocs | 47 μs, 12 allocs | Pure Go is 4x slower but zero-alloc |
| ECDSA Sign | 77 μs, 11 allocs | 35 μs, 13 allocs | Pure Go is 2.2x slower, similar allocs |
| Schnorr Verify | 181 μs, 3 allocs | 51 μs, 8 allocs | Pure Go is 3.5x slower, fewer allocs |
| Schnorr Sign | 110 μs, 6 allocs | 39 μs, 8 allocs | Pure Go is 2.8x slower, fewer allocs |

## Optimizations Applied

### 1. Fixed-Size Array for Batch Operations

**Problem:** `BatchNormalize` and `batchInverse` used slices, causing 80MB+ cumulative allocations.

**Solution:** Created `batchNormalize16` and `batchInverse16` with fixed `[16]FieldElement` arrays since `glvTableSize=16` is constant.

```go
// Before: slice allocation escapes to heap
func BatchNormalize(out []GroupElementAffine, points []GroupElementJacobian)

// After: fixed-size array stays on stack
func batchNormalize16(out *[16]GroupElementAffine, points *[16]GroupElementJacobian)
```

**Files:** `field.go`, `field_32bit.go`, `group.go`, `ecdh.go`

### 2. SHA256/HMAC Context Pooling

**Problem:** Each HMAC operation created 2 new SHA256 contexts. RFC6979 nonce generation created 5+ HMAC contexts.

**Solution:** Added `sync.Pool` for SHA256, HMAC, and RFC6979 contexts.

```go
var sha256Pool = sync.Pool{
    New: func() interface{} { return sha256.New() },
}

var hmacPool = sync.Pool{
    New: func() interface{} { return &HMACSHA256{} },
}

var rfc6979Pool = sync.Pool{
    New: func() interface{} { return &RFC6979HMACSHA256{} },
}
```

**File:** `hash.go`

### 3. Pre-allocated Single-Byte Slices

**Problem:** `[]byte{0x00}` and `[]byte{0x01}` in hot paths allocated new slices each call.

**Solution:** Pre-allocated package-level variables.

```go
var (
    byte0x00 = []byte{0x00}
    byte0x01 = []byte{0x01}
)
```

**File:** `hash.go`

### 4. Fixed-Size Arrays in ECDSA/Schnorr

**Problem:** Dynamic slice allocations in signing functions.

**Solution:** Changed to fixed-size arrays.

```go
// Before: escapes to heap
nonceKey := make([]byte, 64)

// After: stays on stack
var nonceKey [64]byte
```

**Files:** `ecdsa.go`, `schnorr.go`, `schnorr_wasm.go`

### 5. Precomputed Tag Hashes

**Problem:** BIP-340 tag hashes (`SHA256("BIP0340/challenge")`, etc.) computed every call.

**Solution:** Compute once at init, cache for reuse.

```go
var (
    bip340AuxTagHash       [32]byte
    bip340NonceTagHash     [32]byte
    bip340ChallengeTagHash [32]byte
)

func getTaggedHashPrefix(tag []byte) [32]byte {
    // Returns precomputed hash for known tags
}
```

**File:** `hash.go`

### 6. Zero-Allocation Hash Finalization

**Problem:** `hash.Sum(nil)` allocates a new slice for the result.

**Solution:** Use `hash.Sum(buf[:0])` with pre-sized buffer.

```go
// Before: allocates new []byte
copy(temp[:], h.inner.Sum(nil))

// After: appends to existing buffer
h.inner.Sum(temp[:0])
```

**File:** `hash.go`

## Memory Profile Comparison

### Before Optimization (Top Allocators)
```
BatchNormalize:     80 MB (53%)
batchInverse:       32 MB (21%)
SHA256/HMAC:        21 MB (14%)
Other:              18 MB (12%)
```

### After Optimization (Top Allocators)
```
sync.Pool overhead: ~500 B (amortized)
Remaining allocs:   Minimal, mostly pooled
```

## Impact on GC

| Metric | Before | After |
|--------|--------|-------|
| Allocations per verify | 8-11 | 0-3 |
| GC pressure | High | Low |
| Memory churn | ~6 KB/op | <100 B/op |

The reduced allocations significantly decrease garbage collection pressure, improving latency consistency in high-throughput scenarios.

## WASM Compatibility

All optimizations are compatible with WASM/js builds:
- Fixed-size arrays work identically
- `sync.Pool` is supported in WASM
- No CGO dependencies

## Benchmark Commands

```bash
# Run Pure Go benchmarks
go test -bench="BenchmarkPureGo" -benchmem ./bench/

# Compare with btcec
go test -bench="BenchmarkBtcec" -benchmem ./bench/

# Compare with libsecp256k1 (requires libsecp256k1.so)
go test -bench="BenchmarkLibSecp" -benchmem ./bench/

# Run memory profile
go test -bench="BenchmarkPureGo_ECDSA_Sign" -memprofile=mem.prof ./bench/
go tool pprof -alloc_objects mem.prof
```

## Conclusion

The memory optimization work achieved:
- **Zero-allocation ECDSA verification** - critical for high-throughput signature validation
- **85% reduction in Schnorr signing allocations** - important for Nostr and Taproot applications
- **Competitive with btcec** while maintaining cleaner architecture
- **WASM-compatible** pure Go implementation with GLV/Strauss/wNAF optimizations
