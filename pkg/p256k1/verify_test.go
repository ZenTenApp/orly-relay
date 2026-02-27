//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"testing"
)

// TestSecp256k1SchnorrsigVerifyComparison tests that secp256k1_schnorrsig_verify
// produces the same results as the existing SchnorrVerify function
func TestSecp256k1SchnorrsigVerifyComparison(t *testing.T) {
	// Create a context (required by secp256k1_schnorrsig_verify)
	ctx := &secp256k1_context{
		ecmult_gen_ctx: secp256k1_ecmult_gen_context{built: 1},
		declassify:     0,
	}

	// Test case 1: Valid signature
	t.Run("ValidSignature", func(t *testing.T) {
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

		// Convert x-only pubkey to secp256k1_xonly_pubkey format
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		// Test existing implementation (old)
		existingResult := SchnorrVerifyOld(sig[:], msg, xonly)

		// Test new implementation (C-translated)
		newResult := SchnorrVerify(sig[:], msg, xonly)

		// Compare results
		if existingResult != newResult {
			t.Errorf("results differ: existing=%v, new=%v", existingResult, newResult)
		}

		if !existingResult {
			t.Error("signature verification failed (both implementations)")
		}
	})

	// Test case 2: Invalid signature (wrong message)
	t.Run("InvalidSignature_WrongMessage", func(t *testing.T) {
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

		// Create wrong message
		wrongMsg := make([]byte, 32)
		copy(wrongMsg, msg)
		wrongMsg[0] ^= 1

		// Convert x-only pubkey to secp256k1_xonly_pubkey format
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		// Test existing implementation (old)
		existingResult := SchnorrVerifyOld(sig[:], wrongMsg, xonly)

		// Test new implementation (C-translated)
		newResult := SchnorrVerify(sig[:], wrongMsg, xonly)

		// Compare results
		if existingResult != newResult {
			t.Errorf("results differ: existing=%v, new=%v", existingResult, newResult)
		}

		if existingResult {
			t.Error("signature verification should fail with wrong message (both implementations)")
		}
	})

	// Test case 3: Invalid signature (wrong signature)
	t.Run("InvalidSignature_WrongSignature", func(t *testing.T) {
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

		// Create wrong signature
		wrongSig := make([]byte, 64)
		copy(wrongSig, sig[:])
		wrongSig[0] ^= 1

		// Convert x-only pubkey to secp256k1_xonly_pubkey format
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		// Test existing implementation (old)
		existingResult := SchnorrVerifyOld(wrongSig, msg, xonly)

		// Test new implementation (C-translated)
		newResult := SchnorrVerify(wrongSig, msg, xonly)

		// Compare results
		if existingResult != newResult {
			t.Errorf("results differ: existing=%v, new=%v", existingResult, newResult)
		}

		if existingResult {
			t.Error("signature verification should fail with wrong signature (both implementations)")
		}
	})

	// Test case 4: Invalid signature (wrong pubkey)
	t.Run("InvalidSignature_WrongPubkey", func(t *testing.T) {
		// Generate two keypairs
		kp1, err := KeyPairGenerate()
		if err != nil {
			t.Fatalf("failed to generate keypair 1: %v", err)
		}
		defer kp1.Clear()

		kp2, err := KeyPairGenerate()
		if err != nil {
			t.Fatalf("failed to generate keypair 2: %v", err)
		}
		defer kp2.Clear()

		// Get x-only pubkey for kp2 (we sign with kp1, verify with kp2)
		xonly2, err := kp2.XOnlyPubkey()
		if err != nil {
			t.Fatalf("failed to get x-only pubkey 2: %v", err)
		}

		// Create message
		msg := make([]byte, 32)
		for i := range msg {
			msg[i] = byte(i)
		}

		// Sign with keypair 1
		var sig [64]byte
		if err := SchnorrSign(sig[:], msg, kp1, nil); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}

		// Convert x-only pubkey 2 to secp256k1_xonly_pubkey format
		var secp_xonly2 secp256k1_xonly_pubkey
		copy(secp_xonly2.data[:], xonly2.data[:])

		// Test existing implementation (old, verify with wrong pubkey)
		existingResult := SchnorrVerifyOld(sig[:], msg, xonly2)

		// Test new implementation (C-translated, verify with wrong pubkey)
		newResult := SchnorrVerify(sig[:], msg, xonly2)

		// Compare results
		if existingResult != newResult {
			t.Errorf("results differ: existing=%v, new=%v", existingResult, newResult)
		}

		if existingResult {
			t.Error("signature verification should fail with wrong pubkey (both implementations)")
		}
	})

	// Test case 5: Edge cases - nil/invalid inputs
	t.Run("EdgeCases", func(t *testing.T) {
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
		var sig [64]byte

		// Test with nil context
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		newResult := secp256k1_schnorrsig_verify(nil, sig[:], msg, len(msg), &secp_xonly)
		if newResult != 0 {
			t.Error("should return 0 with nil context")
		}

		// Test with nil signature
		newResult = secp256k1_schnorrsig_verify(ctx, nil, msg, len(msg), &secp_xonly)
		if newResult != 0 {
			t.Error("should return 0 with nil signature")
		}

		// Test with nil pubkey
		newResult = secp256k1_schnorrsig_verify(ctx, sig[:], msg, len(msg), nil)
		if newResult != 0 {
			t.Error("should return 0 with nil pubkey")
		}

		// Test with invalid signature length
		if SchnorrVerify([]byte{1}, msg, xonly) {
			t.Error("existing: should fail with invalid signature length")
		}
		newResult = secp256k1_schnorrsig_verify(ctx, []byte{1}, msg, len(msg), &secp_xonly)
		if newResult != 0 {
			t.Error("new: should return 0 with invalid signature length")
		}
	})

	// Test case 6: Multiple signatures with different aux_rand
	t.Run("MultipleSignatures_DifferentAuxRand", func(t *testing.T) {
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

		// Sign with different aux_rand values
		auxRand1 := make([]byte, 32)
		auxRand2 := make([]byte, 32)
		for i := range auxRand1 {
			auxRand1[i] = byte(i)
			auxRand2[i] = byte(i + 1)
		}

		var sig1, sig2 [64]byte
		if err := SchnorrSign(sig1[:], msg, kp, auxRand1); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}
		if err := SchnorrSign(sig2[:], msg, kp, auxRand2); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}

		// Convert x-only pubkey to secp256k1_xonly_pubkey format
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		// Test both signatures with existing implementation
		existingResult1 := SchnorrVerify(sig1[:], msg, xonly)
		existingResult2 := SchnorrVerify(sig2[:], msg, xonly)

		// Test both signatures with new implementation
		newResult1 := secp256k1_schnorrsig_verify(ctx, sig1[:], msg, len(msg), &secp_xonly)
		newResult2 := secp256k1_schnorrsig_verify(ctx, sig2[:], msg, len(msg), &secp_xonly)

		// Compare results
		if existingResult1 != (newResult1 != 0) {
			t.Errorf("signature 1 results differ: existing=%v, new=%d", existingResult1, newResult1)
		}
		if existingResult2 != (newResult2 != 0) {
			t.Errorf("signature 2 results differ: existing=%v, new=%d", existingResult2, newResult2)
		}

		if !existingResult1 || !existingResult2 {
			t.Error("both signatures should verify")
		}
	})

	// Test case 7: Empty message
	t.Run("EmptyMessage", func(t *testing.T) {
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

		// Create 32-byte message (all zeros)
		msg := make([]byte, 32)

		// Sign
		var sig [64]byte
		if err := SchnorrSign(sig[:], msg, kp, nil); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}

		// Convert x-only pubkey to secp256k1_xonly_pubkey format
		var secp_xonly secp256k1_xonly_pubkey
		copy(secp_xonly.data[:], xonly.data[:])

		// Test existing implementation (old)
		existingResult := SchnorrVerifyOld(sig[:], msg, xonly)

		// Test new implementation (C-translated)
		newResult := SchnorrVerify(sig[:], msg, xonly)

		// Compare results
		if existingResult != newResult {
			t.Errorf("results differ: existing=%v, new=%v", existingResult, newResult)
		}

		if !existingResult {
			t.Error("signature verification failed for empty message")
		}
	})
}
