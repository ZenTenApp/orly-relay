package verifier

import (
	"context"
	"net/http"

	"next.orly.dev/pkg/cashu/token"
)

// ContextKey is the type for context keys.
type ContextKey string

const (
	// TokenContextKey is the context key for the verified token.
	TokenContextKey ContextKey = "cashu_token"

	// PubkeyContextKey is the context key for the user's pubkey.
	PubkeyContextKey ContextKey = "cashu_pubkey"
)

// TokenFromContext extracts the verified token from the request context.
func TokenFromContext(ctx context.Context) *token.Token {
	if tok, ok := ctx.Value(TokenContextKey).(*token.Token); ok {
		return tok
	}
	return nil
}

// PubkeyFromContext extracts the user's pubkey from the request context.
func PubkeyFromContext(ctx context.Context) []byte {
	if pubkey, ok := ctx.Value(PubkeyContextKey).([]byte); ok {
		return pubkey
	}
	return nil
}

// Middleware creates an HTTP middleware that verifies Cashu tokens.
func Middleware(v *Verifier, requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := v.VerifyRequest(r.Context(), r, requiredScope)
			if err != nil {
				writeError(w, err)
				return
			}

			// Add token and pubkey to context
			ctx := context.WithValue(r.Context(), TokenContextKey, tok)
			ctx = context.WithValue(ctx, PubkeyContextKey, tok.Pubkey)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareForKind creates middleware that also checks kind permission.
func MiddlewareForKind(v *Verifier, requiredScope string, kind int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := v.VerifyRequestForKind(r.Context(), r, requiredScope, kind)
			if err != nil {
				writeError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), TokenContextKey, tok)
			ctx = context.WithValue(ctx, PubkeyContextKey, tok.Pubkey)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalMiddleware creates middleware that verifies tokens if present,
// but allows requests without tokens to proceed.
func OptionalMiddleware(v *Verifier, requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := v.VerifyRequest(r.Context(), r, requiredScope)
			if err == nil {
				// Token present and valid - add to context
				ctx := context.WithValue(r.Context(), TokenContextKey, tok)
				ctx = context.WithValue(ctx, PubkeyContextKey, tok.Pubkey)
				r = r.WithContext(ctx)
			} else if err != ErrMissingToken {
				// Token present but invalid - reject
				writeError(w, err)
				return
			}
			// No token or valid token - proceed

			next.ServeHTTP(w, r)
		})
	}
}

// writeError writes an appropriate HTTP error response.
func writeError(w http.ResponseWriter, err error) {
	switch err {
	case ErrMissingToken:
		http.Error(w, "Missing token", http.StatusUnauthorized)
	case ErrTokenExpired:
		http.Error(w, "Token expired", http.StatusGone)
	case ErrUnknownKeyset:
		http.Error(w, "Unknown keyset", http.StatusMisdirectedRequest)
	case ErrInvalidSignature:
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
	case ErrScopeMismatch:
		http.Error(w, "Scope mismatch", http.StatusForbidden)
	case ErrKindNotPermitted:
		http.Error(w, "Kind not permitted", http.StatusForbidden)
	case ErrAccessRevoked:
		http.Error(w, "Access revoked", http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
}

// RequireToken is a helper that extracts and verifies a token inline.
// Returns the token or writes an error response and returns nil.
func RequireToken(v *Verifier, w http.ResponseWriter, r *http.Request, requiredScope string) *token.Token {
	tok, err := v.VerifyRequest(r.Context(), r, requiredScope)
	if err != nil {
		writeError(w, err)
		return nil
	}
	return tok
}

// RequireKind is a helper that also checks kind permission inline.
func RequireKind(v *Verifier, w http.ResponseWriter, r *http.Request, requiredScope string, kind int) *token.Token {
	tok, err := v.VerifyRequestForKind(r.Context(), r, requiredScope, kind)
	if err != nil {
		writeError(w, err)
		return nil
	}
	return tok
}
