package text

// NostrEscape for JSON encoding according to RFC8259.
//
// This is the efficient implementation based on the NIP-01 specification:
//
// To prevent implementation differences from creating a different event ID for
// the same event, the following rules MUST be followed while serializing:
//
//	No whitespace, line breaks or other unnecessary formatting should be included
//	in the output JSON. No characters except the following should be escaped, and
//	instead should be included verbatim:
//
//	- A line break, 0x0A, as \n
//	- A double quote, 0x22, as \"
//	- A backslash, 0x5C, as \\
//	- A carriage return, 0x0D, as \r
//	- A tab character, 0x09, as \t
//	- A backspace, 0x08, as \b
//	- A form feed, 0x0C, as \f
//
//	UTF-8 should be used for encoding.
//
// NOTE: We also escape all other control characters (0x00-0x1F excluding those above)
// to ensure valid JSON, even though NIP-01 doesn't require it. This prevents
// JSON parsing errors when events with binary data in content are sent to relays.
func NostrEscape(dst, src []byte) []byte {
	l := len(src)
	// Pre-allocate buffer if nil to reduce reallocations
	// Estimate: worst case is all control chars which expand to 6 bytes each (\u00XX)
	// but most strings have few escapes, so estimate len(src) * 1.5 as a safe middle ground
	if dst == nil && l > 0 {
		estimatedSize := l * 3 / 2
		if estimatedSize < l {
			estimatedSize = l
		}
		dst = make([]byte, 0, estimatedSize)
	}
	for i := 0; i < l; i++ {
		c := src[i]
		if c == '"' {
			dst = append(dst, '\\', '"')
		} else if c == '\\' {
			// if i+1 < l && src[i+1] == 'u' || i+1 < l && src[i+1] == '/' {
			if i+1 < l && src[i+1] == 'u' {
				dst = append(dst, '\\')
			} else {
				dst = append(dst, '\\', '\\')
			}
		} else if c == '\b' {
			dst = append(dst, '\\', 'b')
		} else if c == '\t' {
			dst = append(dst, '\\', 't')
		} else if c == '\n' {
			dst = append(dst, '\\', 'n')
		} else if c == '\f' {
			dst = append(dst, '\\', 'f')
		} else if c == '\r' {
			dst = append(dst, '\\', 'r')
		} else if c < 32 {
			// Escape all other control characters (0x00-0x1F except those handled above) as \uXXXX
			// This ensures valid JSON even when content contains binary data
			dst = append(dst, '\\', 'u', '0', '0')
			hexHigh := (c >> 4) & 0x0F
			hexLow := c & 0x0F
			if hexHigh < 10 {
				dst = append(dst, byte('0'+hexHigh))
			} else {
				dst = append(dst, byte('a'+(hexHigh-10)))
			}
			if hexLow < 10 {
				dst = append(dst, byte('0'+hexLow))
			} else {
				dst = append(dst, byte('a'+(hexLow-10)))
			}
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

// NostrUnescape reverses the operation of NostrEscape except instead of
// appending it to the provided slice, it rewrites it, eliminating a memory
// copy. Keep in mind that the original JSON will be mangled by this operation,
// but the resultant slices will cost zero allocations.
func NostrUnescape(dst []byte) (b []byte) {
	var r, w int
	for ; r < len(dst); r++ {
		if dst[r] == '\\' {
			r++
			c := dst[r]
			switch {

			// nip-01 specifies the following single letter C-style escapes for
			// control codes under 0x20.
			//
			// no others are specified but must be preserved, so only these can
			// be safely decoded at runtime as they must be re-encoded when
			// marshalled.
			case c == '"':
				dst[w] = '"'
				w++
			case c == '\\':
				dst[w] = '\\'
				w++
			case c == 'b':
				dst[w] = '\b'
				w++
			case c == 't':
				dst[w] = '\t'
				w++
			case c == 'n':
				dst[w] = '\n'
				w++
			case c == 'f':
				dst[w] = '\f'
				w++
			case c == 'r':
				dst[w] = '\r'
				w++

			// special cases for non-nip-01 specified json escapes (must be
			// preserved for ID generation).
		case c == 'u':
			// Check if this is a \u0000-\u001F sequence we generated
			if r+4 < len(dst) && dst[r+1] == '0' && dst[r+2] == '0' {
				// Extract hex digits
				hexHigh := dst[r+3]
				hexLow := dst[r+4]
				
				var val byte
				if hexHigh >= '0' && hexHigh <= '9' {
					val = (hexHigh - '0') << 4
				} else if hexHigh >= 'a' && hexHigh <= 'f' {
					val = (hexHigh - 'a' + 10) << 4
				} else if hexHigh >= 'A' && hexHigh <= 'F' {
					val = (hexHigh - 'A' + 10) << 4
				}
				
				if hexLow >= '0' && hexLow <= '9' {
					val |= hexLow - '0'
				} else if hexLow >= 'a' && hexLow <= 'f' {
					val |= hexLow - 'a' + 10
				} else if hexLow >= 'A' && hexLow <= 'F' {
					val |= hexLow - 'A' + 10
				}
				
				// Only decode if it's a control character (0x00-0x1F)
				if val < 32 {
					dst[w] = val
					w++
					r += 4 // Skip the u00XX part
					continue
				}
			}
			// Not our generated \u0000-\u001F, preserve as-is
			dst[w] = '\\'
			w++
			dst[w] = 'u'
			w++
		case c == '/':
				dst[w] = '\\'
				w++
				dst[w] = '/'
				w++

			// special case for octal escapes (must be preserved for ID
			// generation).
			case c >= '0' && c <= '9':
				dst[w] = '\\'
				w++
				dst[w] = c
				w++

				// anything else after a reverse solidus just preserve it.
			default:
				dst[w] = dst[r]
				w++
				dst[w] = c
				w++
			}
		} else {
			dst[w] = dst[r]
			w++
		}
	}
	b = dst[:w]
	return
}
