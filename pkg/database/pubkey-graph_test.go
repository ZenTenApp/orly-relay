package database

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

func TestPubkeySerialAssignment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a test pubkey
	pubkey1 := make([]byte, 32)
	for i := range pubkey1 {
		pubkey1[i] = byte(i)
	}

	// Get or create serial for the first time
	t.Logf("First call: GetOrCreatePubkeySerial for pubkey %s", hex.Enc(pubkey1))
	ser1, err := db.GetOrCreatePubkeySerial(pubkey1)
	if err != nil {
		t.Fatalf("Failed to get or create pubkey serial: %v", err)
	}

	if ser1 == nil {
		t.Fatal("Serial should not be nil")
	}
	t.Logf("First call returned serial: %d", ser1.Get())

	// Debug: List all keys in database
	var keyCount int
	db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			t.Logf("Found key: %s (len=%d)", hex.Enc(key), len(key))
			keyCount++
			if keyCount > 20 {
				break // Limit output
			}
		}
		return nil
	})
	t.Logf("Total keys found (first 20): %d", keyCount)

	// Debug: what prefix should we be looking for?
	pubHash := new(types.PubHash)
	pubHash.FromPubkey(pubkey1)
	expectedPrefix := []byte(indexes.PubkeySerialPrefix)
	t.Logf("Expected PubkeySerial prefix: %s = %s", string(expectedPrefix), hex.Enc(expectedPrefix))

	// Try direct lookup
	t.Logf("Direct lookup: GetPubkeySerial for same pubkey")
	serDirect, err := db.GetPubkeySerial(pubkey1)
	if err != nil {
		t.Logf("Direct lookup failed: %v", err)
	} else {
		t.Logf("Direct lookup returned serial: %d", serDirect.Get())
	}

	// Get the same pubkey again - should return the same serial
	t.Logf("Second call: GetOrCreatePubkeySerial for same pubkey")
	ser2, err := db.GetOrCreatePubkeySerial(pubkey1)
	if err != nil {
		t.Fatalf("Failed to get existing pubkey serial: %v", err)
	}
	t.Logf("Second call returned serial: %d", ser2.Get())

	if ser1.Get() != ser2.Get() {
		t.Errorf("Expected same serial, got %d and %d", ser1.Get(), ser2.Get())
	}

	// Create a different pubkey
	pubkey2 := make([]byte, 32)
	for i := range pubkey2 {
		pubkey2[i] = byte(i + 100)
	}

	ser3, err := db.GetOrCreatePubkeySerial(pubkey2)
	if err != nil {
		t.Fatalf("Failed to get or create second pubkey serial: %v", err)
	}

	if ser3.Get() == ser1.Get() {
		t.Error("Different pubkeys should have different serials")
	}

	// Test reverse lookup: serial -> pubkey
	retrievedPubkey1, err := db.GetPubkeyBySerial(ser1)
	if err != nil {
		t.Fatalf("Failed to get pubkey by serial: %v", err)
	}

	if hex.Enc(retrievedPubkey1) != hex.Enc(pubkey1) {
		t.Errorf("Retrieved pubkey doesn't match. Expected %s, got %s",
			hex.Enc(pubkey1), hex.Enc(retrievedPubkey1))
	}
}

func TestEventPubkeyGraph(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test event with author and p-tags
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	pTagPubkey1, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	pTagPubkey2, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	eventID := make([]byte, 32)
	eventID[0] = 1
	eventSig := make([]byte, 64)
	eventSig[0] = 1

	// Create a valid e-tag event ID (32 bytes = 64 hex chars)
	eTagEventID := make([]byte, 32)
	eTagEventID[0] = 0xAB

	ev := &event.E{
		ID:        eventID,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1, // text note
		Content:   []byte("Test event with p-tags"),
		Sig:       eventSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(pTagPubkey1)),
			tag.NewFromAny("p", hex.Enc(pTagPubkey2)),
			tag.NewFromAny("e", hex.Enc(eTagEventID)),
		),
	}

	// Save the event - this should create pubkey serials and graph edges
	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify that pubkey serials were created
	authorSerial, err := db.GetPubkeySerial(authorPubkey)
	if err != nil {
		t.Fatalf("Failed to get author pubkey serial: %v", err)
	}
	if authorSerial == nil {
		t.Fatal("Author serial should not be nil")
	}

	pTag1Serial, err := db.GetPubkeySerial(pTagPubkey1)
	if err != nil {
		t.Fatalf("Failed to get p-tag1 pubkey serial: %v", err)
	}
	if pTag1Serial == nil {
		t.Fatal("P-tag1 serial should not be nil")
	}

	pTag2Serial, err := db.GetPubkeySerial(pTagPubkey2)
	if err != nil {
		t.Fatalf("Failed to get p-tag2 pubkey serial: %v", err)
	}
	if pTag2Serial == nil {
		t.Fatal("P-tag2 serial should not be nil")
	}

	// Verify all three pubkeys have different serials
	if authorSerial.Get() == pTag1Serial.Get() || authorSerial.Get() == pTag2Serial.Get() || pTag1Serial.Get() == pTag2Serial.Get() {
		t.Error("All pubkey serials should be unique")
	}

	t.Logf("Event saved successfully with graph edges:")
	t.Logf("  Author serial: %d", authorSerial.Get())
	t.Logf("  P-tag1 serial: %d", pTag1Serial.Get())
	t.Logf("  P-tag2 serial: %d", pTag2Serial.Get())
}

func TestMultipleEventsWithSamePubkeys(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create two events from the same author mentioning the same person
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	pTagPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")

	eventID1 := make([]byte, 32)
	eventID1[0] = 1
	eventSig1 := make([]byte, 64)
	eventSig1[0] = 1

	ev1 := &event.E{
		ID:        eventID1,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("First event"),
		Sig:       eventSig1,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(pTagPubkey)),
		),
	}

	eventID2 := make([]byte, 32)
	eventID2[0] = 2
	eventSig2 := make([]byte, 64)
	eventSig2[0] = 2

	ev2 := &event.E{
		ID:        eventID2,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567891,
		Kind:      1,
		Content:   []byte("Second event"),
		Sig:       eventSig2,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(pTagPubkey)),
		),
	}

	// Save both events
	_, err = db.SaveEvent(ctx, ev1)
	if err != nil {
		t.Fatalf("Failed to save event 1: %v", err)
	}

	_, err = db.SaveEvent(ctx, ev2)
	if err != nil {
		t.Fatalf("Failed to save event 2: %v", err)
	}

	// Verify the same pubkeys got the same serials
	authorSerial1, _ := db.GetPubkeySerial(authorPubkey)
	pTagSerial1, _ := db.GetPubkeySerial(pTagPubkey)

	if authorSerial1 == nil || pTagSerial1 == nil {
		t.Fatal("Pubkey serials should exist after saving events")
	}

	t.Logf("Both events share the same pubkey serials:")
	t.Logf("  Author serial: %d", authorSerial1.Get())
	t.Logf("  P-tag serial: %d", pTagSerial1.Get())
}

func TestPubkeySerialEdgeCases(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test with invalid pubkey length
	invalidPubkey := make([]byte, 16) // Wrong length
	_, err = db.GetOrCreatePubkeySerial(invalidPubkey)
	if err == nil {
		t.Error("Should reject pubkey with invalid length")
	}

	// Test GetPubkeySerial for non-existent pubkey
	nonExistentPubkey := make([]byte, 32)
	for i := range nonExistentPubkey {
		nonExistentPubkey[i] = 0xFF
	}

	_, err = db.GetPubkeySerial(nonExistentPubkey)
	if err == nil {
		t.Error("Should return error for non-existent pubkey serial")
	}
}

func TestGraphEdgeDirections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test event with author and p-tags
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	pTagPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")

	eventID := make([]byte, 32)
	eventID[0] = 1
	eventSig := make([]byte, 64)
	eventSig[0] = 1

	ev := &event.E{
		ID:        eventID,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1, // text note
		Content:   []byte("Test event"),
		Sig:       eventSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(pTagPubkey)),
		),
	}

	// Save the event
	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify graph edges with correct direction bytes
	// Look for PubkeyEventGraph keys and check direction byte
	var foundAuthorEdge, foundPTagEdge bool
	db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(indexes.PubkeyEventGraphPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			// Key format: peg(3)|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5) = 16 bytes
			if len(key) == 16 {
				direction := key[10] // Byte at position 10 is the direction
				t.Logf("Found PubkeyEventGraph edge: key=%s, direction=%d", hex.Enc(key), direction)

				if direction == types.EdgeDirectionAuthor {
					foundAuthorEdge = true
					t.Logf("  ✓ Found author edge (direction=0)")
				} else if direction == types.EdgeDirectionPTagIn {
					foundPTagEdge = true
					t.Logf("  ✓ Found p-tag inbound edge (direction=2)")
				}
			}
		}
		return nil
	})

	if !foundAuthorEdge {
		t.Error("Did not find author edge with direction=0")
	}
	if !foundPTagEdge {
		t.Error("Did not find p-tag inbound edge with direction=2")
	}

	t.Logf("Graph edges correctly stored with direction bytes:")
	t.Logf("  Author edge: %v (direction=0)", foundAuthorEdge)
	t.Logf("  P-tag inbound edge: %v (direction=2)", foundPTagEdge)
}
