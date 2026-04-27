package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// BunkerInfoResponse is returned by the /api/bunker/info endpoint.
type BunkerInfoResponse struct {
	RelayURL     string `json:"relay_url"`      // WebSocket URL for NIP-46 connections
	RelayNpub    string `json:"relay_npub"`     // Relay's npub
	RelayPubkey  string `json:"relay_pubkey"`   // Relay's hex pubkey
	ACLMode      string `json:"acl_mode"`       // Current ACL mode
	Available    bool   `json:"available"`      // Whether bunker is available
}

// handleBunkerInfo returns bunker connection information.
// This is a public endpoint that doesn't require authentication.
func (s *Server) handleBunkerInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get relay identity
	relaySecret, err := s.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		log.E.F("failed to get relay identity: %v", err)
		http.Error(w, "Failed to get relay identity", http.StatusInternalServerError)
		return
	}

	// Derive public key
	sign, err := p8k.New()
	if chk.E(err) {
		http.Error(w, "Failed to create signer", http.StatusInternalServerError)
		return
	}
	if err := sign.InitSec(relaySecret); chk.E(err) {
		http.Error(w, "Failed to initialize signer", http.StatusInternalServerError)
		return
	}
	relayPubkey := sign.Pub()
	relayPubkeyHex := hex.Enc(relayPubkey)

	// Encode as npub
	relayNpubBytes, err := bech32encoding.BinToNpub(relayPubkey)
	relayNpub := string(relayNpubBytes)
	if chk.E(err) {
		relayNpub = relayPubkeyHex // Fallback to hex
	}

	// Build WebSocket URL from service URL
	serviceURL := s.ServiceURL(r)
	wsURL := strings.Replace(serviceURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	// Bunker is available when ACL mode is not "none"
	available := s.Config.ACLMode != "none"

	resp := BunkerInfoResponse{
		RelayURL:    wsURL,
		RelayNpub:   relayNpub,
		RelayPubkey: relayPubkeyHex,
		ACLMode:     s.Config.ACLMode,
		Available:   available,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
