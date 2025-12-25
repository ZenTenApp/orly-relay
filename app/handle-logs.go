package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	lol "lol.mleku.dev"
	"lol.mleku.dev/chk"

	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/logbuffer"
)

// LogsResponse is the response structure for GET /api/logs
type LogsResponse struct {
	Logs    []logbuffer.LogEntry `json:"logs"`
	Total   int                  `json:"total"`
	HasMore bool                 `json:"has_more"`
}

// LogLevelResponse is the response structure for GET /api/logs/level
type LogLevelResponse struct {
	Level string `json:"level"`
}

// LogLevelRequest is the request structure for POST /api/logs/level
type LogLevelRequest struct {
	Level string `json:"level"`
}

// handleGetLogs handles GET /api/logs
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
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

	// Check permissions - require owner level only
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if log buffer is available
	if logbuffer.GlobalBuffer == nil {
		http.Error(w, "Log buffer not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	offset := 0
	limit := 100
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	// Get logs from buffer
	logs := logbuffer.GlobalBuffer.Get(offset, limit)
	total := logbuffer.GlobalBuffer.Count()
	hasMore := offset+len(logs) < total

	response := LogsResponse{
		Logs:    logs,
		Total:   total,
		HasMore: hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClearLogs handles POST /api/logs/clear
func (s *Server) handleClearLogs(w http.ResponseWriter, r *http.Request) {
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

	// Check permissions - require owner level only
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if log buffer is available
	if logbuffer.GlobalBuffer == nil {
		http.Error(w, "Log buffer not enabled", http.StatusServiceUnavailable)
		return
	}

	// Clear the buffer
	logbuffer.GlobalBuffer.Clear()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleLogLevel handles GET and POST /api/logs/level
func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetLogLevel(w, r)
	case http.MethodPost:
		s.handleSetLogLevel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetLogLevel handles GET /api/logs/level
func (s *Server) handleGetLogLevel(w http.ResponseWriter, r *http.Request) {
	// No auth required for reading log level
	level := logbuffer.GetCurrentLevel()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LogLevelResponse{Level: level})
}

// handleSetLogLevel handles POST /api/logs/level
func (s *Server) handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
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

	// Check permissions - require owner level only
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Parse request body
	var req LogLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and set log level
	level := logbuffer.SetCurrentLevel(req.Level)
	lol.SetLogLevel(level)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LogLevelResponse{Level: level})
}
