package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// EncryptAttachment encrypts data with a random ChaCha20-Poly1305 key.
// Returns (ciphertext, keyHex). The key is hex-encoded for use in fragment URLs.
func EncryptAttachment(plaintext []byte) (ciphertext []byte, keyHex string, err error) {
	// Generate random 256-bit key
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, "", fmt.Errorf("create AEAD: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: nonce || ciphertext (nonce prepended for decryption)
	encrypted := aead.Seal(nonce, nonce, plaintext, nil)

	return encrypted, hex.EncodeToString(key), nil
}

// DecryptAttachment decrypts data that was encrypted by EncryptAttachment.
// keyHex is the hex-encoded key from the fragment URL.
func DecryptAttachment(ciphertext []byte, keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key length: %d (expected %d)", len(key), chacha20poly1305.KeySize)
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Split nonce and ciphertext
	nonce := ciphertext[:aead.NonceSize()]
	ct := ciphertext[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
