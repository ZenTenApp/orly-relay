//go:build js && wasm

package wasmdb

import (
	"bytes"
	"context"
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/database/indexes/types"
)

// TestDatabaseOpen tests that we can open an IndexedDB database
func TestDatabaseOpen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new database instance
	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Wait for the database to be ready
	select {
	case <-db.Ready():
		t.Log("Database ready")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for database to be ready")
	}

	t.Log("Database opened successfully")
}

// TestDatabaseMetaStorage tests storing and retrieving metadata
func TestDatabaseMetaStorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Test SetMarker and GetMarker
	testKey := "test_key"
	testValue := []byte("test_value_12345")

	err = db.SetMarker(testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to set marker: %v", err)
	}

	retrieved, err := db.GetMarker(testKey)
	if err != nil {
		t.Fatalf("Failed to get marker: %v", err)
	}

	if string(retrieved) != string(testValue) {
		t.Errorf("Retrieved value %q doesn't match expected %q", retrieved, testValue)
	}

	// Test HasMarker
	if !db.HasMarker(testKey) {
		t.Error("HasMarker returned false for existing key")
	}

	if db.HasMarker("nonexistent_key") {
		t.Error("HasMarker returned true for nonexistent key")
	}

	// Test DeleteMarker
	err = db.DeleteMarker(testKey)
	if err != nil {
		t.Fatalf("Failed to delete marker: %v", err)
	}

	if db.HasMarker(testKey) {
		t.Error("HasMarker returned true after deletion")
	}
}

// TestDatabaseSerialCounters tests the serial number generation
func TestDatabaseSerialCounters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Generate some serials and verify they're incrementing
	serial1, err := db.nextEventSerial()
	if err != nil {
		t.Fatalf("Failed to get first serial: %v", err)
	}

	serial2, err := db.nextEventSerial()
	if err != nil {
		t.Fatalf("Failed to get second serial: %v", err)
	}

	if serial2 != serial1+1 {
		t.Errorf("Serials not incrementing: got %d and %d", serial1, serial2)
	}

	t.Logf("Generated serials: %d, %d", serial1, serial2)
}

// TestDatabaseRelayIdentity tests relay identity key management
func TestDatabaseRelayIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// First call should create a new identity
	skb, err := db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		t.Fatalf("Failed to get/create relay identity: %v", err)
	}

	if len(skb) != 32 {
		t.Errorf("Expected 32-byte secret key, got %d bytes", len(skb))
	}

	// Second call should return the same key
	skb2, err := db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		t.Fatalf("Failed to get relay identity second time: %v", err)
	}

	if string(skb) != string(skb2) {
		t.Error("GetOrCreateRelayIdentitySecret returned different keys on second call")
	}

	t.Logf("Relay identity key: %x", skb[:8])
}

// TestDatabaseWipe tests the wipe functionality
func TestDatabaseWipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Store some data
	err = db.SetMarker("wipe_test_key", []byte("wipe_test_value"))
	if err != nil {
		t.Fatalf("Failed to set marker: %v", err)
	}

	// Wipe the database
	err = db.Wipe()
	if err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Verify data is gone
	if db.HasMarker("wipe_test_key") {
		t.Error("Marker still exists after wipe")
	}

	t.Log("Database wipe successful")
}

// createTestEvent creates a test event with fake ID and signature (for storage testing only)
func createTestEvent(kind uint16, content string, pubkey []byte) *event.E {
	ev := event.New()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.CreatedAt = time.Now().Unix()
	ev.Tags = tag.NewS()

	// Use provided pubkey or generate a fake one
	if len(pubkey) == 32 {
		ev.Pubkey = pubkey
	} else {
		ev.Pubkey = make([]byte, 32)
		for i := range ev.Pubkey {
			ev.Pubkey[i] = byte(i)
		}
	}

	// Generate a fake ID (normally would be SHA256 of serialized event)
	ev.ID = make([]byte, 32)
	for i := range ev.ID {
		ev.ID[i] = byte(i + 100)
	}

	// Generate a fake signature (won't verify, but storage doesn't need to verify)
	ev.Sig = make([]byte, 64)
	for i := range ev.Sig {
		ev.Sig[i] = byte(i + 200)
	}

	return ev
}

// TestSaveAndFetchEvent tests saving and fetching events
func TestSaveAndFetchEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wipe the database to start fresh
	db, err := New(ctx, cancel, "/tmp/test", "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create a test event
	ev := createTestEvent(1, "Hello, Nostr!", nil)
	t.Logf("Created event with ID: %x", ev.ID[:8])

	// Save the event
	replaced, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}
	if replaced {
		t.Error("Expected replaced to be false for new event")
	}

	t.Log("Event saved successfully")

	// Look up the serial by ID
	ser, err := db.GetSerialById(ev.ID)
	if err != nil {
		t.Fatalf("Failed to get serial by ID: %v", err)
	}
	if ser == nil {
		t.Fatal("Got nil serial")
	}
	t.Logf("Event serial: %d", ser.Get())

	// Fetch the event by serial
	fetchedEv, err := db.FetchEventBySerial(ser)
	if err != nil {
		t.Fatalf("Failed to fetch event by serial: %v", err)
	}
	if fetchedEv == nil {
		t.Fatal("Fetched event is nil")
	}

	// Verify the event content matches
	if !bytes.Equal(fetchedEv.ID, ev.ID) {
		t.Errorf("Event ID mismatch: got %x, want %x", fetchedEv.ID[:8], ev.ID[:8])
	}
	if !bytes.Equal(fetchedEv.Content, ev.Content) {
		t.Errorf("Event content mismatch: got %q, want %q", fetchedEv.Content, ev.Content)
	}
	if fetchedEv.Kind != ev.Kind {
		t.Errorf("Event kind mismatch: got %d, want %d", fetchedEv.Kind, ev.Kind)
	}

	t.Log("Event fetched and verified successfully")
}

// TestSaveEventDuplicate tests that saving a duplicate event returns an error
func TestSaveEventDuplicate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save a test event
	ev := createTestEvent(1, "Duplicate test", nil)

	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save first event: %v", err)
	}

	// Try to save the same event again
	_, err = db.SaveEvent(ctx, ev)
	if err == nil {
		t.Error("Expected error when saving duplicate event, got nil")
	} else {
		t.Logf("Got expected error for duplicate: %v", err)
	}
}

// TestSaveEventWithTags tests saving an event with tags
func TestSaveEventWithTags(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create an event with tags
	ev := createTestEvent(1, "Event with tags", nil)

	// Make the ID unique
	ev.ID[0] = 0x42

	// Add some tags
	ev.Tags = tag.NewS(
		tag.NewFromAny("t", "nostr"),
		tag.NewFromAny("t", "test"),
	)

	// Save the event
	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with tags: %v", err)
	}

	// Fetch it back
	ser, err := db.GetSerialById(ev.ID)
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}

	fetchedEv, err := db.FetchEventBySerial(ser)
	if err != nil {
		t.Fatalf("Failed to fetch event: %v", err)
	}

	// Verify tags
	if fetchedEv.Tags == nil || fetchedEv.Tags.Len() != 2 {
		t.Errorf("Expected 2 tags, got %v", fetchedEv.Tags)
	}

	t.Log("Event with tags saved and fetched successfully")
}

// TestFetchEventsBySerials tests batch fetching of events
func TestFetchEventsBySerials(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save multiple events
	var serials []*types.Uint40
	for i := 0; i < 3; i++ {
		ev := createTestEvent(1, "Batch test event", nil)
		ev.ID[0] = byte(0x50 + i) // Make IDs unique

		_, err := db.SaveEvent(ctx, ev)
		if err != nil {
			t.Fatalf("Failed to save event %d: %v", i, err)
		}

		ser, err := db.GetSerialById(ev.ID)
		if err != nil {
			t.Fatalf("Failed to get serial for event %d: %v", i, err)
		}
		serials = append(serials, ser)
	}

	// Batch fetch
	events, err := db.FetchEventsBySerials(serials)
	if err != nil {
		t.Fatalf("Failed to batch fetch events: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	t.Logf("Successfully batch fetched %d events", len(events))
}

// TestGetFullIdPubkeyBySerial tests getting event metadata by serial
func TestGetFullIdPubkeyBySerial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save an event
	ev := createTestEvent(1, "Metadata test", nil)
	ev.ID[0] = 0x99 // Make ID unique

	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	ser, err := db.GetSerialById(ev.ID)
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}

	// Get full ID/pubkey/timestamp metadata
	fidpk, err := db.GetFullIdPubkeyBySerial(ser)
	if err != nil {
		t.Fatalf("Failed to get full id pubkey: %v", err)
	}
	if fidpk == nil {
		t.Fatal("Got nil fidpk")
	}

	// Verify the ID matches
	if !bytes.Equal(fidpk.Id, ev.ID) {
		t.Errorf("ID mismatch: got %x, want %x", fidpk.Id[:8], ev.ID[:8])
	}

	t.Logf("Got metadata: ID=%x, Pub=%x, Ts=%d, Ser=%d",
		fidpk.Id[:8], fidpk.Pub[:4], fidpk.Ts, fidpk.Ser)
}

// TestQueryEventsByKind tests querying events by kind
func TestQueryEventsByKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create events of different kinds
	ev1 := createTestEvent(1, "Kind 1 event", nil)
	ev1.ID[0] = 0xA1
	ev2 := createTestEvent(1, "Another kind 1", nil)
	ev2.ID[0] = 0xA2
	ev3 := createTestEvent(7, "Kind 7 event", nil)
	ev3.ID[0] = 0xA3

	// Save events
	for _, ev := range []*event.E{ev1, ev2, ev3} {
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query for kind 1
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	}

	evs, err := db.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 2 {
		t.Errorf("Expected 2 events of kind 1, got %d", len(evs))
	}

	t.Logf("Query by kind 1 returned %d events", len(evs))
}

// TestQueryEventsByAuthor tests querying events by author pubkey
func TestQueryEventsByAuthor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create events from different authors
	author1 := make([]byte, 32)
	for i := range author1 {
		author1[i] = 0x11
	}
	author2 := make([]byte, 32)
	for i := range author2 {
		author2[i] = 0x22
	}

	ev1 := createTestEvent(1, "From author 1", author1)
	ev1.ID[0] = 0xB1
	ev2 := createTestEvent(1, "Also from author 1", author1)
	ev2.ID[0] = 0xB2
	ev3 := createTestEvent(1, "From author 2", author2)
	ev3.ID[0] = 0xB3

	// Save events
	for _, ev := range []*event.E{ev1, ev2, ev3} {
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query for author1
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(author1),
	}

	evs, err := db.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 2 {
		t.Errorf("Expected 2 events from author1, got %d", len(evs))
	}

	t.Logf("Query by author returned %d events", len(evs))
}

// TestCountEvents tests the event counting functionality
func TestCountEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create multiple events
	for i := 0; i < 5; i++ {
		ev := createTestEvent(1, "Count test event", nil)
		ev.ID[0] = byte(0xC0 + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Count all kind 1 events
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	}

	count, approx, err := db.CountEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected count of 5, got %d", count)
	}
	if approx {
		t.Log("Count is approximate")
	}

	t.Logf("CountEvents returned %d", count)
}

// TestQueryEventsWithLimit tests applying a limit to query results
func TestQueryEventsWithLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create 10 events
	for i := 0; i < 10; i++ {
		ev := createTestEvent(1, "Limit test event", nil)
		ev.ID[0] = byte(0xD0 + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query with limit of 3
	limit := uint(3)
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
		Limit: &limit,
	}

	evs, err := db.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 3 {
		t.Errorf("Expected 3 events with limit, got %d", len(evs))
	}

	t.Logf("Query with limit returned %d events", len(evs))
}

// TestQueryEventsByTag tests querying events by tag
func TestQueryEventsByTag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create events with different tags
	ev1 := createTestEvent(1, "Has bitcoin tag", nil)
	ev1.ID[0] = 0xE1
	ev1.Tags = tag.NewS(
		tag.NewFromAny("t", "bitcoin"),
	)

	ev2 := createTestEvent(1, "Has nostr tag", nil)
	ev2.ID[0] = 0xE2
	ev2.Tags = tag.NewS(
		tag.NewFromAny("t", "nostr"),
	)

	ev3 := createTestEvent(1, "Has bitcoin and nostr tags", nil)
	ev3.ID[0] = 0xE3
	ev3.Tags = tag.NewS(
		tag.NewFromAny("t", "bitcoin"),
		tag.NewFromAny("t", "nostr"),
	)

	// Save events
	for _, ev := range []*event.E{ev1, ev2, ev3} {
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query for bitcoin tag
	f := &filter.F{
		Tags: tag.NewS(
			tag.NewFromAny("#t", "bitcoin"),
		),
	}

	evs, err := db.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 2 {
		t.Errorf("Expected 2 events with bitcoin tag, got %d", len(evs))
	}

	t.Logf("Query by tag returned %d events", len(evs))
}

// TestQueryForSerials tests the QueryForSerials method
func TestQueryForSerials(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save events
	for i := 0; i < 3; i++ {
		ev := createTestEvent(1, "Serial test", nil)
		ev.ID[0] = byte(0xF0 + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query for serials
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	}

	sers, err := db.QueryForSerials(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query for serials: %v", err)
	}

	if len(sers) != 3 {
		t.Errorf("Expected 3 serials, got %d", len(sers))
	}

	t.Logf("QueryForSerials returned %d serials", len(sers))
}

// TestDeleteEvent tests deleting an event by ID
func TestDeleteEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save an event
	ev := createTestEvent(1, "Event to delete", nil)
	ev.ID[0] = 0xDE

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify it exists
	ser, err := db.GetSerialById(ev.ID)
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}
	if ser == nil {
		t.Fatal("Serial should not be nil after save")
	}

	// Delete the event
	if err := db.DeleteEvent(ctx, ev.ID); err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify it no longer exists
	ser, err = db.GetSerialById(ev.ID)
	if err == nil && ser != nil {
		t.Error("Event should not exist after deletion")
	}

	t.Log("Event deleted successfully")
}

// TestDeleteEventBySerial tests deleting an event by serial number
func TestDeleteEventBySerial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save an event
	ev := createTestEvent(1, "Event to delete by serial", nil)
	ev.ID[0] = 0xDF

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get the serial
	ser, err := db.GetSerialById(ev.ID)
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}

	// Fetch the event
	fetchedEv, err := db.FetchEventBySerial(ser)
	if err != nil {
		t.Fatalf("Failed to fetch event: %v", err)
	}

	// Delete by serial
	if err := db.DeleteEventBySerial(ctx, ser, fetchedEv); err != nil {
		t.Fatalf("Failed to delete event by serial: %v", err)
	}

	// Verify event is gone
	fetchedEv, err = db.FetchEventBySerial(ser)
	if err == nil && fetchedEv != nil {
		t.Error("Event should not exist after deletion by serial")
	}

	t.Log("Event deleted by serial successfully")
}

// TestCheckForDeleted tests that deleted events are properly detected
func TestCheckForDeleted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create a regular event
	ev := createTestEvent(1, "Event that will be deleted", nil)
	ev.ID[0] = 0xDD
	ev.CreatedAt = time.Now().Unix() - 100 // 100 seconds ago

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Create a kind 5 deletion event referencing it
	// Use binary format for the e-tag value since GetIndexesForEvent hashes the raw value,
	// and NormalizeTagValueForHash converts hex to binary before hashing in filters
	deleteEv := createTestEvent(5, "", ev.Pubkey)
	deleteEv.ID[0] = 0xD5
	deleteEv.CreatedAt = time.Now().Unix() // Now

	// Store the e-tag with binary value in the format matching JSON unmarshal
	// The nostr library stores e/p tag values as 33 bytes (32 bytes + null terminator)
	eTagValue := make([]byte, 33)
	copy(eTagValue[:32], ev.ID)
	eTagValue[32] = 0 // null terminator to match nostr library's binary format
	deleteEv.Tags = tag.NewS(
		tag.NewFromAny("e", string(eTagValue)),
	)

	if _, err := db.SaveEvent(ctx, deleteEv); err != nil {
		t.Fatalf("Failed to save delete event: %v", err)
	}

	// Check if the original event is marked as deleted
	err = db.CheckForDeleted(ev, nil)
	if err == nil {
		t.Error("Expected error indicating event was deleted")
	} else {
		t.Logf("CheckForDeleted correctly detected deletion: %v", err)
	}
}

// ============================================================================
// NIP-43 Membership Tests
// ============================================================================

// TestNIP43AddAndRemoveMember tests adding and removing NIP-43 members
func TestNIP43AddAndRemoveMember(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create a test pubkey
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 1)
	}

	// Initially should not be a member
	isMember, err := db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("Failed to check member status: %v", err)
	}
	if isMember {
		t.Error("Expected non-member initially")
	}

	// Add member with invite code
	inviteCode := "TEST-INVITE-123"
	if err := db.AddNIP43Member(pubkey, inviteCode); err != nil {
		t.Fatalf("Failed to add NIP-43 member: %v", err)
	}

	// Should now be a member
	isMember, err = db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("Failed to check member status: %v", err)
	}
	if !isMember {
		t.Error("Expected to be a member after adding")
	}

	// Get membership details
	membership, err := db.GetNIP43Membership(pubkey)
	if err != nil {
		t.Fatalf("Failed to get membership: %v", err)
	}
	if membership.InviteCode != inviteCode {
		t.Errorf("Invite code mismatch: got %s, want %s", membership.InviteCode, inviteCode)
	}
	if !bytes.Equal(membership.Pubkey, pubkey) {
		t.Error("Pubkey mismatch in membership")
	}

	// Remove member
	if err := db.RemoveNIP43Member(pubkey); err != nil {
		t.Fatalf("Failed to remove member: %v", err)
	}

	// Should no longer be a member
	isMember, err = db.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("Failed to check member status: %v", err)
	}
	if isMember {
		t.Error("Expected non-member after removal")
	}

	t.Log("NIP-43 add/remove member test passed")
}

// TestNIP43GetAllMembers tests retrieving all members
func TestNIP43GetAllMembers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Add multiple members
	pubkeys := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		pubkeys[i] = make([]byte, 32)
		for j := range pubkeys[i] {
			pubkeys[i][j] = byte((i+1)*10 + j)
		}
		if err := db.AddNIP43Member(pubkeys[i], "invite"+string(rune('A'+i))); err != nil {
			t.Fatalf("Failed to add member %d: %v", i, err)
		}
	}

	// Get all members
	members, err := db.GetAllNIP43Members()
	if err != nil {
		t.Fatalf("Failed to get all members: %v", err)
	}

	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}

	t.Logf("Retrieved %d NIP-43 members", len(members))
}

// TestNIP43InviteCode tests invite code functionality
func TestNIP43InviteCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Store a valid invite code (expires in 24 hours)
	validCode := "VALID-CODE-ABC"
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := db.StoreInviteCode(validCode, expiresAt); err != nil {
		t.Fatalf("Failed to store invite code: %v", err)
	}

	// Validate the code
	valid, err := db.ValidateInviteCode(validCode)
	if err != nil {
		t.Fatalf("Failed to validate invite code: %v", err)
	}
	if !valid {
		t.Error("Expected valid invite code to be valid")
	}

	// Check non-existent code
	valid, err = db.ValidateInviteCode("NONEXISTENT")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if valid {
		t.Error("Expected non-existent code to be invalid")
	}

	// Delete the code
	if err := db.DeleteInviteCode(validCode); err != nil {
		t.Fatalf("Failed to delete invite code: %v", err)
	}

	// Should now be invalid
	valid, err = db.ValidateInviteCode(validCode)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if valid {
		t.Error("Expected deleted code to be invalid")
	}

	t.Log("NIP-43 invite code test passed")
}

// TestNIP43ExpiredInviteCode tests that expired invite codes are invalid
func TestNIP43ExpiredInviteCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Store an expired invite code (expired 1 hour ago)
	expiredCode := "EXPIRED-CODE-XYZ"
	expiresAt := time.Now().Add(-1 * time.Hour)
	if err := db.StoreInviteCode(expiredCode, expiresAt); err != nil {
		t.Fatalf("Failed to store invite code: %v", err)
	}

	// Validate the expired code
	valid, err := db.ValidateInviteCode(expiredCode)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if valid {
		t.Error("Expected expired invite code to be invalid")
	}

	t.Log("Expired invite code correctly detected as invalid")
}

// TestNIP43InvalidPubkeyLength tests that invalid pubkey lengths are rejected
func TestNIP43InvalidPubkeyLength(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Try to add a member with invalid pubkey length
	shortPubkey := make([]byte, 16) // Should be 32
	err = db.AddNIP43Member(shortPubkey, "test")
	if err == nil {
		t.Error("Expected error for invalid pubkey length")
	}

	t.Logf("Correctly rejected invalid pubkey length: %v", err)
}

// ============================================================================
// Subscription Management Tests
// ============================================================================

// TestSubscriptionExtend tests extending subscriptions
func TestSubscriptionExtend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 50)
	}

	// Initially no subscription
	sub, err := db.GetSubscription(pubkey)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}
	if sub != nil {
		t.Error("Expected nil subscription initially")
	}

	// Extend subscription by 30 days
	if err := db.ExtendSubscription(pubkey, 30); err != nil {
		t.Fatalf("Failed to extend subscription: %v", err)
	}

	// Should now have a subscription
	sub, err = db.GetSubscription(pubkey)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}
	if sub == nil {
		t.Fatal("Expected subscription after extension")
	}

	// Verify paid until is in the future
	if sub.PaidUntil.Before(time.Now()) {
		t.Error("PaidUntil should be in the future")
	}

	// Extend again
	if err := db.ExtendSubscription(pubkey, 15); err != nil {
		t.Fatalf("Failed to extend subscription again: %v", err)
	}

	sub2, err := db.GetSubscription(pubkey)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}

	// Second extension should add to first
	if !sub2.PaidUntil.After(sub.PaidUntil) {
		t.Error("Expected PaidUntil to increase after second extension")
	}

	t.Log("Subscription extension test passed")
}

// TestSubscriptionActive tests checking if subscription is active
func TestSubscriptionActive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 60)
	}

	// First check creates a trial subscription
	active, err := db.IsSubscriptionActive(pubkey)
	if err != nil {
		t.Fatalf("Failed to check subscription: %v", err)
	}
	if !active {
		t.Error("Expected new user to have active trial subscription")
	}

	// Second check should still be active
	active, err = db.IsSubscriptionActive(pubkey)
	if err != nil {
		t.Fatalf("Failed to check subscription: %v", err)
	}
	if !active {
		t.Error("Expected subscription to remain active")
	}

	t.Log("Subscription active check passed")
}

// TestBlossomSubscription tests blossom storage subscription
func TestBlossomSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 70)
	}

	// Initially no quota
	quota, err := db.GetBlossomStorageQuota(pubkey)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}
	if quota != 0 {
		t.Error("Expected zero quota initially")
	}

	// Add blossom subscription
	if err := db.ExtendBlossomSubscription(pubkey, "premium", 1024, 30); err != nil {
		t.Fatalf("Failed to extend blossom subscription: %v", err)
	}

	// Check quota
	quota, err = db.GetBlossomStorageQuota(pubkey)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}
	if quota != 1024 {
		t.Errorf("Expected quota of 1024, got %d", quota)
	}

	// Check subscription details
	sub, err := db.GetSubscription(pubkey)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}
	if sub.BlossomLevel != "premium" {
		t.Errorf("Expected premium level, got %s", sub.BlossomLevel)
	}

	t.Log("Blossom subscription test passed")
}

// TestPaymentHistory tests recording and retrieving payment history
func TestPaymentHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 80)
	}

	// Initially no payment history
	payments, err := db.GetPaymentHistory(pubkey)
	if err != nil {
		t.Fatalf("Failed to get payment history: %v", err)
	}
	if len(payments) != 0 {
		t.Error("Expected empty payment history initially")
	}

	// Record a payment
	if err := db.RecordPayment(pubkey, 10000, "lnbc100...", "preimage123"); err != nil {
		t.Fatalf("Failed to record payment: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Record another payment
	if err := db.RecordPayment(pubkey, 20000, "lnbc200...", "preimage456"); err != nil {
		t.Fatalf("Failed to record payment: %v", err)
	}

	// Check payment history
	payments, err = db.GetPaymentHistory(pubkey)
	if err != nil {
		t.Fatalf("Failed to get payment history: %v", err)
	}
	if len(payments) != 2 {
		t.Errorf("Expected 2 payments, got %d", len(payments))
	}

	t.Logf("Retrieved %d payments from history", len(payments))
}

// TestIsFirstTimeUser tests first-time user detection
func TestIsFirstTimeUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 90)
	}

	// First check should be true
	isFirst, err := db.IsFirstTimeUser(pubkey)
	if err != nil {
		t.Fatalf("Failed to check first time user: %v", err)
	}
	if !isFirst {
		t.Error("Expected true for first check")
	}

	// Second check should be false
	isFirst, err = db.IsFirstTimeUser(pubkey)
	if err != nil {
		t.Fatalf("Failed to check first time user: %v", err)
	}
	if isFirst {
		t.Error("Expected false for second check")
	}

	t.Log("First time user detection test passed")
}

// TestSubscriptionInvalidDays tests that invalid day counts are rejected
func TestSubscriptionInvalidDays(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	pubkey := make([]byte, 32)

	// Try to extend with 0 days
	err = db.ExtendSubscription(pubkey, 0)
	if err == nil {
		t.Error("Expected error for 0 days")
	}

	// Try to extend with negative days
	err = db.ExtendSubscription(pubkey, -5)
	if err == nil {
		t.Error("Expected error for negative days")
	}

	t.Log("Invalid days correctly rejected")
}

// ============================================================================
// Import/Export Tests
// ============================================================================

// TestImportExport tests basic import/export functionality
func TestImportExport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create and save some events
	for i := 0; i < 3; i++ {
		ev := createTestEvent(1, "Export test event", nil)
		ev.ID[0] = byte(0xAA + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Export to buffer
	var exportBuf bytes.Buffer
	db.Export(ctx, &exportBuf)

	exportData := exportBuf.String()
	if len(exportData) == 0 {
		t.Error("Expected non-empty export data")
	}

	// Count lines (should be 3 JSONL lines)
	lines := bytes.Count(exportBuf.Bytes(), []byte("\n"))
	if lines != 3 {
		t.Errorf("Expected 3 export lines, got %d", lines)
	}

	t.Logf("Exported %d events to %d bytes", lines, len(exportData))
}

// TestImportFromReader tests importing events from JSONL reader
func TestImportFromReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create JSONL import data
	// Note: These are simplified test events - real events would have valid signatures
	jsonl := `{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","pubkey":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","created_at":1700000000,"kind":1,"tags":[],"content":"Test event 1","sig":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
{"id":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","pubkey":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","created_at":1700000001,"kind":1,"tags":[],"content":"Test event 2","sig":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}
`

	reader := bytes.NewReader([]byte(jsonl))
	err = db.ImportEventsFromReader(ctx, reader)
	if err != nil {
		t.Fatalf("Failed to import events: %v", err)
	}

	t.Log("Import from reader test completed")
}

// TestImportExportRoundTrip tests full round-trip import/export
func TestImportExportRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create first database
	db1, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database 1: %v", err)
	}
	defer db1.Close()

	<-db1.Ready()

	// Wipe to ensure clean state
	if err := db1.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create events with specific content for verification
	// Using kinds 1, 7, 10, 20, 30 to avoid kind 3 which requires p tags
	kinds := []uint16{1, 7, 10, 20, 30}
	originalEvents := make([]*event.E, 5)
	for i := 0; i < 5; i++ {
		ev := createTestEvent(kinds[i], "Round trip content "+string(rune('A'+i)), nil)
		ev.ID[0] = byte(0xBB + i)
		originalEvents[i] = ev
		if _, err := db1.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Export from db1
	var exportBuf bytes.Buffer
	db1.Export(ctx, &exportBuf)

	// Wipe db1 (simulating a fresh database)
	if err := db1.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Import back
	db1.Import(&exportBuf)

	// Query all events
	f := &filter.F{}
	evs, err := db1.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) < 5 {
		t.Errorf("Expected at least 5 events after round trip, got %d", len(evs))
	}

	t.Logf("Round trip test: %d events survived", len(evs))
}

// TestGetSerialsByPubkey tests retrieving serials by pubkey
func TestGetSerialsByPubkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create a specific author
	author := make([]byte, 32)
	for i := range author {
		author[i] = 0xCC
	}

	// Create events from this author
	for i := 0; i < 3; i++ {
		ev := createTestEvent(1, "Author test", author)
		ev.ID[0] = byte(0xCC + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Get serials by pubkey
	serials, err := db.GetSerialsByPubkey(author)
	if err != nil {
		t.Fatalf("Failed to get serials by pubkey: %v", err)
	}

	if len(serials) != 3 {
		t.Errorf("Expected 3 serials, got %d", len(serials))
	}

	t.Logf("GetSerialsByPubkey returned %d serials", len(serials))
}

// TestExportByPubkey tests exporting events filtered by pubkey
func TestExportByPubkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := New(ctx, cancel, "/tmp/test", "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Create two different authors
	author1 := make([]byte, 32)
	for i := range author1 {
		author1[i] = 0xDD
	}
	author2 := make([]byte, 32)
	for i := range author2 {
		author2[i] = 0xEE
	}

	// Create events from both authors
	for i := 0; i < 2; i++ {
		ev := createTestEvent(1, "Author 1 content", author1)
		ev.ID[0] = byte(0xD0 + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		ev := createTestEvent(1, "Author 2 content", author2)
		ev.ID[0] = byte(0xE0 + i)
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Export only author1's events
	var exportBuf bytes.Buffer
	db.Export(ctx, &exportBuf, author1)

	lines := bytes.Count(exportBuf.Bytes(), []byte("\n"))
	if lines != 2 {
		t.Errorf("Expected 2 export lines for author1, got %d", lines)
	}

	t.Logf("Exported %d events for specific pubkey", lines)
}

