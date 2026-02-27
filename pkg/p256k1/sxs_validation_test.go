//go:build !js

package p256k1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// Side-by-Side (SxS) Validation Tests
// These tests compare Go implementation results directly against libsecp256k1

// TestSxSGeneratorPoint verifies the generator point G matches
func TestSxSGeneratorPoint(t *testing.T) {
	// Known generator point coordinates from secp256k1 spec
	expectedGx := "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	expectedGy := "483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8"

	var gx, gy [32]byte
	gxFe := Generator.x
	gyFe := Generator.y
	gxFe.normalize()
	gyFe.normalize()
	gxFe.getB32(gx[:])
	gyFe.getB32(gy[:])

	if hex.EncodeToString(gx[:]) != expectedGx {
		t.Errorf("Generator.x mismatch:\n  got:  %s\n  want: %s",
			hex.EncodeToString(gx[:]), expectedGx)
	}
	if hex.EncodeToString(gy[:]) != expectedGy {
		t.Errorf("Generator.y mismatch:\n  got:  %s\n  want: %s",
			hex.EncodeToString(gy[:]), expectedGy)
	}
}

// TestSxSScalarMultGenerator tests k*G for various scalars
func TestSxSScalarMultGenerator(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001", // 1
		"0000000000000000000000000000000000000000000000000000000000000002", // 2
		"0000000000000000000000000000000000000000000000000000000000000003", // 3
		"0000000000000000000000000000000000000000000000000000000000000007", // 7
		"000000000000000000000000000000000000000000000000000000000000000f", // 15
		"00000000000000000000000000000000000000000000000000000000000000ff", // 255
		"000000000000000000000000000000000000000000000000000000000000ffff", // 65535
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", // random
		"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef", // random
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", // n/2
		"5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72", // lambda
	}

	for _, scalarHex := range testCases {
		t.Run(scalarHex[:8], func(t *testing.T) {
			seckey, _ := hex.DecodeString(scalarHex)

			// Get libsecp256k1 result (full compressed pubkey)
			libPubkey, libParity, err := lib.CreatePubkeyCompressed(seckey)
			if err != nil {
				t.Skipf("libsecp256k1 rejected key: %v", err)
			}

			// Parse scalar
			var s Scalar
			s.setB32(seckey)

			// Compute using each Go implementation
			implementations := []struct {
				name string
				fn   func(*GroupElementJacobian, *Scalar)
			}{
				{"EcmultConst", func(r *GroupElementJacobian, s *Scalar) { EcmultConst(r, &Generator, s) }},
				{"ecmultWindowedVar", func(r *GroupElementJacobian, s *Scalar) { ecmultWindowedVar(r, &Generator, s) }},
				{"ecmultGenGLV", func(r *GroupElementJacobian, s *Scalar) { ecmultGenGLV(r, s) }},
				{"ecmultStraussWNAFGLV", func(r *GroupElementJacobian, s *Scalar) { ecmultStraussWNAFGLV(r, &Generator, s) }},
			}

			for _, impl := range implementations {
				t.Run(impl.name, func(t *testing.T) {
					var result GroupElementJacobian
					impl.fn(&result, &s)

					var aff GroupElementAffine
					aff.setGEJ(&result)
					aff.x.normalize()
					aff.y.normalize()

					// Serialize to compressed format
					var goCompressed [33]byte
					var xBytes, yBytes [32]byte
					aff.x.getB32(xBytes[:])
					aff.y.getB32(yBytes[:])

					if yBytes[31]&1 == 0 {
						goCompressed[0] = 0x02
					} else {
						goCompressed[0] = 0x03
					}
					copy(goCompressed[1:], xBytes[:])

					// Compare
					if !bytes.Equal(goCompressed[:], libPubkey) {
						t.Errorf("Mismatch:\n  Go:  %s\n  Lib: %s",
							hex.EncodeToString(goCompressed[:]),
							hex.EncodeToString(libPubkey))
					}

					// Also verify parity
					goParity := 0
					if yBytes[31]&1 == 1 {
						goParity = 1
					}
					if goParity != libParity {
						t.Errorf("Parity mismatch: Go=%d, Lib=%d", goParity, libParity)
					}
				})
			}
		})
	}
}

// TestSxSFullPubkey tests full X,Y coordinates match
func TestSxSFullPubkey(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
	}

	for _, scalarHex := range testCases {
		t.Run(scalarHex[:8], func(t *testing.T) {
			seckey, _ := hex.DecodeString(scalarHex)

			// Get libsecp256k1 uncompressed pubkey (65 bytes: 0x04 || X || Y)
			libPubkey, err := lib.CreatePubkeyUncompressed(seckey)
			if err != nil {
				t.Fatalf("libsecp256k1 failed: %v", err)
			}

			if len(libPubkey) != 65 || libPubkey[0] != 0x04 {
				t.Fatalf("Invalid uncompressed pubkey format")
			}

			libX := libPubkey[1:33]
			libY := libPubkey[33:65]

			// Compute using Go
			var s Scalar
			s.setB32(seckey)

			var result GroupElementJacobian
			EcmultConst(&result, &Generator, &s)

			var aff GroupElementAffine
			aff.setGEJ(&result)
			aff.x.normalize()
			aff.y.normalize()

			var goX, goY [32]byte
			aff.x.getB32(goX[:])
			aff.y.getB32(goY[:])

			t.Logf("Scalar: %s", scalarHex)
			t.Logf("Lib X:  %s", hex.EncodeToString(libX))
			t.Logf("Go  X:  %s", hex.EncodeToString(goX[:]))
			t.Logf("Lib Y:  %s", hex.EncodeToString(libY))
			t.Logf("Go  Y:  %s", hex.EncodeToString(goY[:]))

			if !bytes.Equal(goX[:], libX) {
				t.Errorf("X coordinate mismatch")
			}
			if !bytes.Equal(goY[:], libY) {
				t.Errorf("Y coordinate mismatch")
			}
		})
	}
}

// TestSxSArbitraryPointMult tests k*P for arbitrary points P
func TestSxSArbitraryPointMult(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	// Generate a base point P = baseScalar * G
	baseScalars := []string{
		"0000000000000000000000000000000000000000000000000000000000000007",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
	}

	multScalars := []string{
		"0000000000000000000000000000000000000000000000000000000000000002",
		"0000000000000000000000000000000000000000000000000000000000000003",
		"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef",
	}

	for _, baseHex := range baseScalars {
		for _, multHex := range multScalars {
			name := fmt.Sprintf("base_%s_mult_%s", baseHex[:4], multHex[:4])
			t.Run(name, func(t *testing.T) {
				baseBytes, _ := hex.DecodeString(baseHex)
				multBytes, _ := hex.DecodeString(multHex)

				// Compute P = baseScalar * G using Go
				var baseScalar Scalar
				baseScalar.setB32(baseBytes)

				var pJac GroupElementJacobian
				EcmultConst(&pJac, &Generator, &baseScalar)
				var p GroupElementAffine
				p.setGEJ(&pJac)
				p.x.normalize()
				p.y.normalize()

				// Serialize P as compressed for libsecp256k1
				pCompressed := make([]byte, 33)
				var pX [32]byte
				var pY [32]byte
				p.x.getB32(pX[:])
				p.y.getB32(pY[:])
				if pY[31]&1 == 0 {
					pCompressed[0] = 0x02
				} else {
					pCompressed[0] = 0x03
				}
				copy(pCompressed[1:], pX[:])

				// Compute multScalar * P using libsecp256k1 via ECDH
				// ECDH(seckey, pubkey) computes seckey * pubkey
				libResult, err := lib.ECDH(multBytes, pCompressed)
				if err != nil {
					t.Fatalf("libsecp256k1 ECDH failed: %v", err)
				}

				// Compute multScalar * P using Go implementations
				var multScalar Scalar
				multScalar.setB32(multBytes)

				implementations := []struct {
					name string
					fn   func(*GroupElementJacobian, *GroupElementAffine, *Scalar)
				}{
					{"EcmultConst", func(r *GroupElementJacobian, a *GroupElementAffine, s *Scalar) { EcmultConst(r, a, s) }},
					{"ecmultWindowedVar", func(r *GroupElementJacobian, a *GroupElementAffine, s *Scalar) { ecmultWindowedVar(r, a, s) }},
					{"ecmultStraussWNAFGLV", func(r *GroupElementJacobian, a *GroupElementAffine, s *Scalar) { ecmultStraussWNAFGLV(r, a, s) }},
				}

				for _, impl := range implementations {
					t.Run(impl.name, func(t *testing.T) {
						var result GroupElementJacobian
						impl.fn(&result, &p, &multScalar)

						var resultAff GroupElementAffine
						resultAff.setGEJ(&result)
						resultAff.x.normalize()
						resultAff.y.normalize()

						// libsecp256k1 ECDH returns sha256(compressed_point)
						// so we need to hash our result the same way
						var goCompressed [33]byte
						var resultX, resultY [32]byte
						resultAff.x.getB32(resultX[:])
						resultAff.y.getB32(resultY[:])

						if resultY[31]&1 == 0 {
							goCompressed[0] = 0x02
						} else {
							goCompressed[0] = 0x03
						}
						copy(goCompressed[1:], resultX[:])

						goHash := sha256Sum(goCompressed[:])

						if !bytes.Equal(goHash[:], libResult) {
							t.Errorf("Mismatch:\n  Go hash:  %s\n  Lib hash: %s\n  Go point: %s",
								hex.EncodeToString(goHash[:]),
								hex.EncodeToString(libResult),
								hex.EncodeToString(goCompressed[:]))
						}
					})
				}
			})
		}
	}
}

// sha256Sum computes SHA256 hash
func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// TestSxSGLVConstants verifies GLV constants match libsecp256k1
func TestSxSGLVConstants(t *testing.T) {
	// These are the known GLV constants from libsecp256k1
	// Lambda: cube root of unity mod n
	expectedLambda := "5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72"
	// Beta: cube root of unity mod p
	expectedBeta := "7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee"

	var lambdaBytes [32]byte
	scalarLambda.getB32(lambdaBytes[:])
	if hex.EncodeToString(lambdaBytes[:]) != expectedLambda {
		t.Errorf("Lambda mismatch:\n  got:  %s\n  want: %s",
			hex.EncodeToString(lambdaBytes[:]), expectedLambda)
	}

	var betaBytes [32]byte
	fieldBeta.getB32(betaBytes[:])
	if hex.EncodeToString(betaBytes[:]) != expectedBeta {
		t.Errorf("Beta mismatch:\n  got:  %s\n  want: %s",
			hex.EncodeToString(betaBytes[:]), expectedBeta)
	}

	// Verify lambda^3 = 1 (mod n)
	var lambda2, lambda3 Scalar
	lambda2.mul(&scalarLambda, &scalarLambda)
	lambda3.mul(&lambda2, &scalarLambda)
	if !lambda3.equal(&ScalarOne) {
		t.Error("lambda^3 != 1 (mod n)")
	}

	// Verify beta^3 = 1 (mod p)
	var beta2, beta3 FieldElement
	beta2.sqr(&fieldBeta)
	beta3.mul(&beta2, &fieldBeta)
	beta3.normalize()
	if !beta3.equal(&FieldElementOne) {
		t.Error("beta^3 != 1 (mod p)")
	}
}

// TestSxSEndomorphism verifies λ*P = (β*x, y)
func TestSxSEndomorphism(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
	}

	for _, scalarHex := range testCases {
		t.Run(scalarHex[:8], func(t *testing.T) {
			seckey, _ := hex.DecodeString(scalarHex)

			// Compute P = k * G
			var s Scalar
			s.setB32(seckey)

			var pJac GroupElementJacobian
			EcmultConst(&pJac, &Generator, &s)
			var p GroupElementAffine
			p.setGEJ(&pJac)
			p.x.normalize()
			p.y.normalize()

			// Compute λ*P using scalar multiplication
			var lambdaPJac GroupElementJacobian
			EcmultConst(&lambdaPJac, &p, &scalarLambda)
			var lambdaP GroupElementAffine
			lambdaP.setGEJ(&lambdaPJac)
			lambdaP.x.normalize()
			lambdaP.y.normalize()

			// Compute λ*P using endomorphism (β*x, y)
			var endoP GroupElementAffine
			endoP.mulLambda(&p)
			endoP.x.normalize()
			endoP.y.normalize()

			// They should match
			var lambdaPX, lambdaPY, endoPX, endoPY [32]byte
			lambdaP.x.getB32(lambdaPX[:])
			lambdaP.y.getB32(lambdaPY[:])
			endoP.x.getB32(endoPX[:])
			endoP.y.getB32(endoPY[:])

			if !bytes.Equal(lambdaPX[:], endoPX[:]) {
				t.Errorf("X mismatch:\n  scalar mult: %s\n  endomorphism: %s",
					hex.EncodeToString(lambdaPX[:]), hex.EncodeToString(endoPX[:]))
			}
			if !bytes.Equal(lambdaPY[:], endoPY[:]) {
				t.Errorf("Y mismatch:\n  scalar mult: %s\n  endomorphism: %s",
					hex.EncodeToString(lambdaPY[:]), hex.EncodeToString(endoPY[:]))
			}

			// Also verify against libsecp256k1
			// Compute (k * lambda) * G via libsecp256k1
			var kLambda Scalar
			kLambda.mul(&s, &scalarLambda)
			var kLambdaBytes [32]byte
			kLambda.getB32(kLambdaBytes[:])

			libPubkey, err := lib.CreatePubkeyUncompressed(kLambdaBytes[:])
			if err != nil {
				t.Fatalf("libsecp256k1 failed: %v", err)
			}

			libX := libPubkey[1:33]
			libY := libPubkey[33:65]

			if !bytes.Equal(lambdaPX[:], libX) {
				t.Errorf("X vs libsecp256k1 mismatch:\n  Go:  %s\n  Lib: %s",
					hex.EncodeToString(lambdaPX[:]), hex.EncodeToString(libX))
			}
			if !bytes.Equal(lambdaPY[:], libY) {
				t.Errorf("Y vs libsecp256k1 mismatch:\n  Go:  %s\n  Lib: %s",
					hex.EncodeToString(lambdaPY[:]), hex.EncodeToString(libY))
			}
		})
	}
}

// TestSxSScalarSplit validates the GLV scalar splitting
func TestSxSScalarSplit(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
		"00000000000000000000000000000000ffffffffffffffffffffffffffffffff",
		"ffffffffffffffffffffffffffffffff00000000000000000000000000000000",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
		"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef",
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
	}

	for _, scalarHex := range testCases {
		t.Run(scalarHex[:8], func(t *testing.T) {
			kBytes, _ := hex.DecodeString(scalarHex)

			var k Scalar
			k.setB32(kBytes)

			// Split k into k1, k2
			var k1, k2 Scalar
			scalarSplitLambda(&k1, &k2, &k)

			// Log the split
			var k1Bytes, k2Bytes [32]byte
			k1.getB32(k1Bytes[:])
			k2.getB32(k2Bytes[:])
			t.Logf("k  = %s", scalarHex)
			t.Logf("k1 = %s", hex.EncodeToString(k1Bytes[:]))
			t.Logf("k2 = %s", hex.EncodeToString(k2Bytes[:]))

			// Verify k1 + k2*lambda = k
			var k2Lambda, reconstructed Scalar
			k2Lambda.mul(&k2, &scalarLambda)
			reconstructed.add(&k1, &k2Lambda)

			if !reconstructed.equal(&k) {
				var recBytes [32]byte
				reconstructed.getB32(recBytes[:])
				t.Errorf("Scalar reconstruction failed:\n  k1+k2*lambda = %s\n  k            = %s",
					hex.EncodeToString(recBytes[:]), scalarHex)
			}

			// Verify k1*G + k2*(lambda*G) = k*G via libsecp256k1
			expectedPubkey, err := lib.CreatePubkeyUncompressed(kBytes)
			if err != nil {
				t.Skipf("libsecp256k1 rejected key")
			}

			// Compute k1*G
			var k1G GroupElementJacobian
			EcmultConst(&k1G, &Generator, &k1)

			// Compute lambda*G
			var lambdaG GroupElementAffine
			lambdaG.mulLambda(&Generator)

			// Compute k2*(lambda*G)
			var k2LambdaG GroupElementJacobian
			EcmultConst(&k2LambdaG, &lambdaG, &k2)

			// Add k1*G + k2*(lambda*G)
			var sum GroupElementJacobian
			sum.addVar(&k1G, &k2LambdaG)

			var sumAff GroupElementAffine
			sumAff.setGEJ(&sum)
			sumAff.x.normalize()
			sumAff.y.normalize()

			var sumX, sumY [32]byte
			sumAff.x.getB32(sumX[:])
			sumAff.y.getB32(sumY[:])

			expectedX := expectedPubkey[1:33]
			expectedY := expectedPubkey[33:65]

			if !bytes.Equal(sumX[:], expectedX) {
				t.Errorf("X mismatch:\n  k1*G + k2*(λ*G) = %s\n  k*G (lib)       = %s",
					hex.EncodeToString(sumX[:]), hex.EncodeToString(expectedX))
			}
			if !bytes.Equal(sumY[:], expectedY) {
				t.Errorf("Y mismatch:\n  k1*G + k2*(λ*G) = %s\n  k*G (lib)       = %s",
					hex.EncodeToString(sumY[:]), hex.EncodeToString(expectedY))
			}
		})
	}
}

// TestSxSRandomScalars runs extensive random scalar tests
func TestSxSRandomScalars(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	seed := uint64(0xdeadbeef12345678)
	const numTests = 500

	var failures int
	for i := 0; i < numTests; i++ {
		// Generate pseudo-random scalar
		var seckey [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			seckey[j] = byte(seed >> 56)
		}

		// Get libsecp256k1 result
		libPubkey, _, err := lib.CreatePubkeyCompressed(seckey[:])
		if err != nil {
			continue // Skip invalid keys
		}

		var s Scalar
		s.setB32(seckey[:])

		// Test each implementation
		implementations := []struct {
			name string
			fn   func(*GroupElementJacobian, *Scalar)
		}{
			{"EcmultConst", func(r *GroupElementJacobian, s *Scalar) { EcmultConst(r, &Generator, s) }},
			{"ecmultWindowedVar", func(r *GroupElementJacobian, s *Scalar) { ecmultWindowedVar(r, &Generator, s) }},
			{"ecmultGenGLV", func(r *GroupElementJacobian, s *Scalar) { ecmultGenGLV(r, s) }},
			{"ecmultStraussWNAFGLV", func(r *GroupElementJacobian, s *Scalar) { ecmultStraussWNAFGLV(r, &Generator, s) }},
		}

		for _, impl := range implementations {
			var result GroupElementJacobian
			impl.fn(&result, &s)

			var aff GroupElementAffine
			aff.setGEJ(&result)
			aff.x.normalize()
			aff.y.normalize()

			var goCompressed [33]byte
			var xBytes, yBytes [32]byte
			aff.x.getB32(xBytes[:])
			aff.y.getB32(yBytes[:])

			if yBytes[31]&1 == 0 {
				goCompressed[0] = 0x02
			} else {
				goCompressed[0] = 0x03
			}
			copy(goCompressed[1:], xBytes[:])

			if !bytes.Equal(goCompressed[:], libPubkey) {
				t.Errorf("Test %d %s mismatch:\n  scalar: %s\n  Go:     %s\n  Lib:    %s",
					i, impl.name, hex.EncodeToString(seckey[:]),
					hex.EncodeToString(goCompressed[:]),
					hex.EncodeToString(libPubkey))
				failures++
				if failures > 10 {
					t.Fatalf("Too many failures, stopping")
				}
			}
		}
	}

	t.Logf("Tested %d random scalars with 4 implementations each (%d total operations)", numTests, numTests*4)
}

// TestSxSECDSASignVerify tests ECDSA signature creation and verification
func TestSxSECDSASignVerify(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []struct {
		name   string
		seckey string
		msg    string
	}{
		{"simple", "0000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000000"},
		{"random", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
		{"msg_all_ff", "0000000000000000000000000000000000000000000000000000000000000002", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
		{"high_scalar", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seckey, _ := hex.DecodeString(tc.seckey)
			msg, _ := hex.DecodeString(tc.msg)

			// Get compressed pubkey from libsecp256k1
			pubkeyCompressed, _, err := lib.CreatePubkeyCompressed(seckey)
			if err != nil {
				t.Skipf("libsecp256k1 rejected key: %v", err)
			}

			// 1. Sign with libsecp256k1, verify with libsecp256k1
			libSig, err := lib.ECDSASign(msg, seckey)
			if err != nil {
				t.Fatalf("lib ECDSASign failed: %v", err)
			}
			if !lib.ECDSAVerify(libSig, msg, pubkeyCompressed) {
				t.Error("lib failed to verify its own signature")
			}

			// 2. Sign with libsecp256k1, verify with Go
			// Parse the pubkey for Go
			var pubkey PublicKey
			if err := ECPubkeyParse(&pubkey, pubkeyCompressed); err != nil {
				t.Fatalf("ECPubkeyParse failed: %v", err)
			}

			var goSig ECDSASignature
			if err := goSig.FromCompact((*ECDSASignatureCompact)(libSig)); err != nil {
				t.Fatalf("FromCompact failed: %v", err)
			}
			if !ECDSAVerify(&goSig, msg, &pubkey) {
				t.Error("Go failed to verify libsecp256k1 signature")
			}

			// 3. Sign with Go, verify with libsecp256k1
			var goGenSig ECDSASignature
			if err := ECDSASign(&goGenSig, msg, seckey); err != nil {
				t.Fatalf("Go ECDSASign failed: %v", err)
			}
			goCompact := goGenSig.ToCompact()
			if !lib.ECDSAVerify(goCompact[:], msg, pubkeyCompressed) {
				t.Errorf("libsecp256k1 failed to verify Go signature")
			}

			// 4. Sign with Go, verify with Go
			if !ECDSAVerify(&goGenSig, msg, &pubkey) {
				t.Error("Go failed to verify its own signature")
			}

			// 5. Test DER encoding round-trip
			t.Run("DER", func(t *testing.T) {
				// Get DER from libsecp256k1
				libDER, err := lib.ECDSASignDER(msg, seckey)
				if err != nil {
					t.Fatalf("lib ECDSASignDER failed: %v", err)
				}

				// Verify with libsecp256k1
				if !lib.ECDSAVerifyDER(libDER, msg, pubkeyCompressed) {
					t.Error("lib failed to verify its own DER signature")
				}

				// Parse with Go
				var derSig ECDSASignature
				if err := derSig.ParseDER(libDER); err != nil {
					t.Fatalf("Go ParseDER failed: %v", err)
				}
				if !ECDSAVerify(&derSig, msg, &pubkey) {
					t.Error("Go failed to verify lib DER signature")
				}

				// Sign DER with Go
				goDER, err := ECDSASignDER(msg, seckey)
				if err != nil {
					t.Fatalf("Go ECDSASignDER failed: %v", err)
				}

				// Verify Go DER with libsecp256k1
				if !lib.ECDSAVerifyDER(goDER, msg, pubkeyCompressed) {
					t.Errorf("lib failed to verify Go DER signature:\n  Go:  %s\n  Lib: %s",
						hex.EncodeToString(goDER), hex.EncodeToString(libDER))
				}
			})
		})
	}
}

// TestSxSECDSARandomMessages tests ECDSA with random messages
func TestSxSECDSARandomMessages(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	// Fixed secret key
	seckey, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	pubkeyCompressed, _, err := lib.CreatePubkeyCompressed(seckey)
	if err != nil {
		t.Fatalf("CreatePubkeyCompressed failed: %v", err)
	}
	var pubkey PublicKey
	if err := ECPubkeyParse(&pubkey, pubkeyCompressed); err != nil {
		t.Fatalf("ECPubkeyParse failed: %v", err)
	}

	seed := uint64(0xdeadbeef12345678)
	const numTests = 100

	for i := 0; i < numTests; i++ {
		// Generate random message
		var msg [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			msg[j] = byte(seed >> 56)
		}

		// Sign with Go
		var goSig ECDSASignature
		if err := ECDSASign(&goSig, msg[:], seckey); err != nil {
			t.Fatalf("Test %d: ECDSASign failed: %v", i, err)
		}

		// Verify with libsecp256k1
		goCompact := goSig.ToCompact()
		if !lib.ECDSAVerify(goCompact[:], msg[:], pubkeyCompressed) {
			t.Errorf("Test %d: libsecp256k1 failed to verify Go signature", i)
		}

		// Verify with Go
		if !ECDSAVerify(&goSig, msg[:], &pubkey) {
			t.Errorf("Test %d: Go failed to verify its own signature", i)
		}
	}

	t.Logf("Tested %d random messages", numTests)
}

// TestSxSECDSARecover tests public key recovery from ECDSA signatures
func TestSxSECDSARecover(t *testing.T) {
	testCases := []struct {
		name   string
		seckey string
		msg    string
	}{
		{"simple", "0000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000000"},
		{"random", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seckey, _ := hex.DecodeString(tc.seckey)
			msg, _ := hex.DecodeString(tc.msg)

			// Create public key from secret key
			var originalPubkey PublicKey
			if err := ECPubkeyCreate(&originalPubkey, seckey); err != nil {
				t.Fatalf("ECPubkeyCreate failed: %v", err)
			}

			// Sign with recovery
			var sig ECDSARecoverableSignature
			if err := ECDSASignRecoverable(&sig, msg, seckey); err != nil {
				t.Fatalf("ECDSASignRecoverable failed: %v", err)
			}

			t.Logf("Recovery ID: %d", sig.recid)

			// Recover public key
			var recoveredPubkey PublicKey
			if err := ECDSARecover(&recoveredPubkey, &sig, msg); err != nil {
				t.Fatalf("ECDSARecover failed: %v", err)
			}

			// Compare public keys
			if ECPubkeyCmp(&originalPubkey, &recoveredPubkey) != 0 {
				// Serialize both for comparison
				original := make([]byte, 33)
				recovered := make([]byte, 33)
				ECPubkeySerialize(original, &originalPubkey, 258) // compressed
				ECPubkeySerialize(recovered, &recoveredPubkey, 258)
				t.Errorf("Public key mismatch:\n  original:  %s\n  recovered: %s",
					hex.EncodeToString(original), hex.EncodeToString(recovered))
			}

			// Also verify the signature
			compact, recid := sig.ToCompact()
			t.Logf("Compact signature: %s (recid=%d)", hex.EncodeToString(compact), recid)

			var verifySig ECDSASignature
			verifySig.r = sig.r
			verifySig.s = sig.s
			if !ECDSAVerify(&verifySig, msg, &originalPubkey) {
				t.Error("Signature verification failed")
			}
		})
	}
}

// TestSxSECDSARecoverRandom tests recovery with random keys
func TestSxSECDSARecoverRandom(t *testing.T) {
	seed := uint64(0xdeadbeef12345678)
	const numTests = 50

	for i := 0; i < numTests; i++ {
		// Generate random secret key
		var seckey [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			seckey[j] = byte(seed >> 56)
		}

		// Create public key
		var originalPubkey PublicKey
		if err := ECPubkeyCreate(&originalPubkey, seckey[:]); err != nil {
			continue // Skip invalid keys
		}

		// Generate random message
		var msg [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			msg[j] = byte(seed >> 56)
		}

		// Sign with recovery
		var sig ECDSARecoverableSignature
		if err := ECDSASignRecoverable(&sig, msg[:], seckey[:]); err != nil {
			t.Fatalf("Test %d: ECDSASignRecoverable failed: %v", i, err)
		}

		// Recover public key
		var recoveredPubkey PublicKey
		if err := ECDSARecover(&recoveredPubkey, &sig, msg[:]); err != nil {
			t.Fatalf("Test %d: ECDSARecover failed: %v", i, err)
		}

		// Compare
		if ECPubkeyCmp(&originalPubkey, &recoveredPubkey) != 0 {
			t.Errorf("Test %d: Public key mismatch", i)
		}
	}

	t.Logf("Tested %d random key/message pairs", numTests)
}

// TestSxSECDSALowS verifies signatures have low-S values
func TestSxSECDSALowS(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	seckey, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")

	seed := uint64(0xcafebabe)
	for i := 0; i < 50; i++ {
		var msg [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			msg[j] = byte(seed >> 56)
		}

		// Sign with Go
		var goSig ECDSASignature
		if err := ECDSASign(&goSig, msg[:], seckey); err != nil {
			t.Fatalf("ECDSASign failed: %v", err)
		}

		// Verify it's low-S
		if !goSig.IsLowS() {
			t.Errorf("Test %d: Go signature has high-S", i)
		}

		// Sign with libsecp256k1
		libSig, err := lib.ECDSASign(msg[:], seckey)
		if err != nil {
			t.Fatalf("lib ECDSASign failed: %v", err)
		}

		var libGoSig ECDSASignature
		libGoSig.FromCompact((*ECDSASignatureCompact)(libSig))
		if !libGoSig.IsLowS() {
			t.Errorf("Test %d: lib signature has high-S", i)
		}
	}
}

// TestSxSSchnorrSignVerify tests Schnorr signature creation and verification
func TestSxSSchnorrSignVerify(t *testing.T) {
	lib, err := GetLibSecp256k1()
	if err != nil {
		t.Skipf("libsecp256k1 not available: %v", err)
	}

	testCases := []struct {
		name   string
		seckey string
		msg    string
	}{
		{"simple", "0000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000000"},
		{"random", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
		{"msg_all_ff", "0000000000000000000000000000000000000000000000000000000000000002", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
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

			// 1. Sign with libsecp256k1, verify with libsecp256k1
			libSig, err := lib.SchnorrSign(msg, seckey)
			if err != nil {
				t.Fatalf("lib SchnorrSign failed: %v", err)
			}
			if !lib.SchnorrVerify(libSig, msg, pubkey) {
				t.Error("lib failed to verify its own signature")
			}

			// 2. Sign with libsecp256k1, verify with Go
			xonlyPubkey, err := XOnlyPubkeyParse(pubkey)
			if err != nil {
				t.Fatalf("XOnlyPubkeyParse failed: %v", err)
			}
			if !SchnorrVerify(libSig, msg, xonlyPubkey) {
				t.Error("Go failed to verify libsecp256k1 signature")
			}

			// 3. Sign with Go, verify with libsecp256k1
			keypair, err := KeyPairCreate(seckey)
			if err != nil {
				t.Fatalf("KeyPairCreate failed: %v", err)
			}

			var goSig [64]byte
			if err := SchnorrSign(goSig[:], msg, keypair, nil); err != nil {
				t.Fatalf("Go SchnorrSign failed: %v", err)
			}
			if !lib.SchnorrVerify(goSig[:], msg, pubkey) {
				t.Errorf("libsecp256k1 failed to verify Go signature:\n  sig: %s",
					hex.EncodeToString(goSig[:]))
			}

			// 4. Sign with Go, verify with Go
			if !SchnorrVerify(goSig[:], msg, xonlyPubkey) {
				t.Error("Go failed to verify its own signature")
			}
		})
	}
}
