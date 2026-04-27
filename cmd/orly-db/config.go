package main

import (
	"os"
	"path/filepath"
	"time"

	"go-simpler.org/env"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Config holds the database server configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_DB_LISTEN" default:"127.0.0.1:50051" usage:"gRPC server listen address"`

	// DataDir is the database data directory
	DataDir string `env:"ORLY_DATA_DIR" usage:"database data directory"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_DB_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Badger configuration
	BlockCacheMB int `env:"ORLY_DB_BLOCK_CACHE_MB" default:"1024" usage:"block cache size in MB"`
	IndexCacheMB int `env:"ORLY_DB_INDEX_CACHE_MB" default:"512" usage:"index cache size in MB"`
	ZSTDLevel    int `env:"ORLY_DB_ZSTD_LEVEL" default:"3" usage:"ZSTD compression level (1-19)"`

	// Query cache configuration
	QueryCacheSizeMB   int           `env:"ORLY_DB_QUERY_CACHE_SIZE_MB" default:"256" usage:"query cache size in MB"`
	QueryCacheMaxAge   time.Duration `env:"ORLY_DB_QUERY_CACHE_MAX_AGE" default:"5m" usage:"query cache max age"`
	QueryCacheDisabled bool          `env:"ORLY_DB_QUERY_CACHE_DISABLED" default:"false" usage:"disable query cache"`

	// Serial cache configuration
	SerialCachePubkeys  int `env:"ORLY_SERIAL_CACHE_PUBKEYS" default:"100000" usage:"serial cache pubkeys capacity"`
	SerialCacheEventIds int `env:"ORLY_SERIAL_CACHE_EVENT_IDS" default:"500000" usage:"serial cache event IDs capacity"`

	// gRPC server configuration
	MaxConcurrentQueries int `env:"ORLY_DB_MAX_CONCURRENT_QUERIES" default:"3" usage:"max concurrent queries"`
	StreamBatchSize      int `env:"ORLY_DB_STREAM_BATCH_SIZE" default:"100" usage:"events per stream batch"`
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

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0700); chk.E(err) {
		log.E.F("failed to create data directory %s: %v", cfg.DataDir, err)
		os.Exit(1)
	}

	return cfg
}
