package app

import (
	"encoding/json"
	"net/http"
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
