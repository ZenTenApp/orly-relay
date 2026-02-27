//go:build !js && !wasm && !tinygo && !wasm32

package signer

import (
	"errors"

	"next.orly.dev/pkg/p256k1/exchange"
	"next.orly.dev/pkg/p256k1/keys"
	"next.orly.dev/pkg/p256k1/schnorr"
)

// P256K1Signer implements the I and Gen interfaces using the p256k1 domain packages
type P256K1Signer struct {
	keypair   *keys.KeyPair
	xonlyPub  *schnorr.XOnlyPubkey
	hasSecret bool   // Whether we have the secret key (if false, can only verify)
	sigBuf    []byte // Reusable buffer for signatures to avoid allocations
	ecdhBuf   []byte // Reusable buffer for ECDH shared secrets
}

// NewP256K1Signer creates a new P256K1Signer instance
func NewP256K1Signer() *P256K1Signer {
	return &P256K1Signer{
		hasSecret: false,
	}
}

// Generate creates a fresh new key pair from system entropy, and ensures it is even (so ECDH works)
func (s *P256K1Signer) Generate() error {
	kp, err := keys.Generate()
	if err != nil {
		return err
	}

	// Ensure even Y coordinate for ECDH compatibility
	// Get x-only pubkey and check parity
	xonly, parity, err := schnorr.XOnlyFromPubkey(kp.Pubkey())
	if err != nil {
		return err
	}

	// If parity is 1 (odd Y), negate the secret key
	if parity == 1 {
		seckey, err := keys.NegatePrivate(kp.Seckey())
		if err != nil {
			return errors.New("failed to negate secret key")
		}
		// Recreate keypair with negated secret key
		kp, err = keys.Create(seckey)
		if err != nil {
			return err
		}
		// Get x-only pubkey again (should be even now)
		xonly, _, err = schnorr.XOnlyFromPubkey(kp.Pubkey())
		if err != nil {
			return err
		}
	}

	s.keypair = kp
	s.xonlyPub = xonly
	s.hasSecret = true

	return nil
}

// InitSec initialises the secret (signing) key from the raw bytes, and also derives the public key
func (s *P256K1Signer) InitSec(sec []byte) error {
	if len(sec) != 32 {
		return errors.New("secret key must be 32 bytes")
	}

	kp, err := keys.Create(sec)
	if err != nil {
		return err
	}

	// Ensure even Y coordinate for ECDH compatibility
	xonly, parity, err := schnorr.XOnlyFromPubkey(kp.Pubkey())
	if err != nil {
		return err
	}

	// If parity is 1 (odd Y), negate the secret key and recompute public key
	// With windowed optimization, this is now much faster than before
	if parity == 1 {
		seckey, err := keys.NegatePrivate(kp.Seckey())
		if err != nil {
			return errors.New("failed to negate secret key")
		}
		// Recreate keypair with negated secret key
		// This is now optimized with windowed precomputed tables
		kp, err = keys.Create(seckey)
		if err != nil {
			return err
		}
		xonly, _, err = schnorr.XOnlyFromPubkey(kp.Pubkey())
		if err != nil {
			return err
		}
	}

	s.keypair = kp
	s.xonlyPub = xonly
	s.hasSecret = true

	return nil
}

// InitPub initializes the public (verification) key from raw bytes, this is expected to be an x-only 32 byte pubkey
func (s *P256K1Signer) InitPub(pub []byte) error {
	if len(pub) != 32 {
		return errors.New("public key must be 32 bytes")
	}

	xonly, err := schnorr.ParseXOnlyPubkey(pub)
	if err != nil {
		return err
	}

	s.xonlyPub = xonly
	s.keypair = nil
	s.hasSecret = false

	return nil
}

// Sec returns the secret key bytes
func (s *P256K1Signer) Sec() []byte {
	if !s.hasSecret || s.keypair == nil {
		return nil
	}
	return s.keypair.Seckey()
}

// Pub returns the public key bytes (x-only schnorr pubkey)
// The returned slice is backed by an internal buffer that may be
// reused on subsequent calls. Copy if you need to retain it.
func (s *P256K1Signer) Pub() []byte {
	if s.xonlyPub == nil {
		return nil
	}
	serialized := s.xonlyPub.Serialize()
	return serialized[:]
}

// Sign creates a signature using the stored secret key
// The returned slice is backed by an internal buffer that may be
// reused on subsequent calls. Copy if you need to retain it.
func (s *P256K1Signer) Sign(msg []byte) (sig []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for signing")
	}

	if len(msg) != 32 {
		return nil, errors.New("message must be 32 bytes")
	}

	// Pre-allocate buffer to reuse across calls
	if cap(s.sigBuf) < 64 {
		s.sigBuf = make([]byte, 64)
	} else {
		s.sigBuf = s.sigBuf[:64]
	}

	if err := schnorr.SignRaw(s.sigBuf, msg, s.keypair, nil); err != nil {
		return nil, err
	}

	return s.sigBuf, nil
}

// Verify checks a message hash and signature match the stored public key
func (s *P256K1Signer) Verify(msg, sig []byte) (valid bool, err error) {
	if s.xonlyPub == nil {
		return false, errors.New("no public key available for verification")
	}

	if len(msg) != 32 {
		return false, errors.New("message must be 32 bytes")
	}

	if len(sig) != 64 {
		return false, errors.New("signature must be 64 bytes")
	}

	valid = schnorr.VerifyRaw(sig, msg, s.xonlyPub)
	return valid, nil
}

// Zero wipes the secret key to prevent memory leaks
func (s *P256K1Signer) Zero() {
	if s.keypair != nil {
		s.keypair.Clear()
		s.keypair = nil
	}
	s.hasSecret = false
	// Note: x-only pubkey doesn't contain sensitive data, but we can clear it too
	s.xonlyPub = nil
}

// ECDH returns a shared secret derived using Elliptic Curve Diffie-Hellman on the I secret and provided pubkey
// The returned slice is backed by an internal buffer that may be
// reused on subsequent calls. Copy if you need to retain it.
func (s *P256K1Signer) ECDH(pub []byte) (secret []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for ECDH")
	}

	if len(pub) != 32 {
		return nil, errors.New("public key must be 32 bytes")
	}

	// Convert x-only pubkey (32 bytes) to compressed public key (33 bytes) with even Y
	var compressedPub [33]byte
	compressedPub[0] = 0x02 // Even Y
	copy(compressedPub[1:], pub)

	// Parse the compressed public key
	pubkey, err := keys.ParsePublic(compressedPub[:])
	if err != nil {
		return nil, err
	}

	// Pre-allocate buffer to reuse across calls
	if cap(s.ecdhBuf) < 32 {
		s.ecdhBuf = make([]byte, 32)
	} else {
		s.ecdhBuf = s.ecdhBuf[:32]
	}

	// Compute ECDH shared secret using standard ECDH (hashes the point)
	if err := exchange.SharedSecretRaw(s.ecdhBuf, pubkey, s.keypair.Seckey()); err != nil {
		return nil, err
	}

	return s.ecdhBuf, nil
}

// ECDHRaw returns the raw shared secret point (x-coordinate only, 32 bytes) without hashing.
// This is needed for protocols like NIP-44 that do their own key derivation.
// The pub parameter can be either:
// - 32 bytes (x-only): will be converted to compressed format with 0x02 prefix
// - 33 bytes (compressed): will be used as-is
func (s *P256K1Signer) ECDHRaw(pub []byte) (sharedX []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for ECDH")
	}

	var compressedPub [33]byte
	if len(pub) == 32 {
		// X-only format - convert to compressed with even Y
		compressedPub[0] = 0x02
		copy(compressedPub[1:], pub)
	} else if len(pub) == 33 {
		// Already compressed
		copy(compressedPub[:], pub)
	} else {
		return nil, errors.New("public key must be 32 bytes (x-only) or 33 bytes (compressed)")
	}

	// Parse the compressed public key
	pubkey, err := keys.ParsePublic(compressedPub[:])
	if err != nil {
		// If x-only with even Y failed, try odd Y
		if len(pub) == 32 {
			compressedPub[0] = 0x03
			pubkey, err = keys.ParsePublic(compressedPub[:])
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Pre-allocate buffer to reuse across calls
	if cap(s.ecdhBuf) < 32 {
		s.ecdhBuf = make([]byte, 32)
	} else {
		s.ecdhBuf = s.ecdhBuf[:32]
	}

	// Compute raw ECDH (x-coordinate only, no hashing)
	if err := exchange.XOnlySharedSecretRaw(s.ecdhBuf, pubkey, s.keypair.Seckey()); err != nil {
		return nil, err
	}

	return s.ecdhBuf, nil
}

// PubCompressed returns the compressed public key (33 bytes with 0x02/0x03 prefix).
// This is needed for ECDH operations like NIP-44.
func (s *P256K1Signer) PubCompressed() (compressed []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("keypair not initialized")
	}

	pubkey := s.keypair.Pubkey()
	buf := keys.SerializePublic(pubkey, keys.Compressed)
	if len(buf) != 33 {
		return nil, errors.New("failed to serialize compressed public key")
	}

	return buf, nil
}

// P256K1Gen implements the Gen interface for nostr BIP-340 key generation
type P256K1Gen struct {
	keypair       *keys.KeyPair
	xonlyPub      *schnorr.XOnlyPubkey
	compressedPub *keys.PublicKey
	pubBuf        []byte // Reusable buffer to avoid allocations in KeyPairBytes
}

// NewP256K1Gen creates a new P256K1Gen instance
func NewP256K1Gen() *P256K1Gen {
	return &P256K1Gen{}
}

// Generate gathers entropy and derives pubkey bytes for matching, this returns the 33 byte compressed form for checking the oddness of the Y coordinate
func (g *P256K1Gen) Generate() (pubBytes []byte, err error) {
	kp, err := keys.Generate()
	if err != nil {
		return nil, err
	}

	g.keypair = kp

	// Get compressed public key (33 bytes)
	pubkey := kp.Pubkey()

	compressed := keys.SerializePublic(pubkey, keys.Compressed)
	if len(compressed) != 33 {
		return nil, errors.New("failed to serialize compressed public key")
	}

	g.compressedPub = pubkey

	return compressed, nil
}

// Negate flips the public key Y coordinate between odd and even
func (g *P256K1Gen) Negate() {
	if g.keypair == nil {
		return
	}

	// Negate the secret key
	seckey, err := keys.NegatePrivate(g.keypair.Seckey())
	if err != nil {
		return
	}

	// Recreate keypair with negated secret key
	kp, err := keys.Create(seckey)
	if err != nil {
		return
	}

	g.keypair = kp

	// Update compressed pubkey
	pubkey := kp.Pubkey()
	g.compressedPub = pubkey

	// Update x-only pubkey
	xonly, err := kp.XOnlyPubkey()
	if err == nil {
		g.xonlyPub = xonly
	}
}

// KeyPairBytes returns the raw bytes of the secret and public key, this returns the 32 byte X-only pubkey
// The returned pubkey slice is backed by an internal buffer that may be
// reused on subsequent calls. Copy if you need to retain it.
func (g *P256K1Gen) KeyPairBytes() (secBytes, cmprPubBytes []byte) {
	if g.keypair == nil {
		return nil, nil
	}

	secBytes = g.keypair.Seckey()

	if g.xonlyPub == nil {
		xonly, err := g.keypair.XOnlyPubkey()
		if err != nil {
			return secBytes, nil
		}
		g.xonlyPub = xonly
	}

	// Pre-allocate buffer to reuse across calls
	if cap(g.pubBuf) < 32 {
		g.pubBuf = make([]byte, 32)
	} else {
		g.pubBuf = g.pubBuf[:32]
	}

	// Copy the serialized public key into our buffer
	serialized := g.xonlyPub.Serialize()
	copy(g.pubBuf, serialized[:])
	cmprPubBytes = g.pubBuf

	return secBytes, cmprPubBytes
}
