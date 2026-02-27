package p256k1

// =============================================================================
// Phase 5: Generator Precomputation for GLV Optimization
// =============================================================================
//
// This file contains precomputed tables for the secp256k1 generator point G
// and its λ-transformed version λ*G. These tables enable very fast scalar
// multiplication of the generator point.
//
// The GLV approach splits a 256-bit scalar k into two ~128-bit scalars k1, k2
// such that k = k1 + k2*λ (mod n). Then k*G = k1*G + k2*(λ*G).
//
// We precompute odd multiples of G and λ*G:
//   preGenG[i] = (2*i+1) * G     for i = 0 to tableSize-1
//   preGenLambdaG[i] = (2*i+1) * (λ*G) for i = 0 to tableSize-1
//
// Reference: libsecp256k1 ecmult_gen_impl.h

// Window size for generator multiplication
// Larger window = more precomputation but faster multiplication
// libsecp256k1 uses window 15 for ecmult (8192 entries) and comb with 352 points for ecmult_gen
// Window 8 is maximum due to wNAF int8 digit limit; 128 entries per table
const genWindowSize = 8
const genTableSize = 1 << (genWindowSize - 1) // 128 entries

// Precomputed tables for generator multiplication
// These are computed once at init() time
var (
	// preGenG contains odd multiples of G: preGenG[i] = (2*i+1)*G
	preGenG [genTableSize]GroupElementAffine

	// preGenLambdaG contains odd multiples of λ*G: preGenLambdaG[i] = (2*i+1)*(λ*G)
	preGenLambdaG [genTableSize]GroupElementAffine

	// preGenBetaX contains β*x for each point in preGenG (for potential future optimization)
	preGenBetaX [genTableSize]FieldElement

	// genTablesInitialized tracks whether the tables have been computed
	genTablesInitialized bool
)

// initGenTables computes the precomputed generator tables
// This is called automatically on first use
func initGenTables() {
	if genTablesInitialized {
		return
	}

	// Build odd multiples of G
	var gJac GroupElementJacobian
	gJac.setGE(&Generator)

	var preJacG [genTableSize]GroupElementJacobian
	preJacG[0] = gJac

	// Compute 2*G
	var twoG GroupElementJacobian
	twoG.double(&gJac)

	// Build odd multiples: preJacG[i] = (2*i+1)*G
	for i := 1; i < genTableSize; i++ {
		preJacG[i].addVar(&preJacG[i-1], &twoG)
	}

	// Batch normalize to affine
	BatchNormalize(preGenG[:], preJacG[:])

	// Compute λ*G
	var lambdaG GroupElementAffine
	lambdaG.mulLambda(&Generator)

	// Build odd multiples of λ*G
	var lambdaGJac GroupElementJacobian
	lambdaGJac.setGE(&lambdaG)

	var preJacLambdaG [genTableSize]GroupElementJacobian
	preJacLambdaG[0] = lambdaGJac

	// Compute 2*(λ*G)
	var twoLambdaG GroupElementJacobian
	twoLambdaG.double(&lambdaGJac)

	// Build odd multiples: preJacLambdaG[i] = (2*i+1)*(λ*G)
	for i := 1; i < genTableSize; i++ {
		preJacLambdaG[i].addVar(&preJacLambdaG[i-1], &twoLambdaG)
	}

	// Batch normalize to affine
	BatchNormalize(preGenLambdaG[:], preJacLambdaG[:])

	// Precompute β*x for each point in preGenG
	for i := 0; i < genTableSize; i++ {
		if preGenG[i].isInfinity() {
			preGenBetaX[i] = FieldElementZero
		} else {
			preGenBetaX[i].mul(&preGenG[i].x, &fieldBeta)
		}
	}

	genTablesInitialized = true
}

// EnsureGenTablesInitialized ensures the generator tables are computed
// This is automatically called by ecmultGenGLV, but can be called explicitly
// during application startup to avoid first-use latency
func EnsureGenTablesInitialized() {
	initGenTables()
}

// ecmultGenGLV computes r = k * G using precomputed tables and GLV endomorphism
// This is the fastest method for generator multiplication
func ecmultGenGLV(r *GroupElementJacobian, k *Scalar) {
	if k.isZero() {
		r.setInfinity()
		return
	}

	// Ensure tables are initialized
	initGenTables()

	// Split scalar using GLV: k = k1 + k2*λ
	var k1, k2 Scalar
	scalarSplitLambda(&k1, &k2, k)

	// Normalize k1 and k2 to be "low" (not high)
	// If k1 is high, negate it and we'll negate the final contribution
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

	// Perform Strauss algorithm using precomputed tables
	r.setInfinity()

	for i := maxBits - 1; i >= 0; i-- {
		// Double the result
		if !r.isInfinity() {
			r.double(r)
		}

		// Add contribution from k1 (using preGenG table)
		if i < bits1 && wnaf1[i] != 0 {
			var pt GroupElementAffine
			n := int(wnaf1[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < genTableSize {
				pt = preGenG[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if k1 was negated during normalization
				if neg1 {
					pt.negate(&pt)
				}

				if r.isInfinity() {
					r.setGE(&pt)
				} else {
					r.addGE(r, &pt)
				}
			}
		}

		// Add contribution from k2 (using preGenLambdaG table)
		if i < bits2 && wnaf2[i] != 0 {
			var pt GroupElementAffine
			n := int(wnaf2[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < genTableSize {
				pt = preGenLambdaG[idx]
				// Negate if wNAF digit is negative
				if n < 0 {
					pt.negate(&pt)
				}
				// Negate if k2 was negated during normalization
				if neg2 {
					pt.negate(&pt)
				}

				if r.isInfinity() {
					r.setGE(&pt)
				} else {
					r.addGE(r, &pt)
				}
			}
		}
	}
}


// ecmultGenSimple computes r = k * G using a simple approach without GLV
// This uses the precomputed table for G only, without scalar splitting
// Useful for comparison and as a fallback
func ecmultGenSimple(r *GroupElementJacobian, k *Scalar) {
	if k.isZero() {
		r.setInfinity()
		return
	}

	// Ensure tables are initialized
	initGenTables()

	// Normalize scalar if it's high (has high bit set)
	var kNorm Scalar
	kNorm = *k
	negResult := kNorm.isHigh()
	if negResult {
		kNorm.negate(&kNorm)
	}

	// Convert to wNAF
	var wnaf [257]int8

	bits := kNorm.wNAF(&wnaf, genWindowSize)

	// Perform algorithm using precomputed table
	r.setInfinity()

	for i := bits - 1; i >= 0; i-- {
		// Double the result
		if !r.isInfinity() {
			r.double(r)
		}

		// Add contribution
		if wnaf[i] != 0 {
			var pt GroupElementAffine
			n := int(wnaf[i])

			var idx int
			if n > 0 {
				idx = (n - 1) / 2
			} else {
				idx = (-n - 1) / 2
			}

			if idx < genTableSize {
				pt = preGenG[idx]
				if n < 0 {
					pt.negate(&pt)
				}

				if r.isInfinity() {
					r.setGE(&pt)
				} else {
					r.addGE(r, &pt)
				}
			}
		}
	}

	// Negate result if we negated the scalar
	if negResult {
		r.negate(r)
	}
}

// EcmultGenSimple is the public interface for simple generator multiplication
func EcmultGenSimple(r *GroupElementJacobian, k *Scalar) {
	ecmultGenSimple(r, k)
}

// =============================================================================
// EcmultGenContext - Compatibility layer for existing codebase
// =============================================================================

// EcmultGenContext represents the generator multiplication context
// This wraps the precomputed tables for generator multiplication
type EcmultGenContext struct {
	initialized bool
}

// NewEcmultGenContext creates a new generator multiplication context
// This initializes the precomputed tables if not already done
func NewEcmultGenContext() *EcmultGenContext {
	initGenTables()
	return &EcmultGenContext{
		initialized: true,
	}
}

