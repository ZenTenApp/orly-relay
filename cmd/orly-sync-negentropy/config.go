package main

import (
	"os"
	"strings"
	"time"

	"go-simpler.org/env"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Config holds the negentropy sync service configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_SYNC_NEGENTROPY_LISTEN" default:"127.0.0.1:50064" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_SYNC_NEGENTROPY_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_SYNC_NEGENTROPY_DB_TYPE" default:"grpc" usage:"database type: grpc or badger"`
	GRPCDBServer string `env:"ORLY_SYNC_NEGENTROPY_DB_SERVER" default:"127.0.0.1:50051" usage:"gRPC database server address"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (for badger mode)"`

	// Negentropy configuration
	PeersRaw     string        `env:"ORLY_SYNC_NEGENTROPY_PEERS" usage:"comma-separated peer relay WebSocket URLs"`
	SyncInterval time.Duration `env:"ORLY_SYNC_NEGENTROPY_INTERVAL" default:"60s" usage:"background sync interval"`
	FrameSize    int           `env:"ORLY_SYNC_NEGENTROPY_FRAME_SIZE" default:"4096" usage:"negentropy frame size"`
	IDSize       int           `env:"ORLY_SYNC_NEGENTROPY_ID_SIZE" default:"16" usage:"negentropy ID truncation size"`

	// Client session configuration
	ClientSessionTimeout time.Duration `env:"ORLY_SYNC_NEGENTROPY_SESSION_TIMEOUT" default:"5m" usage:"client session timeout"`

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
