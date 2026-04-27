package database

import (
	"fmt"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/utils"
)

// TestMultipleParameterizedReplaceableEvents tests that when multiple parameterized
// replaceable events with the same pubkey, kind, and d-tag exist, only the newest one
// is returned in query results.
func TestMultipleParameterizedReplaceableEvents(t *testing.T) {
	// Needs fresh database (modifies data)
	db, ctx, cleanup := setupFreshTestDB(t)
	defer cleanup()

	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a base parameterized replaceable event
	baseEvent := event.New()
	baseEvent.Kind = kind.ParameterizedReplaceableStart.K // Kind 30000+ is parameterized replaceable
	baseEvent.CreatedAt = timestamp.Now().V - 7200        // 2 hours ago
	baseEvent.Content = []byte("Original parameterized event")
	baseEvent.Tags = tag.NewS()
	// Add a d-tag
	*baseEvent.Tags = append(
		*baseEvent.Tags, tag.NewFromAny("d", "test-d-tag"),
	)
	baseEvent.Sign(sign)

	// Save the base parameterized replaceable event
	if _, err := db.SaveEvent(ctx, baseEvent); err != nil {
		t.Fatalf("Failed to save base parameterized replaceable event: %v", err)
	}

	// Create a newer parameterized replaceable event with the same pubkey, kind, and d-tag
	newerEvent := event.New()
	newerEvent.Kind = baseEvent.Kind                // Same parameterized kind
	newerEvent.CreatedAt = timestamp.Now().V - 3600 // 1 hour ago (newer than base event)
	newerEvent.Content = []byte("Newer parameterized event")
	newerEvent.Tags = tag.NewS()
	// Add the same d-tag
	*newerEvent.Tags = append(
		*newerEvent.Tags,
		tag.NewFromAny("d", "test-d-tag"),
	)
	newerEvent.Sign(sign)

	// Save the newer parameterized replaceable event
	if _, err := db.SaveEvent(ctx, newerEvent); err != nil {
		t.Fatalf(
			"Failed to save newer parameterized replaceable event: %v", err,
		)
	}

	// Create an even newer parameterized replaceable event with the same pubkey, kind, and d-tag
	newestEvent := event.New()
	newestEvent.Kind = baseEvent.Kind         // Same parameterized kind
	newestEvent.CreatedAt = timestamp.Now().V // Current time (newest)
	newestEvent.Content = []byte("Newest parameterized event")
	newestEvent.Tags = tag.NewS()
	// Add the same d-tag
	*newestEvent.Tags = append(
		*newestEvent.Tags,
		tag.NewFromAny("d", "test-d-tag"),
	)
	newestEvent.Sign(sign)

	// Save the newest parameterized replaceable event
	if _, err := db.SaveEvent(ctx, newestEvent); err != nil {
		t.Fatalf(
			"Failed to save newest parameterized replaceable event: %v", err,
		)
	}

	// Query for all events of this kind and pubkey
	paramKindFilter := kind.NewS(kind.New(baseEvent.Kind))
	paramAuthorFilter := tag.NewFromBytesSlice(baseEvent.Pubkey)

	evs, err := db.QueryEvents(
		ctx, &filter.F{
			Kinds:   paramKindFilter,
			Authors: paramAuthorFilter,
		},
	)
	if err != nil {
		t.Fatalf(
			"Failed to query for parameterized replaceable events: %v", err,
		)
	}

	// Print debug info about the returned events
	fmt.Printf("Debug: Got %d events\n", len(evs))
	for i, ev := range evs {
		fmt.Printf(
			"Debug: Event %d: kind=%d, pubkey=%s, created_at=%d, content=%s\n",
			i, ev.Kind, hex.Enc(ev.Pubkey), ev.CreatedAt, ev.Content,
		)
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag != nil && dTag.Len() > 1 {
			fmt.Printf("Debug: Event %d: d-tag=%s\n", i, dTag.Value())
		}
	}

	// Verify we get exactly one event (the newest one)
	if len(evs) != 1 {
		t.Fatalf(
			"Expected 1 event when querying for parameterized replaceable events, got %d",
			len(evs),
		)
	}

	// Verify it's the newest event
	if !utils.FastEqual(evs[0].ID, newestEvent.ID) {
		t.Fatalf(
			"Event ID doesn't match the newest event. Got %x, expected %x",
			evs[0].ID, newestEvent.ID,
		)
	}

	// Verify the content is from the newest event
	if string(evs[0].Content) != string(newestEvent.Content) {
		t.Fatalf(
			"Event content doesn't match the newest event. Got %s, expected %s",
			evs[0].Content, newestEvent.Content,
		)
	}

	// Query for the base event by ID
	evs, err = db.QueryEvents(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(baseEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for base event by ID: %v", err)
	}

	// Verify we get 1 event when querying for the base event by ID
	// Replaced events should still be accessible by their ID
	if len(evs) != 1 {
		t.Fatalf(
			"Expected 1 event when querying for replaced base event by ID, got %d",
			len(evs),
		)
	}
}
