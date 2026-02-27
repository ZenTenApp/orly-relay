package text

import (
	"io"

	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/utils"
	"github.com/templexxx/xhex"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
)

// JSONKey generates the JSON format for an object key and terminates with the semicolon.
func JSONKey(dst, k []byte) (b []byte) {
	dst = append(dst, '"')
	dst = append(dst, k...)
	dst = append(dst, '"', ':')
	b = dst
	return
}

// UnmarshalHex takes a byte string that should contain a quoted hexadecimal
// encoded value, decodes it using a SIMD hex codec and returns the decoded
// bytes in a newly allocated buffer.
func UnmarshalHex(b []byte) (h []byte, rem []byte, err error) {
	rem = b[:]
	var inQuote bool
	var start int
	for i := 0; i < len(b); i++ {
		if !inQuote {
			if b[i] == '"' {
				inQuote = true
				start = i + 1
			}
		} else if b[i] == '"' {
			hexStr := b[start:i]
			rem = b[i+1:]
			l := len(hexStr)
			if l%2 != 0 {
				err = errorf.E(
					"invalid length for hex: %d, %0x",
					len(hexStr), hexStr,
				)
				return
			}
			// Allocate a new buffer for the decoded data
			h = make([]byte, l/2)
			if err = xhex.Decode(h, hexStr); chk.E(err) {
				return
			}
			return
		}
	}
	if !inQuote {
		err = io.EOF
		return
	}
	return
}

// UnmarshalQuoted performs an in-place unquoting of NIP-01 quoted byte string.
func UnmarshalQuoted(b []byte) (content, rem []byte, err error) {
	if len(b) == 0 {
		err = io.EOF
		return
	}
	rem = b[:]
	for ; len(rem) >= 0; rem = rem[1:] {
		if len(rem) == 0 {
			err = io.EOF
			return
		}
		// advance to open quotes
		if rem[0] == '"' {
			rem = rem[1:]
			content = rem
			break
		}
	}
	if len(rem) == 0 {
		err = io.EOF
		return
	}
	var escaping bool
	var contentLen int
	for len(rem) > 0 {
		if rem[0] == '\\' {
			if !escaping {
				escaping = true
				contentLen++
				rem = rem[1:]
			} else {
				escaping = false
				contentLen++
				rem = rem[1:]
			}
		} else if rem[0] == '"' {
			if !escaping {
				rem = rem[1:]
				content = content[:contentLen]
				// Create a copy of the content to avoid corrupting the original input buffer
				contentCopy := make([]byte, len(content))
				copy(contentCopy, content)
				content = NostrUnescape(contentCopy)
				return
			}
			contentLen++
			rem = rem[1:]
			escaping = false
		} else {
			escaping = false
			switch rem[0] {
			// none of these characters are allowed inside a JSON string:
			//
			// backspace, tab, newline, form feed or carriage return.
			case '\b', '\t', '\n', '\f', '\r':
				pos := len(content) - len(rem)
				contextStart := pos - 10
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := pos + 10
				if contextEnd > len(content) {
					contextEnd = len(content)
				}
				err = errorf.E(
					"invalid character '%s' in quoted string (position %d, context: %q)",
					NostrEscape(nil, rem[:1]),
					pos,
					string(content[contextStart:contextEnd]),
				)
				return
			}
			contentLen++
			rem = rem[1:]
		}
	}
	return
}

func MarshalHexArray(dst []byte, ha [][]byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	// Estimate: [ + (hex encoded item + quotes + comma) * n + ]
	// Each hex item is 2*size + 2 quotes = 2*size + 2, plus comma for all but last
	if b == nil && len(ha) > 0 {
		estimatedSize := 2 // brackets
		if len(ha) > 0 {
			// Estimate based on first item size
			itemSize := len(ha[0]) * 2                    // hex encoding doubles size
			estimatedSize += len(ha) * (itemSize + 2 + 1) // item + quotes + comma
		}
		b = make([]byte, 0, estimatedSize)
	}
	b = append(b, '[')
	for i := range ha {
		b = AppendQuote(b, ha[i], hex.EncAppend)
		if i != len(ha)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// UnmarshalHexArray unpacks a JSON array containing strings with hexadecimal, and checks all
// values have the specified byte size.
func UnmarshalHexArray(b []byte, size int) (t [][]byte, rem []byte, err error) {
	rem = b
	var openBracket bool
	// Pre-allocate slice with estimated capacity to reduce reallocations
	// Estimate based on typical array sizes (can grow if needed)
	t = make([][]byte, 0, 16)
	for ; len(rem) > 0; rem = rem[1:] {
		if rem[0] == '[' {
			openBracket = true
		} else if openBracket {
			if rem[0] == ',' {
				continue
			} else if rem[0] == ']' {
				rem = rem[1:]
				return
			} else if rem[0] == '"' {
				var h []byte
				if h, rem, err = UnmarshalHex(rem); chk.E(err) {
					return
				}
				if len(h) != size {
					err = errorf.E(
						"invalid hex array size, got %d expect %d",
						2*len(h), 2*size,
					)
					return
				}
				t = append(t, h)
				if rem[0] == ']' {
					rem = rem[1:]
					// done
					return
				}
			}
		}
	}
	return
}

// UnmarshalStringArray unpacks a JSON array containing strings.
func UnmarshalStringArray(b []byte) (t [][]byte, rem []byte, err error) {
	rem = b
	var openBracket bool
	// Pre-allocate slice with estimated capacity to reduce reallocations
	// Estimate based on typical array sizes (can grow if needed)
	t = make([][]byte, 0, 16)
	for ; len(rem) > 0; rem = rem[1:] {
		if rem[0] == '[' {
			openBracket = true
		} else if openBracket {
			if rem[0] == ',' {
				continue
			} else if rem[0] == ']' {
				rem = rem[1:]
				return
			} else if rem[0] == '"' {
				var h []byte
				if h, rem, err = UnmarshalQuoted(rem); chk.E(err) {
					return
				}
				t = append(t, h)
				if rem[0] == ']' {
					rem = rem[1:]
					// done
					return
				}
			}
		}
	}
	return
}

func True() []byte  { return []byte("true") }
func False() []byte { return []byte("false") }

func MarshalBool(src []byte, truth bool) []byte {
	if truth {
		return append(src, True()...)
	}
	return append(src, False()...)
}

func UnmarshalBool(src []byte) (rem []byte, truth bool, err error) {
	rem = src
	t, f := True(), False()
	for i := range rem {
		if rem[i] == t[0] {
			if len(rem) < i+len(t) {
				err = io.EOF
				return
			}
			if utils.FastEqual(t, rem[i:i+len(t)]) {
				truth = true
				rem = rem[i+len(t):]
				return
			}
		}
		if rem[i] == f[0] {
			if len(rem) < i+len(f) {
				err = io.EOF
				return
			}
			if utils.FastEqual(f, rem[i:i+len(f)]) {
				rem = rem[i+len(f):]
				return
			}
		}
	}
	// if a truth value is not found in the string it will run to the end
	err = io.EOF
	return
}

func Comma(b []byte) (rem []byte, err error) {
	rem = b
	for i := range rem {
		if rem[i] == ',' {
			rem = rem[i:]
			return
		}
	}
	err = io.EOF
	return
}
