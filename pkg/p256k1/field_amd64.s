//go:build amd64

#include "textflag.h"

// Field multiplication assembly for secp256k1 using 5x52-bit limb representation.
// Ported from bitcoin-core/secp256k1 field_5x52_asm_impl.h
//
// The field element is represented as 5 limbs of 52 bits each:
//   n[0..4] where value = sum(n[i] * 2^(52*i))
//
// Field prime p = 2^256 - 2^32 - 977
// Reduction constant R = 2^256 mod p = 2^32 + 977 = 0x1000003D1
// For 5x52: R shifted = 0x1000003D10 (for 52-bit alignment)
//
// Stack layout for fieldMulAsm (96 bytes):
//   0(SP)  - d_lo
//   8(SP)  - d_hi
//   16(SP) - c_lo
//   24(SP) - c_hi
//   32(SP) - t3
//   40(SP) - t4
//   48(SP) - tx
//   56(SP) - u0
//   64(SP) - temp storage
//   72(SP) - temp storage 2
//   80(SP) - saved b pointer

// Macro-like operations implemented inline:
// rshift52: shift 128-bit value right by 52
//   result_lo = (in_lo >> 52) | (in_hi << 12)
//   result_hi = in_hi >> 52

// func fieldMulAsm(r, a, b *FieldElement)
TEXT ·fieldMulAsm(SB), NOSPLIT, $96-24
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), BX

	// Save b pointer
	MOVQ BX, 80(SP)

	// Load a[0..4] into registers
	MOVQ 0(SI), R8       // a0
	MOVQ 8(SI), R9       // a1
	MOVQ 16(SI), R10     // a2
	MOVQ 24(SI), R11     // a3
	MOVQ 32(SI), R12     // a4

	// Constants we'll use frequently
	// M = 0xFFFFFFFFFFFFF (2^52 - 1)
	// R = 0x1000003D10

	// === Step 1: d = a0*b3 + a1*b2 + a2*b1 + a3*b0 ===
	MOVQ R8, AX
	MULQ 24(BX)          // a0 * b3
	MOVQ AX, 0(SP)       // d_lo
	MOVQ DX, 8(SP)       // d_hi

	MOVQ R9, AX
	MULQ 16(BX)          // a1 * b2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R10, AX
	MULQ 8(BX)           // a2 * b1
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R11, AX
	MULQ 0(BX)           // a3 * b0
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 2: c = a4*b4 ===
	MOVQ R12, AX
	MULQ 32(BX)          // a4 * b4
	MOVQ AX, 16(SP)      // c_lo
	MOVQ DX, 24(SP)      // c_hi

	// === Step 3: d += R * c_lo ===
	// Note: we use full c_lo (64 bits), NOT c_lo & M
	MOVQ 16(SP), AX      // c_lo (full 64 bits)
	MOVQ $0x1000003D10, CX  // R
	MULQ CX              // R * c_lo -> DX:AX
	ADDQ AX, 0(SP)       // d_lo += product_lo
	ADCQ DX, 8(SP)       // d_hi += product_hi + carry

	// === Step 4: c >>= 64 (just take c_hi) ===
	MOVQ 24(SP), AX      // c_hi
	MOVQ AX, 16(SP)      // new c = c_hi (single 64-bit now)
	MOVQ $0, 24(SP)      // c_hi = 0

	// === Step 5: t3 = d & M; d >>= 52 ===
	MOVQ 0(SP), AX       // d_lo
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX          // t3 = d & M
	MOVQ AX, 32(SP)      // save t3

	// d >>= 52: d_lo = (d_lo >> 52) | (d_hi << 12); d_hi >>= 52
	MOVQ 0(SP), AX       // d_lo
	MOVQ 8(SP), CX       // d_hi
	SHRQ $52, AX         // d_lo >> 52
	MOVQ CX, DX
	SHLQ $12, DX         // d_hi << 12
	ORQ DX, AX           // new d_lo
	SHRQ $52, CX         // new d_hi
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 6: d += a0*b4 + a1*b3 + a2*b2 + a3*b1 + a4*b0 ===
	MOVQ 80(SP), BX      // restore b pointer

	MOVQ R8, AX
	MULQ 32(BX)          // a0 * b4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R9, AX
	MULQ 24(BX)          // a1 * b3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R10, AX
	MULQ 16(BX)          // a2 * b2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R11, AX
	MULQ 8(BX)           // a3 * b1
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R12, AX
	MULQ 0(BX)           // a4 * b0
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 7: d += (R << 12) * c ===
	// R << 12 = 0x1000003D10 << 12 = 0x1000003D10000
	MOVQ 16(SP), AX      // c (from c >>= 64)
	MOVQ $0x1000003D10000, CX
	MULQ CX              // (R << 12) * c
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 8: t4 = d & M; tx = t4 >> 48; t4 &= (M >> 4) ===
	MOVQ 0(SP), AX       // d_lo
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX          // t4 = d & M
	MOVQ AX, 40(SP)      // save t4 (before modifications)

	SHRQ $48, AX         // tx = t4 >> 48
	MOVQ AX, 48(SP)      // save tx

	MOVQ 40(SP), AX
	MOVQ $0x0FFFFFFFFFFFF, CX  // M >> 4 = 2^48 - 1
	ANDQ CX, AX          // t4 &= (M >> 4)
	MOVQ AX, 40(SP)      // save final t4

	// === Step 9: d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 10: c = a0*b0 ===
	MOVQ R8, AX
	MULQ 0(BX)           // a0 * b0
	MOVQ AX, 16(SP)      // c_lo
	MOVQ DX, 24(SP)      // c_hi

	// === Step 11: d += a1*b4 + a2*b3 + a3*b2 + a4*b1 ===
	MOVQ R9, AX
	MULQ 32(BX)          // a1 * b4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R10, AX
	MULQ 24(BX)          // a2 * b3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R11, AX
	MULQ 16(BX)          // a3 * b2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R12, AX
	MULQ 8(BX)           // a4 * b1
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 12: u0 = d & M; d >>= 52; u0 = (u0 << 4) | tx ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX          // u0 = d & M
	SHLQ $4, AX          // u0 << 4
	ORQ 48(SP), AX       // u0 |= tx
	MOVQ AX, 56(SP)      // save u0

	// d >>= 52
	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 13: c += (R >> 4) * u0 ===
	// R >> 4 = 0x1000003D10 >> 4 = 0x1000003D1
	MOVQ 56(SP), AX      // u0
	MOVQ $0x1000003D1, CX
	MULQ CX              // (R >> 4) * u0
	ADDQ AX, 16(SP)      // c_lo
	ADCQ DX, 24(SP)      // c_hi

	// === Step 14: r[0] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 0(DI)       // store r[0]

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 15: c += a0*b1 + a1*b0 ===
	MOVQ R8, AX
	MULQ 8(BX)           // a0 * b1
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ R9, AX
	MULQ 0(BX)           // a1 * b0
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// === Step 16: d += a2*b4 + a3*b3 + a4*b2 ===
	MOVQ R10, AX
	MULQ 32(BX)          // a2 * b4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R11, AX
	MULQ 24(BX)          // a3 * b3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R12, AX
	MULQ 16(BX)          // a4 * b2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 17: c += R * (d & M); d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX          // d & M
	MOVQ $0x1000003D10, CX  // R
	MULQ CX              // R * (d & M)
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// d >>= 52
	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 18: r[1] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 8(DI)       // store r[1]

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 19: c += a0*b2 + a1*b1 + a2*b0 ===
	MOVQ R8, AX
	MULQ 16(BX)          // a0 * b2
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ R9, AX
	MULQ 8(BX)           // a1 * b1
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ R10, AX
	MULQ 0(BX)           // a2 * b0
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// === Step 20: d += a3*b4 + a4*b3 ===
	MOVQ R11, AX
	MULQ 32(BX)          // a3 * b4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R12, AX
	MULQ 24(BX)          // a4 * b3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 21: c += R * d_lo; d >>= 64 ===
	// Note: use full d_lo here, not d & M
	MOVQ 0(SP), AX       // d_lo
	MOVQ $0x1000003D10, CX  // R
	MULQ CX              // R * d_lo
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// d >>= 64 (just take d_hi)
	MOVQ 8(SP), AX
	MOVQ AX, 0(SP)
	MOVQ $0, 8(SP)

	// === Step 22: r[2] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 16(DI)      // store r[2]

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 23: c += (R << 12) * d + t3 ===
	MOVQ 0(SP), AX       // d (after d >>= 64)
	MOVQ $0x1000003D10000, CX  // R << 12
	MULQ CX              // (R << 12) * d
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ 32(SP), AX      // t3
	ADDQ AX, 16(SP)
	ADCQ $0, 24(SP)

	// === Step 24: r[3] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 24(DI)      // store r[3]

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX

	// === Step 25: r[4] = c + t4 ===
	ADDQ 40(SP), AX      // c + t4
	MOVQ AX, 32(DI)      // store r[4]

	RET

// func fieldSqrAsm(r, a *FieldElement)
// Squares a field element in 5x52 representation.
// This follows the bitcoin-core secp256k1_fe_sqr_inner algorithm.
// Squaring is optimized since a*a has symmetric terms: a[i]*a[j] appears twice.
TEXT ·fieldSqrAsm(SB), NOSPLIT, $96-16
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI

	// Load a[0..4] into registers
	MOVQ 0(SI), R8       // a0
	MOVQ 8(SI), R9       // a1
	MOVQ 16(SI), R10     // a2
	MOVQ 24(SI), R11     // a3
	MOVQ 32(SI), R12     // a4

	// === Step 1: d = 2*a0*a3 + 2*a1*a2 ===
	MOVQ R8, AX
	ADDQ AX, AX          // 2*a0
	MULQ R11             // 2*a0 * a3
	MOVQ AX, 0(SP)       // d_lo
	MOVQ DX, 8(SP)       // d_hi

	MOVQ R9, AX
	ADDQ AX, AX          // 2*a1
	MULQ R10             // 2*a1 * a2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 2: c = a4*a4 ===
	MOVQ R12, AX
	MULQ R12             // a4 * a4
	MOVQ AX, 16(SP)      // c_lo
	MOVQ DX, 24(SP)      // c_hi

	// === Step 3: d += R * c_lo ===
	// Note: use full c_lo (64 bits), NOT c_lo & M
	MOVQ 16(SP), AX      // c_lo (full 64 bits)
	MOVQ $0x1000003D10, CX
	MULQ CX
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 4: c >>= 64 ===
	MOVQ 24(SP), AX
	MOVQ AX, 16(SP)
	MOVQ $0, 24(SP)

	// === Step 5: t3 = d & M; d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 32(SP)      // t3

	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 6: d += 2*a0*a4 + 2*a1*a3 + a2*a2 ===
	// Pre-compute 2*a4 for later use
	MOVQ R12, CX
	ADDQ CX, CX          // 2*a4
	MOVQ CX, 64(SP)      // save 2*a4

	MOVQ R8, AX
	MULQ CX              // a0 * 2*a4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R9, AX
	ADDQ AX, AX          // 2*a1
	MULQ R11             // 2*a1 * a3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R10, AX
	MULQ R10             // a2 * a2
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 7: d += (R << 12) * c ===
	MOVQ 16(SP), AX
	MOVQ $0x1000003D10000, CX
	MULQ CX
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 8: t4 = d & M; tx = t4 >> 48; t4 &= (M >> 4) ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 40(SP)      // full t4

	SHRQ $48, AX
	MOVQ AX, 48(SP)      // tx

	MOVQ 40(SP), AX
	MOVQ $0x0FFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 40(SP)      // t4

	// === Step 9: d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 10: c = a0*a0 ===
	MOVQ R8, AX
	MULQ R8
	MOVQ AX, 16(SP)
	MOVQ DX, 24(SP)

	// === Step 11: d += a1*2*a4 + 2*a2*a3 ===
	MOVQ R9, AX
	MULQ 64(SP)          // a1 * 2*a4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R10, AX
	ADDQ AX, AX          // 2*a2
	MULQ R11             // 2*a2 * a3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 12: u0 = d & M; d >>= 52; u0 = (u0 << 4) | tx ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	SHLQ $4, AX
	ORQ 48(SP), AX
	MOVQ AX, 56(SP)      // u0

	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 13: c += (R >> 4) * u0 ===
	MOVQ 56(SP), AX
	MOVQ $0x1000003D1, CX
	MULQ CX
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// === Step 14: r[0] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 0(DI)

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 15: c += 2*a0*a1 ===
	MOVQ R8, AX
	ADDQ AX, AX
	MULQ R9
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// === Step 16: d += a2*2*a4 + a3*a3 ===
	MOVQ R10, AX
	MULQ 64(SP)          // a2 * 2*a4
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	MOVQ R11, AX
	MULQ R11             // a3 * a3
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 17: c += R * (d & M); d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ $0x1000003D10, CX
	MULQ CX
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ 0(SP), AX
	MOVQ 8(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	// === Step 18: r[1] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 8(DI)

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 19: c += 2*a0*a2 + a1*a1 ===
	MOVQ R8, AX
	ADDQ AX, AX
	MULQ R10
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ R9, AX
	MULQ R9
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	// === Step 20: d += a3*2*a4 ===
	MOVQ R11, AX
	MULQ 64(SP)
	ADDQ AX, 0(SP)
	ADCQ DX, 8(SP)

	// === Step 21: c += R * d_lo; d >>= 64 ===
	MOVQ 0(SP), AX
	MOVQ $0x1000003D10, CX
	MULQ CX
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ 8(SP), AX
	MOVQ AX, 0(SP)
	MOVQ $0, 8(SP)

	// === Step 22: r[2] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 16(DI)

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX
	SHRQ $52, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 23: c += (R << 12) * d + t3 ===
	MOVQ 0(SP), AX
	MOVQ $0x1000003D10000, CX
	MULQ CX
	ADDQ AX, 16(SP)
	ADCQ DX, 24(SP)

	MOVQ 32(SP), AX
	ADDQ AX, 16(SP)
	ADCQ $0, 24(SP)

	// === Step 24: r[3] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, CX
	ANDQ CX, AX
	MOVQ AX, 24(DI)

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX

	// === Step 25: r[4] = c + t4 ===
	ADDQ 40(SP), AX
	MOVQ AX, 32(DI)

	RET
