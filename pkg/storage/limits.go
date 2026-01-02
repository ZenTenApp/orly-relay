//go:build !windows

// Package storage provides storage management functionality including filesystem
// space detection, access tracking for events, and garbage collection based on
// access patterns.
package storage

import (
	"syscall"
)

// FilesystemStats holds information about filesystem space usage.
type FilesystemStats struct {
	Total     uint64 // Total bytes on filesystem
	Available uint64 // Available bytes (for unprivileged users)
	Used      uint64 // Used bytes
}

// GetFilesystemStats returns filesystem space information for the given path.
// The path should be a directory within the filesystem to check.
func GetFilesystemStats(path string) (stats FilesystemStats, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return
	}
	stats.Total = stat.Blocks * uint64(stat.Bsize)
	stats.Available = stat.Bavail * uint64(stat.Bsize)
	stats.Used = stats.Total - stats.Available
	return
}

// CalculateMaxStorage calculates the maximum storage limit for the relay.
// If configuredMax > 0, it returns that value directly.
// Otherwise, it returns 80% of the available filesystem space.
func CalculateMaxStorage(dataDir string, configuredMax int64) (int64, error) {
	if configuredMax > 0 {
		return configuredMax, nil
	}

	stats, err := GetFilesystemStats(dataDir)
	if err != nil {
		return 0, err
	}

	// Return 80% of available space
	maxBytes := int64(float64(stats.Available) * 0.8)

	// Also ensure we don't exceed 80% of total filesystem
	maxTotal := int64(float64(stats.Total) * 0.8)
	if maxBytes > maxTotal {
		maxBytes = maxTotal
	}

	return maxBytes, nil
}

// GetCurrentStorageUsage calculates the current storage usage of the data directory.
// This is an approximation based on filesystem stats for the given path.
func GetCurrentStorageUsage(dataDir string) (int64, error) {
	stats, err := GetFilesystemStats(dataDir)
	if err != nil {
		return 0, err
	}
	return int64(stats.Used), err
}
