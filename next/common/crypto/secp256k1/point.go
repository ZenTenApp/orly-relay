package secp256k1

// Point on secp256k1 in Jacobian coordinates (X, Y, Z).
// Affine point is (X/Z^2, Y/Z^3). Point at infinity has Z=0.
type Point struct {
	X, Y, Z Fe
}

// Affine point.
type AffinePoint struct {
	X, Y Fe
}

// Generator point G.
var G = AffinePoint{
	X: Fe{
		0x59F2815B16F81798,
		0x029BFCDB2DCE28D9,
		0x55A06295CE870B07,
		0x79BE667EF9DCBBAC,
	},
	Y: Fe{
		0x9C47D08FFB10D4B8,
		0xFD17B448A6855419,
		0x5DA4FBFC0E1108A8,
		0x483ADA7726A3C465,
	},
}

// Curve order n.
var curveN = Fe{
	0xBFD25E8CD0364141,
	0xBAAEDCE6AF48A03B,
	0xFFFFFFFFFFFFFFFE,
	0xFFFFFFFFFFFFFFFF,
}

// Infinity is the point at infinity.
var Infinity = Point{feZero, feOne, feZero}

// pointFromAffine converts an affine point to Jacobian.
func pointFromAffine(p AffinePoint) Point {
	return Point{p.X, p.Y, feOne}
}

// toAffine converts a Jacobian point to affine.
func (p Point) toAffine() AffinePoint {
	if feIsZero(p.Z) {
		return AffinePoint{feZero, feZero}
	}
	zInv := feInv(p.Z)
	z2 := feSqr(zInv)
	z3 := feMul(z2, zInv)
	return AffinePoint{
		X: feMul(p.X, z2),
		Y: feMul(p.Y, z3),
	}
}

// isInfinity returns true if the point is at infinity.
func (p Point) isInfinity() bool {
	return feIsZero(p.Z)
}

// pointDouble computes 2*P in Jacobian coordinates.
func pointDouble(p Point) Point {
	if feIsZero(p.Z) {
		return p
	}

	// http://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html#doubling-dbl-2009-l
	a := feSqr(p.X)
	b := feSqr(p.Y)
	c := feSqr(b)

	// d = 2*((X1+B)^2-A-C)
	xb := feAdd(p.X, b)
	xb2 := feSqr(xb)
	d := feSub(feSub(xb2, a), c)
	d = feAdd(d, d)

	// e = 3*A
	e := feAdd(feAdd(a, a), a)

	f := feSqr(e)

	// X3 = F - 2*D
	x3 := feSub(f, feAdd(d, d))

	// Y3 = E*(D-X3) - 8*C
	y3 := feMul(e, feSub(d, x3))
	c8 := feAdd(c, c)
	c8 = feAdd(c8, c8)
	c8 = feAdd(c8, c8)
	y3 = feSub(y3, c8)

	// Z3 = 2*Y1*Z1
	z3 := feMul(p.Y, p.Z)
	z3 = feAdd(z3, z3)

	return Point{x3, y3, z3}
}

// pointAdd computes P + Q in Jacobian coordinates.
func pointAdd(p, q Point) Point {
	if p.isInfinity() {
		return q
	}
	if q.isInfinity() {
		return p
	}

	// U1 = X1*Z2^2, U2 = X2*Z1^2
	z1sq := feSqr(p.Z)
	z2sq := feSqr(q.Z)
	u1 := feMul(p.X, z2sq)
	u2 := feMul(q.X, z1sq)

	// S1 = Y1*Z2^3, S2 = Y2*Z1^3
	s1 := feMul(p.Y, feMul(z2sq, q.Z))
	s2 := feMul(q.Y, feMul(z1sq, p.Z))

	if u1 == u2 {
		if s1 == s2 {
			return pointDouble(p)
		}
		return Infinity
	}

	// H = U2 - U1
	h := feSub(u2, u1)
	// R = S2 - S1
	r := feSub(s2, s1)

	h2 := feSqr(h)
	h3 := feMul(h2, h)

	// X3 = R^2 - H^3 - 2*U1*H^2
	x3 := feSub(feSub(feSqr(r), h3), feAdd(feMul(u1, h2), feMul(u1, h2)))

	// Y3 = R*(U1*H^2 - X3) - S1*H^3
	y3 := feSub(feMul(r, feSub(feMul(u1, h2), x3)), feMul(s1, h3))

	// Z3 = Z1*Z2*H
	z3 := feMul(feMul(p.Z, q.Z), h)

	return Point{x3, y3, z3}
}

// ScalarBaseMult computes k*G using double-and-add.
func ScalarBaseMult(k Fe) AffinePoint {
	return ScalarMult(pointFromAffine(G), k).toAffine()
}

// ScalarMult computes k*P.
func ScalarMult(p Point, k Fe) Point {
	result := Infinity
	current := p

	for i := 0; i < 4; i++ {
		w := k[i]
		for bit := 0; bit < 64; bit++ {
			if w&1 == 1 {
				result = pointAdd(result, current)
			}
			current = pointDouble(current)
			w >>= 1
		}
	}
	return result
}

// LiftX recovers a point from x-coordinate only (BIP-340).
// Returns the point with even y.
func LiftX(x Fe) (AffinePoint, bool) {
	// y^2 = x^3 + 7
	x3 := feMul(feSqr(x), x)
	y2 := feAdd(x3, Fe{7, 0, 0, 0})
	y, ok := feSqrt(y2)
	if !ok {
		return AffinePoint{}, false
	}
	// BIP-340: choose even y.
	if !feIsEven(y) {
		y = feNeg(y)
	}
	return AffinePoint{x, y}, true
}
