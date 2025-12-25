package database

import (
	"context"
	"fmt"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/utils"
)

// setupFreshTestDB creates a new isolated test database for tests that modify data.
// Use this for tests that need to write/delete events.
func setupFreshTestDB(t *testing.T) (*D, context.Context, func()) {
	if testing.Short() {
		t.Skip("skipping test that requires fresh database in short mode")
	}

	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		cancel()
		os.RemoveAll(tempDir)
	}

	return db, ctx, cleanup
}

func TestQueryEventsByID(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 4 {
		t.Fatalf("Need at least 4 saved events, got %d", len(events))
	}
	testEvent := events[3]

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(testEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by ID: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evs))
	}

	if !utils.FastEqual(evs[0].ID, testEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match. Got %x, expected %x", evs[0].ID,
			testEvent.ID,
		)
	}
}

func TestQueryEventsByKind(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)

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

	if len(evs) == 0 {
		t.Fatal("Expected events with kind 1, but got none")
	}

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
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 2 {
		t.Fatalf("Need at least 2 saved events, got %d", len(events))
	}

	authorFilter := tag.NewFromBytesSlice(events[1].Pubkey)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Authors: authorFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by author: %v", err)
	}

	if len(evs) == 0 {
		t.Fatal("Expected events from author, but got none")
	}

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
	// Needs fresh database (modifies data)
	db, ctx, cleanup := setupFreshTestDB(t)
	defer cleanup()

	// Seed with a few events for pubkey reference
	events := GetSharedEvents(t)
	if len(events) == 0 {
		t.Fatal("Need at least 1 event for pubkey reference")
	}

	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a replaceable event
	replaceableEvent := event.New()
	replaceableEvent.Kind = kind.ProfileMetadata.K
	replaceableEvent.Pubkey = events[0].Pubkey
	replaceableEvent.CreatedAt = timestamp.Now().V - 7200
	replaceableEvent.Content = []byte("Original profile")
	replaceableEvent.Tags = tag.NewS()
	replaceableEvent.Sign(sign)

	if _, err := db.SaveEvent(ctx, replaceableEvent); err != nil {
		t.Errorf("Failed to save replaceable event: %v", err)
	}

	// Create a newer version
	newerEvent := event.New()
	newerEvent.Kind = kind.ProfileMetadata.K
	newerEvent.Pubkey = replaceableEvent.Pubkey
	newerEvent.CreatedAt = timestamp.Now().V - 3600
	newerEvent.Content = []byte("Updated profile")
	newerEvent.Tags = tag.NewS()
	newerEvent.Sign(sign)

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

	if len(evs) != 1 {
		t.Errorf("Expected 1 event when querying for replaced event by ID, got %d", len(evs))
	}

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

	if len(evs) != 1 {
		t.Errorf(
			"Expected 1 event when querying for replaceable events, got %d",
			len(evs),
		)
	}

	if !utils.FastEqual(evs[0].ID, newerEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match when querying for replaceable events. Got %x, expected %x",
			evs[0].ID, newerEvent.ID,
		)
	}

	// Test deletion events
	deletionEvent := event.New()
	deletionEvent.Kind = kind.Deletion.K
	deletionEvent.Pubkey = replaceableEvent.Pubkey
	deletionEvent.CreatedAt = timestamp.Now().V
	deletionEvent.Content = []byte("Deleting the replaceable event")
	deletionEvent.Tags = tag.NewS()
	deletionEvent.Sign(sign)

	*deletionEvent.Tags = append(
		*deletionEvent.Tags,
		tag.NewFromAny("e", hex.Enc(replaceableEvent.ID)),
	)

	if _, err = db.SaveEvent(ctx, deletionEvent); err != nil {
		t.Fatalf("Failed to save deletion event: %v", err)
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

	if len(evs) != 1 {
		t.Fatalf(
			"Expected 1 event when querying for replaceable events after deletion, got %d",
			len(evs),
		)
	}

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

	if len(evs) != 0 {
		t.Errorf("Expected 0 events when querying for deleted event by ID, got %d", len(evs))
	}
}

func TestParameterizedReplaceableEventsAndDeletion(t *testing.T) {
	// Needs fresh database (modifies data)
	db, ctx, cleanup := setupFreshTestDB(t)
	defer cleanup()

	events := GetSharedEvents(t)
	if len(events) == 0 {
		t.Fatal("Need at least 1 event for pubkey reference")
	}

	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a parameterized replaceable event
	paramEvent := event.New()
	paramEvent.Kind = 30000
	paramEvent.Pubkey = events[0].Pubkey
	paramEvent.CreatedAt = timestamp.Now().V - 7200
	paramEvent.Content = []byte("Original parameterized event")
	paramEvent.Tags = tag.NewS()
	*paramEvent.Tags = append(
		*paramEvent.Tags, tag.NewFromAny([]byte{'d'}, []byte("test-d-tag")),
	)
	paramEvent.Sign(sign)

	if _, err := db.SaveEvent(ctx, paramEvent); err != nil {
		t.Fatalf("Failed to save parameterized replaceable event: %v", err)
	}

	// Create a deletion event
	paramDeletionEvent := event.New()
	paramDeletionEvent.Kind = kind.Deletion.K
	paramDeletionEvent.Pubkey = paramEvent.Pubkey
	paramDeletionEvent.CreatedAt = timestamp.Now().V
	paramDeletionEvent.Content = []byte("Deleting the parameterized replaceable event")
	paramDeletionEvent.Tags = tag.NewS()
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

	if _, err := db.SaveEvent(ctx, paramDeletionEvent); err != nil {
		t.Fatalf("Failed to save parameterized deletion event: %v", err)
	}

	// Create deletion with e-tag too
	paramDeletionEvent2 := event.New()
	paramDeletionEvent2.Kind = kind.Deletion.K
	paramDeletionEvent2.Pubkey = paramEvent.Pubkey
	paramDeletionEvent2.CreatedAt = timestamp.Now().V
	paramDeletionEvent2.Content = []byte("Deleting with e-tag")
	paramDeletionEvent2.Tags = tag.NewS()
	*paramDeletionEvent2.Tags = append(
		*paramDeletionEvent2.Tags,
		tag.NewFromAny("e", []byte(hex.Enc(paramEvent.ID))),
	)
	paramDeletionEvent2.Sign(sign)

	if _, err := db.SaveEvent(ctx, paramDeletionEvent2); err != nil {
		t.Fatalf("Failed to save deletion event with e-tag: %v", err)
	}

	// Query for all events of this kind and pubkey
	paramKindFilter := kind.NewS(kind.New(paramEvent.Kind))
	paramAuthorFilter := tag.NewFromBytesSlice(paramEvent.Pubkey)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Kinds:   paramKindFilter,
			Authors: paramAuthorFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for parameterized events: %v", err)
	}

	if len(evs) != 0 {
		t.Fatalf("Expected 0 events after deletion, got %d", len(evs))
	}

	// Query by ID
	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(paramEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for deleted event by ID: %v", err)
	}

	if len(evs) != 0 {
		t.Fatalf("Expected 0 events when querying deleted event by ID, got %d", len(evs))
	}
}

func TestQueryEventsByTimeRange(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 10 {
		t.Fatalf("Need at least 10 saved events, got %d", len(events))
	}

	middleIndex := len(events) / 2
	middleEvent := events[middleIndex]

	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Since: sinceTime,
			Until: untilTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by time range: %v", err)
	}

	if len(evs) == 0 {
		t.Fatal("Expected events in time range, but got none")
	}

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
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	// Find an event with tags
	var testTagEvent *event.E
	for _, ev := range events {
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			for _, tg := range *ev.Tags {
				if tg.Len() >= 2 && len(tg.Key()) == 1 {
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

	var testTag *tag.T
	for _, tg := range *testTagEvent.Tags {
		if tg.Len() >= 2 && len(tg.Key()) == 1 {
			testTag = tg
			break
		}
	}

	tagsFilter := tag.NewS(testTag)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Tags: tagsFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query events by tag: %v", err)
	}

	if len(evs) == 0 {
		t.Fatal("Expected events with tag, but got none")
	}

	for i, ev := range evs {
		var hasTag bool
		for _, tg := range *ev.Tags {
			if tg.Len() >= 2 && len(tg.Key()) == 1 {
				if utils.FastEqual(tg.Key(), testTag.Key()) &&
					utils.FastEqual(tg.Value(), testTag.Value()) {
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
