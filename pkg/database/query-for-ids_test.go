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
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
)

func TestQueryForIds(t *testing.T) {
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

	// Now process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event #%d: %v", eventCount+1, err)
		}

		eventCount++
	}

	t.Logf("Successfully saved %d events to the database", eventCount)

	var idTsPk []*store.IdPkTs
	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Authors: tag.NewFromBytesSlice(events[1].Pubkey),
		},
	)
	if len(idTsPk) < 1 {
		t.Fatalf(
			"got unexpected number of results, expect at least 1, got %d",
			len(idTsPk),
		)
	}
	// Verify that all returned events have the correct author
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
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

	// Test querying by kind
	// Find an event with a specific kind
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Kinds: kindFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for kinds: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with the specified kind")
	}

	// Verify the results have the correct kind
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
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}

	// Test querying by tag
	// Find an event with tags to use for testing
	var testEvent *event.E
	for _, ev := range events {
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			// Find a tag with at least 2 elements and first element of length 1
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

	if testEvent != nil {
		// Get the first tag with at least 2 elements and first element of length 1
		var testTag *tag.T
		for _, tag := range *testEvent.Tags {
			if tag.Len() >= 2 && len(tag.Key()) == 1 {
				testTag = tag
				break
			}
		}

		// Create a tags filter with the test tag
		tagsFilter := tag.NewS(testTag)

		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Tags: tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified tag")
		}

		// Verify the results have the correct tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true

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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by kind and author
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds:   kindFilter,
				Authors: tag.NewFromBytesSlice(events[1].Pubkey),
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds and authors: %v", err)
		}

		// Verify we got results
		if len(idTsPk) > 0 {
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
					t.Fatalf(
						"result %d with ID %x not found in events", i,
						result.Id,
					)
				}
			}
		}

		// Test querying by kind and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds: kind.NewS(kind.New(testEvent.Kind)),
				Tags:  tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds and tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified kind and tag")
		}

		// Verify the results have the correct kind and tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true
					if ev.Kind != testEvent.Kind {
						t.Fatalf(
							"result %d has incorrect kind, got %d, expected %d",
							i, ev.Kind, testEvent.Kind,
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by kind, author, and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds:   kind.NewS(kind.New(testEvent.Kind)),
				Authors: tag.NewFromBytesSlice(testEvent.Pubkey),
				Tags:    tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds, authors, and tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified kind, author, and tag")
		}

		// Verify the results have the correct kind, author, and tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true
					if ev.Kind != testEvent.Kind {
						t.Fatalf(
							"result %d has incorrect kind, got %d, expected %d",
							i, ev.Kind, testEvent.Kind,
						)
					}

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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by author and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Authors: tag.NewFromBytesSlice(testEvent.Pubkey),
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}
	}

	// Test querying by created_at range
	// Use the timestamp from the middle event as a reference
	middleIndex := len(events) / 2
	middleEvent := events[middleIndex]

	// Create a timestamp range that includes events before and after the middle event
	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600 // 1 hour before middle event

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600 // 1 hour after middle event

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Since: sinceTime,
			Until: untilTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for created_at range: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events in the specified time range")
	}

	// Verify the results exist in our events slice
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}
}
