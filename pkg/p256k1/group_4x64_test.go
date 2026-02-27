//go:build amd64 && !purego

package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestGroup4x64DoubleVsOriginal(t *testing.T) {
	// Create a random point
	for i := 0; i < 100; i++ {
		var aBytes [32]byte
		rand.Read(aBytes[:])
		aBytes[0] &= 0x7F // Ensure < p

		// Create original point
		var aField FieldElement
		aField.setB32(aBytes[:])

		var origAffine GroupElementAffine
		if !origAffine.setXOVar(&aField, false) {
			continue // Not on curve, skip
		}

		var origJac GroupElementJacobian
		origJac.setGE(&origAffine)

		// Convert to 4x64
		var jac4x64 GroupElement4x64Jacobian
		jac4x64.FromGroupElementJacobian(&origJac)

		// Double with both implementations
		var origResult GroupElementJacobian
		origResult.double(&origJac)

		var result4x64 GroupElement4x64Jacobian
		result4x64.double(&jac4x64)

		// Convert 4x64 result back
		var result4x64Jac GroupElementJacobian
		result4x64.ToGroupElementJacobian(&result4x64Jac)

		// Compare by converting to affine
		var origAff, result4x64Aff GroupElementAffine
		origAff.setGEJ(&origResult)
		result4x64Aff.setGEJ(&result4x64Jac)

		origAff.x.normalize()
		origAff.y.normalize()
		result4x64Aff.x.normalize()
		result4x64Aff.y.normalize()

		if !origAff.x.equal(&result4x64Aff.x) || !origAff.y.equal(&result4x64Aff.y) {
			t.Errorf("Iteration %d: Double mismatch", i)
		}
	}
}

func TestGroup4x64AddVsOriginal(t *testing.T) {
	// Create random points
	for i := 0; i < 100; i++ {
		var aBytes, bBytes [32]byte
		rand.Read(aBytes[:])
		rand.Read(bBytes[:])
		aBytes[0] &= 0x7F
		bBytes[0] &= 0x7F

		// Create original points
		var aField, bField FieldElement
		aField.setB32(aBytes[:])
		bField.setB32(bBytes[:])

		var aAffine, bAffine GroupElementAffine
		if !aAffine.setXOVar(&aField, false) {
			continue
		}
		if !bAffine.setXOVar(&bField, false) {
			continue
		}

		var aJac, bJac GroupElementJacobian
		aJac.setGE(&aAffine)
		bJac.setGE(&bAffine)

		// Convert to 4x64
		var aJac4x64, bJac4x64 GroupElement4x64Jacobian
		aJac4x64.FromGroupElementJacobian(&aJac)
		bJac4x64.FromGroupElementJacobian(&bJac)

		// Add with both implementations
		var origResult GroupElementJacobian
		origResult.addVar(&aJac, &bJac)

		var result4x64 GroupElement4x64Jacobian
		result4x64.addVar(&aJac4x64, &bJac4x64)

		// Convert 4x64 result back
		var result4x64Jac GroupElementJacobian
		result4x64.ToGroupElementJacobian(&result4x64Jac)

		// Compare by converting to affine
		var origAff, result4x64Aff GroupElementAffine
		origAff.setGEJ(&origResult)
		result4x64Aff.setGEJ(&result4x64Jac)

		origAff.x.normalize()
		origAff.y.normalize()
		result4x64Aff.x.normalize()
		result4x64Aff.y.normalize()

		if !origAff.x.equal(&result4x64Aff.x) || !origAff.y.equal(&result4x64Aff.y) {
			t.Errorf("Iteration %d: Add mismatch", i)
		}
	}
}

func TestGroup4x64AddGEVsOriginal(t *testing.T) {
	// Test addGE (Jacobian + Affine)
	for i := 0; i < 100; i++ {
		var aBytes, bBytes [32]byte
		rand.Read(aBytes[:])
		rand.Read(bBytes[:])
		aBytes[0] &= 0x7F
		bBytes[0] &= 0x7F

		// Create original points
		var aField, bField FieldElement
		aField.setB32(aBytes[:])
		bField.setB32(bBytes[:])

		var aAffine, bAffine GroupElementAffine
		if !aAffine.setXOVar(&aField, false) {
			continue
		}
		if !bAffine.setXOVar(&bField, false) {
			continue
		}

		var aJac GroupElementJacobian
		aJac.setGE(&aAffine)

		// Convert to 4x64
		var aJac4x64 GroupElement4x64Jacobian
		var bAffine4x64 GroupElement4x64Affine
		aJac4x64.FromGroupElementJacobian(&aJac)
		bAffine4x64.FromGroupElementAffine(&bAffine)

		// Add with both implementations
		var origResult GroupElementJacobian
		origResult.addGE(&aJac, &bAffine)

		var result4x64 GroupElement4x64Jacobian
		result4x64.addGE(&aJac4x64, &bAffine4x64)

		// Convert 4x64 result back
		var result4x64Jac GroupElementJacobian
		result4x64.ToGroupElementJacobian(&result4x64Jac)

		// Compare by converting to affine
		var origAff, result4x64Aff GroupElementAffine
		origAff.setGEJ(&origResult)
		result4x64Aff.setGEJ(&result4x64Jac)

		origAff.x.normalize()
		origAff.y.normalize()
		result4x64Aff.x.normalize()
		result4x64Aff.y.normalize()

		if !origAff.x.equal(&result4x64Aff.x) || !origAff.y.equal(&result4x64Aff.y) {
			t.Errorf("Iteration %d: AddGE mismatch", i)
		}
	}
}

// Benchmarks comparing 4x64 vs original group operations

func BenchmarkGroup4x64Double(b *testing.B) {
	// Create a test point
	var xBytes [32]byte
	xBytes[31] = 0x02 // Small value that's on the curve
	var xField FieldElement
	xField.setB32(xBytes[:])

	var aff GroupElementAffine
	aff.setXOVar(&xField, false)

	var jac GroupElement4x64Jacobian
	var affine4x64 GroupElement4x64Affine
	affine4x64.FromGroupElementAffine(&aff)
	jac.setGE(&affine4x64)

	var r GroupElement4x64Jacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.double(&jac)
	}
}

func BenchmarkGroupOriginalDouble(b *testing.B) {
	// Create a test point
	var xBytes [32]byte
	xBytes[31] = 0x02
	var xField FieldElement
	xField.setB32(xBytes[:])

	var aff GroupElementAffine
	aff.setXOVar(&xField, false)

	var jac GroupElementJacobian
	jac.setGE(&aff)

	var r GroupElementJacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.double(&jac)
	}
}

func BenchmarkGroup4x64AddGE(b *testing.B) {
	// Create test points
	var xBytes, yBytes [32]byte
	xBytes[31] = 0x02
	yBytes[31] = 0x03

	var xField, yField FieldElement
	xField.setB32(xBytes[:])
	yField.setB32(yBytes[:])

	var aAff, bAff GroupElementAffine
	aAff.setXOVar(&xField, false)
	bAff.setXOVar(&yField, false)

	var aAff4x64, bAff4x64 GroupElement4x64Affine
	aAff4x64.FromGroupElementAffine(&aAff)
	bAff4x64.FromGroupElementAffine(&bAff)

	var aJac4x64 GroupElement4x64Jacobian
	aJac4x64.setGE(&aAff4x64)

	var r GroupElement4x64Jacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.addGE(&aJac4x64, &bAff4x64)
	}
}

func BenchmarkGroupOriginalAddGE(b *testing.B) {
	// Create test points
	var xBytes, yBytes [32]byte
	xBytes[31] = 0x02
	yBytes[31] = 0x03

	var xField, yField FieldElement
	xField.setB32(xBytes[:])
	yField.setB32(yBytes[:])

	var aAff, bAff GroupElementAffine
	aAff.setXOVar(&xField, false)
	bAff.setXOVar(&yField, false)

	var aJac GroupElementJacobian
	aJac.setGE(&aAff)

	var r GroupElementJacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.addGE(&aJac, &bAff)
	}
}

func BenchmarkGroup4x64AddVar(b *testing.B) {
	// Create test points
	var xBytes, yBytes [32]byte
	xBytes[31] = 0x02
	yBytes[31] = 0x03

	var xField, yField FieldElement
	xField.setB32(xBytes[:])
	yField.setB32(yBytes[:])

	var aAff, bAff GroupElementAffine
	aAff.setXOVar(&xField, false)
	bAff.setXOVar(&yField, false)

	var aAff4x64, bAff4x64 GroupElement4x64Affine
	aAff4x64.FromGroupElementAffine(&aAff)
	bAff4x64.FromGroupElementAffine(&bAff)

	var aJac4x64, bJac4x64 GroupElement4x64Jacobian
	aJac4x64.setGE(&aAff4x64)
	bJac4x64.setGE(&bAff4x64)

	var r GroupElement4x64Jacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.addVar(&aJac4x64, &bJac4x64)
	}
}

func BenchmarkGroupOriginalAddVar(b *testing.B) {
	// Create test points
	var xBytes, yBytes [32]byte
	xBytes[31] = 0x02
	yBytes[31] = 0x03

	var xField, yField FieldElement
	xField.setB32(xBytes[:])
	yField.setB32(yBytes[:])

	var aAff, bAff GroupElementAffine
	aAff.setXOVar(&xField, false)
	bAff.setXOVar(&yField, false)

	var aJac, bJac GroupElementJacobian
	aJac.setGE(&aAff)
	bJac.setGE(&bAff)

	var r GroupElementJacobian

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.addVar(&aJac, &bJac)
	}
}
