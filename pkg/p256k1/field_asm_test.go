//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"testing"
)

// fieldMulPureGo is the pure Go implementation for comparison
func fieldMulPureGo(r, a, b *FieldElement) {
	// Extract limbs for easier access
	a0, a1, a2, a3, a4 := a.n[0], a.n[1], a.n[2], a.n[3], a.n[4]
	b0, b1, b2, b3, b4 := b.n[0], b.n[1], b.n[2], b.n[3], b.n[4]

	const M = uint64(0xFFFFFFFFFFFFF)            // 2^52 - 1
	const R = uint64(fieldReductionConstantShifted) // 0x1000003D10

	// Following the C implementation algorithm exactly
	var c, d uint128
	d = mulU64ToU128(a0, b3)
	d = addMulU128(d, a1, b2)
	d = addMulU128(d, a2, b1)
	d = addMulU128(d, a3, b0)

	c = mulU64ToU128(a4, b4)

	d = addMulU128(d, R, c.lo())
	c = c.rshift(64)

	t3 := d.lo() & M
	d = d.rshift(52)

	d = addMulU128(d, a0, b4)
	d = addMulU128(d, a1, b3)
	d = addMulU128(d, a2, b2)
	d = addMulU128(d, a3, b1)
	d = addMulU128(d, a4, b0)

	d = addMulU128(d, R<<12, c.lo())

	t4 := d.lo() & M
	d = d.rshift(52)
	tx := t4 >> 48
	t4 &= (M >> 4)

	c = mulU64ToU128(a0, b0)

	d = addMulU128(d, a1, b4)
	d = addMulU128(d, a2, b3)
	d = addMulU128(d, a3, b2)
	d = addMulU128(d, a4, b1)

	u0 := d.lo() & M
	d = d.rshift(52)
	u0 = (u0 << 4) | tx

	c = addMulU128(c, u0, R>>4)

	r.n[0] = c.lo() & M
	c = c.rshift(52)

	c = addMulU128(c, a0, b1)
	c = addMulU128(c, a1, b0)

	d = addMulU128(d, a2, b4)
	d = addMulU128(d, a3, b3)
	d = addMulU128(d, a4, b2)

	c = addMulU128(c, R, d.lo()&M)
	d = d.rshift(52)

	r.n[1] = c.lo() & M
	c = c.rshift(52)

	c = addMulU128(c, a0, b2)
	c = addMulU128(c, a1, b1)
	c = addMulU128(c, a2, b0)

	d = addMulU128(d, a3, b4)
	d = addMulU128(d, a4, b3)

	c = addMulU128(c, R, d.lo())
	d = d.rshift(64)

	r.n[2] = c.lo() & M
	c = c.rshift(52)

	c = addMulU128(c, R<<12, d.lo())
	c = addU128(c, t3)

	r.n[3] = c.lo() & M
	c = c.rshift(52)

	r.n[4] = c.lo() + t4

	r.magnitude = 1
	r.normalized = false
}

func TestFieldMulAsmVsPureGo(t *testing.T) {
	// Test with simple values first
	a := FieldElement{n: [5]uint64{1, 0, 0, 0, 0}, magnitude: 1, normalized: true}
	b := FieldElement{n: [5]uint64{2, 0, 0, 0, 0}, magnitude: 1, normalized: true}

	var rAsm, rGo FieldElement

	// Pure Go
	fieldMulPureGo(&rGo, &a, &b)

	// Assembly
	if hasFieldAsm() {
		fieldMulAsm(&rAsm, &a, &b)
		rAsm.magnitude = 1
		rAsm.normalized = false

		t.Logf("a = %v", a.n)
		t.Logf("b = %v", b.n)
		t.Logf("Go result:  %v", rGo.n)
		t.Logf("Asm result: %v", rAsm.n)

		for i := 0; i < 5; i++ {
			if rAsm.n[i] != rGo.n[i] {
				t.Errorf("limb %d mismatch: asm=%x, go=%x", i, rAsm.n[i], rGo.n[i])
			}
		}
	} else {
		t.Skip("Assembly not available")
	}
}

func TestFieldMulAsmVsPureGoLarger(t *testing.T) {
	// Test with larger values
	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}
	b := FieldElement{
		n:          [5]uint64{0xabcdef1234567890 & 0xFFFFFFFFFFFFF, 0x9876543210fedcba & 0xFFFFFFFFFFFFF, 0xfedcba1234567890 & 0xFFFFFFFFFFFFF, 0x0987654321abcdef & 0xFFFFFFFFFFFFF, 0x0fedcba98765 & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rAsm, rGo FieldElement

	// Pure Go
	fieldMulPureGo(&rGo, &a, &b)

	// Assembly
	if hasFieldAsm() {
		fieldMulAsm(&rAsm, &a, &b)
		rAsm.magnitude = 1
		rAsm.normalized = false

		t.Logf("a = %v", a.n)
		t.Logf("b = %v", b.n)
		t.Logf("Go result:  %v", rGo.n)
		t.Logf("Asm result: %v", rAsm.n)

		for i := 0; i < 5; i++ {
			if rAsm.n[i] != rGo.n[i] {
				t.Errorf("limb %d mismatch: asm=%x, go=%x", i, rAsm.n[i], rGo.n[i])
			}
		}
	} else {
		t.Skip("Assembly not available")
	}
}

func TestFieldSqrAsmVsPureGo(t *testing.T) {
	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rAsm, rGo FieldElement

	// Pure Go (a * a)
	fieldMulPureGo(&rGo, &a, &a)

	// Assembly
	if hasFieldAsm() {
		fieldSqrAsm(&rAsm, &a)
		rAsm.magnitude = 1
		rAsm.normalized = false

		t.Logf("a = %v", a.n)
		t.Logf("Go result:  %v", rGo.n)
		t.Logf("Asm result: %v", rAsm.n)

		for i := 0; i < 5; i++ {
			if rAsm.n[i] != rGo.n[i] {
				t.Errorf("limb %d mismatch: asm=%x, go=%x", i, rAsm.n[i], rGo.n[i])
			}
		}
	} else {
		t.Skip("Assembly not available")
	}
}

// BMI2 tests

func TestFieldMulAsmBMI2VsPureGo(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}

	// Test with simple values first
	a := FieldElement{n: [5]uint64{1, 0, 0, 0, 0}, magnitude: 1, normalized: true}
	b := FieldElement{n: [5]uint64{2, 0, 0, 0, 0}, magnitude: 1, normalized: true}

	var rBMI2, rGo FieldElement

	// Pure Go
	fieldMulPureGo(&rGo, &a, &b)

	// BMI2 Assembly
	fieldMulAsmBMI2(&rBMI2, &a, &b)
	rBMI2.magnitude = 1
	rBMI2.normalized = false

	t.Logf("a = %v", a.n)
	t.Logf("b = %v", b.n)
	t.Logf("Go result:   %v", rGo.n)
	t.Logf("BMI2 result: %v", rBMI2.n)

	for i := 0; i < 5; i++ {
		if rBMI2.n[i] != rGo.n[i] {
			t.Errorf("limb %d mismatch: bmi2=%x, go=%x", i, rBMI2.n[i], rGo.n[i])
		}
	}
}

func TestFieldMulAsmBMI2VsPureGoLarger(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}

	// Test with larger values
	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}
	b := FieldElement{
		n:          [5]uint64{0xabcdef1234567890 & 0xFFFFFFFFFFFFF, 0x9876543210fedcba & 0xFFFFFFFFFFFFF, 0xfedcba1234567890 & 0xFFFFFFFFFFFFF, 0x0987654321abcdef & 0xFFFFFFFFFFFFF, 0x0fedcba98765 & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rBMI2, rGo FieldElement

	// Pure Go
	fieldMulPureGo(&rGo, &a, &b)

	// BMI2 Assembly
	fieldMulAsmBMI2(&rBMI2, &a, &b)
	rBMI2.magnitude = 1
	rBMI2.normalized = false

	t.Logf("a = %v", a.n)
	t.Logf("b = %v", b.n)
	t.Logf("Go result:   %v", rGo.n)
	t.Logf("BMI2 result: %v", rBMI2.n)

	for i := 0; i < 5; i++ {
		if rBMI2.n[i] != rGo.n[i] {
			t.Errorf("limb %d mismatch: bmi2=%x, go=%x", i, rBMI2.n[i], rGo.n[i])
		}
	}
}

func TestFieldMulAsmBMI2VsRegularAsm(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}
	if !hasFieldAsm() {
		t.Skip("Regular assembly not available")
	}

	// Test with larger values
	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}
	b := FieldElement{
		n:          [5]uint64{0xabcdef1234567890 & 0xFFFFFFFFFFFFF, 0x9876543210fedcba & 0xFFFFFFFFFFFFF, 0xfedcba1234567890 & 0xFFFFFFFFFFFFF, 0x0987654321abcdef & 0xFFFFFFFFFFFFF, 0x0fedcba98765 & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rBMI2, rAsm FieldElement

	// Regular Assembly
	fieldMulAsm(&rAsm, &a, &b)
	rAsm.magnitude = 1
	rAsm.normalized = false

	// BMI2 Assembly
	fieldMulAsmBMI2(&rBMI2, &a, &b)
	rBMI2.magnitude = 1
	rBMI2.normalized = false

	t.Logf("a = %v", a.n)
	t.Logf("b = %v", b.n)
	t.Logf("Asm result:  %v", rAsm.n)
	t.Logf("BMI2 result: %v", rBMI2.n)

	for i := 0; i < 5; i++ {
		if rBMI2.n[i] != rAsm.n[i] {
			t.Errorf("limb %d mismatch: bmi2=%x, asm=%x", i, rBMI2.n[i], rAsm.n[i])
		}
	}
}

func TestFieldSqrAsmBMI2VsPureGo(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}

	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rBMI2, rGo FieldElement

	// Pure Go (a * a)
	fieldMulPureGo(&rGo, &a, &a)

	// BMI2 Assembly
	fieldSqrAsmBMI2(&rBMI2, &a)
	rBMI2.magnitude = 1
	rBMI2.normalized = false

	t.Logf("a = %v", a.n)
	t.Logf("Go result:   %v", rGo.n)
	t.Logf("BMI2 result: %v", rBMI2.n)

	for i := 0; i < 5; i++ {
		if rBMI2.n[i] != rGo.n[i] {
			t.Errorf("limb %d mismatch: bmi2=%x, go=%x", i, rBMI2.n[i], rGo.n[i])
		}
	}
}

func TestFieldSqrAsmBMI2VsRegularAsm(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}
	if !hasFieldAsm() {
		t.Skip("Regular assembly not available")
	}

	a := FieldElement{
		n:          [5]uint64{0x1234567890abcdef & 0xFFFFFFFFFFFFF, 0xfedcba9876543210 & 0xFFFFFFFFFFFFF, 0x0123456789abcdef & 0xFFFFFFFFFFFFF, 0xfedcba0987654321 & 0xFFFFFFFFFFFFF, 0x0123456789ab & 0x0FFFFFFFFFFFF},
		magnitude:  1,
		normalized: true,
	}

	var rBMI2, rAsm FieldElement

	// Regular Assembly
	fieldSqrAsm(&rAsm, &a)
	rAsm.magnitude = 1
	rAsm.normalized = false

	// BMI2 Assembly
	fieldSqrAsmBMI2(&rBMI2, &a)
	rBMI2.magnitude = 1
	rBMI2.normalized = false

	t.Logf("a = %v", a.n)
	t.Logf("Asm result:  %v", rAsm.n)
	t.Logf("BMI2 result: %v", rBMI2.n)

	for i := 0; i < 5; i++ {
		if rBMI2.n[i] != rAsm.n[i] {
			t.Errorf("limb %d mismatch: bmi2=%x, asm=%x", i, rBMI2.n[i], rAsm.n[i])
		}
	}
}

// TestFieldMulAsmBMI2Random tests with many random values
func TestFieldMulAsmBMI2Random(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}
	if !hasFieldAsm() {
		t.Skip("Regular assembly not available")
	}

	// Test with many random values
	for iter := 0; iter < 10000; iter++ {
		var a, b FieldElement
		a.magnitude = 1
		a.normalized = true
		b.magnitude = 1
		b.normalized = true

		// Generate deterministic but varied test data
		seed := uint64(iter * 12345678901234567)
		for j := 0; j < 5; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407 // LCG
			a.n[j] = seed & 0xFFFFFFFFFFFFF

			seed = seed*6364136223846793005 + 1442695040888963407
			b.n[j] = seed & 0xFFFFFFFFFFFFF
		}
		// Limb 4 is only 48 bits
		a.n[4] &= 0x0FFFFFFFFFFFF
		b.n[4] &= 0x0FFFFFFFFFFFF

		var rAsm, rBMI2 FieldElement

		// Regular Assembly
		fieldMulAsm(&rAsm, &a, &b)
		rAsm.magnitude = 1
		rAsm.normalized = false

		// BMI2 Assembly
		fieldMulAsmBMI2(&rBMI2, &a, &b)
		rBMI2.magnitude = 1
		rBMI2.normalized = false

		// Compare results
		for j := 0; j < 5; j++ {
			if rAsm.n[j] != rBMI2.n[j] {
				t.Errorf("Iteration %d: limb %d mismatch", iter, j)
				t.Errorf("  a = %v", a.n)
				t.Errorf("  b = %v", b.n)
				t.Errorf("  Asm:  %v", rAsm.n)
				t.Errorf("  BMI2: %v", rBMI2.n)
				return
			}
		}
	}
}

// TestFieldSqrAsmBMI2Random tests squaring with many random values
func TestFieldSqrAsmBMI2Random(t *testing.T) {
	if !hasFieldAsmBMI2() {
		t.Skip("BMI2+ADX assembly not available")
	}
	if !hasFieldAsm() {
		t.Skip("Regular assembly not available")
	}

	// Test with many random values
	for iter := 0; iter < 10000; iter++ {
		var a FieldElement
		a.magnitude = 1
		a.normalized = true

		// Generate deterministic but varied test data
		seed := uint64(iter * 98765432109876543)
		for j := 0; j < 5; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407 // LCG
			a.n[j] = seed & 0xFFFFFFFFFFFFF
		}
		// Limb 4 is only 48 bits
		a.n[4] &= 0x0FFFFFFFFFFFF

		var rAsm, rBMI2 FieldElement

		// Regular Assembly
		fieldSqrAsm(&rAsm, &a)
		rAsm.magnitude = 1
		rAsm.normalized = false

		// BMI2 Assembly
		fieldSqrAsmBMI2(&rBMI2, &a)
		rBMI2.magnitude = 1
		rBMI2.normalized = false

		// Compare results
		for j := 0; j < 5; j++ {
			if rAsm.n[j] != rBMI2.n[j] {
				t.Errorf("Iteration %d: limb %d mismatch", iter, j)
				t.Errorf("  a = %v", a.n)
				t.Errorf("  Asm:  %v", rAsm.n)
				t.Errorf("  BMI2: %v", rBMI2.n)
				return
			}
		}
	}
}
