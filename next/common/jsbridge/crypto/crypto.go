// Package crypto provides secp256k1 Schnorr operations via WASM bridge.
// When compiled with tinyjs, these functions become calls to $rt.crypto.FuncName().
package crypto

// PubKeyFromSecKey derives a 32-byte x-only public key from a 32-byte secret key.
func PubKeyFromSecKey(seckey []byte) []byte {
	panic("jsbridge")
}

// SignSchnorr creates a BIP-340 Schnorr signature.
// Returns a 64-byte signature or nil on failure.
func SignSchnorr(seckey, msg, auxRand []byte) []byte {
	panic("jsbridge")
}

// VerifySchnorr verifies a BIP-340 Schnorr signature.
func VerifySchnorr(pubkey, msg, sig []byte) bool {
	panic("jsbridge")
}
