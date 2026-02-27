//go:build amd64

package p256k1

// AMD64-specific scalar operations with optional AVX2/BMI2 acceleration.
// The Scalar type uses 4×uint64 limbs which are memory-compatible with
// the AVX package's 2×Uint128 representation.

// scalarMulAVX2 multiplies two scalars using AVX2 assembly.
// Both input and output use the same memory layout as the pure Go implementation.
//
//go:noescape
func scalarMulAVX2(r, a, b *Scalar)

// scalarMulBMI2 multiplies two scalars using BMI2 MULX instruction.
// This is faster than traditional MUL because MULX doesn't clobber flags,
// allowing better instruction scheduling with carry chains.
//
//go:noescape
func scalarMulBMI2(r, a, b *Scalar)

// scalarAddAVX2 adds two scalars using AVX2 assembly.
//
//go:noescape
func scalarAddAVX2(r, a, b *Scalar)

// scalarSubAVX2 subtracts two scalars using AVX2 assembly.
//
//go:noescape
func scalarSubAVX2(r, a, b *Scalar)
