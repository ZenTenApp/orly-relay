package helpers

// Bech32 encoding/decoding for NIP-19.
// Implements bech32 (BIP-173) without external deps.

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Bech32Encode encodes data with the given human-readable part.
func Bech32Encode(hrp string, data []byte) string {
	values := bytesToBase32(data)
	checksum := bech32Checksum(hrp, values)
	values = append(values, checksum...)

	buf := make([]byte, 0, len(hrp)+1+len(values))
	buf = append(buf, hrp...)
	buf = append(buf, '1')
	for _, v := range values {
		buf = append(buf, bech32Charset[v])
	}
	return string(buf)
}

// Bech32Decode decodes a bech32 string. Returns hrp and data bytes.
func Bech32Decode(s string) (string, []byte) {
	// Find separator.
	pos := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '1' {
			pos = i
			break
		}
	}
	if pos < 1 || pos+7 > len(s) {
		return "", nil
	}

	hrp := s[:pos]
	dataStr := s[pos+1:]

	values := make([]byte, len(dataStr))
	for i := 0; i < len(dataStr); i++ {
		idx := charsetIndex(dataStr[i])
		if idx < 0 {
			return "", nil
		}
		values[i] = byte(idx)
	}

	if !bech32Verify(hrp, values) {
		return "", nil
	}

	// Strip checksum (last 6 chars).
	values = values[:len(values)-6]
	data := base32ToBytes(values)
	return hrp, data
}

// NIP-19 helpers.

// EncodeNpub encodes a 32-byte public key as npub.
func EncodeNpub(pubkey []byte) string {
	return Bech32Encode("npub", pubkey)
}

// EncodeNsec encodes a 32-byte secret key as nsec.
func EncodeNsec(seckey []byte) string {
	return Bech32Encode("nsec", seckey)
}

// EncodeNote encodes a 32-byte event ID as note.
func EncodeNote(eventID []byte) string {
	return Bech32Encode("note", eventID)
}

// DecodeNpub decodes an npub string to 32 bytes.
func DecodeNpub(s string) []byte {
	hrp, data := Bech32Decode(s)
	if hrp != "npub" || len(data) != 32 {
		return nil
	}
	return data
}

// DecodeNsec decodes an nsec string to 32 bytes.
func DecodeNsec(s string) []byte {
	hrp, data := Bech32Decode(s)
	if hrp != "nsec" || len(data) != 32 {
		return nil
	}
	return data
}

// DecodeNote decodes a note string to 32 bytes.
func DecodeNote(s string) []byte {
	hrp, data := Bech32Decode(s)
	if hrp != "note" || len(data) != 32 {
		return nil
	}
	return data
}

// PubkeyShort returns first 8 chars of hex pubkey.
func PubkeyShort(pubkey string) string {
	if len(pubkey) >= 8 {
		return pubkey[:8]
	}
	return pubkey
}

// Internal bech32 functions.

func bytesToBase32(data []byte) []byte {
	var out []byte
	acc := 0
	bits := 0
	for _, b := range data {
		acc = (acc << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, byte((acc>>bits)&0x1f))
		}
		acc &= (1 << uint(bits)) - 1
	}
	if bits > 0 {
		out = append(out, byte((acc<<(5-bits))&0x1f))
	}
	return out
}

func base32ToBytes(data []byte) []byte {
	var out []byte
	acc := 0
	bits := 0
	for _, v := range data {
		acc = (acc << 5) | int(v)
		bits += 5
		for bits >= 8 {
			bits -= 8
			out = append(out, byte((acc>>bits)&0xff))
		}
		acc &= (1 << uint(bits)) - 1
	}
	return out
}

func bech32Polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HRPExpand(hrp string) []byte {
	out := make([]byte, 0, len(hrp)*2+1)
	for i := 0; i < len(hrp); i++ {
		out = append(out, byte(hrp[i]>>5))
	}
	out = append(out, 0)
	for i := 0; i < len(hrp); i++ {
		out = append(out, byte(hrp[i]&0x1f))
	}
	return out
}

func bech32Checksum(hrp string, data []byte) []byte {
	values := append(bech32HRPExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	out := make([]byte, 6)
	for i := 0; i < 6; i++ {
		out[i] = byte((polymod >> (5 * (5 - uint(i)))) & 0x1f)
	}
	return out
}

func bech32Verify(hrp string, data []byte) bool {
	values := append(bech32HRPExpand(hrp), data...)
	return bech32Polymod(values) == 1
}

func charsetIndex(c byte) int {
	for i := 0; i < len(bech32Charset); i++ {
		if bech32Charset[i] == c {
			return i
		}
	}
	return -1
}
