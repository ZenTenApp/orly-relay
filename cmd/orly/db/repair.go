//go:build !(js && wasm)

package db

import (
	"context"
	"fmt"
	"os"

	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/database"
)

func runRepair(args []string) {
	var dryRun bool
	var showHelp bool

	for _, arg := range args {
		if arg == "--dry-run" || arg == "-n" {
			dryRun = true
		} else if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}

	if showHelp {
		printRepairHelp()
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

	// Initialize database directly (repair is Badger-specific)
	log.I.F("initializing Badger database at %s for repair", cfg.DataDir)
	db, err := database.NewWithConfig(ctx, cancel, dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// Wait for database to be ready
	<-db.Ready()

	// Run repair
	opts := &database.RepairOptions{
		DryRun:            dryRun,
		FixMissingSei:     true,
		RemoveOrphanedSei: true,
		FixPubkeyMappings: true,
		Progress:          os.Stdout,
	}

	report, err := db.Repair(ctx, opts)
	if err != nil {
		log.E.F("repair failed: %v", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(report.String())

	if len(report.Errors) > 0 {
		os.Exit(1)
	}
}

func printRepairHelp() {
	fmt.Println(`orly db repair - Database repair

Usage:
  orly db repair [options]

Options:
  --dry-run, -n   Preview repairs without making changes
  --help, -h      Show this help message

Environment variables:
  ORLY_DATA_DIR          Database data directory
  ORLY_DB_LOG_LEVEL      Logging level

The repair operation fixes integrity issues found by health check:
  - Rebuilds missing serial->eventID mappings from compact events
  - Removes orphaned serial->eventID mappings
  - Reports pubkey serial inconsistencies

WARNING: Always backup your database before running repair.
         Use --dry-run to preview changes first.`)
}
