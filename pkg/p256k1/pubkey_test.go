package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestECPubkeyCreate(t *testing.T) {
	// Generate a random private key
	seckey := make([]byte, 32)
	if _, err := rand.Read(seckey); err != nil {
		t.Fatal(err)
	}

	// Ensure it's a valid private key (not zero, not >= order)
	var scalar Scalar
	for !scalar.setB32Seckey(seckey) {
		if _, err := rand.Read(seckey); err != nil {
			t.Fatal(err)
		}
	}

	// Create public key
	var pubkey PublicKey
	err := ECPubkeyCreate(&pubkey, seckey)
	if err != nil {
		t.Errorf("ECPubkeyCreate failed: %v", err)
	}

	// Verify the public key is valid by parsing it
	var parsed PublicKey
	var serialized [65]byte
	length := ECPubkeySerialize(serialized[:], &pubkey, ECUncompressed)
	if length != 65 {
		t.Error("uncompressed serialization should be 65 bytes")
	}

	err = ECPubkeyParse(&parsed, serialized[:length])
	if err != nil {
		t.Errorf("failed to parse created public key: %v", err)
	}

	// Compare original and parsed
	if ECPubkeyCmp(&pubkey, &parsed) != 0 {
		t.Error("parsed public key should equal original")
	}
}

func TestECPubkeyParse(t *testing.T) {
	// Test with generator point (known valid point)
	// Generator X: 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
	// Generator Y: 0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8

	// Uncompressed format
	uncompressed := []byte{
		0x04,
		0x79, 0xBE, 0x66, 0x7E, 0xF9, 0xDC, 0xBB, 0xAC, 0x55, 0xA0, 0x62, 0x95, 0xCE, 0x87, 0x0B, 0x07,
		0x02, 0x9B, 0xFC, 0xDB, 0x2D, 0xCE, 0x28, 0xD9, 0x59, 0xF2, 0x81, 0x5B, 0x16, 0xF8, 0x17, 0x98,
		0x48, 0x3A, 0xDA, 0x77, 0x26, 0xA3, 0xC4, 0x65, 0x5D, 0xA4, 0xFB, 0xFC, 0x0E, 0x11, 0x08, 0xA8,
		0xFD, 0x17, 0xB4, 0x48, 0xA6, 0x85, 0x54, 0x19, 0x9C, 0x47, 0xD0, 0x8F, 0xFB, 0x10, 0xD4, 0xB8,
	}

	var pubkey PublicKey
	err := ECPubkeyParse(&pubkey, uncompressed)
	if err != nil {
		t.Errorf("failed to parse uncompressed generator: %v", err)
	}

	// Compressed format (even Y)
	compressed := []byte{
		0x02,
		0x79, 0xBE, 0x66, 0x7E, 0xF9, 0xDC, 0xBB, 0xAC, 0x55, 0xA0, 0x62, 0x95, 0xCE, 0x87, 0x0B, 0x07,
		0x02, 0x9B, 0xFC, 0xDB, 0x2D, 0xCE, 0x28, 0xD9, 0x59, 0xF2, 0x81, 0x5B, 0x16, 0xF8, 0x17, 0x98,
	}

	var pubkey2 PublicKey
	err = ECPubkeyParse(&pubkey2, compressed)
	if err != nil {
		t.Errorf("failed to parse compressed generator: %v", err)
	}

	// Both should be equal
	if ECPubkeyCmp(&pubkey, &pubkey2) != 0 {
		t.Error("compressed and uncompressed generator should be equal")
	}

	// Test invalid inputs
	invalidInputs := [][]byte{
		{},                    // empty
		{0x05},               // invalid prefix
		{0x04, 0x00},         // too short
		make([]byte, 66),     // too long
		{0x02},               // compressed too short
		make([]byte, 34),     // compressed too long
	}

	for i, invalid := range invalidInputs {
		var dummy PublicKey
		err := ECPubkeyParse(&dummy, invalid)
		if err == nil {
			t.Errorf("invalid input %d should have failed", i)
		}
	}
}

func TestECPubkeySerialize(t *testing.T) {
	// Create a public key from a known private key
	seckey := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}

	var pubkey PublicKey
	err := ECPubkeyCreate(&pubkey, seckey)
	if err != nil {
		t.Fatalf("failed to create public key: %v", err)
	}

	// Test compressed serialization
	var compressed [33]byte
	compressedLength := ECPubkeySerialize(compressed[:], &pubkey, ECCompressed)
	if compressedLength != 33 {
		t.Errorf("compressed serialization should return 33 bytes, got %d", compressedLength)
	}
	if compressed[0] != 0x02 && compressed[0] != 0x03 {
		t.Errorf("compressed format should start with 0x02 or 0x03, got 0x%02x", compressed[0])
	}

	// Test uncompressed serialization
	var uncompressed [65]byte
	uncompressedLength := ECPubkeySerialize(uncompressed[:], &pubkey, ECUncompressed)
	if uncompressedLength != 65 {
		t.Errorf("uncompressed serialization should return 65 bytes, got %d", uncompressedLength)
	}
	if uncompressed[0] != 0x04 {
		t.Errorf("uncompressed format should start with 0x04, got 0x%02x", uncompressed[0])
	}

	// Test round-trip
	var parsed1, parsed2 PublicKey
	err = ECPubkeyParse(&parsed1, compressed[:compressedLength])
	if err != nil {
		t.Errorf("failed to parse compressed: %v", err)
	}

	err = ECPubkeyParse(&parsed2, uncompressed[:uncompressedLength])
	if err != nil {
		t.Errorf("failed to parse uncompressed: %v", err)
	}

	if ECPubkeyCmp(&parsed1, &parsed2) != 0 {
		t.Error("round-trip should preserve public key")
	}

	// Test buffer too small
	var small [32]byte
	smallLength := ECPubkeySerialize(small[:], &pubkey, ECCompressed)
	if smallLength != 0 {
		t.Error("serialization with small buffer should return 0")
	}

	// Test invalid flags
	invalidLength := ECPubkeySerialize(compressed[:], &pubkey, 0xFF)
	if invalidLength != 0 {
		t.Error("serialization with invalid flags should return 0")
	}
}

func TestECPubkeyCmp(t *testing.T) {
	// Create two different public keys
	seckey1 := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}
	seckey2 := []byte{
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
	}

	var pubkey1, pubkey2, pubkey3 PublicKey
	
	err := ECPubkeyCreate(&pubkey1, seckey1)
	if err != nil {
		t.Fatalf("failed to create pubkey1: %v", err)
	}
	
	err = ECPubkeyCreate(&pubkey2, seckey2)
	if err != nil {
		t.Fatalf("failed to create pubkey2: %v", err)
	}
	
	err = ECPubkeyCreate(&pubkey3, seckey1) // Same as pubkey1
	if err != nil {
		t.Fatalf("failed to create pubkey3: %v", err)
	}

	// Test equality
	if ECPubkeyCmp(&pubkey1, &pubkey3) != 0 {
		t.Error("identical public keys should compare equal")
	}

	// Test inequality
	cmp := ECPubkeyCmp(&pubkey1, &pubkey2)
	if cmp == 0 {
		t.Error("different public keys should not compare equal")
	}

	// Test symmetry
	cmp2 := ECPubkeyCmp(&pubkey2, &pubkey1)
	if (cmp > 0 && cmp2 >= 0) || (cmp < 0 && cmp2 <= 0) {
		t.Error("comparison should be antisymmetric")
	}
}

func BenchmarkECPubkeyCreate(b *testing.B) {
	seckey := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pubkey PublicKey
		ECPubkeyCreate(&pubkey, seckey)
	}
}

func BenchmarkECPubkeySerializeCompressed(b *testing.B) {
	seckey := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}

	var pubkey PublicKey
	ECPubkeyCreate(&pubkey, seckey)
	var output [33]byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ECPubkeySerialize(output[:], &pubkey, ECCompressed)
	}
}

func BenchmarkECPubkeyParse(b *testing.B) {
	// Use generator point in compressed format
	compressed := []byte{
		0x02,
		0x79, 0xBE, 0x66, 0x7E, 0xF9, 0xDC, 0xBB, 0xAC, 0x55, 0xA0, 0x62, 0x95, 0xCE, 0x87, 0x0B, 0x07,
		0x02, 0x9B, 0xFC, 0xDB, 0x2D, 0xCE, 0x28, 0xD9, 0x59, 0xF2, 0x81, 0x5B, 0x16, 0xF8, 0x17, 0x98,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pubkey PublicKey
		ECPubkeyParse(&pubkey, compressed)
	}
}
