//go:build windows

package storage

import (
	"errors"
)

// FilesystemStats holds information about filesystem space usage.
type FilesystemStats struct {
	Total     uint64 // Total bytes on filesystem
	Available uint64 // Available bytes (for unprivileged users)
	Used      uint64 // Used bytes
}

// GetFilesystemStats returns filesystem space information for the given path.
// Windows implementation is not yet supported.
func GetFilesystemStats(path string) (stats FilesystemStats, err error) {
	// TODO: Implement using syscall.GetDiskFreeSpaceEx
	err = errors.New("filesystem stats not implemented on Windows")
	return
}

// CalculateMaxStorage calculates the maximum storage limit for the relay.
// If configuredMax > 0, it returns that value directly.
// Windows auto-detection is not yet supported.
func CalculateMaxStorage(dataDir string, configuredMax int64) (int64, error) {
	if configuredMax > 0 {
		return configuredMax, nil
	}
	return 0, errors.New("auto-detect storage limit not implemented on Windows; set ORLY_MAX_STORAGE_BYTES manually")
}

// GetCurrentStorageUsage calculates the current storage usage of the data directory.
// Windows implementation is not yet supported.
func GetCurrentStorageUsage(dataDir string) (int64, error) {
	return 0, errors.New("storage usage detection not implemented on Windows")
}
