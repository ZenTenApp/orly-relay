package exchange

import (
	"next.orly.dev/pkg/p256k1"
)

// =============================================================================
// Type Aliases
// =============================================================================

// PublicKey represents a secp256k1 public key.
type PublicKey = p256k1.PublicKey

// HashFunc is a function type for hashing ECDH shared secrets.
// It receives output buffer, x-coordinate (32 bytes), and y-coordinate (32 bytes).
type HashFunc = p256k1.ECDHHashFunction

// =============================================================================
// ECDH Shared Secret Computation (Domain Service)
// =============================================================================

// SharedSecret computes an ECDH shared secret between a private key and
// a public key.
//
// The result is SHA256(version || x) where version depends on Y coordinate parity.
// This follows the standard secp256k1 ECDH convention.
//
// Parameters:
//   - pubkey: The other party's public key
//   - privateKey: Your 32-byte private key
//
// Returns a 32-byte shared secret suitable for use as a symmetric key.
func SharedSecret(pubkey *PublicKey, privateKey []byte) ([]byte, error) {
	output := make([]byte, 32)
	if err := p256k1.ECDH(output, pubkey, privateKey, nil); err != nil {
		return nil, err
	}
	return output, nil
}

// SharedSecretWithHash computes an ECDH shared secret using a custom hash function.
//
// The hash function receives the x and y coordinates and should produce
// the final shared secret.
func SharedSecretWithHash(pubkey *PublicKey, privateKey []byte, hashFn HashFunc) ([]byte, error) {
	output := make([]byte, 32)
	if err := p256k1.ECDH(output, pubkey, privateKey, hashFn); err != nil {
		return nil, err
	}
	return output, nil
}

// SharedSecretRaw computes an ECDH shared secret and writes it to the provided buffer.
// This avoids allocation by writing directly to the caller's buffer.
func SharedSecretRaw(output []byte, pubkey *PublicKey, privateKey []byte) error {
	return p256k1.ECDH(output, pubkey, privateKey, nil)
}

// =============================================================================
// X-Only ECDH (BIP-340 Compatible)
// =============================================================================

// XOnlySharedSecret computes an X-only ECDH shared secret.
//
// Returns only the X coordinate of the shared point (32 bytes).
// This is useful for BIP-340 compatible protocols.
func XOnlySharedSecret(pubkey *PublicKey, privateKey []byte) ([]byte, error) {
	output := make([]byte, 32)
	if err := p256k1.ECDHXOnly(output, pubkey, privateKey); err != nil {
		return nil, err
	}
	return output, nil
}

// XOnlySharedSecretRaw computes an X-only shared secret and writes it to the provided buffer.
func XOnlySharedSecretRaw(output []byte, pubkey *PublicKey, privateKey []byte) error {
	return p256k1.ECDHXOnly(output, pubkey, privateKey)
}

// =============================================================================
// Key Derivation (Domain Service)
// =============================================================================

// DeriveKey derives a key from input keying material using HKDF (RFC 5869).
//
// Parameters:
//   - output: Buffer to write the derived key (any length)
//   - ikm: Input keying material (e.g., ECDH shared secret)
//   - salt: Optional salt value (can be nil for no salt)
//   - info: Optional context/application-specific info (can be nil)
//
// This is useful for deriving multiple keys from a single shared secret,
// for example separate encryption and MAC keys.
func DeriveKey(output, ikm, salt, info []byte) error {
	return p256k1.HKDF(output, ikm, salt, info)
}

// SharedSecretWithDerivation combines ECDH and HKDF into a single operation.
//
// Computes the ECDH shared secret and then derives a key using HKDF.
// This is a convenience function for the common pattern of:
//  1. Compute ECDH shared secret
//  2. Derive key using HKDF
func SharedSecretWithDerivation(pubkey *PublicKey, privateKey, salt, info []byte, keyLen int) ([]byte, error) {
	output := make([]byte, keyLen)
	if err := p256k1.ECDHWithHKDF(output, pubkey, privateKey, salt, info); err != nil {
		return nil, err
	}
	return output, nil
}

// SharedSecretWithDerivationRaw is like SharedSecretWithDerivation but writes to a provided buffer.
func SharedSecretWithDerivationRaw(output []byte, pubkey *PublicKey, privateKey, salt, info []byte) error {
	return p256k1.ECDHWithHKDF(output, pubkey, privateKey, salt, info)
}
