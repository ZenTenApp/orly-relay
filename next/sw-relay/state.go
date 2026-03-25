package main

import (
	"common/helpers"
	"common/jsbridge/subtle"
)

// Shared state and utilities for the relay SW.

var (
	cryptoCBs    map[int]func(string, string)
	nextCryptoID int
)

func initSharedState() {
	cryptoCBs = make(map[int]func(string, string))
}

// cryptoProxy routes a crypto request through the bus to the shell SW,
// which proxies it to the main page (NIP-07 extension).
func cryptoProxy(method, peerPubkey, data string, cb func(string, string)) {
	id := nextCryptoID
	nextCryptoID++
	cryptoCBs[id] = cb
	busSend("shell", "[\"CRYPTO_REQ\","+jstr("relay")+","+helpers.Itoa(int64(id))+","+jstr(method)+","+jstr(peerPubkey)+","+jstr(data)+"]")
}

// fwd sends a message to a specific browser client via the shell SW.
func fwd(clientID, msg string) {
	busSend("shell", "[\"FWD\","+jstr(clientID)+","+msg+"]")
}

// fwdAll broadcasts a message to all browser clients via the shell SW.
func fwdAll(msg string) {
	busSend("shell", "[\"FWD_ALL\","+msg+"]")
}

func strsJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	out := "["
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += jstr(s)
	}
	return out + "]"
}

func jstr(s string) string { return helpers.JsonString(s) }

func hexTo32(s string) [32]byte {
	out, _ := helpers.HexDecode32(s)
	return out
}

func random32() [32]byte {
	var b [32]byte
	subtle.RandomBytes(b[:])
	return b
}

// jsonField extracts a string value (unquoted) for a key from a JSON object string.
func jsonField(json, key string) string {
	v := jsonFieldRaw(json, key)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// jsonFieldRaw extracts a raw JSON value for a key from a JSON object string.
func jsonFieldRaw(json, key string) string {
	needle := "\"" + key + "\":"
	idx := -1
	for i := 0; i <= len(json)-len(needle); i++ {
		if json[i:i+len(needle)] == needle {
			idx = i + len(needle)
			break
		}
	}
	if idx < 0 {
		return ""
	}
	for idx < len(json) && (json[idx] == ' ' || json[idx] == '\t') {
		idx++
	}
	if idx >= len(json) {
		return ""
	}
	end := skipval(json, idx)
	if end < 0 {
		return ""
	}
	return json[idx:end]
}

// mw is a JSON array message walker for parsing bus messages.
type mw struct {
	s string
	i int
}

func newMW(s string) mw {
	w := mw{s: s}
	for w.i < len(s) && s[w.i] != '[' {
		w.i++
	}
	if w.i < len(s) {
		w.i++
	}
	return w
}

func (w *mw) sep() {
	for w.i < len(w.s) {
		c := w.s[w.i]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' && c != ',' {
			break
		}
		w.i++
	}
}

func (w *mw) str() string {
	w.sep()
	if w.i >= len(w.s) || w.s[w.i] != '"' {
		return ""
	}
	w.i++
	var buf []byte
	hasEsc := false
	start := w.i
	for w.i < len(w.s) && w.s[w.i] != '"' {
		if w.s[w.i] == '\\' && w.i+1 < len(w.s) {
			hasEsc = true
			buf = append(buf, w.s[start:w.i]...)
			w.i++
			switch w.s[w.i] {
			case '"', '\\', '/':
				buf = append(buf, w.s[w.i])
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case 'r':
				buf = append(buf, '\r')
			case 'u':
				if w.i+4 < len(w.s) {
					var cp int
					for k := 1; k <= 4; k++ {
						c := w.s[w.i+k]
						cp <<= 4
						if c >= '0' && c <= '9' {
							cp |= int(c - '0')
						} else if c >= 'a' && c <= 'f' {
							cp |= int(c-'a') + 10
						} else if c >= 'A' && c <= 'F' {
							cp |= int(c-'A') + 10
						}
					}
					if cp < 0x80 {
						buf = append(buf, byte(cp))
					} else if cp < 0x800 {
						buf = append(buf, byte(0xC0|(cp>>6)), byte(0x80|(cp&0x3F)))
					} else {
						buf = append(buf, byte(0xE0|(cp>>12)), byte(0x80|((cp>>6)&0x3F)), byte(0x80|(cp&0x3F)))
					}
					w.i += 4
				} else {
					buf = append(buf, '\\', 'u')
				}
			default:
				buf = append(buf, '\\', w.s[w.i])
			}
			w.i++
			start = w.i
			continue
		}
		w.i++
	}
	if w.i >= len(w.s) {
		return ""
	}
	var result string
	if hasEsc {
		buf = append(buf, w.s[start:w.i]...)
		result = string(buf)
	} else {
		result = w.s[start:w.i]
	}
	w.i++
	return result
}

func (w *mw) num() int64 {
	w.sep()
	neg := false
	if w.i < len(w.s) && w.s[w.i] == '-' {
		neg = true
		w.i++
	}
	var n int64
	for w.i < len(w.s) && w.s[w.i] >= '0' && w.s[w.i] <= '9' {
		n = n*10 + int64(w.s[w.i]-'0')
		w.i++
	}
	if neg {
		n = -n
	}
	return n
}

func (w *mw) raw() string {
	w.sep()
	start := w.i
	w.i = skipval(w.s, w.i)
	if w.i < 0 {
		w.i = len(w.s)
		return ""
	}
	return w.s[start:w.i]
}

func (w *mw) strs() []string {
	w.sep()
	if w.i >= len(w.s) || w.s[w.i] != '[' {
		return nil
	}
	w.i++
	var out []string
	for {
		w.sep()
		if w.i >= len(w.s) {
			return out
		}
		if w.s[w.i] == ']' {
			w.i++
			return out
		}
		if w.s[w.i] != '"' {
			return out
		}
		out = append(out, w.str())
	}
}

func skipval(s string, i int) int {
	if i >= len(s) {
		return -1
	}
	switch s[i] {
	case '"':
		i++
		for i < len(s) {
			if s[i] == '\\' {
				i += 2
				continue
			}
			if s[i] == '"' {
				return i + 1
			}
			i++
		}
		return -1
	case '{':
		return skipBrack(s, i, '{', '}')
	case '[':
		return skipBrack(s, i, '[', ']')
	case 't':
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	case 'f':
		if i+5 <= len(s) {
			return i + 5
		}
		return -1
	case 'n':
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	default:
		for i < len(s) && s[i] != ',' && s[i] != '}' && s[i] != ']' && s[i] != ' ' && s[i] != '\n' {
			i++
		}
		return i
	}
}

func skipBrack(s string, i int, open, close byte) int {
	depth := 1
	i++
	inStr := false
	for i < len(s) && depth > 0 {
		if inStr {
			if s[i] == '\\' {
				i++
			} else if s[i] == '"' {
				inStr = false
			}
		} else {
			switch s[i] {
			case '"':
				inStr = true
			case open:
				depth++
			case close:
				depth--
			}
		}
		i++
	}
	if depth != 0 {
		return -1
	}
	return i
}
