// Package bdhke implements Blind Diffie-Hellman Key Exchange for Cashu-style tokens.
// This is the core cryptographic primitive used in ecash blind signatures.
//
// The protocol allows a mint (issuer) to sign a message without knowing what
// it's signing, providing unlinkability between token issuance and redemption.
//
// Protocol overview:
//  1. User creates secret x, computes Y = HashToCurve(x)
//  2. User blinds: B_ = Y + r*G (r is random blinding factor)
//  3. Mint signs: C_ = k*B_ (k is mint's private key)
//  4. User unblinds: C = C_ - r*K (K is mint's public key)
//  5. Token is (x, C) - mint can verify: C == k*HashToCurve(x)
//
// Reference: https://github.com/cashubtc/nuts/blob/main/00.md
package bdhke

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// DomainSeparator is prepended to messages before hashing to prevent
// cross-protocol attacks.
const DomainSeparator = "Secp256k1_HashToCurve_Cashu_"

// Errors
var (
	ErrHashToCurveFailed  = errors.New("bdhke: hash to curve failed after max iterations")
	ErrInvalidPoint       = errors.New("bdhke: invalid curve point")
	ErrInvalidPrivateKey  = errors.New("bdhke: invalid private key")
	ErrSignatureMismatch  = errors.New("bdhke: signature verification failed")
)

// HashToCurve deterministically maps a message to a point on secp256k1.
// Uses the try-and-increment method as specified in Cashu NUT-00.
//
// Algorithm:
//  1. Compute msg_hash = SHA256(domain_separator || message)
//  2. For counter in 0..65536:
//     a. Compute hash = SHA256(msg_hash || counter)
//     b. Try to parse 02 || hash as compressed point
//     c. If valid point, return it
//  3. Fail if no valid point found (extremely unlikely)
func HashToCurve(message []byte) (*secp256k1.PublicKey, error) {
	// Hash the message with domain separator
	msgHash := sha256.Sum256(append([]byte(DomainSeparator), message...))

	// Try up to 65536 iterations (in practice, ~50% chance on first try)
	counterBytes := make([]byte, 4)
	for counter := uint32(0); counter < 65536; counter++ {
		binary.LittleEndian.PutUint32(counterBytes, counter)

		// Hash again with counter
		toHash := append(msgHash[:], counterBytes...)
		hash := sha256.Sum256(toHash)

		// Try to parse as compressed point with 02 prefix (even y)
		compressed := make([]byte, 33)
		compressed[0] = 0x02
		copy(compressed[1:], hash[:])

		pk, err := secp256k1.ParsePubKey(compressed)
		if err == nil {
			return pk, nil
		}
	}

	return nil, ErrHashToCurveFailed
}

// BlindResult contains the blinding operation result.
type BlindResult struct {
	B  *secp256k1.PublicKey  // Blinded message B_ = Y + r*G
	R  *secp256k1.PrivateKey // Blinding factor (keep secret until unblinding)
	Y  *secp256k1.PublicKey  // Original point Y = HashToCurve(secret)
}

// Blind creates a blinded message from a secret.
// The blinding factor r is generated randomly and must be kept secret
// until the signature is received and needs to be unblinded.
//
// B_ = Y + r*G where:
//   - Y = HashToCurve(secret)
//   - r = random scalar
//   - G = generator point
func Blind(secret []byte) (*BlindResult, error) {
	// Compute Y = HashToCurve(secret)
	Y, err := HashToCurve(secret)
	if err != nil {
		return nil, fmt.Errorf("blind: %w", err)
	}

	// Generate random blinding factor r
	rBytes := make([]byte, 32)
	if _, err := rand.Read(rBytes); err != nil {
		return nil, fmt.Errorf("blind: failed to generate random: %w", err)
	}
	r := secp256k1.PrivKeyFromBytes(rBytes)

	// Compute r*G (blinding factor times generator)
	rG := new(secp256k1.JacobianPoint)
	secp256k1.ScalarBaseMultNonConst(&r.Key, rG)

	// Convert Y to Jacobian
	yJ := new(secp256k1.JacobianPoint)
	Y.AsJacobian(yJ)

	// Compute B_ = Y + r*G
	bJ := new(secp256k1.JacobianPoint)
	secp256k1.AddNonConst(yJ, rG, bJ)
	bJ.ToAffine()

	// Convert back to PublicKey
	B := secp256k1.NewPublicKey(&bJ.X, &bJ.Y)

	return &BlindResult{
		B: B,
		R: r,
		Y: Y,
	}, nil
}

// BlindWithFactor creates a blinded message using a provided blinding factor.
// This is useful for testing or when the blinding factor needs to be deterministic.
func BlindWithFactor(secret []byte, rBytes []byte) (*BlindResult, error) {
	if len(rBytes) != 32 {
		return nil, errors.New("blind: blinding factor must be 32 bytes")
	}

	// Compute Y = HashToCurve(secret)
	Y, err := HashToCurve(secret)
	if err != nil {
		return nil, fmt.Errorf("blind: %w", err)
	}

	r := secp256k1.PrivKeyFromBytes(rBytes)

	// Compute r*G
	rG := new(secp256k1.JacobianPoint)
	secp256k1.ScalarBaseMultNonConst(&r.Key, rG)

	// Convert Y to Jacobian
	yJ := new(secp256k1.JacobianPoint)
	Y.AsJacobian(yJ)

	// Compute B_ = Y + r*G
	bJ := new(secp256k1.JacobianPoint)
	secp256k1.AddNonConst(yJ, rG, bJ)
	bJ.ToAffine()

	B := secp256k1.NewPublicKey(&bJ.X, &bJ.Y)

	return &BlindResult{
		B: B,
		R: r,
		Y: Y,
	}, nil
}

// Sign creates a blinded signature on a blinded message.
// This is performed by the mint using its private key k.
//
// C_ = k * B_ where:
//   - k = mint's private key scalar
//   - B_ = blinded message from user
func Sign(B *secp256k1.PublicKey, k *secp256k1.PrivateKey) (*secp256k1.PublicKey, error) {
	if B == nil || k == nil {
		return nil, ErrInvalidPoint
	}

	// Convert B to Jacobian
	bJ := new(secp256k1.JacobianPoint)
	B.AsJacobian(bJ)

	// Compute C_ = k * B_
	cJ := new(secp256k1.JacobianPoint)
	secp256k1.ScalarMultNonConst(&k.Key, bJ, cJ)
	cJ.ToAffine()

	C := secp256k1.NewPublicKey(&cJ.X, &cJ.Y)
	return C, nil
}

// Unblind removes the blinding factor from the signature.
// This is performed by the user after receiving the blinded signature.
//
// C = C_ - r*K where:
//   - C_ = blinded signature from mint
//   - r = original blinding factor
//   - K = mint's public key
func Unblind(C_ *secp256k1.PublicKey, r *secp256k1.PrivateKey, K *secp256k1.PublicKey) (*secp256k1.PublicKey, error) {
	if C_ == nil || r == nil || K == nil {
		return nil, ErrInvalidPoint
	}

	// Compute r*K
	kJ := new(secp256k1.JacobianPoint)
	K.AsJacobian(kJ)

	rK := new(secp256k1.JacobianPoint)
	secp256k1.ScalarMultNonConst(&r.Key, kJ, rK)

	// Negate r*K to get -r*K
	rK.Y.Negate(1)
	rK.Y.Normalize()

	// Convert C_ to Jacobian
	c_J := new(secp256k1.JacobianPoint)
	C_.AsJacobian(c_J)

	// Compute C = C_ + (-r*K) = C_ - r*K
	cJ := new(secp256k1.JacobianPoint)
	secp256k1.AddNonConst(c_J, rK, cJ)
	cJ.ToAffine()

	C := secp256k1.NewPublicKey(&cJ.X, &cJ.Y)
	return C, nil
}

// Verify checks that a token's signature is valid.
// The mint uses this to verify tokens during redemption.
//
// Checks: C == k * HashToCurve(secret) where:
//   - C = unblinded signature from token
//   - k = mint's private key
//   - secret = token's secret value
func Verify(secret []byte, C *secp256k1.PublicKey, k *secp256k1.PrivateKey) (bool, error) {
	if C == nil || k == nil {
		return false, ErrInvalidPoint
	}

	// Compute Y = HashToCurve(secret)
	Y, err := HashToCurve(secret)
	if err != nil {
		return false, err
	}

	// Compute expected = k * Y
	yJ := new(secp256k1.JacobianPoint)
	Y.AsJacobian(yJ)

	expectedJ := new(secp256k1.JacobianPoint)
	secp256k1.ScalarMultNonConst(&k.Key, yJ, expectedJ)
	expectedJ.ToAffine()

	expected := secp256k1.NewPublicKey(&expectedJ.X, &expectedJ.Y)

	// Compare C with expected
	return C.IsEqual(expected), nil
}

// VerifyWithPublicKey verifies a token without knowing the private key.
// This requires a DLEQ proof (not yet implemented).
// For now, returns error indicating this is not supported.
func VerifyWithPublicKey(secret []byte, C *secp256k1.PublicKey, K *secp256k1.PublicKey) (bool, error) {
	return false, errors.New("bdhke: DLEQ proof verification not implemented")
}

// GenerateKeypair generates a new mint keypair.
func GenerateKeypair() (*secp256k1.PrivateKey, *secp256k1.PublicKey, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	privKey := secp256k1.PrivKeyFromBytes(keyBytes)
	pubKey := privKey.PubKey()

	return privKey, pubKey, nil
}

// SecretFromBytes creates a secret suitable for token issuance.
// The secret should be 32 bytes of random data.
func SecretFromBytes(data []byte) []byte {
	// Just return a copy - secrets are arbitrary byte strings
	secret := make([]byte, len(data))
	copy(secret, data)
	return secret
}

// GenerateSecret creates a new random 32-byte secret.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	return secret, nil
}
