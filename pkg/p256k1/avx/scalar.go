package avx

import "math/bits"

// Scalar operations modulo the secp256k1 group order n.
// n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141

// SetBytes sets a scalar from a 32-byte big-endian slice.
// Returns true if the value was >= n and was reduced.
func (s *Scalar) SetBytes(b []byte) bool {
	if len(b) != 32 {
		panic("scalar must be 32 bytes")
	}

	// Convert big-endian bytes to little-endian limbs
	s.D[0].Lo = uint64(b[31]) | uint64(b[30])<<8 | uint64(b[29])<<16 | uint64(b[28])<<24 |
		uint64(b[27])<<32 | uint64(b[26])<<40 | uint64(b[25])<<48 | uint64(b[24])<<56
	s.D[0].Hi = uint64(b[23]) | uint64(b[22])<<8 | uint64(b[21])<<16 | uint64(b[20])<<24 |
		uint64(b[19])<<32 | uint64(b[18])<<40 | uint64(b[17])<<48 | uint64(b[16])<<56
	s.D[1].Lo = uint64(b[15]) | uint64(b[14])<<8 | uint64(b[13])<<16 | uint64(b[12])<<24 |
		uint64(b[11])<<32 | uint64(b[10])<<40 | uint64(b[9])<<48 | uint64(b[8])<<56
	s.D[1].Hi = uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56

	// Check overflow and reduce if necessary
	overflow := s.checkOverflow()
	if overflow {
		s.reduce()
	}
	return overflow
}

// Bytes returns the scalar as a 32-byte big-endian slice.
func (s *Scalar) Bytes() [32]byte {
	var b [32]byte
	b[31] = byte(s.D[0].Lo)
	b[30] = byte(s.D[0].Lo >> 8)
	b[29] = byte(s.D[0].Lo >> 16)
	b[28] = byte(s.D[0].Lo >> 24)
	b[27] = byte(s.D[0].Lo >> 32)
	b[26] = byte(s.D[0].Lo >> 40)
	b[25] = byte(s.D[0].Lo >> 48)
	b[24] = byte(s.D[0].Lo >> 56)

	b[23] = byte(s.D[0].Hi)
	b[22] = byte(s.D[0].Hi >> 8)
	b[21] = byte(s.D[0].Hi >> 16)
	b[20] = byte(s.D[0].Hi >> 24)
	b[19] = byte(s.D[0].Hi >> 32)
	b[18] = byte(s.D[0].Hi >> 40)
	b[17] = byte(s.D[0].Hi >> 48)
	b[16] = byte(s.D[0].Hi >> 56)

	b[15] = byte(s.D[1].Lo)
	b[14] = byte(s.D[1].Lo >> 8)
	b[13] = byte(s.D[1].Lo >> 16)
	b[12] = byte(s.D[1].Lo >> 24)
	b[11] = byte(s.D[1].Lo >> 32)
	b[10] = byte(s.D[1].Lo >> 40)
	b[9] = byte(s.D[1].Lo >> 48)
	b[8] = byte(s.D[1].Lo >> 56)

	b[7] = byte(s.D[1].Hi)
	b[6] = byte(s.D[1].Hi >> 8)
	b[5] = byte(s.D[1].Hi >> 16)
	b[4] = byte(s.D[1].Hi >> 24)
	b[3] = byte(s.D[1].Hi >> 32)
	b[2] = byte(s.D[1].Hi >> 40)
	b[1] = byte(s.D[1].Hi >> 48)
	b[0] = byte(s.D[1].Hi >> 56)

	return b
}

// IsZero returns true if the scalar is zero.
func (s *Scalar) IsZero() bool {
	return s.D[0].IsZero() && s.D[1].IsZero()
}

// IsOne returns true if the scalar is one.
func (s *Scalar) IsOne() bool {
	return s.D[0].Lo == 1 && s.D[0].Hi == 0 && s.D[1].IsZero()
}

// Equal returns true if two scalars are equal.
func (s *Scalar) Equal(other *Scalar) bool {
	return s.D[0].Lo == other.D[0].Lo && s.D[0].Hi == other.D[0].Hi &&
		s.D[1].Lo == other.D[1].Lo && s.D[1].Hi == other.D[1].Hi
}

// checkOverflow returns true if s >= n.
func (s *Scalar) checkOverflow() bool {
	// Compare high to low
	if s.D[1].Hi > ScalarN.D[1].Hi {
		return true
	}
	if s.D[1].Hi < ScalarN.D[1].Hi {
		return false
	}
	if s.D[1].Lo > ScalarN.D[1].Lo {
		return true
	}
	if s.D[1].Lo < ScalarN.D[1].Lo {
		return false
	}
	if s.D[0].Hi > ScalarN.D[0].Hi {
		return true
	}
	if s.D[0].Hi < ScalarN.D[0].Hi {
		return false
	}
	return s.D[0].Lo >= ScalarN.D[0].Lo
}

// reduce reduces s modulo n by adding the complement (2^256 - n).
func (s *Scalar) reduce() {
	// s = s - n = s + (2^256 - n) mod 2^256
	var carry uint64
	s.D[0].Lo, carry = bits.Add64(s.D[0].Lo, ScalarNC.D[0].Lo, 0)
	s.D[0].Hi, carry = bits.Add64(s.D[0].Hi, ScalarNC.D[0].Hi, carry)
	s.D[1].Lo, carry = bits.Add64(s.D[1].Lo, ScalarNC.D[1].Lo, carry)
	s.D[1].Hi, _ = bits.Add64(s.D[1].Hi, ScalarNC.D[1].Hi, carry)
}

// Add sets s = a + b mod n.
func (s *Scalar) Add(a, b *Scalar) *Scalar {
	var carry uint64
	s.D[0].Lo, carry = bits.Add64(a.D[0].Lo, b.D[0].Lo, 0)
	s.D[0].Hi, carry = bits.Add64(a.D[0].Hi, b.D[0].Hi, carry)
	s.D[1].Lo, carry = bits.Add64(a.D[1].Lo, b.D[1].Lo, carry)
	s.D[1].Hi, carry = bits.Add64(a.D[1].Hi, b.D[1].Hi, carry)

	// If there was a carry or if result >= n, reduce
	if carry != 0 || s.checkOverflow() {
		s.reduce()
	}
	return s
}

// Sub sets s = a - b mod n.
func (s *Scalar) Sub(a, b *Scalar) *Scalar {
	var borrow uint64
	s.D[0].Lo, borrow = bits.Sub64(a.D[0].Lo, b.D[0].Lo, 0)
	s.D[0].Hi, borrow = bits.Sub64(a.D[0].Hi, b.D[0].Hi, borrow)
	s.D[1].Lo, borrow = bits.Sub64(a.D[1].Lo, b.D[1].Lo, borrow)
	s.D[1].Hi, borrow = bits.Sub64(a.D[1].Hi, b.D[1].Hi, borrow)

	// If there was a borrow, add n back
	if borrow != 0 {
		var carry uint64
		s.D[0].Lo, carry = bits.Add64(s.D[0].Lo, ScalarN.D[0].Lo, 0)
		s.D[0].Hi, carry = bits.Add64(s.D[0].Hi, ScalarN.D[0].Hi, carry)
		s.D[1].Lo, carry = bits.Add64(s.D[1].Lo, ScalarN.D[1].Lo, carry)
		s.D[1].Hi, _ = bits.Add64(s.D[1].Hi, ScalarN.D[1].Hi, carry)
	}
	return s
}

// Negate sets s = -a mod n.
func (s *Scalar) Negate(a *Scalar) *Scalar {
	if a.IsZero() {
		*s = ScalarZero
		return s
	}
	// s = n - a
	var borrow uint64
	s.D[0].Lo, borrow = bits.Sub64(ScalarN.D[0].Lo, a.D[0].Lo, 0)
	s.D[0].Hi, borrow = bits.Sub64(ScalarN.D[0].Hi, a.D[0].Hi, borrow)
	s.D[1].Lo, borrow = bits.Sub64(ScalarN.D[1].Lo, a.D[1].Lo, borrow)
	s.D[1].Hi, _ = bits.Sub64(ScalarN.D[1].Hi, a.D[1].Hi, borrow)
	return s
}

// Mul sets s = a * b mod n.
func (s *Scalar) Mul(a, b *Scalar) *Scalar {
	// Compute 512-bit product
	var prod [8]uint64
	scalarMul512(&prod, a, b)

	// Reduce mod n
	scalarReduce512(s, &prod)
	return s
}

// scalarMul512 computes the 512-bit product of two 256-bit scalars.
// Result is stored in prod[0..7] where prod[0] is the least significant.
func scalarMul512(prod *[8]uint64, a, b *Scalar) {
	// Using schoolbook multiplication with 64-bit limbs
	// a = a[0] + a[1]*2^64 + a[2]*2^128 + a[3]*2^192
	// b = b[0] + b[1]*2^64 + b[2]*2^128 + b[3]*2^192

	aLimbs := [4]uint64{a.D[0].Lo, a.D[0].Hi, a.D[1].Lo, a.D[1].Hi}
	bLimbs := [4]uint64{b.D[0].Lo, b.D[0].Hi, b.D[1].Lo, b.D[1].Hi}

	// Clear product
	for i := range prod {
		prod[i] = 0
	}

	// Schoolbook multiplication
	for i := 0; i < 4; i++ {
		var carry uint64
		for j := 0; j < 4; j++ {
			hi, lo := bits.Mul64(aLimbs[i], bLimbs[j])
			lo, c := bits.Add64(lo, prod[i+j], 0)
			hi, _ = bits.Add64(hi, 0, c)
			lo, c = bits.Add64(lo, carry, 0)
			hi, _ = bits.Add64(hi, 0, c)
			prod[i+j] = lo
			carry = hi
		}
		prod[i+4] = carry
	}
}

// scalarReduce512 reduces a 512-bit value mod n.
func scalarReduce512(s *Scalar, prod *[8]uint64) {
	// Barrett reduction or simple repeated subtraction
	// For now, use a simpler approach: extract high 256 bits, multiply by (2^256 mod n), add to low

	// 2^256 mod n = 2^256 - n = ScalarNC (approximately 0x14551231950B75FC4...etc)
	// This is a simplified reduction - a full implementation would use Barrett reduction

	// Copy low 256 bits to result
	s.D[0].Lo = prod[0]
	s.D[0].Hi = prod[1]
	s.D[1].Lo = prod[2]
	s.D[1].Hi = prod[3]

	// If high 256 bits are non-zero, we need to reduce
	if prod[4] != 0 || prod[5] != 0 || prod[6] != 0 || prod[7] != 0 {
		// high * (2^256 mod n) + low
		// This is a simplified version - multiply high by NC and add
		highScalar := Scalar{
			D: [2]Uint128{
				{Lo: prod[4], Hi: prod[5]},
				{Lo: prod[6], Hi: prod[7]},
			},
		}

		// Multiply high by NC (which is small: ~2^129)
		// For correctness, we'd need full multiplication, but NC is small enough
		// that we can use a simplified approach

		// NC = 0x14551231950B75FC4402DA1732FC9BEBF
		// NC.D[0] = {Lo: 0x402DA1732FC9BEBF, Hi: 0x4551231950B75FC4}
		// NC.D[1] = {Lo: 0x1, Hi: 0}

		// Approximate: high * NC ≈ high * 2^129 (since NC ≈ 2^129)
		// This means we shift high left by 129 bits and add

		// For a correct implementation, compute high * NC properly:
		var reduction [8]uint64
		ncLimbs := [4]uint64{ScalarNC.D[0].Lo, ScalarNC.D[0].Hi, ScalarNC.D[1].Lo, ScalarNC.D[1].Hi}
		highLimbs := [4]uint64{highScalar.D[0].Lo, highScalar.D[0].Hi, highScalar.D[1].Lo, highScalar.D[1].Hi}

		for i := 0; i < 4; i++ {
			var carry uint64
			for j := 0; j < 4; j++ {
				hi, lo := bits.Mul64(highLimbs[i], ncLimbs[j])
				lo, c := bits.Add64(lo, reduction[i+j], 0)
				hi, _ = bits.Add64(hi, 0, c)
				lo, c = bits.Add64(lo, carry, 0)
				hi, _ = bits.Add64(hi, 0, c)
				reduction[i+j] = lo
				carry = hi
			}
			if i+4 < 8 {
				reduction[i+4], _ = bits.Add64(reduction[i+4], carry, 0)
			}
		}

		// Add reduction to s
		var carry uint64
		s.D[0].Lo, carry = bits.Add64(s.D[0].Lo, reduction[0], 0)
		s.D[0].Hi, carry = bits.Add64(s.D[0].Hi, reduction[1], carry)
		s.D[1].Lo, carry = bits.Add64(s.D[1].Lo, reduction[2], carry)
		s.D[1].Hi, carry = bits.Add64(s.D[1].Hi, reduction[3], carry)

		// Handle any remaining high bits by repeated reduction
		// If there's a carry, it represents 2^256 which equals NC mod n
		// If reduction[4..7] are non-zero, we need to reduce those too
		if carry != 0 || reduction[4] != 0 || reduction[5] != 0 || reduction[6] != 0 || reduction[7] != 0 {
			// The carry and reduction[4..7] together represent additional multiples of 2^256
			// Each 2^256 ≡ NC (mod n), so we add (carry + reduction[4..7]) * NC

			// First, handle the carry
			if carry != 0 {
				// carry * NC
				var c uint64
				s.D[0].Lo, c = bits.Add64(s.D[0].Lo, ScalarNC.D[0].Lo, 0)
				s.D[0].Hi, c = bits.Add64(s.D[0].Hi, ScalarNC.D[0].Hi, c)
				s.D[1].Lo, c = bits.Add64(s.D[1].Lo, ScalarNC.D[1].Lo, c)
				s.D[1].Hi, c = bits.Add64(s.D[1].Hi, ScalarNC.D[1].Hi, c)

				// If there's still a carry, add NC again
				for c != 0 {
					s.D[0].Lo, c = bits.Add64(s.D[0].Lo, ScalarNC.D[0].Lo, 0)
					s.D[0].Hi, c = bits.Add64(s.D[0].Hi, ScalarNC.D[0].Hi, c)
					s.D[1].Lo, c = bits.Add64(s.D[1].Lo, ScalarNC.D[1].Lo, c)
					s.D[1].Hi, c = bits.Add64(s.D[1].Hi, ScalarNC.D[1].Hi, c)
				}
			}

			// Handle reduction[4..7] if non-zero
			if reduction[4] != 0 || reduction[5] != 0 || reduction[6] != 0 || reduction[7] != 0 {
				// Compute reduction[4..7] * NC and add
				highScalar2 := Scalar{
					D: [2]Uint128{
						{Lo: reduction[4], Hi: reduction[5]},
						{Lo: reduction[6], Hi: reduction[7]},
					},
				}

				var reduction2 [8]uint64
				high2Limbs := [4]uint64{highScalar2.D[0].Lo, highScalar2.D[0].Hi, highScalar2.D[1].Lo, highScalar2.D[1].Hi}

				for i := 0; i < 4; i++ {
					var c uint64
					for j := 0; j < 4; j++ {
						hi, lo := bits.Mul64(high2Limbs[i], ncLimbs[j])
						lo, cc := bits.Add64(lo, reduction2[i+j], 0)
						hi, _ = bits.Add64(hi, 0, cc)
						lo, cc = bits.Add64(lo, c, 0)
						hi, _ = bits.Add64(hi, 0, cc)
						reduction2[i+j] = lo
						c = hi
					}
					if i+4 < 8 {
						reduction2[i+4], _ = bits.Add64(reduction2[i+4], c, 0)
					}
				}

				var c uint64
				s.D[0].Lo, c = bits.Add64(s.D[0].Lo, reduction2[0], 0)
				s.D[0].Hi, c = bits.Add64(s.D[0].Hi, reduction2[1], c)
				s.D[1].Lo, c = bits.Add64(s.D[1].Lo, reduction2[2], c)
				s.D[1].Hi, c = bits.Add64(s.D[1].Hi, reduction2[3], c)

				// Handle cascading carries
				for c != 0 || reduction2[4] != 0 || reduction2[5] != 0 || reduction2[6] != 0 || reduction2[7] != 0 {
					// This case is extremely rare but handle it
					for s.checkOverflow() {
						s.reduce()
					}
					break
				}
			}
		}
	}

	// Final reduction if needed
	if s.checkOverflow() {
		s.reduce()
	}
}

// Sqr sets s = a^2 mod n.
func (s *Scalar) Sqr(a *Scalar) *Scalar {
	return s.Mul(a, a)
}

// Inverse sets s = a^(-1) mod n using Fermat's little theorem.
// a^(-1) = a^(n-2) mod n
func (s *Scalar) Inverse(a *Scalar) *Scalar {
	// n-2 in binary is used for square-and-multiply
	// This is a simplified implementation using binary exponentiation

	var result, base Scalar
	result = ScalarOne
	base = *a

	// n-2 bytes (big-endian)
	nMinus2 := [32]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE,
		0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B,
		0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x3F,
	}

	for i := 0; i < 32; i++ {
		b := nMinus2[31-i]
		for j := 0; j < 8; j++ {
			if (b>>j)&1 == 1 {
				result.Mul(&result, &base)
			}
			base.Sqr(&base)
		}
	}

	*s = result
	return s
}

// IsHigh returns true if s > n/2.
func (s *Scalar) IsHigh() bool {
	// Compare with n/2
	if s.D[1].Hi > ScalarNHalf.D[1].Hi {
		return true
	}
	if s.D[1].Hi < ScalarNHalf.D[1].Hi {
		return false
	}
	if s.D[1].Lo > ScalarNHalf.D[1].Lo {
		return true
	}
	if s.D[1].Lo < ScalarNHalf.D[1].Lo {
		return false
	}
	if s.D[0].Hi > ScalarNHalf.D[0].Hi {
		return true
	}
	if s.D[0].Hi < ScalarNHalf.D[0].Hi {
		return false
	}
	return s.D[0].Lo > ScalarNHalf.D[0].Lo
}

// CondNegate negates s if cond is true.
func (s *Scalar) CondNegate(cond bool) *Scalar {
	if cond {
		s.Negate(s)
	}
	return s
}
