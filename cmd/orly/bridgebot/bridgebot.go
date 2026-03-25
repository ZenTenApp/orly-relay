//go:build !(js && wasm)

// Package bridgebot implements the "orly bridgebot" subcommand.
// It runs a standalone MLS DM bot that processes subscription commands.
package bridgebot

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"next.orly.dev/app"
	"next.orly.dev/pkg/lol/log"
)

// Run executes the bridgebot subcommand.
func Run(args []string) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printHelp()
			return
		}
	}

	relayURL := os.Getenv("ORLY_BRIDGE_BOT_RELAY")
	if relayURL == "" {
		log.E.F("ORLY_BRIDGE_BOT_RELAY is required")
		os.Exit(1)
	}

	freeMode := os.Getenv("ORLY_BRIDGE_BOT_FREE") == "true"
	dataDir := os.Getenv("ORLY_BRIDGE_BOT_DATA_DIR")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	bot, err := app.NewBridgeBot(ctx, relayURL, freeMode, dataDir)
	if err != nil {
		log.E.F("bridge bot init failed: %v", err)
		os.Exit(1)
	}

	if err := bot.Start(ctx); err != nil {
		log.E.F("bridge bot start failed: %v", err)
		os.Exit(1)
	}

	mode := "paid"
	if freeMode {
		mode = "free"
	}
	log.I.F("bridge bot running (pubkey=%s, relay=%s, mode=%s)", bot.PubkeyHex(), relayURL, mode)

	<-ctx.Done()
	bot.Stop()
}

func printHelp() {
	fmt.Println(`orly bridgebot - MLS DM subscription bot

Usage:
  orly bridgebot [options]

The bridge bot listens for MLS-encrypted DMs and processes subscription
commands (status, subscribe, subscribe <alias>).

Environment Variables:
  ORLY_BRIDGE_BOT_RELAY    Relay WebSocket URL (required, e.g. ws://localhost:3334)
  ORLY_BRIDGE_BOT_FREE     Set to "true" to activate subscriptions without payment (default: false)
  ORLY_BRIDGE_BOT_DATA_DIR Directory to persist bot keypair (default: empty = ephemeral)
  ORLY_BRIDGE_BOT_NSEC     Hex secret key override (highest priority)

Examples:
  ORLY_BRIDGE_BOT_RELAY=ws://localhost:3334 orly bridgebot
  ORLY_BRIDGE_BOT_RELAY=ws://localhost:3334 ORLY_BRIDGE_BOT_FREE=true orly bridgebot`)
}
