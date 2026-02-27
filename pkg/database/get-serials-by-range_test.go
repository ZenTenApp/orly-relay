package database

import (
	"testing"

	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/utils"
)

func TestGetSerialsByRange(t *testing.T) {
	// Use shared database (skips in short mode)
	db, _ := GetSharedDB(t)
	savedEvents := GetSharedEvents(t)

	if len(savedEvents) < 10 {
		t.Fatalf("Need at least 10 saved events, got %d", len(savedEvents))
	}

	// Test GetSerialsByRange with a time range filter
	// Use the timestamp from the middle event as a reference
	middleIndex := len(savedEvents) / 2
	middleEvent := savedEvents[middleIndex]

	// Create a timestamp range that includes events before and after the middle event
	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600 // 1 hour before middle event

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600 // 1 hour after middle event

	// Create a filter with the time range
	timeFilter := &filter.F{
		Since: sinceTime,
		Until: untilTime,
	}

	// Get the indexes from the filter
	ranges, err := GetIndexesFromFilter(timeFilter)
	if err != nil {
		t.Fatalf("Failed to get indexes from filter: %v", err)
	}

	// Verify we got at least one range
	if len(ranges) == 0 {
		t.Fatal("Expected at least one range from filter, but got none")
	}

	// Test GetSerialsByRange with the first range
	serials, err := db.GetSerialsByRange(ranges[0])
	if err != nil {
		t.Fatalf("Failed to get serials by range: %v", err)
	}

	// Verify we got results
	if len(serials) == 0 {
		t.Fatal("Expected serials for events in time range, but got none")
	}

	// Verify the serials correspond to events within the time range
	for i, serial := range serials {
		// Fetch the event using the serial
		ev, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch event for serial %d: %v", i, err)
		}

		if ev.CreatedAt < sinceTime.V || ev.CreatedAt > untilTime.V {
			t.Fatalf(
				"Event %d is outside the time range. Got %d, expected between %d and %d",
				i, ev.CreatedAt, sinceTime.V, untilTime.V,
			)
		}
	}

	// Test GetSerialsByRange with a kind filter
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := &filter.F{
		Kinds: kind.NewS(testKind),
	}

	// Get the indexes from the filter
	ranges, err = GetIndexesFromFilter(kindFilter)
	if err != nil {
		t.Fatalf("Failed to get indexes from filter: %v", err)
	}

	// Verify we got at least one range
	if len(ranges) == 0 {
		t.Fatal("Expected at least one range from filter, but got none")
	}

	// Test GetSerialsByRange with the first range
	serials, err = db.GetSerialsByRange(ranges[0])
	if err != nil {
		t.Fatalf("Failed to get serials by range: %v", err)
	}

	// Verify we got results
	if len(serials) == 0 {
		t.Fatal("Expected serials for events with kind 1, but got none")
	}

	// Verify the serials correspond to events with the correct kind
	for i, serial := range serials {
		// Fetch the event using the serial
		ev, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch event for serial %d: %v", i, err)
		}

		if ev.Kind != testKind.K {
			t.Fatalf(
				"Event %d has incorrect kind. Got %d, expected %d",
				i, ev.Kind, testKind.K,
			)
		}
	}

	// Test GetSerialsByRange with an author filter
	authorFilter := &filter.F{
		Authors: tag.NewFromBytesSlice(savedEvents[1].Pubkey),
	}

	// Get the indexes from the filter
	ranges, err = GetIndexesFromFilter(authorFilter)
	if err != nil {
		t.Fatalf("Failed to get indexes from filter: %v", err)
	}

	// Verify we got at least one range
	if len(ranges) == 0 {
		t.Fatal("Expected at least one range from filter, but got none")
	}

	// Test GetSerialsByRange with the first range
	serials, err = db.GetSerialsByRange(ranges[0])
	if err != nil {
		t.Fatalf("Failed to get serials by range: %v", err)
	}

	// Verify we got results
	if len(serials) == 0 {
		t.Fatal("Expected serials for events from author, but got none")
	}

	// Verify the serials correspond to events with the correct author
	for i, serial := range serials {
		// Fetch the event using the serial
		ev, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch event for serial %d: %v", i, err)
		}

		if !utils.FastEqual(ev.Pubkey, savedEvents[1].Pubkey) {
			t.Fatalf(
				"Event %d has incorrect author. Got %x, expected %x",
				i, ev.Pubkey, savedEvents[1].Pubkey,
			)
		}
	}
}
