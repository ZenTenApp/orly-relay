package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"next.orly.dev/app/config"
)

// --- readBoltSEnabled tests ---

func TestReadBoltSEnabled_4xEnabled(t *testing.T) {
	f := writeTempConf(t, `# Neo4j 4.x config
dbms.connector.bolt.tls_level=REQUIRED
dbms.ssl.policy.bolt.enabled=true
`)
	if !readBoltSEnabled(f) {
		t.Error("expected bolt+s enabled for 4.x REQUIRED config")
	}
}

func TestReadBoltSEnabled_4xDisabled(t *testing.T) {
	f := writeTempConf(t, `# Neo4j 4.x config
dbms.connector.bolt.tls_level=DISABLED
`)
	if readBoltSEnabled(f) {
		t.Error("expected bolt+s disabled for 4.x DISABLED config")
	}
}

func TestReadBoltSEnabled_5xEnabled(t *testing.T) {
	f := writeTempConf(t, `# Neo4j 5.x config
server.bolt.tls_level=REQUIRED
`)
	if !readBoltSEnabled(f) {
		t.Error("expected bolt+s enabled for 5.x REQUIRED config")
	}
}

func TestReadBoltSEnabled_5xDisabled(t *testing.T) {
	f := writeTempConf(t, `# Neo4j 5.x config
server.bolt.tls_level=DISABLED
`)
	if readBoltSEnabled(f) {
		t.Error("expected bolt+s disabled for 5.x DISABLED config")
	}
}

func TestReadBoltSEnabled_CommentedOut(t *testing.T) {
	f := writeTempConf(t, `# Some comment
#dbms.connector.bolt.tls_level=REQUIRED
`)
	if readBoltSEnabled(f) {
		t.Error("expected bolt+s disabled when tls_level is commented out")
	}
}

func TestReadBoltSEnabled_MissingSetting(t *testing.T) {
	f := writeTempConf(t, `# No bolt settings here
dbms.connector.bolt.listen_address=0.0.0.0:7687
`)
	if readBoltSEnabled(f) {
		t.Error("expected bolt+s disabled when tls_level setting is absent")
	}
}

func TestReadBoltSEnabled_EmptyFile(t *testing.T) {
	f := writeTempConf(t, "")
	if readBoltSEnabled(f) {
		t.Error("expected bolt+s disabled for empty config")
	}
}

func TestReadBoltSEnabled_MissingFile(t *testing.T) {
	if readBoltSEnabled("/nonexistent/path/neo4j.conf") {
		t.Error("expected bolt+s disabled for missing file")
	}
}

func TestReadBoltSEnabled_CaseInsensitive(t *testing.T) {
	f := writeTempConf(t, `dbms.connector.bolt.tls_level=required
`)
	if !readBoltSEnabled(f) {
		t.Error("expected bolt+s enabled for lowercase 'required'")
	}
}

func TestReadBoltSEnabled_ValueWithSpaces(t *testing.T) {
	f := writeTempConf(t, `dbms.connector.bolt.tls_level= REQUIRED
`)
	if !readBoltSEnabled(f) {
		t.Error("expected bolt+s enabled with trailing/leading spaces around value")
	}
}

// --- updateNeo4jConf tests ---

func TestUpdateNeo4jConf_Enable(t *testing.T) {
	f := writeTempConf(t, `# Default config
dbms.connector.bolt.tls_level=DISABLED
dbms.connector.bolt.listen_address=0.0.0.0:7687
dbms.ssl.policy.bolt.enabled=false
`)

	if err := updateNeo4jConf(f, true, "/etc/letsencrypt/live/example.com", 7687); err != nil {
		t.Fatalf("updateNeo4jConf enable failed: %v", err)
	}

	content := readFile(t, f)

	assertContains(t, content, "dbms.connector.bolt.tls_level=REQUIRED")
	assertContains(t, content, "dbms.ssl.policy.bolt.enabled=true")
	assertContains(t, content, "dbms.ssl.policy.bolt.base_directory=/etc/letsencrypt/live/example.com")
	assertContains(t, content, "dbms.ssl.policy.bolt.private_key=privkey.pem")
	assertContains(t, content, "dbms.ssl.policy.bolt.public_certificate=fullchain.pem")
	assertContains(t, content, "dbms.ssl.policy.bolt.client_auth=NONE")
	assertNotContains(t, content, "tls_level=DISABLED")
}

func TestUpdateNeo4jConf_Disable(t *testing.T) {
	f := writeTempConf(t, `# Enabled config
dbms.connector.bolt.tls_level=REQUIRED
dbms.connector.bolt.listen_address=0.0.0.0:7687
dbms.ssl.policy.bolt.enabled=true
dbms.ssl.policy.bolt.base_directory=/etc/letsencrypt/live/example.com
dbms.ssl.policy.bolt.private_key=privkey.pem
dbms.ssl.policy.bolt.public_certificate=fullchain.pem
dbms.ssl.policy.bolt.client_auth=NONE
`)

	if err := updateNeo4jConf(f, false, "/etc/letsencrypt/live/example.com", 7687); err != nil {
		t.Fatalf("updateNeo4jConf disable failed: %v", err)
	}

	content := readFile(t, f)

	assertContains(t, content, "dbms.connector.bolt.tls_level=DISABLED")
	assertContains(t, content, "dbms.ssl.policy.bolt.enabled=false")
	// base_directory and private_key and public_certificate should be commented out
	assertContains(t, content, "#dbms.ssl.policy.bolt.base_directory=")
	assertContains(t, content, "#dbms.ssl.policy.bolt.private_key=")
	assertContains(t, content, "#dbms.ssl.policy.bolt.public_certificate=")
	assertNotContains(t, content, "tls_level=REQUIRED")
}

func TestUpdateNeo4jConf_EnableFromCommentedOut(t *testing.T) {
	f := writeTempConf(t, `# Default config
#dbms.connector.bolt.tls_level=DISABLED
# dbms.ssl.policy.bolt.enabled=false
`)

	if err := updateNeo4jConf(f, true, "/certs", 7687); err != nil {
		t.Fatalf("updateNeo4jConf from commented failed: %v", err)
	}

	content := readFile(t, f)

	assertContains(t, content, "dbms.connector.bolt.tls_level=REQUIRED")
	assertContains(t, content, "dbms.ssl.policy.bolt.enabled=true")
	// Should not still have the commented versions
	assertNotContains(t, content, "#dbms.connector.bolt.tls_level=")
	assertNotContains(t, content, "# dbms.ssl.policy.bolt.enabled=")
}

func TestUpdateNeo4jConf_AppendsWhenMissing(t *testing.T) {
	f := writeTempConf(t, `# Minimal config
dbms.default_listen_address=0.0.0.0
`)

	if err := updateNeo4jConf(f, true, "/certs", 7687); err != nil {
		t.Fatalf("updateNeo4jConf append failed: %v", err)
	}

	content := readFile(t, f)

	// All required settings should be appended
	assertContains(t, content, "dbms.connector.bolt.tls_level=REQUIRED")
	assertContains(t, content, "dbms.connector.bolt.listen_address=0.0.0.0:7687")
	assertContains(t, content, "dbms.ssl.policy.bolt.enabled=true")
	assertContains(t, content, "dbms.ssl.policy.bolt.base_directory=/certs")
	assertContains(t, content, "dbms.ssl.policy.bolt.private_key=privkey.pem")
	assertContains(t, content, "dbms.ssl.policy.bolt.public_certificate=fullchain.pem")
	assertContains(t, content, "dbms.ssl.policy.bolt.client_auth=NONE")
	// Original content preserved
	assertContains(t, content, "dbms.default_listen_address=0.0.0.0")
}

func TestUpdateNeo4jConf_DisableDoesNotAppend(t *testing.T) {
	f := writeTempConf(t, `# Minimal config
dbms.default_listen_address=0.0.0.0
`)

	if err := updateNeo4jConf(f, false, "", 7687); err != nil {
		t.Fatalf("updateNeo4jConf disable no-append failed: %v", err)
	}

	content := readFile(t, f)

	// Disabling should NOT add new settings
	assertNotContains(t, content, "tls_level")
	assertNotContains(t, content, "ssl.policy.bolt")
	// Original content preserved
	assertContains(t, content, "dbms.default_listen_address=0.0.0.0")
}

func TestUpdateNeo4jConf_5xKeys(t *testing.T) {
	f := writeTempConf(t, `# Neo4j 5.x config
server.bolt.tls_level=DISABLED
server.bolt.listen_address=0.0.0.0:7687
`)

	if err := updateNeo4jConf(f, true, "/certs", 7687); err != nil {
		t.Fatalf("updateNeo4jConf 5.x failed: %v", err)
	}

	content := readFile(t, f)

	assertContains(t, content, "server.bolt.tls_level=REQUIRED")
	assertContains(t, content, "server.bolt.listen_address=0.0.0.0:7687")
}

func TestUpdateNeo4jConf_CustomBoltPort(t *testing.T) {
	f := writeTempConf(t, `# Config
dbms.connector.bolt.listen_address=0.0.0.0:7687
`)

	if err := updateNeo4jConf(f, true, "/certs", 7688); err != nil {
		t.Fatalf("updateNeo4jConf custom port failed: %v", err)
	}

	content := readFile(t, f)
	assertContains(t, content, "dbms.connector.bolt.listen_address=0.0.0.0:7688")
}

func TestUpdateNeo4jConf_MissingFile(t *testing.T) {
	err := updateNeo4jConf("/nonexistent/path/neo4j.conf", true, "/certs", 7687)
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestUpdateNeo4jConf_PreservesUnrelatedLines(t *testing.T) {
	original := `# Neo4j Configuration
# See https://neo4j.com/docs/

# Memory settings
dbms.memory.heap.initial_size=512m
dbms.memory.heap.max_size=1G

# Connector settings
dbms.connector.bolt.tls_level=DISABLED
dbms.connector.http.listen_address=0.0.0.0:7474

# Security
dbms.security.auth_enabled=true
`
	f := writeTempConf(t, original)

	if err := updateNeo4jConf(f, true, "/certs", 7687); err != nil {
		t.Fatalf("updateNeo4jConf preserve failed: %v", err)
	}

	content := readFile(t, f)

	// Unrelated settings preserved
	assertContains(t, content, "dbms.memory.heap.initial_size=512m")
	assertContains(t, content, "dbms.memory.heap.max_size=1G")
	assertContains(t, content, "dbms.connector.http.listen_address=0.0.0.0:7474")
	assertContains(t, content, "dbms.security.auth_enabled=true")
	// Comments preserved
	assertContains(t, content, "# Neo4j Configuration")
	assertContains(t, content, "# Memory settings")
	// Bolt setting changed
	assertContains(t, content, "dbms.connector.bolt.tls_level=REQUIRED")
}

// --- deriveDomain tests ---

func TestDeriveDomain_TLSDomains(t *testing.T) {
	s := &Server{Config: &config.C{
		TLSDomains: []string{"relay.example.com", "backup.example.com"},
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain TLS: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_RelayURLHTTPS(t *testing.T) {
	s := &Server{Config: &config.C{
		RelayURL: "https://relay.example.com",
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain HTTPS: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_RelayURLWSS(t *testing.T) {
	s := &Server{Config: &config.C{
		RelayURL: "wss://relay.example.com/",
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain WSS: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_RelayURLWithPort(t *testing.T) {
	s := &Server{Config: &config.C{
		RelayURL: "https://relay.example.com:3334",
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain with port: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_RelayURLWithPath(t *testing.T) {
	s := &Server{Config: &config.C{
		RelayURL: "https://relay.example.com/path/to/thing",
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain with path: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_RelayAddresses(t *testing.T) {
	s := &Server{Config: &config.C{
		RelayAddresses: []string{"wss://relay.example.com"},
	}}
	got := deriveDomain(s)
	if got != "relay.example.com" {
		t.Errorf("deriveDomain addresses: got %q, want %q", got, "relay.example.com")
	}
}

func TestDeriveDomain_Precedence(t *testing.T) {
	// TLSDomains takes precedence over RelayURL and RelayAddresses
	s := &Server{Config: &config.C{
		TLSDomains:     []string{"tls.example.com"},
		RelayURL:       "https://url.example.com",
		RelayAddresses: []string{"wss://addr.example.com"},
	}}
	got := deriveDomain(s)
	if got != "tls.example.com" {
		t.Errorf("deriveDomain precedence: got %q, want %q", got, "tls.example.com")
	}
}

func TestDeriveDomain_Empty(t *testing.T) {
	s := &Server{Config: &config.C{}}
	got := deriveDomain(s)
	if got != "" {
		t.Errorf("deriveDomain empty: got %q, want empty", got)
	}
}

// --- handleNeo4jConfig handler test ---

func TestHandleNeo4jConfig_ReturnsDBType(t *testing.T) {
	s := &Server{Config: &config.C{DBType: "neo4j"}}

	req := httptest.NewRequest(http.MethodGet, "/api/neo4j/config", nil)
	rr := httptest.NewRecorder()

	s.handleNeo4jConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp Neo4jConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DBType != "neo4j" {
		t.Errorf("db_type: got %q, want %q", resp.DBType, "neo4j")
	}
}

func TestHandleNeo4jConfig_ReturnsBadger(t *testing.T) {
	s := &Server{Config: &config.C{DBType: "badger"}}

	req := httptest.NewRequest(http.MethodGet, "/api/neo4j/config", nil)
	rr := httptest.NewRecorder()

	s.handleNeo4jConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp Neo4jConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DBType != "badger" {
		t.Errorf("db_type: got %q, want %q", resp.DBType, "badger")
	}
}

func TestHandleNeo4jConfig_MethodNotAllowed(t *testing.T) {
	s := &Server{Config: &config.C{DBType: "neo4j"}}

	req := httptest.NewRequest(http.MethodPost, "/api/neo4j/config", nil)
	rr := httptest.NewRecorder()

	s.handleNeo4jConfig(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- restartNeo4j tests ---

func TestRestartNeo4j_EmptyCommand(t *testing.T) {
	err := restartNeo4j("")
	if err == nil {
		t.Error("expected error for empty restart command")
	}
}

func TestRestartNeo4j_SuccessfulCommand(t *testing.T) {
	// Use 'true' command which always succeeds
	err := restartNeo4j("true")
	if err != nil {
		t.Errorf("expected success for 'true' command: %v", err)
	}
}

func TestRestartNeo4j_FailingCommand(t *testing.T) {
	err := restartNeo4j("false")
	if err == nil {
		t.Error("expected error for 'false' command")
	}
}

// --- test helpers ---

func writeTempConf(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "neo4j.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp conf: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected content to contain %q\n--- content ---\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected content NOT to contain %q\n--- content ---\n%s", needle, haystack)
	}
}
