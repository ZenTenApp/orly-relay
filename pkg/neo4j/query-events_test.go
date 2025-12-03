package neo4j

import (
	"context"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// setupTestDatabase creates a fresh Neo4j database connection for testing
func setupTestDatabase(t *testing.T) (*N, context.Context, context.CancelFunc) {
	t.Helper()

	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		cancel()
		t.Fatalf("Failed to create database: %v", err)
	}

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		db.Close()
		cancel()
		t.Fatalf("Failed to wipe database: %v", err)
	}

	return db, ctx, cancel
}

// createTestSigner creates a new signer for test events
func createTestSigner(t *testing.T) *p8k.Signer {
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

// createAndSaveEvent creates a signed event and saves it to the database
func createAndSaveEvent(t *testing.T, ctx context.Context, db *N, signer *p8k.Signer, k uint16, content string, tags *tag.S, ts int64) *event.E {
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

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	return ev
}

func TestQueryEventsByID(t *testing.T) {
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)

	// Create and save a test event
	ev := createAndSaveEvent(t, ctx, db, signer, 1, "Test event for ID query", nil, timestamp.Now().V)

	// Query by ID
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events of different kinds
	createAndSaveEvent(t, ctx, db, signer, 1, "Kind 1 event A", nil, baseTs)
	createAndSaveEvent(t, ctx, db, signer, 1, "Kind 1 event B", nil, baseTs+1)
	createAndSaveEvent(t, ctx, db, signer, 7, "Kind 7 reaction", nil, baseTs+2)
	createAndSaveEvent(t, ctx, db, signer, 30023, "Kind 30023 article", nil, baseTs+3)

	// Query for kind 1
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	alice := createTestSigner(t)
	bob := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events from different authors
	createAndSaveEvent(t, ctx, db, alice, 1, "Alice's event 1", nil, baseTs)
	createAndSaveEvent(t, ctx, db, alice, 1, "Alice's event 2", nil, baseTs+1)
	createAndSaveEvent(t, ctx, db, bob, 1, "Bob's event", nil, baseTs+2)

	// Query for Alice's events
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events at different times
	createAndSaveEvent(t, ctx, db, signer, 1, "Old event", nil, baseTs-7200)   // 2 hours ago
	createAndSaveEvent(t, ctx, db, signer, 1, "Recent event", nil, baseTs-1800) // 30 min ago
	createAndSaveEvent(t, ctx, db, signer, 1, "Current event", nil, baseTs)

	// Query for events in the last hour
	since := &timestamp.T{V: baseTs - 3600}
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events with tags
	createAndSaveEvent(t, ctx, db, signer, 1, "Bitcoin post",
		tag.NewS(tag.NewFromAny("t", "bitcoin")), baseTs)
	createAndSaveEvent(t, ctx, db, signer, 1, "Nostr post",
		tag.NewS(tag.NewFromAny("t", "nostr")), baseTs+1)
	createAndSaveEvent(t, ctx, db, signer, 1, "Bitcoin and Nostr post",
		tag.NewS(tag.NewFromAny("t", "bitcoin"), tag.NewFromAny("t", "nostr")), baseTs+2)

	// Query for bitcoin tagged events
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	alice := createTestSigner(t)
	bob := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events
	createAndSaveEvent(t, ctx, db, alice, 1, "Alice note", nil, baseTs)
	createAndSaveEvent(t, ctx, db, alice, 7, "Alice reaction", nil, baseTs+1)
	createAndSaveEvent(t, ctx, db, bob, 1, "Bob note", nil, baseTs+2)

	// Query for Alice's kind 1 events
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create many events
	for i := 0; i < 20; i++ {
		createAndSaveEvent(t, ctx, db, signer, 1, "Event", nil, baseTs+int64(i))
	}

	// Query with limit
	limit := uint(5)
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events at different times
	createAndSaveEvent(t, ctx, db, signer, 1, "First", nil, baseTs)
	createAndSaveEvent(t, ctx, db, signer, 1, "Second", nil, baseTs+100)
	createAndSaveEvent(t, ctx, db, signer, 1, "Third", nil, baseTs+200)

	// Query and verify order (should be descending by created_at)
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	// Query for non-existent kind
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events of different kinds
	createAndSaveEvent(t, ctx, db, signer, 1, "Note", nil, baseTs)
	createAndSaveEvent(t, ctx, db, signer, 7, "Reaction", nil, baseTs+1)
	createAndSaveEvent(t, ctx, db, signer, 30023, "Article", nil, baseTs+2)

	// Query for multiple kinds
	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	alice := createTestSigner(t)
	bob := createTestSigner(t)
	charlie := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events from different authors
	createAndSaveEvent(t, ctx, db, alice, 1, "Alice", nil, baseTs)
	createAndSaveEvent(t, ctx, db, bob, 1, "Bob", nil, baseTs+1)
	createAndSaveEvent(t, ctx, db, charlie, 1, "Charlie", nil, baseTs+2)

	// Query for Alice and Bob's events
	authors := tag.NewFromBytesSlice(alice.Pub(), bob.Pub())

	evs, err := db.QueryEvents(ctx, &filter.F{
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
	db, ctx, cancel := setupTestDatabase(t)
	defer db.Close()
	defer cancel()

	signer := createTestSigner(t)
	baseTs := timestamp.Now().V

	// Create events
	for i := 0; i < 5; i++ {
		createAndSaveEvent(t, ctx, db, signer, 1, "Event", nil, baseTs+int64(i))
	}

	// Count events
	count, _, err := db.CountEvents(ctx, &filter.F{
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
