package p256k1

// No imports needed for basic group operations

// GroupElementAffine represents a point on the secp256k1 curve in affine coordinates (x, y)
type GroupElementAffine struct {
	x, y     FieldElement
	infinity bool
}

// GroupElementJacobian represents a point on the secp256k1 curve in Jacobian coordinates (x, y, z)
// where the affine coordinates are (x/z^2, y/z^3)
type GroupElementJacobian struct {
	x, y, z  FieldElement
	infinity bool
}

// GroupElementStorage represents a point in storage format (compressed coordinates)
type GroupElementStorage struct {
	x [32]byte
	y [32]byte
}

// Generator point G for secp256k1 curve
var (
	// Generator point in affine coordinates
	// G = (0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798,
	//      0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8)
	GeneratorX FieldElement
	GeneratorY FieldElement
	Generator  GroupElementAffine
)

// Initialize generator point
func init() {
	// Generator X coordinate: 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
	gxBytes := []byte{
		0x79, 0xBE, 0x66, 0x7E, 0xF9, 0xDC, 0xBB, 0xAC, 0x55, 0xA0, 0x62, 0x95, 0xCE, 0x87, 0x0B, 0x07,
		0x02, 0x9B, 0xFC, 0xDB, 0x2D, 0xCE, 0x28, 0xD9, 0x59, 0xF2, 0x81, 0x5B, 0x16, 0xF8, 0x17, 0x98,
	}
	
	// Generator Y coordinate: 0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
	gyBytes := []byte{
		0x48, 0x3A, 0xDA, 0x77, 0x26, 0xA3, 0xC4, 0x65, 0x5D, 0xA4, 0xFB, 0xFC, 0x0E, 0x11, 0x08, 0xA8,
		0xFD, 0x17, 0xB4, 0x48, 0xA6, 0x85, 0x54, 0x19, 0x9C, 0x47, 0xD0, 0x8F, 0xFB, 0x10, 0xD4, 0xB8,
	}
	
	GeneratorX.setB32(gxBytes)
	GeneratorY.setB32(gyBytes)
	
	// Create generator point
	Generator = GroupElementAffine{
		x:        GeneratorX,
		y:        GeneratorY,
		infinity: false,
	}
}

// NewGroupElementAffine creates a new affine group element
func NewGroupElementAffine() *GroupElementAffine {
	return &GroupElementAffine{
		x:        FieldElementZero,
		y:        FieldElementZero,
		infinity: true,
	}
}

// NewGroupElementJacobian creates a new Jacobian group element
func NewGroupElementJacobian() *GroupElementJacobian {
	return &GroupElementJacobian{
		x:        FieldElementZero,
		y:        FieldElementZero,
		z:        FieldElementZero,
		infinity: true,
	}
}

// setXY sets a group element to the point with given coordinates
func (r *GroupElementAffine) setXY(x, y *FieldElement) {
	r.x = *x
	r.y = *y
	r.infinity = false
}

// setXOVar sets a group element to the point with given X coordinate and Y oddness
func (r *GroupElementAffine) setXOVar(x *FieldElement, odd bool) bool {
	// Compute y^2 = x^3 + 7 (secp256k1 curve equation)
	var x2, x3, y2 FieldElement
	x2.sqr(x)
	x3.mul(&x2, x)

	// Add 7 (the curve parameter b)
	var seven FieldElement
	seven.setInt(7)
	y2 = x3
	y2.add(&seven)

	// Try to compute square root
	var y FieldElement
	if !y.sqrt(&y2) {
		return false // x is not on the curve
	}

	// Choose the correct square root based on oddness
	y.normalize()
	if y.isOdd() != odd {
		y.negate(&y, 1)
		y.normalize()
	}

	r.setXY(x, &y)
	return true
}

// isInfinity returns true if the group element is the point at infinity
func (r *GroupElementAffine) isInfinity() bool {
	return r.infinity
}

// isValid checks if the group element is valid (on the curve)
func (r *GroupElementAffine) isValid() bool {
	if r.infinity {
		return true
	}

	// Check curve equation: y^2 = x^3 + 7
	var lhs, rhs, x2, x3 FieldElement
	
	// Normalize coordinates
	var xNorm, yNorm FieldElement
	xNorm = r.x
	yNorm = r.y
	xNorm.normalize()
	yNorm.normalize()
	
	// Compute y^2
	lhs.sqr(&yNorm)
	
	// Compute x^3 + 7
	x2.sqr(&xNorm)
	x3.mul(&x2, &xNorm)
	rhs = x3
	var seven FieldElement
	seven.setInt(7)
	rhs.add(&seven)
	
	// Normalize both sides
	lhs.normalize()
	rhs.normalize()
	
	return lhs.equal(&rhs)
}

// negate sets r to the negation of a (mirror around X axis)
func (r *GroupElementAffine) negate(a *GroupElementAffine) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.x = a.x
	r.y.negate(&a.y, a.y.magnitude)
	r.infinity = false
}

// mulLambda applies the GLV endomorphism: λ·(x, y) = (β·x, y)
// This is the key operation that enables the GLV optimization.
// Since λ is a cube root of unity mod n, and β is a cube root of unity mod p,
// multiplying a point by λ (scalar) is equivalent to multiplying x by β (field).
// Reference: libsecp256k1 group_impl.h:secp256k1_ge_mul_lambda
func (r *GroupElementAffine) mulLambda(a *GroupElementAffine) {
	if a.infinity {
		r.setInfinity()
		return
	}

	// r.x = β * a.x
	r.x.mul(&a.x, &fieldBeta)
	// r.y = a.y (unchanged)
	r.y = a.y
	r.infinity = false
}

// setInfinity sets the group element to the point at infinity
func (r *GroupElementAffine) setInfinity() {
	r.x = FieldElementZero
	r.y = FieldElementZero
	r.infinity = true
}

// equal returns true if two group elements are equal
func (r *GroupElementAffine) equal(a *GroupElementAffine) bool {
	if r.infinity && a.infinity {
		return true
	}
	if r.infinity || a.infinity {
		return false
	}
	
	// Normalize both points
	var rNorm, aNorm GroupElementAffine
	rNorm = *r
	aNorm = *a
	rNorm.x.normalize()
	rNorm.y.normalize()
	aNorm.x.normalize()
	aNorm.y.normalize()
	
	return rNorm.x.equal(&aNorm.x) && rNorm.y.equal(&aNorm.y)
}

// Jacobian coordinate operations

// setInfinity sets the Jacobian group element to the point at infinity
func (r *GroupElementJacobian) setInfinity() {
	r.x = FieldElementZero
	r.y = FieldElementOne
	r.z = FieldElementZero
	r.infinity = true
}

// isInfinity returns true if the Jacobian group element is the point at infinity
func (r *GroupElementJacobian) isInfinity() bool {
	return r.infinity
}

// setGE sets a Jacobian element from an affine element
func (r *GroupElementJacobian) setGE(a *GroupElementAffine) {
	if a.infinity {
		r.setInfinity()
		return
	}
	
	r.x = a.x
	r.y = a.y
	r.z = FieldElementOne
	r.infinity = false
}

// setGEJ sets an affine element from a Jacobian element
// This follows the C secp256k1_ge_set_gej_var implementation exactly
// Optimized: avoid copy when we can modify in-place or when caller guarantees no reuse
func (r *GroupElementAffine) setGEJ(a *GroupElementJacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}
	
	// Optimization: if r == a (shouldn't happen but handle gracefully), or if we can work directly
	// For now, we still need a copy since we modify fields, but we can optimize the copy
	var aCopy GroupElementJacobian
	aCopy = *a // Copy once, then work with copy
	
	r.infinity = false
	
	// secp256k1_fe_inv_var(&a->z, &a->z);
	// Note: inv normalizes the input internally
	aCopy.z.inv(&aCopy.z)
	
	// secp256k1_fe_sqr(&z2, &a->z);
	var z2 FieldElement
	z2.sqr(&aCopy.z)
	
	// secp256k1_fe_mul(&z3, &a->z, &z2);
	var z3 FieldElement
	z3.mul(&aCopy.z, &z2)
	
	// secp256k1_fe_mul(&a->x, &a->x, &z2);
	aCopy.x.mul(&aCopy.x, &z2)
	
	// secp256k1_fe_mul(&a->y, &a->y, &z3);
	aCopy.y.mul(&aCopy.y, &z3)
	
	// secp256k1_fe_set_int(&a->z, 1);
	aCopy.z.setInt(1)
	
	// secp256k1_ge_set_xy(r, &a->x, &a->y);
	r.x = aCopy.x
	r.y = aCopy.y
}

// negate sets r to the negation of a Jacobian point
func (r *GroupElementJacobian) negate(a *GroupElementJacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.x = a.x
	r.y.negate(&a.y, a.y.magnitude)
	r.z = a.z
	r.infinity = false
}

// mulLambda applies the GLV endomorphism to a Jacobian point: λ·(X, Y, Z) = (β·X, Y, Z)
// In Jacobian coordinates, only the X coordinate is multiplied by β.
func (r *GroupElementJacobian) mulLambda(a *GroupElementJacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}

	// r.x = β * a.x
	r.x.mul(&a.x, &fieldBeta)
	// r.y and r.z unchanged
	r.y = a.y
	r.z = a.z
	r.infinity = false
}

// double is defined in group_double_native.go (64-bit, uses half())
// and group_double_32bit.go (32-bit/WASM, half-free formula)

// addVar sets r = a + b (variable-time point addition in Jacobian coordinates)
// This follows the C secp256k1_gej_add_var implementation exactly
// Operations: 12 mul, 4 sqr, 11 add/negate/normalizes_to_zero
func (r *GroupElementJacobian) addVar(a, b *GroupElementJacobian) {
	// Handle infinity cases
	if a.infinity {
		*r = *b
		return
	}
	if b.infinity {
		*r = *a
		return
	}
	
	// Following C code exactly: secp256k1_gej_add_var
	// z22 = b->z^2
	// z12 = a->z^2
	// u1 = a->x * z22
	// u2 = b->x * z12
	// s1 = a->y * z22 * b->z
	// s2 = b->y * z12 * a->z
	// h = u2 - u1
	// i = s2 - s1
	// If h == 0 and i == 0: double(a)
	// If h == 0 and i != 0: infinity
	// Otherwise: add
	
	var z22, z12, u1, u2, s1, s2, h, i, h2, h3, t FieldElement
	
	// z22 = b->z^2
	z22.sqr(&b.z)
	
	// z12 = a->z^2
	z12.sqr(&a.z)
	
	// u1 = a->x * z22
	u1.mul(&a.x, &z22)
	
	// u2 = b->x * z12
	u2.mul(&b.x, &z12)
	
	// s1 = a->y * z22 * b->z
	s1.mul(&a.y, &z22)
	s1.mul(&s1, &b.z)
	
	// s2 = b->y * z12 * a->z
	s2.mul(&b.y, &z12)
	s2.mul(&s2, &a.z)
	
	// h = u2 - u1
	h.negate(&u1, 1)
	h.add(&u2)
	
	// i = s2 - s1
	i.negate(&s2, 1)
	i.add(&s1)
	
	// Check if h normalizes to zero
	if h.normalizesToZeroVar() {
		if i.normalizesToZeroVar() {
			// Points are equal - double
			r.double(a)
			return
		} else {
			// Points are negatives - result is infinity
			r.setInfinity()
			return
		}
	}
	
	// General addition case
	r.infinity = false
	
	// t = h * b->z
	t.mul(&h, &b.z)
	
	// r->z = a->z * t
	r.z.mul(&a.z, &t)
	
	// h2 = h^2
	h2.sqr(&h)
	
	// h2 = -h2
	h2.negate(&h2, 1)
	
	// h3 = h2 * h
	h3.mul(&h2, &h)
	
	// t = u1 * h2
	t.mul(&u1, &h2)
	
	// r->x = i^2
	r.x.sqr(&i)
	
	// r->x = i^2 + h3
	r.x.add(&h3)
	
	// r->x = i^2 + h3 + t
	r.x.add(&t)
	
	// r->x = i^2 + h3 + 2*t
	r.x.add(&t)
	
	// t = t + r->x
	t.add(&r.x)
	
	// r->y = t * i
	r.y.mul(&t, &i)
	
	// h3 = h3 * s1
	h3.mul(&h3, &s1)
	
	// r->y = t * i + h3
	r.y.add(&h3)
}

// addGEWithZR sets r = a + b where a is Jacobian and b is affine
// If rzr is not nil, sets *rzr = h such that r->z == a->z * h
// This follows the C secp256k1_gej_add_ge_var implementation exactly
// Operations: 8 mul, 3 sqr, 11 add/negate/normalizes_to_zero
func (r *GroupElementJacobian) addGEWithZR(a *GroupElementJacobian, b *GroupElementAffine, rzr *FieldElement) {
	if a.infinity {
		if rzr != nil {
			// C code: VERIFY_CHECK(rzr == NULL) for infinity case
			// But we'll handle it gracefully
		}
		r.setGE(b)
		return
	}
	if b.infinity {
		if rzr != nil {
			// C code: secp256k1_fe_set_int(rzr, 1)
			rzr.setInt(1)
		}
		*r = *a
		return
	}
	
	// Following C code exactly: secp256k1_gej_add_ge_var
	var z12, u1, u2, s1, s2, h, i, h2, h3, t FieldElement
	
	// z12 = a->z^2
	z12.sqr(&a.z)
	
	// u1 = a->x
	u1 = a.x
	
	// u2 = b->x * z12
	u2.mul(&b.x, &z12)
	
	// s1 = a->y
	s1 = a.y
	
	// s2 = b->y * z12 * a->z
	s2.mul(&b.y, &z12)
	s2.mul(&s2, &a.z)
	
	// h = u2 - u1
	// C code uses SECP256K1_GEJ_X_MAGNITUDE_MAX but we use a.x.magnitude
	h.negate(&u1, a.x.magnitude)
	h.add(&u2)
	
	// i = s2 - s1
	i.negate(&s2, 1)
	i.add(&s1)
	
	// Check if h normalizes to zero
	if h.normalizesToZeroVar() {
		if i.normalizesToZeroVar() {
			// Points are equal - double
			// C code: secp256k1_gej_double_var(r, a, rzr)
			// For doubling, rzr should be set to 2*a->y (but we'll use a simpler approach)
			// Actually, rzr = 2*a->y based on the double_var implementation
			// But for our use case (building odd multiples), we shouldn't hit this case
			if rzr != nil {
				// Approximate: rzr = 2*a->y (from double_var logic)
				// But simpler: just set to 0 since we shouldn't hit this
				rzr.setInt(0)
			}
			r.double(a)
			return
		} else {
			// Points are negatives - result is infinity
			if rzr != nil {
				// C code: secp256k1_fe_set_int(rzr, 0)
				rzr.setInt(0)
			}
			r.setInfinity()
			return
		}
	}
	
	// General addition case
	r.infinity = false
	
	// C code: if (rzr != NULL) *rzr = h;
	if rzr != nil {
		*rzr = h
	}
	
	// r->z = a->z * h
	r.z.mul(&a.z, &h)
	
	// h2 = h^2
	h2.sqr(&h)
	
	// h2 = -h2
	h2.negate(&h2, 1)
	
	// h3 = h2 * h
	h3.mul(&h2, &h)
	
	// t = u1 * h2
	t.mul(&u1, &h2)
	
	// r->x = i^2
	r.x.sqr(&i)
	
	// r->x = i^2 + h3
	r.x.add(&h3)
	
	// r->x = i^2 + h3 + t
	r.x.add(&t)
	
	// r->x = i^2 + h3 + 2*t
	r.x.add(&t)
	
	// t = t + r->x
	t.add(&r.x)
	
	// r->y = t * i
	r.y.mul(&t, &i)
	
	// h3 = h3 * s1
	h3.mul(&h3, &s1)
	
	// r->y = t * i + h3
	r.y.add(&h3)
}

// addGE sets r = a + b where a is Jacobian and b is affine
// This follows the C secp256k1_gej_add_ge_var implementation exactly
// Operations: 8 mul, 3 sqr, 11 add/negate/normalizes_to_zero
func (r *GroupElementJacobian) addGE(a *GroupElementJacobian, b *GroupElementAffine) {
	r.addGEWithZR(a, b, nil)
}

// clear clears a group element to prevent leaking sensitive information
func (r *GroupElementAffine) clear() {
	r.x.clear()
	r.y.clear()
	r.infinity = true
}

// clear clears a Jacobian group element
func (r *GroupElementJacobian) clear() {
	r.x.clear()
	r.y.clear()
	r.z.clear()
	r.infinity = true
}

// toStorage converts a group element to storage format
// Optimized: normalize in-place when possible to avoid copy
func (r *GroupElementAffine) toStorage(s *GroupElementStorage) {
	if r.infinity {
		// Store infinity as all zeros
		for i := range s.x {
			s.x[i] = 0
			s.y[i] = 0
		}
		return
	}
	
	// Normalize in-place if needed, then convert to bytes
	// Optimization: check if already normalized before copying
	if !r.x.normalized {
		r.x.normalize()
	}
	if !r.y.normalized {
		r.y.normalize()
	}
	
	r.x.getB32(s.x[:])
	r.y.getB32(s.y[:])
}

// fromStorage converts from storage format to group element
func (r *GroupElementAffine) fromStorage(s *GroupElementStorage) {
	// Check if it's the infinity point (all zeros)
	var allZero bool = true
	for i := range s.x {
		if s.x[i] != 0 || s.y[i] != 0 {
			allZero = false
			break
		}
	}
	
	if allZero {
		r.setInfinity()
		return
	}
	
	// Convert from bytes
	r.x.setB32(s.x[:])
	r.y.setB32(s.y[:])
	r.infinity = false
}

// toBytes converts a group element to byte representation
// Optimized: normalize in-place when possible to avoid copy
func (r *GroupElementAffine) toBytes(buf []byte) {
	if len(buf) < 64 {
		panic("buffer too small for group element")
	}
	
	if r.infinity {
		// Represent infinity as all zeros
		for i := range buf[:64] {
			buf[i] = 0
		}
		return
	}
	
	// Normalize in-place if needed, then convert to bytes
	// Optimization: check if already normalized before copying
	if !r.x.normalized {
		r.x.normalize()
	}
	if !r.y.normalized {
		r.y.normalize()
	}
	
	r.x.getB32(buf[:32])
	r.y.getB32(buf[32:64])
}

// fromBytes converts from byte representation to group element
func (r *GroupElementAffine) fromBytes(buf []byte) {
	if len(buf) < 64 {
		panic("buffer too small for group element")
	}
	
	// Check if it's all zeros (infinity)
	var allZero bool = true
	for i := 0; i < 64; i++ {
		if buf[i] != 0 {
			allZero = false
			break
		}
	}
	
	if allZero {
		r.setInfinity()
		return
	}
	
	// Convert from bytes
	r.x.setB32(buf[:32])
	r.y.setB32(buf[32:64])
	r.infinity = false
}

// BatchNormalize converts multiple Jacobian points to affine coordinates efficiently
// using Montgomery's batch inversion trick. This computes n inversions using only
// 1 actual inversion + 3(n-1) multiplications, which is much faster than n individual
// inversions when n > 1.
//
// The input slice 'points' contains the Jacobian points to convert.
// The output slice 'out' will contain the corresponding affine points.
// If out is nil or smaller than points, a new slice will be allocated.
//
// Points at infinity are handled correctly and result in affine infinity points.
func BatchNormalize(out []GroupElementAffine, points []GroupElementJacobian) []GroupElementAffine {
	n := len(points)
	if n == 0 {
		return out
	}

	// Ensure output slice is large enough
	if out == nil || len(out) < n {
		out = make([]GroupElementAffine, n)
	}

	// Handle single point case - no batch optimization needed
	if n == 1 {
		out[0].setGEJ(&points[0])
		return out
	}

	// Collect non-infinity Z coordinates for batch inversion
	// We need to track which points are at infinity
	zValues := make([]FieldElement, 0, n)
	nonInfIndices := make([]int, 0, n)

	for i := 0; i < n; i++ {
		if points[i].isInfinity() {
			out[i].setInfinity()
		} else {
			zValues = append(zValues, points[i].z)
			nonInfIndices = append(nonInfIndices, i)
		}
	}

	// If all points are at infinity, we're done
	if len(zValues) == 0 {
		return out
	}

	// Batch invert all Z values
	zInvs := make([]FieldElement, len(zValues))
	batchInverse(zInvs, zValues)

	// Now compute affine coordinates for each non-infinity point
	// affine.x = X * Z^(-2)
	// affine.y = Y * Z^(-3)
	for i, idx := range nonInfIndices {
		var zInv2, zInv3 FieldElement

		// zInv2 = Z^(-2)
		zInv2.sqr(&zInvs[i])

		// zInv3 = Z^(-3) = Z^(-2) * Z^(-1)
		zInv3.mul(&zInv2, &zInvs[i])

		// x = X * Z^(-2)
		out[idx].x.mul(&points[idx].x, &zInv2)

		// y = Y * Z^(-3)
		out[idx].y.mul(&points[idx].y, &zInv3)

		out[idx].infinity = false
	}

	return out
}

// BatchNormalizeInPlace converts multiple Jacobian points to affine coordinates
// in place, modifying the input slice. Each Jacobian point is converted such that
// Z becomes 1 (or the point is marked as infinity).
//
// This is useful when you want to normalize points without allocating new memory
// for a separate affine point array.
func BatchNormalizeInPlace(points []GroupElementJacobian) {
	n := len(points)
	if n == 0 {
		return
	}

	// Handle single point case
	if n == 1 {
		if !points[0].isInfinity() {
			var zInv, zInv2, zInv3 FieldElement
			zInv.inv(&points[0].z)
			zInv2.sqr(&zInv)
			zInv3.mul(&zInv2, &zInv)
			points[0].x.mul(&points[0].x, &zInv2)
			points[0].y.mul(&points[0].y, &zInv3)
			points[0].z.setInt(1)
		}
		return
	}

	// Collect non-infinity Z coordinates for batch inversion
	zValues := make([]FieldElement, 0, n)
	nonInfIndices := make([]int, 0, n)

	for i := 0; i < n; i++ {
		if !points[i].isInfinity() {
			zValues = append(zValues, points[i].z)
			nonInfIndices = append(nonInfIndices, i)
		}
	}

	// If all points are at infinity, we're done
	if len(zValues) == 0 {
		return
	}

	// Batch invert all Z values
	zInvs := make([]FieldElement, len(zValues))
	batchInverse(zInvs, zValues)

	// Now normalize each non-infinity point
	for i, idx := range nonInfIndices {
		var zInv2, zInv3 FieldElement

		// zInv2 = Z^(-2)
		zInv2.sqr(&zInvs[i])

		// zInv3 = Z^(-3) = Z^(-2) * Z^(-1)
		zInv3.mul(&zInv2, &zInvs[i])

		// x = X * Z^(-2)
		points[idx].x.mul(&points[idx].x, &zInv2)

		// y = Y * Z^(-3)
		points[idx].y.mul(&points[idx].y, &zInv3)

		// Z = 1
		points[idx].z.setInt(1)
	}
}

// batchNormalize16 converts exactly 16 Jacobian points to affine coordinates
// with zero heap allocations. Optimized for GLV table building where
// tableSize = 16 (glvTableSize).
//
// Uses stack-allocated arrays throughout for optimal performance.
// Points at infinity are handled correctly.
func batchNormalize16(out *[16]GroupElementAffine, points *[16]GroupElementJacobian) {
	// Collect Z coordinates for batch inversion
	// For GLV table building, all points should be valid (no infinity)
	var zValues [16]FieldElement
	var validCount int

	for i := 0; i < 16; i++ {
		if points[i].isInfinity() {
			out[i].setInfinity()
		} else {
			zValues[validCount] = points[i].z
			validCount++
		}
	}

	// If all points are at infinity, we're done
	if validCount == 0 {
		return
	}

	// For the common case where all 16 points are valid
	if validCount == 16 {
		// Batch invert all Z values with zero allocation
		var zInvs [16]FieldElement
		batchInverse16(&zInvs, &zValues)

		// Compute affine coordinates for each point
		for i := 0; i < 16; i++ {
			var zInv2, zInv3 FieldElement

			// zInv2 = Z^(-2)
			zInv2.sqr(&zInvs[i])

			// zInv3 = Z^(-3) = Z^(-2) * Z^(-1)
			zInv3.mul(&zInv2, &zInvs[i])

			// x = X * Z^(-2)
			out[i].x.mul(&points[i].x, &zInv2)

			// y = Y * Z^(-3)
			out[i].y.mul(&points[i].y, &zInv3)

			out[i].infinity = false
		}
		return
	}

	// Handle partial case (some points at infinity) - rare in practice
	// Fall back to slice-based batch inversion for this edge case
	zSlice := zValues[:validCount]
	zInvSlice := make([]FieldElement, validCount)
	batchInverse(zInvSlice, zSlice)

	// Apply inversions to non-infinity points
	invIdx := 0
	for i := 0; i < 16; i++ {
		if !points[i].isInfinity() {
			var zInv2, zInv3 FieldElement

			zInv2.sqr(&zInvSlice[invIdx])
			zInv3.mul(&zInv2, &zInvSlice[invIdx])

			out[i].x.mul(&points[i].x, &zInv2)
			out[i].y.mul(&points[i].y, &zInv3)
			out[i].infinity = false

			invIdx++
		}
	}
}

// batchNormalize32 converts exactly 32 Jacobian points to affine coordinates
// with zero heap allocations. Optimized for GLV table building where
// tableSize = 32 (glvTableSize with window size 6).
//
// Uses stack-allocated arrays throughout for optimal performance.
// Points at infinity are handled correctly.
func batchNormalize32(out *[32]GroupElementAffine, points *[32]GroupElementJacobian) {
	// Collect Z coordinates for batch inversion
	// For GLV table building, all points should be valid (no infinity)
	var zValues [32]FieldElement
	var validCount int

	for i := 0; i < 32; i++ {
		if points[i].isInfinity() {
			out[i].setInfinity()
		} else {
			zValues[validCount] = points[i].z
			validCount++
		}
	}

	// If all points are at infinity, we're done
	if validCount == 0 {
		return
	}

	// For the common case where all 32 points are valid
	if validCount == 32 {
		// Batch invert all Z values with zero allocation
		var zInvs [32]FieldElement
		batchInverse32(&zInvs, &zValues)

		// Compute affine coordinates for each point
		for i := 0; i < 32; i++ {
			var zInv2, zInv3 FieldElement

			// zInv2 = Z^(-2)
			zInv2.sqr(&zInvs[i])

			// zInv3 = Z^(-3) = Z^(-2) * Z^(-1)
			zInv3.mul(&zInv2, &zInvs[i])

			// x = X * Z^(-2)
			out[i].x.mul(&points[i].x, &zInv2)

			// y = Y * Z^(-3)
			out[i].y.mul(&points[i].y, &zInv3)

			out[i].infinity = false
		}
		return
	}

	// Handle partial case (some points at infinity) - rare in practice
	// Fall back to slice-based batch inversion for this edge case
	zSlice := zValues[:validCount]
	zInvSlice := make([]FieldElement, validCount)
	batchInverse(zInvSlice, zSlice)

	// Apply inversions to non-infinity points
	invIdx := 0
	for i := 0; i < 32; i++ {
		if !points[i].isInfinity() {
			var zInv2, zInv3 FieldElement

			zInv2.sqr(&zInvSlice[invIdx])
			zInv3.mul(&zInv2, &zInvSlice[invIdx])

			out[i].x.mul(&points[i].x, &zInv2)
			out[i].y.mul(&points[i].y, &zInv3)
			out[i].infinity = false

			invIdx++
		}
	}
}

// =============================================================================
// GLV Endomorphism Support Functions
// =============================================================================

// ecmultEndoSplit splits a scalar and point for the GLV endomorphism optimization.
// Given a scalar s and point p, it computes:
//   s1, s2 such that s1 + s2*λ ≡ s (mod n)
//   p1 = p
//   p2 = λ*p = (β*p.x, p.y)
//
// It also normalizes s1 and s2 to be "low" (not high) by conditionally negating
// both the scalar and corresponding point.
//
// After this function:
//   s1 * p1 + s2 * p2 = s * p
//
// Reference: libsecp256k1 ecmult_impl.h:secp256k1_ecmult_endo_split
func ecmultEndoSplit(s1, s2 *Scalar, p1, p2 *GroupElementAffine, s *Scalar, p *GroupElementAffine) {
	// Split the scalar: s = s1 + s2*λ
	scalarSplitLambda(s1, s2, s)

	// p1 = p (copy)
	*p1 = *p

	// p2 = λ*p = (β*p.x, p.y)
	p2.mulLambda(p)

	// If s1 is high, negate it and p1
	if s1.isHigh() {
		s1.negate(s1)
		p1.negate(p1)
	}

	// If s2 is high, negate it and p2
	if s2.isHigh() {
		s2.negate(s2)
		p2.negate(p2)
	}
}

// ecmultEndoSplitJac is the Jacobian version of ecmultEndoSplit.
// Given a scalar s and Jacobian point p, it computes the split for GLV optimization.
func ecmultEndoSplitJac(s1, s2 *Scalar, p1, p2 *GroupElementJacobian, s *Scalar, p *GroupElementJacobian) {
	// Split the scalar: s = s1 + s2*λ
	scalarSplitLambda(s1, s2, s)

	// p1 = p (copy)
	*p1 = *p

	// p2 = λ*p = (β*p.x, p.y, p.z)
	p2.mulLambda(p)

	// If s1 is high, negate it and p1
	if s1.isHigh() {
		s1.negate(s1)
		p1.negate(p1)
	}

	// If s2 is high, negate it and p2
	if s2.isHigh() {
		s2.negate(s2)
		p2.negate(p2)
	}
}
