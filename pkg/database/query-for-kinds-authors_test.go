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
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
)

func TestQueryForKindsAuthors(t *testing.T) {
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

	// Count the number of events processed
	eventCount := 0

	var events []*event.E

	// First, collect all events from examples.Cache
	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		ev := event.New()

		// Unmarshal the event
		if _, err = ev.Unmarshal(b); chk.E(err) {
			ev.Free()
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
	eventCount = 0
	skippedCount := 0
	var savedEvents []*event.E

	// Now process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			// Skip events that fail validation (e.g., kind 3 without p tags)
			skippedCount++
			continue
		}

		savedEvents = append(savedEvents, ev)
		eventCount++
	}

	t.Logf("Successfully saved %d events to the database (skipped %d invalid events)", eventCount, skippedCount)
	events = savedEvents // Use saved events for the rest of the test

	// Test querying by kind and author
	var idTsPk []*store.IdPkTs

	// Find an event with a specific kind and author
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	// Use the author from events[1]
	authorFilter := tag.NewFromBytesSlice(events[1].Pubkey)

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Kinds:   kindFilter,
			Authors: authorFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for kinds and authors: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with the specified kind and author")
	}

	// Verify the results have the correct kind and author
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
				if ev.Kind != testKind.K {
					t.Fatalf(
						"result %d has incorrect kind, got %d, expected %d",
						i, ev.Kind, testKind.K,
					)
				}
				if !utils.FastEqual(ev.Pubkey, events[1].Pubkey) {
					t.Fatalf(
						"result %d has incorrect author, got %x, expected %x",
						i, ev.Pubkey, events[1].Pubkey,
					)
				}
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}
}
