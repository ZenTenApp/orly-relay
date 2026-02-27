//go:build amd64

package p256k1

// fieldMulAsm multiplies two field elements using x86-64 assembly.
// This is a direct port of bitcoin-core secp256k1_fe_mul_inner.
// r, a, b are 5x52-bit limb representations.
//
//go:noescape
func fieldMulAsm(r, a, b *FieldElement)

// fieldSqrAsm squares a field element using x86-64 assembly.
// This is a direct port of bitcoin-core secp256k1_fe_sqr_inner.
// Squaring is optimized compared to multiplication.
//
//go:noescape
func fieldSqrAsm(r, a *FieldElement)

// fieldMulAsmBMI2 multiplies two field elements using BMI2+ADX instructions.
// Uses MULX for flag-free multiplication enabling parallel carry chains.
// r, a, b are 5x52-bit limb representations.
//
//go:noescape
func fieldMulAsmBMI2(r, a, b *FieldElement)

// fieldSqrAsmBMI2 squares a field element using BMI2+ADX instructions.
// Uses MULX for flag-free multiplication.
//
//go:noescape
func fieldSqrAsmBMI2(r, a *FieldElement)

// hasFieldAsm returns true if field assembly is available.
// On amd64, this is always true.
func hasFieldAsm() bool {
	return true
}

// hasFieldAsmBMI2 returns true if BMI2+ADX optimized field assembly is available.
func hasFieldAsmBMI2() bool {
	return HasBMI2()
}
