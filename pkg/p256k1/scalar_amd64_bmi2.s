//go:build amd64 && !purego

#include "textflag.h"

// Scalar multiplication for secp256k1 using BMI2 MULX instruction.
// This is faster than traditional MUL because MULX doesn't clobber flags,
// allowing better instruction scheduling with carry chains.
//
// Stack layout (64 bytes):
//   SP+0:  l4 (saved)
//   SP+8:  l5 (saved)
//   SP+16: l6 (saved)
//   SP+24: l7 (saved)
//   SP+32: m0
//   SP+40: m1
//   SP+48: m2
//   SP+56: m3
//
// func scalarMulBMI2(r, a, b *Scalar)
TEXT ·scalarMulBMI2(SB), NOSPLIT, $64-24
    MOVQ r+0(FP), DI      // result pointer
    MOVQ a+8(FP), SI      // a pointer
    MOVQ b+16(FP), CX     // b pointer

    // Load a[0..3]
    MOVQ 0(SI), R8        // a0
    MOVQ 8(SI), R9        // a1
    MOVQ 16(SI), R10      // a2
    MOVQ 24(SI), R11      // a3

    // We'll compute the 512-bit product column by column using MULX
    // MULX puts DX as the implicit multiplier, result goes to specified registers

    // Column 0: a0*b0
    MOVQ 0(CX), DX        // b0 into DX for MULX
    MULXQ R8, R12, R13    // a0*b0 -> R13:R12 (hi:lo)

    // Column 1: a0*b1 + a1*b0
    MOVQ 8(CX), DX        // b1
    MULXQ R8, AX, BX      // a0*b1 -> BX:AX
    ADDQ AX, R13
    ADCQ $0, BX

    MOVQ 0(CX), DX        // b0
    MULXQ R9, AX, R14     // a1*b0 -> R14:AX
    ADDQ AX, R13
    ADCQ BX, R14
    MOVQ $0, R15
    ADCQ $0, R15

    // Column 2: a0*b2 + a1*b1 + a2*b0
    MOVQ 16(CX), DX       // b2
    MULXQ R8, AX, BX      // a0*b2 -> BX:AX
    ADDQ AX, R14
    ADCQ BX, R15

    MOVQ 8(CX), DX        // b1
    MULXQ R9, AX, BX      // a1*b1 -> BX:AX
    ADDQ AX, R14
    ADCQ BX, R15
    MOVQ $0, BP
    ADCQ $0, BP

    MOVQ 0(CX), DX        // b0
    MULXQ R10, AX, BX     // a2*b0 -> BX:AX
    ADDQ AX, R14
    ADCQ BX, R15
    ADCQ $0, BP

    // Column 3: a0*b3 + a1*b2 + a2*b1 + a3*b0
    // Save R12-R14 (columns 0-2), use them for column 3+
    MOVQ R12, 0(DI)       // Save l0
    MOVQ R13, 8(DI)       // Save l1
    MOVQ R14, 16(DI)      // Save l2

    // Now R12, R13, R14 are free
    MOVQ R15, R12         // l3 accumulator low
    MOVQ BP, R13          // l3 accumulator high
    XORQ R14, R14         // l4 accumulator

    MOVQ 24(CX), DX       // b3
    MULXQ R8, AX, BX      // a0*b3 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ 16(CX), DX       // b2
    MULXQ R9, AX, BX      // a1*b2 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ 8(CX), DX        // b1
    MULXQ R10, AX, BX     // a2*b1 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ 0(CX), DX        // b0
    MULXQ R11, AX, BX     // a3*b0 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ R12, 24(DI)      // Save l3

    // Column 4: a1*b3 + a2*b2 + a3*b1
    MOVQ R13, R12         // l4 accumulator low
    MOVQ R14, R13         // l4 accumulator high
    XORQ R14, R14

    MOVQ 24(CX), DX       // b3
    MULXQ R9, AX, BX      // a1*b3 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ 16(CX), DX       // b2
    MULXQ R10, AX, BX     // a2*b2 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ 8(CX), DX        // b1
    MULXQ R11, AX, BX     // a3*b1 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    // l4 is in R12, carry in R13:R14

    // Column 5: a2*b3 + a3*b2
    MOVQ R13, R15         // l5 accumulator low
    MOVQ R14, BP          // l5 accumulator high
    XORQ R8, R8           // reuse R8 for l6

    MOVQ 24(CX), DX       // b3
    MULXQ R10, AX, BX     // a2*b3 -> BX:AX
    ADDQ AX, R15
    ADCQ BX, BP
    ADCQ $0, R8

    MOVQ 16(CX), DX       // b2
    MULXQ R11, AX, BX     // a3*b2 -> BX:AX
    ADDQ AX, R15
    ADCQ BX, BP
    ADCQ $0, R8

    // Column 6: a3*b3
    MOVQ BP, R9           // l6 accumulator low
    MOVQ R8, R10          // l6 accumulator high (will be l7)

    MOVQ 24(CX), DX       // b3
    MULXQ R11, AX, BX     // a3*b3 -> BX:AX
    ADDQ AX, R9
    ADCQ BX, R10

    // Now we have:
    // l[0..3] in memory at DI
    // l[4] = R12
    // l[5] = R15
    // l[6] = R9
    // l[7] = R10

    // Save l4-l7 to stack for reduction phase
    MOVQ R12, 0(SP)       // l4
    MOVQ R15, 8(SP)       // l5
    MOVQ R9, 16(SP)       // l6
    MOVQ R10, 24(SP)      // l7

    // === Reduction modulo scalar order n ===
    // n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
    // NC = 2^256 - n = { 0x402DA1732FC9BEBF, 0x4551231950B75FC4, 1, 0 }
    //
    // Phase 1: Reduce 512 bits to 385 bits
    // m[0..6] = l[0..3] + l[4..7] * NC

    // Load constants
    MOVQ $0x402DA1732FC9BEBF, R8  // NC0
    MOVQ $0x4551231950B75FC4, R11 // NC1

    // === m0 ===
    // c0 = l[0], c1 = 0
    // muladd_fast(l4, NC0)
    // m0 = extract_fast()
    MOVQ 0(DI), R13       // c0 = l0
    XORQ R14, R14         // c1 = 0

    MOVQ R12, DX          // l4
    MULXQ R8, AX, BX      // l4 * NC0 -> BX:AX
    ADDQ AX, R13          // c0 += lo
    ADCQ BX, R14          // c1 += hi + carry

    // m0 = c0, shift accum
    MOVQ R13, 32(SP)      // m0
    MOVQ R14, R13         // c0 = c1
    XORQ R14, R14         // c1 = 0
    XORQ BP, BP           // c2 = 0

    // === m1 ===
    // sumadd_fast(l[1])
    // muladd(l5, NC0)
    // muladd(l4, NC1)
    ADDQ 8(DI), R13       // c0 += l1
    ADCQ $0, R14

    MOVQ R15, DX          // l5
    MULXQ R8, AX, BX      // l5 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ R12, DX          // l4
    MULXQ R11, AX, BX     // l4 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    // m1 = c0, shift
    MOVQ R13, 40(SP)      // m1
    MOVQ R14, R13
    MOVQ BP, R14
    XORQ BP, BP

    // === m2 ===
    // sumadd(l[2])
    // muladd(l6, NC0)
    // muladd(l5, NC1)
    // sumadd(l4)  (NC2 = 1)
    ADDQ 16(DI), R13      // c0 += l2
    ADCQ $0, R14
    ADCQ $0, BP

    MOVQ 16(SP), DX       // l6
    MULXQ R8, AX, BX      // l6 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ R15, DX          // l5
    MULXQ R11, AX, BX     // l5 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    ADDQ R12, R13         // c0 += l4 (l4 * NC2 = l4 * 1)
    ADCQ $0, R14
    ADCQ $0, BP

    // m2 = c0
    MOVQ R13, 48(SP)      // m2
    MOVQ R14, R13
    MOVQ BP, R14
    XORQ BP, BP

    // === m3 ===
    // sumadd(l[3])
    // muladd(l7, NC0)
    // muladd(l6, NC1)
    // sumadd(l5)
    ADDQ 24(DI), R13      // c0 += l3
    ADCQ $0, R14
    ADCQ $0, BP

    MOVQ 24(SP), DX       // l7
    MULXQ R8, AX, BX      // l7 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ 16(SP), DX       // l6
    MULXQ R11, AX, BX     // l6 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    ADDQ R15, R13         // c0 += l5
    ADCQ $0, R14
    ADCQ $0, BP

    // m3 = c0
    MOVQ R13, 56(SP)      // m3
    MOVQ R14, R13
    MOVQ BP, R14

    // === m4 ===
    // muladd(l7, NC1)
    // sumadd(l6)
    MOVQ 24(SP), DX       // l7
    MULXQ R11, AX, BX     // l7 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14

    ADDQ 16(SP), R13      // c0 += l6
    ADCQ $0, R14

    // m4 in R13
    MOVQ R13, R12         // m4 = c0
    MOVQ R14, R13         // c0 = c1

    // === m5 ===
    // sumadd_fast(l7)
    ADDQ 24(SP), R13      // c0 += l7
    MOVQ $0, R9
    ADCQ $0, R9
    // m5 in R13
    MOVQ R13, R15         // m5

    // === m6 ===
    // m6 = carry (should be small, often 0)
    // R9 already has the carry

    // Phase 2: Reduce 385 bits to 258 bits
    // p[0..4] = m[0..3] + m[4..6] * NC

    // === p0 ===
    MOVQ 32(SP), R13      // c0 = m0
    XORQ R14, R14         // c1 = 0

    MOVQ R12, DX          // m4
    MULXQ R8, AX, BX      // m4 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14

    MOVQ R13, 0(DI)       // p0 = c0
    MOVQ R14, R13
    XORQ R14, R14
    XORQ BP, BP

    // === p1 ===
    ADDQ 40(SP), R13      // c0 += m1

    MOVQ R15, DX          // m5
    MULXQ R8, AX, BX      // m5 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ R12, DX          // m4
    MULXQ R11, AX, BX     // m4 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ R13, 8(DI)       // p1
    MOVQ R14, R13
    MOVQ BP, R14
    XORQ BP, BP

    // === p2 ===
    ADDQ 48(SP), R13      // c0 += m2
    ADCQ $0, R14
    ADCQ $0, BP

    MOVQ R9, DX           // m6
    MULXQ R8, AX, BX      // m6 * NC0 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    MOVQ R15, DX          // m5
    MULXQ R11, AX, BX     // m5 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14
    ADCQ $0, BP

    ADDQ R12, R13         // c0 += m4
    ADCQ $0, R14
    ADCQ $0, BP

    MOVQ R13, 16(DI)      // p2
    MOVQ R14, R13
    MOVQ BP, R14

    // === p3 ===
    ADDQ 56(SP), R13      // c0 += m3

    MOVQ R9, DX           // m6
    MULXQ R11, AX, BX     // m6 * NC1 -> BX:AX
    ADDQ AX, R13
    ADCQ BX, R14

    ADDQ R15, R13         // c0 += m5
    ADCQ $0, R14

    MOVQ R13, 24(DI)      // p3
    // p4 = c1 + m6
    ADDQ R14, R9          // p4 = R9

    // Phase 3: Reduce 258 bits to 256 bits
    // r[0..3] = p[0..3] + p[4] * NC

    // r0 = p0 + p4 * NC0
    MOVQ R9, DX           // p4
    MULXQ R8, AX, BX      // p4 * NC0 -> BX:AX
    ADDQ 0(DI), AX        // AX = p0 + lo
    ADCQ $0, BX           // BX = hi + carry
    MOVQ AX, R12          // r0
    MOVQ BX, R13          // carry

    // r1 = p1 + p4 * NC1 + carry
    MOVQ R9, DX           // p4
    MULXQ R11, AX, BX     // p4 * NC1 -> BX:AX
    ADDQ R13, AX          // AX += carry
    ADCQ $0, BX
    ADDQ 8(DI), AX        // AX += p1
    ADCQ $0, BX
    MOVQ AX, R14          // r1
    MOVQ BX, R13          // carry

    // r2 = p2 + p4 + carry (NC2 = 1)
    MOVQ 16(DI), AX
    ADDQ R13, AX          // AX = p2 + carry
    MOVQ $0, R13
    ADCQ $0, R13
    ADDQ R9, AX           // AX += p4
    ADCQ $0, R13
    MOVQ AX, R15          // r2

    // r3 = p3 + carry
    MOVQ 24(DI), AX
    ADDQ R13, AX
    MOVQ $0, R10
    ADCQ $0, R10          // final carry
    MOVQ AX, BP           // r3

    // Check if we need to reduce (carry or result >= n)
    TESTQ R10, R10
    JNZ bmi2_do_final_reduce

    // Compare with n (from high to low)
    MOVQ $0xFFFFFFFFFFFFFFFF, R13
    CMPQ BP, R13
    JB bmi2_store_result
    JA bmi2_do_final_reduce
    MOVQ $0xFFFFFFFFFFFFFFFE, R13
    CMPQ R15, R13
    JB bmi2_store_result
    JA bmi2_do_final_reduce
    MOVQ $0xBAAEDCE6AF48A03B, R13
    CMPQ R14, R13
    JB bmi2_store_result
    JA bmi2_do_final_reduce
    MOVQ $0xBFD25E8CD0364141, R13
    CMPQ R12, R13
    JB bmi2_store_result

bmi2_do_final_reduce:
    // Add 2^256 - n
    ADDQ R8, R12          // r0 += NC0
    ADCQ R11, R14         // r1 += NC1
    ADCQ $1, R15          // r2 += NC2 = 1
    ADCQ $0, BP           // r3 += 0

bmi2_store_result:
    // Store result
    MOVQ R12, 0(DI)
    MOVQ R14, 8(DI)
    MOVQ R15, 16(DI)
    MOVQ BP, 24(DI)

    RET
