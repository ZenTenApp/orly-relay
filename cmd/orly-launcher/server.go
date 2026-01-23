package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// AdminServer provides HTTP endpoints for managing the launcher.
type AdminServer struct {
	cfg        *Config
	supervisor *Supervisor
	updater    *Updater
	auth       *AuthMiddleware
	server     *http.Server
	startTime  time.Time
}

// NewAdminServer creates a new admin HTTP server.
func NewAdminServer(cfg *Config, supervisor *Supervisor) *AdminServer {
	return &AdminServer{
		cfg:        cfg,
		supervisor: supervisor,
		updater:    NewUpdater(cfg.BinDir),
		auth:       NewAuthMiddleware(cfg.AdminOwners),
		startTime:  time.Now(),
	}
}

// Start starts the admin HTTP server.
func (s *AdminServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/admin", s.serveUI)
	mux.HandleFunc("/admin/", s.serveUI)

	// Authenticated API endpoints
	mux.HandleFunc("/api/status", s.auth.RequireAuth(s.handleStatus))
	mux.HandleFunc("/api/config", s.auth.RequireAuth(s.handleConfig))
	mux.HandleFunc("/api/binaries", s.auth.RequireAuth(s.handleBinaries))
	mux.HandleFunc("/api/update", s.auth.RequireAuth(s.handleUpdate))
	mux.HandleFunc("/api/restart", s.auth.RequireAuth(s.handleRestart))
	mux.HandleFunc("/api/rollback", s.auth.RequireAuth(s.handleRollback))

	addr := fmt.Sprintf(":%d", s.cfg.AdminPort)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.I.F("starting admin server on %s", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	return s.server.ListenAndServe()
}

// StatusResponse is the response for GET /api/status
type StatusResponse struct {
	Version   string          `json:"version"`
	Uptime    string          `json:"uptime"`
	Processes []ProcessStatus `json:"processes"`
}

// ProcessStatus represents the status of a single managed process.
type ProcessStatus struct {
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	Restarts  int    `json:"restarts"`
	StartedAt string `json:"started_at,omitempty"`
}

func (s *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Round(time.Second).String()
	processes := s.supervisor.GetProcessStatuses()

	response := StatusResponse{
		Version:   s.updater.CurrentVersion(),
		Uptime:    uptime,
		Processes: processes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConfigResponse is the response for GET /api/config
type ConfigResponse struct {
	DBBackend              string   `json:"db_backend"`
	DBBinary               string   `json:"db_binary"`
	RelayBinary            string   `json:"relay_binary"`
	ACLBinary              string   `json:"acl_binary"`
	DBListen               string   `json:"db_listen"`
	ACLListen              string   `json:"acl_listen"`
	ACLEnabled             bool     `json:"acl_enabled"`
	ACLMode                string   `json:"acl_mode"`
	DataDir                string   `json:"data_dir"`
	LogLevel               string   `json:"log_level"`
	DistributedSyncEnabled bool     `json:"distributed_sync_enabled"`
	ClusterSyncEnabled     bool     `json:"cluster_sync_enabled"`
	RelayGroupEnabled      bool     `json:"relay_group_enabled"`
	NegentropyEnabled      bool     `json:"negentropy_enabled"`
	AdminOwners            []string `json:"admin_owners"`
	BinDir                 string   `json:"bin_dir"`
}

func (s *AdminServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPost:
		s.handleSetConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *AdminServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	response := ConfigResponse{
		DBBackend:              s.cfg.DBBackend,
		DBBinary:               s.cfg.DBBinary,
		RelayBinary:            s.cfg.RelayBinary,
		ACLBinary:              s.cfg.ACLBinary,
		DBListen:               s.cfg.DBListen,
		ACLListen:              s.cfg.ACLListen,
		ACLEnabled:             s.cfg.ACLEnabled,
		ACLMode:                s.cfg.ACLMode,
		DataDir:                s.cfg.DataDir,
		LogLevel:               s.cfg.LogLevel,
		DistributedSyncEnabled: s.cfg.DistributedSyncEnabled,
		ClusterSyncEnabled:     s.cfg.ClusterSyncEnabled,
		RelayGroupEnabled:      s.cfg.RelayGroupEnabled,
		NegentropyEnabled:      s.cfg.NegentropyEnabled,
		AdminOwners:            s.auth.Owners(),
		BinDir:                 s.cfg.BinDir,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetConfigRequest is the request body for POST /api/config
type SetConfigRequest struct {
	DBBackend              string   `json:"db_backend,omitempty"`
	DBBinary               string   `json:"db_binary,omitempty"`
	RelayBinary            string   `json:"relay_binary,omitempty"`
	ACLBinary              string   `json:"acl_binary,omitempty"`
	DBListen               string   `json:"db_listen,omitempty"`
	ACLListen              string   `json:"acl_listen,omitempty"`
	ACLEnabled             *bool    `json:"acl_enabled,omitempty"`
	ACLMode                string   `json:"acl_mode,omitempty"`
	DataDir                string   `json:"data_dir,omitempty"`
	LogLevel               string   `json:"log_level,omitempty"`
	AdminPort              *int     `json:"admin_port,omitempty"`
	AdminOwners            []string `json:"admin_owners,omitempty"`
	BinDir                 string   `json:"bin_dir,omitempty"`
	DistributedSyncEnabled *bool    `json:"distributed_sync_enabled,omitempty"`
	ClusterSyncEnabled     *bool    `json:"cluster_sync_enabled,omitempty"`
	RelayGroupEnabled      *bool    `json:"relay_group_enabled,omitempty"`
	NegentropyEnabled      *bool    `json:"negentropy_enabled,omitempty"`
}

// SetConfigResponse is the response for POST /api/config
type SetConfigResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	RestartNeeded  bool   `json:"restart_needed"`
	ConfigFilePath string `json:"config_file_path"`
}

func (s *AdminServer) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var req SetConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); chk.E(err) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Load existing config file or create new
	cf, err := loadConfigFile()
	if chk.E(err) {
		cf = &ConfigFile{}
	}

	// Update only fields that were provided
	if req.DBBackend != "" {
		cf.DBBackend = req.DBBackend
	}
	if req.DBBinary != "" {
		cf.DBBinary = req.DBBinary
	}
	if req.RelayBinary != "" {
		cf.RelayBinary = req.RelayBinary
	}
	if req.ACLBinary != "" {
		cf.ACLBinary = req.ACLBinary
	}
	if req.DBListen != "" {
		cf.DBListen = req.DBListen
	}
	if req.ACLListen != "" {
		cf.ACLListen = req.ACLListen
	}
	if req.ACLEnabled != nil {
		cf.ACLEnabled = req.ACLEnabled
	}
	if req.ACLMode != "" {
		cf.ACLMode = req.ACLMode
	}
	if req.DataDir != "" {
		cf.DataDir = req.DataDir
	}
	if req.LogLevel != "" {
		cf.LogLevel = req.LogLevel
	}
	if req.AdminPort != nil {
		cf.AdminPort = req.AdminPort
	}
	if req.AdminOwners != nil {
		cf.AdminOwners = req.AdminOwners
	}
	if req.BinDir != "" {
		cf.BinDir = req.BinDir
	}
	if req.DistributedSyncEnabled != nil {
		cf.DistributedSyncEnabled = req.DistributedSyncEnabled
	}
	if req.ClusterSyncEnabled != nil {
		cf.ClusterSyncEnabled = req.ClusterSyncEnabled
	}
	if req.RelayGroupEnabled != nil {
		cf.RelayGroupEnabled = req.RelayGroupEnabled
	}
	if req.NegentropyEnabled != nil {
		cf.NegentropyEnabled = req.NegentropyEnabled
	}

	// Save to file
	if err := SaveConfigFile(cf); chk.E(err) {
		response := SetConfigResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save config: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update auth middleware if owners changed
	if req.AdminOwners != nil {
		for _, owner := range s.auth.Owners() {
			s.auth.RemoveOwner(owner)
		}
		for _, owner := range req.AdminOwners {
			s.auth.AddOwner(owner)
		}
	}

	response := SetConfigResponse{
		Success:        true,
		Message:        "Configuration saved. Restart required for most changes to take effect.",
		RestartNeeded:  true,
		ConfigFilePath: configFilePath(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BinariesResponse is the response for GET /api/binaries
type BinariesResponse struct {
	CurrentVersion    string        `json:"current_version"`
	AvailableVersions []VersionInfo `json:"available_versions"`
}

func (s *AdminServer) handleBinaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := BinariesResponse{
		CurrentVersion:    s.updater.CurrentVersion(),
		AvailableVersions: s.updater.ListVersions(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateRequest is the request body for POST /api/update
type UpdateRequest struct {
	Version string            `json:"version"`
	URLs    map[string]string `json:"urls"` // binary name -> download URL
}

// UpdateResponse is the response for POST /api/update
type UpdateResponse struct {
	Success        bool     `json:"success"`
	Message        string   `json:"message"`
	Version        string   `json:"version"`
	DownloadedFiles []string `json:"downloaded_files"`
}

func (s *AdminServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); chk.E(err) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	if len(req.URLs) == 0 {
		http.Error(w, "At least one binary URL is required", http.StatusBadRequest)
		return
	}

	// Perform the update
	downloadedFiles, err := s.updater.Update(req.Version, req.URLs)
	if chk.E(err) {
		response := UpdateResponse{
			Success: false,
			Message: err.Error(),
			Version: req.Version,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := UpdateResponse{
		Success:        true,
		Message:        fmt.Sprintf("Successfully updated to version %s", req.Version),
		Version:        req.Version,
		DownloadedFiles: downloadedFiles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RestartResponse is the response for POST /api/restart
type RestartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *AdminServer) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Signal supervisor to restart all processes
	go func() {
		if err := s.supervisor.RestartAll(); chk.E(err) {
			log.E.F("restart failed: %v", err)
		}
	}()

	response := RestartResponse{
		Success: true,
		Message: "Restart initiated",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RollbackResponse is the response for POST /api/rollback
type RollbackResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	PreviousVersion string `json:"previous_version"`
	CurrentVersion  string `json:"current_version"`
}

func (s *AdminServer) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	previousVersion := s.updater.CurrentVersion()

	if err := s.updater.Rollback(); chk.E(err) {
		response := RollbackResponse{
			Success: false,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := RollbackResponse{
		Success:         true,
		Message:         "Rollback successful - restart required to apply",
		PreviousVersion: previousVersion,
		CurrentVersion:  s.updater.CurrentVersion(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *AdminServer) serveUI(w http.ResponseWriter, r *http.Request) {
	s.serveAdminUI(w, r)
}
