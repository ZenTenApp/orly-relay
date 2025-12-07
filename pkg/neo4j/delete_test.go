//go:build integration
// +build integration

package neo4j

import (
	"context"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// All tests in this file use the shared testDB instance from testmain_test.go
// to avoid Neo4j authentication rate limiting from too many connections.

func TestDeleteEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

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
	ev.Content = []byte("Event to be deleted")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify event exists
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query event: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("Expected 1 event before deletion, got %d", len(evs))
	}

	// Delete the event
	if err := testDB.DeleteEvent(ctx, ev.ID[:]); err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify event is deleted
	evs, err = testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query after deletion: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("Expected 0 events after deletion, got %d", len(evs))
	}

	t.Logf("✓ DeleteEvent successfully removed event")
}

func TestDeleteEventBySerial(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

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
	ev.Content = []byte("Event to be deleted by serial")

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get serial
	serial, err := testDB.GetSerialById(ev.ID[:])
	if err != nil {
		t.Fatalf("Failed to get serial: %v", err)
	}

	// Delete by serial
	if err := testDB.DeleteEventBySerial(ctx, serial, ev); err != nil {
		t.Fatalf("Failed to delete event by serial: %v", err)
	}

	// Verify event is deleted
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query after deletion: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("Expected 0 events after deletion, got %d", len(evs))
	}

	t.Logf("✓ DeleteEventBySerial successfully removed event")
}

func TestProcessDelete_AuthorCanDeleteOwnEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create and save original event
	originalEvent := event.New()
	originalEvent.Pubkey = signer.Pub()
	originalEvent.CreatedAt = timestamp.Now().V
	originalEvent.Kind = 1
	originalEvent.Content = []byte("This event will be deleted via kind 5")

	if err := originalEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, originalEvent); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Create kind 5 deletion event
	deleteEvent := event.New()
	deleteEvent.Pubkey = signer.Pub() // Same author
	deleteEvent.CreatedAt = timestamp.Now().V + 1
	deleteEvent.Kind = kind.Deletion.K
	deleteEvent.Content = []byte("Deleting my event")
	deleteEvent.Tags = tag.NewS(
		tag.NewFromAny("e", hex.Enc(originalEvent.ID[:])),
	)

	if err := deleteEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign delete event: %v", err)
	}

	// Process deletion (no admins)
	if err := testDB.ProcessDelete(deleteEvent, nil); err != nil {
		t.Fatalf("Failed to process delete: %v", err)
	}

	// Verify original event is deleted
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(originalEvent.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query after deletion: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("Expected 0 events after deletion, got %d", len(evs))
	}

	t.Logf("✓ ProcessDelete allowed author to delete own event")
}

func TestProcessDelete_OtherUserCannotDelete(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	alice, _ := p8k.New()
	alice.Generate()

	bob, _ := p8k.New()
	bob.Generate()

	// Alice creates an event
	aliceEvent := event.New()
	aliceEvent.Pubkey = alice.Pub()
	aliceEvent.CreatedAt = timestamp.Now().V
	aliceEvent.Kind = 1
	aliceEvent.Content = []byte("Alice's event")

	if err := aliceEvent.Sign(alice); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, aliceEvent); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Bob tries to delete Alice's event
	deleteEvent := event.New()
	deleteEvent.Pubkey = bob.Pub() // Different author
	deleteEvent.CreatedAt = timestamp.Now().V + 1
	deleteEvent.Kind = kind.Deletion.K
	deleteEvent.Tags = tag.NewS(
		tag.NewFromAny("e", hex.Enc(aliceEvent.ID[:])),
	)

	if err := deleteEvent.Sign(bob); err != nil {
		t.Fatalf("Failed to sign delete event: %v", err)
	}

	// Process deletion (Bob is not an admin)
	_ = testDB.ProcessDelete(deleteEvent, nil)

	// Verify Alice's event still exists
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(aliceEvent.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("Expected Alice's event to still exist, got %d events", len(evs))
	}

	t.Logf("✓ ProcessDelete correctly prevented unauthorized deletion")
}

func TestProcessDelete_AdminCanDeleteAnyEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	alice, _ := p8k.New()
	alice.Generate()

	admin, _ := p8k.New()
	admin.Generate()

	// Alice creates an event
	aliceEvent := event.New()
	aliceEvent.Pubkey = alice.Pub()
	aliceEvent.CreatedAt = timestamp.Now().V
	aliceEvent.Kind = 1
	aliceEvent.Content = []byte("Alice's event to be deleted by admin")

	if err := aliceEvent.Sign(alice); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, aliceEvent); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Admin creates deletion event
	deleteEvent := event.New()
	deleteEvent.Pubkey = admin.Pub()
	deleteEvent.CreatedAt = timestamp.Now().V + 1
	deleteEvent.Kind = kind.Deletion.K
	deleteEvent.Tags = tag.NewS(
		tag.NewFromAny("e", hex.Enc(aliceEvent.ID[:])),
	)

	if err := deleteEvent.Sign(admin); err != nil {
		t.Fatalf("Failed to sign delete event: %v", err)
	}

	// Process deletion with admin pubkey
	adminPubkeys := [][]byte{admin.Pub()}
	if err := testDB.ProcessDelete(deleteEvent, adminPubkeys); err != nil {
		t.Fatalf("Failed to process delete: %v", err)
	}

	// Verify Alice's event is deleted
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(aliceEvent.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("Expected Alice's event to be deleted, got %d events", len(evs))
	}

	t.Logf("✓ ProcessDelete allowed admin to delete event")
}

func TestCheckForDeleted(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create target event
	targetEvent := event.New()
	targetEvent.Pubkey = signer.Pub()
	targetEvent.CreatedAt = timestamp.Now().V
	targetEvent.Kind = 1
	targetEvent.Content = []byte("Target event")

	if err := targetEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign target event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, targetEvent); err != nil {
		t.Fatalf("Failed to save target event: %v", err)
	}

	// Check that event is not deleted (no deletion event exists)
	err = testDB.CheckForDeleted(targetEvent, nil)
	if err != nil {
		t.Fatalf("Expected no error for non-deleted event, got: %v", err)
	}

	// Create deletion event that references target
	deleteEvent := event.New()
	deleteEvent.Pubkey = signer.Pub()
	deleteEvent.CreatedAt = timestamp.Now().V + 1
	deleteEvent.Kind = kind.Deletion.K
	deleteEvent.Tags = tag.NewS(
		tag.NewFromAny("e", hex.Enc(targetEvent.ID[:])),
	)

	if err := deleteEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign delete event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, deleteEvent); err != nil {
		t.Fatalf("Failed to save delete event: %v", err)
	}

	// Now check should return error (event has been deleted)
	err = testDB.CheckForDeleted(targetEvent, nil)
	if err == nil {
		t.Fatal("Expected error for deleted event")
	}

	t.Logf("✓ CheckForDeleted correctly detected deletion event")
}

func TestReplaceableEventDeletion(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create replaceable event (kind 0 - profile)
	profileEvent := event.New()
	profileEvent.Pubkey = signer.Pub()
	profileEvent.CreatedAt = timestamp.Now().V
	profileEvent.Kind = 0
	profileEvent.Content = []byte(`{"name":"Test User"}`)

	if err := profileEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, profileEvent); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify event exists
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds:   kind.NewS(kind.New(0)),
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("Expected 1 profile event, got %d", len(evs))
	}

	// Create a newer replaceable event (replaces the old one)
	newerProfileEvent := event.New()
	newerProfileEvent.Pubkey = signer.Pub()
	newerProfileEvent.CreatedAt = timestamp.Now().V + 100
	newerProfileEvent.Kind = 0
	newerProfileEvent.Content = []byte(`{"name":"Updated User"}`)

	if err := newerProfileEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign newer event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, newerProfileEvent); err != nil {
		t.Fatalf("Failed to save newer event: %v", err)
	}

	// Query should return only the newer event
	evs, err = testDB.QueryEvents(ctx, &filter.F{
		Kinds:   kind.NewS(kind.New(0)),
		Authors: tag.NewFromBytesSlice(signer.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("Expected 1 profile event after replacement, got %d", len(evs))
	}

	if hex.Enc(evs[0].ID[:]) != hex.Enc(newerProfileEvent.ID[:]) {
		t.Fatal("Expected newer profile event to be returned")
	}

	t.Logf("✓ Replaceable event correctly replaced by newer version")
}
