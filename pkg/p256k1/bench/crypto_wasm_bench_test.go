//go:build js || wasm || tinygo || wasm32

package bench

import (
	"crypto/rand"
	"testing"

	"next.orly.dev/pkg/p256k1"
)

// WASM benchmarks for cryptographic operations using 32-bit arithmetic
// This tests both ECDSA and Schnorr on WASM targets

var (
	wasmBenchSeckey   []byte
	wasmBenchMsghash  []byte
	wasmBenchPubkey   *p256k1.PublicKey
	wasmBenchXonly    *p256k1.XOnlyPubkey
	wasmBenchKeypair  *p256k1.KeyPair
	wasmBenchECDSASig p256k1.ECDSASignature
	wasmBenchSchnorrSig [64]byte
)

func initWASMBenchData(b *testing.B) {
	if wasmBenchSeckey != nil {
		return
	}

	// Generate a valid secret key
	wasmBenchSeckey = make([]byte, 32)
	for {
		if _, err := rand.Read(wasmBenchSeckey); err != nil {
			b.Fatal(err)
		}
		// Validate by creating a keypair
		kp, err := p256k1.KeyPairCreate(wasmBenchSeckey)
		if err == nil {
			wasmBenchKeypair = kp
			break
		}
	}

	// Get public keys
	wasmBenchPubkey = wasmBenchKeypair.Pubkey()
	xonly, err := wasmBenchKeypair.XOnlyPubkey()
	if err != nil {
		b.Fatal(err)
	}
	wasmBenchXonly = xonly

	// Generate message hash
	wasmBenchMsghash = make([]byte, 32)
	if _, err := rand.Read(wasmBenchMsghash); err != nil {
		b.Fatal(err)
	}

	// Pre-compute ECDSA signature
	if err := p256k1.ECDSASign(&wasmBenchECDSASig, wasmBenchMsghash, wasmBenchSeckey); err != nil {
		b.Fatal(err)
	}

	// Pre-compute Schnorr signature
	if err := p256k1.SchnorrSign(wasmBenchSchnorrSig[:], wasmBenchMsghash, wasmBenchKeypair, nil); err != nil {
		b.Fatal(err)
	}
}

// =============================================================================
// WASM 32-bit - Schnorr Operations
// =============================================================================

func BenchmarkWASM_Schnorr_PubkeyDerivation(b *testing.B) {
	initWASMBenchData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kp, err := p256k1.KeyPairCreate(wasmBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
		_, err = kp.XOnlyPubkey()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWASM_Schnorr_Sign(b *testing.B) {
	initWASMBenchData(b)
	var sig [64]byte

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.SchnorrSign(sig[:], wasmBenchMsghash, wasmBenchKeypair, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWASM_Schnorr_Verify(b *testing.B) {
	initWASMBenchData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !p256k1.SchnorrVerify(wasmBenchSchnorrSig[:], wasmBenchMsghash, wasmBenchXonly) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// WASM 32-bit - ECDSA Operations
// =============================================================================

func BenchmarkWASM_ECDSA_PubkeyDerivation(b *testing.B) {
	initWASMBenchData(b)
	var pubkey p256k1.PublicKey

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.ECPubkeyCreate(&pubkey, wasmBenchSeckey); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWASM_ECDSA_Sign(b *testing.B) {
	initWASMBenchData(b)
	var sig p256k1.ECDSASignature

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.ECDSASign(&sig, wasmBenchMsghash, wasmBenchSeckey); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWASM_ECDSA_Verify(b *testing.B) {
	initWASMBenchData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !p256k1.ECDSAVerify(&wasmBenchECDSASig, wasmBenchMsghash, wasmBenchPubkey) {
			b.Fatal("verification failed")
		}
	}
}
