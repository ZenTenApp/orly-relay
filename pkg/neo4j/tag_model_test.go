package neo4j

import (
	"context"
	"fmt"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// =============================================================================
// Tag-Based E/P Model Tests
// =============================================================================

// TestTagBasedModel_ETagCreatesTagNode verifies that e-tags create Tag nodes
// with type='e' and TAGGED_WITH relationships from the event.
func TestTagBasedModel_ETagCreatesTagNode(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create a target event first
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

	targetID := hex.Enc(targetEvent.ID[:])

	// Create event with e-tag referencing the target
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V + 1
	ev.Kind = 1
	ev.Content = []byte("Event with e-tag")
	ev.Tags = tag.NewS(
		tag.NewFromAny("e", targetID, "", "reply"),
	)
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	eventID := hex.Enc(ev.ID[:])

	// Verify Tag node was created
	tagCypher := `
		MATCH (t:Tag {type: 'e', value: $targetId})
		RETURN t.type AS type, t.value AS value
	`
	tagResult, err := testDB.ExecuteRead(ctx, tagCypher, map[string]any{"targetId": targetID})
	if err != nil {
		t.Fatalf("Failed to query Tag node: %v", err)
	}

	if !tagResult.Next(ctx) {
		t.Fatal("Expected Tag node with type='e' to be created")
	}

	record := tagResult.Record()
	tagType := record.Values[0].(string)
	tagValue := record.Values[1].(string)

	if tagType != "e" {
		t.Errorf("Expected tag type 'e', got %q", tagType)
	}
	if tagValue != targetID {
		t.Errorf("Expected tag value %q, got %q", targetID, tagValue)
	}

	// Verify TAGGED_WITH relationship exists
	taggedWithCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'e', value: $targetId})
		RETURN count(t) AS count
	`
	twResult, err := testDB.ExecuteRead(ctx, taggedWithCypher, map[string]any{
		"eventId":  eventID,
		"targetId": targetID,
	})
	if err != nil {
		t.Fatalf("Failed to query TAGGED_WITH: %v", err)
	}

	if twResult.Next(ctx) {
		count := twResult.Record().Values[0].(int64)
		if count != 1 {
			t.Errorf("Expected 1 TAGGED_WITH relationship, got %d", count)
		}
	} else {
		t.Fatal("Expected TAGGED_WITH relationship to exist")
	}

	// Verify REFERENCES relationship from Tag to Event
	refCypher := `
		MATCH (t:Tag {type: 'e', value: $targetId})-[:REFERENCES]->(target:Event {id: $targetId})
		RETURN count(target) AS count
	`
	refResult, err := testDB.ExecuteRead(ctx, refCypher, map[string]any{"targetId": targetID})
	if err != nil {
		t.Fatalf("Failed to query REFERENCES: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count != 1 {
			t.Errorf("Expected 1 REFERENCES relationship from Tag to Event, got %d", count)
		}
	} else {
		t.Fatal("Expected REFERENCES relationship from Tag to Event")
	}

	t.Logf("Tag-based e-tag model verified: Event -> Tag{e} -> Event")
}

// TestTagBasedModel_PTagCreatesTagNode verifies that p-tags create Tag nodes
// with type='p' and REFERENCES relationships to NostrUser nodes.
func TestTagBasedModel_PTagCreatesTagNode(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	// Create two signers: author and mentioned user
	author, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create author signer: %v", err)
	}
	if err := author.Generate(); err != nil {
		t.Fatalf("Failed to generate author keypair: %v", err)
	}

	mentioned, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create mentioned signer: %v", err)
	}
	if err := mentioned.Generate(); err != nil {
		t.Fatalf("Failed to generate mentioned keypair: %v", err)
	}

	mentionedPubkey := hex.Enc(mentioned.Pub())

	// Create event with p-tag
	ev := event.New()
	ev.Pubkey = author.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Event mentioning someone")
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", mentionedPubkey),
	)
	if err := ev.Sign(author); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	eventID := hex.Enc(ev.ID[:])

	// Verify Tag node was created
	tagCypher := `
		MATCH (t:Tag {type: 'p', value: $pubkey})
		RETURN t.type AS type, t.value AS value
	`
	tagResult, err := testDB.ExecuteRead(ctx, tagCypher, map[string]any{"pubkey": mentionedPubkey})
	if err != nil {
		t.Fatalf("Failed to query Tag node: %v", err)
	}

	if !tagResult.Next(ctx) {
		t.Fatal("Expected Tag node with type='p' to be created")
	}

	// Verify TAGGED_WITH relationship exists
	taggedWithCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'p', value: $pubkey})
		RETURN count(t) AS count
	`
	twResult, err := testDB.ExecuteRead(ctx, taggedWithCypher, map[string]any{
		"eventId": eventID,
		"pubkey":  mentionedPubkey,
	})
	if err != nil {
		t.Fatalf("Failed to query TAGGED_WITH: %v", err)
	}

	if twResult.Next(ctx) {
		count := twResult.Record().Values[0].(int64)
		if count != 1 {
			t.Errorf("Expected 1 TAGGED_WITH relationship, got %d", count)
		}
	}

	// Verify REFERENCES relationship from Tag to NostrUser
	refCypher := `
		MATCH (t:Tag {type: 'p', value: $pubkey})-[:REFERENCES]->(u:NostrUser {pubkey: $pubkey})
		RETURN count(u) AS count
	`
	refResult, err := testDB.ExecuteRead(ctx, refCypher, map[string]any{"pubkey": mentionedPubkey})
	if err != nil {
		t.Fatalf("Failed to query REFERENCES: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count != 1 {
			t.Errorf("Expected 1 REFERENCES relationship from Tag to NostrUser, got %d", count)
		}
	} else {
		t.Fatal("Expected REFERENCES relationship from Tag to NostrUser")
	}

	// Verify NostrUser was created for the mentioned pubkey
	userCypher := `
		MATCH (u:NostrUser {pubkey: $pubkey})
		RETURN u.pubkey AS pubkey
	`
	userResult, err := testDB.ExecuteRead(ctx, userCypher, map[string]any{"pubkey": mentionedPubkey})
	if err != nil {
		t.Fatalf("Failed to query NostrUser: %v", err)
	}

	if !userResult.Next(ctx) {
		t.Fatal("Expected NostrUser to be created for mentioned pubkey")
	}

	t.Logf("Tag-based p-tag model verified: Event -> Tag{p} -> NostrUser")
}

// TestTagBasedModel_ETagWithoutTargetEvent verifies that e-tags create Tag nodes
// even when the referenced event doesn't exist, but don't create REFERENCES.
func TestTagBasedModel_ETagWithoutTargetEvent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Non-existent event ID
	nonExistentID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// Create event with e-tag referencing non-existent event
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Reply to ghost event")
	ev.Tags = tag.NewS(
		tag.NewFromAny("e", nonExistentID, "", "reply"),
	)
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	eventID := hex.Enc(ev.ID[:])

	// Verify Tag node WAS created (for query purposes)
	tagCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'e', value: $targetId})
		RETURN t.value AS value
	`
	tagResult, err := testDB.ExecuteRead(ctx, tagCypher, map[string]any{
		"eventId":  eventID,
		"targetId": nonExistentID,
	})
	if err != nil {
		t.Fatalf("Failed to query Tag node: %v", err)
	}

	if !tagResult.Next(ctx) {
		t.Fatal("Expected Tag node to be created even for non-existent target")
	}

	// Verify REFERENCES was NOT created (target doesn't exist)
	refCypher := `
		MATCH (t:Tag {type: 'e', value: $targetId})-[:REFERENCES]->(target:Event)
		RETURN count(target) AS count
	`
	refResult, err := testDB.ExecuteRead(ctx, refCypher, map[string]any{"targetId": nonExistentID})
	if err != nil {
		t.Fatalf("Failed to query REFERENCES: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count != 0 {
			t.Errorf("Expected 0 REFERENCES for non-existent event, got %d", count)
		}
	}

	t.Logf("Correctly handled e-tag to non-existent event: Tag created, no REFERENCES")
}

// =============================================================================
// Tag Filter Query Tests (#e and #p filters)
// =============================================================================

// TestTagFilter_ETagQuery tests that #e filters work with the Tag-based model.
func TestTagFilter_ETagQuery(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create root event
	rootEvent := event.New()
	rootEvent.Pubkey = signer.Pub()
	rootEvent.CreatedAt = timestamp.Now().V
	rootEvent.Kind = 1
	rootEvent.Content = []byte("Root event")
	if err := rootEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign root event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, rootEvent); err != nil {
		t.Fatalf("Failed to save root event: %v", err)
	}

	rootID := hex.Enc(rootEvent.ID[:])

	// Create reply event with e-tag
	replyEvent := event.New()
	replyEvent.Pubkey = signer.Pub()
	replyEvent.CreatedAt = timestamp.Now().V + 1
	replyEvent.Kind = 1
	replyEvent.Content = []byte("Reply to root")
	replyEvent.Tags = tag.NewS(
		tag.NewFromAny("e", rootID, "", "root"),
	)
	if err := replyEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign reply event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, replyEvent); err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}

	// Create unrelated event (no e-tag)
	unrelatedEvent := event.New()
	unrelatedEvent.Pubkey = signer.Pub()
	unrelatedEvent.CreatedAt = timestamp.Now().V + 2
	unrelatedEvent.Kind = 1
	unrelatedEvent.Content = []byte("Unrelated event")
	if err := unrelatedEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign unrelated event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, unrelatedEvent); err != nil {
		t.Fatalf("Failed to save unrelated event: %v", err)
	}

	// Query events with #e filter
	f := &filter.F{
		Tags: tag.NewS(tag.NewFromAny("e", rootID)),
	}

	events, err := testDB.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query with #e filter: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event with #e filter, got %d", len(events))
	}

	if len(events) > 0 {
		foundID := hex.Enc(events[0].ID[:])
		expectedID := hex.Enc(replyEvent.ID[:])
		if foundID != expectedID {
			t.Errorf("Expected to find reply event, got event %s", foundID[:8])
		}
	}

	t.Logf("#e filter query working correctly with Tag-based model")
}

// TestTagFilter_PTagQuery tests that #p filters work with the Tag-based model.
func TestTagFilter_PTagQuery(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	// Create two signers
	author, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create author signer: %v", err)
	}
	if err := author.Generate(); err != nil {
		t.Fatalf("Failed to generate author keypair: %v", err)
	}

	mentioned, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create mentioned signer: %v", err)
	}
	if err := mentioned.Generate(); err != nil {
		t.Fatalf("Failed to generate mentioned keypair: %v", err)
	}

	mentionedPubkey := hex.Enc(mentioned.Pub())

	// Create event that mentions someone
	mentionEvent := event.New()
	mentionEvent.Pubkey = author.Pub()
	mentionEvent.CreatedAt = timestamp.Now().V
	mentionEvent.Kind = 1
	mentionEvent.Content = []byte("Hey @someone")
	mentionEvent.Tags = tag.NewS(
		tag.NewFromAny("p", mentionedPubkey),
	)
	if err := mentionEvent.Sign(author); err != nil {
		t.Fatalf("Failed to sign mention event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, mentionEvent); err != nil {
		t.Fatalf("Failed to save mention event: %v", err)
	}

	// Create event without p-tag
	regularEvent := event.New()
	regularEvent.Pubkey = author.Pub()
	regularEvent.CreatedAt = timestamp.Now().V + 1
	regularEvent.Kind = 1
	regularEvent.Content = []byte("Regular post")
	if err := regularEvent.Sign(author); err != nil {
		t.Fatalf("Failed to sign regular event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, regularEvent); err != nil {
		t.Fatalf("Failed to save regular event: %v", err)
	}

	// Query events with #p filter
	f := &filter.F{
		Tags: tag.NewS(tag.NewFromAny("p", mentionedPubkey)),
	}

	events, err := testDB.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query with #p filter: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event with #p filter, got %d", len(events))
	}

	if len(events) > 0 {
		foundID := hex.Enc(events[0].ID[:])
		expectedID := hex.Enc(mentionEvent.ID[:])
		if foundID != expectedID {
			t.Errorf("Expected to find mention event, got event %s", foundID[:8])
		}
	}

	t.Logf("#p filter query working correctly with Tag-based model")
}

// TestTagFilter_MultiplePTags tests events with multiple p-tags.
func TestTagFilter_MultiplePTags(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	author, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create author signer: %v", err)
	}
	if err := author.Generate(); err != nil {
		t.Fatalf("Failed to generate author keypair: %v", err)
	}

	// Generate 5 pubkeys to mention
	var mentionedPubkeys []string
	for i := 0; i < 5; i++ {
		mentionedPubkeys = append(mentionedPubkeys, fmt.Sprintf("%064x", i+1))
	}

	// Create event mentioning all 5
	ev := event.New()
	ev.Pubkey = author.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Group mention")
	tags := tag.NewS()
	for _, pk := range mentionedPubkeys {
		tags.Append(tag.NewFromAny("p", pk))
	}
	ev.Tags = tags
	if err := ev.Sign(author); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify all Tag nodes were created
	countCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'p'})
		RETURN count(t) AS count
	`
	result, err := testDB.ExecuteRead(ctx, countCypher, map[string]any{"eventId": hex.Enc(ev.ID[:])})
	if err != nil {
		t.Fatalf("Failed to count p-tag Tags: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(len(mentionedPubkeys)) {
			t.Errorf("Expected %d p-tag Tag nodes, got %d", len(mentionedPubkeys), count)
		}
	}

	// Query for events mentioning any of the pubkeys
	f := &filter.F{
		Tags: tag.NewS(tag.NewFromAny("p", mentionedPubkeys[2])), // Query for the 3rd pubkey
	}

	events, err := testDB.QueryEvents(ctx, f)
	if err != nil {
		t.Fatalf("Failed to query with #p filter: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event mentioning pubkey, got %d", len(events))
	}

	t.Logf("Multiple p-tags correctly stored and queryable")
}

// =============================================================================
// CheckForDeleted with Tag Traversal Tests
// =============================================================================

// TestCheckForDeleted_WithTagModel tests that CheckForDeleted works with
// the new Tag-based model for e-tag references.
func TestCheckForDeleted_WithTagModel(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

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
	targetEvent.Content = []byte("Event to be deleted")
	if err := targetEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign target event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, targetEvent); err != nil {
		t.Fatalf("Failed to save target event: %v", err)
	}

	targetID := hex.Enc(targetEvent.ID[:])

	// Create kind 5 deletion event with e-tag
	deleteEvent := event.New()
	deleteEvent.Pubkey = signer.Pub()
	deleteEvent.CreatedAt = timestamp.Now().V + 1
	deleteEvent.Kind = 5
	deleteEvent.Content = []byte("Deleting my event")
	deleteEvent.Tags = tag.NewS(
		tag.NewFromAny("e", targetID),
	)
	if err := deleteEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign delete event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, deleteEvent); err != nil {
		t.Fatalf("Failed to save delete event: %v", err)
	}

	// Verify the Tag-based traversal exists:
	// DeleteEvent-[:TAGGED_WITH]->Tag{type:'e'}-[:REFERENCES]->TargetEvent
	traversalCypher := `
		MATCH (delete:Event {kind: 5})-[:TAGGED_WITH]->(t:Tag {type: 'e'})-[:REFERENCES]->(target:Event {id: $targetId})
		RETURN delete.id AS deleteId
	`
	result, err := testDB.ExecuteRead(ctx, traversalCypher, map[string]any{"targetId": targetID})
	if err != nil {
		t.Fatalf("Failed to query traversal: %v", err)
	}

	if !result.Next(ctx) {
		t.Fatal("Expected Tag-based traversal from delete event to target")
	}

	// Test CheckForDeleted
	admins := [][]byte{} // No admins, author can delete own events
	err = testDB.CheckForDeleted(targetEvent, admins)

	if err == nil {
		t.Error("Expected CheckForDeleted to return error for deleted event")
	} else if err.Error() != "event has been deleted" {
		t.Errorf("Unexpected error message: %v", err)
	}

	t.Logf("CheckForDeleted correctly detects deletion via Tag-based traversal")
}

// TestCheckForDeleted_NotDeleted verifies CheckForDeleted returns nil for
// events that haven't been deleted.
func TestCheckForDeleted_NotDeleted(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create event that won't be deleted
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Regular event")
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// CheckForDeleted should return nil
	admins := [][]byte{}
	err = testDB.CheckForDeleted(ev, admins)

	if err != nil {
		t.Errorf("Expected nil for non-deleted event, got: %v", err)
	}

	t.Logf("CheckForDeleted correctly returns nil for non-deleted event")
}

// =============================================================================
// Migration v3 Tests
// =============================================================================

// TestMigrationV3_ConvertDirectReferences tests that the v3 migration
// correctly converts direct REFERENCES relationships to Tag-based model.
func TestMigrationV3_ConvertDirectReferences(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	// Manually create old-style direct REFERENCES relationship
	// (simulating pre-migration data)
	setupCypher := `
		// Create two events
		CREATE (source:Event {
			id: '1111111111111111111111111111111111111111111111111111111111111111',
			pubkey: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			kind: 1,
			created_at: 1700000000,
			content: 'Source event',
			sig: '0000000000000000000000000000000000000000000000000000000000000000' +
			     '0000000000000000000000000000000000000000000000000000000000000000',
			tags: '[]',
			serial: 1,
			expiration: 0
		})
		CREATE (target:Event {
			id: '2222222222222222222222222222222222222222222222222222222222222222',
			pubkey: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			kind: 1,
			created_at: 1699999999,
			content: 'Target event',
			sig: '0000000000000000000000000000000000000000000000000000000000000000' +
			     '0000000000000000000000000000000000000000000000000000000000000000',
			tags: '[]',
			serial: 2,
			expiration: 0
		})
		// Create old-style direct REFERENCES (pre-migration)
		CREATE (source)-[:REFERENCES]->(target)
	`

	if _, err := testDB.ExecuteWrite(ctx, setupCypher, nil); err != nil {
		t.Fatalf("Failed to setup pre-migration data: %v", err)
	}

	// Verify old-style relationship exists
	checkOldCypher := `
		MATCH (s:Event)-[r:REFERENCES]->(t:Event)
		WHERE NOT (s)-[:TAGGED_WITH]->(:Tag)
		RETURN count(r) AS count
	`
	result, err := testDB.ExecuteRead(ctx, checkOldCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check old relationship: %v", err)
	}

	var oldCount int64
	if result.Next(ctx) {
		oldCount = result.Record().Values[0].(int64)
	}

	if oldCount == 0 {
		t.Skip("No old-style REFERENCES to migrate")
	}

	t.Logf("Found %d old-style REFERENCES to migrate", oldCount)

	// Run migration
	err = migrateToTagBasedReferences(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify old-style relationship was removed
	result, err = testDB.ExecuteRead(ctx, checkOldCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check post-migration: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 0 {
			t.Errorf("Expected 0 old-style REFERENCES after migration, got %d", count)
		}
	}

	// Verify new Tag-based structure exists
	checkNewCypher := `
		MATCH (s:Event {id: '1111111111111111111111111111111111111111111111111111111111111111'})
			-[:TAGGED_WITH]->(t:Tag {type: 'e'})
			-[:REFERENCES]->(target:Event {id: '2222222222222222222222222222222222222222222222222222222222222222'})
		RETURN t.value AS tagValue
	`
	result, err = testDB.ExecuteRead(ctx, checkNewCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check new structure: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Expected Tag-based structure after migration")
	} else {
		tagValue := result.Record().Values[0].(string)
		expectedValue := "2222222222222222222222222222222222222222222222222222222222222222"
		if tagValue != expectedValue {
			t.Errorf("Expected tag value %s, got %s", expectedValue, tagValue)
		}
	}

	t.Logf("Migration v3 correctly converted REFERENCES to Tag-based model")
}

// TestMigrationV3_ConvertDirectMentions tests that the v3 migration
// correctly converts direct MENTIONS relationships to Tag-based model.
func TestMigrationV3_ConvertDirectMentions(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	// Manually create old-style direct MENTIONS relationship
	setupCypher := `
		// Create event and user
		CREATE (source:Event {
			id: '3333333333333333333333333333333333333333333333333333333333333333',
			pubkey: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			kind: 1,
			created_at: 1700000000,
			content: 'Event mentioning user',
			sig: '0000000000000000000000000000000000000000000000000000000000000000' +
			     '0000000000000000000000000000000000000000000000000000000000000000',
			tags: '[]',
			serial: 3,
			expiration: 0
		})
		MERGE (user:NostrUser {pubkey: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'})
		// Create old-style direct MENTIONS (pre-migration)
		CREATE (source)-[:MENTIONS]->(user)
	`

	if _, err := testDB.ExecuteWrite(ctx, setupCypher, nil); err != nil {
		t.Fatalf("Failed to setup pre-migration data: %v", err)
	}

	// Verify old-style relationship exists
	checkOldCypher := `
		MATCH (e:Event)-[r:MENTIONS]->(u:NostrUser)
		RETURN count(r) AS count
	`
	result, err := testDB.ExecuteRead(ctx, checkOldCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check old relationship: %v", err)
	}

	var oldCount int64
	if result.Next(ctx) {
		oldCount = result.Record().Values[0].(int64)
	}

	if oldCount == 0 {
		t.Skip("No old-style MENTIONS to migrate")
	}

	t.Logf("Found %d old-style MENTIONS to migrate", oldCount)

	// Run migration
	err = migrateToTagBasedReferences(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify old-style relationship was removed
	result, err = testDB.ExecuteRead(ctx, checkOldCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check post-migration: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != 0 {
			t.Errorf("Expected 0 old-style MENTIONS after migration, got %d", count)
		}
	}

	// Verify new Tag-based structure exists
	checkNewCypher := `
		MATCH (e:Event {id: '3333333333333333333333333333333333333333333333333333333333333333'})
			-[:TAGGED_WITH]->(t:Tag {type: 'p'})
			-[:REFERENCES]->(u:NostrUser {pubkey: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'})
		RETURN t.value AS tagValue
	`
	result, err = testDB.ExecuteRead(ctx, checkNewCypher, nil)
	if err != nil {
		t.Fatalf("Failed to check new structure: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Expected Tag-based structure after migration")
	} else {
		tagValue := result.Record().Values[0].(string)
		expectedValue := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		if tagValue != expectedValue {
			t.Errorf("Expected tag value %s, got %s", expectedValue, tagValue)
		}
	}

	t.Logf("Migration v3 correctly converted MENTIONS to Tag-based model")
}

// TestMigrationV3_Idempotent tests that the v3 migration is idempotent
// (safe to run multiple times).
func TestMigrationV3_Idempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	// Create proper Tag-based structure (as if migration already ran)
	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create event with e-tag using new model
	targetEvent := event.New()
	targetEvent.Pubkey = signer.Pub()
	targetEvent.CreatedAt = timestamp.Now().V
	targetEvent.Kind = 1
	targetEvent.Content = []byte("Target")
	if err := targetEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign target event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, targetEvent); err != nil {
		t.Fatalf("Failed to save target event: %v", err)
	}

	replyEvent := event.New()
	replyEvent.Pubkey = signer.Pub()
	replyEvent.CreatedAt = timestamp.Now().V + 1
	replyEvent.Kind = 1
	replyEvent.Content = []byte("Reply")
	replyEvent.Tags = tag.NewS(
		tag.NewFromAny("e", hex.Enc(targetEvent.ID[:])),
	)
	if err := replyEvent.Sign(signer); err != nil {
		t.Fatalf("Failed to sign reply event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, replyEvent); err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}

	// Count Tag nodes before running migration
	countBefore := countNodes(t, "Tag")

	// Run migration (should be no-op since data is already correct)
	err = migrateToTagBasedReferences(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Count Tag nodes after - should be unchanged
	countAfter := countNodes(t, "Tag")

	if countBefore != countAfter {
		t.Errorf("Migration changed Tag count: before=%d, after=%d", countBefore, countAfter)
	}

	// Run migration again - should still be idempotent
	err = migrateToTagBasedReferences(ctx, testDB)
	if err != nil {
		t.Fatalf("Second migration run failed: %v", err)
	}

	countAfterSecond := countNodes(t, "Tag")
	if countAfter != countAfterSecond {
		t.Errorf("Second migration run changed Tag count: %d -> %d", countAfter, countAfterSecond)
	}

	t.Logf("Migration v3 is idempotent (safe to run multiple times)")
}

// =============================================================================
// Large Dataset Tests
// =============================================================================

// TestLargeETagBatch tests events with many e-tags are handled correctly.
func TestLargeETagBatch(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()
	cleanTestDatabase()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create 100 target events
	numTargets := 100
	var targetIDs []string
	for i := 0; i < numTargets; i++ {
		targetEvent := event.New()
		targetEvent.Pubkey = signer.Pub()
		targetEvent.CreatedAt = timestamp.Now().V + int64(i)
		targetEvent.Kind = 1
		targetEvent.Content = []byte(fmt.Sprintf("Target %d", i))
		if err := targetEvent.Sign(signer); err != nil {
			t.Fatalf("Failed to sign target event %d: %v", i, err)
		}

		if _, err := testDB.SaveEvent(ctx, targetEvent); err != nil {
			t.Fatalf("Failed to save target event %d: %v", i, err)
		}

		targetIDs = append(targetIDs, hex.Enc(targetEvent.ID[:]))
	}

	// Create event referencing all 100 targets
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V + int64(numTargets+1)
	ev.Kind = 1
	ev.Content = []byte("Event with many e-tags")
	tags := tag.NewS()
	for _, id := range targetIDs {
		tags.Append(tag.NewFromAny("e", id))
	}
	ev.Tags = tags
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := testDB.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event with %d e-tags: %v", numTargets, err)
	}

	// Verify all Tag nodes were created
	countCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'e'})
		RETURN count(t) AS count
	`
	result, err := testDB.ExecuteRead(ctx, countCypher, map[string]any{"eventId": hex.Enc(ev.ID[:])})
	if err != nil {
		t.Fatalf("Failed to count e-tag Tags: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count != int64(numTargets) {
			t.Errorf("Expected %d e-tag Tag nodes, got %d", numTargets, count)
		}
	}

	// Verify all REFERENCES were created (since all targets exist)
	refCountCypher := `
		MATCH (e:Event {id: $eventId})-[:TAGGED_WITH]->(t:Tag {type: 'e'})-[:REFERENCES]->(target:Event)
		RETURN count(target) AS count
	`
	refResult, err := testDB.ExecuteRead(ctx, refCountCypher, map[string]any{"eventId": hex.Enc(ev.ID[:])})
	if err != nil {
		t.Fatalf("Failed to count REFERENCES: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count != int64(numTargets) {
			t.Errorf("Expected %d REFERENCES, got %d", numTargets, count)
		}
	}

	t.Logf("Large e-tag batch (%d tags) handled correctly", numTargets)
}
