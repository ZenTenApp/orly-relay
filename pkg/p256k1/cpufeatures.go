//go:build amd64

package p256k1

import (
	"sync"
	"sync/atomic"

	"github.com/klauspost/cpuid/v2"
)

// CPU feature flags
var (
	// hasAVX2CPU indicates whether the CPU supports AVX2 instructions.
	// This is detected at startup and never changes.
	hasAVX2CPU bool

	// hasBMI2CPU indicates whether the CPU supports BMI2 instructions.
	// BMI2 provides MULX, ADCX, ADOX for efficient carry-chain arithmetic.
	hasBMI2CPU bool

	// hasADXCPU indicates whether the CPU supports ADX instructions.
	// ADX provides ADCX/ADOX for parallel carry chains.
	hasADXCPU bool

	// avx2Disabled allows runtime disabling of AVX2 for testing/debugging.
	// Uses atomic operations for thread-safety without locks on the fast path.
	avx2Disabled atomic.Bool

	// bmi2Disabled allows runtime disabling of BMI2 for testing/debugging.
	bmi2Disabled atomic.Bool

	// initOnce ensures CPU detection runs exactly once
	initOnce sync.Once
)

func init() {
	initOnce.Do(detectCPUFeatures)
}

// detectCPUFeatures detects CPU capabilities at startup
func detectCPUFeatures() {
	hasAVX2CPU = cpuid.CPU.Has(cpuid.AVX2)
	hasBMI2CPU = cpuid.CPU.Has(cpuid.BMI2)
	hasADXCPU = cpuid.CPU.Has(cpuid.ADX)
}

// HasAVX2 returns true if AVX2 is available and enabled.
// This is the function that should be called in hot paths to decide
// whether to use AVX2-optimized code paths.
func HasAVX2() bool {
	return hasAVX2CPU && !avx2Disabled.Load()
}

// HasAVX2CPU returns true if the CPU supports AVX2, regardless of whether
// it's been disabled via SetAVX2Enabled.
func HasAVX2CPU() bool {
	return hasAVX2CPU
}

// SetAVX2Enabled enables or disables the use of AVX2 instructions.
// This is useful for benchmarking to compare AVX2 vs non-AVX2 performance,
// or for debugging. Pass true to enable AVX2 (default), false to disable.
// This function is thread-safe.
func SetAVX2Enabled(enabled bool) {
	avx2Disabled.Store(!enabled)
}

// IsAVX2Enabled returns whether AVX2 is currently enabled.
// Returns true if AVX2 is both available on the CPU and not disabled.
func IsAVX2Enabled() bool {
	return HasAVX2()
}

// HasBMI2 returns true if BMI2 is available and enabled.
// BMI2 provides MULX for efficient multiplication without affecting flags,
// enabling parallel carry chains with ADCX/ADOX.
func HasBMI2() bool {
	return hasBMI2CPU && hasADXCPU && !bmi2Disabled.Load()
}

// HasBMI2CPU returns true if the CPU supports BMI2, regardless of whether
// it's been disabled via SetBMI2Enabled.
func HasBMI2CPU() bool {
	return hasBMI2CPU
}

// HasADXCPU returns true if the CPU supports ADX (ADCX/ADOX instructions).
func HasADXCPU() bool {
	return hasADXCPU
}

// SetBMI2Enabled enables or disables the use of BMI2 instructions.
// This is useful for benchmarking to compare BMI2 vs non-BMI2 performance.
// Pass true to enable BMI2 (default), false to disable.
// This function is thread-safe.
func SetBMI2Enabled(enabled bool) {
	bmi2Disabled.Store(!enabled)
}

// IsBMI2Enabled returns whether BMI2 is currently enabled.
// Returns true if BMI2+ADX are both available on the CPU and not disabled.
func IsBMI2Enabled() bool {
	return HasBMI2()
}
