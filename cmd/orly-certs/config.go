package main

import (
	"os"
	"time"

	"go-simpler.org/env"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// Config holds the configuration for the certificate manager.
type Config struct {
	// Domain is the wildcard domain to obtain a certificate for (e.g., "*.myapp.com")
	Domain string `env:"ORLY_CERTS_DOMAIN" required:"true" usage:"wildcard domain (e.g., *.myapp.com)"`

	// Email is the email address for the Let's Encrypt account
	Email string `env:"ORLY_CERTS_EMAIL" required:"true" usage:"email for Let's Encrypt account"`

	// DNSProvider is the name of the DNS provider (cloudflare, route53, hetzner, etc.)
	DNSProvider string `env:"ORLY_CERTS_DNS_PROVIDER" required:"true" usage:"DNS provider name (cloudflare, route53, hetzner, etc.)"`

	// OutputDir is the directory where certificates will be stored
	OutputDir string `env:"ORLY_CERTS_OUTPUT_DIR" default:"/var/cache/orly-certs" usage:"certificate output directory"`

	// RenewDays is the number of days before expiry to trigger renewal
	RenewDays int `env:"ORLY_CERTS_RENEW_DAYS" default:"30" usage:"renew certificate when expiring within N days"`

	// CheckInterval is how often to check for renewal
	CheckInterval time.Duration `env:"ORLY_CERTS_CHECK_INTERVAL" default:"12h" usage:"how often to check for renewal"`

	// ACMEServer is the ACME server URL (empty for production Let's Encrypt)
	ACMEServer string `env:"ORLY_CERTS_ACME_SERVER" default:"" usage:"ACME server URL (empty for production)"`

	// LogLevel is the log level
	LogLevel string `env:"ORLY_CERTS_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// AccountKeyPath is the path to store the ACME account private key
	AccountKeyPath string `env:"ORLY_CERTS_ACCOUNT_KEY" default:"" usage:"path to ACME account key (auto-generated if empty)"`
}

// ProductionACMEServer is the Let's Encrypt production ACME server
const ProductionACMEServer = "https://acme-v02.api.letsencrypt.org/directory"

// StagingACMEServer is the Let's Encrypt staging ACME server (for testing)
const StagingACMEServer = "https://acme-staging-v02.api.letsencrypt.org/directory"

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}
	return cfg
}

// ACMEServerURL returns the ACME server URL to use.
func (c *Config) ACMEServerURL() string {
	if c.ACMEServer != "" {
		return c.ACMEServer
	}
	return ProductionACMEServer
}

// BaseDomain extracts the base domain from the wildcard domain.
// e.g., "*.myapp.com" -> "myapp.com"
func (c *Config) BaseDomain() string {
	domain := c.Domain
	if len(domain) > 2 && domain[:2] == "*." {
		return domain[2:]
	}
	return domain
}
