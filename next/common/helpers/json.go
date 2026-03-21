package helpers

// Minimal JSON serialization for Nostr events.
// No encoding/json dependency.

// JsonString returns a JSON-escaped string with surrounding quotes.
func JsonString(s string) string {
	buf := make([]byte, 0, len(s)+2)
	buf = append(buf, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		case '\b':
			buf = append(buf, '\\', 'b')
		case '\f':
			buf = append(buf, '\\', 'f')
		default:
			if c < 0x20 {
				buf = append(buf, '\\', 'u', '0', '0',
					hexChars[c>>4], hexChars[c&0x0f])
			} else {
				buf = append(buf, c)
			}
		}
	}
	buf = append(buf, '"')
	return string(buf)
}

// JsonGetString extracts a string value for the given key from a JSON object.
// Returns empty string if not found. Handles basic escape sequences.
func JsonGetString(s, key string) string {
	kq := "\"" + key + "\""
	kqLen := len(kq)
	for i := 0; i <= len(s)-kqLen; i++ {
		if s[i:i+kqLen] == kq {
			j := i + kqLen
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j >= len(s) || s[j] != ':' {
				continue
			}
			j++
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j >= len(s) || s[j] != '"' {
				continue
			}
			j++
			var buf []byte
			for j < len(s) {
				if s[j] == '\\' && j+1 < len(s) {
					j++
					switch s[j] {
					case '"', '\\', '/':
						buf = append(buf, s[j])
					case 'n':
						buf = append(buf, '\n')
					case 'r':
						buf = append(buf, '\r')
					case 't':
						buf = append(buf, '\t')
					default:
						buf = append(buf, s[j])
					}
					j++
					continue
				}
				if s[j] == '"' {
					return string(buf)
				}
				buf = append(buf, s[j])
				j++
			}
		}
	}
	return ""
}

// Itoa converts int64 to decimal string.
func Itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
