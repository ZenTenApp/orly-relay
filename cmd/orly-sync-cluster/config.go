package main

import (
	"os"
	"strings"
	"time"

	"go-simpler.org/env"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// Config holds the cluster sync service configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_SYNC_CLUSTER_LISTEN" default:"127.0.0.1:50062" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_SYNC_CLUSTER_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_SYNC_CLUSTER_DB_TYPE" default:"grpc" usage:"database type: grpc or badger"`
	GRPCDBServer string `env:"ORLY_SYNC_CLUSTER_DB_SERVER" default:"127.0.0.1:50051" usage:"gRPC database server address"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (for badger mode)"`

	// Cluster configuration
	AdminNpubsRaw             string        `env:"ORLY_SYNC_CLUSTER_ADMINS" usage:"comma-separated admin npubs"`
	PropagatePrivilegedEvents bool          `env:"ORLY_SYNC_CLUSTER_PROPAGATE_PRIVILEGED" default:"true" usage:"propagate privileged events (DMs, gift wraps)"`
	PollInterval              time.Duration `env:"ORLY_SYNC_CLUSTER_POLL_INTERVAL" default:"5s" usage:"poll interval"`

	// NIP-11 cache configuration
	NIP11CacheTTL time.Duration `env:"ORLY_SYNC_CLUSTER_NIP11_TTL" default:"30m" usage:"NIP-11 cache TTL"`

	// Parsed admin npubs
	AdminNpubs []string
}

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	// Parse admin npubs from comma-separated string
	if cfg.AdminNpubsRaw != "" {
		cfg.AdminNpubs = strings.Split(cfg.AdminNpubsRaw, ",")
		for i, p := range cfg.AdminNpubs {
			cfg.AdminNpubs[i] = strings.TrimSpace(p)
		}
	}

	return cfg
}
