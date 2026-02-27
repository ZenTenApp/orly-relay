//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestSchnorrBatchVerify(t *testing.T) {
	// Generate test keys and signatures
	testCases := []struct {
		name  string
		count int
	}{
		{"empty batch", 0},
		{"single signature", 1},
		{"two signatures", 2},
		{"five signatures", 5},
		{"ten signatures", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			items := make([]BatchSchnorrItem, tc.count)

			for i := 0; i < tc.count; i++ {
				// Generate random key pair
				kp, err := KeyPairGenerate()
				if err != nil {
					t.Fatalf("failed to create keypair: %v", err)
				}
				defer kp.Clear()

				xonly, err := kp.XOnlyPubkey()
				if err != nil {
					t.Fatalf("failed to get x-only pubkey: %v", err)
				}

				// Generate random message
				msg := make([]byte, 32)
				rand.Read(msg)

				// Generate random aux data
				auxRand := make([]byte, 32)
				rand.Read(auxRand)

				// Sign the message
				var sig [64]byte
				if err := SchnorrSign(sig[:], msg, kp, auxRand); err != nil {
					t.Fatalf("failed to sign: %v", err)
				}

				// Verify individual signature works
				if !SchnorrVerify(sig[:], msg, xonly) {
					t.Fatalf("individual signature verification failed for signature %d", i)
				}

				items[i] = BatchSchnorrItem{
					Pubkey:    xonly,
					Message:   msg,
					Signature: sig[:],
				}
			}

			// Batch verify
			if !SchnorrBatchVerify(items) {
				t.Errorf("batch verification failed for %d valid signatures", tc.count)
			}
		})
	}
}

func TestSchnorrBatchVerifyInvalid(t *testing.T) {
	// Create a batch with one invalid signature
	items := make([]BatchSchnorrItem, 5)

	for i := 0; i < 5; i++ {
		kp, err := KeyPairGenerate()
		if err != nil {
			t.Fatalf("failed to create keypair: %v", err)
		}
		defer kp.Clear()

		xonly, err := kp.XOnlyPubkey()
		if err != nil {
			t.Fatalf("failed to get x-only pubkey: %v", err)
		}

		msg := make([]byte, 32)
		rand.Read(msg)

		auxRand := make([]byte, 32)
		rand.Read(auxRand)

		var sig [64]byte
		if err := SchnorrSign(sig[:], msg, kp, auxRand); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}

		items[i] = BatchSchnorrItem{
			Pubkey:    xonly,
			Message:   msg,
			Signature: sig[:],
		}
	}

	// Corrupt one signature
	items[2].Signature[0] ^= 0xFF

	// Batch should fail
	if SchnorrBatchVerify(items) {
		t.Error("batch verification should fail with corrupted signature")
	}

	// Test fallback
	valid, invalidIndices := SchnorrBatchVerifyWithFallback(items)
	if valid {
		t.Error("fallback should report failure")
	}
	if len(invalidIndices) != 1 || invalidIndices[0] != 2 {
		t.Errorf("expected invalid index [2], got %v", invalidIndices)
	}
}

func TestSchnorrBatchVerifyWrongMessage(t *testing.T) {
	// Create signatures where one has wrong message
	items := make([]BatchSchnorrItem, 3)

	for i := 0; i < 3; i++ {
		kp, err := KeyPairGenerate()
		if err != nil {
			t.Fatalf("failed to create keypair: %v", err)
		}
		defer kp.Clear()

		xonly, err := kp.XOnlyPubkey()
		if err != nil {
			t.Fatalf("failed to get x-only pubkey: %v", err)
		}

		msg := make([]byte, 32)
		rand.Read(msg)

		auxRand := make([]byte, 32)
		rand.Read(auxRand)

		var sig [64]byte
		if err := SchnorrSign(sig[:], msg, kp, auxRand); err != nil {
			t.Fatalf("failed to sign: %v", err)
		}

		items[i] = BatchSchnorrItem{
			Pubkey:    xonly,
			Message:   msg,
			Signature: sig[:],
		}
	}

	// Modify message for one item
	items[1].Message[0] ^= 0xFF

	// Batch should fail
	if SchnorrBatchVerify(items) {
		t.Error("batch verification should fail with wrong message")
	}
}

func BenchmarkSchnorrBatchVerify(b *testing.B) {
	// Prepare batch of signatures
	benchCounts := []int{1, 10, 100}

	for _, count := range benchCounts {
		items := make([]BatchSchnorrItem, count)

		for i := 0; i < count; i++ {
			kp, _ := KeyPairGenerate()
			defer kp.Clear()
			xonly, _ := kp.XOnlyPubkey()

			msg := make([]byte, 32)
			rand.Read(msg)

			auxRand := make([]byte, 32)
			rand.Read(auxRand)

			var sig [64]byte
			SchnorrSign(sig[:], msg, kp, auxRand)

			items[i] = BatchSchnorrItem{
				Pubkey:    xonly,
				Message:   msg,
				Signature: sig[:],
			}
		}

		b.Run(fmt.Sprintf("batch_%03d", count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				SchnorrBatchVerify(items)
			}
		})

		// Compare with individual verification
		b.Run(fmt.Sprintf("individual_%03d", count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for j := range items {
					SchnorrVerify(items[j].Signature, items[j].Message, items[j].Pubkey)
				}
			}
		})
	}
}
