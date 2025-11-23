package database

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"sort"
	"testing"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/event/examples"
)

func TestGetSerialById(t *testing.T) {
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
	var allEvents []*event.E
	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		ev := event.New()

		// Unmarshal the event
		if _, err = ev.Unmarshal(b); chk.E(err) {
			ev.Free()
			t.Fatal(err)
		}

		allEvents = append(allEvents, ev)
	}

	// Check for scanner errors
	if err = scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	// Sort events by timestamp to ensure addressable events are processed in chronological order
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].CreatedAt < allEvents[j].CreatedAt
	})

	// Now process the sorted events
	eventCount := 0
	var events []*event.E

	for _, ev := range allEvents {
		events = append(events, ev)

		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event #%d: %v", eventCount+1, err)
		}

		eventCount++
	}

	t.Logf("Successfully saved %d events to the database", eventCount)

	// Test GetSerialById with a known event ID
	testEvent := events[3] // Using the same event as in QueryForIds test

	// Get the serial by ID
	serial, err := db.GetSerialById(testEvent.ID)
	if err != nil {
		t.Fatalf("Failed to get serial by ID: %v", err)
	}

	// Verify the serial is not nil
	if serial == nil {
		t.Fatal("Expected serial to be non-nil, but got nil")
	}

	// Test with a non-existent ID
	nonExistentId := make([]byte, len(testEvent.ID))
	// Ensure it's different from any real ID
	for i := range nonExistentId {
		nonExistentId[i] = ^testEvent.ID[i]
	}

	serial, err = db.GetSerialById(nonExistentId)
	if err == nil {
		t.Fatalf("Expected error for non-existent ID, but got none")
	}

	// For non-existent Ids, the function should return nil serial and an error
	if serial != nil {
		t.Fatalf("Expected nil serial for non-existent ID, but got: %v", serial)
	}
}
