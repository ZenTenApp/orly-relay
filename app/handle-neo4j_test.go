package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"next.orly.dev/app/config"
)

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
