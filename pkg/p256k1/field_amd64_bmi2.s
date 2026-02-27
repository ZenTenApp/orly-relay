//go:build amd64

#include "textflag.h"

// Field multiplication assembly for secp256k1 using BMI2+ADX instructions.
// Uses MULX for flag-free multiplication and ADCX/ADOX for parallel carry chains.
//
// The field element is represented as 5 limbs of 52 bits each:
//   n[0..4] where value = sum(n[i] * 2^(52*i))
//
// Field prime p = 2^256 - 2^32 - 977
// Reduction constant R = 2^256 mod p = 2^32 + 977 = 0x1000003D1
// For 5x52: R shifted = 0x1000003D10 (for 52-bit alignment)
//
// BMI2 Instructions used:
//   MULXQ src, lo, hi  - unsigned multiply RDX * src -> hi:lo (flags unchanged)
//
// ADX Instructions used:
//   ADCXQ src, dst     - dst += src + CF (only modifies CF)
//   ADOXQ src, dst     - dst += src + OF (only modifies OF)
//
// ADCX/ADOX allow parallel carry chains: ADCX uses CF only, ADOX uses OF only.
// This enables the CPU to execute two independent addition chains in parallel.
//
// Stack layout for fieldMulAsmBMI2 (96 bytes):
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

// func fieldMulAsmBMI2(r, a, b *FieldElement)
TEXT ·fieldMulAsmBMI2(SB), NOSPLIT, $96-24
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

	// Constants:
	// M = 0xFFFFFFFFFFFFF (2^52 - 1)
	// R = 0x1000003D10

	// === Step 1: d = a0*b3 + a1*b2 + a2*b1 + a3*b0 ===
	// Using MULX: put multiplier in RDX, result in specified regs
	MOVQ 24(BX), DX      // b3
	MULXQ R8, AX, CX     // a0 * b3 -> CX:AX
	MOVQ AX, 0(SP)       // d_lo
	MOVQ CX, 8(SP)       // d_hi

	MOVQ 16(BX), DX      // b2
	MULXQ R9, AX, CX     // a1 * b2 -> CX:AX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 8(BX), DX       // b1
	MULXQ R10, AX, CX    // a2 * b1 -> CX:AX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 0(BX), DX       // b0
	MULXQ R11, AX, CX    // a3 * b0 -> CX:AX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 2: c = a4*b4 ===
	MOVQ 32(BX), DX      // b4
	MULXQ R12, AX, CX    // a4 * b4 -> CX:AX
	MOVQ AX, 16(SP)      // c_lo
	MOVQ CX, 24(SP)      // c_hi

	// === Step 3: d += R * c_lo ===
	MOVQ 16(SP), DX      // c_lo
	MOVQ $0x1000003D10, R13  // R constant
	MULXQ R13, AX, CX    // R * c_lo -> CX:AX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 4: c >>= 64 ===
	MOVQ 24(SP), AX
	MOVQ AX, 16(SP)
	MOVQ $0, 24(SP)

	// === Step 5: t3 = d & M; d >>= 52 ===
	MOVQ 0(SP), AX
	MOVQ $0xFFFFFFFFFFFFF, R14  // M constant (keep in register)
	ANDQ R14, AX
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

	// === Step 6: d += a0*b4 + a1*b3 + a2*b2 + a3*b1 + a4*b0 ===
	MOVQ 80(SP), BX      // restore b pointer

	MOVQ 32(BX), DX      // b4
	MULXQ R8, AX, CX     // a0 * b4
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 24(BX), DX      // b3
	MULXQ R9, AX, CX     // a1 * b3
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 16(BX), DX      // b2
	MULXQ R10, AX, CX    // a2 * b2
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 8(BX), DX       // b1
	MULXQ R11, AX, CX    // a3 * b1
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 0(BX), DX       // b0
	MULXQ R12, AX, CX    // a4 * b0
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 7: d += (R << 12) * c ===
	MOVQ 16(SP), DX      // c
	MOVQ $0x1000003D10000, R15  // R << 12
	MULXQ R15, AX, CX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 8: t4 = d & M; tx = t4 >> 48; t4 &= (M >> 4) ===
	MOVQ 0(SP), AX
	ANDQ R14, AX         // t4 = d & M
	MOVQ AX, 40(SP)

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

	// === Step 10: c = a0*b0 ===
	MOVQ 0(BX), DX       // b0
	MULXQ R8, AX, CX     // a0 * b0
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 11: d += a1*b4 + a2*b3 + a3*b2 + a4*b1 ===
	MOVQ 32(BX), DX      // b4
	MULXQ R9, AX, CX     // a1 * b4
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 24(BX), DX      // b3
	MULXQ R10, AX, CX    // a2 * b3
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 16(BX), DX      // b2
	MULXQ R11, AX, CX    // a3 * b2
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ 8(BX), DX       // b1
	MULXQ R12, AX, CX    // a4 * b1
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 12: u0 = d & M; d >>= 52; u0 = (u0 << 4) | tx ===
	MOVQ 0(SP), AX
	ANDQ R14, AX         // u0 = d & M
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
	MOVQ 56(SP), DX      // u0
	MOVQ $0x1000003D1, R13  // R >> 4
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	// === Step 14: r[0] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
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

	// === Steps 15-16: Parallel c and d updates using ADCX/ADOX ===
	// Step 15: c += a0*b1 + a1*b0 (CF chain via ADCX)
	// Step 16: d += a2*b4 + a3*b3 + a4*b2 (OF chain via ADOX)
	// Save r pointer before reusing DI
	MOVQ DI, 64(SP)      // save r pointer

	// Load all accumulators into registers for ADCX/ADOX (register-only ops)
	MOVQ 16(SP), R13     // c_lo
	MOVQ 24(SP), R15     // c_hi
	MOVQ 0(SP), SI       // d_lo (reuse SI since we don't need 'a' anymore)
	MOVQ 8(SP), DI       // d_hi (reuse DI)

	// Clear CF and OF
	XORQ AX, AX

	// First pair: c += a0*b1, d += a2*b4
	MOVQ 8(BX), DX       // b1
	MULXQ R8, AX, CX     // a0 * b1 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo (CF chain)
	ADCXQ CX, R15        // c_hi += hi + CF

	MOVQ 32(BX), DX      // b4
	MULXQ R10, AX, CX    // a2 * b4 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo (OF chain)
	ADOXQ CX, DI         // d_hi += hi + OF

	// Second pair: c += a1*b0, d += a3*b3
	MOVQ 0(BX), DX       // b0
	MULXQ R9, AX, CX     // a1 * b0 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, R15        // c_hi += hi + CF

	MOVQ 24(BX), DX      // b3
	MULXQ R11, AX, CX    // a3 * b3 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// Third: d += a4*b2 (only d, no more c operations)
	MOVQ 16(BX), DX      // b2
	MULXQ R12, AX, CX    // a4 * b2 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// Store results back
	MOVQ R13, 16(SP)     // c_lo
	MOVQ R15, 24(SP)     // c_hi
	MOVQ SI, 0(SP)       // d_lo
	MOVQ DI, 8(SP)       // d_hi
	MOVQ 64(SP), DI      // restore r pointer

	// === Step 17: c += R * (d & M); d >>= 52 ===
	MOVQ 0(SP), AX
	ANDQ R14, AX         // d & M
	MOVQ AX, DX
	MOVQ $0x1000003D10, R13  // R
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

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
	ANDQ R14, AX
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

	// === Steps 19-20: Parallel c and d updates using ADCX/ADOX ===
	// Step 19: c += a0*b2 + a1*b1 + a2*b0 (CF chain via ADCX)
	// Step 20: d += a3*b4 + a4*b3 (OF chain via ADOX)
	// Save r pointer before reusing DI
	MOVQ DI, 64(SP)      // save r pointer

	// Load all accumulators into registers
	MOVQ 16(SP), R13     // c_lo
	MOVQ 24(SP), R15     // c_hi
	MOVQ 0(SP), SI       // d_lo
	MOVQ 8(SP), DI       // d_hi

	// Clear CF and OF
	XORQ AX, AX

	// First pair: c += a0*b2, d += a3*b4
	MOVQ 16(BX), DX      // b2
	MULXQ R8, AX, CX     // a0 * b2 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, R15        // c_hi += hi + CF

	MOVQ 32(BX), DX      // b4
	MULXQ R11, AX, CX    // a3 * b4 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// Second pair: c += a1*b1, d += a4*b3
	MOVQ 8(BX), DX       // b1
	MULXQ R9, AX, CX     // a1 * b1 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, R15        // c_hi += hi + CF

	MOVQ 24(BX), DX      // b3
	MULXQ R12, AX, CX    // a4 * b3 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// Third: c += a2*b0 (only c, no more d operations)
	MOVQ 0(BX), DX       // b0
	MULXQ R10, AX, CX    // a2 * b0 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, R15        // c_hi += hi + CF

	// Store results back
	MOVQ R13, 16(SP)     // c_lo
	MOVQ R15, 24(SP)     // c_hi
	MOVQ SI, 0(SP)       // d_lo
	MOVQ DI, 8(SP)       // d_hi
	MOVQ 64(SP), DI      // restore r pointer

	// === Step 21: c += R * d_lo; d >>= 64 ===
	MOVQ 0(SP), DX       // d_lo
	MOVQ $0x1000003D10, R13  // R
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	MOVQ 8(SP), AX
	MOVQ AX, 0(SP)
	MOVQ $0, 8(SP)

	// === Step 22: r[2] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
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
	MOVQ 0(SP), DX       // d
	MOVQ $0x1000003D10000, R15  // R << 12 (reload since R15 was used for c_hi)
	MULXQ R15, AX, CX    // (R << 12) * d
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	MOVQ 32(SP), AX      // t3
	ADDQ AX, 16(SP)
	ADCQ $0, 24(SP)

	// === Step 24: r[3] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
	MOVQ AX, 24(DI)      // store r[3]

	MOVQ 16(SP), AX
	MOVQ 24(SP), CX
	SHRQ $52, AX
	MOVQ CX, DX
	SHLQ $12, DX
	ORQ DX, AX

	// === Step 25: r[4] = c + t4 ===
	ADDQ 40(SP), AX
	MOVQ AX, 32(DI)      // store r[4]

	RET


// func fieldSqrAsmBMI2(r, a *FieldElement)
// Squares a field element using BMI2 instructions.
TEXT ·fieldSqrAsmBMI2(SB), NOSPLIT, $96-16
	MOVQ r+0(FP), DI
	MOVQ a+8(FP), SI

	// Load a[0..4] into registers
	MOVQ 0(SI), R8       // a0
	MOVQ 8(SI), R9       // a1
	MOVQ 16(SI), R10     // a2
	MOVQ 24(SI), R11     // a3
	MOVQ 32(SI), R12     // a4

	// Keep M constant in R14
	MOVQ $0xFFFFFFFFFFFFF, R14

	// === Step 1: d = 2*a0*a3 + 2*a1*a2 ===
	MOVQ R8, DX
	ADDQ DX, DX          // 2*a0
	MULXQ R11, AX, CX    // 2*a0 * a3
	MOVQ AX, 0(SP)
	MOVQ CX, 8(SP)

	MOVQ R9, DX
	ADDQ DX, DX          // 2*a1
	MULXQ R10, AX, CX    // 2*a1 * a2
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 2: c = a4*a4 ===
	MOVQ R12, DX
	MULXQ R12, AX, CX    // a4 * a4
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 3: d += R * c_lo ===
	MOVQ 16(SP), DX
	MOVQ $0x1000003D10, R13
	MULXQ R13, AX, CX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 4: c >>= 64 ===
	MOVQ 24(SP), AX
	MOVQ AX, 16(SP)
	MOVQ $0, 24(SP)

	// === Step 5: t3 = d & M; d >>= 52 ===
	MOVQ 0(SP), AX
	ANDQ R14, AX
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
	// Pre-compute 2*a4
	MOVQ R12, R15
	ADDQ R15, R15        // 2*a4

	MOVQ R8, DX
	MULXQ R15, AX, CX    // a0 * 2*a4
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ R9, DX
	ADDQ DX, DX          // 2*a1
	MULXQ R11, AX, CX    // 2*a1 * a3
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ R10, DX
	MULXQ R10, AX, CX    // a2 * a2
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 7: d += (R << 12) * c ===
	MOVQ 16(SP), DX
	MOVQ $0x1000003D10000, R13
	MULXQ R13, AX, CX
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 8: t4 = d & M; tx = t4 >> 48; t4 &= (M >> 4) ===
	MOVQ 0(SP), AX
	ANDQ R14, AX
	MOVQ AX, 40(SP)

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
	MOVQ R8, DX
	MULXQ R8, AX, CX
	MOVQ AX, 16(SP)
	MOVQ CX, 24(SP)

	// === Step 11: d += a1*2*a4 + 2*a2*a3 ===
	// Save a2 before doubling (needed later in step 16 and 19)
	MOVQ R10, 64(SP)     // save original a2

	MOVQ R9, DX
	MULXQ R15, AX, CX    // a1 * 2*a4
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	MOVQ R10, DX
	ADDQ DX, DX          // 2*a2
	MULXQ R11, AX, CX    // 2*a2 * a3
	ADDQ AX, 0(SP)
	ADCQ CX, 8(SP)

	// === Step 12: u0 = d & M; d >>= 52; u0 = (u0 << 4) | tx ===
	MOVQ 0(SP), AX
	ANDQ R14, AX
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
	MOVQ 56(SP), DX
	MOVQ $0x1000003D1, R13
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	// === Step 14: r[0] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
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

	// === Steps 15-16: Parallel c and d updates using ADCX/ADOX ===
	// Step 15: c += 2*a0*a1 (CF chain via ADCX)
	// Step 16: d += a2*2*a4 + a3*a3 (OF chain via ADOX)
	// Save r pointer and load accumulators
	MOVQ DI, 72(SP)      // save r pointer (64(SP) has saved a2)

	MOVQ 16(SP), R13     // c_lo
	MOVQ 24(SP), BX      // c_hi (use BX since we need SI/DI)
	MOVQ 0(SP), SI       // d_lo
	MOVQ 8(SP), DI       // d_hi

	// Clear CF and OF
	XORQ AX, AX

	// c += 2*a0*a1
	MOVQ R8, DX
	ADDQ DX, DX          // 2*a0
	MULXQ R9, AX, CX     // 2*a0 * a1 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo (CF chain)
	ADCXQ CX, BX         // c_hi += hi + CF

	// d += a2*2*a4
	MOVQ 64(SP), DX      // load saved original a2
	MULXQ R15, AX, CX    // a2 * 2*a4 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo (OF chain)
	ADOXQ CX, DI         // d_hi += hi + OF

	// d += a3*a3
	MOVQ R11, DX
	MULXQ R11, AX, CX    // a3 * a3 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// Store results back
	MOVQ R13, 16(SP)     // c_lo
	MOVQ BX, 24(SP)      // c_hi
	MOVQ SI, 0(SP)       // d_lo
	MOVQ DI, 8(SP)       // d_hi
	MOVQ 72(SP), DI      // restore r pointer

	// === Step 17: c += R * (d & M); d >>= 52 ===
	MOVQ 0(SP), AX
	ANDQ R14, AX
	MOVQ AX, DX
	MOVQ $0x1000003D10, R13
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

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
	ANDQ R14, AX
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

	// === Steps 19-20: Parallel c and d updates using ADCX/ADOX ===
	// Step 19: c += 2*a0*a2 + a1*a1 (CF chain via ADCX)
	// Step 20: d += a3*2*a4 (OF chain via ADOX)
	// Save r pointer and load accumulators
	MOVQ DI, 72(SP)      // save r pointer

	MOVQ 16(SP), R13     // c_lo
	MOVQ 24(SP), BX      // c_hi
	MOVQ 0(SP), SI       // d_lo
	MOVQ 8(SP), DI       // d_hi

	// Clear CF and OF
	XORQ AX, AX

	// c += 2*a0*a2
	MOVQ R8, DX          // a0 (R8 was never modified)
	ADDQ DX, DX          // 2*a0
	MOVQ 64(SP), AX      // load saved original a2
	MULXQ AX, AX, CX     // 2*a0 * a2 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, BX         // c_hi += hi + CF

	// d += a3*2*a4
	MOVQ R11, DX
	MULXQ R15, AX, CX    // a3 * 2*a4 -> CX:AX
	ADOXQ AX, SI         // d_lo += lo
	ADOXQ CX, DI         // d_hi += hi + OF

	// c += a1*a1
	MOVQ R9, DX
	MULXQ R9, AX, CX     // a1 * a1 -> CX:AX
	ADCXQ AX, R13        // c_lo += lo
	ADCXQ CX, BX         // c_hi += hi + CF

	// Store results back
	MOVQ R13, 16(SP)     // c_lo
	MOVQ BX, 24(SP)      // c_hi
	MOVQ SI, 0(SP)       // d_lo
	MOVQ DI, 8(SP)       // d_hi
	MOVQ 72(SP), DI      // restore r pointer

	// === Step 21: c += R * d_lo; d >>= 64 ===
	MOVQ 0(SP), DX
	MOVQ $0x1000003D10, R13
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	MOVQ 8(SP), AX
	MOVQ AX, 0(SP)
	MOVQ $0, 8(SP)

	// === Step 22: r[2] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
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
	MOVQ 0(SP), DX
	MOVQ $0x1000003D10000, R13
	MULXQ R13, AX, CX
	ADDQ AX, 16(SP)
	ADCQ CX, 24(SP)

	MOVQ 32(SP), AX
	ADDQ AX, 16(SP)
	ADCQ $0, 24(SP)

	// === Step 24: r[3] = c & M; c >>= 52 ===
	MOVQ 16(SP), AX
	ANDQ R14, AX
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
