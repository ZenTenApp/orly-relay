// Package types provides fixed-size cryptographic types for Nostr.
//
// These types use fixed-size arrays instead of slices, which provides:
//   - Stack allocation (no heap escape for copies)
//   - Implicit copy-on-assignment (safe concurrent use)
//   - Type safety (compiler enforces correct sizes)
//
// Migration: Existing code using []byte continues to work via Bytes() methods.
// New code should prefer the fixed types for better performance and safety.
package types

import (
	"encoding/hex"
)

// Size constants for Nostr cryptographic values
const (
	EventIDSize   = 32 // SHA256 hash
	PubkeySize    = 32 // secp256k1 x-coordinate (32 bytes)
	SignatureSize = 64 // Schnorr signature
	SecretKeySize = 32 // secp256k1 secret key
)

// EventID is a 32-byte SHA256 hash of an event's canonical serialization.
// Stack-allocated, copied on assignment.
type EventID [EventIDSize]byte

// Pubkey is a 32-byte secp256k1 public key (x-coordinate only, y is derived).
// Stack-allocated, copied on assignment.
type Pubkey [PubkeySize]byte

// Signature is a 64-byte Schnorr signature.
// Stack-allocated, copied on assignment.
type Signature [SignatureSize]byte

// SecretKey is a 32-byte secp256k1 secret key.
// Stack-allocated, copied on assignment.
type SecretKey [SecretKeySize]byte

// --- EventID methods ---

// Bytes returns a slice view of the EventID.
// WARNING: The returned slice shares memory with the EventID.
// Use Copy() if you need an independent slice.
// Pointer receiver ensures the slice references the original, not a copy.
func (id *EventID) Bytes() []byte { return id[:] }

// Copy returns an independent heap-allocated copy as a slice.
// Use when you need to pass to APIs that may retain or mutate the slice.
func (id EventID) Copy() []byte {
	b := make([]byte, EventIDSize)
	copy(b, id[:])
	return b
}

// Hex returns the lowercase hex encoding of the EventID.
func (id EventID) Hex() string { return hex.EncodeToString(id[:]) }

// IsZero returns true if the EventID is all zeros (uninitialized).
func (id EventID) IsZero() bool { return id == EventID{} }

// EventIDFromBytes creates an EventID from a byte slice.
// If src is shorter than 32 bytes, remaining bytes are zero.
// If src is longer, extra bytes are ignored.
func EventIDFromBytes(src []byte) (id EventID) {
	copy(id[:], src)
	return
}

// EventIDFromHex creates an EventID from a hex string.
// Returns zero EventID and error if hex is invalid.
func EventIDFromHex(s string) (id EventID, err error) {
	var b []byte
	b, err = hex.DecodeString(s)
	if err != nil {
		return
	}
	copy(id[:], b)
	return
}

// --- Pubkey methods ---

// Bytes returns a slice view of the Pubkey.
// WARNING: The returned slice shares memory with the Pubkey.
// Pointer receiver ensures the slice references the original, not a copy.
func (pk *Pubkey) Bytes() []byte { return pk[:] }

// Copy returns an independent heap-allocated copy as a slice.
func (pk Pubkey) Copy() []byte {
	b := make([]byte, PubkeySize)
	copy(b, pk[:])
	return b
}

// Hex returns the lowercase hex encoding of the Pubkey.
func (pk Pubkey) Hex() string { return hex.EncodeToString(pk[:]) }

// IsZero returns true if the Pubkey is all zeros (uninitialized).
func (pk Pubkey) IsZero() bool { return pk == Pubkey{} }

// PubkeyFromBytes creates a Pubkey from a byte slice.
func PubkeyFromBytes(src []byte) (pk Pubkey) {
	copy(pk[:], src)
	return
}

// PubkeyFromHex creates a Pubkey from a hex string.
func PubkeyFromHex(s string) (pk Pubkey, err error) {
	var b []byte
	b, err = hex.DecodeString(s)
	if err != nil {
		return
	}
	copy(pk[:], b)
	return
}

// --- Signature methods ---

// Bytes returns a slice view of the Signature.
// WARNING: The returned slice shares memory with the Signature.
// Pointer receiver ensures the slice references the original, not a copy.
func (sig *Signature) Bytes() []byte { return sig[:] }

// Copy returns an independent heap-allocated copy as a slice.
func (sig Signature) Copy() []byte {
	b := make([]byte, SignatureSize)
	copy(b, sig[:])
	return b
}

// Hex returns the lowercase hex encoding of the Signature.
func (sig Signature) Hex() string { return hex.EncodeToString(sig[:]) }

// IsZero returns true if the Signature is all zeros (uninitialized).
func (sig Signature) IsZero() bool { return sig == Signature{} }

// SignatureFromBytes creates a Signature from a byte slice.
func SignatureFromBytes(src []byte) (sig Signature) {
	copy(sig[:], src)
	return
}

// SignatureFromHex creates a Signature from a hex string.
func SignatureFromHex(s string) (sig Signature, err error) {
	var b []byte
	b, err = hex.DecodeString(s)
	if err != nil {
		return
	}
	copy(sig[:], b)
	return
}

// --- SecretKey methods ---

// Bytes returns a slice view of the SecretKey.
// WARNING: The returned slice shares memory with the SecretKey.
// Pointer receiver ensures the slice references the original, not a copy.
func (sk *SecretKey) Bytes() []byte { return sk[:] }

// Copy returns an independent heap-allocated copy as a slice.
func (sk SecretKey) Copy() []byte {
	b := make([]byte, SecretKeySize)
	copy(b, sk[:])
	return b
}

// Hex returns the lowercase hex encoding of the SecretKey.
// WARNING: Be careful with secret key hex representations.
func (sk SecretKey) Hex() string { return hex.EncodeToString(sk[:]) }

// IsZero returns true if the SecretKey is all zeros (uninitialized).
func (sk SecretKey) IsZero() bool { return sk == SecretKey{} }

// SecretKeyFromBytes creates a SecretKey from a byte slice.
func SecretKeyFromBytes(src []byte) (sk SecretKey) {
	copy(sk[:], src)
	return
}

// SecretKeyFromHex creates a SecretKey from a hex string.
func SecretKeyFromHex(s string) (sk SecretKey, err error) {
	var b []byte
	b, err = hex.DecodeString(s)
	if err != nil {
		return
	}
	copy(sk[:], b)
	return
}

// Zero clears the secret key memory.
// Call this when done with a secret key to reduce exposure window.
func (sk *SecretKey) Zero() {
	for i := range sk {
		sk[i] = 0
	}
}
