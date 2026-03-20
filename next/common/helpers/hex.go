// Package helpers provides utility functions for smesh3.
package helpers

const hexChars = "0123456789abcdef"

// HexEncode encodes bytes to lowercase hex string.
func HexEncode(b []byte) string {
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return string(out)
}

// HexDecode decodes a hex string to bytes. Returns nil on invalid input.
func HexDecode(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := hexVal(s[i])
		lo := hexVal(s[i+1])
		if hi < 0 || lo < 0 {
			return nil
		}
		out[i/2] = byte(hi<<4 | lo)
	}
	return out
}

// HexDecode32 decodes a 64-char hex string to [32]byte.
// Returns zero and false on invalid input.
func HexDecode32(s string) (out [32]byte, ok bool) {
	if len(s) != 64 {
		return out, false
	}
	for i := 0; i < 32; i++ {
		hi := hexVal(s[i*2])
		lo := hexVal(s[i*2+1])
		if hi < 0 || lo < 0 {
			return out, false
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, true
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}
