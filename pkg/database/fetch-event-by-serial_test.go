package database

import (
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/utils"
)

func TestFetchEventBySerial(t *testing.T) {
	// Use shared database (skips in short mode)
	db, ctx := GetSharedDB(t)
	savedEvents := GetSharedEvents(t)

	if len(savedEvents) < 4 {
		t.Fatalf("Need at least 4 saved events, got %d", len(savedEvents))
	}
	testEvent := savedEvents[3]

	// Use QueryForIds to get the serial for this event
	sers, err := db.QueryForSerials(
		ctx, &filter.F{
			Ids: tag.NewFromBytesSlice(testEvent.ID),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for Ids: %v", err)
	}

	// Verify we got exactly one result
	if len(sers) != 1 {
		t.Fatalf("Expected 1 serial, got %d", len(sers))
	}

	// Fetch the event by serial
	fetchedEvent, err := db.FetchEventBySerial(sers[0])
	if err != nil {
		t.Fatalf("Failed to fetch event by serial: %v", err)
	}

	// Verify the fetched event is not nil
	if fetchedEvent == nil {
		t.Fatal("Expected fetched event to be non-nil, but got nil")
	}

	// Verify the fetched event has the same ID as the original event
	if !utils.FastEqual(fetchedEvent.ID, testEvent.ID) {
		t.Fatalf(
			"Fetched event ID doesn't match original event ID. Got %x, expected %x",
			fetchedEvent.ID, testEvent.ID,
		)
	}

	// Verify other event properties match
	if fetchedEvent.Kind != testEvent.Kind {
		t.Fatalf(
			"Fetched event kind doesn't match. Got %d, expected %d",
			fetchedEvent.Kind, testEvent.Kind,
		)
	}

	if !utils.FastEqual(fetchedEvent.Pubkey, testEvent.Pubkey) {
		t.Fatalf(
			"Fetched event pubkey doesn't match. Got %x, expected %x",
			fetchedEvent.Pubkey, testEvent.Pubkey,
		)
	}

	if fetchedEvent.CreatedAt != testEvent.CreatedAt {
		t.Fatalf(
			"Fetched event created_at doesn't match. Got %d, expected %d",
			fetchedEvent.CreatedAt, testEvent.CreatedAt,
		)
	}

	// Test with a non-existent serial
	nonExistentSerial := new(types.Uint40)
	err = nonExistentSerial.Set(uint64(0xFFFFFFFFFF)) // Max value
	if err != nil {
		t.Fatalf("Failed to create non-existent serial: %v", err)
	}

	// This should return an error since the serial doesn't exist
	fetchedEvent, err = db.FetchEventBySerial(nonExistentSerial)
	if err == nil {
		t.Fatal("Expected error for non-existent serial, but got nil")
	}

	// The fetched event should be nil
	if fetchedEvent != nil {
		t.Fatalf(
			"Expected nil event for non-existent serial, but got: %v",
			fetchedEvent,
		)
	}
}
