// Package exchange provides Elliptic Curve Diffie-Hellman (ECDH) key exchange
// operations on the secp256k1 curve.
//
// This package is a domain-focused wrapper around the core p256k1 primitives,
// providing a clean API for key exchange and shared secret derivation.
//
// # Bounded Context: Key Exchange
//
// This bounded context encompasses:
//   - ECDH shared secret computation
//   - X-only ECDH (BIP-340 compatible)
//   - Key derivation from shared secrets (HKDF)
//
// # Domain Services
//
//   - SharedSecret: Compute ECDH shared secret
//   - SharedSecretXOnly: Compute X-only shared secret
//   - DeriveKey: Derive keys using HKDF
//
// # Usage
//
//	import "p256k1.mleku.dev/exchange"
//
//	// Compute shared secret between Alice and Bob
//	// Alice has alicePrivate, Bob has bobPublic
//	shared, err := exchange.SharedSecret(bobPublic, alicePrivate)
//	if err != nil {
//	    // handle error
//	}
//
//	// Derive an encryption key from the shared secret
//	key := make([]byte, 32)
//	err = exchange.DeriveKey(key, shared, nil, []byte("encryption"))
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use.
//
// # Security Notes
//
//   - Shared secrets should be used with a KDF (like HKDF) for key derivation
//   - Clear shared secret material when no longer needed
//   - Use X-only mode for BIP-340 compatible protocols
package exchange
