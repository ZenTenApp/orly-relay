package ecdsa

import (
	"errors"

	"git.smesh.lol/orly/pkg/p256k1"
)

// Signature represents an ECDSA signature.
// This is a value object - immutable once created.
type Signature struct {
	sig p256k1.ECDSASignature
}

// CompactSignature is a 64-byte compact signature format (r || s).
type CompactSignature = p256k1.ECDSASignatureCompact

// RecoverableSignature includes a recovery ID for public key recovery.
type RecoverableSignature struct {
	sig p256k1.ECDSARecoverableSignature
}

// PublicKey is an alias for the core public key type.
type PublicKey = p256k1.PublicKey

// =============================================================================
// Signature Creation (Domain Service)
// =============================================================================

// Sign creates an ECDSA signature for a 32-byte message hash using a 32-byte
// private key.
//
// The signature uses RFC 6979 for deterministic nonce generation and is
// automatically normalized to low-S form (BIP-146 compliant).
//
// Returns an error if:
//   - messageHash is not exactly 32 bytes
//   - privateKey is not exactly 32 bytes
//   - privateKey is invalid (zero or >= curve order)
func Sign(messageHash, privateKey []byte) (*Signature, error) {
	var sig p256k1.ECDSASignature
	if err := p256k1.ECDSASign(&sig, messageHash, privateKey); err != nil {
		return nil, err
	}
	return &Signature{sig: sig}, nil
}

// SignCompact creates a compact 64-byte signature.
func SignCompact(messageHash, privateKey []byte) (*CompactSignature, error) {
	var compact CompactSignature
	if err := p256k1.ECDSASignCompact(&compact, messageHash, privateKey); err != nil {
		return nil, err
	}
	return &compact, nil
}

// SignDER creates a DER-encoded signature.
func SignDER(messageHash, privateKey []byte) ([]byte, error) {
	return p256k1.ECDSASignDER(messageHash, privateKey)
}

// SignRecoverable creates a signature with recovery ID.
func SignRecoverable(messageHash, privateKey []byte) (*RecoverableSignature, error) {
	var sig p256k1.ECDSARecoverableSignature
	if err := p256k1.ECDSASignRecoverable(&sig, messageHash, privateKey); err != nil {
		return nil, err
	}
	return &RecoverableSignature{sig: sig}, nil
}

// =============================================================================
// Signature Verification (Domain Service)
// =============================================================================

// Verify verifies an ECDSA signature against a message hash and public key.
//
// Returns true if the signature is valid, false otherwise.
// Rejects high-S signatures (BIP-146 enforcement).
func Verify(sig *Signature, messageHash []byte, pubkey *PublicKey) bool {
	return p256k1.ECDSAVerify(&sig.sig, messageHash, pubkey)
}

// VerifyCompact verifies a compact 64-byte signature.
func VerifyCompact(compact *CompactSignature, messageHash []byte, pubkey *PublicKey) bool {
	return p256k1.ECDSAVerifyCompact(compact, messageHash, pubkey)
}

// VerifyDER verifies a DER-encoded signature.
func VerifyDER(sigDER, messageHash []byte, pubkey *PublicKey) bool {
	return p256k1.ECDSAVerifyDER(sigDER, messageHash, pubkey)
}

// =============================================================================
// Public Key Recovery (Domain Service)
// =============================================================================

// Recover recovers the public key from a recoverable signature.
func Recover(sig *RecoverableSignature, messageHash []byte) (*PublicKey, error) {
	var pubkey PublicKey
	if err := p256k1.ECDSARecover(&pubkey, &sig.sig, messageHash); err != nil {
		return nil, err
	}
	return &pubkey, nil
}

// RecoverCompact recovers the public key from a compact signature with recovery ID.
func RecoverCompact(sig64 []byte, recid int, messageHash []byte) (*PublicKey, error) {
	var pubkey PublicKey
	if err := p256k1.ECDSARecoverCompact(&pubkey, sig64, recid, messageHash); err != nil {
		return nil, err
	}
	return &pubkey, nil
}

// =============================================================================
// Signature Serialization (Value Object Methods)
// =============================================================================

// ToCompact converts a signature to compact 64-byte format.
func (s *Signature) ToCompact() *CompactSignature {
	return s.sig.ToCompact()
}

// FromCompact parses a compact 64-byte signature.
func FromCompact(compact *CompactSignature) (*Signature, error) {
	var sig p256k1.ECDSASignature
	if err := sig.FromCompact(compact); err != nil {
		return nil, err
	}
	return &Signature{sig: sig}, nil
}

// SerializeDER serializes the signature in DER format.
func (s *Signature) SerializeDER() []byte {
	return s.sig.SerializeDER()
}

// ParseDER parses a DER-encoded signature.
func ParseDER(der []byte) (*Signature, error) {
	var sig p256k1.ECDSASignature
	if err := sig.ParseDER(der); err != nil {
		return nil, err
	}
	return &Signature{sig: sig}, nil
}

// R returns the R component of the signature as a 32-byte slice.
func (s *Signature) R() []byte {
	return s.sig.GetR()
}

// S returns the S component of the signature as a 32-byte slice.
func (s *Signature) S() []byte {
	return s.sig.GetS()
}

// IsLowS returns true if the S value is in the lower half of the curve order.
func (s *Signature) IsLowS() bool {
	return s.sig.IsLowS()
}

// NormalizeLowS ensures the S value is in the lower half of the curve order.
func (s *Signature) NormalizeLowS() {
	s.sig.NormalizeLowS()
}

// =============================================================================
// Recoverable Signature Methods
// =============================================================================

// ToCompact returns the 64-byte compact signature and recovery ID.
func (s *RecoverableSignature) ToCompact() ([]byte, int) {
	return s.sig.ToCompact()
}

// FromCompactRecoverable parses a compact signature with recovery ID.
func FromCompactRecoverable(compact []byte, recid int) (*RecoverableSignature, error) {
	if len(compact) != 64 {
		return nil, errors.New("compact signature must be 64 bytes")
	}
	var sig p256k1.ECDSARecoverableSignature
	if err := sig.FromCompact(compact, recid); err != nil {
		return nil, err
	}
	return &RecoverableSignature{sig: sig}, nil
}
