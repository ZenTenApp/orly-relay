package keys

import (
	"errors"

	"next.orly.dev/pkg/p256k1"
)

// =============================================================================
// Type Aliases (Value Objects)
// =============================================================================

// PublicKey represents a secp256k1 public key.
type PublicKey = p256k1.PublicKey

// XOnlyPubkey represents a 32-byte x-only public key (BIP-340).
type XOnlyPubkey = p256k1.XOnlyPubkey

// KeyPair represents a secret/public key pair (Aggregate Root).
type KeyPair = p256k1.KeyPair

// Serialization format flags.
const (
	Compressed   = p256k1.ECCompressed   // 33-byte compressed format
	Uncompressed = p256k1.ECUncompressed // 65-byte uncompressed format
)

// =============================================================================
// Key Pair Management (Aggregate Root Operations)
// =============================================================================

// Generate creates a new random key pair.
//
// The private key is generated using a cryptographically secure random source.
// Returns an error if key generation fails.
func Generate() (*KeyPair, error) {
	return p256k1.KeyPairGenerate()
}

// Create creates a key pair from an existing 32-byte private key.
//
// Returns an error if:
//   - privateKey is not exactly 32 bytes
//   - privateKey is zero
//   - privateKey is >= curve order
func Create(privateKey []byte) (*KeyPair, error) {
	return p256k1.KeyPairCreate(privateKey)
}

// =============================================================================
// Public Key Operations (Value Object Services)
// =============================================================================

// CreatePublic derives a public key from a 32-byte private key.
func CreatePublic(privateKey []byte) (*PublicKey, error) {
	var pubkey PublicKey
	if err := p256k1.ECPubkeyCreate(&pubkey, privateKey); err != nil {
		return nil, err
	}
	return &pubkey, nil
}

// ParsePublic parses a public key from serialized form.
//
// Accepts:
//   - 33 bytes: compressed format (0x02 or 0x03 prefix)
//   - 65 bytes: uncompressed format (0x04 prefix)
func ParsePublic(input []byte) (*PublicKey, error) {
	var pubkey PublicKey
	if err := p256k1.ECPubkeyParse(&pubkey, input); err != nil {
		return nil, err
	}
	return &pubkey, nil
}

// SerializePublic serializes a public key to the specified format.
//
// Returns the serialized bytes and the number of bytes written.
// Use Compressed for 33 bytes, Uncompressed for 65 bytes.
func SerializePublic(pubkey *PublicKey, format uint) []byte {
	var output []byte
	if format == Compressed {
		output = make([]byte, 33)
	} else {
		output = make([]byte, 65)
	}
	n := p256k1.ECPubkeySerialize(output, pubkey, format)
	return output[:n]
}

// ComparePublic compares two public keys lexicographically.
// Returns <0 if a < b, >0 if a > b, 0 if equal.
func ComparePublic(a, b *PublicKey) int {
	return p256k1.ECPubkeyCmp(a, b)
}

// =============================================================================
// X-Only Public Key Operations
// =============================================================================

// ParseXOnly parses a 32-byte x-only public key.
func ParseXOnly(input []byte) (*XOnlyPubkey, error) {
	return p256k1.XOnlyPubkeyParse(input)
}

// XOnlyFromPublic converts a full public key to x-only format.
// Returns the x-only key and parity (1 if Y was odd, 0 if even).
func XOnlyFromPublic(pubkey *PublicKey) (*XOnlyPubkey, int, error) {
	return p256k1.XOnlyPubkeyFromPubkey(pubkey)
}

// SerializeXOnly serializes an x-only public key to 32 bytes.
func SerializeXOnly(pubkey *XOnlyPubkey) [32]byte {
	return pubkey.Serialize()
}

// CompareXOnly compares two x-only public keys lexicographically.
// Returns <0 if a < b, >0 if a > b, 0 if equal.
func CompareXOnly(a, b *XOnlyPubkey) int {
	return p256k1.XOnlyPubkeyCmp(a, b)
}

// =============================================================================
// Private Key Validation
// =============================================================================

// ValidatePrivate checks if a 32-byte slice is a valid private key.
//
// Returns true if the key is valid:
//   - Exactly 32 bytes
//   - Non-zero
//   - Less than the curve order
func ValidatePrivate(privateKey []byte) bool {
	return p256k1.ECSeckeyVerify(privateKey)
}

// =============================================================================
// Key Tweaking (Advanced Operations)
// =============================================================================

// TweakAddPrivate adds a 32-byte tweak to a private key.
// Computes: result = (privateKey + tweak) mod n
func TweakAddPrivate(privateKey, tweak []byte) ([]byte, error) {
	result := make([]byte, 32)
	copy(result, privateKey)
	if err := p256k1.ECSeckeyTweakAdd(result, tweak); err != nil {
		return nil, err
	}
	return result, nil
}

// TweakMulPrivate multiplies a private key by a 32-byte tweak.
// Computes: result = (privateKey * tweak) mod n
func TweakMulPrivate(privateKey, tweak []byte) ([]byte, error) {
	result := make([]byte, 32)
	copy(result, privateKey)
	if err := p256k1.ECSeckeyTweakMul(result, tweak); err != nil {
		return nil, err
	}
	return result, nil
}

// TweakAddPublic adds a 32-byte tweak to a public key.
// Computes: result = pubkey + tweak*G
func TweakAddPublic(pubkey *PublicKey, tweak []byte) (*PublicKey, error) {
	result := *pubkey
	if err := p256k1.ECPubkeyTweakAdd(&result, tweak); err != nil {
		return nil, err
	}
	return &result, nil
}

// TweakMulPublic multiplies a public key by a 32-byte tweak.
// Computes: result = tweak * pubkey
func TweakMulPublic(pubkey *PublicKey, tweak []byte) (*PublicKey, error) {
	result := *pubkey
	if err := p256k1.ECPubkeyTweakMul(&result, tweak); err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// Private Key Operations
// =============================================================================

// NegatePrivate negates a private key.
// Computes: result = -privateKey mod n
func NegatePrivate(privateKey []byte) ([]byte, error) {
	result := make([]byte, 32)
	copy(result, privateKey)
	if !p256k1.ECSeckeyNegate(result) {
		return nil, errors.New("invalid private key")
	}
	return result, nil
}
