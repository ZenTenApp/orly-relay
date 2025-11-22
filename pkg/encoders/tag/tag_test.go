package tag

import (
	"testing"

	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
	"next.orly.dev/pkg/utils"
)

func TestMarshalUnmarshal(t *testing.T) {
	for _ = range 1000 {
		n := frand.Intn(8)
		tg := New()
		for _ = range n {
			b1 := make([]byte, frand.Intn(8))
			_, _ = frand.Read(b1)
			tg.T = append(tg.T, b1)
		}
		tb := tg.Marshal(nil)
		var tbc []byte
		tbc = append(tbc, tb...)
		tg2 := New()
		if _, err := tg2.Unmarshal(tb); chk.E(err) {
			t.Fatal(err)
		}
		tb2 := tg2.Marshal(nil)
		if !utils.FastEqual(tbc, tb2) {
			t.Fatalf("failed to re-marshal back original")
		}
	}
}

// TestBinaryEncodingOptimization verifies that e/p tags are stored in binary format
func TestBinaryEncodingOptimization(t *testing.T) {
	testCases := []struct {
		name          string
		json          string
		expectBinary  bool
		internalLen   int // expected length of Value() field in internal storage
	}{
		{
			name:         "e tag with hex value",
			json:         `["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`,
			expectBinary: true,
			internalLen:  BinaryEncodedLen, // 33 bytes (32 + null terminator)
		},
		{
			name:         "p tag with hex value",
			json:         `["p","fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"]`,
			expectBinary: true,
			internalLen:  BinaryEncodedLen,
		},
		{
			name:         "e tag with relay",
			json:         `["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","wss://relay.example.com"]`,
			expectBinary: true,
			internalLen:  BinaryEncodedLen,
		},
		{
			name:         "t tag not optimized",
			json:         `["t","bitcoin"]`,
			expectBinary: false,
			internalLen:  7, // "bitcoin" as-is
		},
		{
			name:         "e tag with short value not optimized",
			json:         `["e","short"]`,
			expectBinary: false,
			internalLen:  5,
		},
		{
			name:         "e tag with invalid hex not optimized",
			json:         `["e","zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"]`,
			expectBinary: false,
			internalLen:  64,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tag := New()
			_, err := tag.Unmarshal([]byte(tc.json))
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Check internal storage length
			if tag.Len() < 2 {
				t.Fatal("Tag should have at least 2 fields")
			}

			valueField := tag.T[Value]
			if len(valueField) != tc.internalLen {
				t.Errorf("Expected internal value length %d, got %d", tc.internalLen, len(valueField))
			}

			// Check if binary encoded as expected
			if tc.expectBinary {
				if !isBinaryEncoded(valueField) {
					t.Error("Expected binary encoding, but tag is not binary encoded")
				}
				// Verify null terminator
				if valueField[HashLen] != 0 {
					t.Error("Binary encoded value should have null terminator at position 32")
				}
			} else {
				if isBinaryEncoded(valueField) {
					t.Error("Did not expect binary encoding, but tag is binary encoded")
				}
			}

			// Marshal back to JSON and verify it matches original
			marshaled := tag.Marshal(nil)
			if string(marshaled) != tc.json {
				t.Errorf("Marshaled JSON doesn't match original.\nExpected: %s\nGot: %s", tc.json, string(marshaled))
			}
		})
	}
}

// TestValueHexMethod verifies ValueHex() correctly converts binary to hex
func TestValueHexMethod(t *testing.T) {
	json := `["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`
	tag := New()
	_, err := tag.Unmarshal([]byte(json))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Internal storage should be binary
	if !isBinaryEncoded(tag.T[Value]) {
		t.Fatal("Expected binary encoding")
	}

	// ValueHex should return the original hex string
	hexVal := tag.ValueHex()
	expectedHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if string(hexVal) != expectedHex {
		t.Errorf("ValueHex returned wrong value.\nExpected: %s\nGot: %s", expectedHex, string(hexVal))
	}
}

// TestValueBinaryMethod verifies ValueBinary() returns raw hash bytes
func TestValueBinaryMethod(t *testing.T) {
	json := `["p","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`
	tag := New()
	_, err := tag.Unmarshal([]byte(json))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// ValueBinary should return 32 bytes
	binVal := tag.ValueBinary()
	if binVal == nil {
		t.Fatal("ValueBinary returned nil")
	}
	if len(binVal) != HashLen {
		t.Errorf("Expected %d bytes, got %d", HashLen, len(binVal))
	}

	// It should match the first 32 bytes of the internal storage
	if !utils.FastEqual(binVal, tag.T[Value][:HashLen]) {
		t.Error("ValueBinary doesn't match internal storage")
	}
}

// TestEqualsMethod verifies Equals() handles binary vs hex comparison
func TestEqualsMethod(t *testing.T) {
	// Create tag from JSON (will be binary internally)
	tag1 := New()
	_, err := tag1.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Create identical tag
	tag2 := New()
	_, err = tag2.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !tag1.Equals(tag2) {
		t.Error("Identical tags should be equal")
	}

	// Create tag with different value
	tag3 := New()
	_, err = tag3.Unmarshal([]byte(`["e","fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"]`))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tag1.Equals(tag3) {
		t.Error("Different tags should not be equal")
	}

	// Test with relay field
	tag4 := New()
	_, err = tag4.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","wss://relay.example.com"]`))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tag1.Equals(tag4) {
		t.Error("Tags with different lengths should not be equal")
	}
}

// TestBinaryEncodingSavesSpace verifies the optimization reduces memory usage
func TestBinaryEncodingSavesSpace(t *testing.T) {
	// Tag without optimization (non-hex or non e/p tag)
	tagNonOpt := New()
	_, _ = tagNonOpt.Unmarshal([]byte(`["t","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
	nonOptSize := len(tagNonOpt.T[Value])

	// Tag with optimization
	tagOpt := New()
	_, _ = tagOpt.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
	optSize := len(tagOpt.T[Value])

	// Binary should be smaller (33 vs 64 bytes)
	if optSize >= nonOptSize {
		t.Errorf("Binary encoding should save space. Non-opt: %d bytes, Opt: %d bytes", nonOptSize, optSize)
	}

	expectedSavings := HexEncodedLen - BinaryEncodedLen // 64 - 33 = 31 bytes
	actualSavings := nonOptSize - optSize
	if actualSavings != expectedSavings {
		t.Errorf("Expected to save %d bytes, actually saved %d bytes", expectedSavings, actualSavings)
	}

	t.Logf("Space savings: %d bytes per e/p tag value (%.1f%% reduction)",
		actualSavings, float64(actualSavings)/float64(nonOptSize)*100)
}
