//go:build !(js && wasm)

package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/minio/sha256-simd"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/hex"
)

const (
	// Database key prefixes for blob storage (metadata and indexes only, blob data stored as files)
	prefixBlobMeta   = "blob:meta:"
	prefixBlobIndex  = "blob:index:"
	prefixBlobReport = "blob:report:"
)

// getBlobDir returns the directory for storing blob files
func (d *D) getBlobDir() string {
	return filepath.Join(d.dataDir, "blossom")
}

// getBlobPath returns the filesystem path for a blob given its hash and extension
func (d *D) getBlobPath(sha256Hex string, ext string) string {
	filename := sha256Hex + ext
	return filepath.Join(d.getBlobDir(), filename)
}

// ensureBlobDir ensures the blob directory exists
func (d *D) ensureBlobDir() error {
	return os.MkdirAll(d.getBlobDir(), 0755)
}

// SaveBlob stores a blob with its metadata
func (d *D) SaveBlob(
	sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string,
) (err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Verify SHA256 matches
	calculatedHash := sha256.Sum256(data)
	if !bytesEqual(calculatedHash[:], sha256Hash) {
		err = errorf.E(
			"SHA256 mismatch: calculated %x, provided %x",
			calculatedHash[:], sha256Hash,
		)
		return
	}

	// If extension not provided, infer from MIME type
	if extension == "" {
		extension = getExtensionFromMimeType(mimeType)
	}

	// Create metadata with extension
	metadata := &BlobMetadata{
		Pubkey:    pubkey,
		MimeType:  mimeType,
		Uploaded:  time.Now().Unix(),
		Size:      int64(len(data)),
		Extension: extension,
	}
	if mimeType == "" {
		metadata.MimeType = "application/octet-stream"
	}

	var metaData []byte
	if metaData, err = json.Marshal(metadata); chk.E(err) {
		return
	}

	// Ensure blob directory exists
	if err = d.ensureBlobDir(); err != nil {
		return errorf.E("failed to create blob directory: %w", err)
	}

	// Get blob file path
	blobPath := d.getBlobPath(sha256Hex, extension)

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
	if err = d.Update(func(txn *badger.Txn) error {
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
func (d *D) GetBlob(sha256Hash []byte) (data []byte, metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata first to get extension
	metaKey := prefixBlobMeta + sha256Hex
	if err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			metadata = &BlobMetadata{}
			if err = json.Unmarshal(val, metadata); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	// Read blob data from file
	blobPath := d.getBlobPath(sha256Hex, metadata.Extension)
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
func (d *D) HasBlob(sha256Hash []byte) (exists bool, err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata to find extension
	metaKey := prefixBlobMeta + sha256Hex
	var metadata *BlobMetadata
	if err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err == badger.ErrKeyNotFound {
			return badger.ErrKeyNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			metadata = &BlobMetadata{}
			if err = json.Unmarshal(val, metadata); err != nil {
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
	blobPath := d.getBlobPath(sha256Hex, metadata.Extension)
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
func (d *D) DeleteBlob(sha256Hash []byte, pubkey []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Get metadata to find extension
	metaKey := prefixBlobMeta + sha256Hex
	var metadata *BlobMetadata
	if err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err == badger.ErrKeyNotFound {
			return badger.ErrKeyNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			metadata = &BlobMetadata{}
			if err = json.Unmarshal(val, metadata); err != nil {
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

	blobPath := d.getBlobPath(sha256Hex, metadata.Extension)
	indexKey := prefixBlobIndex + hex.Enc(pubkey) + ":" + sha256Hex

	if err = d.Update(func(txn *badger.Txn) error {
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
func (d *D) ListBlobs(
	pubkey []byte, since, until int64,
) (descriptors []*BlobDescriptor, err error) {
	pubkeyHex := hex.Enc(pubkey)
	prefix := prefixBlobIndex + pubkeyHex + ":"

	descriptors = make([]*BlobDescriptor, 0)

	if err = d.View(func(txn *badger.Txn) error {
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
				metadata = &BlobMetadata{}
				if err = json.Unmarshal(val, metadata); err != nil {
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
			blobPath := d.getBlobPath(sha256Hex, metadata.Extension)
			if _, errGet := os.Stat(blobPath); errGet != nil {
				continue
			}

			// Create descriptor (URL will be set by handler)
			mimeType := metadata.MimeType
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			descriptor := &BlobDescriptor{
				URL:      "", // URL will be set by handler
				SHA256:   sha256Hex,
				Size:     metadata.Size,
				Type:     mimeType,
				Uploaded: metadata.Uploaded,
			}

			descriptors = append(descriptors, descriptor)
		}

		return nil
	}); chk.E(err) {
		return
	}

	return
}

// GetBlobMetadata retrieves only metadata for a blob
func (d *D) GetBlobMetadata(sha256Hash []byte) (metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)
	metaKey := prefixBlobMeta + sha256Hex

	if err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			metadata = &BlobMetadata{}
			if err = json.Unmarshal(val, metadata); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	return
}

// GetTotalBlobStorageUsed calculates total storage used by a pubkey in MB
func (d *D) GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error) {
	pubkeyHex := hex.Enc(pubkey)
	prefix := prefixBlobIndex + pubkeyHex + ":"

	totalBytes := int64(0)

	if err = d.View(func(txn *badger.Txn) error {
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
				metadata = &BlobMetadata{}
				if err = json.Unmarshal(val, metadata); err != nil {
					return err
				}
				return nil
			}); err != nil {
				continue
			}

			// Verify blob file exists
			blobPath := d.getBlobPath(sha256Hex, metadata.Extension)
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

// SaveBlobReport stores a report for a blob (BUD-09)
func (d *D) SaveBlobReport(sha256Hash []byte, reportData []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)
	reportKey := prefixBlobReport + sha256Hex

	// Get existing reports
	var existingReports [][]byte
	if err = d.View(func(txn *badger.Txn) error {
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

	if err = d.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(reportKey), reportsData)
	}); chk.E(err) {
		return
	}

	log.D.F("saved report for blob %s", sha256Hex)
	return
}

// ListAllBlobUserStats returns storage statistics for all users who have uploaded blobs
func (d *D) ListAllBlobUserStats() (stats []*UserBlobStats, err error) {
	statsMap := make(map[string]*UserBlobStats)

	if err = d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefixBlobIndex)
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			// Key format: blob:index:<pubkey-hex>:<sha256-hex>
			remainder := key[len(prefixBlobIndex):]
			parts := strings.SplitN(remainder, ":", 2)
			if len(parts) != 2 {
				continue
			}
			pubkeyHex := parts[0]
			sha256Hex := parts[1]

			// Get or create stats entry
			stat, ok := statsMap[pubkeyHex]
			if !ok {
				stat = &UserBlobStats{PubkeyHex: pubkeyHex}
				statsMap[pubkeyHex] = stat
			}
			stat.BlobCount++

			// Get blob size from metadata
			metaKey := prefixBlobMeta + sha256Hex
			metaItem, errGet := txn.Get([]byte(metaKey))
			if errGet != nil {
				continue
			}
			metaItem.Value(func(val []byte) error {
				metadata := &BlobMetadata{}
				if errDeser := json.Unmarshal(val, metadata); errDeser == nil {
					stat.TotalSizeBytes += metadata.Size
				}
				return nil
			})
		}
		return nil
	}); chk.E(err) {
		return
	}

	// Convert map to slice
	stats = make([]*UserBlobStats, 0, len(statsMap))
	for _, stat := range statsMap {
		stats = append(stats, stat)
	}

	// Sort by total size descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalSizeBytes > stats[j].TotalSizeBytes
	})

	return
}

// getExtensionFromMimeType returns a file extension for a MIME type
func getExtensionFromMimeType(mimeType string) string {
	// Common MIME type to extension mapping
	mimeToExt := map[string]string{
		"image/png":       ".png",
		"image/jpeg":      ".jpg",
		"image/gif":       ".gif",
		"image/webp":      ".webp",
		"image/svg+xml":   ".svg",
		"image/bmp":       ".bmp",
		"image/tiff":      ".tiff",
		"video/mp4":       ".mp4",
		"video/webm":      ".webm",
		"video/ogg":       ".ogv",
		"video/quicktime": ".mov",
		"audio/mpeg":      ".mp3",
		"audio/ogg":       ".ogg",
		"audio/wav":       ".wav",
		"audio/webm":      ".weba",
		"audio/flac":      ".flac",
		"application/pdf": ".pdf",
		"application/zip": ".zip",
		"text/plain":      ".txt",
		"text/html":       ".html",
		"text/css":        ".css",
		"text/javascript": ".js",
		"application/json": ".json",
	}

	if ext, ok := mimeToExt[mimeType]; ok {
		return ext
	}
	return "" // No extension for unknown types
}
