# Implementation Plan: wNAF + GLV Endomorphism Optimization

## Overview

This plan details implementing the GLV (Gallant-Lambert-Vanstone) endomorphism optimization combined with wNAF (windowed Non-Adjacent Form) for secp256k1 scalar multiplication, based on:
- The IACR paper "SIMD acceleration of EC operations" (eprint.iacr.org/2021/1151)
- The libsecp256k1 C implementation in `src/ecmult_impl.h` and `src/scalar_impl.h`

### Expected Performance Gain
- **50% reduction** in scalar multiplication time by processing two 128-bit scalars instead of one 256-bit scalar
- The GLV endomorphism exploits secp256k1's special structure: λ·(x,y) = (β·x, y)

---

## Phase 1: Constants and Basic Infrastructure

### Step 1.1: Add GLV Constants to scalar.go

Add the following constants that are already defined in the C implementation:

```go
// Lambda: cube root of unity mod n (group order)
// λ^3 ≡ 1 (mod n), and λ^2 + λ + 1 ≡ 0 (mod n)
var scalarLambda = Scalar{
    d: [4]uint64{
        0xDF02967C1B23BD72, // limb 0
        0x122E22EA20816678, // limb 1
        0xA5261C028812645A, // limb 2
        0x5363AD4CC05C30E0, // limb 3
    },
}

// Constants for scalar splitting (from libsecp256k1 scalar_impl.h lines 142-157)
var scalarMinusB1 = Scalar{
    d: [4]uint64{0x6F547FA90ABFE4C3, 0xE4437ED6010E8828, 0, 0},
}

var scalarMinusB2 = Scalar{
    d: [4]uint64{0xD765CDA83DB1562C, 0x8A280AC50774346D, 0xFFFFFFFFFFFFFFFE, 0xFFFFFFFFFFFFFFFF},
}

var scalarG1 = Scalar{
    d: [4]uint64{0xE893209A45DBB031, 0x3DAA8A1471E8CA7F, 0xE86C90E49284EB15, 0x3086D221A7D46BCD},
}

var scalarG2 = Scalar{
    d: [4]uint64{0x1571B4AE8AC47F71, 0x221208AC9DF506C6, 0x6F547FA90ABFE4C4, 0xE4437ED6010E8828},
}
```

**Files to modify:** `scalar.go`
**Tests:** Add unit tests comparing with known C test vectors

---

### Step 1.2: Add Beta Constant to field.go

Add the field element β (cube root of unity mod p):

```go
// Beta: cube root of unity mod p (field order)
// β^3 ≡ 1 (mod p), and β^2 + β + 1 ≡ 0 (mod p)
// This enables: λ·(x,y) = (β·x, y) on secp256k1
var fieldBeta = FieldElement{
    // In 5×52-bit representation
    n: [5]uint64{...}, // Derived from: 0x7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee
}
```

**Files to modify:** `field.go`
**Tests:** Verify β^3 ≡ 1 (mod p)

---

## Phase 2: Scalar Splitting

### Step 2.1: Implement mul_shift_var

This function computes `(a * b) >> shift` for scalar splitting:

```go
// mulShiftVar computes (a * b) >> shift, returning the result
// This is used in GLV scalar splitting where shift is always 384
func (r *Scalar) mulShiftVar(a, b *Scalar, shift uint) {
    // Compute full 512-bit product
    // Extract bits [shift, shift+256) as the result
}
```

**Reference:** libsecp256k1 `scalar_4x64_impl.h:secp256k1_scalar_mul_shift_var`
**Files to modify:** `scalar.go`
**Tests:** Test with known inputs and compare with C implementation

---

### Step 2.2: Implement splitLambda

The core GLV scalar splitting function:

```go
// splitLambda decomposes scalar k into r1, r2 such that:
//   r1 + λ·r2 ≡ k (mod n)
// where r1 and r2 are approximately 128 bits each
func splitLambda(r1, r2, k *Scalar) {
    // c1 = round(k * g1 / 2^384)
    // c2 = round(k * g2 / 2^384)
    var c1, c2 Scalar
    c1.mulShiftVar(k, &scalarG1, 384)
    c2.mulShiftVar(k, &scalarG2, 384)

    // r2 = c1*(-b1) + c2*(-b2)
    c1.mul(&c1, &scalarMinusB1)
    c2.mul(&c2, &scalarMinusB2)
    r2.add(&c1, &c2)

    // r1 = k - r2*λ
    r1.mul(r2, &scalarLambda)
    r1.negate(r1)
    r1.add(r1, k)
}
```

**Reference:** libsecp256k1 `scalar_impl.h:secp256k1_scalar_split_lambda` (lines 140-178)
**Files to modify:** `scalar.go`
**Tests:**
- Verify r1 + λ·r2 ≡ k (mod n)
- Verify |r1| < 2^128 and |r2| < 2^128

---

## Phase 3: Point Operations with Endomorphism

### Step 3.1: Implement mulLambda for Points

Apply the endomorphism to a point:

```go
// mulLambda applies the GLV endomorphism: λ·(x,y) = (β·x, y)
func (r *GroupElementAffine) mulLambda(a *GroupElementAffine) {
    r.x.mul(&a.x, &fieldBeta)
    r.y = a.y
    r.infinity = a.infinity
}
```

**Reference:** libsecp256k1 `group_impl.h:secp256k1_ge_mul_lambda` (lines 915-922)
**Files to modify:** `group.go`
**Tests:** Verify λ·G equals expected point

---

### Step 3.2: Implement isHigh for Scalars

Check if a scalar is in the upper half of the group order:

```go
// isHigh returns true if s > n/2
func (s *Scalar) isHigh() bool {
    // Compare with n/2
    // n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
    // n/2 = 7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0
}
```

**Files to modify:** `scalar.go`
**Tests:** Test boundary cases around n/2

---

## Phase 4: Strauss Algorithm with GLV

### Step 4.1: Implement Odd Multiples Table with Z-Ratios

The C implementation uses an efficient method to build odd multiples while tracking Z-coordinate ratios:

```go
// buildOddMultiplesTable builds a table of odd multiples [1*a, 3*a, 5*a, ...]
// and tracks Z-coordinate ratios for efficient normalization
func buildOddMultiplesTable(
    n int,
    preA []GroupElementAffine,
    zRatios []FieldElement,
    z *FieldElement,
    a *GroupElementJacobian,
) {
    // Uses isomorphic curve trick for efficient Jacobian+Affine addition
    // See ecmult_impl.h lines 73-115
}
```

**Reference:** libsecp256k1 `ecmult_impl.h:secp256k1_ecmult_odd_multiples_table`
**Files to modify:** `ecdh.go` or new file `ecmult.go`
**Tests:** Verify table correctness

---

### Step 4.2: Implement Table Lookup Functions

```go
// tableGetGE retrieves point from table, handling sign
func tableGetGE(r *GroupElementAffine, pre []GroupElementAffine, n, w int) {
    // n is the wNAF digit (can be negative)
    // Returns pre[(|n|-1)/2], negated if n < 0
}

// tableGetGELambda retrieves λ-transformed point from table
func tableGetGELambda(r *GroupElementAffine, pre []GroupElementAffine, betaX []FieldElement, n, w int) {
    // Same as tableGetGE but uses precomputed β*x values
}
```

**Reference:** libsecp256k1 `ecmult_impl.h` lines 125-143
**Files to modify:** `ecmult.go`

---

### Step 4.3: Implement Full Strauss-GLV Algorithm

This is the main multiplication function:

```go
// ecmultStraussWNAF computes r = na*a + ng*G using Strauss algorithm with GLV
func ecmultStraussWNAF(r *GroupElementJacobian, a *GroupElementJacobian, na *Scalar, ng *Scalar) {
    // 1. Split scalars using GLV endomorphism
    //    na = na1 + λ*na2 (where na1, na2 are ~128 bits)

    // 2. Build odd multiples table for a
    //    Also precompute β*x for λ-transformed lookups

    // 3. Convert both half-scalars to wNAF representation
    //    wNAF size is 129 bits (128 + 1 for potential overflow)

    // 4. For generator G: split scalar and use precomputed tables
    //    ng = ng1 + 2^128*ng2 (simple bit split, not GLV)

    // 5. Main loop (from MSB to LSB):
    //    - Double result
    //    - Add contributions from wNAF digits for na1, na2, ng1, ng2
}
```

**Reference:** libsecp256k1 `ecmult_impl.h:secp256k1_ecmult_strauss_wnaf` (lines 237-347)
**Files to modify:** `ecmult.go`
**Tests:** Compare results with existing implementation

---

## Phase 5: Generator Precomputation

### Step 5.1: Precompute Generator Tables

For maximum performance, precompute tables for G and 2^128*G:

```go
// preG contains precomputed odd multiples of G for window size WINDOW_G
// preG[i] = (2*i+1)*G for i = 0 to (1 << (WINDOW_G-2)) - 1
var preG [1 << (WINDOW_G - 2)]GroupElementStorage

// preG128 contains precomputed odd multiples of 2^128*G
var preG128 [1 << (WINDOW_G - 2)]GroupElementStorage
```

**Options:**
1. Generate at init() time (slower startup, no code bloat)
2. Generate with go:generate and embed (faster startup, larger binary)

**Files to modify:** New file `ecmult_gen_table.go` or `precomputed.go`

---

### Step 5.2: Optimize Generator Multiplication

```go
// ecmultGen computes r = ng*G using precomputed tables
func ecmultGen(r *GroupElementJacobian, ng *Scalar) {
    // Split ng = ng1 + 2^128*ng2
    // Use preG for ng1 lookups
    // Use preG128 for ng2 lookups
    // Combine using Strauss algorithm
}
```

---

## Phase 6: Integration and Testing

### Step 6.1: Update Public APIs

Update the main multiplication functions to use the new implementation:

```go
// Ecmult computes r = na*a + ng*G
func Ecmult(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar) {
    ecmultStraussWNAF(r, a, na, ng)
}

// EcmultGen computes r = ng*G (generator multiplication only)
func EcmultGen(r *GroupElementJacobian, ng *Scalar) {
    ecmultGen(r, ng)
}
```

---

### Step 6.2: Comprehensive Testing

1. **Correctness tests:**
   - Compare with existing slow implementation
   - Test edge cases (zero scalar, infinity point, scalar = n-1)
   - Test with random scalars

2. **Property tests:**
   - Verify r1 + λ·r2 ≡ k (mod n) for splitLambda
   - Verify λ·(x,y) = (β·x, y) for mulLambda
   - Verify β^3 ≡ 1 (mod p)
   - Verify λ^3 ≡ 1 (mod n)

3. **Cross-validation:**
   - Compare with btcec or other Go implementations
   - Test vectors from libsecp256k1

---

### Step 6.3: Benchmarking

Add comprehensive benchmarks:

```go
func BenchmarkEcmultStraussGLV(b *testing.B) {
    // Benchmark new GLV implementation
}

func BenchmarkEcmultOld(b *testing.B) {
    // Benchmark old implementation for comparison
}

func BenchmarkScalarSplitLambda(b *testing.B) {
    // Benchmark scalar splitting
}
```

---

## Implementation Order

The recommended order minimizes dependencies:

| Step | Description | Dependencies | Estimated Complexity |
|------|-------------|--------------|---------------------|
| 1.1 | Add GLV scalar constants | None | Low |
| 1.2 | Add Beta field constant | None | Low |
| 2.1 | Implement mulShiftVar | None | Medium |
| 2.2 | Implement splitLambda | 1.1, 2.1 | Medium |
| 3.1 | Implement mulLambda for points | 1.2 | Low |
| 3.2 | Implement isHigh | None | Low |
| 4.1 | Build odd multiples table | None | Medium |
| 4.2 | Table lookup functions | 4.1 | Low |
| 4.3 | Full Strauss-GLV algorithm | 2.2, 3.1, 3.2, 4.1, 4.2 | High |
| 5.1 | Generator precomputation | 4.1 | Medium |
| 5.2 | Optimized generator mult | 5.1 | Medium |
| 6.x | Testing and integration | All above | Medium |

---

## Key Differences from Current Implementation

The current Go implementation in `ecdh.go` has:
- Basic wNAF conversion (`scalar.go:wNAF`)
- Simple Strauss without GLV (`ecdh.go:ecmultStraussGLV` - misnamed, doesn't use GLV)
- Windowed multiplication without endomorphism

The new implementation adds:
- GLV scalar splitting (reduces 256-bit to two 128-bit multiplications)
- β-multiplication for point transformation
- Combined processing of original and λ-transformed points
- Precomputed generator tables for faster G multiplication

---

## References

1. **libsecp256k1 source:**
   - `src/scalar_impl.h` - GLV constants and splitLambda
   - `src/ecmult_impl.h` - Strauss algorithm with wNAF
   - `src/field.h` - Beta constant
   - `src/group_impl.h` - Point lambda multiplication

2. **Papers:**
   - "Faster Point Multiplication on Elliptic Curves with Efficient Endomorphisms" (GLV, 2001)
   - "Guide to Elliptic Curve Cryptography" (Hankerson, Menezes, Vanstone) - Algorithm 3.74

3. **IACR ePrint 2021/1151:**
   - SIMD acceleration techniques
   - Window size optimization analysis
