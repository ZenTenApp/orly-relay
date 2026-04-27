package neo4j

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// TestIsBinaryEncoded tests the IsBinaryEncoded function
func TestIsBinaryEncoded(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "Valid binary encoded (33 bytes with null terminator)",
			input:    append(make([]byte, 32), 0),
			expected: true,
		},
		{
			name:     "Invalid - 32 bytes without terminator",
			input:    make([]byte, 32),
			expected: false,
		},
		{
			name:     "Invalid - 33 bytes without null terminator",
			input:    append(make([]byte, 32), 1),
			expected: false,
		},
		{
			name:     "Invalid - 64 bytes (hex string)",
			input:    []byte("0000000000000000000000000000000000000000000000000000000000000001"),
			expected: false,
		},
		{
			name:     "Invalid - empty",
			input:    []byte{},
			expected: false,
		},
		{
			name:     "Invalid - too short",
			input:    []byte{0, 1, 2, 3},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBinaryEncoded(tt.input)
			if result != tt.expected {
				t.Errorf("IsBinaryEncoded(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizePubkeyHex tests the NormalizePubkeyHex function
func TestNormalizePubkeyHex(t *testing.T) {
	// Create a 32-byte test value
	testBytes := make([]byte, 32)
	testBytes[31] = 0x01 // Set last byte to 1

	// Create binary-encoded version (33 bytes with null terminator)
	binaryEncoded := append(testBytes, 0)

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Binary encoded to hex",
			input:    binaryEncoded,
			expected: "0000000000000000000000000000000000000000000000000000000000000001",
		},
		{
			name:     "Raw 32-byte binary to hex",
			input:    testBytes,
			expected: "0000000000000000000000000000000000000000000000000000000000000001",
		},
		{
			name:     "Lowercase hex passthrough",
			input:    []byte("0000000000000000000000000000000000000000000000000000000000000001"),
			expected: "0000000000000000000000000000000000000000000000000000000000000001",
		},
		{
			name:     "Uppercase hex to lowercase",
			input:    []byte("ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"),
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "Mixed case hex to lowercase",
			input:    []byte("AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789"),
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "Prefix hex (shorter than 64)",
			input:    []byte("ABCD"),
			expected: "abcd",
		},
		{
			name:     "Empty input",
			input:    []byte{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePubkeyHex(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePubkeyHex(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractPTagValue tests the ExtractPTagValue function
func TestExtractPTagValue(t *testing.T) {
	// Create a valid pubkey hex string
	validHex := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

	tests := []struct {
		name     string
		tag      *tag.T
		expected string
	}{
		{
			name:     "Nil tag",
			tag:      nil,
			expected: "",
		},
		{
			name:     "Empty tag",
			tag:      &tag.T{T: [][]byte{}},
			expected: "",
		},
		{
			name:     "Tag with only key",
			tag:      &tag.T{T: [][]byte{[]byte("p")}},
			expected: "",
		},
		{
			name: "Valid p-tag with hex value",
			tag: &tag.T{T: [][]byte{
				[]byte("p"),
				[]byte(validHex),
			}},
			expected: validHex,
		},
		{
			name: "P-tag with uppercase hex",
			tag: &tag.T{T: [][]byte{
				[]byte("p"),
				[]byte("ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"),
			}},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPTagValue(tt.tag)
			if result != tt.expected {
				t.Errorf("ExtractPTagValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractETagValue tests the ExtractETagValue function
func TestExtractETagValue(t *testing.T) {
	// Create a valid event ID hex string
	validHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	tests := []struct {
		name     string
		tag      *tag.T
		expected string
	}{
		{
			name:     "Nil tag",
			tag:      nil,
			expected: "",
		},
		{
			name:     "Empty tag",
			tag:      &tag.T{T: [][]byte{}},
			expected: "",
		},
		{
			name:     "Tag with only key",
			tag:      &tag.T{T: [][]byte{[]byte("e")}},
			expected: "",
		},
		{
			name: "Valid e-tag with hex value",
			tag: &tag.T{T: [][]byte{
				[]byte("e"),
				[]byte(validHex),
			}},
			expected: validHex,
		},
		{
			name: "E-tag with uppercase hex",
			tag: &tag.T{T: [][]byte{
				[]byte("e"),
				[]byte("1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF"),
			}},
			expected: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractETagValue(tt.tag)
			if result != tt.expected {
				t.Errorf("ExtractETagValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestIsValidHexPubkey tests the IsValidHexPubkey function
func TestIsValidHexPubkey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid lowercase hex",
			input:    "0000000000000000000000000000000000000000000000000000000000000001",
			expected: true,
		},
		{
			name:     "Valid uppercase hex",
			input:    "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789",
			expected: true,
		},
		{
			name:     "Valid mixed case hex",
			input:    "AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789AbCdEf0123456789",
			expected: true,
		},
		{
			name:     "Too short",
			input:    "0000000000000000000000000000000000000000000000000000000000000",
			expected: false,
		},
		{
			name:     "Too long",
			input:    "00000000000000000000000000000000000000000000000000000000000000001",
			expected: false,
		},
		{
			name:     "Contains non-hex character",
			input:    "000000000000000000000000000000000000000000000000000000000000000g",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Contains space",
			input:    "0000000000000000000000000000000000000000000000000000000000000 01",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidHexPubkey(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidHexPubkey(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
