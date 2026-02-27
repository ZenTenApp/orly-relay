//go:build amd64

#include "textflag.h"

// func uint128Mul(a, b Uint128) [4]uint64
// Multiplies two 128-bit values and returns a 256-bit result.
//
// Input:
//   a.Lo = arg+0(FP)
//   a.Hi = arg+8(FP)
//   b.Lo = arg+16(FP)
//   b.Hi = arg+24(FP)
//
// Output:
//   result[0] = ret+32(FP)  (bits 0-63)
//   result[1] = ret+40(FP)  (bits 64-127)
//   result[2] = ret+48(FP)  (bits 128-191)
//   result[3] = ret+56(FP)  (bits 192-255)
//
// Algorithm:
//   (a.Hi*2^64 + a.Lo) * (b.Hi*2^64 + b.Lo)
//   = a.Hi*b.Hi*2^128 + (a.Hi*b.Lo + a.Lo*b.Hi)*2^64 + a.Lo*b.Lo
//
TEXT ·uint128Mul(SB), NOSPLIT, $0-64
	// Load inputs
	MOVQ a_Lo+0(FP), AX      // AX = a.Lo
	MOVQ a_Hi+8(FP), BX      // BX = a.Hi
	MOVQ b_Lo+16(FP), CX     // CX = b.Lo
	MOVQ b_Hi+24(FP), DX     // DX = b.Hi

	// Save b.Hi for later (DX will be clobbered by MUL)
	MOVQ DX, R11             // R11 = b.Hi

	// r0:r1 = a.Lo * b.Lo
	MOVQ AX, R8              // R8 = a.Lo (save for later)
	MULQ CX                  // DX:AX = a.Lo * b.Lo
	MOVQ AX, R9              // R9 = result[0] (low 64 bits)
	MOVQ DX, R10             // R10 = carry to result[1]

	// r1:r2 += a.Hi * b.Lo
	MOVQ BX, AX              // AX = a.Hi
	MULQ CX                  // DX:AX = a.Hi * b.Lo
	ADDQ AX, R10             // R10 += low part
	ADCQ $0, DX              // DX += carry
	MOVQ DX, CX              // CX = carry to result[2]

	// r1:r2 += a.Lo * b.Hi
	MOVQ R8, AX              // AX = a.Lo
	MULQ R11                 // DX:AX = a.Lo * b.Hi
	ADDQ AX, R10             // R10 += low part
	ADCQ DX, CX              // CX += high part + carry
	MOVQ $0, R8
	ADCQ $0, R8              // R8 = carry to result[3]

	// r2:r3 += a.Hi * b.Hi
	MOVQ BX, AX              // AX = a.Hi
	MULQ R11                 // DX:AX = a.Hi * b.Hi
	ADDQ AX, CX              // CX += low part
	ADCQ DX, R8              // R8 += high part + carry

	// Store results
	MOVQ R9, ret+32(FP)      // result[0]
	MOVQ R10, ret+40(FP)     // result[1]
	MOVQ CX, ret+48(FP)      // result[2]
	MOVQ R8, ret+56(FP)      // result[3]

	RET
