package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-simpler.org/env"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Config holds the ACL server configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_ACL_LISTEN" default:"127.0.0.1:50052" usage:"gRPC server listen address"`

	// ACLMode is the active ACL mode (none, follows, managed, curating)
	ACLMode string `env:"ORLY_ACL_MODE" default:"none" usage:"ACL mode: none, follows, managed, curating"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_ACL_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_ACL_DB_TYPE" default:"badger" usage:"database type: badger or grpc"`
	GRPCDBServer string `env:"ORLY_ACL_GRPC_DB_SERVER" usage:"gRPC database server address (when DB_TYPE=grpc)"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (when DB_TYPE=badger)"`

	// Badger configuration (when DB_TYPE=badger)
	BlockCacheMB       int           `env:"ORLY_DB_BLOCK_CACHE_MB" default:"1024" usage:"block cache size in MB"`
	IndexCacheMB       int           `env:"ORLY_DB_INDEX_CACHE_MB" default:"512" usage:"index cache size in MB"`
	ZSTDLevel          int           `env:"ORLY_DB_ZSTD_LEVEL" default:"3" usage:"ZSTD compression level"`
	QueryCacheSizeMB   int           `env:"ORLY_DB_QUERY_CACHE_SIZE_MB" default:"256" usage:"query cache size in MB"`
	QueryCacheMaxAge   time.Duration `env:"ORLY_DB_QUERY_CACHE_MAX_AGE" default:"5m" usage:"query cache max age"`
	QueryCacheDisabled bool          `env:"ORLY_DB_QUERY_CACHE_DISABLED" default:"false" usage:"disable query cache"`
	SerialCachePubkeys int           `env:"ORLY_SERIAL_CACHE_PUBKEYS" default:"100000" usage:"serial cache pubkeys capacity"`
	SerialCacheEventIds int          `env:"ORLY_SERIAL_CACHE_EVENT_IDS" default:"500000" usage:"serial cache event IDs capacity"`

	// ACL configuration
	Owners          string        `env:"ORLY_OWNERS" usage:"comma-separated list of owner npubs"`
	Admins          string        `env:"ORLY_ADMINS" usage:"comma-separated list of admin npubs"`
	BootstrapRelays string        `env:"ORLY_BOOTSTRAP_RELAYS" usage:"comma-separated list of bootstrap relays"`
	RelayAddresses  string        `env:"ORLY_RELAY_ADDRESSES" usage:"comma-separated list of relay addresses (self)"`

	// Follows ACL configuration
	FollowListFrequency   time.Duration `env:"ORLY_FOLLOW_LIST_FREQUENCY" default:"1h" usage:"follow list sync frequency"`
	FollowsThrottleEnabled bool         `env:"ORLY_FOLLOWS_THROTTLE_ENABLED" default:"false" usage:"enable progressive throttle for non-followed users"`
	FollowsThrottlePerEvent time.Duration `env:"ORLY_FOLLOWS_THROTTLE_PER_EVENT" default:"25ms" usage:"throttle delay increment per event"`
	FollowsThrottleMaxDelay time.Duration `env:"ORLY_FOLLOWS_THROTTLE_MAX_DELAY" default:"60s" usage:"maximum throttle delay"`
}

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	// Set default data directory if not specified
	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if chk.E(err) {
			log.E.F("failed to get home directory: %v", err)
			os.Exit(1)
		}
		cfg.DataDir = filepath.Join(home, ".local", "share", "ORLY")
	}

	// Ensure data directory exists (for badger mode)
	if cfg.DBType == "badger" {
		if err := os.MkdirAll(cfg.DataDir, 0700); chk.E(err) {
			log.E.F("failed to create data directory %s: %v", cfg.DataDir, err)
			os.Exit(1)
		}
	}

	return cfg
}

// GetOwners returns the list of owner pubkeys
func (c *Config) GetOwners() []string {
	if c.Owners == "" {
		return nil
	}
	return strings.Split(c.Owners, ",")
}

// GetAdmins returns the list of admin pubkeys
func (c *Config) GetAdmins() []string {
	if c.Admins == "" {
		return nil
	}
	return strings.Split(c.Admins, ",")
}

// GetBootstrapRelays returns the list of bootstrap relays
func (c *Config) GetBootstrapRelays() []string {
	if c.BootstrapRelays == "" {
		return nil
	}
	return strings.Split(c.BootstrapRelays, ",")
}

// GetRelayAddresses returns the list of relay addresses (self)
func (c *Config) GetRelayAddresses() []string {
	if c.RelayAddresses == "" {
		return nil
	}
	return strings.Split(c.RelayAddresses, ",")
}
