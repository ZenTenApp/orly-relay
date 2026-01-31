// Package tcp provides a plain HTTP transport for the relay.
package tcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"lol.mleku.dev/log"
)

// Config holds TCP transport configuration.
type Config struct {
	// Addr is the listen address (e.g., "0.0.0.0:3334").
	Addr string
	// Handler is the HTTP handler to serve.
	Handler http.Handler
}

// Transport serves HTTP over plain TCP.
type Transport struct {
	cfg    *Config
	server *http.Server
	mu     sync.Mutex
}

// New creates a new TCP transport.
func New(cfg *Config) *Transport {
	return &Transport{cfg: cfg}
}

func (t *Transport) Name() string { return "tcp" }

func (t *Transport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.server = &http.Server{
		Addr:    t.cfg.Addr,
		Handler: t.cfg.Handler,
	}

	log.I.F("starting listener on http://%s", t.cfg.Addr)

	errCh := make(chan error, 1)
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Give the server a moment to fail on bind errors
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("tcp listen on %s: %w", t.cfg.Addr, err)
		}
	default:
	}

	return nil
}

func (t *Transport) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.server == nil {
		return nil
	}
	return t.server.Shutdown(ctx)
}

func (t *Transport) Addresses() []string {
	return []string{"ws://" + t.cfg.Addr + "/"}
}
