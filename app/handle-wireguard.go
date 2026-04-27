package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/httpauth"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/database"
)

// WireGuardConfigResponse is returned by the /api/wireguard/config endpoint.
type WireGuardConfigResponse struct {
	ConfigText string      `json:"config_text"`
	Interface  WGInterface `json:"interface"`
	Peer       WGPeer      `json:"peer"`
}

// WGInterface represents the [Interface] section of a WireGuard config.
type WGInterface struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}

// WGPeer represents the [Peer] section of a WireGuard config.
type WGPeer struct {
	PublicKey  string `json:"public_key"`
	Endpoint   string `json:"endpoint"`
	AllowedIPs string `json:"allowed_ips"`
}

// BunkerURLResponse is returned by the /api/bunker/url endpoint.
type BunkerURLResponse struct {
	URL         string `json:"url"`
	RelayNpub   string `json:"relay_npub"`
	RelayPubkey string `json:"relay_pubkey"`
	InternalIP  string `json:"internal_ip"`
}

// handleWireGuardConfig returns the user's WireGuard configuration.
// Requires NIP-98 authentication and write+ access.
func (s *Server) handleWireGuardConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if WireGuard is enabled
	if !s.Config.WGEnabled {
		http.Error(w, "WireGuard is not enabled on this relay", http.StatusNotFound)
		return
	}

	// Check if ACL mode supports WireGuard
	if s.Config.ACLMode == "none" {
		http.Error(w, "WireGuard requires ACL mode 'follows' or 'managed'", http.StatusForbidden)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return
	}

	// Check user has write+ access
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "write" && accessLevel != "admin" && accessLevel != "owner" {
		http.Error(w, "Write access required for WireGuard", http.StatusForbidden)
		return
	}

	// Type assert to Badger database for WireGuard methods
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "WireGuard requires Badger database backend", http.StatusInternalServerError)
		return
	}

	// Check subnet pool is available
	if s.subnetPool == nil {
		http.Error(w, "WireGuard subnet pool not initialized", http.StatusInternalServerError)
		return
	}

	// Get or create WireGuard peer for this user
	peer, err := badgerDB.GetOrCreateWireGuardPeer(pubkey, s.subnetPool)
	if chk.E(err) {
		log.E.F("failed to get/create WireGuard peer: %v", err)
		http.Error(w, "Failed to create WireGuard configuration", http.StatusInternalServerError)
		return
	}

	// Derive subnet IPs from sequence
	subnet := s.subnetPool.SubnetForSequence(peer.Sequence)
	clientIP := subnet.ClientIP.String()
	serverIP := subnet.ServerIP.String()

	// Get server public key
	serverKey, err := badgerDB.GetOrCreateWireGuardServerKey()
	if chk.E(err) {
		log.E.F("failed to get WireGuard server key: %v", err)
		http.Error(w, "WireGuard server not configured", http.StatusInternalServerError)
		return
	}

	serverPubKey, err := deriveWGPublicKey(serverKey)
	if chk.E(err) {
		log.E.F("failed to derive server public key: %v", err)
		http.Error(w, "WireGuard server error", http.StatusInternalServerError)
		return
	}

	// Build endpoint
	endpoint := fmt.Sprintf("%s:%d", s.Config.WGEndpoint, s.Config.WGPort)

	// Build response
	resp := WireGuardConfigResponse{
		Interface: WGInterface{
			Address:    clientIP + "/32",
			PrivateKey: base64.StdEncoding.EncodeToString(peer.WGPrivateKey),
		},
		Peer: WGPeer{
			PublicKey:  base64.StdEncoding.EncodeToString(serverPubKey),
			Endpoint:   endpoint,
			AllowedIPs: serverIP + "/32", // Only route bunker traffic to this peer's server IP
		},
	}

	// Generate config text
	resp.ConfigText = fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, resp.Interface.Address, resp.Interface.PrivateKey,
		resp.Peer.PublicKey, resp.Peer.Endpoint, resp.Peer.AllowedIPs)

	// If WireGuard server is running, add the peer
	if s.wireguardServer != nil && s.wireguardServer.IsRunning() {
		if err := s.wireguardServer.AddPeer(pubkey, peer.WGPublicKey, clientIP); chk.E(err) {
			log.W.F("failed to add peer to running WireGuard server: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleWireGuardRegenerate generates a new WireGuard keypair for the user.
// Requires NIP-98 authentication and write+ access.
func (s *Server) handleWireGuardRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if WireGuard is enabled
	if !s.Config.WGEnabled {
		http.Error(w, "WireGuard is not enabled on this relay", http.StatusNotFound)
		return
	}

	// Check if ACL mode supports WireGuard
	if s.Config.ACLMode == "none" {
		http.Error(w, "WireGuard requires ACL mode 'follows' or 'managed'", http.StatusForbidden)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return
	}

	// Check user has write+ access
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "write" && accessLevel != "admin" && accessLevel != "owner" {
		http.Error(w, "Write access required for WireGuard", http.StatusForbidden)
		return
	}

	// Type assert to Badger database for WireGuard methods
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "WireGuard requires Badger database backend", http.StatusInternalServerError)
		return
	}

	// Check subnet pool is available
	if s.subnetPool == nil {
		http.Error(w, "WireGuard subnet pool not initialized", http.StatusInternalServerError)
		return
	}

	// Remove old peer from running server if exists
	oldPeer, err := badgerDB.GetWireGuardPeer(pubkey)
	if err == nil && oldPeer != nil && s.wireguardServer != nil && s.wireguardServer.IsRunning() {
		s.wireguardServer.RemovePeer(oldPeer.WGPublicKey)
	}

	// Regenerate keypair
	peer, err := badgerDB.RegenerateWireGuardPeer(pubkey, s.subnetPool)
	if chk.E(err) {
		log.E.F("failed to regenerate WireGuard peer: %v", err)
		http.Error(w, "Failed to regenerate WireGuard configuration", http.StatusInternalServerError)
		return
	}

	// Derive subnet IPs from sequence (same sequence as before)
	subnet := s.subnetPool.SubnetForSequence(peer.Sequence)
	clientIP := subnet.ClientIP.String()

	log.I.F("regenerated WireGuard keypair for user: %s", hex.Enc(pubkey[:8]))

	// Return success with IP (same subnet as before)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "regenerated",
		"assigned_ip": clientIP,
	})
}

// handleBunkerURL returns the bunker connection URL.
// Requires NIP-98 authentication and write+ access.
func (s *Server) handleBunkerURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if bunker is enabled
	if !s.Config.BunkerEnabled {
		http.Error(w, "Bunker is not enabled on this relay", http.StatusNotFound)
		return
	}

	// Check if WireGuard is enabled (required for bunker)
	if !s.Config.WGEnabled {
		http.Error(w, "WireGuard is required for bunker access", http.StatusNotFound)
		return
	}

	// Check if ACL mode supports WireGuard
	if s.Config.ACLMode == "none" {
		http.Error(w, "Bunker requires ACL mode 'follows' or 'managed'", http.StatusForbidden)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return
	}

	// Check user has write+ access
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "write" && accessLevel != "admin" && accessLevel != "owner" {
		http.Error(w, "Write access required for bunker", http.StatusForbidden)
		return
	}

	// Type assert to Badger database for WireGuard methods
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "Bunker requires Badger database backend", http.StatusInternalServerError)
		return
	}

	// Check subnet pool is available
	if s.subnetPool == nil {
		http.Error(w, "WireGuard subnet pool not initialized", http.StatusInternalServerError)
		return
	}

	// Get or create WireGuard peer to get their subnet
	peer, err := badgerDB.GetOrCreateWireGuardPeer(pubkey, s.subnetPool)
	if chk.E(err) {
		log.E.F("failed to get/create WireGuard peer for bunker: %v", err)
		http.Error(w, "Failed to get WireGuard configuration", http.StatusInternalServerError)
		return
	}

	// Derive server IP for this peer's subnet
	subnet := s.subnetPool.SubnetForSequence(peer.Sequence)
	serverIP := subnet.ServerIP.String()

	// Get relay identity
	relaySecret, err := s.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		log.E.F("failed to get relay identity: %v", err)
		http.Error(w, "Failed to get relay identity", http.StatusInternalServerError)
		return
	}

	relayPubkey, err := deriveNostrPublicKey(relaySecret)
	if chk.E(err) {
		log.E.F("failed to derive relay public key: %v", err)
		http.Error(w, "Failed to derive relay public key", http.StatusInternalServerError)
		return
	}

	// Encode as npub
	relayNpubBytes, err := bech32encoding.BinToNpub(relayPubkey)
	relayNpub := string(relayNpubBytes)
	if chk.E(err) {
		relayNpub = hex.Enc(relayPubkey) // Fallback to hex
	}

	// Build bunker URL using this peer's server IP
	// Format: bunker://<relay-pubkey-hex>?relay=ws://<server-ip>:3335
	relayPubkeyHex := hex.Enc(relayPubkey)
	bunkerURL := fmt.Sprintf("bunker://%s?relay=ws://%s:%d",
		relayPubkeyHex,
		serverIP,
		s.Config.BunkerPort,
	)

	resp := BunkerURLResponse{
		URL:         bunkerURL,
		RelayNpub:   relayNpub,
		RelayPubkey: relayPubkeyHex,
		InternalIP:  serverIP,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleWireGuardStatus returns whether WireGuard/Bunker are available.
func (s *Server) handleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := map[string]interface{}{
		"wireguard_enabled": s.Config.WGEnabled,
		"bunker_enabled":    s.Config.BunkerEnabled,
		"acl_mode":          s.Config.ACLMode,
		"available":         s.Config.WGEnabled && s.Config.ACLMode != "none",
	}

	if s.wireguardServer != nil {
		resp["wireguard_running"] = s.wireguardServer.IsRunning()
		resp["peer_count"] = s.wireguardServer.PeerCount()
	}

	if s.bunkerServer != nil {
		resp["bunker_sessions"] = s.bunkerServer.SessionCount()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RevokedKeyResponse is the JSON response for revoked keys.
type RevokedKeyResponse struct {
	NostrPubkey  string `json:"nostr_pubkey"`
	WGPublicKey  string `json:"wg_public_key"`
	Sequence     uint32 `json:"sequence"`
	ClientIP     string `json:"client_ip"`
	ServerIP     string `json:"server_ip"`
	CreatedAt    int64  `json:"created_at"`
	RevokedAt    int64  `json:"revoked_at"`
	AccessCount  int    `json:"access_count"`
	LastAccessAt int64  `json:"last_access_at"`
}

// AccessLogResponse is the JSON response for access logs.
type AccessLogResponse struct {
	NostrPubkey string `json:"nostr_pubkey"`
	WGPublicKey string `json:"wg_public_key"`
	Sequence    uint32 `json:"sequence"`
	ClientIP    string `json:"client_ip"`
	Timestamp   int64  `json:"timestamp"`
	RemoteAddr  string `json:"remote_addr"`
}

// handleWireGuardAudit returns the user's own revoked keys and access logs.
// This lets users see if their old WireGuard keys are still being used,
// which could indicate they left something on or someone copied their credentials.
// Requires NIP-98 authentication and write+ access.
func (s *Server) handleWireGuardAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if WireGuard is enabled
	if !s.Config.WGEnabled {
		http.Error(w, "WireGuard is not enabled on this relay", http.StatusNotFound)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return
	}

	// Check user has write+ access (same as other WireGuard endpoints)
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "write" && accessLevel != "admin" && accessLevel != "owner" {
		http.Error(w, "Write access required", http.StatusForbidden)
		return
	}

	// Type assert to Badger database for WireGuard methods
	badgerDB, ok := s.DB.(*database.D)
	if !ok {
		http.Error(w, "WireGuard requires Badger database backend", http.StatusInternalServerError)
		return
	}

	// Check subnet pool is available
	if s.subnetPool == nil {
		http.Error(w, "WireGuard subnet pool not initialized", http.StatusInternalServerError)
		return
	}

	// Get this user's revoked keys only
	revokedKeys, err := badgerDB.GetRevokedKeys(pubkey)
	if chk.E(err) {
		log.E.F("failed to get revoked keys: %v", err)
		http.Error(w, "Failed to get revoked keys", http.StatusInternalServerError)
		return
	}

	// Get this user's access logs only
	accessLogs, err := badgerDB.GetAccessLogs(pubkey)
	if chk.E(err) {
		log.E.F("failed to get access logs: %v", err)
		http.Error(w, "Failed to get access logs", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var revokedResp []RevokedKeyResponse
	for _, key := range revokedKeys {
		subnet := s.subnetPool.SubnetForSequence(key.Sequence)
		revokedResp = append(revokedResp, RevokedKeyResponse{
			NostrPubkey:  hex.Enc(key.NostrPubkey),
			WGPublicKey:  hex.Enc(key.WGPublicKey),
			Sequence:     key.Sequence,
			ClientIP:     subnet.ClientIP.String(),
			ServerIP:     subnet.ServerIP.String(),
			CreatedAt:    key.CreatedAt,
			RevokedAt:    key.RevokedAt,
			AccessCount:  key.AccessCount,
			LastAccessAt: key.LastAccessAt,
		})
	}

	var accessResp []AccessLogResponse
	for _, logEntry := range accessLogs {
		subnet := s.subnetPool.SubnetForSequence(logEntry.Sequence)
		accessResp = append(accessResp, AccessLogResponse{
			NostrPubkey: hex.Enc(logEntry.NostrPubkey),
			WGPublicKey: hex.Enc(logEntry.WGPublicKey),
			Sequence:    logEntry.Sequence,
			ClientIP:    subnet.ClientIP.String(),
			Timestamp:   logEntry.Timestamp,
			RemoteAddr:  logEntry.RemoteAddr,
		})
	}

	resp := map[string]interface{}{
		"revoked_keys": revokedResp,
		"access_logs":  accessResp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// deriveWGPublicKey derives a Curve25519 public key from a private key.
func deriveWGPublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, fmt.Errorf("invalid private key length: %d", len(privateKey))
	}

	// Use wireguard package
	return derivePublicKey(privateKey)
}

// deriveNostrPublicKey derives a secp256k1 public key from a secret key.
func deriveNostrPublicKey(secretKey []byte) ([]byte, error) {
	if len(secretKey) != 32 {
		return nil, fmt.Errorf("invalid secret key length: %d", len(secretKey))
	}

	// Use nostr library's key derivation
	pk, err := deriveSecp256k1PublicKey(secretKey)
	if err != nil {
		return nil, err
	}
	return pk, nil
}
