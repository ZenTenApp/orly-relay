//go:build !nocgo && !js && !wasm && !tinygo && !wasm32

package bench

import (
	"crypto/rand"
	"testing"

	"git.smesh.lol/orly/pkg/p256k1"
	"git.smesh.lol/orly/pkg/p256k1/signer"
)

// This file contains benchmarks comparing:
// 1. P256K1 Pure Go implementation
// 2. P256K1 with AVX2 scalar operations (where applicable)
// 3. libsecp256k1.so via purego (if available)

var (
	avxBenchSeckey    []byte
	avxBenchMsghash   []byte
	avxBenchSigner    *signer.P256K1Signer
	avxBenchSigner2   *signer.P256K1Signer
	avxBenchSig       []byte
	avxBenchLibSecp   *p256k1.LibSecp256k1
)

func initAVXBenchData() {
	if avxBenchSeckey == nil {
		avxBenchSeckey = []byte{
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		}

		for {
			testSigner := signer.NewP256K1Signer()
			if err := testSigner.InitSec(avxBenchSeckey); err == nil {
				break
			}
			if _, err := rand.Read(avxBenchSeckey); err != nil {
				panic(err)
			}
		}

		avxBenchMsghash = make([]byte, 32)
		if _, err := rand.Read(avxBenchMsghash); err != nil {
			panic(err)
		}
	}

	// Setup P256K1Signer
	s := signer.NewP256K1Signer()
	if err := s.InitSec(avxBenchSeckey); err != nil {
		panic(err)
	}
	avxBenchSigner = s

	var err error
	avxBenchSig, err = s.Sign(avxBenchMsghash)
	if err != nil {
		panic(err)
	}

	// Generate second key pair for ECDH
	seckey2 := make([]byte, 32)
	for {
		if _, err := rand.Read(seckey2); err != nil {
			panic(err)
		}
		testSigner := signer.NewP256K1Signer()
		if err := testSigner.InitSec(seckey2); err == nil {
			break
		}
	}

	s2 := signer.NewP256K1Signer()
	if err := s2.InitSec(seckey2); err != nil {
		panic(err)
	}
	avxBenchSigner2 = s2

	// Try to load libsecp256k1
	avxBenchLibSecp, _ = p256k1.GetLibSecp256k1()
}

// Pure Go benchmarks (AVX2 disabled)
func BenchmarkPureGo_PubkeyDerivation(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := signer.NewP256K1Signer()
		if err := s.InitSec(avxBenchSeckey); err != nil {
			b.Fatalf("failed to create signer: %v", err)
		}
		_ = s.Pub()
	}
}

func BenchmarkPureGo_Sign(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchSigner.Sign(avxBenchMsghash)
		if err != nil {
			b.Fatalf("failed to sign: %v", err)
		}
	}
}

func BenchmarkPureGo_Verify(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := signer.NewP256K1Signer()
		if err := verifier.InitPub(avxBenchSigner.Pub()); err != nil {
			b.Fatalf("failed to create verifier: %v", err)
		}
		valid, err := verifier.Verify(avxBenchMsghash, avxBenchSig)
		if err != nil {
			b.Fatalf("verification error: %v", err)
		}
		if !valid {
			b.Fatalf("verification failed")
		}
	}
}

func BenchmarkPureGo_ECDH(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	p256k1.SetAVX2Enabled(false)
	defer p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchSigner.ECDH(avxBenchSigner2.Pub())
		if err != nil {
			b.Fatalf("ECDH failed: %v", err)
		}
	}
}

// AVX2-enabled benchmarks
func BenchmarkAVX2_PubkeyDerivation(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := signer.NewP256K1Signer()
		if err := s.InitSec(avxBenchSeckey); err != nil {
			b.Fatalf("failed to create signer: %v", err)
		}
		_ = s.Pub()
	}
}

func BenchmarkAVX2_Sign(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchSigner.Sign(avxBenchMsghash)
		if err != nil {
			b.Fatalf("failed to sign: %v", err)
		}
	}
}

func BenchmarkAVX2_Verify(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := signer.NewP256K1Signer()
		if err := verifier.InitPub(avxBenchSigner.Pub()); err != nil {
			b.Fatalf("failed to create verifier: %v", err)
		}
		valid, err := verifier.Verify(avxBenchMsghash, avxBenchSig)
		if err != nil {
			b.Fatalf("verification error: %v", err)
		}
		if !valid {
			b.Fatalf("verification failed")
		}
	}
}

func BenchmarkAVX2_ECDH(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if !p256k1.HasAVX2CPU() {
		b.Skip("AVX2 not available")
	}

	p256k1.SetAVX2Enabled(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchSigner.ECDH(avxBenchSigner2.Pub())
		if err != nil {
			b.Fatalf("ECDH failed: %v", err)
		}
	}
}

// libsecp256k1.so benchmarks via purego
func BenchmarkLibSecp_Sign(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if avxBenchLibSecp == nil || !avxBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchLibSecp.SchnorrSign(avxBenchMsghash, avxBenchSeckey)
		if err != nil {
			b.Fatalf("signing failed: %v", err)
		}
	}
}

func BenchmarkLibSecp_PubkeyDerivation(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if avxBenchLibSecp == nil || !avxBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := avxBenchLibSecp.CreatePubkey(avxBenchSeckey)
		if err != nil {
			b.Fatalf("pubkey creation failed: %v", err)
		}
	}
}

func BenchmarkLibSecp_Verify(b *testing.B) {
	if avxBenchSeckey == nil {
		initAVXBenchData()
	}

	if avxBenchLibSecp == nil || !avxBenchLibSecp.IsLoaded() {
		b.Skip("libsecp256k1.so not available")
	}

	// Sign with libsecp to get compatible signature
	sig, err := avxBenchLibSecp.SchnorrSign(avxBenchMsghash, avxBenchSeckey)
	if err != nil {
		b.Fatalf("signing failed: %v", err)
	}

	pubkey, err := avxBenchLibSecp.CreatePubkey(avxBenchSeckey)
	if err != nil {
		b.Fatalf("pubkey creation failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !avxBenchLibSecp.SchnorrVerify(sig, avxBenchMsghash, pubkey) {
			b.Fatalf("verification failed")
		}
	}
}
