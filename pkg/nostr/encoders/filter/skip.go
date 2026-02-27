package filter

import (
	"next.orly.dev/pkg/lol/errorf"
)

// skipJSONValue skips over an arbitrary JSON value and returns the raw bytes and remainder.
// It handles: objects {}, arrays [], strings "", numbers, true, false, null.
// The input `b` should start at the first character of the value (after the colon in "key":value).
func skipJSONValue(b []byte) (val []byte, r []byte, err error) {
	if len(b) == 0 {
		err = errorf.E("empty input")
		return
	}

	start := 0
	end := 0

	switch b[0] {
	case '{':
		// Object - find matching closing brace
		end, err = findMatchingBrace(b, '{', '}')
	case '[':
		// Array - find matching closing bracket
		end, err = findMatchingBrace(b, '[', ']')
	case '"':
		// String - find closing quote (handling escapes)
		end, err = findClosingQuote(b)
	case 't':
		// true
		if len(b) >= 4 && string(b[:4]) == "true" {
			end = 4
		} else {
			err = errorf.E("invalid JSON value starting with 't'")
		}
	case 'f':
		// false
		if len(b) >= 5 && string(b[:5]) == "false" {
			end = 5
		} else {
			err = errorf.E("invalid JSON value starting with 'f'")
		}
	case 'n':
		// null
		if len(b) >= 4 && string(b[:4]) == "null" {
			end = 4
		} else {
			err = errorf.E("invalid JSON value starting with 'n'")
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Number - scan until we hit a non-number character
		end = scanNumber(b)
	default:
		err = errorf.E("invalid JSON value starting with '%c'", b[0])
	}

	if err != nil {
		return
	}

	val = b[start:end]
	r = b[end:]
	return
}

// findMatchingBrace finds the index after the closing brace/bracket that matches the opening one.
// It handles nested structures and strings.
func findMatchingBrace(b []byte, open, close byte) (end int, err error) {
	if len(b) == 0 || b[0] != open {
		err = errorf.E("expected '%c'", open)
		return
	}

	depth := 0
	inString := false
	escaped := false

	for i := 0; i < len(b); i++ {
		c := b[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				end = i + 1
				return
			}
		}
	}

	err = errorf.E("unmatched '%c'", open)
	return
}

// findClosingQuote finds the index after the closing quote of a JSON string.
// Handles escape sequences.
func findClosingQuote(b []byte) (end int, err error) {
	if len(b) == 0 || b[0] != '"' {
		err = errorf.E("expected '\"'")
		return
	}

	escaped := false
	for i := 1; i < len(b); i++ {
		c := b[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if c == '"' {
			end = i + 1
			return
		}
	}

	err = errorf.E("unclosed string")
	return
}

// scanNumber scans a JSON number and returns the index after it.
// Handles integers, decimals, and scientific notation.
func scanNumber(b []byte) (end int) {
	for i := 0; i < len(b); i++ {
		c := b[i]
		// Number characters: digits, minus, plus, dot, e, E
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			continue
		}
		end = i
		return
	}
	end = len(b)
	return
}
