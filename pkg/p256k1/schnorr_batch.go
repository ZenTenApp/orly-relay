//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"crypto/sha256"
	"encoding/binary"
)

// BatchSchnorrItem represents a single signature to be verified in a batch
type BatchSchnorrItem struct {
	Pubkey    *XOnlyPubkey // 32-byte x-only public key
	Message   []byte       // 32-byte message
	Signature []byte       // 64-byte signature (r || s)
}

// SchnorrBatchVerify verifies multiple Schnorr signatures in a single batch.
// This is significantly faster than verifying signatures individually when n > 1.
//
// The algorithm uses random linear combination to verify all signatures at once:
//   - Generate random coefficients z_i from hash of all inputs
//   - Compute: R = (Σ z_i * s_i) * G - Σ (z_i * e_i) * P_i
//   - If R is the point at infinity, all signatures are valid
//
// If the batch fails, the caller should fall back to individual verification
// to identify which signature(s) are invalid.
//
// Returns true if all signatures in the batch are valid.
func SchnorrBatchVerify(items []BatchSchnorrItem) bool {
	n := len(items)
	if n == 0 {
		return true // Empty batch is trivially valid
	}

	// For single signature, fall back to individual verification
	if n == 1 {
		return SchnorrVerify(items[0].Signature, items[0].Message, items[0].Pubkey)
	}

	// Validate all inputs first
	for i := range items {
		if items[i].Pubkey == nil {
			return false
		}
		if len(items[i].Message) != 32 {
			return false
		}
		if len(items[i].Signature) != 64 {
			return false
		}
	}

	// Generate the batch seed for random coefficient generation
	// seed = SHA256(all signatures || all messages || all pubkeys)
	batchSeed := computeBatchSeed(items)

	// Parse all signatures and compute challenges
	type parsedItem struct {
		rx FieldElement     // r value from signature
		s  Scalar           // s value from signature
		e  Scalar           // challenge e
		pk GroupElementAffine // public key point
		z  Scalar           // random coefficient
	}

	parsed := make([]parsedItem, n)

	for i := range items {
		// Parse r as field element
		var r32 [32]byte
		copy(r32[:], items[i].Signature[:32])
		if err := parsed[i].rx.setB32(r32[:]); err != nil {
			return false
		}

		// Parse s as scalar
		var s32 [32]byte
		copy(s32[:], items[i].Signature[32:])
		parsed[i].s.setB32(s32[:])
		if parsed[i].s.isZero() {
			return false
		}

		// Check that s < order (additional validation)
		if parsed[i].s.checkOverflow() {
			return false
		}

		// Parse public key - x-only pubkey with even Y
		if err := parsed[i].pk.x.setB32(items[i].Pubkey.data[:]); err != nil {
			return false
		}
		if !parsed[i].pk.setXOVar(&parsed[i].pk.x, false) {
			return false
		}

		// Compute challenge e = TaggedHash("BIP0340/challenge", r || pk || msg)
		var challengeInput [96]byte
		copy(challengeInput[0:32], items[i].Signature[:32])
		copy(challengeInput[32:64], items[i].Pubkey.data[:])
		copy(challengeInput[64:96], items[i].Message)
		challengeHash := TaggedHash(bip340ChallengeTag, challengeInput[:])
		parsed[i].e.setB32(challengeHash[:])

		// Generate random coefficient z_i from batch seed and index
		parsed[i].z = generateBatchCoefficient(batchSeed, uint32(i))
	}

	// Batch verification equation:
	// (Σ z_i * s_i) * G - Σ (z_i * e_i) * P_i - Σ z_i * R_i = O
	//
	// For efficiency, we use Strauss-style multi-scalar multiplication:
	// 1. Prepare all points and scalars
	// 2. Use a combined loop that shares doublings

	// Prepare points and scalars for multi-scalar multiplication
	// We have 2n+1 terms: 1 generator term + n pubkey terms + n R terms
	points := make([]GroupElementAffine, 2*n)
	scalars := make([]Scalar, 2*n)

	// Accumulate scalar for generator: sumS = Σ z_i * s_i
	var sumS Scalar
	sumS.setInt(0)

	for i := range parsed {
		// Compute z_i * s_i for generator
		var zs Scalar
		zs.mul(&parsed[i].z, &parsed[i].s)
		sumS.add(&sumS, &zs)

		// Store -z_i * e_i * P_i (negated because we subtract)
		var ze Scalar
		ze.mul(&parsed[i].z, &parsed[i].e)
		ze.negate(&ze)
		scalars[i] = ze
		points[i] = parsed[i].pk

		// Lift r to point R_i with even Y
		var Ri GroupElementAffine
		if !Ri.setXOVar(&parsed[i].rx, false) {
			return false
		}

		// Store -z_i * R_i (negated because we subtract)
		var negZ Scalar
		negZ.negate(&parsed[i].z)
		scalars[n+i] = negZ
		points[n+i] = Ri
	}

	// Use multi-scalar multiplication with Strauss algorithm
	// result = sumS*G + Σ scalars[i] * points[i]
	result := ecmultMultiStrauss(&sumS, points, scalars)

	// Batch is valid if result is the point at infinity
	return result.isInfinity()
}

// ecmultMultiStrauss computes ng*G + Σ scalars[i]*points[i] using Strauss algorithm
// This shares doublings across all scalar multiplications for improved performance.
func ecmultMultiStrauss(ng *Scalar, points []GroupElementAffine, scalars []Scalar) GroupElementJacobian {
	n := len(points)
	if n == 0 {
		var result GroupElementJacobian
		EcmultGen(&result, ng)
		return result
	}

	// Use wNAF representation for all scalars
	const w = 5
	const tableSize = 1 << (w - 1) // 16

	// Build tables for all points
	type tableEntry struct {
		pre [tableSize]GroupElementAffine
	}
	tables := make([]tableEntry, n)

	for i := range points {
		if points[i].isInfinity() {
			continue
		}

		var pJac GroupElementJacobian
		pJac.setGE(&points[i])

		// Build odd multiples table
		var preJac [tableSize]GroupElementJacobian
		preJac[0] = pJac

		if tableSize > 1 {
			var twoP GroupElementJacobian
			twoP.double(&pJac)

			for j := 1; j < tableSize; j++ {
				preJac[j].addVar(&preJac[j-1], &twoP)
			}
		}

		// Convert to affine
		batchNormalize16(&tables[i].pre, &preJac)
	}

	// Convert all scalars to wNAF
	wnafs := make([][257]int8, n)
	wnafBits := make([]int, n)

	maxBits := 0
	for i := range scalars {
		wnafBits[i] = scalars[i].wNAF(&wnafs[i], w)
		if wnafBits[i] > maxBits {
			maxBits = wnafBits[i]
		}
	}

	// Also convert generator scalar to wNAF
	var ngWNAF [257]int8
	ngBits := ng.wNAF(&ngWNAF, w)
	if ngBits > maxBits {
		maxBits = ngBits
	}

	// Perform Strauss algorithm
	var result GroupElementJacobian
	result.setInfinity()

	for bit := maxBits - 1; bit >= 0; bit-- {
		// Double
		if !result.isInfinity() {
			result.double(&result)
		}

		// Add generator contribution
		if bit < ngBits && ngWNAF[bit] != 0 {
			idx := ngWNAF[bit]
			if idx > 0 {
				// Use precomputed generator table
				var pt GroupElementAffine
				pt = preGenG[(idx-1)/2]
				if result.isInfinity() {
					result.setGE(&pt)
				} else {
					result.addGE(&result, &pt)
				}
			} else {
				var pt GroupElementAffine
				pt = preGenG[(-idx-1)/2]
				pt.negate(&pt)
				if result.isInfinity() {
					result.setGE(&pt)
				} else {
					result.addGE(&result, &pt)
				}
			}
		}

		// Add contributions from each point
		for i := range scalars {
			if bit >= wnafBits[i] || wnafs[i][bit] == 0 {
				continue
			}

			idx := wnafs[i][bit]
			var pt GroupElementAffine

			if idx > 0 {
				pt = tables[i].pre[(idx-1)/2]
			} else {
				pt = tables[i].pre[(-idx-1)/2]
				pt.negate(&pt)
			}

			if result.isInfinity() {
				result.setGE(&pt)
			} else {
				result.addGE(&result, &pt)
			}
		}
	}

	return result
}

// computeBatchSeed computes a deterministic seed for generating random coefficients
// seed = SHA256("SchnorrBatchVerify" || n || sig_0 || ... || sig_{n-1} || msg_0 || ... || pk_0 || ...)
func computeBatchSeed(items []BatchSchnorrItem) [32]byte {
	h := sha256.New()

	// Domain separator
	h.Write([]byte("SchnorrBatchVerify"))

	// Number of items
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(items)))
	h.Write(buf[:])

	// All signatures
	for i := range items {
		h.Write(items[i].Signature)
	}

	// All messages
	for i := range items {
		h.Write(items[i].Message)
	}

	// All pubkeys
	for i := range items {
		h.Write(items[i].Pubkey.data[:])
	}

	var result [32]byte
	h.Sum(result[:0])
	return result
}

// generateBatchCoefficient generates a random coefficient for batch verification
// z_i = SHA256(seed || i) mod n
func generateBatchCoefficient(seed [32]byte, index uint32) Scalar {
	h := sha256.New()
	h.Write(seed[:])

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], index)
	h.Write(buf[:])

	var hash [32]byte
	h.Sum(hash[:0])

	var z Scalar
	z.setB32(hash[:])

	// Ensure z is non-zero
	if z.isZero() {
		z.setInt(1)
	}

	return z
}

// SchnorrBatchVerifyWithFallback verifies a batch and returns the indices of invalid signatures.
// This is useful when you need to identify which specific signatures failed.
// Returns (true, nil) if all signatures are valid.
// Returns (false, invalidIndices) if some signatures are invalid.
func SchnorrBatchVerifyWithFallback(items []BatchSchnorrItem) (bool, []int) {
	// First try batch verification
	if SchnorrBatchVerify(items) {
		return true, nil
	}

	// Batch failed - identify invalid signatures
	var invalidIndices []int
	for i := range items {
		if !SchnorrVerify(items[i].Signature, items[i].Message, items[i].Pubkey) {
			invalidIndices = append(invalidIndices, i)
		}
	}

	return false, invalidIndices
}
