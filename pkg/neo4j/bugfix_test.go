//go:build integration
// +build integration

// Integration tests for Neo4j bug fixes.
// These tests require a running Neo4j instance and are not run by default.
//
// To run these tests:
//   1. Start Neo4j: docker compose -f pkg/neo4j/docker-compose.yaml up -d
//   2. Run tests: go test -tags=integration ./pkg/neo4j/... -v
//   3. Stop Neo4j: docker compose -f pkg/neo4j/docker-compose.yaml down
//
// Or use the helper script:
//   ./scripts/test-neo4j-integration.sh

package neo4j

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// TestLargeContactListBatching tests that kind 3 events with many follows
// don't cause OOM errors by verifying batched processing works correctly.
// This tests the fix for: "java out of memory error broadcasting a kind 3 event"
func TestLargeContactListBatching(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	// Generate a test pubkey for the author
	authorPubkey := generateTestPubkey()

	// Create a kind 3 event with 2000 follows (enough to require multiple batches)
	// With contactListBatchSize = 1000, this will require 2 batches
	numFollows := 2000
	followPubkeys := make([]string, numFollows)
	tagsList := tag.NewS()

	for i := 0; i < numFollows; i++ {
		followPubkeys[i] = generateTestPubkey()
		tagsList.Append(tag.NewFromAny("p", followPubkeys[i]))
	}

	// Create the kind 3 event
	ev := createTestEvent(t, authorPubkey, 3, tagsList, "")

	// Save the event - this should NOT cause OOM with batching
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save large contact list event: %v", err)
	}
	if exists {
		t.Fatal("Event unexpectedly already exists")
	}

	// Verify the event was saved
	eventID := hex.EncodeToString(ev.ID[:])
	checkCypher := "MATCH (e:Event {id: $id}) RETURN e.id AS id"
	result, err := testDB.ExecuteRead(ctx, checkCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to check event existence: %v", err)
	}
	if !result.Next(ctx) {
		t.Fatal("Event was not saved")
	}

	// Verify FOLLOWS relationships were created
	followsCypher := `
		MATCH (author:NostrUser {pubkey: $pubkey})-[:FOLLOWS]->(followed:NostrUser)
		RETURN count(followed) AS count
	`
	result, err = testDB.ExecuteRead(ctx, followsCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to count follows: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(numFollows) {
			t.Errorf("Expected %d follows, got %d", numFollows, count)
		}
		t.Logf("Successfully created %d FOLLOWS relationships in batches", count)
	} else {
		t.Fatal("No follow count returned")
	}

	// Verify ProcessedSocialEvent was created with correct relationship_count
	psCypher := `
		MATCH (ps:ProcessedSocialEvent {pubkey: $pubkey, event_kind: 3})
		RETURN ps.relationship_count AS count
	`
	result, err = testDB.ExecuteRead(ctx, psCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to check ProcessedSocialEvent: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(numFollows) {
			t.Errorf("ProcessedSocialEvent.relationship_count: expected %d, got %d", numFollows, count)
		}
	} else {
		t.Fatal("ProcessedSocialEvent not created")
	}
}

// TestMultipleETagsWithClause tests that events with multiple e-tags
// generate valid Cypher (WITH between FOREACH and OPTIONAL MATCH).
// This tests the fix for: "WITH is required between FOREACH and MATCH"
func TestMultipleETagsWithClause(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	// First, create some events that will be referenced
	refEventIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		refPubkey := generateTestPubkey()
		refTags := tag.NewS()
		refEv := createTestEvent(t, refPubkey, 1, refTags, "referenced event")
		exists, err := testDB.SaveEvent(ctx, refEv)
		if err != nil {
			t.Fatalf("Failed to save reference event %d: %v", i, err)
		}
		if exists {
			t.Fatalf("Reference event %d unexpectedly exists", i)
		}
		refEventIDs[i] = hex.EncodeToString(refEv.ID[:])
	}

	// Create a kind 5 delete event that references multiple events (multiple e-tags)
	authorPubkey := generateTestPubkey()
	tagsList := tag.NewS()
	for _, refID := range refEventIDs {
		tagsList.Append(tag.NewFromAny("e", refID))
	}

	// Create the kind 5 event with multiple e-tags
	ev := createTestEvent(t, authorPubkey, 5, tagsList, "")

	// Save the event - this should NOT fail with Cypher syntax error
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with multiple e-tags: %v\n"+
			"This indicates the WITH clause fix is not working", err)
	}
	if exists {
		t.Fatal("Event unexpectedly already exists")
	}

	// Verify the event was saved
	eventID := hex.EncodeToString(ev.ID[:])
	checkCypher := "MATCH (e:Event {id: $id}) RETURN e.id AS id"
	result, err := testDB.ExecuteRead(ctx, checkCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to check event existence: %v", err)
	}
	if !result.Next(ctx) {
		t.Fatal("Event was not saved")
	}

	// Verify REFERENCES relationships were created
	refCypher := `
		MATCH (e:Event {id: $id})-[:REFERENCES]->(ref:Event)
		RETURN count(ref) AS count
	`
	result, err = testDB.ExecuteRead(ctx, refCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to count references: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(len(refEventIDs)) {
			t.Errorf("Expected %d REFERENCES relationships, got %d", len(refEventIDs), count)
		}
		t.Logf("Successfully created %d REFERENCES relationships", count)
	} else {
		t.Fatal("No reference count returned")
	}
}

// TestLargeMuteListBatching tests that kind 10000 events with many mutes
// don't cause OOM errors by verifying batched processing works correctly.
func TestLargeMuteListBatching(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	// Generate a test pubkey for the author
	authorPubkey := generateTestPubkey()

	// Create a kind 10000 event with 1500 mutes (enough to require 2 batches)
	numMutes := 1500
	tagsList := tag.NewS()

	for i := 0; i < numMutes; i++ {
		mutePubkey := generateTestPubkey()
		tagsList.Append(tag.NewFromAny("p", mutePubkey))
	}

	// Create the kind 10000 event
	ev := createTestEvent(t, authorPubkey, 10000, tagsList, "")

	// Save the event - this should NOT cause OOM with batching
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save large mute list event: %v", err)
	}
	if exists {
		t.Fatal("Event unexpectedly already exists")
	}

	// Verify MUTES relationships were created
	mutesCypher := `
		MATCH (author:NostrUser {pubkey: $pubkey})-[:MUTES]->(muted:NostrUser)
		RETURN count(muted) AS count
	`
	result, err := testDB.ExecuteRead(ctx, mutesCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to count mutes: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(numMutes) {
			t.Errorf("Expected %d mutes, got %d", numMutes, count)
		}
		t.Logf("Successfully created %d MUTES relationships in batches", count)
	} else {
		t.Fatal("No mute count returned")
	}
}

// TestContactListUpdate tests that updating a contact list (replacing one kind 3 with another)
// correctly handles the diff and batching.
func TestContactListUpdate(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	authorPubkey := generateTestPubkey()

	// Create initial contact list with 500 follows
	initialFollows := make([]string, 500)
	tagsList1 := tag.NewS()
	for i := 0; i < 500; i++ {
		initialFollows[i] = generateTestPubkey()
		tagsList1.Append(tag.NewFromAny("p", initialFollows[i]))
	}

	ev1 := createTestEventWithTimestamp(t, authorPubkey, 3, tagsList1, "", time.Now().Unix()-100)
	_, err := testDB.SaveEvent(ctx, ev1)
	if err != nil {
		t.Fatalf("Failed to save initial contact list: %v", err)
	}

	// Verify initial follows count
	countCypher := `
		MATCH (author:NostrUser {pubkey: $pubkey})-[:FOLLOWS]->(followed:NostrUser)
		RETURN count(followed) AS count
	`
	result, err := testDB.ExecuteRead(ctx, countCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to count initial follows: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 500 {
			t.Errorf("Initial follows: expected 500, got %d", count)
		}
	}

	// Create updated contact list: remove 100 old follows, add 200 new ones
	tagsList2 := tag.NewS()
	// Keep first 400 of the original follows
	for i := 0; i < 400; i++ {
		tagsList2.Append(tag.NewFromAny("p", initialFollows[i]))
	}
	// Add 200 new follows
	for i := 0; i < 200; i++ {
		tagsList2.Append(tag.NewFromAny("p", generateTestPubkey()))
	}

	ev2 := createTestEventWithTimestamp(t, authorPubkey, 3, tagsList2, "", time.Now().Unix())
	_, err = testDB.SaveEvent(ctx, ev2)
	if err != nil {
		t.Fatalf("Failed to save updated contact list: %v", err)
	}

	// Verify final follows count (should be 600)
	result, err = testDB.ExecuteRead(ctx, countCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to count final follows: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 600 {
			t.Errorf("Final follows: expected 600, got %d", count)
		}
		t.Logf("Contact list update successful: 500 -> 600 follows (removed 100, added 200)")
	}

	// Verify old ProcessedSocialEvent is marked as superseded
	supersededCypher := `
		MATCH (ps:ProcessedSocialEvent {pubkey: $pubkey, event_kind: 3})
		WHERE ps.superseded_by IS NOT NULL
		RETURN count(ps) AS count
	`
	result, err = testDB.ExecuteRead(ctx, supersededCypher, map[string]any{"pubkey": authorPubkey})
	if err != nil {
		t.Fatalf("Failed to check superseded events: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 1 {
			t.Errorf("Expected 1 superseded ProcessedSocialEvent, got %d", count)
		}
	}
}

// TestMixedTagsEvent tests that events with e-tags, p-tags, and other tags
// all generate valid Cypher with proper WITH clauses.
func TestMixedTagsEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	// Create some referenced events
	refEventIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		refPubkey := generateTestPubkey()
		refTags := tag.NewS()
		refEv := createTestEvent(t, refPubkey, 1, refTags, "ref")
		testDB.SaveEvent(ctx, refEv)
		refEventIDs[i] = hex.EncodeToString(refEv.ID[:])
	}

	// Create an event with mixed tags: e-tags, p-tags, and other tags
	authorPubkey := generateTestPubkey()
	tagsList := tag.NewS(
		// e-tags (event references)
		tag.NewFromAny("e", refEventIDs[0]),
		tag.NewFromAny("e", refEventIDs[1]),
		tag.NewFromAny("e", refEventIDs[2]),
		// p-tags (pubkey mentions)
		tag.NewFromAny("p", generateTestPubkey()),
		tag.NewFromAny("p", generateTestPubkey()),
		// other tags
		tag.NewFromAny("t", "nostr"),
		tag.NewFromAny("t", "test"),
		tag.NewFromAny("subject", "Test Subject"),
	)

	ev := createTestEvent(t, authorPubkey, 1, tagsList, "Mixed tags test")

	// Save the event - should not fail with Cypher syntax errors
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with mixed tags: %v", err)
	}
	if exists {
		t.Fatal("Event unexpectedly already exists")
	}

	eventID := hex.EncodeToString(ev.ID[:])

	// Verify REFERENCES relationships
	refCypher := `MATCH (e:Event {id: $id})-[:REFERENCES]->(ref:Event) RETURN count(ref) AS count`
	result, err := testDB.ExecuteRead(ctx, refCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to count references: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 3 {
			t.Errorf("Expected 3 REFERENCES, got %d", count)
		}
	}

	// Verify MENTIONS relationships
	mentionsCypher := `MATCH (e:Event {id: $id})-[:MENTIONS]->(u:NostrUser) RETURN count(u) AS count`
	result, err = testDB.ExecuteRead(ctx, mentionsCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to count mentions: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 2 {
			t.Errorf("Expected 2 MENTIONS, got %d", count)
		}
	}

	// Verify TAGGED_WITH relationships
	taggedCypher := `MATCH (e:Event {id: $id})-[:TAGGED_WITH]->(t:Tag) RETURN count(t) AS count`
	result, err = testDB.ExecuteRead(ctx, taggedCypher, map[string]any{"id": eventID})
	if err != nil {
		t.Fatalf("Failed to count tags: %v", err)
	}
	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 3 {
			t.Errorf("Expected 3 TAGGED_WITH, got %d", count)
		}
	}

	t.Log("Mixed tags event saved successfully with all relationship types")
}

// Helper functions

func generateTestPubkey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func createTestEvent(t *testing.T, pubkey string, kind uint16, tagsList *tag.S, content string) *event.E {
	t.Helper()
	return createTestEventWithTimestamp(t, pubkey, kind, tagsList, content, time.Now().Unix())
}

func createTestEventWithTimestamp(t *testing.T, pubkey string, kind uint16, tagsList *tag.S, content string, timestamp int64) *event.E {
	t.Helper()

	// Decode pubkey
	pubkeyBytes, err := hex.DecodeString(pubkey)
	if err != nil {
		t.Fatalf("Invalid pubkey: %v", err)
	}

	// Generate random ID and signature (for testing purposes)
	idBytes := make([]byte, 32)
	rand.Read(idBytes)
	sigBytes := make([]byte, 64)
	rand.Read(sigBytes)

	// event.E uses []byte slices, not [32]byte arrays, so we need to assign directly
	ev := &event.E{
		Kind:      kind,
		Tags:      tagsList,
		Content:   []byte(content),
		CreatedAt: timestamp,
		Pubkey:    pubkeyBytes,
		ID:        idBytes,
		Sig:       sigBytes,
	}

	return ev
}
