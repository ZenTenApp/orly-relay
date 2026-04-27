package database

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"sync"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/event/examples"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Shared test fixtures - initialized once in TestMain
var (
	sharedDB         *D
	sharedDBDir      string
	sharedDBCtx      context.Context
	sharedDBCancel   context.CancelFunc
	sharedDBOnce     sync.Once
	sharedEvents     []*event.E // Events that were successfully saved
	sharedSetupError error
)

// initSharedDB initializes the shared test database with seeded data.
// This is called once and shared across all tests that need seeded data.
func initSharedDB() {
	sharedDBOnce.Do(func() {
		var err error

		// Create a temporary directory for the shared database
		sharedDBDir, err = os.MkdirTemp("", "shared-test-db-*")
		if err != nil {
			sharedSetupError = err
			return
		}

		// Create a context for the database
		sharedDBCtx, sharedDBCancel = context.WithCancel(context.Background())

		// Initialize the database
		sharedDB, err = New(sharedDBCtx, sharedDBCancel, sharedDBDir, "info")
		if err != nil {
			sharedSetupError = err
			return
		}

		// Seed the database with events from examples.Cache
		scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
		scanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)

		var events []*event.E
		for scanner.Scan() {
			b := scanner.Bytes()
			ev := event.New()
			if _, err = ev.Unmarshal(b); err != nil {
				continue
			}
			events = append(events, ev)
		}

		// Sort events by CreatedAt for consistent processing
		sort.Slice(events, func(i, j int) bool {
			return events[i].CreatedAt < events[j].CreatedAt
		})

		// Save events to the database
		for _, ev := range events {
			if _, err = sharedDB.SaveEvent(sharedDBCtx, ev); err != nil {
				continue // Skip invalid events
			}
			sharedEvents = append(sharedEvents, ev)
		}
	})
}

// GetSharedDB returns the shared test database.
// Returns nil if testing.Short() is set or if setup failed.
func GetSharedDB(t *testing.T) (*D, context.Context) {
	if testing.Short() {
		t.Skip("skipping test that requires seeded database in short mode")
	}

	initSharedDB()

	if sharedSetupError != nil {
		t.Fatalf("Failed to initialize shared database: %v", sharedSetupError)
	}

	return sharedDB, sharedDBCtx
}

// GetSharedEvents returns the events that were successfully saved to the shared database.
func GetSharedEvents(t *testing.T) []*event.E {
	if testing.Short() {
		t.Skip("skipping test that requires seeded events in short mode")
	}

	initSharedDB()

	if sharedSetupError != nil {
		t.Fatalf("Failed to initialize shared database: %v", sharedSetupError)
	}

	return sharedEvents
}

// findEventWithTag finds an event with a single-character tag key and at least 2 elements.
// Returns nil if no suitable event is found.
func findEventWithTag(events []*event.E) *event.E {
	for _, ev := range events {
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			for _, tg := range *ev.Tags {
				if tg.Len() >= 2 && len(tg.Key()) == 1 {
					return ev
				}
			}
		}
	}
	return nil
}

func TestMain(m *testing.M) {
	// Disable all logging during tests unless explicitly enabled
	if os.Getenv("TEST_LOG") == "" {
		// Set log level to Off to suppress all logs
		lol.SetLogLevel("off")
		// Also redirect output to discard
		lol.Writer = io.Discard
		// Disable all log printers
		log.T = lol.GetNullPrinter()
		log.D = lol.GetNullPrinter()
		log.I = lol.GetNullPrinter()
		log.W = lol.GetNullPrinter()
		log.E = lol.GetNullPrinter()
		log.F = lol.GetNullPrinter()

		// Also suppress badger logs
		os.Setenv("BADGER_LOG_LEVEL", "CRITICAL")
	}

	// Run tests
	code := m.Run()

	// Cleanup shared database
	if sharedDBCancel != nil {
		sharedDBCancel()
	}
	if sharedDB != nil {
		sharedDB.Close()
	}
	if sharedDBDir != "" {
		os.RemoveAll(sharedDBDir)
	}

	os.Exit(code)
}
