package helpers

// Minimal JSON serialization for Nostr events.
// No encoding/json dependency.

// JsonString returns a JSON-escaped string with surrounding quotes.
// Uses string concat to preserve non-ASCII in tinyjs.
func JsonString(s string) string {
	result := "\""
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var esc string
		switch c {
		case '"':
			esc = "\\\""
		case '\\':
			esc = "\\\\"
		case '\n':
			esc = "\\n"
		case '\r':
			esc = "\\r"
		case '\t':
			esc = "\\t"
		case '\b':
			esc = "\\b"
		case '\f':
			esc = "\\f"
		default:
			if c < 0x20 {
				esc = "\\u00" + string(hexChars[c>>4]) + string(hexChars[c&0x0f])
			} else {
				continue
			}
		}
		result += s[start:i] + esc
		start = i + 1
	}
	return result + s[start:] + "\""
}

// JsonGetString extracts a string value for the given key from a JSON object.
// Returns empty string if not found. Handles basic escape sequences.
// Uses string concat (not []byte) to preserve non-ASCII characters in tinyjs.
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
			start := j
			result := ""
			for j < len(s) {
				if s[j] == '\\' && j+1 < len(s) {
					result += s[start:j]
					j++
					switch s[j] {
					case '"', '\\', '/':
						result += s[j : j+1]
					case 'n':
						result += "\n"
					case 'r':
						result += "\r"
					case 't':
						result += "\t"
					default:
						result += s[j : j+1]
					}
					j++
					start = j
					continue
				}
				if s[j] == '"' {
					return result + s[start:j]
				}
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
