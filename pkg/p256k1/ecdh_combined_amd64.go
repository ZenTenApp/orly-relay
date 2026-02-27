//go:build amd64 && !purego

package p256k1

// EcmultCombined computes r = na*a + ng*G using 4x64-optimized combined Strauss
// This uses BMI2 MULX instructions for faster field operations
func EcmultCombined(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar) {
	EcmultCombined4x64(r, a, na, ng)
}
