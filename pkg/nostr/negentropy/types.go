// Package negentropy implements NIP-77 negentropy-based set reconciliation.
// It provides efficient synchronization between two sets of Nostr events
// by exchanging fingerprints and identifying missing items.
package negentropy

import (
	"encoding/hex"
	"fmt"
)

// Mode represents the type of range in the negentropy protocol.
type Mode uint8

const (
	// ModeSkip indicates the range should be skipped (already in sync).
	ModeSkip Mode = 0
	// ModeFingerprint indicates the range is represented by a fingerprint.
	ModeFingerprint Mode = 1
	// ModeIdList indicates the range contains an explicit list of IDs.
	ModeIdList Mode = 2
)

// ProtocolVersion is the negentropy protocol version byte.
const ProtocolVersion byte = 0x61 // Version 1

// DefaultFrameSizeLimit is the default maximum message size.
// Set to 4MB to reduce round-trip count for large syncs (206k+ events).
// WebSocket max message size is typically 10MB, so 4MB is well within bounds.
const DefaultFrameSizeLimit = 4 * 1024 * 1024 // 4MB

// DefaultIDSize is the fingerprint size (16 bytes).
const DefaultIDSize = 16

// FullIDSize is the size of a full event ID (32 bytes).
// Used for ID lists in the negentropy protocol.
const FullIDSize = 32

// Item represents a single item in the negentropy set.
// Items are sorted by timestamp first, then by ID.
type Item struct {
	Timestamp int64  // Unix timestamp in seconds
	ID        string // 32-byte event ID as hex string (64 chars)
}

// Compare compares two items for sorting.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (a Item) Compare(b Item) int {
	if a.Timestamp < b.Timestamp {
		return -1
	}
	if a.Timestamp > b.Timestamp {
		return 1
	}
	// Timestamps equal, compare IDs lexicographically
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}

// String returns a string representation of the item.
func (i Item) String() string {
	return fmt.Sprintf("Item{ts=%d, id=%s}", i.Timestamp, truncateID(i.ID))
}

// Bound represents a boundary in the negentropy range.
type Bound struct {
	Item
}

// MinBound returns the minimum possible bound.
func MinBound() Bound {
	return Bound{Item{Timestamp: 0, ID: ""}}
}

// MaxBound returns the maximum possible bound.
func MaxBound() Bound {
	return Bound{Item{Timestamp: MaxTimestamp, ID: ""}}
}

// MaxTimestamp is the maximum timestamp value.
const MaxTimestamp int64 = 1<<62 - 1

// IsMin returns true if this is the minimum bound.
func (b Bound) IsMin() bool {
	return b.Timestamp == 0 && b.ID == ""
}

// IsMax returns true if this is the maximum bound.
func (b Bound) IsMax() bool {
	return b.Timestamp == MaxTimestamp
}

// String returns a string representation of the bound.
func (b Bound) String() string {
	if b.IsMin() {
		return "Bound{MIN}"
	}
	if b.IsMax() {
		return "Bound{MAX}"
	}
	return fmt.Sprintf("Bound{ts=%d, id=%s}", b.Timestamp, truncateID(b.ID))
}

// truncateID truncates an ID for display purposes.
func truncateID(id string) string {
	if len(id) > 16 {
		return id[:8] + "..." + id[len(id)-4:]
	}
	return id
}

// Fingerprint represents a 16-byte fingerprint of a range.
type Fingerprint [DefaultIDSize]byte

// String returns the hex representation of the fingerprint.
func (f Fingerprint) String() string {
	return hex.EncodeToString(f[:])
}

// EmptyFingerprint is a zero fingerprint.
var EmptyFingerprint = Fingerprint{}

// XOR combines two fingerprints using XOR.
func (f Fingerprint) XOR(other Fingerprint) Fingerprint {
	var result Fingerprint
	for i := 0; i < DefaultIDSize; i++ {
		result[i] = f[i] ^ other[i]
	}
	return result
}
