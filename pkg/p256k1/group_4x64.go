//go:build amd64 && !purego

package p256k1

// Group operations using Field4x64 for maximum performance on AMD64 with BMI2.
// These types mirror GroupElementAffine and GroupElementJacobian but use the
// faster 4x64 field representation.

// GroupElement4x64Affine represents a point on the secp256k1 curve in affine coordinates.
type GroupElement4x64Affine struct {
	x, y     Field4x64
	infinity bool
}

// GroupElement4x64Jacobian represents a point on the secp256k1 curve in Jacobian coordinates.
// Affine coordinates are (x/z^2, y/z^3).
type GroupElement4x64Jacobian struct {
	x, y, z  Field4x64
	infinity bool
}

// setInfinity sets the point to infinity.
func (r *GroupElement4x64Affine) setInfinity() {
	r.x.SetZero()
	r.y.SetZero()
	r.infinity = true
}

// setInfinity sets the Jacobian point to infinity.
func (r *GroupElement4x64Jacobian) setInfinity() {
	r.x.SetZero()
	r.y.SetOne()
	r.z.SetZero()
	r.infinity = true
}

// isInfinity returns true if the point is at infinity.
func (r *GroupElement4x64Affine) isInfinity() bool {
	return r.infinity
}

// isInfinity returns true if the Jacobian point is at infinity.
func (r *GroupElement4x64Jacobian) isInfinity() bool {
	return r.infinity
}

// setGE sets a Jacobian element from an affine element.
func (r *GroupElement4x64Jacobian) setGE(a *GroupElement4x64Affine) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.x = a.x
	r.y = a.y
	r.z.SetOne()
	r.infinity = false
}

// negate sets r to the negation of a (mirror around X axis).
func (r *GroupElement4x64Affine) negate(a *GroupElement4x64Affine) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.x = a.x
	r.y.Negate(&a.y)
	r.infinity = false
}

// negate sets r to the negation of a (mirror around X axis).
func (r *GroupElement4x64Jacobian) negate(a *GroupElement4x64Jacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.x = a.x
	r.y.Negate(&a.y)
	r.z = a.z
	r.infinity = false
}

// double sets r = 2*a (point doubling in Jacobian coordinates).
// This follows the secp256k1 algorithm exactly, optimized for Field4x64.
// Operations: 3 mul, 4 sqr, plus add/negate/half
func (r *GroupElement4x64Jacobian) double(a *GroupElement4x64Jacobian) {
	var l, s, t Field4x64

	r.infinity = a.infinity

	// Z3 = Y1*Z1
	r.z.Mul(&a.z, &a.y)

	// S = Y1^2
	s.Sqr(&a.y)

	// L = X1^2
	l.Sqr(&a.x)

	// L = 3*X1^2
	l.mulInt(3)

	// L = 3/2*X1^2
	l.half(&l)

	// T = -S = -Y1^2
	t.Negate(&s)

	// T = -X1*S = -X1*Y1^2
	t.Mul(&t, &a.x)

	// X3 = L^2
	r.x.Sqr(&l)

	// X3 = L^2 + T
	r.x.Add(&r.x, &t)

	// X3 = L^2 + 2*T
	r.x.Add(&r.x, &t)

	// S = S^2 = Y1^4
	s.Sqr(&s)

	// T = X3 + T
	t.Add(&t, &r.x)

	// Y3 = L*(X3 + T)
	r.y.Mul(&t, &l)

	// Y3 = L*(X3 + T) + S^2
	r.y.Add(&r.y, &s)

	// Y3 = -(L*(X3 + T) + S^2)
	r.y.Negate(&r.y)
}

// addGE sets r = a + b where a is Jacobian and b is affine.
// This follows the secp256k1_gej_add_ge_var algorithm.
// Operations: 8 mul, 3 sqr
func (r *GroupElement4x64Jacobian) addGE(a *GroupElement4x64Jacobian, b *GroupElement4x64Affine) {
	if a.infinity {
		r.setGE(b)
		return
	}
	if b.infinity {
		*r = *a
		return
	}

	var z12, u1, u2, s1, s2, h, i, h2, h3, t Field4x64

	// z12 = a->z^2
	z12.Sqr(&a.z)

	// u1 = a->x
	u1 = a.x

	// u2 = b->x * z12
	u2.Mul(&b.x, &z12)

	// s1 = a->y
	s1 = a.y

	// s2 = b->y * z12 * a->z
	s2.Mul(&b.y, &z12)
	s2.Mul(&s2, &a.z)

	// h = u2 - u1
	h.Negate(&u1)
	h.Add(&h, &u2)

	// i = s2 - s1
	i.Negate(&s2)
	i.Add(&i, &s1)

	// Check if h normalizes to zero
	h.Reduce()
	if h.IsZero() {
		i.Reduce()
		if i.IsZero() {
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

	// r->z = a->z * h
	r.z.Mul(&a.z, &h)

	// h2 = h^2
	h2.Sqr(&h)

	// h2 = -h2
	h2.Negate(&h2)

	// h3 = h2 * h
	h3.Mul(&h2, &h)

	// t = u1 * h2
	t.Mul(&u1, &h2)

	// r->x = i^2
	r.x.Sqr(&i)

	// r->x = i^2 + h3
	r.x.Add(&r.x, &h3)

	// r->x = i^2 + h3 + t
	r.x.Add(&r.x, &t)

	// r->x = i^2 + h3 + 2*t
	r.x.Add(&r.x, &t)

	// t = t + r->x
	t.Add(&t, &r.x)

	// r->y = t * i
	r.y.Mul(&t, &i)

	// h3 = h3 * s1
	h3.Mul(&h3, &s1)

	// r->y = t * i + h3
	r.y.Add(&r.y, &h3)
}

// addVar sets r = a + b (variable-time point addition in Jacobian coordinates).
// Operations: 12 mul, 4 sqr
func (r *GroupElement4x64Jacobian) addVar(a, b *GroupElement4x64Jacobian) {
	if a.infinity {
		*r = *b
		return
	}
	if b.infinity {
		*r = *a
		return
	}

	var z22, z12, u1, u2, s1, s2, h, i, h2, h3, t Field4x64

	// z22 = b->z^2
	z22.Sqr(&b.z)

	// z12 = a->z^2
	z12.Sqr(&a.z)

	// u1 = a->x * z22
	u1.Mul(&a.x, &z22)

	// u2 = b->x * z12
	u2.Mul(&b.x, &z12)

	// s1 = a->y * z22 * b->z
	s1.Mul(&a.y, &z22)
	s1.Mul(&s1, &b.z)

	// s2 = b->y * z12 * a->z
	s2.Mul(&b.y, &z12)
	s2.Mul(&s2, &a.z)

	// h = u2 - u1
	h.Negate(&u1)
	h.Add(&h, &u2)

	// i = s2 - s1
	i.Negate(&s2)
	i.Add(&i, &s1)

	// Check if h normalizes to zero
	h.Reduce()
	if h.IsZero() {
		i.Reduce()
		if i.IsZero() {
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
	t.Mul(&h, &b.z)

	// r->z = a->z * t
	r.z.Mul(&a.z, &t)

	// h2 = h^2
	h2.Sqr(&h)

	// h2 = -h2
	h2.Negate(&h2)

	// h3 = h2 * h
	h3.Mul(&h2, &h)

	// t = u1 * h2
	t.Mul(&u1, &h2)

	// r->x = i^2
	r.x.Sqr(&i)

	// r->x = i^2 + h3
	r.x.Add(&r.x, &h3)

	// r->x = i^2 + h3 + t
	r.x.Add(&r.x, &t)

	// r->x = i^2 + h3 + 2*t
	r.x.Add(&r.x, &t)

	// t = t + r->x
	t.Add(&t, &r.x)

	// r->y = t * i
	r.y.Mul(&t, &i)

	// h3 = h3 * s1
	h3.Mul(&h3, &s1)

	// r->y = t * i + h3
	r.y.Add(&r.y, &h3)
}

// setGEJ converts a Jacobian point to affine.
func (r *GroupElement4x64Affine) setGEJ(a *GroupElement4x64Jacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}

	r.infinity = false

	// Compute z^(-1)
	var zInv Field4x64
	zInv.inv(&a.z)

	// z2 = z^(-2)
	var z2 Field4x64
	z2.Sqr(&zInv)

	// z3 = z^(-3)
	var z3 Field4x64
	z3.Mul(&zInv, &z2)

	// x = X * z^(-2)
	r.x.Mul(&a.x, &z2)

	// y = Y * z^(-3)
	r.y.Mul(&a.y, &z3)
}

// batchNormalize4x64 batch converts Jacobian points to affine using Montgomery's trick.
// This is much more efficient than individual conversions (1 inversion vs N inversions).
func batchNormalize4x64(dst []GroupElement4x64Affine, src []GroupElement4x64Jacobian) {
	n := len(src)
	if n == 0 {
		return
	}
	if len(dst) < n {
		panic("dst too small")
	}

	// Use Montgomery's trick: compute product of all z values, invert once,
	// then extract individual inverses

	// Allocate temporary storage for running products
	var products [glvTableSize]Field4x64

	// Compute running products: products[i] = z[0] * z[1] * ... * z[i]
	// Skip infinity points
	validCount := 0
	for i := 0; i < n; i++ {
		if src[i].infinity {
			dst[i].setInfinity()
			continue
		}

		if validCount == 0 {
			products[validCount] = src[i].z
		} else {
			products[validCount].Mul(&products[validCount-1], &src[i].z)
		}
		validCount++
	}

	if validCount == 0 {
		return
	}

	// Invert the final product (only 1 inversion!)
	var invProduct Field4x64
	invProduct.inv(&products[validCount-1])

	// Extract individual inverses using Montgomery's trick
	// z_inv[i] = invProduct * products[i-1]
	validIdx := validCount - 1
	for i := n - 1; i >= 0; i-- {
		if src[i].infinity {
			continue
		}

		var zInv Field4x64
		if validIdx == 0 {
			zInv = invProduct
		} else {
			zInv.Mul(&invProduct, &products[validIdx-1])
			// Update invProduct for next iteration
			invProduct.Mul(&invProduct, &src[i].z)
		}
		validIdx--

		// Compute affine coordinates
		dst[i].infinity = false

		// z2 = z^(-2)
		var z2 Field4x64
		z2.Sqr(&zInv)

		// z3 = z^(-3)
		var z3 Field4x64
		z3.Mul(&zInv, &z2)

		// x = X * z^(-2)
		dst[i].x.Mul(&src[i].x, &z2)

		// y = Y * z^(-3)
		dst[i].y.Mul(&src[i].y, &z3)
	}
}

// FromFieldElement converts a FieldElement to Field4x64.
func (f *Field4x64) FromFieldElement(a *FieldElement) {
	// Normalize input first
	var aNorm FieldElement
	aNorm = *a
	aNorm.normalizeWeak()

	// Pack 5x52 limbs into 4x64
	f.n[0] = aNorm.n[0] | (aNorm.n[1] << 52)
	f.n[1] = (aNorm.n[1] >> 12) | (aNorm.n[2] << 40)
	f.n[2] = (aNorm.n[2] >> 24) | (aNorm.n[3] << 28)
	f.n[3] = (aNorm.n[3] >> 36) | (aNorm.n[4] << 16)
	f.magnitude = 1
	f.normalized = false
}

// ToFieldElement converts Field4x64 to FieldElement.
func (f *Field4x64) ToFieldElement(r *FieldElement) {
	// Ensure normalized
	if f.magnitude > 1 {
		f.Reduce()
	}

	// Unpack 4x64 to 5x52
	r.n[0] = f.n[0] & 0xFFFFFFFFFFFFF
	r.n[1] = ((f.n[0] >> 52) | (f.n[1] << 12)) & 0xFFFFFFFFFFFFF
	r.n[2] = ((f.n[1] >> 40) | (f.n[2] << 24)) & 0xFFFFFFFFFFFFF
	r.n[3] = ((f.n[2] >> 28) | (f.n[3] << 36)) & 0xFFFFFFFFFFFFF
	r.n[4] = (f.n[3] >> 16) & 0x0FFFFFFFFFFFF

	r.magnitude = 1
	r.normalized = false
}

// FromGroupElementJacobian converts from the standard type.
func (r *GroupElement4x64Jacobian) FromGroupElementJacobian(a *GroupElementJacobian) {
	if a.infinity {
		r.setInfinity()
		return
	}
	r.x.FromFieldElement(&a.x)
	r.y.FromFieldElement(&a.y)
	r.z.FromFieldElement(&a.z)
	r.infinity = false
}

// ToGroupElementJacobian converts to the standard type.
func (r *GroupElement4x64Jacobian) ToGroupElementJacobian(a *GroupElementJacobian) {
	if r.infinity {
		a.setInfinity()
		return
	}
	r.x.ToFieldElement(&a.x)
	r.y.ToFieldElement(&a.y)
	r.z.ToFieldElement(&a.z)
	a.infinity = false
}

// FromGroupElementAffine converts from the standard type.
func (r *GroupElement4x64Affine) FromGroupElementAffine(a *GroupElementAffine) {
	if a.infinity {
		r.setInfinity()
		return
	}
	r.x.FromFieldElement(&a.x)
	r.y.FromFieldElement(&a.y)
	r.infinity = false
}

// ToGroupElementAffine converts to the standard type.
func (r *GroupElement4x64Affine) ToGroupElementAffine(a *GroupElementAffine) {
	if r.infinity {
		a.setInfinity()
		return
	}
	r.x.ToFieldElement(&a.x)
	r.y.ToFieldElement(&a.y)
	a.infinity = false
}
