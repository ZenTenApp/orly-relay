package neo4j

import (
	"context"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/database/indexes/types"
)

func TestFetchEventBySerial(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save a test event
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Test event for fetch by serial")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get the serial for this event
	serial, err := db.GetSerialById(ev.ID[:])
	if err != nil {
		t.Fatalf("Failed to get serial by ID: %v", err)
	}

	// Fetch event by serial
	fetchedEvent, err := db.FetchEventBySerial(serial)
	if err != nil {
		t.Fatalf("Failed to fetch event by serial: %v", err)
	}

	if fetchedEvent == nil {
		t.Fatal("Expected fetched event to be non-nil")
	}

	// Verify event properties
	if hex.Enc(fetchedEvent.ID[:]) != hex.Enc(ev.ID[:]) {
		t.Fatalf("Event ID mismatch: got %s, expected %s",
			hex.Enc(fetchedEvent.ID[:]), hex.Enc(ev.ID[:]))
	}

	if fetchedEvent.Kind != ev.Kind {
		t.Fatalf("Kind mismatch: got %d, expected %d", fetchedEvent.Kind, ev.Kind)
	}

	if hex.Enc(fetchedEvent.Pubkey[:]) != hex.Enc(ev.Pubkey[:]) {
		t.Fatalf("Pubkey mismatch")
	}

	if fetchedEvent.CreatedAt != ev.CreatedAt {
		t.Fatalf("CreatedAt mismatch: got %d, expected %d",
			fetchedEvent.CreatedAt, ev.CreatedAt)
	}

	t.Logf("✓ FetchEventBySerial returned correct event")
}

func TestFetchEventBySerial_NonExistent(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Try to fetch with non-existent serial
	nonExistentSerial := &types.Uint40{}
	nonExistentSerial.Set(0xFFFFFFFFFF) // Max value

	_, err = db.FetchEventBySerial(nonExistentSerial)
	if err == nil {
		t.Fatal("Expected error for non-existent serial")
	}

	t.Logf("✓ FetchEventBySerial correctly returned error for non-existent serial")
}

func TestFetchEventsBySerials(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save multiple events
	var serials []*types.Uint40
	eventIDs := make(map[uint64]string)

	for i := 0; i < 5; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Test event")

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}

		serial, err := db.GetSerialById(ev.ID[:])
		if err != nil {
			t.Fatalf("Failed to get serial: %v", err)
		}

		serials = append(serials, serial)
		eventIDs[serial.Get()] = hex.Enc(ev.ID[:])
	}

	// Fetch all events by serials
	events, err := db.FetchEventsBySerials(serials)
	if err != nil {
		t.Fatalf("Failed to fetch events by serials: %v", err)
	}

	if len(events) != 5 {
		t.Fatalf("Expected 5 events, got %d", len(events))
	}

	// Verify each event
	for serial, expectedID := range eventIDs {
		ev, exists := events[serial]
		if !exists {
			t.Fatalf("Event with serial %d not found", serial)
		}
		if hex.Enc(ev.ID[:]) != expectedID {
			t.Fatalf("Event ID mismatch for serial %d", serial)
		}
	}

	t.Logf("✓ FetchEventsBySerials returned %d correct events", len(events))
}

func TestGetSerialById(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save event
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Test event")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get serial by ID
	serial, err := db.GetSerialById(ev.ID[:])
	if err != nil {
		t.Fatalf("Failed to get serial by ID: %v", err)
	}

	if serial == nil {
		t.Fatal("Expected serial to be non-nil")
	}

	if serial.Get() == 0 {
		t.Fatal("Expected non-zero serial")
	}

	t.Logf("✓ GetSerialById returned serial: %d", serial.Get())
}

func TestGetSerialById_NonExistent(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Try to get serial for non-existent event
	fakeID, _ := hex.Dec("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	_, err = db.GetSerialById(fakeID)
	if err == nil {
		t.Fatal("Expected error for non-existent event ID")
	}

	t.Logf("✓ GetSerialById correctly returned error for non-existent ID")
}

func TestGetSerialsByIds(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save multiple events
	ids := tag.NewS()
	for i := 0; i < 3; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Test event")

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}

		ids.Append(tag.NewFromAny("", hex.Enc(ev.ID[:])))
	}

	// Get serials by IDs
	serials, err := db.GetSerialsByIds(ids)
	if err != nil {
		t.Fatalf("Failed to get serials by IDs: %v", err)
	}

	if len(serials) != 3 {
		t.Fatalf("Expected 3 serials, got %d", len(serials))
	}

	t.Logf("✓ GetSerialsByIds returned %d serials", len(serials))
}

func TestGetFullIdPubkeyBySerial(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save event
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Test event")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get serial
	serial, err := db.GetSerialById(ev.ID[:])
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}

	// Get full ID and pubkey
	idPkTs, err := db.GetFullIdPubkeyBySerial(serial)
	if err != nil {
		t.Fatalf("Failed to get full ID and pubkey: %v", err)
	}

	if idPkTs == nil {
		t.Fatal("Expected non-nil result")
	}

	if hex.Enc(idPkTs.Id) != hex.Enc(ev.ID[:]) {
		t.Fatalf("ID mismatch")
	}

	if hex.Enc(idPkTs.Pub) != hex.Enc(ev.Pubkey[:]) {
		t.Fatalf("Pubkey mismatch")
	}

	if idPkTs.Ts != ev.CreatedAt {
		t.Fatalf("Timestamp mismatch")
	}

	t.Logf("✓ GetFullIdPubkeyBySerial returned correct data")
}

func TestQueryForSerials(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save events
	for i := 0; i < 5; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Test event")

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query for serials
	serials, err := db.QueryForSerials(ctx, &filter.F{
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query for serials: %v", err)
	}

	if len(serials) != 5 {
		t.Fatalf("Expected 5 serials, got %d", len(serials))
	}

	t.Logf("✓ QueryForSerials returned %d serials", len(serials))
}
