//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"encoding/hex"
	"testing"
)

// TestGLVScalarConstants verifies that the GLV scalar constants are correctly defined
func TestGLVScalarConstants(t *testing.T) {
	// Test lambda constant
	// Expected: 0x5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72
	var lambdaBytes [32]byte
	scalarLambda.getB32(lambdaBytes[:])
	expectedLambda := "5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72"
	if hex.EncodeToString(lambdaBytes[:]) != expectedLambda {
		t.Errorf("scalarLambda mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(lambdaBytes[:]), expectedLambda)
	}

	// Test g1 constant
	// Expected: 0x3086D221A7D46BCDE86C90E49284EB153DAA8A1471E8CA7FE893209A45DBB031
	var g1Bytes [32]byte
	scalarG1.getB32(g1Bytes[:])
	expectedG1 := "3086d221a7d46bcde86c90e49284eb153daa8a1471e8ca7fe893209a45dbb031"
	if hex.EncodeToString(g1Bytes[:]) != expectedG1 {
		t.Errorf("scalarG1 mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(g1Bytes[:]), expectedG1)
	}

	// Test g2 constant
	// Expected: 0xE4437ED6010E88286F547FA90ABFE4C4221208AC9DF506C61571B4AE8AC47F71
	var g2Bytes [32]byte
	scalarG2.getB32(g2Bytes[:])
	expectedG2 := "e4437ed6010e88286f547fa90abfe4c4221208ac9df506c61571b4ae8ac47f71"
	if hex.EncodeToString(g2Bytes[:]) != expectedG2 {
		t.Errorf("scalarG2 mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(g2Bytes[:]), expectedG2)
	}

	// Test -b1 constant
	// Expected: 0x00000000000000000000000000000000E4437ED6010E88286F547FA90ABFE4C3
	var minusB1Bytes [32]byte
	scalarMinusB1.getB32(minusB1Bytes[:])
	expectedMinusB1 := "00000000000000000000000000000000e4437ed6010e88286f547fa90abfe4c3"
	if hex.EncodeToString(minusB1Bytes[:]) != expectedMinusB1 {
		t.Errorf("scalarMinusB1 mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(minusB1Bytes[:]), expectedMinusB1)
	}

	// Test -b2 constant
	// Expected: 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFE8A280AC50774346DD765CDA83DB1562C
	var minusB2Bytes [32]byte
	scalarMinusB2.getB32(minusB2Bytes[:])
	expectedMinusB2 := "fffffffffffffffffffffffffffffffe8a280ac50774346dd765cda83db1562c"
	if hex.EncodeToString(minusB2Bytes[:]) != expectedMinusB2 {
		t.Errorf("scalarMinusB2 mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(minusB2Bytes[:]), expectedMinusB2)
	}
}

// TestGLVLambdaCubeRoot verifies that λ^3 ≡ 1 (mod n)
func TestGLVLambdaCubeRoot(t *testing.T) {
	// Compute λ^2
	var lambda2 Scalar
	lambda2.mul(&scalarLambda, &scalarLambda)

	// Compute λ^3 = λ^2 * λ
	var lambda3 Scalar
	lambda3.mul(&lambda2, &scalarLambda)

	// Check if λ^3 ≡ 1 (mod n)
	if !lambda3.equal(&ScalarOne) {
		var bytes [32]byte
		lambda3.getB32(bytes[:])
		t.Errorf("λ^3 ≠ 1 (mod n), got: %s", hex.EncodeToString(bytes[:]))
	}
}

// TestGLVLambdaProperty verifies that λ^2 + λ + 1 ≡ 0 (mod n)
func TestGLVLambdaProperty(t *testing.T) {
	// Compute λ^2
	var lambda2 Scalar
	lambda2.mul(&scalarLambda, &scalarLambda)

	// Compute λ^2 + λ
	var sum Scalar
	sum.add(&lambda2, &scalarLambda)

	// Compute λ^2 + λ + 1
	sum.add(&sum, &ScalarOne)

	// Check if λ^2 + λ + 1 ≡ 0 (mod n)
	if !sum.isZero() {
		var bytes [32]byte
		sum.getB32(bytes[:])
		t.Errorf("λ^2 + λ + 1 ≠ 0 (mod n), got: %s", hex.EncodeToString(bytes[:]))
	}
}

// TestGLVBetaConstant verifies that the Beta field constant is correctly defined
func TestGLVBetaConstant(t *testing.T) {
	// Expected: 0x7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee
	var betaBytes [32]byte
	fieldBeta.getB32(betaBytes[:])
	expectedBeta := "7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee"
	if hex.EncodeToString(betaBytes[:]) != expectedBeta {
		t.Errorf("fieldBeta mismatch:\n  got:  %s\n  want: %s", hex.EncodeToString(betaBytes[:]), expectedBeta)
	}
}

// TestGLVBetaCubeRoot verifies that β^3 ≡ 1 (mod p)
func TestGLVBetaCubeRoot(t *testing.T) {
	// Compute β^2
	var beta2 FieldElement
	beta2.sqr(&fieldBeta)
	beta2.normalize()

	// Compute β^3 = β^2 * β
	var beta3 FieldElement
	beta3.mul(&beta2, &fieldBeta)
	beta3.normalize()

	// Check if β^3 ≡ 1 (mod p)
	if !beta3.equal(&FieldElementOne) {
		var bytes [32]byte
		beta3.getB32(bytes[:])
		t.Errorf("β^3 ≠ 1 (mod p), got: %s", hex.EncodeToString(bytes[:]))
	}
}

// TestGLVBetaProperty verifies that β^2 + β + 1 ≡ 0 (mod p)
func TestGLVBetaProperty(t *testing.T) {
	// Compute β^2
	var beta2 FieldElement
	beta2.sqr(&fieldBeta)

	// Compute β^2 + β
	var sum FieldElement
	sum = beta2
	sum.add(&fieldBeta)

	// Compute β^2 + β + 1
	sum.add(&FieldElementOne)
	sum.normalize()

	// Check if β^2 + β + 1 ≡ 0 (mod p)
	if !sum.normalizesToZeroVar() {
		var bytes [32]byte
		sum.getB32(bytes[:])
		t.Errorf("β^2 + β + 1 ≠ 0 (mod p), got: %s", hex.EncodeToString(bytes[:]))
	}
}

// TestGLVEndomorphismOnGenerator verifies that λ·G = (β·Gx, Gy)
// This confirms that the endomorphism relationship holds
func TestGLVEndomorphismOnGenerator(t *testing.T) {
	// Compute λ·G using scalar multiplication
	var lambdaG GroupElementJacobian
	EcmultConst(&lambdaG, &Generator, &scalarLambda)

	// Convert to affine
	var lambdaGAff GroupElementAffine
	lambdaGAff.setGEJ(&lambdaG)
	lambdaGAff.x.normalize()
	lambdaGAff.y.normalize()

	// Compute β·Gx
	var betaGx FieldElement
	betaGx.mul(&Generator.x, &fieldBeta)
	betaGx.normalize()

	// Check X coordinate: λ·G.x should equal β·G.x
	if !lambdaGAff.x.equal(&betaGx) {
		var got, want [32]byte
		lambdaGAff.x.getB32(got[:])
		betaGx.getB32(want[:])
		t.Errorf("λ·G.x ≠ β·G.x:\n  got:  %s\n  want: %s", hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}

	// Check Y coordinate: λ·G.y should equal G.y
	genY := Generator.y
	genY.normalize()
	if !lambdaGAff.y.equal(&genY) {
		var got, want [32]byte
		lambdaGAff.y.getB32(got[:])
		genY.getB32(want[:])
		t.Errorf("λ·G.y ≠ G.y:\n  got:  %s\n  want: %s", hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}
}

// =============================================================================
// Phase 2 Tests: Scalar Splitting
// =============================================================================

// TestScalarSplitLambdaProperty verifies that k1 + k2*λ ≡ k (mod n)
func TestScalarSplitLambdaProperty(t *testing.T) {
	testCases := []struct {
		name  string
		kHex  string
	}{
		{"one", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"small", "0000000000000000000000000000000000000000000000000000000000000100"},
		{"medium", "0000000000000000000000000000000100000000000000000000000000000000"},
		{"large", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413f"}, // n-2
		{"random1", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"random2", "0000000000000000000000000000000014551231950b75fc4402da1732fc9bebe"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse k
			kBytes, _ := hex.DecodeString(tc.kHex)
			var k Scalar
			k.setB32(kBytes)

			// Split k into k1, k2
			var k1, k2 Scalar
			scalarSplitLambda(&k1, &k2, &k)

			// Verify: k1 + k2*λ ≡ k (mod n)
			var k2Lambda, sum Scalar
			k2Lambda.mul(&k2, &scalarLambda)
			sum.add(&k1, &k2Lambda)

			if !sum.equal(&k) {
				var sumBytes, kBytes [32]byte
				sum.getB32(sumBytes[:])
				k.getB32(kBytes[:])
				t.Errorf("k1 + k2*λ ≠ k:\n  sum: %s\n  k:   %s",
					hex.EncodeToString(sumBytes[:]), hex.EncodeToString(kBytes[:]))
			}
		})
	}
}

// TestScalarSplitLambdaBounds verifies that k1 and k2 are bounded (< 2^128)
func TestScalarSplitLambdaBounds(t *testing.T) {
	// Test with various random-ish scalars
	testScalars := []string{
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", // n/2
		"0000000000000000000000000000000100000000000000000000000000000000",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	// Bounds from libsecp256k1: k1 < 2^128 or -k1 < 2^128, same for k2
	// Actually the bounds are tighter: (a1 + a2 + 1)/2 for k1, (-b1 + b2)/2 + 1 for k2
	// But we'll just check they fit in ~128 bits (upper limbs are small)

	for _, hexStr := range testScalars {
		kBytes, _ := hex.DecodeString(hexStr)
		var k Scalar
		k.setB32(kBytes)

		var k1, k2 Scalar
		scalarSplitLambda(&k1, &k2, &k)

		// Check that k1 is small (upper two limbs should be 0 or the scalar is negated)
		// If k1 is "high", then -k1 should be small
		k1Small := (k1.d[2] == 0 && k1.d[3] == 0)
		var negK1 Scalar
		negK1.negate(&k1)
		negK1Small := (negK1.d[2] == 0 && negK1.d[3] == 0)

		if !k1Small && !negK1Small {
			t.Errorf("k1 not bounded for k=%s: k1.d[2]=%x, k1.d[3]=%x", hexStr, k1.d[2], k1.d[3])
		}

		// Same for k2
		k2Small := (k2.d[2] == 0 && k2.d[3] == 0)
		var negK2 Scalar
		negK2.negate(&k2)
		negK2Small := (negK2.d[2] == 0 && negK2.d[3] == 0)

		if !k2Small && !negK2Small {
			t.Errorf("k2 not bounded for k=%s: k2.d[2]=%x, k2.d[3]=%x", hexStr, k2.d[2], k2.d[3])
		}
	}
}

// TestScalarSplitLambdaRandom tests splitLambda with random scalars
func TestScalarSplitLambdaRandom(t *testing.T) {
	// Use deterministic "random" values based on hashing
	for i := 0; i < 100; i++ {
		// Create a pseudo-random scalar
		sha := NewSHA256()
		sha.Write([]byte{byte(i), byte(i >> 8)})
		var kBytes [32]byte
		sha.Finalize(kBytes[:])

		var k Scalar
		k.setB32(kBytes[:])

		// Split
		var k1, k2 Scalar
		scalarSplitLambda(&k1, &k2, &k)

		// Verify property
		var k2Lambda, sum Scalar
		k2Lambda.mul(&k2, &scalarLambda)
		sum.add(&k1, &k2Lambda)

		if !sum.equal(&k) {
			t.Errorf("Iteration %d: k1 + k2*λ ≠ k", i)
		}
	}
}

// TestScalarSplit128 tests the 128-bit split function
func TestScalarSplit128(t *testing.T) {
	// Test scalar: 0x0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20
	kBytes, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	var k Scalar
	k.setB32(kBytes)

	var k1, k2 Scalar
	scalarSplit128(&k1, &k2, &k)

	// k1 should be low 128 bits
	// k2 should be high 128 bits
	// In limb order: k.d[0], k.d[1] are low, k.d[2], k.d[3] are high
	if k1.d[0] != k.d[0] || k1.d[1] != k.d[1] || k1.d[2] != 0 || k1.d[3] != 0 {
		t.Errorf("k1 incorrect: got d[0:4]=%x,%x,%x,%x", k1.d[0], k1.d[1], k1.d[2], k1.d[3])
	}
	if k2.d[0] != k.d[2] || k2.d[1] != k.d[3] || k2.d[2] != 0 || k2.d[3] != 0 {
		t.Errorf("k2 incorrect: got d[0:4]=%x,%x,%x,%x", k2.d[0], k2.d[1], k2.d[2], k2.d[3])
	}
}

// TestMulShiftVar tests the mul_shift_var function
func TestMulShiftVar(t *testing.T) {
	// Test case: multiply two known values and shift by 384
	// This is the exact operation used in splitLambda

	// Simple test: 1 * g1 >> 384 should give a small result
	var result Scalar
	result.mulShiftVar(&ScalarOne, &scalarG1, 384)

	// The result should be 0 since 1 * g1 < 2^384
	// (g1 is a 256-bit value, so 1 * g1 = g1 which is < 2^256 < 2^384)
	if result.d[0] != 0 || result.d[1] != 0 || result.d[2] != 0 || result.d[3] != 0 {
		t.Errorf("1 * g1 >> 384 should be 0, got: %x %x %x %x",
			result.d[0], result.d[1], result.d[2], result.d[3])
	}

	// Test with a larger value that will produce non-zero result
	// Use a value close to n to get a meaningful result
	nMinus1Bytes, _ := hex.DecodeString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140")
	var nMinus1 Scalar
	nMinus1.setB32(nMinus1Bytes)

	result.mulShiftVar(&nMinus1, &scalarG1, 384)

	// Result should be non-zero and fit in ~128 bits
	// (since g1 is ~256 bits, (n-1)*g1 is ~512 bits, >> 384 gives ~128 bits)
	if result.d[2] != 0 || result.d[3] != 0 {
		t.Logf("Warning: result has high limbs set, may indicate issue")
	}
	t.Logf("(n-1) * g1 >> 384 = %x %x %x %x", result.d[3], result.d[2], result.d[1], result.d[0])
}

// TestIsHigh tests the isHigh function
func TestIsHigh(t *testing.T) {
	testCases := []struct {
		name     string
		hexValue string
		expected bool
	}{
		{"zero", "0000000000000000000000000000000000000000000000000000000000000000", false},
		{"one", "0000000000000000000000000000000000000000000000000000000000000001", false},
		{"n/2", "7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", false},
		{"n/2+1", "7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a1", true},
		{"n-1", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", true},
		{"n-2", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413f", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bytes, _ := hex.DecodeString(tc.hexValue)
			var s Scalar
			s.setB32(bytes)

			result := s.isHigh()
			if result != tc.expected {
				t.Errorf("isHigh(%s) = %v, want %v", tc.name, result, tc.expected)
			}
		})
	}
}

// BenchmarkScalarSplitLambda benchmarks the scalar splitting operation
func BenchmarkScalarSplitLambda(b *testing.B) {
	// Use a representative scalar
	kBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var k Scalar
	k.setB32(kBytes)

	var k1, k2 Scalar

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scalarSplitLambda(&k1, &k2, &k)
	}
}

// BenchmarkMulShiftVar benchmarks the mulShiftVar operation
func BenchmarkMulShiftVar(b *testing.B) {
	kBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var k Scalar
	k.setB32(kBytes)

	var result Scalar

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result.mulShiftVar(&k, &scalarG1, 384)
	}
}

// =============================================================================
// Phase 3 Tests: Point Operations with Endomorphism
// =============================================================================

// TestMulLambdaAffine tests the affine mulLambda operation
func TestMulLambdaAffine(t *testing.T) {
	// Test that mulLambda(G) produces the same result as λ*G via scalar multiplication
	var lambdaG GroupElementJacobian
	EcmultConst(&lambdaG, &Generator, &scalarLambda)

	var lambdaGAff GroupElementAffine
	lambdaGAff.setGEJ(&lambdaG)
	lambdaGAff.x.normalize()
	lambdaGAff.y.normalize()

	// Now test mulLambda
	var mulLambdaG GroupElementAffine
	mulLambdaG.mulLambda(&Generator)
	mulLambdaG.x.normalize()
	mulLambdaG.y.normalize()

	// They should be equal
	if !lambdaGAff.x.equal(&mulLambdaG.x) {
		var got, want [32]byte
		mulLambdaG.x.getB32(got[:])
		lambdaGAff.x.getB32(want[:])
		t.Errorf("mulLambda(G).x ≠ λ*G.x:\n  got:  %s\n  want: %s",
			hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}

	if !lambdaGAff.y.equal(&mulLambdaG.y) {
		var got, want [32]byte
		mulLambdaG.y.getB32(got[:])
		lambdaGAff.y.getB32(want[:])
		t.Errorf("mulLambda(G).y ≠ λ*G.y:\n  got:  %s\n  want: %s",
			hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}
}

// TestMulLambdaJacobian tests the Jacobian mulLambda operation
func TestMulLambdaJacobian(t *testing.T) {
	// Convert generator to Jacobian
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	// Apply mulLambda
	var mulLambdaG GroupElementJacobian
	mulLambdaG.mulLambda(&gJac)

	// Convert back to affine for comparison
	var mulLambdaGAff GroupElementAffine
	mulLambdaGAff.setGEJ(&mulLambdaG)
	mulLambdaGAff.x.normalize()
	mulLambdaGAff.y.normalize()

	// Compute expected via scalar multiplication
	var lambdaG GroupElementJacobian
	EcmultConst(&lambdaG, &Generator, &scalarLambda)
	var lambdaGAff GroupElementAffine
	lambdaGAff.setGEJ(&lambdaG)
	lambdaGAff.x.normalize()
	lambdaGAff.y.normalize()

	// They should be equal
	if !lambdaGAff.equal(&mulLambdaGAff) {
		t.Errorf("Jacobian mulLambda(G) ≠ λ*G")
	}
}

// TestMulLambdaInfinity tests that mulLambda handles infinity correctly
func TestMulLambdaInfinity(t *testing.T) {
	var inf GroupElementAffine
	inf.setInfinity()

	var result GroupElementAffine
	result.mulLambda(&inf)

	if !result.isInfinity() {
		t.Errorf("mulLambda(infinity) should be infinity")
	}

	// Jacobian version
	var infJac GroupElementJacobian
	infJac.setInfinity()

	var resultJac GroupElementJacobian
	resultJac.mulLambda(&infJac)

	if !resultJac.isInfinity() {
		t.Errorf("Jacobian mulLambda(infinity) should be infinity")
	}
}

// TestMulLambdaCubed tests that λ^3 * P = P (applying mulLambda 3 times returns to original)
func TestMulLambdaCubed(t *testing.T) {
	// Start with generator
	p := Generator

	// Apply mulLambda three times
	var p1, p2, p3 GroupElementAffine
	p1.mulLambda(&p)
	p2.mulLambda(&p1)
	p3.mulLambda(&p2)

	// Normalize for comparison
	p3.x.normalize()
	p3.y.normalize()

	genNorm := Generator
	genNorm.x.normalize()
	genNorm.y.normalize()

	// p3 should equal the original generator
	if !p3.equal(&genNorm) {
		var got, want [32]byte
		p3.x.getB32(got[:])
		genNorm.x.getB32(want[:])
		t.Errorf("λ^3 * G ≠ G:\n  got x:  %s\n  want x: %s",
			hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}
}

// TestEcmultEndoSplit tests the combined endomorphism split operation
func TestEcmultEndoSplit(t *testing.T) {
	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", // n/2
	}

	for _, hexStr := range testCases {
		t.Run(hexStr[:16]+"...", func(t *testing.T) {
			// Parse scalar
			sBytes, _ := hex.DecodeString(hexStr)
			var s Scalar
			s.setB32(sBytes)

			// Use generator as the point
			p := Generator

			// Split
			var s1, s2 Scalar
			var p1, p2 GroupElementAffine
			ecmultEndoSplit(&s1, &s2, &p1, &p2, &s, &p)

			// Verify s1 and s2 are not high (after negation adjustment)
			if s1.isHigh() {
				t.Errorf("s1 should not be high after split")
			}
			if s2.isHigh() {
				t.Errorf("s2 should not be high after split")
			}

			// Verify: s1*p1 + s2*p2 = s*p
			var s1p1, s2p2, sum, expected GroupElementJacobian

			EcmultConst(&s1p1, &p1, &s1)
			EcmultConst(&s2p2, &p2, &s2)
			sum.addVar(&s1p1, &s2p2)

			EcmultConst(&expected, &p, &s)

			// Convert to affine for comparison
			var sumAff, expectedAff GroupElementAffine
			sumAff.setGEJ(&sum)
			expectedAff.setGEJ(&expected)
			sumAff.x.normalize()
			sumAff.y.normalize()
			expectedAff.x.normalize()
			expectedAff.y.normalize()

			if !sumAff.equal(&expectedAff) {
				t.Errorf("s1*p1 + s2*p2 ≠ s*p")
			}
		})
	}
}

// TestEcmultEndoSplitRandom tests endomorphism split with random scalars
func TestEcmultEndoSplitRandom(t *testing.T) {
	for i := 0; i < 20; i++ {
		// Generate pseudo-random scalar
		sha := NewSHA256()
		sha.Write([]byte{byte(i), byte(i >> 8), 0xAB, 0xCD})
		var sBytes [32]byte
		sha.Finalize(sBytes[:])

		var s Scalar
		s.setB32(sBytes[:])

		// Use generator
		p := Generator

		// Split
		var s1, s2 Scalar
		var p1, p2 GroupElementAffine
		ecmultEndoSplit(&s1, &s2, &p1, &p2, &s, &p)

		// Verify: s1*p1 + s2*p2 = s*p
		var s1p1, s2p2, sum, expected GroupElementJacobian

		EcmultConst(&s1p1, &p1, &s1)
		EcmultConst(&s2p2, &p2, &s2)
		sum.addVar(&s1p1, &s2p2)

		EcmultConst(&expected, &p, &s)

		var sumAff, expectedAff GroupElementAffine
		sumAff.setGEJ(&sum)
		expectedAff.setGEJ(&expected)
		sumAff.x.normalize()
		sumAff.y.normalize()
		expectedAff.x.normalize()
		expectedAff.y.normalize()

		if !sumAff.equal(&expectedAff) {
			t.Errorf("Iteration %d: s1*p1 + s2*p2 ≠ s*p", i)
		}
	}
}

// BenchmarkMulLambdaAffine benchmarks the affine mulLambda operation
func BenchmarkMulLambdaAffine(b *testing.B) {
	p := Generator
	var result GroupElementAffine

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result.mulLambda(&p)
	}
}

// BenchmarkMulLambdaJacobian benchmarks the Jacobian mulLambda operation
func BenchmarkMulLambdaJacobian(b *testing.B) {
	var p GroupElementJacobian
	p.setGE(&Generator)
	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result.mulLambda(&p)
	}
}

// BenchmarkEcmultEndoSplit benchmarks the combined endomorphism split
func BenchmarkEcmultEndoSplit(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)
	p := Generator

	var s1, s2 Scalar
	var p1, p2 GroupElementAffine

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultEndoSplit(&s1, &s2, &p1, &p2, &s, &p)
	}
}

// =============================================================================
// Phase 4 Tests: Strauss-GLV Algorithm
// =============================================================================

// TestBuildOddMultiplesTableAffine tests the odd multiples table building
func TestBuildOddMultiplesTableAffine(t *testing.T) {
	// Build table from generator
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	const tableSize = 16 // Window size 5
	preA := make([]GroupElementAffine, tableSize)
	preBetaX := make([]FieldElement, tableSize)

	buildOddMultiplesTableAffine(preA, preBetaX, &gJac, tableSize)

	// Verify: preA[i] should equal (2*i+1)*G
	for i := 0; i < tableSize; i++ {
		var scalar Scalar
		scalar.setInt(uint(2*i + 1))

		var expected GroupElementJacobian
		EcmultConst(&expected, &Generator, &scalar)

		var expectedAff GroupElementAffine
		expectedAff.setGEJ(&expected)
		expectedAff.x.normalize()
		expectedAff.y.normalize()

		preA[i].x.normalize()
		preA[i].y.normalize()

		if !preA[i].equal(&expectedAff) {
			t.Errorf("preA[%d] ≠ %d*G", i, 2*i+1)
		}

		// Verify β*x is correct
		var expectedBetaX FieldElement
		expectedBetaX.mul(&expectedAff.x, &fieldBeta)
		expectedBetaX.normalize()
		preBetaX[i].normalize()

		if !preBetaX[i].equal(&expectedBetaX) {
			t.Errorf("preBetaX[%d] ≠ β * preA[%d].x", i, i)
		}
	}
}

// TestTableGetGE tests the table lookup function
func TestTableGetGE(t *testing.T) {
	// Build table from generator
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	const tableSize = 16
	preA := make([]GroupElementAffine, tableSize)
	preBetaX := make([]FieldElement, tableSize)

	buildOddMultiplesTableAffine(preA, preBetaX, &gJac, tableSize)

	testCases := []struct {
		name     string
		n        int
		expected int // expected multiple (negative means negated)
	}{
		{"n=1", 1, 1},
		{"n=3", 3, 3},
		{"n=5", 5, 5},
		{"n=15", 15, 15},
		{"n=-1", -1, -1},
		{"n=-3", -3, -3},
		{"n=-15", -15, -15},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pt GroupElementAffine
			tableGetGE(&pt, preA, tc.n)

			// Compute expected point
			absN := tc.expected
			if absN < 0 {
				absN = -absN
			}
			var scalar Scalar
			scalar.setInt(uint(absN))

			var expectedJac GroupElementJacobian
			EcmultConst(&expectedJac, &Generator, &scalar)

			var expectedAff GroupElementAffine
			expectedAff.setGEJ(&expectedJac)

			// Negate if needed
			if tc.expected < 0 {
				expectedAff.negate(&expectedAff)
			}

			expectedAff.x.normalize()
			expectedAff.y.normalize()
			pt.x.normalize()
			pt.y.normalize()

			if !pt.equal(&expectedAff) {
				t.Errorf("tableGetGE(%d) returned wrong point", tc.n)
			}
		})
	}

	// Test n=0 returns infinity
	var pt GroupElementAffine
	tableGetGE(&pt, preA, 0)
	if !pt.isInfinity() {
		t.Error("tableGetGE(0) should return infinity")
	}
}

// TestTableGetGELambda tests the lambda-transformed table lookup
func TestTableGetGELambda(t *testing.T) {
	// Build table from generator
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	const tableSize = 16
	preA := make([]GroupElementAffine, tableSize)
	preBetaX := make([]FieldElement, tableSize)

	buildOddMultiplesTableAffine(preA, preBetaX, &gJac, tableSize)

	// Test that tableGetGELambda(n) returns λ * tableGetGE(n)
	for n := 1; n <= 15; n += 2 {
		var ptLambda GroupElementAffine
		tableGetGELambda(&ptLambda, preA, preBetaX, n)

		// Get regular point and apply lambda
		var pt GroupElementAffine
		tableGetGE(&pt, preA, n)
		var expected GroupElementAffine
		expected.mulLambda(&pt)

		ptLambda.x.normalize()
		ptLambda.y.normalize()
		expected.x.normalize()
		expected.y.normalize()

		if !ptLambda.equal(&expected) {
			t.Errorf("tableGetGELambda(%d) ≠ λ * tableGetGE(%d)", n, n)
		}

		// Test negative n
		tableGetGELambda(&ptLambda, preA, preBetaX, -n)
		tableGetGE(&pt, preA, -n)
		expected.mulLambda(&pt)

		ptLambda.x.normalize()
		ptLambda.y.normalize()
		expected.x.normalize()
		expected.y.normalize()

		if !ptLambda.equal(&expected) {
			t.Errorf("tableGetGELambda(%d) ≠ λ * tableGetGE(%d)", -n, -n)
		}
	}
}

// TestEcmultStraussWNAFGLV tests the full Strauss-GLV algorithm
func TestEcmultStraussWNAFGLV(t *testing.T) {
	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001", // 1
		"0000000000000000000000000000000000000000000000000000000000000002", // 2
		"0000000000000000000000000000000000000000000000000000000000000010", // 16
		"00000000000000000000000000000000000000000000000000000000000000ff", // 255
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", // random
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", // n/2
	}

	for _, hexStr := range testCases {
		t.Run(hexStr[:8]+"...", func(t *testing.T) {
			// Parse scalar
			sBytes, _ := hex.DecodeString(hexStr)
			var s Scalar
			s.setB32(sBytes)

			// Compute using Strauss-GLV
			var resultGLV GroupElementJacobian
			ecmultStraussWNAFGLV(&resultGLV, &Generator, &s)

			// Compute using reference implementation
			var resultRef GroupElementJacobian
			EcmultConst(&resultRef, &Generator, &s)

			// Convert to affine for comparison
			var resultGLVAff, resultRefAff GroupElementAffine
			resultGLVAff.setGEJ(&resultGLV)
			resultRefAff.setGEJ(&resultRef)
			resultGLVAff.x.normalize()
			resultGLVAff.y.normalize()
			resultRefAff.x.normalize()
			resultRefAff.y.normalize()

			if !resultGLVAff.equal(&resultRefAff) {
				var gotX, wantX [32]byte
				resultGLVAff.x.getB32(gotX[:])
				resultRefAff.x.getB32(wantX[:])
				t.Errorf("GLV result mismatch:\n  got x:  %s\n  want x: %s",
					hex.EncodeToString(gotX[:]), hex.EncodeToString(wantX[:]))
			}
		})
	}
}

// TestEcmultStraussWNAFGLVRandom tests the Strauss-GLV algorithm with random scalars
func TestEcmultStraussWNAFGLVRandom(t *testing.T) {
	for i := 0; i < 50; i++ {
		// Generate pseudo-random scalar
		sha := NewSHA256()
		sha.Write([]byte{byte(i), byte(i >> 8), 0xDE, 0xAD})
		var sBytes [32]byte
		sha.Finalize(sBytes[:])

		var s Scalar
		s.setB32(sBytes[:])

		// Compute using Strauss-GLV
		var resultGLV GroupElementJacobian
		ecmultStraussWNAFGLV(&resultGLV, &Generator, &s)

		// Compute using reference implementation
		var resultRef GroupElementJacobian
		EcmultConst(&resultRef, &Generator, &s)

		// Convert to affine for comparison
		var resultGLVAff, resultRefAff GroupElementAffine
		resultGLVAff.setGEJ(&resultGLV)
		resultRefAff.setGEJ(&resultRef)
		resultGLVAff.x.normalize()
		resultGLVAff.y.normalize()
		resultRefAff.x.normalize()
		resultRefAff.y.normalize()

		if !resultGLVAff.equal(&resultRefAff) {
			t.Errorf("Iteration %d: GLV result mismatch", i)
		}
	}
}

// TestEcmultStraussWNAFGLVNonGenerator tests with a non-generator point
func TestEcmultStraussWNAFGLVNonGenerator(t *testing.T) {
	// Create a non-generator point: 2*G
	var two Scalar
	two.setInt(2)
	var twoGJac GroupElementJacobian
	EcmultConst(&twoGJac, &Generator, &two)
	var twoG GroupElementAffine
	twoG.setGEJ(&twoGJac)

	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000003",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
	}

	for _, hexStr := range testCases {
		t.Run(hexStr[:8]+"...", func(t *testing.T) {
			sBytes, _ := hex.DecodeString(hexStr)
			var s Scalar
			s.setB32(sBytes)

			// Compute using Strauss-GLV
			var resultGLV GroupElementJacobian
			ecmultStraussWNAFGLV(&resultGLV, &twoG, &s)

			// Compute using reference implementation
			var resultRef GroupElementJacobian
			EcmultConst(&resultRef, &twoG, &s)

			// Convert to affine for comparison
			var resultGLVAff, resultRefAff GroupElementAffine
			resultGLVAff.setGEJ(&resultGLV)
			resultRefAff.setGEJ(&resultRef)
			resultGLVAff.x.normalize()
			resultGLVAff.y.normalize()
			resultRefAff.x.normalize()
			resultRefAff.y.normalize()

			if !resultGLVAff.equal(&resultRefAff) {
				t.Errorf("GLV result mismatch for 2*G")
			}
		})
	}
}

// TestEcmultStraussWNAFGLVEdgeCases tests edge cases
func TestEcmultStraussWNAFGLVEdgeCases(t *testing.T) {
	// Test with zero scalar
	var zero Scalar
	var result GroupElementJacobian
	ecmultStraussWNAFGLV(&result, &Generator, &zero)
	if !result.isInfinity() {
		t.Error("0 * G should be infinity")
	}

	// Test with infinity point
	var inf GroupElementAffine
	inf.setInfinity()
	var one Scalar
	one.setInt(1)
	ecmultStraussWNAFGLV(&result, &inf, &one)
	if !result.isInfinity() {
		t.Error("1 * infinity should be infinity")
	}
}

// BenchmarkEcmultStraussWNAFGLV benchmarks the Strauss-GLV algorithm
func BenchmarkEcmultStraussWNAFGLV(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultStraussWNAFGLV(&result, &Generator, &s)
	}
}

// BenchmarkEcmultConst benchmarks the reference constant-time implementation
func BenchmarkEcmultConst(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EcmultConst(&result, &Generator, &s)
	}
}

// BenchmarkEcmultWindowedVar benchmarks the windowed variable-time implementation
func BenchmarkEcmultWindowedVar(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultWindowedVar(&result, &Generator, &s)
	}
}

// BenchmarkBuildOddMultiplesTableAffine benchmarks table building
func BenchmarkBuildOddMultiplesTableAffine(b *testing.B) {
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	const tableSize = 16
	preA := make([]GroupElementAffine, tableSize)
	preBetaX := make([]FieldElement, tableSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildOddMultiplesTableAffine(preA, preBetaX, &gJac, tableSize)
	}
}

// =============================================================================
// Phase 5 Tests: Generator Precomputation
// =============================================================================

// TestGenTablesInitialization tests that the generator tables are properly initialized
func TestGenTablesInitialization(t *testing.T) {
	// Force initialization
	EnsureGenTablesInitialized()

	// Verify preGenG[0] = 1*G = G
	preGenG[0].x.normalize()
	preGenG[0].y.normalize()
	genNorm := Generator
	genNorm.x.normalize()
	genNorm.y.normalize()

	if !preGenG[0].equal(&genNorm) {
		t.Error("preGenG[0] should equal G")
	}

	// Verify preGenG[i] = (2*i+1)*G
	for i := 0; i < 8; i++ { // Check first 8 entries
		var scalar Scalar
		scalar.setInt(uint(2*i + 1))

		var expected GroupElementJacobian
		EcmultConst(&expected, &Generator, &scalar)

		var expectedAff GroupElementAffine
		expectedAff.setGEJ(&expected)
		expectedAff.x.normalize()
		expectedAff.y.normalize()

		preGenG[i].x.normalize()
		preGenG[i].y.normalize()

		if !preGenG[i].equal(&expectedAff) {
			t.Errorf("preGenG[%d] ≠ %d*G", i, 2*i+1)
		}
	}

	// Verify preGenLambdaG[0] = λ*G
	var lambdaG GroupElementAffine
	lambdaG.mulLambda(&Generator)
	lambdaG.x.normalize()
	lambdaG.y.normalize()

	preGenLambdaG[0].x.normalize()
	preGenLambdaG[0].y.normalize()

	if !preGenLambdaG[0].equal(&lambdaG) {
		t.Error("preGenLambdaG[0] should equal λ*G")
	}

	// Verify preGenLambdaG[i] = (2*i+1)*(λ*G)
	for i := 0; i < 8; i++ {
		var scalar Scalar
		scalar.setInt(uint(2*i + 1))

		var expected GroupElementJacobian
		EcmultConst(&expected, &lambdaG, &scalar)

		var expectedAff GroupElementAffine
		expectedAff.setGEJ(&expected)
		expectedAff.x.normalize()
		expectedAff.y.normalize()

		preGenLambdaG[i].x.normalize()
		preGenLambdaG[i].y.normalize()

		if !preGenLambdaG[i].equal(&expectedAff) {
			t.Errorf("preGenLambdaG[%d] ≠ %d*(λ*G)", i, 2*i+1)
		}
	}
}

// TestEcmultGenGLV tests the GLV generator multiplication
func TestEcmultGenGLV(t *testing.T) {
	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001", // 1
		"0000000000000000000000000000000000000000000000000000000000000002", // 2
		"0000000000000000000000000000000000000000000000000000000000000010", // 16
		"00000000000000000000000000000000000000000000000000000000000000ff", // 255
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", // random
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140", // n-1
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0", // n/2
	}

	for _, hexStr := range testCases {
		t.Run(hexStr[:8]+"...", func(t *testing.T) {
			// Parse scalar
			sBytes, _ := hex.DecodeString(hexStr)
			var s Scalar
			s.setB32(sBytes)

			// Compute using GLV generator multiplication
			var resultGLV GroupElementJacobian
			ecmultGenGLV(&resultGLV, &s)

			// Compute using reference implementation
			var resultRef GroupElementJacobian
			EcmultConst(&resultRef, &Generator, &s)

			// Convert to affine for comparison
			var resultGLVAff, resultRefAff GroupElementAffine
			resultGLVAff.setGEJ(&resultGLV)
			resultRefAff.setGEJ(&resultRef)
			resultGLVAff.x.normalize()
			resultGLVAff.y.normalize()
			resultRefAff.x.normalize()
			resultRefAff.y.normalize()

			if !resultGLVAff.equal(&resultRefAff) {
				var gotX, wantX [32]byte
				resultGLVAff.x.getB32(gotX[:])
				resultRefAff.x.getB32(wantX[:])
				t.Errorf("GLV gen result mismatch:\n  got x:  %s\n  want x: %s",
					hex.EncodeToString(gotX[:]), hex.EncodeToString(wantX[:]))
			}
		})
	}
}

// TestEcmultGenGLVRandom tests GLV generator multiplication with random scalars
func TestEcmultGenGLVRandom(t *testing.T) {
	for i := 0; i < 50; i++ {
		// Generate pseudo-random scalar
		sha := NewSHA256()
		sha.Write([]byte{byte(i), byte(i >> 8), 0xBE, 0xEF})
		var sBytes [32]byte
		sha.Finalize(sBytes[:])

		var s Scalar
		s.setB32(sBytes[:])

		// Compute using GLV
		var resultGLV GroupElementJacobian
		ecmultGenGLV(&resultGLV, &s)

		// Compute using reference
		var resultRef GroupElementJacobian
		EcmultConst(&resultRef, &Generator, &s)

		// Compare
		var resultGLVAff, resultRefAff GroupElementAffine
		resultGLVAff.setGEJ(&resultGLV)
		resultRefAff.setGEJ(&resultRef)
		resultGLVAff.x.normalize()
		resultGLVAff.y.normalize()
		resultRefAff.x.normalize()
		resultRefAff.y.normalize()

		if !resultGLVAff.equal(&resultRefAff) {
			t.Errorf("Iteration %d: GLV gen result mismatch", i)
		}
	}
}

// TestEcmultGenSimple tests the simple generator multiplication
func TestEcmultGenSimple(t *testing.T) {
	testCases := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"00000000000000000000000000000000000000000000000000000000000000ff",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
	}

	for _, hexStr := range testCases {
		t.Run(hexStr[:8]+"...", func(t *testing.T) {
			sBytes, _ := hex.DecodeString(hexStr)
			var s Scalar
			s.setB32(sBytes)

			// Compute using simple generator multiplication
			var resultSimple GroupElementJacobian
			ecmultGenSimple(&resultSimple, &s)

			// Compute using reference
			var resultRef GroupElementJacobian
			EcmultConst(&resultRef, &Generator, &s)

			// Compare
			var resultSimpleAff, resultRefAff GroupElementAffine
			resultSimpleAff.setGEJ(&resultSimple)
			resultRefAff.setGEJ(&resultRef)
			resultSimpleAff.x.normalize()
			resultSimpleAff.y.normalize()
			resultRefAff.x.normalize()
			resultRefAff.y.normalize()

			if !resultSimpleAff.equal(&resultRefAff) {
				t.Errorf("Simple gen result mismatch")
			}
		})
	}
}

// TestEcmultGenGLVEdgeCases tests edge cases
func TestEcmultGenGLVEdgeCases(t *testing.T) {
	// Test with zero scalar
	var zero Scalar
	var result GroupElementJacobian
	ecmultGenGLV(&result, &zero)
	if !result.isInfinity() {
		t.Error("0 * G should be infinity")
	}

	// Test with scalar 1
	var one Scalar
	one.setInt(1)
	ecmultGenGLV(&result, &one)

	var resultAff GroupElementAffine
	resultAff.setGEJ(&result)
	resultAff.x.normalize()
	resultAff.y.normalize()

	genNorm := Generator
	genNorm.x.normalize()
	genNorm.y.normalize()

	if !resultAff.equal(&genNorm) {
		t.Error("1 * G should equal G")
	}
}

// BenchmarkEcmultGenGLV benchmarks the GLV generator multiplication
func BenchmarkEcmultGenGLV(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	// Ensure tables are initialized before benchmark
	EnsureGenTablesInitialized()

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultGenGLV(&result, &s)
	}
}

// BenchmarkEcmultGenSimple benchmarks the simple generator multiplication
func BenchmarkEcmultGenSimple(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	// Ensure tables are initialized before benchmark
	EnsureGenTablesInitialized()

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultGenSimple(&result, &s)
	}
}

// BenchmarkEcmultGenConstRef benchmarks the reference constant-time implementation for G
func BenchmarkEcmultGenConstRef(b *testing.B) {
	sBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	var s Scalar
	s.setB32(sBytes)

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EcmultConst(&result, &Generator, &s)
	}
}

// =============================================================================
// Phase 6: Cross-Validation Tests
// =============================================================================

// TestCrossValidationAllImplementations verifies that all scalar multiplication
// implementations produce the same results
func TestCrossValidationAllImplementations(t *testing.T) {
	testCases := []struct {
		name   string
		scalar string
	}{
		{"one", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"two", "0000000000000000000000000000000000000000000000000000000000000002"},
		{"random1", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"random2", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef"},
		{"high_bits", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
		{"half_order", "7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0"},
		{"small", "0000000000000000000000000000000000000000000000000000000000000010"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sBytes, _ := hex.DecodeString(tc.scalar)
			var s Scalar
			s.setB32(sBytes)

			if s.isZero() {
				return // Skip zero scalar
			}

			// Reference: EcmultConst (constant-time binary method)
			var refResult GroupElementJacobian
			EcmultConst(&refResult, &Generator, &s)
			var refAff GroupElementAffine
			refAff.setGEJ(&refResult)
			refAff.x.normalize()
			refAff.y.normalize()

			// Test 1: ecmultGenGLV (GLV generator multiplication)
			var glvResult GroupElementJacobian
			ecmultGenGLV(&glvResult, &s)
			var glvAff GroupElementAffine
			glvAff.setGEJ(&glvResult)
			glvAff.x.normalize()
			glvAff.y.normalize()

			if !refAff.x.equal(&glvAff.x) || !refAff.y.equal(&glvAff.y) {
				t.Errorf("ecmultGenGLV mismatch for %s", tc.name)
			}

			// Test 2: ecmultGenSimple (simple generator multiplication)
			var simpleResult GroupElementJacobian
			ecmultGenSimple(&simpleResult, &s)
			var simpleAff GroupElementAffine
			simpleAff.setGEJ(&simpleResult)
			simpleAff.x.normalize()
			simpleAff.y.normalize()

			if !refAff.x.equal(&simpleAff.x) || !refAff.y.equal(&simpleAff.y) {
				t.Errorf("ecmultGenSimple mismatch for %s", tc.name)
			}

			// Test 3: EcmultGen (public interface)
			var publicResult GroupElementJacobian
			EcmultGen(&publicResult, &s)
			var publicAff GroupElementAffine
			publicAff.setGEJ(&publicResult)
			publicAff.x.normalize()
			publicAff.y.normalize()

			if !refAff.x.equal(&publicAff.x) || !refAff.y.equal(&publicAff.y) {
				t.Errorf("EcmultGen mismatch for %s", tc.name)
			}

			// Test 4: ecmultStraussWNAFGLV with Generator
			var straussResult GroupElementJacobian
			ecmultStraussWNAFGLV(&straussResult, &Generator, &s)
			var straussAff GroupElementAffine
			straussAff.setGEJ(&straussResult)
			straussAff.x.normalize()
			straussAff.y.normalize()

			if !refAff.x.equal(&straussAff.x) || !refAff.y.equal(&straussAff.y) {
				t.Errorf("ecmultStraussWNAFGLV mismatch for %s", tc.name)
			}

			// Test 5: Ecmult (public interface with Jacobian input)
			var genJac GroupElementJacobian
			genJac.setGE(&Generator)
			var ecmultResult GroupElementJacobian
			Ecmult(&ecmultResult, &genJac, &s)
			var ecmultAff GroupElementAffine
			ecmultAff.setGEJ(&ecmultResult)
			ecmultAff.x.normalize()
			ecmultAff.y.normalize()

			if !refAff.x.equal(&ecmultAff.x) || !refAff.y.equal(&ecmultAff.y) {
				t.Errorf("Ecmult mismatch for %s", tc.name)
			}

			// Test 6: ecmultWindowedVar
			var windowedResult GroupElementJacobian
			ecmultWindowedVar(&windowedResult, &Generator, &s)
			var windowedAff GroupElementAffine
			windowedAff.setGEJ(&windowedResult)
			windowedAff.x.normalize()
			windowedAff.y.normalize()

			if !refAff.x.equal(&windowedAff.x) || !refAff.y.equal(&windowedAff.y) {
				t.Errorf("ecmultWindowedVar mismatch for %s", tc.name)
			}
		})
	}
}

// TestCrossValidationArbitraryPoint tests all implementations with a non-generator point
func TestCrossValidationArbitraryPoint(t *testing.T) {
	// Create an arbitrary point P = 7*G
	var sevenScalar Scalar
	sevenScalar.d[0] = 7

	var pJac GroupElementJacobian
	EcmultConst(&pJac, &Generator, &sevenScalar)
	var p GroupElementAffine
	p.setGEJ(&pJac)
	p.x.normalize()
	p.y.normalize()

	testCases := []struct {
		name   string
		scalar string
	}{
		{"one", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"random1", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"random2", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sBytes, _ := hex.DecodeString(tc.scalar)
			var s Scalar
			s.setB32(sBytes)

			if s.isZero() {
				return
			}

			// Reference: EcmultConst
			var refResult GroupElementJacobian
			EcmultConst(&refResult, &p, &s)
			var refAff GroupElementAffine
			refAff.setGEJ(&refResult)
			refAff.x.normalize()
			refAff.y.normalize()

			// Test 1: ecmultStraussWNAFGLV
			var straussResult GroupElementJacobian
			ecmultStraussWNAFGLV(&straussResult, &p, &s)
			var straussAff GroupElementAffine
			straussAff.setGEJ(&straussResult)
			straussAff.x.normalize()
			straussAff.y.normalize()

			if !refAff.x.equal(&straussAff.x) || !refAff.y.equal(&straussAff.y) {
				t.Errorf("ecmultStraussWNAFGLV mismatch for arbitrary point with %s", tc.name)
			}

			// Test 2: ecmultWindowedVar
			var windowedResult GroupElementJacobian
			ecmultWindowedVar(&windowedResult, &p, &s)
			var windowedAff GroupElementAffine
			windowedAff.setGEJ(&windowedResult)
			windowedAff.x.normalize()
			windowedAff.y.normalize()

			if !refAff.x.equal(&windowedAff.x) || !refAff.y.equal(&windowedAff.y) {
				t.Errorf("ecmultWindowedVar mismatch for arbitrary point with %s", tc.name)
			}

			// Test 3: Ecmult (public interface)
			var pJac2 GroupElementJacobian
			pJac2.setGE(&p)
			var ecmultResult GroupElementJacobian
			Ecmult(&ecmultResult, &pJac2, &s)
			var ecmultAff GroupElementAffine
			ecmultAff.setGEJ(&ecmultResult)
			ecmultAff.x.normalize()
			ecmultAff.y.normalize()

			if !refAff.x.equal(&ecmultAff.x) || !refAff.y.equal(&ecmultAff.y) {
				t.Errorf("Ecmult mismatch for arbitrary point with %s", tc.name)
			}
		})
	}
}

// TestCrossValidationRandomScalars tests with random scalars
func TestCrossValidationRandomScalars(t *testing.T) {
	seed := uint64(0xdeadbeef12345678)

	for i := 0; i < 50; i++ {
		// Generate pseudo-random scalar
		var sBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			sBytes[j] = byte(seed >> 56)
		}

		var s Scalar
		s.setB32(sBytes[:])

		if s.isZero() {
			continue
		}

		// Reference
		var refResult GroupElementJacobian
		EcmultConst(&refResult, &Generator, &s)
		var refAff GroupElementAffine
		refAff.setGEJ(&refResult)
		refAff.x.normalize()
		refAff.y.normalize()

		// GLV
		var glvResult GroupElementJacobian
		ecmultGenGLV(&glvResult, &s)
		var glvAff GroupElementAffine
		glvAff.setGEJ(&glvResult)
		glvAff.x.normalize()
		glvAff.y.normalize()

		if !refAff.x.equal(&glvAff.x) || !refAff.y.equal(&glvAff.y) {
			var sHex [32]byte
			s.getB32(sHex[:])
			t.Errorf("Random test %d failed: scalar=%s", i, hex.EncodeToString(sHex[:]))
		}
	}
}

// =============================================================================
// Phase 6: Property-Based Tests
// =============================================================================

// TestPropertyScalarSplitReconstruction verifies that k1 + k2*λ ≡ k (mod n)
func TestPropertyScalarSplitReconstruction(t *testing.T) {
	seed := uint64(0xcafebabe98765432)

	for i := 0; i < 100; i++ {
		// Generate pseudo-random scalar
		var kBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			kBytes[j] = byte(seed >> 56)
		}

		var k Scalar
		k.setB32(kBytes[:])

		if k.isZero() {
			continue
		}

		// Split k
		var k1, k2 Scalar
		scalarSplitLambda(&k1, &k2, &k)

		// Compute k2 * λ
		var k2Lambda Scalar
		k2Lambda.mul(&k2, &scalarLambda)

		// Compute k1 + k2*λ
		var reconstructed Scalar
		reconstructed.add(&k1, &k2Lambda)

		// Verify k1 + k2*λ ≡ k (mod n)
		if !reconstructed.equal(&k) {
			var kHex, rHex [32]byte
			k.getB32(kHex[:])
			reconstructed.getB32(rHex[:])
			t.Errorf("Scalar split reconstruction failed:\n  k=%s\n  reconstructed=%s",
				hex.EncodeToString(kHex[:]), hex.EncodeToString(rHex[:]))
		}
	}
}

// TestPropertyLambdaEndomorphism verifies that λ·P = (β·x, y) for points on the curve
func TestPropertyLambdaEndomorphism(t *testing.T) {
	seed := uint64(0x1234567890abcdef)

	for i := 0; i < 50; i++ {
		// Generate a random point by multiplying G by a random scalar
		var sBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			sBytes[j] = byte(seed >> 56)
		}

		var s Scalar
		s.setB32(sBytes[:])

		if s.isZero() {
			continue
		}

		// P = s * G
		var pJac GroupElementJacobian
		EcmultConst(&pJac, &Generator, &s)
		var p GroupElementAffine
		p.setGEJ(&pJac)
		p.x.normalize()
		p.y.normalize()

		if p.isInfinity() {
			continue
		}

		// Method 1: λ·P via scalar multiplication
		var lambdaPJac GroupElementJacobian
		EcmultConst(&lambdaPJac, &p, &scalarLambda)
		var lambdaP1 GroupElementAffine
		lambdaP1.setGEJ(&lambdaPJac)
		lambdaP1.x.normalize()
		lambdaP1.y.normalize()

		// Method 2: λ·P via endomorphism (β·x, y)
		var lambdaP2 GroupElementAffine
		lambdaP2.mulLambda(&p)
		lambdaP2.x.normalize()
		lambdaP2.y.normalize()

		// Verify they match
		if !lambdaP1.x.equal(&lambdaP2.x) || !lambdaP1.y.equal(&lambdaP2.y) {
			t.Errorf("Lambda endomorphism mismatch at iteration %d", i)
		}
	}
}

// TestPropertyBetaCubeRoot verifies that β^3 ≡ 1 (mod p)
func TestPropertyBetaCubeRoot(t *testing.T) {
	// Compute β^2
	var beta2 FieldElement
	beta2.mul(&fieldBeta, &fieldBeta)

	// Compute β^3 = β^2 * β
	var beta3 FieldElement
	beta3.mul(&beta2, &fieldBeta)
	beta3.normalize()

	// Check if β^3 ≡ 1 (mod p)
	if !beta3.equal(&FieldElementOne) {
		var bytes [32]byte
		beta3.getB32(bytes[:])
		t.Errorf("β^3 ≠ 1 (mod p), got: %s", hex.EncodeToString(bytes[:]))
	}
}

// TestPropertyDoubleNegate verifies that -(-P) = P
func TestPropertyDoubleNegate(t *testing.T) {
	seed := uint64(0xfedcba0987654321)

	for i := 0; i < 50; i++ {
		// Generate a random point
		var sBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			sBytes[j] = byte(seed >> 56)
		}

		var s Scalar
		s.setB32(sBytes[:])

		if s.isZero() {
			continue
		}

		var pJac GroupElementJacobian
		ecmultGenGLV(&pJac, &s)
		var p GroupElementAffine
		p.setGEJ(&pJac)
		p.x.normalize()
		p.y.normalize()

		if p.isInfinity() {
			continue
		}

		// Negate twice
		var negP, negNegP GroupElementAffine
		negP.negate(&p)
		negNegP.negate(&negP)
		negNegP.x.normalize()
		negNegP.y.normalize()

		// Verify -(-P) = P
		if !p.x.equal(&negNegP.x) || !p.y.equal(&negNegP.y) {
			t.Errorf("Double negation property failed at iteration %d", i)
		}
	}
}

// TestPropertyScalarAddition verifies that (a+b)*G = a*G + b*G
func TestPropertyScalarAddition(t *testing.T) {
	seed := uint64(0xabcdef1234567890)

	for i := 0; i < 50; i++ {
		// Generate two random scalars
		var aBytes, bBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			aBytes[j] = byte(seed >> 56)
			seed = seed*6364136223846793005 + 1442695040888963407
			bBytes[j] = byte(seed >> 56)
		}

		var a, b Scalar
		a.setB32(aBytes[:])
		b.setB32(bBytes[:])

		// Compute (a+b)*G
		var sum Scalar
		sum.add(&a, &b)

		var sumG GroupElementJacobian
		ecmultGenGLV(&sumG, &sum)
		var sumGAff GroupElementAffine
		sumGAff.setGEJ(&sumG)
		sumGAff.x.normalize()
		sumGAff.y.normalize()

		// Compute a*G + b*G
		var aG, bG GroupElementJacobian
		ecmultGenGLV(&aG, &a)
		ecmultGenGLV(&bG, &b)

		var aGplusBG GroupElementJacobian
		aGplusBG.addVar(&aG, &bG)
		var aGplusBGAff GroupElementAffine
		aGplusBGAff.setGEJ(&aGplusBG)
		aGplusBGAff.x.normalize()
		aGplusBGAff.y.normalize()

		// Verify (a+b)*G = a*G + b*G
		if sumGAff.isInfinity() != aGplusBGAff.isInfinity() {
			t.Errorf("Scalar addition property failed at iteration %d (infinity mismatch)", i)
			continue
		}
		if !sumGAff.isInfinity() {
			if !sumGAff.x.equal(&aGplusBGAff.x) || !sumGAff.y.equal(&aGplusBGAff.y) {
				t.Errorf("Scalar addition property failed at iteration %d", i)
			}
		}
	}
}

// TestPropertyScalarMultiplication verifies that (a*b)*G = a*(b*G)
func TestPropertyScalarMultiplication(t *testing.T) {
	seed := uint64(0x9876543210fedcba)

	for i := 0; i < 50; i++ {
		// Generate two random scalars
		var aBytes, bBytes [32]byte
		for j := 0; j < 32; j++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			aBytes[j] = byte(seed >> 56)
			seed = seed*6364136223846793005 + 1442695040888963407
			bBytes[j] = byte(seed >> 56)
		}

		var a, b Scalar
		a.setB32(aBytes[:])
		b.setB32(bBytes[:])

		// Compute (a*b)*G
		var product Scalar
		product.mul(&a, &b)

		var productG GroupElementJacobian
		ecmultGenGLV(&productG, &product)
		var productGAff GroupElementAffine
		productGAff.setGEJ(&productG)
		productGAff.x.normalize()
		productGAff.y.normalize()

		// Compute a*(b*G)
		var bG GroupElementJacobian
		ecmultGenGLV(&bG, &b)
		var bGAff GroupElementAffine
		bGAff.setGEJ(&bG)
		bGAff.x.normalize()
		bGAff.y.normalize()

		var aBG GroupElementJacobian
		ecmultStraussWNAFGLV(&aBG, &bGAff, &a)
		var aBGAff GroupElementAffine
		aBGAff.setGEJ(&aBG)
		aBGAff.x.normalize()
		aBGAff.y.normalize()

		// Verify (a*b)*G = a*(b*G)
		if productGAff.isInfinity() != aBGAff.isInfinity() {
			t.Errorf("Scalar multiplication property failed at iteration %d (infinity mismatch)", i)
			continue
		}
		if !productGAff.isInfinity() {
			if !productGAff.x.equal(&aBGAff.x) || !productGAff.y.equal(&aBGAff.y) {
				t.Errorf("Scalar multiplication property failed at iteration %d", i)
			}
		}
	}
}

// TestEcmultCombined tests the combined na*a + ng*G function
func TestEcmultCombined(t *testing.T) {
	testCases := []struct {
		name string
		na   string
		ng   string
	}{
		{"both_one", "0000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"na_only", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", "0000000000000000000000000000000000000000000000000000000000000000"},
		{"ng_only", "0000000000000000000000000000000000000000000000000000000000000000", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
		{"random_both", "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
	}

	// Create an arbitrary point P = 7*G
	var sevenScalar Scalar
	sevenScalar.d[0] = 7
	var pJac GroupElementJacobian
	EcmultConst(&pJac, &Generator, &sevenScalar)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			naBytes, _ := hex.DecodeString(tc.na)
			ngBytes, _ := hex.DecodeString(tc.ng)

			var na, ng Scalar
			na.setB32(naBytes)
			ng.setB32(ngBytes)

			// Compute using EcmultCombined
			var combined GroupElementJacobian
			EcmultCombined(&combined, &pJac, &na, &ng)
			var combinedAff GroupElementAffine
			combinedAff.setGEJ(&combined)
			combinedAff.x.normalize()
			combinedAff.y.normalize()

			// Compute reference: na*P + ng*G separately
			var naP, ngG, ref GroupElementJacobian

			if !na.isZero() {
				var pAff GroupElementAffine
				pAff.setGEJ(&pJac)
				EcmultConst(&naP, &pAff, &na)
			} else {
				naP.setInfinity()
			}

			if !ng.isZero() {
				EcmultConst(&ngG, &Generator, &ng)
			} else {
				ngG.setInfinity()
			}

			ref.addVar(&naP, &ngG)
			var refAff GroupElementAffine
			refAff.setGEJ(&ref)
			refAff.x.normalize()
			refAff.y.normalize()

			// Verify
			if combinedAff.isInfinity() != refAff.isInfinity() {
				t.Errorf("EcmultCombined infinity mismatch for %s", tc.name)
				return
			}

			if !combinedAff.isInfinity() {
				if !combinedAff.x.equal(&refAff.x) || !combinedAff.y.equal(&refAff.y) {
					t.Errorf("EcmultCombined result mismatch for %s", tc.name)
				}
			}
		})
	}
}

// BenchmarkEcmultCombined benchmarks the combined multiplication
func BenchmarkEcmultCombined(b *testing.B) {
	naBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	ngBytes, _ := hex.DecodeString("deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef")

	var na, ng Scalar
	na.setB32(naBytes)
	ng.setB32(ngBytes)

	// Create point P = 7*G
	var sevenScalar Scalar
	sevenScalar.d[0] = 7
	var pJac GroupElementJacobian
	EcmultConst(&pJac, &Generator, &sevenScalar)

	EnsureGenTablesInitialized()

	var result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EcmultCombined(&result, &pJac, &na, &ng)
	}
}

// BenchmarkEcmultSeparate benchmarks separate na*P and ng*G computations
func BenchmarkEcmultSeparate(b *testing.B) {
	naBytes, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	ngBytes, _ := hex.DecodeString("deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef")

	var na, ng Scalar
	na.setB32(naBytes)
	ng.setB32(ngBytes)

	// Create point P = 7*G
	var sevenScalar Scalar
	sevenScalar.d[0] = 7
	var pJac GroupElementJacobian
	EcmultConst(&pJac, &Generator, &sevenScalar)
	var pAff GroupElementAffine
	pAff.setGEJ(&pJac)

	EnsureGenTablesInitialized()

	var naP, ngG, result GroupElementJacobian

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ecmultStraussWNAFGLV(&naP, &pAff, &na)
		ecmultGenGLV(&ngG, &ng)
		result.addVar(&naP, &ngG)
	}
}
