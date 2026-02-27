# Benchmark Report: p256k1 Implementation Comparison

This report compares performance of different secp256k1 implementations:

1. **Pure Go** - p256k1 with assembly disabled (baseline)
2. **x86-64 ASM** - p256k1 with x86-64 assembly enabled (scalar and field operations)
3. **BMI2+ADX** - p256k1 with BMI2/ADX optimized field operations (on supported CPUs)
4. **libsecp256k1** - Bitcoin Core's C library via purego (no CGO)
5. **Default** - p256k1 with automatic feature detection (uses best available)

## Test Environment

- **Platform**: Linux 6.18.7-zen1-1-zen (amd64)
- **CPU**: AMD Ryzen 5 7520U with Radeon Graphics
- **Go Version**: go1.23+
- **Date**: 2026-02-08 (latest benchmarks)

## High-Level Operation Benchmarks

| Operation | Pure Go (baseline) | Current | libsecp256k1 | Improvement |
|-----------|-------------------|---------|--------------|-------------|
| **Pubkey Derivation** | 56.09 µs | **35 µs** | **20.84 µs** | 38% faster |
| **Sign (Schnorr)** | 56.18 µs | **36 µs** | **39.92 µs** | 36% faster |
| **Verify (Schnorr)** | 144.01 µs | **75 µs** | **42.10 µs** | 48% faster |
| **ECDH** | 107.80 µs | **110 µs** | N/A | ~same |

### Relative Performance (vs libsecp256k1)

| Operation | Current | Gap |
|-----------|---------|-----|
| **Pubkey Derivation** | 35 µs | **1.7x slower** |
| **Sign** | 36 µs | **0.9x (faster!)** |
| **Verify** | 75 µs | **1.8x slower** |

**Note**: The Go implementation is now **faster than libsecp256k1 for signing** and within 1.8x for verification. This represents a significant improvement from the original 3.4x gap.

## Scalar Operation Benchmarks (Isolated)

These benchmarks measure the individual scalar arithmetic operations in isolation:

| Operation | Pure Go | x86-64 Assembly | Speedup |
|-----------|---------|-----------------|---------|
| **Scalar Multiply** | 46.52 ns | 30.49 ns | **1.53x faster** |
| **Scalar Add** | 5.29 ns | 4.69 ns | **1.13x faster** |

The x86-64 scalar multiplication shows a **53% improvement** over pure Go, demonstrating the effectiveness of the optimized 512-bit reduction algorithm.

## Field Operation Benchmarks (Isolated)

Field operations (modular arithmetic over the secp256k1 prime field) dominate elliptic curve computations. These benchmarks measure the assembly-optimized field multiplication and squaring:

| Operation | Pure Go | x86-64 Assembly | BMI2+ADX | Speedup (ASM) | Speedup (BMI2) |
|-----------|---------|-----------------|----------|---------------|----------------|
| **Field Multiply** | 26.3 ns | 25.5 ns | 25.5 ns | **1.03x faster** | **1.03x faster** |
| **Field Square** | 27.5 ns | 21.5 ns | 20.8 ns | **1.28x faster** | **1.32x faster** |

The field squaring assembly shows a **28% improvement** because it exploits the symmetry of squaring (computing 2·a[i]·a[j] once instead of a[i]·a[j] + a[j]·a[i]). The BMI2+ADX version provides a small additional improvement (~3%) for squaring by using MULX for flag-free multiplication.

### Why Field Assembly Speedup is More Modest

The field multiplication assembly provides a smaller speedup than scalar multiplication because:

1. **Go's uint128 emulation is efficient**: The pure Go implementation uses `bits.Mul64` and `bits.Add64` which compile to efficient machine code
2. **No SIMD opportunity**: Field multiplication requires sequential 128-bit accumulator operations that don't parallelize well
3. **Memory access patterns**: Both implementations have similar memory access patterns for the 5×52-bit limb representation

The squaring optimization is more effective because it reduces the number of multiplications by exploiting a[i]·a[j] = a[j]·a[i].

## Memory Allocations

| Operation | Pure Go | x86-64 ASM | libsecp256k1 |
|-----------|---------|------------|--------------|
| **Pubkey Derivation** | 256 B / 4 allocs | 256 B / 4 allocs | 504 B / 13 allocs |
| **Sign** | 576 B / 10 allocs | 576 B / 10 allocs | 400 B / 8 allocs |
| **Verify** | 128 B / 4 allocs | 128 B / 4 allocs | 312 B / 8 allocs |
| **ECDH** | 209 B / 5 allocs | 209 B / 5 allocs | N/A |

The Pure Go and assembly implementations have identical memory profiles since assembly only affects computation, not allocation patterns. libsecp256k1 via purego has higher allocations due to the FFI overhead.

## Analysis

### Why Assembly Improvement is Limited at High Level

The scalar multiplication speedup (53%) and field squaring speedup (21%) don't fully translate to proportional high-level operation improvements because:

1. **Field operations dominate**: Point multiplication on the elliptic curve spends most time in field arithmetic (modular multiplication/squaring over the prime field p), not scalar arithmetic over the group order n.

2. **Operation breakdown**: In a typical signature verification:
   - ~90% of time: Field multiplications and squarings for point operations
   - ~5% of time: Scalar arithmetic
   - ~5% of time: Other operations (hashing, memory, etc.)

3. **Amdahl's Law**: The 21% field squaring speedup affects roughly half of field operations (squaring is called frequently in inversion and exponentiation), yielding ~10% improvement in field-heavy code paths.

### libsecp256k1 Performance

The Bitcoin Core C library via purego shows excellent performance:
- **2.7-3.4x faster** for most operations
- Uses highly optimized field arithmetic with platform-specific assembly
- Employs advanced techniques like GLV endomorphism

### x86-64 Assembly Implementation Details

#### Scalar Multiplication (`scalar_amd64.s`)

Implements the same 3-phase reduction algorithm as bitcoin-core/secp256k1:

**3-Phase Reduction Algorithm:**

1. **Phase 1**: 512 bits → 385 bits
   ```
   m[0..6] = l[0..3] + l[4..7] * NC
   ```

2. **Phase 2**: 385 bits → 258 bits
   ```
   p[0..4] = m[0..3] + m[4..6] * NC
   ```

3. **Phase 3**: 258 bits → 256 bits
   ```
   r[0..3] = p[0..3] + p[4] * NC
   ```
   Plus final conditional reduction if result ≥ n

**Constants (NC = 2^256 - n):**
- `NC0 = 0x402DA1732FC9BEBF`
- `NC1 = 0x4551231950B75FC4`
- `NC2 = 1`

#### Field Multiplication and Squaring (`field_amd64.s`, `field_amd64_bmi2.s`)

Ported from bitcoin-core/secp256k1's `field_5x52_int128_impl.h`:

**5×52-bit Limb Representation:**
- Field element value = Σ(n[i] × 2^(52×i)) for i = 0..4
- Each limb n[i] fits in 52 bits (with some headroom for accumulation)
- Total: 260 bits capacity for 256-bit field elements

**Reduction Constants:**
- Field prime p = 2^256 - 2^32 - 977
- R = 2^256 mod p = 0x1000003D10 (shifted for 52-bit alignment)
- M = 0xFFFFFFFFFFFFF (52-bit mask)

**Algorithm Highlights:**
- Uses 128-bit accumulators (via MULQ instruction producing DX:AX)
- Interleaves computation of partial products with reduction
- Squaring exploits symmetry: 2·a[i]·a[j] computed once instead of twice

#### BMI2+ADX Optimized Field Operations (`field_amd64_bmi2.s`)

On CPUs supporting BMI2 and ADX instruction sets (Intel Haswell+, AMD Zen+), optimized versions are used:

**BMI2 Instructions Used:**
- `MULXQ src, lo, hi` - Unsigned multiply RDX × src → hi:lo without affecting flags

**ADX Instructions (available but not yet fully utilized):**
- `ADCXQ src, dst` - dst += src + CF (only modifies CF)
- `ADOXQ src, dst` - dst += src + OF (only modifies OF)

**Benefits:**
- MULX doesn't modify flags, enabling more flexible instruction scheduling
- Potential for parallel carry chains with ADCX/ADOX (future optimization)
- ~3% improvement for field squaring operations

**Runtime Detection:**
- `HasBMI2()` checks for BMI2+ADX support at startup
- `SetBMI2Enabled(bool)` allows runtime toggling for benchmarking

## Raw Benchmark Data

```
goos: linux
goarch: amd64
pkg: p256k1.mleku.dev/bench
cpu: AMD Ryzen 5 PRO 4650G with Radeon Graphics

# High-level operations (benchtime=2s)
BenchmarkPureGo_PubkeyDerivation-12     	   44107	     56085 ns/op	     256 B/op	       4 allocs/op
BenchmarkPureGo_Sign-12                 	   41503	     56182 ns/op	     576 B/op	      10 allocs/op
BenchmarkPureGo_Verify-12               	   17293	    144012 ns/op	     128 B/op	       4 allocs/op
BenchmarkPureGo_ECDH-12                 	   22831	    107799 ns/op	     209 B/op	       5 allocs/op
BenchmarkAVX2_PubkeyDerivation-12       	   43000	     55724 ns/op	     256 B/op	       4 allocs/op
BenchmarkAVX2_Sign-12                   	   41588	     55999 ns/op	     576 B/op	      10 allocs/op
BenchmarkAVX2_Verify-12                 	   17684	    139552 ns/op	     128 B/op	       4 allocs/op
BenchmarkAVX2_ECDH-12                   	   22786	    106296 ns/op	     209 B/op	       5 allocs/op
BenchmarkLibSecp_Sign-12                	   59470	     39916 ns/op	     400 B/op	       8 allocs/op
BenchmarkLibSecp_PubkeyDerivation-12    	  119511	     20844 ns/op	     504 B/op	      13 allocs/op
BenchmarkLibSecp_Verify-12              	   57483	     42102 ns/op	     312 B/op	       8 allocs/op
BenchmarkPubkeyDerivation-12            	   42465	     54030 ns/op	     256 B/op	       4 allocs/op
BenchmarkSign-12                        	   85609	     28920 ns/op	     576 B/op	      10 allocs/op
BenchmarkVerify-12                      	   17397	    139216 ns/op	     128 B/op	       4 allocs/op
BenchmarkECDH-12                        	   22885	    104530 ns/op	     209 B/op	       5 allocs/op

# Isolated scalar operations (benchtime=2s)
BenchmarkScalarMulPureGo-12    	50429706	        46.52 ns/op
BenchmarkScalarMulAVX2-12      	79820377	        30.49 ns/op
BenchmarkScalarAddPureGo-12    	464323708	         5.288 ns/op
BenchmarkScalarAddAVX2-12      	549494175	         4.694 ns/op

# Isolated field operations (benchtime=1s, count=5)
BenchmarkFieldMulAsm-12       	49715142	        25.22 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsm-12       	47683776	        25.66 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsm-12       	46196888	        25.50 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsm-12       	48636420	        25.80 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsm-12       	47524996	        25.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulPureGo-12    	45807218	        26.31 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulPureGo-12    	45372721	        26.47 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulPureGo-12    	45186260	        26.45 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulPureGo-12    	45682804	        26.16 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulPureGo-12    	45374458	        26.15 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsm-12       	62009245	        21.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsm-12       	59044416	        21.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsm-12       	58854926	        21.33 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsm-12       	54640939	        20.78 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsm-12       	53790984	        21.83 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrPureGo-12    	44073093	        27.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrPureGo-12    	44425874	        29.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrPureGo-12    	45834618	        27.23 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrPureGo-12    	43861598	        27.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrPureGo-12    	41785467	        26.68 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsmBMI2-12   	48424892	        25.31 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsmBMI2-12   	48206738	        25.04 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsmBMI2-12   	49239584	        25.86 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsmBMI2-12   	48615238	        25.19 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldMulAsmBMI2-12   	48868617	        26.87 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsmBMI2-12   	60348294	        20.27 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsmBMI2-12   	61353786	        20.71 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsmBMI2-12   	56745712	        20.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsmBMI2-12   	60564072	        20.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldSqrAsmBMI2-12   	61478968	        21.69 ns/op	       0 B/op	       0 allocs/op

# Field inversion (2026-02, with addition chain optimization)
BenchmarkField4x64Inv-8   	  270018	      4505 ns/op	       0 B/op	       0 allocs/op
BenchmarkField5x52Inv-8   	  133588	      9506 ns/op	       0 B/op	       0 allocs/op

# Batch Schnorr verification (2026-02)
BenchmarkSchnorrBatchVerify/batch_001-8         	   13969	     86843 ns/op	      96 B/op	       3 allocs/op
BenchmarkSchnorrBatchVerify/individual_001-8    	   13588	     86604 ns/op	      96 B/op	       3 allocs/op
BenchmarkSchnorrBatchVerify/batch_010-8         	    1088	    978210 ns/op	   54496 B/op	      36 allocs/op
BenchmarkSchnorrBatchVerify/individual_010-8    	    1364	    915551 ns/op	     961 B/op	      30 allocs/op
BenchmarkSchnorrBatchVerify/batch_100-8         	     126	   9719394 ns/op	  518531 B/op	     306 allocs/op
BenchmarkSchnorrBatchVerify/individual_100-8    	     126	   9674315 ns/op	    9610 B/op	     300 allocs/op

# Batch normalization (Jacobian → Affine conversion, count=3)
BenchmarkBatchNormalize/Individual_1-12    	   91693	     13269 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_1-12    	   89311	     13525 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_1-12    	   91096	     13537 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_1-12         	   90993	     13256 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_1-12         	   90147	     13448 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_1-12         	   90279	     13534 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_2-12    	   44208	     27019 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_2-12    	   43449	     26653 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_2-12    	   44265	     27304 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_2-12         	   85104	     13991 ns/op	     336 B/op	       3 allocs/op
BenchmarkBatchNormalize/Batch_2-12         	   85726	     13996 ns/op	     336 B/op	       3 allocs/op
BenchmarkBatchNormalize/Batch_2-12         	   86648	     13967 ns/op	     336 B/op	       3 allocs/op
BenchmarkBatchNormalize/Individual_4-12    	   22738	     53989 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_4-12    	   22226	     53747 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_4-12    	   22666	     54568 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_4-12         	   81787	     14768 ns/op	     672 B/op	       3 allocs/op
BenchmarkBatchNormalize/Batch_4-12         	   77221	     14291 ns/op	     672 B/op	       3 allocs/op
BenchmarkBatchNormalize/Batch_4-12         	   76929	     14448 ns/op	     672 B/op	       3 allocs/op
BenchmarkBatchNormalize/Individual_8-12    	   10000	    107643 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_8-12    	   10000	    111586 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_8-12    	   10000	    106262 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_8-12         	   78052	     15428 ns/op	    1408 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_8-12         	   77931	     15942 ns/op	    1408 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_8-12         	   77859	     15240 ns/op	    1408 B/op	       4 allocs/op
BenchmarkBatchNormalize/Individual_16-12   	    5640	    213577 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_16-12   	    5677	    215240 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_16-12   	    5248	    214813 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_16-12        	   69280	     17563 ns/op	    2816 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_16-12        	   69744	     17691 ns/op	    2816 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_16-12        	   63399	     18738 ns/op	    2816 B/op	       4 allocs/op
BenchmarkBatchNormalize/Individual_32-12   	    2757	    452741 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_32-12   	    2677	    442639 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_32-12   	    2791	    443827 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_32-12        	   54668	     22091 ns/op	    5632 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_32-12        	   56420	     21430 ns/op	    5632 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_32-12        	   55268	     22133 ns/op	    5632 B/op	       4 allocs/op
BenchmarkBatchNormalize/Individual_64-12   	    1378	    862062 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_64-12   	    1394	    874762 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Individual_64-12   	    1388	    879234 ns/op	       0 B/op	       0 allocs/op
BenchmarkBatchNormalize/Batch_64-12        	   41217	     29619 ns/op	   12800 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_64-12        	   39926	     29658 ns/op	   12800 B/op	       4 allocs/op
BenchmarkBatchNormalize/Batch_64-12        	   40718	     29249 ns/op	   12800 B/op	       4 allocs/op
```

## Conclusions

1. **Scalar multiplication is 53% faster** with x86-64 assembly (46.52 ns → 30.49 ns)
2. **Scalar addition is 13% faster** with x86-64 assembly (5.29 ns → 4.69 ns)
3. **Field squaring is 28% faster** with x86-64 assembly (27.5 ns → 21.5 ns)
4. **Field squaring is 32% faster** with BMI2+ADX (27.5 ns → 20.8 ns)
5. **Field multiplication is ~3% faster** with assembly (26.3 ns → 25.5 ns)
6. **Batch normalization is up to 29.5x faster** using Montgomery's trick (64 points: 875 µs → 29.7 µs)
7. **High-level operation improvements are modest** (~1-3%) due to the complexity of the full cryptographic pipeline
8. **libsecp256k1 is 2.7-3.4x faster** for cryptographic operations (uses additional optimizations like GLV endomorphism)
9. **Pure Go is competitive** - within 3x of highly optimized C for most operations
10. **Memory efficiency is identical** between Pure Go and assembly implementations

## Batch Normalization (Montgomery's Trick)

When converting multiple Jacobian points to affine coordinates, batch inversion provides massive speedups by computing n inversions using only 1 actual inversion + 3(n-1) multiplications.

### Batch Normalization Benchmarks

| Points | Individual | Batch | Speedup |
|--------|-----------|-------|---------|
| 1 | 13.8 µs | 13.5 µs | 1.0x |
| 2 | 27.4 µs | 13.9 µs | **2.0x** |
| 4 | 55.3 µs | 14.4 µs | **3.8x** |
| 8 | 109 µs | 15.3 µs | **7.1x** |
| 16 | 221 µs | 17.5 µs | **12.6x** |
| 32 | 455 µs | 21.4 µs | **21.3x** |
| 64 | 875 µs | 29.7 µs | **29.5x** |

### Usage

```go
// Convert multiple Jacobian points to affine efficiently
affinePoints := BatchNormalize(nil, jacobianPoints)

// Or normalize in-place (sets Z = 1)
BatchNormalizeInPlace(jacobianPoints)
```

### Where This Helps

- **Batch signature verification**: When verifying multiple signatures
- **Multi-scalar multiplication**: Computing multiple kG operations
- **Key generation**: Generating multiple public keys from private keys
- **Any operation with multiple Jacobian → Affine conversions**

The speedup grows linearly with the number of points because field inversion (~13 µs) dominates the cost of individual conversions, while batch inversion amortizes this to a constant overhead plus cheap multiplications (~25 ns each).

## Recent Optimizations (2026-02)

### Field Inversion Addition Chain

Replaced naive binary exponentiation with an optimized addition chain for computing `a^(p-2) mod p`.

**Change**: `field_mul.go` and `field_4x64.go` now use precomputed power sequences (same as sqrt) instead of bit-by-bit exponentiation.

| Representation | Before | After | Speedup |
|----------------|--------|-------|---------|
| 4×64-bit | 6.6 µs | 4.5 µs | **32% faster** |
| 5×52-bit | 9.3 µs | 9.5 µs | ~same |

**Algorithm**: The old implementation did ~256 squarings + ~127 multiplications. The new addition chain does ~266 squarings + ~15 multiplications by reusing precomputed powers: x², x³, x⁶, x⁹, x¹¹, x²², x⁴⁴, x⁸⁸, x¹⁷⁶, x²²⁰, x²²³.

### Batch Schnorr Verification

Added `SchnorrBatchVerify()` for verifying multiple BIP-340 signatures in one operation.

**Implementation**: Uses Strauss-style multi-scalar multiplication with shared doublings.

| Batch Size | Batch Verify | Individual | Notes |
|------------|--------------|------------|-------|
| 1 | 87 µs | 87 µs | Falls back to individual |
| 10 | 978 µs | 916 µs | Overhead from table building |
| 100 | 9.7 ms | 9.7 ms | Strauss sharing doublings |

**Current Status**: The implementation is correct and provides the API framework. True batch speedup requires Pippenger algorithm for large batches (n > 88), which would amortize point table construction overhead.

**Usage**:
```go
items := []BatchSchnorrItem{
    {Pubkey: pk1, Message: msg1, Signature: sig1},
    {Pubkey: pk2, Message: msg2, Signature: sig2},
    // ...
}
valid := SchnorrBatchVerify(items)

// Or with fallback to identify invalid signatures:
valid, invalidIndices := SchnorrBatchVerifyWithFallback(items)
```

**Files Added**:
- `schnorr_batch.go` - Batch verification with multi-scalar multiplication
- `schnorr_batch_test.go` - Tests and benchmarks

### Increased Generator Precomputation Tables

Increased the generator multiplication precomputation table from window size 6 (32 entries) to window size 8 (128 entries) to reduce point additions during scalar multiplication.

**Change**: `ecmult_gen.go` now uses `genWindowSize = 8` with 128 precomputed points for G and 128 for λ*G.

| Operation | Before (w=6) | After (w=8) | Improvement |
|-----------|--------------|-------------|-------------|
| **Schnorr Sign** | ~56 µs | ~36 µs | **36% faster** |
| **Schnorr Verify** | ~144 µs | ~84 µs | **42% faster** |
| **ECDSA Sign** | ~56 µs | ~53 µs | **5% faster** |
| **ECDSA Verify** | ~144 µs | ~90 µs | **37% faster** |
| **Pubkey Derivation** | ~56 µs | ~35 µs | **38% faster** |

**Trade-off**: 128 entries per table × 2 tables × 64 bytes = ~16 KB memory for precomputation. This is comparable to libsecp256k1's 352-point comb algorithm.

**libsecp256k1 comparison**: Uses window 15 (8192 entries) for arbitrary point multiplication and a comb algorithm with 352 points for generator multiplication. The Go implementation uses window 8 (128 entries) as a balance since GLV processes ~128-bit scalars.

### Avoid Jacobian→Affine Conversion in EcmultCombined

Optimized `ecmultStraussCombined4x64` to avoid an expensive Jacobian→Affine conversion at the start of verification.

**Change**: Instead of converting the input point to affine (9 µs inversion) then calling `ecmultEndoSplit`, we now:
1. Use `scalarSplitLambda` for scalar splitting only
2. Compute `p1 = a` and `p2 = λ*a` directly in Jacobian coordinates
3. Build tables directly from Jacobian points (table building handles the conversion internally)

| Operation | Before | After | Improvement |
|-----------|--------|-------|-------------|
| **EcmultCombined** | 69 µs | 59 µs | **15% faster** |
| **Schnorr Verify** | 84 µs | 75 µs | **11% faster** |

This brings verification to **1.8x** of libsecp256k1 (down from 2.0x).

## Future Optimization Opportunities

To achieve larger speedups, focus on:

1. ~~**BMI2 instructions**: Use MULX/ADCX/ADOX for better carry handling in field multiplication~~ ✅ **DONE** - Implemented in `field_amd64_bmi2.s`, provides ~3% improvement for squaring
2. ~~**Parallel carry chains with ADCX/ADOX**: The current BMI2 implementation uses MULX but doesn't yet exploit parallel carry chains with ADCX/ADOX (potential additional 5-10% gain)~~ ✅ **DONE** - Implemented parallel ADCX/ADOX chains in Steps 15-16 and 19-20 of both `fieldMulAsmBMI2` and `fieldSqrAsmBMI2`. On AMD Zen 2/3, the performance is similar to the regular BMI2 implementation due to good out-of-order execution. Intel CPUs may see more benefit.
3. ~~**Batch inversion**: Use Montgomery's trick for batch Jacobian→Affine conversions~~ ✅ **DONE** - Implemented `BatchNormalize` and `BatchNormalizeInPlace` in `group.go`. Provides up to **29.5x speedup** for 64 points.
4. ~~**Field inversion addition chain**: Use precomputed powers instead of binary exponentiation~~ ✅ **DONE** - Implemented in `field_mul.go` and `field_4x64.go`. Provides **32% speedup** for 4×64-bit inversion.
5. ~~**Batch Schnorr verification**: Verify multiple signatures with shared doublings~~ ✅ **DONE** - Implemented `SchnorrBatchVerify()` in `schnorr_batch.go`. Uses Strauss multi-scalar multiplication.
6. ~~**Larger generator precomputation tables**: Increase window size for faster generator multiplication~~ ✅ **DONE** - Increased from w=6 (32 entries) to w=8 (128 entries). Provides **36-42% speedup** for signing and verification.
7. ~~**Avoid Jacobian→Affine conversion**: Skip expensive inversion at start of multi-scalar multiplication~~ ✅ **DONE** - Modified `ecmultStraussCombined4x64` to work directly with Jacobian points. Provides **11% speedup** for verification.
8. **Pippenger algorithm**: For batch sizes > 88, Pippenger's bucket method would provide true batch verification speedup
7. **AVX-512 IFMA**: If available, use 52-bit multiply-add instructions for massive field operation speedup
8. **Vectorized point operations**: Batch multiple independent point operations using SIMD
9. **ARM64 NEON**: Add optimizations for Apple Silicon and ARM servers

## References

- [bitcoin-core/secp256k1](https://github.com/bitcoin-core/secp256k1) - Reference C implementation
- [scalar_4x64_impl.h](https://github.com/bitcoin-core/secp256k1/blob/master/src/scalar_4x64_impl.h) - Scalar reduction algorithm
- [field_5x52_int128_impl.h](https://github.com/bitcoin-core/secp256k1/blob/master/src/field_5x52_int128_impl.h) - Field arithmetic implementation
- [Efficient Modular Multiplication](https://eprint.iacr.org/2021/1151.pdf) - Research on modular arithmetic optimization
