package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/lol/log"
)

func main() {
	dbPath := flag.String("db", "", "Path to badger database directory")
	flag.Parse()

	if *dbPath == "" {
		log.E.F("Usage: reindex-ppg -db /path/to/database")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.I.F("Received shutdown signal, canceling...")
		cancel()
	}()

	log.I.F("Opening database at %s", *dbPath)
	db, err := database.New(ctx, cancel, *dbPath, "info")
	if err != nil {
		log.E.F("Failed to open database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	log.I.F("Starting PPG/GPP index rebuild...")
	db.BackfillPubkeyPubkeyGraph()
	log.I.F("PPG/GPP index rebuild complete!")
}
