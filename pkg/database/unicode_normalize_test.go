//go:build !(js && wasm)

package database

import (
	"bytes"
	"testing"
)

func TestNormalizeRune(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected rune
	}{
		// Small caps
		{"small cap A", 'ᴀ', 'a'},
		{"small cap B", 'ʙ', 'b'},
		{"small cap C", 'ᴄ', 'c'},
		{"small cap D", 'ᴅ', 'd'},
		{"small cap E", 'ᴇ', 'e'},
		{"small cap F", 'ꜰ', 'f'},
		{"small cap G", 'ɢ', 'g'},
		{"small cap H", 'ʜ', 'h'},
		{"small cap I", 'ɪ', 'i'},
		{"small cap J", 'ᴊ', 'j'},
		{"small cap K", 'ᴋ', 'k'},
		{"small cap L", 'ʟ', 'l'},
		{"small cap M", 'ᴍ', 'm'},
		{"small cap N", 'ɴ', 'n'},
		{"small cap O", 'ᴏ', 'o'},
		{"small cap P", 'ᴘ', 'p'},
		{"small cap Q (ogonek)", 'ǫ', 'q'},
		{"small cap R", 'ʀ', 'r'},
		{"small cap S", 'ꜱ', 's'},
		{"small cap T", 'ᴛ', 't'},
		{"small cap U", 'ᴜ', 'u'},
		{"small cap V", 'ᴠ', 'v'},
		{"small cap W", 'ᴡ', 'w'},
		{"small cap Y", 'ʏ', 'y'},
		{"small cap Z", 'ᴢ', 'z'},

		// Fraktur lowercase
		{"fraktur lower a", '𝔞', 'a'},
		{"fraktur lower b", '𝔟', 'b'},
		{"fraktur lower c", '𝔠', 'c'},
		{"fraktur lower d", '𝔡', 'd'},
		{"fraktur lower e", '𝔢', 'e'},
		{"fraktur lower f", '𝔣', 'f'},
		{"fraktur lower g", '𝔤', 'g'},
		{"fraktur lower h", '𝔥', 'h'},
		{"fraktur lower i", '𝔦', 'i'},
		{"fraktur lower j", '𝔧', 'j'},
		{"fraktur lower k", '𝔨', 'k'},
		{"fraktur lower l", '𝔩', 'l'},
		{"fraktur lower m", '𝔪', 'm'},
		{"fraktur lower n", '𝔫', 'n'},
		{"fraktur lower o", '𝔬', 'o'},
		{"fraktur lower p", '𝔭', 'p'},
		{"fraktur lower q", '𝔮', 'q'},
		{"fraktur lower r", '𝔯', 'r'},
		{"fraktur lower s", '𝔰', 's'},
		{"fraktur lower t", '𝔱', 't'},
		{"fraktur lower u", '𝔲', 'u'},
		{"fraktur lower v", '𝔳', 'v'},
		{"fraktur lower w", '𝔴', 'w'},
		{"fraktur lower x", '𝔵', 'x'},
		{"fraktur lower y", '𝔶', 'y'},
		{"fraktur lower z", '𝔷', 'z'},

		// Fraktur uppercase (main range)
		{"fraktur upper A", '𝔄', 'a'},
		{"fraktur upper B", '𝔅', 'b'},
		{"fraktur upper D", '𝔇', 'd'},
		{"fraktur upper E", '𝔈', 'e'},
		{"fraktur upper F", '𝔉', 'f'},
		{"fraktur upper G", '𝔊', 'g'},
		{"fraktur upper J", '𝔍', 'j'},
		{"fraktur upper K", '𝔎', 'k'},
		{"fraktur upper L", '𝔏', 'l'},
		{"fraktur upper M", '𝔐', 'm'},
		{"fraktur upper N", '𝔑', 'n'},
		{"fraktur upper O", '𝔒', 'o'},
		{"fraktur upper P", '𝔓', 'p'},
		{"fraktur upper Q", '𝔔', 'q'},
		{"fraktur upper S", '𝔖', 's'},
		{"fraktur upper T", '𝔗', 't'},
		{"fraktur upper U", '𝔘', 'u'},
		{"fraktur upper V", '𝔙', 'v'},
		{"fraktur upper W", '𝔚', 'w'},
		{"fraktur upper X", '𝔛', 'x'},
		{"fraktur upper Y", '𝔜', 'y'},

		// Fraktur uppercase (Letterlike Symbols block)
		{"fraktur upper C (letterlike)", 'ℭ', 'c'},
		{"fraktur upper H (letterlike)", 'ℌ', 'h'},
		{"fraktur upper I (letterlike)", 'ℑ', 'i'},
		{"fraktur upper R (letterlike)", 'ℜ', 'r'},
		{"fraktur upper Z (letterlike)", 'ℨ', 'z'},

		// Regular ASCII should pass through unchanged
		{"regular lowercase a", 'a', 'a'},
		{"regular lowercase z", 'z', 'z'},
		{"regular uppercase A", 'A', 'A'},
		{"regular digit 5", '5', '5'},

		// Other unicode should pass through unchanged
		{"cyrillic д", 'д', 'д'},
		{"greek α", 'α', 'α'},
		{"emoji", '🎉', '🎉'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRune(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRune(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasDecorativeUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"plain ASCII", "hello world", false},
		{"small caps word", "ᴅᴇᴀᴛʜ", true},
		{"fraktur lowercase", "𝔥𝔢𝔩𝔩𝔬", true},
		{"fraktur uppercase", "𝔇𝔈𝔄𝔗ℌ", true},
		{"mixed with ASCII", "hello ᴡᴏʀʟᴅ", true},
		{"single small cap", "aᴀa", true},
		{"cyrillic (no normalize)", "привет", false},
		{"empty string", "", false},
		{"letterlike fraktur C", "ℭool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasDecorativeUnicode(tt.input)
			if result != tt.expected {
				t.Errorf("hasDecorativeUnicode(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenHashesNormalization(t *testing.T) {
	// All three representations should produce the same hash
	ascii := TokenHashes([]byte("death"))
	smallCaps := TokenHashes([]byte("ᴅᴇᴀᴛʜ"))
	frakturLower := TokenHashes([]byte("𝔡𝔢𝔞𝔱𝔥"))
	frakturUpper := TokenHashes([]byte("𝔇𝔈𝔄𝔗ℌ"))

	if len(ascii) != 1 {
		t.Fatalf("expected 1 hash for 'death', got %d", len(ascii))
	}
	if len(smallCaps) != 1 {
		t.Fatalf("expected 1 hash for small caps, got %d", len(smallCaps))
	}
	if len(frakturLower) != 1 {
		t.Fatalf("expected 1 hash for fraktur lower, got %d", len(frakturLower))
	}
	if len(frakturUpper) != 1 {
		t.Fatalf("expected 1 hash for fraktur upper, got %d", len(frakturUpper))
	}

	// All should match the ASCII version
	if !bytes.Equal(ascii[0], smallCaps[0]) {
		t.Errorf("small caps hash differs from ASCII\nASCII:      %x\nsmall caps: %x", ascii[0], smallCaps[0])
	}
	if !bytes.Equal(ascii[0], frakturLower[0]) {
		t.Errorf("fraktur lower hash differs from ASCII\nASCII:         %x\nfraktur lower: %x", ascii[0], frakturLower[0])
	}
	if !bytes.Equal(ascii[0], frakturUpper[0]) {
		t.Errorf("fraktur upper hash differs from ASCII\nASCII:         %x\nfraktur upper: %x", ascii[0], frakturUpper[0])
	}
}

func TestTokenHashesMixedContent(t *testing.T) {
	// Test that mixed content normalizes correctly
	content := []byte("ᴛʜᴇ quick 𝔟𝔯𝔬𝔴𝔫 fox")
	hashes := TokenHashes(content)

	// Should get: "the", "quick", "brown", "fox" (4 unique words)
	if len(hashes) != 4 {
		t.Errorf("expected 4 hashes from mixed content, got %d", len(hashes))
	}

	// Verify "the" matches between decorated and plain
	thePlain := TokenHashes([]byte("the"))
	theDecorated := TokenHashes([]byte("ᴛʜᴇ"))
	if !bytes.Equal(thePlain[0], theDecorated[0]) {
		t.Errorf("'the' hash mismatch: plain=%x, decorated=%x", thePlain[0], theDecorated[0])
	}

	// Verify "brown" matches between decorated and plain
	brownPlain := TokenHashes([]byte("brown"))
	brownDecorated := TokenHashes([]byte("𝔟𝔯𝔬𝔴𝔫"))
	if !bytes.Equal(brownPlain[0], brownDecorated[0]) {
		t.Errorf("'brown' hash mismatch: plain=%x, decorated=%x", brownPlain[0], brownDecorated[0])
	}
}
