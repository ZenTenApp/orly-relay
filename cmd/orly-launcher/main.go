// orly-launcher is a process supervisor that manages the database and relay
// processes in split mode. It starts the database server first, waits for it
// to be ready, then starts the relay with the gRPC database backend.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/version"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Handle version request
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println(version.V)
		os.Exit(0)
	}

	// Handle help request
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help") {
		printHelp()
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	supervisor := NewSupervisor(ctx, cancel, cfg)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.I.F("received signal %v, shutting down...", sig)
		cancel()
	}()

	log.I.F("starting orly-launcher %s", version.V)
	log.I.F("database binary: %s", cfg.DBBinary)
	log.I.F("relay binary: %s", cfg.RelayBinary)
	log.I.F("database listen: %s", cfg.DBListen)

	// Start admin server if enabled
	var adminServer *AdminServer
	if cfg.AdminEnabled {
		adminServer = NewAdminServer(cfg, supervisor)

		// Ensure binary directory structure exists
		if err := adminServer.updater.EnsureDirectories(); chk.E(err) {
			log.W.F("failed to create binary directories: %v", err)
		}

		go func() {
			if err := adminServer.Start(ctx); err != nil {
				// Don't exit on admin server error, just log it
				log.W.F("admin server stopped: %v", err)
			}
		}()
		log.I.F("admin UI available at http://localhost:%d/admin", cfg.AdminPort)
		if len(cfg.AdminOwners) > 0 {
			log.I.F("admin owners: %v", cfg.AdminOwners)
		} else {
			log.W.F("no admin owners configured - admin API access disabled")
		}
	}

	if err := supervisor.Start(); chk.E(err) {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}

	// Wait for context cancellation (signal received)
	<-ctx.Done()

	log.I.F("stopping supervisor...")
	if err := supervisor.Stop(); chk.E(err) {
		log.E.F("error during shutdown: %v", err)
	}

	log.I.F("orly-launcher stopped")
}

func printHelp() {
	fmt.Printf(`orly-launcher %s

Process supervisor for split-mode deployment of ORLY relay with admin web UI.

Usage: orly-launcher [command]

Commands:
  help, -h, --help       Show this help
  version, -v, --version Show version

Environment Variables:
  Process Management:
    ORLY_LAUNCHER_DB_BINARY        Path to orly-db binary (default: orly-db-{backend})
    ORLY_LAUNCHER_RELAY_BINARY     Path to orly binary (default: orly)
    ORLY_LAUNCHER_ACL_BINARY       Path to orly-acl binary (default: orly-acl-{mode})
    ORLY_LAUNCHER_DB_BACKEND       Database backend: badger, neo4j (default: badger)
    ORLY_LAUNCHER_DB_LISTEN        Address for database server (default: 127.0.0.1:50051)
    ORLY_LAUNCHER_ACL_LISTEN       Address for ACL server (default: 127.0.0.1:50052)
    ORLY_LAUNCHER_ACL_ENABLED      Enable ACL server (default: false)
    ORLY_ACL_MODE                  ACL mode: follows, managed, curation (default: follows)
    ORLY_LAUNCHER_DB_READY_TIMEOUT Timeout waiting for DB ready (default: 30s)
    ORLY_LAUNCHER_STOP_TIMEOUT     Timeout for graceful stop (default: 30s)
    ORLY_DATA_DIR                  Data directory (passed to orly-db)
    ORLY_LOG_LEVEL                 Log level for all processes (default: info)

  Admin UI:
    ORLY_LAUNCHER_ADMIN_ENABLED    Enable admin HTTP server (default: true)
    ORLY_LAUNCHER_ADMIN_PORT       Admin server port (default: 8080)
    ORLY_LAUNCHER_OWNERS           Comma-separated hex pubkeys for admin access
    ORLY_LAUNCHER_BIN_DIR          Directory for versioned binaries

The launcher will:
1. Start the admin HTTP server (optional)
2. Start the database server (orly-db)
3. Wait for the database to be ready
4. Start the ACL server if enabled (orly-acl)
5. Start sync services if enabled
6. Start the relay (orly) with ORLY_DB_TYPE=grpc
7. Monitor all processes and restart if they crash
8. On shutdown, stop in reverse dependency order

Admin UI Features:
- View process status and versions
- Update binaries from release URLs
- Edit configuration
- Restart/rollback binaries

Example:
  # Start with default binaries in PATH
  orly-launcher

  # Start with admin access for a specific pubkey
  ORLY_LAUNCHER_OWNERS=abc123... orly-launcher

  # Start with custom binary paths
  ORLY_LAUNCHER_DB_BINARY=/opt/orly/orly-db-badger \
  ORLY_LAUNCHER_RELAY_BINARY=/opt/orly/orly \
  orly-launcher
`, version.V)
}
