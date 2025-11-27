package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/blossom"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/policy"
	"git.mleku.dev/mleku/nostr/protocol/auth"
	"git.mleku.dev/mleku/nostr/httpauth"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/spider"
	dsync "next.orly.dev/pkg/sync"
)

type Server struct {
	mux        *http.ServeMux
	Config     *config.C
	Ctx        context.Context
	publishers *publish.S
	Admins     [][]byte
	Owners     [][]byte
	DB         database.Database // Changed from embedded *database.D to interface field

	// optional reverse proxy for dev web server
	devProxy *httputil.ReverseProxy

	// Challenge storage for HTTP UI authentication
	challengeMutex sync.RWMutex
	challenges     map[string][]byte

	// Message processing pause mutex for policy/follow list updates
	// Use RLock() for normal message processing, Lock() for updates
	messagePauseMutex sync.RWMutex

	paymentProcessor  *PaymentProcessor
	sprocketManager   *SprocketManager
	policyManager     *policy.P
	spiderManager     *spider.Spider
	directorySpider   *spider.DirectorySpider
	syncManager       *dsync.Manager
	relayGroupMgr     *dsync.RelayGroupManager
	clusterManager    *dsync.ClusterManager
	blossomServer     *blossom.Server
	InviteManager     *nip43.InviteManager
	cfg               *config.C
	db                database.Database // Changed from *database.D to interface
}

// isIPBlacklisted checks if an IP address is blacklisted using the managed ACL system
func (s *Server) isIPBlacklisted(remote string) bool {
	// Extract IP from remote address (e.g., "192.168.1.1:12345" -> "192.168.1.1")
	remoteIP := strings.Split(remote, ":")[0]

	// Check static IP blacklist from config first
	if len(s.Config.IPBlacklist) > 0 {
		for _, blocked := range s.Config.IPBlacklist {
			// Allow simple prefix matching for subnets (e.g., "192.168" matches 192.168.0.0/16)
			if blocked != "" && strings.HasPrefix(remoteIP, blocked) {
				return true
			}
		}
	}

	// Check if managed ACL is available and active
	if s.Config.ACLMode == "managed" {
		for _, aclInstance := range acl.Registry.ACL {
			if aclInstance.Type() == "managed" {
				if managed, ok := aclInstance.(*acl.Managed); ok {
					return managed.IsIPBlocked(remoteIP)
				}
			}
		}
	}

	return false
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers should be handled by the reverse proxy (Caddy/nginx)
	// to avoid duplicate headers. If running without a reverse proxy,
	// uncomment the CORS configuration below or configure via environment variable.

	// Handle preflight OPTIONS requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Log proxy information for debugging (only for WebSocket requests to avoid spam)
	if r.Header.Get("Upgrade") == "websocket" {
		LogProxyInfo(r, "HTTP request")
	}

	// If this is a websocket request, only intercept the relay root path.
	// This allows other websocket paths (e.g., Vite HMR) to be handled by the dev proxy when enabled.
	if r.Header.Get("Upgrade") == "websocket" {
		if s.mux != nil && s.Config != nil && s.Config.WebDisableEmbedded && s.Config.WebDevProxyURL != "" && r.URL.Path != "/" {
			// forward to mux (which will proxy to dev server)
			s.mux.ServeHTTP(w, r)
			return
		}
		s.HandleWebsocket(w, r)
		return
	}

	if r.Header.Get("Accept") == "application/nostr+json" {
		s.HandleRelayInfo(w, r)
		return
	}

	if s.mux == nil {
		http.Error(w, "Upgrade required", http.StatusUpgradeRequired)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) ServiceURL(req *http.Request) (url string) {
	proto := req.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if req.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = req.Host
	}
	return proto + "://" + host
}

func (s *Server) WebSocketURL(req *http.Request) (url string) {
	proto := req.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if req.TLS != nil {
			proto = "wss"
		} else {
			proto = "ws"
		}
	} else {
		// Convert HTTP scheme to WebSocket scheme
		if proto == "https" {
			proto = "wss"
		} else if proto == "http" {
			proto = "ws"
		}
	}
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = req.Host
	}
	return proto + "://" + strings.TrimRight(host, "/") + "/"
}

func (s *Server) DashboardURL(req *http.Request) (url string) {
	return s.ServiceURL(req) + "/"
}

// UserInterface sets up a basic Nostr NDK interface that allows users to log into the relay user interface
func (s *Server) UserInterface() {
	if s.mux == nil {
		s.mux = http.NewServeMux()
	}

	// If dev proxy is configured, initialize it
	if s.Config != nil && s.Config.WebDisableEmbedded && s.Config.WebDevProxyURL != "" {
		proxyURL := s.Config.WebDevProxyURL
		// Add default scheme if missing to avoid: proxy error: unsupported protocol scheme ""
		if !strings.Contains(proxyURL, "://") {
			proxyURL = "http://" + proxyURL
		}
		if target, err := url.Parse(proxyURL); !chk.E(err) {
			if target.Scheme == "" || target.Host == "" {
				// invalid URL, disable proxy
				log.Printf(
					"invalid ORLY_WEB_DEV_PROXY_URL: %q — disabling dev proxy\n",
					s.Config.WebDevProxyURL,
				)
			} else {
				s.devProxy = httputil.NewSingleHostReverseProxy(target)
				// Ensure Host header points to upstream for dev servers that care
				origDirector := s.devProxy.Director
				s.devProxy.Director = func(req *http.Request) {
					origDirector(req)
					req.Host = target.Host
				}
			}
		}
	}

	// Initialize challenge storage if not already done
	if s.challenges == nil {
		s.challengeMutex.Lock()
		s.challenges = make(map[string][]byte)
		s.challengeMutex.Unlock()
	}

	// Serve favicon.ico by serving orly-favicon.png
	s.mux.HandleFunc("/favicon.ico", s.handleFavicon)

	// Serve the main login interface (and static assets) or proxy in dev mode
	s.mux.HandleFunc("/", s.handleLoginInterface)

	// API endpoints for authentication
	s.mux.HandleFunc("/api/auth/challenge", s.handleAuthChallenge)
	s.mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("/api/permissions/", s.handlePermissions)
	// Export endpoint
	s.mux.HandleFunc("/api/export", s.handleExport)
	// Events endpoints
	s.mux.HandleFunc("/api/events/mine", s.handleEventsMine)
	// Import endpoint (admin only)
	s.mux.HandleFunc("/api/import", s.handleImport)
	// Sprocket endpoints (owner only)
	s.mux.HandleFunc("/api/sprocket/status", s.handleSprocketStatus)
	s.mux.HandleFunc("/api/sprocket/update", s.handleSprocketUpdate)
	s.mux.HandleFunc("/api/sprocket/restart", s.handleSprocketRestart)
	s.mux.HandleFunc("/api/sprocket/versions", s.handleSprocketVersions)
	s.mux.HandleFunc(
		"/api/sprocket/delete-version", s.handleSprocketDeleteVersion,
	)
	s.mux.HandleFunc("/api/sprocket/config", s.handleSprocketConfig)
	// NIP-86 management endpoint
	s.mux.HandleFunc("/api/nip86", s.handleNIP86Management)
	// ACL mode endpoint
	s.mux.HandleFunc("/api/acl-mode", s.handleACLMode)

	// Sync endpoints for distributed synchronization
	if s.syncManager != nil {
		s.mux.HandleFunc("/api/sync/current", s.handleSyncCurrent)
		s.mux.HandleFunc("/api/sync/event-ids", s.handleSyncEventIDs)
		log.Printf("Distributed sync API enabled at /api/sync")
	}

	// Blossom blob storage API endpoint
	if s.blossomServer != nil {
		s.mux.HandleFunc("/blossom/", s.blossomHandler)
		log.Printf("Blossom blob storage API enabled at /blossom")
	} else {
		log.Printf("WARNING: Blossom server is nil, routes not registered")
	}

	// Cluster replication API endpoints
	if s.clusterManager != nil {
		s.mux.HandleFunc("/cluster/latest", s.clusterManager.HandleLatestSerial)
		s.mux.HandleFunc("/cluster/events", s.clusterManager.HandleEventsRange)
		log.Printf("Cluster replication API enabled at /cluster")
	}
}

// handleFavicon serves orly-favicon.png as favicon.ico
func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	// In dev mode with proxy configured, forward to dev server
	if s.devProxy != nil {
		s.devProxy.ServeHTTP(w, r)
		return
	}

	// Serve orly-favicon.png as favicon.ico from embedded web app
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day

	// Create a request for orly-favicon.png and serve it
	faviconReq := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/orly-favicon.png"},
	}
	ServeEmbeddedWeb(w, faviconReq)
}

// handleLoginInterface serves the main user interface for login
func (s *Server) handleLoginInterface(w http.ResponseWriter, r *http.Request) {
	// In dev mode with proxy configured, forward to dev server
	if s.devProxy != nil {
		s.devProxy.ServeHTTP(w, r)
		return
	}

	// Serve embedded web interface
	ServeEmbeddedWeb(w, r)
}

// handleAuthChallenge generates a new authentication challenge
func (s *Server) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Generate a new challenge
	challenge := auth.GenerateChallenge()
	challengeHex := hex.Enc(challenge)

	// Store the challenge with expiration (5 minutes)
	s.challengeMutex.Lock()
	if s.challenges == nil {
		s.challenges = make(map[string][]byte)
	}
	s.challenges[challengeHex] = challenge
	s.challengeMutex.Unlock()

	// Clean up expired challenges
	go func() {
		time.Sleep(5 * time.Minute)
		s.challengeMutex.Lock()
		delete(s.challenges, challengeHex)
		s.challengeMutex.Unlock()
	}()

	// Return the challenge
	response := struct {
		Challenge string `json:"challenge"`
	}{
		Challenge: challengeHex,
	}

	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(
			w, "Error generating challenge", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleAuthLogin processes authentication requests
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if chk.E(err) {
		w.Write([]byte(`{"success": false, "error": "Failed to read request body"}`))
		return
	}

	// Parse the signed event
	var evt event.E
	if err = json.Unmarshal(body, &evt); chk.E(err) {
		w.Write([]byte(`{"success": false, "error": "Invalid event format"}`))
		return
	}

	// Extract the challenge from the event to look up the stored challenge
	challengeTag := evt.Tags.GetFirst([]byte("challenge"))
	if challengeTag == nil {
		w.Write([]byte(`{"success": false, "error": "Challenge tag missing from event"}`))
		return
	}

	challengeHex := string(challengeTag.Value())

	// Retrieve the stored challenge
	s.challengeMutex.RLock()
	_, exists := s.challenges[challengeHex]
	s.challengeMutex.RUnlock()

	if !exists {
		w.Write([]byte(`{"success": false, "error": "Invalid or expired challenge"}`))
		return
	}

	// Clean up the used challenge
	s.challengeMutex.Lock()
	delete(s.challenges, challengeHex)
	s.challengeMutex.Unlock()

	relayURL := s.WebSocketURL(r)

	// Validate the authentication event with the correct challenge
	// The challenge in the event tag is hex-encoded, so we need to pass the hex string as bytes
	ok, err := auth.Validate(&evt, []byte(challengeHex), relayURL)
	if chk.E(err) || !ok {
		errorMsg := "Authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		w.Write([]byte(`{"success": false, "error": "` + errorMsg + `"}`))
		return
	}

	// Authentication successful: set a simple session cookie with the pubkey
	cookie := &http.Cookie{
		Name:     "orly_auth",
		Value:    hex.Enc(evt.Pubkey),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30, // 30 days
	}
	http.SetCookie(w, cookie)

	w.Write([]byte(`{"success": true}`))
}

// handleAuthStatus checks if the user is authenticated
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check for auth cookie
	c, err := r.Cookie("orly_auth")
	if err != nil || c.Value == "" {
		w.Write([]byte(`{"authenticated": false}`))
		return
	}

	// Validate the pubkey format
	pubkey, err := hex.Dec(c.Value)
	if chk.E(err) {
		w.Write([]byte(`{"authenticated": false}`))
		return
	}

	// Get user permissions
	permission := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)

	response := struct {
		Authenticated bool   `json:"authenticated"`
		Pubkey        string `json:"pubkey"`
		Permission    string `json:"permission"`
	}{
		Authenticated: true,
		Pubkey:        c.Value,
		Permission:    permission,
	}

	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		w.Write([]byte(`{"authenticated": false}`))
		return
	}

	w.Write(jsonData)
}

// handleAuthLogout clears the authentication cookie
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Clear the auth cookie
	cookie := &http.Cookie{
		Name:     "orly_auth",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Expire immediately
	}
	http.SetCookie(w, cookie)

	w.Write([]byte(`{"success": true}`))
}

// handlePermissions returns the permission level for a given pubkey
func (s *Server) handlePermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pubkey from URL path
	pubkeyHex := strings.TrimPrefix(r.URL.Path, "/api/permissions/")
	if pubkeyHex == "" || pubkeyHex == "/" {
		http.Error(w, "Invalid pubkey", http.StatusBadRequest)
		return
	}

	// Convert hex to binary pubkey
	pubkey, err := hex.Dec(pubkeyHex)
	if chk.E(err) {
		http.Error(w, "Invalid pubkey format", http.StatusBadRequest)
		return
	}

	// Get access level using acl registry
	permission := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)

	// Set content type and write JSON response
	w.Header().Set("Content-Type", "application/json")

	// Format response as proper JSON
	response := struct {
		Permission string `json:"permission"`
	}{
		Permission: permission,
	}

	// Marshal and write the response
	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleExport streams events as JSONL (NDJSON) using NIP-98 authentication.
// Supports both GET (query params) and POST (JSON body) for pubkey filtering.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
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

	// Check permissions - require write, admin, or owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "write" && accessLevel != "admin" && accessLevel != "owner" {
		http.Error(
			w, "Write, admin, or owner permission required",
			http.StatusForbidden,
		)
		return
	}

	// Parse pubkeys from request
	var pks [][]byte

	if r.Method == http.MethodPost {
		// Parse JSON body for pubkeys
		var requestBody struct {
			Pubkeys []string `json:"pubkeys"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err == nil {
			// If JSON parsing succeeds, use pubkeys from body
			for _, pkHex := range requestBody.Pubkeys {
				if pkHex == "" {
					continue
				}
				if pk, err := hex.Dec(pkHex); !chk.E(err) {
					pks = append(pks, pk)
				}
			}
		}
		// If JSON parsing fails, fall back to empty pubkeys (export all)
	} else {
		// GET method - parse query parameters
		q := r.URL.Query()
		for _, pkHex := range q["pubkey"] {
			if pkHex == "" {
				continue
			}
			if pk, err := hex.Dec(pkHex); !chk.E(err) {
				pks = append(pks, pk)
			}
		}
	}

	// Determine filename based on whether filtering by pubkeys
	var filename string
	if len(pks) == 0 {
		filename = "all-events-" + time.Now().UTC().Format("20060102-150405Z") + ".jsonl"
	} else if len(pks) == 1 {
		filename = "my-events-" + time.Now().UTC().Format("20060102-150405Z") + ".jsonl"
	} else {
		filename = "filtered-events-" + time.Now().UTC().Format("20060102-150405Z") + ".jsonl"
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set(
		"Content-Disposition", "attachment; filename=\""+filename+"\"",
	)

	// Stream export
	s.DB.Export(s.Ctx, w, pks...)
}

// handleEventsMine returns the authenticated user's events in JSON format with pagination using NIP-98 authentication.
func (s *Server) handleEventsMine(w http.ResponseWriter, r *http.Request) {
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

	// Parse pagination parameters
	query := r.URL.Query()
	limit := 50 // default limit
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Use QueryEvents with filter for this user's events
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(pubkey),
	}

	log.Printf("DEBUG: Querying events for pubkey: %s", hex.Enc(pubkey))
	events, err := s.DB.QueryEvents(s.Ctx, f)
	if chk.E(err) {
		log.Printf("DEBUG: QueryEvents failed: %v", err)
		http.Error(w, "Failed to query events", http.StatusInternalServerError)
		return
	}
	log.Printf("DEBUG: QueryEvents returned %d events", len(events))

	// Apply pagination
	totalEvents := len(events)
	if offset >= totalEvents {
		events = event.S{} // Empty slice
	} else {
		end := offset + limit
		if end > totalEvents {
			end = totalEvents
		}
		events = events[offset:end]
	}

	// Set content type and write JSON response
	w.Header().Set("Content-Type", "application/json")

	// Format response as proper JSON
	response := struct {
		Events []*event.E `json:"events"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{
		Events: events,
		Total:  totalEvents,
		Limit:  limit,
		Offset: offset,
	}

	// Marshal and write the response
	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleImport receives a JSONL/NDJSON file or body and enqueues an async import using NIP-98 authentication. Admins only.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
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

	// Check permissions - require admin or owner level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "admin" && accessLevel != "owner" {
		http.Error(
			w, "Admin or owner permission required", http.StatusForbidden,
		)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); chk.E(err) { // 32MB memory, rest to temp files
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if chk.E(err) {
			http.Error(w, "Missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()
		s.DB.Import(file)
	} else {
		if r.Body == nil {
			http.Error(w, "Empty request body", http.StatusBadRequest)
			return
		}
		s.DB.Import(r.Body)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"success": true, "message": "Import started"}`))
}

// handleSprocketStatus returns the current status of the sprocket script
func (s *Server) handleSprocketStatus(w http.ResponseWriter, r *http.Request) {
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

	status := s.sprocketManager.GetSprocketStatus()

	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(status)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleSprocketUpdate updates the sprocket script and restarts it
func (s *Server) handleSprocketUpdate(w http.ResponseWriter, r *http.Request) {
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

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if chk.E(err) {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Update the sprocket script
	if err := s.sprocketManager.UpdateSprocket(string(body)); chk.E(err) {
		http.Error(
			w, fmt.Sprintf("Failed to update sprocket: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true, "message": "Sprocket updated successfully"}`))
}

// handleSprocketRestart restarts the sprocket script
func (s *Server) handleSprocketRestart(w http.ResponseWriter, r *http.Request) {
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

	// Restart the sprocket script
	if err := s.sprocketManager.RestartSprocket(); chk.E(err) {
		http.Error(
			w, fmt.Sprintf("Failed to restart sprocket: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true, "message": "Sprocket restarted successfully"}`))
}

// handleSprocketVersions returns all sprocket script versions
func (s *Server) handleSprocketVersions(
	w http.ResponseWriter, r *http.Request,
) {
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

	versions, err := s.sprocketManager.GetSprocketVersions()
	if chk.E(err) {
		http.Error(
			w, fmt.Sprintf("Failed to get sprocket versions: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(versions)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleSprocketDeleteVersion deletes a specific sprocket version
func (s *Server) handleSprocketDeleteVersion(
	w http.ResponseWriter, r *http.Request,
) {
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

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if chk.E(err) {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var request struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(body, &request); chk.E(err) {
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	if request.Filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	// Delete the sprocket version
	if err := s.sprocketManager.DeleteSprocketVersion(request.Filename); chk.E(err) {
		http.Error(
			w, fmt.Sprintf("Failed to delete sprocket version: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true, "message": "Sprocket version deleted successfully"}`))
}

// handleSprocketConfig returns the sprocket configuration status
func (s *Server) handleSprocketConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := struct {
		Enabled bool `json:"enabled"`
	}{
		Enabled: s.Config.SprocketEnabled,
	}

	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleACLMode returns the current ACL mode
func (s *Server) handleACLMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := struct {
		ACLMode string `json:"acl_mode"`
	}{
		ACLMode: acl.Registry.Type(),
	}

	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(
			w, "Error generating response", http.StatusInternalServerError,
		)
		return
	}

	w.Write(jsonData)
}

// handleSyncCurrent handles requests for the current serial number
func (s *Server) handleSyncCurrent(w http.ResponseWriter, r *http.Request) {
	if s.syncManager == nil {
		http.Error(
			w, "Sync manager not initialized", http.StatusServiceUnavailable,
		)
		return
	}

	// Validate NIP-98 authentication and check peer authorization
	if !s.validatePeerRequest(w, r) {
		return
	}

	s.syncManager.HandleCurrentRequest(w, r)
}

// handleSyncEventIDs handles requests for event IDs with their serial numbers
func (s *Server) handleSyncEventIDs(w http.ResponseWriter, r *http.Request) {
	if s.syncManager == nil {
		http.Error(
			w, "Sync manager not initialized", http.StatusServiceUnavailable,
		)
		return
	}

	// Validate NIP-98 authentication and check peer authorization
	if !s.validatePeerRequest(w, r) {
		return
	}

	s.syncManager.HandleEventIDsRequest(w, r)
}

// validatePeerRequest validates NIP-98 authentication and checks if the requesting peer is authorized
func (s *Server) validatePeerRequest(
	w http.ResponseWriter, r *http.Request,
) bool {
	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if err != nil {
		log.Printf("NIP-98 auth validation error: %v", err)
		http.Error(
			w, "Authentication validation failed", http.StatusUnauthorized,
		)
		return false
	}
	if !valid {
		http.Error(w, "NIP-98 authentication required", http.StatusUnauthorized)
		return false
	}

	if s.syncManager == nil {
		log.Printf("Sync manager not available for peer validation")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return false
	}

	// Extract the relay URL from the request (this should be in the request body)
	// For now, we'll check against all configured peers
	peerPubkeyHex := hex.Enc(pubkey)

	// Check if this pubkey matches any of our configured peer relays' NIP-11 pubkeys
	for _, peerURL := range s.syncManager.GetPeers() {
		if s.syncManager.IsAuthorizedPeer(peerURL, peerPubkeyHex) {
			// Also update ACL to grant admin access to this peer pubkey
			s.updatePeerAdminACL(pubkey)
			return true
		}
	}

	log.Printf("Unauthorized sync request from pubkey: %s", peerPubkeyHex)
	http.Error(w, "Unauthorized peer", http.StatusForbidden)
	return false
}

// updatePeerAdminACL grants admin access to peer relay identity pubkeys
func (s *Server) updatePeerAdminACL(peerPubkey []byte) {
	// Find the managed ACL instance and update peer admins
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == "managed" {
			if managed, ok := aclInstance.(*acl.Managed); ok {
				// Collect all current peer pubkeys
				var peerPubkeys [][]byte
				for _, peerURL := range s.syncManager.GetPeers() {
					if pubkey, err := s.syncManager.GetPeerPubkey(peerURL); err == nil {
						peerPubkeys = append(peerPubkeys, []byte(pubkey))
					}
				}
				managed.UpdatePeerAdmins(peerPubkeys)
				break
			}
		}
	}
}

// =============================================================================
// Message Processing Pause/Resume for Policy and Follow List Updates
// =============================================================================

// PauseMessageProcessing acquires an exclusive lock to pause all message processing.
// This should be called before updating policy configuration or follow lists.
// Call ResumeMessageProcessing to release the lock after updates are complete.
func (s *Server) PauseMessageProcessing() {
	s.messagePauseMutex.Lock()
}

// ResumeMessageProcessing releases the exclusive lock to resume message processing.
// This should be called after policy configuration or follow list updates are complete.
func (s *Server) ResumeMessageProcessing() {
	s.messagePauseMutex.Unlock()
}

// AcquireMessageProcessingLock acquires a read lock for normal message processing.
// This allows concurrent message processing while blocking during policy updates.
// Call ReleaseMessageProcessingLock when message processing is complete.
func (s *Server) AcquireMessageProcessingLock() {
	s.messagePauseMutex.RLock()
}

// ReleaseMessageProcessingLock releases the read lock after message processing.
func (s *Server) ReleaseMessageProcessingLock() {
	s.messagePauseMutex.RUnlock()
}
