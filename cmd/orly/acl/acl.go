//go:build !(js && wasm)

// Package acl implements the "orly acl" subcommand for ACL server operations.
package acl

import (
	"fmt"
	"os"
	"strings"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
)

// Run executes the acl subcommand.
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
		printACLHelp()
		return
	}

	if listDrivers {
		drivers := acl.ListDriversWithInfo()
		if len(drivers) == 0 {
			fmt.Println("No ACL drivers available.")
			fmt.Println("Build with appropriate tags to include drivers.")
			return
		}
		fmt.Println("Available ACL drivers:")
		for _, d := range drivers {
			fmt.Printf("  %-10s - %s\n", d.Name, d.Description)
		}
		return
	}

	if driver == "" {
		// Check if any driver is registered
		drivers := acl.ListDrivers()
		if len(drivers) == 0 {
			fmt.Fprintln(os.Stderr, "error: no ACL drivers available")
			os.Exit(1)
		}
		if len(drivers) == 1 {
			// Use the only available driver
			driver = drivers[0]
			log.I.F("using default ACL driver: %s", driver)
		} else {
			fmt.Fprintln(os.Stderr, "error: --driver required (multiple drivers available)")
			fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(drivers, ", "))
			os.Exit(1)
		}
	}

	// Check if driver is available
	if !acl.HasDriver(driver) {
		fmt.Fprintf(os.Stderr, "error: ACL driver %q not available\n", driver)
		fmt.Fprintf(os.Stderr, "available: %s\n", strings.Join(acl.ListDrivers(), ", "))
		os.Exit(1)
	}

	runACLServer(driver, args)
}

func runACLServer(driver string, args []string) {
	log.I.F("ACL server with driver=%s not yet implemented via unified binary", driver)
	log.I.F("Use the standalone binary: orly-acl-%s", driver)
	os.Exit(1)
}

func printACLHelp() {
	fmt.Println(`orly acl - ACL server operations

Usage:
  orly acl --driver=NAME [options]

Options:
  --driver=NAME      Select ACL driver (follows, managed, curation)
  --list-drivers     List available ACL drivers
  --help, -h         Show this help message

Drivers:
  follows    Whitelist based on admin follow lists
  managed    NIP-86 fine-grained access control
  curation   Rate-limited trust tier system

Environment variables:
  ORLY_ACL_LISTEN               gRPC server listen address
  ORLY_ACL_LOG_LEVEL            Logging level
  ORLY_ACL_DB_TYPE              Database type (grpc or badger)
  ORLY_ACL_GRPC_DB_SERVER       gRPC database server address
  ORLY_OWNERS                   Comma-separated owner npubs
  ORLY_ADMINS                   Comma-separated admin npubs

Examples:
  orly acl --driver=follows     Run follows ACL server
  orly acl --list-drivers       List available drivers`)
}
