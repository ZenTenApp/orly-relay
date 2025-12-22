//go:build !(js && wasm)

package database

// normalizeRune maps decorative unicode characters (small caps, fraktur) back to
// their ASCII equivalents for consistent word indexing. This ensures that text
// written with decorative alphabets (e.g., "ᴅᴇᴀᴛʜ" or "𝔇𝔢𝔞𝔱𝔥") indexes the same
// as regular ASCII ("death").
//
// Character sets normalized:
// - Small Caps (used for DEATH-style text in Terry Pratchett tradition)
// - Mathematical Fraktur lowercase (𝔞-𝔷)
// - Mathematical Fraktur uppercase (𝔄-ℨ, including Letterlike Symbols block exceptions)
func normalizeRune(r rune) rune {
	// Check small caps first (scattered codepoints)
	if mapped, ok := smallCapsToASCII[r]; ok {
		return mapped
	}

	// Check fraktur lowercase: U+1D51E to U+1D537 (contiguous range)
	if r >= 0x1D51E && r <= 0x1D537 {
		return 'a' + (r - 0x1D51E)
	}

	// Check fraktur uppercase main range: U+1D504 to U+1D51C (with gaps)
	if r >= 0x1D504 && r <= 0x1D51C {
		if mapped, ok := frakturUpperToASCII[r]; ok {
			return mapped
		}
	}

	// Check fraktur uppercase exceptions from Letterlike Symbols block
	if mapped, ok := frakturLetterlikeToASCII[r]; ok {
		return mapped
	}

	return r
}

// smallCapsToASCII maps small capital letters to lowercase ASCII.
// These are scattered across multiple Unicode blocks (IPA Extensions,
// Phonetic Extensions, Latin Extended-D).
var smallCapsToASCII = map[rune]rune{
	'ᴀ': 'a', // U+1D00 LATIN LETTER SMALL CAPITAL A
	'ʙ': 'b', // U+0299 LATIN LETTER SMALL CAPITAL B
	'ᴄ': 'c', // U+1D04 LATIN LETTER SMALL CAPITAL C
	'ᴅ': 'd', // U+1D05 LATIN LETTER SMALL CAPITAL D
	'ᴇ': 'e', // U+1D07 LATIN LETTER SMALL CAPITAL E
	'ꜰ': 'f', // U+A730 LATIN LETTER SMALL CAPITAL F
	'ɢ': 'g', // U+0262 LATIN LETTER SMALL CAPITAL G
	'ʜ': 'h', // U+029C LATIN LETTER SMALL CAPITAL H
	'ɪ': 'i', // U+026A LATIN LETTER SMALL CAPITAL I
	'ᴊ': 'j', // U+1D0A LATIN LETTER SMALL CAPITAL J
	'ᴋ': 'k', // U+1D0B LATIN LETTER SMALL CAPITAL K
	'ʟ': 'l', // U+029F LATIN LETTER SMALL CAPITAL L
	'ᴍ': 'm', // U+1D0D LATIN LETTER SMALL CAPITAL M
	'ɴ': 'n', // U+0274 LATIN LETTER SMALL CAPITAL N
	'ᴏ': 'o', // U+1D0F LATIN LETTER SMALL CAPITAL O
	'ᴘ': 'p', // U+1D18 LATIN LETTER SMALL CAPITAL P
	'ǫ': 'q', // U+01EB LATIN SMALL LETTER O WITH OGONEK (no true small cap Q)
	'ʀ': 'r', // U+0280 LATIN LETTER SMALL CAPITAL R
	'ꜱ': 's', // U+A731 LATIN LETTER SMALL CAPITAL S
	'ᴛ': 't', // U+1D1B LATIN LETTER SMALL CAPITAL T
	'ᴜ': 'u', // U+1D1C LATIN LETTER SMALL CAPITAL U
	'ᴠ': 'v', // U+1D20 LATIN LETTER SMALL CAPITAL V
	'ᴡ': 'w', // U+1D21 LATIN LETTER SMALL CAPITAL W
	// Note: no small cap X exists in standard use
	'ʏ': 'y', // U+028F LATIN LETTER SMALL CAPITAL Y
	'ᴢ': 'z', // U+1D22 LATIN LETTER SMALL CAPITAL Z
}

// frakturUpperToASCII maps Mathematical Fraktur uppercase letters to lowercase ASCII.
// The main range U+1D504-U+1D51C has gaps where C, H, I, R, Z use Letterlike Symbols.
var frakturUpperToASCII = map[rune]rune{
	'𝔄': 'a', // U+1D504 MATHEMATICAL FRAKTUR CAPITAL A
	'𝔅': 'b', // U+1D505 MATHEMATICAL FRAKTUR CAPITAL B
	// C is at U+212D (Letterlike Symbols)
	'𝔇': 'd', // U+1D507 MATHEMATICAL FRAKTUR CAPITAL D
	'𝔈': 'e', // U+1D508 MATHEMATICAL FRAKTUR CAPITAL E
	'𝔉': 'f', // U+1D509 MATHEMATICAL FRAKTUR CAPITAL F
	'𝔊': 'g', // U+1D50A MATHEMATICAL FRAKTUR CAPITAL G
	// H is at U+210C (Letterlike Symbols)
	// I is at U+2111 (Letterlike Symbols)
	'𝔍': 'j', // U+1D50D MATHEMATICAL FRAKTUR CAPITAL J
	'𝔎': 'k', // U+1D50E MATHEMATICAL FRAKTUR CAPITAL K
	'𝔏': 'l', // U+1D50F MATHEMATICAL FRAKTUR CAPITAL L
	'𝔐': 'm', // U+1D510 MATHEMATICAL FRAKTUR CAPITAL M
	'𝔑': 'n', // U+1D511 MATHEMATICAL FRAKTUR CAPITAL N
	'𝔒': 'o', // U+1D512 MATHEMATICAL FRAKTUR CAPITAL O
	'𝔓': 'p', // U+1D513 MATHEMATICAL FRAKTUR CAPITAL P
	'𝔔': 'q', // U+1D514 MATHEMATICAL FRAKTUR CAPITAL Q
	// R is at U+211C (Letterlike Symbols)
	'𝔖': 's', // U+1D516 MATHEMATICAL FRAKTUR CAPITAL S
	'𝔗': 't', // U+1D517 MATHEMATICAL FRAKTUR CAPITAL T
	'𝔘': 'u', // U+1D518 MATHEMATICAL FRAKTUR CAPITAL U
	'𝔙': 'v', // U+1D519 MATHEMATICAL FRAKTUR CAPITAL V
	'𝔚': 'w', // U+1D51A MATHEMATICAL FRAKTUR CAPITAL W
	'𝔛': 'x', // U+1D51B MATHEMATICAL FRAKTUR CAPITAL X
	'𝔜': 'y', // U+1D51C MATHEMATICAL FRAKTUR CAPITAL Y
	// Z is at U+2128 (Letterlike Symbols)
}

// frakturLetterlikeToASCII maps the Fraktur characters that live in the
// Letterlike Symbols block (U+2100-U+214F) rather than Mathematical Alphanumeric Symbols.
var frakturLetterlikeToASCII = map[rune]rune{
	'ℭ': 'c', // U+212D BLACK-LETTER CAPITAL C
	'ℌ': 'h', // U+210C BLACK-LETTER CAPITAL H
	'ℑ': 'i', // U+2111 BLACK-LETTER CAPITAL I
	'ℜ': 'r', // U+211C BLACK-LETTER CAPITAL R
	'ℨ': 'z', // U+2128 BLACK-LETTER CAPITAL Z
}

// hasDecorativeUnicode checks if text contains any small caps or fraktur characters
// that would need normalization. Used by migration to identify events needing re-indexing.
func hasDecorativeUnicode(s string) bool {
	for _, r := range s {
		// Check small caps
		if _, ok := smallCapsToASCII[r]; ok {
			return true
		}
		// Check fraktur lowercase range
		if r >= 0x1D51E && r <= 0x1D537 {
			return true
		}
		// Check fraktur uppercase range
		if r >= 0x1D504 && r <= 0x1D51C {
			return true
		}
		// Check letterlike symbols fraktur
		if _, ok := frakturLetterlikeToASCII[r]; ok {
			return true
		}
	}
	return false
}
