// Package schnorr provides BIP-340 Schnorr signature operations on secp256k1.
//
// This package is a domain-focused wrapper around the core p256k1 primitives,
// providing a clean API for Schnorr signature creation and verification.
//
// # Bounded Context: Digital Signatures (Schnorr/BIP-340)
//
// BIP-340 Schnorr signatures offer several advantages over ECDSA:
//   - Simpler, more elegant mathematical structure
//   - Native support for signature aggregation (future)
//   - Faster batch verification
//   - Smaller signatures with x-only public keys
//
// # Value Objects
//
//   - Signature: A 64-byte Schnorr signature (r || s)
//   - XOnlyPubkey: A 32-byte x-only public key
//   - KeyPair: A secret/public key pair
//
// # Domain Services
//
//   - Sign: Create a signature
//   - Verify: Verify a single signature
//   - VerifyBatch: Verify multiple signatures efficiently
//
// # Usage
//
//	import "next.orly.dev/pkg/p256k1/schnorr"
//
//	// Create a key pair
//	keypair, err := schnorr.NewKeyPair(privateKey)
//	if err != nil {
//	    // handle error
//	}
//
//	// Sign a message
//	sig, err := schnorr.Sign(message32, keypair, auxRand)
//	if err != nil {
//	    // handle error
//	}
//
//	// Verify the signature
//	xonlyPub := keypair.XOnlyPubkey()
//	valid := schnorr.Verify(sig, message32, xonlyPub)
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use.
//
// # Security Notes
//
//   - Uses BIP-340 compliant nonce generation
//   - X-only public keys (32 bytes) implicitly have even Y coordinate
//   - Auxiliary randomness (auxRand) provides additional security against side-channel attacks
package schnorr
