package smtp

import (
	"context"
	"fmt"
	"net"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"next.orly.dev/pkg/lol/log"
)

// InboundEmail represents a parsed inbound email ready for bridge processing.
type InboundEmail struct {
	From        string
	To          []string
	RawMessage  []byte
	ReceivedAt  time.Time
}

// InboundHandler is called when a valid inbound email is received.
type InboundHandler func(email *InboundEmail) error

// ServerConfig holds SMTP server configuration.
type ServerConfig struct {
	// Domain is the bridge's email domain (e.g., "relay.example.com").
	Domain string
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:2525").
	ListenAddr string
	// MaxMessageBytes is the maximum message size (default: 25MB).
	MaxMessageBytes int64
	// MaxRecipients is the maximum recipients per message (default: 10).
	MaxRecipients int
	// ReadTimeout is the per-command read timeout (default: 60s).
	ReadTimeout time.Duration
	// WriteTimeout is the per-command write timeout (default: 60s).
	WriteTimeout time.Duration
}

// DefaultServerConfig returns sensible defaults for the SMTP server.
func DefaultServerConfig(domain string) ServerConfig {
	return ServerConfig{
		Domain:          domain,
		ListenAddr:      "0.0.0.0:2525",
		MaxMessageBytes: 25 * 1024 * 1024, // 25MB
		MaxRecipients:   10,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
	}
}

// Server wraps go-smtp to receive inbound emails for the bridge.
type Server struct {
	cfg     ServerConfig
	handler InboundHandler
	server  *gosmtp.Server
	ln      net.Listener
}

// NewServer creates an SMTP server with the given configuration.
// The handler is called for each valid inbound email.
func NewServer(cfg ServerConfig, handler InboundHandler) *Server {
	backend := &backend{
		domain:  cfg.Domain,
		handler: handler,
	}

	s := gosmtp.NewServer(backend)
	s.Addr = cfg.ListenAddr
	s.Domain = cfg.Domain
	s.AllowInsecureAuth = true // We're behind a reverse proxy
	s.MaxRecipients = cfg.MaxRecipients

	if cfg.MaxMessageBytes > 0 {
		s.MaxMessageBytes = cfg.MaxMessageBytes
	}
	if cfg.ReadTimeout > 0 {
		s.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		s.WriteTimeout = cfg.WriteTimeout
	}

	return &Server{
		cfg:     cfg,
		handler: handler,
		server:  s,
	}
}

// Start begins listening for SMTP connections.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.ListenAddr, err)
	}
	s.ln = ln

	log.I.F("SMTP server listening on %s for domain %s", s.cfg.ListenAddr, s.cfg.Domain)

	go func() {
		if err := s.server.Serve(ln); err != nil {
			log.E.F("SMTP server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the SMTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	log.I.F("SMTP server shutting down")
	return s.server.Shutdown(ctx)
}

// Addr returns the listener's address, useful for tests.
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}
