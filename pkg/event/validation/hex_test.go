package validation

import "testing"

func TestContainsUppercaseHex(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"lowercase only", []byte("abcdef0123456789"), false},
		{"uppercase A", []byte("Abcdef0123456789"), true},
		{"uppercase F", []byte("abcdeF0123456789"), true},
		{"mixed uppercase", []byte("ABCDEF"), true},
		{"numbers only", []byte("0123456789"), false},
		{"lowercase with numbers", []byte("abc123def456"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsUppercaseHex(tt.input)
			if result != tt.expected {
				t.Errorf("containsUppercaseHex(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateLowercaseHexInJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		wantError bool
	}{
		{
			name:      "valid lowercase",
			json:      []byte(`{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","sig":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}`),
			wantError: false,
		},
		{
			name:      "uppercase in id",
			json:      []byte(`{"id":"ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"}`),
			wantError: true,
		},
		{
			name:      "uppercase in pubkey",
			json:      []byte(`{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"FEDCBA9876543210fedcba9876543210fedcba9876543210fedcba9876543210"}`),
			wantError: true,
		},
		{
			name:      "uppercase in sig",
			json:      []byte(`{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","sig":"ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}`),
			wantError: true,
		},
		{
			name:      "no hex fields",
			json:      []byte(`{"kind":1,"content":"hello"}`),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateLowercaseHexInJSON(tt.json)
			hasError := result != ""
			if hasError != tt.wantError {
				t.Errorf("ValidateLowercaseHexInJSON() error = %v, wantError %v, msg: %s", hasError, tt.wantError, result)
			}
		})
	}
}

func TestValidateEPTagsInJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		wantError bool
	}{
		{
			name:      "valid lowercase e tag",
			json:      []byte(`{"tags":[["e","abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"]]}`),
			wantError: false,
		},
		{
			name:      "valid lowercase p tag",
			json:      []byte(`{"tags":[["p","abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"]]}`),
			wantError: false,
		},
		{
			name:      "uppercase in e tag",
			json:      []byte(`{"tags":[["e","ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789"]]}`),
			wantError: true,
		},
		{
			name:      "uppercase in p tag",
			json:      []byte(`{"tags":[["p","ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789"]]}`),
			wantError: true,
		},
		{
			name:      "mixed valid tags",
			json:      []byte(`{"tags":[["e","abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"],["p","fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"]]}`),
			wantError: false,
		},
		{
			name:      "no tags",
			json:      []byte(`{"kind":1,"content":"hello"}`),
			wantError: false,
		},
		{
			name:      "non-hex tag value",
			json:      []byte(`{"tags":[["t","sometag"]]}`),
			wantError: false, // Non e/p tags are not checked
		},
		{
			name:      "short e tag value",
			json:      []byte(`{"tags":[["e","short"]]}`),
			wantError: false, // Short values are not 64 chars so skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateEPTagsInJSON(tt.json)
			hasError := result != ""
			if hasError != tt.wantError {
				t.Errorf("validateEPTagsInJSON() error = %v, wantError %v, msg: %s", hasError, tt.wantError, result)
			}
		})
	}
}

func TestValidateJSONHexField(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		fieldName string
		wantError bool
	}{
		{
			name:      "valid lowercase id",
			json:      []byte(`{"id":"abcdef0123456789"}`),
			fieldName: `"id"`,
			wantError: false,
		},
		{
			name:      "uppercase in field",
			json:      []byte(`{"id":"ABCDEF0123456789"}`),
			fieldName: `"id"`,
			wantError: true,
		},
		{
			name:      "field not found",
			json:      []byte(`{"other":"value"}`),
			fieldName: `"id"`,
			wantError: false,
		},
		{
			name:      "field with whitespace",
			json:      []byte(`{"id": "abcdef0123456789"}`),
			fieldName: `"id"`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateJSONHexField(tt.json, tt.fieldName)
			hasError := result != ""
			if hasError != tt.wantError {
				t.Errorf("validateJSONHexField() error = %v, wantError %v, msg: %s", hasError, tt.wantError, result)
			}
		})
	}
}
