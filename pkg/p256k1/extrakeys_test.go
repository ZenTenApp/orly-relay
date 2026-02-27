package p256k1

import (
	"testing"
)

func TestXOnlyPubkeyParse(t *testing.T) {
	// Generate a keypair and get its x-only pubkey
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey: %v", err)
	}

	// Serialize and parse back
	serialized := xonly.Serialize()
	parsed, err := XOnlyPubkeyParse(serialized[:])
	if err != nil {
		t.Fatalf("failed to parse x-only pubkey: %v", err)
	}

	// Should match
	if XOnlyPubkeyCmp(xonly, parsed) != 0 {
		t.Error("parsed x-only pubkey does not match original")
	}
}

func TestXOnlyPubkeyFromPubkey(t *testing.T) {
	// Generate keypair
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	// Convert to x-only
	xonly, parity, err := XOnlyPubkeyFromPubkey(kp.Pubkey())
	if err != nil {
		t.Fatalf("failed to convert to x-only: %v", err)
	}

	// Parity should be 0 or 1
	if parity != 0 && parity != 1 {
		t.Errorf("invalid parity: %d", parity)
	}

	// X coordinate should match
	var pkX [32]byte
	var pt GroupElementAffine
	pt.fromBytes(kp.Pubkey().data[:])
	if parity == 1 {
		pt.negate(&pt)
	}
	pt.x.normalize()
	pt.x.getB32(pkX[:])

	xonlySerialized := xonly.Serialize()
	for i := 0; i < 32; i++ {
		if pkX[i] != xonlySerialized[i] {
			t.Errorf("X coordinate mismatch at byte %d", i)
		}
	}
}

func TestKeyPairCreate(t *testing.T) {
	// Generate a secret key
	seckey, err := ECSeckeyGenerate()
	if err != nil {
		t.Fatalf("failed to generate secret key: %v", err)
	}

	// Create keypair
	kp, err := KeyPairCreate(seckey)
	if err != nil {
		t.Fatalf("failed to create keypair: %v", err)
	}

	// Verify secret key matches
	kpSeckey := kp.Seckey()
	for i := 0; i < 32; i++ {
		if kpSeckey[i] != seckey[i] {
			t.Errorf("secret key mismatch at byte %d", i)
		}
	}

	// Verify public key matches
	var expectedPubkey PublicKey
	if err := ECPubkeyCreate(&expectedPubkey, seckey); err != nil {
		t.Fatalf("failed to create expected pubkey: %v", err)
	}

	if ECPubkeyCmp(kp.Pubkey(), &expectedPubkey) != 0 {
		t.Error("public key does not match")
	}
}

func TestKeyPairGenerate(t *testing.T) {
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	// Verify secret key is valid
	if !ECSeckeyVerify(kp.Seckey()) {
		t.Error("generated secret key is invalid")
	}

	// Verify public key matches secret key
	var expectedPubkey PublicKey
	if err := ECPubkeyCreate(&expectedPubkey, kp.Seckey()); err != nil {
		t.Fatalf("failed to create expected pubkey: %v", err)
	}

	if ECPubkeyCmp(kp.Pubkey(), &expectedPubkey) != 0 {
		t.Error("public key does not match secret key")
	}
}

func TestXOnlyPubkeyCmp(t *testing.T) {
	kp1, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair 1: %v", err)
	}

	kp2, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair 2: %v", err)
	}

	xonly1, err := kp1.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey 1: %v", err)
	}

	xonly2, err := kp2.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey 2: %v", err)
	}

	// Compare with itself should return 0
	if XOnlyPubkeyCmp(xonly1, xonly1) != 0 {
		t.Error("x-only pubkey should equal itself")
	}

	// Compare with different key should return non-zero
	cmp := XOnlyPubkeyCmp(xonly1, xonly2)
	if cmp == 0 {
		t.Error("different x-only pubkeys should not compare equal")
	}
}
