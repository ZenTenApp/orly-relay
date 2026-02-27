package types

import (
	"bytes"
	"testing"
)

func TestEventID(t *testing.T) {
	// Test creation from bytes
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}

	id := EventIDFromBytes(src)

	// Verify contents
	if !bytes.Equal(id[:], src) {
		t.Errorf("EventIDFromBytes: got %x, want %x", id[:], src)
	}

	// Test Hex encoding
	expectedHex := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if got := id.Hex(); got != expectedHex {
		t.Errorf("Hex: got %s, want %s", got, expectedHex)
	}

	// Test FromHex
	id2, err := EventIDFromHex(expectedHex)
	if err != nil {
		t.Fatalf("EventIDFromHex error: %v", err)
	}
	if id2 != id {
		t.Errorf("EventIDFromHex: got %x, want %x", id2[:], id[:])
	}

	// Test Copy returns independent slice
	cp := id.Copy()
	cp[0] = 0xFF
	if id[0] == 0xFF {
		t.Error("Copy: modification affected original")
	}

	// Test Bytes returns view (shares memory)
	view := id.Bytes()
	if &view[0] != &id[0] {
		t.Error("Bytes: should return view sharing memory")
	}

	// Test IsZero
	var zero EventID
	if !zero.IsZero() {
		t.Error("IsZero: zero value should return true")
	}
	if id.IsZero() {
		t.Error("IsZero: non-zero value should return false")
	}
}

func TestPubkey(t *testing.T) {
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i + 32)
	}

	pk := PubkeyFromBytes(src)

	if !bytes.Equal(pk[:], src) {
		t.Errorf("PubkeyFromBytes: got %x, want %x", pk[:], src)
	}

	// Test hex round-trip
	hexStr := pk.Hex()
	pk2, err := PubkeyFromHex(hexStr)
	if err != nil {
		t.Fatalf("PubkeyFromHex error: %v", err)
	}
	if pk2 != pk {
		t.Errorf("PubkeyFromHex round-trip failed")
	}
}

func TestSignature(t *testing.T) {
	src := make([]byte, 64)
	for i := range src {
		src[i] = byte(i)
	}

	sig := SignatureFromBytes(src)

	if !bytes.Equal(sig[:], src) {
		t.Errorf("SignatureFromBytes: got %x, want %x", sig[:], src)
	}

	// Test size
	if len(sig.Bytes()) != SignatureSize {
		t.Errorf("Signature size: got %d, want %d", len(sig.Bytes()), SignatureSize)
	}
}

func TestSecretKey(t *testing.T) {
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i + 100)
	}

	sk := SecretKeyFromBytes(src)

	if !bytes.Equal(sk[:], src) {
		t.Errorf("SecretKeyFromBytes: got %x, want %x", sk[:], src)
	}

	// Test Zero
	sk.Zero()
	if !sk.IsZero() {
		t.Error("Zero: should clear all bytes")
	}
}

func TestCopyOnAssignment(t *testing.T) {
	// Verify that assignment creates a copy (key safety property)
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}

	id1 := EventIDFromBytes(src)
	id2 := id1 // Should copy

	// Modify id2
	id2[0] = 0xFF

	// id1 should be unchanged
	if id1[0] == 0xFF {
		t.Error("Assignment should create a copy, but original was modified")
	}
}

func TestShortInput(t *testing.T) {
	// Test that short inputs are zero-padded
	short := []byte{1, 2, 3}
	id := EventIDFromBytes(short)

	if id[0] != 1 || id[1] != 2 || id[2] != 3 {
		t.Error("Short input: first bytes should match")
	}
	if id[3] != 0 || id[31] != 0 {
		t.Error("Short input: remaining bytes should be zero")
	}
}

func BenchmarkEventIDCopy(b *testing.B) {
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}
	id := EventIDFromBytes(src)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Copy()
	}
}

func BenchmarkEventIDAssignment(b *testing.B) {
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}
	id := EventIDFromBytes(src)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id2 := id // Stack copy
		_ = id2
	}
}

func BenchmarkSliceCopy(b *testing.B) {
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := make([]byte, 32)
		copy(dst, src)
		_ = dst
	}
}
