// Package keys provides secp256k1 key management operations.
//
// This package is a domain-focused wrapper around the core p256k1 primitives,
// providing a clean API for key generation, parsing, and serialization.
//
// # Bounded Context: Key Management
//
// This bounded context encompasses:
//   - Key pair generation (secret + public key)
//   - Public key creation from private key
//   - Key parsing and serialization
//   - Key validation
//   - Key tweaking (for advanced protocols)
//
// # Aggregate Root: KeyPair
//
// The KeyPair type is the aggregate root for key management. It encapsulates
// the relationship between a secret key and its corresponding public key,
// ensuring consistency and providing a unified interface for key operations.
//
// # Value Objects
//
//   - PublicKey: A secp256k1 public key (can be compressed or uncompressed)
//   - XOnlyPubkey: A 32-byte x-only public key (BIP-340 style)
//   - SecretKey: A 32-byte private key (represented as []byte)
//
// # Domain Services
//
//   - Generate: Generate a new random key pair
//   - Create: Create a key pair from an existing private key
//   - ParsePublicKey: Parse a serialized public key
//   - SerializePublicKey: Serialize a public key
//
// # Usage
//
//	import "p256k1.mleku.dev/keys"
//
//	// Generate a new key pair
//	keypair, err := keys.Generate()
//	if err != nil {
//	    // handle error
//	}
//
//	// Get the public key in compressed format
//	pubkeyBytes := keys.SerializePublic(keypair.PublicKey(), keys.Compressed)
//
//	// Parse a public key
//	pubkey, err := keys.ParsePublic(pubkeyBytes)
//	if err != nil {
//	    // handle error
//	}
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use.
//
// # Security Notes
//
//   - Private keys should be generated with a cryptographically secure random source
//   - Clear private key material when no longer needed using KeyPair.Clear()
//   - Never log or expose private key bytes
package keys
