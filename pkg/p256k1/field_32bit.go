//go:build js || wasm || tinygo || wasm32

// Copyright (c) 2024 mleku
// Adapted from github.com/decred/dcrd/dcrec/secp256k1/v4
// Copyright (c) 2013-2024 The btcsuite developers
// Copyright (c) 2015-2024 The Decred developers

package p256k1

import (
	"crypto/subtle"
	"errors"
	"unsafe"
)

// FieldElement represents a field element modulo the secp256k1 field prime.
// This implementation uses 10 uint32 limbs in base 2^26, optimized for 32-bit platforms.
//
// The representation provides 6 bits of overflow in each word (10 bits in the MSB)
// for a total of 64 bits of overflow, allowing intermediate calculations without
// immediate carry propagation.
type FieldElement struct {
	// n represents the sum(i=0..9, n[i] << (i*26)) mod p
	// where p is the field modulus, 2^256 - 2^32 - 977
	n [10]uint32

	magnitude  int  // magnitude of the field element (max 32)
	normalized bool // whether the field element is normalized
}

// FieldElementStorage represents a field element in storage format (4 uint64 limbs)
type FieldElementStorage struct {
	n [4]uint64
}

// Field constants for 10x26 representation
const (
	// Bit masks
	twoBitsMask   = 0x3
	fourBitsMask  = 0xf
	sixBitsMask   = 0x3f
	eightBitsMask = 0xff

	// fieldBase is the exponent used to form the numeric base of each word.
	fieldBase32 = 26

	// fieldBaseMask is the mask for the bits in each word (except MSB)
	fieldBaseMask32 = (1 << fieldBase32) - 1

	// fieldMSBBits is the number of bits in the most significant word
	fieldMSBBits32 = 256 - (fieldBase32 * 9) // 256 - 234 = 22

	// fieldMSBMask is the mask for the bits in the MSB
	fieldMSBMask32 = (1 << fieldMSBBits32) - 1 // 0x3fffff

	// Prime field words in 10x26 representation
	fieldPrimeWord0 = 0x03fffc2f
	fieldPrimeWord1 = 0x03ffffbf
	fieldPrimeWord2 = 0x03ffffff
	fieldPrimeWord3 = 0x03ffffff
	fieldPrimeWord4 = 0x03ffffff
	fieldPrimeWord5 = 0x03ffffff
	fieldPrimeWord6 = 0x03ffffff
	fieldPrimeWord7 = 0x03ffffff
	fieldPrimeWord8 = 0x03ffffff
	fieldPrimeWord9 = 0x003fffff
)

// Field element constants
var (
	// FieldElementOne represents the field element 1
	FieldElementOne = FieldElement{
		n:          [10]uint32{1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		magnitude:  1,
		normalized: true,
	}

	// FieldElementZero represents the field element 0
	FieldElementZero = FieldElement{
		n:          [10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		magnitude:  0,
		normalized: true,
	}

	// fieldBeta is the GLV endomorphism constant β (cube root of unity mod p)
	// Value: 0x7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee
	fieldBeta = FieldElement{
		n: [10]uint32{
			0x0019501ee, // n[0]
			0x004e5662c, // n[1]
			0x022d7e625, // n[2]
			0x00f51d3c0, // n[3]
			0x0334cd1a, // n[4]
			0x007b23d12, // n[5]
			0x0311c923, // n[6]
			0x02c1957b9, // n[7]
			0x0109ab65a, // n[8]
			0x001eb94c, // n[9]
		},
		magnitude:  1,
		normalized: true,
	}
)

func NewFieldElement() *FieldElement {
	return &FieldElement{
		n:          [10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		magnitude:  0,
		normalized: true,
	}
}

// setB32 sets a field element from a 32-byte big-endian array
func (f *FieldElement) setB32(b []byte) error {
	if len(b) != 32 {
		return errors.New("field element byte array must be 32 bytes")
	}

	// Pack the 256 total bits across the 10 uint32 words with max 26 bits per word
	f.n[0] = uint32(b[31]) | uint32(b[30])<<8 | uint32(b[29])<<16 |
		(uint32(b[28])&twoBitsMask)<<24
	f.n[1] = uint32(b[28])>>2 | uint32(b[27])<<6 | uint32(b[26])<<14 |
		(uint32(b[25])&fourBitsMask)<<22
	f.n[2] = uint32(b[25])>>4 | uint32(b[24])<<4 | uint32(b[23])<<12 |
		(uint32(b[22])&sixBitsMask)<<20
	f.n[3] = uint32(b[22])>>6 | uint32(b[21])<<2 | uint32(b[20])<<10 |
		uint32(b[19])<<18
	f.n[4] = uint32(b[18]) | uint32(b[17])<<8 | uint32(b[16])<<16 |
		(uint32(b[15])&twoBitsMask)<<24
	f.n[5] = uint32(b[15])>>2 | uint32(b[14])<<6 | uint32(b[13])<<14 |
		(uint32(b[12])&fourBitsMask)<<22
	f.n[6] = uint32(b[12])>>4 | uint32(b[11])<<4 | uint32(b[10])<<12 |
		(uint32(b[9])&sixBitsMask)<<20
	f.n[7] = uint32(b[9])>>6 | uint32(b[8])<<2 | uint32(b[7])<<10 |
		uint32(b[6])<<18
	f.n[8] = uint32(b[5]) | uint32(b[4])<<8 | uint32(b[3])<<16 |
		(uint32(b[2])&twoBitsMask)<<24
	f.n[9] = uint32(b[2])>>2 | uint32(b[1])<<6 | uint32(b[0])<<14

	f.magnitude = 1
	f.normalized = false

	return nil
}

// getB32 converts a field element to a 32-byte big-endian array
func (f *FieldElement) getB32(b []byte) {
	if len(b) != 32 {
		panic("field element byte array must be 32 bytes")
	}

	// Normalize first
	var normalized FieldElement
	normalized = *f
	normalized.normalize()

	// Unpack the 256 total bits from the 10 uint32 words
	b[31] = byte(normalized.n[0] & eightBitsMask)
	b[30] = byte((normalized.n[0] >> 8) & eightBitsMask)
	b[29] = byte((normalized.n[0] >> 16) & eightBitsMask)
	b[28] = byte((normalized.n[0]>>24)&twoBitsMask | (normalized.n[1]&sixBitsMask)<<2)
	b[27] = byte((normalized.n[1] >> 6) & eightBitsMask)
	b[26] = byte((normalized.n[1] >> 14) & eightBitsMask)
	b[25] = byte((normalized.n[1]>>22)&fourBitsMask | (normalized.n[2]&fourBitsMask)<<4)
	b[24] = byte((normalized.n[2] >> 4) & eightBitsMask)
	b[23] = byte((normalized.n[2] >> 12) & eightBitsMask)
	b[22] = byte((normalized.n[2]>>20)&sixBitsMask | (normalized.n[3]&twoBitsMask)<<6)
	b[21] = byte((normalized.n[3] >> 2) & eightBitsMask)
	b[20] = byte((normalized.n[3] >> 10) & eightBitsMask)
	b[19] = byte((normalized.n[3] >> 18) & eightBitsMask)
	b[18] = byte(normalized.n[4] & eightBitsMask)
	b[17] = byte((normalized.n[4] >> 8) & eightBitsMask)
	b[16] = byte((normalized.n[4] >> 16) & eightBitsMask)
	b[15] = byte((normalized.n[4]>>24)&twoBitsMask | (normalized.n[5]&sixBitsMask)<<2)
	b[14] = byte((normalized.n[5] >> 6) & eightBitsMask)
	b[13] = byte((normalized.n[5] >> 14) & eightBitsMask)
	b[12] = byte((normalized.n[5]>>22)&fourBitsMask | (normalized.n[6]&fourBitsMask)<<4)
	b[11] = byte((normalized.n[6] >> 4) & eightBitsMask)
	b[10] = byte((normalized.n[6] >> 12) & eightBitsMask)
	b[9] = byte((normalized.n[6]>>20)&sixBitsMask | (normalized.n[7]&twoBitsMask)<<6)
	b[8] = byte((normalized.n[7] >> 2) & eightBitsMask)
	b[7] = byte((normalized.n[7] >> 10) & eightBitsMask)
	b[6] = byte((normalized.n[7] >> 18) & eightBitsMask)
	b[5] = byte(normalized.n[8] & eightBitsMask)
	b[4] = byte((normalized.n[8] >> 8) & eightBitsMask)
	b[3] = byte((normalized.n[8] >> 16) & eightBitsMask)
	b[2] = byte((normalized.n[8]>>24)&twoBitsMask | (normalized.n[9]&sixBitsMask)<<2)
	b[1] = byte((normalized.n[9] >> 6) & eightBitsMask)
	b[0] = byte((normalized.n[9] >> 14) & eightBitsMask)
}

// normalize normalizes a field element to its canonical representation
func (f *FieldElement) normalize() {
	// Reduce modulo the prime using the special form p = 2^256 - 2^32 - 977
	// = 2^256 - 4294968273
	// 4294968273 in 10x26 representation: n[0]=977, n[1]=64
	t9 := f.n[9]
	m := t9 >> fieldMSBBits32
	t9 &= fieldMSBMask32
	t0 := f.n[0] + m*977
	t1 := (t0 >> fieldBase32) + f.n[1] + (m << 6)
	t0 &= fieldBaseMask32
	t2 := (t1 >> fieldBase32) + f.n[2]
	t1 &= fieldBaseMask32
	t3 := (t2 >> fieldBase32) + f.n[3]
	t2 &= fieldBaseMask32
	t4 := (t3 >> fieldBase32) + f.n[4]
	t3 &= fieldBaseMask32
	t5 := (t4 >> fieldBase32) + f.n[5]
	t4 &= fieldBaseMask32
	t6 := (t5 >> fieldBase32) + f.n[6]
	t5 &= fieldBaseMask32
	t7 := (t6 >> fieldBase32) + f.n[7]
	t6 &= fieldBaseMask32
	t8 := (t7 >> fieldBase32) + f.n[8]
	t7 &= fieldBaseMask32
	t9 = (t8 >> fieldBase32) + t9
	t8 &= fieldBaseMask32

	// Final reduction if needed (constant-time)
	// Check if value >= p
	m = constantTimeEq32(t9, fieldMSBMask32)
	m &= constantTimeEq32(t8&t7&t6&t5&t4&t3&t2, fieldBaseMask32)
	m &= constantTimeGreater32(t1+64+((t0+977)>>fieldBase32), fieldBaseMask32)
	m |= t9 >> fieldMSBBits32

	t0 += m * 977
	t1 = (t0 >> fieldBase32) + t1 + (m << 6)
	t0 &= fieldBaseMask32
	t2 = (t1 >> fieldBase32) + t2
	t1 &= fieldBaseMask32
	t3 = (t2 >> fieldBase32) + t3
	t2 &= fieldBaseMask32
	t4 = (t3 >> fieldBase32) + t4
	t3 &= fieldBaseMask32
	t5 = (t4 >> fieldBase32) + t5
	t4 &= fieldBaseMask32
	t6 = (t5 >> fieldBase32) + t6
	t5 &= fieldBaseMask32
	t7 = (t6 >> fieldBase32) + t7
	t6 &= fieldBaseMask32
	t8 = (t7 >> fieldBase32) + t8
	t7 &= fieldBaseMask32
	t9 = (t8 >> fieldBase32) + t9
	t8 &= fieldBaseMask32
	t9 &= fieldMSBMask32

	f.n[0] = t0
	f.n[1] = t1
	f.n[2] = t2
	f.n[3] = t3
	f.n[4] = t4
	f.n[5] = t5
	f.n[6] = t6
	f.n[7] = t7
	f.n[8] = t8
	f.n[9] = t9
	f.magnitude = 1
	f.normalized = true
}

// normalizeWeak gives a field element magnitude 1 without full normalization
func (f *FieldElement) normalizeWeak() {
	t9 := f.n[9]
	m := t9 >> fieldMSBBits32
	t9 &= fieldMSBMask32
	t0 := f.n[0] + m*977
	t1 := (t0 >> fieldBase32) + f.n[1] + (m << 6)
	t0 &= fieldBaseMask32
	t2 := (t1 >> fieldBase32) + f.n[2]
	t1 &= fieldBaseMask32
	t3 := (t2 >> fieldBase32) + f.n[3]
	t2 &= fieldBaseMask32
	t4 := (t3 >> fieldBase32) + f.n[4]
	t3 &= fieldBaseMask32
	t5 := (t4 >> fieldBase32) + f.n[5]
	t4 &= fieldBaseMask32
	t6 := (t5 >> fieldBase32) + f.n[6]
	t5 &= fieldBaseMask32
	t7 := (t6 >> fieldBase32) + f.n[7]
	t6 &= fieldBaseMask32
	t8 := (t7 >> fieldBase32) + f.n[8]
	t7 &= fieldBaseMask32
	t9 = (t8 >> fieldBase32) + t9
	t8 &= fieldBaseMask32

	f.n[0] = t0
	f.n[1] = t1
	f.n[2] = t2
	f.n[3] = t3
	f.n[4] = t4
	f.n[5] = t5
	f.n[6] = t6
	f.n[7] = t7
	f.n[8] = t8
	f.n[9] = t9
	f.magnitude = 1
}

func (f *FieldElement) reduce() {
	f.normalize()
}

// isZero returns true if the field element represents zero
func (f *FieldElement) isZero() bool {
	if !f.normalized {
		panic("field element must be normalized")
	}
	bits := f.n[0] | f.n[1] | f.n[2] | f.n[3] | f.n[4] |
		f.n[5] | f.n[6] | f.n[7] | f.n[8] | f.n[9]
	return bits == 0
}

// isOdd returns true if the field element is odd
func (f *FieldElement) isOdd() bool {
	if !f.normalized {
		panic("field element must be normalized")
	}
	return f.n[0]&1 == 1
}

// normalizesToZeroVar checks if the field element normalizes to zero
func (f *FieldElement) normalizesToZeroVar() bool {
	var t FieldElement
	t = *f
	t.normalize()
	return t.isZero()
}

// equal returns true if two field elements are equal
func (f *FieldElement) equal(a *FieldElement) bool {
	if !f.normalized || !a.normalized {
		panic("field elements must be normalized for comparison")
	}
	return subtle.ConstantTimeCompare(
		(*[40]byte)(unsafe.Pointer(&f.n[0]))[:40],
		(*[40]byte)(unsafe.Pointer(&a.n[0]))[:40],
	) == 1
}

// setInt sets a field element to a small integer value
func (f *FieldElement) setInt(a int) {
	if a < 0 || a > 0x7FFF {
		panic("value out of range")
	}
	f.n[0] = uint32(a)
	for i := 1; i < 10; i++ {
		f.n[i] = 0
	}
	if a == 0 {
		f.magnitude = 0
	} else {
		f.magnitude = 1
	}
	f.normalized = true
}

// clear clears a field element
func (f *FieldElement) clear() {
	for i := 0; i < 10; i++ {
		f.n[i] = 0
	}
	f.magnitude = 0
	f.normalized = true
}

// negate negates a field element: r = -a
func (f *FieldElement) negate(a *FieldElement, m int) {
	if m < 0 || m > 31 {
		panic("magnitude out of range")
	}
	f.n[0] = (uint32(m)+1)*fieldPrimeWord0 - a.n[0]
	f.n[1] = (uint32(m)+1)*fieldPrimeWord1 - a.n[1]
	f.n[2] = (uint32(m)+1)*fieldPrimeWord2 - a.n[2]
	f.n[3] = (uint32(m)+1)*fieldPrimeWord3 - a.n[3]
	f.n[4] = (uint32(m)+1)*fieldPrimeWord4 - a.n[4]
	f.n[5] = (uint32(m)+1)*fieldPrimeWord5 - a.n[5]
	f.n[6] = (uint32(m)+1)*fieldPrimeWord6 - a.n[6]
	f.n[7] = (uint32(m)+1)*fieldPrimeWord7 - a.n[7]
	f.n[8] = (uint32(m)+1)*fieldPrimeWord8 - a.n[8]
	f.n[9] = (uint32(m)+1)*fieldPrimeWord9 - a.n[9]
	f.magnitude = m + 1
	f.normalized = false
}

// add adds two field elements: r += a
func (f *FieldElement) add(a *FieldElement) {
	f.n[0] += a.n[0]
	f.n[1] += a.n[1]
	f.n[2] += a.n[2]
	f.n[3] += a.n[3]
	f.n[4] += a.n[4]
	f.n[5] += a.n[5]
	f.n[6] += a.n[6]
	f.n[7] += a.n[7]
	f.n[8] += a.n[8]
	f.n[9] += a.n[9]
	f.magnitude += a.magnitude
	f.normalized = false
}

// sub subtracts a field element: r -= a
func (f *FieldElement) sub(a *FieldElement) {
	var negA FieldElement
	negA.negate(a, a.magnitude)
	f.add(&negA)
}

// mulInt multiplies a field element by a small integer
func (f *FieldElement) mulInt(a int) {
	if a < 0 || a > 32 {
		panic("multiplier out of range")
	}
	ua := uint32(a)
	f.n[0] *= ua
	f.n[1] *= ua
	f.n[2] *= ua
	f.n[3] *= ua
	f.n[4] *= ua
	f.n[5] *= ua
	f.n[6] *= ua
	f.n[7] *= ua
	f.n[8] *= ua
	f.n[9] *= ua
	f.magnitude *= a
	f.normalized = false
}

// cmov conditionally moves a field element
func (f *FieldElement) cmov(a *FieldElement, flag int) {
	mask := uint32(-(int32(flag) & 1))
	for i := 0; i < 10; i++ {
		f.n[i] ^= mask & (f.n[i] ^ a.n[i])
	}
	if flag != 0 {
		f.magnitude = a.magnitude
		f.normalized = a.normalized
	}
}

// toStorage converts a field element to storage format
func (f *FieldElement) toStorage(s *FieldElementStorage) {
	var normalized FieldElement
	normalized = *f
	normalized.normalize()

	// Convert from 10x26 to 4x64
	s.n[0] = uint64(normalized.n[0]) |
		uint64(normalized.n[1])<<26 |
		uint64(normalized.n[2])<<52
	s.n[1] = uint64(normalized.n[2])>>12 |
		uint64(normalized.n[3])<<14 |
		uint64(normalized.n[4])<<40
	s.n[2] = uint64(normalized.n[4])>>24 |
		uint64(normalized.n[5])<<2 |
		uint64(normalized.n[6])<<28 |
		uint64(normalized.n[7])<<54
	s.n[3] = uint64(normalized.n[7])>>10 |
		uint64(normalized.n[8])<<16 |
		uint64(normalized.n[9])<<42
}

// fromStorage converts from storage format to field element
func (f *FieldElement) fromStorage(s *FieldElementStorage) {
	// Convert from 4x64 to 10x26
	f.n[0] = uint32(s.n[0]) & fieldBaseMask32
	f.n[1] = uint32(s.n[0]>>26) & fieldBaseMask32
	f.n[2] = uint32(s.n[0]>>52) | (uint32(s.n[1]<<12) & fieldBaseMask32)
	f.n[3] = uint32(s.n[1]>>14) & fieldBaseMask32
	f.n[4] = uint32(s.n[1]>>40) | (uint32(s.n[2]<<24) & fieldBaseMask32)
	f.n[5] = uint32(s.n[2]>>2) & fieldBaseMask32
	f.n[6] = uint32(s.n[2]>>28) & fieldBaseMask32
	f.n[7] = uint32(s.n[2]>>54) | (uint32(s.n[3]<<10) & fieldBaseMask32)
	f.n[8] = uint32(s.n[3]>>16) & fieldBaseMask32
	f.n[9] = uint32(s.n[3] >> 42)
	f.magnitude = 1
	f.normalized = false
}

// half computes r = a/2 mod p
func (f *FieldElement) half(a *FieldElement) {
	// If a is odd, add p first (so result is even and can be divided by 2)
	// p = 2^256 - 2^32 - 977
	var t [10]uint32
	copy(t[:], a.n[:])

	// Check if odd (least significant bit)
	isOdd := t[0] & 1

	// Add p if odd: p in 10x26 representation
	if isOdd == 1 {
		var c uint64
		c = uint64(t[0]) + uint64(fieldPrimeWord0)
		t[0] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[1]) + uint64(fieldPrimeWord1)
		t[1] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[2]) + uint64(fieldPrimeWord2)
		t[2] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[3]) + uint64(fieldPrimeWord3)
		t[3] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[4]) + uint64(fieldPrimeWord4)
		t[4] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[5]) + uint64(fieldPrimeWord5)
		t[5] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[6]) + uint64(fieldPrimeWord6)
		t[6] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[7]) + uint64(fieldPrimeWord7)
		t[7] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[8]) + uint64(fieldPrimeWord8)
		t[8] = uint32(c & fieldBaseMask32)
		c = (c >> 26) + uint64(t[9]) + uint64(fieldPrimeWord9)
		t[9] = uint32(c & fieldMSBMask32)
	}

	// Divide by 2 (right shift by 1)
	f.n[0] = (t[0] >> 1) | ((t[1] & 1) << 25)
	f.n[1] = (t[1] >> 1) | ((t[2] & 1) << 25)
	f.n[2] = (t[2] >> 1) | ((t[3] & 1) << 25)
	f.n[3] = (t[3] >> 1) | ((t[4] & 1) << 25)
	f.n[4] = (t[4] >> 1) | ((t[5] & 1) << 25)
	f.n[5] = (t[5] >> 1) | ((t[6] & 1) << 25)
	f.n[6] = (t[6] >> 1) | ((t[7] & 1) << 25)
	f.n[7] = (t[7] >> 1) | ((t[8] & 1) << 25)
	f.n[8] = (t[8] >> 1) | ((t[9] & 1) << 25)
	f.n[9] = t[9] >> 1

	f.magnitude = (a.magnitude + 1) / 2
	f.normalized = false
}

// mul multiplies two field elements: r = a * b
func (f *FieldElement) mul(a, b *FieldElement) {
	// Using 32-bit multiplication with 64-bit intermediate results
	// This is optimized for platforms without native 64-bit multiplication
	var c, d uint64

	// Compute all 100 cross products
	c = uint64(a.n[0]) * uint64(b.n[0])
	d0 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[1]) + uint64(a.n[1])*uint64(b.n[0])
	d1 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[2]) + uint64(a.n[1])*uint64(b.n[1]) +
		uint64(a.n[2])*uint64(b.n[0])
	d2 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[3]) + uint64(a.n[1])*uint64(b.n[2]) +
		uint64(a.n[2])*uint64(b.n[1]) + uint64(a.n[3])*uint64(b.n[0])
	d3 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[4]) + uint64(a.n[1])*uint64(b.n[3]) +
		uint64(a.n[2])*uint64(b.n[2]) + uint64(a.n[3])*uint64(b.n[1]) +
		uint64(a.n[4])*uint64(b.n[0])
	d4 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[5]) + uint64(a.n[1])*uint64(b.n[4]) +
		uint64(a.n[2])*uint64(b.n[3]) + uint64(a.n[3])*uint64(b.n[2]) +
		uint64(a.n[4])*uint64(b.n[1]) + uint64(a.n[5])*uint64(b.n[0])
	d5 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[6]) + uint64(a.n[1])*uint64(b.n[5]) +
		uint64(a.n[2])*uint64(b.n[4]) + uint64(a.n[3])*uint64(b.n[3]) +
		uint64(a.n[4])*uint64(b.n[2]) + uint64(a.n[5])*uint64(b.n[1]) +
		uint64(a.n[6])*uint64(b.n[0])
	d6 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[7]) + uint64(a.n[1])*uint64(b.n[6]) +
		uint64(a.n[2])*uint64(b.n[5]) + uint64(a.n[3])*uint64(b.n[4]) +
		uint64(a.n[4])*uint64(b.n[3]) + uint64(a.n[5])*uint64(b.n[2]) +
		uint64(a.n[6])*uint64(b.n[1]) + uint64(a.n[7])*uint64(b.n[0])
	d7 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[8]) + uint64(a.n[1])*uint64(b.n[7]) +
		uint64(a.n[2])*uint64(b.n[6]) + uint64(a.n[3])*uint64(b.n[5]) +
		uint64(a.n[4])*uint64(b.n[4]) + uint64(a.n[5])*uint64(b.n[3]) +
		uint64(a.n[6])*uint64(b.n[2]) + uint64(a.n[7])*uint64(b.n[1]) +
		uint64(a.n[8])*uint64(b.n[0])
	d8 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[0])*uint64(b.n[9]) + uint64(a.n[1])*uint64(b.n[8]) +
		uint64(a.n[2])*uint64(b.n[7]) + uint64(a.n[3])*uint64(b.n[6]) +
		uint64(a.n[4])*uint64(b.n[5]) + uint64(a.n[5])*uint64(b.n[4]) +
		uint64(a.n[6])*uint64(b.n[3]) + uint64(a.n[7])*uint64(b.n[2]) +
		uint64(a.n[8])*uint64(b.n[1]) + uint64(a.n[9])*uint64(b.n[0])
	d9 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[1])*uint64(b.n[9]) + uint64(a.n[2])*uint64(b.n[8]) +
		uint64(a.n[3])*uint64(b.n[7]) + uint64(a.n[4])*uint64(b.n[6]) +
		uint64(a.n[5])*uint64(b.n[5]) + uint64(a.n[6])*uint64(b.n[4]) +
		uint64(a.n[7])*uint64(b.n[3]) + uint64(a.n[8])*uint64(b.n[2]) +
		uint64(a.n[9])*uint64(b.n[1])
	d10 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[2])*uint64(b.n[9]) + uint64(a.n[3])*uint64(b.n[8]) +
		uint64(a.n[4])*uint64(b.n[7]) + uint64(a.n[5])*uint64(b.n[6]) +
		uint64(a.n[6])*uint64(b.n[5]) + uint64(a.n[7])*uint64(b.n[4]) +
		uint64(a.n[8])*uint64(b.n[3]) + uint64(a.n[9])*uint64(b.n[2])
	d11 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[3])*uint64(b.n[9]) + uint64(a.n[4])*uint64(b.n[8]) +
		uint64(a.n[5])*uint64(b.n[7]) + uint64(a.n[6])*uint64(b.n[6]) +
		uint64(a.n[7])*uint64(b.n[5]) + uint64(a.n[8])*uint64(b.n[4]) +
		uint64(a.n[9])*uint64(b.n[3])
	d12 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[4])*uint64(b.n[9]) + uint64(a.n[5])*uint64(b.n[8]) +
		uint64(a.n[6])*uint64(b.n[7]) + uint64(a.n[7])*uint64(b.n[6]) +
		uint64(a.n[8])*uint64(b.n[5]) + uint64(a.n[9])*uint64(b.n[4])
	d13 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[5])*uint64(b.n[9]) + uint64(a.n[6])*uint64(b.n[8]) +
		uint64(a.n[7])*uint64(b.n[7]) + uint64(a.n[8])*uint64(b.n[6]) +
		uint64(a.n[9])*uint64(b.n[5])
	d14 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[6])*uint64(b.n[9]) + uint64(a.n[7])*uint64(b.n[8]) +
		uint64(a.n[8])*uint64(b.n[7]) + uint64(a.n[9])*uint64(b.n[6])
	d15 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[7])*uint64(b.n[9]) + uint64(a.n[8])*uint64(b.n[8]) +
		uint64(a.n[9])*uint64(b.n[7])
	d16 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[8])*uint64(b.n[9]) + uint64(a.n[9])*uint64(b.n[8])
	d17 := uint32(c & 0x3FFFFFF)
	c >>= 26

	c += uint64(a.n[9]) * uint64(b.n[9])
	d18 := uint32(c & 0x3FFFFFF)
	d19 := uint32(c >> 26)

	// Reduce modulo p using p = 2^256 - 2^32 - 977 = 2^256 - 4294968273.
	// Upper terms (d10+) start at bit 260 (10*26), which is 4 bits above 256.
	// So the reduction constant 4294968273 must be scaled by 2^4 = 16:
	//   977 * 16 = 15632, 64 * 16 = 1024
	// The final term d19 uses the full constant 4294968273 * 16 = 68719492368.
	d = uint64(d0) + uint64(d10)*15632
	t0 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d1) + uint64(d10)*1024 + uint64(d11)*15632
	t1 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d2) + uint64(d11)*1024 + uint64(d12)*15632
	t2 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d3) + uint64(d12)*1024 + uint64(d13)*15632
	t3 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d4) + uint64(d13)*1024 + uint64(d14)*15632
	t4 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d5) + uint64(d14)*1024 + uint64(d15)*15632
	t5 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d6) + uint64(d15)*1024 + uint64(d16)*15632
	t6 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d7) + uint64(d16)*1024 + uint64(d17)*15632
	t7 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d8) + uint64(d17)*1024 + uint64(d18)*15632
	t8 := uint32(d & 0x3FFFFFF)
	d >>= 26
	d += uint64(d9) + uint64(d18)*1024 + uint64(d19)*68719492368
	t9 := uint32(d & 0x3FFFFF)
	d >>= 22

	// Final reduction: d is the overflow magnitude (number of times
	// the value exceeds 2^256). At this stage it's at bit 256, so
	// we use the unscaled prime complement (977, 64).
	m := d
	d = uint64(t0) + m*977
	f.n[0] = uint32(d & 0x3FFFFFF)
	d = (d >> 26) + uint64(t1) + m*64
	f.n[1] = uint32(d & 0x3FFFFFF)
	f.n[2] = uint32((d >> 26) + uint64(t2))
	f.n[3] = t3
	f.n[4] = t4
	f.n[5] = t5
	f.n[6] = t6
	f.n[7] = t7
	f.n[8] = t8
	f.n[9] = t9

	f.magnitude = 1
	f.normalized = false
}

// sqr squares a field element: r = a^2
func (f *FieldElement) sqr(a *FieldElement) {
	// For squaring, we can exploit the symmetry to reduce operations
	f.mul(a, a)
}

// inv computes the modular inverse using Fermat's little theorem.
// a^(-1) = a^(p-2) mod p, where p-2 = 2^256 - 4294968275.
// Uses the Decred/btcec addition chain: 258 squarings, 33 multiplications.
func (f *FieldElement) inv(a *FieldElement) {
	var a2, a3, a4, a10, a11, a21, a42, a45, a63, a1019, a1023 FieldElement
	a2.sqr(a)
	a3.mul(&a2, a)
	a4.sqr(&a2)
	a10.sqr(&a4)
	a10.mul(&a10, &a2)
	a11.mul(&a10, a)
	a21.mul(&a10, &a11)
	a42.sqr(&a21)
	a45.mul(&a42, &a3)
	a63.mul(&a42, &a21)
	a1019.sqr(&a63)
	a1019.sqr(&a1019)
	a1019.sqr(&a1019)
	a1019.sqr(&a1019)
	a1019.mul(&a1019, &a11)
	a1023.mul(&a1019, &a4)

	// Build a^(2^16 - 1) through a^(2^226 - 5) using 10-bit windows
	*f = a63
	for i := 0; i < 5; i++ { f.sqr(f) } // 2^11 - 32
	for i := 0; i < 5; i++ { f.sqr(f) } // 2^16 - 1024
	f.mul(f, &a1023)                      // 2^16 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^26 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^36 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^46 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^56 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^66 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^76 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^86 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^96 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^106 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^116 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^126 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^136 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^146 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^156 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^166 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^176 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^186 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^196 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^206 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^216 - 1
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1019)                      // 2^226 - 5
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^236 - 4097
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a1023)                      // 2^246 - 4194305
	for i := 0; i < 5; i++ { f.sqr(f) }
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.mul(f, &a45)                        // 2^256 - 4294968275 = p-2

	f.magnitude = 1
	f.normalized = false
}

// sqrt computes the modular square root if it exists.
// sqrt(a) = a^((p+1)/4) = a^(2^254 - 2^30 - 244) for secp256k1.
// Uses the Decred/btcec addition chain: 254 squarings, 13 multiplications.
func (f *FieldElement) sqrt(a *FieldElement) bool {
	var aa, a2, a3, a6, a9, a11, a22, a44, a88, a176, a220, a223 FieldElement
	aa = *a
	a2.sqr(&aa)
	a2.mul(&a2, &aa)       // a^(2^2 - 1)
	a3.sqr(&a2)
	a3.mul(&a3, &aa)       // a^(2^3 - 1)
	a6.sqr(&a3)
	a6.sqr(&a6)
	a6.sqr(&a6)
	a6.mul(&a6, &a3)       // a^(2^6 - 1)
	a9.sqr(&a6)
	a9.sqr(&a9)
	a9.sqr(&a9)
	a9.mul(&a9, &a3)       // a^(2^9 - 1)
	a11.sqr(&a9)
	a11.sqr(&a11)
	a11.mul(&a11, &a2)     // a^(2^11 - 1)
	a22.sqr(&a11)
	for i := 0; i < 10; i++ { a22.sqr(&a22) }
	a22.mul(&a22, &a11)    // a^(2^22 - 1)
	a44.sqr(&a22)
	for i := 0; i < 21; i++ { a44.sqr(&a44) }
	a44.mul(&a44, &a22)    // a^(2^44 - 1)
	a88.sqr(&a44)
	for i := 0; i < 43; i++ { a88.sqr(&a88) }
	a88.mul(&a88, &a44)    // a^(2^88 - 1)
	a176.sqr(&a88)
	for i := 0; i < 87; i++ { a176.sqr(&a176) }
	a176.mul(&a176, &a88)  // a^(2^176 - 1)
	a220.sqr(&a176)
	for i := 0; i < 43; i++ { a220.sqr(&a220) }
	a220.mul(&a220, &a44)  // a^(2^220 - 1)
	a223.sqr(&a220)
	a223.sqr(&a223)
	a223.sqr(&a223)
	a223.mul(&a223, &a3)   // a^(2^223 - 1)

	// Final exponent: (p+1)/4 = 2^254 - 2^30 - 244
	*f = a223
	for i := 0; i < 23; i++ { f.sqr(f) } // a^(2^246 - 2^23)
	f.mul(f, &a22)          // a^(2^246 - 2^22 - 1)
	for i := 0; i < 5; i++ { f.sqr(f) }
	f.sqr(f)                // a^(2^252 - 2^28 - 2^6)
	f.mul(f, &a2)           // a^(2^252 - 2^28 - 2^6 + 2^2 - 1)
	f.sqr(f)
	f.sqr(f)                // a^(2^254 - 2^30 - 2^8 + 2^4 - 4)
	//                      //   = a^(2^254 - 2^30 - 244)
	//                      //   = a^((p+1)/4)
	f.normalize()

	// Verify: f^2 == a
	var check FieldElement
	check.sqr(f)
	check.normalize()

	var aNorm FieldElement
	aNorm = *a
	aNorm.normalize()

	return check.equal(&aNorm)
}

// Constant-time helper functions for 32-bit values
func constantTimeEq32(a, b uint32) uint32 {
	return uint32((uint64(a^b) - 1) >> 63)
}

func constantTimeGreater32(a, b uint32) uint32 {
	return uint32((uint64(b) - uint64(a)) >> 63)
}

// memclear clears memory
func memclear(ptr unsafe.Pointer, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(uintptr(ptr) + i)) = 0
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// batchInverse computes the inverses of a slice of FieldElements
func batchInverse(out []FieldElement, a []FieldElement) {
	n := len(a)
	if n == 0 {
		return
	}

	s := make([]FieldElement, n)
	s[0].setInt(1)
	for i := 1; i < n; i++ {
		s[i].mul(&s[i-1], &a[i-1])
	}

	var u FieldElement
	u.mul(&s[n-1], &a[n-1])
	u.inv(&u)

	for i := n - 1; i >= 0; i-- {
		out[i].mul(&u, &s[i])
		u.mul(&u, &a[i])
	}
}

// batchInverse16 computes the inverses of exactly 16 FieldElements with zero heap allocations.
// This is optimized for the GLV table size (glvTableSize = 16).
// Uses Montgomery's trick with stack-allocated arrays.
func batchInverse16(out *[16]FieldElement, a *[16]FieldElement) {
	// Stack-allocated scratch array for prefix products
	var s [16]FieldElement

	// s_i = a_0 * a_1 * ... * a_{i-1}
	s[0].setInt(1)
	for i := 1; i < 16; i++ {
		s[i].mul(&s[i-1], &a[i-1])
	}

	// u = (a_0 * a_1 * ... * a_{15})^-1
	var u FieldElement
	u.mul(&s[15], &a[15])
	u.inv(&u)

	// out_i = (a_0 * ... * a_{i-1}) * (a_0 * ... * a_i)^-1
	// Loop backwards for in-place computation
	for i := 15; i >= 0; i-- {
		out[i].mul(&u, &s[i])
		u.mul(&u, &a[i])
	}
}

// batchInverse32 computes the inverses of exactly 32 FieldElements.
// Delegates to two batchInverse16 calls.
func batchInverse32(out *[32]FieldElement, a *[32]FieldElement) {
	var lo, hi [16]FieldElement
	var alo, ahi [16]FieldElement
	for i := 0; i < 16; i++ {
		alo[i] = a[i]
		ahi[i] = a[i+16]
	}
	batchInverse16(&lo, &alo)
	batchInverse16(&hi, &ahi)
	for i := 0; i < 16; i++ {
		out[i] = lo[i]
		out[i+16] = hi[i]
	}
}

// Placeholder stubs for Montgomery functions (not used in 10x26 representation)
const montgomeryPPrime = 0

var montgomeryR2 *FieldElement = nil

func (f *FieldElement) ToMontgomery() *FieldElement   { return f }
func (f *FieldElement) FromMontgomery() *FieldElement { return f }
func MontgomeryMul(a, b *FieldElement) *FieldElement {
	var r FieldElement
	r.mul(a, b)
	return &r
}

// Direct function versions
func fieldNormalize(r *FieldElement)            { r.normalize() }
func fieldNormalizeWeak(r *FieldElement)        { r.normalizeWeak() }
func fieldAdd(r, a *FieldElement)               { r.add(a) }
func fieldIsZero(a *FieldElement) bool          { return a.isZero() }
func fieldGetB32(b []byte, a *FieldElement)     { a.getB32(b) }
func fieldMul(r, a, b []uint64)                 {} // Not used for 32-bit
func fieldSqr(r, a []uint64)                    {} // Not used for 32-bit
func fieldInvVar(r, a []uint64)                 {} // Not used for 32-bit
func fieldSqrt(r, a []uint64) bool              { return false }
