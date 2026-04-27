//go:build !nocgo && !js && !wasm && !tinygo && !wasm32

package bench

import (
	"crypto/rand"
	"testing"

	"git.smesh.lol/orly/pkg/p256k1/signer"
)

// This file contains benchmarks for the P256K1Signer implementation
// (pure Go port from Bitcoin Core secp256k1)

var (
	benchSeckey            []byte
	benchMsghash           []byte
	compBenchSignerP256K1  *signer.P256K1Signer
	compBenchSignerP256K12 *signer.P256K1Signer
	compBenchSigP256K1     []byte
)

func initComparisonBenchData() {
	// Generate a fixed secret key for benchmarks
	if benchSeckey == nil {
		benchSeckey = []byte{
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		}

		// Ensure it's valid (non-zero and less than order)
		// We'll validate by trying to create a signer
		for {
			testSigner := signer.NewP256K1Signer()
			if err := testSigner.InitSec(benchSeckey); err == nil {
				break
			}
			if _, err := rand.Read(benchSeckey); err != nil {
				panic(err)
			}
		}

		// Create message hash
		benchMsghash = make([]byte, 32)
		if _, err := rand.Read(benchMsghash); err != nil {
			panic(err)
		}
	}

	// Setup P256K1Signer (this repo's implementation)
	signer1 := signer.NewP256K1Signer()
	if err := signer1.InitSec(benchSeckey); err != nil {
		panic(err)
	}
	compBenchSignerP256K1 = signer1

	var err error
	compBenchSigP256K1, err = signer1.Sign(benchMsghash)
	if err != nil {
		panic(err)
	}

	// Generate second key pair for ECDH
	seckey2 := make([]byte, 32)
	for {
		if _, err := rand.Read(seckey2); err != nil {
			panic(err)
		}
		// Validate by trying to create a signer
		testSigner := signer.NewP256K1Signer()
		if err := testSigner.InitSec(seckey2); err == nil {
			break
		}
	}

	// P256K1Signer second key pair
	signer12 := signer.NewP256K1Signer()
	if err := signer12.InitSec(seckey2); err != nil {
		panic(err)
	}
	compBenchSignerP256K12 = signer12
}

// BenchmarkPubkeyDerivation benchmarks public key derivation from private key
func BenchmarkPubkeyDerivation(b *testing.B) {
	if benchSeckey == nil {
		initComparisonBenchData()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := signer.NewP256K1Signer()
		if err := s.InitSec(benchSeckey); err != nil {
			b.Fatalf("failed to create signer: %v", err)
		}
		_ = s.Pub()
	}
}

// BenchmarkSign benchmarks Schnorr signing
func BenchmarkSign(b *testing.B) {
	if benchSeckey == nil {
		initComparisonBenchData()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if compBenchSignerP256K1 == nil {
			initComparisonBenchData()
		}
		_, err := compBenchSignerP256K1.Sign(benchMsghash)
		if err != nil {
			b.Fatalf("failed to sign: %v", err)
		}
	}
}

// BenchmarkVerify benchmarks Schnorr verification
func BenchmarkVerify(b *testing.B) {
	if benchSeckey == nil {
		initComparisonBenchData()
	}

	if compBenchSignerP256K1 == nil || compBenchSigP256K1 == nil {
		initComparisonBenchData()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := signer.NewP256K1Signer()
		if err := verifier.InitPub(compBenchSignerP256K1.Pub()); err != nil {
			b.Fatalf("failed to create verifier: %v", err)
		}
		valid, err := verifier.Verify(benchMsghash, compBenchSigP256K1)
		if err != nil {
			b.Fatalf("verification error: %v", err)
		}
		if !valid {
			b.Fatalf("verification failed")
		}
	}
}

// BenchmarkECDH benchmarks ECDH shared secret generation
func BenchmarkECDH(b *testing.B) {
	if benchSeckey == nil {
		initComparisonBenchData()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if compBenchSignerP256K1 == nil || compBenchSignerP256K12 == nil {
			initComparisonBenchData()
		}
		_, err := compBenchSignerP256K1.ECDH(compBenchSignerP256K12.Pub())
		if err != nil {
			b.Fatalf("ECDH failed: %v", err)
		}
	}
}
