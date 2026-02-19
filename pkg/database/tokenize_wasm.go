//go:build js && wasm

package database

import (
	"crypto/sha256"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stopWords contains common English function words that are too frequent to be
// useful in search results. Filtering these reduces index size and prevents
// high-fanout Word nodes that slow down queries.
var stopWords = map[string]struct{}{
	"an": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"do": {}, "he": {}, "if": {}, "in": {}, "is": {},
	"it": {}, "me": {}, "my": {}, "no": {}, "of": {},
	"on": {}, "or": {}, "so": {}, "to": {}, "up": {},
	"us": {}, "we": {},
	"and": {}, "are": {}, "but": {}, "for": {}, "had": {},
	"has": {}, "her": {}, "him": {}, "his": {}, "how": {},
	"its": {}, "not": {}, "our": {}, "the": {}, "too": {},
	"was": {}, "who": {}, "you": {},
	"all": {}, "can": {}, "did": {}, "got": {}, "she": {},
	"been": {}, "does": {}, "each": {}, "from": {},
	"have": {}, "just": {}, "more": {}, "much": {},
	"must": {}, "only": {}, "some": {}, "such": {}, "than": {},
	"that": {}, "them": {}, "then": {}, "they": {}, "this": {},
	"very": {}, "were": {}, "what": {}, "when": {}, "will": {},
	"with": {}, "your": {},
	"about": {}, "could": {}, "other": {}, "their": {}, "there": {},
	"these": {}, "those": {}, "which": {}, "would": {},
}

// TokenWords extracts unique word tokens from content, returning both the
// normalized word text and its 8-byte truncated SHA-256 hash.
// Rules:
// - Unicode-aware: words are sequences of letters or numbers.
// - Lowercased using unicode case mapping.
// - Ignore URLs (starting with http://, https://, www., or containing "://").
// - Ignore nostr: URIs and #[n] mentions.
// - Ignore words shorter than 2 runes.
// - Exclude 64-character hexadecimal strings (likely IDs/pubkeys).
func TokenWords(content []byte) []WordToken {
	s := string(content)
	var out []WordToken
	seen := make(map[string]struct{})

	i := 0
	for i < len(s) {
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(s[i:])
		}

		// Skip whitespace
		if unicode.IsSpace(r) {
			i += size
			continue
		}

		// Skip URLs and schemes
		if hasPrefixFold(s[i:], "http://") || hasPrefixFold(s[i:], "https://") || hasPrefixFold(s[i:], "nostr:") || hasPrefixFold(s[i:], "www.") {
			i = skipUntilSpace(s, i)
			continue
		}
		// If token contains "://" ahead, treat as URL and skip to space
		if j := strings.Index(s[i:], "://"); j == 0 || (j > 0 && isWordStart(r)) {
			// Only if it's at start of token
			before := s[i : i+j]
			if len(before) == 0 || allAlphaNum(before) {
				i = skipUntilSpace(s, i)
				continue
			}
		}
		// Skip #[n] mentions
		if r == '#' && i+size < len(s) && s[i+size] == '[' {
			end := strings.IndexByte(s[i:], ']')
			if end >= 0 {
				i += end + 1
				continue
			}
		}

		// Collect a word
		start := i
		var runes []rune
		for i < len(s) {
			r2, size2 := rune(s[i]), 1
			if r2 >= 0x80 {
				r2, size2 = utf8.DecodeRuneInString(s[i:])
			}
			if unicode.IsLetter(r2) || unicode.IsNumber(r2) {
				// Normalize decorative unicode (small caps, fraktur) to ASCII
				// before lowercasing for consistent indexing
				runes = append(runes, unicode.ToLower(normalizeRune(r2)))
				i += size2
				continue
			}
			break
		}
		// If we didn't consume any rune for a word, advance by one rune to avoid stalling
		if i == start {
			_, size2 := utf8.DecodeRuneInString(s[i:])
			i += size2
			continue
		}
		if len(runes) >= 2 {
			w := string(runes)
			// Exclude 64-char hex strings
			if isHex64(w) {
				continue
			}
			// Exclude common stop words
			if _, ok := stopWords[w]; ok {
				continue
			}
			if _, ok := seen[w]; !ok {
				seen[w] = struct{}{}
				h := sha256.Sum256([]byte(w))
				out = append(out, WordToken{Word: w, Hash: h[:8]})
			}
		}
	}
	return out
}

// TokenHashes extracts unique word hashes (8-byte truncated sha256) from content.
// This is a convenience wrapper around TokenWords that returns only the hashes.
func TokenHashes(content []byte) [][]byte {
	tokens := TokenWords(content)
	out := make([][]byte, len(tokens))
	for i, t := range tokens {
		out[i] = t.Hash
	}
	return out
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		p := prefix[i]
		if c == p {
			continue
		}
		// ASCII case-insensitive
		if 'A' <= c && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		if 'A' <= p && p <= 'Z' {
			p = p - 'A' + 'a'
		}
		if c != p {
			return false
		}
	}
	return true
}

func skipUntilSpace(s string, i int) int {
	for i < len(s) {
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(s[i:])
		}
		if unicode.IsSpace(r) {
			return i
		}
		i += size
	}
	return i
}

func allAlphaNum(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r)) {
			return false
		}
	}
	return true
}

func isWordStart(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// isHex64 returns true if s is exactly 64 hex characters (0-9, a-f)
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < 64; i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		if c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}
