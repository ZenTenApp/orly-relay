package app

import (
	"golang.org/x/crypto/curve25519"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
)

// derivePublicKey derives a Curve25519 public key from a private key.
func derivePublicKey(privateKey []byte) ([]byte, error) {
	publicKey := make([]byte, 32)
	curve25519.ScalarBaseMult((*[32]byte)(publicKey), (*[32]byte)(privateKey))
	return publicKey, nil
}

// deriveSecp256k1PublicKey derives a secp256k1 public key from a secret key.
func deriveSecp256k1PublicKey(secretKey []byte) ([]byte, error) {
	return keys.SecretBytesToPubKeyBytes(secretKey)
}
