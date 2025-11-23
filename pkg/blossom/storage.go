package blossom

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/utils"
)

const (
	// Database key prefixes (metadata and indexes only, blob data stored as files)
	prefixBlobMeta   = "blob:meta:"
	prefixBlobIndex  = "blob:index:"
	prefixBlobReport = "blob:report:"
)

// Storage provides blob storage operations
type Storage struct {
	db      *database.D
	blobDir string // Directory for storing blob files
}

// NewStorage creates a new storage instance
func NewStorage(db *database.D) *Storage {
	// Derive blob directory from database path
	blobDir := filepath.Join(db.Path(), "blossom")

	// Ensure blob directory exists
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		log.E.F("failed to create blob directory %s: %v", blobDir, err)
	}

	return &Storage{
		db:      db,
		blobDir: blobDir,
	}
}

// getBlobPath returns the filesystem path for a blob given its hash and extension
func (s *Storage) getBlobPath(sha256Hex string, ext string) string {
	filename := sha256Hex + ext
	return filepath.Join(s.blobDir, filename)
}

// SaveBlob stores a blob with its metadata
func (s *Storage) SaveBlob(
	sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string,
) (err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Verify SHA256 matches
	calculatedHash := sha256.Sum256(data)
	if !utils.FastEqual(calculatedHash[:], sha256Hash) {
		err = errorf.E(
			"SHA256 mismatch: calculated %x, provided %x",
			calculatedHash[:], sha256Hash,
		)
		return
	}

	// If extension not provided, infer from MIME type
	if extension == "" {
		extension = GetExtensionFromMimeType(mimeType)
	}

	// Create metadata with extension
	metadata := NewBlobMetadata(pubkey, mimeType, int64(len(data)))
	metadata.Extension = extension
	var metaData []byte
	if metaData, err = metadata.Serialize(); chk.E(err) {
		return
	}

	// Get blob file path
	blobPath := s.getBlobPath(sha256Hex, extension)

	// Check if blob file already exists (deduplication)
	if _, err = os.Stat(blobPath); err == nil {
		// File exists, just update metadata and index
		log.D.F("blob file already exists: %s", blobPath)
	} else if !os.IsNotExist(err) {
		return errorf.E("error checking blob file: %w", err)
	} else {
		// Write blob data to file
		if err = os.WriteFile(blobPath, data, 0644); chk.E(err) {
			return errorf.E("failed to write blob file: %w", err)
		}
		log.D.F("wrote blob file: %s (%d bytes)", blobPath, len(data))
	}

	// Store metadata and index in database
	if err = s.db.Update(func(txn *badger.Txn) error {
		// Store metadata
		metaKey := prefixBlobMeta + sha256Hex
		if err := txn.Set([]byte(metaKey), metaData); err != nil {
			return err
		}

		// Index by pubkey
		indexKey := prefixBlobIndex + hex.Enc(pubkey) + ":" + sha256Hex
		if err := txn.Set([]byte(indexKey), []byte{1}); err != nil {
			return err
		}

		return nil
	}); chk.E(err) {
		return
	}

	log.D.F("saved blob %s (%d bytes) for pubkey %s", sha256Hex, len(data), hex.Enc(pubkey))
	return
}

// GetBlob retrieves blob data by SHA256 hash
func (s *Storage) GetBlob(sha256Hash []byte) (data []byte, metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata first to get extension
	metaKey := prefixBlobMeta + sha256Hex
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	// Read blob data from file
	blobPath := s.getBlobPath(sha256Hex, metadata.Extension)
	data, err = os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = badger.ErrKeyNotFound
		}
		return
	}

	return
}

// HasBlob checks if a blob exists
func (s *Storage) HasBlob(sha256Hash []byte) (exists bool, err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata to find extension
	metaKey := prefixBlobMeta + sha256Hex
	var metadata *BlobMetadata
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err == badger.ErrKeyNotFound {
			return badger.ErrKeyNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); err == badger.ErrKeyNotFound {
		exists = false
		return false, nil
	}
	if err != nil {
		return
	}

	// Check if file exists
	blobPath := s.getBlobPath(sha256Hex, metadata.Extension)
	if _, err = os.Stat(blobPath); err == nil {
		exists = true
		return
	}
	if os.IsNotExist(err) {
		exists = false
		err = nil
		return
	}
	return
}

// DeleteBlob deletes a blob and its metadata
func (s *Storage) DeleteBlob(sha256Hash []byte, pubkey []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata to find extension
	metaKey := prefixBlobMeta + sha256Hex
	var metadata *BlobMetadata
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err == badger.ErrKeyNotFound {
			return badger.ErrKeyNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); err == badger.ErrKeyNotFound {
		return errorf.E("blob %s not found", sha256Hex)
	}
	if err != nil {
		return
	}

	blobPath := s.getBlobPath(sha256Hex, metadata.Extension)
	indexKey := prefixBlobIndex + hex.Enc(pubkey) + ":" + sha256Hex

	if err = s.db.Update(func(txn *badger.Txn) error {
		// Delete metadata
		if err := txn.Delete([]byte(metaKey)); err != nil {
			return err
		}

		// Delete index entry
		if err := txn.Delete([]byte(indexKey)); err != nil {
			return err
		}

		return nil
	}); chk.E(err) {
		return
	}

	// Delete blob file
	if err = os.Remove(blobPath); err != nil && !os.IsNotExist(err) {
		log.E.F("failed to delete blob file %s: %v", blobPath, err)
		// Don't fail if file doesn't exist
	}

	log.D.F("deleted blob %s for pubkey %s", sha256Hex, hex.Enc(pubkey))
	return
}

// ListBlobs lists all blobs for a given pubkey
func (s *Storage) ListBlobs(
	pubkey []byte, since, until int64,
) (descriptors []*BlobDescriptor, err error) {
	pubkeyHex := hex.Enc(pubkey)
	prefix := prefixBlobIndex + pubkeyHex + ":"

	descriptors = make([]*BlobDescriptor, 0)

	if err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract SHA256 from key: prefixBlobIndex + pubkeyHex + ":" + sha256Hex
			sha256Hex := string(key[len(prefix):])

			// Get blob metadata
			metaKey := prefixBlobMeta + sha256Hex
			metaItem, err := txn.Get([]byte(metaKey))
			if err != nil {
				continue
			}

			var metadata *BlobMetadata
			if err = metaItem.Value(func(val []byte) error {
				if metadata, err = DeserializeBlobMetadata(val); err != nil {
					return err
				}
				return nil
			}); err != nil {
				continue
			}

			// Filter by time range
			if since > 0 && metadata.Uploaded < since {
				continue
			}
			if until > 0 && metadata.Uploaded > until {
				continue
			}

			// Verify blob file exists
			blobPath := s.getBlobPath(sha256Hex, metadata.Extension)
			if _, errGet := os.Stat(blobPath); errGet != nil {
				continue
			}

			// Create descriptor (URL will be set by handler)
			descriptor := NewBlobDescriptor(
				"", // URL will be set by handler
				sha256Hex,
				metadata.Size,
				metadata.MimeType,
				metadata.Uploaded,
			)

			descriptors = append(descriptors, descriptor)
		}

		return nil
	}); chk.E(err) {
		return
	}

	return
}

// GetTotalStorageUsed calculates total storage used by a pubkey in MB
func (s *Storage) GetTotalStorageUsed(pubkey []byte) (totalMB int64, err error) {
	pubkeyHex := hex.Enc(pubkey)
	prefix := prefixBlobIndex + pubkeyHex + ":"

	totalBytes := int64(0)

	if err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract SHA256 from key: prefixBlobIndex + pubkeyHex + ":" + sha256Hex
			sha256Hex := string(key[len(prefix):])

			// Get blob metadata
			metaKey := prefixBlobMeta + sha256Hex
			metaItem, err := txn.Get([]byte(metaKey))
			if err != nil {
				continue
			}

			var metadata *BlobMetadata
			if err = metaItem.Value(func(val []byte) error {
				if metadata, err = DeserializeBlobMetadata(val); err != nil {
					return err
				}
				return nil
			}); err != nil {
				continue
			}

			// Verify blob file exists
			blobPath := s.getBlobPath(sha256Hex, metadata.Extension)
			if _, errGet := os.Stat(blobPath); errGet != nil {
				continue
			}

			totalBytes += metadata.Size
		}

		return nil
	}); chk.E(err) {
		return
	}

	// Convert bytes to MB (rounding up)
	totalMB = (totalBytes + 1024*1024 - 1) / (1024 * 1024)
	return
}

// SaveReport stores a report for a blob (BUD-09)
func (s *Storage) SaveReport(sha256Hash []byte, reportData []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)
	reportKey := prefixBlobReport + sha256Hex

	// Get existing reports
	var existingReports [][]byte
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(reportKey))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if err = json.Unmarshal(val, &existingReports); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	// Append new report
	existingReports = append(existingReports, reportData)

	// Store updated reports
	var reportsData []byte
	if reportsData, err = json.Marshal(existingReports); chk.E(err) {
		return
	}

	if err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(reportKey), reportsData)
	}); chk.E(err) {
		return
	}

	log.D.F("saved report for blob %s", sha256Hex)
	return
}

// GetBlobMetadata retrieves only metadata for a blob
func (s *Storage) GetBlobMetadata(sha256Hash []byte) (metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)
	metaKey := prefixBlobMeta + sha256Hex

	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	return
}
