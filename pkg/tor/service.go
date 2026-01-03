// Package tor provides Tor hidden service integration for the ORLY relay.
// It manages a listener on a dedicated port that receives traffic forwarded
// from the Tor daemon, and exposes the .onion address for NIP-11 integration.
package tor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Config holds Tor hidden service configuration.
type Config struct {
	// Port is the internal port that Tor forwards .onion traffic to
	Port int
	// HSDir is the Tor HiddenServiceDir path to read .onion hostname from
	HSDir string
	// OnionAddress is an optional manual override for the .onion address
	OnionAddress string
	// Handler is the HTTP handler to serve (typically the main relay handler)
	Handler http.Handler
}

// Service manages the Tor hidden service listener.
type Service struct {
	cfg      *Config
	listener net.Listener
	server   *http.Server

	// onionAddress is the detected or configured .onion address
	onionAddress string

	// hostname watcher
	hostnameWatcher *HostnameWatcher

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// New creates a new Tor service with the given configuration.
func New(cfg *Config) (*Service, error) {
	if cfg.Port == 0 {
		cfg.Port = 3336
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// If manual address provided, use it
	if cfg.OnionAddress != "" {
		s.onionAddress = cfg.OnionAddress
		log.I.F("using configured .onion address: %s", s.onionAddress)
	}

	return s, nil
}

// Start initializes the Tor listener and hostname watcher.
func (s *Service) Start() error {
	// Start hostname watcher if HSDir is configured
	if s.cfg.HSDir != "" {
		s.hostnameWatcher = NewHostnameWatcher(s.cfg.HSDir)
		s.hostnameWatcher.OnChange(func(addr string) {
			s.mu.Lock()
			s.onionAddress = addr
			s.mu.Unlock()
			log.I.F("detected .onion address: %s", addr)
		})
		if err := s.hostnameWatcher.Start(); err != nil {
			log.W.F("failed to start hostname watcher: %v", err)
		} else {
			// Get initial address
			if addr := s.hostnameWatcher.Address(); addr != "" {
				s.mu.Lock()
				s.onionAddress = addr
				s.mu.Unlock()
			}
		}
	}

	// Create listener
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if chk.E(err) {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Create HTTP server with the provided handler
	s.server = &http.Server{
		Handler:      s.cfg.Handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start serving
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.I.F("Tor listener started on %s", addr)
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("Tor server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the Tor service.
func (s *Service) Stop() error {
	s.cancel()

	// Stop hostname watcher
	if s.hostnameWatcher != nil {
		s.hostnameWatcher.Stop()
	}

	// Shutdown HTTP server
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); chk.E(err) {
			return err
		}
	}

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	log.I.F("Tor service stopped")
	return nil
}

// OnionAddress returns the current .onion address (without .onion suffix).
func (s *Service) OnionAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.onionAddress
}

// OnionWSAddress returns the full WebSocket URL for the hidden service.
// Format: ws://<address>.onion/
func (s *Service) OnionWSAddress() string {
	addr := s.OnionAddress()
	if addr == "" {
		return ""
	}
	// Ensure address ends with .onion
	if len(addr) >= 6 && addr[len(addr)-6:] != ".onion" {
		addr = addr + ".onion"
	}
	return "ws://" + addr + "/"
}

// IsRunning returns whether the Tor service is currently running.
func (s *Service) IsRunning() bool {
	return s.listener != nil
}

// Upgrader returns a WebSocket upgrader configured for Tor connections.
// Tor connections don't send Origin headers, so we skip origin check.
func (s *Service) Upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for Tor
		},
	}
}
