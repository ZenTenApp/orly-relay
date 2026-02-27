//go:build js || wasm || tinygo || wasm32

package p256k1

import (
	"sync"
	"unsafe"
)

// WASM-compatible secp256k1 verification types and functions
// These use the 32-bit field/scalar representations

// secp256k1_context represents a context
type secp256k1_context struct {
	ecmult_gen_ctx secp256k1_ecmult_gen_context
	declassify     int
}

type secp256k1_ecmult_gen_context struct {
	built int
}

// secp256k1_declassify declassifies data (no-op in non-VERIFY builds)
func secp256k1_declassify(ctx *secp256k1_context, p unsafe.Pointer, len uintptr) {
	// No-op
}

// secp256k1_xonly_pubkey represents an x-only public key
type secp256k1_xonly_pubkey struct {
	data [32]byte
}

// secp256k1_scalar wraps 32-bit Scalar for C-like API
type secp256k1_scalar struct {
	s Scalar
}

// secp256k1_fe wraps 32-bit FieldElement for C-like API
type secp256k1_fe struct {
	fe FieldElement
}

// secp256k1_ge represents a group element in affine coordinates
type secp256k1_ge struct {
	x, y     secp256k1_fe
	infinity int
}

// secp256k1_gej represents a group element in Jacobian coordinates
type secp256k1_gej struct {
	x, y, z  secp256k1_fe
	infinity int
}

// ============================================================================
// SCALAR OPERATIONS
// ============================================================================

// secp256k1_scalar_set_b32 sets scalar from 32 bytes
func secp256k1_scalar_set_b32(r *secp256k1_scalar, b32 []byte, overflow *int) {
	of := r.s.setB32(b32)
	if overflow != nil {
		if of {
			*overflow = 1
		} else {
			*overflow = 0
		}
	}
}

// secp256k1_scalar_negate negates a scalar
func secp256k1_scalar_negate(r *secp256k1_scalar, a *secp256k1_scalar) {
	r.s = a.s
	r.s.negate(&r.s)
}

// secp256k1_scalar_is_zero checks if scalar is zero
func secp256k1_scalar_is_zero(a *secp256k1_scalar) bool {
	return a.s.isZero()
}

// ============================================================================
// FIELD OPERATIONS
// ============================================================================

// secp256k1_fe_set_b32_limit sets field element from 32 bytes, checking limit
func secp256k1_fe_set_b32_limit(r *secp256k1_fe, b32 []byte) bool {
	return r.fe.setB32(b32) == nil
}

// secp256k1_fe_normalize_var normalizes field element
func secp256k1_fe_normalize_var(r *secp256k1_fe) {
	r.fe.normalize()
}

// secp256k1_fe_is_odd checks if field element is odd
func secp256k1_fe_is_odd(a *secp256k1_fe) bool {
	return a.fe.n[0]&1 == 1
}

// secp256k1_fe_get_b32 gets 32 bytes from field element
func secp256k1_fe_get_b32(r []byte, a *secp256k1_fe) {
	a.fe.getB32(r)
}

// secp256k1_fe_set_int sets field element to integer
func secp256k1_fe_set_int(r *secp256k1_fe, a int) {
	r.fe.setInt(a)
}

// secp256k1_fe_equal compares two field elements
func secp256k1_fe_equal(a, b *secp256k1_fe) bool {
	// Compare all 10 limbs
	for i := 0; i < 10; i++ {
		if a.fe.n[i] != b.fe.n[i] {
			return false
		}
	}
	return true
}

// ============================================================================
// GROUP OPERATIONS
// ============================================================================

// secp256k1_ge_is_infinity checks if group element is infinity
func secp256k1_ge_is_infinity(a *secp256k1_ge) bool {
	return a.infinity != 0
}

// secp256k1_gej_set_ge sets Jacobian from affine
func secp256k1_gej_set_ge(r *secp256k1_gej, a *secp256k1_ge) {
	r.infinity = a.infinity
	r.x = a.x
	r.y = a.y
	secp256k1_fe_set_int(&r.z, 1)
}

// secp256k1_ge_set_gej_var sets affine from Jacobian
func secp256k1_ge_set_gej_var(r *secp256k1_ge, a *secp256k1_gej) {
	if a.infinity != 0 {
		r.infinity = 1
		return
	}
	r.infinity = 0

	// Convert from Jacobian to affine
	var gej GroupElementJacobian
	gej.x = a.x.fe
	gej.y = a.y.fe
	gej.z = a.z.fe
	gej.infinity = a.infinity != 0

	var ge GroupElementAffine
	ge.setGEJ(&gej)

	r.x.fe = ge.x
	r.y.fe = ge.y
}

// secp256k1_xonly_pubkey_load loads x-only public key
func secp256k1_xonly_pubkey_load(ctx *secp256k1_context, ge *secp256k1_ge, pubkey *secp256k1_xonly_pubkey) bool {
	// Reconstruct point from X coordinate (x-only pubkey only has X)
	var x FieldElement
	if err := x.setB32(pubkey.data[:]); err != nil {
		return false
	}

	// Try to recover Y coordinate (use even Y for BIP-340)
	var gep GroupElementAffine
	if !gep.setXOVar(&x, false) {
		return false
	}

	ge.x.fe = gep.x
	ge.y.fe = gep.y
	if gep.infinity {
		ge.infinity = 1
	} else {
		ge.infinity = 0
	}

	return true
}

// secp256k1_ecmult performs scalar multiplication: r = a*s + G*t
func secp256k1_ecmult(r *secp256k1_gej, a *secp256k1_gej, na *secp256k1_scalar, ng *secp256k1_scalar) {
	// Convert types
	var aJac GroupElementJacobian
	aJac.x = a.x.fe
	aJac.y = a.y.fe
	aJac.z = a.z.fe
	aJac.infinity = a.infinity != 0

	var aAff GroupElementAffine
	aAff.setGEJ(&aJac)

	var result GroupElementJacobian

	// Use GLV/Strauss/wNAF for the scalar multiplication
	// r = na*a + ng*G
	if !na.s.isZero() && !ng.s.isZero() {
		// Both scalars non-zero: compute na*a, then add ng*G
		ecmultStraussWNAFGLV(&result, &aAff, &na.s)

		var gMul GroupElementJacobian
		ecmultGenGLV(&gMul, &ng.s)

		result.addVar(&result, &gMul)
	} else if !na.s.isZero() {
		// Only na*a
		ecmultStraussWNAFGLV(&result, &aAff, &na.s)
	} else if !ng.s.isZero() {
		// Only ng*G
		ecmultGenGLV(&result, &ng.s)
	} else {
		result.infinity = true
	}

	// Convert back
	r.x.fe = result.x
	r.y.fe = result.y
	r.z.fe = result.z
	if result.infinity {
		r.infinity = 1
	} else {
		r.infinity = 0
	}
}

// ============================================================================
// SCHNORR CHALLENGE
// ============================================================================

// secp256k1_schnorrsig_challenge computes BIP-340 challenge
func secp256k1_schnorrsig_challenge(e *secp256k1_scalar, r32 []byte, msg []byte, msglen int, pk32 []byte) {
	// TaggedHash("BIP0340/challenge", r32 || pk32 || msg)
	var input []byte
	input = append(input, r32[:32]...)
	input = append(input, pk32[:32]...)
	input = append(input, msg[:msglen]...)

	hash := TaggedHash([]byte("BIP0340/challenge"), input)

	var overflow int
	secp256k1_scalar_set_b32(e, hash[:], &overflow)
}

// ============================================================================
// SCHNORR VERIFICATION
// ============================================================================

// Global precomputed context for Schnorr verification
var (
	schnorrVerifyContext     *secp256k1_context
	schnorrVerifyContextOnce sync.Once
)

// initSchnorrVerifyContext initializes the global Schnorr verification context
func initSchnorrVerifyContext() {
	schnorrVerifyContext = &secp256k1_context{
		ecmult_gen_ctx: secp256k1_ecmult_gen_context{built: 1},
		declassify:     0,
	}
}

// getSchnorrVerifyContext returns the precomputed Schnorr verification context
func getSchnorrVerifyContext() *secp256k1_context {
	schnorrVerifyContextOnce.Do(initSchnorrVerifyContext)
	return schnorrVerifyContext
}

// secp256k1_schnorrsig_verify verifies a BIP-340 Schnorr signature
func secp256k1_schnorrsig_verify(ctx *secp256k1_context, sig64 []byte, msg []byte, msglen int, pubkey *secp256k1_xonly_pubkey) int {
	var s secp256k1_scalar
	var e secp256k1_scalar
	var rj secp256k1_gej
	var pk secp256k1_ge
	var pkj secp256k1_gej
	var rx secp256k1_fe
	var r secp256k1_ge
	var overflow int

	if ctx == nil {
		return 0
	}
	if sig64 == nil {
		return 0
	}
	if msg == nil && msglen != 0 {
		return 0
	}
	if pubkey == nil {
		return 0
	}

	// Check signature length
	if len(sig64) < 64 {
		return 0
	}

	if !secp256k1_fe_set_b32_limit(&rx, sig64[:32]) {
		return 0
	}

	secp256k1_scalar_set_b32(&s, sig64[32:], &overflow)
	if overflow != 0 {
		return 0
	}

	if !secp256k1_xonly_pubkey_load(ctx, &pk, pubkey) {
		return 0
	}

	// Compute e - extract normalized pk.x bytes efficiently
	secp256k1_fe_normalize_var(&pk.x)
	var pkXBytes [32]byte
	secp256k1_fe_get_b32(pkXBytes[:], &pk.x)
	secp256k1_schnorrsig_challenge(&e, sig64[:32], msg, msglen, pkXBytes[:])

	// Compute rj = s*G + (-e)*pkj
	secp256k1_scalar_negate(&e, &e)
	secp256k1_gej_set_ge(&pkj, &pk)
	secp256k1_ecmult(&rj, &pkj, &e, &s)

	secp256k1_ge_set_gej_var(&r, &rj)
	if secp256k1_ge_is_infinity(&r) {
		return 0
	}

	// Normalize r.y and check if odd
	secp256k1_fe_normalize_var(&r.y)
	if secp256k1_fe_is_odd(&r.y) {
		return 0
	}

	// Normalize r.x and rx, then compare
	secp256k1_fe_normalize_var(&r.x)
	secp256k1_fe_normalize_var(&rx)

	if !secp256k1_fe_equal(&rx, &r.x) {
		return 0
	}

	return 1
}
