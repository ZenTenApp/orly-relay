//go:build !amd64 || purego

package p256k1

// hasField4x64 returns false on non-amd64 platforms.
func hasField4x64() bool {
	return false
}

// field4x64Mul is a stub for non-amd64 platforms.
func field4x64Mul(r, a, b *FieldElement) {
	panic("field4x64Mul not available on this platform")
}

// field4x64Sqr is a stub for non-amd64 platforms.
func field4x64Sqr(r, a *FieldElement) {
	panic("field4x64Sqr not available on this platform")
}
