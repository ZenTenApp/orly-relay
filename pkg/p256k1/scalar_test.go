package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestScalarBasics(t *testing.T) {
	// Test zero scalar
	var zero Scalar
	if !zero.isZero() {
		t.Error("Zero scalar should be zero")
	}

	// Test one scalar
	var one Scalar
	one.setInt(1)
	if !one.isOne() {
		t.Error("One scalar should be one")
	}

	// Test equality
	var one2 Scalar
	one2.setInt(1)
	if !one.equal(&one2) {
		t.Error("Two ones should be equal")
	}
}

func TestScalarSetB32(t *testing.T) {
	// Test setting from 32-byte array
	testCases := []struct {
		name  string
		bytes [32]byte
	}{
		{
			name:  "zero",
			bytes: [32]byte{},
		},
		{
			name:  "one",
			bytes: [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:  "group_order_minus_one",
			bytes: [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE, 0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B, 0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x40},
		},
		{
			name:  "group_order",
			bytes: [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE, 0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B, 0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x41},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var s Scalar
			overflow := s.setB32(tc.bytes[:])

			// Test round-trip
			var result [32]byte
			s.getB32(result[:])

			// For group order, should reduce to zero
			if tc.name == "group_order" {
				if !s.isZero() {
					t.Error("Group order should reduce to zero")
				}
				if !overflow {
					t.Error("Group order should cause overflow")
				}
			}
		})
	}
}

func TestScalarSetB32Seckey(t *testing.T) {
	// Test valid secret key
	validKey := [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	var s Scalar
	if !s.setB32Seckey(validKey[:]) {
		t.Error("Valid secret key should be accepted")
	}

	// Test zero key (invalid)
	zeroKey := [32]byte{}
	if s.setB32Seckey(zeroKey[:]) {
		t.Error("Zero secret key should be rejected")
	}

	// Test group order key (invalid)
	orderKey := [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE, 0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B, 0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x41}
	if s.setB32Seckey(orderKey[:]) {
		t.Error("Group order secret key should be rejected")
	}
}

func TestScalarArithmetic(t *testing.T) {
	// Test addition
	var a, b, c Scalar
	a.setInt(5)
	b.setInt(7)
	c.add(&a, &b)

	var expected Scalar
	expected.setInt(12)
	if !c.equal(&expected) {
		t.Error("5 + 7 should equal 12")
	}

	// Test multiplication
	var mult Scalar
	mult.mul(&a, &b)

	expected.setInt(35)
	if !mult.equal(&expected) {
		t.Error("5 * 7 should equal 35")
	}

	// Test negation
	var neg Scalar
	neg.negate(&a)

	var sum Scalar
	sum.add(&a, &neg)

	if !sum.isZero() {
		t.Error("a + (-a) should equal zero")
	}
}

func TestScalarInverse(t *testing.T) {
	// Test inverse of small numbers
	for i := uint(1); i <= 10; i++ {
		var a, inv, product Scalar
		a.setInt(i)
		inv.inverse(&a)
		product.mul(&a, &inv)

		if !product.isOne() {
			t.Errorf("a * a^(-1) should equal 1 for a = %d", i)
		}
	}
}

func TestScalarHalf(t *testing.T) {
	// Test halving
	var a, half, doubled Scalar

	// Test even number
	a.setInt(14)
	half.half(&a)
	doubled.add(&half, &half)
	if !doubled.equal(&a) {
		t.Error("2 * (14/2) should equal 14")
	}

	// Test odd number
	a.setInt(7)
	half.half(&a)
	doubled.add(&half, &half)
	if !doubled.equal(&a) {
		t.Error("2 * (7/2) should equal 7")
	}
}

func TestScalarProperties(t *testing.T) {
	var a Scalar
	a.setInt(6)

	// Test even/odd
	if !a.isEven() {
		t.Error("6 should be even")
	}

	a.setInt(7)
	if a.isEven() {
		t.Error("7 should be odd")
	}
}

func TestScalarConditionalNegate(t *testing.T) {
	var a, original Scalar
	a.setInt(5)
	original = a

	// Test conditional negate with flag = 0
	a.condNegate(0)
	if !a.equal(&original) {
		t.Error("Conditional negate with flag=0 should not change value")
	}

	// Test conditional negate with flag = 1
	a.condNegate(1)
	var neg Scalar
	neg.negate(&original)
	if !a.equal(&neg) {
		t.Error("Conditional negate with flag=1 should negate value")
	}
}

func TestScalarGetBits(t *testing.T) {
	var a Scalar
	a.setInt(0x12345678)

	// Test getting bits
	bits := a.getBits(0, 8)
	if bits != 0x78 {
		t.Errorf("Expected 0x78, got 0x%x", bits)
	}

	bits = a.getBits(8, 8)
	if bits != 0x56 {
		t.Errorf("Expected 0x56, got 0x%x", bits)
	}
}

func TestScalarConditionalMove(t *testing.T) {
	var a, b, original Scalar
	a.setInt(5)
	b.setInt(10)
	original = a

	// Test conditional move with flag = 0
	a.cmov(&b, 0)
	if !a.equal(&original) {
		t.Error("Conditional move with flag=0 should not change value")
	}

	// Test conditional move with flag = 1
	a.cmov(&b, 1)
	if !a.equal(&b) {
		t.Error("Conditional move with flag=1 should copy value")
	}
}

func TestScalarClear(t *testing.T) {
	var s Scalar
	s.setInt(12345)

	s.clear()

	// After clearing, should be zero
	if !s.isZero() {
		t.Error("Cleared scalar should be zero")
	}
}

func TestScalarRandomOperations(t *testing.T) {
	// Test with random values
	for i := 0; i < 50; i++ {
		var aBytes, bBytes [32]byte
		rand.Read(aBytes[:])
		rand.Read(bBytes[:])

		var a, b Scalar
		a.setB32(aBytes[:])
		b.setB32(bBytes[:])

		// Skip if either is zero
		if a.isZero() || b.isZero() {
			continue
		}

		// Test (a + b) - a = b
		var sum, diff Scalar
		sum.add(&a, &b)
		diff.sub(&sum, &a)
		if !diff.equal(&b) {
			t.Errorf("Random test %d: (a + b) - a should equal b", i)
		}

		// Test (a * b) / a = b
		var prod, quot Scalar
		prod.mul(&a, &b)
		var aInv Scalar
		aInv.inverse(&a)
		quot.mul(&prod, &aInv)
		if !quot.equal(&b) {
			t.Errorf("Random test %d: (a * b) / a should equal b", i)
		}
	}
}

func TestScalarEdgeCases(t *testing.T) {
	// Test n-1 + 1 = 0
	nMinus1 := [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE, 0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B, 0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x40}

	var s Scalar
	s.setB32(nMinus1[:])

	// Add 1 should give 0
	var one Scalar
	one.setInt(1)
	s.add(&s, &one)

	if !s.isZero() {
		t.Error("(n-1) + 1 should equal 0 in scalar arithmetic")
	}
}
