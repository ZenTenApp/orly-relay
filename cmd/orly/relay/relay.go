//go:build !(js && wasm)

// Package relay implements the "orly relay" subcommand (the default command).
package relay

import (
	"fmt"
	"os"

	"lol.mleku.dev/log"
)

// Run executes the relay subcommand.
func Run(args []string) {
	var showHelp bool

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}

	if showHelp {
		printRelayHelp()
		return
	}

	// For now, redirect to the main binary
	// In the future, this will contain the relay logic directly
	log.I.F("Relay not yet implemented via unified binary subcommand")
	log.I.F("Use the main binary: orly (in project root)")
	os.Exit(1)
}

func printRelayHelp() {
	fmt.Println(`orly relay - Main Nostr relay server

Usage:
  orly relay [options]
  orly [options]            (relay is the default command)

Options:
  --help, -h                Show this help message

The relay is the main Nostr server that:
  - Accepts WebSocket connections from clients
  - Processes EVENT, REQ, and other Nostr messages
  - Stores events in the database (direct or via gRPC)
  - Enforces ACL policies (direct or via gRPC)

Environment variables:
  ORLY_PORT                 Server port (default: 3334)
  ORLY_LOG_LEVEL            Logging level
  ORLY_DB_TYPE              Database type (badger, neo4j, grpc)
  ORLY_ACL_MODE             ACL mode (none, follows, managed)
  ORLY_TLS_DOMAINS          Let's Encrypt domains
  ORLY_AUTH_TO_WRITE        Require auth for writes

For split-mode deployment:
  ORLY_DB_TYPE=grpc         Use gRPC database server
  ORLY_DB_GRPC_SERVER       gRPC database server address
  ORLY_ACL_GRPC_SERVER      gRPC ACL server address

Example:
  orly                      Start the relay (default mode)
  orly relay                Start the relay (explicit)`)
}
