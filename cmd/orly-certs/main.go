// orly-certs is a certificate management service that obtains and renews
// wildcard SSL certificates from Let's Encrypt using DNS-01 challenges.
//
// It supports multiple DNS providers via the lego library and stores
// certificates at a conventional file path for web apps to consume.
//
// Configuration is via environment variables:
//   - ORLY_CERTS_DOMAIN: Wildcard domain (e.g., "*.myapp.com")
//   - ORLY_CERTS_EMAIL: Email for Let's Encrypt account
//   - ORLY_CERTS_DNS_PROVIDER: DNS provider name (cloudflare, route53, etc.)
//   - ORLY_CERTS_OUTPUT_DIR: Certificate output directory (default: /var/cache/orly-certs)
//
// Provider-specific credentials are set via standard lego environment variables.
// See https://go-acme.github.io/lego/dns/ for documentation.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

func main() {
	cfg := loadConfig()
	lol.SetLogLevel(cfg.LogLevel)

	log.I.F("orly-certs starting")
	log.I.F("  domain: %s", cfg.Domain)
	log.I.F("  email: %s", cfg.Email)
	log.I.F("  dns provider: %s", cfg.DNSProvider)
	log.I.F("  output dir: %s", cfg.OutputDir)
	log.I.F("  acme server: %s", cfg.ACMEServerURL())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.I.F("shutdown signal received")
		cancel()
	}()

	// Create certificate manager
	manager, err := NewCertManager(cfg)
	if chk.E(err) {
		log.F.F("failed to create certificate manager: %v", err)
	}

	// Initial certificate check/obtain
	if err := manager.EnsureCertificate(); chk.E(err) {
		log.F.F("failed to ensure certificate: %v", err)
	}

	// Start renewal loop
	log.I.F("starting renewal check loop (interval: %s)", cfg.CheckInterval)
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := manager.CheckRenewal(); chk.E(err) {
				log.E.F("renewal check failed: %v", err)
			}
		case <-ctx.Done():
			log.I.F("orly-certs shutting down")
			return
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `orly-certs - DNS-01 wildcard certificate manager

Usage: orly-certs [options]

Environment Variables:
  ORLY_CERTS_DOMAIN         Wildcard domain (e.g., *.myapp.com) [required]
  ORLY_CERTS_EMAIL          Email for Let's Encrypt account [required]
  ORLY_CERTS_DNS_PROVIDER   DNS provider name [required]
  ORLY_CERTS_OUTPUT_DIR     Certificate output directory [default: /var/cache/orly-certs]
  ORLY_CERTS_RENEW_DAYS     Renew when expiring within N days [default: 30]
  ORLY_CERTS_CHECK_INTERVAL Renewal check interval [default: 12h]
  ORLY_CERTS_ACME_SERVER    ACME server URL [default: production Let's Encrypt]
  ORLY_CERTS_LOG_LEVEL      Log level [default: info]

Supported DNS Providers:
  cloudflare, route53, hetzner, digitalocean, google, namecheap, godaddy,
  ovh, vultr, linode, gandi, dnsimple, duckdns, azure, alidns, and 80+ more.

Provider credentials are set via standard lego environment variables.
See https://go-acme.github.io/lego/dns/ for documentation.

Example:
  export CF_API_TOKEN="your-cloudflare-api-token"
  export ORLY_CERTS_DOMAIN="*.myapp.com"
  export ORLY_CERTS_EMAIL="admin@myapp.com"
  export ORLY_CERTS_DNS_PROVIDER="cloudflare"
  ./orly-certs
`)
}
