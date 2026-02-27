//go:build amd64 && !purego

package p256k1

// field4x64Mul uses the faster 4x64 representation for field multiplication.
// Converts 5x52 inputs to 4x64, multiplies, and converts back.
func field4x64Mul(r, a, b *FieldElement) {
	// Convert 5x52 to 4x64
	var a4, b4, r4 Field4x64

	// Normalize inputs first - 5x52 with magnitude > 1 has limbs that may exceed 52 bits
	var aNorm, bNorm FieldElement
	aNorm = *a
	bNorm = *b
	aNorm.normalizeWeak()
	bNorm.normalizeWeak()

	// Pack 5x52 limbs into 4x64
	// 5x52: n[0..4] where each is 52 bits (except n[4] which is 48 bits)
	// 4x64: n[0..3] where each is 64 bits
	a4.n[0] = aNorm.n[0] | (aNorm.n[1] << 52)
	a4.n[1] = (aNorm.n[1] >> 12) | (aNorm.n[2] << 40)
	a4.n[2] = (aNorm.n[2] >> 24) | (aNorm.n[3] << 28)
	a4.n[3] = (aNorm.n[3] >> 36) | (aNorm.n[4] << 16)
	a4.magnitude = 1
	a4.normalized = false

	b4.n[0] = bNorm.n[0] | (bNorm.n[1] << 52)
	b4.n[1] = (bNorm.n[1] >> 12) | (bNorm.n[2] << 40)
	b4.n[2] = (bNorm.n[2] >> 24) | (bNorm.n[3] << 28)
	b4.n[3] = (bNorm.n[3] >> 36) | (bNorm.n[4] << 16)
	b4.magnitude = 1
	b4.normalized = false

	// Multiply using BMI2 assembly
	r4.Mul(&a4, &b4)

	// Convert 4x64 back to 5x52
	r.n[0] = r4.n[0] & 0xFFFFFFFFFFFFF
	r.n[1] = ((r4.n[0] >> 52) | (r4.n[1] << 12)) & 0xFFFFFFFFFFFFF
	r.n[2] = ((r4.n[1] >> 40) | (r4.n[2] << 24)) & 0xFFFFFFFFFFFFF
	r.n[3] = ((r4.n[2] >> 28) | (r4.n[3] << 36)) & 0xFFFFFFFFFFFFF
	r.n[4] = (r4.n[3] >> 16) & 0x0FFFFFFFFFFFF

	r.magnitude = 1
	r.normalized = false
}

// field4x64Sqr uses the faster 4x64 representation for field squaring.
func field4x64Sqr(r, a *FieldElement) {
	// Convert 5x52 to 4x64
	var a4, r4 Field4x64

	// Normalize input first
	var aNorm FieldElement
	aNorm = *a
	aNorm.normalizeWeak()

	a4.n[0] = aNorm.n[0] | (aNorm.n[1] << 52)
	a4.n[1] = (aNorm.n[1] >> 12) | (aNorm.n[2] << 40)
	a4.n[2] = (aNorm.n[2] >> 24) | (aNorm.n[3] << 28)
	a4.n[3] = (aNorm.n[3] >> 36) | (aNorm.n[4] << 16)
	a4.magnitude = 1
	a4.normalized = false

	// Square using BMI2 assembly
	r4.Sqr(&a4)

	// Convert 4x64 back to 5x52
	r.n[0] = r4.n[0] & 0xFFFFFFFFFFFFF
	r.n[1] = ((r4.n[0] >> 52) | (r4.n[1] << 12)) & 0xFFFFFFFFFFFFF
	r.n[2] = ((r4.n[1] >> 40) | (r4.n[2] << 24)) & 0xFFFFFFFFFFFFF
	r.n[3] = ((r4.n[2] >> 28) | (r4.n[3] << 36)) & 0xFFFFFFFFFFFFF
	r.n[4] = (r4.n[3] >> 16) & 0x0FFFFFFFFFFFF

	r.magnitude = 1
	r.normalized = false
}

// hasField4x64 returns false - the bridge has conversion issues.
// The existing fieldMulAsmBMI2/fieldSqrAsmBMI2 work correctly for 5x52.
// Field4x64 can be used directly for new code that doesn't need 5x52 compatibility.
func hasField4x64() bool {
	return false
}
