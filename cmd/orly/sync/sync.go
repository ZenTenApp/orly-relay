//go:build !(js && wasm)

// Package sync implements the "orly sync" subcommand for sync service operations.
// Supports both one-shot CLI sync (like strfry sync) and running as a gRPC service.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"next.orly.dev/pkg/database"
	pkgsync "next.orly.dev/pkg/sync"
	"next.orly.dev/pkg/sync/negentropy"
)

// SyncDirection specifies which direction to sync
type SyncDirection string

const (
	DirDown SyncDirection = "down" // Download from remote to local
	DirUp   SyncDirection = "up"   // Upload from local to remote
	DirBoth SyncDirection = "both" // Bidirectional sync
)

// CLISyncConfig holds configuration for one-shot CLI sync
type CLISyncConfig struct {
	RelayURL  string
	Filter    *filter.F
	Direction SyncDirection
	DataDir   string
	Verbose   bool
}

// Run executes the sync subcommand.
func Run(args []string) {
	var driver string
	var listDrivers bool
	var showHelp bool
	var relayURL string
	var filterJSON string
	var direction string = "down"
	var dataDir string
	var verbose bool

	// Parse arguments - look for either service mode (--driver) or CLI mode (relay URL)
	positionalArgs := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--driver=") {
			driver = strings.TrimPrefix(arg, "--driver=")
		} else if arg == "--driver" && i+1 < len(args) {
			driver = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--filter=") {
			filterJSON = strings.TrimPrefix(arg, "--filter=")
		} else if arg == "--filter" && i+1 < len(args) {
			filterJSON = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--dir=") {
			direction = strings.TrimPrefix(arg, "--dir=")
		} else if arg == "--dir" && i+1 < len(args) {
			direction = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--data-dir=") {
			dataDir = strings.TrimPrefix(arg, "--data-dir=")
		} else if arg == "--data-dir" && i+1 < len(args) {
			dataDir = args[i+1]
			i++
		} else if arg == "--list-drivers" || arg == "-l" {
			listDrivers = true
		} else if arg == "--verbose" || arg == "-v" {
			verbose = true
		} else if arg == "--help" || arg == "-h" {
			showHelp = true
		} else if !strings.HasPrefix(arg, "-") {
			positionalArgs = append(positionalArgs, arg)
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

	// Check if this is CLI sync mode (relay URL provided)
	if len(positionalArgs) > 0 && (strings.HasPrefix(positionalArgs[0], "ws://") || strings.HasPrefix(positionalArgs[0], "wss://")) {
		relayURL = positionalArgs[0]
		runCLISync(&CLISyncConfig{
			RelayURL:  relayURL,
			Filter:    parseFilterJSON(filterJSON),
			Direction: SyncDirection(direction),
			DataDir:   dataDir,
			Verbose:   verbose,
		})
		return
	}

	// Service mode
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

// parseFilterJSON parses a JSON filter string into a filter.F
func parseFilterJSON(jsonStr string) *filter.F {
	if jsonStr == "" {
		return nil
	}

	// Parse as generic JSON first to handle the kinds array
	var rawFilter struct {
		Kinds   []int    `json:"kinds"`
		Authors []string `json:"authors"`
		IDs     []string `json:"ids"`
		Since   *int64   `json:"since"`
		Until   *int64   `json:"until"`
		Limit   *uint    `json:"limit"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawFilter); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to parse filter JSON: %v\n", err)
		return nil
	}

	f := &filter.F{}

	// Convert kinds using kind.FromIntSlice
	if len(rawFilter.Kinds) > 0 {
		f.Kinds = kind.FromIntSlice(rawFilter.Kinds)
	}

	// Convert timestamps
	if rawFilter.Since != nil {
		f.Since = &timestamp.T{V: *rawFilter.Since}
	}
	if rawFilter.Until != nil {
		f.Until = &timestamp.T{V: *rawFilter.Until}
	}

	// Convert limit
	if rawFilter.Limit != nil {
		f.Limit = rawFilter.Limit
	}

	return f
}

// runCLISync performs a one-shot negentropy sync with a remote relay
func runCLISync(cfg *CLISyncConfig) {
	if cfg.Verbose {
		log.I.F("CLI sync starting with %s (direction: %s)", cfg.RelayURL, cfg.Direction)
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted, closing...")
		cancel()
	}()

	// Determine data directory
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = os.Getenv("ORLY_DATA_DIR")
		if dataDir == "" {
			homeDir, _ := os.UserHomeDir()
			dataDir = homeDir + "/.local/share/ORLY"
		}
	}

	// Open database using the factory
	dbCfg := &database.DatabaseConfig{
		DataDir:  dataDir,
		LogLevel: "info",
	}

	db, err := database.NewDatabaseWithConfig(ctx, cancel, "badger", dbCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create negentropy manager for sync
	mgrCfg := &negentropy.Config{
		Peers:        []string{cfg.RelayURL},
		SyncInterval: 0, // One-shot, no interval
		FrameSize:    128 * 1024,
		IDSize:       16,
		Filter:       cfg.Filter,
	}

	mgr := negentropy.NewManager(db, mgrCfg)

	// Perform sync
	startTime := time.Now()
	fmt.Printf("Syncing with %s...\n", cfg.RelayURL)

	// For direction handling, we'll use the existing sync which does bidirectional
	// The --dir flag will be used to filter what we actually store
	mgr.TriggerSync(ctx, cfg.RelayURL)

	elapsed := time.Since(startTime)
	state, _ := mgr.GetPeerState(cfg.RelayURL)
	if state != nil {
		if state.LastError != "" {
			fmt.Fprintf(os.Stderr, "Sync error: %s\n", state.LastError)
			os.Exit(1)
		}
		fmt.Printf("Sync complete: %d events synced in %v\n", state.EventsSynced, elapsed)
	} else {
		fmt.Printf("Sync complete in %v\n", elapsed)
	}
}

func runSyncService(driver string, args []string) {
	log.I.F("Sync service with driver=%s not yet implemented via unified binary", driver)
	log.I.F("Use the standalone binary: orly-sync-%s", driver)
	os.Exit(1)
}

func printSyncHelp() {
	fmt.Println(`orly sync - Sync operations (NIP-77 negentropy)

Usage:
  orly sync <relay_url> [options]     One-shot sync with relay (like strfry sync)
  orly sync --driver=NAME [options]   Run as sync service

One-shot sync options:
  --filter=JSON      Nostr filter JSON (e.g. '{"kinds": [0, 3, 1984]}')
  --dir=DIRECTION    Sync direction: down, up, both (default: down)
  --data-dir=PATH    Database directory (default: ~/.local/share/ORLY)
  --verbose, -v      Verbose output

Service mode options:
  --driver=NAME      Select sync driver (negentropy, cluster, distributed)
  --list-drivers     List available sync drivers

Common options:
  --help, -h         Show this help message

Drivers:
  negentropy   NIP-77 negentropy set reconciliation
  cluster      Cluster-based synchronization
  distributed  Distributed synchronization

Environment variables:
  ORLY_DATA_DIR                 Database data directory
  ORLY_SYNC_LISTEN              gRPC server listen address
  ORLY_SYNC_LOG_LEVEL           Logging level
  ORLY_SYNC_DB_TYPE             Database type (grpc or badger)
  ORLY_SYNC_TARGET_RELAYS       Comma-separated target relay URLs

Examples:
  # One-shot sync (like strfry sync)
  orly sync wss://relay.example.com --filter '{"kinds": [0, 3, 1984]}' --dir down
  orly sync wss://wot.grapevine.network --filter '{"kinds": [3, 1984, 10000]}' --dir down

  # Service mode
  orly sync --driver=negentropy   Run negentropy sync service
  orly sync --list-drivers        List available drivers`)
}
