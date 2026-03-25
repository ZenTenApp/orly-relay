//go:build js || wasm || tinygo || wasm32

package signer

import (
	"errors"

	"next.orly.dev/pkg/p256k1/exchange"
	"next.orly.dev/pkg/p256k1/keys"
	"next.orly.dev/pkg/p256k1/schnorr"
)

// P256K1Signer implements the I and Gen interfaces using the p256k1 domain packages (WASM build)
type P256K1Signer struct {
	keypair   *keys.KeyPair
	xonlyPub  *schnorr.XOnlyPubkey
	hasSecret bool
	sigBuf    []byte
	ecdhBuf   []byte
}

func NewP256K1Signer() *P256K1Signer {
	return &P256K1Signer{hasSecret: false}
}

func (s *P256K1Signer) Generate() error {
	kp, err := keys.Generate()
	if err != nil {
		return err
	}
	xonly, parity, err := schnorr.XOnlyFromPubkey(kp.Pubkey())
	if err != nil {
		return err
	}
	if parity == 1 {
		seckey, err := keys.NegatePrivate(kp.Seckey())
		if err != nil {
			return errors.New("failed to negate secret key")
		}
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

func (s *P256K1Signer) InitSec(sec []byte) error {
	if len(sec) != 32 {
		return errors.New("secret key must be 32 bytes")
	}
	kp, err := keys.Create(sec)
	if err != nil {
		return err
	}
	xonly, parity, err := schnorr.XOnlyFromPubkey(kp.Pubkey())
	if err != nil {
		return err
	}
	if parity == 1 {
		seckey, err := keys.NegatePrivate(kp.Seckey())
		if err != nil {
			return errors.New("failed to negate secret key")
		}
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

func (s *P256K1Signer) Sec() []byte {
	if !s.hasSecret || s.keypair == nil {
		return nil
	}
	return s.keypair.Seckey()
}

func (s *P256K1Signer) Pub() []byte {
	if s.xonlyPub == nil {
		return nil
	}
	serialized := s.xonlyPub.Serialize()
	return serialized[:]
}

func (s *P256K1Signer) Sign(msg []byte) (sig []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for signing")
	}
	if len(msg) != 32 {
		return nil, errors.New("message must be 32 bytes")
	}
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

func (s *P256K1Signer) Zero() {
	if s.keypair != nil {
		s.keypair.Clear()
		s.keypair = nil
	}
	s.hasSecret = false
	s.xonlyPub = nil
}

func (s *P256K1Signer) ECDH(pub []byte) (secret []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for ECDH")
	}
	if len(pub) != 32 {
		return nil, errors.New("public key must be 32 bytes")
	}
	var compressedPub [33]byte
	compressedPub[0] = 0x02
	copy(compressedPub[1:], pub)
	pubkey, err := keys.ParsePublic(compressedPub[:])
	if err != nil {
		return nil, err
	}
	if cap(s.ecdhBuf) < 32 {
		s.ecdhBuf = make([]byte, 32)
	} else {
		s.ecdhBuf = s.ecdhBuf[:32]
	}
	if err := exchange.SharedSecretRaw(s.ecdhBuf, pubkey, s.keypair.Seckey()); err != nil {
		return nil, err
	}
	return s.ecdhBuf, nil
}

func (s *P256K1Signer) ECDHRaw(pub []byte) (sharedX []byte, err error) {
	if !s.hasSecret || s.keypair == nil {
		return nil, errors.New("no secret key available for ECDH")
	}
	var compressedPub [33]byte
	if len(pub) == 32 {
		compressedPub[0] = 0x02
		copy(compressedPub[1:], pub)
	} else if len(pub) == 33 {
		copy(compressedPub[:], pub)
	} else {
		return nil, errors.New("public key must be 32 bytes (x-only) or 33 bytes (compressed)")
	}
	pubkey, err := keys.ParsePublic(compressedPub[:])
	if err != nil {
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
	if cap(s.ecdhBuf) < 32 {
		s.ecdhBuf = make([]byte, 32)
	} else {
		s.ecdhBuf = s.ecdhBuf[:32]
	}
	if err := exchange.XOnlySharedSecretRaw(s.ecdhBuf, pubkey, s.keypair.Seckey()); err != nil {
		return nil, err
	}
	return s.ecdhBuf, nil
}
