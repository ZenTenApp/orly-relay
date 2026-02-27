package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestECDSASignVerify(t *testing.T) {
	// Generate a random private key
	seckey := make([]byte, 32)
	if _, err := rand.Read(seckey); err != nil {
		t.Fatal(err)
	}
	
	// Ensure it's a valid private key
	var scalar Scalar
	for !scalar.setB32Seckey(seckey) {
		if _, err := rand.Read(seckey); err != nil {
			t.Fatal(err)
		}
	}
	
	// Create public key
	var pubkey PublicKey
	if err := ECPubkeyCreate(&pubkey, seckey); err != nil {
		t.Fatalf("failed to create public key: %v", err)
	}
	
	// Create message hash
	msghash := make([]byte, 32)
	if _, err := rand.Read(msghash); err != nil {
		t.Fatal(err)
	}
	
	// Sign
	var sig ECDSASignature
	if err := ECDSASign(&sig, msghash, seckey); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	
	// Verify
	if !ECDSAVerify(&sig, msghash, &pubkey) {
		t.Error("signature verification failed")
	}
	
	// Test with wrong message
	wrongMsg := make([]byte, 32)
	copy(wrongMsg, msghash)
	wrongMsg[0] ^= 1
	if ECDSAVerify(&sig, wrongMsg, &pubkey) {
		t.Error("signature verification should fail with wrong message")
	}
}

func TestECDSASignCompact(t *testing.T) {
	// Generate a random private key
	seckey := make([]byte, 32)
	if _, err := rand.Read(seckey); err != nil {
		t.Fatal(err)
	}
	
	// Ensure it's a valid private key
	var scalar Scalar
	for !scalar.setB32Seckey(seckey) {
		if _, err := rand.Read(seckey); err != nil {
			t.Fatal(err)
		}
	}
	
	// Create public key
	var pubkey PublicKey
	if err := ECPubkeyCreate(&pubkey, seckey); err != nil {
		t.Fatalf("failed to create public key: %v", err)
	}
	
	// Create message hash
	msghash := make([]byte, 32)
	if _, err := rand.Read(msghash); err != nil {
		t.Fatal(err)
	}
	
	// Sign using compact format
	var compactSig ECDSASignatureCompact
	if err := ECDSASignCompact(&compactSig, msghash, seckey); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	
	// Verify compact signature
	if !ECDSAVerifyCompact(&compactSig, msghash, &pubkey) {
		t.Error("compact signature verification failed")
	}
	
	// Test conversion
	var sig ECDSASignature
	if err := sig.FromCompact(&compactSig); err != nil {
		t.Fatalf("failed to parse compact signature: %v", err)
	}
	
	// Verify using regular format
	if !ECDSAVerify(&sig, msghash, &pubkey) {
		t.Error("signature verification failed after conversion")
	}
}

