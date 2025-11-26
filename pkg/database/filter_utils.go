// Package database provides filter utilities for normalizing tag values.
//
// The nostr library optimizes e/p tag values by storing them in binary format
// (32 bytes + null terminator) rather than hex strings (64 chars). However,
// filter tags from client queries come as hex strings and don't go through
// the same binary encoding during unmarshalling.
//
// This file provides utilities to normalize filter tags to match the binary
// encoding used in stored events, ensuring consistent index lookups and
// tag comparisons.
package database

import (
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// Tag binary encoding constants (matching the nostr library)
const (
	// BinaryEncodedLen is the length of a binary-encoded 32-byte hash with null terminator
	BinaryEncodedLen = 33
	// HexEncodedLen is the length of a hex-encoded 32-byte hash
	HexEncodedLen = 64
	// HashLen is the raw length of a hash (pubkey/event ID)
	HashLen = 32
)

// binaryOptimizedTags defines which tag keys use binary encoding optimization
var binaryOptimizedTags = map[byte]bool{
	'e': true, // event references
	'p': true, // pubkey references
}

// IsBinaryOptimizedTag returns true if the given tag key uses binary encoding
func IsBinaryOptimizedTag(key byte) bool {
	return binaryOptimizedTags[key]
}

// IsBinaryEncoded checks if a value field is stored in optimized binary format
func IsBinaryEncoded(val []byte) bool {
	return len(val) == BinaryEncodedLen && val[HashLen] == 0
}

// IsValidHexValue checks if a byte slice is a valid 64-character hex string
func IsValidHexValue(b []byte) bool {
	if len(b) != HexEncodedLen {
		return false
	}
	return IsHexString(b)
}

// HexToBinary converts a 64-character hex string to 33-byte binary format
// Returns nil if the input is not a valid hex string
func HexToBinary(hexVal []byte) []byte {
	if !IsValidHexValue(hexVal) {
		return nil
	}
	binVal := make([]byte, BinaryEncodedLen)
	if _, err := hex.DecBytes(binVal[:HashLen], hexVal); err != nil {
		return nil
	}
	binVal[HashLen] = 0 // null terminator
	return binVal
}

// BinaryToHex converts a 33-byte binary value to 64-character hex string
// Returns nil if the input is not in binary format
func BinaryToHex(binVal []byte) []byte {
	if !IsBinaryEncoded(binVal) {
		return nil
	}
	return hex.EncAppend(nil, binVal[:HashLen])
}

// NormalizeTagValue normalizes a tag value for the given key.
// For e/p tags, hex values are converted to binary format.
// Other tags are returned unchanged.
func NormalizeTagValue(key byte, val []byte) []byte {
	if !IsBinaryOptimizedTag(key) {
		return val
	}
	// If already binary, return as-is
	if IsBinaryEncoded(val) {
		return val
	}
	// If valid hex, convert to binary
	if binVal := HexToBinary(val); binVal != nil {
		return binVal
	}
	// Otherwise return as-is
	return val
}

// NormalizeTagToHex returns the hex representation of a tag value.
// For binary-encoded values, converts to hex. For hex values, returns as-is.
func NormalizeTagToHex(val []byte) []byte {
	if IsBinaryEncoded(val) {
		return BinaryToHex(val)
	}
	return val
}

// NormalizeFilterTag creates a new tag with binary-encoded values for e/p tags.
// The original tag is not modified.
func NormalizeFilterTag(t *tag.T) *tag.T {
	if t == nil || t.Len() < 2 {
		return t
	}

	keyBytes := t.Key()
	var key byte

	// Handle both "e" and "#e" style keys
	if len(keyBytes) == 1 {
		key = keyBytes[0]
	} else if len(keyBytes) == 2 && keyBytes[0] == '#' {
		key = keyBytes[1]
	} else {
		return t // Not a single-letter tag
	}

	if !IsBinaryOptimizedTag(key) {
		return t // Not an optimized tag type
	}

	// Create new tag with normalized values
	normalized := tag.NewWithCap(t.Len())
	normalized.T = append(normalized.T, t.T[0]) // Keep key as-is

	// Normalize each value
	for _, val := range t.T[1:] {
		normalizedVal := NormalizeTagValue(key, val)
		normalized.T = append(normalized.T, normalizedVal)
	}

	return normalized
}

// NormalizeFilterTags normalizes all tags in a tag.S, converting e/p hex values to binary.
// Returns a new tag.S with normalized tags.
func NormalizeFilterTags(tags *tag.S) *tag.S {
	if tags == nil || tags.Len() == 0 {
		return tags
	}

	normalized := tag.NewSWithCap(tags.Len())
	for _, t := range *tags {
		normalizedTag := NormalizeFilterTag(t)
		normalized.Append(normalizedTag)
	}
	return normalized
}

// NormalizeFilter normalizes a filter's tags for consistent database queries.
// This should be called before using a filter for database lookups.
// The original filter is not modified; a copy with normalized tags is returned.
func NormalizeFilter(f *filter.F) *filter.F {
	if f == nil {
		return nil
	}

	// Create a shallow copy of the filter
	normalized := &filter.F{
		Ids:     f.Ids,
		Kinds:   f.Kinds,
		Authors: f.Authors,
		Since:   f.Since,
		Until:   f.Until,
		Search:  f.Search,
		Limit:   f.Limit,
	}

	// Normalize the tags
	normalized.Tags = NormalizeFilterTags(f.Tags)

	return normalized
}

// TagValuesMatch compares two tag values, handling both binary and hex encodings.
// This is useful for post-query tag matching where event values may be binary
// and filter values may be hex (or vice versa).
func TagValuesMatch(key byte, eventVal, filterVal []byte) bool {
	// If both are the same, they match
	if len(eventVal) == len(filterVal) {
		for i := range eventVal {
			if eventVal[i] != filterVal[i] {
				goto different
			}
		}
		return true
	}
different:

	// For non-optimized tags, require exact match
	if !IsBinaryOptimizedTag(key) {
		return false
	}

	// Normalize both to hex and compare
	eventHex := NormalizeTagToHex(eventVal)
	filterHex := NormalizeTagToHex(filterVal)

	if len(eventHex) != len(filterHex) {
		return false
	}
	for i := range eventHex {
		if eventHex[i] != filterHex[i] {
			return false
		}
	}
	return true
}

// TagValuesMatchUsingTagMethods compares an event tag's value with a filter value
// using the tag.T methods. This leverages the nostr library's ValueHex() method
// for proper binary/hex conversion.
func TagValuesMatchUsingTagMethods(eventTag *tag.T, filterVal []byte) bool {
	if eventTag == nil {
		return false
	}

	keyBytes := eventTag.Key()
	if len(keyBytes) != 1 {
		// Not a single-letter tag, use direct comparison
		return bytesEqual(eventTag.Value(), filterVal)
	}

	key := keyBytes[0]
	if !IsBinaryOptimizedTag(key) {
		// Not an optimized tag, use direct comparison
		return bytesEqual(eventTag.Value(), filterVal)
	}

	// For e/p tags, use ValueHex() for proper conversion
	eventHex := eventTag.ValueHex()
	filterHex := NormalizeTagToHex(filterVal)

	return bytesEqual(eventHex, filterHex)
}

// bytesEqual is a fast equality check that avoids allocation
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
