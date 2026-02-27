// Package neo4j provides hex utilities for normalizing pubkeys and event IDs.
//
// The nostr library applies binary optimization to e/p tags, storing 64-character
// hex strings as 33-byte binary (32 bytes + null terminator). This file provides
// utilities to ensure all pubkeys and event IDs stored in Neo4j are in consistent
// lowercase hex format.
package neo4j

import (
	"strings"

	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

// Tag binary encoding constants (matching the nostr library)
const (
	// BinaryEncodedLen is the length of a binary-encoded 32-byte hash with null terminator
	BinaryEncodedLen = 33
	// HexEncodedLen is the length of a hex-encoded 32-byte hash (pubkey or event ID)
	HexEncodedLen = 64
	// HashLen is the raw length of a hash (pubkey/event ID)
	HashLen = 32
)

// IsBinaryEncoded checks if a value is stored in the nostr library's binary-optimized format
func IsBinaryEncoded(val []byte) bool {
	return len(val) == BinaryEncodedLen && val[HashLen] == 0
}

// NormalizePubkeyHex ensures a pubkey/event ID is in lowercase hex format.
// It handles:
// - Binary-encoded values (33 bytes with null terminator) -> converts to lowercase hex
// - Raw binary values (32 bytes) -> converts to lowercase hex
// - Uppercase hex strings -> converts to lowercase
// - Already lowercase hex -> returns as-is
//
// This should be used for all pubkeys and event IDs before storing in Neo4j
// to prevent duplicate nodes due to case differences.
func NormalizePubkeyHex(val []byte) string {
	// Handle binary-encoded values from the nostr library (33 bytes with null terminator)
	if IsBinaryEncoded(val) {
		// Convert binary to lowercase hex
		return hex.Enc(val[:HashLen])
	}

	// Handle raw binary values (32 bytes) - common when passing ev.ID or ev.Pubkey directly
	if len(val) == HashLen {
		// Convert binary to lowercase hex
		return hex.Enc(val)
	}

	// Handle hex strings (may be uppercase from external sources)
	if len(val) == HexEncodedLen {
		return strings.ToLower(string(val))
	}

	// For other lengths (possibly prefixes), lowercase the hex
	return strings.ToLower(string(val))
}

// ExtractPTagValue extracts a pubkey from a p-tag, handling binary encoding.
// Returns lowercase hex string suitable for Neo4j storage.
// Returns empty string if the tag doesn't have a valid value.
func ExtractPTagValue(t *tag.T) string {
	if t == nil || len(t.T) < 2 {
		return ""
	}

	// Use ValueHex() which properly handles both binary and hex formats
	hexVal := t.ValueHex()
	if len(hexVal) == 0 {
		return ""
	}

	// Ensure lowercase (ValueHex returns the library's encoding which is lowercase,
	// but we normalize anyway for safety with external data)
	return strings.ToLower(string(hexVal))
}

// ExtractETagValue extracts an event ID from an e-tag, handling binary encoding.
// Returns lowercase hex string suitable for Neo4j storage.
// Returns empty string if the tag doesn't have a valid value.
func ExtractETagValue(t *tag.T) string {
	if t == nil || len(t.T) < 2 {
		return ""
	}

	// Use ValueHex() which properly handles both binary and hex formats
	hexVal := t.ValueHex()
	if len(hexVal) == 0 {
		return ""
	}

	// Ensure lowercase
	return strings.ToLower(string(hexVal))
}

// IsValidHexPubkey checks if a string is a valid 64-character hex pubkey
func IsValidHexPubkey(s string) bool {
	if len(s) != HexEncodedLen {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
