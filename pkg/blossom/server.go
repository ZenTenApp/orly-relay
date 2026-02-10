package blossom

import (
	"net/http"
	"strings"

	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
)

// Server provides a Blossom server implementation
type Server struct {
	db      database.Database
	storage *Storage
	acl     *acl.S
	baseURL string

	// Configuration
	maxBlobSize            int64
	allowedMimeTypes       map[string]bool
	requireAuth            bool
	deleteRequireServerTag bool

	// Rate limiting for uploads
	bandwidthLimiter *BandwidthLimiter
}

// Config holds configuration for the Blossom server
type Config struct {
	BaseURL          string
	MaxBlobSize      int64
	AllowedMimeTypes []string
	RequireAuth      bool

	// Rate limiting (for non-followed users)
	RateLimitEnabled bool
	DailyLimitMB     int64
	BurstLimitMB     int64

	// Delete replay protection (proposed BUD enhancement)
	// When true, DELETE auth events must include a 'server' tag matching this server
	DeleteRequireServerTag bool
}

// NewServer creates a new Blossom server instance
func NewServer(db database.Database, aclRegistry *acl.S, cfg *Config) *Server {
	if cfg == nil {
		cfg = &Config{
			MaxBlobSize: 100 * 1024 * 1024, // 100MB default
			RequireAuth: false,
		}
	}

	storage := NewStorage(db)

	// Build allowed MIME types map
	allowedMap := make(map[string]bool)
	if len(cfg.AllowedMimeTypes) > 0 {
		for _, mime := range cfg.AllowedMimeTypes {
			allowedMap[mime] = true
		}
	}

	// Initialize bandwidth limiter if enabled
	var bwLimiter *BandwidthLimiter
	if cfg.RateLimitEnabled {
		dailyMB := cfg.DailyLimitMB
		if dailyMB <= 0 {
			dailyMB = 10 // 10MB default
		}
		burstMB := cfg.BurstLimitMB
		if burstMB <= 0 {
			burstMB = 50 // 50MB default burst
		}
		bwLimiter = NewBandwidthLimiter(dailyMB, burstMB)
	}

	return &Server{
		db:                     db,
		storage:                storage,
		acl:                    aclRegistry,
		baseURL:                cfg.BaseURL,
		maxBlobSize:            cfg.MaxBlobSize,
		allowedMimeTypes:       allowedMap,
		requireAuth:            cfg.RequireAuth,
		deleteRequireServerTag: cfg.DeleteRequireServerTag,
		bandwidthLimiter:       bwLimiter,
	}
}

// Handler returns an http.Handler that can be attached to a router
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers (BUD-01 requirement)
		s.setCORSHeaders(w, r)

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Route based on path and method
		path := r.URL.Path

		// Remove leading slash
		path = strings.TrimPrefix(path, "/")

		// Handle specific endpoints
		switch {
		case r.Method == http.MethodGet && path == "upload":
			// This shouldn't happen, but handle gracefully
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case r.Method == http.MethodHead && path == "upload":
			s.handleUploadRequirements(w, r)
			return

		case r.Method == http.MethodPut && path == "upload":
			s.handleUpload(w, r)
			return

		case r.Method == http.MethodHead && path == "media":
			s.handleMediaHead(w, r)
			return

		case r.Method == http.MethodPut && path == "media":
			s.handleMediaUpload(w, r)
			return

		case r.Method == http.MethodPut && path == "mirror":
			s.handleMirror(w, r)
			return

		case r.Method == http.MethodPut && path == "report":
			s.handleReport(w, r)
			return

		case path == "admin/users":
			if r.Method == http.MethodGet {
				s.handleAdminListUsers(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case path == "admin/generate-thumbnails":
			if r.Method == http.MethodPost {
				s.handleGenerateThumbnails(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case strings.HasPrefix(path, "list/"):
			if r.Method == http.MethodGet {
				s.handleListBlobs(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case strings.HasPrefix(path, "delete-variants/"):
			if r.Method == http.MethodDelete {
				s.handleDeleteVariants(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case r.Method == http.MethodGet:
			// Handle GET /<sha256>
			s.handleGetBlob(w, r)
			return

		case r.Method == http.MethodHead:
			// Handle HEAD /<sha256>
			s.handleHeadBlob(w, r)
			return

		case r.Method == http.MethodDelete:
			// Handle DELETE /<sha256>
			s.handleDeleteBlob(w, r)
			return

		default:
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	})
}

// setCORSHeaders sets CORS headers as required by BUD-01
func (s *Server) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE, OPTIONS")
	// Include all headers used by Blossom clients (BUD-01, BUD-06)
	// Include both cases for maximum compatibility with various clients
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, authorization, Content-Type, content-type, X-SHA-256, x-sha-256, X-Content-Length, x-content-length, X-Content-Type, x-content-type, Accept, accept")
	w.Header().Set("Access-Control-Expose-Headers", "X-Reason, Content-Length, Content-Type, Accept-Ranges")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
}

// setErrorResponse sets an error response with X-Reason header (BUD-01)
func (s *Server) setErrorResponse(w http.ResponseWriter, status int, reason string) {
	w.Header().Set("X-Reason", reason)
	http.Error(w, reason, status)
}

// getRemoteAddr extracts the remote address from the request
func (s *Server) getRemoteAddr(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// checkACL checks if the user has the required access level
func (s *Server) checkACL(
	pubkey []byte, remoteAddr string, requiredLevel string,
) bool {
	if s.acl == nil {
		return true // No ACL configured, allow all
	}

	level := s.acl.GetAccessLevel(pubkey, remoteAddr)

	// Map ACL levels to permissions
	levelMap := map[string]int{
		"none":  0,
		"read":  1,
		"write": 2,
		"admin": 3,
		"owner": 4,
	}

	required := levelMap[requiredLevel]
	actual := levelMap[level]

	return actual >= required
}

// isRateLimitExempt returns true if the user is exempt from rate limiting.
// Users with write access or higher (followed users, admins, owners) are exempt.
func (s *Server) isRateLimitExempt(pubkey []byte, remoteAddr string) bool {
	if s.acl == nil {
		return true // No ACL configured, no rate limiting
	}

	level := s.acl.GetAccessLevel(pubkey, remoteAddr)

	// Followed users get "write" level, admins/owners get higher
	// Only "read" and "none" are rate limited
	return level == "write" || level == "admin" || level == "owner"
}

// checkBandwidthLimit checks if the upload is allowed under rate limits.
// Returns true if allowed, false if rate limited.
// Exempt users (followed, admin, owner) always return true.
func (s *Server) checkBandwidthLimit(pubkey []byte, remoteAddr string, sizeBytes int64) bool {
	if s.bandwidthLimiter == nil {
		return true // No rate limiting configured
	}

	// Check if user is exempt
	if s.isRateLimitExempt(pubkey, remoteAddr) {
		return true
	}

	// Use pubkey hex if available, otherwise IP
	var identity string
	if len(pubkey) > 0 {
		identity = string(pubkey) // Will be converted to hex in handler
	} else {
		identity = remoteAddr
	}

	return s.bandwidthLimiter.CheckAndConsume(identity, sizeBytes)
}

// BaseURLKey is the context key for the base URL (exported for use by app handler)
type BaseURLKey struct{}

// getBaseURL returns the base URL, preferring request context if available
func (s *Server) getBaseURL(r *http.Request) string {
	if baseURL := r.Context().Value(BaseURLKey{}); baseURL != nil {
		if url, ok := baseURL.(string); ok && url != "" {
			return url
		}
	}
	return s.baseURL
}
