package helpers

const b64encode = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// Base64Encode encodes bytes to standard base64.
func Base64Encode(data []byte) string {
	n := len(data)
	out := make([]byte, 0, (n+2)/3*4)
	for i := 0; i < n; i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < n {
			b1 = data[i+1]
		}
		if i+2 < n {
			b2 = data[i+2]
		}
		out = append(out, b64encode[(b0>>2)&0x3f])
		out = append(out, b64encode[((b0<<4)|(b1>>4))&0x3f])
		if i+1 < n {
			out = append(out, b64encode[((b1<<2)|(b2>>6))&0x3f])
		} else {
			out = append(out, '=')
		}
		if i+2 < n {
			out = append(out, b64encode[b2&0x3f])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

// Base64Decode decodes standard base64. Returns nil on invalid input.
func Base64Decode(s string) []byte {
	// Strip padding.
	n := len(s)
	pad := 0
	for n > 0 && s[n-1] == '=' {
		pad++
		n--
	}
	out := make([]byte, 0, n*3/4)
	var acc uint32
	var bits int
	for i := 0; i < n; i++ {
		v := b64val(s[i])
		if v < 0 {
			return nil
		}
		acc = (acc << 6) | uint32(v)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>uint(bits)))
		}
	}
	_ = pad
	return out
}

func b64val(c byte) int {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26
	case c >= '0' && c <= '9':
		return int(c-'0') + 52
	case c == '+':
		return 62
	case c == '/':
		return 63
	default:
		return -1
	}
}
