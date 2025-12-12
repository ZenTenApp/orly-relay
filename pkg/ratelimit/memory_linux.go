//go:build linux && !(js && wasm)

package ratelimit

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ReadProcessMemoryStats reads memory statistics from /proc/self/status.
// This provides accurate RSS (Resident Set Size) information on Linux,
// including the breakdown between shared and anonymous memory.
func ReadProcessMemoryStats() ProcessMemoryStats {
	stats := ProcessMemoryStats{}

	file, err := os.Open("/proc/self/status")
	if err != nil {
		// Fallback to runtime stats if /proc is not available
		return readProcessMemoryStatsFallback()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		valueStr := fields[1]

		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			continue
		}

		// Values in /proc/self/status are in kB
		valueBytes := value * 1024

		switch key {
		case "VmRSS":
			stats.VmRSS = valueBytes
		case "RssShmem":
			stats.RssShmem = valueBytes
		case "RssAnon":
			stats.RssAnon = valueBytes
		case "VmHWM":
			stats.VmHWM = valueBytes
		}
	}

	// If we didn't get VmRSS, fall back to runtime stats
	if stats.VmRSS == 0 {
		return readProcessMemoryStatsFallback()
	}

	return stats
}
