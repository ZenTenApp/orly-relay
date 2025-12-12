//go:build !linux && !(js && wasm)

package ratelimit

// ReadProcessMemoryStats returns memory statistics using Go runtime stats.
// On non-Linux platforms, we cannot read /proc/self/status, so we approximate
// using the Go runtime's memory statistics.
//
// Note: This is less accurate than the Linux implementation because:
// - runtime.MemStats.Sys includes memory reserved but not necessarily resident
// - We cannot distinguish shared vs anonymous memory
// - The values may not match what the OS reports for the process
func ReadProcessMemoryStats() ProcessMemoryStats {
	return readProcessMemoryStatsFallback()
}
