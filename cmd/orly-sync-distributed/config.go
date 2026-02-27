package main

import (
	"os"
	"strings"
	"time"

	"go-simpler.org/env"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// Config holds the distributed sync service configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_SYNC_DISTRIBUTED_LISTEN" default:"127.0.0.1:50061" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_SYNC_DISTRIBUTED_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_SYNC_DISTRIBUTED_DB_TYPE" default:"grpc" usage:"database type: grpc or badger"`
	GRPCDBServer string `env:"ORLY_SYNC_DISTRIBUTED_DB_SERVER" default:"127.0.0.1:50051" usage:"gRPC database server address"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (for badger mode)"`

	// Sync configuration
	NodeID       string        `env:"ORLY_SYNC_DISTRIBUTED_NODE_ID" usage:"node identity (relay pubkey)"`
	RelayURL     string        `env:"ORLY_SYNC_DISTRIBUTED_RELAY_URL" usage:"this relay's URL"`
	PeersRaw     string        `env:"ORLY_SYNC_DISTRIBUTED_PEERS" usage:"comma-separated peer relay URLs"`
	SyncInterval time.Duration `env:"ORLY_SYNC_DISTRIBUTED_INTERVAL" default:"5s" usage:"sync poll interval"`

	// NIP-11 cache configuration
	NIP11CacheTTL time.Duration `env:"ORLY_SYNC_DISTRIBUTED_NIP11_TTL" default:"30m" usage:"NIP-11 cache TTL"`

	// Parsed peers
	Peers []string
}

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	// Parse peers from comma-separated string
	if cfg.PeersRaw != "" {
		cfg.Peers = strings.Split(cfg.PeersRaw, ",")
		for i, p := range cfg.Peers {
			cfg.Peers[i] = strings.TrimSpace(p)
		}
	}

	return cfg
}
