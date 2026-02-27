package avx

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDebugDouble(t *testing.T) {
	// Known value: 2G for secp256k1 (verified using Python)
	expectedX := "c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
	expectedY := "1ae168fea63dc339a3c58419466ceaeef7f632653266d0e1236431a950cfe52a"

	var g, doubled JacobianPoint
	var affineResult AffinePoint

	g.FromAffine(&Generator)
	doubled.Double(&g)
	doubled.ToAffine(&affineResult)

	xBytes := affineResult.X.Bytes()
	yBytes := affineResult.Y.Bytes()
	
	t.Logf("Generator X: %x", Generator.X.Bytes())
	t.Logf("Generator Y: %x", Generator.Y.Bytes())
	t.Logf("2G X: %x", xBytes)
	t.Logf("2G Y: %x", yBytes)
	
	expectedXBytes, _ := hex.DecodeString(expectedX)
	expectedYBytes, _ := hex.DecodeString(expectedY)
	
	t.Logf("Expected X: %s", expectedX)
	t.Logf("Expected Y: %s", expectedY)
	
	if !bytes.Equal(xBytes[:], expectedXBytes) {
		t.Errorf("2G X coordinate mismatch")
	}
	if !bytes.Equal(yBytes[:], expectedYBytes) {
		t.Errorf("2G Y coordinate mismatch")
	}
	
	// Check if 2G is on curve
	if !affineResult.IsOnCurve() {
		// Let's verify manually
		var y2, x2, x3, rhs FieldElement
		y2.Sqr(&affineResult.Y)
		x2.Sqr(&affineResult.X)
		x3.Mul(&x2, &affineResult.X)
		var seven FieldElement
		seven.N[0].Lo = 7
		rhs.Add(&x3, &seven)
		
		y2Bytes := y2.Bytes()
		rhsBytes := rhs.Bytes()
		t.Logf("y^2 = %x", y2Bytes)
		t.Logf("x^3 + 7 = %x", rhsBytes)
		t.Logf("y^2 == x^3+7: %v", y2.Equal(&rhs))
	}
}
