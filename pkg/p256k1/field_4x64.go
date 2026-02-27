//go:build amd64 && !purego

package p256k1

import (
	"crypto/subtle"
	"errors"
	"math/bits"
	"unsafe"
)

var errField4x64InvalidLen = errors.New("field element must be 32 bytes")

// Field4x64 represents a field element using 4x64-bit limbs with lazy reduction.
// This representation allows fewer partial products (16 vs 25 for 5x52) and
// deferred reduction for sequences of additions.
//
// The value is: n[0] + n[1]*2^64 + n[2]*2^128 + n[3]*2^192
//
// Magnitude tracks how many additions have occurred since the last reduction.
// Multiplication requires magnitude ≤ 1 (fully reduced inputs).
type Field4x64 struct {
	n          [4]uint64
	magnitude  int  // 1 = normalized, increases with additions
	normalized bool // whether the field element is fully normalized
}

// secp256k1 field prime: p = 2^256 - 2^32 - 977
// p = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
var field4x64Prime = [4]uint64{
	0xFFFFFFFEFFFFFC2F, // n[0]
	0xFFFFFFFFFFFFFFFF, // n[1]
	0xFFFFFFFFFFFFFFFF, // n[2]
	0xFFFFFFFFFFFFFFFF, // n[3]
}

// 2^256 mod p = 2^32 + 977 = 0x1000003D1
const field4x64R = uint64(0x1000003D1)

// SetZero sets f to zero.
func (f *Field4x64) SetZero() *Field4x64 {
	f.n[0], f.n[1], f.n[2], f.n[3] = 0, 0, 0, 0
	f.magnitude = 0
	f.normalized = true
	return f
}

// SetOne sets f to one.
func (f *Field4x64) SetOne() *Field4x64 {
	f.n[0], f.n[1], f.n[2], f.n[3] = 1, 0, 0, 0
	f.magnitude = 1
	f.normalized = true
	return f
}

// Set copies a into f.
func (f *Field4x64) Set(a *Field4x64) *Field4x64 {
	f.n = a.n
	f.magnitude = a.magnitude
	f.normalized = a.normalized
	return f
}

// IsZero returns true if f is zero.
func (f *Field4x64) IsZero() bool {
	// Must be normalized first
	return f.n[0] == 0 && f.n[1] == 0 && f.n[2] == 0 && f.n[3] == 0
}

// Equal returns true if f equals a.
func (f *Field4x64) Equal(a *Field4x64) bool {
	// Both must be normalized
	return f.n[0] == a.n[0] && f.n[1] == a.n[1] && f.n[2] == a.n[2] && f.n[3] == a.n[3]
}

// Reduce reduces f modulo p, setting magnitude to 1.
// Uses the identity: 2^256 ≡ 2^32 + 977 (mod p)
func (f *Field4x64) Reduce() *Field4x64 {
	// Fast path: already normalized
	if f.normalized {
		return f
	}

	n0, n1, n2, n3 := f.n[0], f.n[1], f.n[2], f.n[3]

	// Reduce: if value >= p, subtract p
	// Check if n >= p by comparing from high limb
	var c uint64
	var borrow uint64

	// Subtract p and check for borrow
	t0, borrow := bits.Sub64(n0, field4x64Prime[0], 0)
	t1, borrow := bits.Sub64(n1, field4x64Prime[1], borrow)
	t2, borrow := bits.Sub64(n2, field4x64Prime[2], borrow)
	t3, borrow := bits.Sub64(n3, field4x64Prime[3], borrow)

	// If no borrow, result is valid (n >= p), use subtracted value
	// If borrow, n < p, keep original
	c = borrow - 1 // c = 0xFFFF... if no borrow, 0 if borrow

	f.n[0] = (t0 & c) | (n0 & ^c)
	f.n[1] = (t1 & c) | (n1 & ^c)
	f.n[2] = (t2 & c) | (n2 & ^c)
	f.n[3] = (t3 & c) | (n3 & ^c)
	f.magnitude = 1
	f.normalized = true

	return f
}

// Add computes f = a + b (mod p) with lazy reduction.
func (f *Field4x64) Add(a, b *Field4x64) *Field4x64 {
	var carry uint64

	f.n[0], carry = bits.Add64(a.n[0], b.n[0], 0)
	f.n[1], carry = bits.Add64(a.n[1], b.n[1], carry)
	f.n[2], carry = bits.Add64(a.n[2], b.n[2], carry)
	f.n[3], carry = bits.Add64(a.n[3], b.n[3], carry)

	// If carry, we overflowed 2^256, reduce by adding R = 2^256 mod p
	if carry != 0 {
		f.n[0], carry = bits.Add64(f.n[0], field4x64R, 0)
		f.n[1], carry = bits.Add64(f.n[1], 0, carry)
		f.n[2], carry = bits.Add64(f.n[2], 0, carry)
		f.n[3], _ = bits.Add64(f.n[3], 0, carry)
	}

	f.magnitude = a.magnitude + b.magnitude
	f.normalized = false
	if f.magnitude > 8 { // Reduce if magnitude gets too high
		f.Reduce()
	}

	return f
}

// Sub computes f = a - b (mod p).
func (f *Field4x64) Sub(a, b *Field4x64) *Field4x64 {
	var borrow uint64

	f.n[0], borrow = bits.Sub64(a.n[0], b.n[0], 0)
	f.n[1], borrow = bits.Sub64(a.n[1], b.n[1], borrow)
	f.n[2], borrow = bits.Sub64(a.n[2], b.n[2], borrow)
	f.n[3], borrow = bits.Sub64(a.n[3], b.n[3], borrow)

	// If borrow, we went negative, add p back
	if borrow != 0 {
		var carry uint64
		f.n[0], carry = bits.Add64(f.n[0], field4x64Prime[0], 0)
		f.n[1], carry = bits.Add64(f.n[1], field4x64Prime[1], carry)
		f.n[2], carry = bits.Add64(f.n[2], field4x64Prime[2], carry)
		f.n[3], _ = bits.Add64(f.n[3], field4x64Prime[3], carry)
	}

	f.magnitude = 1
	f.normalized = false
	return f
}

// Negate computes f = -a (mod p).
// Handles inputs that may be in range [0, 2p).
func (f *Field4x64) Negate(a *Field4x64) *Field4x64 {
	// -a = p - a (when a != 0)
	if a.IsZero() {
		return f.SetZero()
	}

	// Compute p - a. If a >= p, this will underflow (borrow != 0).
	var borrow uint64
	f.n[0], borrow = bits.Sub64(field4x64Prime[0], a.n[0], 0)
	f.n[1], borrow = bits.Sub64(field4x64Prime[1], a.n[1], borrow)
	f.n[2], borrow = bits.Sub64(field4x64Prime[2], a.n[2], borrow)
	f.n[3], borrow = bits.Sub64(field4x64Prime[3], a.n[3], borrow)

	// If there was a borrow, a >= p. We need to compute p - (a - p) = 2p - a.
	// But since we computed p - a which underflowed, the result is 2^256 + p - a = 2^256 - (a - p).
	// We need to add p to get: 2^256 + 2p - a. Then subtract 2^256 (which is implicit).
	// Actually: if a in [p, 2p), then -a = p - a is negative, and we wrapped to 2^256 + p - a.
	// We want the result in [0, p). Since a < 2p, we have a - p < p, so -(a-p) = p - (a-p) = 2p - a.
	// The wrapped result 2^256 + p - a needs to be converted to 2p - a by adding p (mod 2^256).
	if borrow != 0 {
		var carry uint64
		f.n[0], carry = bits.Add64(f.n[0], field4x64Prime[0], 0)
		f.n[1], carry = bits.Add64(f.n[1], field4x64Prime[1], carry)
		f.n[2], carry = bits.Add64(f.n[2], field4x64Prime[2], carry)
		f.n[3], _ = bits.Add64(f.n[3], field4x64Prime[3], carry)
	}

	f.magnitude = 1
	f.normalized = false
	return f
}

// MulPureGo computes f = a * b (mod p) using pure Go.
// This is the reference implementation; assembly version is faster.
func (f *Field4x64) MulPureGo(a, b *Field4x64) *Field4x64 {
	// Ensure inputs are reduced
	if a.magnitude > 1 {
		a.Reduce()
	}
	if b.magnitude > 1 {
		b.Reduce()
	}

	// Compute 512-bit product using schoolbook multiplication
	// Use 128-bit accumulators (represented as hi:lo pairs)
	// Note: bits.Mul64 returns (hi, lo) - high word first!
	var r [8]uint64

	// For each output limb r[k], accumulate all products a[i]*b[j] where i+j=k
	// Use proper carry propagation throughout

	var c0, c1, c2 uint64 // Three-word accumulator (c2:c1:c0)
	var hi, lo uint64

	// Column 0: a0*b0
	hi, lo = bits.Mul64(a.n[0], b.n[0])
	c0 = lo
	c1 = hi
	c2 = 0
	r[0] = c0
	c0 = c1
	c1 = c2
	c2 = 0

	// Column 1: a0*b1 + a1*b0
	hi, lo = bits.Mul64(a.n[0], b.n[1])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[1], b.n[0])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	r[1] = c0
	c0 = c1
	c1 = c2
	c2 = 0

	// Column 2: a0*b2 + a1*b1 + a2*b0
	hi, lo = bits.Mul64(a.n[0], b.n[2])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[1], b.n[1])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[2], b.n[0])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	r[2] = c0
	c0 = c1
	c1 = c2
	c2 = 0

	// Column 3: a0*b3 + a1*b2 + a2*b1 + a3*b0
	hi, lo = bits.Mul64(a.n[0], b.n[3])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[1], b.n[2])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[2], b.n[1])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[3], b.n[0])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	r[3] = c0
	c0 = c1
	c1 = c2
	c2 = 0

	// Column 4: a1*b3 + a2*b2 + a3*b1
	hi, lo = bits.Mul64(a.n[1], b.n[3])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[2], b.n[2])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[3], b.n[1])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	r[4] = c0
	c0 = c1
	c1 = c2
	c2 = 0

	// Column 5: a2*b3 + a3*b2
	hi, lo = bits.Mul64(a.n[2], b.n[3])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	hi, lo = bits.Mul64(a.n[3], b.n[2])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, hi = bits.Add64(c1, hi, lo)
	c2 += hi

	r[5] = c0
	c0 = c1
	c1 = c2

	// Column 6: a3*b3
	hi, lo = bits.Mul64(a.n[3], b.n[3])
	c0, lo = bits.Add64(c0, lo, 0)
	c1, _ = bits.Add64(c1, hi, lo)

	r[6] = c0
	r[7] = c1

	// Reduce 512-bit result modulo p
	f.reduce512(&r)

	return f
}

// reduce512 reduces a 512-bit value to 256 bits modulo p.
// Uses the identity: 2^256 ≡ 2^32 + 977 (mod p)
func (f *Field4x64) reduce512(r *[8]uint64) {
	// High part (r[4..7]) needs to be multiplied by R = 2^32 + 977 and added to low part

	// Step 1: Compute r[4..7] * R and add to r[0..3]
	// R = 0x1000003D1
	// Note: bits.Mul64 returns (hi, lo)

	var hi, lo, carry uint64
	var t [5]uint64 // Accumulator for r[4..7] * R

	// r[4] * R
	hi, lo = bits.Mul64(r[4], field4x64R)
	t[0] = lo
	t[1] = hi

	// r[5] * R
	hi, lo = bits.Mul64(r[5], field4x64R)
	t[1], carry = bits.Add64(t[1], lo, 0)
	t[2] = hi + carry

	// r[6] * R
	hi, lo = bits.Mul64(r[6], field4x64R)
	t[2], carry = bits.Add64(t[2], lo, 0)
	t[3] = hi + carry

	// r[7] * R
	hi, lo = bits.Mul64(r[7], field4x64R)
	t[3], carry = bits.Add64(t[3], lo, 0)
	t[4] = hi + carry

	// Add t to r[0..3]
	var c uint64
	f.n[0], c = bits.Add64(r[0], t[0], 0)
	f.n[1], c = bits.Add64(r[1], t[1], c)
	f.n[2], c = bits.Add64(r[2], t[2], c)
	f.n[3], c = bits.Add64(r[3], t[3], c)

	// Handle overflow from t[4] + carry
	overflow := t[4] + c

	// If there's overflow, multiply it by R and add again
	if overflow != 0 {
		hi, lo = bits.Mul64(overflow, field4x64R)
		f.n[0], c = bits.Add64(f.n[0], lo, 0)
		f.n[1], c = bits.Add64(f.n[1], hi, c)
		f.n[2], c = bits.Add64(f.n[2], 0, c)
		f.n[3], c = bits.Add64(f.n[3], 0, c)

		// One more reduction if still overflowing (very rare)
		if c != 0 {
			f.n[0], c = bits.Add64(f.n[0], field4x64R, 0)
			f.n[1], c = bits.Add64(f.n[1], 0, c)
			f.n[2], c = bits.Add64(f.n[2], 0, c)
			f.n[3], _ = bits.Add64(f.n[3], 0, c)
		}
	}

	f.magnitude = 1
	f.Reduce() // Final reduction to ensure result < p
}

// Sqr computes f = a^2 (mod p).
// Uses optimized assembly that exploits squaring symmetry.
func (f *Field4x64) Sqr(a *Field4x64) *Field4x64 {
	// Ensure input is reduced
	if a.magnitude > 1 {
		a.Reduce()
	}
	field4x64SqrAsm(&f.n, &a.n)
	f.magnitude = 1
	f.normalized = false
	return f
}

// Mul computes f = a * b (mod p).
// Uses BMI2 MULX assembly for high performance.
func (f *Field4x64) Mul(a, b *Field4x64) *Field4x64 {
	// Ensure inputs are reduced
	if a.magnitude > 1 {
		a.Reduce()
	}
	if b.magnitude > 1 {
		b.Reduce()
	}
	field4x64MulAsm(&f.n, &a.n, &b.n)
	f.magnitude = 1
	f.normalized = false
	return f
}

// SetB32 sets f from a 32-byte big-endian representation.
func (f *Field4x64) SetB32(b []byte) error {
	if len(b) != 32 {
		return errField4x64InvalidLen
	}

	// Big-endian: b[0] is most significant
	f.n[3] = uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	f.n[2] = uint64(b[8])<<56 | uint64(b[9])<<48 | uint64(b[10])<<40 | uint64(b[11])<<32 |
		uint64(b[12])<<24 | uint64(b[13])<<16 | uint64(b[14])<<8 | uint64(b[15])
	f.n[1] = uint64(b[16])<<56 | uint64(b[17])<<48 | uint64(b[18])<<40 | uint64(b[19])<<32 |
		uint64(b[20])<<24 | uint64(b[21])<<16 | uint64(b[22])<<8 | uint64(b[23])
	f.n[0] = uint64(b[24])<<56 | uint64(b[25])<<48 | uint64(b[26])<<40 | uint64(b[27])<<32 |
		uint64(b[28])<<24 | uint64(b[29])<<16 | uint64(b[30])<<8 | uint64(b[31])

	f.magnitude = 1
	f.normalized = false
	return nil
}

// GetB32 writes f to a 32-byte big-endian representation.
func (f *Field4x64) GetB32(b []byte) {
	if len(b) != 32 {
		panic("GetB32: buffer must be 32 bytes")
	}

	// Ensure normalized
	if f.magnitude > 1 {
		f.Reduce()
	}

	// Big-endian: b[0] is most significant
	b[0] = byte(f.n[3] >> 56)
	b[1] = byte(f.n[3] >> 48)
	b[2] = byte(f.n[3] >> 40)
	b[3] = byte(f.n[3] >> 32)
	b[4] = byte(f.n[3] >> 24)
	b[5] = byte(f.n[3] >> 16)
	b[6] = byte(f.n[3] >> 8)
	b[7] = byte(f.n[3])
	b[8] = byte(f.n[2] >> 56)
	b[9] = byte(f.n[2] >> 48)
	b[10] = byte(f.n[2] >> 40)
	b[11] = byte(f.n[2] >> 32)
	b[12] = byte(f.n[2] >> 24)
	b[13] = byte(f.n[2] >> 16)
	b[14] = byte(f.n[2] >> 8)
	b[15] = byte(f.n[2])
	b[16] = byte(f.n[1] >> 56)
	b[17] = byte(f.n[1] >> 48)
	b[18] = byte(f.n[1] >> 40)
	b[19] = byte(f.n[1] >> 32)
	b[20] = byte(f.n[1] >> 24)
	b[21] = byte(f.n[1] >> 16)
	b[22] = byte(f.n[1] >> 8)
	b[23] = byte(f.n[1])
	b[24] = byte(f.n[0] >> 56)
	b[25] = byte(f.n[0] >> 48)
	b[26] = byte(f.n[0] >> 40)
	b[27] = byte(f.n[0] >> 32)
	b[28] = byte(f.n[0] >> 24)
	b[29] = byte(f.n[0] >> 16)
	b[30] = byte(f.n[0] >> 8)
	b[31] = byte(f.n[0])
}

// ============================================================================
// FieldElement-compatible methods (lowercase for internal API compatibility)
// ============================================================================

// normalize normalizes f to canonical form (same as Reduce).
func (f *Field4x64) normalize() {
	f.Reduce()
}

// normalizeWeak performs weak normalization (magnitude <= 1).
func (f *Field4x64) normalizeWeak() {
	if f.magnitude <= 1 {
		return
	}
	f.Reduce()
}

// isZero returns true if f is zero (must be normalized).
func (f *Field4x64) isZero() bool {
	if !f.normalized {
		panic("field element must be normalized")
	}
	return f.n[0] == 0 && f.n[1] == 0 && f.n[2] == 0 && f.n[3] == 0
}

// isOdd returns true if f is odd (must be normalized).
func (f *Field4x64) isOdd() bool {
	if !f.normalized {
		panic("field element must be normalized")
	}
	return f.n[0]&1 == 1
}

// normalizesToZeroVar checks if f normalizes to zero (variable-time).
func (f *Field4x64) normalizesToZeroVar() bool {
	var t Field4x64
	t = *f
	t.normalize()
	return t.isZero()
}

// equal returns true if f equals a (both must be normalized).
func (f *Field4x64) equal(a *Field4x64) bool {
	if !f.normalized || !a.normalized {
		panic("field elements must be normalized for comparison")
	}
	return subtle.ConstantTimeCompare(
		(*[32]byte)(unsafe.Pointer(&f.n[0]))[:32],
		(*[32]byte)(unsafe.Pointer(&a.n[0]))[:32],
	) == 1
}

// setInt sets f to a small integer value.
func (f *Field4x64) setInt(a int) {
	if a < 0 || a > 0x7FFF {
		panic("value out of range")
	}
	f.n[0] = uint64(a)
	f.n[1] = 0
	f.n[2] = 0
	f.n[3] = 0
	if a == 0 {
		f.magnitude = 0
	} else {
		f.magnitude = 1
	}
	f.normalized = true
}

// clear clears f to prevent leaking sensitive information.
func (f *Field4x64) clear() {
	f.n[0], f.n[1], f.n[2], f.n[3] = 0, 0, 0, 0
	f.magnitude = 0
	f.normalized = true
}

// negate negates f: r = -a with given magnitude.
func (f *Field4x64) negate(a *Field4x64, m int) {
	if m < 0 || m > 31 {
		panic("magnitude out of range")
	}
	// For 4x64, we use a simpler approach: just compute p - a
	// The magnitude parameter is for compatibility with 5x52
	f.Negate(a)
	f.magnitude = m + 1
	f.normalized = false
}

// add adds a to f in place: f += a.
func (f *Field4x64) add(a *Field4x64) {
	var carry uint64
	f.n[0], carry = bits.Add64(f.n[0], a.n[0], 0)
	f.n[1], carry = bits.Add64(f.n[1], a.n[1], carry)
	f.n[2], carry = bits.Add64(f.n[2], a.n[2], carry)
	f.n[3], carry = bits.Add64(f.n[3], a.n[3], carry)

	if carry != 0 {
		f.n[0], carry = bits.Add64(f.n[0], field4x64R, 0)
		f.n[1], carry = bits.Add64(f.n[1], 0, carry)
		f.n[2], carry = bits.Add64(f.n[2], 0, carry)
		f.n[3], _ = bits.Add64(f.n[3], 0, carry)
	}

	f.magnitude += a.magnitude
	f.normalized = false
	if f.magnitude > 8 {
		f.Reduce()
	}
}

// sub subtracts a from f in place: f -= a.
func (f *Field4x64) sub(a *Field4x64) {
	var negA Field4x64
	negA.negate(a, a.magnitude)
	f.add(&negA)
}

// mulInt multiplies f by a small integer.
func (f *Field4x64) mulInt(a int) {
	if a < 0 || a > 32 {
		panic("multiplier out of range")
	}
	ua := uint64(a)

	// Multiply each limb
	var carry uint64
	var hi uint64

	hi, f.n[0] = bits.Mul64(f.n[0], ua)
	carry = hi

	hi, f.n[1] = bits.Mul64(f.n[1], ua)
	f.n[1], carry = bits.Add64(f.n[1], carry, 0)
	carry = hi + carry

	hi, f.n[2] = bits.Mul64(f.n[2], ua)
	f.n[2], carry = bits.Add64(f.n[2], carry, 0)
	carry = hi + carry

	hi, f.n[3] = bits.Mul64(f.n[3], ua)
	f.n[3], carry = bits.Add64(f.n[3], carry, 0)
	carry = hi + carry

	// Handle overflow
	if carry != 0 {
		var c uint64
		hi, lo := bits.Mul64(carry, field4x64R)
		f.n[0], c = bits.Add64(f.n[0], lo, 0)
		f.n[1], c = bits.Add64(f.n[1], hi, c)
		f.n[2], c = bits.Add64(f.n[2], 0, c)
		f.n[3], _ = bits.Add64(f.n[3], 0, c)
	}

	f.magnitude *= a
	f.normalized = false
}

// cmov conditionally moves a into f if flag is non-zero.
func (f *Field4x64) cmov(a *Field4x64, flag int) {
	mask := uint64(-(int64(flag) & 1))
	f.n[0] ^= mask & (f.n[0] ^ a.n[0])
	f.n[1] ^= mask & (f.n[1] ^ a.n[1])
	f.n[2] ^= mask & (f.n[2] ^ a.n[2])
	f.n[3] ^= mask & (f.n[3] ^ a.n[3])

	if flag != 0 {
		f.magnitude = a.magnitude
		f.normalized = a.normalized
	}
}

// toStorage converts f to storage format.
func (f *Field4x64) toStorage(s *FieldElementStorage) {
	if !f.normalized {
		f.normalize()
	}
	s.n = f.n
}

// fromStorage converts from storage format to f.
func (f *Field4x64) fromStorage(s *FieldElementStorage) {
	f.n = s.n
	f.magnitude = 1
	f.normalized = false
}

// setB32 sets f from 32-byte big-endian representation (lowercase version).
func (f *Field4x64) setB32(b []byte) error {
	return f.SetB32(b)
}

// getB32 writes f to 32-byte big-endian representation (lowercase version).
func (f *Field4x64) getB32(b []byte) {
	f.GetB32(b)
}

// mul multiplies two field elements: f = a * b (lowercase version).
func (f *Field4x64) mul(a, b *Field4x64) {
	f.Mul(a, b)
}

// sqr squares a field element: f = a^2 (lowercase version).
func (f *Field4x64) sqr(a *Field4x64) {
	f.Sqr(a)
}

// inv computes modular inverse using Fermat's little theorem: f = a^(p-2).
// Uses an optimized addition chain for efficiency.
//
// p-2 = 0xFFFFFFFF FFFFFFFF FFFFFFFF FFFFFFFF FFFFFFFF FFFFFFFF FFFFFFFE FFFFFC2D
//
// Binary structure:
//   - bits 255-33: 223 ones
//   - bit 32: 0
//   - bits 31-12: 20 ones
//   - bits 11-0: 0xC2D
func (f *Field4x64) inv(a *Field4x64) {
	var aNorm Field4x64
	aNorm = *a
	aNorm.normalize()

	// Build up powers using addition chain (same as sqrt)
	var x2, x3, x6, x9, x11, x22, x44, x88, x176, x220, x223, t1 Field4x64

	// x2 = a^(2^2 - 1) = a^3
	x2.sqr(&aNorm)
	x2.mul(&x2, &aNorm)

	// x3 = a^(2^3 - 1) = a^7
	x3.sqr(&x2)
	x3.mul(&x3, &aNorm)

	// x6 = a^(2^6 - 1) = a^63
	x6 = x3
	for j := 0; j < 3; j++ {
		x6.sqr(&x6)
	}
	x6.mul(&x6, &x3)

	// x9 = a^(2^9 - 1) = a^511
	x9 = x6
	for j := 0; j < 3; j++ {
		x9.sqr(&x9)
	}
	x9.mul(&x9, &x3)

	// x11 = a^(2^11 - 1) = a^2047
	x11 = x9
	for j := 0; j < 2; j++ {
		x11.sqr(&x11)
	}
	x11.mul(&x11, &x2)

	// x22 = a^(2^22 - 1)
	x22 = x11
	for j := 0; j < 11; j++ {
		x22.sqr(&x22)
	}
	x22.mul(&x22, &x11)

	// x44 = a^(2^44 - 1)
	x44 = x22
	for j := 0; j < 22; j++ {
		x44.sqr(&x44)
	}
	x44.mul(&x44, &x22)

	// x88 = a^(2^88 - 1)
	x88 = x44
	for j := 0; j < 44; j++ {
		x88.sqr(&x88)
	}
	x88.mul(&x88, &x44)

	// x176 = a^(2^176 - 1)
	x176 = x88
	for j := 0; j < 88; j++ {
		x176.sqr(&x176)
	}
	x176.mul(&x176, &x88)

	// x220 = a^(2^220 - 1)
	x220 = x176
	for j := 0; j < 44; j++ {
		x220.sqr(&x220)
	}
	x220.mul(&x220, &x44)

	// x223 = a^(2^223 - 1)
	x223 = x220
	for j := 0; j < 3; j++ {
		x223.sqr(&x223)
	}
	x223.mul(&x223, &x3)

	// x20 = a^(2^20 - 1) (needed for bits 31-12)
	var x20 Field4x64
	x20 = x11
	for j := 0; j < 9; j++ {
		x20.sqr(&x20)
	}
	x20.mul(&x20, &x9)

	// Assemble p-2:
	// Start with x223 and shift by 33 bits
	t1 = x223
	for j := 0; j < 33; j++ {
		t1.sqr(&t1)
	}
	// t1 = a^((2^223-1) * 2^33) = a^(2^256 - 2^33)

	// Add contribution for bits 31-12 (20 ones): x20 shifted by 12
	var temp Field4x64
	temp = x20
	for j := 0; j < 12; j++ {
		temp.sqr(&temp)
	}
	// temp = a^((2^20-1)*2^12) = a^(2^32 - 2^12)

	t1.mul(&t1, &temp)
	// t1 = a^(2^256 - 2^32 - 2^12)

	// Add bits 11-0 = 0xC2D = 3117
	// Compute a^3117 using binary exponentiation (small number)
	var low, base Field4x64
	low.setInt(1)
	base = aNorm
	e := uint32(3117)
	for e > 0 {
		if e&1 == 1 {
			low.mul(&low, &base)
		}
		e >>= 1
		if e > 0 {
			base.sqr(&base)
		}
	}
	// low = a^3117

	t1.mul(&t1, &low)
	// t1 = a^(2^256 - 2^32 - 979) = a^(p-2)

	*f = t1
	f.magnitude = 1
	f.normalized = true
}

// sqrt computes square root of f if it exists.
func (f *Field4x64) sqrt(a *Field4x64) bool {
	// Normalize input
	var aNorm Field4x64
	aNorm = *a
	if aNorm.magnitude > 8 {
		aNorm.normalizeWeak()
	} else {
		aNorm.normalize()
	}

	// For p ≡ 3 (mod 4), sqrt(a) = a^((p+1)/4)
	// Use addition chain for efficiency
	var x2, x3, x6, x9, x11, x22, x44, x88, x176, x220, x223, t1 Field4x64

	// x2 = a^3
	x2.sqr(&aNorm)
	x2.mul(&x2, &aNorm)

	// x3 = a^7
	x3.sqr(&x2)
	x3.mul(&x3, &aNorm)

	// x6 = a^63
	x6 = x3
	for j := 0; j < 3; j++ {
		x6.sqr(&x6)
	}
	x6.mul(&x6, &x3)

	// x9 = a^511
	x9 = x6
	for j := 0; j < 3; j++ {
		x9.sqr(&x9)
	}
	x9.mul(&x9, &x3)

	// x11 = a^2047
	x11 = x9
	for j := 0; j < 2; j++ {
		x11.sqr(&x11)
	}
	x11.mul(&x11, &x2)

	// x22 = a^4194303
	x22 = x11
	for j := 0; j < 11; j++ {
		x22.sqr(&x22)
	}
	x22.mul(&x22, &x11)

	// x44 = a^17592186044415
	x44 = x22
	for j := 0; j < 22; j++ {
		x44.sqr(&x44)
	}
	x44.mul(&x44, &x22)

	// x88
	x88 = x44
	for j := 0; j < 44; j++ {
		x88.sqr(&x88)
	}
	x88.mul(&x88, &x44)

	// x176
	x176 = x88
	for j := 0; j < 88; j++ {
		x176.sqr(&x176)
	}
	x176.mul(&x176, &x88)

	// x220
	x220 = x176
	for j := 0; j < 44; j++ {
		x220.sqr(&x220)
	}
	x220.mul(&x220, &x44)

	// x223
	x223 = x220
	for j := 0; j < 3; j++ {
		x223.sqr(&x223)
	}
	x223.mul(&x223, &x3)

	// Final assembly
	t1 = x223
	for j := 0; j < 23; j++ {
		t1.sqr(&t1)
	}
	t1.mul(&t1, &x22)
	for j := 0; j < 6; j++ {
		t1.sqr(&t1)
	}
	t1.mul(&t1, &x2)
	t1.sqr(&t1)
	f.sqr(&t1)

	// Verify: check that f^2 == a
	var check Field4x64
	check.sqr(f)
	check.normalize()
	aNorm.normalize()

	return check.equal(&aNorm)
}

// half computes f = a/2 (mod p).
func (f *Field4x64) half(a *Field4x64) {
	*f = *a

	// If odd, add p first (so result is even)
	// We need to preserve the overflow bit for the right shift
	var overflow uint64
	if f.n[0]&1 == 1 {
		var carry uint64
		f.n[0], carry = bits.Add64(f.n[0], field4x64Prime[0], 0)
		f.n[1], carry = bits.Add64(f.n[1], field4x64Prime[1], carry)
		f.n[2], carry = bits.Add64(f.n[2], field4x64Prime[2], carry)
		f.n[3], overflow = bits.Add64(f.n[3], field4x64Prime[3], carry)
	}

	// Right shift by 1, with overflow bit becoming the top bit
	f.n[0] = (f.n[0] >> 1) | (f.n[1] << 63)
	f.n[1] = (f.n[1] >> 1) | (f.n[2] << 63)
	f.n[2] = (f.n[2] >> 1) | (f.n[3] << 63)
	f.n[3] = (f.n[3] >> 1) | (overflow << 63)

	f.magnitude = (f.magnitude >> 1) + 1
	f.normalized = false
}
