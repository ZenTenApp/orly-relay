// Package token implements the Cashu access token format as defined in NIP-XX.
// Tokens are privacy-preserving bearer credentials with kind permissions.
package token

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Prefix for serialized tokens.
const Prefix = "cashuA"

// Predefined scopes.
const (
	ScopeRelay   = "relay"   // Standard relay WebSocket access
	ScopeNIP46   = "nip46"   // NIP-46 remote signing / bunker
	ScopeBlossom = "blossom" // Blossom media server
	ScopeAPI     = "api"     // HTTP API access
	ScopeNRC     = "nrc"     // Nostr Relay Connect tunneling
)

// WildcardKind indicates all kinds are permitted.
const WildcardKind = -1

// Errors.
var (
	ErrInvalidPrefix    = errors.New("token: invalid prefix, expected cashuA")
	ErrInvalidEncoding  = errors.New("token: invalid base64url encoding")
	ErrInvalidJSON      = errors.New("token: invalid JSON structure")
	ErrTokenExpired     = errors.New("token: expired")
	ErrKindNotPermitted = errors.New("token: kind not permitted")
	ErrScopeMismatch    = errors.New("token: scope mismatch")
)

// Token represents a Cashu access token with kind permissions.
type Token struct {
	// Cryptographic fields
	KeysetID  string `json:"k"`  // Keyset ID (hex)
	Secret    []byte `json:"s"`  // Random secret (32 bytes)
	Signature []byte `json:"c"`  // Blind signature (33 bytes compressed)
	Pubkey    []byte `json:"p"`  // User's Nostr pubkey (32 bytes)

	// Metadata
	Expiry int64  `json:"e"`  // Unix timestamp when token expires
	Scope  string `json:"sc"` // Token scope (relay, nip46, etc.)

	// Kind permissions
	Kinds      []int   `json:"kinds,omitempty"`       // Explicit list of permitted kinds
	KindRanges [][]int `json:"kind_ranges,omitempty"` // Ranges as [min, max] pairs
}

// tokenJSON is the JSON-serializable form with hex-encoded bytes.
type tokenJSON struct {
	KeysetID   string  `json:"k"`
	Secret     string  `json:"s"`
	Signature  string  `json:"c"`
	Pubkey     string  `json:"p"`
	Expiry     int64   `json:"e"`
	Scope      string  `json:"sc"`
	Kinds      []int   `json:"kinds,omitempty"`
	KindRanges [][]int `json:"kind_ranges,omitempty"`
}

// New creates a new token with the given parameters.
func New(keysetID string, secret, signature, pubkey []byte, expiry time.Time, scope string) *Token {
	return &Token{
		KeysetID:  keysetID,
		Secret:    secret,
		Signature: signature,
		Pubkey:    pubkey,
		Expiry:    expiry.Unix(),
		Scope:     scope,
	}
}

// SetKinds sets explicit permitted kinds.
// Use WildcardKind (-1) to allow all kinds.
func (t *Token) SetKinds(kinds ...int) {
	t.Kinds = kinds
}

// SetKindRanges sets permitted kind ranges.
// Each range is [min, max] inclusive.
func (t *Token) SetKindRanges(ranges ...[]int) {
	t.KindRanges = ranges
}

// AddKindRange adds a single kind range.
func (t *Token) AddKindRange(min, max int) {
	t.KindRanges = append(t.KindRanges, []int{min, max})
}

// IsExpired returns true if the token has expired.
func (t *Token) IsExpired() bool {
	return time.Now().Unix() > t.Expiry
}

// ExpiresAt returns the expiry time.
func (t *Token) ExpiresAt() time.Time {
	return time.Unix(t.Expiry, 0)
}

// TimeRemaining returns the duration until expiry.
func (t *Token) TimeRemaining() time.Duration {
	return time.Until(t.ExpiresAt())
}

// IsKindPermitted checks if a given event kind is permitted by this token.
func (t *Token) IsKindPermitted(kind int) bool {
	// Check for wildcard
	for _, k := range t.Kinds {
		if k == WildcardKind {
			return true
		}
	}

	// Check explicit kinds
	for _, k := range t.Kinds {
		if k == kind {
			return true
		}
	}

	// Check kind ranges
	for _, r := range t.KindRanges {
		if len(r) >= 2 && kind >= r[0] && kind <= r[1] {
			return true
		}
	}

	// If no kinds or ranges specified, check scope defaults
	if len(t.Kinds) == 0 && len(t.KindRanges) == 0 {
		return t.defaultKindPermitted(kind)
	}

	return false
}

// defaultKindPermitted returns default permissions based on scope.
func (t *Token) defaultKindPermitted(kind int) bool {
	switch t.Scope {
	case ScopeRelay:
		// Default relay scope allows common kinds
		return true
	case ScopeNIP46:
		// NIP-46 scope allows NIP-46 kinds (24133)
		return kind == 24133
	case ScopeBlossom:
		// Blossom scope allows auth kinds
		return kind == 24242
	default:
		return false
	}
}

// HasWritePermission returns true if any kind is permitted (not read-only).
func (t *Token) HasWritePermission() bool {
	return len(t.Kinds) > 0 || len(t.KindRanges) > 0
}

// IsReadOnly returns true if no kinds are permitted.
func (t *Token) IsReadOnly() bool {
	return !t.HasWritePermission()
}

// MatchesScope checks if the token scope matches the required scope.
func (t *Token) MatchesScope(requiredScope string) bool {
	return t.Scope == requiredScope
}

// PubkeyHex returns the pubkey as a hex string.
func (t *Token) PubkeyHex() string {
	return hex.EncodeToString(t.Pubkey)
}

// Encode serializes the token to the wire format: cashuA<base64url(json)>
func (t *Token) Encode() (string, error) {
	// Convert to JSON-friendly format
	tj := tokenJSON{
		KeysetID:   t.KeysetID,
		Secret:     hex.EncodeToString(t.Secret),
		Signature:  hex.EncodeToString(t.Signature),
		Pubkey:     hex.EncodeToString(t.Pubkey),
		Expiry:     t.Expiry,
		Scope:      t.Scope,
		Kinds:      t.Kinds,
		KindRanges: t.KindRanges,
	}

	jsonBytes, err := json.Marshal(tj)
	if err != nil {
		return "", fmt.Errorf("token: failed to encode: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(jsonBytes)
	return Prefix + encoded, nil
}

// Parse decodes a token from the wire format.
func Parse(s string) (*Token, error) {
	// Check prefix
	if !strings.HasPrefix(s, Prefix) {
		return nil, ErrInvalidPrefix
	}

	// Decode base64url
	encoded := strings.TrimPrefix(s, Prefix)
	jsonBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEncoding, err)
	}

	// Parse JSON
	var tj tokenJSON
	if err := json.Unmarshal(jsonBytes, &tj); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	// Decode hex fields
	secret, err := hex.DecodeString(tj.Secret)
	if err != nil {
		return nil, fmt.Errorf("token: invalid secret hex: %w", err)
	}

	signature, err := hex.DecodeString(tj.Signature)
	if err != nil {
		return nil, fmt.Errorf("token: invalid signature hex: %w", err)
	}

	pubkey, err := hex.DecodeString(tj.Pubkey)
	if err != nil {
		return nil, fmt.Errorf("token: invalid pubkey hex: %w", err)
	}

	return &Token{
		KeysetID:   tj.KeysetID,
		Secret:     secret,
		Signature:  signature,
		Pubkey:     pubkey,
		Expiry:     tj.Expiry,
		Scope:      tj.Scope,
		Kinds:      tj.Kinds,
		KindRanges: tj.KindRanges,
	}, nil
}

// ParseFromHeader extracts and parses a token from HTTP headers.
// Supports:
//   - X-Cashu-Token: cashuA...
//   - Authorization: Cashu cashuA...
func ParseFromHeader(header string) (*Token, error) {
	// Try X-Cashu-Token format (raw token)
	if strings.HasPrefix(header, Prefix) {
		return Parse(header)
	}

	// Try Authorization format
	if strings.HasPrefix(header, "Cashu ") {
		tokenStr := strings.TrimPrefix(header, "Cashu ")
		return Parse(strings.TrimSpace(tokenStr))
	}

	return nil, ErrInvalidPrefix
}

// Validate performs basic validation on the token.
// Does NOT verify the cryptographic signature - use Verifier for that.
func (t *Token) Validate() error {
	if t.IsExpired() {
		return ErrTokenExpired
	}

	if len(t.KeysetID) != 14 {
		return fmt.Errorf("token: invalid keyset ID length: %d", len(t.KeysetID))
	}

	if len(t.Secret) != 32 {
		return fmt.Errorf("token: invalid secret length: %d", len(t.Secret))
	}

	if len(t.Signature) != 33 {
		return fmt.Errorf("token: invalid signature length: %d", len(t.Signature))
	}

	if len(t.Pubkey) != 32 {
		return fmt.Errorf("token: invalid pubkey length: %d", len(t.Pubkey))
	}

	if t.Scope == "" {
		return errors.New("token: missing scope")
	}

	// Validate kind ranges
	for i, r := range t.KindRanges {
		if len(r) != 2 {
			return fmt.Errorf("token: kind range %d must have 2 elements", i)
		}
		if r[0] > r[1] {
			return fmt.Errorf("token: kind range %d min > max: %d > %d", i, r[0], r[1])
		}
	}

	return nil
}

// Clone creates a copy of the token.
func (t *Token) Clone() *Token {
	clone := &Token{
		KeysetID: t.KeysetID,
		Secret:   make([]byte, len(t.Secret)),
		Signature: make([]byte, len(t.Signature)),
		Pubkey:   make([]byte, len(t.Pubkey)),
		Expiry:   t.Expiry,
		Scope:    t.Scope,
	}

	copy(clone.Secret, t.Secret)
	copy(clone.Signature, t.Signature)
	copy(clone.Pubkey, t.Pubkey)

	if len(t.Kinds) > 0 {
		clone.Kinds = make([]int, len(t.Kinds))
		copy(clone.Kinds, t.Kinds)
	}

	if len(t.KindRanges) > 0 {
		clone.KindRanges = make([][]int, len(t.KindRanges))
		for i, r := range t.KindRanges {
			clone.KindRanges[i] = make([]int, len(r))
			copy(clone.KindRanges[i], r)
		}
	}

	return clone
}

// String returns the encoded token string.
func (t *Token) String() string {
	s, _ := t.Encode()
	return s
}
