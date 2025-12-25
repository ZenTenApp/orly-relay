package validation

import (
	"bytes"
	"fmt"
)

// ValidateLowercaseHexInJSON checks that all hex-encoded fields in the raw JSON are lowercase.
// NIP-01 specifies that hex encoding must be lowercase.
// This must be called on the raw message BEFORE unmarshaling, since unmarshal converts
// hex strings to binary and loses case information.
// Returns an error message if validation fails, or empty string if valid.
func ValidateLowercaseHexInJSON(msg []byte) string {
	// Find and validate "id" field (64 hex chars)
	if err := validateJSONHexField(msg, `"id"`); err != "" {
		return err + " (id)"
	}

	// Find and validate "pubkey" field (64 hex chars)
	if err := validateJSONHexField(msg, `"pubkey"`); err != "" {
		return err + " (pubkey)"
	}

	// Find and validate "sig" field (128 hex chars)
	if err := validateJSONHexField(msg, `"sig"`); err != "" {
		return err + " (sig)"
	}

	// Validate e and p tags in the tags array
	// Tags format: ["e", "hexvalue", ...] or ["p", "hexvalue", ...]
	if err := validateEPTagsInJSON(msg); err != "" {
		return err
	}

	return "" // Valid
}

// validateJSONHexField finds a JSON field and checks if its hex value contains uppercase.
func validateJSONHexField(msg []byte, fieldName string) string {
	// Find the field name
	idx := bytes.Index(msg, []byte(fieldName))
	if idx == -1 {
		return "" // Field not found, skip
	}

	// Find the colon after the field name
	colonIdx := bytes.Index(msg[idx:], []byte(":"))
	if colonIdx == -1 {
		return ""
	}

	// Find the opening quote of the value
	valueStart := idx + colonIdx + 1
	for valueStart < len(msg) && (msg[valueStart] == ' ' || msg[valueStart] == '\t' || msg[valueStart] == '\n' || msg[valueStart] == '\r') {
		valueStart++
	}
	if valueStart >= len(msg) || msg[valueStart] != '"' {
		return ""
	}
	valueStart++ // Skip the opening quote

	// Find the closing quote
	valueEnd := valueStart
	for valueEnd < len(msg) && msg[valueEnd] != '"' {
		valueEnd++
	}

	// Extract the hex value and check for uppercase
	hexValue := msg[valueStart:valueEnd]
	if containsUppercaseHex(hexValue) {
		return "blocked: hex fields may only be lower case, see NIP-01"
	}

	return ""
}

// validateEPTagsInJSON checks e and p tags in the JSON for uppercase hex.
func validateEPTagsInJSON(msg []byte) string {
	// Find the tags array
	tagsIdx := bytes.Index(msg, []byte(`"tags"`))
	if tagsIdx == -1 {
		return "" // No tags
	}

	// Find the opening bracket of the tags array
	bracketIdx := bytes.Index(msg[tagsIdx:], []byte("["))
	if bracketIdx == -1 {
		return ""
	}

	tagsStart := tagsIdx + bracketIdx

	// Scan through to find ["e", ...] and ["p", ...] patterns
	// This is a simplified parser that looks for specific patterns
	pos := tagsStart
	for pos < len(msg) {
		// Look for ["e" or ["p" pattern
		eTagPattern := bytes.Index(msg[pos:], []byte(`["e"`))
		pTagPattern := bytes.Index(msg[pos:], []byte(`["p"`))

		var tagType string
		var nextIdx int

		if eTagPattern == -1 && pTagPattern == -1 {
			break // No more e or p tags
		} else if eTagPattern == -1 {
			nextIdx = pos + pTagPattern
			tagType = "p"
		} else if pTagPattern == -1 {
			nextIdx = pos + eTagPattern
			tagType = "e"
		} else if eTagPattern < pTagPattern {
			nextIdx = pos + eTagPattern
			tagType = "e"
		} else {
			nextIdx = pos + pTagPattern
			tagType = "p"
		}

		// Find the hex value after the tag type
		// Pattern: ["e", "hexvalue" or ["p", "hexvalue"
		commaIdx := bytes.Index(msg[nextIdx:], []byte(","))
		if commaIdx == -1 {
			pos = nextIdx + 4
			continue
		}

		// Find the opening quote of the hex value
		valueStart := nextIdx + commaIdx + 1
		for valueStart < len(msg) && (msg[valueStart] == ' ' || msg[valueStart] == '\t' || msg[valueStart] == '"') {
			if msg[valueStart] == '"' {
				valueStart++
				break
			}
			valueStart++
		}

		// Find the closing quote
		valueEnd := valueStart
		for valueEnd < len(msg) && msg[valueEnd] != '"' {
			valueEnd++
		}

		// Check if this looks like a hex value (64 chars for pubkey/event ID)
		hexValue := msg[valueStart:valueEnd]
		if len(hexValue) == 64 && containsUppercaseHex(hexValue) {
			return fmt.Sprintf("blocked: hex fields may only be lower case, see NIP-01 (%s tag)", tagType)
		}

		pos = valueEnd + 1
	}

	return ""
}

// containsUppercaseHex checks if a byte slice (representing hex) contains uppercase letters A-F.
func containsUppercaseHex(b []byte) bool {
	for _, c := range b {
		if c >= 'A' && c <= 'F' {
			return true
		}
	}
	return false
}
