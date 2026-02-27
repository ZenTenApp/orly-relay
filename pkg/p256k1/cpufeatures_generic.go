//go:build !amd64

package p256k1

// Generic stubs for non-AMD64 architectures.
// AVX2 and BMI2 are not available on non-x86 platforms.

// HasAVX2 always returns false on non-AMD64 platforms.
func HasAVX2() bool {
	return false
}

// HasAVX2CPU always returns false on non-AMD64 platforms.
func HasAVX2CPU() bool {
	return false
}

// SetAVX2Enabled is a no-op on non-AMD64 platforms.
func SetAVX2Enabled(enabled bool) {
	// No-op: AVX2 is not available
}

// IsAVX2Enabled always returns false on non-AMD64 platforms.
func IsAVX2Enabled() bool {
	return false
}

// HasBMI2 always returns false on non-AMD64 platforms.
func HasBMI2() bool {
	return false
}

// HasBMI2CPU always returns false on non-AMD64 platforms.
func HasBMI2CPU() bool {
	return false
}

// HasADXCPU always returns false on non-AMD64 platforms.
func HasADXCPU() bool {
	return false
}

// SetBMI2Enabled is a no-op on non-AMD64 platforms.
func SetBMI2Enabled(enabled bool) {
	// No-op: BMI2 is not available
}

// IsBMI2Enabled always returns false on non-AMD64 platforms.
func IsBMI2Enabled() bool {
	return false
}
