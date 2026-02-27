package negentropy

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"iter"
	"math/bits"
	"sort"
)

// Storage is the interface for negentropy item storage.
// Implementations must maintain items sorted by (timestamp, id).
type Storage interface {
	// Size returns the number of items in storage.
	Size() int

	// Range returns an iterator over items in the index range [begin, end).
	Range(begin, end int) iter.Seq2[int, Item]

	// FindLowerBound returns the index of the first item >= bound in range [begin, end).
	// If no such item exists, returns end.
	FindLowerBound(begin, end int, bound Bound) int

	// GetBound returns the bound at the given index.
	// If idx == Size(), returns MaxBound.
	GetBound(idx int) Bound

	// Fingerprint computes the fingerprint of items in range [begin, end).
	Fingerprint(begin, end int) Fingerprint
}

// Vector is an in-memory Storage implementation backed by a sorted slice.
type Vector struct {
	items  []Item
	sealed bool
}

// NewVector creates a new empty Vector.
func NewVector() *Vector {
	return &Vector{
		items: make([]Item, 0),
	}
}

// Insert adds an item to the vector.
// Items can only be inserted before Seal() is called.
func (v *Vector) Insert(timestamp int64, id string) {
	if v.sealed {
		panic("cannot insert into sealed vector")
	}
	v.items = append(v.items, Item{Timestamp: timestamp, ID: id})
}

// Seal sorts the items and marks the vector as read-only.
// Must be called before using the vector for reconciliation.
func (v *Vector) Seal() {
	if v.sealed {
		return
	}
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Compare(v.items[j]) < 0
	})
	v.sealed = true
}

// Size returns the number of items.
func (v *Vector) Size() int {
	return len(v.items)
}

// Range returns an iterator over items in range [begin, end).
func (v *Vector) Range(begin, end int) iter.Seq2[int, Item] {
	return func(yield func(int, Item) bool) {
		for i := begin; i < end && i < len(v.items); i++ {
			if !yield(i, v.items[i]) {
				return
			}
		}
	}
}

// FindLowerBound returns the index of the first item >= bound in range [begin, end).
func (v *Vector) FindLowerBound(begin, end int, bound Bound) int {
	if begin >= end || begin >= len(v.items) {
		return end
	}
	if end > len(v.items) {
		end = len(v.items)
	}

	// Binary search for the lower bound
	idx := sort.Search(end-begin, func(i int) bool {
		item := v.items[begin+i]
		cmp := item.Compare(bound.Item)
		return cmp >= 0
	})

	return begin + idx
}

// GetBound returns the bound at the given index.
func (v *Vector) GetBound(idx int) Bound {
	if idx >= len(v.items) {
		return MaxBound()
	}
	return Bound{v.items[idx]}
}

// Fingerprint computes the fingerprint of items in range [begin, end).
// Per the negentropy protocol v1 spec:
// 1. Sum all 32-byte IDs as 256-bit integers (modular addition mod 2^256)
// 2. Append the element count as a varint
// 3. SHA-256 hash the result
// 4. Take the first 16 bytes
func (v *Vector) Fingerprint(begin, end int) Fingerprint {
	if end > len(v.items) {
		end = len(v.items)
	}

	// Accumulate sum of all IDs as a 256-bit integer (4 little-endian uint64s)
	var accum [4]uint64

	for i := begin; i < end; i++ {
		idBytes, err := hex.DecodeString(v.items[i].ID)
		if err != nil || len(idBytes) != FullIDSize {
			continue
		}

		// Add this ID to the accumulator with carry propagation
		var carry uint64
		for j := 0; j < 4; j++ {
			val := binary.LittleEndian.Uint64(idBytes[j*8 : j*8+8])
			accum[j], carry = bits.Add64(accum[j], val, carry)
		}
	}

	// Convert accumulator back to bytes (little-endian)
	var accumBytes [FullIDSize]byte
	for j := 0; j < 4; j++ {
		binary.LittleEndian.PutUint64(accumBytes[j*8:j*8+8], accum[j])
	}

	// Append element count as varint, then SHA-256 hash
	count := end - begin
	if count < 0 {
		count = 0
	}
	countBytes := EncodeVarInt(uint64(count))

	hasher := sha256.New()
	hasher.Write(accumBytes[:])
	hasher.Write(countBytes)
	hash := hasher.Sum(nil)

	var fp Fingerprint
	copy(fp[:], hash[:DefaultIDSize])
	return fp
}

// FingerprintWithHash is an alias for Fingerprint (which now uses the correct
// spec-compliant computation: modular addition + count + SHA-256).
// Deprecated: use Fingerprint() directly.
func (v *Vector) FingerprintWithHash(begin, end int) Fingerprint {
	return v.Fingerprint(begin, end)
}

// GetItems returns a copy of all items (for testing).
func (v *Vector) GetItems() []Item {
	result := make([]Item, len(v.items))
	copy(result, v.items)
	return result
}

// GetItem returns the item at the given index.
func (v *Vector) GetItem(idx int) Item {
	if idx >= len(v.items) {
		return Item{}
	}
	return v.items[idx]
}
