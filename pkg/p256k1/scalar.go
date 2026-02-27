//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"crypto/subtle"
	"math/bits"
	"unsafe"
)

// Scalar represents a scalar value modulo the secp256k1 group order.
// Uses 4 uint64 limbs to represent a 256-bit scalar.
type Scalar struct {
	d [4]uint64
}

// Scalar constants from the C implementation
const (
	// Limbs of the secp256k1 order n
	scalarN0 = 0xBFD25E8CD0364141
	scalarN1 = 0xBAAEDCE6AF48A03B
	scalarN2 = 0xFFFFFFFFFFFFFFFE
	scalarN3 = 0xFFFFFFFFFFFFFFFF

	// Limbs of 2^256 minus the secp256k1 order (complement constants)
	scalarNC0 = 0x402DA1732FC9BEBF // ~scalarN0 + 1
	scalarNC1 = 0x4551231950B75FC4 // ~scalarN1
	scalarNC2 = 0x0000000000000001 // 1

	// Limbs of half the secp256k1 order
	scalarNH0 = 0xDFE92F46681B20A0
	scalarNH1 = 0x5D576E7357A4501D
	scalarNH2 = 0xFFFFFFFFFFFFFFFF
	scalarNH3 = 0x7FFFFFFFFFFFFFFF
)

// Scalar element constants
var (
	// ScalarZero represents the scalar 0
	ScalarZero = Scalar{d: [4]uint64{0, 0, 0, 0}}

	// ScalarOne represents the scalar 1
	ScalarOne = Scalar{d: [4]uint64{1, 0, 0, 0}}

	// scalarLambda is the GLV endomorphism constant λ (cube root of unity mod n)
	// λ^3 ≡ 1 (mod n), and λ^2 + λ + 1 ≡ 0 (mod n)
	// Value: 0x5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72
	// From libsecp256k1 scalar_impl.h line 81-84
	scalarLambda = Scalar{
		d: [4]uint64{
			0xDF02967C1B23BD72, // limb 0 (least significant)
			0x122E22EA20816678, // limb 1
			0xA5261C028812645A, // limb 2
			0x5363AD4CC05C30E0, // limb 3 (most significant)
		},
	}

	// GLV scalar splitting constants from libsecp256k1 scalar_impl.h lines 142-157
	// These are used in the splitLambda function to decompose a scalar k
	// into k1 and k2 such that k1 + k2*λ ≡ k (mod n)

	// scalarMinusB1 = -b1 where b1 is from the GLV basis
	// Value: 0x00000000000000000000000000000000E4437ED6010E88286F547FA90ABFE4C3
	scalarMinusB1 = Scalar{
		d: [4]uint64{
			0x6F547FA90ABFE4C3, // limb 0
			0xE4437ED6010E8828, // limb 1
			0x0000000000000000, // limb 2
			0x0000000000000000, // limb 3
		},
	}

	// scalarMinusB2 = -b2 where b2 is from the GLV basis
	// Value: 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFE8A280AC50774346DD765CDA83DB1562C
	scalarMinusB2 = Scalar{
		d: [4]uint64{
			0xD765CDA83DB1562C, // limb 0
			0x8A280AC50774346D, // limb 1
			0xFFFFFFFFFFFFFFFE, // limb 2
			0xFFFFFFFFFFFFFFFF, // limb 3
		},
	}

	// scalarG1 is a precomputed constant for scalar splitting: g1 = round(2^384 * b2 / n)
	// Value: 0x3086D221A7D46BCDE86C90E49284EB153DAA8A1471E8CA7FE893209A45DBB031
	scalarG1 = Scalar{
		d: [4]uint64{
			0xE893209A45DBB031, // limb 0
			0x3DAA8A1471E8CA7F, // limb 1
			0xE86C90E49284EB15, // limb 2
			0x3086D221A7D46BCD, // limb 3
		},
	}

	// scalarG2 is a precomputed constant for scalar splitting: g2 = round(2^384 * (-b1) / n)
	// Value: 0xE4437ED6010E88286F547FA90ABFE4C4221208AC9DF506C61571B4AE8AC47F71
	scalarG2 = Scalar{
		d: [4]uint64{
			0x1571B4AE8AC47F71, // limb 0
			0x221208AC9DF506C6, // limb 1
			0x6F547FA90ABFE4C4, // limb 2
			0xE4437ED6010E8828, // limb 3
		},
	}
)

// setInt sets a scalar to a small integer value
func (r *Scalar) setInt(v uint) {
	r.d[0] = uint64(v)
	r.d[1] = 0
	r.d[2] = 0
	r.d[3] = 0
}

// setB32 sets a scalar from a 32-byte big-endian array
func (r *Scalar) setB32(b []byte) bool {
	if len(b) != 32 {
		panic("scalar byte array must be 32 bytes")
	}

	// Convert from big-endian bytes to uint64 limbs
	r.d[0] = uint64(b[31]) | uint64(b[30])<<8 | uint64(b[29])<<16 | uint64(b[28])<<24 |
		uint64(b[27])<<32 | uint64(b[26])<<40 | uint64(b[25])<<48 | uint64(b[24])<<56
	r.d[1] = uint64(b[23]) | uint64(b[22])<<8 | uint64(b[21])<<16 | uint64(b[20])<<24 |
		uint64(b[19])<<32 | uint64(b[18])<<40 | uint64(b[17])<<48 | uint64(b[16])<<56
	r.d[2] = uint64(b[15]) | uint64(b[14])<<8 | uint64(b[13])<<16 | uint64(b[12])<<24 |
		uint64(b[11])<<32 | uint64(b[10])<<40 | uint64(b[9])<<48 | uint64(b[8])<<56
	r.d[3] = uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56

	// Check if the scalar overflows the group order
	overflow := r.checkOverflow()
	if overflow {
		r.reduce(1)
	}

	return overflow
}

// setB32Seckey sets a scalar from a 32-byte secret key, returns true if valid
func (r *Scalar) setB32Seckey(b []byte) bool {
	overflow := r.setB32(b)
	return !r.isZero() && !overflow
}

// getB32 converts a scalar to a 32-byte big-endian array
func (r *Scalar) getB32(b []byte) {
	if len(b) != 32 {
		panic("scalar byte array must be 32 bytes")
	}

	// Convert from uint64 limbs to big-endian bytes
	b[31] = byte(r.d[0])
	b[30] = byte(r.d[0] >> 8)
	b[29] = byte(r.d[0] >> 16)
	b[28] = byte(r.d[0] >> 24)
	b[27] = byte(r.d[0] >> 32)
	b[26] = byte(r.d[0] >> 40)
	b[25] = byte(r.d[0] >> 48)
	b[24] = byte(r.d[0] >> 56)

	b[23] = byte(r.d[1])
	b[22] = byte(r.d[1] >> 8)
	b[21] = byte(r.d[1] >> 16)
	b[20] = byte(r.d[1] >> 24)
	b[19] = byte(r.d[1] >> 32)
	b[18] = byte(r.d[1] >> 40)
	b[17] = byte(r.d[1] >> 48)
	b[16] = byte(r.d[1] >> 56)

	b[15] = byte(r.d[2])
	b[14] = byte(r.d[2] >> 8)
	b[13] = byte(r.d[2] >> 16)
	b[12] = byte(r.d[2] >> 24)
	b[11] = byte(r.d[2] >> 32)
	b[10] = byte(r.d[2] >> 40)
	b[9] = byte(r.d[2] >> 48)
	b[8] = byte(r.d[2] >> 56)

	b[7] = byte(r.d[3])
	b[6] = byte(r.d[3] >> 8)
	b[5] = byte(r.d[3] >> 16)
	b[4] = byte(r.d[3] >> 24)
	b[3] = byte(r.d[3] >> 32)
	b[2] = byte(r.d[3] >> 40)
	b[1] = byte(r.d[3] >> 48)
	b[0] = byte(r.d[3] >> 56)
}

// checkOverflow checks if the scalar is >= the group order
func (r *Scalar) checkOverflow() bool {
	yes := 0
	no := 0

	// Check each limb from most significant to least significant
	if r.d[3] < scalarN3 {
		no = 1
	}
	if r.d[3] > scalarN3 {
		yes = 1
	}

	if r.d[2] < scalarN2 {
		no |= (yes ^ 1)
	}
	if r.d[2] > scalarN2 {
		yes |= (no ^ 1)
	}

	if r.d[1] < scalarN1 {
		no |= (yes ^ 1)
	}
	if r.d[1] > scalarN1 {
		yes |= (no ^ 1)
	}

	if r.d[0] >= scalarN0 {
		yes |= (no ^ 1)
	}

	return yes != 0
}

// reduce reduces the scalar modulo the group order
func (r *Scalar) reduce(overflow int) {
	if overflow < 0 || overflow > 1 {
		panic("overflow must be 0 or 1")
	}

	// Use 128-bit arithmetic for the reduction
	var t uint128

	// d[0] += overflow * scalarNC0
	t = uint128FromU64(r.d[0])
	t = t.addU64(uint64(overflow) * scalarNC0)
	r.d[0] = t.lo()
	t = t.rshift(64)

	// d[1] += overflow * scalarNC1 + carry
	t = t.addU64(r.d[1])
	t = t.addU64(uint64(overflow) * scalarNC1)
	r.d[1] = t.lo()
	t = t.rshift(64)

	// d[2] += overflow * scalarNC2 + carry
	t = t.addU64(r.d[2])
	t = t.addU64(uint64(overflow) * scalarNC2)
	r.d[2] = t.lo()
	t = t.rshift(64)

	// d[3] += carry (scalarNC3 = 0)
	t = t.addU64(r.d[3])
	r.d[3] = t.lo()
}

// add adds two scalars: r = a + b, returns overflow
func (r *Scalar) add(a, b *Scalar) bool {
	// Use AVX2 if available (AMD64 only)
	if HasAVX2() {
		scalarAddAVX2(r, a, b)
		return false // AVX2 version handles reduction internally
	}
	return r.addPureGo(a, b)
}

// addPureGo is the pure Go implementation of scalar addition
func (r *Scalar) addPureGo(a, b *Scalar) bool {
	var carry uint64

	r.d[0], carry = bits.Add64(a.d[0], b.d[0], 0)
	r.d[1], carry = bits.Add64(a.d[1], b.d[1], carry)
	r.d[2], carry = bits.Add64(a.d[2], b.d[2], carry)
	r.d[3], carry = bits.Add64(a.d[3], b.d[3], carry)

	overflow := carry != 0 || r.checkOverflow()
	if overflow {
		r.reduce(1)
	}

	return overflow
}

// sub subtracts two scalars: r = a - b
func (r *Scalar) sub(a, b *Scalar) {
	// Use AVX2 if available (AMD64 only)
	if HasAVX2() {
		scalarSubAVX2(r, a, b)
		return
	}
	r.subPureGo(a, b)
}

// subPureGo is the pure Go implementation of scalar subtraction
func (r *Scalar) subPureGo(a, b *Scalar) {
	// Compute a - b = a + (-b)
	var negB Scalar
	negB.negate(b)
	*r = *a
	r.addPureGo(r, &negB)
}

// mul multiplies two scalars: r = a * b
func (r *Scalar) mul(a, b *Scalar) {
	// Use AVX2 if available (AMD64 only)
	if HasAVX2() {
		scalarMulAVX2(r, a, b)
		return
	}
	r.mulPureGo(a, b)
}

// mulPureGo is the pure Go implementation of scalar multiplication
func (r *Scalar) mulPureGo(a, b *Scalar) {
	// Compute full 512-bit product using all 16 cross products
	var l [8]uint64
	r.mul512(l[:], a, b)
	r.reduce512(l[:])
}

// mul512 computes the 512-bit product of two scalars (from C implementation)
func (r *Scalar) mul512(l8 []uint64, a, b *Scalar) {
	// 160-bit accumulator (c0, c1, c2)
	var c0, c1 uint64
	var c2 uint32

	// Helper macros translated from C
	muladd := func(ai, bi uint64) {
		hi, lo := bits.Mul64(ai, bi)
		var carry uint64
		c0, carry = bits.Add64(c0, lo, 0)
		c1, carry = bits.Add64(c1, hi, carry)
		c2 += uint32(carry)
	}

	muladdFast := func(ai, bi uint64) {
		hi, lo := bits.Mul64(ai, bi)
		var carry uint64
		c0, carry = bits.Add64(c0, lo, 0)
		c1 += hi + carry
	}

	extract := func() uint64 {
		result := c0
		c0 = c1
		c1 = uint64(c2)
		c2 = 0
		return result
	}

	extractFast := func() uint64 {
		result := c0
		c0 = c1
		c1 = 0
		return result
	}

	// l8[0..7] = a[0..3] * b[0..3] (following C implementation exactly)
	muladdFast(a.d[0], b.d[0])
	l8[0] = extractFast()

	muladd(a.d[0], b.d[1])
	muladd(a.d[1], b.d[0])
	l8[1] = extract()

	muladd(a.d[0], b.d[2])
	muladd(a.d[1], b.d[1])
	muladd(a.d[2], b.d[0])
	l8[2] = extract()

	muladd(a.d[0], b.d[3])
	muladd(a.d[1], b.d[2])
	muladd(a.d[2], b.d[1])
	muladd(a.d[3], b.d[0])
	l8[3] = extract()

	muladd(a.d[1], b.d[3])
	muladd(a.d[2], b.d[2])
	muladd(a.d[3], b.d[1])
	l8[4] = extract()

	muladd(a.d[2], b.d[3])
	muladd(a.d[3], b.d[2])
	l8[5] = extract()

	muladdFast(a.d[3], b.d[3])
	l8[6] = extractFast()
	l8[7] = c0
}

// reduce512 reduces a 512-bit value to 256-bit (from C implementation)
func (r *Scalar) reduce512(l []uint64) {
	// 160-bit accumulator
	var c0, c1 uint64
	var c2 uint32

	// Extract upper 256 bits
	n0, n1, n2, n3 := l[4], l[5], l[6], l[7]

	// Helper macros
	muladd := func(ai, bi uint64) {
		hi, lo := bits.Mul64(ai, bi)
		var carry uint64
		c0, carry = bits.Add64(c0, lo, 0)
		c1, carry = bits.Add64(c1, hi, carry)
		c2 += uint32(carry)
	}

	muladdFast := func(ai, bi uint64) {
		hi, lo := bits.Mul64(ai, bi)
		var carry uint64
		c0, carry = bits.Add64(c0, lo, 0)
		c1 += hi + carry
	}

	sumadd := func(a uint64) {
		var carry uint64
		c0, carry = bits.Add64(c0, a, 0)
		c1, carry = bits.Add64(c1, 0, carry)
		c2 += uint32(carry)
	}

	sumaddFast := func(a uint64) {
		var carry uint64
		c0, carry = bits.Add64(c0, a, 0)
		c1 += carry
	}

	extract := func() uint64 {
		result := c0
		c0 = c1
		c1 = uint64(c2)
		c2 = 0
		return result
	}

	extractFast := func() uint64 {
		result := c0
		c0 = c1
		c1 = 0
		return result
	}

	// Reduce 512 bits into 385 bits
	// m[0..6] = l[0..3] + n[0..3] * SECP256K1_N_C
	c0 = l[0]
	c1 = 0
	c2 = 0
	muladdFast(n0, scalarNC0)
	m0 := extractFast()

	sumaddFast(l[1])
	muladd(n1, scalarNC0)
	muladd(n0, scalarNC1)
	m1 := extract()

	sumadd(l[2])
	muladd(n2, scalarNC0)
	muladd(n1, scalarNC1)
	sumadd(n0)
	m2 := extract()

	sumadd(l[3])
	muladd(n3, scalarNC0)
	muladd(n2, scalarNC1)
	sumadd(n1)
	m3 := extract()

	muladd(n3, scalarNC1)
	sumadd(n2)
	m4 := extract()

	sumaddFast(n3)
	m5 := extractFast()
	m6 := uint32(c0)

	// Reduce 385 bits into 258 bits
	// p[0..4] = m[0..3] + m[4..6] * SECP256K1_N_C
	c0 = m0
	c1 = 0
	c2 = 0
	muladdFast(m4, scalarNC0)
	p0 := extractFast()

	sumaddFast(m1)
	muladd(m5, scalarNC0)
	muladd(m4, scalarNC1)
	p1 := extract()

	sumadd(m2)
	muladd(uint64(m6), scalarNC0)
	muladd(m5, scalarNC1)
	sumadd(m4)
	p2 := extract()

	sumaddFast(m3)
	muladdFast(uint64(m6), scalarNC1)
	sumaddFast(m5)
	p3 := extractFast()
	p4 := uint32(c0 + uint64(m6))

	// Reduce 258 bits into 256 bits
	// r[0..3] = p[0..3] + p[4] * SECP256K1_N_C
	var t uint128

	t = uint128FromU64(p0)
	t = t.addMul(scalarNC0, uint64(p4))
	r.d[0] = t.lo()
	t = t.rshift(64)

	t = t.addU64(p1)
	t = t.addMul(scalarNC1, uint64(p4))
	r.d[1] = t.lo()
	t = t.rshift(64)

	t = t.addU64(p2)
	t = t.addU64(uint64(p4))
	r.d[2] = t.lo()
	t = t.rshift(64)

	t = t.addU64(p3)
	r.d[3] = t.lo()
	c := t.hi()

	// Final reduction
	r.reduce(int(c) + boolToInt(r.checkOverflow()))
}

// negate negates a scalar: r = -a
func (r *Scalar) negate(a *Scalar) {
	// r = n - a where n is the group order
	var borrow uint64

	r.d[0], borrow = bits.Sub64(scalarN0, a.d[0], 0)
	r.d[1], borrow = bits.Sub64(scalarN1, a.d[1], borrow)
	r.d[2], borrow = bits.Sub64(scalarN2, a.d[2], borrow)
	r.d[3], _ = bits.Sub64(scalarN3, a.d[3], borrow)
}

// inverse computes the modular inverse of a scalar
func (r *Scalar) inverse(a *Scalar) {
	// Use Fermat's little theorem: a^(-1) = a^(n-2) mod n
	// where n is the group order (which is prime)

	// Use binary exponentiation with n-2
	var exp Scalar
	var borrow uint64
	exp.d[0], borrow = bits.Sub64(scalarN0, 2, 0)
	exp.d[1], borrow = bits.Sub64(scalarN1, 0, borrow)
	exp.d[2], borrow = bits.Sub64(scalarN2, 0, borrow)
	exp.d[3], _ = bits.Sub64(scalarN3, 0, borrow)

	r.exp(a, &exp)
}

// exp computes r = a^b mod n using binary exponentiation
func (r *Scalar) exp(a, b *Scalar) {
	*r = ScalarOne
	base := *a

	for i := 0; i < 4; i++ {
		limb := b.d[i]
		for j := 0; j < 64; j++ {
			if limb&1 != 0 {
				r.mul(r, &base)
			}
			base.mul(&base, &base)
			limb >>= 1
		}
	}
}

// half computes r = a/2 mod n
func (r *Scalar) half(a *Scalar) {
	*r = *a

	if r.d[0]&1 == 0 {
		// Even case: simple right shift
		r.d[0] = (r.d[0] >> 1) | ((r.d[1] & 1) << 63)
		r.d[1] = (r.d[1] >> 1) | ((r.d[2] & 1) << 63)
		r.d[2] = (r.d[2] >> 1) | ((r.d[3] & 1) << 63)
		r.d[3] = r.d[3] >> 1
	} else {
		// Odd case: add n then divide by 2
		var carry uint64
		r.d[0], carry = bits.Add64(r.d[0], scalarN0, 0)
		r.d[1], carry = bits.Add64(r.d[1], scalarN1, carry)
		r.d[2], carry = bits.Add64(r.d[2], scalarN2, carry)
		r.d[3], _ = bits.Add64(r.d[3], scalarN3, carry)

		// Now divide by 2
		r.d[0] = (r.d[0] >> 1) | ((r.d[1] & 1) << 63)
		r.d[1] = (r.d[1] >> 1) | ((r.d[2] & 1) << 63)
		r.d[2] = (r.d[2] >> 1) | ((r.d[3] & 1) << 63)
		r.d[3] = r.d[3] >> 1
	}
}

// isZero returns true if the scalar is zero
func (r *Scalar) isZero() bool {
	return (r.d[0] | r.d[1] | r.d[2] | r.d[3]) == 0
}

// isOne returns true if the scalar is one
func (r *Scalar) isOne() bool {
	return r.d[0] == 1 && r.d[1] == 0 && r.d[2] == 0 && r.d[3] == 0
}

// isEven returns true if the scalar is even
func (r *Scalar) isEven() bool {
	return r.d[0]&1 == 0
}

// isHigh returns true if the scalar is > n/2
func (r *Scalar) isHigh() bool {
	var yes, no int

	if r.d[3] < scalarNH3 {
		no = 1
	}
	if r.d[3] > scalarNH3 {
		yes = 1
	}

	if r.d[2] < scalarNH2 {
		no |= (yes ^ 1)
	}
	if r.d[2] > scalarNH2 {
		yes |= (no ^ 1)
	}

	if r.d[1] < scalarNH1 {
		no |= (yes ^ 1)
	}
	if r.d[1] > scalarNH1 {
		yes |= (no ^ 1)
	}

	if r.d[0] > scalarNH0 {
		yes |= (no ^ 1)
	}

	return yes != 0
}

// condNegate conditionally negates the scalar if flag is true
func (r *Scalar) condNegate(flag int) {
	if flag != 0 {
		var neg Scalar
		neg.negate(r)
		*r = neg
	}
}

// equal returns true if two scalars are equal
func (r *Scalar) equal(a *Scalar) bool {
	return subtle.ConstantTimeCompare(
		(*[32]byte)(unsafe.Pointer(&r.d[0]))[:32],
		(*[32]byte)(unsafe.Pointer(&a.d[0]))[:32],
	) == 1
}

// getBits extracts count bits starting at offset
func (r *Scalar) getBits(offset, count uint) uint32 {
	if count == 0 || count > 32 {
		panic("count must be 1-32")
	}
	if offset+count > 256 {
		panic("offset + count must be <= 256")
	}

	limbIdx := offset / 64
	bitIdx := offset % 64

	if bitIdx+count <= 64 {
		// Bits are within a single limb
		return uint32((r.d[limbIdx] >> bitIdx) & ((1 << count) - 1))
	} else {
		// Bits span two limbs
		lowBits := 64 - bitIdx
		highBits := count - lowBits
		low := uint32((r.d[limbIdx] >> bitIdx) & ((1 << lowBits) - 1))
		high := uint32(r.d[limbIdx+1] & ((1 << highBits) - 1))
		return low | (high << lowBits)
	}
}

// cmov conditionally moves a scalar. If flag is true, r = a; otherwise r is unchanged.
func (r *Scalar) cmov(a *Scalar, flag int) {
	mask := uint64(-(int64(flag) & 1))
	r.d[0] ^= mask & (r.d[0] ^ a.d[0])
	r.d[1] ^= mask & (r.d[1] ^ a.d[1])
	r.d[2] ^= mask & (r.d[2] ^ a.d[2])
	r.d[3] ^= mask & (r.d[3] ^ a.d[3])
}

// clear clears a scalar to prevent leaking sensitive information
func (r *Scalar) clear() {
	memclear(unsafe.Pointer(&r.d[0]), unsafe.Sizeof(r.d))
}

// Helper functions for 128-bit arithmetic (using uint128 from field_mul.go)

func uint128FromU64(x uint64) uint128 {
	return uint128{low: x, high: 0}
}

func (x uint128) addU64(y uint64) uint128 {
	low, carry := bits.Add64(x.low, y, 0)
	high := x.high + carry
	return uint128{low: low, high: high}
}

func (x uint128) addMul(a, b uint64) uint128 {
	hi, lo := bits.Mul64(a, b)
	low, carry := bits.Add64(x.low, lo, 0)
	high, _ := bits.Add64(x.high, hi, carry)
	return uint128{low: low, high: high}
}

// Direct function versions to reduce method call overhead
// These are equivalent to the method versions but avoid interface dispatch

// scalarAdd adds two scalars: r = a + b, returns overflow
func scalarAdd(r, a, b *Scalar) bool {
	var carry uint64

	r.d[0], carry = bits.Add64(a.d[0], b.d[0], 0)
	r.d[1], carry = bits.Add64(a.d[1], b.d[1], carry)
	r.d[2], carry = bits.Add64(a.d[2], b.d[2], carry)
	r.d[3], carry = bits.Add64(a.d[3], b.d[3], carry)

	overflow := carry != 0 || scalarCheckOverflow(r)
	if overflow {
		scalarReduce(r, 1)
	}

	return overflow
}

// scalarMul multiplies two scalars: r = a * b
func scalarMul(r, a, b *Scalar) {
	// Use the method version which has the correct 512-bit reduction
	r.mulPureGo(a, b)
}

// scalarGetB32 serializes a scalar to 32 bytes in big-endian format
func scalarGetB32(bin []byte, a *Scalar) {
	if len(bin) != 32 {
		panic("scalar byte array must be 32 bytes")
	}

	// Convert to big-endian bytes
	for i := 0; i < 4; i++ {
		bin[31-8*i] = byte(a.d[i])
		bin[30-8*i] = byte(a.d[i] >> 8)
		bin[29-8*i] = byte(a.d[i] >> 16)
		bin[28-8*i] = byte(a.d[i] >> 24)
		bin[27-8*i] = byte(a.d[i] >> 32)
		bin[26-8*i] = byte(a.d[i] >> 40)
		bin[25-8*i] = byte(a.d[i] >> 48)
		bin[24-8*i] = byte(a.d[i] >> 56)
	}
}

// scalarIsZero returns true if the scalar is zero
func scalarIsZero(a *Scalar) bool {
	return a.d[0] == 0 && a.d[1] == 0 && a.d[2] == 0 && a.d[3] == 0
}

// scalarCheckOverflow checks if the scalar is >= the group order
func scalarCheckOverflow(r *Scalar) bool {
	return (r.d[3] > scalarN3) ||
		(r.d[3] == scalarN3 && r.d[2] > scalarN2) ||
		(r.d[3] == scalarN3 && r.d[2] == scalarN2 && r.d[1] > scalarN1) ||
		(r.d[3] == scalarN3 && r.d[2] == scalarN2 && r.d[1] == scalarN1 && r.d[0] >= scalarN0)
}

// scalarReduce reduces the scalar modulo the group order
func scalarReduce(r *Scalar, overflow int) {
	var t Scalar
	var c uint64

	// Compute r + overflow * N_C
	t.d[0], c = bits.Add64(r.d[0], uint64(overflow)*scalarNC0, 0)
	t.d[1], c = bits.Add64(r.d[1], uint64(overflow)*scalarNC1, c)
	t.d[2], c = bits.Add64(r.d[2], uint64(overflow)*scalarNC2, c)
	t.d[3], c = bits.Add64(r.d[3], 0, c)

	// Mask to keep only the low 256 bits
	r.d[0] = t.d[0] & 0xFFFFFFFFFFFFFFFF
	r.d[1] = t.d[1] & 0xFFFFFFFFFFFFFFFF
	r.d[2] = t.d[2] & 0xFFFFFFFFFFFFFFFF
	r.d[3] = t.d[3] & 0xFFFFFFFFFFFFFFFF

	// Ensure result is in range [0, N)
	if scalarCheckOverflow(r) {
		scalarReduce(r, 1)
	}
}

// wNAF converts a scalar to Windowed Non-Adjacent Form representation
// wNAF represents the scalar using digits in the range [-(2^(w-1)-1), 2^(w-1)-1]
// with the property that non-zero digits are separated by at least w-1 zeros.
//
// Returns the number of digits in the wNAF representation (at most 257 for 256-bit scalars)
// and fills the wnaf array with the digits.
func (s *Scalar) wNAF(wnaf *[257]int8, w uint) int {
	if w < 2 || w > 8 {
		panic("w must be between 2 and 8")
	}

	var k Scalar
	k = *s

	// Note: We do NOT negate the scalar here. The caller is responsible for
	// ensuring the scalar is in the appropriate form. The ecmultEndoSplit
	// function already handles sign normalization.

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

		word := uint32(k.getBits(uint(bit), window)) + carry

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

// wNAFSigned converts a scalar to Windowed Non-Adjacent Form representation,
// handling sign normalization. If the scalar has its high bit set (is "negative"
// in the modular sense), it will be negated and the negated flag will be true.
//
// Returns the number of digits and whether the scalar was negated.
// The caller must negate the result point if negated is true.
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

// =============================================================================
// GLV Endomorphism Support Functions
// =============================================================================

// caddBit conditionally adds a power of 2 to the scalar
// If flag is non-zero, adds 2^bit to r
func (r *Scalar) caddBit(bit uint, flag int) {
	if flag == 0 {
		return
	}

	limbIdx := bit >> 6        // bit / 64
	bitIdx := bit & 0x3F       // bit % 64
	addVal := uint64(1) << bitIdx

	var carry uint64
	if limbIdx == 0 {
		r.d[0], carry = bits.Add64(r.d[0], addVal, 0)
		r.d[1], carry = bits.Add64(r.d[1], 0, carry)
		r.d[2], carry = bits.Add64(r.d[2], 0, carry)
		r.d[3], _ = bits.Add64(r.d[3], 0, carry)
	} else if limbIdx == 1 {
		r.d[1], carry = bits.Add64(r.d[1], addVal, 0)
		r.d[2], carry = bits.Add64(r.d[2], 0, carry)
		r.d[3], _ = bits.Add64(r.d[3], 0, carry)
	} else if limbIdx == 2 {
		r.d[2], carry = bits.Add64(r.d[2], addVal, 0)
		r.d[3], _ = bits.Add64(r.d[3], 0, carry)
	} else if limbIdx == 3 {
		r.d[3], _ = bits.Add64(r.d[3], addVal, 0)
	}
}

// mulShiftVar computes r = round((a * b) >> shift) for shift >= 256
// This is used in GLV scalar splitting to compute c1 = round(k * g1 / 2^384)
// The rounding is achieved by adding the bit just below the shift position
func (r *Scalar) mulShiftVar(a, b *Scalar, shift uint) {
	if shift < 256 {
		panic("mulShiftVar requires shift >= 256")
	}

	// Compute full 512-bit product
	var l [8]uint64
	r.mul512(l[:], a, b)

	// Extract bits [shift, shift+256) from the 512-bit product
	shiftLimbs := shift >> 6      // Number of full 64-bit limbs to skip
	shiftLow := shift & 0x3F      // Bit offset within the limb
	shiftHigh := 64 - shiftLow    // Complementary shift for combining limbs

	// Extract each limb of the result
	// For shift=384, shiftLimbs=6, shiftLow=0
	// r.d[0] = l[6], r.d[1] = l[7], r.d[2] = 0, r.d[3] = 0

	if shift < 512 {
		if shiftLow != 0 {
			r.d[0] = (l[shiftLimbs] >> shiftLow) | (l[shiftLimbs+1] << shiftHigh)
		} else {
			r.d[0] = l[shiftLimbs]
		}
	} else {
		r.d[0] = 0
	}

	if shift < 448 {
		if shiftLow != 0 && shift < 384 {
			r.d[1] = (l[shiftLimbs+1] >> shiftLow) | (l[shiftLimbs+2] << shiftHigh)
		} else if shiftLow != 0 {
			r.d[1] = l[shiftLimbs+1] >> shiftLow
		} else {
			r.d[1] = l[shiftLimbs+1]
		}
	} else {
		r.d[1] = 0
	}

	if shift < 384 {
		if shiftLow != 0 && shift < 320 {
			r.d[2] = (l[shiftLimbs+2] >> shiftLow) | (l[shiftLimbs+3] << shiftHigh)
		} else if shiftLow != 0 {
			r.d[2] = l[shiftLimbs+2] >> shiftLow
		} else {
			r.d[2] = l[shiftLimbs+2]
		}
	} else {
		r.d[2] = 0
	}

	if shift < 320 {
		r.d[3] = l[shiftLimbs+3] >> shiftLow
	} else {
		r.d[3] = 0
	}

	// Round by adding the bit just below the shift position
	// This implements round() instead of floor()
	roundBit := int((l[(shift-1)>>6] >> ((shift - 1) & 0x3F)) & 1)
	r.caddBit(0, roundBit)
}

// splitLambda decomposes scalar k into k1, k2 such that k1 + k2*λ ≡ k (mod n)
// where k1 and k2 are approximately 128 bits each.
// This is the core of the GLV endomorphism optimization.
//
// The algorithm uses precomputed constants g1, g2 to compute:
//   c1 = round(k * g1 / 2^384)
//   c2 = round(k * g2 / 2^384)
//   k2 = c1*(-b1) + c2*(-b2)
//   k1 = k - k2*λ
//
// Reference: libsecp256k1 scalar_impl.h:secp256k1_scalar_split_lambda
func scalarSplitLambda(r1, r2, k *Scalar) {
	var c1, c2 Scalar

	// c1 = round(k * g1 / 2^384)
	c1.mulShiftVar(k, &scalarG1, 384)

	// c2 = round(k * g2 / 2^384)
	c2.mulShiftVar(k, &scalarG2, 384)

	// c1 = c1 * (-b1)
	c1.mul(&c1, &scalarMinusB1)

	// c2 = c2 * (-b2)
	c2.mul(&c2, &scalarMinusB2)

	// r2 = c1 + c2
	r2.add(&c1, &c2)

	// r1 = r2 * λ
	r1.mul(r2, &scalarLambda)

	// r1 = -r1
	r1.negate(r1)

	// r1 = k + (-r2*λ) = k - r2*λ
	r1.add(r1, k)
}

// scalarSplit128 splits a scalar into two 128-bit halves
// r1 = k & ((1 << 128) - 1)  (low 128 bits)
// r2 = k >> 128               (high 128 bits)
// This is used for generator multiplication optimization
func scalarSplit128(r1, r2, k *Scalar) {
	r1.d[0] = k.d[0]
	r1.d[1] = k.d[1]
	r1.d[2] = 0
	r1.d[3] = 0

	r2.d[0] = k.d[2]
	r2.d[1] = k.d[3]
	r2.d[2] = 0
	r2.d[3] = 0
}

