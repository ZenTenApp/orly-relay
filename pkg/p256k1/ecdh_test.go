package p256k1

import (
	"testing"
)

func TestEcmultConst(t *testing.T) {
	// Test with generator point
	var scalar Scalar
	scalar.setInt(5)

	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	var result GroupElementJacobian
	EcmultConst(&result, &Generator, &scalar)

	if result.isInfinity() {
		t.Error("5*G should not be infinity")
	}

	// Verify it matches EcmultGen for generator
	var expected GroupElementJacobian
	EcmultGen(&expected, &scalar)

	var resultAff, expectedAff GroupElementAffine
	resultAff.setGEJ(&result)
	expectedAff.setGEJ(&expected)

	resultAff.x.normalize()
	resultAff.y.normalize()
	expectedAff.x.normalize()
	expectedAff.y.normalize()

	if !resultAff.x.equal(&expectedAff.x) || !resultAff.y.equal(&expectedAff.y) {
		t.Error("EcmultConst result does not match EcmultGen for generator")
	}
}

func TestEcmult(t *testing.T) {
	// Test with arbitrary point
	var scalar Scalar
	scalar.setInt(3)

	var point GroupElementAffine
	point.setXY(&Generator.x, &Generator.y)

	var pointJac GroupElementJacobian
	pointJac.setGE(&point)

	var result GroupElementJacobian
	Ecmult(&result, &pointJac, &scalar)

	if result.isInfinity() {
		t.Error("3*P should not be infinity")
	}

	// Verify it matches EcmultConst
	var expected GroupElementJacobian
	EcmultConst(&expected, &point, &scalar)

	var resultAff, expectedAff GroupElementAffine
	resultAff.setGEJ(&result)
	expectedAff.setGEJ(&expected)

	resultAff.x.normalize()
	resultAff.y.normalize()
	expectedAff.x.normalize()
	expectedAff.y.normalize()

	if !resultAff.x.equal(&expectedAff.x) || !resultAff.y.equal(&expectedAff.y) {
		t.Error("Ecmult result does not match EcmultConst")
	}
}

func TestECDH(t *testing.T) {
	// Generate two key pairs
	seckey1, pubkey1, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 1: %v", err)
	}

	seckey2, pubkey2, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 2: %v", err)
	}

	// Compute shared secrets
	var shared1, shared2 [32]byte

	// Alice computes shared secret with Bob's public key
	if err := ECDH(shared1[:], pubkey2, seckey1, nil); err != nil {
		t.Fatalf("ECDH failed for Alice: %v", err)
	}

	// Bob computes shared secret with Alice's public key
	if err := ECDH(shared2[:], pubkey1, seckey2, nil); err != nil {
		t.Fatalf("ECDH failed for Bob: %v", err)
	}

	// Both should have the same shared secret
	for i := 0; i < 32; i++ {
		if shared1[i] != shared2[i] {
			t.Errorf("shared secrets differ at byte %d: 0x%02x != 0x%02x", i, shared1[i], shared2[i])
		}
	}
}

func TestECDHZeroKey(t *testing.T) {
	// Test that zero key is rejected
	_, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	zeroKey := make([]byte, 32)
	var output [32]byte

	err = ECDH(output[:], pubkey, zeroKey, nil)
	if err == nil {
		t.Error("ECDH should fail with zero key")
	}
}

func TestECDHInvalidKey(t *testing.T) {
	_, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Test with invalid key (all 0xFF - likely invalid)
	invalidKey := make([]byte, 32)
	for i := range invalidKey {
		invalidKey[i] = 0xFF
	}

	var output [32]byte
	err = ECDH(output[:], pubkey, invalidKey, nil)
	if err == nil {
		// If it doesn't fail, verify the key is actually valid
		if !ECSeckeyVerify(invalidKey) {
			t.Error("ECDH should fail with invalid key")
		}
	}
}

func TestECDHCustomHash(t *testing.T) {
	// Test with custom hash function
	seckey1, pubkey1, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 1: %v", err)
	}

	seckey2, pubkey2, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 2: %v", err)
	}

	// Custom hash: just XOR x and y
	customHash := func(output []byte, x32 []byte, y32 []byte) bool {
		if len(output) != 32 {
			return false
		}
		for i := 0; i < 32; i++ {
			output[i] = x32[i] ^ y32[i]
		}
		return true
	}

	var shared1, shared2 [32]byte

	if err := ECDH(shared1[:], pubkey2, seckey1, customHash); err != nil {
		t.Fatalf("ECDH failed: %v", err)
	}

	if err := ECDH(shared2[:], pubkey1, seckey2, customHash); err != nil {
		t.Fatalf("ECDH failed: %v", err)
	}

	for i := 0; i < 32; i++ {
		if shared1[i] != shared2[i] {
			t.Errorf("shared secrets differ at byte %d", i)
		}
	}
}

func TestHKDF(t *testing.T) {
	// Test HKDF with known inputs
	ikm := []byte("test input key material")
	salt := []byte("test salt")
	info := []byte("test info")

	output := make([]byte, 64)
	if err := HKDF(output, ikm, salt, info); err != nil {
		t.Fatalf("HKDF failed: %v", err)
	}

	// Verify output is not all zeros
	allZero := true
	for i := 0; i < len(output); i++ {
		if output[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("HKDF output is all zeros")
	}

	// Test with empty salt
	output2 := make([]byte, 32)
	if err := HKDF(output2, ikm, nil, info); err != nil {
		t.Fatalf("HKDF failed with empty salt: %v", err)
	}

	// Test with empty info
	output3 := make([]byte, 32)
	if err := HKDF(output3, ikm, salt, nil); err != nil {
		t.Fatalf("HKDF failed with empty info: %v", err)
	}
}

func TestECDHWithHKDF(t *testing.T) {
	seckey1, pubkey1, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 1: %v", err)
	}

	seckey2, pubkey2, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 2: %v", err)
	}

	salt := []byte("test salt")
	info := []byte("test info")

	// Derive keys
	var key1, key2 [64]byte
	if err := ECDHWithHKDF(key1[:], pubkey2, seckey1, salt, info); err != nil {
		t.Fatalf("ECDHWithHKDF failed: %v", err)
	}

	if err := ECDHWithHKDF(key2[:], pubkey1, seckey2, salt, info); err != nil {
		t.Fatalf("ECDHWithHKDF failed: %v", err)
	}

	// Keys should match
	for i := 0; i < 64; i++ {
		if key1[i] != key2[i] {
			t.Errorf("derived keys differ at byte %d", i)
		}
	}
}

func TestECDHXOnly(t *testing.T) {
	seckey1, pubkey1, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 1: %v", err)
	}

	seckey2, pubkey2, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair 2: %v", err)
	}

	// Compute X-only shared secrets
	var x1, x2 [32]byte

	if err := ECDHXOnly(x1[:], pubkey2, seckey1); err != nil {
		t.Fatalf("ECDHXOnly failed: %v", err)
	}

	if err := ECDHXOnly(x2[:], pubkey1, seckey2); err != nil {
		t.Fatalf("ECDHXOnly failed: %v", err)
	}

	// X coordinates should match
	for i := 0; i < 32; i++ {
		if x1[i] != x2[i] {
			t.Errorf("X-only shared secrets differ at byte %d", i)
		}
	}
}
