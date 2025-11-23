package dgraph

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"sort"
	"testing"
	"time"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/event/examples"
)

// isDgraphAvailable checks if a dgraph server is running
func isDgraphAvailable() bool {
	dgraphURL := os.Getenv("ORLY_DGRAPH_URL")
	if dgraphURL == "" {
		dgraphURL = "localhost:9080"
	}

	conn, err := net.DialTimeout("tcp", dgraphURL, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// skipIfDgraphNotAvailable skips the test if dgraph is not available
func skipIfDgraphNotAvailable(t *testing.T) {
	if !isDgraphAvailable() {
		dgraphURL := os.Getenv("ORLY_DGRAPH_URL")
		if dgraphURL == "" {
			dgraphURL = "localhost:9080"
		}
		t.Skipf("Dgraph server not available at %s. Start with: docker run -p 9080:9080 dgraph/standalone:latest", dgraphURL)
	}
}

// setupTestDB creates a new test dgraph database and loads example events
func setupTestDB(t *testing.T) (
	*D, []*event.E, context.Context, context.CancelFunc, string,
) {
	skipIfDgraphNotAvailable(t)

	// Create a temporary directory for metadata storage
	tempDir, err := os.MkdirTemp("", "test-dgraph-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the dgraph database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		cancel()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create dgraph database: %v", err)
	}

	// Drop all data to start fresh
	if err := db.dropAll(ctx); err != nil {
		db.Close()
		cancel()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to drop all data: %v", err)
	}

	// Create a scanner to read events from examples.Cache
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	scanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)

	var events []*event.E

	// First, collect all events from examples.Cache
	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		ev := event.New()

		// Unmarshal the event
		if _, err = ev.Unmarshal(b); chk.E(err) {
			ev.Free()
			db.Close()
			cancel()
			os.RemoveAll(tempDir)
			t.Fatal(err)
		}

		events = append(events, ev)
	}

	// Check for scanner errors
	if err = scanner.Err(); err != nil {
		db.Close()
		cancel()
		os.RemoveAll(tempDir)
		t.Fatalf("Scanner error: %v", err)
	}

	// Sort events by CreatedAt to ensure addressable events are processed in chronological order
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	// Count the number of events processed
	eventCount := 0

	// Now process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			db.Close()
			cancel()
			os.RemoveAll(tempDir)
			t.Fatalf("Failed to save event #%d: %v", eventCount+1, err)
		}

		eventCount++
	}

	t.Logf("Successfully saved %d events to dgraph database", eventCount)

	return db, events, ctx, cancel, tempDir
}

// cleanupTestDB cleans up the test database
func cleanupTestDB(t *testing.T, db *D, cancel context.CancelFunc, tempDir string) {
	if db != nil {
		db.Close()
	}
	if cancel != nil {
		cancel()
	}
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
}
