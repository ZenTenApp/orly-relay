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

Process supervisor for split-mode deployment of ORLY relay.

Usage: orly-launcher [command]

Commands:
  help, -h, --help       Show this help
  version, -v, --version Show version

Environment Variables:
  ORLY_LAUNCHER_DB_BINARY       Path to orly-db binary (default: orly-db)
  ORLY_LAUNCHER_RELAY_BINARY    Path to orly binary (default: orly)
  ORLY_LAUNCHER_DB_LISTEN       Address for database server (default: 127.0.0.1:50051)
  ORLY_LAUNCHER_DB_READY_TIMEOUT Timeout waiting for DB ready (default: 30s)
  ORLY_LAUNCHER_STOP_TIMEOUT    Timeout for graceful stop (default: 10s)
  ORLY_DATA_DIR                 Data directory (passed to orly-db)
  ORLY_DB_LOG_LEVEL             Database log level (passed to orly-db)

The launcher will:
1. Start the database server (orly-db)
2. Wait for the database to be ready
3. Start the relay (orly) with ORLY_DB_TYPE=grpc
4. Monitor both processes and restart if they crash
5. On shutdown, stop relay first, then database

Example:
  # Start with default binaries in PATH
  orly-launcher

  # Start with custom binary paths
  ORLY_LAUNCHER_DB_BINARY=/opt/orly/orly-db \
  ORLY_LAUNCHER_RELAY_BINARY=/opt/orly/orly \
  orly-launcher
`, version.V)
}
