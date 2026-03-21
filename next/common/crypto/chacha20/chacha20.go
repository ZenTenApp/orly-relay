package chacha20

// XOR encrypts/decrypts data using ChaCha20.
// key: 32 bytes, nonce: 12 bytes.
func XOR(key [32]byte, nonce [12]byte, data []byte) []byte {
	out := make([]byte, len(data))
	var state [16]uint32
	// "expand 32-byte k"
	state[0] = 0x61707865
	state[1] = 0x3320646e
	state[2] = 0x79622d32
	state[3] = 0x6b206574
	state[4] = le32(key[0:4])
	state[5] = le32(key[4:8])
	state[6] = le32(key[8:12])
	state[7] = le32(key[12:16])
	state[8] = le32(key[16:20])
	state[9] = le32(key[20:24])
	state[10] = le32(key[24:28])
	state[11] = le32(key[28:32])
	// counter = 0
	state[12] = 0
	state[13] = le32(nonce[0:4])
	state[14] = le32(nonce[4:8])
	state[15] = le32(nonce[8:12])

	pos := 0
	for pos < len(data) {
		var block [64]byte
		blockFn(&state, &block)
		state[12]++ // increment counter
		n := min(len(data)-pos, 64)
		for i := range n {
			out[pos+i] = data[pos+i] ^ block[i]
		}
		pos += n
	}
	return out
}

func blockFn(state *[16]uint32, out *[64]byte) {
	var s [16]uint32
	copy(s[:], state[:])

	for range 10 {
		// column rounds
		qr(&s, 0, 4, 8, 12)
		qr(&s, 1, 5, 9, 13)
		qr(&s, 2, 6, 10, 14)
		qr(&s, 3, 7, 11, 15)
		// diagonal rounds
		qr(&s, 0, 5, 10, 15)
		qr(&s, 1, 6, 11, 12)
		qr(&s, 2, 7, 8, 13)
		qr(&s, 3, 4, 9, 14)
	}

	for i := range 16 {
		v := s[i] + state[i]
		out[i*4] = byte(v)
		out[i*4+1] = byte(v >> 8)
		out[i*4+2] = byte(v >> 16)
		out[i*4+3] = byte(v >> 24)
	}
}

func qr(s *[16]uint32, a, b, c, d int) {
	s[a] += s[b]
	s[d] ^= s[a]
	s[d] = (s[d] << 16) | (s[d] >> 16)
	s[c] += s[d]
	s[b] ^= s[c]
	s[b] = (s[b] << 12) | (s[b] >> 20)
	s[a] += s[b]
	s[d] ^= s[a]
	s[d] = (s[d] << 8) | (s[d] >> 24)
	s[c] += s[d]
	s[b] ^= s[c]
	s[b] = (s[b] << 7) | (s[b] >> 25)
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
