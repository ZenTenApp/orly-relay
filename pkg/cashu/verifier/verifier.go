// Package verifier implements Cashu token verification with optional re-authorization.
package verifier

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"next.orly.dev/pkg/cashu/bdhke"
	"next.orly.dev/pkg/cashu/keyset"
	"next.orly.dev/pkg/cashu/token"
	cashuiface "next.orly.dev/pkg/interfaces/cashu"
)

// Errors.
var (
	ErrTokenExpired      = errors.New("verifier: token expired")
	ErrUnknownKeyset     = errors.New("verifier: unknown keyset")
	ErrInvalidSignature  = errors.New("verifier: invalid signature")
	ErrScopeMismatch     = errors.New("verifier: scope mismatch")
	ErrKindNotPermitted  = errors.New("verifier: kind not permitted")
	ErrAccessRevoked     = errors.New("verifier: access revoked")
	ErrMissingToken      = errors.New("verifier: missing token")
)

// Config holds verifier configuration.
type Config struct {
	// Reauthorize enables re-checking authorization on each verification.
	// This provides "stateless revocation" at the cost of an extra check.
	Reauthorize bool
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Reauthorize: true, // Enable stateless revocation by default
	}
}

// Verifier validates Cashu tokens and checks permissions.
type Verifier struct {
	keysets        *keyset.Manager
	authz          cashuiface.AuthzChecker
	claimValidator cashuiface.ClaimValidator
	config         Config
}

// New creates a new verifier.
func New(keysets *keyset.Manager, authz cashuiface.AuthzChecker, config Config) *Verifier {
	return &Verifier{
		keysets: keysets,
		authz:   authz,
		config:  config,
	}
}

// SetClaimValidator sets an optional claim validator.
func (v *Verifier) SetClaimValidator(cv cashuiface.ClaimValidator) {
	v.claimValidator = cv
}

// Verify validates a token's cryptographic signature and checks expiry.
func (v *Verifier) Verify(ctx context.Context, tok *token.Token, remoteAddr string) error {
	// Basic validation
	if err := tok.Validate(); err != nil {
		return err
	}

	// Check expiry
	if tok.IsExpired() {
		return ErrTokenExpired
	}

	// Find keyset
	ks := v.keysets.FindByID(tok.KeysetID)
	if ks == nil {
		return fmt.Errorf("%w: %s", ErrUnknownKeyset, tok.KeysetID)
	}

	// Verify signature
	valid, err := v.verifySignature(tok, ks)
	if err != nil {
		return fmt.Errorf("verifier: signature check failed: %w", err)
	}
	if !valid {
		return ErrInvalidSignature
	}

	// Re-check authorization if enabled
	if v.config.Reauthorize && v.authz != nil {
		if err := v.authz.CheckAuthorization(ctx, tok.Pubkey, tok.Scope, remoteAddr); err != nil {
			return fmt.Errorf("%w: %v", ErrAccessRevoked, err)
		}
	}

	return nil
}

// VerifyForScope verifies a token and checks that it has the required scope.
func (v *Verifier) VerifyForScope(ctx context.Context, tok *token.Token, requiredScope string, remoteAddr string) error {
	if err := v.Verify(ctx, tok, remoteAddr); err != nil {
		return err
	}

	if !tok.MatchesScope(requiredScope) {
		return fmt.Errorf("%w: expected %s, got %s", ErrScopeMismatch, requiredScope, tok.Scope)
	}

	return nil
}

// VerifyForKind verifies a token and checks that the specified kind is permitted.
func (v *Verifier) VerifyForKind(ctx context.Context, tok *token.Token, kind int, remoteAddr string) error {
	if err := v.Verify(ctx, tok, remoteAddr); err != nil {
		return err
	}

	if !tok.IsKindPermitted(kind) {
		return fmt.Errorf("%w: kind %d", ErrKindNotPermitted, kind)
	}

	return nil
}

// verifySignature checks the BDHKE signature.
func (v *Verifier) verifySignature(tok *token.Token, ks *keyset.Keyset) (bool, error) {
	// Parse signature as curve point
	C, err := secp256k1.ParsePubKey(tok.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature format: %w", err)
	}

	// Verify: C == k * HashToCurve(secret)
	return bdhke.Verify(tok.Secret, C, ks.PrivateKey)
}

// ExtractFromRequest extracts and parses a token from an HTTP request.
// Checks headers in order: X-Cashu-Token, Authorization (Cashu scheme).
func (v *Verifier) ExtractFromRequest(r *http.Request) (*token.Token, error) {
	// Try X-Cashu-Token header first
	if header := r.Header.Get("X-Cashu-Token"); header != "" {
		return token.ParseFromHeader(header)
	}

	// Try Authorization header
	if header := r.Header.Get("Authorization"); header != "" {
		return token.ParseFromHeader(header)
	}

	return nil, ErrMissingToken
}

// VerifyRequest extracts, parses, and verifies a token from an HTTP request.
func (v *Verifier) VerifyRequest(ctx context.Context, r *http.Request, requiredScope string) (*token.Token, error) {
	tok, err := v.ExtractFromRequest(r)
	if err != nil {
		return nil, err
	}

	if err := v.VerifyForScope(ctx, tok, requiredScope, r.RemoteAddr); err != nil {
		return nil, err
	}

	return tok, nil
}

// VerifyRequestForKind extracts, parses, and verifies a token for a specific kind.
func (v *Verifier) VerifyRequestForKind(ctx context.Context, r *http.Request, requiredScope string, kind int) (*token.Token, error) {
	tok, err := v.ExtractFromRequest(r)
	if err != nil {
		return nil, err
	}

	if err := v.VerifyForScope(ctx, tok, requiredScope, r.RemoteAddr); err != nil {
		return nil, err
	}

	if !tok.IsKindPermitted(kind) {
		return nil, fmt.Errorf("%w: kind %d", ErrKindNotPermitted, kind)
	}

	return tok, nil
}
