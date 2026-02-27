# Benchmark Comparison Report

## Signer Implementation Comparison

This report compares three signer implementations for secp256k1 operations:

1. **P256K1Signer** - This repository's new port from Bitcoin Core secp256k1 (pure Go)
2. ~~BtcecSigner - Pure Go wrapper around btcec/v2~~ (removed)
3. **LibSecp256k1** - Native C library via purego (no CGO required)

**Generated:** 2025-11-29 (Updated after GLV endomorphism optimization)
**Platform:** linux/amd64
**CPU:** AMD Ryzen 5 PRO 4650G with Radeon Graphics
**Go Version:** go1.25.3

**Key Optimizations:**
- Implemented 8-bit byte-based precomputed tables matching btcec's approach
- Optimized windowed multiplication (6-bit windows)
- **GLV Endomorphism (Nov 2025):**
  - GLV scalar splitting reduces 256-bit to two 128-bit multiplications
  - Strauss algorithm with wNAF (windowed Non-Adjacent Form) representation
  - Precomputed tables for generator G and λ*G (32 entries each)
  - **EcmultGenGLV: 2.7x faster** than reference (122 → 45 µs)
  - **Scalar multiplication: 17% faster** with GLV + Strauss (121 → 101 µs)
- **Previous CPU optimizations:**
  - Precomputed TaggedHash prefixes for common BIP-340 tags
  - Eliminated unnecessary copies in field element operations
  - Pre-allocated group elements to reduce allocations

---

## Summary Results

| Operation | P256K1Signer (Pure Go) | LibSecp256k1 (C) | Winner |
|-----------|------------------------|------------------|--------|
| **Pubkey Derivation** | 56 µs | 22 µs | LibSecp (2.5x faster) |
| **Sign** | 58 µs | 41 µs | LibSecp (1.4x faster) |
| **Verify** | 182 µs | 47 µs | LibSecp (3.9x faster) |
| **ECDH** | 119 µs | N/A | P256K1 |

### Internal Scalar Multiplication Benchmarks

| Operation | Time | Description |
|-----------|------|-------------|
| **EcmultGenGLV** | 45 µs | GLV-optimized generator multiplication |
| **EcmultGenSimple** | 68 µs | Precomputed table (no GLV) |
| **EcmultGenConstRef** | 122 µs | Reference implementation |
| **EcmultStraussWNAFGLV** | 101 µs | GLV + Strauss for arbitrary point |
| **EcmultConst** | 122 µs | Constant-time binary method |

---

## GLV Endomorphism Optimization Details

The GLV (Gallant-Lambert-Vanstone) endomorphism exploits secp256k1's special structure where:
- λ·(x, y) = (β·x, y) for the endomorphism constant λ
- β³ ≡ 1 (mod p) and λ³ ≡ 1 (mod n)

### Implementation Components

1. **Scalar Splitting**: Decompose 256-bit scalar k into two ~128-bit scalars k1, k2 such that k = k1 + k2·λ
2. **wNAF Representation**: Convert scalars to windowed Non-Adjacent Form (window size 6)
3. **Precomputed Tables**: 32 entries each for G and λ·G (odd multiples)
4. **Strauss Algorithm**: Process both scalars simultaneously with interleaved doubling/adding

### Performance Gains

| Metric | Before GLV | After GLV | Improvement |
|--------|------------|-----------|-------------|
| Generator mult (EcmultGen) | 122 µs | 45 µs | **2.7x faster** |
| Arbitrary point mult | 122 µs | 101 µs | **17% faster** |
| Scalar split overhead | N/A | 0.2 µs | Negligible |

---

## Detailed Results

### Public Key Derivation

Deriving public key from private key (32 bytes → 32 bytes x-only pubkey).

| Implementation | Time per op | Notes |
|----------------|-------------|-------|
| **P256K1Signer** | 56 µs | Pure Go with GLV optimization |
| **LibSecp256k1** | 22 µs | Native C library via purego |

### Signing (Schnorr)

Creating BIP-340 Schnorr signatures (32-byte message → 64-byte signature).

| Implementation | Time per op | Notes |
|----------------|-------------|-------|
| **P256K1Signer** | 58 µs | Pure Go with GLV |
| **LibSecp256k1** | 41 µs | Native C library |

### Verification (Schnorr)

Verifying BIP-340 Schnorr signatures (32-byte message + 64-byte signature).

| Implementation | Time per op | Notes |
|----------------|-------------|-------|
| **P256K1Signer** | 182 µs | Pure Go with GLV |
| **LibSecp256k1** | 47 µs | Native C library (3.9x faster) |

### ECDH (Shared Secret Generation)

Generating shared secret using Elliptic Curve Diffie-Hellman.

| Implementation | Time per op | Notes |
|----------------|-------------|-------|
| **P256K1Signer** | 119 µs | Pure Go with GLV |

---

## Performance Analysis

### Pure Go vs Native C

The native libsecp256k1 library maintains significant advantages due to:
- Assembly-optimized field arithmetic (ADX/BMI2 instructions)
- Highly tuned memory layout and cache optimization
- Platform-specific optimizations

However, the pure Go implementation with GLV is now competitive for many use cases.

### GLV Optimization Impact

The GLV endomorphism provides the most benefit for generator multiplication (used in signing):
- **2.7x speedup** for k*G operations
- **17% speedup** for arbitrary point multiplication

### Recommendations

**Use LibSecp256k1 when:**
- Maximum performance is critical
- Running on platforms where purego works (Linux, macOS, Windows with .so/.dylib/.dll)
- Verification-heavy workloads (3.9x faster)

**Use P256K1Signer when:**
- Pure Go is required (WebAssembly, cross-compilation, no shared libraries)
- Portability is important
- Security auditing of Go code is preferred over C

---

## Conclusion

The GLV endomorphism optimization significantly improves secp256k1 performance in pure Go:

1. **Generator multiplication: 2.7x faster** (122 → 45 µs)
2. **Arbitrary point multiplication: 17% faster** (122 → 101 µs)
3. **Scalar splitting: negligible overhead** (0.2 µs)

While the native C library remains faster (especially for verification), the pure Go implementation is now much more competitive for signing operations where generator multiplication dominates.

---

## Running the Benchmarks

To reproduce these benchmarks:

```bash
# Run all benchmarks
go test ./... -bench=. -benchmem -benchtime=2s

# Run specific scalar multiplication benchmarks
go test -bench='BenchmarkEcmultGen|BenchmarkEcmultStraussWNAFGLV' -benchtime=2s

# Run comparison benchmarks
go test ./bench -bench=. -benchtime=2s
```

