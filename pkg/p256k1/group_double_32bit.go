//go:build js || wasm || tinygo || wasm32

package p256k1

// double sets r = 2*a (point doubling in Jacobian coordinates)
// Uses the half-free Decred-style formula because half() has a carry bug
// in the 10×26-bit limb representation used on 32-bit/WASM.
//   A = X1^2, B = Y1^2, C = B^2, D = 2*((X1+B)^2-A-C)
//   E = 3*A, F = E^2, X3 = F-2*D, Y3 = E*(D-X3)-8*C, Z3 = 2*Y1*Z1
func (r *GroupElementJacobian) double(a *GroupElementJacobian) {
	r.infinity = a.infinity

	var aa, b, c, d, e, f FieldElement

	// Z3 = 2*Y1*Z1
	r.z.mul(&a.y, &a.z)
	r.z.mulInt(2)

	// A = X1^2
	aa.sqr(&a.x)

	// B = Y1^2
	b.sqr(&a.y)

	// C = B^2 = Y1^4
	c.sqr(&b)

	// D = 2*((X1+B)^2 - A - C) = 4*X1*Y1^2
	b.add(&a.x)
	b.sqr(&b)
	d = aa
	d.add(&c)
	d.negate(&d, 2)
	d.add(&b)
	d.mulInt(2)

	// E = 3*A = 3*X1^2
	e = aa
	e.mulInt(3)

	// F = E^2
	f.sqr(&e)

	// X3 = F - 2*D
	r.x = d
	r.x.mulInt(2)
	r.x.negate(&r.x, 16)
	r.x.add(&f)
	r.x.normalize()

	// Y3 = E*(D-X3) - 8*C
	f.negate(&r.x, 1)
	f.add(&d)
	f.normalize()
	f.mul(&f, &e)

	c.mulInt(8)
	c.negate(&c, 8)
	r.y = f
	r.y.add(&c)

	r.x.normalize()
	r.y.normalize()
	r.z.normalize()
}
