//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import "fmt"

// GetGeneratorXY returns the generator point coordinates as byte slices.
func GetGeneratorXY() (x, y []byte) {
	gx := make([]byte, 32)
	gy := make([]byte, 32)
	g := Generator
	g.x.normalize()
	g.y.normalize()
	g.x.getB32(gx)
	g.y.getB32(gy)
	return gx, gy
}

// TestFieldSetGetB32 tests the setB32→getB32 round-trip.
func TestFieldSetGetB32(in []byte) []byte {
	var fe FieldElement
	fe.setB32(in)
	out := make([]byte, 32)
	fe.getB32(out)
	return out
}

// DumpFieldLimbs prints the internal limb representation of a field element.
func DumpFieldLimbs(b []byte) {
	var fe FieldElement
	fe.setB32(b)
	fmt.Printf("  5x52 limbs: [")
	for i := 0; i < 5; i++ {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("0x%013x", fe.n[i])
	}
	fmt.Printf("]\n")
}

// TestFieldMul multiplies two field elements given as 32-byte big-endian.
func TestFieldMul(a, b []byte) []byte {
	var fa, fb, fc FieldElement
	fa.setB32(a)
	fb.setB32(b)
	fc.mul(&fa, &fb)
	out := make([]byte, 32)
	fc.getB32(out)
	return out
}

// TestFieldSqr squares a field element given as 32-byte big-endian.
func TestFieldSqr(a []byte) []byte {
	var fa, fc FieldElement
	fa.setB32(a)
	fc.sqr(&fa)
	out := make([]byte, 32)
	fc.getB32(out)
	return out
}

// TestFieldInvMul computes inv(a)*a — should return 1.
func TestFieldInvMul(a []byte) []byte {
	var fa, inv, result FieldElement
	fa.setB32(a)
	inv.inv(&fa)
	result.mul(&inv, &fa)
	out := make([]byte, 32)
	result.getB32(out)
	return out
}

// TestFieldSqrt computes sqrt(a) and returns (result, ok).
func TestFieldSqrt(a []byte) ([]byte, bool) {
	var fa, fr FieldElement
	fa.setB32(a)
	ok := fr.sqrt(&fa)
	out := make([]byte, 32)
	fr.getB32(out)
	return out, ok
}

// TestFieldAdd computes a + b mod p.
func TestFieldAdd(a, b []byte) []byte {
	var fa, fb FieldElement
	fa.setB32(a)
	fb.setB32(b)
	fa.add(&fb)
	out := make([]byte, 32)
	fa.getB32(out)
	return out
}

// TestJacobianRoundTrip converts affine→Jacobian(z=1)→affine.
func TestJacobianRoundTrip(xb, yb []byte) ([]byte, []byte) {
	var pt GroupElementAffine
	pt.x.setB32(xb)
	pt.y.setB32(yb)
	pt.infinity = false

	var jac GroupElementJacobian
	jac.setGE(&pt)

	var aff GroupElementAffine
	aff.setGEJ(&jac)
	aff.x.normalize()
	aff.y.normalize()

	ox := make([]byte, 32)
	oy := make([]byte, 32)
	aff.x.getB32(ox)
	aff.y.getB32(oy)
	return ox, oy
}

// TestPointDouble doubles an affine point and returns the result as affine.
func TestPointDouble(xb, yb []byte) ([]byte, []byte) {
	var pt GroupElementAffine
	pt.x.setB32(xb)
	pt.y.setB32(yb)
	pt.infinity = false

	var jac GroupElementJacobian
	jac.setGE(&pt)

	var doubled GroupElementJacobian
	doubled.double(&jac)

	var aff GroupElementAffine
	aff.setGEJ(&doubled)
	aff.x.normalize()
	aff.y.normalize()

	ox := make([]byte, 32)
	oy := make([]byte, 32)
	aff.x.getB32(ox)
	aff.y.getB32(oy)
	return ox, oy
}
