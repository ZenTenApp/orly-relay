# AVX2 secp256k1 Implementation Plan

## Overview

This implementation uses 128-bit limbs with AVX2 256-bit registers for secp256k1 cryptographic operations. The key insight is that AVX2's YMM registers can hold two 128-bit values, enabling efficient parallel processing.

## Data Layout

### Register Mapping

| Type | Size | AVX2 Representation | Registers |
|------|------|---------------------|-----------|
| Uint128 | 128-bit | 1×128-bit in XMM or half YMM | 0.5 YMM |
| Scalar | 256-bit | 2×128-bit limbs | 1 YMM |
| FieldElement | 256-bit | 2×128-bit limbs | 1 YMM |
| AffinePoint | 512-bit | 2×FieldElement (x, y) | 2 YMM |
| JacobianPoint | 768-bit | 3×FieldElement (x, y, z) | 3 YMM |

### Memory Layout

```
Uint128:
  [Lo:64][Hi:64] = 128 bits

Scalar/FieldElement (in YMM register):
  YMM = [D[0].Lo:64][D[0].Hi:64][D[1].Lo:64][D[1].Hi:64]
        ├─── 128-bit limb 0 ────┤├─── 128-bit limb 1 ────┤

AffinePoint (2 YMM registers):
  YMM0 = X coordinate (256 bits)
  YMM1 = Y coordinate (256 bits)

JacobianPoint (3 YMM registers):
  YMM0 = X coordinate (256 bits)
  YMM1 = Y coordinate (256 bits)
  YMM2 = Z coordinate (256 bits)
```

## Implementation Phases

### Phase 1: Core 128-bit Operations

File: `uint128_amd64.s`

1. **uint128Add** - Add two 128-bit values with carry out
   - Instructions: `ADDQ`, `ADCQ`
   - Input: XMM0 (a), XMM1 (b)
   - Output: XMM0 (result), carry flag

2. **uint128Sub** - Subtract with borrow
   - Instructions: `SUBQ`, `SBBQ`

3. **uint128Mul** - Multiply two 64-bit values to get 128-bit result
   - Instructions: `MULQ` (scalar) or `VPMULUDQ` (SIMD)

4. **uint128Mul128** - Full 128×128→256 multiplication
   - This is the critical operation for field/scalar multiplication
   - Uses Karatsuba or schoolbook with `VPMULUDQ`

### Phase 2: Scalar Operations (mod n)

File: `scalar_amd64.go` (stubs), `scalar_amd64.s` (assembly)

1. **ScalarAdd** - Add two scalars mod n
   ```
   Load a into YMM0
   Load b into YMM1
   VPADDQ YMM0, YMM0, YMM1    ; parallel add of 64-bit lanes
   Handle carries between 64-bit lanes
   Conditional subtract n if >= n
   ```

2. **ScalarSub** - Subtract scalars mod n
   - Similar to add but with `VPSUBQ` and conditional add of n

3. **ScalarMul** - Multiply scalars mod n
   - Compute 512-bit product using 128×128 multiplications
   - Reduce mod n using Barrett or Montgomery reduction
   - 512-bit intermediate fits in 2 YMM registers

4. **ScalarNegate** - Compute -a mod n
   - `n - a` using subtraction

5. **ScalarInverse** - Compute a^(-1) mod n
   - Use Fermat's little theorem: a^(n-2) mod n
   - Requires efficient square-and-multiply

6. **ScalarIsZero**, **ScalarIsHigh**, **ScalarEqual** - Comparisons

### Phase 3: Field Operations (mod p)

File: `field_amd64.go` (stubs), `field_amd64.s` (assembly)

1. **FieldAdd** - Add two field elements mod p
   ```
   Load a into YMM0
   Load b into YMM1
   VPADDQ YMM0, YMM0, YMM1
   Handle carries
   Conditional subtract p if >= p
   ```

2. **FieldSub** - Subtract field elements mod p

3. **FieldMul** - Multiply field elements mod p
   - Most critical operation for performance
   - 256×256→512 bit product, then reduce mod p
   - secp256k1 has special structure: p = 2^256 - 2^32 - 977
   - Reduction: if result >= 2^256, add (2^32 + 977) to lower bits

4. **FieldSqr** - Square a field element (optimized mul(a,a))
   - Can save ~25% multiplications vs general multiply

5. **FieldInv** - Compute a^(-1) mod p
   - Fermat: a^(p-2) mod p
   - Use addition chain for efficiency

6. **FieldSqrt** - Compute square root mod p
   - p ≡ 3 (mod 4), so sqrt(a) = a^((p+1)/4) mod p

7. **FieldNegate**, **FieldIsZero**, **FieldEqual** - Basic operations

### Phase 4: Point Operations

File: `point_amd64.go` (stubs), `point_amd64.s` (assembly)

1. **AffineToJacobian** - Convert (x, y) to (x, y, 1)

2. **JacobianToAffine** - Convert (X, Y, Z) to (X/Z², Y/Z³)
   - Requires field inversion

3. **JacobianDouble** - Point doubling
   - ~4 field multiplications, ~4 field squarings, ~6 field additions
   - All field ops can use AVX2 versions

4. **JacobianAdd** - Add two Jacobian points
   - ~12 field multiplications, ~4 field squarings

5. **JacobianAddAffine** - Add Jacobian + Affine (optimized)
   - ~8 field multiplications, ~3 field squarings
   - Common case in scalar multiplication

6. **ScalarMult** - Compute k*P for scalar k and point P
   - Use windowed NAF or GLV decomposition
   - Core loop: double + conditional add

7. **ScalarBaseMult** - Compute k*G using precomputed table
   - Precompute multiples of generator G
   - Faster than general scalar mult

### Phase 5: High-Level Operations

File: `ecdsa.go`, `schnorr.go`

1. **ECDSA Sign/Verify**
2. **Schnorr Sign/Verify** (BIP-340)
3. **ECDH** - Shared secret computation

## Assembly Conventions

### Register Usage

```
YMM0-YMM7:  Scratch registers (caller-saved)
YMM8-YMM15: Can be used but should be preserved

For our operations:
YMM0: Primary operand/result
YMM1: Secondary operand
YMM2-YMM5: Intermediate calculations
YMM6-YMM7: Constants (field prime, masks, etc.)
```

### Key AVX2 Instructions

```asm
; Data movement
VMOVDQU     YMM0, [mem]       ; Load 256 bits unaligned
VMOVDQA     YMM0, [mem]       ; Load 256 bits aligned
VBROADCASTI128 YMM0, [mem]    ; Broadcast 128-bit to both lanes

; Arithmetic
VPADDQ      YMM0, YMM1, YMM2  ; Add packed 64-bit integers
VPSUBQ      YMM0, YMM1, YMM2  ; Subtract packed 64-bit integers
VPMULUDQ    YMM0, YMM1, YMM2  ; Multiply low 32-bits of each 64-bit lane

; Logical
VPAND       YMM0, YMM1, YMM2  ; Bitwise AND
VPOR        YMM0, YMM1, YMM2  ; Bitwise OR
VPXOR       YMM0, YMM1, YMM2  ; Bitwise XOR

; Shifts
VPSLLQ      YMM0, YMM1, imm   ; Shift left logical 64-bit
VPSRLQ      YMM0, YMM1, imm   ; Shift right logical 64-bit

; Shuffles and permutes
VPERMQ      YMM0, YMM1, imm   ; Permute 64-bit elements
VPERM2I128  YMM0, YMM1, YMM2, imm ; Permute 128-bit lanes
VPALIGNR    YMM0, YMM1, YMM2, imm ; Byte align

; Comparisons
VPCMPEQQ    YMM0, YMM1, YMM2  ; Compare equal 64-bit
VPCMPGTQ    YMM0, YMM1, YMM2  ; Compare greater than 64-bit

; Blending
VPBLENDVB   YMM0, YMM1, YMM2, YMM3 ; Conditional blend
```

## Carry Propagation Strategy

The tricky part of 128-bit limb arithmetic is carry propagation between the 64-bit halves and between the two 128-bit limbs.

### Addition Carry Chain

```
Given: A = [A0.Lo, A0.Hi, A1.Lo, A1.Hi]  (256 bits as 4×64)
       B = [B0.Lo, B0.Hi, B1.Lo, B1.Hi]

Step 1: Add with VPADDQ (no carries)
  R = A + B (per-lane, ignoring overflow)

Step 2: Detect carries
  carry_0_to_1 = (R0.Lo < A0.Lo) ? 1 : 0  ; carry from Lo to Hi in limb 0
  carry_1_to_2 = (R0.Hi < A0.Hi) ? 1 : 0  ; carry from limb 0 to limb 1
  carry_2_to_3 = (R1.Lo < A1.Lo) ? 1 : 0  ; carry within limb 1
  carry_out    = (R1.Hi < A1.Hi) ? 1 : 0  ; overflow

Step 3: Propagate carries
  R0.Hi += carry_0_to_1
  R1.Lo += carry_1_to_2 + (R0.Hi < carry_0_to_1 ? 1 : 0)
  R1.Hi += carry_2_to_3 + ...
```

This is complex in SIMD. Alternative: use `ADCX`/`ADOX` instructions (ADX extension) for scalar carry chains, which may be faster for sequential operations.

### Multiplication Strategy

For 128×128→256 multiplication:

```
A = A.Hi * 2^64 + A.Lo
B = B.Hi * 2^64 + B.Lo

A * B = A.Hi*B.Hi * 2^128
      + (A.Hi*B.Lo + A.Lo*B.Hi) * 2^64
      + A.Lo*B.Lo

Using MULX (BMI2) for efficient 64×64→128:
  MULX r1, r0, A.Lo    ; r1:r0 = A.Lo * B.Lo
  MULX r3, r2, A.Hi    ; r3:r2 = A.Hi * B.Lo
  ... (4 multiplications total, then accumulate)
```

## Testing Strategy

1. **Unit tests for each operation** comparing against reference (main package)
2. **Edge cases**: zero, one, max values, values near modulus
3. **Random tests**: generate random inputs, compare results
4. **Benchmark comparisons**: AVX2 vs pure Go implementation

## File Structure

```
avx/
├── IMPLEMENTATION_PLAN.md  (this file)
├── types.go                (type definitions)
├── uint128.go              (pure Go fallback)
├── uint128_amd64.go        (Go stubs for assembly)
├── uint128_amd64.s         (AVX2 assembly)
├── scalar.go               (pure Go fallback)
├── scalar_amd64.go         (Go stubs)
├── scalar_amd64.s          (AVX2 assembly)
├── field.go                (pure Go fallback)
├── field_amd64.go          (Go stubs)
├── field_amd64.s           (AVX2 assembly)
├── point.go                (pure Go fallback)
├── point_amd64.go          (Go stubs)
├── point_amd64.s           (AVX2 assembly)
├── avx_test.go             (tests)
└── bench_test.go           (benchmarks)
```

## Performance Targets

Compared to the current pure Go implementation:
- Scalar multiplication: 2-3x faster
- Field multiplication: 2-4x faster
- Point operations: 2-3x faster (dominated by field ops)
- ECDSA sign/verify: 2-3x faster overall

## Dependencies

- Go 1.21+ (for assembly support)
- CPU with AVX2 support (Intel Haswell+, AMD Excavator+)
- Optional: BMI2 for MULX instruction (faster 64×64→128 multiply)
