// Package tag provides an implementation of a nostr tag list, an array of
// strings with a usually single letter first "key" field, including methods to
// compare, marshal/unmarshal and access elements with their proper semantics.
package tag

import (
	"bytes"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/text"
	"git.smesh.lol/orly/pkg/nostr/utils"
	"git.smesh.lol/orly/pkg/lol/errorf"
)

// The tag position meanings, so they are clear when reading.
const (
	Key = iota
	Value
	Relay
)

// Binary encoding constants for optimized storage of hex-encoded identifiers
const (
	// BinaryEncodedLen is the length of a binary-encoded 32-byte hash with null terminator
	BinaryEncodedLen = 33
	// HexEncodedLen is the length of a hex-encoded 32-byte hash
	HexEncodedLen = 64
	// HashLen is the raw length of a hash (pubkey/event ID)
	HashLen = 32
)

// Tags that use binary encoding optimization for their value field
var binaryOptimizedTags = map[string]bool{
	"e": true, // event references
	"p": true, // pubkey references
}

type T struct {
	T [][]byte
}

func New() *T { return &T{} }

func NewFromBytesSlice(t ...[]byte) (tt *T) {
	tt = &T{T: t}
	return
}

func NewFromAny(t ...any) (tt *T) {
	tt = &T{}
	for _, v := range t {
		switch vv := v.(type) {
		case []byte:
			tt.T = append(tt.T, vv)
		case string:
			tt.T = append(tt.T, []byte(vv))
		default:
			panic("invalid type for tag fields, must be []byte or string")
		}
	}
	return
}

func NewWithCap(c int) *T {
	return &T{T: make([][]byte, 0, c)}
}

func (t *T) Free() {
	t.T = nil
}

func (t *T) Len() int {
	if t == nil {
		return 0
	}
	return len(t.T)
}

func (t *T) Less(i, j int) bool {
	return bytes.Compare(t.T[i], t.T[j]) < 0
}

func (t *T) Swap(i, j int) { t.T[i], t.T[j] = t.T[j], t.T[i] }

// Contains returns true if the provided element is found in the tag slice.
func (t *T) Contains(s []byte) (b bool) {
	for i := range t.T {
		if utils.FastEqual(t.T[i], s) {
			return true
		}
	}
	return false
}

// Marshal encodes a tag.T as standard minified JSON array of strings.
// Binary-encoded values (e/p tags) are automatically converted back to hex.
func (t *T) Marshal(dst []byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	// Estimate: [ + (quoted field + comma) * n + ]
	// Each field might be escaped, so estimate len(field) * 1.5 + 2 quotes + comma
	if b == nil && len(t.T) > 0 {
		estimatedSize := 2 // brackets
		for i, s := range t.T {
			fieldLen := len(s)
			// Binary-encoded fields become hex (33 -> 64 chars)
			if i == Value && isBinaryEncoded(s) {
				fieldLen = HexEncodedLen
			}
			estimatedSize += fieldLen*3/2 + 4 // escaped field + quotes + comma
		}
		b = make([]byte, 0, estimatedSize)
	}
	b = append(b, '[')
	for i, s := range t.T {
		// Convert binary-encoded value fields back to hex for JSON
		if i == Value && isBinaryEncoded(s) {
			hexVal := hex.EncAppend(nil, s[:HashLen])
			b = text.AppendQuote(b, hexVal, text.NostrEscape)
		} else {
			b = text.AppendQuote(b, s, text.NostrEscape)
		}
		if i < len(t.T)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// MarshalJSON encodes a tag.T as standard minified JSON array of strings.
//
// Warning: this will mangle the output if the tag fields contain <, > or &
// characters. do not use json.Marshal in the hopes of rendering tags verbatim
// in an event as you will have a bad time. Use the json.Marshal function in the
// pkg/encoders/json package instead, this has a fork of the json library that
// disables html escaping for json.Marshal.
func (t *T) MarshalJSON() (b []byte, err error) {
	b = t.Marshal(nil)
	return
}

// Unmarshal decodes a standard minified JSON array of strings to a tags.T.
// For "e" and "p" tags with 64-character hex values, it converts them to
// 33-byte binary format (32 bytes hash + null terminator) for efficiency.
func (t *T) Unmarshal(b []byte) (r []byte, err error) {
	var inQuotes, openedBracket bool
	var quoteStart int
	// Pre-allocate slice with estimated capacity to reduce reallocations
	// Estimate based on typical tag sizes (can grow if needed)
	t.T = make([][]byte, 0, 4)
	for i := 0; i < len(b); i++ {
		if !openedBracket && b[i] == '[' {
			openedBracket = true
		} else if !inQuotes {
			if b[i] == '"' {
				inQuotes, quoteStart = true, i+1
			} else if b[i] == ']' {
				return b[i+1:], err
			}
		} else if b[i] == '\\' && i < len(b)-1 {
			i++
		} else if b[i] == '"' {
			inQuotes = false
			// Copy the quoted substring before unescaping so we don't mutate the
			// original JSON buffer in-place (which would corrupt subsequent parsing).
			copyBuf := make([]byte, i-quoteStart)
			copy(copyBuf, b[quoteStart:i])
			unescaped := text.NostrUnescape(copyBuf)

			// Optimize e/p tag values by converting hex to binary
			fieldIdx := len(t.T)
			if fieldIdx == Value && len(t.T) > 0 && shouldOptimize(
				t.T[Key], unescaped,
			) {
				// Decode hex to binary format: 32 bytes + null terminator
				binVal := make([]byte, BinaryEncodedLen)
				if _, err = hex.DecBytes(
					binVal[:HashLen], unescaped,
				); err == nil {
					binVal[HashLen] = 0 // null terminator
					t.T = append(t.T, binVal)
				} else {
					// If decode fails, store as-is
					t.T = append(t.T, unescaped)
				}
			} else {
				t.T = append(t.T, unescaped)
			}
		}
	}
	if !openedBracket || inQuotes {
		return nil, errorf.E("tag: failed to parse tag")
	}
	return
}

func (t *T) UnmarshalJSON(b []byte) (err error) {
	_, err = t.Unmarshal(b)
	return
}

func (t *T) Key() (key []byte) {
	if len(t.T) > Key {
		return t.T[Key]
	}
	return
}

func (t *T) Value() (key []byte) {
	if t == nil {
		return
	}
	if len(t.T) > Value {
		return t.T[Value]
	}
	return
}

func (t *T) Relay() (key []byte) {
	if len(t.T) > Relay {
		return t.T[Relay]
	}
	return
}

// ToSliceOfStrings returns the tag's bytes slices as a slice of strings. This
// method provides a convenient way to access the tag's contents in string format.
//
// # Return Values
//
// - s ([]string): A slice containing all tag elements converted to strings.
//
// # Expected Behaviour
//
// Returns an empty slice if the tag is empty, otherwise returns a new slice with
// each byte slice element converted to a string.
func (t *T) ToSliceOfStrings() (s []string) {
	if len(t.T) == 0 {
		return
	}
	// Pre-allocate slice with exact capacity to reduce reallocations
	s = make([]string, 0, len(t.T))
	for _, v := range t.T {
		s = append(s, string(v))
	}
	return
}

// isBinaryEncoded checks if a value field is stored in optimized binary format
// (32-byte hash + null terminator = 33 bytes total)
func isBinaryEncoded(val []byte) bool {
	return len(val) == BinaryEncodedLen && val[HashLen] == 0
}

// shouldOptimize checks if a tag should use binary encoding optimization
func shouldOptimize(key []byte, val []byte) bool {
	if len(key) != 1 {
		return false
	}
	keyStr := string(key)
	if !binaryOptimizedTags[keyStr] {
		return false
	}
	// Only optimize if it's a valid 64-character hex string
	return len(val) == HexEncodedLen && isValidHex(val)
}

// isValidHex checks if all bytes are valid hex characters
func isValidHex(b []byte) bool {
	for _, c := range b {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ValueHex returns the value field as hex string. If the value is stored in
// binary format, it converts it to hex. Otherwise, it returns the value as-is.
func (t *T) ValueHex() []byte {
	if t == nil || len(t.T) <= Value {
		return nil
	}
	val := t.T[Value]
	if isBinaryEncoded(val) {
		// Convert binary back to hex
		return hex.EncAppend(nil, val[:HashLen])
	}
	return val
}

// ValueBinary returns the raw binary value if it's binary-encoded, or nil otherwise.
// This is useful for database operations that need the raw hash bytes.
func (t *T) ValueBinary() []byte {
	if t == nil || len(t.T) <= Value {
		return nil
	}
	val := t.T[Value]
	if isBinaryEncoded(val) {
		return val[:HashLen]
	}
	return nil
}

// Equals compares two tags for equality, handling both binary and hex encodings.
// This ensures that ["e", "abc..."] and ["e", <binary>] are equal if they
// represent the same hash. This method does NOT allocate memory.
func (t *T) Equals(other *T) bool {
	if t == nil && other == nil {
		return true
	}
	if t == nil || other == nil {
		return false
	}
	if len(t.T) != len(other.T) {
		return false
	}
	for i := range t.T {
		if i == Value && len(t.T) > Value {
			// Special handling for value field to compare binary vs hex without allocating
			tVal := t.T[Value]
			oVal := other.T[Value]

			tIsBinary := isBinaryEncoded(tVal)
			oIsBinary := isBinaryEncoded(oVal)

			// Both binary - compare first 32 bytes directly
			if tIsBinary && oIsBinary {
				if !bytes.Equal(tVal[:HashLen], oVal[:HashLen]) {
					return false
				}
			} else if tIsBinary || oIsBinary {
				// One is binary, one is hex - need to compare carefully
				// Compare the binary one's raw bytes with hex-decoded version of the other
				var binBytes, hexBytes []byte
				if tIsBinary {
					binBytes = tVal[:HashLen]
					hexBytes = oVal
				} else {
					binBytes = oVal[:HashLen]
					hexBytes = tVal
				}

				// Decode hex inline without allocation by comparing byte by byte
				if len(hexBytes) != HexEncodedLen {
					return false
				}
				for j := 0; j < HashLen; j++ {
					// Convert two hex chars to one byte and compare
					hi := hexBytes[j*2]
					lo := hexBytes[j*2+1]

					var hiByte, loByte byte
					if hi >= '0' && hi <= '9' {
						hiByte = hi - '0'
					} else if hi >= 'a' && hi <= 'f' {
						hiByte = hi - 'a' + 10
					} else if hi >= 'A' && hi <= 'F' {
						hiByte = hi - 'A' + 10
					} else {
						return false
					}

					if lo >= '0' && lo <= '9' {
						loByte = lo - '0'
					} else if lo >= 'a' && lo <= 'f' {
						loByte = lo - 'a' + 10
					} else if lo >= 'A' && lo <= 'F' {
						loByte = lo - 'A' + 10
					} else {
						return false
					}

					expectedByte := (hiByte << 4) | loByte
					if binBytes[j] != expectedByte {
						return false
					}
				}
			} else {
				// Both are regular (hex or other) - direct comparison
				if !bytes.Equal(tVal, oVal) {
					return false
				}
			}
		} else {
			if !bytes.Equal(t.T[i], other.T[i]) {
				return false
			}
		}
	}
	return true
}
