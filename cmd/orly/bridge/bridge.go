//go:build !(js && wasm)

// Package bridge implements the "orly bridge" subcommand for the Nostr-Email bridge.
package bridge

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/app/config"
	bridgepkg "next.orly.dev/pkg/bridge"
	"next.orly.dev/pkg/version"
)

// Run executes the bridge subcommand.
func Run(args []string) {
	var showHelp bool

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			showHelp = true
		}
	}

	if showHelp {
		printBridgeHelp()
		return
	}

	// Load configuration
	cfg, err := config.New()
	if chk.T(err) {
		return
	}
	log.I.F("starting bridge %s", version.V)

	// Extract bridge-specific config
	enabled, domain, nsec, relayURL, smtpPort, smtpHost,
		dataDir, dkimKeyPath, dkimSelector, nwcURI,
		monthlyPriceSats, composeURL,
		smtpRelayHost, smtpRelayPort, smtpRelayUsername, smtpRelayPassword,
		aclGRPCServer, aliasPriceSats, profilePath := cfg.GetBridgeConfigValues()

	if !enabled && relayURL == "" {
		// When run as a subcommand, enable by default even if ORLY_BRIDGE_ENABLED is false
		// (the subcommand is the explicit opt-in). But we still need a relay URL in standalone mode.
		log.W.F("no ORLY_BRIDGE_RELAY_URL configured — bridge needs a relay to connect to")
	}

	bridgeCfg := &bridgepkg.Config{
		Domain:            domain,
		NSEC:              nsec,
		RelayURL:          relayURL,
		SMTPPort:          smtpPort,
		SMTPHost:          smtpHost,
		DataDir:           dataDir,
		DKIMKeyPath:       dkimKeyPath,
		DKIMSelector:      dkimSelector,
		NWCURI:            nwcURI,
		MonthlyPriceSats:  monthlyPriceSats,
		ComposeURL:        composeURL,
		SMTPRelayHost:     smtpRelayHost,
		SMTPRelayPort:     smtpRelayPort,
		SMTPRelayUsername: smtpRelayUsername,
		SMTPRelayPassword: smtpRelayPassword,
		ACLGRPCServer:     aclGRPCServer,
		AliasPriceSats:    aliasPriceSats,
		ProfilePath:       profilePath,
	}

	// In standalone subcommand mode, no database getter is available.
	// Identity falls back to NSEC env var or file.
	b := bridgepkg.New(bridgeCfg, nil)

	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := b.Start(ctx); err != nil {
		log.E.F("bridge start failed: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	b.Stop()
}

func printBridgeHelp() {
	fmt.Println(`orly bridge - Nostr-Email Bridge (Marmot)

Usage:
  orly bridge [options]

The bridge connects Nostr DMs (via Marmot/MLS encryption) to email (via SMTP).
Users DM the bridge's npub to send email; the bridge delivers incoming email
as encrypted DMs.

Deployment Modes:
  STANDALONE  Run the bridge as a separate process connecting to any relay
              via WebSocket. Set ORLY_BRIDGE_RELAY_URL to the relay's ws:// URL.
              Identity is read from ORLY_BRIDGE_NSEC or auto-generated to file.

  MONOLITHIC  Run as part of the ORLY relay (ORLY_BRIDGE_ENABLED=true).
              Identity is shared with the relay (read from database).
              No separate subcommand needed — the relay starts the bridge.

  SPLIT IPC   Managed by 'orly launcher'. The launcher reads relay identity
              from the database and injects it via ORLY_BRIDGE_NSEC.

Environment Variables:
  ORLY_BRIDGE_ENABLED            Enable bridge in monolithic mode (default: false)
  ORLY_BRIDGE_DOMAIN             Email domain (e.g., relay.example.com)
  ORLY_BRIDGE_NSEC               Bridge identity nsec or hex secret key
  ORLY_BRIDGE_RELAY_URL          WebSocket relay URL for standalone mode
  ORLY_BRIDGE_SMTP_PORT          SMTP server port (default: 2525)
  ORLY_BRIDGE_SMTP_HOST          SMTP server bind address (default: 0.0.0.0)
  ORLY_BRIDGE_DATA_DIR           Bridge data directory
  ORLY_BRIDGE_DKIM_KEY           Path to DKIM private key PEM file
  ORLY_BRIDGE_DKIM_SELECTOR      DKIM selector (default: marmot)
  ORLY_BRIDGE_NWC_URI            NWC connection string for subscriptions
  ORLY_BRIDGE_MONTHLY_PRICE_SATS Monthly subscription price in sats (default: 2100)
  ORLY_BRIDGE_COMPOSE_URL        Public URL of the compose form
  ORLY_BRIDGE_SMTP_RELAY_HOST    SMTP smarthost (e.g., smtp.migadu.com)
  ORLY_BRIDGE_SMTP_RELAY_PORT    SMTP smarthost port (default: 587)
  ORLY_BRIDGE_SMTP_RELAY_USERNAME SMTP smarthost AUTH username
  ORLY_BRIDGE_SMTP_RELAY_PASSWORD SMTP smarthost AUTH password
  ORLY_BRIDGE_ACL_GRPC_SERVER    ACL gRPC server address for paid subscriptions
  ORLY_BRIDGE_ALIAS_PRICE_SATS   Monthly alias email price in sats (default: 4200)
  ORLY_BRIDGE_PROFILE            Path to profile template file (default: $DATA_DIR/profile.txt)

Examples:
  # Standalone: connect to an external relay
  ORLY_BRIDGE_RELAY_URL=wss://relay.example.com \
  ORLY_BRIDGE_DOMAIN=mail.example.com \
    orly bridge

  # With identity from environment
  ORLY_BRIDGE_NSEC=nsec1... \
  ORLY_BRIDGE_RELAY_URL=wss://relay.example.com \
    orly bridge`)
}
