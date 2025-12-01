package database

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"sort"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/event/examples"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/utils"
)

func TestFetchEventBySerial(t *testing.T) {
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

	var events []*event.E

	// First, collect all events
	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		ev := event.New()

		// Unmarshal the event
		if _, err = ev.Unmarshal(b); chk.E(err) {
			t.Fatal(err)
		}

		events = append(events, ev)
	}

	// Check for scanner errors
	if err = scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	// Sort events by CreatedAt to ensure addressable events are processed in chronological order
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	// Count the number of events processed
	eventCount := 0
	skippedCount := 0
	var savedEvents []*event.E

	// Process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			// Skip events that fail validation (e.g., kind 3 without p tags)
			// This can happen with real-world test data from examples.Cache
			skippedCount++
			continue
		}

		savedEvents = append(savedEvents, ev)
		eventCount++
	}

	t.Logf("Successfully saved %d events to the database (skipped %d invalid events)", eventCount, skippedCount)

	// Instead of trying to find a valid serial directly, let's use QueryForIds
	// which is known to work from the other tests
	// Use the first successfully saved event (not original events which may include skipped ones)
	if len(savedEvents) < 4 {
		t.Fatalf("Need at least 4 saved events, got %d", len(savedEvents))
	}
	testEvent := savedEvents[3]

	// Use QueryForIds to get the IdPkTs for this event
	var sers types.Uint40s
	sers, err = db.QueryForSerials(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(testEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for Ids: %v", err)
	}

	// Verify we got exactly one result
	if len(sers) != 1 {
		t.Fatalf("Expected 1 IdPkTs, got %d", len(sers))
	}

	// Fetch the event by serial
	fetchedEvent, err := db.FetchEventBySerial(sers[0])
	if err != nil {
		t.Fatalf("Failed to fetch event by serial: %v", err)
	}

	// Verify the fetched event is not nil
	if fetchedEvent == nil {
		t.Fatal("Expected fetched event to be non-nil, but got nil")
	}

	// Verify the fetched event has the same ID as the original event
	if !utils.FastEqual(fetchedEvent.ID, testEvent.ID) {
		t.Fatalf(
			"Fetched event ID doesn't match original event ID. Got %x, expected %x",
			fetchedEvent.ID, testEvent.ID,
		)
	}

	// Verify other event properties match
	if fetchedEvent.Kind != testEvent.Kind {
		t.Fatalf(
			"Fetched event kind doesn't match. Got %d, expected %d",
			fetchedEvent.Kind, testEvent.Kind,
		)
	}

	if !utils.FastEqual(fetchedEvent.Pubkey, testEvent.Pubkey) {
		t.Fatalf(
			"Fetched event pubkey doesn't match. Got %x, expected %x",
			fetchedEvent.Pubkey, testEvent.Pubkey,
		)
	}

	if fetchedEvent.CreatedAt != testEvent.CreatedAt {
		t.Fatalf(
			"Fetched event created_at doesn't match. Got %d, expected %d",
			fetchedEvent.CreatedAt, testEvent.CreatedAt,
		)
	}

	// Test with a non-existent serial
	nonExistentSerial := new(types.Uint40)
	err = nonExistentSerial.Set(uint64(0xFFFFFFFFFF)) // Max value
	if err != nil {
		t.Fatalf("Failed to create non-existent serial: %v", err)
	}

	// This should return an error since the serial doesn't exist
	fetchedEvent, err = db.FetchEventBySerial(nonExistentSerial)
	if err == nil {
		t.Fatal("Expected error for non-existent serial, but got nil")
	}

	// The fetched event should be nil
	if fetchedEvent != nil {
		t.Fatalf(
			"Expected nil event for non-existent serial, but got: %v",
			fetchedEvent,
		)
	}
}
