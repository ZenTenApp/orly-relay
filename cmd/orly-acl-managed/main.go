// orly-acl-managed is a standalone gRPC ACL server using the Managed mode.
// It provides fine-grained control via NIP-86 management API.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-simpler.org/env"
	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/acl/server"
	"next.orly.dev/pkg/database"
)

// Config holds the ACL server configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_ACL_LISTEN" default:"127.0.0.1:50052" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_ACL_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration - Managed mode requires direct Badger access
	DataDir string `env:"ORLY_DATA_DIR" usage:"database data directory"`

	// Badger configuration
	BlockCacheMB        int           `env:"ORLY_DB_BLOCK_CACHE_MB" default:"256" usage:"block cache size in MB"`
	IndexCacheMB        int           `env:"ORLY_DB_INDEX_CACHE_MB" default:"128" usage:"index cache size in MB"`
	ZSTDLevel           int           `env:"ORLY_DB_ZSTD_LEVEL" default:"3" usage:"ZSTD compression level"`
	QueryCacheSizeMB    int           `env:"ORLY_DB_QUERY_CACHE_SIZE_MB" default:"64" usage:"query cache size in MB"`
	QueryCacheMaxAge    time.Duration `env:"ORLY_DB_QUERY_CACHE_MAX_AGE" default:"5m" usage:"query cache max age"`
	QueryCacheDisabled  bool          `env:"ORLY_DB_QUERY_CACHE_DISABLED" default:"false" usage:"disable query cache"`
	SerialCachePubkeys  int           `env:"ORLY_SERIAL_CACHE_PUBKEYS" default:"100000" usage:"serial cache pubkeys capacity"`
	SerialCacheEventIds int           `env:"ORLY_SERIAL_CACHE_EVENT_IDS" default:"500000" usage:"serial cache event IDs capacity"`

	// ACL configuration
	Owners         string `env:"ORLY_OWNERS" usage:"comma-separated list of owner npubs"`
	Admins         string `env:"ORLY_ADMINS" usage:"comma-separated list of admin npubs"`
	RelayAddresses string `env:"ORLY_RELAY_ADDRESSES" usage:"comma-separated list of relay addresses (self)"`
}

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-acl-managed starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Managed mode requires direct Badger database access (not gRPC)
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

	// Create server config
	serverCfg := &server.Config{
		Listen:         cfg.Listen,
		ACLMode:        "managed", // Hardcoded for this binary
		LogLevel:       cfg.LogLevel,
		Owners:         splitList(cfg.Owners),
		Admins:         splitList(cfg.Admins),
		RelayAddresses: splitList(cfg.RelayAddresses),
	}

	// Create and configure server
	srv := server.New(db, serverCfg, true)
	if err := srv.ConfigureACL(ctx); chk.E(err) {
		log.E.F("failed to configure ACL: %v", err)
		os.Exit(1)
	}

	// Start server
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

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
