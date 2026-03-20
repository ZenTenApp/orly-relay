// Package sha256 implements SHA-256 in pure Go.
// No stdlib crypto dependency — compiles cleanly through tinyjs.
package sha256

const (
	Size      = 32
	BlockSize = 64
)

var k = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5,
	0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
	0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc,
	0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
	0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
	0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3,
	0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5,
	0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
	0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

var init0 = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// Sum returns the SHA-256 checksum of data.
func Sum(data []byte) [Size]byte {
	var d digest
	d.Reset()
	d.Write(data)
	return d.Sum()
}

// SumHex returns the SHA-256 checksum as a lowercase hex string.
func SumHex(data []byte) string {
	h := Sum(data)
	return bytesToHex(h[:])
}

type digest struct {
	h   [8]uint32
	buf [BlockSize]byte
	len int
	tot uint64
}

func (d *digest) Reset() {
	d.h = init0
	d.len = 0
	d.tot = 0
}

func (d *digest) Write(p []byte) {
	d.tot += uint64(len(p))
	// Fill buffer.
	if d.len > 0 {
		n := copy(d.buf[d.len:], p)
		d.len += n
		p = p[n:]
		if d.len == BlockSize {
			block(&d.h, d.buf[:])
			d.len = 0
		}
	}
	// Process full blocks.
	for len(p) >= BlockSize {
		block(&d.h, p[:BlockSize])
		p = p[BlockSize:]
	}
	// Save remainder.
	if len(p) > 0 {
		d.len = copy(d.buf[:], p)
	}
}

func (d *digest) Sum() [Size]byte {
	// Padding.
	var tmp [BlockSize]byte
	tmp[0] = 0x80
	bits := d.tot * 8

	if d.len < 56 {
		d.Write(tmp[:56-d.len])
	} else {
		d.Write(tmp[:64+56-d.len])
	}

	// Length in bits, big-endian.
	var lb [8]byte
	lb[0] = byte(bits >> 56)
	lb[1] = byte(bits >> 48)
	lb[2] = byte(bits >> 40)
	lb[3] = byte(bits >> 32)
	lb[4] = byte(bits >> 24)
	lb[5] = byte(bits >> 16)
	lb[6] = byte(bits >> 8)
	lb[7] = byte(bits)
	d.Write(lb[:])

	var out [Size]byte
	for i := 0; i < 8; i++ {
		out[i*4] = byte(d.h[i] >> 24)
		out[i*4+1] = byte(d.h[i] >> 16)
		out[i*4+2] = byte(d.h[i] >> 8)
		out[i*4+3] = byte(d.h[i])
	}
	return out
}

func block(h *[8]uint32, data []byte) {
	var w [64]uint32

	for i := 0; i < 16; i++ {
		j := i * 4
		w[i] = uint32(data[j])<<24 | uint32(data[j+1])<<16 |
			uint32(data[j+2])<<8 | uint32(data[j+3])
	}

	for i := 16; i < 64; i++ {
		s0 := rotr(w[i-15], 7) ^ rotr(w[i-15], 18) ^ (w[i-15] >> 3)
		s1 := rotr(w[i-2], 17) ^ rotr(w[i-2], 19) ^ (w[i-2] >> 10)
		w[i] = w[i-16] + s0 + w[i-7] + s1
	}

	a, b, c, d, e, f, g, hh := h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7]

	for i := 0; i < 64; i++ {
		S1 := rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)
		ch := (e & f) ^ (^e & g)
		temp1 := hh + S1 + ch + k[i] + w[i]
		S0 := rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)
		maj := (a & b) ^ (a & c) ^ (b & c)
		temp2 := S0 + maj

		hh = g
		g = f
		f = e
		e = d + temp1
		d = c
		c = b
		b = a
		a = temp1 + temp2
	}

	h[0] += a
	h[1] += b
	h[2] += c
	h[3] += d
	h[4] += e
	h[5] += f
	h[6] += g
	h[7] += hh
}

func rotr(x uint32, n uint) uint32 {
	return (x >> n) | (x << (32 - n))
}

const hex = "0123456789abcdef"

func bytesToHex(b []byte) string {
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}
