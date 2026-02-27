package avx

// Point operations on the secp256k1 curve.
// Affine: (x, y) where y² = x³ + 7
// Jacobian: (X, Y, Z) where affine = (X/Z², Y/Z³)

// SetInfinity sets the point to the point at infinity.
func (p *AffinePoint) SetInfinity() *AffinePoint {
	p.X = FieldZero
	p.Y = FieldZero
	p.Infinity = true
	return p
}

// IsInfinity returns true if the point is the point at infinity.
func (p *AffinePoint) IsInfinity() bool {
	return p.Infinity
}

// Set sets p to the value of q.
func (p *AffinePoint) Set(q *AffinePoint) *AffinePoint {
	p.X = q.X
	p.Y = q.Y
	p.Infinity = q.Infinity
	return p
}

// Equal returns true if two points are equal.
func (p *AffinePoint) Equal(q *AffinePoint) bool {
	if p.Infinity && q.Infinity {
		return true
	}
	if p.Infinity || q.Infinity {
		return false
	}
	return p.X.Equal(&q.X) && p.Y.Equal(&q.Y)
}

// Negate sets p = -q (reflection over x-axis).
func (p *AffinePoint) Negate(q *AffinePoint) *AffinePoint {
	if q.Infinity {
		p.SetInfinity()
		return p
	}
	p.X = q.X
	p.Y.Negate(&q.Y)
	p.Infinity = false
	return p
}

// IsOnCurve returns true if the point is on the secp256k1 curve.
func (p *AffinePoint) IsOnCurve() bool {
	if p.Infinity {
		return true
	}

	// Check y² = x³ + 7
	var y2, x2, x3, rhs FieldElement

	y2.Sqr(&p.Y)

	x2.Sqr(&p.X)
	x3.Mul(&x2, &p.X)

	// rhs = x³ + 7
	var seven FieldElement
	seven.N[0].Lo = 7
	rhs.Add(&x3, &seven)

	return y2.Equal(&rhs)
}

// SetXY sets the point to (x, y).
func (p *AffinePoint) SetXY(x, y *FieldElement) *AffinePoint {
	p.X = *x
	p.Y = *y
	p.Infinity = false
	return p
}

// SetCompressed sets the point from compressed form (x coordinate + sign bit).
// Returns true if successful.
func (p *AffinePoint) SetCompressed(x *FieldElement, odd bool) bool {
	// Compute y² = x³ + 7
	var y2, x2, x3 FieldElement

	x2.Sqr(x)
	x3.Mul(&x2, x)

	// y² = x³ + 7
	var seven FieldElement
	seven.N[0].Lo = 7
	y2.Add(&x3, &seven)

	// Compute y = sqrt(y²)
	var y FieldElement
	if !y.Sqrt(&y2) {
		return false // No square root exists
	}

	// Choose the correct sign
	if y.IsOdd() != odd {
		y.Negate(&y)
	}

	p.X = *x
	p.Y = y
	p.Infinity = false
	return true
}

// Jacobian point operations

// SetInfinity sets the Jacobian point to the point at infinity.
func (p *JacobianPoint) SetInfinity() *JacobianPoint {
	p.X = FieldOne
	p.Y = FieldOne
	p.Z = FieldZero
	p.Infinity = true
	return p
}

// IsInfinity returns true if the point is the point at infinity.
func (p *JacobianPoint) IsInfinity() bool {
	return p.Infinity || p.Z.IsZero()
}

// Set sets p to the value of q.
func (p *JacobianPoint) Set(q *JacobianPoint) *JacobianPoint {
	p.X = q.X
	p.Y = q.Y
	p.Z = q.Z
	p.Infinity = q.Infinity
	return p
}

// FromAffine converts an affine point to Jacobian coordinates.
func (p *JacobianPoint) FromAffine(q *AffinePoint) *JacobianPoint {
	if q.Infinity {
		p.SetInfinity()
		return p
	}
	p.X = q.X
	p.Y = q.Y
	p.Z = FieldOne
	p.Infinity = false
	return p
}

// ToAffine converts a Jacobian point to affine coordinates.
func (p *JacobianPoint) ToAffine(q *AffinePoint) *AffinePoint {
	if p.IsInfinity() {
		q.SetInfinity()
		return q
	}

	// affine = (X/Z², Y/Z³)
	var zInv, zInv2, zInv3 FieldElement

	zInv.Inverse(&p.Z)
	zInv2.Sqr(&zInv)
	zInv3.Mul(&zInv2, &zInv)

	q.X.Mul(&p.X, &zInv2)
	q.Y.Mul(&p.Y, &zInv3)
	q.Infinity = false

	return q
}

// Double sets p = 2*q using Jacobian coordinates.
// Standard Jacobian doubling for y²=x³+b (secp256k1 has a=0):
// M = 3*X₁²
// S = 4*X₁*Y₁²
// T = 8*Y₁⁴
// X₃ = M² - 2*S
// Y₃ = M*(S - X₃) - T
// Z₃ = 2*Y₁*Z₁
func (p *JacobianPoint) Double(q *JacobianPoint) *JacobianPoint {
	if q.IsInfinity() {
		p.SetInfinity()
		return p
	}

	var y2, m, x2, s, t, tmp FieldElement
	var x3, y3, z3 FieldElement // Use temporaries to avoid aliasing issues

	// Y² = Y₁²
	y2.Sqr(&q.Y)

	// M = 3*X₁² (for a=0 curves like secp256k1)
	x2.Sqr(&q.X)
	m.MulInt(&x2, 3)

	// S = 4*X₁*Y₁²
	s.Mul(&q.X, &y2)
	s.MulInt(&s, 4)

	// T = 8*Y₁⁴
	t.Sqr(&y2)
	t.MulInt(&t, 8)

	// X₃ = M² - 2*S
	x3.Sqr(&m)
	tmp.Double(&s)
	x3.Sub(&x3, &tmp)

	// Y₃ = M*(S - X₃) - T
	tmp.Sub(&s, &x3)
	y3.Mul(&m, &tmp)
	y3.Sub(&y3, &t)

	// Z₃ = 2*Y₁*Z₁
	z3.Mul(&q.Y, &q.Z)
	z3.Double(&z3)

	// Now copy to output (safe even if p == q)
	p.X = x3
	p.Y = y3
	p.Z = z3
	p.Infinity = false
	return p
}

// Add sets p = q + r using Jacobian coordinates.
// This is the complete addition formula.
func (p *JacobianPoint) Add(q, r *JacobianPoint) *JacobianPoint {
	if q.IsInfinity() {
		p.Set(r)
		return p
	}
	if r.IsInfinity() {
		p.Set(q)
		return p
	}

	// Algorithm:
	// U₁ = X₁*Z₂²
	// U₂ = X₂*Z₁²
	// S₁ = Y₁*Z₂³
	// S₂ = Y₂*Z₁³
	// H = U₂ - U₁
	// R = S₂ - S₁
	// If H = 0 and R = 0: return Double(q)
	// If H = 0 and R ≠ 0: return Infinity
	// X₃ = R² - H³ - 2*U₁*H²
	// Y₃ = R*(U₁*H² - X₃) - S₁*H³
	// Z₃ = H*Z₁*Z₂

	var u1, u2, s1, s2, h, rr, h2, h3, u1h2 FieldElement
	var z1sq, z2sq, z1cu, z2cu FieldElement
	var x3, y3, z3 FieldElement // Use temporaries to avoid aliasing issues

	z1sq.Sqr(&q.Z)
	z2sq.Sqr(&r.Z)
	z1cu.Mul(&z1sq, &q.Z)
	z2cu.Mul(&z2sq, &r.Z)

	u1.Mul(&q.X, &z2sq)
	u2.Mul(&r.X, &z1sq)
	s1.Mul(&q.Y, &z2cu)
	s2.Mul(&r.Y, &z1cu)

	h.Sub(&u2, &u1)
	rr.Sub(&s2, &s1)

	// Check for special cases
	if h.IsZero() {
		if rr.IsZero() {
			// Points are equal, use doubling
			return p.Double(q)
		}
		// Points are inverses, return infinity
		p.SetInfinity()
		return p
	}

	h2.Sqr(&h)
	h3.Mul(&h2, &h)
	u1h2.Mul(&u1, &h2)

	// X₃ = R² - H³ - 2*U₁*H²
	var r2, u1h2_2 FieldElement
	r2.Sqr(&rr)
	u1h2_2.Double(&u1h2)
	x3.Sub(&r2, &h3)
	x3.Sub(&x3, &u1h2_2)

	// Y₃ = R*(U₁*H² - X₃) - S₁*H³
	var tmp, s1h3 FieldElement
	tmp.Sub(&u1h2, &x3)
	y3.Mul(&rr, &tmp)
	s1h3.Mul(&s1, &h3)
	y3.Sub(&y3, &s1h3)

	// Z₃ = H*Z₁*Z₂
	z3.Mul(&q.Z, &r.Z)
	z3.Mul(&z3, &h)

	// Now copy to output (safe even if p == q or p == r)
	p.X = x3
	p.Y = y3
	p.Z = z3
	p.Infinity = false
	return p
}

// AddAffine sets p = q + r where q is Jacobian and r is affine.
// More efficient than converting r to Jacobian first.
func (p *JacobianPoint) AddAffine(q *JacobianPoint, r *AffinePoint) *JacobianPoint {
	if q.IsInfinity() {
		p.FromAffine(r)
		return p
	}
	if r.Infinity {
		p.Set(q)
		return p
	}

	// When Z₂ = 1 (affine point), formulas simplify:
	// U₁ = X₁
	// U₂ = X₂*Z₁²
	// S₁ = Y₁
	// S₂ = Y₂*Z₁³

	var u2, s2, h, rr, h2, h3, u1h2 FieldElement
	var z1sq, z1cu FieldElement
	var x3, y3, z3 FieldElement // Use temporaries to avoid aliasing issues

	z1sq.Sqr(&q.Z)
	z1cu.Mul(&z1sq, &q.Z)

	u2.Mul(&r.X, &z1sq)
	s2.Mul(&r.Y, &z1cu)

	h.Sub(&u2, &q.X)
	rr.Sub(&s2, &q.Y)

	if h.IsZero() {
		if rr.IsZero() {
			return p.Double(q)
		}
		p.SetInfinity()
		return p
	}

	h2.Sqr(&h)
	h3.Mul(&h2, &h)
	u1h2.Mul(&q.X, &h2)

	// X₃ = R² - H³ - 2*U₁*H²
	var r2, u1h2_2 FieldElement
	r2.Sqr(&rr)
	u1h2_2.Double(&u1h2)
	x3.Sub(&r2, &h3)
	x3.Sub(&x3, &u1h2_2)

	// Y₃ = R*(U₁*H² - X₃) - S₁*H³
	var tmp, s1h3 FieldElement
	tmp.Sub(&u1h2, &x3)
	y3.Mul(&rr, &tmp)
	s1h3.Mul(&q.Y, &h3)
	y3.Sub(&y3, &s1h3)

	// Z₃ = H*Z₁
	z3.Mul(&q.Z, &h)

	// Now copy to output (safe even if p == q)
	p.X = x3
	p.Y = y3
	p.Z = z3
	p.Infinity = false
	return p
}

// Negate sets p = -q (reflection over x-axis).
func (p *JacobianPoint) Negate(q *JacobianPoint) *JacobianPoint {
	if q.IsInfinity() {
		p.SetInfinity()
		return p
	}
	p.X = q.X
	p.Y.Negate(&q.Y)
	p.Z = q.Z
	p.Infinity = false
	return p
}

// ScalarMult computes p = k*q using double-and-add.
func (p *JacobianPoint) ScalarMult(q *JacobianPoint, k *Scalar) *JacobianPoint {
	// Simple double-and-add (not constant-time)
	// A proper implementation would use windowed NAF or similar

	p.SetInfinity()

	// Process bits from high to low
	bytes := k.Bytes()
	for i := 0; i < 32; i++ {
		b := bytes[i]
		for j := 7; j >= 0; j-- {
			p.Double(p)
			if (b>>j)&1 == 1 {
				p.Add(p, q)
			}
		}
	}

	return p
}

// ScalarBaseMult computes p = k*G where G is the generator.
func (p *JacobianPoint) ScalarBaseMult(k *Scalar) *JacobianPoint {
	var g JacobianPoint
	g.FromAffine(&Generator)
	return p.ScalarMult(&g, k)
}

// BasePointMult computes k*G and returns the result in affine coordinates.
func BasePointMult(k *Scalar) *AffinePoint {
	var jac JacobianPoint
	var aff AffinePoint
	jac.ScalarBaseMult(k)
	jac.ToAffine(&aff)
	return &aff
}
