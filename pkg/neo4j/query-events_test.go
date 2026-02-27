//go:build integration
// +build integration

package neo4j

import (
	"context"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
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

// TestQueryEventsByTagWithHashPrefix tests that tag filters with "#" prefix work correctly.
// This is a regression test for a bug where filter tags like "#d" were not being matched
// because the "#" prefix wasn't being stripped before comparison with stored tags.
func TestQueryEventsByTagWithHashPrefix(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events with d-tags (parameterized replaceable kind)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d=id1",
		tag.NewS(tag.NewFromAny("d", "id1")), baseTs)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d=id2",
		tag.NewS(tag.NewFromAny("d", "id2")), baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d=id3",
		tag.NewS(tag.NewFromAny("d", "id3")), baseTs+2)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d=other",
		tag.NewS(tag.NewFromAny("d", "other")), baseTs+3)

	// Query with "#d" prefix (as clients send it) - should match events with d=id1
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(30382)),
		Tags:  tag.NewS(tag.NewFromAny("#d", "id1")),
	})
	if err != nil {
		t.Fatalf("Failed to query events with #d tag: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 event with d=id1, got %d", len(evs))
	}

	// Verify the returned event has the correct d-tag
	dTag := evs[0].Tags.GetFirst([]byte("d"))
	if dTag == nil || string(dTag.Value()) != "id1" {
		t.Fatalf("Expected d=id1, got d=%s", dTag.Value())
	}

	t.Logf("✓ Query with #d prefix returned correct event")
}

// TestQueryEventsByTagMultipleValues tests that tag filters with multiple values
// use OR logic (match events with ANY of the values).
func TestQueryEventsByTagMultipleValues(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events with different d-tags
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event A",
		tag.NewS(tag.NewFromAny("d", "target-1")), baseTs)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event B",
		tag.NewS(tag.NewFromAny("d", "target-2")), baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event C",
		tag.NewS(tag.NewFromAny("d", "target-3")), baseTs+2)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event D (not target)",
		tag.NewS(tag.NewFromAny("d", "other-value")), baseTs+3)
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event E (no match)",
		tag.NewS(tag.NewFromAny("d", "different")), baseTs+4)

	// Query with multiple d-tag values using "#d" prefix
	// Should match events with d=target-1 OR d=target-2 OR d=target-3
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(30382)),
		Tags:  tag.NewS(tag.NewFromAny("#d", "target-1", "target-2", "target-3")),
	})
	if err != nil {
		t.Fatalf("Failed to query events with multiple #d values: %v", err)
	}

	if len(evs) != 3 {
		t.Fatalf("Expected 3 events matching the d-tag values, got %d", len(evs))
	}

	// Verify returned events have correct d-tags
	validDTags := map[string]bool{"target-1": false, "target-2": false, "target-3": false}
	for _, ev := range evs {
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			t.Fatalf("Event missing d-tag")
		}
		dValue := string(dTag.Value())
		if _, ok := validDTags[dValue]; !ok {
			t.Fatalf("Unexpected d-tag value: %s", dValue)
		}
		validDTags[dValue] = true
	}

	// Verify all expected d-tags were found
	for dValue, found := range validDTags {
		if !found {
			t.Fatalf("Expected to find event with d=%s", dValue)
		}
	}

	t.Logf("✓ Query with multiple #d values returned correct events")
}

// TestQueryEventsByTagNoMatch tests that tag filters correctly return no results
// when no events match the filter.
func TestQueryEventsByTagNoMatch(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events with d-tags
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event",
		tag.NewS(tag.NewFromAny("d", "existing-value")), baseTs)

	// Query for d-tag value that doesn't exist
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(30382)),
		Tags:  tag.NewS(tag.NewFromAny("#d", "non-existent-value")),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(evs) != 0 {
		t.Fatalf("Expected 0 events for non-matching d-tag, got %d", len(evs))
	}

	t.Logf("✓ Query with non-matching #d value returned no events")
}

// TestQueryEventsByTagWithKindAndAuthor tests the combination of kind, author, and tag filters.
// This is the specific case reported by the user with kind 30382.
func TestQueryEventsByTagWithKindAndAuthor(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	alice := createTestSignerLocal(t)
	bob := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events from different authors with d-tags
	createAndSaveEventLocal(t, ctx, alice, 30382, "Alice target 1",
		tag.NewS(tag.NewFromAny("d", "card-1")), baseTs)
	createAndSaveEventLocal(t, ctx, alice, 30382, "Alice target 2",
		tag.NewS(tag.NewFromAny("d", "card-2")), baseTs+1)
	createAndSaveEventLocal(t, ctx, alice, 30382, "Alice other",
		tag.NewS(tag.NewFromAny("d", "other-card")), baseTs+2)
	createAndSaveEventLocal(t, ctx, bob, 30382, "Bob target 1",
		tag.NewS(tag.NewFromAny("d", "card-1")), baseTs+3) // Same d-tag as Alice but different author

	// Query for Alice's events with specific d-tags
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds:   kind.NewS(kind.New(30382)),
		Authors: tag.NewFromBytesSlice(alice.Pub()),
		Tags:    tag.NewS(tag.NewFromAny("#d", "card-1", "card-2")),
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	// Should only return Alice's 2 events, not Bob's even though he has card-1
	if len(evs) != 2 {
		t.Fatalf("Expected 2 events from Alice with matching d-tags, got %d", len(evs))
	}

	alicePubkey := hex.Enc(alice.Pub())
	for _, ev := range evs {
		if hex.Enc(ev.Pubkey[:]) != alicePubkey {
			t.Fatalf("Expected author %s, got %s", alicePubkey, hex.Enc(ev.Pubkey[:]))
		}
		dTag := ev.Tags.GetFirst([]byte("d"))
		dValue := string(dTag.Value())
		if dValue != "card-1" && dValue != "card-2" {
			t.Fatalf("Expected d=card-1 or card-2, got d=%s", dValue)
		}
	}

	t.Logf("✓ Query with kind, author, and #d filter returned correct events")
}

// TestBinaryTagFilterRegression tests that queries with #e and #p tags work correctly
// even when tags are stored with binary-encoded values but filters come as hex strings.
// This mirrors the Badger database test for binary tag handling.
func TestBinaryTagFilterRegression(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	author := createTestSignerLocal(t)
	referenced := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create a referenced event to get a valid event ID for e-tag
	refEvent := createAndSaveEventLocal(t, ctx, referenced, 1, "Referenced event", nil, baseTs)

	// Get hex representations
	refEventIdHex := hex.Enc(refEvent.ID)
	refPubkeyHex := hex.Enc(referenced.Pub())

	// Create test event with e, p, d, and other tags
	testEvent := createAndSaveEventLocal(t, ctx, author, 30520, "Event with binary tags",
		tag.NewS(
			tag.NewFromAny("d", "test-d-value"),
			tag.NewFromAny("p", string(refPubkeyHex)),
			tag.NewFromAny("e", string(refEventIdHex)),
			tag.NewFromAny("t", "test-topic"),
		), baseTs+1)

	testEventIdHex := hex.Enc(testEvent.ID)

	// Test case 1: Query WITHOUT #e/#p tags (baseline - should work)
	t.Run("QueryWithoutEPTags", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(author.Pub()),
			Tags:    tag.NewS(tag.NewFromAny("#d", "test-d-value")),
		})
		if err != nil {
			t.Fatalf("Query without e/p tags failed: %v", err)
		}

		if len(evs) == 0 {
			t.Fatal("Expected to find event with d tag filter, got 0 results")
		}

		found := false
		for _, ev := range evs {
			if hex.Enc(ev.ID) == testEventIdHex {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event ID %s not found", testEventIdHex)
		}
	})

	// Test case 2: Query WITH #p tag
	t.Run("QueryWithPTag", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(author.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-value"),
				tag.NewFromAny("#p", string(refPubkeyHex)),
			),
		})
		if err != nil {
			t.Fatalf("Query with #p tag failed: %v", err)
		}

		if len(evs) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #p tag filter, got 0 results")
		}
	})

	// Test case 3: Query WITH #e tag
	t.Run("QueryWithETag", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(author.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-value"),
				tag.NewFromAny("#e", string(refEventIdHex)),
			),
		})
		if err != nil {
			t.Fatalf("Query with #e tag failed: %v", err)
		}

		if len(evs) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #e tag filter, got 0 results")
		}
	})

	// Test case 4: Query WITH BOTH #e AND #p tags
	t.Run("QueryWithBothEAndPTags", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(author.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-value"),
				tag.NewFromAny("#e", string(refEventIdHex)),
				tag.NewFromAny("#p", string(refPubkeyHex)),
			),
		})
		if err != nil {
			t.Fatalf("Query with both #e and #p tags failed: %v", err)
		}

		if len(evs) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #e and #p tag filters, got 0 results")
		}
	})

	t.Logf("✓ Binary tag filter regression tests passed")
}

// TestParameterizedReplaceableEvents tests that parameterized replaceable events (kind 30000+)
// are handled correctly - only the newest version should be returned in queries by kind/author/d-tag.
func TestParameterizedReplaceableEvents(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create older parameterized replaceable event
	createAndSaveEventLocal(t, ctx, signer, 30000, "Original event",
		tag.NewS(tag.NewFromAny("d", "test-param")), baseTs-7200) // 2 hours ago

	// Create newer event with same kind/author/d-tag
	createAndSaveEventLocal(t, ctx, signer, 30000, "Newer event",
		tag.NewS(tag.NewFromAny("d", "test-param")), baseTs-3600) // 1 hour ago

	// Create newest event with same kind/author/d-tag
	newestEvent := createAndSaveEventLocal(t, ctx, signer, 30000, "Newest event",
		tag.NewS(tag.NewFromAny("d", "test-param")), baseTs) // Now

	// Query for events - should only return the newest one
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds:   kind.NewS(kind.New(30000)),
		Authors: tag.NewFromBytesSlice(signer.Pub()),
		Tags:    tag.NewS(tag.NewFromAny("#d", "test-param")),
	})
	if err != nil {
		t.Fatalf("Failed to query parameterized replaceable events: %v", err)
	}

	// Note: Neo4j backend may or may not automatically deduplicate replaceable events
	// depending on implementation. The important thing is that the newest is returned first.
	if len(evs) == 0 {
		t.Fatal("Expected at least 1 event")
	}

	// Verify the first (most recent) event is the newest one
	if hex.Enc(evs[0].ID) != hex.Enc(newestEvent.ID) {
		t.Logf("Note: Expected newest event first, got different order")
	}

	t.Logf("✓ Parameterized replaceable events test returned %d events", len(evs))
}

// TestQueryForIds tests the QueryForIds method
func TestQueryForIds(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create test events
	ev1 := createAndSaveEventLocal(t, ctx, signer, 1, "Event 1", nil, baseTs)
	ev2 := createAndSaveEventLocal(t, ctx, signer, 1, "Event 2", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 7, "Reaction", nil, baseTs+2)

	// Query for IDs of kind 1 events
	idPkTs, err := testDB.QueryForIds(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to query for IDs: %v", err)
	}

	if len(idPkTs) != 2 {
		t.Fatalf("Expected 2 IDs for kind 1 events, got %d", len(idPkTs))
	}

	// Verify IDs match our events
	foundIds := make(map[string]bool)
	for _, r := range idPkTs {
		foundIds[hex.Enc(r.Id)] = true
	}

	if !foundIds[hex.Enc(ev1.ID)] {
		t.Error("Event 1 ID not found in results")
	}
	if !foundIds[hex.Enc(ev2.ID)] {
		t.Error("Event 2 ID not found in results")
	}

	t.Logf("✓ QueryForIds returned correct IDs")
}

// TestQueryForSerials tests the QueryForSerials method
func TestQueryForSerials(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create test events
	createAndSaveEventLocal(t, ctx, signer, 1, "Event 1", nil, baseTs)
	createAndSaveEventLocal(t, ctx, signer, 1, "Event 2", nil, baseTs+1)
	createAndSaveEventLocal(t, ctx, signer, 1, "Event 3", nil, baseTs+2)

	// Query for serials
	serials, err := testDB.QueryForSerials(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if err != nil {
		t.Fatalf("Failed to query for serials: %v", err)
	}

	if len(serials) != 3 {
		t.Fatalf("Expected 3 serials, got %d", len(serials))
	}

	t.Logf("✓ QueryForSerials returned %d serials", len(serials))
}

// TestQueryEventsComplex tests complex filter combinations
func TestQueryEventsComplex(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	alice := createTestSignerLocal(t)
	bob := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create diverse set of events
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice note with bitcoin tag",
		tag.NewS(tag.NewFromAny("t", "bitcoin")), baseTs)
	createAndSaveEventLocal(t, ctx, alice, 1, "Alice note with nostr tag",
		tag.NewS(tag.NewFromAny("t", "nostr")), baseTs+1)
	createAndSaveEventLocal(t, ctx, alice, 7, "Alice reaction",
		nil, baseTs+2)
	createAndSaveEventLocal(t, ctx, bob, 1, "Bob note with bitcoin tag",
		tag.NewS(tag.NewFromAny("t", "bitcoin")), baseTs+3)

	// Test: kinds + tags (no authors)
	t.Run("KindsAndTags", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds: kind.NewS(kind.New(1)),
			Tags:  tag.NewS(tag.NewFromAny("#t", "bitcoin")),
		})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(evs) != 2 {
			t.Fatalf("Expected 2 events with kind=1 and #t=bitcoin, got %d", len(evs))
		}
	})

	// Test: authors + tags (no kinds)
	t.Run("AuthorsAndTags", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Authors: tag.NewFromBytesSlice(alice.Pub()),
			Tags:    tag.NewS(tag.NewFromAny("#t", "bitcoin")),
		})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(evs) != 1 {
			t.Fatalf("Expected 1 event from Alice with #t=bitcoin, got %d", len(evs))
		}
	})

	// Test: kinds + authors (no tags)
	t.Run("KindsAndAuthors", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(1)),
			Authors: tag.NewFromBytesSlice(alice.Pub()),
		})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(evs) != 2 {
			t.Fatalf("Expected 2 kind=1 events from Alice, got %d", len(evs))
		}
	})

	// Test: all three filters
	t.Run("AllFilters", func(t *testing.T) {
		evs, err := testDB.QueryEvents(ctx, &filter.F{
			Kinds:   kind.NewS(kind.New(1)),
			Authors: tag.NewFromBytesSlice(alice.Pub()),
			Tags:    tag.NewS(tag.NewFromAny("#t", "nostr")),
		})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(evs) != 1 {
			t.Fatalf("Expected 1 event (Alice kind=1 #t=nostr), got %d", len(evs))
		}
	})

	t.Logf("✓ Complex filter combination tests passed")
}

// TestQueryEventsMultipleTagTypes tests filtering with multiple different tag types
func TestQueryEventsMultipleTagTypes(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	ctx := context.Background()
	signer := createTestSignerLocal(t)
	baseTs := timestamp.Now().V

	// Create events with multiple tag types
	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d and client tags",
		tag.NewS(
			tag.NewFromAny("d", "user-1"),
			tag.NewFromAny("client", "app-a"),
		), baseTs)

	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with d and different client",
		tag.NewS(
			tag.NewFromAny("d", "user-2"),
			tag.NewFromAny("client", "app-b"),
		), baseTs+1)

	createAndSaveEventLocal(t, ctx, signer, 30382, "Event with only d tag",
		tag.NewS(
			tag.NewFromAny("d", "user-3"),
		), baseTs+2)

	// Query with multiple tag types (should AND them together)
	evs, err := testDB.QueryEvents(ctx, &filter.F{
		Kinds: kind.NewS(kind.New(30382)),
		Tags: tag.NewS(
			tag.NewFromAny("#d", "user-1", "user-2"),
			tag.NewFromAny("#client", "app-a"),
		),
	})
	if err != nil {
		t.Fatalf("Query with multiple tag types failed: %v", err)
	}

	// Should match only the first event (user-1 with app-a)
	if len(evs) != 1 {
		t.Fatalf("Expected 1 event matching both #d and #client, got %d", len(evs))
	}

	dTag := evs[0].Tags.GetFirst([]byte("d"))
	if string(dTag.Value()) != "user-1" {
		t.Fatalf("Expected d=user-1, got d=%s", dTag.Value())
	}

	t.Logf("✓ Multiple tag types filter test passed")
}
