//go:build js && wasm

package wasmdb

import (
	"errors"

	"next.orly.dev/pkg/database"
)

// Blob storage methods - WasmDB does not support blob storage
// These are stub implementations that return errors.
// Blob storage is not supported in the browser environment due to
// IndexedDB size limitations and the complexity of binary data handling.

var errBlobNotSupported = errors.New("blob storage not supported in WasmDB backend")

// SaveBlob stores a blob with its metadata (not supported in WasmDB)
func (w *W) SaveBlob(sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string) error {
	return errBlobNotSupported
}

// GetBlob retrieves blob data by SHA256 hash (not supported in WasmDB)
func (w *W) GetBlob(sha256Hash []byte) (data []byte, metadata *database.BlobMetadata, err error) {
	return nil, nil, errBlobNotSupported
}

// HasBlob checks if a blob exists (not supported in WasmDB)
func (w *W) HasBlob(sha256Hash []byte) (exists bool, err error) {
	return false, errBlobNotSupported
}

// DeleteBlob deletes a blob and its metadata (not supported in WasmDB)
func (w *W) DeleteBlob(sha256Hash []byte, pubkey []byte) error {
	return errBlobNotSupported
}

// ListBlobs lists all blobs for a given pubkey (not supported in WasmDB)
func (w *W) ListBlobs(pubkey []byte, since, until int64) ([]*database.BlobDescriptor, error) {
	return nil, errBlobNotSupported
}

// GetBlobMetadata retrieves only metadata for a blob (not supported in WasmDB)
func (w *W) GetBlobMetadata(sha256Hash []byte) (*database.BlobMetadata, error) {
	return nil, errBlobNotSupported
}

// GetTotalBlobStorageUsed calculates total storage used by a pubkey in MB (not supported in WasmDB)
func (w *W) GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error) {
	return 0, errBlobNotSupported
}

// SaveBlobReport stores a report for a blob (not supported in WasmDB)
func (w *W) SaveBlobReport(sha256Hash []byte, reportData []byte) error {
	return errBlobNotSupported
}

// ListAllBlobUserStats returns storage statistics for all users (not supported in WasmDB)
func (w *W) ListAllBlobUserStats() ([]*database.UserBlobStats, error) {
	return nil, errBlobNotSupported
}
