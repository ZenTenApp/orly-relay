//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"testing"
)

func TestSchnorrSignVerify(t *testing.T) {
	// Generate keypair
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	defer kp.Clear()

	// Get x-only pubkey
	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey: %v", err)
	}

	// Create message
	msg := make([]byte, 32)
	for i := range msg {
		msg[i] = byte(i)
	}

	// Sign
	var sig [64]byte
	if err := SchnorrSign(sig[:], msg, kp, nil); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Verify
	if !SchnorrVerify(sig[:], msg, xonly) {
		t.Error("signature verification failed")
	}

	// Test with wrong message
	wrongMsg := make([]byte, 32)
	copy(wrongMsg, msg)
	wrongMsg[0] ^= 1
	if SchnorrVerify(sig[:], wrongMsg, xonly) {
		t.Error("signature verification should fail with wrong message")
	}
}

func TestSchnorrSignWithAuxRand(t *testing.T) {
	// Generate keypair
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	defer kp.Clear()

	// Get x-only pubkey
	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey: %v", err)
	}

	// Create message
	msg := make([]byte, 32)
	for i := range msg {
		msg[i] = byte(i)
	}

	// Auxiliary randomness
	auxRand := make([]byte, 32)
	for i := range auxRand {
		auxRand[i] = byte(i + 100)
	}

	// Sign
	var sig [64]byte
	if err := SchnorrSign(sig[:], msg, kp, auxRand); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Verify
	if !SchnorrVerify(sig[:], msg, xonly) {
		t.Error("signature verification failed")
	}
}

func TestSchnorrVerifyInvalid(t *testing.T) {
	// Generate keypair
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	defer kp.Clear()

	// Get x-only pubkey
	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey: %v", err)
	}

	msg := make([]byte, 32)

	// Test with invalid signature length
	if SchnorrVerify([]byte{1}, msg, xonly) {
		t.Error("should fail with invalid signature length")
	}

	// Test with invalid message length
	var sig [64]byte
	if SchnorrVerify(sig[:], []byte{1}, xonly) {
		t.Error("should fail with invalid message length")
	}

	// Test with nil pubkey
	if SchnorrVerify(sig[:], msg, nil) {
		t.Error("should fail with nil pubkey")
	}
}

func TestNonceFunctionBIP340(t *testing.T) {
	key32 := make([]byte, 32)
	xonlyPk32 := make([]byte, 32)
	msg := make([]byte, 32) // BIP-340 requires 32-byte message hash
	auxRand32 := make([]byte, 32)

	// Initialize test data
	for i := range key32 {
		key32[i] = byte(i)
	}
	for i := range xonlyPk32 {
		xonlyPk32[i] = byte(i + 10)
	}
	for i := range auxRand32 {
		auxRand32[i] = byte(i + 20)
	}

	// Test with aux random
	var nonce1 [32]byte
	if err := NonceFunctionBIP340(nonce1[:], msg, key32, xonlyPk32, auxRand32); err != nil {
		t.Fatalf("nonce generation failed: %v", err)
	}

	// Test without aux random
	var nonce2 [32]byte
	if err := NonceFunctionBIP340(nonce2[:], msg, key32, xonlyPk32, nil); err != nil {
		t.Fatalf("nonce generation failed: %v", err)
	}

	// Nonces should be different
	allSame := true
	for i := 0; i < 32; i++ {
		if nonce1[i] != nonce2[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("nonces should differ with different aux random")
	}
}

func TestSchnorrMultipleSignatures(t *testing.T) {
	// Test that multiple signatures with same keypair are different when using different aux_rand
	kp, err := KeyPairGenerate()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	defer kp.Clear()

	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		t.Fatalf("failed to get x-only pubkey: %v", err)
	}

	msg := make([]byte, 32)

	// Sign without aux_rand (deterministic - should be same)
	var sig1, sig2 [64]byte
	if err := SchnorrSign(sig1[:], msg, kp, nil); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	if err := SchnorrSign(sig2[:], msg, kp, nil); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Both should verify
	if !SchnorrVerify(sig1[:], msg, xonly) {
		t.Error("signature 1 verification failed")
	}
	if !SchnorrVerify(sig2[:], msg, xonly) {
		t.Error("signature 2 verification failed")
	}

	// Without aux_rand, signatures should be deterministic (same)
	allSame := true
	for i := 0; i < 64; i++ {
		if sig1[i] != sig2[i] {
			allSame = false
			break
		}
	}
	if !allSame {
		t.Error("without aux_rand, signatures should be deterministic (same)")
	}

	// Sign with different aux_rand (should be different)
	auxRand1 := make([]byte, 32)
	auxRand2 := make([]byte, 32)
	for i := range auxRand1 {
		auxRand1[i] = byte(i)
		auxRand2[i] = byte(i + 1)
	}

	if err := SchnorrSign(sig1[:], msg, kp, auxRand1); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	if err := SchnorrSign(sig2[:], msg, kp, auxRand2); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Both should verify
	if !SchnorrVerify(sig1[:], msg, xonly) {
		t.Error("signature 1 verification failed")
	}
	if !SchnorrVerify(sig2[:], msg, xonly) {
		t.Error("signature 2 verification failed")
	}

	// With different aux_rand, signatures should differ
	allSame = true
	for i := 0; i < 64; i++ {
		if sig1[i] != sig2[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("with different aux_rand, signatures should differ")
	}
}

func BenchmarkSchnorrVerify(b *testing.B) {
	// Generate test data once outside the benchmark loop
	kp, err := KeyPairGenerate()
	if err != nil {
		b.Fatalf("failed to generate keypair: %v", err)
	}
	defer kp.Clear()

	xonly, err := kp.XOnlyPubkey()
	if err != nil {
		b.Fatalf("failed to get x-only pubkey: %v", err)
	}

	msg := make([]byte, 32)
	for i := range msg {
		msg[i] = byte(i)
	}

	sig := make([]byte, 64)
	if err := SchnorrSign(sig, msg, kp, nil); err != nil {
		b.Fatalf("failed to sign: %v", err)
	}

	// Convert to internal types once
	var secpXonly secp256k1_xonly_pubkey
	copy(secpXonly.data[:], xonly.data[:])

	// Benchmark verification with pre-computed values
	b.ResetTimer()
	b.ReportAllocs()

	ctx := getSchnorrVerifyContext()
	for i := 0; i < b.N; i++ {
		result := secp256k1_schnorrsig_verify(ctx, sig, msg, 32, &secpXonly)
		if result == 0 {
			b.Fatal("verification failed")
		}
	}
}
