package blossom

import (
	"os"
	"path/filepath"

	"lol.mleku.dev/log"

	"next.orly.dev/pkg/database"
)

// Storage provides blob storage operations using the database interface.
// This is a thin wrapper that delegates to the database's blob methods.
type Storage struct {
	db      database.Database
	blobDir string // Directory for storing blob files (for backward compatibility info)
}

// NewStorage creates a new storage instance using the database interface.
func NewStorage(db database.Database) *Storage {
	// Derive blob directory from database path (for informational purposes)
	blobDir := filepath.Join(db.Path(), "blossom")

	// Ensure blob directory exists (the database implementation handles this,
	// but we keep this for backward compatibility with code that checks the directory)
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		log.E.F("failed to create blob directory %s: %v", blobDir, err)
	}

	return &Storage{
		db:      db,
		blobDir: blobDir,
	}
}

// SaveBlob stores a blob with its metadata.
// Delegates to the database interface's SaveBlob method.
func (s *Storage) SaveBlob(
	sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string,
) error {
	return s.db.SaveBlob(sha256Hash, data, pubkey, mimeType, extension)
}

// GetBlob retrieves blob data by SHA256 hash.
// Returns the data and metadata from the database.
func (s *Storage) GetBlob(sha256Hash []byte) (data []byte, metadata *BlobMetadata, err error) {
	data, dbMeta, err := s.db.GetBlob(sha256Hash)
	if err != nil {
		return nil, nil, err
	}
	// Convert database.BlobMetadata to blossom.BlobMetadata
	metadata = &BlobMetadata{
		Pubkey:    dbMeta.Pubkey,
		MimeType:  dbMeta.MimeType,
		Uploaded:  dbMeta.Uploaded,
		Size:      dbMeta.Size,
		Extension: dbMeta.Extension,
	}
	return data, metadata, nil
}

// HasBlob checks if a blob exists.
func (s *Storage) HasBlob(sha256Hash []byte) (exists bool, err error) {
	return s.db.HasBlob(sha256Hash)
}

// DeleteBlob deletes a blob and its metadata.
func (s *Storage) DeleteBlob(sha256Hash []byte, pubkey []byte) error {
	return s.db.DeleteBlob(sha256Hash, pubkey)
}

// ListBlobs lists all blobs for a given pubkey.
// Returns blob descriptors with time filtering.
func (s *Storage) ListBlobs(pubkey []byte, since, until int64) ([]*BlobDescriptor, error) {
	dbDescriptors, err := s.db.ListBlobs(pubkey, since, until)
	if err != nil {
		return nil, err
	}
	// Convert database.BlobDescriptor to blossom.BlobDescriptor
	descriptors := make([]*BlobDescriptor, 0, len(dbDescriptors))
	for _, d := range dbDescriptors {
		descriptors = append(descriptors, &BlobDescriptor{
			URL:      d.URL,
			SHA256:   d.SHA256,
			Size:     d.Size,
			Type:     d.Type,
			Uploaded: d.Uploaded,
			NIP94:    d.NIP94,
		})
	}
	return descriptors, nil
}

// GetBlobPath returns the filesystem path for a blob given its hash and extension.
// This is used by handlers that need to serve files directly via http.ServeFile.
func (s *Storage) GetBlobPath(sha256Hex string, extension string) string {
	filename := sha256Hex + extension
	return filepath.Join(s.blobDir, filename)
}

// GetBlobMetadata retrieves only metadata for a blob.
func (s *Storage) GetBlobMetadata(sha256Hash []byte) (*BlobMetadata, error) {
	dbMeta, err := s.db.GetBlobMetadata(sha256Hash)
	if err != nil {
		return nil, err
	}
	// Convert database.BlobMetadata to blossom.BlobMetadata
	return &BlobMetadata{
		Pubkey:    dbMeta.Pubkey,
		MimeType:  dbMeta.MimeType,
		Uploaded:  dbMeta.Uploaded,
		Size:      dbMeta.Size,
		Extension: dbMeta.Extension,
	}, nil
}

// GetTotalStorageUsed calculates total storage used by a pubkey in MB.
func (s *Storage) GetTotalStorageUsed(pubkey []byte) (totalMB int64, err error) {
	return s.db.GetTotalBlobStorageUsed(pubkey)
}

// SaveReport stores a report for a blob (BUD-09).
func (s *Storage) SaveReport(sha256Hash []byte, reportData []byte) error {
	return s.db.SaveBlobReport(sha256Hash, reportData)
}

// ListAllUserStats returns storage statistics for all users who have uploaded blobs.
func (s *Storage) ListAllUserStats() ([]*UserBlobStats, error) {
	dbStats, err := s.db.ListAllBlobUserStats()
	if err != nil {
		return nil, err
	}
	// Convert database.UserBlobStats to blossom.UserBlobStats
	stats := make([]*UserBlobStats, 0, len(dbStats))
	for _, s := range dbStats {
		stats = append(stats, &UserBlobStats{
			PubkeyHex:      s.PubkeyHex,
			BlobCount:      s.BlobCount,
			TotalSizeBytes: s.TotalSizeBytes,
		})
	}
	return stats, nil
}

// UserBlobStats represents storage statistics for a single user.
// This mirrors database.UserBlobStats for API compatibility.
type UserBlobStats struct {
	PubkeyHex      string `json:"pubkey"`
	BlobCount      int64  `json:"blob_count"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
}
