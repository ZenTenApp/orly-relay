//go:build !(js && wasm)

// Package launcher implements the "orly launcher" subcommand for process supervision.
package launcher

import (
	"fmt"
	"os"

	"lol.mleku.dev/log"
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

	// For now, redirect to standalone binary
	log.I.F("Launcher not yet implemented via unified binary")
	log.I.F("Use the standalone binary: orly-launcher")
	os.Exit(1)
}

func printLauncherHelp() {
	fmt.Println(`orly launcher - Process supervisor

Usage:
  orly launcher [options]

Options:
  --help, -h         Show this help message

The launcher supervises the split-mode deployment:
  - Starts and monitors orly-db (database server)
  - Starts and monitors orly-acl (ACL server)
  - Starts and monitors orly (main relay)
  - Handles graceful shutdown and restart

Environment variables:
  ORLY_LAUNCHER_DB_ENABLED      Enable database subprocess
  ORLY_LAUNCHER_DB_LISTEN       Database server listen address
  ORLY_LAUNCHER_ACL_ENABLED     Enable ACL subprocess
  ORLY_LAUNCHER_ACL_LISTEN      ACL server listen address
  ORLY_LAUNCHER_ACL_MODE        ACL mode (follows, managed, curation)

Example:
  orly launcher                 Start process supervisor`)
}
