//go:build !js && !wasm && !tinygo && !wasm32

package bench

import (
	"crypto/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"next.orly.dev/pkg/p256k1"
	"next.orly.dev/pkg/p256k1/signer"
)

// This file contains comprehensive benchmarks comparing:
// 1. btcec/v2 (decred's secp256k1 implementation)
// 2. P256K1 Pure Go (AVX2 disabled)
// 3. P256K1 with ASM/BMI2 (AVX2 enabled where applicable)
// 4. libsecp256k1.so via purego (dlopen)

var (
	simdBenchSeckey  []byte
	simdBenchSeckey2 []byte
	simdBenchMsghash []byte

	// btcec
	btcecPrivKey  *btcec.PrivateKey
	btcecPrivKey2 *btcec.PrivateKey
	btcecSig      *schnorr.Signature

	// P256K1
	p256k1Signer  *signer.P256K1Signer
	p256k1Signer2 *signer.P256K1Signer
	p256k1Sig     []byte

	// libsecp256k1
	libsecp *p256k1.LibSecp256k1
)

func initSIMDBenchData() {
	if simdBenchSeckey != nil {
		return
	}

	// Generate deterministic secret key
	simdBenchSeckey = []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	}

	// Second key for ECDH
	simdBenchSeckey2 = make([]byte, 32)
	for {
		if _, err := rand.Read(simdBenchSeckey2); err != nil {
			panic(err)
		}
		// Validate
		_, err := btcec.PrivKeyFromBytes(simdBenchSeckey2)
		if err == nil {
			break
		}
	}

	// Message hash
	simdBenchMsghash = make([]byte, 32)
	if _, err := rand.Read(simdBenchMsghash); err != nil {
		panic(err)
	}

	// Initialize btcec
	btcecPrivKey, _ = btcec.PrivKeyFromBytes(simdBenchSeckey)
	btcecPrivKey2, _ = btcec.PrivKeyFromBytes(simdBenchSeckey2)
	btcecSig, _ = schnorr.Sign(btcecPrivKey, simdBenchMsghash)

	// Initialize P256K1
	p256k1Signer = signer.NewP256K1Signer()
	if err := p256k1Signer.InitSec(simdBenchSeckey); err != nil {
		panic(err)
	}
	p256k1Signer2 = signer.NewP256K1Signer()
	if err := p256k1Signer2.InitSec(simdBenchSeckey2); err != nil {
		panic(err)
	}
	p256k1Sig, _ = p256k1Signer.Sign(simdBenchMsghash)

	// Initialize libsecp256k1
	libsecp, _ = p256k1.GetLibSecp256k1()
}

// =============================================================================
// btcec/v2 Benchmarks
// =============================================================================

func BenchmarkBtcec_PubkeyDerivation(b *testing.B) {
	initSIMDBenchData()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		priv, _ := btcec.PrivKeyFromBytes(simdBenchSeckey)
		_ = priv.PubKey()
	}
}

func BenchmarkBtcec_Sign(b *testing.B) {
	initSIMDBenchData()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := schnorr.Sign(btcecPrivKey, simdBenchMsghash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBtcec_Verify(b *testing.B) {
	initSIMDBenchData()

	pubKey := btcecPrivKey.PubKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !btcecSig.Verify(simdBenchMsghash, pubKey) {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkBtcec_ECDH(b *testing.B) {
	initSIMDBenchData()

	pub2 := btcecPrivKey2.PubKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// ECDH: privKey1 * pubKey2
		x, y := btcec.S256().ScalarMult(pub2.X(), pub2.Y(), simdBenchSeckey)
		_ = x
		_ = y
	}
}

// =============================================================================
// P256K1 Pure Go Benchmarks (AVX2 disabled)
// =============================================================================

func BenchmarkP256K1PureGo_PubkeyDerivation(b *testing.B) {
	initSIMDBenchData()

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := signer.NewP256K1Signer()
		if err := s.InitSec(simdBenchSeckey); err != nil {
			b.Fatal(err)
		}
		_ = s.Pub()
	}
}

func BenchmarkP256K1PureGo_Sign(b *testing.B) {
	initSIMDBenchData()

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p256k1Signer.Sign(simdBenchMsghash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP256K1PureGo_Verify(b *testing.B) {
	initSIMDBenchData()

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := signer.NewP256K1Signer()
		if err := verifier.InitPub(p256k1Signer.Pub()); err != nil {
			b.Fatal(err)
		}
		valid, err := verifier.Verify(simdBenchMsghash, p256k1Sig)
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkP256K1PureGo_ECDH(b *testing.B) {
	initSIMDBenchData()

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p256k1Signer.ECDH(p256k1Signer2.Pub())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// P256K1 with ASM/BMI2 Benchmarks (AVX2 enabled)
// =============================================================================

func BenchmarkP256K1ASM_PubkeyDerivation(b *testing.B) {
	initSIMDBenchData()

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2/BMI2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := signer.NewP256K1Signer()
		if err := s.InitSec(simdBenchSeckey); err != nil {
			b.Fatal(err)
		}
		_ = s.Pub()
	}
}

func BenchmarkP256K1ASM_Sign(b *testing.B) {
	initSIMDBenchData()

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2/BMI2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p256k1Signer.Sign(simdBenchMsghash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkP256K1ASM_Verify(b *testing.B) {
	initSIMDBenchData()

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2/BMI2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := signer.NewP256K1Signer()
		if err := verifier.InitPub(p256k1Signer.Pub()); err != nil {
			b.Fatal(err)
		}
		valid, err := verifier.Verify(simdBenchMsghash, p256k1Sig)
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkP256K1ASM_ECDH(b *testing.B) {
	initSIMDBenchData()

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2/BMI2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p256k1Signer.ECDH(p256k1Signer2.Pub())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// libsecp256k1.so via purego (dlopen) Benchmarks
// =============================================================================

func BenchmarkLibSecp256k1_PubkeyDerivation(b *testing.B) {
	initSIMDBenchData()

	if libsecp == nil || !libsecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := libsecp.CreatePubkey(simdBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLibSecp256k1_Sign(b *testing.B) {
	initSIMDBenchData()

	if libsecp == nil || !libsecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := libsecp.SchnorrSign(simdBenchMsghash, simdBenchSeckey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLibSecp256k1_Verify(b *testing.B) {
	initSIMDBenchData()

	if libsecp == nil || !libsecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	sig, err := libsecp.SchnorrSign(simdBenchMsghash, simdBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}

	pubkey, err := libsecp.CreatePubkey(simdBenchSeckey)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !libsecp.SchnorrVerify(sig, simdBenchMsghash, pubkey) {
			b.Fatal("verification failed")
		}
	}
}

