package token

import (
	"encoding/hex"
	"testing"
	"time"
)

func makeTestToken() *Token {
	secret := make([]byte, 32)
	signature := make([]byte, 33)
	pubkey := make([]byte, 32)

	for i := range secret {
		secret[i] = byte(i)
	}
	for i := range signature {
		signature[i] = byte(i + 32)
	}
	for i := range pubkey {
		pubkey[i] = byte(i + 64)
	}

	signature[0] = 0x02 // Valid compressed point prefix

	return New(
		"0a1b2c3d4e5f67",
		secret,
		signature,
		pubkey,
		time.Now().Add(time.Hour),
		ScopeRelay,
	)
}

func TestTokenEncodeDecode(t *testing.T) {
	tok := makeTestToken()
	tok.SetKinds(0, 1, 3, 7)
	tok.AddKindRange(30000, 39999)

	encoded, err := tok.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Should have correct prefix
	if encoded[:6] != Prefix {
		t.Errorf("Encoded token should start with %s, got %s", Prefix, encoded[:6])
	}

	// Decode
	decoded, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Compare fields
	if decoded.KeysetID != tok.KeysetID {
		t.Errorf("KeysetID mismatch: %s != %s", decoded.KeysetID, tok.KeysetID)
	}
	if hex.EncodeToString(decoded.Secret) != hex.EncodeToString(tok.Secret) {
		t.Error("Secret mismatch")
	}
	if hex.EncodeToString(decoded.Signature) != hex.EncodeToString(tok.Signature) {
		t.Error("Signature mismatch")
	}
	if hex.EncodeToString(decoded.Pubkey) != hex.EncodeToString(tok.Pubkey) {
		t.Error("Pubkey mismatch")
	}
	if decoded.Expiry != tok.Expiry {
		t.Errorf("Expiry mismatch: %d != %d", decoded.Expiry, tok.Expiry)
	}
	if decoded.Scope != tok.Scope {
		t.Errorf("Scope mismatch: %s != %s", decoded.Scope, tok.Scope)
	}

	// Check kinds
	if len(decoded.Kinds) != len(tok.Kinds) {
		t.Errorf("Kinds length mismatch: %d != %d", len(decoded.Kinds), len(tok.Kinds))
	}
	for i, k := range decoded.Kinds {
		if k != tok.Kinds[i] {
			t.Errorf("Kinds[%d] mismatch: %d != %d", i, k, tok.Kinds[i])
		}
	}

	// Check kind ranges
	if len(decoded.KindRanges) != len(tok.KindRanges) {
		t.Errorf("KindRanges length mismatch: %d != %d", len(decoded.KindRanges), len(tok.KindRanges))
	}
}

func TestTokenKindPermissions(t *testing.T) {
	tok := makeTestToken()
	tok.SetKinds(0, 1, 3)
	tok.AddKindRange(30000, 39999)

	tests := []struct {
		kind     int
		expected bool
	}{
		{0, true},          // Explicit kind
		{1, true},          // Explicit kind
		{3, true},          // Explicit kind
		{2, false},         // Not in list
		{7, false},         // Not in list
		{30000, true},      // Start of range
		{35000, true},      // Middle of range
		{39999, true},      // End of range
		{29999, false},     // Just before range
		{40000, false},     // Just after range
	}

	for _, tt := range tests {
		result := tok.IsKindPermitted(tt.kind)
		if result != tt.expected {
			t.Errorf("IsKindPermitted(%d) = %v, want %v", tt.kind, result, tt.expected)
		}
	}
}

func TestTokenWildcardKind(t *testing.T) {
	tok := makeTestToken()
	tok.SetKinds(WildcardKind)

	// All kinds should be permitted
	for _, kind := range []int{0, 1, 100, 1000, 30000, 65535} {
		if !tok.IsKindPermitted(kind) {
			t.Errorf("Wildcard should permit kind %d", kind)
		}
	}
}

func TestTokenReadOnly(t *testing.T) {
	tok := makeTestToken()

	// No kinds set - should be read-only by kinds check
	if tok.HasWritePermission() {
		t.Error("Token with no kinds should not have write permission")
	}

	tok.SetKinds(1)
	if !tok.HasWritePermission() {
		t.Error("Token with kinds should have write permission")
	}
}

func TestTokenExpiry(t *testing.T) {
	// Token that expires in 1 hour
	tok := makeTestToken()
	if tok.IsExpired() {
		t.Error("Token should not be expired yet")
	}

	// Token that expired 1 hour ago
	tok.Expiry = time.Now().Add(-time.Hour).Unix()
	if !tok.IsExpired() {
		t.Error("Token should be expired")
	}
}

func TestTokenTimeRemaining(t *testing.T) {
	tok := makeTestToken()
	remaining := tok.TimeRemaining()

	// Should be close to 1 hour
	if remaining < 59*time.Minute || remaining > 61*time.Minute {
		t.Errorf("TimeRemaining = %v, expected ~1 hour", remaining)
	}
}

func TestTokenValidate(t *testing.T) {
	// Valid token
	tok := makeTestToken()
	if err := tok.Validate(); err != nil {
		t.Errorf("Validate failed for valid token: %v", err)
	}

	// Expired token
	expired := makeTestToken()
	expired.Expiry = time.Now().Add(-time.Hour).Unix()
	if err := expired.Validate(); err != ErrTokenExpired {
		t.Errorf("Validate should return ErrTokenExpired, got %v", err)
	}

	// Invalid keyset ID
	badKeyset := makeTestToken()
	badKeyset.KeysetID = "short"
	if err := badKeyset.Validate(); err == nil {
		t.Error("Validate should fail for short keyset ID")
	}

	// Invalid secret length
	badSecret := makeTestToken()
	badSecret.Secret = []byte{1, 2, 3}
	if err := badSecret.Validate(); err == nil {
		t.Error("Validate should fail for wrong secret length")
	}

	// Invalid kind range
	badRange := makeTestToken()
	badRange.KindRanges = [][]int{{100, 50}} // min > max
	if err := badRange.Validate(); err == nil {
		t.Error("Validate should fail for invalid kind range")
	}
}

func TestParseFromHeader(t *testing.T) {
	tok := makeTestToken()
	encoded, _ := tok.Encode()

	// Test X-Cashu-Token format
	parsed, err := ParseFromHeader(encoded)
	if err != nil {
		t.Fatalf("ParseFromHeader failed for raw token: %v", err)
	}
	if parsed.KeysetID != tok.KeysetID {
		t.Error("Parsed token has wrong KeysetID")
	}

	// Test Authorization format
	parsed, err = ParseFromHeader("Cashu " + encoded)
	if err != nil {
		t.Fatalf("ParseFromHeader failed for Authorization format: %v", err)
	}
	if parsed.KeysetID != tok.KeysetID {
		t.Error("Parsed token has wrong KeysetID")
	}

	// Test invalid format
	_, err = ParseFromHeader("Bearer xyz")
	if err != ErrInvalidPrefix {
		t.Errorf("Expected ErrInvalidPrefix, got %v", err)
	}
}

func TestTokenClone(t *testing.T) {
	tok := makeTestToken()
	tok.SetKinds(1, 2, 3)
	tok.AddKindRange(100, 200)

	clone := tok.Clone()

	// Modify original
	tok.Secret[0] = 0xFF
	tok.Kinds[0] = 999
	tok.KindRanges[0][0] = 999

	// Clone should be unchanged
	if clone.Secret[0] == 0xFF {
		t.Error("Clone secret was modified when original changed")
	}
	if clone.Kinds[0] == 999 {
		t.Error("Clone kinds was modified when original changed")
	}
	if clone.KindRanges[0][0] == 999 {
		t.Error("Clone kind ranges was modified when original changed")
	}
}

func TestTokenMatchesScope(t *testing.T) {
	tok := makeTestToken()
	tok.Scope = ScopeNIP46

	if !tok.MatchesScope(ScopeNIP46) {
		t.Error("Should match ScopeNIP46")
	}
	if tok.MatchesScope(ScopeRelay) {
		t.Error("Should not match ScopeRelay")
	}
}

func TestTokenPubkeyHex(t *testing.T) {
	tok := makeTestToken()
	hexPubkey := tok.PubkeyHex()

	// Should be 64 characters (32 bytes * 2)
	if len(hexPubkey) != 64 {
		t.Errorf("PubkeyHex length = %d, want 64", len(hexPubkey))
	}

	// Should decode back to original
	decoded, err := hex.DecodeString(hexPubkey)
	if err != nil {
		t.Fatalf("PubkeyHex is not valid hex: %v", err)
	}
	for i, b := range decoded {
		if b != tok.Pubkey[i] {
			t.Errorf("PubkeyHex[%d] mismatch", i)
		}
	}
}

func TestTokenString(t *testing.T) {
	tok := makeTestToken()
	s := tok.String()

	if s[:6] != Prefix {
		t.Errorf("String() should start with prefix, got %s", s[:6])
	}
}

func BenchmarkTokenEncode(b *testing.B) {
	tok := makeTestToken()
	tok.SetKinds(0, 1, 3, 7)
	tok.AddKindRange(30000, 39999)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tok.Encode()
	}
}

func BenchmarkTokenParse(b *testing.B) {
	tok := makeTestToken()
	tok.SetKinds(0, 1, 3, 7)
	tok.AddKindRange(30000, 39999)
	encoded, _ := tok.Encode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(encoded)
	}
}

func BenchmarkTokenIsKindPermitted(b *testing.B) {
	tok := makeTestToken()
	tok.SetKinds(0, 1, 3, 7, 10, 20, 30, 40, 50)
	tok.AddKindRange(30000, 39999)
	tok.AddKindRange(20000, 29999)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tok.IsKindPermitted(35000)
	}
}
