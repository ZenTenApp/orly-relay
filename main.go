package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"golang.org/x/term"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/branding"
	"next.orly.dev/app/config"
	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/relay"
	"next.orly.dev/pkg/version"
)

func main() {
	// Handle 'version' subcommand early, before any other initialization
	if config.VersionRequested() {
		fmt.Println(version.V)
		os.Exit(0)
	}

	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)

	// Handle 'init-branding' subcommand: create branding directory with default assets
	if requested, targetDir, style := config.InitBrandingRequested(); requested {
		if targetDir == "" {
			targetDir = filepath.Join(xdg.ConfigHome, cfg.AppName, "branding")
		}

		// Validate and convert style
		var brandingStyle branding.BrandingStyle
		switch style {
		case "orly":
			brandingStyle = branding.StyleORLY
		case "generic", "":
			brandingStyle = branding.StyleGeneric
		default:
			fmt.Fprintf(os.Stderr, "Unknown style: %s (use 'orly' or 'generic')\n", style)
			os.Exit(1)
		}

		fmt.Printf("Initializing %s branding kit at: %s\n", style, targetDir)
		if err := branding.InitBrandingKit(targetDir, app.GetEmbeddedWebFS(), brandingStyle); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\nBranding kit created successfully!")
		fmt.Println("\nFiles created:")
		fmt.Println("  branding.json     - Main configuration file")
		fmt.Println("  assets/           - Logo, favicon, and PWA icons")
		fmt.Println("  css/custom.css    - Full CSS override template")
		fmt.Println("  css/variables.css - CSS variables-only template")
		fmt.Println("\nEdit these files to customize your relay's appearance.")
		fmt.Println("Restart the relay to apply changes.")
		os.Exit(0)
	}

	// Handle 'identity' subcommand: print relay identity secret and pubkey and exit
	if config.IdentityRequested() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var db database.Database
		if db, err = database.NewDatabaseWithConfig(
			ctx, cancel, cfg.DBType, makeDatabaseConfig(cfg),
		); chk.E(err) {
			os.Exit(1)
		}
		defer db.Close()
		skb, err := db.GetOrCreateRelayIdentitySecret()
		if chk.E(err) {
			os.Exit(1)
		}
		pk, err := keys.SecretBytesToPubKeyHex(skb)
		if chk.E(err) {
			os.Exit(1)
		}
		fmt.Printf(
			"identity secret: %s\nidentity pubkey: %s\n", hex.Enc(skb), pk,
		)
		os.Exit(0)
	}

	// Handle 'migrate' subcommand: migrate data between database backends
	if requested, fromType, toType, targetPath := config.MigrateRequested(); requested {
		if fromType == "" || toType == "" {
			fmt.Println("Usage: orly migrate --from <type> --to <type> [--target-path <path>]")
			fmt.Println("")
			fmt.Println("Migrate data between database backends.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --from <type>         Source database type (badger, neo4j)")
			fmt.Println("  --to <type>           Destination database type (badger, neo4j)")
			fmt.Println("  --target-path <path>  Optional: destination data directory")
			fmt.Println("                        (default: $ORLY_DATA_DIR/<type>)")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  orly migrate --from badger --to neo4j")
			fmt.Println("  orly migrate --from badger --to neo4j --target-path /mnt/hdd/orly-neo4j")
			os.Exit(1)
		}

		// Set target path if not specified
		if targetPath == "" {
			targetPath = cfg.DataDir + "-" + toType
		}

		log.I.F("migrate: %s -> %s", fromType, toType)
		log.I.F("migrate: source path: %s", cfg.DataDir)
		log.I.F("migrate: target path: %s", targetPath)

		// Open source database
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		srcCfg := makeDatabaseConfig(cfg)
		var srcDB database.Database
		if srcDB, err = database.NewDatabaseWithConfig(ctx, cancel, fromType, srcCfg); chk.E(err) {
			log.E.F("migrate: failed to open source database: %v", err)
			os.Exit(1)
		}

		// Wait for source database to be ready
		select {
		case <-srcDB.Ready():
			log.I.F("migrate: source database ready")
		case <-time.After(60 * time.Second):
			log.E.F("migrate: timeout waiting for source database")
			os.Exit(1)
		}

		// Open destination database
		dstCfg := makeDatabaseConfig(cfg)
		dstCfg.DataDir = targetPath
		var dstDB database.Database
		if dstDB, err = database.NewDatabaseWithConfig(ctx, cancel, toType, dstCfg); chk.E(err) {
			log.E.F("migrate: failed to open destination database: %v", err)
			srcDB.Close()
			os.Exit(1)
		}

		// Wait for destination database to be ready
		select {
		case <-dstDB.Ready():
			log.I.F("migrate: destination database ready")
		case <-time.After(60 * time.Second):
			log.E.F("migrate: timeout waiting for destination database")
			srcDB.Close()
			os.Exit(1)
		}

		// Migrate using pipe (export from source, import to destination)
		log.I.F("migrate: starting data transfer...")
		pr, pw, pipeErr := os.Pipe()
		if pipeErr != nil {
			log.E.F("migrate: failed to create pipe: %v", pipeErr)
			srcDB.Close()
			dstDB.Close()
			os.Exit(1)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Export goroutine
		go func() {
			defer wg.Done()
			defer pw.Close()
			srcDB.Export(ctx, pw)
			log.I.F("migrate: export complete")
		}()

		// Import goroutine
		go func() {
			defer wg.Done()
			if importErr := dstDB.ImportEventsFromReader(ctx, pr); importErr != nil {
				log.E.F("migrate: import error: %v", importErr)
			}
			log.I.F("migrate: import complete")
		}()

		wg.Wait()

		// Sync and close databases
		if err = dstDB.Sync(); chk.E(err) {
			log.W.F("migrate: sync warning: %v", err)
		}
		srcDB.Close()
		dstDB.Close()

		log.I.F("migrate: migration complete!")
		os.Exit(0)
	}

	// Handle 'nrc' subcommand: NRC (Nostr Relay Connect) utilities
	if requested, subcommand, args := config.NRCRequested(); requested {
		handleNRCCommand(cfg, subcommand, args)
		os.Exit(0)
	}

	// Handle 'serve' subcommand: start ephemeral relay with RAM-based storage
	if config.ServeRequested() {
		const serveDataDir = "/dev/shm/orlyserve"
		log.I.F("serve mode: configuring ephemeral relay at %s", serveDataDir)

		// Delete existing directory completely
		if err = os.RemoveAll(serveDataDir); err != nil && !os.IsNotExist(err) {
			log.E.F("failed to remove existing serve directory: %v", err)
			os.Exit(1)
		}

		// Create fresh directory
		if err = os.MkdirAll(serveDataDir, 0755); chk.E(err) {
			log.E.F("failed to create serve directory: %v", err)
			os.Exit(1)
		}

		// Override configuration for serve mode
		cfg.DataDir = serveDataDir
		cfg.Listen = "0.0.0.0"
		cfg.Port = 10547
		cfg.ACLMode = "none"
		cfg.ServeMode = true // Grant full owner access to all users

		log.I.F("serve mode: listening on %s:%d with ACL mode '%s' (full owner access)",
			cfg.Listen, cfg.Port, cfg.ACLMode)
	}

	// Handle 'curatingmode' subcommand: start relay in curating mode with specified owner
	if requested, ownerKey := config.CuratingModeRequested(); requested {
		if ownerKey == "" {
			fmt.Println("Usage: orly curatingmode <npub|hex_pubkey>")
			fmt.Println("")
			fmt.Println("Starts the relay in curating mode with the specified pubkey as owner.")
			fmt.Println("Opens a browser to the curation setup page where you must log in")
			fmt.Println("with a Nostr extension to configure the relay.")
			fmt.Println("")
			fmt.Println("Press Escape or Ctrl+C to stop the relay.")
			os.Exit(1)
		}

		// Parse the owner key (npub or hex)
		var ownerHex string
		if strings.HasPrefix(ownerKey, "npub1") {
			// Decode npub to hex
			_, pubBytes, err := bech32encoding.Decode([]byte(ownerKey))
			if err != nil {
				fmt.Printf("Error: invalid npub: %v\n", err)
				os.Exit(1)
			}
			if pb, ok := pubBytes.([]byte); ok {
				ownerHex = hex.Enc(pb)
			} else {
				fmt.Println("Error: invalid npub encoding")
				os.Exit(1)
			}
		} else if len(ownerKey) == 64 {
			// Assume hex pubkey
			ownerHex = strings.ToLower(ownerKey)
		} else {
			fmt.Println("Error: owner key must be an npub or 64-character hex pubkey")
			os.Exit(1)
		}

		// Configure for curating mode
		cfg.ACLMode = "curating"
		cfg.Owners = []string{ownerHex}

		log.I.F("curatingmode: starting with owner %s", ownerHex)
		log.I.F("curatingmode: listening on %s:%d", cfg.Listen, cfg.Port)

		// Start a goroutine to open browser after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			url := fmt.Sprintf("http://%s:%d/#curation", cfg.Listen, cfg.Port)
			log.I.F("curatingmode: opening browser to %s", url)
			openBrowser(url)
		}()

		// Start a goroutine to listen for Escape key
		go func() {
			// Set terminal to raw mode to capture individual key presses
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				log.W.F("could not set terminal to raw mode: %v", err)
				return
			}
			defer term.Restore(int(os.Stdin.Fd()), oldState)

			buf := make([]byte, 1)
			for {
				_, err := os.Stdin.Read(buf)
				if err != nil {
					return
				}
				// Escape key is 0x1b (27)
				if buf[0] == 0x1b {
					fmt.Println("\nEscape pressed, shutting down...")
					p, _ := os.FindProcess(os.Getpid())
					_ = p.Signal(os.Interrupt)
					return
				}
			}
		}()

		fmt.Println("")
		fmt.Println("Curating Mode Setup")
		fmt.Println("===================")
		fmt.Printf("Owner: %s\n", ownerHex)
		fmt.Printf("URL:   http://%s:%d/#curation\n", cfg.Listen, cfg.Port)
		fmt.Println("")
		fmt.Println("Log in with your Nostr extension to configure allowed event kinds")
		fmt.Println("and rate limiting settings.")
		fmt.Println("")
		fmt.Println("Press Escape or Ctrl+C to stop the relay.")
		fmt.Println("")
	}

	// Start the relay using shared startup logic
	if err := relay.RunWithSignals(cfg); err != nil {
		log.F.F("relay error: %v", err)
	}
}

// makeDatabaseConfig creates a database.DatabaseConfig from the app config.
// Delegates to the shared relay package implementation.
func makeDatabaseConfig(cfg *config.C) *database.DatabaseConfig {
	return relay.MakeDatabaseConfig(cfg)
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.W.F("could not open browser: %v", err)
	}
}

// handleNRCCommand handles the 'nrc' CLI subcommand for NRC (Nostr Relay Connect) utilities.
func handleNRCCommand(cfg *config.C, subcommand string, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch subcommand {
	case "generate":
		handleNRCGenerate(ctx, cfg, args)
	case "list":
		handleNRCList(cfg)
	case "revoke":
		handleNRCRevoke(args)
	default:
		printNRCUsage()
	}
}

// printNRCUsage prints the usage information for the nrc subcommand.
func printNRCUsage() {
	fmt.Println("Usage: orly nrc <subcommand> [options]")
	fmt.Println("")
	fmt.Println("Nostr Relay Connect (NRC) utilities for private relay access.")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  generate [--name <device>]  Generate a new connection URI")
	fmt.Println("  list                        List currently configured authorized secrets")
	fmt.Println("  revoke <name>               Revoke access for a device (show instructions)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  orly nrc generate")
	fmt.Println("  orly nrc generate --name phone")
	fmt.Println("  orly nrc list")
	fmt.Println("  orly nrc revoke phone")
	fmt.Println("")
	fmt.Println("To enable NRC, set these environment variables:")
	fmt.Println("  ORLY_NRC_ENABLED=true")
	fmt.Println("  ORLY_NRC_RENDEZVOUS_URL=wss://public-relay.example.com")
	fmt.Println("  ORLY_NRC_AUTHORIZED_KEYS=<secret1>:<name1>,<secret2>:<name2>")
}

// handleNRCGenerate generates a new NRC connection URI.
func handleNRCGenerate(ctx context.Context, cfg *config.C, args []string) {
	// Parse device name from args
	var deviceName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			deviceName = args[i+1]
			i++
		}
	}

	// Get relay identity
	var db database.Database
	var err error
	if db, err = database.NewDatabaseWithConfig(
		ctx, nil, cfg.DBType, makeDatabaseConfig(cfg),
	); chk.E(err) {
		fmt.Printf("Error: failed to open database: %v\n", err)
		return
	}
	defer db.Close()

	<-db.Ready()

	relaySecretKey, err := db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		fmt.Printf("Error: failed to get relay identity: %v\n", err)
		return
	}

	relayPubkey, err := keys.SecretBytesToPubKeyBytes(relaySecretKey)
	if err != nil {
		fmt.Printf("Error: failed to derive relay pubkey: %v\n", err)
		return
	}

	// Get rendezvous URL from config
	nrcEnabled, nrcRendezvousURL, _, _ := cfg.GetNRCConfigValues()
	if !nrcEnabled || nrcRendezvousURL == "" {
		fmt.Println("Error: NRC is not configured. Set ORLY_NRC_ENABLED=true and ORLY_NRC_RENDEZVOUS_URL")
		return
	}

	// Generate a new random secret
	secret := make([]byte, 32)
	if _, err := os.ReadFile("/dev/urandom"); err != nil {
		// Fallback - use crypto/rand
		fmt.Printf("Error: failed to generate random secret: %v\n", err)
		return
	}
	f, _ := os.Open("/dev/urandom")
	defer f.Close()
	f.Read(secret)

	secretHex := hex.Enc(secret)

	// Build the URI
	uri := fmt.Sprintf("nostr+relayconnect://%s?relay=%s&secret=%s",
		hex.Enc(relayPubkey), nrcRendezvousURL, secretHex)
	if deviceName != "" {
		uri += fmt.Sprintf("&name=%s", deviceName)
	}

	fmt.Println("Generated NRC Connection URI:")
	fmt.Println("")
	fmt.Println(uri)
	fmt.Println("")
	fmt.Println("Add this secret to ORLY_NRC_AUTHORIZED_KEYS:")
	if deviceName != "" {
		fmt.Printf("  %s:%s\n", secretHex, deviceName)
	} else {
		fmt.Printf("  %s\n", secretHex)
	}
	fmt.Println("")
	fmt.Println("IMPORTANT: Store this URI securely - anyone with this URI can access your relay.")
}

// handleNRCList lists configured authorized secrets from environment.
func handleNRCList(cfg *config.C) {
	_, _, authorizedKeys, _ := cfg.GetNRCConfigValues()

	fmt.Println("NRC Configuration:")
	fmt.Println("")

	if len(authorizedKeys) == 0 {
		fmt.Println("  No authorized secrets configured.")
		fmt.Println("")
		fmt.Println("  To add secrets, set ORLY_NRC_AUTHORIZED_KEYS=<secret>:<name>,...")
	} else {
		fmt.Printf("  Authorized secrets: %d\n", len(authorizedKeys))
		fmt.Println("")
		for _, entry := range authorizedKeys {
			parts := strings.SplitN(entry, ":", 2)
			secretHex := parts[0]
			name := "(unnamed)"
			if len(parts) == 2 && parts[1] != "" {
				name = parts[1]
			}
			// Show truncated secret for identification
			truncated := secretHex
			if len(secretHex) > 16 {
				truncated = secretHex[:8] + "..." + secretHex[len(secretHex)-8:]
			}
			fmt.Printf("  - %s: %s\n", name, truncated)
		}
	}
}

// handleNRCRevoke provides instructions for revoking access.
func handleNRCRevoke(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: orly nrc revoke <device-name>")
		fmt.Println("")
		fmt.Println("To revoke access for a device:")
		fmt.Println("1. Remove the corresponding secret from ORLY_NRC_AUTHORIZED_KEYS")
		fmt.Println("2. Restart the relay")
		fmt.Println("")
		fmt.Println("Example: If ORLY_NRC_AUTHORIZED_KEYS=\"abc123:phone,def456:laptop\"")
		fmt.Println("To revoke 'phone', change to: ORLY_NRC_AUTHORIZED_KEYS=\"def456:laptop\"")
		return
	}

	deviceName := args[0]
	fmt.Printf("To revoke access for '%s':\n", deviceName)
	fmt.Println("")
	fmt.Println("1. Edit ORLY_NRC_AUTHORIZED_KEYS and remove the entry for this device")
	fmt.Println("2. Restart the relay")
	fmt.Println("")
	fmt.Println("The device will no longer be able to connect after the restart.")
}
