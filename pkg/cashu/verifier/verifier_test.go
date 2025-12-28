package verifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"next.orly.dev/pkg/cashu/bdhke"
	"next.orly.dev/pkg/cashu/issuer"
	"next.orly.dev/pkg/cashu/keyset"
	"next.orly.dev/pkg/cashu/token"
	cashuiface "next.orly.dev/pkg/interfaces/cashu"
)

func setupVerifier() (*Verifier, *issuer.Issuer, *keyset.Manager) {
	store := keyset.NewMemoryStore()
	manager := keyset.NewManager(store, keyset.DefaultActiveWindow, keyset.DefaultVerifyWindow)
	manager.Init()

	issuerConfig := issuer.DefaultConfig()
	iss := issuer.New(manager, cashuiface.AllowAllChecker{}, issuerConfig)

	verifierConfig := DefaultConfig()
	ver := New(manager, cashuiface.AllowAllChecker{}, verifierConfig)

	return ver, iss, manager
}

func issueTestToken(iss *issuer.Issuer, scope string, kinds []int) (*token.Token, error) {
	secret, err := bdhke.GenerateSecret()
	if err != nil {
		return nil, err
	}

	blindResult, err := bdhke.Blind(secret)
	if err != nil {
		return nil, err
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	req := &issuer.IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          scope,
		Kinds:          kinds,
	}

	resp, err := iss.Issue(context.Background(), req, "127.0.0.1")
	if err != nil {
		return nil, err
	}

	return issuer.BuildToken(resp, secret, blindResult.R, pubkey, scope, kinds, nil)
}

func TestVerifySuccess(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	err = ver.Verify(context.Background(), tok, "127.0.0.1")
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestVerifyExpired(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Expire the token
	tok.Expiry = time.Now().Add(-time.Hour).Unix()

	err = ver.Verify(context.Background(), tok, "127.0.0.1")
	if err == nil {
		t.Error("Verify should fail for expired token")
	}
}

func TestVerifyInvalidSignature(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Corrupt the signature
	tok.Signature[10] ^= 0xFF

	err = ver.Verify(context.Background(), tok, "127.0.0.1")
	if err == nil {
		t.Error("Verify should fail for invalid signature")
	}
}

func TestVerifyUnknownKeyset(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Change keyset ID
	tok.KeysetID = "00000000000000"

	err = ver.Verify(context.Background(), tok, "127.0.0.1")
	if err == nil {
		t.Error("Verify should fail for unknown keyset")
	}
}

func TestVerifyForScope(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeNIP46, []int{24133})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Should pass for correct scope
	err = ver.VerifyForScope(context.Background(), tok, token.ScopeNIP46, "127.0.0.1")
	if err != nil {
		t.Errorf("VerifyForScope failed for correct scope: %v", err)
	}

	// Should fail for wrong scope
	err = ver.VerifyForScope(context.Background(), tok, token.ScopeRelay, "127.0.0.1")
	if err == nil {
		t.Error("VerifyForScope should fail for wrong scope")
	}
}

func TestVerifyForKind(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Should pass for permitted kind
	err = ver.VerifyForKind(context.Background(), tok, 1, "127.0.0.1")
	if err != nil {
		t.Errorf("VerifyForKind failed for permitted kind: %v", err)
	}

	// Should fail for non-permitted kind
	err = ver.VerifyForKind(context.Background(), tok, 100, "127.0.0.1")
	if err == nil {
		t.Error("VerifyForKind should fail for non-permitted kind")
	}
}

func TestVerifyReauthorization(t *testing.T) {
	store := keyset.NewMemoryStore()
	manager := keyset.NewManager(store, keyset.DefaultActiveWindow, keyset.DefaultVerifyWindow)
	manager.Init()

	iss := issuer.New(manager, cashuiface.AllowAllChecker{}, issuer.DefaultConfig())

	// Create verifier that denies authorization
	config := DefaultConfig()
	config.Reauthorize = true
	ver := New(manager, cashuiface.DenyAllChecker{}, config)

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	// Should fail due to reauthorization check
	err = ver.Verify(context.Background(), tok, "127.0.0.1")
	if err == nil {
		t.Error("Verify should fail when reauthorization fails")
	}
}

func TestExtractFromRequest(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	encoded, _ := tok.Encode()

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"X-Cashu-Token", "X-Cashu-Token", encoded},
		{"Authorization Cashu", "Authorization", "Cashu " + encoded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(tt.header, tt.value)

			extracted, err := ver.ExtractFromRequest(req)
			if err != nil {
				t.Fatalf("ExtractFromRequest failed: %v", err)
			}

			if extracted.KeysetID != tok.KeysetID {
				t.Errorf("KeysetID mismatch: %s != %s", extracted.KeysetID, tok.KeysetID)
			}
		})
	}
}

func TestExtractFromRequestMissing(t *testing.T) {
	ver, _, _ := setupVerifier()

	req := httptest.NewRequest("GET", "/", nil)

	_, err := ver.ExtractFromRequest(req)
	if err != ErrMissingToken {
		t.Errorf("Expected ErrMissingToken, got %v", err)
	}
}

func TestVerifyRequest(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	encoded, _ := tok.Encode()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Cashu-Token", encoded)

	verified, err := ver.VerifyRequest(context.Background(), req, token.ScopeRelay)
	if err != nil {
		t.Fatalf("VerifyRequest failed: %v", err)
	}

	if verified.KeysetID != tok.KeysetID {
		t.Error("VerifyRequest returned wrong token")
	}
}

func TestMiddleware(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	encoded, _ := tok.Encode()

	// Handler that checks context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxTok := TokenFromContext(r.Context())
		if ctxTok == nil {
			t.Error("Token not in context")
		}
		pubkey := PubkeyFromContext(r.Context())
		if pubkey == nil {
			t.Error("Pubkey not in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(ver, token.ScopeRelay)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Cashu-Token", encoded)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want 200", rec.Code)
	}
}

func TestMiddlewareUnauthorized(t *testing.T) {
	ver, _, _ := setupVerifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(ver, token.ScopeRelay)(handler)

	// Request without token
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want 401", rec.Code)
	}
}

func TestOptionalMiddleware(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	encoded, _ := tok.Encode()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := OptionalMiddleware(ver, token.ScopeRelay)(handler)

	// With token
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Cashu-Token", encoded)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("With token: Status = %d, want 200", rec1.Code)
	}

	// Without token
	req2 := httptest.NewRequest("GET", "/", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("Without token: Status = %d, want 200", rec2.Code)
	}
}

func TestRequireToken(t *testing.T) {
	ver, iss, _ := setupVerifier()

	tok, err := issueTestToken(iss, token.ScopeRelay, []int{1})
	if err != nil {
		t.Fatalf("issueTestToken failed: %v", err)
	}

	encoded, _ := tok.Encode()

	// With valid token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Cashu-Token", encoded)
	rec := httptest.NewRecorder()

	result := RequireToken(ver, rec, req, token.ScopeRelay)
	if result == nil {
		t.Error("RequireToken should return token")
	}

	// Without token
	req2 := httptest.NewRequest("GET", "/", nil)
	rec2 := httptest.NewRecorder()

	result2 := RequireToken(ver, rec2, req2, token.ScopeRelay)
	if result2 != nil {
		t.Error("RequireToken should return nil for missing token")
	}
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want 401", rec2.Code)
	}
}

// Helper to parse point
func mustParsePoint(data []byte) *secp256k1.PublicKey {
	pk, err := secp256k1.ParsePubKey(data)
	if err != nil {
		panic(err)
	}
	return pk
}
