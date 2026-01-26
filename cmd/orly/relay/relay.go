//go:build !(js && wasm)

// Package relay implements the "orly relay" subcommand (the default command).
package relay

import (
	"fmt"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	relaycore "next.orly.dev/pkg/relay"
	"next.orly.dev/pkg/version"
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

	// Load configuration
	cfg, err := config.New()
	if chk.T(err) {
		return
	}
	log.I.F("starting %s %s (unified binary)", cfg.AppName, version.V)

	// Use shared relay startup logic
	if err := relaycore.RunWithSignals(cfg); err != nil {
		log.F.F("relay error: %v", err)
	}
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
  ORLY_DB_TYPE              Database type: badger, neo4j, grpc (default: badger)
  ORLY_ACL_MODE             ACL mode: none, follows, managed, curating (default: none)
  ORLY_TLS_DOMAINS          Let's Encrypt domains
  ORLY_AUTH_TO_WRITE        Require auth for writes

gRPC Backend Configuration (for split-mode deployment):
  ORLY_DB_TYPE=grpc              Use remote gRPC database server
  ORLY_GRPC_SERVER               Database server address (default: 127.0.0.1:50051)

  ORLY_ACL_TYPE=grpc             Use remote gRPC ACL server
  ORLY_GRPC_ACL_SERVER           ACL server address (default: 127.0.0.1:50052)

  ORLY_SYNC_TYPE=grpc            Use remote gRPC sync services
  ORLY_GRPC_SYNC_NEGENTROPY      Negentropy server address (default: 127.0.0.1:50056)
  ORLY_GRPC_SYNC_DISTRIBUTED     Distributed sync address (default: 127.0.0.1:50053)
  ORLY_GRPC_SYNC_CLUSTER         Cluster sync address (default: 127.0.0.1:50054)

Examples:
  orly                                    Start relay (monolithic mode)
  orly relay                              Start relay (explicit)
  ORLY_DB_TYPE=grpc orly                  Connect to gRPC database at 127.0.0.1:50051
  ORLY_ACL_TYPE=grpc orly                 Connect to gRPC ACL at 127.0.0.1:50052
  ORLY_DB_TYPE=grpc ORLY_ACL_TYPE=grpc orly   Full split mode`)
}
