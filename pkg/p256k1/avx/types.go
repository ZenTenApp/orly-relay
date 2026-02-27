// Package avx provides AVX2-accelerated secp256k1 operations using 128-bit limbs.
//
// This implementation uses 128-bit limbs stored in 256-bit AVX2 registers:
//   - Scalar: 256-bit value as 2×128-bit limbs (fits in 1 YMM register)
//   - FieldElement: 256-bit value as 2×128-bit limbs (fits in 1 YMM register)
//   - AffinePoint: 512-bit (x,y) as 2×256-bit (fits in 2 YMM registers)
//   - JacobianPoint: 768-bit (x,y,z) as 3×256-bit (fits in 3 YMM registers)
package avx

// Uint128 represents a 128-bit unsigned integer as two 64-bit limbs.
// This is the fundamental building block for AVX2 operations.
// In AVX2 assembly, two Uint128 values fit in a single YMM register.
type Uint128 struct {
	Lo, Hi uint64 // Lo + Hi<<64
}

// Scalar represents a 256-bit scalar value modulo the secp256k1 group order.
// Uses 2×128-bit limbs for efficient AVX2 processing.
// The entire scalar fits in a single YMM register.
type Scalar struct {
	D [2]Uint128 // D[0] is low 128 bits, D[1] is high 128 bits
}

// FieldElement represents a field element modulo the secp256k1 field prime.
// Uses 2×128-bit limbs for efficient AVX2 processing.
// The entire field element fits in a single YMM register.
type FieldElement struct {
	N [2]Uint128 // N[0] is low 128 bits, N[1] is high 128 bits
}

// AffinePoint represents a point on the secp256k1 curve in affine coordinates.
// Uses 2 YMM registers (one for X, one for Y).
type AffinePoint struct {
	X, Y     FieldElement
	Infinity bool
}

// JacobianPoint represents a point in Jacobian coordinates (X, Y, Z).
// Affine coordinates are (X/Z², Y/Z³).
// Uses 3 YMM registers (one each for X, Y, Z).
type JacobianPoint struct {
	X, Y, Z  FieldElement
	Infinity bool
}

// Constants for secp256k1

// Group order n = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
var (
	ScalarN = Scalar{
		D: [2]Uint128{
			{Lo: 0xBFD25E8CD0364141, Hi: 0xBAAEDCE6AF48A03B}, // low 128 bits
			{Lo: 0xFFFFFFFFFFFFFFFE, Hi: 0xFFFFFFFFFFFFFFFF}, // high 128 bits
		},
	}

	// 2^256 - n (used for reduction)
	ScalarNC = Scalar{
		D: [2]Uint128{
			{Lo: 0x402DA1732FC9BEBF, Hi: 0x4551231950B75FC4}, // low 128 bits
			{Lo: 0x0000000000000001, Hi: 0x0000000000000000}, // high 128 bits
		},
	}

	// n/2 (for checking if scalar is high)
	ScalarNHalf = Scalar{
		D: [2]Uint128{
			{Lo: 0xDFE92F46681B20A0, Hi: 0x5D576E7357A4501D}, // low 128 bits
			{Lo: 0xFFFFFFFFFFFFFFFF, Hi: 0x7FFFFFFFFFFFFFFF}, // high 128 bits
		},
	}

	ScalarZero = Scalar{}
	ScalarOne  = Scalar{D: [2]Uint128{{Lo: 1, Hi: 0}, {Lo: 0, Hi: 0}}}
)

// Field prime p = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
var (
	FieldP = FieldElement{
		N: [2]Uint128{
			{Lo: 0xFFFFFFFEFFFFFC2F, Hi: 0xFFFFFFFFFFFFFFFF}, // low 128 bits
			{Lo: 0xFFFFFFFFFFFFFFFF, Hi: 0xFFFFFFFFFFFFFFFF}, // high 128 bits
		},
	}

	// 2^256 - p = 2^32 + 977 = 0x1000003D1
	FieldPC = FieldElement{
		N: [2]Uint128{
			{Lo: 0x1000003D1, Hi: 0}, // low 128 bits
			{Lo: 0, Hi: 0},           // high 128 bits
		},
	}

	FieldZero = FieldElement{}
	FieldOne  = FieldElement{N: [2]Uint128{{Lo: 1, Hi: 0}, {Lo: 0, Hi: 0}}}
)

// Generator point G for secp256k1
var (
	GeneratorX = FieldElement{
		N: [2]Uint128{
			{Lo: 0x59F2815B16F81798, Hi: 0x029BFCDB2DCE28D9},
			{Lo: 0x55A06295CE870B07, Hi: 0x79BE667EF9DCBBAC},
		},
	}

	GeneratorY = FieldElement{
		N: [2]Uint128{
			{Lo: 0x9C47D08FFB10D4B8, Hi: 0xFD17B448A6855419},
			{Lo: 0x5DA4FBFC0E1108A8, Hi: 0x483ADA7726A3C465},
		},
	}

	Generator = AffinePoint{
		X:        GeneratorX,
		Y:        GeneratorY,
		Infinity: false,
	}
)
