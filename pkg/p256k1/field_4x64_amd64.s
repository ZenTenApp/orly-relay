//go:build amd64 && !purego

#include "textflag.h"

// Field multiplication for secp256k1 using 4x64-bit limbs with BMI2 instructions.
// Uses MULX for flag-free multiplication.
//
// The field element is represented as 4 limbs of 64 bits each:
//   n[0..3] where value = n[0] + n[1]*2^64 + n[2]*2^128 + n[3]*2^192
//
// Field prime p = 2^256 - 2^32 - 977
// Reduction constant R = 2^256 mod p = 2^32 + 977 = 0x1000003D1
//
// func field4x64MulAsm(r, a, b *[4]uint64)
TEXT ·field4x64MulAsm(SB), NOSPLIT, $0-24
    MOVQ r+0(FP), DI      // result pointer
    MOVQ a+8(FP), SI      // a pointer
    MOVQ b+16(FP), CX     // b pointer

    // Load a[0..3]
    MOVQ 0(SI), R8        // a0
    MOVQ 8(SI), R9        // a1
    MOVQ 16(SI), R10      // a2
    MOVQ 24(SI), R11      // a3

    // We'll compute the 512-bit product in R12:R13:R14:R15:AX:BX:BP:DX
    // Actually, we'll use a different approach: accumulate column by column

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
    MOVQ R12, 0(DI)       // Save r0
    MOVQ R13, 8(DI)       // Save r1
    MOVQ R14, 16(DI)      // Save r2

    // Now R12, R13, R14 are free
    MOVQ R15, R12         // r3 accumulator low
    MOVQ BP, R13          // r3 accumulator high
    XORQ R14, R14         // r4 accumulator

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

    MOVQ R12, 24(DI)      // Save r3

    // Column 4: a1*b3 + a2*b2 + a3*b1
    MOVQ R13, R12         // r4 accumulator low
    MOVQ R14, R13         // r4 accumulator high
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

    // r4 is in R12, carry in R13:R14

    // Column 5: a2*b3 + a3*b2
    MOVQ R13, R15         // r5 accumulator low
    MOVQ R14, BP          // r5 accumulator high
    XORQ R8, R8           // reuse R8 for r6

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
    MOVQ BP, R9           // r6 accumulator low
    MOVQ R8, R10          // r6 accumulator high (will be r7)

    MOVQ 24(CX), DX       // b3
    MULXQ R11, AX, BX     // a3*b3 -> BX:AX
    ADDQ AX, R9
    ADCQ BX, R10

    // Now we have:
    // r[0..3] in memory at DI
    // r[4] = R12
    // r[5] = R15
    // r[6] = R9
    // r[7] = R10

    // === Reduction: r[4..7] * R where R = 0x1000003D1 ===
    // t[i] = r[i+4] * R, then add t to r[0..3]

    MOVQ $0x1000003D1, DX // R constant

    // t0 = r4 * R
    MULXQ R12, R8, R11    // r4 * R -> R11:R8 (hi:lo)

    // t1 = r5 * R + hi(t0)
    MULXQ R15, AX, BX     // r5 * R -> BX:AX
    ADDQ R11, AX
    ADCQ $0, BX
    MOVQ AX, R11          // t1 low
    MOVQ BX, R12          // t1 hi -> will be t2

    // t2 = r6 * R + hi(t1)
    MULXQ R9, AX, BX      // r6 * R -> BX:AX
    ADDQ R12, AX
    ADCQ $0, BX
    MOVQ AX, R12          // t2 low
    MOVQ BX, R13          // t2 hi -> will be t3

    // t3 = r7 * R + hi(t2)
    MULXQ R10, AX, BX     // r7 * R -> BX:AX
    ADDQ R13, AX
    ADCQ $0, BX
    MOVQ AX, R13          // t3 low
    MOVQ BX, R14          // t4 (overflow)

    // Add t[0..3] to r[0..3]
    ADDQ R8, 0(DI)        // r0 += t0
    ADCQ R11, 8(DI)       // r1 += t1
    ADCQ R12, 16(DI)      // r2 += t2
    ADCQ R13, 24(DI)      // r3 += t3
    ADCQ $0, R14          // capture final carry into t4

    // If t4 != 0, we need another reduction round
    TESTQ R14, R14
    JZ done

    // overflow * R
    MULXQ R14, AX, BX     // t4 * R -> BX:AX
    ADDQ AX, 0(DI)
    ADCQ BX, 8(DI)
    ADCQ $0, 16(DI)
    ADCQ $0, 24(DI)
    // If this still overflows, add R one more time (extremely rare)
    JNC done
    MOVQ $0x1000003D1, AX
    ADDQ AX, 0(DI)
    ADCQ $0, 8(DI)
    ADCQ $0, 16(DI)
    ADCQ $0, 24(DI)

done:
    RET

// func field4x64SqrAsm(r, a *[4]uint64)
// Optimized squaring: exploits symmetry a[i]*a[j] = a[j]*a[i]
// For now, inline calls to mul logic with b=a
TEXT ·field4x64SqrAsm(SB), NOSPLIT, $0-16
    MOVQ r+0(FP), DI      // result pointer
    MOVQ a+8(FP), SI      // a pointer
    MOVQ SI, CX           // b = a (same pointer)

    // Load a[0..3]
    MOVQ 0(SI), R8        // a0
    MOVQ 8(SI), R9        // a1
    MOVQ 16(SI), R10      // a2
    MOVQ 24(SI), R11      // a3

    // Column 0: a0*a0
    MOVQ R8, DX           // a0 into DX for MULX
    MULXQ R8, R12, R13    // a0*a0 -> R13:R12 (hi:lo)

    // Column 1: 2*a0*a1
    // Need to compute: R14:R13 += 2*(BX:AX) where BX:AX = a0*a1
    MOVQ R9, DX           // a1
    MULXQ R8, AX, BX      // a0*a1 -> BX:AX
    XORQ R14, R14
    XORQ R15, R15
    ADDQ AX, R13          // R13 += AX, CF1
    ADCQ $0, R14          // R14 = CF1
    ADDQ AX, R13          // R13 += AX again (2*AX total), CF2
    ADCQ BX, R14          // R14 += BX + CF2
    ADCQ $0, R15          // R15 = overflow from R14
    ADDQ BX, R14          // R14 += BX again (2*BX total), CF3
    ADCQ $0, R15          // R15 += CF3

    // Column 2: 2*a0*a2 + a1*a1
    MOVQ R10, DX          // a2
    MULXQ R8, AX, BX      // a0*a2 -> BX:AX
    ADDQ AX, R14
    ADCQ BX, R15
    ADDQ AX, R14          // double it
    ADCQ BX, R15
    MOVQ $0, BP
    ADCQ $0, BP

    MOVQ R9, DX           // a1
    MULXQ R9, AX, BX      // a1*a1 -> BX:AX
    ADDQ AX, R14
    ADCQ BX, R15
    ADCQ $0, BP

    // Save r0, r1, r2
    MOVQ R12, 0(DI)
    MOVQ R13, 8(DI)
    MOVQ R14, 16(DI)

    // Column 3: 2*a0*a3 + 2*a1*a2
    MOVQ R15, R12
    MOVQ BP, R13
    XORQ R14, R14

    MOVQ R11, DX          // a3
    MULXQ R8, AX, BX      // a0*a3 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14
    ADDQ AX, R12          // double
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ R10, DX          // a2
    MULXQ R9, AX, BX      // a1*a2 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14
    ADDQ AX, R12          // double
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ R12, 24(DI)      // Save r3

    // Column 4: 2*a1*a3 + a2*a2
    MOVQ R13, R12
    MOVQ R14, R13
    XORQ R14, R14

    MOVQ R11, DX          // a3
    MULXQ R9, AX, BX      // a1*a3 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14
    ADDQ AX, R12          // double
    ADCQ BX, R13
    ADCQ $0, R14

    MOVQ R10, DX          // a2
    MULXQ R10, AX, BX     // a2*a2 -> BX:AX
    ADDQ AX, R12
    ADCQ BX, R13
    ADCQ $0, R14

    // Column 5: 2*a2*a3
    MOVQ R13, R15
    MOVQ R14, BP
    XORQ R8, R8

    MOVQ R11, DX          // a3
    MULXQ R10, AX, BX     // a2*a3 -> BX:AX
    ADDQ AX, R15
    ADCQ BX, BP
    ADCQ $0, R8
    ADDQ AX, R15          // double
    ADCQ BX, BP
    ADCQ $0, R8

    // Column 6: a3*a3
    MOVQ BP, R9
    MOVQ R8, R10

    MOVQ R11, DX          // a3
    MULXQ R11, AX, BX     // a3*a3 -> BX:AX
    ADDQ AX, R9
    ADCQ BX, R10

    // Now we have:
    // r[0..3] in memory at DI
    // r[4] = R12, r[5] = R15, r[6] = R9, r[7] = R10

    // === Reduction: r[4..7] * R where R = 0x1000003D1 ===
    MOVQ $0x1000003D1, DX

    // t0 = r4 * R
    MULXQ R12, R8, R11    // r4 * R -> R11:R8

    // t1 = r5 * R + hi(t0)
    MULXQ R15, AX, BX     // r5 * R -> BX:AX
    ADDQ R11, AX
    ADCQ $0, BX
    MOVQ AX, R11
    MOVQ BX, R12

    // t2 = r6 * R + hi(t1)
    MULXQ R9, AX, BX      // r6 * R -> BX:AX
    ADDQ R12, AX
    ADCQ $0, BX
    MOVQ AX, R12
    MOVQ BX, R13

    // t3 = r7 * R + hi(t2)
    MULXQ R10, AX, BX     // r7 * R -> BX:AX
    ADDQ R13, AX
    ADCQ $0, BX
    MOVQ AX, R13
    MOVQ BX, R14

    // Add t[0..3] to r[0..3]
    ADDQ R8, 0(DI)
    ADCQ R11, 8(DI)
    ADCQ R12, 16(DI)
    ADCQ R13, 24(DI)
    ADCQ $0, R14

    // If t4 != 0, we need another reduction round
    TESTQ R14, R14
    JZ sqr_done

    // overflow * R
    MULXQ R14, AX, BX
    ADDQ AX, 0(DI)
    ADCQ BX, 8(DI)
    ADCQ $0, 16(DI)
    ADCQ $0, 24(DI)
    JNC sqr_done
    MOVQ $0x1000003D1, AX
    ADDQ AX, 0(DI)
    ADCQ $0, 8(DI)
    ADCQ $0, 16(DI)
    ADCQ $0, 24(DI)

sqr_done:
    RET
