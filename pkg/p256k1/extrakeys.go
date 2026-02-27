package p256k1

import (
	"errors"
	"unsafe"
)

// XOnlyPubkey represents an x-only public key (32 bytes, just X coordinate)
// Following BIP-340 specification
type XOnlyPubkey struct {
	data [32]byte
}

// KeyPair represents a keypair consisting of a secret key and public key
// Used for Schnorr signatures
type KeyPair struct {
	seckey [32]byte
	pubkey PublicKey
}

// XOnlyPubkeyParse parses a 32-byte sequence into an x-only public key
func XOnlyPubkeyParse(input32 []byte) (*XOnlyPubkey, error) {
	if len(input32) != 32 {
		return nil, errors.New("input must be 32 bytes")
	}

	// Create a point from X coordinate
	var x FieldElement
	if err := x.setB32(input32); err != nil {
		return nil, errors.New("invalid X coordinate")
	}

	// Try to recover Y coordinate (check if point is on curve)
	var point GroupElementAffine
	if !point.setXOVar(&x, false) {
		// Try with odd Y
		if !point.setXOVar(&x, true) {
			return nil, errors.New("X coordinate does not correspond to a valid point")
		}
	}

	// Verify point is valid
	if !point.isValid() {
		return nil, errors.New("invalid point")
	}

	// Create x-only pubkey (just X coordinate)
	var xonly XOnlyPubkey
	copy(xonly.data[:], input32)
	return &xonly, nil
}

// Serialize serializes an x-only public key to 32 bytes
func (xonly *XOnlyPubkey) Serialize() [32]byte {
	return xonly.data
}

// XOnlyPubkeyFromPubkey converts a PublicKey to an XOnlyPubkey
// Returns the x-only pubkey and parity (1 if Y was odd, 0 if even)
func XOnlyPubkeyFromPubkey(pubkey *PublicKey) (*XOnlyPubkey, int, error) {
	if pubkey == nil {
		return nil, 0, errors.New("pubkey cannot be nil")
	}

	// Load public key
	var pt GroupElementAffine
	pt.fromBytes(pubkey.data[:])
	if pt.isInfinity() {
		return nil, 0, errors.New("invalid public key")
	}

	// Normalize Y coordinate
	pt.y.normalize()

	// Check parity
	parity := 0
	if pt.y.isOdd() {
		parity = 1
		// Negate point if Y is odd to get even Y
		pt.negate(&pt)
	}

	// Extract X coordinate
	var xonly XOnlyPubkey
	pt.x.normalize()
	pt.x.getB32(xonly.data[:])

	return &xonly, parity, nil
}

// XOnlyPubkeyCmp compares two x-only public keys lexicographically
// Returns: <0 if xonly1 < xonly2, >0 if xonly1 > xonly2, 0 if equal
func XOnlyPubkeyCmp(xonly1, xonly2 *XOnlyPubkey) int {
	if xonly1 == nil || xonly2 == nil {
		panic("xonly pubkey cannot be nil")
	}

	for i := 31; i >= 0; i-- {
		if xonly1.data[i] < xonly2.data[i] {
			return -1
		}
		if xonly1.data[i] > xonly2.data[i] {
			return 1
		}
	}
	return 0
}

// KeyPairCreate creates a keypair from a secret key
func KeyPairCreate(seckey []byte) (*KeyPair, error) {
	if len(seckey) != 32 {
		return nil, errors.New("secret key must be 32 bytes")
	}

	if !ECSeckeyVerify(seckey) {
		return nil, errors.New("invalid secret key")
	}

	// Create public key
	var pubkey PublicKey
	if err := ECPubkeyCreate(&pubkey, seckey); err != nil {
		return nil, err
	}

	kp := &KeyPair{}
	copy(kp.seckey[:], seckey)
	kp.pubkey = pubkey

	return kp, nil
}

// KeyPairGenerate generates a new random keypair
func KeyPairGenerate() (*KeyPair, error) {
	seckey, pubkey, err := ECKeyPairGenerate()
	if err != nil {
		return nil, err
	}

	kp := &KeyPair{}
	copy(kp.seckey[:], seckey)
	kp.pubkey = *pubkey

	return kp, nil
}

// Seckey returns the secret key
func (kp *KeyPair) Seckey() []byte {
	return kp.seckey[:]
}

// Pubkey returns the public key
func (kp *KeyPair) Pubkey() *PublicKey {
	return &kp.pubkey
}

// XOnlyPubkey returns the x-only public key
func (kp *KeyPair) XOnlyPubkey() (*XOnlyPubkey, error) {
	xonly, _, err := XOnlyPubkeyFromPubkey(&kp.pubkey)
	return xonly, err
}

// Clear clears the keypair to prevent leaking sensitive information
func (kp *KeyPair) Clear() {
	memclear(unsafe.Pointer(&kp.seckey[0]), 32)
	kp.pubkey.data = [64]byte{}
}
