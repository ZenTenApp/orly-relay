//go:build !amd64 || purego

package p256k1

// EcmultGenGLV is the public interface for fast generator multiplication
// r = k * G
func EcmultGenGLV(r *GroupElementJacobian, k *Scalar) {
	ecmultGenGLV(r, k)
}

// EcmultGen computes r = k * G using the fastest available method
// This is the main entry point for generator multiplication throughout the codebase
func EcmultGen(r *GroupElementJacobian, k *Scalar) {
	ecmultGenGLV(r, k)
}
