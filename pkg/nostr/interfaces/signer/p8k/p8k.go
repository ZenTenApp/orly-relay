// Package p8k provides a signer.I implementation using the pure Go
// p256k1.mleku.dev library with BMI2-accelerated assembly on AMD64.
package p8k

import (
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	p256k1signer "git.smesh.lol/orly/pkg/p256k1/signer"
)

// Signer implements the signer.I interface using p256k1.mleku.dev
type Signer struct {
	*p256k1signer.P256K1Signer
}

// Ensure Signer implements signer.I
var _ signer.I = (*Signer)(nil)

// New creates a new P8K signer using p256k1.mleku.dev
func New() (s *Signer, err error) {
	return &Signer{
		P256K1Signer: p256k1signer.NewP256K1Signer(),
	}, nil
}

// MustNew creates a new P8K signer and panics on error
func MustNew() *Signer {
	s, err := New()
	if err != nil {
		panic(err)
	}
	return s
}

// GetModuleStatus returns information about which modules are available
func (s *Signer) GetModuleStatus() map[string]bool {
	return map[string]bool{
		"implementation": true, // Using p256k1.mleku.dev
		"schnorr":        true,
		"ecdh":           true,
		"recovery":       true,
	}
}
