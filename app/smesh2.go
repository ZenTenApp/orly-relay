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

//go:embed smesh2
var smesh2FS embed.FS

// Smesh2Server serves the embedded smesh2 web client on a dedicated port.
type Smesh2Server struct {
	server   *http.Server
	listener net.Listener
	port     int
}

// NewSmesh2Server creates a new smesh2 HTTP server on the given port.
func NewSmesh2Server(port int) *Smesh2Server {
	return &Smesh2Server{port: port}
}

// Start begins serving the embedded smesh2 client.
func (s *Smesh2Server) Start(ctx context.Context) error {
	webDist, err := fs.Sub(smesh2FS, "smesh2")
	if err != nil {
		return fmt.Errorf("failed to load embedded smesh2 app: %w", err)
	}

	fileServer := http.FileServer(http.FS(webDist))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if f, err := webDist.Open(cleanPath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback
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
		return fmt.Errorf("smesh2: failed to listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.W.F("smesh2 server shutdown error: %v", err)
		}
	}()

	log.I.F("smesh2 web client serving on http://%s", addr)

	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("smesh2 server error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the smesh2 server.
func (s *Smesh2Server) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}
