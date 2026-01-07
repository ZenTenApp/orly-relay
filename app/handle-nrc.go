package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
)

// getCashuMintURL returns the Cashu mint URL based on relay configuration.
// Returns empty string if Cashu is not enabled.
func (s *Server) getCashuMintURL() string {
	if !s.Config.CashuEnabled || s.CashuIssuer == nil {
		return ""
	}
	// Use configured relay URL with /cashu/mint path
	relayURL := strings.TrimSuffix(s.Config.RelayURL, "/")
	if relayURL == "" {
		return ""
	}
	return relayURL + "/cashu/mint"
}

// NRCConnectionResponse is the response structure for NRC connection API.
type NRCConnectionResponse struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used"`
	UseCashu  bool   `json:"use_cashu"`
	URI       string `json:"uri,omitempty"` // Only included when specifically requested
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
	MintURL       string `json:"mint_url,omitempty"`
	RelayPubkey   string `json:"relay_pubkey"`
}

// NRCCreateRequest is the request body for creating a connection.
type NRCCreateRequest struct {
	Label    string `json:"label"`
	UseCashu bool   `json:"use_cashu"`
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

	// Get database (must be Badger)
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "NRC requires Badger database backend", http.StatusServiceUnavailable)
		return
	}

	// Get all connections
	conns, err := badgerDB.GetAllNRCConnections()
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
	nrcEnabled, nrcRendezvousURL, _, nrcUseCashu, _ := s.Config.GetNRCConfigValues()

	// Build response
	response := NRCConnectionsResponse{
		Connections: make([]NRCConnectionResponse, 0, len(conns)),
		Config: NRCConfigResponse{
			Enabled:       nrcEnabled,
			RendezvousURL: nrcRendezvousURL,
			RelayPubkey:   string(hex.Enc(relayPubkey)),
		},
	}

	// Add mint URL if Cashu is enabled
	mintURL := s.getCashuMintURL()
	if nrcUseCashu && mintURL != "" {
		response.Config.MintURL = mintURL
	}

	for _, conn := range conns {
		response.Connections = append(response.Connections, NRCConnectionResponse{
			ID:        conn.ID,
			Label:     conn.Label,
			CreatedAt: conn.CreatedAt,
			LastUsed:  conn.LastUsed,
			UseCashu:  conn.UseCashu,
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

	// Get database (must be Badger)
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "NRC requires Badger database backend", http.StatusServiceUnavailable)
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

	// Create the connection
	conn, err := badgerDB.CreateNRCConnection(req.Label, req.UseCashu)
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

	// Get NRC config values
	_, nrcRendezvousURL, _, nrcUseCashu, _ := s.Config.GetNRCConfigValues()

	// Get mint URL if Cashu enabled
	mintURL := ""
	if nrcUseCashu {
		mintURL = s.getCashuMintURL()
	}

	// Generate URI
	uri, err := badgerDB.GetNRCConnectionURI(conn, relayPubkey, nrcRendezvousURL, mintURL)
	if chk.E(err) {
		log.W.F("failed to generate URI for new connection: %v", err)
	}

	// Update bridge authorized secrets if bridge is running
	s.updateNRCBridgeSecrets(badgerDB)

	// Build response with URI
	response := NRCConnectionResponse{
		ID:        conn.ID,
		Label:     conn.Label,
		CreatedAt: conn.CreatedAt,
		LastUsed:  conn.LastUsed,
		UseCashu:  conn.UseCashu,
		URI:       uri,
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

	// Get database (must be Badger)
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "NRC requires Badger database backend", http.StatusServiceUnavailable)
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
	if err := badgerDB.DeleteNRCConnection(connID); chk.E(err) {
		http.Error(w, "Failed to delete connection", http.StatusInternalServerError)
		return
	}

	// Update bridge authorized secrets if bridge is running
	s.updateNRCBridgeSecrets(badgerDB)

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

	// Get database (must be Badger)
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "NRC requires Badger database backend", http.StatusServiceUnavailable)
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
	conn, err := badgerDB.GetNRCConnection(connID)
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

	// Get NRC config values
	_, nrcRendezvousURL, _, nrcUseCashu, _ := s.Config.GetNRCConfigValues()

	// Get mint URL if Cashu enabled
	mintURL := ""
	if nrcUseCashu {
		mintURL = s.getCashuMintURL()
	}

	// Generate URI
	uri, err := badgerDB.GetNRCConnectionURI(conn, relayPubkey, nrcRendezvousURL, mintURL)
	if chk.E(err) {
		http.Error(w, "Failed to generate URI", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"uri": uri})
}

// updateNRCBridgeSecrets updates the NRC bridge with current authorized secrets from database.
func (s *Server) updateNRCBridgeSecrets(badgerDB *database.D) {
	if s.nrcBridge == nil {
		return
	}

	secrets, err := badgerDB.GetNRCAuthorizedSecrets()
	if chk.E(err) {
		log.W.F("failed to get NRC authorized secrets: %v", err)
		return
	}

	s.nrcBridge.UpdateAuthorizedSecrets(secrets)
	log.D.F("updated NRC bridge with %d authorized secrets", len(secrets))
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
	nrcEnabled, nrcRendezvousURL, _, nrcUseCashu, _ := s.Config.GetNRCConfigValues()

	// Check if Badger is available (NRC requires Badger)
	_, badgerAvailable := s.DB.(*database.D)

	response := struct {
		Enabled        bool   `json:"enabled"`
		BadgerRequired bool   `json:"badger_required"`
		RendezvousURL  string `json:"rendezvous_url,omitempty"`
		UseCashu       bool   `json:"use_cashu"`
		MintURL        string `json:"mint_url,omitempty"`
	}{
		Enabled:        nrcEnabled && badgerAvailable,
		BadgerRequired: !badgerAvailable,
		RendezvousURL:  nrcRendezvousURL,
		UseCashu:       nrcUseCashu,
	}

	// Add mint URL if Cashu is enabled
	if nrcUseCashu {
		mintURL := s.getCashuMintURL()
		if mintURL != "" {
			response.MintURL = mintURL
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
