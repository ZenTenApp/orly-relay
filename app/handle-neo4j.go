package app

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/interfaces/store"
)

// Neo4jConfigResponse is the public response for GET /api/neo4j/config.
// No authentication required — used by the UI to decide whether to show the Neo4j tab.
type Neo4jConfigResponse struct {
	DBType string `json:"db_type"`
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

// cypherRequest is the JSON body for POST /api/neo4j/cypher.
type cypherRequest struct {
	Query   string         `json:"query"`
	Params  map[string]any `json:"params"`
	Timeout int            `json:"timeout"`
}

const (
	cypherDefaultTimeoutSec = 30
	cypherMaxTimeoutSec     = 120
)

// handleNeo4jCypher handles POST /api/neo4j/cypher — NIP-98 owner-gated read-only
// Cypher query proxy. In split IPC mode this flows through gRPC to the database
// process, which holds the actual Neo4j driver connection.
func (s *Server) handleNeo4jCypher(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.Config.Neo4jCypherEnabled {
		http.Error(w, "Cypher endpoint disabled", http.StatusNotFound)
		return
	}

	// NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication required"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Owner-only
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check database supports Cypher
	executor, ok := s.DB.(store.CypherExecutor)
	if !ok {
		http.Error(w, "Database backend does not support Cypher queries", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req cypherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "Missing 'query' field", http.StatusBadRequest)
		return
	}

	// Determine timeout
	timeoutSec := s.Config.Neo4jCypherTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = cypherDefaultTimeoutSec
	}
	if req.Timeout > 0 {
		timeoutSec = req.Timeout
	}
	if timeoutSec > cypherMaxTimeoutSec {
		timeoutSec = cypherMaxTimeoutSec
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	records, err := executor.ExecuteCypherRead(ctx, req.Query, req.Params)
	if err != nil {
		log.W.F("cypher query failed: %v", err)
		http.Error(w, "Cypher query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Enforce max rows
	maxRows := s.Config.Neo4jCypherMaxRows
	if maxRows > 0 && len(records) > maxRows {
		records = records[:maxRows]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}
