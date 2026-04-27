package neo4j

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// TestBuildBaseEventCypher verifies the base event creation query generates correct Cypher.
// The new architecture separates event creation from tag processing to avoid stack overflow.
func TestBuildBaseEventCypher(t *testing.T) {
	n := &N{}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	tests := []struct {
		name        string
		tags        *tag.S
		description string
	}{
		{
			name:        "NoTags",
			tags:        nil,
			description: "Event without tags",
		},
		{
			name: "WithPTags",
			tags: tag.NewS(
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000002"),
			),
			description: "Event with p-tags (stored in tags JSON, relationships added separately)",
		},
		{
			name: "WithETags",
			tags: tag.NewS(
				tag.NewFromAny("e", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				tag.NewFromAny("e", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			),
			description: "Event with e-tags (stored in tags JSON, relationships added separately)",
		},
		{
			name: "MixedTags",
			tags: tag.NewS(
				tag.NewFromAny("t", "nostr"),
				tag.NewFromAny("e", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000003"),
			),
			description: "Event with mixed tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := event.New()
			ev.Pubkey = signer.Pub()
			ev.CreatedAt = timestamp.Now().V
			ev.Kind = 1
			ev.Content = []byte(fmt.Sprintf("Test content for %s", tt.name))
			ev.Tags = tt.tags

			if err := ev.Sign(signer); err != nil {
				t.Fatalf("Failed to sign event: %v", err)
			}

			cypher, params := n.buildBaseEventCypher(ev, 12345)

			// Base event Cypher should NOT contain tag relationship clauses
			// (tags are added separately via addTagsInBatches)
			if strings.Contains(cypher, "OPTIONAL MATCH") {
				t.Errorf("%s: buildBaseEventCypher should NOT contain OPTIONAL MATCH", tt.description)
			}
			if strings.Contains(cypher, "UNWIND") {
				t.Errorf("%s: buildBaseEventCypher should NOT contain UNWIND", tt.description)
			}
			if strings.Contains(cypher, ":REFERENCES") {
				t.Errorf("%s: buildBaseEventCypher should NOT contain :REFERENCES", tt.description)
			}
			if strings.Contains(cypher, ":MENTIONS") {
				t.Errorf("%s: buildBaseEventCypher should NOT contain :MENTIONS", tt.description)
			}
			if strings.Contains(cypher, ":TAGGED_WITH") {
				t.Errorf("%s: buildBaseEventCypher should NOT contain :TAGGED_WITH", tt.description)
			}

			// Should contain basic event creation elements
			if !strings.Contains(cypher, "CREATE (e:Event") {
				t.Errorf("%s: should CREATE Event node", tt.description)
			}
			if !strings.Contains(cypher, "MERGE (a:NostrUser") {
				t.Errorf("%s: should MERGE NostrUser node", tt.description)
			}
			if !strings.Contains(cypher, ":AUTHORED_BY") {
				t.Errorf("%s: should create AUTHORED_BY relationship", tt.description)
			}

			// Should have tags serialized in params
			if _, ok := params["tags"]; !ok {
				t.Errorf("%s: params should contain serialized tags", tt.description)
			}

			// Validate params have required fields
			requiredParams := []string{"eventId", "serial", "kind", "createdAt", "content", "sig", "pubkey", "tags", "expiration"}
			for _, p := range requiredParams {
				if _, ok := params[p]; !ok {
					t.Errorf("%s: missing required param: %s", tt.description, p)
				}
			}

			t.Logf("✓ %s: base event Cypher is clean (no tag relationships)", tt.name)
		})
	}
}

// TestSafePrefix validates the safePrefix helper function
func TestSafePrefix(t *testing.T) {
	tests := []struct {
		input    string
		n        int
		expected string
	}{
		{"hello world", 5, "hello"},
		{"hi", 5, "hi"},
		{"", 5, ""},
		{"1234567890", 10, "1234567890"},
		{"1234567890", 11, "1234567890"},
		{"0123456789abcdef", 8, "01234567"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q[:%d]", tt.input, tt.n), func(t *testing.T) {
			result := safePrefix(tt.input, tt.n)
			if result != tt.expected {
				t.Errorf("safePrefix(%q, %d) = %q; want %q", tt.input, tt.n, result, tt.expected)
			}
		})
	}
}

// TestSaveEvent_ETagReference tests that events with e-tags are saved correctly
// using the Tag-based model: Event-[:TAGGED_WITH]->Tag-[:REFERENCES]->Event.
// Uses shared testDB from testmain_test.go to avoid auth rate limiting.
func TestSaveEvent_ETagReference(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	// Generate keypairs
	alice, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := alice.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	bob, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := bob.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create a root event from Alice
	rootEvent := event.New()
	rootEvent.Pubkey = alice.Pub()
	rootEvent.CreatedAt = timestamp.Now().V
	rootEvent.Kind = 1
	rootEvent.Content = []byte("This is the root event")

	if err := rootEvent.Sign(alice); err != nil {
		t.Fatalf("Failed to sign root event: %v", err)
	}

	// Save root event
	exists, err := testDB.SaveEvent(ctx, rootEvent)
	if err != nil {
		t.Fatalf("Failed to save root event: %v", err)
	}
	if exists {
		t.Fatal("Root event should not exist yet")
	}

	rootEventID := hex.Enc(rootEvent.ID[:])

	// Create a reply from Bob that references the root event
	replyEvent := event.New()
	replyEvent.Pubkey = bob.Pub()
	replyEvent.CreatedAt = timestamp.Now().V + 1
	replyEvent.Kind = 1
	replyEvent.Content = []byte("This is a reply to Alice")
	replyEvent.Tags = tag.NewS(
		tag.NewFromAny("e", rootEventID, "", "root"),
		tag.NewFromAny("p", hex.Enc(alice.Pub())),
	)

	if err := replyEvent.Sign(bob); err != nil {
		t.Fatalf("Failed to sign reply event: %v", err)
	}

	// Save reply event - this exercises the batched tag creation
	exists, err = testDB.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}
	if exists {
		t.Fatal("Reply event should not exist yet")
	}

	// Verify Tag-based e-tag model: Event-[:TAGGED_WITH]->Tag{type:'e'}-[:REFERENCES]->Event
	cypher := `
		MATCH (reply:Event {id: $replyId})-[:TAGGED_WITH]->(t:Tag {type: 'e', value: $rootId})-[:REFERENCES]->(root:Event {id: $rootId})
		RETURN reply.id AS replyId, t.value AS tagValue, root.id AS rootId
	`
	params := map[string]any{
		"replyId": hex.Enc(replyEvent.ID[:]),
		"rootId":  rootEventID,
	}

	result, err := testDB.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query Tag-based REFERENCES: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Expected Tag-based REFERENCES relationship between reply and root events")
	} else {
		record := result.Record()
		returnedReplyId := record.Values[0].(string)
		tagValue := record.Values[1].(string)
		returnedRootId := record.Values[2].(string)
		t.Logf("✓ Tag-based REFERENCES verified: Event(%s) -> Tag{e:%s} -> Event(%s)", returnedReplyId[:8], tagValue[:8], returnedRootId[:8])
	}

	// Verify Tag-based p-tag model: Event-[:TAGGED_WITH]->Tag{type:'p'}-[:REFERENCES]->NostrUser
	pTagCypher := `
		MATCH (reply:Event {id: $replyId})-[:TAGGED_WITH]->(t:Tag {type: 'p', value: $authorPubkey})-[:REFERENCES]->(author:NostrUser {pubkey: $authorPubkey})
		RETURN author.pubkey AS pubkey, t.value AS tagValue
	`
	pTagParams := map[string]any{
		"replyId":      hex.Enc(replyEvent.ID[:]),
		"authorPubkey": hex.Enc(alice.Pub()),
	}

	pTagResult, err := testDB.ExecuteRead(ctx, pTagCypher, pTagParams)
	if err != nil {
		t.Fatalf("Failed to query Tag-based p-tag: %v", err)
	}

	if !pTagResult.Next(ctx) {
		t.Error("Expected Tag-based p-tag relationship")
	} else {
		t.Logf("✓ Tag-based p-tag relationship verified")
	}
}

// TestSaveEvent_ETagMissingReference tests that e-tags to non-existent events
// create Tag nodes but don't create REFERENCES relationships to missing events.
// Uses shared testDB from testmain_test.go to avoid auth rate limiting.
func TestSaveEvent_ETagMissingReference(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create an event that references a non-existent event
	nonExistentEventID := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Reply to ghost event")
	ev.Tags = tag.NewS(
		tag.NewFromAny("e", nonExistentEventID, "", "reply"),
	)

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// Save should succeed (batched e-tag processing handles missing reference)
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with missing reference: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify event was saved
	checkCypher := "MATCH (e:Event {id: $id}) RETURN e.id AS id"
	checkParams := map[string]any{"id": hex.Enc(ev.ID[:])}

	result, err := testDB.ExecuteRead(ctx, checkCypher, checkParams)
	if err != nil {
		t.Fatalf("Failed to check event: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Event should have been saved despite missing reference")
	}

	// Verify Tag node was created with TAGGED_WITH relationship
	tagCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'e', value: $refId})
		RETURN t.value AS tagValue
	`
	tagParams := map[string]any{
		"eventId": hex.Enc(ev.ID[:]),
		"refId":   nonExistentEventID,
	}

	tagResult, err := testDB.ExecuteRead(ctx, tagCypher, tagParams)
	if err != nil {
		t.Fatalf("Failed to check Tag node: %v", err)
	}

	if !tagResult.Next(ctx) {
		t.Error("Expected Tag node to be created for e-tag even when target doesn't exist")
	} else {
		t.Logf("✓ Tag node created for missing reference")
	}

	// Verify no REFERENCES relationship was created from Tag (as the target Event doesn't exist)
	refCypher := `
		MATCH (t:Tag {type: 'e', value: $refId})-[:REFERENCES]->(ref:Event)
		RETURN count(ref) AS refCount
	`
	refParams := map[string]any{"refId": nonExistentEventID}

	refResult, err := testDB.ExecuteRead(ctx, refCypher, refParams)
	if err != nil {
		t.Fatalf("Failed to check REFERENCES from Tag: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count > 0 {
			t.Errorf("Expected no REFERENCES from Tag for non-existent event, got %d", count)
		} else {
			t.Logf("✓ Correctly handled missing reference (no REFERENCES from Tag)")
		}
	}
}

// TestSaveEvent_MultipleETags tests events with multiple e-tags using Tag-based model.
// Uses shared testDB from testmain_test.go to avoid auth rate limiting.
func TestSaveEvent_MultipleETags(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create three events first
	var eventIDs []string
	for i := 0; i < 3; i++ {
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Kind = 1
		ev.Content = []byte(fmt.Sprintf("Event %d", i))

		if err := ev.Sign(signer); err != nil {
			t.Fatalf("Failed to sign event %d: %v", i, err)
		}

		if _, err := testDB.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save event %d: %v", i, err)
		}

		eventIDs = append(eventIDs, hex.Enc(ev.ID[:]))
	}

	// Create an event that references all three
	replyEvent := event.New()
	replyEvent.Pubkey = signer.Pub()
	replyEvent.CreatedAt = timestamp.Now().V + 10
	replyEvent.Kind = 1
	replyEvent.Content = []byte("This references multiple events")
	replyEvent.Tags = tag.NewS(
		tag.NewFromAny("e", eventIDs[0], "", "root"),
		tag.NewFromAny("e", eventIDs[1], "", "reply"),
		tag.NewFromAny("e", eventIDs[2], "", "mention"),
	)

	if err := replyEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign reply event: %v", err)
	}

	// Save reply event - tests batched e-tag creation with Tag nodes
	exists, err := testDB.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save multi-reference event: %v", err)
	}
	if exists {
		t.Fatal("Reply event should not exist yet")
	}

	// Verify all Tag-based REFERENCES relationships were created
	// Event-[:TAGGED_WITH]->Tag{type:'e'}-[:REFERENCES]->Event
	cypher := `
		MATCH (reply:Event {id: $replyId})-[:TAGGED_WITH]->(t:Tag {type: 'e'})-[:REFERENCES]->(ref:Event)
		RETURN ref.id AS refId
	`
	params := map[string]any{"replyId": hex.Enc(replyEvent.ID[:])}

	result, err := testDB.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query Tag-based REFERENCES: %v", err)
	}

	referencedIDs := make(map[string]bool)
	for result.Next(ctx) {
		refID := result.Record().Values[0].(string)
		referencedIDs[refID] = true
	}

	if len(referencedIDs) != 3 {
		t.Errorf("Expected 3 Tag-based REFERENCES, got %d", len(referencedIDs))
	}

	for i, id := range eventIDs {
		if !referencedIDs[id] {
			t.Errorf("Missing Tag-based REFERENCES to event %d (%s)", i, id[:8])
		}
	}

	t.Logf("✓ All %d Tag-based REFERENCES created successfully", len(referencedIDs))
}

// TestSaveEvent_LargePTagBatch tests that events with many p-tags are saved correctly
// using batched Tag-based processing to avoid Neo4j stack overflow.
// Uses shared testDB from testmain_test.go to avoid auth rate limiting.
func TestSaveEvent_LargePTagBatch(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean up before test
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create event with many p-tags (enough to require multiple batches)
	// With tagBatchSize = 500, this will require 2 batches
	numTags := 600
	manyPTags := tag.NewS()
	for i := 0; i < numTags; i++ {
		manyPTags.Append(tag.NewFromAny("p", fmt.Sprintf("%064x", i)))
	}

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 3 // Contact list
	ev.Content = []byte("")
	ev.Tags = manyPTags

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// This should succeed with batched processing
	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with %d p-tags: %v", numTags, err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify all Tag nodes were created with TAGGED_WITH relationships
	tagCountCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'p'})
		RETURN count(t) AS tagCount
	`
	tagCountParams := map[string]any{"eventId": hex.Enc(ev.ID[:])}

	tagResult, err := testDB.ExecuteRead(ctx, tagCountCypher, tagCountParams)
	if err != nil {
		t.Fatalf("Failed to count p-tag Tag nodes: %v", err)
	}

	if tagResult.Next(ctx) {
		count := tagResult.Record().Values[0].(int64)
		if count != int64(numTags) {
			t.Errorf("Expected %d Tag nodes, got %d", numTags, count)
		} else {
			t.Logf("✓ All %d p-tag Tag nodes created via batched processing", count)
		}
	}

	// Verify all REFERENCES relationships to NostrUser were created
	refCountCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'p'})-[:REFERENCES]->(u:NostrUser)
		RETURN count(u) AS refCount
	`
	refCountParams := map[string]any{"eventId": hex.Enc(ev.ID[:])}

	refResult, err := testDB.ExecuteRead(ctx, refCountCypher, refCountParams)
	if err != nil {
		t.Fatalf("Failed to count Tag-based REFERENCES to NostrUser: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count != int64(numTags) {
			t.Errorf("Expected %d REFERENCES to NostrUser, got %d", numTags, count)
		} else {
			t.Logf("✓ All %d Tag-based REFERENCES to NostrUser created via batched processing", count)
		}
	}
}
