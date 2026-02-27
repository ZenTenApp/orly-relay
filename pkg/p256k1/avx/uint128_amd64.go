//go:build amd64

package avx

import "math/bits"

// AMD64 implementation with AVX2 assembly where beneficial.
// For simple operations, Go with compiler intrinsics is often as fast as assembly.

// Add adds two Uint128 values, returning the result and carry.
func (a Uint128) Add(b Uint128) (result Uint128, carry uint64) {
	result.Lo, carry = bits.Add64(a.Lo, b.Lo, 0)
	result.Hi, carry = bits.Add64(a.Hi, b.Hi, carry)
	return
}

// AddCarry adds two Uint128 values with an input carry.
func (a Uint128) AddCarry(b Uint128, carryIn uint64) (result Uint128, carryOut uint64) {
	result.Lo, carryOut = bits.Add64(a.Lo, b.Lo, carryIn)
	result.Hi, carryOut = bits.Add64(a.Hi, b.Hi, carryOut)
	return
}

// Sub subtracts b from a, returning the result and borrow.
func (a Uint128) Sub(b Uint128) (result Uint128, borrow uint64) {
	result.Lo, borrow = bits.Sub64(a.Lo, b.Lo, 0)
	result.Hi, borrow = bits.Sub64(a.Hi, b.Hi, borrow)
	return
}

// SubBorrow subtracts b from a with an input borrow.
func (a Uint128) SubBorrow(b Uint128, borrowIn uint64) (result Uint128, borrowOut uint64) {
	result.Lo, borrowOut = bits.Sub64(a.Lo, b.Lo, borrowIn)
	result.Hi, borrowOut = bits.Sub64(a.Hi, b.Hi, borrowOut)
	return
}

// Mul64 multiplies two 64-bit values and returns a 128-bit result.
func Mul64(a, b uint64) Uint128 {
	hi, lo := bits.Mul64(a, b)
	return Uint128{Lo: lo, Hi: hi}
}

// Mul multiplies two Uint128 values and returns a 256-bit result as [4]uint64.
// Result is [lo0, lo1, hi0, hi1] where value = lo0 + lo1<<64 + hi0<<128 + hi1<<192
func (a Uint128) Mul(b Uint128) [4]uint64 {
	// Use assembly for the full 128x128->256 multiplication
	return uint128Mul(a, b)
}

// uint128Mul performs 128x128->256 bit multiplication using optimized assembly.
//
//go:noescape
func uint128Mul(a, b Uint128) [4]uint64

// IsZero returns true if the Uint128 is zero.
func (a Uint128) IsZero() bool {
	return a.Lo == 0 && a.Hi == 0
}

// Cmp compares two Uint128 values.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (a Uint128) Cmp(b Uint128) int {
	if a.Hi < b.Hi {
		return -1
	}
	if a.Hi > b.Hi {
		return 1
	}
	if a.Lo < b.Lo {
		return -1
	}
	if a.Lo > b.Lo {
		return 1
	}
	return 0
}

// Lsh shifts a Uint128 left by n bits (n < 128).
func (a Uint128) Lsh(n uint) Uint128 {
	if n >= 64 {
		return Uint128{Lo: 0, Hi: a.Lo << (n - 64)}
	}
	if n == 0 {
		return a
	}
	return Uint128{
		Lo: a.Lo << n,
		Hi: (a.Hi << n) | (a.Lo >> (64 - n)),
	}
}

// Rsh shifts a Uint128 right by n bits (n < 128).
func (a Uint128) Rsh(n uint) Uint128 {
	if n >= 64 {
		return Uint128{Lo: a.Hi >> (n - 64), Hi: 0}
	}
	if n == 0 {
		return a
	}
	return Uint128{
		Lo: (a.Lo >> n) | (a.Hi << (64 - n)),
		Hi: a.Hi >> n,
	}
}

// Or returns the bitwise OR of two Uint128 values.
func (a Uint128) Or(b Uint128) Uint128 {
	return Uint128{Lo: a.Lo | b.Lo, Hi: a.Hi | b.Hi}
}

// And returns the bitwise AND of two Uint128 values.
func (a Uint128) And(b Uint128) Uint128 {
	return Uint128{Lo: a.Lo & b.Lo, Hi: a.Hi & b.Hi}
}

// Xor returns the bitwise XOR of two Uint128 values.
func (a Uint128) Xor(b Uint128) Uint128 {
	return Uint128{Lo: a.Lo ^ b.Lo, Hi: a.Hi ^ b.Hi}
}

// Not returns the bitwise NOT of a Uint128.
func (a Uint128) Not() Uint128 {
	return Uint128{Lo: ^a.Lo, Hi: ^a.Hi}
}
