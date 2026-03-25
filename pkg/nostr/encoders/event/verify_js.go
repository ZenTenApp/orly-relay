//go:build js || wasm

package event

import "errors"

// Verify is a stub for WASM builds — signature verification is handled
// by the browser extension via the crypto proxy chain.
func (ev *E) Verify() (valid bool, err error) {
	return false, errors.New("verify not available in WASM build")
}
