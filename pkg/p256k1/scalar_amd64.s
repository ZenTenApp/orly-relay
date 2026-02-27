//go:build amd64

#include "textflag.h"

// Constants for scalar reduction
// n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
DATA p256k1ScalarN<>+0x00(SB)/8, $0xBFD25E8CD0364141
DATA p256k1ScalarN<>+0x08(SB)/8, $0xBAAEDCE6AF48A03B
DATA p256k1ScalarN<>+0x10(SB)/8, $0xFFFFFFFFFFFFFFFE
DATA p256k1ScalarN<>+0x18(SB)/8, $0xFFFFFFFFFFFFFFFF
GLOBL p256k1ScalarN<>(SB), RODATA|NOPTR, $32

// 2^256 - n (for reduction)
// NC0 = 0x402DA1732FC9BEBF
// NC1 = 0x4551231950B75FC4
// NC2 = 1
DATA p256k1ScalarNC<>+0x00(SB)/8, $0x402DA1732FC9BEBF
DATA p256k1ScalarNC<>+0x08(SB)/8, $0x4551231950B75FC4
DATA p256k1ScalarNC<>+0x10(SB)/8, $0x0000000000000001
DATA p256k1ScalarNC<>+0x18(SB)/8, $0x0000000000000000
GLOBL p256k1ScalarNC<>(SB), RODATA|NOPTR, $32

// func scalarAddAVX2(r, a, b *Scalar)
// Adds two 256-bit scalars with carry chain and modular reduction.
TEXT ·scalarAddAVX2(SB), NOSPLIT, $0-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a and b into registers (scalar loads for carry chain)
	MOVQ 0(SI), AX      // a.d[0]
	MOVQ 8(SI), BX      // a.d[1]
	MOVQ 16(SI), CX     // a.d[2]
	MOVQ 24(SI), R8     // a.d[3]

	// Add b with carry chain
	ADDQ 0(DX), AX      // a.d[0] + b.d[0]
	ADCQ 8(DX), BX      // a.d[1] + b.d[1] + carry
	ADCQ 16(DX), CX     // a.d[2] + b.d[2] + carry
	ADCQ 24(DX), R8     // a.d[3] + b.d[3] + carry

	// Save carry flag
	SETCS R9B

	// Store preliminary result
	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

	// Check if we need to reduce (carry set or result >= n)
	TESTB R9B, R9B
	JNZ add_reduce

	// Compare with n (from high to low)
	MOVQ $0xFFFFFFFFFFFFFFFF, R10
	CMPQ R8, R10
	JB add_done
	JA add_reduce
	MOVQ p256k1ScalarN<>+0x10(SB), R10
	CMPQ CX, R10
	JB add_done
	JA add_reduce
	MOVQ p256k1ScalarN<>+0x08(SB), R10
	CMPQ BX, R10
	JB add_done
	JA add_reduce
	MOVQ p256k1ScalarN<>+0x00(SB), R10
	CMPQ AX, R10
	JB add_done

add_reduce:
	// Add 2^256 - n (which is equivalent to subtracting n)
	MOVQ 0(DI), AX
	MOVQ 8(DI), BX
	MOVQ 16(DI), CX
	MOVQ 24(DI), R8

	MOVQ p256k1ScalarNC<>+0x00(SB), R10
	ADDQ R10, AX
	MOVQ p256k1ScalarNC<>+0x08(SB), R10
	ADCQ R10, BX
	MOVQ p256k1ScalarNC<>+0x10(SB), R10
	ADCQ R10, CX
	MOVQ p256k1ScalarNC<>+0x18(SB), R10
	ADCQ R10, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

add_done:
	VZEROUPPER
	RET

// func scalarSubAVX2(r, a, b *Scalar)
// Subtracts two 256-bit scalars.
TEXT ·scalarSubAVX2(SB), NOSPLIT, $0-24
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
	JZ sub_done

	// Add n
	MOVQ p256k1ScalarN<>+0x00(SB), R10
	ADDQ R10, AX
	MOVQ p256k1ScalarN<>+0x08(SB), R10
	ADCQ R10, BX
	MOVQ p256k1ScalarN<>+0x10(SB), R10
	ADCQ R10, CX
	MOVQ p256k1ScalarN<>+0x18(SB), R10
	ADCQ R10, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

sub_done:
	VZEROUPPER
	RET

// func scalarMulAVX2(r, a, b *Scalar)
// Multiplies two 256-bit scalars and reduces mod n.
// This implementation follows the bitcoin-core secp256k1 algorithm exactly.
TEXT ·scalarMulAVX2(SB), NOSPLIT, $128-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a limbs
	MOVQ 0(SI), R8      // a0
	MOVQ 8(SI), R9      // a1
	MOVQ 16(SI), R10    // a2
	MOVQ 24(SI), R11    // a3

	// Store b pointer for later use
	MOVQ DX, R12

	// Compute 512-bit product using schoolbook multiplication
	// Product stored on stack at SP+0 to SP+56 (8 limbs: l0..l7)

	// Initialize product to zero
	XORQ AX, AX
	MOVQ AX, 0(SP)      // l0
	MOVQ AX, 8(SP)      // l1
	MOVQ AX, 16(SP)     // l2
	MOVQ AX, 24(SP)     // l3
	MOVQ AX, 32(SP)     // l4
	MOVQ AX, 40(SP)     // l5
	MOVQ AX, 48(SP)     // l6
	MOVQ AX, 56(SP)     // l7

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

	// Now we have the 512-bit product in SP+0..SP+56 (l[0..7])
	// Reduce using the exact algorithm from bitcoin-core secp256k1
	//
	// Phase 1: Reduce 512 bits into 385 bits
	// m[0..6] = l[0..3] + n[0..3] * SECP256K1_N_C
	// where n[0..3] = l[4..7] (high 256 bits)
	//
	// NC0 = 0x402DA1732FC9BEBF
	// NC1 = 0x4551231950B75FC4
	// NC2 = 1

	// Load high limbs (l4..l7 = n0..n3)
	MOVQ 32(SP), R8     // n0 = l4
	MOVQ 40(SP), R9     // n1 = l5
	MOVQ 48(SP), R10    // n2 = l6
	MOVQ 56(SP), R11    // n3 = l7

	// Load constants
	MOVQ $0x402DA1732FC9BEBF, R12  // NC0
	MOVQ $0x4551231950B75FC4, R13  // NC1

	// Use stack locations 64-112 for intermediate m values
	// We'll use a 160-bit accumulator approach like the C code
	// c0 (R14), c1 (R15), c2 (stored on stack at 120(SP))

	// === m0 ===
	// c0 = l[0], c1 = 0
	// muladd_fast(n0, NC0): hi,lo = n0*NC0; c0 += lo, c1 += hi + carry
	// m0 = extract_fast() = c0; c0 = c1; c1 = 0
	MOVQ 0(SP), R14     // c0 = l0
	XORQ R15, R15       // c1 = 0
	MOVQ R8, AX
	MULQ R12            // DX:AX = n0 * NC0
	ADDQ AX, R14        // c0 += lo
	ADCQ DX, R15        // c1 += hi + carry
	MOVQ R14, 64(SP)    // m0 = c0
	MOVQ R15, R14       // c0 = c1
	XORQ R15, R15       // c1 = 0
	MOVQ $0, 120(SP)    // c2 = 0

	// === m1 ===
	// sumadd_fast(l[1])
	// muladd(n1, NC0)
	// muladd(n0, NC1)
	// m1 = extract()
	ADDQ 8(SP), R14     // c0 += l1
	ADCQ $0, R15        // c1 += carry

	MOVQ R9, AX
	MULQ R12            // DX:AX = n1 * NC0
	ADDQ AX, R14        // c0 += lo
	ADCQ DX, R15        // c1 += hi + carry
	ADCQ $0, 120(SP)    // c2 += carry

	MOVQ R8, AX
	MULQ R13            // DX:AX = n0 * NC1
	ADDQ AX, R14        // c0 += lo
	ADCQ DX, R15        // c1 += hi + carry
	ADCQ $0, 120(SP)    // c2 += carry

	MOVQ R14, 72(SP)    // m1 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2
	MOVQ $0, 120(SP)    // c2 = 0

	// === m2 ===
	// sumadd(l[2])
	// muladd(n2, NC0)
	// muladd(n1, NC1)
	// sumadd(n0)  (because NC2 = 1)
	// m2 = extract()
	ADDQ 16(SP), R14    // c0 += l2
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R10, AX
	MULQ R12            // DX:AX = n2 * NC0
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	MOVQ R9, AX
	MULQ R13            // DX:AX = n1 * NC1
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	ADDQ R8, R14        // c0 += n0 (n0 * NC2 = n0 * 1)
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R14, 80(SP)    // m2 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2
	MOVQ $0, 120(SP)    // c2 = 0

	// === m3 ===
	// sumadd(l[3])
	// muladd(n3, NC0)
	// muladd(n2, NC1)
	// sumadd(n1)
	// m3 = extract()
	ADDQ 24(SP), R14    // c0 += l3
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R11, AX
	MULQ R12            // DX:AX = n3 * NC0
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	MOVQ R10, AX
	MULQ R13            // DX:AX = n2 * NC1
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	ADDQ R9, R14        // c0 += n1
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R14, 88(SP)    // m3 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2
	MOVQ $0, 120(SP)    // c2 = 0

	// === m4 ===
	// muladd(n3, NC1)
	// sumadd(n2)
	// m4 = extract()
	MOVQ R11, AX
	MULQ R13            // DX:AX = n3 * NC1
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	ADDQ R10, R14       // c0 += n2
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R14, 96(SP)    // m4 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2

	// === m5 ===
	// sumadd_fast(n3)
	// m5 = extract_fast()
	ADDQ R11, R14       // c0 += n3
	ADCQ $0, R15        // c1 += carry

	MOVQ R14, 104(SP)   // m5 = c0
	MOVQ R15, R14       // c0 = c1

	// === m6 ===
	// m6 = c0 (low 32 bits only, but we keep full 64 bits for simplicity)
	MOVQ R14, 112(SP)   // m6 = c0

	// Phase 2: Reduce 385 bits into 258 bits
	// p[0..4] = m[0..3] + m[4..6] * SECP256K1_N_C
	// m4, m5 are 64-bit, m6 is at most 33 bits

	// Load m values
	MOVQ 96(SP), R8     // m4
	MOVQ 104(SP), R9    // m5
	MOVQ 112(SP), R10   // m6

	// === p0 ===
	// c0 = m0, c1 = 0
	// muladd_fast(m4, NC0)
	// p0 = extract_fast()
	MOVQ 64(SP), R14    // c0 = m0
	XORQ R15, R15       // c1 = 0

	MOVQ R8, AX
	MULQ R12            // DX:AX = m4 * NC0
	ADDQ AX, R14
	ADCQ DX, R15

	MOVQ R14, 64(SP)    // p0 = c0 (reuse m0 location)
	MOVQ R15, R14       // c0 = c1
	XORQ R15, R15       // c1 = 0
	MOVQ $0, 120(SP)    // c2 = 0

	// === p1 ===
	// sumadd_fast(m1)
	// muladd(m5, NC0)
	// muladd(m4, NC1)
	// p1 = extract()
	ADDQ 72(SP), R14    // c0 += m1
	ADCQ $0, R15

	MOVQ R9, AX
	MULQ R12            // DX:AX = m5 * NC0
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	MOVQ R8, AX
	MULQ R13            // DX:AX = m4 * NC1
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	MOVQ R14, 72(SP)    // p1 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2
	MOVQ $0, 120(SP)    // c2 = 0

	// === p2 ===
	// sumadd(m2)
	// muladd(m6, NC0)
	// muladd(m5, NC1)
	// sumadd(m4)
	// p2 = extract()
	ADDQ 80(SP), R14    // c0 += m2
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R10, AX
	MULQ R12            // DX:AX = m6 * NC0
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	MOVQ R9, AX
	MULQ R13            // DX:AX = m5 * NC1
	ADDQ AX, R14
	ADCQ DX, R15
	ADCQ $0, 120(SP)

	ADDQ R8, R14        // c0 += m4
	ADCQ $0, R15
	ADCQ $0, 120(SP)

	MOVQ R14, 80(SP)    // p2 = c0
	MOVQ R15, R14       // c0 = c1
	MOVQ 120(SP), R15   // c1 = c2

	// === p3 ===
	// sumadd_fast(m3)
	// muladd_fast(m6, NC1)
	// sumadd_fast(m5)
	// p3 = extract_fast()
	ADDQ 88(SP), R14    // c0 += m3
	ADCQ $0, R15

	MOVQ R10, AX
	MULQ R13            // DX:AX = m6 * NC1
	ADDQ AX, R14
	ADCQ DX, R15

	ADDQ R9, R14        // c0 += m5
	ADCQ $0, R15

	MOVQ R14, 88(SP)    // p3 = c0
	// p4 = c1 + m6
	ADDQ R15, R10       // p4 = c1 + m6

	// === p4 ===
	MOVQ R10, 96(SP)    // p4

	// Phase 3: Reduce 258 bits into 256 bits
	// r[0..3] = p[0..3] + p[4] * SECP256K1_N_C
	// Then check for overflow and reduce once more if needed

	// Use 128-bit arithmetic for this phase
	// t = p0 + p4 * NC0
	MOVQ 96(SP), R11    // p4

	// r0 = (p0 + p4 * NC0) mod 2^64, carry to next
	MOVQ R11, AX
	MULQ R12            // DX:AX = p4 * NC0
	ADDQ 64(SP), AX     // AX = p0 + lo
	ADCQ $0, DX         // DX = hi + carry
	MOVQ AX, R8         // r0
	MOVQ DX, R14        // carry

	// r1 = p1 + p4 * NC1 + carry
	MOVQ R11, AX
	MULQ R13            // DX:AX = p4 * NC1
	ADDQ R14, AX        // AX += carry
	ADCQ $0, DX
	ADDQ 72(SP), AX     // AX += p1
	ADCQ $0, DX
	MOVQ AX, R9         // r1
	MOVQ DX, R14        // carry

	// r2 = p2 + p4 * NC2 + carry = p2 + p4 + carry
	MOVQ 80(SP), AX
	ADDQ R14, AX        // AX = p2 + carry
	MOVQ $0, DX
	ADCQ $0, DX
	ADDQ R11, AX        // AX += p4 (NC2 = 1)
	ADCQ $0, DX
	MOVQ AX, R10        // r2
	MOVQ DX, R14        // carry

	// r3 = p3 + carry
	MOVQ 88(SP), AX
	ADDQ R14, AX
	SETCS R14B          // final carry
	MOVQ AX, R11        // r3

	// Check if we need to reduce (carry or result >= n)
	TESTB R14B, R14B
	JNZ mul_do_final_reduce

	// Compare with n (from high to low)
	MOVQ $0xFFFFFFFFFFFFFFFF, R15
	CMPQ R11, R15
	JB mul_store_result
	JA mul_do_final_reduce
	MOVQ $0xFFFFFFFFFFFFFFFE, R15
	CMPQ R10, R15
	JB mul_store_result
	JA mul_do_final_reduce
	MOVQ $0xBAAEDCE6AF48A03B, R15
	CMPQ R9, R15
	JB mul_store_result
	JA mul_do_final_reduce
	MOVQ $0xBFD25E8CD0364141, R15
	CMPQ R8, R15
	JB mul_store_result

mul_do_final_reduce:
	// Add 2^256 - n
	ADDQ R12, R8        // r0 += NC0
	ADCQ R13, R9        // r1 += NC1
	ADCQ $1, R10        // r2 += NC2 = 1
	ADCQ $0, R11        // r3 += 0

mul_store_result:
	// Store result
	MOVQ r+0(FP), DI
	MOVQ R8, 0(DI)
	MOVQ R9, 8(DI)
	MOVQ R10, 16(DI)
	MOVQ R11, 24(DI)

	VZEROUPPER
	RET
