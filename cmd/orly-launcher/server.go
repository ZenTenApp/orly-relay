package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	mux.HandleFunc("/api/releases", s.auth.RequireAuth(s.handleReleases))
	mux.HandleFunc("/api/restart", s.auth.RequireAuth(s.handleRestart))
	mux.HandleFunc("/api/restart-service", s.auth.RequireAuth(s.handleRestartService))
	mux.HandleFunc("/api/rollback", s.auth.RequireAuth(s.handleRollback))
	mux.HandleFunc("/api/start-services", s.auth.RequireAuth(s.handleStartServices))
	mux.HandleFunc("/api/stop-services", s.auth.RequireAuth(s.handleStopServices))
	mux.HandleFunc("/api/start-service", s.auth.RequireAuth(s.handleStartService))
	mux.HandleFunc("/api/stop-service", s.auth.RequireAuth(s.handleStopService))

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
	Version         string          `json:"version"`
	Uptime          string          `json:"uptime"`
	ServicesRunning bool            `json:"services_running"`
	Processes       []ProcessStatus `json:"processes"`
}

// ProcessStatus represents the status of a single managed process.
type ProcessStatus struct {
	Name        string `json:"name"`
	Binary      string `json:"binary"`
	Version     string `json:"version"`
	Status      string `json:"status"`   // running, stopped, disabled
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category"` // database, acl, sync, certs, relay
	Description string `json:"description"`
	PID         int    `json:"pid"`
	Restarts    int    `json:"restarts"`
	StartedAt   string `json:"started_at,omitempty"`
}

func (s *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Round(time.Second).String()
	processes := s.supervisor.GetProcessStatuses()

	response := StatusResponse{
		Version:         s.updater.CurrentVersion(),
		Uptime:          uptime,
		ServicesRunning: s.supervisor.IsRunning(),
		Processes:       processes,
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

// ReleasesResponse is the response for GET /api/releases
type ReleasesResponse struct {
	Releases []ReleaseInfo `json:"releases"`
}

// ReleaseInfo represents a single release/tag
type ReleaseInfo struct {
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

const tagsAPIURL = "https://git.nostrdev.com/api/v1/repos/mleku/next.orly.dev/tags"

func (s *AdminServer) handleReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch tags from upstream
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(tagsAPIURL)
	if err != nil {
		log.E.F("failed to fetch tags: %v", err)
		http.Error(w, "Failed to fetch releases", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch releases", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// Parse the tags response
	var tags []struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &tags); chk.E(err) {
		http.Error(w, "Failed to parse response", http.StatusInternalServerError)
		return
	}

	// Filter and transform to our response format
	var releases []ReleaseInfo
	for _, tag := range tags {
		if len(tag.Name) > 0 && tag.Name[0] == 'v' {
			msg := tag.Message
			// Get first line only
			for i, c := range msg {
				if c == '\n' {
					msg = msg[:i]
					break
				}
			}
			releases = append(releases, ReleaseInfo{
				Tag:     tag.Name,
				Message: msg,
			})
		}
		if len(releases) >= 15 {
			break
		}
	}

	response := ReleasesResponse{Releases: releases}
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

// RestartServiceRequest is the request body for POST /api/restart-service
type RestartServiceRequest struct {
	Service string `json:"service"`
}

// RestartServiceResponse is the response for POST /api/restart-service
type RestartServiceResponse struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	Restarted []string `json:"restarted"`
}

func (s *AdminServer) handleRestartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RestartServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); chk.E(err) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Service == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}

	// Map binary names to service names
	serviceName := req.Service
	switch req.Service {
	case "orly-db-badger", "orly-db-neo4j":
		serviceName = "orly-db"
	case "orly-acl-follows", "orly-acl-managed", "orly-acl-curation":
		serviceName = "orly-acl"
	}

	// Perform the restart in a goroutine to avoid blocking
	go func() {
		if restarted, err := s.supervisor.RestartService(serviceName); chk.E(err) {
			log.E.F("restart service %s failed: %v", serviceName, err)
		} else {
			log.I.F("restart service completed: %v", restarted)
		}
	}()

	response := RestartServiceResponse{
		Success: true,
		Message: fmt.Sprintf("Restart of %s initiated", serviceName),
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

// StartServicesResponse is the response for POST /api/start-services
type StartServicesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *AdminServer) handleStartServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if services are already running
	if s.supervisor.IsRunning() {
		response := StartServicesResponse{
			Success: false,
			Message: "Services are already running",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Start services in a goroutine
	go func() {
		if err := s.supervisor.Start(); chk.E(err) {
			log.E.F("failed to start services: %v", err)
		}
	}()

	response := StartServicesResponse{
		Success: true,
		Message: "Services starting...",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StopServicesResponse is the response for POST /api/stop-services
type StopServicesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *AdminServer) handleStopServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if services are running
	if !s.supervisor.IsRunning() {
		response := StopServicesResponse{
			Success: false,
			Message: "Services are not running",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Stop services in a goroutine
	go func() {
		if err := s.supervisor.Stop(); chk.E(err) {
			log.E.F("failed to stop services: %v", err)
		}
	}()

	response := StopServicesResponse{
		Success: true,
		Message: "Services stopping...",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StartServiceRequest is the request body for POST /api/start-service
type StartServiceRequest struct {
	Service string `json:"service"`
}

// StartServiceResponse is the response for POST /api/start-service
type StartServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *AdminServer) handleStartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); chk.E(err) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Service == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}

	// Start the service
	go func() {
		if err := s.supervisor.StartService(req.Service); chk.E(err) {
			log.E.F("start service %s failed: %v", req.Service, err)
		} else {
			log.I.F("started service: %s", req.Service)
		}
	}()

	response := StartServiceResponse{
		Success: true,
		Message: fmt.Sprintf("Start of %s initiated", req.Service),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StopServiceRequest is the request body for POST /api/stop-service
type StopServiceRequest struct {
	Service string `json:"service"`
}

// StopServiceResponse is the response for POST /api/stop-service
type StopServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *AdminServer) handleStopService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StopServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); chk.E(err) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Service == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}

	// Stop the service
	go func() {
		if err := s.supervisor.StopService(req.Service); chk.E(err) {
			log.E.F("stop service %s failed: %v", req.Service, err)
		} else {
			log.I.F("stopped service: %s", req.Service)
		}
	}()

	response := StopServiceResponse{
		Success: true,
		Message: fmt.Sprintf("Stop of %s initiated", req.Service),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *AdminServer) serveUI(w http.ResponseWriter, r *http.Request) {
	s.serveAdminUI(w, r)
}
