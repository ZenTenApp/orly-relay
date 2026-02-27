package p256k1

import (
	"testing"
)

func TestECSeckeyVerify(t *testing.T) {
	// Test valid key
	validKey := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}
	if !ECSeckeyVerify(validKey) {
		t.Error("valid key should verify")
	}
	
	// Test invalid key (all zeros)
	invalidKey := make([]byte, 32)
	if ECSeckeyVerify(invalidKey) {
		t.Error("zero key should not verify")
	}
	
	// Test wrong length
	if ECSeckeyVerify(validKey[:31]) {
		t.Error("wrong length should not verify")
	}
}

func TestECSeckeyGenerate(t *testing.T) {
	key, err := ECSeckeyGenerate()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length should be 32, got %d", len(key))
	}
	if !ECSeckeyVerify(key) {
		t.Error("generated key should be valid")
	}
}

func TestECKeyPairGenerate(t *testing.T) {
	seckey, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	if len(seckey) != 32 {
		t.Errorf("secret key length should be 32, got %d", len(seckey))
	}
	if pubkey == nil {
		t.Fatal("public key should not be nil")
	}
	
	// Verify the public key matches the secret key
	var expectedPubkey PublicKey
	if err := ECPubkeyCreate(&expectedPubkey, seckey); err != nil {
		t.Fatalf("failed to create expected public key: %v", err)
	}
	
	if ECPubkeyCmp(pubkey, &expectedPubkey) != 0 {
		t.Error("generated public key does not match secret key")
	}
}

func TestECSeckeyNegate(t *testing.T) {
	key := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}
	
	keyCopy := make([]byte, 32)
	copy(keyCopy, key)
	
	if !ECSeckeyNegate(keyCopy) {
		t.Error("negation should succeed")
	}
	
	// Negating twice should give original
	if !ECSeckeyNegate(keyCopy) {
		t.Error("second negation should succeed")
	}
	
	// Keys should be equal
	for i := 0; i < 32; i++ {
		if key[i] != keyCopy[i] {
			t.Error("double negation should restore original")
			break
		}
	}
}

func TestECSeckeyTweakAdd(t *testing.T) {
	seckey := []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}
	
	tweak := []byte{
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
	}
	
	originalSeckey := make([]byte, 32)
	copy(originalSeckey, seckey)
	
	if err := ECSeckeyTweakAdd(seckey, tweak); err != nil {
		t.Fatalf("tweak add failed: %v", err)
	}
	
	// Verify key is still valid
	if !ECSeckeyVerify(seckey) {
		t.Error("tweaked key should be valid")
	}
	
	// Keys should be different
	allSame := true
	for i := 0; i < 32; i++ {
		if seckey[i] != originalSeckey[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("tweaked key should be different from original")
	}
}

func TestECPubkeyTweakAdd(t *testing.T) {
	// Generate key pair
	seckey, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	
	tweak := []byte{
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
	}
	
	originalPubkey := *pubkey
	
	// Tweak secret key
	seckeyCopy := make([]byte, 32)
	copy(seckeyCopy, seckey)
	if err := ECSeckeyTweakAdd(seckeyCopy, tweak); err != nil {
		t.Fatalf("failed to tweak secret key: %v", err)
	}
	
	// Compute expected public key from tweaked secret key
	var expectedPubkey PublicKey
	if err := ECPubkeyCreate(&expectedPubkey, seckeyCopy); err != nil {
		t.Fatalf("failed to create expected public key: %v", err)
	}
	
	// Tweak public key
	if err := ECPubkeyTweakAdd(pubkey, tweak); err != nil {
		t.Fatalf("failed to tweak public key: %v", err)
	}
	
	// Public keys should match
	if ECPubkeyCmp(pubkey, &expectedPubkey) != 0 {
		t.Error("tweaked public key does not match tweaked secret key")
	}
	
	// Should be different from original
	if ECPubkeyCmp(pubkey, &originalPubkey) == 0 {
		t.Error("tweaked public key should be different from original")
	}
}

func TestECPubkeyTweakMul(t *testing.T) {
	// Generate key pair
	seckey, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	
	tweak := []byte{
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
	}
	
	originalPubkey := *pubkey
	
	// Tweak secret key
	seckeyCopy := make([]byte, 32)
	copy(seckeyCopy, seckey)
	if err := ECSeckeyTweakMul(seckeyCopy, tweak); err != nil {
		t.Fatalf("failed to tweak secret key: %v", err)
	}
	
	// Compute expected public key from tweaked secret key
	var expectedPubkey PublicKey
	if err := ECPubkeyCreate(&expectedPubkey, seckeyCopy); err != nil {
		t.Fatalf("failed to create expected public key: %v", err)
	}
	
	// Tweak public key
	if err := ECPubkeyTweakMul(pubkey, tweak); err != nil {
		t.Fatalf("failed to tweak public key: %v", err)
	}
	
	// Public keys should match
	if ECPubkeyCmp(pubkey, &expectedPubkey) != 0 {
		t.Error("tweaked public key does not match tweaked secret key")
	}
	
	// Should be different from original
	if ECPubkeyCmp(pubkey, &originalPubkey) == 0 {
		t.Error("tweaked public key should be different from original")
	}
}

