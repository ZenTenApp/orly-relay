//go:build amd64

package avx

// AMD64-specific scalar operations with AVX2 assembly.

// ScalarAddAVX2 adds two scalars using AVX2.
// This loads both scalars into YMM registers and performs parallel addition.
//
//go:noescape
func ScalarAddAVX2(r, a, b *Scalar)

// ScalarSubAVX2 subtracts two scalars using AVX2.
//
//go:noescape
func ScalarSubAVX2(r, a, b *Scalar)

// ScalarMulAVX2 multiplies two scalars using AVX2.
// Computes 512-bit product and reduces mod n.
//
//go:noescape
func ScalarMulAVX2(r, a, b *Scalar)

// hasAVX2 returns true if the CPU supports AVX2.
//
//go:noescape
func hasAVX2() bool
