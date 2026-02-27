//go:build amd64

#include "textflag.h"

// Field prime p = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
DATA fieldP<>+0x00(SB)/8, $0xFFFFFFFEFFFFFC2F
DATA fieldP<>+0x08(SB)/8, $0xFFFFFFFFFFFFFFFF
DATA fieldP<>+0x10(SB)/8, $0xFFFFFFFFFFFFFFFF
DATA fieldP<>+0x18(SB)/8, $0xFFFFFFFFFFFFFFFF
GLOBL fieldP<>(SB), RODATA|NOPTR, $32

// 2^256 - p = 2^32 + 977 = 0x1000003D1
DATA fieldPC<>+0x00(SB)/8, $0x1000003D1
DATA fieldPC<>+0x08(SB)/8, $0x0000000000000000
DATA fieldPC<>+0x10(SB)/8, $0x0000000000000000
DATA fieldPC<>+0x18(SB)/8, $0x0000000000000000
GLOBL fieldPC<>(SB), RODATA|NOPTR, $32

// func FieldAddAVX2(r, a, b *FieldElement)
// Adds two 256-bit field elements mod p.
TEXT ·FieldAddAVX2(SB), NOSPLIT, $0-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a
	MOVQ 0(SI), AX
	MOVQ 8(SI), BX
	MOVQ 16(SI), CX
	MOVQ 24(SI), R8

	// Add b with carry chain
	ADDQ 0(DX), AX
	ADCQ 8(DX), BX
	ADCQ 16(DX), CX
	ADCQ 24(DX), R8

	// Save carry
	SETCS R9B

	// Store preliminary result
	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

	// Check if we need to reduce
	TESTB R9B, R9B
	JNZ field_reduce

	// Compare with p (from high to low)
	// p.Hi = 0xFFFFFFFFFFFFFFFF (all limbs except first)
	// p.Lo = 0xFFFFFFFEFFFFFC2F
	MOVQ $0xFFFFFFFFFFFFFFFF, R10
	CMPQ R8, R10
	JB field_done
	JA field_reduce
	CMPQ CX, R10
	JB field_done
	JA field_reduce
	CMPQ BX, R10
	JB field_done
	JA field_reduce
	MOVQ fieldP<>+0x00(SB), R10
	CMPQ AX, R10
	JB field_done

field_reduce:
	// Subtract p by adding 2^256 - p = 0x1000003D1
	MOVQ 0(DI), AX
	MOVQ 8(DI), BX
	MOVQ 16(DI), CX
	MOVQ 24(DI), R8

	MOVQ fieldPC<>+0x00(SB), R10
	ADDQ R10, AX
	ADCQ $0, BX
	ADCQ $0, CX
	ADCQ $0, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

field_done:
	VZEROUPPER
	RET

// func FieldSubAVX2(r, a, b *FieldElement)
// Subtracts two 256-bit field elements mod p.
TEXT ·FieldSubAVX2(SB), NOSPLIT, $0-24
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

	// Save borrow
	SETCS R9B

	// Store preliminary result
	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

	// If borrow, add p back
	TESTB R9B, R9B
	JZ field_sub_done

	// Add p from memory
	MOVQ fieldP<>+0x00(SB), R10
	ADDQ R10, AX
	MOVQ fieldP<>+0x08(SB), R10
	ADCQ R10, BX
	MOVQ fieldP<>+0x10(SB), R10
	ADCQ R10, CX
	MOVQ fieldP<>+0x18(SB), R10
	ADCQ R10, R8

	MOVQ AX, 0(DI)
	MOVQ BX, 8(DI)
	MOVQ CX, 16(DI)
	MOVQ R8, 24(DI)

field_sub_done:
	VZEROUPPER
	RET

// func FieldMulAVX2(r, a, b *FieldElement)
// Multiplies two 256-bit field elements mod p.
TEXT ·FieldMulAVX2(SB), NOSPLIT, $64-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX

	// Load a limbs
	MOVQ 0(SI), R8      // a0
	MOVQ 8(SI), R9      // a1
	MOVQ 16(SI), R10    // a2
	MOVQ 24(SI), R11    // a3

	// Store b pointer
	MOVQ DX, R12

	// Initialize 512-bit product on stack
	XORQ AX, AX
	MOVQ AX, 0(SP)
	MOVQ AX, 8(SP)
	MOVQ AX, 16(SP)
	MOVQ AX, 24(SP)
	MOVQ AX, 32(SP)
	MOVQ AX, 40(SP)
	MOVQ AX, 48(SP)
	MOVQ AX, 56(SP)

	// Schoolbook multiplication (same as scalar, but with field reduction)
	// a0 * b[0..3]
	MOVQ R8, AX
	MULQ 0(R12)
	MOVQ AX, 0(SP)
	MOVQ DX, R13

	MOVQ R8, AX
	MULQ 8(R12)
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 8(SP)
	MOVQ DX, R13

	MOVQ R8, AX
	MULQ 16(R12)
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 16(SP)
	MOVQ DX, R13

	MOVQ R8, AX
	MULQ 24(R12)
	ADDQ R13, AX
	ADCQ $0, DX
	MOVQ AX, 24(SP)
	MOVQ DX, 32(SP)

	// a1 * b[0..3]
	MOVQ R9, AX
	MULQ 0(R12)
	ADDQ AX, 8(SP)
	ADCQ DX, 16(SP)
	ADCQ $0, 24(SP)
	ADCQ $0, 32(SP)

	MOVQ R9, AX
	MULQ 8(R12)
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)
	ADCQ $0, 32(SP)

	MOVQ R9, AX
	MULQ 16(R12)
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R9, AX
	MULQ 24(R12)
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)

	// a2 * b[0..3]
	MOVQ R10, AX
	MULQ 0(R12)
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)
	ADCQ $0, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R10, AX
	MULQ 8(R12)
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)

	MOVQ R10, AX
	MULQ 16(R12)
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R10, AX
	MULQ 24(R12)
	ADDQ AX, 40(SP)
	ADCQ DX, 48(SP)

	// a3 * b[0..3]
	MOVQ R11, AX
	MULQ 0(R12)
	ADDQ AX, 24(SP)
	ADCQ DX, 32(SP)
	ADCQ $0, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R11, AX
	MULQ 8(R12)
	ADDQ AX, 32(SP)
	ADCQ DX, 40(SP)
	ADCQ $0, 48(SP)

	MOVQ R11, AX
	MULQ 16(R12)
	ADDQ AX, 40(SP)
	ADCQ DX, 48(SP)
	ADCQ $0, 56(SP)

	MOVQ R11, AX
	MULQ 24(R12)
	ADDQ AX, 48(SP)
	ADCQ DX, 56(SP)

	// Now reduce 512-bit product mod p
	// Using 2^256 ≡ 2^32 + 977 (mod p)

	// high = [32(SP), 40(SP), 48(SP), 56(SP)]
	// low = [0(SP), 8(SP), 16(SP), 24(SP)]
	// result = low + high * (2^32 + 977)

	// Multiply high * 0x1000003D1
	MOVQ fieldPC<>+0x00(SB), R13

	MOVQ 32(SP), AX
	MULQ R13
	MOVQ AX, R8     // reduction[0]
	MOVQ DX, R14    // carry

	MOVQ 40(SP), AX
	MULQ R13
	ADDQ R14, AX
	ADCQ $0, DX
	MOVQ AX, R9     // reduction[1]
	MOVQ DX, R14

	MOVQ 48(SP), AX
	MULQ R13
	ADDQ R14, AX
	ADCQ $0, DX
	MOVQ AX, R10    // reduction[2]
	MOVQ DX, R14

	MOVQ 56(SP), AX
	MULQ R13
	ADDQ R14, AX
	ADCQ $0, DX
	MOVQ AX, R11    // reduction[3]
	MOVQ DX, R14    // reduction[4] (overflow)

	// Add low + reduction
	ADDQ 0(SP), R8
	ADCQ 8(SP), R9
	ADCQ 16(SP), R10
	ADCQ 24(SP), R11
	ADCQ $0, R14    // Capture any carry into R14

	// If R14 is non-zero, reduce again
	TESTQ R14, R14
	JZ field_mul_check

	// R14 * 0x1000003D1
	MOVQ R14, AX
	MULQ R13
	ADDQ AX, R8
	ADCQ DX, R9
	ADCQ $0, R10
	ADCQ $0, R11

field_mul_check:
	// Check if result >= p and reduce if needed
	MOVQ $0xFFFFFFFFFFFFFFFF, R15
	CMPQ R11, R15
	JB field_mul_store
	JA field_mul_reduce2
	CMPQ R10, R15
	JB field_mul_store
	JA field_mul_reduce2
	CMPQ R9, R15
	JB field_mul_store
	JA field_mul_reduce2
	MOVQ fieldP<>+0x00(SB), R15
	CMPQ R8, R15
	JB field_mul_store

field_mul_reduce2:
	MOVQ fieldPC<>+0x00(SB), R15
	ADDQ R15, R8
	ADCQ $0, R9
	ADCQ $0, R10
	ADCQ $0, R11

field_mul_store:
	MOVQ r+0(FP), DI
	MOVQ R8, 0(DI)
	MOVQ R9, 8(DI)
	MOVQ R10, 16(DI)
	MOVQ R11, 24(DI)

	VZEROUPPER
	RET

// func FieldSqrAVX2(r, a *FieldElement)
// Squares a 256-bit field element mod p.
// For now, just calls FieldMulAVX2(r, a, a)
TEXT ·FieldSqrAVX2(SB), NOSPLIT, $24-16
	MOVQ r+0(FP), AX
	MOVQ a+8(FP), BX
	MOVQ AX, 0(SP)
	MOVQ BX, 8(SP)
	MOVQ BX, 16(SP)
	CALL ·FieldMulAVX2(SB)
	RET
