//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

// double sets r = 2*a (point doubling in Jacobian coordinates)
// Uses the bitcoin-core half()-based formula (correct on 64-bit field elements).
func (r *GroupElementJacobian) double(a *GroupElementJacobian) {
	var l, s, t FieldElement

	r.infinity = a.infinity

	// Z3 = Y1*Z1 (1)
	r.z.mul(&a.z, &a.y)

	// S = Y1^2 (1)
	s.sqr(&a.y)

	// L = X1^2 (1)
	l.sqr(&a.x)

	// L = 3*X1^2 (3)
	l.mulInt(3)

	// L = 3/2*X1^2 (2)
	l.half(&l)

	// T = -S (2) where S = Y1^2
	t.negate(&s, 1)

	// T = -X1*S = -X1*Y1^2 (1)
	t.mul(&t, &a.x)

	// X3 = L^2 (1)
	r.x.sqr(&l)

	// X3 = L^2 + T (2)
	r.x.add(&t)

	// X3 = L^2 + 2*T (3)
	r.x.add(&t)

	// S = S^2 = (Y1^2)^2 = Y1^4 (1)
	s.sqr(&s)

	// T = X3 + T = X3 + (-X1*Y1^2) (4)
	t.add(&r.x)

	// Y3 = L*(X3 + T) = L*(X3 + (-X1*Y1^2)) (1)
	r.y.mul(&t, &l)

	// Y3 = L*(X3 + T) + S^2 = L*(X3 + (-X1*Y1^2)) + Y1^4 (2)
	r.y.add(&s)

	// Y3 = -(L*(X3 + T) + S^2) (3)
	r.y.negate(&r.y, 2)
}
