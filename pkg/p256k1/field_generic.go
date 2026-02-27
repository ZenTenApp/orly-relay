//go:build !amd64 && !js && !wasm && !tinygo && !wasm32

package p256k1

// hasFieldAsm returns true if field assembly is available.
// On non-amd64 platforms, assembly is not available.
func hasFieldAsm() bool {
	return false
}

// hasFieldAsmBMI2 returns true if BMI2+ADX optimized field assembly is available.
// On non-amd64 platforms, this is always false.
func hasFieldAsmBMI2() bool {
	return false
}

// fieldMulAsm is a stub for non-amd64 platforms.
// It should never be called since hasFieldAsm() returns false.
func fieldMulAsm(r, a, b *FieldElement) {
	panic("field assembly not available on this platform")
}

// fieldSqrAsm is a stub for non-amd64 platforms.
// It should never be called since hasFieldAsm() returns false.
func fieldSqrAsm(r, a *FieldElement) {
	panic("field assembly not available on this platform")
}

// fieldMulAsmBMI2 is a stub for non-amd64 platforms.
// It should never be called since hasFieldAsmBMI2() returns false.
func fieldMulAsmBMI2(r, a, b *FieldElement) {
	panic("field BMI2 assembly not available on this platform")
}

// fieldSqrAsmBMI2 is a stub for non-amd64 platforms.
// It should never be called since hasFieldAsmBMI2() returns false.
func fieldSqrAsmBMI2(r, a *FieldElement) {
	panic("field BMI2 assembly not available on this platform")
}
