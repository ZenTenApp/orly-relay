//go:build !js

package p256k1

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestExternalValidationGeneratorMult validates k*G against libsecp256k1
// This is the primary test for verifying our scalar multiplication is correct.
func TestExternalValidationGeneratorMult(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	testCases := []struct {
		name   string
		scalar string
	}{
		{"one", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"two", "0000000000000000000000000000000000000000000000000000000000000002"},
		{"three", "0000000000000000000000000000000000000000000000000000000000000003"},
		{"seven", "0000000000000000000000000000000000000000000000000000000000000007"},
		{"small", "0000000000000000000000000000000000000000000000000000000000000100"},
		{"random1", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"random2", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
		{"half_order", "7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0"},
		// Edge cases for GLV splitting
		{"lambda_minus_one", "5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd71"},
		{"near_lambda", "5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd73"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seckey, _ := hex.DecodeString(tc.scalar)

			// Get expected result from libsecp256k1
			expectedPubkey, err := lib.CreatePubkey(seckey)
			if err != nil {
				t.Fatalf("libsecp256k1.CreatePubkey failed: %v", err)
			}

			// Parse scalar
			var s Scalar
			s.setB32(seckey)

			if s.isZero() {
				t.Skip("zero scalar")
			}

			// Test EcmultConst (reference binary method)
			t.Run("EcmultConst", func(t *testing.T) {
				var result GroupElementJacobian
				EcmultConst(&result, &Generator, &s)
				var aff GroupElementAffine
				aff.setGEJ(&result)
				aff.x.normalize()

				var gotX [32]byte
				aff.x.getB32(gotX[:])

				if hex.EncodeToString(gotX[:]) != hex.EncodeToString(expectedPubkey) {
					t.Errorf("EcmultConst mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotX[:]), hex.EncodeToString(expectedPubkey))
				}
			})

			// Test ecmultGenGLV (GLV-optimized generator multiplication)
			t.Run("ecmultGenGLV", func(t *testing.T) {
				var result GroupElementJacobian
				ecmultGenGLV(&result, &s)
				var aff GroupElementAffine
				aff.setGEJ(&result)
				aff.x.normalize()

				var gotX [32]byte
				aff.x.getB32(gotX[:])

				if hex.EncodeToString(gotX[:]) != hex.EncodeToString(expectedPubkey) {
					t.Errorf("ecmultGenGLV mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotX[:]), hex.EncodeToString(expectedPubkey))
				}
			})

			// Test ecmultStraussWNAFGLV with Generator
			t.Run("ecmultStraussWNAFGLV", func(t *testing.T) {
				var result GroupElementJacobian
				ecmultStraussWNAFGLV(&result, &Generator, &s)
				var aff GroupElementAffine
				aff.setGEJ(&result)
				aff.x.normalize()

				var gotX [32]byte
				aff.x.getB32(gotX[:])

				if hex.EncodeToString(gotX[:]) != hex.EncodeToString(expectedPubkey) {
					t.Errorf("ecmultStraussWNAFGLV mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotX[:]), hex.EncodeToString(expectedPubkey))
				}
			})

			// Test ecmultWindowedVar
			t.Run("ecmultWindowedVar", func(t *testing.T) {
				var result GroupElementJacobian
				ecmultWindowedVar(&result, &Generator, &s)
				var aff GroupElementAffine
				aff.setGEJ(&result)
				aff.x.normalize()

				var gotX [32]byte
				aff.x.getB32(gotX[:])

				if hex.EncodeToString(gotX[:]) != hex.EncodeToString(expectedPubkey) {
					t.Errorf("ecmultWindowedVar mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotX[:]), hex.EncodeToString(expectedPubkey))
				}
			})
		})
	}
}

// TestExternalValidationECDH validates arbitrary point multiplication via ECDH
// libsecp256k1's ECDH computes hash(k*P), so if our k*P is correct, the hash should match.
func TestExternalValidationECDH(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	// Test with different secret keys and public key points
	testCases := []struct {
		name      string
		seckey    string
		pubkeyGen string // scalar to generate the pubkey (pubkey = pubkeyGen * G)
	}{
		{"simple", "0000000000000000000000000000000000000000000000000000000000000002",
			"0000000000000000000000000000000000000000000000000000000000000003"},
		{"random", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
			"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seckey, _ := hex.DecodeString(tc.seckey)
			pubkeyGenBytes, _ := hex.DecodeString(tc.pubkeyGen)

			// Generate the public key point using libsecp256k1
			// First get the compressed pubkey for pubkeyGen * G
			var pubkeyGenScalar Scalar
			pubkeyGenScalar.setB32(pubkeyGenBytes)

			// Compute pubkey = pubkeyGen * G using our reference implementation
			var pubkeyJac GroupElementJacobian
			EcmultConst(&pubkeyJac, &Generator, &pubkeyGenScalar)
			var pubkeyAff GroupElementAffine
			pubkeyAff.setGEJ(&pubkeyJac)
			pubkeyAff.x.normalize()
			pubkeyAff.y.normalize()

			// Serialize as compressed pubkey (33 bytes)
			pubkey33 := make([]byte, 33)
			var xBytes [32]byte
			pubkeyAff.x.getB32(xBytes[:])
			copy(pubkey33[1:], xBytes[:])

			// Determine prefix based on Y coordinate parity
			var yBytes [32]byte
			pubkeyAff.y.getB32(yBytes[:])
			if yBytes[31]&1 == 0 {
				pubkey33[0] = 0x02 // even Y
			} else {
				pubkey33[0] = 0x03 // odd Y
			}

			// Get expected ECDH result from libsecp256k1
			expectedECDH, err := lib.ECDH(seckey, pubkey33)
			if err != nil {
				t.Fatalf("libsecp256k1.ECDH failed: %v", err)
			}

			// Now compute ECDH using our implementation
			// ECDH = sha256(seckey * pubkey)
			var seckeyScalar Scalar
			seckeyScalar.setB32(seckey)

			// Helper to compute ECDH hash matching libsecp256k1's default
			// libsecp256k1 hashes sha256(0x02/0x03 || x) - the compressed point
			computeECDH := func(result *GroupElementJacobian) [32]byte {
				var resultAff GroupElementAffine
				resultAff.setGEJ(result)
				resultAff.x.normalize()
				resultAff.y.normalize()

				// Serialize as compressed point (33 bytes)
				var compressed [33]byte
				var xBytes [32]byte
				resultAff.x.getB32(xBytes[:])
				copy(compressed[1:], xBytes[:])

				// Determine prefix based on Y parity
				var yBytes [32]byte
				resultAff.y.getB32(yBytes[:])
				if yBytes[31]&1 == 0 {
					compressed[0] = 0x02 // even Y
				} else {
					compressed[0] = 0x03 // odd Y
				}

				return sha256.Sum256(compressed[:])
			}

			// Test using EcmultConst (reference)
			t.Run("EcmultConst", func(t *testing.T) {
				var result GroupElementJacobian
				EcmultConst(&result, &pubkeyAff, &seckeyScalar)
				gotECDH := computeECDH(&result)

				if hex.EncodeToString(gotECDH[:]) != hex.EncodeToString(expectedECDH) {
					t.Errorf("ECDH mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotECDH[:]), hex.EncodeToString(expectedECDH))
				}
			})

			// Test using ecmultStraussWNAFGLV
			t.Run("ecmultStraussWNAFGLV", func(t *testing.T) {
				var result GroupElementJacobian
				ecmultStraussWNAFGLV(&result, &pubkeyAff, &seckeyScalar)
				gotECDH := computeECDH(&result)

				if hex.EncodeToString(gotECDH[:]) != hex.EncodeToString(expectedECDH) {
					t.Errorf("ECDH mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotECDH[:]), hex.EncodeToString(expectedECDH))
				}
			})

			// Test using ecmultWindowedVar
			t.Run("ecmultWindowedVar", func(t *testing.T) {
				var result GroupElementJacobian
				ecmultWindowedVar(&result, &pubkeyAff, &seckeyScalar)
				gotECDH := computeECDH(&result)

				if hex.EncodeToString(gotECDH[:]) != hex.EncodeToString(expectedECDH) {
					t.Errorf("ECDH mismatch:\n  got:  %s\n  want: %s",
						hex.EncodeToString(gotECDH[:]), hex.EncodeToString(expectedECDH))
				}
			})
		})
	}
}

// TestExternalValidationRandomScalars tests with pseudo-random scalars
func TestExternalValidationRandomScalars(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	seed := uint64(0xdeadbeef12345678)

	for i := 0; i < 100; i++ {
		// Generate pseudo-random scalar
		var seckey [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			seckey[j] = byte(seed >> 56)
		}

		// Get expected from libsecp256k1
		expectedPubkey, err := lib.CreatePubkey(seckey[:])
		if err != nil {
			continue // Skip invalid keys
		}

		var s Scalar
		s.setB32(seckey[:])

		if s.isZero() {
			continue
		}

		// Test EcmultConst
		var refResult GroupElementJacobian
		EcmultConst(&refResult, &Generator, &s)
		var refAff GroupElementAffine
		refAff.setGEJ(&refResult)
		refAff.x.normalize()

		var refX [32]byte
		refAff.x.getB32(refX[:])

		if hex.EncodeToString(refX[:]) != hex.EncodeToString(expectedPubkey) {
			t.Errorf("Random test %d: EcmultConst mismatch:\n  scalar: %s\n  got:    %s\n  want:   %s",
				i, hex.EncodeToString(seckey[:]),
				hex.EncodeToString(refX[:]), hex.EncodeToString(expectedPubkey))
		}

		// Test ecmultStraussWNAFGLV
		var straussResult GroupElementJacobian
		ecmultStraussWNAFGLV(&straussResult, &Generator, &s)
		var straussAff GroupElementAffine
		straussAff.setGEJ(&straussResult)
		straussAff.x.normalize()

		var straussX [32]byte
		straussAff.x.getB32(straussX[:])

		if hex.EncodeToString(straussX[:]) != hex.EncodeToString(expectedPubkey) {
			t.Errorf("Random test %d: ecmultStraussWNAFGLV mismatch:\n  scalar: %s\n  got:    %s\n  want:   %s",
				i, hex.EncodeToString(seckey[:]),
				hex.EncodeToString(straussX[:]), hex.EncodeToString(expectedPubkey))
		}

		// Test ecmultGenGLV
		var glvResult GroupElementJacobian
		ecmultGenGLV(&glvResult, &s)
		var glvAff GroupElementAffine
		glvAff.setGEJ(&glvResult)
		glvAff.x.normalize()

		var glvX [32]byte
		glvAff.x.getB32(glvX[:])

		if hex.EncodeToString(glvX[:]) != hex.EncodeToString(expectedPubkey) {
			t.Errorf("Random test %d: ecmultGenGLV mismatch:\n  scalar: %s\n  got:    %s\n  want:   %s",
				i, hex.EncodeToString(seckey[:]),
				hex.EncodeToString(glvX[:]), hex.EncodeToString(expectedPubkey))
		}
	}
}

// TestExternalValidationGLVSplit validates the GLV scalar splitting specifically
// We can verify the split is correct by checking that k1 + k2*λ ≡ k (mod n)
// AND that k1*G + k2*(λ*G) = k*G (via libsecp256k1)
func TestExternalValidationGLVSplit(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	testCases := []struct {
		name   string
		scalar string
	}{
		{"one", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"random1", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"near_order", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413e"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kBytes, _ := hex.DecodeString(tc.scalar)

			var k Scalar
			k.setB32(kBytes)

			if k.isZero() {
				t.Skip("zero scalar")
			}

			// Get expected k*G from libsecp256k1
			expectedPubkey, err := lib.CreatePubkey(kBytes)
			if err != nil {
				t.Fatalf("libsecp256k1.CreatePubkey failed: %v", err)
			}

			// Split k into k1, k2
			var k1, k2 Scalar
			scalarSplitLambda(&k1, &k2, &k)

			// Log split values for debugging
			var k1Bytes, k2Bytes [32]byte
			k1.getB32(k1Bytes[:])
			k2.getB32(k2Bytes[:])
			t.Logf("k  = %s", hex.EncodeToString(kBytes))
			t.Logf("k1 = %s", hex.EncodeToString(k1Bytes[:]))
			t.Logf("k2 = %s", hex.EncodeToString(k2Bytes[:]))

			// Verify algebraically: k1 + k2*λ ≡ k (mod n)
			var k2Lambda Scalar
			k2Lambda.mul(&k2, &scalarLambda)

			var reconstructed Scalar
			reconstructed.add(&k1, &k2Lambda)

			if !reconstructed.equal(&k) {
				var recBytes [32]byte
				reconstructed.getB32(recBytes[:])
				t.Errorf("Scalar split reconstruction failed:\n  k1 + k2*λ = %s\n  k         = %s",
					hex.EncodeToString(recBytes[:]), hex.EncodeToString(kBytes))
			}

			// Verify geometrically: k1*G + k2*(λ*G) = k*G
			// Compute k1*G
			var k1G GroupElementJacobian
			EcmultConst(&k1G, &Generator, &k1)

			// Compute λ*G (using the endomorphism: (β*Gx, Gy))
			var lambdaG GroupElementAffine
			lambdaG.mulLambda(&Generator)

			// Compute k2*(λ*G)
			var k2LambdaG GroupElementJacobian
			EcmultConst(&k2LambdaG, &lambdaG, &k2)

			// Add them
			var sum GroupElementJacobian
			sum.addVar(&k1G, &k2LambdaG)

			var sumAff GroupElementAffine
			sumAff.setGEJ(&sum)
			sumAff.x.normalize()

			var sumX [32]byte
			sumAff.x.getB32(sumX[:])

			if hex.EncodeToString(sumX[:]) != hex.EncodeToString(expectedPubkey) {
				t.Errorf("Geometric verification failed:\n  k1*G + k2*(λ*G) = %s\n  k*G (expected)  = %s",
					hex.EncodeToString(sumX[:]), hex.EncodeToString(expectedPubkey))
			}
		})
	}
}

// TestExternalValidationSchnorr validates our Schnorr implementation against libsecp256k1
func TestExternalValidationSchnorr(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	// Test that signatures created by libsecp256k1 can be verified by our code
	// and vice versa
	testCases := []struct {
		name   string
		seckey string
		msg    string
	}{
		{"simple", "0000000000000000000000000000000000000000000000000000000000000001",
			"0000000000000000000000000000000000000000000000000000000000000000"},
		{"random", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
			"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seckey, _ := hex.DecodeString(tc.seckey)
			msg, _ := hex.DecodeString(tc.msg)

			// Get pubkey from libsecp256k1
			pubkey, err := lib.CreatePubkey(seckey)
			if err != nil {
				t.Fatalf("CreatePubkey failed: %v", err)
			}

			// Sign with libsecp256k1
			sig, err := lib.SchnorrSign(msg, seckey)
			if err != nil {
				t.Fatalf("SchnorrSign failed: %v", err)
			}

			// Verify with libsecp256k1 (sanity check)
			if !lib.SchnorrVerify(sig, msg, pubkey) {
				t.Error("libsecp256k1 failed to verify its own signature")
			}

			// Verify with our implementation
			xonlyPubkey, err := XOnlyPubkeyParse(pubkey)
			if err != nil {
				t.Fatalf("XOnlyPubkeyParse failed: %v", err)
			}
			valid := SchnorrVerify(sig, msg, xonlyPubkey)
			if !valid {
				t.Error("Our SchnorrVerify failed to verify libsecp256k1 signature")
			}
		})
	}
}

// TestExternalValidationDebugSingleScalar is a focused debug test for a single scalar
func TestExternalValidationDebugSingleScalar(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}
	if !lib.IsLoaded() {
		t.Skip("libsecp256k1 not loaded")
	}

	// Use scalar 2 for simple debugging
	seckey, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000002")

	expectedPubkey, err := lib.CreatePubkey(seckey)
	if err != nil {
		t.Fatalf("CreatePubkey failed: %v", err)
	}
	t.Logf("Expected (2*G).x = %s", hex.EncodeToString(expectedPubkey))

	var s Scalar
	s.setB32(seckey)

	// EcmultConst
	var constResult GroupElementJacobian
	EcmultConst(&constResult, &Generator, &s)
	var constAff GroupElementAffine
	constAff.setGEJ(&constResult)
	constAff.x.normalize()
	var constX [32]byte
	constAff.x.getB32(constX[:])
	t.Logf("EcmultConst:         %s", hex.EncodeToString(constX[:]))

	// ecmultWindowedVar
	var windowedResult GroupElementJacobian
	ecmultWindowedVar(&windowedResult, &Generator, &s)
	var windowedAff GroupElementAffine
	windowedAff.setGEJ(&windowedResult)
	windowedAff.x.normalize()
	var windowedX [32]byte
	windowedAff.x.getB32(windowedX[:])
	t.Logf("ecmultWindowedVar:   %s", hex.EncodeToString(windowedX[:]))

	// ecmultGenGLV
	var glvResult GroupElementJacobian
	ecmultGenGLV(&glvResult, &s)
	var glvAff GroupElementAffine
	glvAff.setGEJ(&glvResult)
	glvAff.x.normalize()
	var glvX [32]byte
	glvAff.x.getB32(glvX[:])
	t.Logf("ecmultGenGLV:        %s", hex.EncodeToString(glvX[:]))

	// ecmultStraussWNAFGLV
	var straussResult GroupElementJacobian
	ecmultStraussWNAFGLV(&straussResult, &Generator, &s)
	var straussAff GroupElementAffine
	straussAff.setGEJ(&straussResult)
	straussAff.x.normalize()
	var straussX [32]byte
	straussAff.x.getB32(straussX[:])
	t.Logf("ecmultStraussWNAFGLV: %s", hex.EncodeToString(straussX[:]))

	// Check all match expected
	if hex.EncodeToString(constX[:]) != hex.EncodeToString(expectedPubkey) {
		t.Errorf("EcmultConst MISMATCH")
	}
	if hex.EncodeToString(windowedX[:]) != hex.EncodeToString(expectedPubkey) {
		t.Errorf("ecmultWindowedVar MISMATCH")
	}
	if hex.EncodeToString(glvX[:]) != hex.EncodeToString(expectedPubkey) {
		t.Errorf("ecmultGenGLV MISMATCH")
	}
	if hex.EncodeToString(straussX[:]) != hex.EncodeToString(expectedPubkey) {
		t.Errorf("ecmultStraussWNAFGLV MISMATCH")
	}
}
