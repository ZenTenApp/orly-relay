//go:build !amd64 || purego

package p256k1

// EcmultGenGLV is the public interface for fast generator multiplication
// r = k * G
func EcmultGenGLV(r *GroupElementJacobian, k *Scalar) {
	ecmultGenGLV(r, k)
}

// EcmultGen computes r = k * G using the fastest available method
// This is the main entry point for generator multiplication throughout the codebase
// On non-amd64 (including WASM), uses the simple wNAF path because the 32-bit
// scalar multiplication needed for GLV decomposition has a known bug.
func EcmultGen(r *GroupElementJacobian, k *Scalar) {
	ecmultGenSimple(r, k)
}
