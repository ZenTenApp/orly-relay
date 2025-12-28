package issuer

import (
	"context"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"next.orly.dev/pkg/cashu/bdhke"
	"next.orly.dev/pkg/cashu/keyset"
	"next.orly.dev/pkg/cashu/token"
	cashuiface "next.orly.dev/pkg/interfaces/cashu"
)

func setupIssuer(authz cashuiface.AuthzChecker) (*Issuer, *keyset.Manager) {
	store := keyset.NewMemoryStore()
	manager := keyset.NewManager(store, keyset.DefaultActiveWindow, keyset.DefaultVerifyWindow)
	manager.Init()

	config := DefaultConfig()
	issuer := New(manager, authz, config)

	return issuer, manager
}

func TestIssueSuccess(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	// Generate user keypair
	secret, err := bdhke.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	// Generate blinded message
	blindResult, err := bdhke.Blind(secret)
	if err != nil {
		t.Fatalf("Blind failed: %v", err)
	}

	// User pubkey
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          token.ScopeRelay,
		Kinds:          []int{0, 1, 3},
		KindRanges:     [][]int{{30000, 39999}},
	}

	resp, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// Check response
	if len(resp.BlindedSignature) != 33 {
		t.Errorf("BlindedSignature length = %d, want 33", len(resp.BlindedSignature))
	}
	if resp.KeysetID == "" {
		t.Error("KeysetID is empty")
	}
	if resp.Expiry <= time.Now().Unix() {
		t.Error("Expiry should be in the future")
	}
	if len(resp.MintPubkey) != 33 {
		t.Errorf("MintPubkey length = %d, want 33", len(resp.MintPubkey))
	}
}

func TestIssueAuthorizationDenied(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.DenyAllChecker{})

	secret, _ := bdhke.GenerateSecret()
	blindResult, _ := bdhke.Blind(secret)
	pubkey := make([]byte, 32)

	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          token.ScopeRelay,
	}

	_, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err == nil {
		t.Error("Issue should fail when authorization is denied")
	}
}

func TestIssueInvalidBlindedMessage(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	pubkey := make([]byte, 32)

	req := &IssueRequest{
		BlindedMessage: []byte{1, 2, 3}, // Invalid
		Pubkey:         pubkey,
		Scope:          token.ScopeRelay,
	}

	_, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err == nil {
		t.Error("Issue should fail with invalid blinded message")
	}
}

func TestIssueInvalidPubkey(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	secret, _ := bdhke.GenerateSecret()
	blindResult, _ := bdhke.Blind(secret)

	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         []byte{1, 2, 3}, // Invalid length
		Scope:          token.ScopeRelay,
	}

	_, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err == nil {
		t.Error("Issue should fail with invalid pubkey")
	}
}

func TestIssueInvalidScope(t *testing.T) {
	store := keyset.NewMemoryStore()
	manager := keyset.NewManager(store, keyset.DefaultActiveWindow, keyset.DefaultVerifyWindow)
	manager.Init()

	config := DefaultConfig()
	config.AllowedScopes = []string{token.ScopeRelay} // Only relay scope allowed

	issuer := New(manager, cashuiface.AllowAllChecker{}, config)

	secret, _ := bdhke.GenerateSecret()
	blindResult, _ := bdhke.Blind(secret)
	pubkey := make([]byte, 32)

	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          token.ScopeNIP46, // Not allowed
	}

	_, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err == nil {
		t.Error("Issue should fail with disallowed scope")
	}
}

func TestIssueTTL(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	secret, _ := bdhke.GenerateSecret()
	blindResult, _ := bdhke.Blind(secret)
	pubkey := make([]byte, 32)

	// Request with custom TTL
	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          token.ScopeRelay,
		TTL:            time.Hour,
	}

	resp, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// Expiry should be ~1 hour from now
	expectedExpiry := time.Now().Add(time.Hour).Unix()
	if resp.Expiry < expectedExpiry-60 || resp.Expiry > expectedExpiry+60 {
		t.Errorf("Expiry %d not within expected range of %d", resp.Expiry, expectedExpiry)
	}
}

func TestBuildToken(t *testing.T) {
	issuer, manager := setupIssuer(cashuiface.AllowAllChecker{})

	// Generate secret and blind it
	secret, _ := bdhke.GenerateSecret()
	blindResult, _ := bdhke.Blind(secret)
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	// Issue token
	req := &IssueRequest{
		BlindedMessage: blindResult.B.SerializeCompressed(),
		Pubkey:         pubkey,
		Scope:          token.ScopeRelay,
		Kinds:          []int{1, 2, 3},
	}

	resp, err := issuer.Issue(context.Background(), req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// Build complete token
	tok, err := BuildToken(resp, secret, blindResult.R, pubkey, token.ScopeRelay, []int{1, 2, 3}, nil)
	if err != nil {
		t.Fatalf("BuildToken failed: %v", err)
	}

	// Verify token structure
	if tok.KeysetID != resp.KeysetID {
		t.Errorf("KeysetID mismatch: %s != %s", tok.KeysetID, resp.KeysetID)
	}
	if tok.Scope != token.ScopeRelay {
		t.Errorf("Scope = %s, want %s", tok.Scope, token.ScopeRelay)
	}

	// Verify signature (using the keyset)
	ks := manager.FindByID(tok.KeysetID)
	if ks == nil {
		t.Fatal("Keyset not found")
	}

	valid, err := bdhke.Verify(tok.Secret, mustParsePoint(tok.Signature), ks.PrivateKey)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !valid {
		t.Error("Token signature is not valid")
	}
}

func TestGetKeysetInfo(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	infos := issuer.GetKeysetInfo()
	if len(infos) == 0 {
		t.Error("GetKeysetInfo returned empty")
	}

	for _, info := range infos {
		if info.ID == "" {
			t.Error("KeysetInfo has empty ID")
		}
		if info.PublicKey == "" {
			t.Error("KeysetInfo has empty PublicKey")
		}
	}
}

func TestGetActiveKeysetID(t *testing.T) {
	issuer, _ := setupIssuer(cashuiface.AllowAllChecker{})

	id := issuer.GetActiveKeysetID()
	if id == "" {
		t.Error("GetActiveKeysetID returned empty")
	}
	if len(id) != 14 {
		t.Errorf("KeysetID length = %d, want 14", len(id))
	}
}

func TestGetMintInfo(t *testing.T) {
	store := keyset.NewMemoryStore()
	manager := keyset.NewManager(store, keyset.DefaultActiveWindow, keyset.DefaultVerifyWindow)
	manager.Init()

	config := DefaultConfig()
	config.AllowedScopes = []string{token.ScopeRelay, token.ScopeNIP46}

	issuer := New(manager, cashuiface.AllowAllChecker{}, config)

	info := issuer.GetMintInfo("Test Relay")

	if info.Name != "Test Relay" {
		t.Errorf("Name = %s, want Test Relay", info.Name)
	}
	if info.Version != "NIP-XX/1" {
		t.Errorf("Version = %s, want NIP-XX/1", info.Version)
	}
	if len(info.SupportedScopes) != 2 {
		t.Errorf("SupportedScopes length = %d, want 2", len(info.SupportedScopes))
	}
}

// Helper to parse point for testing
func mustParsePoint(data []byte) *secp256k1.PublicKey {
	pk, err := secp256k1.ParsePubKey(data)
	if err != nil {
		panic(err)
	}
	return pk
}
