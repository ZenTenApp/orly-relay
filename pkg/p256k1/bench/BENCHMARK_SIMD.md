# SIMD/ASM Optimization Benchmark Comparison

This document compares four secp256k1 implementations:

1. **btcec/v2** - Pure Go (github.com/btcsuite/btcd/btcec/v2)
2. **P256K1 Pure Go** - This repository with AVX2/BMI2 disabled
3. **P256K1 ASM** - This repository with AVX2/BMI2 assembly optimizations enabled
4. **libsecp256k1** - Native C library via purego (dlopen, no CGO)

**Generated:** 2025-11-29
**Platform:** linux/amd64
**CPU:** AMD Ryzen 5 PRO 4650G with Radeon Graphics (AVX2/BMI2 supported)
**Go Version:** go1.25.3

---

## Summary Comparison

| Operation | btcec/v2 | P256K1 Pure Go | P256K1 ASM | libsecp256k1 (C) |
|-----------|----------|----------------|------------|------------------|
| **Pubkey Derivation** | ~50 µs | 56 µs | 56 µs* | 22 µs |
| **Sign** | ~60 µs | 58 µs | 58 µs* | 41 µs |
| **Verify** | ~100 µs | 182 µs | 182 µs* | 47 µs |
| **ECDH** | ~120 µs | 119 µs | 119 µs* | N/A |

*Note: AVX2/BMI2 assembly optimizations are currently implemented for field operations but require additional integration work to show speedups at the high-level API. The assembly code is available in `field_amd64_bmi2.s`.

---

## Detailed Results

### btcec/v2

The btcec library is the widely-used pure Go implementation from the btcd project:

| Operation | Time per op |
|-----------|-------------|
| Pubkey Derivation | ~50 µs |
| Schnorr Sign | ~60 µs |
| Schnorr Verify | ~100 µs |
| ECDH | ~120 µs |

### P256K1 Pure Go (AVX2 disabled)

This implementation with `SetAVX2Enabled(false)`:

| Operation | Time per op |
|-----------|-------------|
| Pubkey Derivation | 56 µs |
| Schnorr Sign | 58 µs |
| Schnorr Verify | 182 µs |
| ECDH | 119 µs |

### P256K1 with ASM/BMI2 (AVX2 enabled)

This implementation with `SetAVX2Enabled(true)`:

| Operation | Time per op | Notes |
|-----------|-------------|-------|
| Pubkey Derivation | 56 µs | Uses GLV optimization |
| Schnorr Sign | 58 µs | Uses GLV for k*G |
| Schnorr Verify | 182 µs | Signature verification |
| ECDH | 119 µs | Uses GLV for scalar mult |

**Field Operation Speedups (Low-level):**
The BMI2-based field multiplication is available in `field_amd64_bmi2.s` and provides faster 256-bit modular arithmetic using the MULX instruction.

### libsecp256k1 (Native C via purego)

The fastest option, using the Bitcoin Core C library:

| Operation | Time per op |
|-----------|-------------|
| Pubkey Derivation | 22 µs |
| Schnorr Sign | 41 µs |
| Schnorr Verify | 47 µs |
| ECDH | N/A |

---

## Key Optimizations in P256K1

### GLV Endomorphism (Primary Speedup)

The GLV (Gallant-Lambert-Vanstone) endomorphism exploits secp256k1's special curve structure:
- λ·(x, y) = (β·x, y) for endomorphism constant λ
- β³ ≡ 1 (mod p) and λ³ ≡ 1 (mod n)

This reduces 256-bit scalar multiplication to two 128-bit multiplications:

| Operation | Without GLV | With GLV | Speedup |
|-----------|-------------|----------|---------|
| Generator mult (k*G) | 122 µs | 45 µs | **2.7x** |
| Arbitrary point mult | 122 µs | 101 µs | **17%** |

### BMI2 Assembly (Field Operations)

The `field_amd64_bmi2.s` file contains optimized assembly using:
- **MULX** instruction for carry-free multiplication
- **ADCX/ADOX** for parallel add-with-carry chains
- Register allocation optimized for secp256k1's field prime

### Precomputed Tables

- **Generator table**: 32 precomputed odd multiples of G
- **λ*G table**: 32 precomputed odd multiples for GLV
- **8-bit byte table**: For constant-time lookup

---

## Performance Ranking

From fastest to slowest for typical cryptographic operations:

1. **libsecp256k1 (C)** - Best choice when native library available
   - 2-4x faster than pure Go implementations
   - Uses purego (no CGO required)

2. **btcec/v2** - Good pure Go option
   - Mature, well-tested codebase
   - Slightly faster verification than P256K1

3. **P256K1 (This Repo)** - GLV-optimized pure Go
   - Competitive signing performance
   - 2.7x faster generator multiplication with GLV
   - Ongoing BMI2 assembly integration

---

## Recommendations

**Use libsecp256k1 when:**
- Maximum performance is critical
- Running on platforms where purego works (Linux, macOS, Windows)
- Verification-heavy workloads (3.9x faster than pure Go)

**Use btcec/v2 when:**
- Need a battle-tested, widely-used library
- Verification performance matters more than signing

**Use P256K1 when:**
- Pure Go is required (WebAssembly, embedded, cross-compilation)
- Signing-heavy workloads (GLV optimization helps most here)
- Portability is important
- Prefer Go code auditing over C

---

## Running Benchmarks

```bash
# Run all SIMD comparison benchmarks
go test ./bench -bench='BenchmarkBtcec|BenchmarkP256K1PureGo|BenchmarkP256K1ASM|BenchmarkLibSecp256k1' -benchtime=1s -run=^$

# Run specific benchmark category
go test ./bench -bench=BenchmarkBtcec -benchtime=1s -run=^$
go test ./bench -bench=BenchmarkP256K1PureGo -benchtime=1s -run=^$
go test ./bench -bench=BenchmarkP256K1ASM -benchtime=1s -run=^$
go test ./bench -bench=BenchmarkLibSecp256k1 -benchtime=1s -run=^$

# Run internal scalar multiplication benchmarks
go test -bench='BenchmarkEcmultGen|BenchmarkEcmultStraussWNAFGLV' -benchtime=1s
```

---

## CPU Feature Detection

The P256K1 implementation automatically detects CPU features:

```go
import "p256k1.mleku.dev"

// Check if AVX2/BMI2 is available
if p256k1.HasAVX2CPU() {
    // Use optimized path
}

// Manually control AVX2 usage
p256k1.SetAVX2Enabled(false)  // Force pure Go
p256k1.SetAVX2Enabled(true)   // Enable AVX2/BMI2 (if available)
```

---

## Future Work

1. **Integrate BMI2 field multiplication** into high-level operations
2. **Batch verification** using Strauss or Pippenger algorithms
3. **ARM64 optimizations** using NEON instructions
4. **WebAssembly SIMD** for browser performance
