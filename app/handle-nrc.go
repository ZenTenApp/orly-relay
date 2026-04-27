package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/httpauth"
	"git.smesh.lol/orly/pkg/acl"
)

// NRCConnectionResponse is the response structure for NRC connection API.
type NRCConnectionResponse struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	RendezvousURL string `json:"rendezvous_url"`
	CreatedAt     int64  `json:"created_at"`
	LastUsed      int64  `json:"last_used"`
	URI           string `json:"uri,omitempty"` // Only included when specifically requested
}

// NRCConnectionsResponse is the response for listing all connections.
type NRCConnectionsResponse struct {
	Connections []NRCConnectionResponse `json:"connections"`
	Config      NRCConfigResponse       `json:"config"`
}

// NRCConfigResponse contains NRC configuration status.
type NRCConfigResponse struct {
	Enabled       bool   `json:"enabled"`
	RendezvousURL string `json:"rendezvous_url"`
	RelayPubkey   string `json:"relay_pubkey"`
}

// NRCCreateRequest is the request body for creating a connection.
type NRCCreateRequest struct {
	Label         string `json:"label"`
	RendezvousURL string `json:"rendezvous_url"` // WebSocket URL of the rendezvous relay
}

// handleNRCConnections handles GET /api/nrc/connections
func (s *Server) handleNRCConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Check permissions - require owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if event store is available
	if s.nrcEventStore == nil {
		http.Error(w, "NRC not configured", http.StatusServiceUnavailable)
		return
	}

	// Get all connections
	conns, err := s.nrcEventStore.GetAllNRCConnections()
	if chk.E(err) {
		http.Error(w, "Failed to get connections", http.StatusInternalServerError)
		return
	}

	// Get relay identity for config
	relaySecretKey, err := s.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		http.Error(w, "Failed to get relay identity", http.StatusInternalServerError)
		return
	}
	relayPubkey, _ := keys.SecretBytesToPubKeyBytes(relaySecretKey)

	// Get NRC config values
	nrcEnabled, nrcRendezvousURL, _, _ := s.Config.GetNRCConfigValues()

	// Build response
	response := NRCConnectionsResponse{
		Connections: make([]NRCConnectionResponse, 0, len(conns)),
		Config: NRCConfigResponse{
			Enabled:       nrcEnabled,
			RendezvousURL: nrcRendezvousURL,
			RelayPubkey:   string(hex.Enc(relayPubkey)),
		},
	}

	for _, conn := range conns {
		response.Connections = append(response.Connections, NRCConnectionResponse{
			ID:            conn.ID,
			Label:         conn.Label,
			RendezvousURL: conn.RendezvousURL,
			CreatedAt:     conn.CreatedAt,
			LastUsed:      conn.LastUsed,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNRCCreate handles POST /api/nrc/connections
func (s *Server) handleNRCCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Check permissions - require owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if event store is available
	if s.nrcEventStore == nil {
		http.Error(w, "NRC not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req NRCCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate label
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" {
		http.Error(w, "Label is required", http.StatusBadRequest)
		return
	}

	// Validate rendezvous URL
	req.RendezvousURL = strings.TrimSpace(req.RendezvousURL)
	if req.RendezvousURL == "" {
		http.Error(w, "Rendezvous URL is required", http.StatusBadRequest)
		return
	}

	// Create the connection (pass the creator's pubkey for tracking)
	conn, err := s.nrcEventStore.CreateNRCConnection(req.Label, req.RendezvousURL, pubkey)
	if chk.E(err) {
		http.Error(w, "Failed to create connection", http.StatusInternalServerError)
		return
	}

	// Get relay identity for URI generation
	relaySecretKey, err := s.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		http.Error(w, "Failed to get relay identity", http.StatusInternalServerError)
		return
	}
	relayPubkey, _ := keys.SecretBytesToPubKeyBytes(relaySecretKey)

	// Generate URI (uses rendezvous URL stored in connection)
	uri, err := s.nrcEventStore.GetNRCConnectionURI(conn, relayPubkey)
	if chk.E(err) {
		log.W.F("failed to generate URI for new connection: %v", err)
	}

	// Update bridge authorized secrets if bridge is running
	s.updateNRCBridgeSecretsFromEventStore()

	// Build response with URI
	response := NRCConnectionResponse{
		ID:            conn.ID,
		Label:         conn.Label,
		RendezvousURL: conn.RendezvousURL,
		CreatedAt:     conn.CreatedAt,
		LastUsed:      conn.LastUsed,
		URI:           uri,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleNRCDelete handles DELETE /api/nrc/connections/{id}
func (s *Server) handleNRCDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Check permissions - require owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if event store is available
	if s.nrcEventStore == nil {
		http.Error(w, "NRC not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract connection ID from URL path
	// URL format: /api/nrc/connections/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/nrc/connections/")
	connID := strings.TrimSpace(path)
	if connID == "" {
		http.Error(w, "Connection ID required", http.StatusBadRequest)
		return
	}

	// Delete the connection
	if err := s.nrcEventStore.DeleteNRCConnection(connID); chk.E(err) {
		http.Error(w, "Failed to delete connection", http.StatusInternalServerError)
		return
	}

	// Update bridge authorized secrets if bridge is running
	s.updateNRCBridgeSecretsFromEventStore()

	log.I.F("deleted NRC connection: %s", connID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleNRCGetURI handles GET /api/nrc/connections/{id}/uri
func (s *Server) handleNRCGetURI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Check permissions - require owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if event store is available
	if s.nrcEventStore == nil {
		http.Error(w, "NRC not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract connection ID from URL path
	// URL format: /api/nrc/connections/{id}/uri
	path := strings.TrimPrefix(r.URL.Path, "/api/nrc/connections/")
	path = strings.TrimSuffix(path, "/uri")
	connID := strings.TrimSpace(path)
	if connID == "" {
		http.Error(w, "Connection ID required", http.StatusBadRequest)
		return
	}

	// Get the connection
	conn, err := s.nrcEventStore.GetNRCConnection(connID)
	if err != nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	// Get relay identity
	relaySecretKey, err := s.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		http.Error(w, "Failed to get relay identity", http.StatusInternalServerError)
		return
	}
	relayPubkey, _ := keys.SecretBytesToPubKeyBytes(relaySecretKey)

	// Generate URI (uses rendezvous URL stored in connection)
	uri, err := s.nrcEventStore.GetNRCConnectionURI(conn, relayPubkey)
	if chk.E(err) {
		http.Error(w, "Failed to generate URI", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"uri": uri})
}

// updateNRCBridgeSecretsFromEventStore updates the NRC bridge with current authorized secrets from event store.
func (s *Server) updateNRCBridgeSecretsFromEventStore() {
	if s.nrcBridge == nil || s.nrcEventStore == nil {
		return
	}

	secrets, err := s.nrcEventStore.GetNRCAuthorizedSecrets()
	if chk.E(err) {
		log.W.F("failed to get NRC authorized secrets: %v", err)
		return
	}

	s.nrcBridge.UpdateAuthorizedSecrets(secrets)
	log.D.F("updated NRC bridge with %d authorized secrets from event store", len(secrets))
}

// handleNRCConnectionsRouter routes NRC connection requests.
func (s *Server) handleNRCConnectionsRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Exact match for /api/nrc/connections
	if path == "/api/nrc/connections" {
		switch r.Method {
		case http.MethodGet:
			s.handleNRCConnections(w, r)
		case http.MethodPost:
			s.handleNRCCreate(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Check for /api/nrc/connections/{id}/uri
	if strings.HasSuffix(path, "/uri") {
		s.handleNRCGetURI(w, r)
		return
	}

	// Otherwise it's /api/nrc/connections/{id}
	s.handleNRCDelete(w, r)
}

// handleNRCConfig returns NRC configuration status.
func (s *Server) handleNRCConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get NRC config values
	nrcEnabled, nrcRendezvousURL, _, _ := s.Config.GetNRCConfigValues()

	// Check if NRC bridge is actually running
	bridgeRunning := s.nrcBridge != nil

	// Check if event store is available for connection management
	eventStoreAvailable := s.nrcEventStore != nil

	response := struct {
		Enabled           bool   `json:"enabled"`
		ConnectionMgmtOK  bool   `json:"connection_mgmt_ok"`
		RendezvousURL     string `json:"rendezvous_url,omitempty"`
	}{
		Enabled:          nrcEnabled && bridgeRunning,
		ConnectionMgmtOK: eventStoreAvailable,
		RendezvousURL:    nrcRendezvousURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
