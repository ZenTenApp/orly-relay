//go:build amd64 && !purego

package p256k1

// AMD64-optimized Strauss+GLV+wNAF multiplication using Group4x64
// This provides significant speedup for verification and ECDH operations

// ecmultStraussWNAFGLV4x64 computes r = q * a using Strauss algorithm with GLV
// and Field4x64 operations for maximum performance on AMD64 with BMI2.
func ecmultStraussWNAFGLV4x64(r *GroupElementJacobian, a *GroupElementAffine, q *Scalar) {
	if a.isInfinity() {
		r.setInfinity()
		return
	}

	if q.isZero() {
		r.setInfinity()
		return
	}

	// Split scalar using GLV endomorphism: q = q1 + q2*λ
	// Also get the transformed points p1 = a, p2 = λ*a
	var q1, q2 Scalar
	var p1, p2 GroupElementAffine
	ecmultEndoSplit(&q1, &q2, &p1, &p2, q, a)

	// Normalize scalars if high
	neg1 := q1.isHigh()
	if neg1 {
		q1.negate(&q1)
	}
	neg2 := q2.isHigh()
	if neg2 {
		q2.negate(&q2)
	}

	// Build odd multiples tables in 4x64 format
	var p1Jac GroupElementJacobian
	p1Jac.setGE(&p1)

	var preA1_4x64 [glvTableSize]GroupElement4x64Affine
	buildOddMultiplesTable4x64(&preA1_4x64, &p1Jac)

	// Build table for p2 (λ*a)
	var p2Jac GroupElementJacobian
	p2Jac.setGE(&p2)

	var preA2_4x64 [glvTableSize]GroupElement4x64Affine
	buildOddMultiplesTable4x64(&preA2_4x64, &p2Jac)

	// Convert scalars to wNAF representation
	var wnaf1, wnaf2 [257]int8

	bits1 := q1.wNAF(&wnaf1, glvWNAFW)
	bits2 := q2.wNAF(&wnaf2, glvWNAFW)

	// Find the maximum bit position
	maxBits := bits1
	if bits2 > maxBits {
		maxBits = bits2
	}

	// Perform the Strauss algorithm using 4x64 operations
	var r4x64 GroupElement4x64Jacobian
	r4x64.setInfinity()

	for i := maxBits - 1; i >= 0; i-- {
		// Double the result
		if !r4x64.isInfinity() {
			r4x64.double(&r4x64)
		}

		// Add contribution from q1
		if i < bits1 && wnaf1[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnaf1[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < glvTableSize {
				pt = preA1_4x64[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if q1 was negated during normalization
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

		// Add contribution from q2
		if i < bits2 && wnaf2[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnaf2[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < glvTableSize {
				pt = preA2_4x64[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if q2 was negated during normalization
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

// buildOddMultiplesTable4x64 builds a precomputation table in 4x64 format
func buildOddMultiplesTable4x64(pre *[glvTableSize]GroupElement4x64Affine, a *GroupElementJacobian) {
	// Build odd multiples in 4x64 Jacobian coordinates
	var a4x64 GroupElement4x64Jacobian
	a4x64.FromGroupElementJacobian(a)

	var preJac [glvTableSize]GroupElement4x64Jacobian

	// pre[0] = a (which is 1*a)
	preJac[0] = a4x64

	if glvTableSize > 1 {
		// Compute 2*a
		var twoA GroupElement4x64Jacobian
		twoA.double(&a4x64)

		// Build odd multiples: pre[i] = pre[i-1] + 2*a for i >= 1
		for i := 1; i < glvTableSize; i++ {
			preJac[i].addVar(&preJac[i-1], &twoA)
		}
	}

	// Batch convert to affine (much more efficient than individual conversions)
	batchNormalize4x64(pre[:], preJac[:])
}

// Ecmult4x64 computes r = q * a using 4x64 optimized operations
// This is the AMD64-specific optimized version
func Ecmult4x64(r *GroupElementJacobian, a *GroupElementJacobian, q *Scalar) {
	if a.isInfinity() {
		r.setInfinity()
		return
	}

	if q.isZero() {
		r.setInfinity()
		return
	}

	// Convert to affine for GLV multiplication
	var aAff GroupElementAffine
	aAff.setGEJ(a)

	// Use optimized 4x64 GLV+Strauss+wNAF multiplication
	ecmultStraussWNAFGLV4x64(r, &aAff, q)
}

// ecmultStraussCombined4x64 computes r = na*a + ng*G using 4x64 operations
// This is the AMD64-optimized combined Strauss algorithm
func ecmultStraussCombined4x64(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar) {
	// Ensure 4x64 generator tables are initialized
	initGen4x64Tables()

	// Split na using GLV: na = na1 + na2*λ (scalar split only)
	var na1, na2 Scalar
	scalarSplitLambda(&na1, &na2, na)

	// Split ng using GLV: ng = ng1 + ng2*λ
	var ng1, ng2 Scalar
	scalarSplitLambda(&ng1, &ng2, ng)

	// Compute p1 = a, p2 = λ*a directly in Jacobian coordinates
	// This avoids the expensive Jacobian→Affine conversion
	var p1Jac, p2Jac GroupElementJacobian
	p1Jac = *a
	p2Jac.mulLambda(a)

	// Normalize all scalars to be "low" (not high)
	// If scalar is high, negate both scalar and point
	if na1.isHigh() {
		na1.negate(&na1)
		p1Jac.negate(&p1Jac)
	}
	if na2.isHigh() {
		na2.negate(&na2)
		p2Jac.negate(&p2Jac)
	}
	negNg1 := ng1.isHigh()
	if negNg1 {
		ng1.negate(&ng1)
	}
	negNg2 := ng2.isHigh()
	if negNg2 {
		ng2.negate(&ng2)
	}

	// Build precomputed tables for a and λ*a in 4x64 format
	// buildOddMultiplesTable4x64 handles Jacobian→Affine conversion internally
	var preA1_4x64 [glvTableSize]GroupElement4x64Affine
	buildOddMultiplesTable4x64(&preA1_4x64, &p1Jac)

	var preA2_4x64 [glvTableSize]GroupElement4x64Affine
	buildOddMultiplesTable4x64(&preA2_4x64, &p2Jac)

	// Convert all four scalars to wNAF
	var wnafNa1, wnafNa2 [257]int8
	var wnafNg1, wnafNg2 [257]int8

	bitsNa1 := na1.wNAF(&wnafNa1, glvWNAFW)
	bitsNa2 := na2.wNAF(&wnafNa2, glvWNAFW)
	bitsNg1 := ng1.wNAF(&wnafNg1, genWindowSize)
	bitsNg2 := ng2.wNAF(&wnafNg2, genWindowSize)

	// Find maximum bit position across all four
	maxBits := bitsNa1
	if bitsNa2 > maxBits {
		maxBits = bitsNa2
	}
	if bitsNg1 > maxBits {
		maxBits = bitsNg1
	}
	if bitsNg2 > maxBits {
		maxBits = bitsNg2
	}

	// Combined Strauss loop using 4x64 operations
	var r4x64 GroupElement4x64Jacobian
	r4x64.setInfinity()

	for i := maxBits - 1; i >= 0; i-- {
		// Double once (shared across all four multiplications)
		if !r4x64.isInfinity() {
			r4x64.double(&r4x64)
		}

		// Add contribution from na1 (using preA1_4x64 table)
		if i < bitsNa1 && wnafNa1[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnafNa1[i])
			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}
			if idx < glvTableSize {
				pt = preA1_4x64[idx]
				if n < 0 {
					pt.negate(&pt)
				}
				// Note: scalar normalization is handled at table-build time
				// by negating p1Jac if na1 was high
				if r4x64.isInfinity() {
					r4x64.setGE(&pt)
				} else {
					r4x64.addGE(&r4x64, &pt)
				}
			}
		}

		// Add contribution from na2 (using preA2_4x64 table)
		if i < bitsNa2 && wnafNa2[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnafNa2[i])
			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}
			if idx < glvTableSize {
				pt = preA2_4x64[idx]
				if n < 0 {
					pt.negate(&pt)
				}
				// Note: scalar normalization is handled at table-build time
				// by negating p2Jac if na2 was high
				if r4x64.isInfinity() {
					r4x64.setGE(&pt)
				} else {
					r4x64.addGE(&r4x64, &pt)
				}
			}
		}

		// Add contribution from ng1 (using preGenG4x64 table)
		if i < bitsNg1 && wnafNg1[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnafNg1[i])
			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}
			if idx < genTableSize {
				pt = preGenG4x64[idx]
				if n < 0 {
					pt.negate(&pt)
				}
				if negNg1 {
					pt.negate(&pt)
				}
				if r4x64.isInfinity() {
					r4x64.setGE(&pt)
				} else {
					r4x64.addGE(&r4x64, &pt)
				}
			}
		}

		// Add contribution from ng2 (using preGenLambdaG4x64 table)
		if i < bitsNg2 && wnafNg2[i] != 0 {
			var pt GroupElement4x64Affine
			n := int(wnafNg2[i])
			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}
			if idx < genTableSize {
				pt = preGenLambdaG4x64[idx]
				if n < 0 {
					pt.negate(&pt)
				}
				if negNg2 {
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

// EcmultCombined4x64 is the AMD64-optimized version of EcmultCombined
func EcmultCombined4x64(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar) {
	// Handle edge cases
	naZero := na == nil || na.isZero()
	ngZero := ng == nil || ng.isZero()
	aInf := a == nil || a.isInfinity()

	if naZero && ngZero {
		r.setInfinity()
		return
	}

	if naZero || aInf {
		EcmultGen(r, ng)
		return
	}

	if ngZero {
		var aAff GroupElementAffine
		aAff.setGEJ(a)
		ecmultStraussWNAFGLV4x64(r, &aAff, na)
		return
	}

	ecmultStraussCombined4x64(r, a, na, ng)
}

