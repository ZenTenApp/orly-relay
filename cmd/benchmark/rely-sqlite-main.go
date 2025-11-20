//go:build ignore
// +build ignore

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nbd-wtf/go-nostr"
	sqlite "github.com/vertex-lab/nostr-sqlite"
	"github.com/pippellia-btc/rely"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Get configuration from environment with defaults
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./relay.db"
	}

	listenAddr := os.Getenv("RELAY_LISTEN")
	if listenAddr == "" {
		listenAddr = "0.0.0.0:3334"
	}

	// Initialize database
	db, err := sqlite.New(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create relay with handlers
	relay := rely.NewRelay(
		rely.WithQueueCapacity(10_000),
		rely.WithMaxProcessors(10),
	)

	// Register event handlers using the correct API
	relay.On.Event = Save(db)
	relay.On.Req = Query(db)
	relay.On.Count = Count(db)

	// Start relay
	log.Printf("Starting rely-sqlite on %s with database %s", listenAddr, dbPath)
	err = relay.StartAndServe(ctx, listenAddr)
	if err != nil {
		log.Fatalf("relay failed: %v", err)
	}
}

// Save handles incoming events
func Save(db *sqlite.Store) func(_ rely.Client, e *nostr.Event) error {
	return func(_ rely.Client, e *nostr.Event) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		switch {
		case nostr.IsRegularKind(e.Kind):
			_, err := db.Save(ctx, e)
			return err
		case nostr.IsReplaceableKind(e.Kind) || nostr.IsAddressableKind(e.Kind):
			_, err := db.Replace(ctx, e)
			return err
		default:
			return nil
		}
	}
}

// Query retrieves events matching filters
func Query(db *sqlite.Store) func(ctx context.Context, _ rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	return func(ctx context.Context, _ rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return db.Query(ctx, filters...)
	}
}

// Count counts events matching filters
func Count(db *sqlite.Store) func(_ rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
	return func(_ rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		count, err = db.Count(ctx, filters...)
		if err != nil {
			return -1, false, err
		}
		return count, false, nil
	}
}
