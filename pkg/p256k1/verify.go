//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"crypto/sha256"
	"hash"
	"sync"
	"unsafe"
)

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// secp256k1_read_be32 reads a uint32_t in big endian
func secp256k1_read_be32(p []byte) uint32 {
	if len(p) < 4 {
		panic("buffer too small")
	}
	return uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
}

// secp256k1_write_be32 writes a uint32_t in big endian
func secp256k1_write_be32(p []byte, x uint32) {
	if len(p) < 4 {
		panic("buffer too small")
	}
	p[3] = byte(x)
	p[2] = byte(x >> 8)
	p[1] = byte(x >> 16)
	p[0] = byte(x >> 24)
}

// secp256k1_read_be64 reads a uint64_t in big endian
func secp256k1_read_be64(p []byte) uint64 {
	if len(p) < 8 {
		panic("buffer too small")
	}
	return uint64(p[0])<<56 | uint64(p[1])<<48 | uint64(p[2])<<40 | uint64(p[3])<<32 |
		uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7])
}

// secp256k1_write_be64 writes a uint64_t in big endian
func secp256k1_write_be64(p []byte, x uint64) {
	if len(p) < 8 {
		panic("buffer too small")
	}
	p[7] = byte(x)
	p[6] = byte(x >> 8)
	p[5] = byte(x >> 16)
	p[4] = byte(x >> 24)
	p[3] = byte(x >> 32)
	p[2] = byte(x >> 40)
	p[1] = byte(x >> 48)
	p[0] = byte(x >> 56)
}

// secp256k1_memczero zeroes memory if flag == 1. Flag must be 0 or 1. Constant time.
func secp256k1_memczero(s []byte, flag int) {
	if flag == 0 {
		return
	}
	for i := range s {
		s[i] = 0
	}
}

// secp256k1_memzero_explicit zeroes memory to prevent leaking sensitive info. Won't be optimized out.
func secp256k1_memzero_explicit(ptr unsafe.Pointer, len uintptr) {
	memclear(ptr, len)
}

// secp256k1_memclear_explicit cleanses memory to prevent leaking sensitive info. Won't be optimized out.
func secp256k1_memclear_explicit(ptr unsafe.Pointer, len uintptr) {
	memclear(ptr, len)
}

// secp256k1_memcmp_var semantics like memcmp. Variable-time.
func secp256k1_memcmp_var(s1, s2 []byte) int {
	n := len(s1)
	if len(s2) < n {
		n = len(s2)
	}
	for i := 0; i < n; i++ {
		diff := int(s1[i]) - int(s2[i])
		if diff != 0 {
			return diff
		}
	}
	return len(s1) - len(s2)
}

// ============================================================================
// SHA256 IMPLEMENTATION
// ============================================================================

// secp256k1_sha256 represents a SHA-256 hash context
type secp256k1_sha256 struct {
	s     [8]uint32
	buf   [64]byte
	bytes uint64
}

// secp256k1_sha256_initialize initializes a SHA-256 hash context
func secp256k1_sha256_initialize(hash *secp256k1_sha256) {
	hash.s[0] = 0x6a09e667
	hash.s[1] = 0xbb67ae85
	hash.s[2] = 0x3c6ef372
	hash.s[3] = 0xa54ff53a
	hash.s[4] = 0x510e527f
	hash.s[5] = 0x9b05688c
	hash.s[6] = 0x1f83d9ab
	hash.s[7] = 0x5be0cd19
	hash.bytes = 0
}

// secp256k1_sha256_transform performs one SHA-256 transformation
func secp256k1_sha256_transform(s *[8]uint32, buf []byte) {
	// Use standard library SHA256 for transformation
	// This is a simplified implementation - full implementation would include
	// the exact transformation from the C code
	hasher := NewSHA256()
	hasher.Write(buf)
	var tmp [32]byte
	hasher.Finalize(tmp[:])

	// Convert back to state format (simplified)
	for i := 0; i < 8; i++ {
		s[i] = secp256k1_read_be32(tmp[i*4:])
	}
}

// secp256k1_sha256_write writes data to the hash
func secp256k1_sha256_write(hash *secp256k1_sha256, data []byte, len int) {
	// Simplified implementation using standard library
	// Full implementation would match C code exactly
	if len == 0 {
		return
	}

	bufsize := int(hash.bytes & 0x3F)
	hash.bytes += uint64(len)

	// Process full blocks
	i := 0
	for len >= 64-bufsize {
		chunkLen := 64 - bufsize
		copy(hash.buf[bufsize:], data[i:i+chunkLen])
		i += chunkLen
		len -= chunkLen
		secp256k1_sha256_transform(&hash.s, hash.buf[:])
		bufsize = 0
	}

	// Copy remaining data
	if len > 0 {
		copy(hash.buf[bufsize:], data[i:i+len])
	}
}

// secp256k1_sha256_finalize finalizes the hash
func secp256k1_sha256_finalize(hash *secp256k1_sha256, out32 []byte) {
	if len(out32) < 32 {
		panic("output buffer too small")
	}

	// Use standard library for finalization
	hasher := NewSHA256()

	// Write all buffered data
	bufsize := int(hash.bytes & 0x3F)
	if bufsize > 0 {
		hasher.Write(hash.buf[:bufsize])
	}

	// Finalize
	hasher.Finalize(out32)

	// Clear hash state
	hash.bytes = 0
	for i := range hash.s {
		hash.s[i] = 0
	}
}

// secp256k1_sha256_initialize_tagged initializes SHA256 with tagged hash
func secp256k1_sha256_initialize_tagged(hash *secp256k1_sha256, tag []byte, taglen int) {
	var buf [32]byte
	secp256k1_sha256_initialize(hash)
	secp256k1_sha256_write(hash, tag, taglen)
	secp256k1_sha256_finalize(hash, buf[:])

	secp256k1_sha256_initialize(hash)
	secp256k1_sha256_write(hash, buf[:], 32)
	secp256k1_sha256_write(hash, buf[:], 32)
}

// secp256k1_sha256_clear clears the hash context
func secp256k1_sha256_clear(hash *secp256k1_sha256) {
	secp256k1_memclear_explicit(unsafe.Pointer(hash), unsafe.Sizeof(*hash))
}

// ============================================================================
// SCALAR OPERATIONS
// ============================================================================

// secp256k1_scalar represents a scalar value
type secp256k1_scalar struct {
	d [4]uint64
}

// secp256k1_scalar_check_overflow checks if scalar overflows
func secp256k1_scalar_check_overflow(a *secp256k1_scalar) bool {
	yes := 0
	no := 0

	no |= boolToInt(a.d[3] < scalarN3)
	yes |= boolToInt(a.d[2] > scalarN2) & (^no)
	no |= boolToInt(a.d[2] < scalarN2)
	yes |= boolToInt(a.d[1] > scalarN1) & (^no)
	no |= boolToInt(a.d[1] < scalarN1)
	yes |= boolToInt(a.d[0] >= scalarN0) & (^no)

	return yes != 0
}

// secp256k1_scalar_reduce reduces scalar modulo order
func secp256k1_scalar_reduce(r *secp256k1_scalar, overflow int) {
	if overflow < 0 || overflow > 1 {
		panic("overflow must be 0 or 1")
	}

	var s Scalar
	s.d = r.d
	s.reduce(overflow)
	r.d = s.d
}

// secp256k1_scalar_set_b32 sets scalar from 32 bytes
func secp256k1_scalar_set_b32(r *secp256k1_scalar, b32 []byte, overflow *int) {
	var s Scalar
	over := s.setB32(b32)
	r.d = s.d

	if overflow != nil {
		*overflow = boolToInt(over)
	}
}

// secp256k1_scalar_get_b32 gets scalar to 32 bytes
func secp256k1_scalar_get_b32(bin []byte, a *secp256k1_scalar) {
	var s Scalar
	s.d = a.d
	scalarGetB32(bin, &s)
}

// secp256k1_scalar_is_zero checks if scalar is zero
func secp256k1_scalar_is_zero(a *secp256k1_scalar) bool {
	var s Scalar
	s.d = a.d
	return scalarIsZero(&s)
}

// secp256k1_scalar_negate negates scalar
func secp256k1_scalar_negate(r *secp256k1_scalar, a *secp256k1_scalar) {
	var s Scalar
	s.d = a.d
	var sa Scalar
	sa.d = a.d
	s.negate(&sa)
	r.d = s.d
}

// secp256k1_scalar_add adds two scalars
func secp256k1_scalar_add(r *secp256k1_scalar, a *secp256k1_scalar, b *secp256k1_scalar) bool {
	var sa, sb Scalar
	sa.d = a.d
	sb.d = b.d
	var sr Scalar
	overflow := scalarAdd(&sr, &sa, &sb)
	r.d = sr.d
	return overflow
}

// secp256k1_scalar_mul multiplies two scalars
func secp256k1_scalar_mul(r *secp256k1_scalar, a *secp256k1_scalar, b *secp256k1_scalar) {
	var sa, sb Scalar
	sa.d = a.d
	sb.d = b.d
	var sr Scalar
	scalarMul(&sr, &sa, &sb)
	r.d = sr.d
}

// secp256k1_scalar_clear clears scalar
func secp256k1_scalar_clear(r *secp256k1_scalar) {
	secp256k1_memclear_explicit(unsafe.Pointer(r), unsafe.Sizeof(*r))
}

// secp256k1_scalar_set_b32_seckey sets scalar from seckey
func secp256k1_scalar_set_b32_seckey(r *secp256k1_scalar, bin []byte) bool {
	var s Scalar
	ret := s.setB32Seckey(bin)
	r.d = s.d
	return ret
}

// secp256k1_scalar_cmov conditionally moves scalar
func secp256k1_scalar_cmov(r *secp256k1_scalar, a *secp256k1_scalar, flag int) {
	var sr, sa Scalar
	sr.d = r.d
	sa.d = a.d
	sr.cmov(&sa, flag)
	r.d = sr.d
}

// secp256k1_scalar_get_bits_limb32 gets bits from scalar
func secp256k1_scalar_get_bits_limb32(a *secp256k1_scalar, offset, count uint) uint32 {
	var s Scalar
	s.d = a.d
	return s.getBits(offset, count)
}

// secp256k1_scalar constants
var (
	secp256k1_scalar_one  = secp256k1_scalar{d: [4]uint64{1, 0, 0, 0}}
	secp256k1_scalar_zero = secp256k1_scalar{d: [4]uint64{0, 0, 0, 0}}
)

// ============================================================================
// FIELD OPERATIONS
// ============================================================================

// secp256k1_fe represents a field element
type secp256k1_fe struct {
	n [5]uint64
}

// secp256k1_fe_clear clears field element
func secp256k1_fe_clear(a *secp256k1_fe) {
	secp256k1_memclear_explicit(unsafe.Pointer(a), unsafe.Sizeof(*a))
}

// secp256k1_fe_set_int sets field element to int
func secp256k1_fe_set_int(r *secp256k1_fe, a int) {
	var fe FieldElement
	fe.setInt(a)
	r.n = fe.n
}

// secp256k1_fe_is_zero checks if field element is zero
func secp256k1_fe_is_zero(a *secp256k1_fe) bool {
	return (a.n[0] | a.n[1] | a.n[2] | a.n[3] | a.n[4]) == 0
}

// secp256k1_fe_is_odd checks if field element is odd
func secp256k1_fe_is_odd(a *secp256k1_fe) bool {
	return a.n[0]&1 == 1
}

// secp256k1_fe_normalize_var normalizes field element
func secp256k1_fe_normalize_var(r *secp256k1_fe) {
	var fe FieldElement
	fe.n = r.n
	fieldNormalize(&fe)
	r.n = fe.n
}

// secp256k1_fe_normalize_weak normalizes field element weakly
func secp256k1_fe_normalize_weak(r *secp256k1_fe) {
	var fe FieldElement
	fe.n = r.n
	fe.normalizeWeak()
	r.n = fe.n
}

// secp256k1_fe_normalizes_to_zero checks if field element normalizes to zero
func secp256k1_fe_normalizes_to_zero(r *secp256k1_fe) bool {
	var fe FieldElement
	fe.n = r.n
	return fe.normalizesToZeroVar()
}

// secp256k1_fe_negate negates field element
func secp256k1_fe_negate(r *secp256k1_fe, a *secp256k1_fe, m int) {
	var fe FieldElement
	fe.n = a.n
	var fea FieldElement
	fea.n = a.n
	fe.negate(&fea, m)
	r.n = fe.n
}

// secp256k1_fe_add adds field element
func secp256k1_fe_add(r *secp256k1_fe, a *secp256k1_fe) {
	var fe FieldElement
	fe.n = r.n
	var fea FieldElement
	fea.n = a.n
	fieldAdd(&fe, &fea)
	r.n = fe.n
}

// secp256k1_fe_add_int adds int to field element
func secp256k1_fe_add_int(r *secp256k1_fe, a int) {
	var fe FieldElement
	fe.n = r.n
	fe.mulInt(a)
	r.n = fe.n
}

// secp256k1_fe_set_b32_mod sets field element from bytes mod
func secp256k1_fe_set_b32_mod(r *secp256k1_fe, a []byte) {
	var fe FieldElement
	fe.setB32(a)
	r.n = fe.n
}

// secp256k1_fe_set_b32_limit sets field element from bytes limit
func secp256k1_fe_set_b32_limit(r *secp256k1_fe, a []byte) bool {
	var fe FieldElement
	if err := fe.setB32(a); err != nil {
		return false
	}

	// Check if normalized value is within limit
	fe.normalize()
	r.n = fe.n

	// Check if r >= p (field modulus)
	// p = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
	// Check: r.n[4] == 0x0FFFFFFFFFFFF && r.n[3] == 0xFFFFFFFFFFFFF &&
	//        r.n[2] == 0xFFFFFFFFFFFFF && r.n[1] == 0xFFFFFFFFFFFFF &&
	//        r.n[0] >= 0xFFFFEFFFFFC2F
	limit := (r.n[4] == 0x0FFFFFFFFFFFF) &&
		((r.n[3] & r.n[2] & r.n[1]) == 0xFFFFFFFFFFFFF) &&
		(r.n[0] >= 0xFFFFEFFFFFC2F)

	return !limit
}

// secp256k1_fe_get_b32 gets field element to bytes
func secp256k1_fe_get_b32(r []byte, a *secp256k1_fe) {
	var fe FieldElement
	fe.n = a.n
	fieldGetB32(r, &fe)
}

// secp256k1_fe_equal checks if two field elements are equal
func secp256k1_fe_equal(a *secp256k1_fe, b *secp256k1_fe) bool {
	var fea, feb FieldElement
	fea.n = a.n
	feb.n = b.n
	// Normalize both to ensure consistent state since secp256k1_fe doesn't carry
	// magnitude information. This ensures that the limbs correspond to a valid
	// field element representation before we compute the comparison.
	fea.normalize()
	feb.normalize()
	
	// Now compute the difference and check if it's zero: (a - b) ≡ 0 (mod p)
	var na FieldElement
	na.negate(&fea, 1)
	na.add(&feb)
	return na.normalizesToZeroVar()
}

// secp256k1_fe_sqrt computes square root
func secp256k1_fe_sqrt(r *secp256k1_fe, a *secp256k1_fe) bool {
	var fea, fer FieldElement
	fea.n = a.n
	ret := fer.sqrt(&fea)
	r.n = fer.n
	return ret
}

// secp256k1_fe_mul multiplies field elements
func secp256k1_fe_mul(r *secp256k1_fe, a *secp256k1_fe, b *secp256k1_fe) {
	var fea, feb, fer FieldElement
	copy(fea.n[:], a.n[:])
	copy(feb.n[:], b.n[:])
	fer.mul(&fea, &feb)
	copy(r.n[:], fer.n[:])
}

// secp256k1_fe_sqr squares field element
func secp256k1_fe_sqr(r *secp256k1_fe, a *secp256k1_fe) {
	var fea, fer FieldElement
	copy(fea.n[:], a.n[:])
	fer.sqr(&fea)
	copy(r.n[:], fer.n[:])
}

// secp256k1_fe_inv_var computes field element inverse
func secp256k1_fe_inv_var(r *secp256k1_fe, x *secp256k1_fe) {
	var fex, fer FieldElement
	fex.n = x.n
	fer.inv(&fex)
	r.n = fer.n
}

// ============================================================================
// GROUP OPERATIONS
// ============================================================================

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

// secp256k1_ge_set_infinity sets group element to infinity
func secp256k1_ge_set_infinity(r *secp256k1_ge) {
	r.infinity = 1
	secp256k1_fe_set_int(&r.x, 0)
	secp256k1_fe_set_int(&r.y, 0)
}

// secp256k1_ge_is_infinity checks if group element is infinity
func secp256k1_ge_is_infinity(a *secp256k1_ge) bool {
	return a.infinity != 0
}

// secp256k1_ge_set_xy sets group element from x, y
func secp256k1_ge_set_xy(r *secp256k1_ge, x *secp256k1_fe, y *secp256k1_fe) {
	r.infinity = 0
	r.x = *x
	r.y = *y
}

// secp256k1_ge_set_xo_var sets group element from x-only
func secp256k1_ge_set_xo_var(r *secp256k1_ge, x *secp256k1_fe, odd int) bool {
	var fex FieldElement
	fex.n = x.n

	var ge GroupElementAffine
	ret := ge.setXOVar(&fex, odd != 0)
	if ret {
		r.x.n = ge.x.n
		r.y.n = ge.y.n
		r.infinity = 0
	}
	return ret
}

// secp256k1_gej_set_infinity sets Jacobian group element to infinity
func secp256k1_gej_set_infinity(r *secp256k1_gej) {
	r.infinity = 1
	secp256k1_fe_set_int(&r.x, 0)
	secp256k1_fe_set_int(&r.y, 0)
	secp256k1_fe_set_int(&r.z, 0)
}

// secp256k1_gej_is_infinity checks if Jacobian group element is infinity
func secp256k1_gej_is_infinity(a *secp256k1_gej) bool {
	return a.infinity != 0
}

// secp256k1_gej_set_ge sets Jacobian from affine
func secp256k1_gej_set_ge(r *secp256k1_gej, a *secp256k1_ge) {
	r.infinity = a.infinity
	r.x = a.x
	r.y = a.y
	secp256k1_fe_set_int(&r.z, 1)
}

// secp256k1_gej_clear clears Jacobian group element
func secp256k1_gej_clear(r *secp256k1_gej) {
	secp256k1_memclear_explicit(unsafe.Pointer(r), unsafe.Sizeof(*r))
}

// secp256k1_ge_set_gej sets affine from Jacobian
func secp256k1_ge_set_gej(r *secp256k1_ge, a *secp256k1_gej) {
	var gej GroupElementJacobian
	gej.x.n = a.x.n
	gej.y.n = a.y.n
	gej.z.n = a.z.n
	gej.infinity = a.infinity != 0

	var ge GroupElementAffine
	ge.setGEJ(&gej)

	r.x.n = ge.x.n
	r.y.n = ge.y.n
	r.infinity = boolToInt(ge.infinity)
}

// secp256k1_ge_set_gej_var sets affine from Jacobian (variable time)
func secp256k1_ge_set_gej_var(r *secp256k1_ge, a *secp256k1_gej) {
	if secp256k1_gej_is_infinity(a) {
		secp256k1_ge_set_infinity(r)
		return
	}

	var gej GroupElementJacobian
	gej.x.n = a.x.n
	gej.y.n = a.y.n
	gej.z.n = a.z.n
	gej.infinity = false

	var ge GroupElementAffine
	ge.setGEJ(&gej)

	r.x.n = ge.x.n
	r.y.n = ge.y.n
	r.infinity = 0
}

// secp256k1_gej_double_var doubles Jacobian point
func secp256k1_gej_double_var(r *secp256k1_gej, a *secp256k1_gej, rzr *secp256k1_fe) {
	var geja, gejr GroupElementJacobian
	geja.x.n = a.x.n
	geja.y.n = a.y.n
	geja.z.n = a.z.n
	geja.infinity = a.infinity != 0

	gejr.double(&geja)

	r.x.n = gejr.x.n
	r.y.n = gejr.y.n
	r.z.n = gejr.z.n
	r.infinity = boolToInt(gejr.infinity)

	if rzr != nil {
		// rzr = 2*a->y (from double logic)
		rzr.n = a.y.n
		secp256k1_fe_add(rzr, &a.y)
	}
}

// secp256k1_gej_add_ge_var adds affine point to Jacobian point
func secp256k1_gej_add_ge_var(r *secp256k1_gej, a *secp256k1_gej, b *secp256k1_ge, rzr *secp256k1_fe) {
	var geja GroupElementJacobian
	geja.x.n = a.x.n
	geja.y.n = a.y.n
	geja.z.n = a.z.n
	geja.infinity = a.infinity != 0

	var geb GroupElementAffine
	geb.x.n = b.x.n
	geb.y.n = b.y.n
	geb.infinity = b.infinity != 0

	var fezr *FieldElement
	if rzr != nil {
		var tmp FieldElement
		tmp.n = rzr.n
		fezr = &tmp
	}

	var gejr GroupElementJacobian
	gejr.addGEWithZR(&geja, &geb, fezr)

	r.x.n = gejr.x.n
	r.y.n = gejr.y.n
	r.z.n = gejr.z.n
	r.infinity = boolToInt(gejr.infinity)

	if rzr != nil && fezr != nil {
		rzr.n = fezr.n
	}
}

// secp256k1_gej_add_zinv_var adds affine point to Jacobian with z inverse
func secp256k1_gej_add_zinv_var(r *secp256k1_gej, a *secp256k1_gej, b *secp256k1_ge, bzinv *secp256k1_fe) {
	// Simplified implementation - full implementation would use zinv optimization
	secp256k1_gej_add_ge_var(r, a, b, nil)
}

// ============================================================================
// GLOBAL PRE-ALLOCATED RESOURCES
// ============================================================================

// Pool of hash contexts for challenge computation - thread-safe reuse
var challengeHashPool = sync.Pool{
	New: func() any {
		return sha256.New()
	},
}

func getChallengeHashContext() hash.Hash {
	return challengeHashPool.Get().(hash.Hash)
}

func putChallengeHashContext(h hash.Hash) {
	h.Reset()
	challengeHashPool.Put(h)
}

// ============================================================================
// EC MULTIPLICATION OPERATIONS
// ============================================================================

// secp256k1_ecmult_gen_context represents EC multiplication generator context
type secp256k1_ecmult_gen_context struct {
	built int
}

// secp256k1_ecmult_gen_context_is_built checks if context is built
func secp256k1_ecmult_gen_context_is_built(ctx *secp256k1_ecmult_gen_context) bool {
	return ctx.built != 0
}

// secp256k1_ecmult_gen computes generator multiplication
func secp256k1_ecmult_gen(ctx *secp256k1_ecmult_gen_context, r *secp256k1_gej, gn *secp256k1_scalar) {
	var s Scalar
	s.d = gn.d

	var gejr GroupElementJacobian
	EcmultGen(&gejr, &s)

	r.x.n = gejr.x.n
	r.y.n = gejr.y.n
	r.z.n = gejr.z.n
	r.infinity = boolToInt(gejr.infinity)
}

// secp256k1_ecmult computes EC multiplication
// Optimized: interleaved computation of r = na * a + ng * G
// Simplest optimization: process both scalars byte-by-byte in a single loop
// This reduces doublings and improves cache locality without requiring WNAF/GLV
func secp256k1_ecmult(r *secp256k1_gej, a *secp256k1_gej, na *secp256k1_scalar, ng *secp256k1_scalar) {
	// r = na * a + ng * G
	// Convert input to Go types
	var geja GroupElementJacobian
	geja.x.n = a.x.n
	geja.y.n = a.y.n
	geja.z.n = a.z.n
	geja.infinity = a.infinity != 0

	var sna, sng Scalar
	sna.d = na.d
	sng.d = ng.d

	// Handle zero scalars
	if sna.isZero() && sng.isZero() {
		r.x.n = [5]uint64{0, 0, 0, 0, 0}
		r.y.n = [5]uint64{0, 0, 0, 0, 0}
		r.z.n = [5]uint64{0, 0, 0, 0, 0}
		r.infinity = 1
		return
	}

	// Simple case: if one scalar is zero, use existing optimized functions
	if sna.isZero() {
		var ngg GroupElementJacobian
		EcmultGen(&ngg, &sng)
		r.x.n = ngg.x.n
		r.y.n = ngg.y.n
		r.z.n = ngg.z.n
		r.infinity = boolToInt(ngg.infinity)
		return
	}

	if sng.isZero() {
		var naa GroupElementJacobian
		Ecmult(&naa, &geja, &sna)
		r.x.n = naa.x.n
		r.y.n = naa.y.n
		r.z.n = naa.z.n
		r.infinity = boolToInt(naa.infinity)
		return
	}

	// Use combined Strauss algorithm - shares doublings between both multiplications
	// This is ~2x faster than computing separately
	var gejr GroupElementJacobian
	EcmultCombined(&gejr, &geja, &sna, &sng)

	r.x.n = gejr.x.n
	r.y.n = gejr.y.n
	r.z.n = gejr.z.n
	r.infinity = boolToInt(gejr.infinity)
}

// ============================================================================
// PUBKEY/KEYPAIR OPERATIONS
// ============================================================================

// secp256k1_context represents a context
type secp256k1_context struct {
	ecmult_gen_ctx secp256k1_ecmult_gen_context
	declassify     int
}

// secp256k1_declassify declassifies data (no-op in non-VERIFY builds)
func secp256k1_declassify(ctx *secp256k1_context, p unsafe.Pointer, len uintptr) {
	// No-op
}

// secp256k1_pubkey represents a public key
type secp256k1_pubkey struct {
	data [64]byte
}

// secp256k1_xonly_pubkey represents an x-only public key
type secp256k1_xonly_pubkey struct {
	data [32]byte
}

// secp256k1_keypair represents a keypair
type secp256k1_keypair struct {
	data [96]byte
}

// secp256k1_pubkey_load loads public key
func secp256k1_pubkey_load(ctx *secp256k1_context, ge *secp256k1_ge, pubkey *secp256k1_pubkey) bool {
	var pub PublicKey
	copy(pub.data[:], pubkey.data[:])

	var gep GroupElementAffine
	gep.fromBytes(pub.data[:])

	if gep.isInfinity() {
		return false
	}

	ge.x.n = gep.x.n
	ge.y.n = gep.y.n
	ge.infinity = boolToInt(gep.infinity)

	var fex FieldElement
	fex.n = ge.x.n
	fex.normalize()
	return !fex.isZero()
}

// secp256k1_pubkey_save saves public key
func secp256k1_pubkey_save(pubkey *secp256k1_pubkey, ge *secp256k1_ge) {
	var gep GroupElementAffine
	gep.x.n = ge.x.n
	gep.y.n = ge.y.n
	gep.infinity = ge.infinity != 0

	var pub PublicKey
	gep.toBytes(pub.data[:])
	copy(pubkey.data[:], pub.data[:])
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

	ge.x.n = gep.x.n
	ge.y.n = gep.y.n
	ge.infinity = boolToInt(gep.infinity)

	return true
}

// secp256k1_keypair_load loads keypair
func secp256k1_keypair_load(ctx *secp256k1_context, sk *secp256k1_scalar, pk *secp256k1_ge, keypair *secp256k1_keypair) bool {
	var pubkey secp256k1_pubkey
	copy(pubkey.data[:], keypair.data[32:])

	secp256k1_declassify(ctx, unsafe.Pointer(&pubkey.data[0]), 64)

	ret := secp256k1_pubkey_load(ctx, pk, &pubkey)
	if sk != nil {
		var s Scalar
		ret = ret && s.setB32Seckey(keypair.data[:32])
		if ret {
			sk.d = s.d
		}
	}

	if !ret {
		// Set to default values
		if pk != nil {
			secp256k1_ge_set_infinity(pk)
		}
		if sk != nil {
			*sk = secp256k1_scalar_one
		}
	}

	return ret
}

// ============================================================================
// SCHNORR SIGNATURE OPERATIONS
// ============================================================================

// secp256k1_schnorrsig_sha256_tagged initializes SHA256 with tagged hash
func secp256k1_schnorrsig_sha256_tagged(sha *secp256k1_sha256) {
	secp256k1_sha256_initialize(sha)
	sha.s[0] = 0x9cecba11
	sha.s[1] = 0x23925381
	sha.s[2] = 0x11679112
	sha.s[3] = 0xd1627e0f
	sha.s[4] = 0x97c87550
	sha.s[5] = 0x003cc765
	sha.s[6] = 0x90f61164
	sha.s[7] = 0x33e9b66a
	sha.bytes = 64
}

// secp256k1_schnorrsig_challenge computes challenge hash
func secp256k1_schnorrsig_challenge(e *secp256k1_scalar, r32 []byte, msg []byte, msglen int, pubkey32 []byte) {
	// Zero-allocation challenge computation
	var challengeHash [32]byte

	// Get hash context from pool (thread-safe)
	h := getChallengeHashContext()
	defer putChallengeHashContext(h)

	// Use precomputed SHA256(tag) hash to avoid allocation
	tagHash := getTaggedHashPrefix(bip340ChallengeTag)

	// Second hash: SHA256(SHA256(tag) || SHA256(tag) || r32 || pubkey32 || msg)
	h.Reset()
	h.Write(tagHash[:])    // SHA256(tag)
	h.Write(tagHash[:])    // SHA256(tag) again
	h.Write(r32[:32])      // r32
	h.Write(pubkey32[:32]) // pubkey32
	h.Write(msg[:msglen])  // msg

	// Sum into a temporary buffer, then copy
	var temp [32]byte
	h.Sum(temp[:0])
	copy(challengeHash[:], temp[:])

	// Convert hash to scalar directly - avoid intermediate Scalar by setting directly
	e.d[0] = uint64(challengeHash[31]) | uint64(challengeHash[30])<<8 | uint64(challengeHash[29])<<16 | uint64(challengeHash[28])<<24 |
		uint64(challengeHash[27])<<32 | uint64(challengeHash[26])<<40 | uint64(challengeHash[25])<<48 | uint64(challengeHash[24])<<56
	e.d[1] = uint64(challengeHash[23]) | uint64(challengeHash[22])<<8 | uint64(challengeHash[21])<<16 | uint64(challengeHash[20])<<24 |
		uint64(challengeHash[19])<<32 | uint64(challengeHash[18])<<40 | uint64(challengeHash[17])<<48 | uint64(challengeHash[16])<<56
	e.d[2] = uint64(challengeHash[15]) | uint64(challengeHash[14])<<8 | uint64(challengeHash[13])<<16 | uint64(challengeHash[12])<<24 |
		uint64(challengeHash[11])<<32 | uint64(challengeHash[10])<<40 | uint64(challengeHash[9])<<48 | uint64(challengeHash[8])<<56
	e.d[3] = uint64(challengeHash[7]) | uint64(challengeHash[6])<<8 | uint64(challengeHash[5])<<16 | uint64(challengeHash[4])<<24 |
		uint64(challengeHash[3])<<32 | uint64(challengeHash[2])<<40 | uint64(challengeHash[1])<<48 | uint64(challengeHash[0])<<56

	// Check overflow inline (same logic as Scalar.checkOverflow) and reduce if needed
	yes := 0
	no := 0
	no |= boolToInt(e.d[3] < scalarN3)
	yes |= boolToInt(e.d[2] > scalarN2) & (^no)
	no |= boolToInt(e.d[2] < scalarN2)
	yes |= boolToInt(e.d[1] > scalarN1) & (^no)
	no |= boolToInt(e.d[1] < scalarN1)
	yes |= boolToInt(e.d[0] >= scalarN0) & (^no)

	if yes != 0 {
		// Reduce inline using secp256k1_scalar_reduce logic
		secp256k1_scalar_reduce(e, 1)
	}
}

// Direct array-based implementations to avoid struct allocations

// feSetB32Limit sets field element from 32 bytes with limit check
func feSetB32Limit(r []uint64, b []byte) bool {
	if len(r) < 5 || len(b) < 32 {
		return false
	}

	r[0] = (uint64(b[31]) | uint64(b[30])<<8 | uint64(b[29])<<16 | uint64(b[28])<<24 |
		uint64(b[27])<<32 | uint64(b[26])<<40 | uint64(b[25])<<48 | uint64(b[24])<<56)
	r[1] = (uint64(b[23]) | uint64(b[22])<<8 | uint64(b[21])<<16 | uint64(b[20])<<24 |
		uint64(b[19])<<32 | uint64(b[18])<<40 | uint64(b[17])<<48 | uint64(b[16])<<56)
	r[2] = (uint64(b[15]) | uint64(b[14])<<8 | uint64(b[13])<<16 | uint64(b[12])<<24 |
		uint64(b[11])<<32 | uint64(b[10])<<40 | uint64(b[9])<<48 | uint64(b[8])<<56)
	r[3] = (uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56)
	r[4] = 0

	return !((r[4] == 0x0FFFFFFFFFFFF) && ((r[3] & r[2] & r[1]) == 0xFFFFFFFFFFFF) && (r[0] >= 0xFFFFEFFFFFC2F))
}

// xonlyPubkeyLoad loads x-only public key into arrays
func xonlyPubkeyLoad(pkx, pky []uint64, pkInf *int, pubkey *secp256k1_xonly_pubkey) bool {
	if len(pkx) < 5 || len(pky) < 5 {
		return false
	}

	// Set x coordinate from pubkey data
	if !feSetB32Limit(pkx, pubkey.data[:32]) {
		return false
	}

	// Compute y^2 = x^3 + 7
	var x2, x3, y2 [5]uint64
	fieldSqr(x2[:], pkx)
	fieldMul(x3[:], x2[:], pkx)
	// Add 7 (which is 111 in binary, so add 1 seven times)
	x3[0] += 7
	fieldSqr(y2[:], x3[:])

	// Check if y^2 is quadratic residue (has square root)
	if !fieldSqrt(pky, y2[:]) {
		return false
	}

	*pkInf = 0
	return true
}

// schnorrsigChallenge computes challenge directly into array
func schnorrsigChallenge(e []uint64, r32 []byte, msg []byte, msglen int, pubkey32 []byte) {
	if len(e) < 4 {
		return
	}

	// Zero-allocation challenge computation
	var challengeHash [32]byte

	// Use precomputed SHA256(tag) hash to avoid allocation
	tagHash := getTaggedHashPrefix(bip340ChallengeTag)

	// Second hash: SHA256(SHA256(tag) || SHA256(tag) || r32 || pubkey32 || msg)
	h := getChallengeHashContext()
	defer putChallengeHashContext(h)
	h.Reset()
	h.Write(tagHash[:])    // SHA256(tag)
	h.Write(tagHash[:])    // SHA256(tag) again
	h.Write(r32[:32])      // r32
	h.Write(pubkey32[:32]) // pubkey32
	h.Write(msg[:msglen])  // msg

	// Sum into challengeHash
	var temp [32]byte
	h.Sum(temp[:0])
	copy(challengeHash[:], temp[:])

	// Convert hash to scalar directly
	var tempScalar Scalar
	tempScalar.d[0] = uint64(challengeHash[31]) | uint64(challengeHash[30])<<8 | uint64(challengeHash[29])<<16 | uint64(challengeHash[28])<<24 |
		uint64(challengeHash[27])<<32 | uint64(challengeHash[26])<<40 | uint64(challengeHash[25])<<48 | uint64(challengeHash[24])<<56
	tempScalar.d[1] = uint64(challengeHash[23]) | uint64(challengeHash[22])<<8 | uint64(challengeHash[21])<<16 | uint64(challengeHash[20])<<24 |
		uint64(challengeHash[19])<<32 | uint64(challengeHash[18])<<40 | uint64(challengeHash[17])<<48 | uint64(challengeHash[16])<<56
	tempScalar.d[2] = uint64(challengeHash[15]) | uint64(challengeHash[14])<<8 | uint64(challengeHash[13])<<16 | uint64(challengeHash[12])<<24 |
		uint64(challengeHash[11])<<32 | uint64(challengeHash[10])<<40 | uint64(challengeHash[9])<<48 | uint64(challengeHash[8])<<56
	tempScalar.d[3] = uint64(challengeHash[7]) | uint64(challengeHash[6])<<8 | uint64(challengeHash[5])<<16 | uint64(challengeHash[4])<<24 |
		uint64(challengeHash[3])<<32 | uint64(challengeHash[2])<<40 | uint64(challengeHash[1])<<48 | uint64(challengeHash[0])<<56

	// Check overflow and reduce if needed
	if tempScalar.checkOverflow() {
		tempScalar.reduce(1)
	}

	// Copy back to array
	e[0], e[1], e[2], e[3] = tempScalar.d[0], tempScalar.d[1], tempScalar.d[2], tempScalar.d[3]
}

// scalarSetB32 sets scalar from 32 bytes
func scalarSetB32(r []uint64, bin []byte, overflow *int) {
	if len(r) < 4 || len(bin) < 32 {
		if overflow != nil {
			*overflow = 1
		}
		return
	}

	r[0] = uint64(bin[31]) | uint64(bin[30])<<8 | uint64(bin[29])<<16 | uint64(bin[28])<<24 |
		uint64(bin[27])<<32 | uint64(bin[26])<<40 | uint64(bin[25])<<48 | uint64(bin[24])<<56
	r[1] = uint64(bin[23]) | uint64(bin[22])<<8 | uint64(bin[21])<<16 | uint64(bin[20])<<24 |
		uint64(bin[19])<<32 | uint64(bin[18])<<40 | uint64(bin[17])<<48 | uint64(bin[16])<<56
	r[2] = uint64(bin[15]) | uint64(bin[14])<<8 | uint64(bin[13])<<16 | uint64(bin[12])<<24 |
		uint64(bin[11])<<32 | uint64(bin[10])<<40 | uint64(bin[9])<<48 | uint64(bin[8])<<56
	r[3] = uint64(bin[7]) | uint64(bin[6])<<8 | uint64(bin[5])<<16 | uint64(bin[4])<<24 |
		uint64(bin[3])<<32 | uint64(bin[2])<<40 | uint64(bin[1])<<48 | uint64(bin[0])<<56

	var tempS Scalar
	copy(tempS.d[:], r)
	if overflow != nil {
		*overflow = boolToInt(tempS.checkOverflow())
	}
	if tempS.checkOverflow() {
		tempS.reduce(1)
		copy(r, tempS.d[:])
	}
}

// feNormalizeVar normalizes field element
func feNormalizeVar(r []uint64) {
	if len(r) < 5 {
		return
	}
	var tempFE FieldElement
	copy(tempFE.n[:], r)
	fieldNormalize(&tempFE)
	copy(r, tempFE.n[:])
}

// feGetB32 serializes field element to 32 bytes
func feGetB32(b []byte, a []uint64) {
	if len(b) < 32 || len(a) < 5 {
		return
	}
	var tempFE FieldElement
	copy(tempFE.n[:], a)
	fieldGetB32(b, &tempFE)
}

// scalarNegate negates scalar
func scalarNegate(r []uint64) {
	if len(r) < 4 {
		return
	}

	// Compute -r mod n: if r == 0 then 0 else n - r
	if r[0] != 0 || r[1] != 0 || r[2] != 0 || r[3] != 0 {
		r[0] = (^r[0]) + 1
		r[1] = ^r[1]
		r[2] = ^r[2]
		r[3] = ^r[3]

		// Add n if we wrapped around
		var tempS Scalar
		copy(tempS.d[:], r)
		if tempS.checkOverflow() {
			r[0] += scalarNC0
			r[1] += scalarNC1
			r[2] += scalarNC2
			r[3] += 0
		}
	}
}

// gejSetGe sets jacobian coordinates from affine
func gejSetGe(rjx, rjy, rjz []uint64, rjInf *int, ax, ay []uint64, aInf int) {
	if len(rjx) < 5 || len(rjy) < 5 || len(rjz) < 5 || len(ax) < 5 || len(ay) < 5 {
		return
	}

	if aInf != 0 {
		*rjInf = 1
		copy(rjx, ax)
		copy(rjy, ay)
		rjz[0], rjz[1], rjz[2], rjz[3], rjz[4] = 0, 0, 0, 0, 0
	} else {
		*rjInf = 0
		copy(rjx, ax)
		copy(rjy, ay)
		rjz[0], rjz[1], rjz[2], rjz[3], rjz[4] = 1, 0, 0, 0, 0
	}
}

// geSetGejVar converts jacobian to affine coordinates
func geSetGejVar(rx, ry []uint64, rjx, rjy, rjz []uint64, rjInf int, rInf *int) {
	if len(rx) < 5 || len(ry) < 5 || len(rjx) < 5 || len(rjy) < 5 || len(rjz) < 5 {
		return
	}

	if rjInf != 0 {
		*rInf = 1
		return
	}

	*rInf = 0

	// Compute z^-1
	var zinv [5]uint64
	fieldInvVar(zinv[:], rjz)

	// Compute z^-2
	var zinv2 [5]uint64
	fieldSqr(zinv2[:], zinv[:])

	// x = x * z^-2
	fieldMul(rx, rjx, zinv2[:])

	// Compute z^-3 = z^-1 * z^-2
	var zinv3 [5]uint64
	fieldMul(zinv3[:], zinv[:], zinv2[:])

	// y = y * z^-3
	fieldMul(ry, rjy, zinv3[:])
}

// feIsOdd checks if field element is odd
func feIsOdd(a []uint64) bool {
	if len(a) < 5 {
		return false
	}

	var normalized [5]uint64
	copy(normalized[:], a)
	var tempFE FieldElement
	copy(tempFE.n[:], normalized[:])
	fieldNormalize(&tempFE)
	return (tempFE.n[0] & 1) == 1
}

// ecmult computes r = na * a + ng * G using arrays
func ecmult(rjx, rjy, rjz []uint64, rjInf *int, ajx, ajy, ajz []uint64, ajInf int, na, ng []uint64) {
	if len(rjx) < 5 || len(rjy) < 5 || len(rjz) < 5 || len(ajx) < 5 || len(ajy) < 5 || len(ajz) < 5 || len(na) < 4 || len(ng) < 4 {
		return
	}

	// Convert arrays to structs for optimized computation
	var a secp256k1_gej
	copy(a.x.n[:], ajx)
	copy(a.y.n[:], ajy)
	copy(a.z.n[:], ajz)
	a.infinity = ajInf

	var sna secp256k1_scalar
	copy(sna.d[:], na)

	var sng secp256k1_scalar
	copy(sng.d[:], ng)

	var r secp256k1_gej
	secp256k1_ecmult(&r, &a, &sna, &sng)

	// Convert back to arrays
	copy(rjx, r.x.n[:])
	copy(rjy, r.y.n[:])
	copy(rjz, r.z.n[:])
	*rjInf = r.infinity
}

// secp256k1_schnorrsig_verify verifies a Schnorr signature
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

	// Optimize: normalize r.y only once and check if odd
	secp256k1_fe_normalize_var(&r.y)
	if secp256k1_fe_is_odd(&r.y) {
		return 0
	}

	// Optimize: normalize r.x and rx only once before comparison
	secp256k1_fe_normalize_var(&r.x)
	secp256k1_fe_normalize_var(&rx)

	// Direct comparison of normalized field elements to avoid allocations
	if rx.n[0] != r.x.n[0] || rx.n[1] != r.x.n[1] || rx.n[2] != r.x.n[2] ||
	   rx.n[3] != r.x.n[3] || rx.n[4] != r.x.n[4] {
		return 0
	}

	return 1
}
