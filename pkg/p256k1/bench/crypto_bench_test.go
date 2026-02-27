//go:build !js && !wasm && !tinygo && !wasm32

package bench

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"p256k1.mleku.dev"
)

// Benchmark comparing pure Go implementation vs libsecp256k1 C library
// for ECDSA and Schnorr operations on AMD64

var (
	cryptoBenchSeckey  []byte
	cryptoBenchMsghash []byte
	cryptoBenchPubkey  *p256k1.PublicKey
	cryptoBenchXonly   *p256k1.XOnlyPubkey
	cryptoBenchKeypair *p256k1.KeyPair

	// Pre-computed signatures
	cryptoBenchSchnorrSig [64]byte
	cryptoBenchECDSASig   p256k1.ECDSASignature

	// libsecp256k1
	cryptoBenchLibSecp *p256k1.LibSecp256k1
)

func initCryptoBenchData(b *testing.B) {
	if cryptoBenchSeckey != nil {
		return
	}

	// Generate a valid secret key
	cryptoBenchSeckey = make([]byte, 32)
	for {
		if _, err := rand.Read(cryptoBenchSeckey); err != nil {
			b.Fatal(err)
		}
		// Validate by creating a keypair
		kp, err := p256k1.KeyPairCreate(cryptoBenchSeckey)
		if err == nil {
			cryptoBenchKeypair = kp
			break
		}
	}

	// Get public keys
	cryptoBenchPubkey = cryptoBenchKeypair.Pubkey()
	xonly, err := cryptoBenchKeypair.XOnlyPubkey()
	if err != nil {
		b.Fatal(err)
	}
	cryptoBenchXonly = xonly

	// Generate message hash
	cryptoBenchMsghash = make([]byte, 32)
	if _, err := rand.Read(cryptoBenchMsghash); err != nil {
		b.Fatal(err)
	}

	// Pre-compute Schnorr signature
	if err := p256k1.SchnorrSign(cryptoBenchSchnorrSig[:], cryptoBenchMsghash, cryptoBenchKeypair, nil); err != nil {
		b.Fatal(err)
	}

	// Pre-compute ECDSA signature
	if err := p256k1.ECDSASign(&cryptoBenchECDSASig, cryptoBenchMsghash, cryptoBenchSeckey); err != nil {
		b.Fatal(err)
	}

	// Try to load libsecp256k1
	cryptoBenchLibSecp, _ = p256k1.GetLibSecp256k1()
}

// =============================================================================
// Pure Go - Schnorr
// =============================================================================

func BenchmarkPureGo_Schnorr_PubkeyDerivation(b *testing.B) {
	initCryptoBenchData(b)
	var sig [64]byte
	_ = sig

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kp, err := p256k1.KeyPairCreate(cryptoBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
		_, err = kp.XOnlyPubkey()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPureGo_Schnorr_Sign(b *testing.B) {
	initCryptoBenchData(b)
	var sig [64]byte

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.SchnorrSign(sig[:], cryptoBenchMsghash, cryptoBenchKeypair, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPureGo_Schnorr_Verify(b *testing.B) {
	initCryptoBenchData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !p256k1.SchnorrVerify(cryptoBenchSchnorrSig[:], cryptoBenchMsghash, cryptoBenchXonly) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// Pure Go - ECDSA
// =============================================================================

func BenchmarkPureGo_ECDSA_PubkeyDerivation(b *testing.B) {
	initCryptoBenchData(b)
	var pubkey p256k1.PublicKey

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.ECPubkeyCreate(&pubkey, cryptoBenchSeckey); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPureGo_ECDSA_Sign(b *testing.B) {
	initCryptoBenchData(b)
	var sig p256k1.ECDSASignature

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := p256k1.ECDSASign(&sig, cryptoBenchMsghash, cryptoBenchSeckey); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPureGo_ECDSA_Verify(b *testing.B) {
	initCryptoBenchData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !p256k1.ECDSAVerify(&cryptoBenchECDSASig, cryptoBenchMsghash, cryptoBenchPubkey) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// libsecp256k1 (C library via purego) - Schnorr
// =============================================================================

func BenchmarkLibSecp_Schnorr_PubkeyDerivation(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	// libsecp256k1 derives x-only pubkey via compressed pubkey
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pubkey, _, err := cryptoBenchLibSecp.CreatePubkeyCompressed(cryptoBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
		// x-only is just the x coordinate (bytes 1-32 of compressed)
		_ = pubkey[1:33]
	}
}

func BenchmarkLibSecp_Schnorr_Sign(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cryptoBenchLibSecp.SchnorrSign(cryptoBenchMsghash, cryptoBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLibSecp_Schnorr_Verify(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	// Get signature from libsecp for fair comparison
	sig, err := cryptoBenchLibSecp.SchnorrSign(cryptoBenchMsghash, cryptoBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}
	// Get x-only pubkey (first 32 bytes after prefix of compressed pubkey)
	compressedPubkey, _, err := cryptoBenchLibSecp.CreatePubkeyCompressed(cryptoBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}
	pubkey := compressedPubkey[1:33] // x-only is just x coordinate

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !cryptoBenchLibSecp.SchnorrVerify(sig, cryptoBenchMsghash, pubkey) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// libsecp256k1 (C library via purego) - ECDSA
// =============================================================================

func BenchmarkLibSecp_ECDSA_PubkeyDerivation(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cryptoBenchLibSecp.CreatePubkey(cryptoBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLibSecp_ECDSA_Sign(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cryptoBenchLibSecp.ECDSASign(cryptoBenchMsghash, cryptoBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLibSecp_ECDSA_Verify(b *testing.B) {
	initCryptoBenchData(b)

	if cryptoBenchLibSecp == nil || !cryptoBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	// Get signature and pubkey from libsecp for fair comparison
	sig, err := cryptoBenchLibSecp.ECDSASign(cryptoBenchMsghash, cryptoBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}
	// ECDSA needs compressed pubkey (33 bytes), not x-only
	pubkey, _, err := cryptoBenchLibSecp.CreatePubkeyCompressed(cryptoBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !cryptoBenchLibSecp.ECDSAVerify(sig, cryptoBenchMsghash, pubkey) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// btcec (decred's secp256k1 in pure Go) - Schnorr
// =============================================================================

var (
	cryptoBtcecPrivKey    *btcec.PrivateKey
	cryptoBtcecPubKey     *btcec.PublicKey
	cryptoBtcecSchnorrSig *schnorr.Signature
	cryptoBtcecECDSASig   *ecdsa.Signature
)

func initBtcecData(b *testing.B) {
	if cryptoBtcecPrivKey != nil {
		return
	}
	initCryptoBenchData(b)

	var err error
	cryptoBtcecPrivKey, cryptoBtcecPubKey = btcec.PrivKeyFromBytes(cryptoBenchSeckey)
	if cryptoBtcecPrivKey == nil {
		b.Fatal("failed to create btcec private key")
	}

	cryptoBtcecSchnorrSig, err = schnorr.Sign(cryptoBtcecPrivKey, cryptoBenchMsghash)
	if err != nil {
		b.Fatal(err)
	}

	cryptoBtcecECDSASig = ecdsa.Sign(cryptoBtcecPrivKey, cryptoBenchMsghash)
}

func BenchmarkBtcec_Schnorr_PubkeyDerivation(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		priv, _ := btcec.PrivKeyFromBytes(cryptoBenchSeckey)
		_ = priv.PubKey()
	}
}

func BenchmarkBtcec_Schnorr_Sign(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := schnorr.Sign(cryptoBtcecPrivKey, cryptoBenchMsghash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBtcec_Schnorr_Verify(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !cryptoBtcecSchnorrSig.Verify(cryptoBenchMsghash, cryptoBtcecPubKey) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// btcec (decred's secp256k1 in pure Go) - ECDSA
// =============================================================================

func BenchmarkBtcec_ECDSA_PubkeyDerivation(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		priv, _ := btcec.PrivKeyFromBytes(cryptoBenchSeckey)
		_ = priv.PubKey()
	}
}

func BenchmarkBtcec_ECDSA_Sign(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ecdsa.Sign(cryptoBtcecPrivKey, cryptoBenchMsghash)
	}
}

func BenchmarkBtcec_ECDSA_Verify(b *testing.B) {
	initBtcecData(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !cryptoBtcecECDSASig.Verify(cryptoBenchMsghash, cryptoBtcecPubKey) {
			b.Fatal("verification failed")
		}
	}
}

// =============================================================================
// Summary benchmark that prints comparison table
// =============================================================================

func BenchmarkCryptoComparison(b *testing.B) {
	initCryptoBenchData(b)

	hasLibSecp := cryptoBenchLibSecp != nil && cryptoBenchLibSecp.IsLoaded()

	fmt.Println("\n=== Cryptographic Operations Benchmark ===")
	fmt.Printf("Platform: AMD64, libsecp256k1 available: %v\n\n", hasLibSecp)

	// This is a meta-benchmark that just prints info
	b.Skip("Run individual benchmarks with: go test -bench='PureGo|LibSecp|Btcec' -benchtime=1s")
}
