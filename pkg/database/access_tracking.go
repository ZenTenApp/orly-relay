//go:build !(js && wasm)

package database

import (
	"encoding/binary"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const (
	// accessTrackingPrefix is the key prefix for access tracking records.
	// Key format: acc:{8-byte serial} -> {8-byte lastAccessTime}{4-byte accessCount}
	accessTrackingPrefix = "acc:"
)

// RecordEventAccess updates access tracking for an event.
// This increments the access count and updates the last access time.
// The connectionID is currently not used for deduplication in the database layer,
// but is passed for potential future use. Deduplication is handled in the
// higher-level AccessTracker which maintains an in-memory cache.
func (d *D) RecordEventAccess(serial uint64, connectionID string) error {
	key := d.accessKey(serial)

	return d.Update(func(txn *badger.Txn) error {
		var lastAccess int64
		var accessCount uint32

		// Try to get existing record
		item, err := txn.Get(key)
		if err == nil {
			err = item.Value(func(val []byte) error {
				if len(val) >= 12 {
					lastAccess = int64(binary.BigEndian.Uint64(val[0:8]))
					accessCount = binary.BigEndian.Uint32(val[8:12])
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		// Update values
		_ = lastAccess // unused in simple increment mode
		lastAccess = time.Now().Unix()
		accessCount++

		// Write back
		val := make([]byte, 12)
		binary.BigEndian.PutUint64(val[0:8], uint64(lastAccess))
		binary.BigEndian.PutUint32(val[8:12], accessCount)

		return txn.Set(key, val)
	})
}

// GetEventAccessInfo returns access information for an event.
// Returns (0, 0, nil) if the event has never been accessed.
func (d *D) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	key := d.accessKey(serial)

	err = d.View(func(txn *badger.Txn) error {
		item, gerr := txn.Get(key)
		if gerr != nil {
			if gerr == badger.ErrKeyNotFound {
				// Not found is not an error - just return zeros
				return nil
			}
			return gerr
		}

		return item.Value(func(val []byte) error {
			if len(val) >= 12 {
				lastAccess = int64(binary.BigEndian.Uint64(val[0:8]))
				accessCount = binary.BigEndian.Uint32(val[8:12])
			}
			return nil
		})
	})

	return
}

// accessEntry holds access metadata for sorting
type accessEntry struct {
	serial     uint64
	lastAccess int64
	count      uint32
}

// GetLeastAccessedEvents returns event serials sorted by coldness.
// Events with older last access times and lower access counts are returned first.
// limit: maximum number of events to return
// minAgeSec: minimum age in seconds since last access (events accessed more recently are excluded)
func (d *D) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	cutoffTime := time.Now().Unix() - minAgeSec

	var entries []accessEntry

	err = d.View(func(txn *badger.Txn) error {
		prefix := []byte(accessTrackingPrefix)
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract serial from key (after prefix)
			if len(key) <= len(prefix) {
				continue
			}
			serial := binary.BigEndian.Uint64(key[len(prefix):])

			var lastAccess int64
			var accessCount uint32

			err := item.Value(func(val []byte) error {
				if len(val) >= 12 {
					lastAccess = int64(binary.BigEndian.Uint64(val[0:8]))
					accessCount = binary.BigEndian.Uint32(val[8:12])
				}
				return nil
			})
			if err != nil {
				continue
			}

			// Only include events older than cutoff
			if lastAccess < cutoffTime {
				entries = append(entries, accessEntry{serial, lastAccess, accessCount})
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by coldness score (older + fewer accesses = colder = lower score)
	// Score = lastAccess + (accessCount * 3600)
	// Lower score = colder = evict first
	sort.Slice(entries, func(i, j int) bool {
		scoreI := entries[i].lastAccess + int64(entries[i].count)*3600
		scoreJ := entries[j].lastAccess + int64(entries[j].count)*3600
		return scoreI < scoreJ
	})

	// Return up to limit
	for i := 0; i < len(entries) && i < limit; i++ {
		serials = append(serials, entries[i].serial)
	}

	return serials, nil
}

// accessKey generates the database key for an access tracking record.
func (d *D) accessKey(serial uint64) []byte {
	key := make([]byte, len(accessTrackingPrefix)+8)
	copy(key, accessTrackingPrefix)
	binary.BigEndian.PutUint64(key[len(accessTrackingPrefix):], serial)
	return key
}

// DeleteAccessRecord removes the access tracking record for an event.
// This should be called when an event is deleted.
func (d *D) DeleteAccessRecord(serial uint64) error {
	key := d.accessKey(serial)

	return d.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}
