//go:build js || wasm || tinygo || wasm32

package p256k1

import (
	"encoding/hex"
	"fmt"
)

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
	fmt.Printf("  10x26 limbs: [")
	for i := 0; i < 10; i++ {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("0x%07x", fe.n[i])
	}
	fmt.Printf("]\n")
	fmt.Printf("  magnitude=%d normalized=%v\n", fe.magnitude, fe.normalized)
	fe.normalize()
	fmt.Printf("  after normalize: [")
	for i := 0; i < 10; i++ {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("0x%07x", fe.n[i])
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

// TestJacobianNonTrivialZ tests Jacobian round-trip with z=3.
func TestJacobianNonTrivialZ(xb, yb []byte) ([]byte, []byte) {
	var x, y, z FieldElement
	x.setB32(xb)
	y.setB32(yb)
	z.setInt(3)

	// Scale: X' = x*z^2, Y' = y*z^3
	var z2, z3 FieldElement
	z2.sqr(&z)       // z^2 = 9
	z3.mul(&z2, &z)  // z^3 = 27
	var xj, yj FieldElement
	xj.mul(&x, &z2)
	yj.mul(&y, &z3)

	// Build Jacobian point
	var jac GroupElementJacobian
	jac.x = xj
	jac.y = yj
	jac.z = z
	jac.infinity = false

	// Convert back
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

// TestPointDoubleVerbose dumps intermediate values from point doubling.
func TestPointDoubleVerbose(xb, yb []byte) {
	var pt GroupElementAffine
	pt.x.setB32(xb)
	pt.y.setB32(yb)
	pt.infinity = false

	var jac GroupElementJacobian
	jac.setGE(&pt)

	var doubled GroupElementJacobian
	doubled.double(&jac)

	// Dump Jacobian coords before conversion
	var xj, yj, zj [32]byte
	doubled.x.normalize()
	doubled.y.normalize()
	doubled.z.normalize()
	doubled.x.getB32(xj[:])
	doubled.y.getB32(yj[:])
	doubled.z.getB32(zj[:])
	fmt.Printf("  Jac X = %s\n", hex.EncodeToString(xj[:]))
	fmt.Printf("  Jac Y = %s\n", hex.EncodeToString(yj[:]))
	fmt.Printf("  Jac Z = %s\n", hex.EncodeToString(zj[:]))

	// Test: inv(Z) * Z = 1?
	var zinv, check FieldElement
	zinv.inv(&doubled.z)
	check.mul(&zinv, &doubled.z)
	check.normalize()
	var cb [32]byte
	check.getB32(cb[:])
	fmt.Printf("  inv(Z)*Z = %s\n", hex.EncodeToString(cb[:]))

	// Convert to affine
	var aff GroupElementAffine
	aff.setGEJ(&doubled)
	aff.x.normalize()
	aff.y.normalize()
	var ax, ay [32]byte
	aff.x.getB32(ax[:])
	aff.y.getB32(ay[:])
	fmt.Printf("  Aff X = %s\n", hex.EncodeToString(ax[:]))
	fmt.Printf("  Aff Y = %s\n", hex.EncodeToString(ay[:]))
}

// TestEcmultGenSimple computes seckey*G via the non-GLV simple path
// and returns the compressed pubkey (33 bytes).
func TestEcmultGenSimple(seckey []byte) [33]byte {
	var scalar Scalar
	scalar.setB32(seckey)
	var jac GroupElementJacobian
	ecmultGenSimple(&jac, &scalar)
	var aff GroupElementAffine
	aff.setGEJ(&jac)
	aff.x.normalize()
	aff.y.normalize()
	var result [33]byte
	aff.x.getB32(result[1:33])
	// Determine parity
	var yb [32]byte
	aff.y.getB32(yb[:])
	if yb[31]&1 == 0 {
		result[0] = 0x02
	} else {
		result[0] = 0x03
	}
	return result
}
