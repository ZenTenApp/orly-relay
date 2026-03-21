package nip44

import (
	"common/crypto/chacha20"
	"common/crypto/hkdf"
	"common/crypto/hmac"
	"common/crypto/secp256k1"
	"common/helpers"
)

// ConversationKey derives the NIP-44 conversation key from ECDH shared secret.
func ConversationKey(seckey, pubkey [32]byte) ([32]byte, bool) {
	shared, ok := secp256k1.ECDH(seckey, pubkey)
	if !ok {
		return [32]byte{}, false
	}
	return hkdf.Extract([]byte("nip44-v2"), shared[:]), true
}

// Encrypt encrypts plaintext using NIP-44 v2.
// nonce must be 32 random bytes.
// Returns base64-encoded payload.
func Encrypt(plaintext string, conversationKey [32]byte, nonce [32]byte) string {
	// Derive message keys.
	keys := hkdf.Expand(conversationKey[:], nonce[:], 76)
	var chachaKey [32]byte
	var chaChaNonce [12]byte
	copy(chachaKey[:], keys[:32])
	copy(chaChaNonce[:], keys[32:44])
	hmacKey := keys[44:76]

	// Pad plaintext.
	padded := pad([]byte(plaintext))

	// Encrypt.
	ciphertext := chacha20.XOR(chachaKey, chaChaNonce, padded)

	// MAC = HMAC-SHA256(hmacKey, nonce || ciphertext).
	macInput := make([]byte, 32+len(ciphertext))
	copy(macInput, nonce[:])
	copy(macInput[32:], ciphertext)
	mac := hmac.Sum(hmacKey, macInput)

	// Payload = version(1) || nonce(32) || ciphertext || mac(32).
	payload := make([]byte, 0, 1+32+len(ciphertext)+32)
	payload = append(payload, 2) // version
	payload = append(payload, nonce[:]...)
	payload = append(payload, ciphertext...)
	payload = append(payload, mac[:]...)
	return helpers.Base64Encode(payload)
}

// Decrypt decrypts a NIP-44 v2 base64 payload.
// Returns plaintext and true on success.
func Decrypt(b64payload string, conversationKey [32]byte) (string, bool) {
	raw := helpers.Base64Decode(b64payload)
	if len(raw) < 99 { // 1 + 32 + 32 + 2 + 32 minimum
		return "", false
	}
	if raw[0] != 2 {
		return "", false
	}

	nonce := raw[1:33]
	ciphertext := raw[33 : len(raw)-32]
	mac := raw[len(raw)-32:]

	// Derive message keys.
	keys := hkdf.Expand(conversationKey[:], nonce, 76)
	var chachaKey [32]byte
	var chaChaNonce [12]byte
	copy(chachaKey[:], keys[:32])
	copy(chaChaNonce[:], keys[32:44])
	hmacKey := keys[44:76]

	// Verify MAC.
	macInput := make([]byte, 32+len(ciphertext))
	copy(macInput, nonce)
	copy(macInput[32:], ciphertext)
	expectedMac := hmac.Sum(hmacKey, macInput)
	ok := true
	for i := range 32 {
		if mac[i] != expectedMac[i] {
			ok = false
		}
	}
	if !ok {
		return "", false
	}

	// Decrypt.
	padded := chacha20.XOR(chachaKey, chaChaNonce, ciphertext)
	return unpad(padded)
}

func calcPadding(length int) int {
	if length <= 32 {
		return 32
	}
	// Next power of 2.
	bits := 0
	v := length - 1
	for v > 0 {
		v >>= 1
		bits++
	}
	nextPow := 1 << bits
	chunk := max(nextPow/8, 32)
	return chunk * ((length-1)/chunk + 1)
}

func pad(plaintext []byte) []byte {
	n := len(plaintext)
	paddedLen := calcPadding(n)
	out := make([]byte, 2+paddedLen)
	out[0] = byte(n >> 8)
	out[1] = byte(n)
	copy(out[2:], plaintext)
	return out
}

func unpad(padded []byte) (string, bool) {
	if len(padded) < 2 {
		return "", false
	}
	n := int(padded[0])<<8 | int(padded[1])
	if n < 1 || n > len(padded)-2 {
		return "", false
	}
	// Verify zero padding.
	for i := 2 + n; i < len(padded); i++ {
		if padded[i] != 0 {
			return "", false
		}
	}
	return string(padded[2 : 2+n]), true
}
