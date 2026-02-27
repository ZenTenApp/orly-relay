//go:build !(js && wasm)

// Package launcher implements the "orly launcher" subcommand for process supervision.
// It uses a self-exec pattern: instead of spawning separate binaries, it spawns
// the same unified binary with different subcommands (db, acl, sync, relay).
package launcher

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// Run executes the launcher subcommand.
func Run(args []string) {
	var showHelp bool

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}

	if showHelp {
		printLauncherHelp()
		return
	}

	// Load configuration
	cfg, err := loadConfig()
	if chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly launcher starting (unified binary with self-exec)")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create supervisor
	supervisor, err := NewSupervisor(ctx, cancel, cfg)
	if chk.E(err) {
		log.E.F("failed to create supervisor: %v", err)
		os.Exit(1)
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start services if enabled
	if cfg.ServicesEnabled {
		log.I.F("starting services (db driver=%s, acl driver=%s, acl enabled=%v)",
			cfg.DBDriver, cfg.ACLDriver, cfg.ACLEnabled)
		if err := supervisor.Start(); chk.E(err) {
			log.E.F("failed to start services: %v", err)
			os.Exit(1)
		}
		log.I.F("all services started successfully")
	} else {
		log.I.F("services disabled, running admin UI only")
	}

	// Wait for shutdown signal
	<-sigCh
	log.I.F("received shutdown signal")

	// Stop supervisor
	if cfg.ServicesEnabled {
		if err := supervisor.Stop(); chk.E(err) {
			log.E.F("error during shutdown: %v", err)
		}
	}

	log.I.F("launcher stopped")
}

func printLauncherHelp() {
	fmt.Println(`orly launcher - Unified binary process supervisor

Usage:
  orly launcher [options]

Options:
  --help, -h         Show this help message

The launcher supervises split-mode deployment using self-exec pattern.
Instead of running separate binaries, it spawns the same unified binary
with different subcommands:

  - orly db --driver=badger     (database server)
  - orly acl --driver=follows   (ACL server)
  - orly sync --driver=X        (sync services)
  - orly                        (main relay)

This reduces total binary size from ~130MB (4 separate binaries) to ~33MB
(single stripped binary) by eliminating duplicate Go runtime and shared code.

Environment variables:
  ORLY_LAUNCHER_DB_DRIVER         Database driver (badger, neo4j)
  ORLY_LAUNCHER_DB_LISTEN         Database server listen address
  ORLY_LAUNCHER_ACL_ENABLED       Enable ACL subprocess
  ORLY_LAUNCHER_ACL_LISTEN        ACL server listen address
  ORLY_ACL_MODE                   ACL driver (follows, managed, curating)
  ORLY_DATA_DIR                   Data directory for database
  ORLY_LOG_LEVEL                  Log level (trace, debug, info, warn, error)

Sync services:
  ORLY_LAUNCHER_SYNC_DISTRIBUTED_ENABLED   Enable distributed sync
  ORLY_LAUNCHER_SYNC_CLUSTER_ENABLED       Enable cluster sync
  ORLY_LAUNCHER_SYNC_RELAYGROUP_ENABLED    Enable relay group sync
  ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED    Enable negentropy sync

Other:
  ORLY_LAUNCHER_CERTS_ENABLED     Enable certificate service
  ORLY_LAUNCHER_SERVICES_ENABLED  Enable all services (default: true)
  ORLY_LAUNCHER_ADMIN_ENABLED     Enable admin HTTP server

Example:
  # Start with default settings (badger DB, no ACL)
  orly launcher

  # Start with follows ACL enabled
  ORLY_LAUNCHER_ACL_ENABLED=true ORLY_ACL_MODE=follows orly launcher

  # Start with neo4j database
  ORLY_LAUNCHER_DB_DRIVER=neo4j orly launcher`)
}
