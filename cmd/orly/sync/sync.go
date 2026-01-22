//go:build !(js && wasm)

// Package sync implements the "orly sync" subcommand for sync service operations.
package sync

import (
	"fmt"
	"os"
	"strings"

	"lol.mleku.dev/log"
	pkgsync "next.orly.dev/pkg/sync"
)

// Run executes the sync subcommand.
func Run(args []string) {
	var driver string
	var listDrivers bool
	var showHelp bool

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
		}
	}

	if showHelp {
		printSyncHelp()
		return
	}

	if listDrivers {
		drivers := pkgsync.ListDriversWithInfo()
		if len(drivers) == 0 {
			fmt.Println("No sync drivers available.")
			fmt.Println("Build with appropriate tags to include drivers.")
			return
		}
		fmt.Println("Available sync drivers:")
		for _, d := range drivers {
			fmt.Printf("  %-12s - %s\n", d.Name, d.Description)
		}
		return
	}

	if driver == "" {
		// Check if any driver is registered
		drivers := pkgsync.ListDrivers()
		if len(drivers) == 0 {
			fmt.Fprintln(os.Stderr, "error: no sync drivers available")
			os.Exit(1)
		}
		if len(drivers) == 1 {
			// Use the only available driver
			driver = drivers[0]
			log.I.F("using default sync driver: %s", driver)
		} else {
			fmt.Fprintln(os.Stderr, "error: --driver required (multiple drivers available)")
			fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(drivers, ", "))
			os.Exit(1)
		}
	}

	// Check if driver is available
	if !pkgsync.HasDriver(driver) {
		fmt.Fprintf(os.Stderr, "error: sync driver %q not available\n", driver)
		fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(pkgsync.ListDrivers(), ", "))
		os.Exit(1)
	}

	runSyncService(driver, args)
}

func runSyncService(driver string, args []string) {
	log.I.F("Sync service with driver=%s not yet implemented via unified binary", driver)
	log.I.F("Use the standalone binary: orly-sync-%s", driver)
	os.Exit(1)
}

func printSyncHelp() {
	fmt.Println(`orly sync - Sync service operations

Usage:
  orly sync --driver=NAME [options]

Options:
  --driver=NAME      Select sync driver (negentropy, cluster, distributed)
  --list-drivers     List available sync drivers
  --help, -h         Show this help message

Drivers:
  negentropy   NIP-77 negentropy set reconciliation
  cluster      Cluster-based synchronization
  distributed  Distributed synchronization

Environment variables:
  ORLY_SYNC_LISTEN              gRPC server listen address
  ORLY_SYNC_LOG_LEVEL           Logging level
  ORLY_SYNC_DB_TYPE             Database type (grpc or badger)
  ORLY_SYNC_TARGET_RELAYS       Comma-separated target relay URLs

Examples:
  orly sync --driver=negentropy   Run negentropy sync service
  orly sync --list-drivers        List available drivers`)
}
