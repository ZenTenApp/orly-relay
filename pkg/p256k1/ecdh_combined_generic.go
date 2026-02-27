//go:build !amd64 || purego

package p256k1

// EcmultCombined computes r = na*a + ng*G using combined Strauss algorithm
// This shares doublings between both multiplications for improved performance
func EcmultCombined(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar) {
	ecmultCombinedGeneric(r, a, na, ng)
}
