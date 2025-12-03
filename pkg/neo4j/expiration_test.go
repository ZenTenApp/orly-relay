package neo4j

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func TestExpiration_SaveEventWithExpiration(t *testing.T) {
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

	// Create event with expiration tag (expires in 1 hour)
	futureExpiration := time.Now().Add(1 * time.Hour).Unix()

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Event with expiration")
	ev.Tags = tag.NewS(tag.NewFromAny("expiration", timestamp.FromUnix(futureExpiration).String()))

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Query the event to verify it was saved
	evs, err := db.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query event: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evs))
	}

	t.Logf("✓ Event with expiration tag saved successfully")
}

func TestExpiration_DeleteExpiredEvents(t *testing.T) {
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

	// Create an expired event (expired 1 hour ago)
	pastExpiration := time.Now().Add(-1 * time.Hour).Unix()

	expiredEv := event.New()
	expiredEv.Pubkey = signer.Pub()
	expiredEv.CreatedAt = timestamp.Now().V - 7200 // 2 hours ago
	expiredEv.Kind = 1
	expiredEv.Content = []byte("Expired event")
	expiredEv.Tags = tag.NewS(tag.NewFromAny("expiration", timestamp.FromUnix(pastExpiration).String()))

	if err := expiredEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign expired event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, expiredEv); err != nil {
		t.Fatalf("Failed to save expired event: %v", err)
	}

	// Create a non-expired event (expires in 1 hour)
	futureExpiration := time.Now().Add(1 * time.Hour).Unix()

	validEv := event.New()
	validEv.Pubkey = signer.Pub()
	validEv.CreatedAt = timestamp.Now().V
	validEv.Kind = 1
	validEv.Content = []byte("Valid event")
	validEv.Tags = tag.NewS(tag.NewFromAny("expiration", timestamp.FromUnix(futureExpiration).String()))

	if err := validEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign valid event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, validEv); err != nil {
		t.Fatalf("Failed to save valid event: %v", err)
	}

	// Create an event without expiration
	permanentEv := event.New()
	permanentEv.Pubkey = signer.Pub()
	permanentEv.CreatedAt = timestamp.Now().V + 1
	permanentEv.Kind = 1
	permanentEv.Content = []byte("Permanent event (no expiration)")

	if err := permanentEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign permanent event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, permanentEv); err != nil {
		t.Fatalf("Failed to save permanent event: %v", err)
	}

	// Verify all 3 events exist
	evs, err := db.QueryEvents(ctx, &filter.F{
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("Expected 3 events before deletion, got %d", len(evs))
	}

	// Run DeleteExpired
	db.DeleteExpired()

	// Verify only expired event was deleted
	evs, err = db.QueryEvents(ctx, &filter.F{
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query events after deletion: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 events after deletion (expired removed), got %d", len(evs))
	}

	// Verify the correct events remain
	foundValid := false
	foundPermanent := false
	for _, ev := range evs {
		if hex.Enc(ev.ID[:]) == hex.Enc(validEv.ID[:]) {
			foundValid = true
		}
		if hex.Enc(ev.ID[:]) == hex.Enc(permanentEv.ID[:]) {
			foundPermanent = true
		}
	}

	if !foundValid {
		t.Fatal("Valid event (with future expiration) was incorrectly deleted")
	}
	if !foundPermanent {
		t.Fatal("Permanent event (no expiration) was incorrectly deleted")
	}

	t.Logf("✓ DeleteExpired correctly removed only expired events")
}

func TestExpiration_NoExpirationTag(t *testing.T) {
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

	// Create event without expiration tag
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Event without expiration")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Run DeleteExpired - event should not be deleted
	db.DeleteExpired()

	// Verify event still exists
	evs, err := db.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query event: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 event (no expiration should not be deleted), got %d", len(evs))
	}

	t.Logf("✓ Events without expiration tag are not deleted")
}

func TestExport_AllEvents(t *testing.T) {
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

	// Create and save some events
	for i := 0; i < 5; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Test event for export")
		ev.Tags = tag.NewS(tag.NewFromAny("t", "test"))

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Export all events
	var buf bytes.Buffer
	db.Export(ctx, &buf)

	// Parse the exported JSONL
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	validLines := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev event.E
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("Failed to parse exported event: %v", err)
		}
		validLines++
	}

	if validLines != 5 {
		t.Fatalf("Expected 5 exported events, got %d", validLines)
	}

	t.Logf("✓ Export all events returned %d events in JSONL format", validLines)
}

func TestExport_FilterByPubkey(t *testing.T) {
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

	// Create two signers
	alice, _ := p8k.New()
	alice.Generate()

	bob, _ := p8k.New()
	bob.Generate()

	baseTs := timestamp.Now().V

	// Create events from Alice
	for i := 0; i < 3; i++ {
		ev := event.New()
		ev.Pubkey = alice.Pub()
		ev.CreatedAt = baseTs + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Alice's event")

		if err := ev.Sign(alice); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Create events from Bob
	for i := 0; i < 2; i++ {
		ev := event.New()
		ev.Pubkey = bob.Pub()
		ev.CreatedAt = baseTs + int64(i) + 10
		ev.Kind = 1
		ev.Content = []byte("Bob's event")

		if err := ev.Sign(bob); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Export only Alice's events
	var buf bytes.Buffer
	db.Export(ctx, &buf, alice.Pub())

	// Parse the exported JSONL
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	validLines := 0
	alicePubkey := hex.Enc(alice.Pub())
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev event.E
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("Failed to parse exported event: %v", err)
		}
		if hex.Enc(ev.Pubkey[:]) != alicePubkey {
			t.Fatalf("Exported event has wrong pubkey (expected Alice)")
		}
		validLines++
	}

	if validLines != 3 {
		t.Fatalf("Expected 3 events from Alice, got %d", validLines)
	}

	t.Logf("✓ Export with pubkey filter returned %d events from Alice only", validLines)
}

func TestExport_Empty(t *testing.T) {
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

	// Export from empty database
	var buf bytes.Buffer
	db.Export(ctx, &buf)

	// Should be empty or just whitespace
	content := bytes.TrimSpace(buf.Bytes())
	if len(content) != 0 {
		t.Fatalf("Expected empty export, got: %s", string(content))
	}

	t.Logf("✓ Export from empty database returns empty result")
}

func TestImportExport_RoundTrip(t *testing.T) {
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

	signer, _ := p8k.New()
	signer.Generate()

	// Create original events
	originalEvents := make([]*event.E, 3)
	for i := 0; i < 3; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte("Round trip test event")
		ev.Tags = tag.NewS(tag.NewFromAny("t", "roundtrip"))

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
		originalEvents[i] = ev
	}

	// Export events
	var buf bytes.Buffer
	db.Export(ctx, &buf)

	// Wipe database
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Verify database is empty
	evs, err := db.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("Expected 0 events after wipe, got %d", len(evs))
	}

	// Import events
	db.Import(bytes.NewReader(buf.Bytes()))

	// Verify events were restored
	evs, err = db.QueryEvents(ctx, &filter.F{
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query imported events: %v", err)
	}

	if len(evs) != 3 {
		t.Fatalf("Expected 3 imported events, got %d", len(evs))
	}

	// Verify event IDs match
	importedIDs := make(map[string]bool)
	for _, ev := range evs {
		importedIDs[hex.Enc(ev.ID[:])] = true
	}

	for _, orig := range originalEvents {
		if !importedIDs[hex.Enc(orig.ID[:])] {
			t.Fatalf("Original event %s not found after import", hex.Enc(orig.ID[:]))
		}
	}

	t.Logf("✓ Export/Import round trip preserved %d events correctly", len(evs))
}
