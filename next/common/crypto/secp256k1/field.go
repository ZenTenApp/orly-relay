// Package secp256k1 implements secp256k1 curve operations and BIP-340 Schnorr signatures.
// Pure Go, no math/big, no stdlib crypto.
package secp256k1

// Field element: 256-bit integer mod p, stored as 4 x uint64 (little-endian limbs).
// p = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F

type Fe [4]uint64

var feZero = Fe{0, 0, 0, 0}

var feOne = Fe{1, 0, 0, 0}

// p = field prime
var feP = Fe{
	0xFFFFFFFEFFFFFC2F,
	0xFFFFFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFFFF,
}

// feAdd returns a + b mod p.
func feAdd(a, b Fe) Fe {
	var r Fe
	var carry uint64

	r[0], carry = addWithCarry(a[0], b[0], 0)
	r[1], carry = addWithCarry(a[1], b[1], carry)
	r[2], carry = addWithCarry(a[2], b[2], carry)
	r[3], carry = addWithCarry(a[3], b[3], carry)

	// Reduce mod p if needed.
	return feReduce(r, carry)
}

// feSub returns a - b mod p.
func feSub(a, b Fe) Fe {
	var r Fe
	var borrow uint64

	r[0], borrow = subWithBorrow(a[0], b[0], 0)
	r[1], borrow = subWithBorrow(a[1], b[1], borrow)
	r[2], borrow = subWithBorrow(a[2], b[2], borrow)
	r[3], borrow = subWithBorrow(a[3], b[3], borrow)

	if borrow != 0 {
		// Add p.
		var c uint64
		r[0], c = addWithCarry(r[0], feP[0], 0)
		r[1], c = addWithCarry(r[1], feP[1], c)
		r[2], c = addWithCarry(r[2], feP[2], c)
		r[3], _ = addWithCarry(r[3], feP[3], c)
	}
	return r
}

// feNeg returns -a mod p.
func feNeg(a Fe) Fe {
	if a == feZero {
		return feZero
	}
	return feSub(feP, a)
}

// feMul returns a * b mod p using schoolbook multiplication + Barrett-like reduction.
func feMul(a, b Fe) Fe {
	// Full 512-bit product in 8 limbs.
	var t [8]uint64

	// Schoolbook 4x4 multiply.
	for i := 0; i < 4; i++ {
		var carry uint64
		for j := 0; j < 4; j++ {
			hi, lo := mul64(a[i], b[j])
			lo, c1 := addWithCarry(lo, t[i+j], 0)
			hi += c1
			lo, c2 := addWithCarry(lo, carry, 0)
			hi += c2
			t[i+j] = lo
			carry = hi
		}
		t[i+4] = carry
	}

	return feReduceFull(t)
}

// feSqr returns a^2 mod p.
func feSqr(a Fe) Fe {
	return feMul(a, a)
}

// feInv returns a^(-1) mod p using Fermat's little theorem: a^(p-2) mod p.
func feInv(a Fe) Fe {
	// p-2 = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2D
	// Use square-and-multiply with a compact addition chain.
	// For simplicity, use binary method on p-2.
	pm2 := Fe{
		0xFFFFFFFEFFFFFC2D,
		0xFFFFFFFFFFFFFFFF,
		0xFFFFFFFFFFFFFFFF,
		0xFFFFFFFFFFFFFFFF,
	}
	return feExp(a, pm2)
}

// feExp computes a^e mod p via square-and-multiply.
func feExp(base, exp Fe) Fe {
	result := feOne
	b := base
	for i := 0; i < 4; i++ {
		w := exp[i]
		for bit := 0; bit < 64; bit++ {
			if w&1 == 1 {
				result = feMul(result, b)
			}
			b = feSqr(b)
			w >>= 1
		}
	}
	return result
}

// feSqrt computes sqrt(a) mod p. Returns (result, exists).
// p % 4 == 3, so sqrt(a) = a^((p+1)/4) mod p.
func feSqrt(a Fe) (Fe, bool) {
	// (p+1)/4
	exp := Fe{
		0xFFFFFFFFBFFFFF0C,
		0xFFFFFFFFFFFFFFFF,
		0xFFFFFFFFFFFFFFFF,
		0x3FFFFFFFFFFFFFFF,
	}
	r := feExp(a, exp)
	// Verify: r^2 == a?
	check := feSqr(r)
	if check == a {
		return r, true
	}
	return feZero, false
}

// feFromBytes reads a 32-byte big-endian integer into an Fe.
func feFromBytes(b []byte) Fe {
	var r Fe
	if len(b) < 32 {
		return r
	}
	r[3] = beUint64(b[0:8])
	r[2] = beUint64(b[8:16])
	r[1] = beUint64(b[16:24])
	r[0] = beUint64(b[24:32])
	return r
}

// feToBytes writes an Fe as 32-byte big-endian.
func feToBytes(a Fe) [32]byte {
	var b [32]byte
	putBeUint64(b[0:8], a[3])
	putBeUint64(b[8:16], a[2])
	putBeUint64(b[16:24], a[1])
	putBeUint64(b[24:32], a[0])
	return b
}

// feIsZero returns true if a == 0.
func feIsZero(a Fe) bool {
	return a == feZero
}

// feIsEven returns true if a is even.
func feIsEven(a Fe) bool {
	return a[0]&1 == 0
}

// feCmp returns -1, 0, or 1 comparing a and b.
func feCmp(a, b Fe) int {
	for i := 3; i >= 0; i-- {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// feReduce reduces r with carry into [0, p).
func feReduce(r Fe, carry uint64) Fe {
	// If carry or r >= p, subtract p.
	if carry != 0 || feCmp(r, feP) >= 0 {
		var borrow uint64
		r[0], borrow = subWithBorrow(r[0], feP[0], 0)
		r[1], borrow = subWithBorrow(r[1], feP[1], borrow)
		r[2], borrow = subWithBorrow(r[2], feP[2], borrow)
		r[3], _ = subWithBorrow(r[3], feP[3], borrow)
	}
	return r
}

// feReduceFull reduces a 512-bit product modulo p.
// Uses the property: 2^256 ≡ 0x1000003D1 (mod p).
func feReduceFull(t [8]uint64) Fe {
	// r = t[0..3] + t[4..7] * 2^256
	// 2^256 mod p = 0x1000003D1
	const c = 0x1000003D1

	var r Fe
	var carry uint64

	// Multiply high part by c and add to low part.
	for i := 0; i < 4; i++ {
		hi, lo := mul64(t[i+4], c)
		lo, c1 := addWithCarry(lo, t[i], 0)
		hi += c1
		lo, c2 := addWithCarry(lo, carry, 0)
		hi += c2
		r[i] = lo
		carry = hi
	}

	// carry might still be non-zero; reduce again.
	if carry != 0 {
		hi, lo := mul64(carry, c)
		_ = hi // at most ~38 bits, second reduction won't overflow
		var c3 uint64
		r[0], c3 = addWithCarry(r[0], lo, 0)
		r[1], c3 = addWithCarry(r[1], hi, c3)
		r[2], c3 = addWithCarry(r[2], 0, c3)
		r[3], _ = addWithCarry(r[3], 0, c3)
	}

	// Final reduction: if r >= p, subtract p.
	if feCmp(r, feP) >= 0 {
		var borrow uint64
		r[0], borrow = subWithBorrow(r[0], feP[0], 0)
		r[1], borrow = subWithBorrow(r[1], feP[1], borrow)
		r[2], borrow = subWithBorrow(r[2], feP[2], borrow)
		r[3], _ = subWithBorrow(r[3], feP[3], borrow)
	}

	return r
}

// 64-bit arithmetic helpers.

func addWithCarry(a, b, carry uint64) (uint64, uint64) {
	sum := a + b + carry
	// Carry if sum < a or (sum == a and carry was 1) etc.
	var c uint64
	if sum < a || (sum == a && (b|carry) != 0) {
		c = 1
	}
	return sum, c
}

func subWithBorrow(a, b, borrow uint64) (uint64, uint64) {
	diff := a - b - borrow
	var c uint64
	if a < b+borrow || (borrow != 0 && b == 0xFFFFFFFFFFFFFFFF) {
		c = 1
	}
	return diff, c
}

// mul64 returns the full 128-bit product of a * b as (hi, lo).
func mul64(a, b uint64) (uint64, uint64) {
	// Split into 32-bit halves to avoid overflow.
	aHi := a >> 32
	aLo := a & 0xFFFFFFFF
	bHi := b >> 32
	bLo := b & 0xFFFFFFFF

	ll := aLo * bLo
	hl := aHi * bLo
	lh := aLo * bHi
	hh := aHi * bHi

	mid := hl + (ll >> 32)
	// Check for overflow in mid.
	if mid < hl {
		hh += 1 << 32
	}
	mid2 := mid + lh
	if mid2 < mid {
		hh += 1 << 32
	}

	lo := (mid2 << 32) | (ll & 0xFFFFFFFF)
	hi := hh + (mid2 >> 32)

	return hi, lo
}

func beUint64(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func putBeUint64(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}
