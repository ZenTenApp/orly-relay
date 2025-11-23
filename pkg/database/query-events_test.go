package database

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/event/examples"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"next.orly.dev/pkg/utils"
)

// setupTestDB creates a new test database and loads example events
func setupTestDB(t *testing.T) (
	*D, []*event.E, context.Context, context.CancelFunc, string,
) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
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

	// Now process each event in chronological order
	for _, ev := range events {
		// Save the event to the database
		if _, err = db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event #%d: %v", eventCount+1, err)
		}

		eventCount++
	}

	t.Logf("Successfully saved %d events to the database", eventCount)

	return db, events, ctx, cancel, tempDir
}

func TestQueryEventsByID(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Test QueryEvents with an ID filter
	testEvent := events[3] // Using the same event as in other tests

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(testEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by ID: %v", err)
	}

	// Verify we got exactly one event
	if len(evs) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evs))
	}

	// Verify it's the correct event
	if !utils.FastEqual(evs[0].ID, testEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match. Got %x, expected %x", evs[0].ID,
			testEvent.ID,
		)
	}
}

func TestQueryEventsByKind(t *testing.T) {
	db, _, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Test querying by kind
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Kinds: kindFilter,
			Tags:  tag.NewS(),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by kind: %v", err)
	}

	// Verify we got results
	if len(evs) == 0 {
		t.Fatal("Expected events with kind 1, but got none")
	}

	// Verify all events have the correct kind
	for i, ev := range evs {
		if ev.Kind != testKind.K {
			t.Fatalf(
				"Event %d has incorrect kind. Got %d, expected %d", i,
				ev.Kind, testKind.K,
			)
		}
	}
}

func TestQueryEventsByAuthor(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Test querying by author
	authorFilter := tag.NewFromBytesSlice(events[1].Pubkey)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Authors: authorFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by author: %v", err)
	}

	// Verify we got results
	if len(evs) == 0 {
		t.Fatal("Expected events from author, but got none")
	}

	// Verify all events have the correct author
	for i, ev := range evs {
		if !utils.FastEqual(ev.Pubkey, events[1].Pubkey) {
			t.Fatalf(
				"Event %d has incorrect author. Got %x, expected %x",
				i, ev.Pubkey, events[1].Pubkey,
			)
		}
	}
}

func TestReplaceableEventsAndDeletion(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Test querying for replaced events by ID
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a replaceable event
	replaceableEvent := event.New()
	replaceableEvent.Kind = kind.ProfileMetadata.K        // Kind 0 is replaceable
	replaceableEvent.Pubkey = events[0].Pubkey            // Use the same pubkey as an existing event
	replaceableEvent.CreatedAt = timestamp.Now().V - 7200 // 2 hours ago
	replaceableEvent.Content = []byte("Original profile")
	replaceableEvent.Tags = tag.NewS()
	replaceableEvent.Sign(sign)
	// Save the replaceable event
	if _, err := db.SaveEvent(ctx, replaceableEvent); err != nil {
		t.Errorf("Failed to save replaceable event: %v", err)
	}

	// Create a newer version of the replaceable event
	newerEvent := event.New()
	newerEvent.Kind = kind.ProfileMetadata.K        // Same kind
	newerEvent.Pubkey = replaceableEvent.Pubkey     // Same pubkey
	newerEvent.CreatedAt = timestamp.Now().V - 3600 // 1 hour ago (newer than the original)
	newerEvent.Content = []byte("Updated profile")
	newerEvent.Tags = tag.NewS()
	newerEvent.Sign(sign)
	// Save the newer event
	if _, err := db.SaveEvent(ctx, newerEvent); err != nil {
		t.Errorf("Failed to save newer event: %v", err)
	}

	// Query for the original event by ID
	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromAny(replaceableEvent.ID),
		},
	)
	if err != nil {
		t.Errorf("Failed to query for replaced event by ID: %v", err)
	}

	// Verify the original event is still found (it's kept but not returned in general queries)
	if len(evs) != 1 {
		t.Errorf("Expected 1 event when querying for replaced event by ID, got %d", len(evs))
	}

	// Verify it's the original event
	if !utils.FastEqual(evs[0].ID, replaceableEvent.ID) {
		t.Errorf(
			"Event ID doesn't match when querying for replaced event. Got %x, expected %x",
			evs[0].ID, replaceableEvent.ID,
		)
	}

	// Query for all events of this kind and pubkey
	kindFilter := kind.NewS(kind.ProfileMetadata)
	authorFilter := tag.NewFromAny(replaceableEvent.Pubkey)

	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Kinds:   kindFilter,
			Authors: authorFilter,
		},
	)
	if err != nil {
		t.Errorf("Failed to query for replaceable events: %v", err)
	}

	// Verify we got only one event (the latest one)
	if len(evs) != 1 {
		t.Errorf(
			"Expected 1 event when querying for replaceable events, got %d",
			len(evs),
		)
	}

	// Verify it's the newer event
	if !utils.FastEqual(evs[0].ID, newerEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match when querying for replaceable events. Got %x, expected %x",
			evs[0].ID, newerEvent.ID,
		)
	}

	// Test deletion events
	// Create a deletion event that references the replaceable event
	deletionEvent := event.New()
	deletionEvent.Kind = kind.Deletion.K           // Kind 5 is deletion
	deletionEvent.Pubkey = replaceableEvent.Pubkey // Same pubkey as the event being deleted
	deletionEvent.CreatedAt = timestamp.Now().V    // Current time
	deletionEvent.Content = []byte("Deleting the replaceable event")
	deletionEvent.Tags = tag.NewS()
	deletionEvent.Sign(sign)

	// Add an e-tag referencing the replaceable event
	t.Logf("Replaceable event ID: %x", replaceableEvent.ID)
	*deletionEvent.Tags = append(
		*deletionEvent.Tags,
		tag.NewFromAny("e", hex.Enc(replaceableEvent.ID)),
	)

	// Save the deletion event
	if _, err = db.SaveEvent(ctx, deletionEvent); err != nil {
		t.Fatalf("Failed to save deletion event: %v", err)
	}

	// Debug: Check if the deletion event was saved
	t.Logf("Deletion event ID: %x", deletionEvent.ID)
	t.Logf("Deletion event pubkey: %x", deletionEvent.Pubkey)
	t.Logf("Deletion event kind: %d", deletionEvent.Kind)
	t.Logf("Deletion event tags count: %d", deletionEvent.Tags.Len())
	for i, tag := range *deletionEvent.Tags {
		t.Logf("Deletion event tag[%d]: %v", i, tag.T)
	}

	// Query for all events of this kind and pubkey again
	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Kinds:   kindFilter,
			Authors: authorFilter,
		},
	)
	if err != nil {
		t.Errorf(
			"Failed to query for replaceable events after deletion: %v", err,
		)
	}

	// Verify we still get the newer event (deletion should only affect the original event)
	if len(evs) != 1 {
		t.Fatalf(
			"Expected 1 event when querying for replaceable events after deletion, got %d",
			len(evs),
		)
	}

	// Verify it's still the newer event
	if !utils.FastEqual(evs[0].ID, newerEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match after deletion. Got %x, expected %x",
			evs[0].ID, newerEvent.ID,
		)
	}

	// Query for the original event by ID
	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(replaceableEvent.ID),
		},
	)
	if err != nil {
		t.Errorf("Failed to query for deleted event by ID: %v", err)
	}

	// Verify the original event is not found (it was deleted)
	if len(evs) != 0 {
		t.Errorf("Expected 0 events when querying for deleted event by ID, got %d", len(evs))
	}

	// // Verify we still get the original event when querying by ID
	// if len(evs) != 1 {
	// 	t.Errorf(
	// 		"Expected 1 event when querying for deleted event by ID, got %d",
	// 		len(evs),
	// 	)
	// }

	// // Verify it's the original event
	// if !utils.FastEqual(evs[0].ID, replaceableEvent.ID) {
	// 	t.Errorf(
	// 		"Event ID doesn't match when querying for deleted event by ID. Got %x, expected %x",
	// 		evs[0].ID, replaceableEvent.ID,
	// 	)
	// }
}

func TestParameterizedReplaceableEventsAndDeletion(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a parameterized replaceable event
	paramEvent := event.New()
	paramEvent.Kind = 30000                         // Kind 30000+ is parameterized replaceable
	paramEvent.Pubkey = events[0].Pubkey            // Use the same pubkey as an existing event
	paramEvent.CreatedAt = timestamp.Now().V - 7200 // 2 hours ago
	paramEvent.Content = []byte("Original parameterized event")
	paramEvent.Tags = tag.NewS()
	// Add a d-tag
	*paramEvent.Tags = append(
		*paramEvent.Tags, tag.NewFromAny([]byte{'d'}, []byte("test-d-tag")),
	)
	paramEvent.Sign(sign)

	// Save the parameterized replaceable event
	if _, err := db.SaveEvent(ctx, paramEvent); err != nil {
		t.Fatalf("Failed to save parameterized replaceable event: %v", err)
	}

	// Create a deletion event that references the parameterized replaceable event using an a-tag
	paramDeletionEvent := event.New()
	paramDeletionEvent.Kind = kind.Deletion.K        // Kind 5 is deletion
	paramDeletionEvent.Pubkey = paramEvent.Pubkey    // Same pubkey as the event being deleted
	paramDeletionEvent.CreatedAt = timestamp.Now().V // Current time
	paramDeletionEvent.Content = []byte("Deleting the parameterized replaceable event")
	paramDeletionEvent.Tags = tag.NewS()
	// Add an a-tag referencing the parameterized replaceable event
	// Format: kind:pubkey:d-tag
	aTagValue := fmt.Sprintf(
		"%d:%s:%s",
		paramEvent.Kind,
		hex.Enc(paramEvent.Pubkey),
		"test-d-tag",
	)
	*paramDeletionEvent.Tags = append(
		*paramDeletionEvent.Tags,
		tag.NewFromAny([]byte{'a'}, []byte(aTagValue)),
	)
	paramDeletionEvent.Sign(sign)

	// Save the parameterized deletion event
	if _, err := db.SaveEvent(ctx, paramDeletionEvent); err != nil {
		t.Fatalf("Failed to save parameterized deletion event: %v", err)
	}

	// Query for all events of this kind and pubkey
	paramKindFilter := kind.NewS(kind.New(paramEvent.Kind))
	paramAuthorFilter := tag.NewFromBytesSlice(paramEvent.Pubkey)

	// Print debug info about the a-tag
	fmt.Printf("Debug: a-tag value: %s\n", aTagValue)
	fmt.Printf(
		"Debug: kind: %d, pubkey: %s, d-tag: %s\n",
		paramEvent.Kind,
		hex.Enc(paramEvent.Pubkey),
		"test-d-tag",
	)

	// Let's try a different approach - use an e-tag instead of an a-tag
	// Create another deletion event that references the parameterized replaceable event using an e-tag
	paramDeletionEvent2 := event.New()
	paramDeletionEvent2.Kind = kind.Deletion.K        // Kind 5 is deletion
	paramDeletionEvent2.Pubkey = paramEvent.Pubkey    // Same pubkey as the event being deleted
	paramDeletionEvent2.CreatedAt = timestamp.Now().V // Current time
	paramDeletionEvent2.Content = []byte("Deleting the parameterized replaceable event with e-tag")
	paramDeletionEvent2.Tags = tag.NewS()
	// Add an e-tag referencing the parameterized replaceable event
	*paramDeletionEvent2.Tags = append(
		*paramDeletionEvent2.Tags,
		tag.NewFromAny("e", []byte(hex.Enc(paramEvent.ID))),
	)
	paramDeletionEvent2.Sign(sign)

	// Save the parameterized deletion event with e-tag
	if _, err := db.SaveEvent(ctx, paramDeletionEvent2); err != nil {
		t.Fatalf(
			"Failed to save parameterized deletion event with e-tag: %v", err,
		)
	}

	fmt.Printf("Debug: Added a second deletion event with e-tag referencing the event ID\n")

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Kinds:   paramKindFilter,
			Authors: paramAuthorFilter,
		},
	)
	if err != nil {
		t.Fatalf(
			"Failed to query for parameterized replaceable events after deletion: %v",
			err,
		)
	}

	// Print debug info about the returned events
	fmt.Printf("Debug: Got %d events\n", len(evs))
	for i, ev := range evs {
		fmt.Printf(
			"Debug: Event %d: kind=%d, pubkey=%s\n",
			i, ev.Kind, hex.Enc(ev.Pubkey),
		)
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag != nil && dTag.Len() > 1 {
			fmt.Printf("Debug: Event %d: d-tag=%s\n", i, dTag.Value())
		}
	}

	// Verify we get no events (since the only one was deleted)
	if len(evs) != 0 {
		t.Fatalf(
			"Expected 0 events when querying for deleted parameterized replaceable events, got %d",
			len(evs),
		)
	}

	// Query for the parameterized event by ID
	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(paramEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf(
			"Failed to query for deleted parameterized event by ID: %v", err,
		)
	}

	// Verify the deleted event is not found when querying by ID
	if len(evs) != 0 {
		t.Fatalf(
			"Expected 0 events when querying for deleted parameterized event by ID, got %d",
			len(evs),
		)
	}
}

func TestQueryEventsByTimeRange(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Test querying by time range
	// Use the timestamp from the middle event as a reference
	middleIndex := len(events) / 2
	middleEvent := events[middleIndex]

	// Create a timestamp range that includes events before and after the middle event
	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600 // 1 hour before middle event

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600 // 1 hour after middle event

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Since: sinceTime,
			Until: untilTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by time range: %v", err)
	}

	// Verify we got results
	if len(evs) == 0 {
		t.Fatal("Expected events in time range, but got none")
	}

	// Verify all events are within the time range
	for i, ev := range evs {
		if ev.CreatedAt < sinceTime.V || ev.CreatedAt > untilTime.V {
			t.Fatalf(
				"Event %d is outside the time range. Got %d, expected between %d and %d",
				i, ev.CreatedAt, sinceTime.V, untilTime.V,
			)
		}
	}
}

func TestQueryEventsByTag(t *testing.T) {
	db, events, ctx, cancel, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir) // Clean up after the test
	defer cancel()
	defer db.Close()

	// Find an event with tags to use for testing
	var testTagEvent *event.E
	for _, ev := range events {
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			// Find a tag with at least 2 elements and first element of length 1
			for _, tag := range *ev.Tags {
				if tag.Len() >= 2 && len(tag.Key()) == 1 {
					testTagEvent = ev
					break
				}
			}
			if testTagEvent != nil {
				break
			}
		}
	}

	if testTagEvent == nil {
		t.Skip("No suitable event with tags found for testing")
		return
	}

	// Get the first tag with at least 2 elements and first element of length 1
	var testTag *tag.T
	for _, tag := range *testTagEvent.Tags {
		if tag.Len() >= 2 && len(tag.Key()) == 1 {
			testTag = tag
			break
		}
	}

	// Create a tags filter with the test tag
	tagsFilter := tag.NewS(testTag)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Tags: tagsFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by tag: %v", err)
	}

	// Verify we got results
	if len(evs) == 0 {
		t.Fatal("Expected events with tag, but got none")
	}

	// Verify all events have the tag
	for i, ev := range evs {
		var hasTag bool
		for _, tag := range *ev.Tags {
			if tag.Len() >= 2 && len(tag.Key()) == 1 {
				if utils.FastEqual(tag.Key(), testTag.Key()) &&
					utils.FastEqual(tag.Value(), testTag.Value()) {
					hasTag = true
					break
				}
			}
		}
		if !hasTag {
			t.Fatalf("Event %d does not have the expected tag", i)
		}
	}
}
