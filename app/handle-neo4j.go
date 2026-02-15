package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/acl"
)

// Neo4jConfigResponse is the public response for GET /api/neo4j/config.
// No authentication required — used by the UI to decide whether to show the Neo4j tab.
type Neo4jConfigResponse struct {
	DBType string `json:"db_type"`
}

// Neo4jBoltConfigResponse is the response for GET /api/neo4j/bolt.
// Requires owner-level NIP-98 authentication.
type Neo4jBoltConfigResponse struct {
	BoltSEnabled bool   `json:"bolt_s_enabled"`
	BoltURI      string `json:"bolt_uri,omitempty"`
	TLSCertDir   string `json:"tls_cert_dir,omitempty"`
	BoltPort     int    `json:"bolt_port"`
	ConfPath     string `json:"conf_path"`
	HasCertDir   bool   `json:"has_cert_dir"`
}

// Neo4jBoltToggleRequest is the request body for POST /api/neo4j/bolt/toggle.
type Neo4jBoltToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// Neo4jBoltToggleResponse is the response for POST /api/neo4j/bolt/toggle.
type Neo4jBoltToggleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// handleNeo4jConfig returns basic Neo4j configuration status.
// No authentication required — the UI uses this to decide tab visibility.
func (s *Server) handleNeo4jConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Neo4jConfigResponse{
		DBType: s.Config.DBType,
	})
}

// handleNeo4jBoltConfig returns the current bolt+s configuration status.
// Requires owner-level NIP-98 authentication.
func (s *Server) handleNeo4jBoltConfig(w http.ResponseWriter, r *http.Request) {
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

	// Check if Neo4j is even the active DB type
	if s.Config.DBType != "neo4j" {
		http.Error(w, "Neo4j is not the active database backend", http.StatusBadRequest)
		return
	}

	// Read current bolt+s status from neo4j.conf
	boltSEnabled := readBoltSEnabled(s.Config.Neo4jConfPath)
	hasCertDir := s.Config.Neo4jTLSCertDir != ""

	// Build the bolt+s URI if enabled
	var boltURI string
	if boltSEnabled && hasCertDir {
		// Derive domain from relay URL or TLS domains
		domain := deriveDomain(s)
		if domain != "" {
			boltURI = fmt.Sprintf("bolt+s://%s:%d", domain, s.Config.Neo4jBoltPort)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Neo4jBoltConfigResponse{
		BoltSEnabled: boltSEnabled,
		BoltURI:      boltURI,
		TLSCertDir:   s.Config.Neo4jTLSCertDir,
		BoltPort:     s.Config.Neo4jBoltPort,
		ConfPath:     s.Config.Neo4jConfPath,
		HasCertDir:   hasCertDir,
	})
}

// handleNeo4jBoltToggle enables or disables bolt+s in neo4j.conf and restarts Neo4j.
// Requires owner-level NIP-98 authentication.
func (s *Server) handleNeo4jBoltToggle(w http.ResponseWriter, r *http.Request) {
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

	// Check if Neo4j is even the active DB type
	if s.Config.DBType != "neo4j" {
		http.Error(w, "Neo4j is not the active database backend", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req Neo4jBoltToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate that TLS cert dir is configured when enabling
	if req.Enabled && s.Config.Neo4jTLSCertDir == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Neo4jBoltToggleResponse{
			Success: false,
			Message: "ORLY_NEO4J_TLS_CERT_DIR must be set before enabling bolt+s (e.g., /etc/letsencrypt/live/example.com)",
			Enabled: false,
		})
		return
	}

	// Update neo4j.conf
	confPath := s.Config.Neo4jConfPath
	if err := updateNeo4jConf(confPath, req.Enabled, s.Config.Neo4jTLSCertDir, s.Config.Neo4jBoltPort); err != nil {
		log.E.F("failed to update neo4j.conf: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Neo4jBoltToggleResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update neo4j.conf: %v", err),
			Enabled: readBoltSEnabled(confPath),
		})
		return
	}

	// Restart Neo4j
	restartCmd := s.Config.Neo4jRestartCmd
	if err := restartNeo4j(restartCmd); err != nil {
		log.E.F("failed to restart Neo4j: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Neo4jBoltToggleResponse{
			Success: false,
			Message: fmt.Sprintf("Config updated but Neo4j restart failed: %v", err),
			Enabled: req.Enabled,
		})
		return
	}

	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}
	log.I.F("bolt+s %s in neo4j.conf and Neo4j restarted", action)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Neo4jBoltToggleResponse{
		Success: true,
		Message: fmt.Sprintf("Bolt+s %s and Neo4j restarted successfully", action),
		Enabled: req.Enabled,
	})
}

// readBoltSEnabled reads neo4j.conf and returns whether bolt TLS is set to REQUIRED.
func readBoltSEnabled(confPath string) bool {
	f, err := os.Open(confPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "dbms.connector.bolt.tls_level=") {
			val := strings.TrimPrefix(line, "dbms.connector.bolt.tls_level=")
			return strings.EqualFold(strings.TrimSpace(val), "REQUIRED")
		}
		// Also check newer Neo4j 5.x format
		if strings.HasPrefix(line, "server.bolt.tls_level=") {
			val := strings.TrimPrefix(line, "server.bolt.tls_level=")
			return strings.EqualFold(strings.TrimSpace(val), "REQUIRED")
		}
	}
	return false
}

// updateNeo4jConf modifies neo4j.conf to enable or disable bolt+s TLS.
func updateNeo4jConf(confPath string, enable bool, certDir string, boltPort int) error {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading neo4j.conf: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	result := make([]string, 0, len(lines)+10)

	// Track which settings we've seen so we can append missing ones
	seen := map[string]bool{}

	// Settings to manage — support both Neo4j 4.x and 5.x config keys
	type confSetting struct {
		key4x    string // Neo4j 4.x config key
		key5x    string // Neo4j 5.x config key
		onValue  string // value when bolt+s is enabled
		offValue string // value when bolt+s is disabled
	}

	settings := []confSetting{
		{"dbms.connector.bolt.tls_level", "server.bolt.tls_level", "REQUIRED", "DISABLED"},
		{"dbms.connector.bolt.listen_address", "server.bolt.listen_address", fmt.Sprintf("0.0.0.0:%d", boltPort), fmt.Sprintf("0.0.0.0:%d", boltPort)},
		{"dbms.ssl.policy.bolt.enabled", "dbms.ssl.policy.bolt.enabled", "true", "false"},
		{"dbms.ssl.policy.bolt.base_directory", "dbms.ssl.policy.bolt.base_directory", certDir, ""},
		{"dbms.ssl.policy.bolt.private_key", "dbms.ssl.policy.bolt.private_key", "privkey.pem", ""},
		{"dbms.ssl.policy.bolt.public_certificate", "dbms.ssl.policy.bolt.public_certificate", "fullchain.pem", ""},
		{"dbms.ssl.policy.bolt.client_auth", "dbms.ssl.policy.bolt.client_auth", "NONE", "NONE"},
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line matches any of our managed settings
		matched := false
		for _, s := range settings {
			// Match either the key or a commented-out version of it
			for _, key := range []string{s.key4x, s.key5x} {
				if key == "" {
					continue
				}
				prefix := key + "="
				commentedPrefix := "#" + key + "="
				// Also handle "# key=" with space
				commentedPrefixSpace := "# " + key + "="

				if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(trimmed, commentedPrefix) || strings.HasPrefix(trimmed, commentedPrefixSpace) {
					val := s.onValue
					if !enable {
						val = s.offValue
					}
					if val != "" {
						result = append(result, key+"="+val)
					} else {
						// Comment out the line when disabling
						result = append(result, "#"+key+"=")
					}
					seen[key] = true
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		if !matched {
			result = append(result, line)
		}
	}

	// Append any settings that weren't found in the file (only when enabling)
	if enable {
		for _, s := range settings {
			key := s.key4x // default to 4.x keys for new entries
			if seen[key] || seen[s.key5x] {
				continue
			}
			if s.onValue != "" {
				result = append(result, key+"="+s.onValue)
			}
		}
	}

	// Write back
	output := strings.Join(result, "\n")
	if err := os.WriteFile(confPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing neo4j.conf: %w", err)
	}

	return nil
}

// restartNeo4j executes the configured restart command.
func restartNeo4j(restartCmd string) error {
	parts := strings.Fields(restartCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty restart command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	const restartTimeout = 30 * time.Second
	select {
	case err := <-done:
		return err
	case <-time.After(restartTimeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("restart command timed out after %v", restartTimeout)
	}
}

// deriveDomain extracts the relay's public domain from TLS config or relay URL.
func deriveDomain(s *Server) string {
	// Try TLS domains first
	if len(s.Config.TLSDomains) > 0 {
		return s.Config.TLSDomains[0]
	}
	// Try relay URL
	if s.Config.RelayURL != "" {
		u := s.Config.RelayURL
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
		u = strings.TrimPrefix(u, "wss://")
		u = strings.TrimPrefix(u, "ws://")
		if idx := strings.IndexByte(u, '/'); idx >= 0 {
			u = u[:idx]
		}
		if idx := strings.IndexByte(u, ':'); idx >= 0 {
			u = u[:idx]
		}
		return u
	}
	// Try relay addresses
	if len(s.Config.RelayAddresses) > 0 {
		u := s.Config.RelayAddresses[0]
		u = strings.TrimPrefix(u, "wss://")
		u = strings.TrimPrefix(u, "ws://")
		if idx := strings.IndexByte(u, '/'); idx >= 0 {
			u = u[:idx]
		}
		if idx := strings.IndexByte(u, ':'); idx >= 0 {
			u = u[:idx]
		}
		return u
	}
	return ""
}
