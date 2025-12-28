package bdhke

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// Test vectors from Cashu NUT-00 specification
// https://github.com/cashubtc/nuts/blob/main/00.md

func TestHashToCurve(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string // Expected compressed public key in hex
	}{
		{
			name:     "test vector 1",
			message:  "0000000000000000000000000000000000000000000000000000000000000000",
			expected: "024cce997d3b518f739663b757deaec95bcd9473c30a14ac2fd04023a739d1a725",
		},
		{
			name:     "test vector 2",
			message:  "0000000000000000000000000000000000000000000000000000000000000001",
			expected: "022e7158e11c9506f1aa4248bf531298daa7febd6194f003edcd9b93ade6253acf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgBytes, err := hex.DecodeString(tt.message)
			if err != nil {
				t.Fatalf("failed to decode message: %v", err)
			}

			point, err := HashToCurve(msgBytes)
			if err != nil {
				t.Fatalf("HashToCurve failed: %v", err)
			}

			got := hex.EncodeToString(point.SerializeCompressed())
			if got != tt.expected {
				t.Errorf("HashToCurve(%s) = %s, want %s", tt.message, got, tt.expected)
			}
		})
	}
}

func TestBlindSignUnblindVerify(t *testing.T) {
	// Generate mint keypair
	k, K, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	// Generate a secret
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	// User blinds the secret
	blindResult, err := Blind(secret)
	if err != nil {
		t.Fatalf("Blind failed: %v", err)
	}

	// Mint signs the blinded message
	C_, err := Sign(blindResult.B, k)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// User unblinds the signature
	C, err := Unblind(C_, blindResult.R, K)
	if err != nil {
		t.Fatalf("Unblind failed: %v", err)
	}

	// Verify the token
	valid, err := Verify(secret, C, k)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !valid {
		t.Error("Verify returned false, expected true")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	k, K, _ := GenerateKeypair()
	secret1, _ := GenerateSecret()
	secret2, _ := GenerateSecret()

	// Create token with secret1
	blindResult, _ := Blind(secret1)
	C_, _ := Sign(blindResult.B, k)
	C, _ := Unblind(C_, blindResult.R, K)

	// Try to verify with secret2
	valid, err := Verify(secret2, C, k)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if valid {
		t.Error("Verify returned true for wrong secret")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	k1, K1, _ := GenerateKeypair()
	k2, _, _ := GenerateKeypair()

	secret, _ := GenerateSecret()

	// Create token with k1
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k1)
	C, _ := Unblind(C_, blindResult.R, K1)

	// Try to verify with k2
	valid, err := Verify(secret, C, k2)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if valid {
		t.Error("Verify returned true for wrong key")
	}
}

func TestBlindWithFactor(t *testing.T) {
	k, K, _ := GenerateKeypair()
	secret := []byte("test secret message")

	// Use deterministic blinding factor
	rBytes := make([]byte, 32)
	for i := range rBytes {
		rBytes[i] = byte(i)
	}

	blindResult, err := BlindWithFactor(secret, rBytes)
	if err != nil {
		t.Fatalf("BlindWithFactor failed: %v", err)
	}

	// Complete the protocol
	C_, _ := Sign(blindResult.B, k)
	C, _ := Unblind(C_, blindResult.R, K)

	valid, _ := Verify(secret, C, k)
	if !valid {
		t.Error("BlindWithFactor: verification failed")
	}

	// Do it again with same factor - should get same B
	blindResult2, _ := BlindWithFactor(secret, rBytes)
	if !bytes.Equal(blindResult.B.SerializeCompressed(), blindResult2.B.SerializeCompressed()) {
		t.Error("BlindWithFactor not deterministic")
	}
}

func TestHashToCurveDeterministic(t *testing.T) {
	message := []byte("deterministic test")

	p1, err := HashToCurve(message)
	if err != nil {
		t.Fatalf("HashToCurve failed: %v", err)
	}

	p2, err := HashToCurve(message)
	if err != nil {
		t.Fatalf("HashToCurve failed: %v", err)
	}

	if !p1.IsEqual(p2) {
		t.Error("HashToCurve not deterministic")
	}
}

func TestSignNilInputs(t *testing.T) {
	k, _, _ := GenerateKeypair()

	_, err := Sign(nil, k)
	if err == nil {
		t.Error("Sign(nil, k) should error")
	}

	B, _ := HashToCurve([]byte("test"))
	_, err = Sign(B, nil)
	if err == nil {
		t.Error("Sign(B, nil) should error")
	}
}

func TestUnblindNilInputs(t *testing.T) {
	k, K, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k)

	_, err := Unblind(nil, blindResult.R, K)
	if err == nil {
		t.Error("Unblind(nil, r, K) should error")
	}

	_, err = Unblind(C_, nil, K)
	if err == nil {
		t.Error("Unblind(C_, nil, K) should error")
	}

	_, err = Unblind(C_, blindResult.R, nil)
	if err == nil {
		t.Error("Unblind(C_, r, nil) should error")
	}
}

func TestVerifyNilInputs(t *testing.T) {
	k, K, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k)
	C, _ := Unblind(C_, blindResult.R, K)

	_, err := Verify(secret, nil, k)
	if err == nil {
		t.Error("Verify(secret, nil, k) should error")
	}

	_, err = Verify(secret, C, nil)
	if err == nil {
		t.Error("Verify(secret, C, nil) should error")
	}
}

// Benchmark functions
func BenchmarkHashToCurve(b *testing.B) {
	secret, _ := GenerateSecret()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HashToCurve(secret)
	}
}

func BenchmarkBlind(b *testing.B) {
	secret, _ := GenerateSecret()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Blind(secret)
	}
}

func BenchmarkSign(b *testing.B) {
	k, _, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Sign(blindResult.B, k)
	}
}

func BenchmarkUnblind(b *testing.B) {
	k, K, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Unblind(C_, blindResult.R, K)
	}
}

func BenchmarkVerify(b *testing.B) {
	k, K, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k)
	C, _ := Unblind(C_, blindResult.R, K)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Verify(secret, C, k)
	}
}

func BenchmarkFullProtocol(b *testing.B) {
	k, K, _ := GenerateKeypair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secret, _ := GenerateSecret()
		blindResult, _ := Blind(secret)
		C_, _ := Sign(blindResult.B, k)
		C, _ := Unblind(C_, blindResult.R, K)
		Verify(secret, C, k)
	}
}

// Test that serialization/deserialization works correctly
func TestPointSerialization(t *testing.T) {
	k, K, _ := GenerateKeypair()
	secret, _ := GenerateSecret()
	blindResult, _ := Blind(secret)
	C_, _ := Sign(blindResult.B, k)
	C, _ := Unblind(C_, blindResult.R, K)

	// Serialize and deserialize C
	serialized := C.SerializeCompressed()
	deserialized, err := secp256k1.ParsePubKey(serialized)
	if err != nil {
		t.Fatalf("failed to parse serialized point: %v", err)
	}

	// Verify with deserialized point
	valid, err := Verify(secret, deserialized, k)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !valid {
		t.Error("Verify failed after point serialization round-trip")
	}

	// Same for K
	kSerialized := K.SerializeCompressed()
	kDeserialized, err := secp256k1.ParsePubKey(kSerialized)
	if err != nil {
		t.Fatalf("failed to parse serialized K: %v", err)
	}

	// Unblind with deserialized K
	C2, err := Unblind(C_, blindResult.R, kDeserialized)
	if err != nil {
		t.Fatalf("Unblind with deserialized K failed: %v", err)
	}
	if !C.IsEqual(C2) {
		t.Error("Unblind result differs after K round-trip")
	}
}
