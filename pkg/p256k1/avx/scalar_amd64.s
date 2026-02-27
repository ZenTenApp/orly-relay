//go:build amd64

#include "textflag.h"

// Constants for scalar reduction
// n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
DATA scalarN<>+0x00(SB)/8, $0xBFD25E8CD0364141
DATA scalarN<>+0x08(SB)/8, $0xBAAEDCE6AF48A03B
DATA scalarN<>+0x10(SB)/8, $0xFFFFFFFFFFFFFFFE
DATA scalarN<>+0x18(SB)/8, $0xFFFFFFFFFFFFFFFF
GLOBL scalarN<>(SB), RODATA|NOPTR, $32

// 2^256 - n (for reduction)
DATA scalarNC<>+0x00(SB)/8, $0x402DA1732FC9BEBF
DATA scalarNC<>+0x08(SB)/8, $0x4551231950B75FC4
DATA scalarNC<>+0x10(SB)/8, $0x0000000000000001
DATA scalarNC<>+0x18(SB)/8, $0x0000000000000000
GLOBL scalarNC<>(SB), RODATA|NOPTR, $32

// func hasAVX2() bool
TEXT ·hasAVX2(SB), NOSPLIT, $0-1
	MOVL $7, AX
	MOVL $0, CX
	CPUID
	ANDL $0x20, BX  // Check bit 5 of EBX for AVX2
	SETNE AL
	MOVB AL, ret+0(FP)
	RET

// func ScalarAddAVX2(r, a, b *Scalar)
// Adds two 256-bit scalars using AVX2 for loading/storing and scalar ADD with carry.
//
// YMM layout: [D[0].Lo, D[0].Hi, D[1].Lo, D[1].Hi] = 4 x 64-bit
TEXT ·ScalarAddAVX2(SB), NOSPLIT, $0-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a and b into registers (scalar loads for carry chain)
	MOVQ 0(SI), AX      // a.D[0].Lo
	MOVQ 8(SI), BX      // a.D[0].Hi
	MOVQ 16(SI), CX     // a.D[1].Lo
	MOVQ 24(SI), R8     // a.D[1].Hi

	// Add b with carry chain
	ADDQ 0(DX), AX      // a.D[0].Lo + b.D[0].Lo
	ADCQ 8(DX), BX      // a.D[0].Hi + b.D[0].Hi + carry
	ADCQ 16(DX), CX     // a.D[1].Lo + b.D[1].Lo + carry
	ADCQ 24(DX), R8     // a.D[1].Hi + b.D[1].Hi + carry

	// Save carry flag
	SETCS R9B

	// Store preliminary result
	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

	// Check if we need to reduce (carry set or result >= n)
	TESTB R9B, R9B
	JNZ reduce

	// Compare with n (from high to low)
	MOVQ $0xFFFFFFFFFFFFFFFF, R10
	CMPQ R8, R10
	JB done
	JA reduce
	MOVQ scalarN<>+0x10(SB), R10
	CMPQ CX, R10
	JB done
	JA reduce
	MOVQ scalarN<>+0x08(SB), R10
	CMPQ BX, R10
	JB done
	JA reduce
	MOVQ scalarN<>+0x00(SB), R10
	CMPQ AX, R10
	JB done

reduce:
	// Add 2^256 - n (which is equivalent to subtracting n)
	MOVQ 0(DI), AX
	MOVQ 8(DI), BX
	MOVQ 16(DI), CX
	MOVQ 24(DI), R8

	MOVQ scalarNC<>+0x00(SB), R10
	ADDQ R10, AX
	MOVQ scalarNC<>+0x08(SB), R10
	ADCQ R10, BX
	MOVQ scalarNC<>+0x10(SB), R10
	ADCQ R10, CX
	MOVQ scalarNC<>+0x18(SB), R10
	ADCQ R10, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

done:
	VZEROUPPER
	RET

// func ScalarSubAVX2(r, a, b *Scalar)
// Subtracts two 256-bit scalars.
TEXT ·ScalarSubAVX2(SB), NOSPLIT, $0-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a
	MOVQ 0(SI), AX
	MOVQ 8(SI), BX
	MOVQ 16(SI), CX
	MOVQ 24(SI), R8

	// Subtract b with borrow chain
	SUBQ 0(DX), AX
	SBBQ 8(DX), BX
	SBBQ 16(DX), CX
	SBBQ 24(DX), R8

	// Save borrow flag
	SETCS R9B

	// Store preliminary result
	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

	// If borrow, add n back
	TESTB R9B, R9B
	JZ done_sub

	// Add n
	MOVQ scalarN<>+0x00(SB), R10
	ADDQ R10, AX
	MOVQ scalarN<>+0x08(SB), R10
	ADCQ R10, BX
	MOVQ scalarN<>+0x10(SB), R10
	ADCQ R10, CX
	MOVQ scalarN<>+0x18(SB), R10
	ADCQ R10, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

done_sub:
	VZEROUPPER
	RET

// func ScalarMulAVX2(r, a, b *Scalar)
// Multiplies two 256-bit scalars and reduces mod n.
// This is a complex operation requiring 512-bit intermediate.
TEXT ·ScalarMulAVX2(SB), NOSPLIT, $64-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// We need to compute a 512-bit product and reduce mod n.
	// For now, use scalar multiplication with MULX (if BMI2 available) or MUL.

	// Load a limbs
	MOVQ 0(SI), R8      // a0
	MOVQ 8(SI), R9      // a1
	MOVQ 16(SI), R10    // a2
	MOVQ 24(SI), R11    // a3

	// Store b pointer for later use
	MOVQ DX, R12

	// Compute 512-bit product using schoolbook multiplication
	// Product stored on stack at SP+0 to SP+56 (8 limbs)

	// Initialize product to zero
	XORQ AX, AX
	MOVQ AX, 0(SP)
	MOVQ AX, 8(SP)
	MOVQ AX, 16(SP)
	MOVQ AX, 24(SP)
	MOVQ AX, 32(SP)
	MOVQ AX, 40(SP)
	MOVQ AX, 48(SP)
	MOVQ AX, 56(SP)

	// Multiply a0 * b[0..3]
	MOVQ R8, AX
	MULQ 0(R12)         // a0 * b0
	MOVQ AX, 0(SP)
	MOVQ DX, R13        // carry

	MOVQ R8, AX
	MULQ 8(R12)         // a0 * b1
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 8(SP)
	MOVQ DX, R13

	MOVQ R8, AX
	MULQ 16(R12)        // a0 * b2
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 16(SP)
	MOVQ DX, R13

	MOVQ R8, AX
	MULQ 24(R12)        // a0 * b3
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 24(SP)
	MOVQ DX, 32(SP)

	// Multiply a1 * b[0..3] and add
	MOVQ R9, AX
	MULQ 0(R12)         // a1 * b0
	ADDQ AX, 8(SP)
	ADCQ DX, 16(SP)
	ADCQ $0, 24(SP)
	ADCQ $0, 32(SP)

	MOVQ R9, AX
	MULQ 8(R12)         // a1 * b1
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)
	ADCQ $0, 32(SP)

	MOVQ R9, AX
	MULQ 16(R12)        // a1 * b2
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R9, AX
	MULQ 24(R12)        // a1 * b3
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)

	// Multiply a2 * b[0..3] and add
	MOVQ R10, AX
	MULQ 0(R12)         // a2 * b0
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)
	ADCQ $0, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R10, AX
	MULQ 8(R12)         // a2 * b1
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R10, AX
	MULQ 16(R12)        // a2 * b2
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R10, AX
	MULQ 24(R12)        // a2 * b3
	ADDQ AX, 40(SP)
	ADCQ DX, 48(SP)

	// Multiply a3 * b[0..3] and add
	MOVQ R11, AX
	MULQ 0(R12)         // a3 * b0
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R11, AX
	MULQ 8(R12)         // a3 * b1
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R11, AX
	MULQ 16(R12)        // a3 * b2
	ADDQ AX, 40(SP)
	ADCQ DX, 48(SP)
	ADCQ $0, 56(SP)

	MOVQ R11, AX
	MULQ 24(R12)        // a3 * b3
	ADDQ AX, 48(SP)
	ADCQ DX, 56(SP)

	// Now we have the 512-bit product in SP+0 to SP+56 (l[0..7])
	// Need to reduce mod n using the bitcoin-core algorithm:
	//
	// Phase 1: 512->385 bits
	//   c0..c4 = l[0..3] + l[4..7] * NC   (where NC = 2^256 - n)
	// Phase 2: 385->258 bits
	//   d0..d4 = c[0..3] + c[4] * NC
	// Phase 3: 258->256 bits
	//   r[0..3] = d[0..3] + d[4] * NC, then final reduce if >= n
	//
	// NC = [0x402DA1732FC9BEBF, 0x4551231950B75FC4, 1, 0]

	// ========== Phase 1: 512->385 bits ==========
	// Compute c[0..4] = l[0..3] + l[4..7] * NC
	// NC has only 3 significant limbs: NC[0], NC[1], NC[2]=1

	// Start with c = l[0..3], then add contributions from l[4..7] * NC
	MOVQ 0(SP), R8      // c0 = l0
	MOVQ 8(SP), R9      // c1 = l1
	MOVQ 16(SP), R10    // c2 = l2
	MOVQ 24(SP), R11    // c3 = l3
	XORQ R14, R14       // c4 = 0
	XORQ R15, R15       // c5 for overflow

	// l4 * NC[0]
	MOVQ 32(SP), AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12            // DX:AX = l4 * NC[0]
	ADDQ AX, R8
	ADCQ DX, R9
	ADCQ $0, R10
	ADCQ $0, R11
	ADCQ $0, R14

	// l4 * NC[1]
	MOVQ 32(SP), AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12            // DX:AX = l4 * NC[1]
	ADDQ AX, R9
	ADCQ DX, R10
	ADCQ $0, R11
	ADCQ $0, R14

	// l4 * NC[2] (NC[2] = 1)
	MOVQ 32(SP), AX
	ADDQ AX, R10
	ADCQ $0, R11
	ADCQ $0, R14

	// l5 * NC[0]
	MOVQ 40(SP), AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R9
	ADCQ DX, R10
	ADCQ $0, R11
	ADCQ $0, R14

	// l5 * NC[1]
	MOVQ 40(SP), AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R10
	ADCQ DX, R11
	ADCQ $0, R14

	// l5 * NC[2] (NC[2] = 1)
	MOVQ 40(SP), AX
	ADDQ AX, R11
	ADCQ $0, R14

	// l6 * NC[0]
	MOVQ 48(SP), AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R10
	ADCQ DX, R11
	ADCQ $0, R14

	// l6 * NC[1]
	MOVQ 48(SP), AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R11
	ADCQ DX, R14

	// l6 * NC[2] (NC[2] = 1)
	MOVQ 48(SP), AX
	ADDQ AX, R14
	ADCQ $0, R15

	// l7 * NC[0]
	MOVQ 56(SP), AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R11
	ADCQ DX, R14
	ADCQ $0, R15

	// l7 * NC[1]
	MOVQ 56(SP), AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R14
	ADCQ DX, R15

	// l7 * NC[2] (NC[2] = 1)
	MOVQ 56(SP), AX
	ADDQ AX, R15

	// Now c[0..5] = R8, R9, R10, R11, R14, R15 (~385 bits max)

	// ========== Phase 2: 385->258 bits ==========
	// Reduce c[4..5] by multiplying by NC and adding to c[0..3]

	// c4 * NC[0]
	MOVQ R14, AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R8
	ADCQ DX, R9
	ADCQ $0, R10
	ADCQ $0, R11

	// c4 * NC[1]
	MOVQ R14, AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R9
	ADCQ DX, R10
	ADCQ $0, R11

	// c4 * NC[2] (NC[2] = 1)
	ADDQ R14, R10
	ADCQ $0, R11

	// c5 * NC[0]
	MOVQ R15, AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R9
	ADCQ DX, R10
	ADCQ $0, R11

	// c5 * NC[1]
	MOVQ R15, AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R10
	ADCQ DX, R11

	// c5 * NC[2] (NC[2] = 1)
	ADDQ R15, R11
	// Capture any final carry into R14
	MOVQ $0, R14
	ADCQ $0, R14

	// Now we have ~258 bits in R8, R9, R10, R11, R14

	// ========== Phase 3: 258->256 bits ==========
	// If R14 (the overflow) is non-zero, reduce again
	TESTQ R14, R14
	JZ check_overflow

	// R14 * NC
	MOVQ R14, AX
	MOVQ scalarNC<>+0x00(SB), R12
	MULQ R12
	ADDQ AX, R8
	ADCQ DX, R9
	ADCQ $0, R10
	ADCQ $0, R11

	MOVQ R14, AX
	MOVQ scalarNC<>+0x08(SB), R12
	MULQ R12
	ADDQ AX, R9
	ADCQ DX, R10
	ADCQ $0, R11

	// R14 * NC[2] (NC[2] = 1)
	ADDQ R14, R10
	ADCQ $0, R11

check_overflow:
	// Check if result >= n and reduce if needed
	MOVQ $0xFFFFFFFFFFFFFFFF, R13
	CMPQ R11, R13
	JB store_result
	JA do_reduce
	MOVQ scalarN<>+0x10(SB), R13
	CMPQ R10, R13
	JB store_result
	JA do_reduce
	MOVQ scalarN<>+0x08(SB), R13
	CMPQ R9, R13
	JB store_result
	JA do_reduce
	MOVQ scalarN<>+0x00(SB), R13
	CMPQ R8, R13
	JB store_result

do_reduce:
	// Subtract n (add 2^256 - n)
	MOVQ scalarNC<>+0x00(SB), R13
	ADDQ R13, R8
	MOVQ scalarNC<>+0x08(SB), R13
	ADCQ R13, R9
	MOVQ scalarNC<>+0x10(SB), R13
	ADCQ R13, R10
	MOVQ scalarNC<>+0x18(SB), R13
	ADCQ R13, R11

store_result:
	// Store result
	MOVQ r+0(FP), DI
	MOVQ R8, 0(DI)
	MOVQ R9, 8(DI)
	MOVQ R10, 16(DI)
	MOVQ R11, 24(DI)

	VZEROUPPER
	RET
