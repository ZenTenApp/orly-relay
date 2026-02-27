//go:build amd64 && !purego

package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestField4x64SetGet(t *testing.T) {
	// Test round-trip through SetB32/GetB32
	original := make([]byte, 32)
	rand.Read(original)

	// Make sure it's less than p
	original[0] &= 0x7F // Clear top bit to ensure < p

	var f Field4x64
	if err := f.SetB32(original); err != nil {
		t.Fatalf("SetB32 failed: %v", err)
	}

	result := make([]byte, 32)
	f.GetB32(result)

	for i := 0; i < 32; i++ {
		if original[i] != result[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, result[i], original[i])
		}
	}
}

func TestField4x64Add(t *testing.T) {
	// Test: a + 0 = a
	var a, zero, result Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1
	zero.SetZero()

	result.Add(&a, &zero)
	result.Reduce()

	if !result.Equal(&a) {
		t.Error("a + 0 != a")
	}

	// Test: a + (-a) = 0
	var negA Field4x64
	negA.Negate(&a)
	result.Add(&a, &negA)
	result.Reduce()

	if !result.IsZero() {
		t.Errorf("a + (-a) != 0, got %v", result.n)
	}
}

func TestField4x64Mul(t *testing.T) {
	// Test: a * 1 = a
	var a, one, result Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1
	one.SetOne()

	result.Mul(&a, &one)

	if !result.Equal(&a) {
		t.Errorf("a * 1 != a\ngot:  %v\nwant: %v", result.n, a.n)
	}

	// Test: a * 0 = 0
	var zero Field4x64
	zero.SetZero()
	result.Mul(&a, &zero)

	if !result.IsZero() {
		t.Errorf("a * 0 != 0, got %v", result.n)
	}

	// Test: 2 * 3 = 6
	var two, three, six Field4x64
	two.n = [4]uint64{2, 0, 0, 0}
	two.magnitude = 1
	three.n = [4]uint64{3, 0, 0, 0}
	three.magnitude = 1
	six.n = [4]uint64{6, 0, 0, 0}
	six.magnitude = 1

	result.Mul(&two, &three)
	if !result.Equal(&six) {
		t.Errorf("2 * 3 != 6, got %v", result.n)
	}
}

func TestField4x64MulLarge(t *testing.T) {
	// Test multiplication of larger values
	// (2^128) * (2^128) = 2^256 ≡ R (mod p) where R = 2^32 + 977

	var a, result Field4x64
	a.n = [4]uint64{0, 0, 1, 0} // 2^128
	a.magnitude = 1

	result.Mul(&a, &a) // (2^128)^2 = 2^256

	// Expected: 2^32 + 977 = 0x1000003D1
	expected := Field4x64{n: [4]uint64{0x1000003D1, 0, 0, 0}, magnitude: 1}

	if !result.Equal(&expected) {
		t.Errorf("2^256 mod p != R\ngot:  %v\nwant: %v", result.n, expected.n)
	}
}

func TestField4x64Comparison5x52(t *testing.T) {
	// Compare 4x64 results with existing 5x52 implementation
	for i := 0; i < 100; i++ {
		// Generate random field elements
		aBytes := make([]byte, 32)
		bBytes := make([]byte, 32)
		rand.Read(aBytes)
		rand.Read(bBytes)

		// Ensure < p
		aBytes[0] &= 0x7F
		bBytes[0] &= 0x7F

		// 4x64 multiplication
		var a4, b4, r4 Field4x64
		a4.SetB32(aBytes)
		b4.SetB32(bBytes)
		r4.Mul(&a4, &b4)

		// 5x52 multiplication
		var a5, b5, r5 FieldElement
		a5.setB32(aBytes)
		b5.setB32(bBytes)
		r5.mul(&a5, &b5)
		r5.normalize()

		// Compare results
		result4 := make([]byte, 32)
		result5 := make([]byte, 32)
		r4.GetB32(result4)
		r5.getB32(result5)

		for j := 0; j < 32; j++ {
			if result4[j] != result5[j] {
				t.Errorf("iteration %d: 4x64 and 5x52 differ at byte %d\n4x64: %x\n5x52: %x",
					i, j, result4, result5)
				break
			}
		}
	}
}

func BenchmarkField4x64Mul(b *testing.B) {
	var a, c, r Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1
	c.n = [4]uint64{0xAAAABBBBCCCCDDDD, 0xEEEEFFFF00001111, 0x3333444455556666, 0x7777888899990000}
	c.magnitude = 1

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Mul(&a, &c)
	}
}

func BenchmarkField4x64Add(b *testing.B) {
	var a, c, r Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1
	c.n = [4]uint64{0xAAAABBBBCCCCDDDD, 0xEEEEFFFF00001111, 0x3333444455556666, 0x7777888899990000}
	c.magnitude = 1

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Add(&a, &c)
	}
}

func BenchmarkField4x64Sqr(b *testing.B) {
	var a, r Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Sqr(&a)
	}
}

// Comparison benchmarks
func BenchmarkField5x52Mul(b *testing.B) {
	var a, c, r FieldElement
	aBytes := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0,
		0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10,
		0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11,
		0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22}
	cBytes := []byte{0xAA, 0xAA, 0xBB, 0xBB, 0xCC, 0xCC, 0xDD, 0xDD,
		0xEE, 0xEE, 0xFF, 0xFF, 0x00, 0x00, 0x11, 0x11,
		0x33, 0x33, 0x44, 0x44, 0x55, 0x55, 0x66, 0x66,
		0x77, 0x77, 0x88, 0x88, 0x99, 0x99, 0x00, 0x00}

	a.setB32(aBytes)
	c.setB32(cBytes)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.mul(&a, &c)
	}
}

func BenchmarkField5x52Sqr(b *testing.B) {
	var a, r FieldElement
	aBytes := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0,
		0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10,
		0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11,
		0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22}

	a.setB32(aBytes)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.sqr(&a)
	}
}

func BenchmarkField4x64Inv(b *testing.B) {
	var a, r Field4x64
	a.n = [4]uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210, 0x1111111111111111, 0x2222222222222222}
	a.magnitude = 1
	a.normalized = true

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.inv(&a)
	}
}

func BenchmarkField5x52Inv(b *testing.B) {
	var a, r FieldElement
	aBytes := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0,
		0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10,
		0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11,
		0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22}

	a.setB32(aBytes)
	a.normalize()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.inv(&a)
	}
}
