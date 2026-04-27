//go:build !(js && wasm)

package db

import (
	"context"
	"fmt"
	"os"

	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database"
)

func runHealth(args []string) {
	var showHelp bool

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}

	if showHelp {
		printHealthHelp()
		return
	}

	cfg := loadConfig()
	lol.SetLogLevel(cfg.LogLevel)

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

	// Initialize database directly (health check is Badger-specific)
	log.I.F("initializing Badger database at %s for health check", cfg.DataDir)
	db, err := database.NewWithConfig(ctx, cancel, dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// Wait for database to be ready
	<-db.Ready()

	// Run health check
	report, err := db.HealthCheck(os.Stdout)
	if err != nil {
		log.E.F("health check failed: %v", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(report.String())

	// Exit with non-zero if health score is critical
	if report.HealthScore < 50 {
		os.Exit(2)
	}
}

func printHealthHelp() {
	fmt.Println(`orly db health - Database health check

Usage:
  orly db health [options]

Options:
  --help, -h    Show this help message

Environment variables:
  ORLY_DATA_DIR          Database data directory
  ORLY_DB_LOG_LEVEL      Logging level

The health check scans the database for integrity issues:
  - Missing serial->eventID mappings (sei)
  - Orphaned serial->eventID mappings
  - Pubkey serial inconsistencies
  - Orphaned index entries

Exit codes:
  0 - Database is healthy (score >= 50)
  1 - Error running health check
  2 - Database has critical issues (score < 50)`)
}
