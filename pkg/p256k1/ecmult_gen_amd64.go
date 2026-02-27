//go:build amd64 && !purego

package p256k1

// =============================================================================
// AMD64-optimized Generator Precomputation using Group4x64
// =============================================================================
//
// This file contains optimized generator multiplication using the faster
// Group4x64 field representation with BMI2 MULX instructions.

// Precomputed tables in 4x64 format for faster operations
var (
	// preGenG4x64 contains odd multiples of G in 4x64 format
	preGenG4x64 [genTableSize]GroupElement4x64Affine

	// preGenLambdaG4x64 contains odd multiples of λ*G in 4x64 format
	preGenLambdaG4x64 [genTableSize]GroupElement4x64Affine

	// gen4x64TablesInitialized tracks whether the 4x64 tables have been computed
	gen4x64TablesInitialized bool
)

// initGen4x64Tables converts the precomputed tables to 4x64 format
func initGen4x64Tables() {
	if gen4x64TablesInitialized {
		return
	}

	// Ensure base tables are initialized first
	initGenTables()

	// Convert preGenG to 4x64 format
	for i := 0; i < genTableSize; i++ {
		preGenG4x64[i].FromGroupElementAffine(&preGenG[i])
	}

	// Convert preGenLambdaG to 4x64 format
	for i := 0; i < genTableSize; i++ {
		preGenLambdaG4x64[i].FromGroupElementAffine(&preGenLambdaG[i])
	}

	gen4x64TablesInitialized = true
}

// ecmultGenGLV4x64 computes r = k * G using 4x64 optimized operations
// This is significantly faster than the generic version on AMD64
func ecmultGenGLV4x64(r *GroupElementJacobian, k *Scalar) {
	if k.isZero() {
		r.setInfinity()
		return
	}

	// Ensure tables are initialized
	initGen4x64Tables()

	// Split scalar using GLV: k = k1 + k2*λ
	var k1, k2 Scalar
	scalarSplitLambda(&k1, &k2, k)

	// Normalize k1 and k2 to be "low" (not high)
	neg1 := k1.isHigh()
	if neg1 {
		k1.negate(&k1)
	}

	neg2 := k2.isHigh()
	if neg2 {
		k2.negate(&k2)
	}

	// Convert to wNAF
	var wnaf1, wnaf2 [257]int8

	bits1 := k1.wNAF(&wnaf1, genWindowSize)
	bits2 := k2.wNAF(&wnaf2, genWindowSize)

	// Find maximum bit position
	maxBits := bits1
	if bits2 > maxBits {
		maxBits = bits2
	}

	// Perform Strauss algorithm using 4x64 operations
	var r4x64 GroupElement4x64Jacobian
	r4x64.setInfinity()

	for i := maxBits - 1; i >= 0; i-- {
		// Double the result
		if !r4x64.isInfinity() {
			r4x64.double(&r4x64)
		}

		// Add contribution from k1 (using preGenG4x64 table)
		if i < bits1 && wnaf1[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnaf1[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < genTableSize {
				pt = preGenG4x64[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if k1 was negated during normalization
				if neg1 {
					pt.negate(&pt)
				}

				if r4x64.isInfinity() {
					r4x64.setGE(&pt)
				} else {
					r4x64.addGE(&r4x64, &pt)
				}
			}
		}

		// Add contribution from k2 (using preGenLambdaG4x64 table)
		if i < bits2 && wnaf2[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnaf2[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < genTableSize {
				pt = preGenLambdaG4x64[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if k2 was negated during normalization
				if neg2 {
					pt.negate(&pt)
				}

				if r4x64.isInfinity() {
					r4x64.setGE(&pt)
				} else {
					r4x64.addGE(&r4x64, &pt)
				}
			}
		}
	}

	// Convert result back to standard format
	r4x64.ToGroupElementJacobian(r)
}

// EcmultGen computes r = k * G using the fastest available method
// On AMD64, this uses the optimized 4x64 implementation
func EcmultGen(r *GroupElementJacobian, k *Scalar) {
	ecmultGenGLV4x64(r, k)
}

// EcmultGenGLV is the public interface for fast generator multiplication
// r = k * G
func EcmultGenGLV(r *GroupElementJacobian, k *Scalar) {
	ecmultGenGLV4x64(r, k)
}
