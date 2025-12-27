// Package wireguard provides an embedded WireGuard VPN server for secure
// NIP-46 bunker access. It uses wireguard-go with gVisor netstack for
// userspace networking (no root required).
package wireguard

import (
	"crypto/rand"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair generates a new Curve25519 keypair for WireGuard.
// Returns the private key and public key as 32-byte slices.
func GenerateKeyPair() (privateKey, publicKey []byte, err error) {
	privateKey = make([]byte, 32)
	if _, err = rand.Read(privateKey); err != nil {
		return nil, nil, err
	}

	// Curve25519 clamping (required by WireGuard spec)
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key from private key
	publicKey = make([]byte, 32)
	curve25519.ScalarBaseMult((*[32]byte)(publicKey), (*[32]byte)(privateKey))

	return privateKey, publicKey, nil
}

// DerivePublicKey derives the public key from a private key.
func DerivePublicKey(privateKey []byte) (publicKey []byte, err error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	publicKey = make([]byte, 32)
	curve25519.ScalarBaseMult((*[32]byte)(publicKey), (*[32]byte)(privateKey))

	return publicKey, nil
}
