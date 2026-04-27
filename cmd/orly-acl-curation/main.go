// orly-acl-curation is a standalone gRPC ACL server using the Curating mode.
// It provides three-tier classification: trusted, blacklisted, and unclassified (rate-limited).
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-simpler.org/env"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/acl/server"
	"git.smesh.lol/orly/pkg/database"
	databasegrpc "git.smesh.lol/orly/pkg/database/grpc"
)

// Config holds the ACL server configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_ACL_LISTEN" default:"127.0.0.1:50052" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_ACL_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// Database configuration
	DBType       string `env:"ORLY_ACL_DB_TYPE" default:"grpc" usage:"database type: badger or grpc"`
	GRPCDBServer string `env:"ORLY_ACL_GRPC_DB_SERVER" usage:"gRPC database server address (when DB_TYPE=grpc)"`
	DataDir      string `env:"ORLY_DATA_DIR" usage:"database data directory (when DB_TYPE=badger)"`

	// Badger configuration (when DB_TYPE=badger)
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
	log.I.F("orly-acl-curation starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database (direct Badger or gRPC client)
	var db database.Database
	var err error
	var ownsDB bool

	if cfg.DBType == "grpc" {
		// Use gRPC database client
		log.I.F("connecting to gRPC database server at %s", cfg.GRPCDBServer)
		db, err = databasegrpc.New(ctx, &databasegrpc.ClientConfig{
			ServerAddress:  cfg.GRPCDBServer,
			ConnectTimeout: 30 * time.Second,
		})
		if chk.E(err) {
			log.E.F("failed to connect to gRPC database: %v", err)
			os.Exit(1)
		}
		ownsDB = false // gRPC client doesn't own the database
	} else {
		// Use direct Badger database
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
		db, err = database.NewWithConfig(ctx, cancel, dbCfg)
		if chk.E(err) {
			log.E.F("failed to initialize database: %v", err)
			os.Exit(1)
		}
		ownsDB = true
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create server config
	serverCfg := &server.Config{
		Listen:         cfg.Listen,
		ACLMode:        "curating", // Hardcoded for this binary
		LogLevel:       cfg.LogLevel,
		Owners:         splitList(cfg.Owners),
		Admins:         splitList(cfg.Admins),
		RelayAddresses: splitList(cfg.RelayAddresses),
	}

	// Create and configure server
	srv := server.New(db, serverCfg, ownsDB)
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

	// Ensure data directory exists (for badger mode)
	if cfg.DBType == "badger" || cfg.DBType == "" {
		if err := os.MkdirAll(cfg.DataDir, 0700); chk.E(err) {
			log.E.F("failed to create data directory %s: %v", cfg.DataDir, err)
			os.Exit(1)
		}
	}

	return cfg
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
