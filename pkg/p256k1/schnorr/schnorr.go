package schnorr

import (
	"errors"

	"p256k1.mleku.dev"
)

// Signature is a 64-byte BIP-340 Schnorr signature.
// This is a value object - immutable once created.
type Signature = p256k1.SchnorrSignature

// XOnlyPubkey is a 32-byte x-only public key (BIP-340).
// X-only keys implicitly have even Y coordinate.
type XOnlyPubkey = p256k1.XOnlyPubkey

// KeyPair represents a secret/public key pair for Schnorr signing.
// This is an aggregate root for key management within the Schnorr context.
type KeyPair = p256k1.KeyPair

// PublicKey is an alias for the core public key type.
type PublicKey = p256k1.PublicKey

// =============================================================================
// Key Management
// =============================================================================

// NewKeyPair creates a key pair from a 32-byte private key.
func NewKeyPair(privateKey []byte) (*KeyPair, error) {
	return p256k1.KeyPairCreate(privateKey)
}

// GenerateKeyPair generates a new random key pair.
func GenerateKeyPair() (*KeyPair, error) {
	return p256k1.KeyPairGenerate()
}

// ParseXOnlyPubkey parses a 32-byte x-only public key.
func ParseXOnlyPubkey(input []byte) (*XOnlyPubkey, error) {
	return p256k1.XOnlyPubkeyParse(input)
}

// XOnlyFromPubkey converts a full public key to x-only format.
// Returns the x-only key and parity (1 if Y was odd, 0 if even).
func XOnlyFromPubkey(pubkey *PublicKey) (*XOnlyPubkey, int, error) {
	return p256k1.XOnlyPubkeyFromPubkey(pubkey)
}

// =============================================================================
// Signature Creation (Domain Service)
// =============================================================================

// Sign creates a BIP-340 Schnorr signature.
//
// Parameters:
//   - message: The 32-byte message hash to sign
//   - keypair: The key pair to sign with
//   - auxRand: Optional 32-byte auxiliary randomness (can be nil)
//
// The auxiliary randomness provides additional security against side-channel
// attacks. If nil, a deterministic fallback is used.
//
// Returns a 64-byte signature (r || s).
func Sign(message []byte, keypair *KeyPair, auxRand []byte) (*Signature, error) {
	if len(message) != 32 {
		return nil, errors.New("message must be 32 bytes")
	}
	if keypair == nil {
		return nil, errors.New("keypair cannot be nil")
	}

	var sig Signature
	if err := p256k1.SchnorrSign(sig[:], message, keypair, auxRand); err != nil {
		return nil, err
	}
	return &sig, nil
}

// SignRaw creates a signature and writes it to the provided 64-byte buffer.
// This is a lower-level function that avoids allocation.
func SignRaw(sig64 []byte, message []byte, keypair *KeyPair, auxRand []byte) error {
	return p256k1.SchnorrSign(sig64, message, keypair, auxRand)
}

// =============================================================================
// Signature Verification (Domain Service)
// =============================================================================

// Verify verifies a BIP-340 Schnorr signature.
//
// Parameters:
//   - sig: The 64-byte signature
//   - message: The 32-byte message hash
//   - pubkey: The x-only public key
//
// Returns true if the signature is valid, false otherwise.
func Verify(sig *Signature, message []byte, pubkey *XOnlyPubkey) bool {
	return p256k1.SchnorrVerify(sig[:], message, pubkey)
}

// VerifyRaw verifies a signature from raw byte slices.
func VerifyRaw(sig64, message []byte, pubkey *XOnlyPubkey) bool {
	return p256k1.SchnorrVerify(sig64, message, pubkey)
}

// =============================================================================
// Batch Verification
// =============================================================================

// VerifyItem represents a single signature to verify in a batch.
type VerifyItem struct {
	Signature *Signature
	Message   []byte
	Pubkey    *XOnlyPubkey
}

// VerifyBatch verifies multiple Schnorr signatures efficiently.
//
// Batch verification uses the Strauss algorithm to share doublings across
// all verifications, making it significantly faster than individual verification
// when verifying multiple signatures.
//
// Returns true only if ALL signatures are valid.
func VerifyBatch(items []VerifyItem) bool {
	// Convert to the internal batch format
	batchItems := make([]p256k1.BatchSchnorrItem, len(items))
	for i, item := range items {
		batchItems[i] = p256k1.BatchSchnorrItem{
			Signature: item.Signature[:],
			Message:   item.Message,
			Pubkey:    item.Pubkey,
		}
	}
	return p256k1.SchnorrBatchVerify(batchItems)
}

// =============================================================================
// Signature Serialization
// =============================================================================

// ParseSignature parses a 64-byte Schnorr signature.
func ParseSignature(sig64 []byte) (*Signature, error) {
	if len(sig64) != 64 {
		return nil, errors.New("signature must be 64 bytes")
	}
	var sig Signature
	copy(sig[:], sig64)
	return &sig, nil
}

// Bytes returns the signature as a 64-byte slice.
func Bytes(sig *Signature) []byte {
	return sig[:]
}

// R returns the R component (first 32 bytes) of the signature.
func R(sig *Signature) []byte {
	return sig[:32]
}

// S returns the S component (last 32 bytes) of the signature.
func S(sig *Signature) []byte {
	return sig[32:]
}

// =============================================================================
// X-Only Public Key Operations
// =============================================================================

// SerializeXOnly serializes an x-only public key to 32 bytes.
func SerializeXOnly(pubkey *XOnlyPubkey) [32]byte {
	return pubkey.Serialize()
}

// CompareXOnly compares two x-only public keys lexicographically.
// Returns <0 if a < b, >0 if a > b, 0 if equal.
func CompareXOnly(a, b *XOnlyPubkey) int {
	return p256k1.XOnlyPubkeyCmp(a, b)
}
