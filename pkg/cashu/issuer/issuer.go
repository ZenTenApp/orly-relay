// Package issuer implements Cashu token issuance with authorization checks.
package issuer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"next.orly.dev/pkg/cashu/bdhke"
	"next.orly.dev/pkg/cashu/keyset"
	"next.orly.dev/pkg/cashu/token"
	cashuiface "next.orly.dev/pkg/interfaces/cashu"
)

// Errors.
var (
	ErrNoActiveKeyset    = errors.New("issuer: no active keyset available")
	ErrInvalidBlindedMsg = errors.New("issuer: invalid blinded message")
	ErrInvalidPubkey     = errors.New("issuer: invalid pubkey")
	ErrInvalidScope      = errors.New("issuer: invalid scope")
)

// Config holds issuer configuration.
type Config struct {
	// DefaultTTL is the default token lifetime.
	DefaultTTL time.Duration

	// MaxTTL is the maximum allowed token lifetime.
	MaxTTL time.Duration

	// AllowedScopes is the list of scopes this issuer can issue tokens for.
	// Empty means all scopes are allowed.
	AllowedScopes []string

	// MaxKinds is the maximum number of explicit kinds in a token.
	// 0 means unlimited.
	MaxKinds int

	// MaxKindRanges is the maximum number of kind ranges in a token.
	// 0 means unlimited.
	MaxKindRanges int
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultTTL:    7 * 24 * time.Hour, // 1 week
		MaxTTL:        7 * 24 * time.Hour, // 1 week
		MaxKinds:      100,
		MaxKindRanges: 10,
	}
}

// Issuer handles token issuance with authorization checks.
type Issuer struct {
	keysets *keyset.Manager
	authz   cashuiface.AuthzChecker
	config  Config
}

// New creates a new issuer.
func New(keysets *keyset.Manager, authz cashuiface.AuthzChecker, config Config) *Issuer {
	return &Issuer{
		keysets: keysets,
		authz:   authz,
		config:  config,
	}
}

// IssueRequest contains the request parameters for token issuance.
type IssueRequest struct {
	// BlindedMessage is the blinded point B_ (33 bytes compressed).
	BlindedMessage []byte

	// Pubkey is the user's Nostr pubkey (32 bytes).
	Pubkey []byte

	// Scope is the requested token scope.
	Scope string

	// Kinds is the list of permitted event kinds.
	Kinds []int

	// KindRanges is the list of permitted kind ranges.
	KindRanges [][]int

	// TTL is the requested token lifetime (optional, uses default if zero).
	TTL time.Duration
}

// IssueResponse contains the response from token issuance.
type IssueResponse struct {
	// BlindedSignature is the blinded signature C_ (33 bytes compressed).
	BlindedSignature []byte

	// KeysetID is the ID of the keyset used for signing.
	KeysetID string

	// Expiry is the token expiration timestamp.
	Expiry int64

	// MintPubkey is the public key of the keyset (for unblinding).
	MintPubkey []byte
}

// Issue creates a blinded signature after authorization check.
func (i *Issuer) Issue(ctx context.Context, req *IssueRequest, remoteAddr string) (*IssueResponse, error) {
	// Validate request
	if err := i.validateRequest(req); err != nil {
		return nil, err
	}

	// Check authorization
	if err := i.authz.CheckAuthorization(ctx, req.Pubkey, req.Scope, remoteAddr); err != nil {
		return nil, fmt.Errorf("issuer: authorization failed: %w", err)
	}

	// Get active keyset
	ks := i.keysets.GetSigningKeyset()
	if ks == nil || !ks.IsActiveForSigning() {
		return nil, ErrNoActiveKeyset
	}

	// Parse blinded message
	B_, err := secp256k1.ParsePubKey(req.BlindedMessage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBlindedMsg, err)
	}

	// Sign the blinded message
	C_, err := bdhke.Sign(B_, ks.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("issuer: signing failed: %w", err)
	}

	// Calculate expiry
	ttl := req.TTL
	if ttl <= 0 {
		ttl = i.config.DefaultTTL
	}
	if ttl > i.config.MaxTTL {
		ttl = i.config.MaxTTL
	}
	expiry := time.Now().Add(ttl).Unix()

	return &IssueResponse{
		BlindedSignature: C_.SerializeCompressed(),
		KeysetID:         ks.ID,
		Expiry:           expiry,
		MintPubkey:       ks.SerializePublicKey(),
	}, nil
}

// validateRequest validates the issue request.
func (i *Issuer) validateRequest(req *IssueRequest) error {
	// Validate blinded message
	if len(req.BlindedMessage) != 33 {
		return fmt.Errorf("%w: expected 33 bytes, got %d", ErrInvalidBlindedMsg, len(req.BlindedMessage))
	}

	// Validate pubkey
	if len(req.Pubkey) != 32 {
		return fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidPubkey, len(req.Pubkey))
	}

	// Validate scope
	if req.Scope == "" {
		return ErrInvalidScope
	}
	if len(i.config.AllowedScopes) > 0 {
		allowed := false
		for _, s := range i.config.AllowedScopes {
			if s == req.Scope {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: %s not in allowed scopes", ErrInvalidScope, req.Scope)
		}
	}

	// Validate kinds count
	if i.config.MaxKinds > 0 && len(req.Kinds) > i.config.MaxKinds {
		return fmt.Errorf("issuer: too many kinds: %d > %d", len(req.Kinds), i.config.MaxKinds)
	}

	// Validate kind ranges count
	if i.config.MaxKindRanges > 0 && len(req.KindRanges) > i.config.MaxKindRanges {
		return fmt.Errorf("issuer: too many kind ranges: %d > %d", len(req.KindRanges), i.config.MaxKindRanges)
	}

	// Validate kind ranges format
	for idx, r := range req.KindRanges {
		if len(r) != 2 {
			return fmt.Errorf("issuer: kind range %d must have 2 elements", idx)
		}
		if r[0] > r[1] {
			return fmt.Errorf("issuer: kind range %d min > max: %d > %d", idx, r[0], r[1])
		}
	}

	return nil
}

// GetKeysetInfo returns public information about available keysets.
func (i *Issuer) GetKeysetInfo() []keyset.KeysetInfo {
	return i.keysets.ListKeysetInfo()
}

// GetActiveKeysetID returns the ID of the currently active keyset.
func (i *Issuer) GetActiveKeysetID() string {
	ks := i.keysets.GetSigningKeyset()
	if ks == nil {
		return ""
	}
	return ks.ID
}

// MintInfo contains public information about the mint.
type MintInfo struct {
	Name            string   `json:"name,omitempty"`
	Version         string   `json:"version"`
	Pubkey          string   `json:"pubkey"`
	TokenTTL        int64    `json:"token_ttl"`
	MaxKinds        int      `json:"max_kinds,omitempty"`
	MaxKindRanges   int      `json:"max_kind_ranges,omitempty"`
	SupportedScopes []string `json:"supported_scopes,omitempty"`
}

// GetMintInfo returns public information about the issuer.
func (i *Issuer) GetMintInfo(name string) MintInfo {
	var pubkeyHex string
	if ks := i.keysets.GetSigningKeyset(); ks != nil {
		pubkeyHex = hex.EncodeToString(ks.SerializePublicKey())
	}
	return MintInfo{
		Name:            name,
		Version:         "NIP-XX/1",
		Pubkey:          pubkeyHex,
		TokenTTL:        int64(i.config.DefaultTTL.Seconds()),
		MaxKinds:        i.config.MaxKinds,
		MaxKindRanges:   i.config.MaxKindRanges,
		SupportedScopes: i.config.AllowedScopes,
	}
}

// BuildToken is a helper that creates a complete token from the issue response
// and the user's secret and blinding factor.
// This is typically done client-side, but provided for testing and CLI tools.
func BuildToken(
	resp *IssueResponse,
	secret []byte,
	blindingFactor *secp256k1.PrivateKey,
	pubkey []byte,
	scope string,
	kinds []int,
	kindRanges [][]int,
) (*token.Token, error) {
	// Parse mint pubkey
	mintPubkey, err := secp256k1.ParsePubKey(resp.MintPubkey)
	if err != nil {
		return nil, fmt.Errorf("invalid mint pubkey: %w", err)
	}

	// Parse blinded signature
	C_, err := secp256k1.ParsePubKey(resp.BlindedSignature)
	if err != nil {
		return nil, fmt.Errorf("invalid blinded signature: %w", err)
	}

	// Unblind the signature
	C, err := bdhke.Unblind(C_, blindingFactor, mintPubkey)
	if err != nil {
		return nil, fmt.Errorf("unblind failed: %w", err)
	}

	// Create token
	tok := token.New(
		resp.KeysetID,
		secret,
		C.SerializeCompressed(),
		pubkey,
		time.Unix(resp.Expiry, 0),
		scope,
	)
	tok.SetKinds(kinds...)
	tok.KindRanges = kindRanges

	return tok, nil
}
