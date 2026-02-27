// Package apputil provides utility functions for file and directory operations.
package apputil

import (
	"os"
	"path/filepath"

	"next.orly.dev/pkg/lol/chk"
)

// EnsureDir checks if a file could be written to a path and creates the
// necessary directories if they don't exist. It ensures that all parent
// directories in the path are created with the appropriate permissions.
//
// # Parameters
//
//   - fileName: The full path to the file for which directories need to be
//     created.
//
// Expected behavior:
//
//   - Extracts the directory path from the fileName.
//
//   - Checks if the directory exists.
//
//   - If the directory doesn't exist, creates it and all parent directories.
func EnsureDir(fileName string) (merr error) {
	dirName := filepath.Dir(fileName)
	if _, err := os.Stat(dirName); chk.E(err) {
		merr = os.MkdirAll(dirName, os.ModePerm)
		if chk.E(merr) {
			return
		}
		return
	}
	return
}

// FileExists reports whether the named file or directory exists.
//
// # Parameters
//
//   - filePath: The full path to the file or directory to check.
//
// Returns:
//
//   - bool: true if the file or directory exists, false otherwise.
//
// Behavior:
//
//   - Uses os.Stat to check if the file or directory exists.
//
//   - Returns true if the file exists and can be accessed.
//
//   - Returns false if the file doesn't exist or cannot be accessed due to
//     permissions.
func FileExists(filePath string) bool {
	_, e := os.Stat(filePath)
	return e == nil
}
