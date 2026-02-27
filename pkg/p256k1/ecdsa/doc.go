// Package ecdsa provides ECDSA (Elliptic Curve Digital Signature Algorithm)
// operations on the secp256k1 curve.
//
// This package is a domain-focused wrapper around the core p256k1 primitives,
// providing a clean API for ECDSA signature creation and verification.
//
// # Bounded Context: Digital Signatures (ECDSA)
//
// This bounded context encompasses:
//   - Signature creation from message hash and private key
//   - Signature verification against public key
//   - Signature serialization (compact and DER formats)
//   - Public key recovery from signatures
//
// # Value Objects
//
//   - Signature: An ECDSA signature (r, s components)
//   - CompactSignature: 64-byte compact format (r || s)
//   - RecoverableSignature: Signature with recovery ID
//
// # Domain Services
//
//   - Sign: Create a signature
//   - Verify: Verify a signature
//   - Recover: Recover public key from signature
//
// # Usage
//
//	import "p256k1.mleku.dev/ecdsa"
//
//	// Sign a message hash
//	sig, err := ecdsa.Sign(messageHash, privateKey)
//	if err != nil {
//	    // handle error
//	}
//
//	// Verify the signature
//	valid := ecdsa.Verify(sig, messageHash, publicKey)
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use.
//
// # Security Notes
//
//   - Uses RFC 6979 for deterministic nonce generation
//   - Automatically normalizes to low-S form (BIP-146)
//   - Message must be a 32-byte hash (use crypto/sha256 or similar)
package ecdsa
