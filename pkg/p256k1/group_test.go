package p256k1

import (
	"fmt"
	"testing"
)

func TestGroupElementAffine(t *testing.T) {
	// Test infinity point
	var inf GroupElementAffine
	inf.setInfinity()
	if !inf.isInfinity() {
		t.Error("setInfinity should create infinity point")
	}
	if !inf.isValid() {
		t.Error("infinity point should be valid")
	}

	// Test generator point
	if Generator.isInfinity() {
		t.Error("generator should not be infinity")
	}
	if !Generator.isValid() {
		t.Error("generator should be valid")
	}

	// Test point negation
	var neg GroupElementAffine
	neg.negate(&Generator)
	if neg.isInfinity() {
		t.Error("negated generator should not be infinity")
	}
	if !neg.isValid() {
		t.Error("negated generator should be valid")
	}

	// Test that G + (-G) = O (using Jacobian arithmetic)
	var gJac, negJac, result GroupElementJacobian
	gJac.setGE(&Generator)
	negJac.setGE(&neg)
	result.addVar(&gJac, &negJac)
	if !result.isInfinity() {
		t.Error("G + (-G) should equal infinity")
	}
}

func TestGroupElementJacobian(t *testing.T) {
	// Test conversion between affine and Jacobian
	var jac GroupElementJacobian
	var aff GroupElementAffine

	// Convert generator to Jacobian and back
	jac.setGE(&Generator)
	aff.setGEJ(&jac)
	
	if !aff.equal(&Generator) {
		t.Error("conversion G -> Jacobian -> affine should preserve point")
	}

	// Test point doubling
	var doubled GroupElementJacobian
	doubled.double(&jac)
	if doubled.isInfinity() {
		t.Error("2*G should not be infinity")
	}

	// Convert back to affine to validate
	var doubledAff GroupElementAffine
	doubledAff.setGEJ(&doubled)
	if !doubledAff.isValid() {
		t.Error("2*G should be valid point")
	}
}

func TestGroupElementStorage(t *testing.T) {
	// Test storage conversion
	var storage GroupElementStorage
	var restored GroupElementAffine

	// Store and restore generator
	Generator.toStorage(&storage)
	restored.fromStorage(&storage)

	if !restored.equal(&Generator) {
		t.Error("storage conversion should preserve point")
	}

	// Test infinity storage
	var inf GroupElementAffine
	inf.setInfinity()
	inf.toStorage(&storage)
	restored.fromStorage(&storage)

	if !restored.isInfinity() {
		t.Error("infinity should be preserved in storage")
	}
}

func TestGroupElementBytes(t *testing.T) {
	var buf [64]byte
	var restored GroupElementAffine

	// Test generator conversion
	Generator.toBytes(buf[:])
	restored.fromBytes(buf[:])

	if !restored.equal(&Generator) {
		t.Error("byte conversion should preserve point")
	}

	// Test infinity conversion
	var inf GroupElementAffine
	inf.setInfinity()
	inf.toBytes(buf[:])
	restored.fromBytes(buf[:])

	if !restored.isInfinity() {
		t.Error("infinity should be preserved in byte conversion")
	}
}

func BenchmarkGroupDouble(b *testing.B) {
	var jac GroupElementJacobian
	jac.setGE(&Generator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jac.double(&jac)
	}
}

func BenchmarkGroupAdd(b *testing.B) {
	var jac1, jac2 GroupElementJacobian
	jac1.setGE(&Generator)
	jac2.setGE(&Generator)
	jac2.double(&jac2) // Make it 2*G

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jac1.addVar(&jac1, &jac2)
	}
}

// TestBatchNormalize tests that BatchNormalize produces the same results as individual conversions
func TestBatchNormalize(t *testing.T) {
	// Create several Jacobian points: G, 2G, 3G, 4G, ...
	n := 10
	points := make([]GroupElementJacobian, n)
	expected := make([]GroupElementAffine, n)

	var current GroupElementJacobian
	current.setGE(&Generator)

	for i := 0; i < n; i++ {
		points[i] = current
		// Get expected result using individual conversion
		expected[i].setGEJ(&current)
		// Move to next point
		var next GroupElementJacobian
		next.addVar(&current, &points[0]) // Add G each time
		current = next
	}

	// Now use BatchNormalize
	result := BatchNormalize(nil, points)

	// Compare results
	for i := 0; i < n; i++ {
		// Normalize both for comparison
		expected[i].x.normalize()
		expected[i].y.normalize()
		result[i].x.normalize()
		result[i].y.normalize()

		if !expected[i].x.equal(&result[i].x) {
			t.Errorf("Point %d: X mismatch", i)
		}
		if !expected[i].y.equal(&result[i].y) {
			t.Errorf("Point %d: Y mismatch", i)
		}
		if expected[i].infinity != result[i].infinity {
			t.Errorf("Point %d: infinity mismatch", i)
		}
	}
}

// TestBatchNormalizeWithInfinity tests that BatchNormalize handles infinity points correctly
func TestBatchNormalizeWithInfinity(t *testing.T) {
	points := make([]GroupElementJacobian, 5)

	// Set some points to generator, some to infinity
	points[0].setGE(&Generator)
	points[1].setInfinity()
	points[2].setGE(&Generator)
	points[2].double(&points[2]) // 2G
	points[3].setInfinity()
	points[4].setGE(&Generator)

	result := BatchNormalize(nil, points)

	// Check infinity points
	if !result[1].isInfinity() {
		t.Error("Point 1 should be infinity")
	}
	if !result[3].isInfinity() {
		t.Error("Point 3 should be infinity")
	}

	// Check non-infinity points
	if result[0].isInfinity() {
		t.Error("Point 0 should not be infinity")
	}
	if result[2].isInfinity() {
		t.Error("Point 2 should not be infinity")
	}
	if result[4].isInfinity() {
		t.Error("Point 4 should not be infinity")
	}

	// Verify non-infinity points are on the curve
	if !result[0].isValid() {
		t.Error("Point 0 should be valid")
	}
	if !result[2].isValid() {
		t.Error("Point 2 should be valid")
	}
	if !result[4].isValid() {
		t.Error("Point 4 should be valid")
	}
}

// TestBatchNormalizeInPlace tests in-place batch normalization
func TestBatchNormalizeInPlace(t *testing.T) {
	n := 5
	points := make([]GroupElementJacobian, n)
	expected := make([]GroupElementAffine, n)

	var current GroupElementJacobian
	current.setGE(&Generator)

	for i := 0; i < n; i++ {
		points[i] = current
		expected[i].setGEJ(&current)
		var next GroupElementJacobian
		next.addVar(&current, &points[0])
		current = next
	}

	// Normalize in place
	BatchNormalizeInPlace(points)

	// After normalization, Z should be 1 for all non-infinity points
	for i := 0; i < n; i++ {
		if !points[i].isInfinity() {
			var one FieldElement
			one.setInt(1)
			points[i].z.normalize()
			if !points[i].z.equal(&one) {
				t.Errorf("Point %d: Z should be 1 after normalization", i)
			}
		}

		// Check X and Y match expected
		points[i].x.normalize()
		points[i].y.normalize()
		expected[i].x.normalize()
		expected[i].y.normalize()

		if !points[i].x.equal(&expected[i].x) {
			t.Errorf("Point %d: X mismatch after in-place normalization", i)
		}
		if !points[i].y.equal(&expected[i].y) {
			t.Errorf("Point %d: Y mismatch after in-place normalization", i)
		}
	}
}

// BenchmarkBatchNormalize benchmarks batch normalization vs individual conversions
func BenchmarkBatchNormalize(b *testing.B) {
	sizes := []int{1, 2, 4, 8, 16, 32, 64}

	for _, size := range sizes {
		n := size // capture for closure

		// Create n Jacobian points
		points := make([]GroupElementJacobian, n)
		var current GroupElementJacobian
		current.setGE(&Generator)
		for i := 0; i < n; i++ {
			points[i] = current
			current.double(&current)
		}

		b.Run(
			fmt.Sprintf("Individual_%d", n),
			func(b *testing.B) {
				out := make([]GroupElementAffine, n)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for j := 0; j < n; j++ {
						out[j].setGEJ(&points[j])
					}
				}
			},
		)

		b.Run(
			fmt.Sprintf("Batch_%d", n),
			func(b *testing.B) {
				out := make([]GroupElementAffine, n)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					BatchNormalize(out, points)
				}
			},
		)
	}
}
