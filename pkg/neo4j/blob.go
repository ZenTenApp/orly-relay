package neo4j

import (
	"errors"

	"next.orly.dev/pkg/database"
)

// Blob storage methods - Neo4j does not support blob storage
// These are stub implementations that return errors

var errBlobNotSupported = errors.New("blob storage not supported in Neo4j backend")

// SaveBlob stores a blob with its metadata (not supported in Neo4j)
func (n *N) SaveBlob(sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string) error {
	return errBlobNotSupported
}

// GetBlob retrieves blob data by SHA256 hash (not supported in Neo4j)
func (n *N) GetBlob(sha256Hash []byte) (data []byte, metadata *database.BlobMetadata, err error) {
	return nil, nil, errBlobNotSupported
}

// HasBlob checks if a blob exists (not supported in Neo4j)
func (n *N) HasBlob(sha256Hash []byte) (exists bool, err error) {
	return false, errBlobNotSupported
}

// DeleteBlob deletes a blob and its metadata (not supported in Neo4j)
func (n *N) DeleteBlob(sha256Hash []byte, pubkey []byte) error {
	return errBlobNotSupported
}

// ListBlobs lists all blobs for a given pubkey (not supported in Neo4j)
func (n *N) ListBlobs(pubkey []byte, since, until int64) ([]*database.BlobDescriptor, error) {
	return nil, errBlobNotSupported
}

// GetBlobMetadata retrieves only metadata for a blob (not supported in Neo4j)
func (n *N) GetBlobMetadata(sha256Hash []byte) (*database.BlobMetadata, error) {
	return nil, errBlobNotSupported
}

// GetTotalBlobStorageUsed calculates total storage used by a pubkey in MB (not supported in Neo4j)
func (n *N) GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error) {
	return 0, errBlobNotSupported
}

// SaveBlobReport stores a report for a blob (not supported in Neo4j)
func (n *N) SaveBlobReport(sha256Hash []byte, reportData []byte) error {
	return errBlobNotSupported
}

// ListAllBlobUserStats returns storage statistics for all users (not supported in Neo4j)
func (n *N) ListAllBlobUserStats() ([]*database.UserBlobStats, error) {
	return nil, errBlobNotSupported
}

// ReconcileBlobMetadata is not supported in Neo4j backend
func (n *N) ReconcileBlobMetadata() (int, error) {
	return 0, errBlobNotSupported
}

// ListAllBlobs returns all blobs (not supported in Neo4j)
func (n *N) ListAllBlobs() ([]*database.BlobDescriptor, error) {
	return nil, errBlobNotSupported
}

// GetThumbnail retrieves a cached thumbnail (not supported in Neo4j)
func (n *N) GetThumbnail(key string) ([]byte, error) {
	return nil, errBlobNotSupported
}

// SaveThumbnail caches a thumbnail (not supported in Neo4j)
func (n *N) SaveThumbnail(key string, data []byte) error {
	return errBlobNotSupported
}
