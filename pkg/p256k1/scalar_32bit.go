//go:build js || wasm || tinygo || wasm32

// Copyright (c) 2024 mleku
// Adapted from github.com/decred/dcrd/dcrec/secp256k1/v4
// Copyright (c) 2020-2024 The Decred developers

package p256k1

import (
	"crypto/subtle"
	"unsafe"
)

// Scalar represents a scalar value modulo the secp256k1 group order.
// This implementation uses 8 uint32 limbs in base 2^32, optimized for 32-bit platforms.
type Scalar struct {
	n [8]uint32
}

// Scalar constants in 8x32 representation
const (
	// Order words (from least to most significant)
	orderWord0 uint32 = 0xd0364141
	orderWord1 uint32 = 0xbfd25e8c
	orderWord2 uint32 = 0xaf48a03b
	orderWord3 uint32 = 0xbaaedce6
	orderWord4 uint32 = 0xfffffffe
	orderWord5 uint32 = 0xffffffff
	orderWord6 uint32 = 0xffffffff
	orderWord7 uint32 = 0xffffffff

	// Two's complement of order (for reduction)
	orderCompWord0 uint32 = 0x2fc9bebf // ~orderWord0 + 1
	orderCompWord1 uint32 = 0x402da173 // ~orderWord1
	orderCompWord2 uint32 = 0x50b75fc4 // ~orderWord2
	orderCompWord3 uint32 = 0x45512319 // ~orderWord3

	// Half order words
	halfOrderWord0 uint32 = 0x681b20a0
	halfOrderWord1 uint32 = 0xdfe92f46
	halfOrderWord2 uint32 = 0x57a4501d
	halfOrderWord3 uint32 = 0x5d576e73
	halfOrderWord4 uint32 = 0xffffffff
	halfOrderWord5 uint32 = 0xffffffff
	halfOrderWord6 uint32 = 0xffffffff
	halfOrderWord7 uint32 = 0x7fffffff

	uint32Mask = 0xffffffff
)

// Scalar element constants
var (
	ScalarZero = Scalar{n: [8]uint32{0, 0, 0, 0, 0, 0, 0, 0}}
	ScalarOne  = Scalar{n: [8]uint32{1, 0, 0, 0, 0, 0, 0, 0}}

	// GLV constants in 8x32 representation
	scalarLambda = Scalar{
		n: [8]uint32{
			0x1b23bd72, 0xdf02967c, 0x20816678, 0x122e22ea,
			0x8812645a, 0xa5261c02, 0xc05c30e0, 0x5363ad4c,
		},
	}

	scalarMinusB1 = Scalar{
		n: [8]uint32{
			0x0abfe4c3, 0x6f547fa9, 0x010e8828, 0xe4437ed6,
			0x00000000, 0x00000000, 0x00000000, 0x00000000,
		},
	}

	scalarMinusB2 = Scalar{
		n: [8]uint32{
			0x3db1562c, 0xd765cda8, 0x0774346d, 0x8a280ac5,
			0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff,
		},
	}

	scalarG1 = Scalar{
		n: [8]uint32{
			0x45dbb031, 0xe893209a, 0x71e8ca7f, 0x3daa8a14,
			0x9284eb15, 0xe86c90e4, 0xa7d46bcd, 0x3086d221,
		},
	}

	scalarG2 = Scalar{
		n: [8]uint32{
			0x8ac47f71, 0x1571b4ae, 0x9df506c6, 0x221208ac,
			0x0abfe4c4, 0x6f547fa9, 0x010e8828, 0xe4437ed6,
		},
	}
)

// setInt sets a scalar to a small integer value
func (s *Scalar) setInt(v uint) {
	s.n[0] = uint32(v)
	for i := 1; i < 8; i++ {
		s.n[i] = 0
	}
}

// setB32 sets a scalar from a 32-byte big-endian array
func (s *Scalar) setB32(b []byte) bool {
	if len(b) != 32 {
		panic("scalar byte array must be 32 bytes")
	}

	s.n[0] = uint32(b[31]) | uint32(b[30])<<8 | uint32(b[29])<<16 | uint32(b[28])<<24
	s.n[1] = uint32(b[27]) | uint32(b[26])<<8 | uint32(b[25])<<16 | uint32(b[24])<<24
	s.n[2] = uint32(b[23]) | uint32(b[22])<<8 | uint32(b[21])<<16 | uint32(b[20])<<24
	s.n[3] = uint32(b[19]) | uint32(b[18])<<8 | uint32(b[17])<<16 | uint32(b[16])<<24
	s.n[4] = uint32(b[15]) | uint32(b[14])<<8 | uint32(b[13])<<16 | uint32(b[12])<<24
	s.n[5] = uint32(b[11]) | uint32(b[10])<<8 | uint32(b[9])<<16 | uint32(b[8])<<24
	s.n[6] = uint32(b[7]) | uint32(b[6])<<8 | uint32(b[5])<<16 | uint32(b[4])<<24
	s.n[7] = uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24

	overflow := s.overflows()
	s.reduce256(overflow)
	return overflow != 0
}

// setB32Seckey sets a scalar from a 32-byte secret key, returns true if valid
func (s *Scalar) setB32Seckey(b []byte) bool {
	overflow := s.setB32(b)
	return !s.isZero() && !overflow
}

// getB32 converts a scalar to a 32-byte big-endian array
func (s *Scalar) getB32(b []byte) {
	if len(b) != 32 {
		panic("scalar byte array must be 32 bytes")
	}

	b[31] = byte(s.n[0])
	b[30] = byte(s.n[0] >> 8)
	b[29] = byte(s.n[0] >> 16)
	b[28] = byte(s.n[0] >> 24)
	b[27] = byte(s.n[1])
	b[26] = byte(s.n[1] >> 8)
	b[25] = byte(s.n[1] >> 16)
	b[24] = byte(s.n[1] >> 24)
	b[23] = byte(s.n[2])
	b[22] = byte(s.n[2] >> 8)
	b[21] = byte(s.n[2] >> 16)
	b[20] = byte(s.n[2] >> 24)
	b[19] = byte(s.n[3])
	b[18] = byte(s.n[3] >> 8)
	b[17] = byte(s.n[3] >> 16)
	b[16] = byte(s.n[3] >> 24)
	b[15] = byte(s.n[4])
	b[14] = byte(s.n[4] >> 8)
	b[13] = byte(s.n[4] >> 16)
	b[12] = byte(s.n[4] >> 24)
	b[11] = byte(s.n[5])
	b[10] = byte(s.n[5] >> 8)
	b[9] = byte(s.n[5] >> 16)
	b[8] = byte(s.n[5] >> 24)
	b[7] = byte(s.n[6])
	b[6] = byte(s.n[6] >> 8)
	b[5] = byte(s.n[6] >> 16)
	b[4] = byte(s.n[6] >> 24)
	b[3] = byte(s.n[7])
	b[2] = byte(s.n[7] >> 8)
	b[1] = byte(s.n[7] >> 16)
	b[0] = byte(s.n[7] >> 24)
}

// overflows determines if the scalar >= order
func (s *Scalar) overflows() uint32 {
	highWordsEqual := constantTimeEq32(s.n[7], orderWord7)
	highWordsEqual &= constantTimeEq32(s.n[6], orderWord6)
	highWordsEqual &= constantTimeEq32(s.n[5], orderWord5)
	overflow := highWordsEqual & constantTimeGreater32(s.n[4], orderWord4)
	highWordsEqual &= constantTimeEq32(s.n[4], orderWord4)
	overflow |= highWordsEqual & constantTimeGreater32(s.n[3], orderWord3)
	highWordsEqual &= constantTimeEq32(s.n[3], orderWord3)
	overflow |= highWordsEqual & constantTimeGreater32(s.n[2], orderWord2)
	highWordsEqual &= constantTimeEq32(s.n[2], orderWord2)
	overflow |= highWordsEqual & constantTimeGreater32(s.n[1], orderWord1)
	highWordsEqual &= constantTimeEq32(s.n[1], orderWord1)
	overflow |= highWordsEqual & constantTimeGreaterOrEq32(s.n[0], orderWord0)
	return overflow
}

// reduce256 reduces the scalar modulo the order
func (s *Scalar) reduce256(overflows uint32) {
	overflows64 := uint64(overflows)
	c := uint64(s.n[0]) + overflows64*uint64(orderCompWord0)
	s.n[0] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[1]) + overflows64*uint64(orderCompWord1)
	s.n[1] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[2]) + overflows64*uint64(orderCompWord2)
	s.n[2] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[3]) + overflows64*uint64(orderCompWord3)
	s.n[3] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[4]) + overflows64
	s.n[4] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[5])
	s.n[5] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[6])
	s.n[6] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(s.n[7])
	s.n[7] = uint32(c & uint32Mask)
}

// checkOverflow checks if the scalar overflows
func (s *Scalar) checkOverflow() bool {
	return s.overflows() != 0
}

// reduce reduces the scalar modulo the order
func (s *Scalar) reduce(overflow int) {
	s.reduce256(uint32(overflow))
}

// add adds two scalars: r = a + b
func (s *Scalar) add(a, b *Scalar) bool {
	c := uint64(a.n[0]) + uint64(b.n[0])
	s.n[0] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[1]) + uint64(b.n[1])
	s.n[1] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[2]) + uint64(b.n[2])
	s.n[2] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[3]) + uint64(b.n[3])
	s.n[3] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[4]) + uint64(b.n[4])
	s.n[4] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[5]) + uint64(b.n[5])
	s.n[5] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[6]) + uint64(b.n[6])
	s.n[6] = uint32(c & uint32Mask)
	c = (c >> 32) + uint64(a.n[7]) + uint64(b.n[7])
	s.n[7] = uint32(c & uint32Mask)

	s.reduce256(uint32(c>>32) + s.overflows())
	return false
}

// addPureGo is an alias for add in 32-bit mode
func (s *Scalar) addPureGo(a, b *Scalar) bool {
	return s.add(a, b)
}

// sub subtracts two scalars: r = a - b
func (s *Scalar) sub(a, b *Scalar) {
	var negB Scalar
	negB.negate(b)
	s.add(a, &negB)
}

// subPureGo is an alias for sub in 32-bit mode
func (s *Scalar) subPureGo(a, b *Scalar) {
	s.sub(a, b)
}

// negate negates a scalar
func (s *Scalar) negate(a *Scalar) {
	bits := a.n[0] | a.n[1] | a.n[2] | a.n[3] | a.n[4] | a.n[5] | a.n[6] | a.n[7]
	mask := uint64(uint32Mask * constantTimeNotEq32(bits, 0))
	c := uint64(orderWord0) + (uint64(^a.n[0]) + 1)
	s.n[0] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord1) + uint64(^a.n[1])
	s.n[1] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord2) + uint64(^a.n[2])
	s.n[2] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord3) + uint64(^a.n[3])
	s.n[3] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord4) + uint64(^a.n[4])
	s.n[4] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord5) + uint64(^a.n[5])
	s.n[5] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord6) + uint64(^a.n[6])
	s.n[6] = uint32(c & mask)
	c = (c >> 32) + uint64(orderWord7) + uint64(^a.n[7])
	s.n[7] = uint32(c & mask)
}

// mul multiplies two scalars: r = a * b
func (s *Scalar) mul(a, b *Scalar) {
	s.mulPureGo(a, b)
}

// mulPureGo performs multiplication using 32-bit arithmetic
func (s *Scalar) mulPureGo(a, b *Scalar) {
	// Compute 512-bit product then reduce
	var l [16]uint64

	// Full 512-bit multiplication (using 64-bit intermediates for 32x32->64)
	for i := 0; i < 8; i++ {
		var c uint64
		for j := 0; j < 8; j++ {
			c += l[i+j] + uint64(a.n[i])*uint64(b.n[j])
			l[i+j] = c & uint32Mask
			c >>= 32
		}
		l[i+8] = c
	}

	// Reduce 512 bits to 256 bits modulo order
	s.reduce512_32(l[:])
}

// reduce512_32 reduces a 512-bit value modulo the order (32-bit version)
func (s *Scalar) reduce512_32(l []uint64) {
	// First reduction: 512 -> 385 bits
	var m [13]uint64
	var c uint64

	c = l[0] + l[8]*uint64(orderCompWord0)
	m[0] = c & uint32Mask
	c >>= 32
	c += l[1] + l[8]*uint64(orderCompWord1) + l[9]*uint64(orderCompWord0)
	m[1] = c & uint32Mask
	c >>= 32
	c += l[2] + l[8]*uint64(orderCompWord2) + l[9]*uint64(orderCompWord1) + l[10]*uint64(orderCompWord0)
	m[2] = c & uint32Mask
	c >>= 32
	c += l[3] + l[8]*uint64(orderCompWord3) + l[9]*uint64(orderCompWord2) + l[10]*uint64(orderCompWord1) + l[11]*uint64(orderCompWord0)
	m[3] = c & uint32Mask
	c >>= 32
	c += l[4] + l[8] + l[9]*uint64(orderCompWord3) + l[10]*uint64(orderCompWord2) + l[11]*uint64(orderCompWord1) + l[12]*uint64(orderCompWord0)
	m[4] = c & uint32Mask
	c >>= 32
	c += l[5] + l[9] + l[10]*uint64(orderCompWord3) + l[11]*uint64(orderCompWord2) + l[12]*uint64(orderCompWord1) + l[13]*uint64(orderCompWord0)
	m[5] = c & uint32Mask
	c >>= 32
	c += l[6] + l[10] + l[11]*uint64(orderCompWord3) + l[12]*uint64(orderCompWord2) + l[13]*uint64(orderCompWord1) + l[14]*uint64(orderCompWord0)
	m[6] = c & uint32Mask
	c >>= 32
	c += l[7] + l[11] + l[12]*uint64(orderCompWord3) + l[13]*uint64(orderCompWord2) + l[14]*uint64(orderCompWord1) + l[15]*uint64(orderCompWord0)
	m[7] = c & uint32Mask
	c >>= 32
	c += l[12] + l[13]*uint64(orderCompWord3) + l[14]*uint64(orderCompWord2) + l[15]*uint64(orderCompWord1)
	m[8] = c & uint32Mask
	c >>= 32
	c += l[13] + l[14]*uint64(orderCompWord3) + l[15]*uint64(orderCompWord2)
	m[9] = c & uint32Mask
	c >>= 32
	c += l[14] + l[15]*uint64(orderCompWord3)
	m[10] = c & uint32Mask
	c >>= 32
	c += l[15]
	m[11] = c & uint32Mask
	c >>= 32
	m[12] = c

	// Second reduction: 385 -> 258 bits
	var p [9]uint64
	c = m[0] + m[8]*uint64(orderCompWord0)
	p[0] = c & uint32Mask
	c >>= 32
	c += m[1] + m[8]*uint64(orderCompWord1) + m[9]*uint64(orderCompWord0)
	p[1] = c & uint32Mask
	c >>= 32
	c += m[2] + m[8]*uint64(orderCompWord2) + m[9]*uint64(orderCompWord1) + m[10]*uint64(orderCompWord0)
	p[2] = c & uint32Mask
	c >>= 32
	c += m[3] + m[8]*uint64(orderCompWord3) + m[9]*uint64(orderCompWord2) + m[10]*uint64(orderCompWord1) + m[11]*uint64(orderCompWord0)
	p[3] = c & uint32Mask
	c >>= 32
	c += m[4] + m[8] + m[9]*uint64(orderCompWord3) + m[10]*uint64(orderCompWord2) + m[11]*uint64(orderCompWord1) + m[12]*uint64(orderCompWord0)
	p[4] = c & uint32Mask
	c >>= 32
	c += m[5] + m[9] + m[10]*uint64(orderCompWord3) + m[11]*uint64(orderCompWord2) + m[12]*uint64(orderCompWord1)
	p[5] = c & uint32Mask
	c >>= 32
	c += m[6] + m[10] + m[11]*uint64(orderCompWord3) + m[12]*uint64(orderCompWord2)
	p[6] = c & uint32Mask
	c >>= 32
	c += m[7] + m[11] + m[12]*uint64(orderCompWord3)
	p[7] = c & uint32Mask
	c >>= 32
	p[8] = c + m[12]

	// Final reduction: 258 -> 256 bits
	c = p[0] + p[8]*uint64(orderCompWord0)
	s.n[0] = uint32(c & uint32Mask)
	c >>= 32
	c += p[1] + p[8]*uint64(orderCompWord1)
	s.n[1] = uint32(c & uint32Mask)
	c >>= 32
	c += p[2] + p[8]*uint64(orderCompWord2)
	s.n[2] = uint32(c & uint32Mask)
	c >>= 32
	c += p[3] + p[8]*uint64(orderCompWord3)
	s.n[3] = uint32(c & uint32Mask)
	c >>= 32
	c += p[4] + p[8]
	s.n[4] = uint32(c & uint32Mask)
	c >>= 32
	c += p[5]
	s.n[5] = uint32(c & uint32Mask)
	c >>= 32
	c += p[6]
	s.n[6] = uint32(c & uint32Mask)
	c >>= 32
	c += p[7]
	s.n[7] = uint32(c & uint32Mask)

	s.reduce256(uint32(c>>32) + s.overflows())
}

// inverse computes the modular inverse
func (s *Scalar) inverse(a *Scalar) {
	// Use Fermat's little theorem: a^(-1) = a^(n-2) mod n
	var exp Scalar
	exp.n[0] = orderWord0 - 2
	exp.n[1] = orderWord1
	exp.n[2] = orderWord2
	exp.n[3] = orderWord3
	exp.n[4] = orderWord4
	exp.n[5] = orderWord5
	exp.n[6] = orderWord6
	exp.n[7] = orderWord7

	s.exp(a, &exp)
}

// exp computes s = a^b mod n
func (s *Scalar) exp(a, b *Scalar) {
	*s = ScalarOne
	base := *a

	for i := 0; i < 8; i++ {
		limb := b.n[i]
		for j := 0; j < 32; j++ {
			if limb&1 != 0 {
				s.mul(s, &base)
			}
			base.mul(&base, &base)
			limb >>= 1
		}
	}
}

// half computes s = a/2 mod n
func (s *Scalar) half(a *Scalar) {
	*s = *a
	if s.n[0]&1 == 0 {
		// Even: simple right shift
		for i := 0; i < 7; i++ {
			s.n[i] = (s.n[i] >> 1) | ((s.n[i+1] & 1) << 31)
		}
		s.n[7] >>= 1
	} else {
		// Odd: add n then divide by 2
		var c uint64
		c = uint64(s.n[0]) + uint64(orderWord0)
		s.n[0] = uint32(c)
		c = (c >> 32) + uint64(s.n[1]) + uint64(orderWord1)
		s.n[1] = uint32(c)
		c = (c >> 32) + uint64(s.n[2]) + uint64(orderWord2)
		s.n[2] = uint32(c)
		c = (c >> 32) + uint64(s.n[3]) + uint64(orderWord3)
		s.n[3] = uint32(c)
		c = (c >> 32) + uint64(s.n[4]) + uint64(orderWord4)
		s.n[4] = uint32(c)
		c = (c >> 32) + uint64(s.n[5]) + uint64(orderWord5)
		s.n[5] = uint32(c)
		c = (c >> 32) + uint64(s.n[6]) + uint64(orderWord6)
		s.n[6] = uint32(c)
		c = (c >> 32) + uint64(s.n[7]) + uint64(orderWord7)
		s.n[7] = uint32(c)

		// Divide by 2
		for i := 0; i < 7; i++ {
			s.n[i] = (s.n[i] >> 1) | ((s.n[i+1] & 1) << 31)
		}
		s.n[7] >>= 1
	}
}

// isZero returns true if the scalar is zero
func (s *Scalar) isZero() bool {
	bits := s.n[0] | s.n[1] | s.n[2] | s.n[3] | s.n[4] | s.n[5] | s.n[6] | s.n[7]
	return bits == 0
}

// isOne returns true if the scalar is one
func (s *Scalar) isOne() bool {
	return s.n[0] == 1 && s.n[1] == 0 && s.n[2] == 0 && s.n[3] == 0 &&
		s.n[4] == 0 && s.n[5] == 0 && s.n[6] == 0 && s.n[7] == 0
}

// isEven returns true if the scalar is even
func (s *Scalar) isEven() bool {
	return s.n[0]&1 == 0
}

// isHigh returns true if the scalar is > n/2
func (s *Scalar) isHigh() bool {
	result := constantTimeGreater32(s.n[7], halfOrderWord7)
	highWordsEqual := constantTimeEq32(s.n[7], halfOrderWord7)
	highWordsEqual &= constantTimeEq32(s.n[6], halfOrderWord6)
	highWordsEqual &= constantTimeEq32(s.n[5], halfOrderWord5)
	highWordsEqual &= constantTimeEq32(s.n[4], halfOrderWord4)
	result |= highWordsEqual & constantTimeGreater32(s.n[3], halfOrderWord3)
	highWordsEqual &= constantTimeEq32(s.n[3], halfOrderWord3)
	result |= highWordsEqual & constantTimeGreater32(s.n[2], halfOrderWord2)
	highWordsEqual &= constantTimeEq32(s.n[2], halfOrderWord2)
	result |= highWordsEqual & constantTimeGreater32(s.n[1], halfOrderWord1)
	highWordsEqual &= constantTimeEq32(s.n[1], halfOrderWord1)
	result |= highWordsEqual & constantTimeGreater32(s.n[0], halfOrderWord0)
	return result != 0
}

// condNegate conditionally negates the scalar
func (s *Scalar) condNegate(flag int) {
	if flag != 0 {
		var neg Scalar
		neg.negate(s)
		*s = neg
	}
}

// equal returns true if two scalars are equal
func (s *Scalar) equal(a *Scalar) bool {
	return subtle.ConstantTimeCompare(
		(*[32]byte)(unsafe.Pointer(&s.n[0]))[:32],
		(*[32]byte)(unsafe.Pointer(&a.n[0]))[:32],
	) == 1
}

// getBits extracts count bits starting at offset
func (s *Scalar) getBits(offset, count uint) uint32 {
	if count == 0 || count > 32 {
		panic("count must be 1-32")
	}
	if offset+count > 256 {
		panic("offset + count must be <= 256")
	}

	limbIdx := offset / 32
	bitIdx := offset % 32

	if bitIdx+count <= 32 {
		return (s.n[limbIdx] >> bitIdx) & ((1 << count) - 1)
	}
	lowBits := 32 - bitIdx
	highBits := count - lowBits
	low := (s.n[limbIdx] >> bitIdx) & ((1 << lowBits) - 1)
	high := s.n[limbIdx+1] & ((1 << highBits) - 1)
	return low | (high << lowBits)
}

// cmov conditionally moves a scalar
func (s *Scalar) cmov(a *Scalar, flag int) {
	mask := uint32(-(int32(flag) & 1))
	for i := 0; i < 8; i++ {
		s.n[i] ^= mask & (s.n[i] ^ a.n[i])
	}
}

// clear clears a scalar
func (s *Scalar) clear() {
	for i := 0; i < 8; i++ {
		s.n[i] = 0
	}
}

// wNAF converts a scalar to wNAF representation
func (s *Scalar) wNAF(wnaf *[257]int8, w uint) int {
	if w < 2 || w > 8 {
		panic("w must be between 2 and 8")
	}

	var k Scalar
	k = *s

	numBits := 0
	var carry uint32

	*wnaf = [257]int8{}

	bit := 0
	for bit < 256 {
		if k.getBits(uint(bit), 1) == carry {
			bit++
			continue
		}

		window := w
		if bit+int(window) > 256 {
			window = uint(256 - bit)
		}

		word := k.getBits(uint(bit), window) + carry
		carry = (word >> (window - 1)) & 1
		word -= carry << window

		wnaf[bit] = int8(int32(word))
		numBits = bit + int(window) - 1

		bit += int(window)
	}

	if carry != 0 {
		wnaf[256] = int8(carry)
		numBits = 256
	}

	return numBits + 1
}

// wNAFSigned converts a scalar to wNAF representation with sign handling
func (s *Scalar) wNAFSigned(wnaf *[257]int8, w uint) (int, bool) {
	if w < 2 || w > 8 {
		panic("w must be between 2 and 8")
	}

	var k Scalar
	k = *s

	negated := false
	if k.getBits(255, 1) == 1 {
		k.negate(&k)
		negated = true
	}

	bits := k.wNAF(wnaf, w)
	return bits, negated
}

// caddBit conditionally adds a power of 2
func (s *Scalar) caddBit(bit uint, flag int) {
	if flag == 0 {
		return
	}

	limbIdx := bit >> 5
	bitIdx := bit & 0x1F
	addVal := uint32(1) << bitIdx

	var c uint64
	for i := limbIdx; i < 8; i++ {
		if i == limbIdx {
			c = uint64(s.n[i]) + uint64(addVal)
		} else {
			c = uint64(s.n[i]) + (c >> 32)
		}
		s.n[i] = uint32(c)
		if c>>32 == 0 {
			break
		}
	}
}

// mulShiftVar computes r = round((a * b) >> shift)
func (s *Scalar) mulShiftVar(a, b *Scalar, shift uint) {
	if shift < 256 {
		panic("mulShiftVar requires shift >= 256")
	}

	// Compute full 512-bit product
	var l [16]uint64
	for i := 0; i < 8; i++ {
		var c uint64
		for j := 0; j < 8; j++ {
			c += l[i+j] + uint64(a.n[i])*uint64(b.n[j])
			l[i+j] = c & uint32Mask
			c >>= 32
		}
		l[i+8] = c
	}

	// Extract bits [shift, shift+256)
	shiftLimbs := shift >> 5
	shiftLow := shift & 0x1F
	shiftHigh := 32 - shiftLow

	for i := 0; i < 8; i++ {
		srcIdx := shiftLimbs + uint(i)
		if srcIdx < 16 {
			if shiftLow != 0 && srcIdx+1 < 16 {
				s.n[i] = uint32((l[srcIdx] >> shiftLow) | (l[srcIdx+1] << shiftHigh))
			} else {
				s.n[i] = uint32(l[srcIdx] >> shiftLow)
			}
		} else {
			s.n[i] = 0
		}
	}

	// Round by adding bit just below shift
	roundBit := int((l[(shift-1)>>5] >> ((shift - 1) & 0x1F)) & 1)
	s.caddBit(0, roundBit)
}

// Constant-time helper functions
func constantTimeNotEq32(a, b uint32) uint32 {
	return ^constantTimeEq32(a, b) & 1
}

func constantTimeGreaterOrEq32(a, b uint32) uint32 {
	return uint32((uint64(a) - uint64(b) - 1) >> 63) ^ 1
}

// scalarSplitLambda decomposes k into k1, k2 for GLV
func scalarSplitLambda(r1, r2, k *Scalar) {
	var c1, c2 Scalar

	c1.mulShiftVar(k, &scalarG1, 384)
	c2.mulShiftVar(k, &scalarG2, 384)

	c1.mul(&c1, &scalarMinusB1)
	c2.mul(&c2, &scalarMinusB2)

	r2.add(&c1, &c2)
	r1.mul(r2, &scalarLambda)
	r1.negate(r1)
	r1.add(r1, k)
}

// scalarSplit128 splits a scalar into two 128-bit halves
func scalarSplit128(r1, r2, k *Scalar) {
	r1.n[0] = k.n[0]
	r1.n[1] = k.n[1]
	r1.n[2] = k.n[2]
	r1.n[3] = k.n[3]
	r1.n[4] = 0
	r1.n[5] = 0
	r1.n[6] = 0
	r1.n[7] = 0

	r2.n[0] = k.n[4]
	r2.n[1] = k.n[5]
	r2.n[2] = k.n[6]
	r2.n[3] = k.n[7]
	r2.n[4] = 0
	r2.n[5] = 0
	r2.n[6] = 0
	r2.n[7] = 0
}

// Direct function versions for compatibility
func scalarAdd(r, a, b *Scalar) bool  { return r.add(a, b) }
func scalarMul(r, a, b *Scalar)       { r.mul(a, b) }
func scalarGetB32(bin []byte, a *Scalar) { a.getB32(bin) }
func scalarIsZero(a *Scalar) bool     { return a.isZero() }
func scalarCheckOverflow(r *Scalar) bool { return r.checkOverflow() }
func scalarReduce(r *Scalar, overflow int) { r.reduce(overflow) }

// Stubs for AVX2 functions (not available on WASM) - these forward to pure Go
func scalarAddAVX2(r, a, b *Scalar) { r.add(a, b) }
func scalarSubAVX2(r, a, b *Scalar) { r.sub(a, b) }
func scalarMulAVX2(r, a, b *Scalar) { r.mul(a, b) }

// Compatibility constants for verify.go (these map to our 8x32 representation)
const (
	scalarN0 = uint64(orderWord0) | uint64(orderWord1)<<32
	scalarN1 = uint64(orderWord2) | uint64(orderWord3)<<32
	scalarN2 = uint64(orderWord4) | uint64(orderWord5)<<32
	scalarN3 = uint64(orderWord6) | uint64(orderWord7)<<32
)

// d returns the scalar limbs as a 4-element uint64 array for compatibility
// This converts from 8x32 to 4x64 representation
func (s *Scalar) d() [4]uint64 {
	return [4]uint64{
		uint64(s.n[0]) | uint64(s.n[1])<<32,
		uint64(s.n[2]) | uint64(s.n[3])<<32,
		uint64(s.n[4]) | uint64(s.n[5])<<32,
		uint64(s.n[6]) | uint64(s.n[7])<<32,
	}
}
