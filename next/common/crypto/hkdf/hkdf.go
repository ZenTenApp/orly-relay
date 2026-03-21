package hkdf

import "common/crypto/hmac"

// Extract computes HKDF-Extract: PRK = HMAC-SHA256(salt, ikm).
func Extract(salt, ikm []byte) [32]byte {
	if len(salt) == 0 {
		salt = make([]byte, 32)
	}
	return hmac.Sum(salt, ikm)
}

// Expand computes HKDF-Expand using HMAC-SHA256.
// Returns length bytes of output keying material.
func Expand(prk, info []byte, length int) []byte {
	out := make([]byte, 0, length)
	var prev []byte
	counter := byte(1)
	for len(out) < length {
		// T(i) = HMAC(PRK, T(i-1) || info || counter)
		input := make([]byte, len(prev)+len(info)+1)
		copy(input, prev)
		copy(input[len(prev):], info)
		input[len(input)-1] = counter
		t := hmac.Sum(prk[:], input)
		prev = t[:]
		out = append(out, t[:]...)
		counter++
	}
	return out[:length]
}
