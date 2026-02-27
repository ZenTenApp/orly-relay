package wnaf

import (
	"crypto/rand"
	"math/big"
	"testing"
)

// smallScalar constructs a [4]uint64 from a small uint64 value.
func smallScalar(v uint64) [4]uint64 {
	return [4]uint64{v, 0, 0, 0}
}

// limbs4ToBigInt converts [4]uint64 little-endian limbs to *big.Int (for test readability).
func limbs4ToBigInt(s [4]uint64) *big.Int {
	b := new(big.Int)
	for i := 3; i >= 0; i-- {
		b.Lsh(b, 64)
		b.Or(b, new(big.Int).SetUint64(s[i]))
	}
	return b
}

// bigIntToLimbs4 converts *big.Int to [4]uint64 little-endian limbs.
func bigIntToLimbs4(b *big.Int) [4]uint64 {
	var s [4]uint64
	mask := new(big.Int).SetUint64(^uint64(0))
	tmp := new(big.Int).Set(b)
	for i := range 4 {
		s[i] = new(big.Int).And(tmp, mask).Uint64()
		tmp.Rsh(tmp, 64)
	}
	return s
}

// randomScalar generates a random 256-bit scalar for testing.
func randomScalar() [4]uint64 {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return FromBytes(buf)
}

// TestEncodeSmallNAF tests wNAF with w=2 (standard NAF) against hand-computed values.
func TestEncodeSmallNAF(t *testing.T) {
	tests := []struct {
		name     string
		scalar   uint64
		expected map[int]int8 // position -> digit
	}{
		{"zero", 0, nil},
		{"one", 1, map[int]int8{0: 1}},
		{"two", 2, map[int]int8{1: 1}},
		{"three", 3, map[int]int8{0: -1, 2: 1}},
		{"seven", 7, map[int]int8{0: -1, 3: 1}},
		{"eleven", 11, map[int]int8{0: -1, 2: -1, 4: 1}},
		{"four", 4, map[int]int8{2: 1}},
		{"eight", 8, map[int]int8{3: 1}},
		{"sixteen", 16, map[int]int8{4: 1}},
		{"five", 5, map[int]int8{0: 1, 2: 1}},
		{"six", 6, map[int]int8{1: -1, 3: 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Encode(smallScalar(tc.scalar), 2)

			if err := d.Valid(2); err != nil {
				t.Fatalf("invalid wNAF: %v", err)
			}

			// Check expected non-zero digits
			for pos, digit := range tc.expected {
				if d.D[pos] != digit {
					t.Errorf("position %d: expected %d, got %d", pos, digit, d.D[pos])
				}
			}

			// Check no unexpected non-zero digits
			for i := range 257 {
				if d.D[i] != 0 {
					if _, ok := tc.expected[i]; !ok {
						t.Errorf("unexpected non-zero digit at position %d: %d", i, d.D[i])
					}
				}
			}

			// Round-trip
			got := d.Reconstruct()
			want := smallScalar(tc.scalar)
			if got != want {
				t.Errorf("reconstruct mismatch: got %v, want %v", got, want)
			}
		})
	}
}

// TestEncodeW5 tests wNAF with w=5 (libsecp256k1 default) against hand-computed values.
func TestEncodeW5(t *testing.T) {
	tests := []struct {
		name     string
		scalar   uint64
		expected map[int]int8
	}{
		{"zero", 0, nil},
		{"one", 1, map[int]int8{0: 1}},
		{"fifteen", 15, map[int]int8{0: 15}},
		{"seventeen", 17, map[int]int8{0: -15, 5: 1}},
		{"thirty_one", 31, map[int]int8{0: -1, 5: 1}},
		{"thirty_two", 32, map[int]int8{5: 1}},
		{"thirty_three", 33, map[int]int8{0: 1, 5: 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Encode(smallScalar(tc.scalar), 5)

			if err := d.Valid(5); err != nil {
				t.Fatalf("invalid wNAF: %v", err)
			}

			for pos, digit := range tc.expected {
				if d.D[pos] != digit {
					t.Errorf("position %d: expected %d, got %d", pos, digit, d.D[pos])
				}
			}

			for i := range 257 {
				if d.D[i] != 0 {
					if _, ok := tc.expected[i]; !ok {
						t.Errorf("unexpected non-zero digit at position %d: %d", i, d.D[i])
					}
				}
			}

			got := d.Reconstruct()
			want := smallScalar(tc.scalar)
			if got != want {
				t.Errorf("reconstruct mismatch: got %v, want %v", got, want)
			}
		})
	}
}

// TestEncodeEdgeCases tests boundary conditions.
func TestEncodeEdgeCases(t *testing.T) {
	for w := 2; w <= 8; w++ {
		t.Run("zero/w="+itoa(w), func(t *testing.T) {
			d := Encode([4]uint64{}, w)
			if err := d.Valid(w); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			if d.Reconstruct() != [4]uint64{} {
				t.Error("zero scalar should reconstruct to zero")
			}
		})

		t.Run("one/w="+itoa(w), func(t *testing.T) {
			d := Encode(smallScalar(1), w)
			if err := d.Valid(w); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			if d.Reconstruct() != smallScalar(1) {
				t.Error("scalar 1 should reconstruct to 1")
			}
		})

		t.Run("powers_of_2/w="+itoa(w), func(t *testing.T) {
			for bit := 0; bit < 256; bit++ {
				var s [4]uint64
				s[bit/64] = 1 << (bit % 64)
				d := Encode(s, w)
				if err := d.Valid(w); err != nil {
					t.Fatalf("bit %d: invalid: %v", bit, err)
				}
				if d.Reconstruct() != s {
					t.Errorf("bit %d: reconstruct mismatch", bit)
				}
			}
		})

		t.Run("all_ones_128/w="+itoa(w), func(t *testing.T) {
			s := [4]uint64{^uint64(0), ^uint64(0), 0, 0}
			d := Encode(s, w)
			if err := d.Valid(w); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			if d.Reconstruct() != s {
				t.Errorf("reconstruct mismatch: got %v, want %v", d.Reconstruct(), s)
			}
		})

		t.Run("all_ones_256/w="+itoa(w), func(t *testing.T) {
			s := [4]uint64{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}
			d := Encode(s, w)
			if err := d.Valid(w); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			if d.Reconstruct() != s {
				t.Errorf("reconstruct mismatch: got %v, want %v", d.Reconstruct(), s)
			}
		})
	}
}

// TestEncodePropertyRoundTrip verifies that Encode followed by Reconstruct
// is the identity for random 256-bit scalars across all supported window widths.
func TestEncodePropertyRoundTrip(t *testing.T) {
	const iterations = 500

	for w := 2; w <= 8; w++ {
		t.Run("w="+itoa(w), func(t *testing.T) {
			for i := range iterations {
				s := randomScalar()
				d := Encode(s, w)

				if err := d.Valid(w); err != nil {
					t.Fatalf("iteration %d: invalid wNAF: %v", i, err)
				}

				got := d.Reconstruct()
				if got != s {
					t.Fatalf("iteration %d: round-trip failed\n  input:  %016x %016x %016x %016x\n  output: %016x %016x %016x %016x",
						i, s[3], s[2], s[1], s[0], got[3], got[2], got[1], got[0])
				}
			}
		})
	}
}

// TestEncodePropertyValidity checks that every encoded wNAF satisfies
// all structural invariants.
func TestEncodePropertyValidity(t *testing.T) {
	const iterations = 500

	for w := 2; w <= 8; w++ {
		t.Run("w="+itoa(w), func(t *testing.T) {
			for i := range iterations {
				s := randomScalar()
				d := Encode(s, w)
				if err := d.Valid(w); err != nil {
					t.Fatalf("iteration %d: %v\n  scalar: %016x %016x %016x %016x",
						i, err, s[3], s[2], s[1], s[0])
				}
			}
		})
	}
}

// TestReconstructWithBigInt cross-checks Reconstruct against big.Int arithmetic.
func TestReconstructWithBigInt(t *testing.T) {
	const iterations = 200

	for w := 2; w <= 8; w++ {
		t.Run("w="+itoa(w), func(t *testing.T) {
			for iter := range iterations {
				s := randomScalar()
				d := Encode(s, w)

				// Compute expected value using big.Int
				expected := new(big.Int)
				for i := range 257 {
					if d.D[i] == 0 {
						continue
					}
					shifted := new(big.Int).Lsh(big.NewInt(int64(d.D[i])), uint(i))
					expected.Add(expected, shifted)
				}

				got := limbs4ToBigInt(d.Reconstruct())
				if got.Cmp(expected) != 0 {
					t.Fatalf("iteration %d: big.Int mismatch\n  Reconstruct: %s\n  big.Int:     %s",
						iter, got.Text(16), expected.Text(16))
				}
			}
		})
	}
}

// TestFromBytes verifies the byte-to-limb conversion.
func TestFromBytes(t *testing.T) {
	// All zeros
	var zero [32]byte
	if FromBytes(zero) != [4]uint64{} {
		t.Error("zero bytes should produce zero limbs")
	}

	// Single byte at the end (LSB)
	var one [32]byte
	one[31] = 1
	if FromBytes(one) != smallScalar(1) {
		t.Errorf("got %v, want %v", FromBytes(one), smallScalar(1))
	}

	// 0x0102030405060708 in the highest limb
	var high [32]byte
	high[0] = 0x01
	high[1] = 0x02
	high[2] = 0x03
	high[3] = 0x04
	high[4] = 0x05
	high[5] = 0x06
	high[6] = 0x07
	high[7] = 0x08
	s := FromBytes(high)
	if s[3] != 0x0102030405060708 {
		t.Errorf("high limb: got %016x, want 0102030405060708", s[3])
	}
	if s[0] != 0 || s[1] != 0 || s[2] != 0 {
		t.Error("lower limbs should be zero")
	}
}

// TestGetBits verifies bit extraction across limb boundaries.
func TestGetBits(t *testing.T) {
	s := [4]uint64{0xDEADBEEFCAFEBABE, 0x1234567890ABCDEF, 0, 0}

	// Single bit
	if getBits(s, 0, 1) != 0 {
		t.Error("bit 0 should be 0")
	}
	if getBits(s, 1, 1) != 1 {
		t.Error("bit 1 should be 1")
	}

	// Byte at offset 0
	if getBits(s, 0, 8) != 0xBE {
		t.Errorf("bits [0:8] = %x, want 0xBE", getBits(s, 0, 8))
	}

	// Cross limb boundary (bits 60-67)
	val := getBits(s, 60, 8)
	// Low 4 bits from limb 0 (bits 60-63): 0xD (top nibble of 0xDEADBEEFCAFEBABE)
	// High 4 bits from limb 1 (bits 0-3): 0xF (low nibble of 0x1234567890ABCDEF)
	if val != 0xFD {
		t.Errorf("cross-limb bits [60:68] = %x, want 0xFD", val)
	}
}

// TestEncodeSignedNegation verifies that EncodeSigned handles high-bit scalars.
func TestEncodeSignedNegation(t *testing.T) {
	// Scalar with bit 255 set
	s := [4]uint64{1, 0, 0, 1 << 63}
	d, negated := EncodeSigned(s, 5)
	if !negated {
		t.Error("should have negated")
	}
	if err := d.Valid(5); err != nil {
		t.Fatalf("invalid: %v", err)
	}

	// Scalar without bit 255
	s2 := [4]uint64{0x12345, 0, 0, 0}
	d2, negated2 := EncodeSigned(s2, 5)
	if negated2 {
		t.Error("should not have negated")
	}
	if d2.Reconstruct() != s2 {
		t.Error("non-negated round-trip failed")
	}
}

// TestValidRejectsInvalid checks that Valid catches structural violations.
func TestValidRejectsInvalid(t *testing.T) {
	// Even non-zero digit
	d := Digits{}
	d.D[0] = 2
	if d.Valid(5) == nil {
		t.Error("should reject even digit")
	}

	// Out of range digit for w=3 (max is 3)
	d = Digits{}
	d.D[0] = 5
	if d.Valid(3) == nil {
		t.Error("should reject digit 5 for w=3")
	}

	// Adjacent non-zero digits (spacing violation for w=5)
	d = Digits{}
	d.D[0] = 1
	d.D[3] = 1 // only 3 apart, need >= 5
	if d.Valid(5) == nil {
		t.Error("should reject spacing violation")
	}

	// Valid: non-zero digits exactly w apart
	d = Digits{}
	d.D[0] = 1
	d.D[5] = 1
	if err := d.Valid(5); err != nil {
		t.Errorf("should accept spacing of exactly w: %v", err)
	}
}

// TestNonZeroDigitCount verifies that wNAF produces a sparse representation.
func TestNonZeroDigitCount(t *testing.T) {
	const iterations = 200

	for w := 2; w <= 8; w++ {
		t.Run("w="+itoa(w), func(t *testing.T) {
			maxExpected := 256/w + 2 // floor(256/w) + 1 body digits + 1 possible carry
			for range iterations {
				s := randomScalar()
				d := Encode(s, w)

				count := 0
				for i := range 257 {
					if d.D[i] != 0 {
						count++
					}
				}

				if count > maxExpected {
					t.Errorf("too many non-zero digits: %d (expected <= %d) for w=%d",
						count, maxExpected, w)
				}
			}
		})
	}
}

// itoa is a simple int-to-string for test names without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
