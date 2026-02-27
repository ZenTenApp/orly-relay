package avx

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

// Test vectors from Bitcoin/secp256k1

func TestUint128Add(t *testing.T) {
	tests := []struct {
		a, b   Uint128
		expect Uint128
		carry  uint64
	}{
		{Uint128{0, 0}, Uint128{0, 0}, Uint128{0, 0}, 0},
		{Uint128{1, 0}, Uint128{1, 0}, Uint128{2, 0}, 0},
		{Uint128{^uint64(0), 0}, Uint128{1, 0}, Uint128{0, 1}, 0},
		{Uint128{^uint64(0), ^uint64(0)}, Uint128{1, 0}, Uint128{0, 0}, 1},
	}

	for i, tt := range tests {
		result, carry := tt.a.Add(tt.b)
		if result != tt.expect || carry != tt.carry {
			t.Errorf("test %d: got (%v, %d), want (%v, %d)", i, result, carry, tt.expect, tt.carry)
		}
	}
}

func TestUint128Mul(t *testing.T) {
	// Test: 2^64 * 2^64 = 2^128
	a := Uint128{0, 1} // 2^64
	b := Uint128{0, 1} // 2^64
	result := a.Mul(b)
	// Expected: 2^128 = [0, 0, 1, 0]
	expected := [4]uint64{0, 0, 1, 0}
	if result != expected {
		t.Errorf("2^64 * 2^64: got %v, want %v", result, expected)
	}

	// Test: (2^64 - 1) * (2^64 - 1)
	a = Uint128{^uint64(0), 0}
	b = Uint128{^uint64(0), 0}
	result = a.Mul(b)
	// (2^64 - 1)^2 = 2^128 - 2^65 + 1
	// = [1, 0xFFFFFFFFFFFFFFFE, 0, 0]
	expected = [4]uint64{1, 0xFFFFFFFFFFFFFFFE, 0, 0}
	if result != expected {
		t.Errorf("(2^64-1)^2: got %v, want %v", result, expected)
	}
}

func TestScalarSetBytes(t *testing.T) {
	// Test with a known scalar
	bytes32 := make([]byte, 32)
	bytes32[31] = 1 // scalar = 1

	var s Scalar
	s.SetBytes(bytes32)

	if !s.IsOne() {
		t.Errorf("expected scalar to be 1, got %+v", s)
	}

	// Test zero
	bytes32 = make([]byte, 32)
	s.SetBytes(bytes32)
	if !s.IsZero() {
		t.Errorf("expected scalar to be 0, got %+v", s)
	}
}

func TestScalarAddSub(t *testing.T) {
	var a, b, sum, diff, recovered Scalar

	// a = 1, b = 2
	a = ScalarOne
	b.D[0].Lo = 2

	sum.Add(&a, &b)
	if sum.D[0].Lo != 3 {
		t.Errorf("1 + 2: expected 3, got %d", sum.D[0].Lo)
	}

	diff.Sub(&sum, &b)
	if !diff.Equal(&a) {
		t.Errorf("(1+2) - 2: expected 1, got %+v", diff)
	}

	// Test with overflow
	a = ScalarN
	a.D[0].Lo-- // n - 1
	b = ScalarOne

	sum.Add(&a, &b)
	// n - 1 + 1 = n ≡ 0 (mod n)
	if !sum.IsZero() {
		t.Errorf("(n-1) + 1 should be 0 mod n, got %+v", sum)
	}

	// Test subtraction with borrow
	a = ScalarZero
	b = ScalarOne
	diff.Sub(&a, &b)
	// 0 - 1 = -1 ≡ n - 1 (mod n)
	recovered.Add(&diff, &b)
	if !recovered.IsZero() {
		t.Errorf("(0-1) + 1 should be 0, got %+v", recovered)
	}
}

func TestScalarMul(t *testing.T) {
	var a, b, product Scalar

	// 2 * 3 = 6
	a.D[0].Lo = 2
	b.D[0].Lo = 3
	product.Mul(&a, &b)
	if product.D[0].Lo != 6 || product.D[0].Hi != 0 || !product.D[1].IsZero() {
		t.Errorf("2 * 3: expected 6, got %+v", product)
	}

	// Test with larger values
	a.D[0].Lo = 0xFFFFFFFFFFFFFFFF
	a.D[0].Hi = 0
	b.D[0].Lo = 2
	product.Mul(&a, &b)
	// (2^64 - 1) * 2 = 2^65 - 2
	if product.D[0].Lo != 0xFFFFFFFFFFFFFFFE || product.D[0].Hi != 1 {
		t.Errorf("(2^64-1) * 2: got %+v", product)
	}
}

func TestScalarNegate(t *testing.T) {
	var a, neg, sum Scalar

	a.D[0].Lo = 12345
	neg.Negate(&a)
	sum.Add(&a, &neg)

	if !sum.IsZero() {
		t.Errorf("a + (-a) should be 0, got %+v", sum)
	}
}

func TestFieldSetBytes(t *testing.T) {
	bytes32 := make([]byte, 32)
	bytes32[31] = 1

	var f FieldElement
	f.SetBytes(bytes32)

	if !f.IsOne() {
		t.Errorf("expected field element to be 1, got %+v", f)
	}
}

func TestFieldAddSub(t *testing.T) {
	var a, b, sum, diff FieldElement

	a.N[0].Lo = 100
	b.N[0].Lo = 200

	sum.Add(&a, &b)
	if sum.N[0].Lo != 300 {
		t.Errorf("100 + 200: expected 300, got %d", sum.N[0].Lo)
	}

	diff.Sub(&sum, &b)
	if !diff.Equal(&a) {
		t.Errorf("(100+200) - 200: expected 100, got %+v", diff)
	}
}

func TestFieldMul(t *testing.T) {
	var a, b, product FieldElement

	a.N[0].Lo = 7
	b.N[0].Lo = 8
	product.Mul(&a, &b)
	if product.N[0].Lo != 56 {
		t.Errorf("7 * 8: expected 56, got %d", product.N[0].Lo)
	}
}

func TestFieldInverse(t *testing.T) {
	var a, inv, product FieldElement

	a.N[0].Lo = 7
	inv.Inverse(&a)
	product.Mul(&a, &inv)

	if !product.IsOne() {
		t.Errorf("7 * 7^(-1) should be 1, got %+v", product)
	}
}

func TestFieldSqrt(t *testing.T) {
	// Test sqrt(4) = 2
	var four, root, check FieldElement
	four.N[0].Lo = 4

	if !root.Sqrt(&four) {
		t.Fatal("sqrt(4) should exist")
	}

	check.Sqr(&root)
	if !check.Equal(&four) {
		t.Errorf("sqrt(4)^2 should be 4, got %+v", check)
	}
}

func TestGeneratorOnCurve(t *testing.T) {
	if !Generator.IsOnCurve() {
		t.Error("generator point should be on the curve")
	}
}

func TestPointDouble(t *testing.T) {
	var g, doubled JacobianPoint
	var affineResult AffinePoint

	g.FromAffine(&Generator)
	doubled.Double(&g)
	doubled.ToAffine(&affineResult)

	if affineResult.Infinity {
		t.Error("2G should not be infinity")
	}

	if !affineResult.IsOnCurve() {
		t.Error("2G should be on the curve")
	}
}

func TestPointAdd(t *testing.T) {
	var g, twoG, threeG JacobianPoint
	var affineResult AffinePoint

	g.FromAffine(&Generator)
	twoG.Double(&g)
	threeG.Add(&twoG, &g)
	threeG.ToAffine(&affineResult)

	if !affineResult.IsOnCurve() {
		t.Error("3G should be on the curve")
	}

	// Also test via scalar multiplication
	var three Scalar
	three.D[0].Lo = 3

	var expected JacobianPoint
	expected.ScalarMult(&g, &three)
	var expectedAffine AffinePoint
	expected.ToAffine(&expectedAffine)

	if !affineResult.Equal(&expectedAffine) {
		t.Error("G + 2G should equal 3G")
	}
}

func TestPointAddInfinity(t *testing.T) {
	var g, inf, result JacobianPoint
	var affineResult AffinePoint

	g.FromAffine(&Generator)
	inf.SetInfinity()

	result.Add(&g, &inf)
	result.ToAffine(&affineResult)

	if !affineResult.Equal(&Generator) {
		t.Error("G + O should equal G")
	}

	result.Add(&inf, &g)
	result.ToAffine(&affineResult)

	if !affineResult.Equal(&Generator) {
		t.Error("O + G should equal G")
	}
}

func TestScalarBaseMult(t *testing.T) {
	// Test 1*G = G
	result := BasePointMult(&ScalarOne)
	if !result.Equal(&Generator) {
		t.Error("1*G should equal G")
	}

	// Test 2*G
	var two Scalar
	two.D[0].Lo = 2
	result = BasePointMult(&two)

	var g, twoG JacobianPoint
	var expected AffinePoint
	g.FromAffine(&Generator)
	twoG.Double(&g)
	twoG.ToAffine(&expected)

	if !result.Equal(&expected) {
		t.Error("2*G via scalar mult should equal 2*G via doubling")
	}
}

func TestKnownScalarMult(t *testing.T) {
	// Test vector: private key and public key from Bitcoin
	// This is a well-known test vector
	privKeyHex := "0000000000000000000000000000000000000000000000000000000000000001"
	expectedXHex := "79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"
	expectedYHex := "483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"

	privKeyBytes, _ := hex.DecodeString(privKeyHex)
	var k Scalar
	k.SetBytes(privKeyBytes)

	result := BasePointMult(&k)

	xBytes := result.X.Bytes()
	yBytes := result.Y.Bytes()

	expectedX, _ := hex.DecodeString(expectedXHex)
	expectedY, _ := hex.DecodeString(expectedYHex)

	if !bytes.Equal(xBytes[:], expectedX) {
		t.Errorf("X coordinate mismatch:\ngot:  %x\nwant: %x", xBytes, expectedX)
	}
	if !bytes.Equal(yBytes[:], expectedY) {
		t.Errorf("Y coordinate mismatch:\ngot:  %x\nwant: %x", yBytes, expectedY)
	}
}

// Benchmark tests

func BenchmarkUint128Mul(b *testing.B) {
	a := Uint128{0x123456789ABCDEF0, 0xFEDCBA9876543210}
	c := Uint128{0xABCDEF0123456789, 0x9876543210FEDCBA}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Mul(c)
	}
}

func BenchmarkScalarAdd(b *testing.B) {
	var a, c, r Scalar
	aBytes := make([]byte, 32)
	cBytes := make([]byte, 32)
	rand.Read(aBytes)
	rand.Read(cBytes)
	a.SetBytes(aBytes)
	c.SetBytes(cBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Add(&a, &c)
	}
}

func BenchmarkScalarMul(b *testing.B) {
	var a, c, r Scalar
	aBytes := make([]byte, 32)
	cBytes := make([]byte, 32)
	rand.Read(aBytes)
	rand.Read(cBytes)
	a.SetBytes(aBytes)
	c.SetBytes(cBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Mul(&a, &c)
	}
}

func BenchmarkFieldAdd(b *testing.B) {
	var a, c, r FieldElement
	aBytes := make([]byte, 32)
	cBytes := make([]byte, 32)
	rand.Read(aBytes)
	rand.Read(cBytes)
	a.SetBytes(aBytes)
	c.SetBytes(cBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Add(&a, &c)
	}
}

func BenchmarkFieldMul(b *testing.B) {
	var a, c, r FieldElement
	aBytes := make([]byte, 32)
	cBytes := make([]byte, 32)
	rand.Read(aBytes)
	rand.Read(cBytes)
	a.SetBytes(aBytes)
	c.SetBytes(cBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Mul(&a, &c)
	}
}

func BenchmarkFieldInverse(b *testing.B) {
	var a, r FieldElement
	aBytes := make([]byte, 32)
	rand.Read(aBytes)
	a.SetBytes(aBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Inverse(&a)
	}
}

func BenchmarkPointDouble(b *testing.B) {
	var g, r JacobianPoint
	g.FromAffine(&Generator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Double(&g)
	}
}

func BenchmarkPointAdd(b *testing.B) {
	var g, twoG, r JacobianPoint
	g.FromAffine(&Generator)
	twoG.Double(&g)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Add(&g, &twoG)
	}
}

func BenchmarkScalarBaseMult(b *testing.B) {
	var k Scalar
	kBytes := make([]byte, 32)
	rand.Read(kBytes)
	k.SetBytes(kBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BasePointMult(&k)
	}
}
