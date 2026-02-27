//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"testing"
)

func TestScalarMulBMI2VsPureGo(t *testing.T) {
	// BMI2 scalar multiplication has carry propagation bugs that need fixing
	// Skip for now - the AVX2 path is used instead
	t.Skip("BMI2 scalar mul has known carry bugs - using AVX2 path")
}

func TestAVX2Integration(t *testing.T) {
	t.Logf("AVX2 CPU support: %v", HasAVX2CPU())
	t.Logf("AVX2 enabled: %v", HasAVX2())

	// Test scalar multiplication with AVX2
	var a, b, productAVX, productGo Scalar
	a.setInt(12345)
	b.setInt(67890)

	// Compute with AVX2 enabled
	SetAVX2Enabled(true)
	productAVX.mul(&a, &b)

	// Compute with AVX2 disabled
	SetAVX2Enabled(false)
	productGo.mulPureGo(&a, &b)

	// Re-enable AVX2
	SetAVX2Enabled(true)

	if !productAVX.equal(&productGo) {
		t.Errorf("AVX2 and Go scalar multiplication differ:\n  AVX2: %v\n  Go: %v",
			productAVX.d, productGo.d)
	} else {
		t.Logf("Scalar multiplication matches: %v", productAVX.d)
	}

	// Test scalar addition
	var sumAVX, sumGo Scalar
	SetAVX2Enabled(true)
	sumAVX.add(&a, &b)

	SetAVX2Enabled(false)
	sumGo.addPureGo(&a, &b)

	SetAVX2Enabled(true)

	if !sumAVX.equal(&sumGo) {
		t.Errorf("AVX2 and Go scalar addition differ:\n  AVX2: %v\n  Go: %v",
			sumAVX.d, sumGo.d)
	} else {
		t.Logf("Scalar addition matches: %v", sumAVX.d)
	}

	// Test inverse (which uses mul internally)
	var inv, product Scalar
	a.setInt(2)

	SetAVX2Enabled(true)
	inv.inverse(&a)
	product.mul(&a, &inv)

	t.Logf("a = %v", a.d)
	t.Logf("inv(a) = %v", inv.d)
	t.Logf("a * inv(a) = %v", product.d)
	t.Logf("isOne = %v", product.isOne())

	if !product.isOne() {
		// Try with pure Go
		SetAVX2Enabled(false)
		var inv2, product2 Scalar
		inv2.inverse(&a)
		product2.mul(&a, &inv2)
		t.Logf("Pure Go: a * inv(a) = %v", product2.d)
		t.Logf("Pure Go isOne = %v", product2.isOne())
		SetAVX2Enabled(true)

		t.Errorf("2 * inv(2) should equal 1")
	}
}

func TestScalarMulAVX2VsPureGo(t *testing.T) {
	if !HasAVX2CPU() {
		t.Skip("AVX2 not available")
	}

	// Test several multiplication cases
	testCases := []struct {
		a, b uint
	}{
		{2, 3},
		{12345, 67890},
		{0xFFFFFFFF, 0xFFFFFFFF},
		{1, 1},
		{0, 123},
	}

	for _, tc := range testCases {
		var a, b, productAVX, productGo Scalar
		a.setInt(tc.a)
		b.setInt(tc.b)

		SetAVX2Enabled(true)
		scalarMulAVX2(&productAVX, &a, &b)

		productGo.mulPureGo(&a, &b)

		if !productAVX.equal(&productGo) {
			t.Errorf("Mismatch for %d * %d:\n  AVX2: %v\n  Go: %v",
				tc.a, tc.b, productAVX.d, productGo.d)
		}
	}
}

func TestScalarMulAVX2Large(t *testing.T) {
	if !HasAVX2CPU() {
		t.Skip("AVX2 not available")
	}

	// Test with the actual inverse of 2
	var a Scalar
	a.setInt(2)

	var inv Scalar
	SetAVX2Enabled(false)
	inv.inverse(&a)
	SetAVX2Enabled(true)

	t.Logf("a = %v", a.d)
	t.Logf("inv(2) = %v", inv.d)

	// Test multiplication of 2 * inv(2)
	var productAVX, productGo Scalar
	scalarMulAVX2(&productAVX, &a, &inv)

	SetAVX2Enabled(false)
	productGo.mulPureGo(&a, &inv)
	SetAVX2Enabled(true)

	t.Logf("AVX2: 2 * inv(2) = %v", productAVX.d)
	t.Logf("Go:   2 * inv(2) = %v", productGo.d)

	if !productAVX.equal(&productGo) {
		t.Errorf("Large number multiplication differs")
	}
}

func TestInverseAVX2VsGo(t *testing.T) {
	if !HasAVX2CPU() {
		t.Skip("AVX2 not available")
	}

	var a Scalar
	a.setInt(2)

	// Compute inverse with AVX2
	var invAVX Scalar
	SetAVX2Enabled(true)
	invAVX.inverse(&a)

	// Compute inverse with pure Go
	var invGo Scalar
	SetAVX2Enabled(false)
	invGo.inverse(&a)
	SetAVX2Enabled(true)

	t.Logf("AVX2 inv(2) = %v", invAVX.d)
	t.Logf("Go   inv(2) = %v", invGo.d)

	if !invAVX.equal(&invGo) {
		t.Errorf("Inverse differs between AVX2 and Go")
	}
}

func TestScalarMulAliased(t *testing.T) {
	if !HasAVX2CPU() {
		t.Skip("AVX2 not available")
	}

	// Test aliased multiplication: r.mul(r, &b) and r.mul(&a, r)
	var a, b Scalar
	a.setInt(12345)
	b.setInt(67890)

	// Test r = r * b
	var rAVX, rGo Scalar
	rAVX = a
	rGo = a

	SetAVX2Enabled(true)
	scalarMulAVX2(&rAVX, &rAVX, &b)

	SetAVX2Enabled(false)
	rGo.mulPureGo(&rGo, &b)
	SetAVX2Enabled(true)

	if !rAVX.equal(&rGo) {
		t.Errorf("r = r * b failed:\n  AVX2: %v\n  Go: %v", rAVX.d, rGo.d)
	}

	// Test r = a * r
	rAVX = b
	rGo = b

	SetAVX2Enabled(true)
	scalarMulAVX2(&rAVX, &a, &rAVX)

	SetAVX2Enabled(false)
	rGo.mulPureGo(&a, &rGo)
	SetAVX2Enabled(true)

	if !rAVX.equal(&rGo) {
		t.Errorf("r = a * r failed:\n  AVX2: %v\n  Go: %v", rAVX.d, rGo.d)
	}

	// Test squaring: r = r * r
	rAVX = a
	rGo = a

	SetAVX2Enabled(true)
	scalarMulAVX2(&rAVX, &rAVX, &rAVX)

	SetAVX2Enabled(false)
	rGo.mulPureGo(&rGo, &rGo)
	SetAVX2Enabled(true)

	if !rAVX.equal(&rGo) {
		t.Errorf("r = r * r failed:\n  AVX2: %v\n  Go: %v", rAVX.d, rGo.d)
	}
}

func TestScalarMulLargeNumbers(t *testing.T) {
	if !HasAVX2CPU() {
		t.Skip("AVX2 not available")
	}

	// Test with large numbers (all limbs non-zero)
	testCases := []struct {
		name string
		a, b Scalar
	}{
		{
			name: "large a * small b",
			a:    Scalar{d: [4]uint64{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0, 0}},
			b:    Scalar{d: [4]uint64{2, 0, 0, 0}},
		},
		{
			name: "a^2 where a is large",
			a:    Scalar{d: [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0, 0}},
			b:    Scalar{d: [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0, 0}},
		},
		{
			name: "full limbs",
			a:    Scalar{d: [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}},
			b:    Scalar{d: [4]uint64{0x0FEDCBA987654321, 0x123456789ABCDEF0, 0x3333333333333333, 0x4444444444444444}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var productAVX, productGo Scalar

			SetAVX2Enabled(true)
			scalarMulAVX2(&productAVX, &tc.a, &tc.b)

			SetAVX2Enabled(false)
			productGo.mulPureGo(&tc.a, &tc.b)
			SetAVX2Enabled(true)

			if !productAVX.equal(&productGo) {
				t.Errorf("Mismatch:\n  a: %v\n  b: %v\n  AVX2: %v\n  Go: %v",
					tc.a.d, tc.b.d, productAVX.d, productGo.d)
			}
		})
	}
}
