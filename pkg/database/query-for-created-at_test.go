package database

import (
	"bytes"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
)

func TestQueryForCreatedAt(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 3 {
		t.Fatalf("Need at least 3 saved events, got %d", len(events))
	}

	// Find a timestamp range that should include some events
	// Use the timestamp from the middle event as a reference
	middleIndex := len(events) / 2
	middleEvent := events[middleIndex]

	// Create a timestamp range that includes events before and after the middle event
	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600 // 1 hour before middle event

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600 // 1 hour after middle event

	// Test querying by created_at range
	idTsPk, err := db.QueryForIds(
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

	// Verify the results exist in our events slice and are within the timestamp range
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if bytes.Equal(result.Id[:], ev.ID[:]) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}

		// Verify the timestamp is within the range
		if result.Ts < sinceTime.V || result.Ts > untilTime.V {
			t.Fatalf(
				"result %d with ID %x has timestamp %d outside the range [%d, %d]",
				i, result.Id, result.Ts, sinceTime.V, untilTime.V,
			)
		}
	}

	// Test with only Since
	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Since: sinceTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query with Since: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with Since filter")
	}

	// Verify the results exist in our events slice and are after the Since timestamp
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if bytes.Equal(result.Id[:], ev.ID[:]) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}

		// Verify the timestamp is after the Since timestamp
		if result.Ts < sinceTime.V {
			t.Fatalf(
				"result %d with ID %x has timestamp %d before the Since timestamp %d",
				i, result.Id, result.Ts, sinceTime.V,
			)
		}
	}

	// Test with only Until
	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Until: untilTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query with Until: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with Until filter")
	}

	// Verify the results exist in our events slice and are before the Until timestamp
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if bytes.Equal(result.Id[:], ev.ID[:]) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}

		// Verify the timestamp is before the Until timestamp
		if result.Ts > untilTime.V {
			t.Fatalf(
				"result %d with ID %x has timestamp %d after the Until timestamp %d",
				i, result.Id, result.Ts, untilTime.V,
			)
		}
	}
}
