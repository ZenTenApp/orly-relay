//go:build !(js && wasm)

package ratelimit

import (
	"errors"
	"runtime"

	"github.com/pbnjay/memory"
)

// MinimumMemoryMB is the minimum memory required to run the relay with rate limiting.
const MinimumMemoryMB = 500

// AutoDetectMemoryFraction is the fraction of available memory to use when auto-detecting.
const AutoDetectMemoryFraction = 0.66

// DefaultMaxMemoryMB is the default maximum memory target when auto-detecting.
// This caps the auto-detected value to ensure optimal performance.
const DefaultMaxMemoryMB = 1500

// ErrInsufficientMemory is returned when there isn't enough memory to run the relay.
var ErrInsufficientMemory = errors.New("insufficient memory: relay requires at least 500MB of available memory")

// ProcessMemoryStats contains memory statistics for the current process.
// On Linux, these are read from /proc/self/status for accurate RSS values.
// On other platforms, these are approximated from Go runtime stats.
type ProcessMemoryStats struct {
	// VmRSS is the resident set size (total physical memory in use) in bytes
	VmRSS uint64
	// RssShmem is the shared memory portion of RSS in bytes
	RssShmem uint64
	// RssAnon is the anonymous (non-shared) memory in bytes
	RssAnon uint64
	// VmHWM is the peak RSS (high water mark) in bytes
	VmHWM uint64
}

// PhysicalMemoryBytes returns the actual physical memory usage (RSS - shared)
func (p ProcessMemoryStats) PhysicalMemoryBytes() uint64 {
	if p.VmRSS > p.RssShmem {
		return p.VmRSS - p.RssShmem
	}
	return p.VmRSS
}

// PhysicalMemoryMB returns the actual physical memory usage in MB
func (p ProcessMemoryStats) PhysicalMemoryMB() uint64 {
	return p.PhysicalMemoryBytes() / (1024 * 1024)
}

// DetectAvailableMemoryMB returns the available system memory in megabytes.
// On Linux, this returns the actual available memory (free + cached).
// On other systems, it returns total memory minus the Go runtime's current usage.
func DetectAvailableMemoryMB() uint64 {
	// Use pbnjay/memory for cross-platform memory detection
	available := memory.FreeMemory()
	if available == 0 {
		// Fallback: use total memory
		available = memory.TotalMemory()
	}
	return available / (1024 * 1024)
}

// DetectTotalMemoryMB returns the total system memory in megabytes.
func DetectTotalMemoryMB() uint64 {
	return memory.TotalMemory() / (1024 * 1024)
}

// CalculateTargetMemoryMB calculates the target memory limit based on configuration.
// If configuredMB is 0, it auto-detects based on available memory (66% of available, capped at 1.5GB).
// If configuredMB is non-zero, it validates that it's achievable.
// Returns an error if there isn't enough memory.
func CalculateTargetMemoryMB(configuredMB int) (int, error) {
	availableMB := int(DetectAvailableMemoryMB())

	// If configured to auto-detect (0), calculate target
	if configuredMB == 0 {
		// First check if we have minimum available memory
		if availableMB < MinimumMemoryMB {
			return 0, ErrInsufficientMemory
		}

		// Calculate 66% of available
		targetMB := int(float64(availableMB) * AutoDetectMemoryFraction)

		// If 66% is less than minimum, use minimum (we've already verified we have enough)
		if targetMB < MinimumMemoryMB {
			targetMB = MinimumMemoryMB
		}

		// Cap at default maximum for optimal performance
		if targetMB > DefaultMaxMemoryMB {
			targetMB = DefaultMaxMemoryMB
		}

		return targetMB, nil
	}

	// If explicitly configured, validate it's achievable
	if configuredMB < MinimumMemoryMB {
		return 0, ErrInsufficientMemory
	}

	// Warn but allow if configured target exceeds available
	// (the PID controller will throttle as needed)
	return configuredMB, nil
}

// GetMemoryStats returns current memory statistics for logging.
type MemoryStats struct {
	TotalMB       uint64
	AvailableMB   uint64
	TargetMB      int
	GoAllocatedMB uint64
	GoSysMB       uint64
}

// GetMemoryStats returns current memory statistics.
func GetMemoryStats(targetMB int) MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		TotalMB:       DetectTotalMemoryMB(),
		AvailableMB:   DetectAvailableMemoryMB(),
		TargetMB:      targetMB,
		GoAllocatedMB: m.Alloc / (1024 * 1024),
		GoSysMB:       m.Sys / (1024 * 1024),
	}
}

// readProcessMemoryStatsFallback returns memory stats using Go runtime.
// This is used on non-Linux platforms or when /proc is unavailable.
// The values are approximations and may not accurately reflect OS-level metrics.
func readProcessMemoryStatsFallback() ProcessMemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Use Sys as an approximation of RSS (includes all memory from OS)
	// HeapAlloc approximates anonymous memory (live heap objects)
	// We cannot determine shared memory from Go runtime, so leave it at 0
	return ProcessMemoryStats{
		VmRSS:    m.Sys,
		RssAnon:  m.HeapAlloc,
		RssShmem: 0, // Cannot determine shared memory from Go runtime
		VmHWM:    0, // Not available from Go runtime
	}
}
