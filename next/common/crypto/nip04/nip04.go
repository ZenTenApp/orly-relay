package nip04

import (
	"common/crypto/secp256k1"
	"common/helpers"
	"common/jsbridge/subtle"
)

// SharedKey derives the NIP-04 shared secret from ECDH (x-coordinate only).
func SharedKey(seckey, pubkey [32]byte) ([32]byte, bool) {
	return secp256k1.ECDH(seckey, pubkey)
}

// Encrypt encrypts plaintext using NIP-04 (AES-256-CBC).
// sharedKey: 32 bytes from SharedKey.
// Calls fn with the encoded string "base64(ciphertext)?iv=base64(iv)".
func Encrypt(sharedKey [32]byte, plaintext string, fn func(string)) {
	iv := make([]byte, 16)
	subtle.RandomBytes(iv)
	subtle.AESCBCEncrypt(sharedKey[:], iv, []byte(plaintext), func(ct []byte) {
		fn(helpers.Base64Encode(ct) + "?iv=" + helpers.Base64Encode(iv))
	})
}

// Decrypt decrypts a NIP-04 message "base64(ciphertext)?iv=base64(iv)".
// Calls fn with the plaintext.
func Decrypt(sharedKey [32]byte, message string, fn func(string)) {
	// Split on "?iv=".
	ivIdx := -1
	for i := 0; i < len(message)-3; i++ {
		if message[i] == '?' && message[i+1] == 'i' && message[i+2] == 'v' && message[i+3] == '=' {
			ivIdx = i
			break
		}
	}
	if ivIdx < 0 {
		fn("")
		return
	}
	ct := helpers.Base64Decode(message[:ivIdx])
	iv := helpers.Base64Decode(message[ivIdx+4:])
	if ct == nil || iv == nil {
		fn("")
		return
	}
	subtle.AESCBCDecrypt(sharedKey[:], iv, ct, func(pt []byte) {
		fn(string(pt))
	})
}
