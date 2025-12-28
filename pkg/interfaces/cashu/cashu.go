// Package cashu defines interfaces for the Cashu access token system.
// Implement these interfaces to integrate with your authorization backend.
package cashu

import (
	"context"
)

// AuthzChecker determines if a pubkey is authorized for a given scope.
// Implement this interface to integrate with your access control system.
type AuthzChecker interface {
	// CheckAuthorization returns nil if the pubkey is authorized for the scope,
	// or an error describing why authorization failed.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - pubkey: User's Nostr pubkey (32 bytes)
	//   - scope: Token scope (e.g., "relay", "nip46", "api")
	//   - remoteAddr: Client's remote address (for IP-based checks)
	//
	// The implementation should check if the user has sufficient permissions
	// for the requested scope. This is called during token issuance.
	CheckAuthorization(ctx context.Context, pubkey []byte, scope string, remoteAddr string) error
}

// ReauthorizationChecker is an optional extension of AuthzChecker that
// supports re-checking authorization during token verification.
// This enables "stateless revocation" - tokens become invalid immediately
// when the user is removed from the access list.
type ReauthorizationChecker interface {
	AuthzChecker

	// ReauthorizationEnabled returns true if authorization should be
	// re-checked on every token verification.
	ReauthorizationEnabled() bool
}

// ClaimValidator validates custom claims in tokens.
// Implement this for application-specific claim validation.
type ClaimValidator interface {
	// ValidateClaims validates custom claims embedded in a token.
	// Returns nil if claims are valid, error otherwise.
	ValidateClaims(claims map[string]any) error
}

// KindPermissionChecker validates event kind permissions.
// This is typically implemented by the token itself, but can be
// extended for additional validation logic.
type KindPermissionChecker interface {
	// IsKindPermitted returns true if the given event kind is allowed.
	IsKindPermitted(kind int) bool

	// HasWritePermission returns true if any kinds are permitted.
	HasWritePermission() bool
}

// Common error types that implementations may return.
type AuthzError struct {
	Code    string
	Message string
}

func (e *AuthzError) Error() string {
	return e.Message
}

// Predefined authorization error codes.
const (
	ErrCodeNotAuthorized   = "not_authorized"
	ErrCodeBanned          = "banned"
	ErrCodeBlocked         = "blocked"
	ErrCodeInvalidScope    = "invalid_scope"
	ErrCodeRateLimited     = "rate_limited"
	ErrCodeInsufficientAccess = "insufficient_access"
)

// NewAuthzError creates a new authorization error.
func NewAuthzError(code, message string) *AuthzError {
	return &AuthzError{Code: code, Message: message}
}

// Common authorization errors.
var (
	ErrNotAuthorized = NewAuthzError(ErrCodeNotAuthorized, "not authorized for this scope")
	ErrBanned        = NewAuthzError(ErrCodeBanned, "user is banned")
	ErrBlocked       = NewAuthzError(ErrCodeBlocked, "IP address is blocked")
	ErrInvalidScope  = NewAuthzError(ErrCodeInvalidScope, "invalid scope requested")
)

// AllowAllChecker is a simple implementation that allows all requests.
// Useful for testing or open relays.
type AllowAllChecker struct{}

// CheckAuthorization always returns nil (allowed).
func (AllowAllChecker) CheckAuthorization(ctx context.Context, pubkey []byte, scope string, remoteAddr string) error {
	return nil
}

// DenyAllChecker is a simple implementation that denies all requests.
// Useful for testing or temporarily disabling token issuance.
type DenyAllChecker struct{}

// CheckAuthorization always returns ErrNotAuthorized.
func (DenyAllChecker) CheckAuthorization(ctx context.Context, pubkey []byte, scope string, remoteAddr string) error {
	return ErrNotAuthorized
}
