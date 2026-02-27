# Benchmark Comparison Report

## Signer Implementation Comparison

This report compares three signer implementations for secp256k1 operations:

1. **P256K1Signer** - This repository's new port from Bitcoin Core secp256k1 (pure Go)
2. ~~BtcecSigner - Pure Go wrapper around btcec/v2~~ (removed)
3. **NextP256K Signer** - CGO version using next.orly.dev/pkg/crypto/p256k (CGO bindings to libsecp256k1)

**Generated:** 2025-11-02 (Updated after comprehensive CPU optimizations)  
**Platform:** linux/amd64  
**CPU:** AMD Ryzen 5 PRO 4650G with Radeon Graphics  
**Go Version:** go1.25.3

**Key Optimizations:** 
- Implemented 8-bit byte-based precomputed tables matching btcec's approach, resulting in 4x improvement in pubkey derivation and 4.3x improvement in signing.
- Optimized windowed multiplication for verification (6-bit windows, increased from 5-bit): 8% improvement (149,511 → 138,127 ns/op).
- Optimized ECDH with windowed multiplication (6-bit windows): 5% improvement (109,068 → 103,345 ns/op).
- **Major CPU optimizations (Nov 2025):**
  - Precomputed TaggedHash prefixes for common BIP-340 tags: 28% faster (310 → 230 ns/op)
  - Eliminated unnecessary copies in field element operations (mul/sqr): faster when magnitude ≤ 8
  - Optimized group element operations (toBytes/toStorage): in-place normalization to avoid copies
  - Optimized EcmultGen: pre-allocated group elements to reduce allocations
  - **Sign optimizations:** 54% faster (63,421 → 29,237 ns/op), 47% fewer allocations (17 → 9 allocs/op)
  - **Verify optimizations:** 8% faster (149,511 → 138,127 ns/op), 78% fewer allocations (9 → 2 allocs/op)
  - **Pubkey derivation:** 6% faster (58,383 → 55,091 ns/op), eliminated intermediate copies

---

## Summary Results

| Operation | P256K1Signer | BtcecSigner | NextP256K | Winner |
|-----------|-------------|-------------|-----------|--------|
| **Pubkey Derivation** | 55,091 ns/op | 64,177 ns/op | 271,394 ns/op | P256K1 (14% faster than Btcec) |
| **Sign** | 29,237 ns/op | 225,514 ns/op | 53,015 ns/op | P256K1 (1.8x faster than NextP256K) |
| **Verify** | 138,127 ns/op | 177,622 ns/op | 44,776 ns/op | NextP256K (3.1x faster) |
| **ECDH** | 103,345 ns/op | 129,392 ns/op | 125,835 ns/op | P256K1 (1.2x faster than NextP256K) |

---

## Detailed Results

### Public Key Derivation

Deriving public key from private key (32 bytes → 32 bytes x-only pubkey).

| Implementation | Time per op | Memory | Allocations | Speedup vs P256K1 |
|----------------|-------------|--------|-------------|-------------------|
| **P256K1Signer** | 55,091 ns/op | 256 B/op | 4 allocs/op | 1.0x (baseline) |
| **BtcecSigner** | 64,177 ns/op | 368 B/op | 7 allocs/op | 0.9x slower |
| **NextP256K** | 271,394 ns/op | 983,394 B/op | 9 allocs/op | 0.2x slower |

**Analysis:**
- **P256K1 is fastest** (14% faster than Btcec) after implementing 8-bit byte-based precomputed tables
- **6% improvement** from CPU optimizations (58,383 → 55,091 ns/op)
- Massive improvement: 4x faster than original implementation (232,922 → 55,091 ns/op)
- NextP256K is slowest, likely due to CGO overhead for small operations
- P256K1 has lowest memory allocation overhead (256 B vs 368 B)

### Signing (Schnorr)

Creating BIP-340 Schnorr signatures (32-byte message → 64-byte signature).

| Implementation | Time per op | Memory | Allocations | Speedup vs P256K1 |
|----------------|-------------|--------|-------------|-------------------|
| **P256K1Signer** | 29,237 ns/op | 576 B/op | 9 allocs/op | 1.0x (baseline) |
| **BtcecSigner** | 225,514 ns/op | 2,193 B/op | 38 allocs/op | 0.1x slower |
| **NextP256K** | 53,015 ns/op | 128 B/op | 3 allocs/op | 0.6x slower |

**Analysis:**
- **P256K1 is fastest** (1.8x faster than NextP256K) after comprehensive CPU optimizations
- **54% improvement** from optimizations (63,421 → 29,237 ns/op)
- **47% reduction in allocations** (17 → 9 allocs/op)
- P256K1 is 7.7x faster than Btcec
- Optimizations: precomputed TaggedHash prefixes, eliminated intermediate copies, optimized hash operations
- NextP256K has lowest memory usage (128 B vs 576 B) but P256K1 is significantly faster

### Verification (Schnorr)

Verifying BIP-340 Schnorr signatures (32-byte message + 64-byte signature).

| Implementation | Time per op | Memory | Allocations | Speedup vs P256K1 |
|----------------|-------------|--------|-------------|-------------------|
| **P256K1Signer** | 138,127 ns/op | 64 B/op | 2 allocs/op | 1.0x (baseline) |
| **BtcecSigner** | 177,622 ns/op | 1,120 B/op | 18 allocs/op | 0.8x slower |
| **NextP256K** | 44,776 ns/op | 96 B/op | 2 allocs/op | **3.1x faster** |

**Analysis:**
- NextP256K is dramatically fastest (3.1x faster), showcasing CGO advantage for verification
- **P256K1 is fastest pure Go implementation** (22% faster than Btcec) after comprehensive optimizations
- **8% improvement** from CPU optimizations (149,511 → 138,127 ns/op)
- **78% reduction in allocations** (9 → 2 allocs/op), **89% reduction in memory** (576 → 64 B/op)
- **Total improvement:** 26% faster than original (186,054 → 138,127 ns/op)
- Optimizations: 6-bit windowed multiplication (increased from 5-bit), precomputed TaggedHash, eliminated intermediate copies
- P256K1 now has minimal memory footprint (64 B vs 96 B for NextP256K)

### ECDH (Shared Secret Generation)

Generating shared secret using Elliptic Curve Diffie-Hellman.

| Implementation | Time per op | Memory | Allocations | Speedup vs P256K1 |
|----------------|-------------|--------|-------------|-------------------|
| **P256K1Signer** | 103,345 ns/op | 241 B/op | 6 allocs/op | 1.0x (baseline) |
| **BtcecSigner** | 129,392 ns/op | 832 B/op | 13 allocs/op | 0.8x slower |
| **NextP256K** | 125,835 ns/op | 160 B/op | 3 allocs/op | 0.8x slower |

**Analysis:**
- **P256K1 is fastest** (1.2x faster than NextP256K) after optimizing with windowed multiplication
- **5% improvement** from CPU optimizations (109,068 → 103,345 ns/op)
- **Total improvement:** 37% faster than original (163,356 → 103,345 ns/op)
- Optimizations: 6-bit windowed multiplication (increased from 5-bit), optimized field operations
- P256K1 has lowest memory usage (241 B vs 832 B for Btcec)

---

## Performance Analysis

### Overall Winner: Mixed (P256K1 wins 3/4 operations, NextP256K wins 1/4 operations)

After comprehensive CPU optimizations:
- **P256K1Signer** wins in 3 out of 4 operations:
  - **Pubkey Derivation:** Fastest (14% faster than Btcec) - **6% improvement**
  - **Signing:** Fastest (1.8x faster than NextP256K) - **54% improvement!**
  - **ECDH:** Fastest (1.2x faster than NextP256K) - **5% improvement**
- **NextP256K** wins in 1 operation:
  - **Verification:** Fastest (3.1x faster than P256K1, CGO advantage) - but P256K1 is 8% faster than before

### Best Pure Go: P256K1Signer

For pure Go implementations:
- **P256K1** wins for key derivation (14% faster than Btcec) - **6% improvement**
- **P256K1** wins for signing (7.7x faster than Btcec) - **54% improvement!**
- **P256K1** wins for verification (22% faster than Btcec) - **fastest pure Go!** (**8% improvement**)
- **P256K1** wins for ECDH (1.25x faster than Btcec) - **fastest pure Go!** (**5% improvement**)

### Memory Efficiency

| Implementation | Avg Memory per Operation | Notes |
|----------------|-------------------------|-------|
| **P256K1Signer** | ~270 B avg | Low memory footprint, significantly reduced after optimizations |
| **NextP256K** | ~300 KB avg | Very efficient, minimal allocations (except pubkey derivation overhead) |
| **BtcecSigner** | ~1.1 KB avg | Higher allocations, but acceptable |

**Note:** NextP256K shows high memory in pubkey derivation (983 KB) due to one-time CGO initialization overhead, but this is amortized across operations.

**Memory Improvements:**
- **Sign:** 1,152 → 576 B/op (50% reduction)
- **Verify:** 576 → 64 B/op (89% reduction!)
- **Pubkey Derivation:** Already optimized (256 B/op)

---

## Recommendations

### Use NextP256K (CGO) when:
- Maximum verification performance is critical (3.1x faster than P256K1)
- CGO is acceptable in your build environment
- Low memory footprint is important
- Verification speed is critical (3.1x faster)

### Use P256K1Signer when:
- Pure Go is required (no CGO)
- **Signing performance is critical** (1.8x faster than NextP256K, 7.7x faster than Btcec)
- **Pubkey derivation, verification, or ECDH performance is critical** (fastest pure Go for all operations!)
- Lower memory allocations are preferred (64 B for verify, 576 B for sign)
- You want to avoid external C dependencies
- You need the best overall pure Go performance
- **Now competitive with CGO for signing** (faster than NextP256K)

### Use BtcecSigner when:
- Pure Go is required
- You're already using btcec in your project
- Note: P256K1Signer is faster across all operations

---

## Conclusion

The benchmarks demonstrate that:

1. **After comprehensive CPU optimizations**, P256K1Signer achieves:
   - **Fastest pubkey derivation** among all implementations (55,091 ns/op) - **6% improvement**
   - **Fastest signing** among all implementations (29,237 ns/op) - **54% improvement!** (63,421 → 29,237 ns/op)
   - **Fastest ECDH** among all implementations (103,345 ns/op) - **5% improvement** (109,068 → 103,345 ns/op)
   - **Fastest pure Go verification** (138,127 ns/op) - **8% improvement** (149,511 → 138,127 ns/op)
   - **Now faster than NextP256K for signing** (1.8x faster!)

2. **CPU optimization results (Nov 2025):**
   - Precomputed TaggedHash prefixes: 28% faster (310 → 230 ns/op)
   - Increased window size from 5-bit to 6-bit: fewer iterations (~43 vs ~52 windows)
   - Eliminated unnecessary copies in field/group operations
   - Optimized memory allocations: 78% reduction in verify (9 → 2 allocs/op), 47% reduction in sign (17 → 9 allocs/op)
   - **Sign: 54% faster** (63,421 → 29,237 ns/op)
   - **Verify: 8% faster** (149,511 → 138,127 ns/op), **89% less memory** (576 → 64 B/op)
   - **Pubkey Derivation: 6% faster** (58,383 → 55,091 ns/op)
   - **ECDH: 5% faster** (109,068 → 103,345 ns/op)

3. **CGO implementations (NextP256K) still provide advantages** for verification (3.1x faster) but P256K1 is now faster for signing

4. **Pure Go implementations are highly competitive**, with P256K1Signer leading in 3 out of 4 operations (pubkey derivation, signing, ECDH)

5. **Memory efficiency** significantly improved, with P256K1Signer maintaining very low memory usage:
   - Verify: 64 B/op (89% reduction!)
   - Sign: 576 B/op (50% reduction)
   - Pubkey Derivation: 256 B/op
   - ECDH: 241 B/op

The choice between implementations depends on your specific requirements:
- **Maximum verification performance:** Use NextP256K (CGO) - 3.1x faster for verification
- **Maximum signing performance:** Use P256K1Signer (Pure Go) - 1.8x faster than NextP256K, 7.7x faster than Btcec!
- **Best pure Go performance:** Use P256K1Signer - fastest pure Go for all operations, now competitive with CGO for signing
- **Best overall performance:** Use P256K1Signer - wins 3 out of 4 operations, fastest overall for signing
- **Pure Go alternative:** Use BtcecSigner (but P256K1Signer is significantly faster across all operations)

---

## Running the Benchmarks

To reproduce these benchmarks:

```bash
# Run all benchmarks
CGO_ENABLED=1 go test -tags=cgo ./bench -bench=. -benchmem

# Run specific operation
CGO_ENABLED=1 go test -tags=cgo ./bench -bench=BenchmarkSign

# Run specific implementation
CGO_ENABLED=1 go test -tags=cgo ./bench -bench=Benchmark.*_P256K1
```

**Note:** All benchmarks require CGO to be enabled (`CGO_ENABLED=1`) and the `cgo` build tag.

