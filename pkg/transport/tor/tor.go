// Package tor provides a Tor hidden service transport for the relay.
// It wraps the existing pkg/tor service as a pluggable transport.
package tor

import (
	"context"
	"net/http"

	"git.smesh.lol/orly/pkg/lol/log"

	torservice "git.smesh.lol/orly/pkg/tor"
)

// Config holds Tor transport configuration.
type Config struct {
	// Port is the internal port for the hidden service.
	Port int
	// DataDir is the directory for Tor data (torrc, keys, hostname, etc.).
	DataDir string
	// Binary is the path to the tor executable.
	Binary string
	// SOCKSPort is the port for outbound SOCKS connections (0 = disabled).
	SOCKSPort int
	// Handler is the HTTP handler to serve.
	Handler http.Handler
}

// Transport serves the relay as a Tor hidden service.
type Transport struct {
	cfg     *Config
	service *torservice.Service
}

// New creates a new Tor transport.
func New(cfg *Config) *Transport {
	return &Transport{cfg: cfg}
}

func (t *Transport) Name() string { return "tor" }

func (t *Transport) Start(ctx context.Context) error {
	svcCfg := &torservice.Config{
		Port:      t.cfg.Port,
		DataDir:   t.cfg.DataDir,
		Binary:    t.cfg.Binary,
		SOCKSPort: t.cfg.SOCKSPort,
		Handler:   t.cfg.Handler,
	}

	var err error
	t.service, err = torservice.New(svcCfg)
	if err != nil {
		return err
	}

	if err = t.service.Start(); err != nil {
		t.service = nil
		return err
	}

	if addr := t.service.OnionWSAddress(); addr != "" {
		log.I.F("Tor hidden service listening on port %d, address: %s", t.cfg.Port, addr)
	} else {
		log.I.F("Tor hidden service listening on port %d (waiting for .onion address)", t.cfg.Port)
	}

	return nil
}

func (t *Transport) Stop(ctx context.Context) error {
	if t.service == nil {
		return nil
	}
	return t.service.Stop()
}

func (t *Transport) Addresses() []string {
	if t.service == nil {
		return nil
	}
	if addr := t.service.OnionWSAddress(); addr != "" {
		return []string{addr}
	}
	return nil
}

// Service returns the underlying Tor service for access to Tor-specific
// functionality (e.g., OnionAddress, DataDir).
func (t *Transport) Service() *torservice.Service {
	return t.service
}
