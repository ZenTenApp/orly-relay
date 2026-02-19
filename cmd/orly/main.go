// orly is a unified binary for the ORLY Nostr relay system.
// It provides subcommands for running database servers, ACL servers,
// sync services, and the main relay.
//
// Usage:
//
//	orly [command] [options]
//
// Commands:
//
//	db      - Database server (requires --driver flag)
//	acl     - ACL server (requires --driver flag)
//	sync    - Sync service (requires --driver flag)
//	launcher - Process supervisor
//	relay   - Main relay (default if no command specified)
//	version - Show version information
//	help    - Show help
//
// Examples:
//
//	orly                        # Run the main relay (default)
//	orly relay                  # Run the main relay explicitly
//	orly db --driver=badger     # Run Badger database server
//	orly db --list-drivers      # List available database drivers
//	orly db health              # Run database health check
//	orly db repair              # Repair database issues
//	orly db repair --dry-run    # Preview repairs without applying
//	orly acl --driver=follows   # Run follows ACL server
//	orly sync --driver=negentropy # Run negentropy sync service
//	orly launcher               # Run process supervisor
package main

import (
	"fmt"
	"os"

	"next.orly.dev/cmd/orly/acl"
	"next.orly.dev/cmd/orly/bridge"
	"next.orly.dev/cmd/orly/db"
	"next.orly.dev/cmd/orly/launcher"
	"next.orly.dev/cmd/orly/relay"
	"next.orly.dev/cmd/orly/sync"
)

// Version information (set by build flags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		// Default: run the relay
		relay.Run(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "bridge":
		bridge.Run(os.Args[2:])
	case "db":
		db.Run(os.Args[2:])
	case "acl":
		acl.Run(os.Args[2:])
	case "sync":
		sync.Run(os.Args[2:])
	case "launcher":
		launcher.Run(os.Args[2:])
	case "relay":
		relay.Run(os.Args[2:])
	case "version", "-v", "--version":
		printVersion()
	case "help", "-h", "--help":
		printHelp()
	default:
		// Check if it's a flag (starts with -)
		if os.Args[1][0] == '-' {
			// Treat as relay arguments
			relay.Run(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	}
}

func printVersion() {
	fmt.Printf("orly %s\n", Version)
	fmt.Printf("  commit:  %s\n", Commit)
	fmt.Printf("  built:   %s\n", BuildDate)
}

func printHelp() {
	fmt.Println(`orly - ORLY Nostr Relay System

Usage:
  orly [command] [options]

Commands:
  bridge    Nostr-Email bridge (Marmot DM to SMTP)

  db        Database server operations
            --driver=NAME    Select database driver (badger, neo4j)
            --list-drivers   List available database drivers
            health           Run database health check
            repair           Repair database issues (--dry-run for preview)

  acl       ACL server operations
            --driver=NAME    Select ACL driver (follows, managed, curation)
            --list-drivers   List available ACL drivers

  sync      Sync service operations
            --driver=NAME    Select sync driver (negentropy, cluster, distributed)
            --list-drivers   List available sync drivers

  launcher  Process supervisor for split-mode deployment

  relay     Main relay server (default if no command given)

  version   Show version information
  help      Show this help message

Examples:
  orly                            Run the main relay (default)
  orly db --driver=badger         Run Badger database server
  orly db health                  Check database health
  orly db repair --dry-run        Preview database repairs
  orly acl --driver=follows       Run follows ACL server
  orly sync --driver=negentropy   Run negentropy sync service
  orly launcher                   Run process supervisor

Environment variables:
  ORLY_DATA_DIR              Database data directory
  ORLY_DB_LISTEN             Database server listen address
  ORLY_ACL_LISTEN            ACL server listen address
  ORLY_LOG_LEVEL             Logging level (trace, debug, info, warn, error)
  ORLY_BRIDGE_ENABLED        Enable bridge in monolithic mode
  ORLY_BRIDGE_DOMAIN         Email domain for the bridge
  ORLY_BRIDGE_RELAY_URL      Relay WebSocket URL (standalone mode)

See 'orly [command] --help' for command-specific options.`)
}
