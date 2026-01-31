// Package tls provides a TLS/ACME transport for the relay.
package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/acme/autocert"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Config holds TLS transport configuration.
type Config struct {
	// Domains is the list of domains for ACME auto-cert.
	Domains []string
	// Certs is a list of manual certificate paths (without extension).
	// For each path, .pem and .key files are loaded.
	Certs []string
	// DataDir is the base data directory for the autocert cache.
	DataDir string
	// Handler is the HTTP handler to serve.
	Handler http.Handler
}

// Transport serves HTTPS with automatic or manual TLS certificates.
// It runs two servers: HTTPS on :443 and HTTP on :80 for ACME challenges.
type Transport struct {
	cfg        *Config
	tlsServer  *http.Server
	httpServer *http.Server
	mu         sync.Mutex
}

// New creates a new TLS transport.
func New(cfg *Config) *Transport {
	return &Transport{cfg: cfg}
}

func (t *Transport) Name() string { return "tls" }

func (t *Transport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := ValidateConfig(t.cfg.Domains, t.cfg.Certs); err != nil {
		return fmt.Errorf("invalid TLS configuration: %w", err)
	}

	// Create cache directory for autocert
	cacheDir := filepath.Join(t.cfg.DataDir, "autocert")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create autocert cache directory: %w", err)
	}

	// Set up autocert manager
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(t.cfg.Domains...),
	}

	// Create TLS server on port 443
	t.tlsServer = &http.Server{
		Addr:      ":443",
		Handler:   t.cfg.Handler,
		TLSConfig: tlsConfig(m, t.cfg.Certs...),
	}

	// Create HTTP server for ACME challenges and redirects on port 80
	t.httpServer = &http.Server{
		Addr:    ":80",
		Handler: m.HTTPHandler(nil),
	}

	log.I.F("TLS enabled for domains: %v", t.cfg.Domains)

	// Start TLS server
	go func() {
		log.I.F("starting TLS listener on https://:443")
		if err := t.tlsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.E.F("TLS server error: %v", err)
		}
	}()

	// Start HTTP server for ACME challenges
	go func() {
		log.I.F("starting HTTP listener on http://:80 for ACME challenges")
		if err := t.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.E.F("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (t *Transport) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var firstErr error

	if t.tlsServer != nil {
		if err := t.tlsServer.Shutdown(ctx); err != nil {
			log.E.F("TLS server shutdown error: %v", err)
			firstErr = err
		} else {
			log.I.F("TLS server shutdown completed")
		}
	}

	if t.httpServer != nil {
		if err := t.httpServer.Shutdown(ctx); err != nil {
			log.E.F("HTTP server shutdown error: %v", err)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			log.I.F("HTTP server shutdown completed")
		}
	}

	return firstErr
}

func (t *Transport) Addresses() []string {
	var addrs []string
	for _, domain := range t.cfg.Domains {
		addrs = append(addrs, "wss://"+domain+"/")
	}
	return addrs
}

// ValidateConfig checks if the TLS configuration is valid.
func ValidateConfig(domains []string, certs []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no TLS domains specified")
	}

	for _, domain := range domains {
		if domain == "" {
			continue
		}
		if strings.Contains(domain, " ") || strings.Contains(domain, "\t") {
			return fmt.Errorf("invalid domain name: %s", domain)
		}
	}

	return nil
}

// tlsConfig returns a TLS configuration that works with LetsEncrypt automatic
// SSL cert issuer as well as any provided certificate files.
//
// Certs are provided as paths where .pem and .key files exist.
func tlsConfig(m *autocert.Manager, certs ...string) *tls.Config {
	certMap := make(map[string]*tls.Certificate)
	var mx sync.Mutex

	for _, certPath := range certs {
		if certPath == "" {
			continue
		}

		var err error
		var c tls.Certificate

		if c, err = tls.LoadX509KeyPair(
			certPath+".pem", certPath+".key",
		); chk.E(err) {
			log.E.F("failed to load certificate from %s: %v", certPath, err)
			continue
		}

		if len(c.Certificate) > 0 {
			if x509Cert, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
				if x509Cert.Subject.CommonName != "" {
					certMap[x509Cert.Subject.CommonName] = &c
					log.I.F("loaded certificate for domain: %s", x509Cert.Subject.CommonName)
				}
				for _, san := range x509Cert.DNSNames {
					if san != "" {
						certMap[san] = &c
						log.I.F("loaded certificate for SAN domain: %s", san)
					}
				}
			}
		}
	}

	if m == nil {
		return &tls.Config{
			GetCertificate: func(helo *tls.ClientHelloInfo) (*tls.Certificate, error) {
				mx.Lock()
				defer mx.Unlock()

				if cert, exists := certMap[helo.ServerName]; exists {
					return cert, nil
				}

				for domain, cert := range certMap {
					if strings.HasPrefix(domain, "*.") {
						baseDomain := domain[2:]
						if strings.HasSuffix(helo.ServerName, baseDomain) {
							return cert, nil
						}
					}
				}

				return nil, fmt.Errorf("no certificate found for %s", helo.ServerName)
			},
		}
	}

	tc := m.TLSConfig()
	tc.GetCertificate = func(helo *tls.ClientHelloInfo) (*tls.Certificate, error) {
		mx.Lock()

		if cert, exists := certMap[helo.ServerName]; exists {
			mx.Unlock()
			return cert, nil
		}

		for domain, cert := range certMap {
			if strings.HasPrefix(domain, "*.") {
				baseDomain := domain[2:]
				if strings.HasSuffix(helo.ServerName, baseDomain) {
					mx.Unlock()
					return cert, nil
				}
			}
		}

		mx.Unlock()

		return m.GetCertificate(helo)
	}

	return tc
}
