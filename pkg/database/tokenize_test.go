//go:build !(js && wasm)

package database

import (
	"bytes"
	"strings"
	"testing"
)

func TestTokenWords_StopWords(t *testing.T) {
	tokens := TokenWords([]byte("the quick brown fox and the lazy dog"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	// "the" and "and" are stop words
	if words["the"] {
		t.Error("stop word 'the' should be filtered")
	}
	if words["and"] {
		t.Error("stop word 'and' should be filtered")
	}

	// content words should remain
	for _, w := range []string{"quick", "brown", "fox", "lazy", "dog"} {
		if !words[w] {
			t.Errorf("expected word %q to be present", w)
		}
	}

	if len(tokens) != 5 {
		t.Errorf("expected 5 tokens, got %d", len(tokens))
	}
}

func TestTokenWords_AllStopWords(t *testing.T) {
	// Input consisting entirely of stop words should produce zero tokens
	tokens := TokenWords([]byte("the and for with from that this they"))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for all-stop-word input, got %d", len(tokens))
	}
}

func TestTokenWords_EmptyContent(t *testing.T) {
	tokens := TokenWords([]byte(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty content, got %d", len(tokens))
	}

	tokens = TokenWords(nil)
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for nil content, got %d", len(tokens))
	}
}

func TestTokenWords_OnlyURLs(t *testing.T) {
	tokens := TokenWords([]byte("https://example.com http://test.org www.foo.bar"))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for URL-only content, got %d", len(tokens))
	}
}

func TestTokenWords_NumbersIncluded(t *testing.T) {
	tokens := TokenWords([]byte("42 1337 99"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	// "42" and "99" are 2 chars, should be included
	if !words["42"] {
		t.Error("expected '42' to be tokenized")
	}
	if !words["99"] {
		t.Error("expected '99' to be tokenized")
	}
	if !words["1337"] {
		t.Error("expected '1337' to be tokenized")
	}
}

func TestTokenWords_LongContent(t *testing.T) {
	// Generate a large input to ensure no panic or infinite loop
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString("word ")
	}
	tokens := TokenWords([]byte(sb.String()))

	// "word" is deduplicated so only 1 unique token
	if len(tokens) != 1 {
		t.Errorf("expected 1 unique token from repeated word, got %d", len(tokens))
	}
	if tokens[0].Word != "word" {
		t.Errorf("expected 'word', got %q", tokens[0].Word)
	}
}

func TestTokenWords_MixedUnicode(t *testing.T) {
	// CJK characters are letters and should be tokenized if 2+ runes
	// Single CJK chars are filtered by min-length
	tokens := TokenWords([]byte("hello 世界 café"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	if !words["hello"] {
		t.Error("expected 'hello'")
	}
	// "世界" is 2 runes, both are letters — should be included
	if !words["世界"] {
		t.Error("expected '世界' (2 CJK characters)")
	}
	// "café" normalizes to lowercase
	if !words["café"] {
		t.Error("expected 'café'")
	}
}

func TestTokenWords_Hex64Exclusion(t *testing.T) {
	hex := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	content := "before " + hex + " after"
	tokens := TokenWords([]byte(content))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	if words[hex] {
		t.Error("64-char hex string should be excluded")
	}
	if !words["before"] {
		t.Error("expected 'before'")
	}
	if !words["after"] {
		t.Error("expected 'after'")
	}
}

func TestTokenWords_NostrURIExclusion(t *testing.T) {
	tokens := TokenWords([]byte("check nostr:npub1abc123def456 now"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	if !words["check"] {
		t.Error("expected 'check'")
	}
	if !words["now"] {
		t.Error("expected 'now'")
	}
	// Nothing from the nostr: URI should appear
	if words["npub1abc123def456"] {
		t.Error("nostr: URI content should be excluded")
	}
}

func TestTokenWords_MentionExclusion(t *testing.T) {
	tokens := TokenWords([]byte("see #[0] here #[12] done"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	if !words["see"] {
		t.Error("expected 'see'")
	}
	if !words["here"] {
		t.Error("expected 'here'")
	}
	if !words["done"] {
		t.Error("expected 'done'")
	}
}

func TestTokenWords_URLSchemes(t *testing.T) {
	// Various URL-like schemes with "://" should be skipped
	tokens := TokenWords([]byte("visit ftp://files.example.com and wss://relay.damus.io"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	if words["ftp"] || words["files"] || words["example"] {
		t.Error("ftp:// URL content should be excluded")
	}
	if words["wss"] || words["relay"] || words["damus"] {
		t.Error("wss:// URL content should be excluded")
	}
	// "visit" should survive; "and" is a stop word
	if !words["visit"] {
		t.Error("expected 'visit'")
	}
}

func TestTokenWords_MinLengthBoundary(t *testing.T) {
	// 1-rune words are filtered, 2-rune words are kept (unless stop words)
	tokens := TokenWords([]byte("I go ok hi"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	// "I" is 1 rune — filtered by length
	if words["i"] {
		t.Error("single-rune 'I' should be filtered by min-length")
	}
	// "go" is 2 runes but not a stop word — should be present
	if !words["go"] {
		t.Error("expected 'go' (2 runes, not a stop word)")
	}
	// "ok" is 2 runes, not a stop word
	if !words["ok"] {
		t.Error("expected 'ok'")
	}
	// "hi" is 2 runes, not a stop word
	if !words["hi"] {
		t.Error("expected 'hi'")
	}
}

func TestTokenWords_Deduplication(t *testing.T) {
	tokens := TokenWords([]byte("Bitcoin bitcoin BITCOIN"))
	if len(tokens) != 1 {
		t.Fatalf("expected 1 deduplicated token, got %d", len(tokens))
	}
	if tokens[0].Word != "bitcoin" {
		t.Errorf("expected 'bitcoin', got %q", tokens[0].Word)
	}
}

func TestTokenWords_WhitespaceVariants(t *testing.T) {
	// Tabs, newlines, and other whitespace should split tokens
	tokens := TokenWords([]byte("hello\tworld\nnew\rline"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	for _, w := range []string{"hello", "world", "new", "line"} {
		if !words[w] {
			t.Errorf("expected word %q after whitespace splitting", w)
		}
	}
}

func TestTokenWords_PunctuationSplit(t *testing.T) {
	// Punctuation should split words, not be included in them
	tokens := TokenWords([]byte("hello, world! foo-bar baz.qux"))
	words := make(map[string]bool)
	for _, tok := range tokens {
		words[tok.Word] = true
	}

	for _, w := range []string{"hello", "world", "foo", "bar", "baz", "qux"} {
		if !words[w] {
			t.Errorf("expected word %q after punctuation split", w)
		}
	}
}

func TestTokenHashes_MatchesTokenWords(t *testing.T) {
	content := []byte("bitcoin lightning network relay")
	tokens := TokenWords(content)
	hashes := TokenHashes(content)

	if len(tokens) != len(hashes) {
		t.Fatalf("TokenWords returned %d, TokenHashes returned %d", len(tokens), len(hashes))
	}

	for i, tok := range tokens {
		if !bytes.Equal(tok.Hash, hashes[i]) {
			t.Errorf("hash mismatch at index %d: TokenWords=%x, TokenHashes=%x",
				i, tok.Hash, hashes[i])
		}
	}
}

func TestTokenWords_StopWordsList(t *testing.T) {
	// Verify a representative sample of stop words are actually filtered
	samples := []string{
		"the", "and", "for", "with", "from", "that", "this",
		"they", "were", "will", "been", "have", "about",
		"which", "would", "their", "there", "these", "those",
		"is", "in", "of", "to", "or", "an", "as", "at",
	}
	for _, sw := range samples {
		tokens := TokenWords([]byte(sw))
		if len(tokens) != 0 {
			t.Errorf("stop word %q should produce 0 tokens, got %d", sw, len(tokens))
		}
	}
}
