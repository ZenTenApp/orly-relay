package app

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/cashu/issuer"
	"next.orly.dev/pkg/cashu/keyset"
	"next.orly.dev/pkg/cashu/token"
)

// CashuMintRequest is the request body for token issuance.
type CashuMintRequest struct {
	BlindedMessage string  `json:"blinded_message"` // Hex-encoded blinded point B_
	Scope          string  `json:"scope"`           // Token scope (e.g., "relay", "nip46")
	Kinds          []int   `json:"kinds,omitempty"` // Permitted event kinds
	KindRanges     [][]int `json:"kind_ranges,omitempty"` // Permitted kind ranges
}

// CashuMintResponse is the response body for token issuance.
type CashuMintResponse struct {
	BlindedSignature string `json:"blinded_signature"` // Hex-encoded blinded signature C_
	KeysetID         string `json:"keyset_id"`         // Keyset ID used
	Expiry           int64  `json:"expiry"`            // Token expiration timestamp
	MintPubkey       string `json:"mint_pubkey"`       // Hex-encoded mint public key
}

// handleCashuMint handles POST /cashu/mint - issues a new token.
func (s *Server) handleCashuMint(w http.ResponseWriter, r *http.Request) {
	// Check if Cashu is enabled
	if s.CashuIssuer == nil {
		http.Error(w, "Cashu tokens not enabled", http.StatusNotImplemented)
		return
	}

	// Require NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req CashuMintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Decode blinded message from hex
	blindedMsg, err := hex.DecodeString(req.BlindedMessage)
	if err != nil {
		http.Error(w, "Invalid blinded_message: must be hex", http.StatusBadRequest)
		return
	}

	// Default scope
	if req.Scope == "" {
		req.Scope = token.ScopeRelay
	}

	// Issue token
	issueReq := &issuer.IssueRequest{
		BlindedMessage: blindedMsg,
		Pubkey:         pubkey,
		Scope:          req.Scope,
		Kinds:          req.Kinds,
		KindRanges:     req.KindRanges,
	}

	resp, err := s.CashuIssuer.Issue(r.Context(), issueReq, r.RemoteAddr)
	if err != nil {
		log.W.F("Cashu mint failed for %x: %v", pubkey[:8], err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	log.D.F("Cashu token issued for %x, scope=%s, keyset=%s", pubkey[:8], req.Scope, resp.KeysetID)

	// Return response
	mintResp := CashuMintResponse{
		BlindedSignature: hex.EncodeToString(resp.BlindedSignature),
		KeysetID:         resp.KeysetID,
		Expiry:           resp.Expiry,
		MintPubkey:       hex.EncodeToString(resp.MintPubkey),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mintResp)
}

// handleCashuKeysets handles GET /cashu/keysets - returns available keysets.
func (s *Server) handleCashuKeysets(w http.ResponseWriter, r *http.Request) {
	if s.CashuIssuer == nil {
		http.Error(w, "Cashu tokens not enabled", http.StatusNotImplemented)
		return
	}

	infos := s.CashuIssuer.GetKeysetInfo()

	type KeysetsResponse struct {
		Keysets []keyset.KeysetInfo `json:"keysets"`
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KeysetsResponse{Keysets: infos})
}

// handleCashuInfo handles GET /cashu/info - returns mint information.
func (s *Server) handleCashuInfo(w http.ResponseWriter, r *http.Request) {
	if s.CashuIssuer == nil {
		http.Error(w, "Cashu tokens not enabled", http.StatusNotImplemented)
		return
	}

	info := s.CashuIssuer.GetMintInfo(s.Config.AppName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// CashuTokenTTL returns the configured token TTL.
func (s *Server) CashuTokenTTL() time.Duration {
	enabled, tokenTTL, _, _, _, _ := s.Config.GetCashuConfigValues()
	if !enabled {
		return 0
	}
	return tokenTTL
}

// CashuKeysetTTL returns the configured keyset TTL.
func (s *Server) CashuKeysetTTL() time.Duration {
	enabled, _, keysetTTL, _, _, _ := s.Config.GetCashuConfigValues()
	if !enabled {
		return 0
	}
	return keysetTTL
}
