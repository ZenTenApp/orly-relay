package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"
)

//go:embed smesh/dist
var smeshFS embed.FS

// SmeshServer serves the embedded Smesh web client on a dedicated port.
type SmeshServer struct {
	server   *http.Server
	listener net.Listener
	port     int
}

// NewSmeshServer creates a new smesh HTTP server on the given port.
func NewSmeshServer(port int) *SmeshServer {
	return &SmeshServer{port: port}
}

// Start begins serving the embedded smesh SPA.
func (s *SmeshServer) Start(ctx context.Context) error {
	// Extract the dist/ subtree from the embedded filesystem
	webDist, err := fs.Sub(smeshFS, "smesh/dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded smesh app: %w", err)
	}

	fileServer := http.FileServer(http.FS(webDist))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// For SPA routing: serve index.html for paths that don't match a real file.
		// Real assets (JS, CSS, images) are served directly.
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the path maps to a real file in the embedded FS
		cleanPath := strings.TrimPrefix(path, "/")
		if f, err := webDist.Open(cleanPath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("smesh: failed to listen on %s: %w", addr, err)
	}

	// Graceful shutdown on context cancellation
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.W.F("smesh server shutdown error: %v", err)
		}
	}()

	log.I.F("smesh web client serving on http://%s", addr)

	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("smesh server error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the smesh server.
func (s *SmeshServer) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}
