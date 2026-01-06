//go:build !(js && wasm)

package bbolt

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
)

const bloomFilterKey = "edge_bloom_filter"

// EdgeBloomFilter provides fast negative lookups for edge existence checks.
// Uses a bloom filter to avoid disk seeks when checking if an edge exists.
type EdgeBloomFilter struct {
	mu     sync.RWMutex
	filter *bloom.BloomFilter

	// Track if filter has been modified since last persist
	dirty bool
}

// NewEdgeBloomFilter creates or loads the edge bloom filter.
// sizeMB is the approximate size in megabytes.
// With 1% false positive rate, 16MB can hold ~10 million edges.
func NewEdgeBloomFilter(sizeMB int, db *bolt.DB) (*EdgeBloomFilter, error) {
	ebf := &EdgeBloomFilter{}

	// Try to load from database
	var loaded bool
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte(bloomFilterKey))
		if data == nil {
			return nil
		}

		// Deserialize bloom filter
		reader := bytes.NewReader(data)
		filter := &bloom.BloomFilter{}
		if _, err := filter.ReadFrom(reader); err != nil {
			return err
		}

		ebf.filter = filter
		loaded = true
		return nil
	})
	if chk.E(err) {
		return nil, err
	}

	if !loaded {
		// Create new filter
		// Calculate parameters: m bits, k hash functions
		// For 1% false positive rate: m/n ≈ 9.6, k ≈ 7
		bitsPerMB := 8 * 1024 * 1024
		totalBits := uint(sizeMB * bitsPerMB)
		// Estimate capacity based on 10 bits per element for 1% FPR
		estimatedCapacity := uint(totalBits / 10)

		ebf.filter = bloom.NewWithEstimates(estimatedCapacity, 0.01)
	}

	return ebf, nil
}

// Add adds an edge to the bloom filter.
// An edge is represented by source and destination serials plus edge type.
func (ebf *EdgeBloomFilter) Add(srcSerial, dstSerial uint64, edgeType byte) {
	ebf.mu.Lock()
	defer ebf.mu.Unlock()

	key := ebf.makeKey(srcSerial, dstSerial, edgeType)
	ebf.filter.Add(key)
	ebf.dirty = true
}

// AddBatch adds multiple edges to the bloom filter.
func (ebf *EdgeBloomFilter) AddBatch(edges []EdgeKey) {
	ebf.mu.Lock()
	defer ebf.mu.Unlock()

	for _, edge := range edges {
		key := ebf.makeKey(edge.SrcSerial, edge.DstSerial, edge.EdgeType)
		ebf.filter.Add(key)
	}
	ebf.dirty = true
}

// MayExist checks if an edge might exist.
// Returns false if definitely doesn't exist (no disk access needed).
// Returns true if might exist (need to check disk to confirm).
func (ebf *EdgeBloomFilter) MayExist(srcSerial, dstSerial uint64, edgeType byte) bool {
	ebf.mu.RLock()
	defer ebf.mu.RUnlock()

	key := ebf.makeKey(srcSerial, dstSerial, edgeType)
	return ebf.filter.Test(key)
}

// Persist saves the bloom filter to the database.
func (ebf *EdgeBloomFilter) Persist(db *bolt.DB) error {
	ebf.mu.Lock()
	if !ebf.dirty {
		ebf.mu.Unlock()
		return nil
	}

	// Serialize while holding lock
	var buf bytes.Buffer
	if _, err := ebf.filter.WriteTo(&buf); err != nil {
		ebf.mu.Unlock()
		return err
	}
	data := buf.Bytes()
	ebf.dirty = false
	ebf.mu.Unlock()

	// Write to database
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}
		return bucket.Put([]byte(bloomFilterKey), data)
	})
}

// Reset clears the bloom filter.
func (ebf *EdgeBloomFilter) Reset() {
	ebf.mu.Lock()
	defer ebf.mu.Unlock()

	ebf.filter.ClearAll()
	ebf.dirty = true
}

// makeKey creates a unique key for an edge.
func (ebf *EdgeBloomFilter) makeKey(srcSerial, dstSerial uint64, edgeType byte) []byte {
	key := make([]byte, 17) // 8 + 8 + 1
	binary.BigEndian.PutUint64(key[0:8], srcSerial)
	binary.BigEndian.PutUint64(key[8:16], dstSerial)
	key[16] = edgeType
	return key
}

// Stats returns bloom filter statistics.
func (ebf *EdgeBloomFilter) Stats() BloomStats {
	ebf.mu.RLock()
	defer ebf.mu.RUnlock()

	approxCount := uint64(ebf.filter.ApproximatedSize())
	cap := ebf.filter.Cap()

	return BloomStats{
		ApproxCount: approxCount,
		Cap:         cap,
	}
}

// BloomStats contains bloom filter statistics.
type BloomStats struct {
	ApproxCount uint64 // Approximate number of elements
	Cap         uint   // Capacity in bits
}

// EdgeKey represents an edge for batch operations.
type EdgeKey struct {
	SrcSerial uint64
	DstSerial uint64
	EdgeType  byte
}

// Edge type constants
const (
	EdgeTypeAuthor    byte = 0 // Event author relationship
	EdgeTypePTag      byte = 1 // P-tag reference (event mentions pubkey)
	EdgeTypeETag      byte = 2 // E-tag reference (event references event)
	EdgeTypeFollows   byte = 3 // Kind 3 follows relationship
	EdgeTypeReaction  byte = 4 // Kind 7 reaction
	EdgeTypeRepost    byte = 5 // Kind 6 repost
	EdgeTypeReply     byte = 6 // Reply (kind 1 with e-tag)
)
