package neo4j

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// TestCypherQueryGeneration_WithClause is a unit test that validates the WITH clause fix
// without requiring a Neo4j instance. This test verifies the generated Cypher string
// has correct syntax for different tag combinations.
func TestCypherQueryGeneration_WithClause(t *testing.T) {
	// Create a mock N struct - we only need it to call buildEventCreationCypher
	// No actual Neo4j connection is needed for this unit test
	n := &N{}

	// Generate test keypair
	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	tests := []struct {
		name               string
		tags               *tag.S
		expectWithClause   bool
		expectOptionalMatch bool
		description        string
	}{
		{
			name:               "NoTags",
			tags:               nil,
			expectWithClause:   false,
			expectOptionalMatch: false,
			description:        "Event without tags",
		},
		{
			name: "OnlyPTags_NoWithNeeded",
			tags: tag.NewS(
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000002"),
			),
			expectWithClause:   false,
			expectOptionalMatch: false,
			description:        "p-tags use MERGE (not OPTIONAL MATCH), no WITH needed",
		},
		{
			name: "OnlyETags_WithRequired",
			tags: tag.NewS(
				tag.NewFromAny("e", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				tag.NewFromAny("e", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			),
			expectWithClause:   true,
			expectOptionalMatch: true,
			description:        "e-tags use OPTIONAL MATCH which requires WITH clause after CREATE",
		},
		{
			name: "ETagBeforePTag",
			tags: tag.NewS(
				tag.NewFromAny("e", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000003"),
			),
			expectWithClause:   true,
			expectOptionalMatch: true,
			description:        "e-tag appearing first triggers WITH clause",
		},
		{
			name: "PTagBeforeETag",
			tags: tag.NewS(
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000004"),
				tag.NewFromAny("e", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
			),
			expectWithClause:   true,
			expectOptionalMatch: true,
			description:        "WITH clause needed even when p-tag comes before e-tag",
		},
		{
			name: "GenericTagsBeforeETag",
			tags: tag.NewS(
				tag.NewFromAny("t", "nostr"),
				tag.NewFromAny("r", "https://example.com"),
				tag.NewFromAny("e", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
			),
			expectWithClause:   true,
			expectOptionalMatch: true,
			description:        "WITH clause needed when e-tag follows generic tags",
		},
		{
			name: "OnlyGenericTags",
			tags: tag.NewS(
				tag.NewFromAny("t", "bitcoin"),
				tag.NewFromAny("d", "identifier"),
				tag.NewFromAny("r", "wss://relay.example.com"),
			),
			expectWithClause:   false,
			expectOptionalMatch: false,
			description:        "Generic tags use MERGE, no WITH needed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test event
			ev := event.New()
			ev.Pubkey = signer.Pub()
			ev.CreatedAt = timestamp.Now().V
			ev.Kind = 1
			ev.Content = []byte(fmt.Sprintf("Test content for %s", tt.name))
			ev.Tags = tt.tags

			if err := ev.Sign(signer); err != nil {
				t.Fatalf("Failed to sign event: %v", err)
			}

			// Generate Cypher query
			cypher, params := n.buildEventCreationCypher(ev, 12345)

			// Validate WITH clause presence
			hasWithClause := strings.Contains(cypher, "WITH e, a")
			if tt.expectWithClause && !hasWithClause {
				t.Errorf("%s: expected WITH clause but none found in Cypher:\n%s", tt.description, cypher)
			}
			if !tt.expectWithClause && hasWithClause {
				t.Errorf("%s: unexpected WITH clause in Cypher:\n%s", tt.description, cypher)
			}

			// Validate OPTIONAL MATCH presence
			hasOptionalMatch := strings.Contains(cypher, "OPTIONAL MATCH")
			if tt.expectOptionalMatch && !hasOptionalMatch {
				t.Errorf("%s: expected OPTIONAL MATCH but none found", tt.description)
			}
			if !tt.expectOptionalMatch && hasOptionalMatch {
				t.Errorf("%s: unexpected OPTIONAL MATCH found", tt.description)
			}

			// Validate WITH clause comes BEFORE first OPTIONAL MATCH (if both present)
			if hasWithClause && hasOptionalMatch {
				withIndex := strings.Index(cypher, "WITH e, a")
				optionalIndex := strings.Index(cypher, "OPTIONAL MATCH")
				if withIndex > optionalIndex {
					t.Errorf("%s: WITH clause must come BEFORE OPTIONAL MATCH.\nWITH at %d, OPTIONAL MATCH at %d\nCypher:\n%s",
						tt.description, withIndex, optionalIndex, cypher)
				}
			}

			// Validate parameters are set
			if params == nil {
				t.Error("params should not be nil")
			}

			// Validate basic required params exist
			if _, ok := params["eventId"]; !ok {
				t.Error("params should contain eventId")
			}
			if _, ok := params["serial"]; !ok {
				t.Error("params should contain serial")
			}

			t.Logf("✓ %s: WITH=%v, OPTIONAL_MATCH=%v", tt.name, hasWithClause, hasOptionalMatch)
		})
	}
}

// TestCypherQueryGeneration_MultipleETags verifies WITH clause is added exactly once
// even with multiple e-tags.
func TestCypherQueryGeneration_MultipleETags(t *testing.T) {
	n := &N{}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Create event with many e-tags
	manyETags := tag.NewS()
	for i := 0; i < 10; i++ {
		manyETags.Append(tag.NewFromAny("e", fmt.Sprintf("%064x", i)))
	}

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Event with many e-tags")
	ev.Tags = manyETags

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	cypher, _ := n.buildEventCreationCypher(ev, 1)

	// Count WITH clauses - should be exactly 1
	withCount := strings.Count(cypher, "WITH e, a")
	if withCount != 1 {
		t.Errorf("Expected exactly 1 WITH clause, found %d\nCypher:\n%s", withCount, cypher)
	}

	// Count OPTIONAL MATCH - should match number of e-tags
	optionalMatchCount := strings.Count(cypher, "OPTIONAL MATCH")
	if optionalMatchCount != 10 {
		t.Errorf("Expected 10 OPTIONAL MATCH statements (one per e-tag), found %d", optionalMatchCount)
	}

	// Count FOREACH (which wraps the conditional relationship creation)
	foreachCount := strings.Count(cypher, "FOREACH")
	if foreachCount != 10 {
		t.Errorf("Expected 10 FOREACH blocks, found %d", foreachCount)
	}

	t.Logf("✓ WITH clause added once, followed by %d OPTIONAL MATCH + FOREACH pairs", optionalMatchCount)
}

// TestCypherQueryGeneration_CriticalBugScenario reproduces the exact bug scenario
// that was fixed: CREATE followed by OPTIONAL MATCH without WITH clause.
func TestCypherQueryGeneration_CriticalBugScenario(t *testing.T) {
	n := &N{}

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// This is the exact scenario that caused the bug:
	// An event with just one e-tag should have:
	// 1. CREATE clause for the event
	// 2. WITH clause to carry forward variables
	// 3. OPTIONAL MATCH for the referenced event
	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Reply to an event")
	ev.Tags = tag.NewS(
		tag.NewFromAny("e", "1234567890123456789012345678901234567890123456789012345678901234"),
	)

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	cypher, _ := n.buildEventCreationCypher(ev, 1)

	// The critical validation: WITH must appear between CREATE and OPTIONAL MATCH
	createIndex := strings.Index(cypher, "CREATE (e)-[:AUTHORED_BY]->(a)")
	withIndex := strings.Index(cypher, "WITH e, a")
	optionalMatchIndex := strings.Index(cypher, "OPTIONAL MATCH")

	if createIndex == -1 {
		t.Fatal("CREATE clause not found in Cypher")
	}
	if withIndex == -1 {
		t.Fatal("WITH clause not found in Cypher - THIS IS THE BUG!")
	}
	if optionalMatchIndex == -1 {
		t.Fatal("OPTIONAL MATCH not found in Cypher")
	}

	// Validate order: CREATE < WITH < OPTIONAL MATCH
	if !(createIndex < withIndex && withIndex < optionalMatchIndex) {
		t.Errorf("Invalid clause ordering. Expected: CREATE (%d) < WITH (%d) < OPTIONAL MATCH (%d)\nCypher:\n%s",
			createIndex, withIndex, optionalMatchIndex, cypher)
	}

	t.Log("✓ Critical bug scenario validated: WITH clause correctly placed between CREATE and OPTIONAL MATCH")
}

// TestBuildEventCreationCypher_WithClause validates the WITH clause fix for Cypher queries.
// The bug was that OPTIONAL MATCH cannot directly follow CREATE in Cypher - a WITH clause
// is required to carry forward bound variables (e, a) from the CREATE to the MATCH.
func TestBuildEventCreationCypher_WithClause(t *testing.T) {
	// Skip if Neo4j is not available
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	// Create test database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Wait for database to be ready
	<-db.Ready()

	// Wipe database to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Generate test keypair
	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Test cases for different tag combinations
	tests := []struct {
		name        string
		tags        *tag.S
		wantWithClause bool
		description string
	}{
		{
			name:        "NoTags",
			tags:        nil,
			wantWithClause: false,
			description: "Event without tags should not have WITH clause",
		},
		{
			name: "OnlyPTags",
			tags: tag.NewS(
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000002"),
			),
			wantWithClause: false,
			description: "Event with only p-tags (MERGE) should not have WITH clause",
		},
		{
			name: "OnlyETags",
			tags: tag.NewS(
				tag.NewFromAny("e", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				tag.NewFromAny("e", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			),
			wantWithClause: true,
			description: "Event with e-tags (OPTIONAL MATCH) MUST have WITH clause",
		},
		{
			name: "ETagFirst",
			tags: tag.NewS(
				tag.NewFromAny("e", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000003"),
			),
			wantWithClause: true,
			description: "Event with e-tag first MUST have WITH clause before OPTIONAL MATCH",
		},
		{
			name: "PTagFirst",
			tags: tag.NewS(
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000004"),
				tag.NewFromAny("e", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
			),
			wantWithClause: true,
			description: "Event with p-tag first still needs WITH clause before e-tag's OPTIONAL MATCH",
		},
		{
			name: "MixedTags",
			tags: tag.NewS(
				tag.NewFromAny("t", "nostr"),
				tag.NewFromAny("e", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
				tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000005"),
				tag.NewFromAny("r", "https://example.com"),
			),
			wantWithClause: true,
			description: "Mixed tags with e-tag requires WITH clause",
		},
		{
			name: "OnlyGenericTags",
			tags: tag.NewS(
				tag.NewFromAny("t", "bitcoin"),
				tag.NewFromAny("r", "wss://relay.example.com"),
				tag.NewFromAny("d", "identifier"),
			),
			wantWithClause: false,
			description: "Generic tags (MERGE) don't require WITH clause",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create event
			ev := event.New()
			ev.Pubkey = signer.Pub()
			ev.CreatedAt = timestamp.Now().V
			ev.Kind = 1
			ev.Content = []byte(fmt.Sprintf("Test content for %s", tt.name))
			ev.Tags = tt.tags

			if err := ev.Sign(signer); err != nil {
				t.Fatalf("Failed to sign event: %v", err)
			}

			// Build Cypher query
			cypher, params := db.buildEventCreationCypher(ev, 1)

			// Check if WITH clause is present
			hasWithClause := strings.Contains(cypher, "WITH e, a")

			if tt.wantWithClause && !hasWithClause {
				t.Errorf("%s: expected WITH clause but none found.\nCypher:\n%s", tt.description, cypher)
			}
			if !tt.wantWithClause && hasWithClause {
				t.Errorf("%s: unexpected WITH clause found.\nCypher:\n%s", tt.description, cypher)
			}

			// Verify Cypher syntax by executing it against Neo4j
			// This is the key test - invalid Cypher will fail here
			_, err := db.ExecuteWrite(ctx, cypher, params)
			if err != nil {
				t.Errorf("%s: Cypher query failed (invalid syntax): %v\nCypher:\n%s", tt.description, err, cypher)
			}
		})
	}
}

// TestSaveEvent_ETagReference tests that events with e-tags are saved correctly
// and the REFERENCES relationships are created when the referenced event exists.
func TestSaveEvent_ETagReference(t *testing.T) {
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
	exists, err := db.SaveEvent(ctx, rootEvent)
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

	// Save reply event - this exercises the WITH clause fix
	exists, err = db.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}
	if exists {
		t.Fatal("Reply event should not exist yet")
	}

	// Verify REFERENCES relationship was created
	cypher := `
		MATCH (reply:Event {id: $replyId})-[:REFERENCES]->(root:Event {id: $rootId})
		RETURN reply.id AS replyId, root.id AS rootId
	`
	params := map[string]any{
		"replyId": hex.Enc(replyEvent.ID[:]),
		"rootId":  rootEventID,
	}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query REFERENCES relationship: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Expected REFERENCES relationship between reply and root events")
	} else {
		record := result.Record()
		returnedReplyId := record.Values[0].(string)
		returnedRootId := record.Values[1].(string)
		t.Logf("✓ REFERENCES relationship verified: %s -> %s", returnedReplyId[:8], returnedRootId[:8])
	}

	// Verify MENTIONS relationship was also created for the p-tag
	mentionsCypher := `
		MATCH (reply:Event {id: $replyId})-[:MENTIONS]->(author:Author {pubkey: $authorPubkey})
		RETURN author.pubkey AS pubkey
	`
	mentionsParams := map[string]any{
		"replyId":      hex.Enc(replyEvent.ID[:]),
		"authorPubkey": hex.Enc(alice.Pub()),
	}

	mentionsResult, err := db.ExecuteRead(ctx, mentionsCypher, mentionsParams)
	if err != nil {
		t.Fatalf("Failed to query MENTIONS relationship: %v", err)
	}

	if !mentionsResult.Next(ctx) {
		t.Error("Expected MENTIONS relationship for p-tag")
	} else {
		t.Logf("✓ MENTIONS relationship verified")
	}
}

// TestSaveEvent_ETagMissingReference tests that e-tags to non-existent events
// don't create broken relationships (OPTIONAL MATCH handles this gracefully).
func TestSaveEvent_ETagMissingReference(t *testing.T) {
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

	// Save should succeed (OPTIONAL MATCH handles missing reference)
	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event with missing reference: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify event was saved
	checkCypher := "MATCH (e:Event {id: $id}) RETURN e.id AS id"
	checkParams := map[string]any{"id": hex.Enc(ev.ID[:])}

	result, err := db.ExecuteRead(ctx, checkCypher, checkParams)
	if err != nil {
		t.Fatalf("Failed to check event: %v", err)
	}

	if !result.Next(ctx) {
		t.Error("Event should have been saved despite missing reference")
	}

	// Verify no REFERENCES relationship was created (as the target doesn't exist)
	refCypher := `
		MATCH (e:Event {id: $eventId})-[:REFERENCES]->(ref:Event)
		RETURN count(ref) AS refCount
	`
	refParams := map[string]any{"eventId": hex.Enc(ev.ID[:])}

	refResult, err := db.ExecuteRead(ctx, refCypher, refParams)
	if err != nil {
		t.Fatalf("Failed to check references: %v", err)
	}

	if refResult.Next(ctx) {
		count := refResult.Record().Values[0].(int64)
		if count > 0 {
			t.Errorf("Expected no REFERENCES relationship for non-existent event, got %d", count)
		} else {
			t.Logf("✓ Correctly handled missing reference (no relationship created)")
		}
	}
}

// TestSaveEvent_MultipleETags tests events with multiple e-tags.
func TestSaveEvent_MultipleETags(t *testing.T) {
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

		if _, err := db.SaveEvent(ctx, ev); err != nil {
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

	// Save reply event - tests multiple OPTIONAL MATCH statements after WITH
	exists, err := db.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save multi-reference event: %v", err)
	}
	if exists {
		t.Fatal("Reply event should not exist yet")
	}

	// Verify all REFERENCES relationships were created
	cypher := `
		MATCH (reply:Event {id: $replyId})-[:REFERENCES]->(ref:Event)
		RETURN ref.id AS refId
	`
	params := map[string]any{"replyId": hex.Enc(replyEvent.ID[:])}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query REFERENCES relationships: %v", err)
	}

	referencedIDs := make(map[string]bool)
	for result.Next(ctx) {
		refID := result.Record().Values[0].(string)
		referencedIDs[refID] = true
	}

	if len(referencedIDs) != 3 {
		t.Errorf("Expected 3 REFERENCES relationships, got %d", len(referencedIDs))
	}

	for i, id := range eventIDs {
		if !referencedIDs[id] {
			t.Errorf("Missing REFERENCES relationship to event %d (%s)", i, id[:8])
		}
	}

	t.Logf("✓ All %d REFERENCES relationships created successfully", len(referencedIDs))
}

// TestBuildEventCreationCypher_CypherSyntaxValidation validates the generated Cypher
// is syntactically correct for all edge cases.
func TestBuildEventCreationCypher_CypherSyntaxValidation(t *testing.T) {
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

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Test many e-tags to ensure WITH clause is added only once
	manyETags := tag.NewS()
	for i := 0; i < 10; i++ {
		manyETags.Append(tag.NewFromAny("e", fmt.Sprintf("%064x", i)))
	}

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 1
	ev.Content = []byte("Event with many e-tags")
	ev.Tags = manyETags

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	cypher, _ := db.buildEventCreationCypher(ev, 1)

	// Count occurrences of WITH clause - should be exactly 1
	withCount := strings.Count(cypher, "WITH e, a")
	if withCount != 1 {
		t.Errorf("Expected exactly 1 WITH clause, found %d\nCypher:\n%s", withCount, cypher)
	}

	// Count OPTIONAL MATCH statements - should equal number of e-tags
	optionalMatchCount := strings.Count(cypher, "OPTIONAL MATCH")
	if optionalMatchCount != 10 {
		t.Errorf("Expected 10 OPTIONAL MATCH statements, found %d", optionalMatchCount)
	}

	t.Logf("✓ WITH clause correctly added once, followed by %d OPTIONAL MATCH statements", optionalMatchCount)
}