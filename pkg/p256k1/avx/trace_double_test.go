package avx

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestGeneratorConstants(t *testing.T) {
	// Verify the generator X and Y constants
	expectedGx := "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	expectedGy := "483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8"

	gx := Generator.X.Bytes()
	gy := Generator.Y.Bytes()

	t.Logf("Generator X: %x", gx)
	t.Logf("Expected  X: %s", expectedGx)
	t.Logf("Generator Y: %x", gy)
	t.Logf("Expected  Y: %s", expectedGy)

	// They should match
	if expectedGx != fmt.Sprintf("%x", gx) {
		t.Error("Generator X mismatch")
	}
	if expectedGy != fmt.Sprintf("%x", gy) {
		t.Error("Generator Y mismatch")
	}

	// Verify G is on the curve
	if !Generator.IsOnCurve() {
		t.Error("Generator should be on curve")
	}

	// Let me test squaring and multiplication more carefully
	// Y² should equal X³ + 7
	var y2, x2, x3, seven, rhs FieldElement
	y2.Sqr(&Generator.Y)
	x2.Sqr(&Generator.X)
	x3.Mul(&x2, &Generator.X)
	seven.N[0].Lo = 7
	rhs.Add(&x3, &seven)

	t.Logf("Y² = %x", y2.Bytes())
	t.Logf("X³ + 7 = %x", rhs.Bytes())

	if !y2.Equal(&rhs) {
		t.Error("Y² != X³ + 7 for generator")
	}
}

func TestTraceDouble(t *testing.T) {
	// Test the point doubling step by step
	var g JacobianPoint
	g.FromAffine(&Generator)

	t.Logf("Input G:")
	t.Logf("  X = %x", g.X.Bytes())
	t.Logf("  Y = %x", g.Y.Bytes())
	t.Logf("  Z = %x", g.Z.Bytes())

	// Standard Jacobian doubling for y²=x³+b (secp256k1 has a=0):
	// M = 3*X₁²
	// S = 4*X₁*Y₁²
	// T = 8*Y₁⁴
	// X₃ = M² - 2*S
	// Y₃ = M*(S - X₃) - T
	// Z₃ = 2*Y₁*Z₁

	var y2, m, x2, s, t_val, x3, y3, z3, tmp FieldElement

	// Y² = Y₁²
	y2.Sqr(&g.Y)
	t.Logf("Y² = %x", y2.Bytes())

	// M = 3*X²
	x2.Sqr(&g.X)
	t.Logf("X² = %x", x2.Bytes())
	m.MulInt(&x2, 3)
	t.Logf("M = 3*X² = %x", m.Bytes())

	// S = 4*X₁*Y₁²
	s.Mul(&g.X, &y2)
	t.Logf("X*Y² = %x", s.Bytes())
	s.MulInt(&s, 4)
	t.Logf("S = 4*X*Y² = %x", s.Bytes())

	// T = 8*Y₁⁴
	t_val.Sqr(&y2)
	t.Logf("Y⁴ = %x", t_val.Bytes())
	t_val.MulInt(&t_val, 8)
	t.Logf("T = 8*Y⁴ = %x", t_val.Bytes())

	// X₃ = M² - 2*S
	x3.Sqr(&m)
	t.Logf("M² = %x", x3.Bytes())
	tmp.Double(&s)
	t.Logf("2*S = %x", tmp.Bytes())
	x3.Sub(&x3, &tmp)
	t.Logf("X₃ = M² - 2*S = %x", x3.Bytes())

	// Y₃ = M*(S - X₃) - T
	tmp.Sub(&s, &x3)
	t.Logf("S - X₃ = %x", tmp.Bytes())
	y3.Mul(&m, &tmp)
	t.Logf("M*(S-X₃) = %x", y3.Bytes())
	y3.Sub(&y3, &t_val)
	t.Logf("Y₃ = M*(S-X₃) - T = %x", y3.Bytes())

	// Z₃ = 2*Y₁*Z₁
	z3.Mul(&g.Y, &g.Z)
	z3.Double(&z3)
	t.Logf("Z₃ = 2*Y*Z = %x", z3.Bytes())

	// Now convert to affine
	var doubled JacobianPoint
	doubled.X = x3
	doubled.Y = y3
	doubled.Z = z3
	doubled.Infinity = false

	var affineResult AffinePoint
	doubled.ToAffine(&affineResult)
	t.Logf("Affine result (correct formula):")
	t.Logf("  X = %x", affineResult.X.Bytes())
	t.Logf("  Y = %x", affineResult.Y.Bytes())

	// Expected 2G
	expectedX := "c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
	expectedY := "1ae168fea63dc339a3c58419466ceae1061b7c24a6b3e36e3b4d04f7a8f63301"
	t.Logf("Expected:")
	t.Logf("  X = %s", expectedX)
	t.Logf("  Y = %s", expectedY)

	// Verify by computing 2G using the existing Double method
	var doubled2 JacobianPoint
	doubled2.Double(&g)
	var affine2 AffinePoint
	doubled2.ToAffine(&affine2)
	t.Logf("Current Double method result:")
	t.Logf("  X = %x", affine2.X.Bytes())
	t.Logf("  Y = %x", affine2.Y.Bytes())

	// Compare results
	expectedXBytes, _ := hex.DecodeString(expectedX)
	expectedYBytes, _ := hex.DecodeString(expectedY)

	if fmt.Sprintf("%x", affineResult.X.Bytes()) == expectedX &&
		fmt.Sprintf("%x", affineResult.Y.Bytes()) == expectedY {
		t.Logf("Correct formula produces expected result!")
	} else {
		t.Logf("Even correct formula doesn't match - problem elsewhere")
	}

	_ = expectedXBytes
	_ = expectedYBytes

	// Verify the result is on the curve
	t.Logf("Result is on curve: %v", affineResult.IsOnCurve())

	// Compute y² for the computed result
	var verifyY2, verifyX2, verifyX3, verifySeven, verifyRhs FieldElement
	verifyY2.Sqr(&affineResult.Y)
	verifyX2.Sqr(&affineResult.X)
	verifyX3.Mul(&verifyX2, &affineResult.X)
	verifySeven.N[0].Lo = 7
	verifyRhs.Add(&verifyX3, &verifySeven)
	t.Logf("Computed y² = %x", verifyY2.Bytes())
	t.Logf("Computed x³+7 = %x", verifyRhs.Bytes())
	t.Logf("y² == x³+7: %v", verifyY2.Equal(&verifyRhs))

	// Now test with the expected Y value
	var expectedYField, expectedY2Field FieldElement
	expectedYField.SetBytes(expectedYBytes)
	expectedY2Field.Sqr(&expectedYField)
	t.Logf("Expected Y² = %x", expectedY2Field.Bytes())
	t.Logf("Expected Y² == x³+7: %v", expectedY2Field.Equal(&verifyRhs))

	// Maybe I have the negative Y - let's check the negation
	var negY FieldElement
	negY.Negate(&affineResult.Y)
	t.Logf("Negated computed Y = %x", negY.Bytes())

	// Also check if the expected value is valid at all
	// The expected 2G should be:
	// X = c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5
	// Y = 1ae168fea63dc339a3c58419466ceae1061b7c24a6b3e36e3b4d04f7a8f63301
	// Let me verify this is correct by computing y² directly
	t.Log("--- Verifying expected 2G values ---")
	var expXField FieldElement
	expXField.SetBytes(expectedXBytes)

	// Compute x³ + 7 for the expected X
	var expX2, expX3, expRhs FieldElement
	expX2.Sqr(&expXField)
	expX3.Mul(&expX2, &expXField)
	var seven2 FieldElement
	seven2.N[0].Lo = 7
	expRhs.Add(&expX3, &seven2)
	t.Logf("For expected X, x³+7 = %x", expRhs.Bytes())

	// Compute sqrt
	var sqrtY FieldElement
	if sqrtY.Sqrt(&expRhs) {
		t.Logf("sqrt(x³+7) = %x", sqrtY.Bytes())
		var negSqrtY FieldElement
		negSqrtY.Negate(&sqrtY)
		t.Logf("-sqrt(x³+7) = %x", negSqrtY.Bytes())
	}
}

func TestDebugPointAdd(t *testing.T) {
	// Compute 3G two ways: (1) G + 2G and (2) 3*G via scalar mult
	var g, twoG, threeGAdd JacobianPoint
	var affine3GAdd, affine3GSM AffinePoint

	g.FromAffine(&Generator)
	twoG.Double(&g)
	threeGAdd.Add(&twoG, &g)
	threeGAdd.ToAffine(&affine3GAdd)

	t.Logf("2G (Jacobian):")
	t.Logf("  X = %x", twoG.X.Bytes())
	t.Logf("  Y = %x", twoG.Y.Bytes())
	t.Logf("  Z = %x", twoG.Z.Bytes())

	t.Logf("3G via Add (affine):")
	t.Logf("  X = %x", affine3GAdd.X.Bytes())
	t.Logf("  Y = %x", affine3GAdd.Y.Bytes())
	t.Logf("  On curve: %v", affine3GAdd.IsOnCurve())

	// Now via scalar mult
	var three Scalar
	three.D[0].Lo = 3
	var threeGSM JacobianPoint
	threeGSM.ScalarMult(&g, &three)
	threeGSM.ToAffine(&affine3GSM)

	t.Logf("3G via ScalarMult (affine):")
	t.Logf("  X = %x", affine3GSM.X.Bytes())
	t.Logf("  Y = %x", affine3GSM.Y.Bytes())
	t.Logf("  On curve: %v", affine3GSM.IsOnCurve())

	// Compute expected 3G using Python
	// This should be:
	// X = f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9
	// Y = 388f7b0f632de8140fe337e62a37f3566500a99934c2231b6cb9fd7584b8e672
	t.Logf("Equal: %v", affine3GAdd.Equal(&affine3GSM))
}

func TestAVX2Operations(t *testing.T) {
	// Test that AVX2 assembly produces same results as Go code
	if !hasAVX2() {
		t.Skip("AVX2 not available")
	}

	// Test field addition
	var a, b, resultGo, resultAVX FieldElement
	a.N[0].Lo = 0x123456789ABCDEF0
	a.N[0].Hi = 0xFEDCBA9876543210
	a.N[1].Lo = 0x1111111111111111
	a.N[1].Hi = 0x2222222222222222

	b.N[0].Lo = 0x0FEDCBA987654321
	b.N[0].Hi = 0x123456789ABCDEF0
	b.N[1].Lo = 0x3333333333333333
	b.N[1].Hi = 0x4444444444444444

	resultGo.Add(&a, &b)
	FieldAddAVX2(&resultAVX, &a, &b)

	if !resultGo.Equal(&resultAVX) {
		t.Errorf("FieldAddAVX2 mismatch:\n  Go:   %x\n  AVX2: %x", resultGo.Bytes(), resultAVX.Bytes())
	}

	// Test field subtraction
	resultGo.Sub(&a, &b)
	FieldSubAVX2(&resultAVX, &a, &b)

	if !resultGo.Equal(&resultAVX) {
		t.Errorf("FieldSubAVX2 mismatch:\n  Go:   %x\n  AVX2: %x", resultGo.Bytes(), resultAVX.Bytes())
	}

	// Test field multiplication
	resultGo.Mul(&a, &b)
	FieldMulAVX2(&resultAVX, &a, &b)

	if !resultGo.Equal(&resultAVX) {
		t.Errorf("FieldMulAVX2 mismatch:\n  Go:   %x\n  AVX2: %x", resultGo.Bytes(), resultAVX.Bytes())
	}

	// Test scalar addition
	var sa, sb, sResultGo, sResultAVX Scalar
	sa.D[0].Lo = 0x123456789ABCDEF0
	sa.D[0].Hi = 0xFEDCBA9876543210
	sa.D[1].Lo = 0x1111111111111111
	sa.D[1].Hi = 0x2222222222222222

	sb.D[0].Lo = 0x0FEDCBA987654321
	sb.D[0].Hi = 0x123456789ABCDEF0
	sb.D[1].Lo = 0x3333333333333333
	sb.D[1].Hi = 0x4444444444444444

	sResultGo.Add(&sa, &sb)
	ScalarAddAVX2(&sResultAVX, &sa, &sb)

	if !sResultGo.Equal(&sResultAVX) {
		t.Errorf("ScalarAddAVX2 mismatch:\n  Go:   %x\n  AVX2: %x", sResultGo.Bytes(), sResultAVX.Bytes())
	}

	// Test scalar multiplication
	sResultGo.Mul(&sa, &sb)
	ScalarMulAVX2(&sResultAVX, &sa, &sb)

	if !sResultGo.Equal(&sResultAVX) {
		t.Errorf("ScalarMulAVX2 mismatch:\n  Go:   %x\n  AVX2: %x", sResultGo.Bytes(), sResultAVX.Bytes())
	}

	t.Logf("Field and Scalar Add/Sub AVX2 operations match Go implementations")
}

func TestDebugScalarMult(t *testing.T) {
	// Test 2*G via scalar mult
	var g, twoGDouble, twoGSM JacobianPoint
	var affineDouble, affineSM AffinePoint

	g.FromAffine(&Generator)

	// Via doubling
	twoGDouble.Double(&g)
	twoGDouble.ToAffine(&affineDouble)

	// Via scalar mult (k=2)
	var two Scalar
	two.D[0].Lo = 2

	// Print the bytes of k=2
	twoBytes := two.Bytes()
	t.Logf("k=2 bytes: %x", twoBytes[:])

	twoGSM.ScalarMult(&g, &two)
	twoGSM.ToAffine(&affineSM)

	t.Logf("2G via Double (affine):")
	t.Logf("  X = %x", affineDouble.X.Bytes())
	t.Logf("  Y = %x", affineDouble.Y.Bytes())
	t.Logf("  On curve: %v", affineDouble.IsOnCurve())

	t.Logf("2G via ScalarMult (affine):")
	t.Logf("  X = %x", affineSM.X.Bytes())
	t.Logf("  Y = %x", affineSM.Y.Bytes())
	t.Logf("  On curve: %v", affineSM.IsOnCurve())

	t.Logf("Equal: %v", affineDouble.Equal(&affineSM))

	// Manual scalar mult for k=2
	// Binary: 10 (2 bits)
	// Start with p = infinity
	// bit 1: p = 2*infinity = infinity, then p = p + G = G
	// bit 0: p = 2*G, no add
	// Result should be 2G

	var p JacobianPoint
	p.SetInfinity()

	// Process bit 1 (the high bit of 2)
	p.Double(&p)
	t.Logf("After double of infinity: IsInfinity=%v", p.IsInfinity())
	p.Add(&p, &g)
	t.Logf("After add G: IsInfinity=%v", p.IsInfinity())

	var affineP AffinePoint
	p.ToAffine(&affineP)
	t.Logf("After first iteration (should be G):")
	t.Logf("  X = %x", affineP.X.Bytes())
	t.Logf("  Y = %x", affineP.Y.Bytes())
	t.Logf("  Equal to G: %v", affineP.Equal(&Generator))

	// Process bit 0
	p.Double(&p)
	p.ToAffine(&affineP)
	t.Logf("After second iteration (should be 2G):")
	t.Logf("  X = %x", affineP.X.Bytes())
	t.Logf("  Y = %x", affineP.Y.Bytes())
	t.Logf("  On curve: %v", affineP.IsOnCurve())
	t.Logf("  Equal to Double result: %v", affineP.Equal(&affineDouble))

	// Test: does doubling G into a fresh variable work?
	var fresh JacobianPoint
	var freshAffine AffinePoint
	fresh.Double(&g)
	fresh.ToAffine(&freshAffine)
	t.Logf("Fresh Double(g):")
	t.Logf("  X = %x", freshAffine.X.Bytes())
	t.Logf("  Y = %x", freshAffine.Y.Bytes())
	t.Logf("  On curve: %v", freshAffine.IsOnCurve())

	// Test: what about p.Double(p) when p == g?
	var pCopy JacobianPoint
	pCopy = p // now p is already set to some value
	pCopy.FromAffine(&Generator)
	t.Logf("Before in-place double, pCopy X: %x", pCopy.X.Bytes())
	pCopy.Double(&pCopy)
	var pCopyAffine AffinePoint
	pCopy.ToAffine(&pCopyAffine)
	t.Logf("After in-place Double(&pCopy):")
	t.Logf("  X = %x", pCopyAffine.X.Bytes())
	t.Logf("  Y = %x", pCopyAffine.Y.Bytes())
	t.Logf("  On curve: %v", pCopyAffine.IsOnCurve())
}
