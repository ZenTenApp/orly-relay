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
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
)

func TestQueryForAuthorsTags(t *testing.T) {
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

	// Find an event with tags to use for testing
	var testEvent *event.E
	for _, ev := range events {
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			// Find a tag with at least 2 elements and the first element of
			// length 1
			for _, tag := range *ev.Tags {
				if tag.Len() >= 2 && len(tag.Key()) == 1 {
					testEvent = ev
					break
				}
			}
			if testEvent != nil {
				break
			}
		}
	}

	if testEvent == nil {
		t.Skip("No suitable event with tags found for testing")
	}

	// Get the first tag with at least 2 elements and first element of length 1
	var testTag *tag.T
	for _, tag := range *testEvent.Tags {
		if tag.Len() >= 2 && len(tag.Key()) == 1 {
			testTag = tag
			break
		}
	}

	// Test querying by author and tag
	var idTsPk []*store.IdPkTs

	// Use the author from the test event
	authorFilter := tag.NewFromBytesSlice(testEvent.Pubkey)

	// Create a tags filter with the test tag
	tagsFilter := tag.NewS(testTag)

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Authors: authorFilter,
			Tags:    tagsFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for authors and tags: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with the specified author and tag")
	}

	// Verify the results have the correct author and tag
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true

				if !utils.FastEqual(ev.Pubkey, testEvent.Pubkey) {
					t.Fatalf(
						"result %d has incorrect author, got %x, expected %x",
						i, ev.Pubkey, testEvent.Pubkey,
					)
				}

				// Check if the event has the tag we're looking for
				var hasTag bool
				for _, tag := range *ev.Tags {
					if tag.Len() >= 2 && len(tag.Key()) == 1 {
						if utils.FastEqual(
							tag.Key(), testTag.Key(),
						) && utils.FastEqual(tag.Value(), testTag.Value()) {
							hasTag = true
							break
						}
					}
				}

				if !hasTag {
					t.Fatalf(
						"result %d does not have the expected tag",
						i,
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
