// Package wnaf implements windowed Non-Adjacent Form (wNAF) encoding for
// 256-bit scalars. wNAF is a signed-digit representation that minimizes
// the number of non-zero digits, reducing point additions in elliptic
// curve scalar multiplication.
//
// All types are stack-allocated fixed-length arrays with no heap allocation.
package wnaf

import (
	"fmt"
	"math/bits"
)

// Digits holds a wNAF representation: up to 257 signed digits.
// Each non-zero digit is an odd integer in [-(2^(w-1)-1), 2^(w-1)-1],
// and non-zero digits are separated by at least w-1 zeros.
type Digits struct {
	D   [257]int8 // signed digits
	Len int       // number of significant positions (highest non-zero index + 1)
}

// Encode converts a 256-bit scalar (as [4]uint64, little-endian limbs) into
// wNAF representation with window width w. w must be in [2, 8].
//
// The algorithm processes bits from LSB to MSB, extracting w-bit windows
// at each non-zero position and using carry propagation to ensure the
// non-adjacency property.
func Encode(scalar [4]uint64, w int) Digits {
	if w < 2 || w > 8 {
		panic("wnaf: w must be in [2, 8]")
	}

	var d Digits
	var carry uint32

	bit := 0
	for bit < 256 {
		if getBits(scalar, uint(bit), 1) == carry {
			bit++
			continue
		}

		window := uint(w)
		if bit+int(window) > 256 {
			window = uint(256 - bit)
		}

		word := getBits(scalar, uint(bit), window) + carry

		carry = (word >> (window - 1)) & 1
		word -= carry << window

		d.D[bit] = int8(int32(word))
		d.Len = bit + int(window)

		bit += int(window)
	}

	if carry != 0 {
		d.D[256] = int8(carry)
		d.Len = 257
	}

	return d
}

// EncodeSigned converts a scalar to wNAF, handling sign normalization.
// If the scalar has bit 255 set (is "negative" in modular sense), it is
// negated before encoding. Returns the digits and whether negation occurred.
// The caller must negate the final EC point result if negated is true.
func EncodeSigned(scalar [4]uint64, w int) (Digits, bool) {
	if getBits(scalar, 255, 1) == 1 {
		scalar = negate256(scalar)
		d := Encode(scalar, w)
		return d, true
	}
	return Encode(scalar, w), false
}

// Reconstruct recovers the original scalar value from a wNAF representation.
// Returns [4]uint64 in little-endian limb order.
//
// Splits digits into positive and negative contributions, accumulates each
// as an unsigned 320-bit value, then subtracts to get the result.
func (d *Digits) Reconstruct() [4]uint64 {
	var pos, neg [5]uint64

	for i := range 257 {
		if d.D[i] == 0 {
			continue
		}

		limb := uint(i) / 64
		shift := uint(i) % 64

		if d.D[i] > 0 {
			addShifted(&pos, uint64(d.D[i]), limb, shift)
		} else {
			addShifted(&neg, uint64(-d.D[i]), limb, shift)
		}
	}

	return sub320(&pos, &neg)
}

// addShifted adds val << (limb*64 + shift) to a 320-bit accumulator.
func addShifted(acc *[5]uint64, val uint64, limb, shift uint) {
	lo := val << shift
	old := acc[limb]
	acc[limb] += lo
	if acc[limb] < old {
		for j := limb + 1; j < 5; j++ {
			acc[j]++
			if acc[j] != 0 {
				break
			}
		}
	}

	if shift > 0 {
		hi := val >> (64 - shift)
		if hi > 0 && limb+1 < 5 {
			old = acc[limb+1]
			acc[limb+1] += hi
			if acc[limb+1] < old {
				for j := limb + 2; j < 5; j++ {
					acc[j]++
					if acc[j] != 0 {
						break
					}
				}
			}
		}
	}
}

// sub320 computes a - b for 320-bit values, returning the lower 256 bits.
func sub320(a, b *[5]uint64) [4]uint64 {
	var result [4]uint64
	var borrow uint64
	for i := range 4 {
		result[i], borrow = bits.Sub64(a[i], b[i], borrow)
	}
	return result
}

// Valid checks all wNAF structural invariants for window width w.
// Returns nil if valid, or an error describing the violation.
func (d *Digits) Valid(w int) error {
	if w < 2 || w > 8 {
		return fmt.Errorf("wnaf: invalid window width %d, must be in [2, 8]", w)
	}

	maxDigit := int8((1 << (w - 1)) - 1)
	lastNonZero := -1

	for i := range 257 {
		digit := d.D[i]
		if digit == 0 {
			continue
		}

		// Position 256 is the carry bit: always 1, exempt from spacing rule
		if i == 256 {
			if digit != 1 {
				return fmt.Errorf("wnaf: carry digit at position 256 must be 1, got %d", digit)
			}
			lastNonZero = i
			continue
		}

		if digit%2 == 0 {
			return fmt.Errorf("wnaf: digit at position %d is even (%d)", i, digit)
		}

		if digit > maxDigit || digit < -maxDigit {
			return fmt.Errorf("wnaf: digit at position %d out of range: %d (max %d)",
				i, digit, maxDigit)
		}

		if lastNonZero >= 0 && lastNonZero < 256 && i-lastNonZero < w {
			return fmt.Errorf("wnaf: non-zero digits at positions %d and %d "+
				"are only %d apart (need >= %d)", lastNonZero, i, i-lastNonZero, w)
		}

		lastNonZero = i
	}

	return nil
}

// FromBytes converts a 32-byte big-endian scalar into [4]uint64 little-endian
// limb order (matching the internal Scalar.d layout).
func FromBytes(b [32]byte) [4]uint64 {
	var s [4]uint64
	s[3] = uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	s[2] = uint64(b[8])<<56 | uint64(b[9])<<48 | uint64(b[10])<<40 | uint64(b[11])<<32 |
		uint64(b[12])<<24 | uint64(b[13])<<16 | uint64(b[14])<<8 | uint64(b[15])
	s[1] = uint64(b[16])<<56 | uint64(b[17])<<48 | uint64(b[18])<<40 | uint64(b[19])<<32 |
		uint64(b[20])<<24 | uint64(b[21])<<16 | uint64(b[22])<<8 | uint64(b[23])
	s[0] = uint64(b[24])<<56 | uint64(b[25])<<48 | uint64(b[26])<<40 | uint64(b[27])<<32 |
		uint64(b[28])<<24 | uint64(b[29])<<16 | uint64(b[30])<<8 | uint64(b[31])
	return s
}

// getBits extracts count bits starting at offset from a [4]uint64 scalar.
// offset+count must be <= 256. count must be in [1, 32].
func getBits(scalar [4]uint64, offset, count uint) uint32 {
	limbIdx := offset / 64
	bitIdx := offset % 64

	if bitIdx+count <= 64 {
		return uint32((scalar[limbIdx] >> bitIdx) & ((1 << count) - 1))
	}
	lowBits := 64 - bitIdx
	highBits := count - lowBits
	low := uint32((scalar[limbIdx] >> bitIdx) & ((1 << lowBits) - 1))
	high := uint32(scalar[limbIdx+1] & ((1 << highBits) - 1))
	return low | (high << lowBits)
}

// negate256 computes the two's complement negation of a 256-bit value.
func negate256(s [4]uint64) [4]uint64 {
	s[0] = ^s[0]
	s[1] = ^s[1]
	s[2] = ^s[2]
	s[3] = ^s[3]
	s[0]++
	if s[0] == 0 {
		s[1]++
		if s[1] == 0 {
			s[2]++
			if s[2] == 0 {
				s[3]++
			}
		}
	}
	return s
}
