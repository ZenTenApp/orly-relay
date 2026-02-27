//go:build amd64

package avx

// AMD64-specific field operations with AVX2 assembly.

// FieldAddAVX2 adds two field elements using AVX2.
//
//go:noescape
func FieldAddAVX2(r, a, b *FieldElement)

// FieldSubAVX2 subtracts two field elements using AVX2.
//
//go:noescape
func FieldSubAVX2(r, a, b *FieldElement)

// FieldMulAVX2 multiplies two field elements using AVX2.
//
//go:noescape
func FieldMulAVX2(r, a, b *FieldElement)

// FieldSqrAVX2 squares a field element using AVX2.
//
//go:noescape
func FieldSqrAVX2(r, a *FieldElement)
