package hmac

import "common/crypto/sha256"

const blockSize = 64

// Sum computes HMAC-SHA256(key, message).
func Sum(key, message []byte) [32]byte {
	// If key > blockSize, hash it.
	var k [blockSize]byte
	if len(key) > blockSize {
		h := sha256.Sum(key)
		copy(k[:], h[:])
	} else {
		copy(k[:], key)
	}

	var ipad, opad [blockSize]byte
	for i := 0; i < blockSize; i++ {
		ipad[i] = k[i] ^ 0x36
		opad[i] = k[i] ^ 0x5c
	}

	// inner = SHA256(ipad || message)
	inner := make([]byte, blockSize+len(message))
	copy(inner, ipad[:])
	copy(inner[blockSize:], message)
	innerHash := sha256.Sum(inner)

	// outer = SHA256(opad || inner_hash)
	var outer [blockSize + 32]byte
	copy(outer[:blockSize], opad[:])
	copy(outer[blockSize:], innerHash[:])
	return sha256.Sum(outer[:])
}
