//go:build !js && !wasm && !tinygo && !wasm32

package signer

import (
	"testing"

	"p256k1.mleku.dev"
)

func TestP256K1Signer_Generate(t *testing.T) {
	s := NewP256K1Signer()
	if err := s.Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that we have a secret key
	sec := s.Sec()
	if sec == nil || len(sec) != 32 {
		t.Error("secret key should be 32 bytes")
	}

	// Check that we have a public key
	pub := s.Pub()
	if pub == nil || len(pub) != 32 {
		t.Error("public key should be 32 bytes")
	}

	// Check that we can sign
	msg := make([]byte, 32)
	sig, err := s.Sign(msg)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if len(sig) != 64 {
		t.Error("signature should be 64 bytes")
	}

	// Check that we can verify
	valid, err := s.Verify(msg, sig)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !valid {
		t.Error("signature should be valid")
	}

	// Test with wrong message
	wrongMsg := make([]byte, 32)
	wrongMsg[0] = 1
	valid, err = s.Verify(wrongMsg, sig)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if valid {
		t.Error("signature should be invalid for wrong message")
	}

	s.Zero()
}

func TestP256K1Signer_InitSec(t *testing.T) {
	// Generate a secret key
	seckey := make([]byte, 32)
	for i := range seckey {
		seckey[i] = byte(i + 1)
	}

	s := NewP256K1Signer()
	if err := s.InitSec(seckey); err != nil {
		t.Fatalf("InitSec failed: %v", err)
	}

	// Check secret key matches
	sec := s.Sec()
	for i := 0; i < 32; i++ {
		if sec[i] != seckey[i] {
			t.Errorf("secret key mismatch at byte %d", i)
		}
	}

	// Check we can sign
	msg := make([]byte, 32)
	sig, err := s.Sign(msg)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if len(sig) != 64 {
		t.Error("signature should be 64 bytes")
	}

	s.Zero()
}

func TestP256K1Signer_InitPub(t *testing.T) {
	// Generate a keypair first to get a valid x-only pubkey
	kp, err := p256k1.KeyPairGenerate()
	if err != nil {
		t.Fatalf("KeyPairGenerate failed: %v", err)
	}

	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("XOnlyPubkey failed: %v", err)
	}

	pubBytes := xonly.Serialize()

	// Create signer with only public key
	s := NewP256K1Signer()
	if err := s.InitPub(pubBytes[:]); err != nil {
		t.Fatalf("InitPub failed: %v", err)
	}

	// Check public key matches
	pub := s.Pub()
	for i := 0; i < 32; i++ {
		if pub[i] != pubBytes[i] {
			t.Errorf("public key mismatch at byte %d", i)
		}
	}

	// Should not be able to sign
	msg := make([]byte, 32)
	_, err = s.Sign(msg)
	if err == nil {
		t.Error("should not be able to sign with only public key")
	}

	// Should be able to verify (create a signature with the original keypair)
	var sig [64]byte
	if err := p256k1.SchnorrSign(sig[:], msg, kp, nil); err != nil {
		t.Fatalf("SchnorrSign failed: %v", err)
	}

	valid, err := s.Verify(msg, sig[:])
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !valid {
		t.Error("signature should be valid")
	}

	s.Zero()
}

func TestP256K1Signer_ECDH(t *testing.T) {
	// Generate two keypairs
	s1 := NewP256K1Signer()
	if err := s1.Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	defer s1.Zero()

	s2 := NewP256K1Signer()
	if err := s2.Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	defer s2.Zero()

	// Compute shared secrets
	pub1 := s1.Pub()
	pub2 := s2.Pub()

	secret1, err := s1.ECDH(pub2)
	if err != nil {
		t.Fatalf("ECDH failed: %v", err)
	}

	secret2, err := s2.ECDH(pub1)
	if err != nil {
		t.Fatalf("ECDH failed: %v", err)
	}

	// Shared secrets should match
	if len(secret1) != 32 || len(secret2) != 32 {
		t.Error("shared secrets should be 32 bytes")
	}

	for i := 0; i < 32; i++ {
		if secret1[i] != secret2[i] {
			t.Errorf("shared secrets mismatch at byte %d", i)
		}
	}
}

func TestP256K1Gen_Generate(t *testing.T) {
	g := NewP256K1Gen()

	pubBytes, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(pubBytes) != 33 {
		t.Errorf("compressed pubkey should be 33 bytes, got %d", len(pubBytes))
	}

	// Check prefix is 0x02 or 0x03
	if pubBytes[0] != 0x02 && pubBytes[0] != 0x03 {
		t.Errorf("invalid compressed pubkey prefix: 0x%02x", pubBytes[0])
	}
}

func TestP256K1Gen_Negate(t *testing.T) {
	g := NewP256K1Gen()

	pubBytes1, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Store the original prefix
	originalPrefix := pubBytes1[0]

	// Negate and check prefix changes
	g.Negate()

	// Get compressed pubkey from the keypair (don't generate new one)
	if g.compressedPub == nil {
		t.Fatal("compressedPub should not be nil after Generate")
	}

	var compressedPub [33]byte
	n := p256k1.ECPubkeySerialize(compressedPub[:], g.compressedPub, p256k1.ECCompressed)
	if n != 33 {
		t.Fatal("failed to serialize compressed pubkey")
	}

	// Prefixes should be different (02 vs 03)
	if originalPrefix == compressedPub[0] {
		t.Error("Negate should flip the Y coordinate parity")
	}

	// X coordinates should be the same
	for i := 1; i < 33; i++ {
		if pubBytes1[i] != compressedPub[i] {
			t.Errorf("X coordinate should not change, mismatch at byte %d", i)
		}
	}
}

func TestP256K1Gen_KeyPairBytes(t *testing.T) {
	g := NewP256K1Gen()

	compressedPub, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	secBytes, pubBytes := g.KeyPairBytes()

	if len(secBytes) != 32 {
		t.Errorf("secret key should be 32 bytes, got %d", len(secBytes))
	}

	if len(pubBytes) != 32 {
		t.Errorf("x-only pubkey should be 32 bytes, got %d", len(pubBytes))
	}

	// Verify the pubkey matches the compressed pubkey X coordinate
	// (compressedPub[1:] is the X coordinate)
	for i := 0; i < 32; i++ {
		if pubBytes[i] != compressedPub[i+1] {
			t.Errorf("x-only pubkey mismatch at byte %d", i)
		}
	}
}
