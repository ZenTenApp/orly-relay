package main

import (
	"os"
	"strings"

	"go-simpler.org/env"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Config holds the relay group service configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_SYNC_RELAYGROUP_LISTEN" default:"127.0.0.1:50063" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_SYNC_RELAYGROUP_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_SYNC_RELAYGROUP_DB_TYPE" default:"grpc" usage:"database type: grpc or badger"`
	GRPCDBServer string `env:"ORLY_SYNC_RELAYGROUP_DB_SERVER" default:"127.0.0.1:50051" usage:"gRPC database server address"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (for badger mode)"`

	// Relay group configuration
	AdminNpubsRaw string `env:"ORLY_SYNC_RELAYGROUP_ADMINS" usage:"comma-separated admin npubs"`

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
