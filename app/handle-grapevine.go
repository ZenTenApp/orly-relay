package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/httpauth"
	"git.smesh.lol/orly/pkg/lol/chk"
)

// handleGrapeVineScores handles GET /api/grapevine/scores?observer=<hex>
// Returns the full score set for an observer.
func (s *Server) handleGrapeVineScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.grapeVineEngine == nil {
		http.Error(w, "GrapeVine API not enabled", http.StatusServiceUnavailable)
		return
	}

	// NIP-98 auth
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	authedHex := hex.Enc(pubkey)
	observerHex := r.URL.Query().Get("observer")
	if observerHex == "" {
		observerHex = authedHex
	}

	// Non-owner can only query their own scores
	if observerHex != authedHex {
		accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
		if accessLevel != "owner" && accessLevel != "admin" {
			http.Error(w, "Can only query your own scores", http.StatusForbidden)
			return
		}
	}

	set, err := s.grapeVineEngine.GetScores(observerHex)
	if err != nil {
		http.Error(w, "Failed to retrieve scores", http.StatusInternalServerError)
		return
	}
	if set == nil {
		http.Error(w, "No scores computed for this observer", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(set)
}

// handleGrapeVineScore handles GET /api/grapevine/score?observer=<hex>&target=<hex>
// Returns a single score entry.
func (s *Server) handleGrapeVineScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.grapeVineEngine == nil {
		http.Error(w, "GrapeVine API not enabled", http.StatusServiceUnavailable)
		return
	}

	// NIP-98 auth
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	authedHex := hex.Enc(pubkey)
	observerHex := r.URL.Query().Get("observer")
	if observerHex == "" {
		observerHex = authedHex
	}
	targetHex := r.URL.Query().Get("target")
	if targetHex == "" {
		http.Error(w, "Missing required 'target' parameter", http.StatusBadRequest)
		return
	}

	// Non-owner can only query their own scores
	if observerHex != authedHex {
		accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
		if accessLevel != "owner" && accessLevel != "admin" {
			http.Error(w, "Can only query your own scores", http.StatusForbidden)
			return
		}
	}

	entry, err := s.grapeVineEngine.GetScore(observerHex, targetHex)
	if err != nil {
		http.Error(w, "Failed to retrieve score", http.StatusInternalServerError)
		return
	}
	if entry == nil {
		http.Error(w, "Score not found", http.StatusNotFound)
		return
	}

	resp := struct {
		Observer  string  `json:"observer"`
		Target    string  `json:"target"`
		Influence float64 `json:"influence"`
		Average   float64 `json:"average"`
		Certainty float64 `json:"certainty"`
		Input     float64 `json:"input"`
		WoTScore  int     `json:"wot_score"`
		Depth     int     `json:"depth"`
	}{
		Observer:  observerHex,
		Target:    targetHex,
		Influence: entry.Influence,
		Average:   entry.Average,
		Certainty: entry.Certainty,
		Input:     entry.Input,
		WoTScore:  entry.WoTScore,
		Depth:     entry.Depth,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGrapeVineRecalculate handles POST /api/grapevine/recalculate
// Body: {"observer":"<hex>"} — triggers async recomputation.
func (s *Server) handleGrapeVineRecalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.grapeVineEngine == nil || s.grapeVineScheduler == nil {
		http.Error(w, "GrapeVine API not enabled", http.StatusServiceUnavailable)
		return
	}

	// NIP-98 auth
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	authedHex := hex.Enc(pubkey)

	var req struct {
		Observer string `json:"observer"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Observer == "" {
		req.Observer = authedHex
	}

	// Non-owner can only trigger their own recalculation
	if req.Observer != authedHex {
		accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
		if accessLevel != "owner" && accessLevel != "admin" {
			http.Error(w, "Can only recalculate your own scores", http.StatusForbidden)
			return
		}
	}

	// Validate hex pubkey format
	req.Observer = strings.ToLower(req.Observer)
	if len(req.Observer) != 64 {
		http.Error(w, "Invalid observer pubkey (must be 64-char hex)", http.StatusBadRequest)
		return
	}

	started := s.grapeVineScheduler.TriggerCompute(req.Observer)

	status := "started"
	if !started {
		status = "already_computing"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   status,
		"observer": req.Observer,
	})
}
