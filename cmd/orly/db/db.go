//go:build !(js && wasm)

// Package db implements the "orly db" subcommand for database operations.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-simpler.org/env"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/server"
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

// Run executes the db subcommand.
func Run(args []string) {
	var driver string
	var listDrivers bool
	var showHelp bool

	// Parse args for subcommands and flags
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--driver=") {
			driver = strings.TrimPrefix(arg, "--driver=")
		} else if arg == "--driver" && i+1 < len(args) {
			driver = args[i+1]
			i++
		} else if arg == "--list-drivers" || arg == "-l" {
			listDrivers = true
		} else if arg == "--help" || arg == "-h" {
			showHelp = true
		} else if arg == "health" {
			runHealth(args[i+1:])
			return
		} else if arg == "repair" {
			runRepair(args[i+1:])
			return
		}
	}

	if showHelp {
		printDBHelp()
		return
	}

	if listDrivers {
		drivers := database.ListDriversWithInfo()
		if len(drivers) == 0 {
			fmt.Println("No database drivers available.")
			fmt.Println("Build with appropriate tags: -tags=badger or -tags=neo4j or -tags=all")
			return
		}
		fmt.Println("Available database drivers:")
		for _, d := range drivers {
			fmt.Printf("  %-10s - %s\n", d.Name, d.Description)
		}
		return
	}

	if driver == "" {
		// Check if any driver is registered
		drivers := database.ListDrivers()
		if len(drivers) == 0 {
			fmt.Fprintln(os.Stderr, "error: no database drivers available")
			fmt.Fprintln(os.Stderr, "build with: -tags=badger or -tags=neo4j")
			os.Exit(1)
		}
		if len(drivers) == 1 {
			// Use the only available driver
			driver = drivers[0]
			log.I.F("using default driver: %s", driver)
		} else {
			fmt.Fprintln(os.Stderr, "error: --driver required (multiple drivers available)")
			fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(drivers, ", "))
			os.Exit(1)
		}
	}

	// Check if driver is available
	if !database.HasDriver(driver) {
		fmt.Fprintf(os.Stderr, "error: driver %q not available\n", driver)
		fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(database.ListDrivers(), ", "))
		os.Exit(1)
	}

	runServer(driver)
}

func runServer(driver string) {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly db --driver=%s starting with log level: %s", driver, cfg.LogLevel)

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

	// Initialize database using the driver registry
	log.I.F("initializing %s database at %s", driver, cfg.DataDir)
	db, err := database.NewFromDriver(ctx, cancel, driver, dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize database: %v", err)
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

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

func printDBHelp() {
	fmt.Println(`orly db - Database server operations

Usage:
  orly db [options]
  orly db health [options]
  orly db repair [options]

Options:
  --driver=NAME      Select database driver (badger, neo4j)
  --list-drivers     List available database drivers
  --help, -h         Show this help message

Subcommands:
  health             Run database health check
  repair             Repair database issues

Environment variables:
  ORLY_DATA_DIR                Database data directory
  ORLY_DB_LISTEN               gRPC server listen address
  ORLY_DB_LOG_LEVEL            Logging level
  ORLY_DB_BLOCK_CACHE_MB       Block cache size in MB
  ORLY_DB_INDEX_CACHE_MB       Index cache size in MB
  ORLY_DB_ZSTD_LEVEL           ZSTD compression level (1-19)
  ORLY_DB_QUERY_CACHE_SIZE_MB  Query cache size in MB
  ORLY_DB_QUERY_CACHE_MAX_AGE  Query cache max age
  ORLY_DB_QUERY_CACHE_DISABLED Disable query cache

Examples:
  orly db --driver=badger      Run Badger database server
  orly db --list-drivers       List available drivers
  orly db health               Check database health
  orly db repair --dry-run     Preview repairs without applying`)
}
