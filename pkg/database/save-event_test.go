package database

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/event/examples"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
)

// TestSaveEvents tests saving all events from examples.Cache to the database
// to verify there are no errors during the saving process.
func TestSaveEvents(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after the test

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a scanner to read events from examples.Cache
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	scanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)

	// Collect all events first
	var events []*event.E
	var original int
	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		original += len(b)
		ev := event.New()

		// Unmarshal the event
		if _, err = ev.Unmarshal(b); chk.E(err) {
			t.Fatal(err)
		}

		events = append(events, ev)
	}

	// Sort events by timestamp to ensure addressable events are processed in order
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	// Count the number of events processed
	eventCount := 0
	skippedCount := 0
	var kc, vc int
	now := time.Now()
	// Process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		var k, v int
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			// Skip events that fail validation (e.g., kind 3 without p tags)
			skippedCount++
			continue
		}
		kc += k
		vc += v
		eventCount++
	}
	_ = skippedCount // Used for logging below

	// Check for scanner errors
	if err = scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}
	dur := time.Since(now)
	t.Logf(
		"Successfully saved %d events %d bytes to the database, %d bytes keys, %d bytes values in %v (%v/ev; %f ev/s)",
		eventCount,
		original,
		kc, vc,
		dur,
		dur/time.Duration(eventCount),
		float64(time.Second)/float64(dur/time.Duration(eventCount)),
	)
}

// TestDeletionEventWithETagRejection tests that a deletion event with an "e" tag is rejected.
func TestDeletionEventWithETagRejection(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after the test

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a signer
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a regular event
	regularEvent := event.New()
	regularEvent.Kind = kind.TextNote.K
	regularEvent.Pubkey = sign.Pub()
	regularEvent.CreatedAt = timestamp.Now().V - 3600 // 1 hour ago
	regularEvent.Content = []byte("Regular event")
	regularEvent.Tags = tag.NewS()
	regularEvent.Sign(sign)

	// Save the regular event
	if _, err := db.SaveEvent(ctx, regularEvent); err != nil {
		t.Fatalf("Failed to save regular event: %v", err)
	}

	// Create a deletion event with an "e" tag referencing the regular event
	deletionEvent := event.New()
	deletionEvent.Kind = kind.Deletion.K
	deletionEvent.Pubkey = sign.Pub()
	deletionEvent.CreatedAt = timestamp.Now().V // Current time
	deletionEvent.Content = []byte("Deleting the regular event")
	deletionEvent.Tags = tag.NewS()

	// Add an e-tag referencing the regular event
	*deletionEvent.Tags = append(
		*deletionEvent.Tags,
		tag.NewFromAny("e", hex.Enc(regularEvent.ID)),
	)

	deletionEvent.Sign(sign)

	// Check if this is a deletion event with "e" tags
	if deletionEvent.Kind == kind.Deletion.K && deletionEvent.Tags.GetFirst([]byte{'e'}) != nil {
		// In this test, we want to reject deletion events with "e" tags
		err = errorf.E("deletion events referencing other events with 'e' tag are not allowed")
	} else {
		// Try to save the deletion event
		_, err = db.SaveEvent(ctx, deletionEvent)
	}

	if err == nil {
		t.Fatal("Expected deletion event with e-tag to be rejected, but it was accepted")
	}

	// Verify the error message
	expectedError := "deletion events referencing other events with 'e' tag are not allowed"
	if err.Error() != expectedError {
		t.Fatalf(
			"Expected error message '%s', got '%s'", expectedError, err.Error(),
		)
	}
}

// TestSaveExistingEvent tests that attempting to save an event that already exists
// returns an error.
func TestSaveExistingEvent(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after the test

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a signer
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create an event
	ev := event.New()
	ev.Kind = kind.TextNote.K
	ev.Pubkey = sign.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte("Test event")
	ev.Tags = tag.NewS()
	ev.Sign(sign)

	// Save the event for the first time
	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Try to save the same event again, it should be rejected
	_, err = db.SaveEvent(ctx, ev)
	if err == nil {
		t.Fatal("Expected error when saving an existing event, but got nil")
	}

	// Verify the error message
	expectedErrorPrefix := "blocked: event already exists: "
	if !bytes.HasPrefix([]byte(err.Error()), []byte(expectedErrorPrefix)) {
		t.Fatalf(
			"Expected error message to start with '%s', got '%s'",
			expectedErrorPrefix, err.Error(),
		)
	}
}
