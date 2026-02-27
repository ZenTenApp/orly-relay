//go:build amd64 && !purego

package p256k1

// Assembly function declarations for 4x64 field operations.
// These are implemented in field_4x64_amd64.s using BMI2 instructions (MULX).

// field4x64MulAsm computes r = a * b (mod p) using BMI2 MULX.
// All pointers must be aligned and point to valid [4]uint64 arrays.
//
//go:noescape
func field4x64MulAsm(r, a, b *[4]uint64)

// field4x64SqrAsm computes r = a^2 (mod p) using BMI2 MULX.
// All pointers must be aligned and point to valid [4]uint64 arrays.
//
//go:noescape
func field4x64SqrAsm(r, a *[4]uint64)
