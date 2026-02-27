package database

import (
	"bytes"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

func TestQueryForKindsAuthors(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 2 {
		t.Fatalf("Need at least 2 saved events, got %d", len(events))
	}

	// Test querying by kind and author
	// Find an event with a specific kind and author
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	// Use the author from events[1]
	authorFilter := tag.NewFromBytesSlice(events[1].Pubkey)

	idTsPk, err := db.QueryForIds(
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
			if bytes.Equal(result.Id[:], ev.ID[:]) {
				found = true
				if ev.Kind != testKind.K {
					t.Fatalf(
						"result %d has incorrect kind, got %d, expected %d",
						i, ev.Kind, testKind.K,
					)
				}
				if !bytes.Equal(ev.Pubkey[:], events[1].Pubkey[:]) {
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
