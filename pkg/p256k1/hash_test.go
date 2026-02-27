package p256k1

import (
	"testing"
)

func TestSHA256(t *testing.T) {
	// Test basic SHA-256 functionality
	h := NewSHA256()
	testData := []byte("For this sample, this 63-byte string will be used as input data")
	h.Write(testData)
	
	var result [32]byte
	h.Finalize(result[:])
	
	// Expected result from C selftest
	expected := [32]byte{
		0xf0, 0x8a, 0x78, 0xcb, 0xba, 0xee, 0x08, 0x2b, 0x05, 0x2a, 0xe0, 0x70, 0x8f, 0x32, 0xfa, 0x1e,
		0x50, 0xc5, 0xc4, 0x21, 0xaa, 0x77, 0x2b, 0xa5, 0xdb, 0xb4, 0x06, 0xa2, 0xea, 0x6b, 0xe3, 0x42,
	}
	
	for i := 0; i < 32; i++ {
		if result[i] != expected[i] {
			t.Errorf("SHA-256 mismatch at byte %d: got 0x%02x, expected 0x%02x", i, result[i], expected[i])
		}
	}
	
	h.Clear()
}

func TestHMACSHA256(t *testing.T) {
	// Test HMAC-SHA256 with known test vectors
	key := []byte("key")
	message := []byte("The quick brown fox jumps over the lazy dog")
	
	h := NewHMACSHA256(key)
	h.Write(message)
	
	var result [32]byte
	h.Finalize(result[:])
	
	// Basic test - just verify it produces output
	allZero := true
	for i := 0; i < 32; i++ {
		if result[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("HMAC-SHA256 produced all zeros")
	}
	
	h.Clear()
}

func TestRFC6979(t *testing.T) {
	// Test RFC6979 nonce generation
	key := []byte("test key for RFC6979")
	rng := NewRFC6979HMACSHA256(key)
	
	var nonce1 [32]byte
	rng.Generate(nonce1[:])
	
	// Generate more bytes
	var nonce2 [32]byte
	rng.Generate(nonce2[:])
	
	// Nonces should be different
	allSame := true
	for i := 0; i < 32; i++ {
		if nonce1[i] != nonce2[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("RFC6979 produced identical nonces")
	}
	
	rng.Finalize()
	rng.Clear()
}

func TestTaggedHash(t *testing.T) {
	// Test tagged hash function
	tag := []byte("BIP0340/challenge")
	data := []byte("test data")
	
	result := TaggedHash(tag, data)
	
	// Verify it produces output
	allZero := true
	for i := 0; i < 32; i++ {
		if result[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("TaggedHash produced all zeros")
	}
}

func TestHashToScalar(t *testing.T) {
	hash := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hash[i] = byte(i)
	}
	
	scalar, err := HashToScalar(hash)
	if err != nil {
		t.Fatalf("HashToScalar failed: %v", err)
	}
	if scalar == nil {
		t.Fatal("HashToScalar returned nil")
	}
}

func TestHashToField(t *testing.T) {
	hash := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hash[i] = byte(i)
	}
	
	field, err := HashToField(hash)
	if err != nil {
		t.Fatalf("HashToField failed: %v", err)
	}
	if field == nil {
		t.Fatal("HashToField returned nil")
	}
}

