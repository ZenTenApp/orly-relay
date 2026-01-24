package database

import (
	"bytes"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
)

func TestQueryForKinds(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) == 0 {
		t.Fatal("Need at least 1 saved event")
	}

	// Test querying by kind
	// Find an event with a specific kind
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	idTsPk, err := db.QueryForIds(
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
			if bytes.Equal(result.Id[:], ev.ID[:]) {
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
}
