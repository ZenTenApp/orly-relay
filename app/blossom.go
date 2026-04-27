package app

import (
	"context"
	"net/http"
	"strings"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/database"
	blossom "git.smesh.lol/orly/pkg/blossom"
)

// initializeBlossomServer creates and configures the Blossom blob storage server
func initializeBlossomServer(
	ctx context.Context, cfg *config.C, db database.Database,
) (*blossom.Server, error) {
	// Create blossom server configuration
	blossomCfg := &blossom.Config{
		BaseURL:          "", // Will be set dynamically per request
		MaxBlobSize:      100 * 1024 * 1024, // 100MB default
		AllowedMimeTypes: nil,               // Allow all MIME types by default
		RequireAuth:      cfg.AuthRequired || cfg.AuthToWrite,
		// Rate limiting for non-followed users
		RateLimitEnabled: cfg.BlossomRateLimitEnabled,
		DailyLimitMB:     cfg.BlossomDailyLimitMB,
		BurstLimitMB:     cfg.BlossomBurstLimitMB,
		// Delete replay protection (proposed BUD enhancement)
		DeleteRequireServerTag: cfg.BlossomDeleteRequireServerTag,
	}

	// Create blossom server with relay's ACL registry
	bs := blossom.NewServer(db, acl.Registry, blossomCfg)

	// Override baseURL getter to use request-based URL
	// We'll need to modify the handler to inject the baseURL per request
	// For now, we'll use a middleware approach

	if cfg.BlossomRateLimitEnabled {
		log.I.F("blossom server initialized with ACL mode: %s, rate limit: %dMB/day (burst: %dMB)",
			cfg.ACLMode, cfg.BlossomDailyLimitMB, cfg.BlossomBurstLimitMB)
	} else {
		log.I.F("blossom server initialized with ACL mode: %s", cfg.ACLMode)
	}
	return bs, nil
}

// blossomHandler wraps the blossom server handler to inject baseURL per request
func (s *Server) blossomHandler(w http.ResponseWriter, r *http.Request) {
	// Strip /blossom prefix and pass to blossom handler
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/blossom")
	if !strings.HasPrefix(r.URL.Path, "/") {
		r.URL.Path = "/" + r.URL.Path
	}

	// Set baseURL in request context for blossom server to use
	// Use the exported key type from the blossom package
	baseURL := s.ServiceURL(r) + "/blossom"
	r = r.WithContext(context.WithValue(r.Context(), blossom.BaseURLKey{}, baseURL))

	s.blossomServer.Handler().ServeHTTP(w, r)
}

// blossomRootHandler handles blossom requests at root level (for clients like Jumble)
// Note: Even though requests come to root-level paths like /upload, we return URLs
// with /blossom prefix because that's where the blob download handlers are registered.
func (s *Server) blossomRootHandler(w http.ResponseWriter, r *http.Request) {
	// Set baseURL with /blossom prefix so returned blob URLs point to working handlers
	baseURL := s.ServiceURL(r) + "/blossom"
	r = r.WithContext(context.WithValue(r.Context(), blossom.BaseURLKey{}, baseURL))

	s.blossomServer.Handler().ServeHTTP(w, r)
}

