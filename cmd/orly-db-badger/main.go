// orly-db-badger is a standalone gRPC database server using the Badger backend.
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"go-simpler.org/env"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/database/server"
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
	StreamBatchSize int `env:"ORLY_DB_STREAM_BATCH_SIZE" default:"100" usage:"events per stream batch"`
}

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-db-badger starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create database configuration
	dbCfg := &database.DatabaseConfig{
		DataDir:             cfg.DataDir,
		LogLevel:            cfg.LogLevel,
		BlockCacheMB:        cfg.BlockCacheMB,
		IndexCacheMB:        cfg.IndexCacheMB,
		QueryCacheSizeMB:    cfg.QueryCacheSizeMB,
		QueryCacheMaxAge:    cfg.QueryCacheMaxAge,
		QueryCacheDisabled:  cfg.QueryCacheDisabled,
		SerialCachePubkeys:  cfg.SerialCachePubkeys,
		SerialCacheEventIds: cfg.SerialCacheEventIds,
		ZSTDLevel:           cfg.ZSTDLevel,
	}

	// Initialize Badger database
	log.I.F("initializing Badger database at %s", cfg.DataDir)
	db, err := database.NewWithConfig(ctx, cancel, dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize database: %v", err)
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Reconcile any orphaned blob files (files on disk without metadata)
	if reconciled, err := db.ReconcileBlobMetadata(); err != nil {
		log.W.F("blob metadata reconciliation failed: %v", err)
	} else if reconciled > 0 {
		log.I.F("reconciled %d blob metadata entries", reconciled)
	}

	// Create and start gRPC server
	serverCfg := &server.Config{
		Listen:          cfg.Listen,
		LogLevel:        cfg.LogLevel,
		StreamBatchSize: cfg.StreamBatchSize,
	}

	srv := server.New(db, serverCfg)
	if err := srv.ListenAndServe(ctx, cancel); err != nil {
		log.E.F("gRPC server error: %v", err)
	}
}

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
