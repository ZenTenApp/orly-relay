//go:build !amd64 && !js && !wasm && !tinygo && !wasm32

package p256k1

// Generic stub implementations for non-AMD64 architectures.
// These simply forward to the pure Go implementations.

func scalarMulAVX2(r, a, b *Scalar) {
	r.mulPureGo(a, b)
}

func scalarAddAVX2(r, a, b *Scalar) {
	r.addPureGo(a, b)
}

func scalarSubAVX2(r, a, b *Scalar) {
	r.subPureGo(a, b)
}
