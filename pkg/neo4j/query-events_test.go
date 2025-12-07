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

// createTestSignerLocal creates a new signer for test events
func createTestSignerLocal(t *testing.T) *p8k.Signer {
	t.Helper()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	return signer
}

// createAndSaveEventLocal creates a signed event and saves it to the database
func createAndSaveEventLocal(t *testing.T, ctx context.Context, signer *p8k.Signer, k uint16, content string, tags *tag.S, ts int64) *event.E {
	t.Helper()

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = ts
	ev.Kind = k
	ev.Content = []byte(content)
	ev.Tags = tags

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	return ev
}

func TestQueryEventsByID(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)

	// Create and save a test event
	ev := createAndSaveEventLocal(t, ctx, signer, 1, "Test event for ID query", nil, timestamp.Now().V)

	// Query by ID
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Ids: tag.NewFromBytesSlice(ev.ID),
	})
	if err != nil {
		t.Fatalf("Failed to query events by ID: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evs))
	}

	if hex.Enc(evs[0].ID[:]) != hex.Enc(ev.ID[:]) {
		t.Fatalf("Event ID mismatch: got %s, expected %s",
			hex.Enc(evs[0].ID[:]), hex.Enc(ev.ID[:]))
	}

	t.Logf("✓ Query by ID returned correct event")
}

func TestQueryEventsByKind(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events of different kinds
	createAndSaveEventLocal(t, ctx, signer, 1, "Kind 1 event A", nil, baseTs)
	createAndSaveEventLocal(t, ctx, signer, 1, "Kind 1 event B", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 7, "Kind 7 reaction", nil, baseTs+2)
	createAndSaveEventLocal(t, ctx, signer, 30023, "Kind 30023 article", nil, baseTs+3)

	// Query for kind 1
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to query events by kind: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 kind 1 events, got %d", len(evs))
	}

	for _, ev := range evs {
		if ev.Kind != 1 {
			t.Fatalf("Expected kind 1, got %d", ev.Kind)
		}
	}

	t.Logf("✓ Query by kind returned %d correct events", len(evs))
}

func TestQueryEventsByAuthor(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	alice := createTestSignerLocal(t)
	bob := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events from different authors
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice's event 1", nil, baseTs)
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice's event 2", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, bob, 1, "Bob's event", nil, baseTs+2)

	// Query for Alice's events
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Authors: tag.NewFromBytesSlice(alice.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query events by author: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 events from Alice, got %d", len(evs))
	}

	alicePubkey := hex.Enc(alice.Pub())
	for _, ev := range evs {
		if hex.Enc(ev.Pubkey[:]) != alicePubkey {
			t.Fatalf("Expected author %s, got %s", alicePubkey, hex.Enc(ev.Pubkey[:]))
		}
	}

	t.Logf("✓ Query by author returned %d correct events", len(evs))
}

func TestQueryEventsByTimeRange(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events at different times
	createAndSaveEventLocal(t, ctx, signer, 1, "Old event", nil, baseTs-7200)    // 2 hours ago
	createAndSaveEventLocal(t, ctx, signer, 1, "Recent event", nil, baseTs-1800) // 30 min ago
	createAndSaveEventLocal(t, ctx, signer, 1, "Current event", nil, baseTs)

	// Query for events in the last hour
	since := &timestamp.T{V: baseTs - 3600}
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Since: since,
	})
	if err != nil {
		t.Fatalf("Failed to query events by time range: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 events in last hour, got %d", len(evs))
	}

	for _, ev := range evs {
		if ev.CreatedAt < since.V {
			t.Fatalf("Event created_at %d is before since %d", ev.CreatedAt, since.V)
		}
	}

	t.Logf("✓ Query by time range returned %d correct events", len(evs))
}

func TestQueryEventsByTag(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events with tags
	createAndSaveEventLocal(t, ctx, signer, 1, "Bitcoin post",
		tag.NewS(tag.NewFromAny("t", "bitcoin")), baseTs)
	createAndSaveEventLocal(t, ctx, signer, 1, "Nostr post",
		tag.NewS(tag.NewFromAny("t", "nostr")), baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 1, "Bitcoin and Nostr post",
		tag.NewS(tag.NewFromAny("t", "bitcoin"), tag.NewFromAny("t", "nostr")), baseTs+2)

	// Query for bitcoin tagged events
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Tags: tag.NewS(tag.NewFromAny("t", "bitcoin")),
	})
	if err != nil {
		t.Fatalf("Failed to query events by tag: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 bitcoin-tagged events, got %d", len(evs))
	}

	t.Logf("✓ Query by tag returned %d correct events", len(evs))
}

func TestQueryEventsByKindAndAuthor(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	alice := createTestSignerLocal(t)
	bob := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice note", nil, baseTs)
	createAndSaveEventLocal(t, ctx, alice, 7, "Alice reaction", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, bob, 1, "Bob note", nil, baseTs+2)

	// Query for Alice's kind 1 events
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds:   kind.NewS(kind.New(1)),
		Authors: tag.NewFromBytesSlice(alice.Pub()),
	})
	if err != nil {
		t.Fatalf("Failed to query events by kind and author: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 kind 1 event from Alice, got %d", len(evs))
	}

	t.Logf("✓ Query by kind and author returned correct events")
}

func TestQueryEventsWithLimit(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create many events
	for i := 0; i < 20; i++ {
		createAndSaveEventLocal(t, ctx, signer, 1, "Event", nil, baseTs+int64(i))
	}

	// Query with limit
	limit := uint(5)
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
		Limit: &limit,
	})
	if err != nil {
		t.Fatalf("Failed to query events with limit: %v", err)
	}

	if len(evs) != int(limit) {
		t.Fatalf("Expected %d events with limit, got %d", limit, len(evs))
	}

	t.Logf("✓ Query with limit returned %d events", len(evs))
}

func TestQueryEventsOrderByCreatedAt(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events at different times
	createAndSaveEventLocal(t, ctx, signer, 1, "First", nil, baseTs)
	createAndSaveEventLocal(t, ctx, signer, 1, "Second", nil, baseTs+100)
	createAndSaveEventLocal(t, ctx, signer, 1, "Third", nil, baseTs+200)

	// Query and verify order (should be descending by created_at)
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) < 2 {
		t.Fatalf("Expected at least 2 events, got %d", len(evs))
	}

	// Verify descending order
	for i := 1; i < len(evs); i++ {
		if evs[i-1].CreatedAt < evs[i].CreatedAt {
			t.Fatalf("Events not in descending order: %d < %d at index %d",
				evs[i-1].CreatedAt, evs[i].CreatedAt, i)
		}
	}

	t.Logf("✓ Query returned events in correct descending order")
}

func TestQueryEventsEmpty(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()

	// Query for non-existent kind
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(99999)),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 0 {
		t.Fatalf("Expected 0 events, got %d", len(evs))
	}

	t.Logf("✓ Query for non-existent kind returned empty result")
}

func TestQueryEventsMultipleKinds(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events of different kinds
	createAndSaveEventLocal(t, ctx, signer, 1, "Note", nil, baseTs)
	createAndSaveEventLocal(t, ctx, signer, 7, "Reaction", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 30023, "Article", nil, baseTs+2)

	// Query for multiple kinds
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1), kind.New(7)),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 events (kind 1 and 7), got %d", len(evs))
	}

	t.Logf("✓ Query for multiple kinds returned correct events")
}

func TestQueryEventsMultipleAuthors(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	alice := createTestSignerLocal(t)
	bob := createTestSignerLocal(t)
	charlie := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events from different authors
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice", nil, baseTs)
	createAndSaveEventLocal(t, ctx, bob, 1, "Bob", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, charlie, 1, "Charlie", nil, baseTs+2)

	// Query for Alice and Bob's events
	authors := tag.NewFromBytesSlice(alice.Pub(), bob.Pub())

	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Authors: authors,
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 events from Alice and Bob, got %d", len(evs))
	}

	t.Logf("✓ Query for multiple authors returned correct events")
}

func TestCountEvents(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events
	for i := 0; i < 5; i++ {
		createAndSaveEventLocal(t, ctx, signer, 1, "Event", nil, baseTs+int64(i))
	}

	// Count events
	count, _, err := testDB.CountEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 5 {
		t.Fatalf("Expected count 5, got %d", count)
	}

	t.Logf("✓ Count events returned correct count: %d", count)
}
