package neo4j

import (
	"context"
	"os"
	"testing"
	"time"

	"next.orly.dev/pkg/database"
)

// testDB is the shared database instance for tests
var testDB *N

// TestMain sets up and tears down the test database
func TestMain(m *testing.M) {
	// Skip integration tests if NEO4J_TEST_URI is not set
	neo4jURI := os.Getenv("NEO4J_TEST_URI")
	if neo4jURI == "" {
		neo4jURI = "bolt://localhost:7687"
	}
	neo4jUser := os.Getenv("NEO4J_TEST_USER")
	if neo4jUser == "" {
		neo4jUser = "neo4j"
	}
	neo4jPassword := os.Getenv("NEO4J_TEST_PASSWORD")
	if neo4jPassword == "" {
		neo4jPassword = "testpassword"
	}

	// Try to connect to Neo4j
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &database.DatabaseConfig{
		DataDir:       os.TempDir(),
		Neo4jURI:      neo4jURI,
		Neo4jUser:     neo4jUser,
		Neo4jPassword: neo4jPassword,
	}

	var err error
	testDB, err = NewWithConfig(ctx, cancel, cfg)
	if err != nil {
		// If Neo4j is not available, skip integration tests
		os.Stderr.WriteString("Neo4j not available, skipping integration tests: " + err.Error() + "\n")
		os.Stderr.WriteString("Start Neo4j with: docker compose -f pkg/neo4j/docker-compose.yaml up -d\n")
		os.Exit(0)
	}

	// Wait for database to be ready
	select {
	case <-testDB.Ready():
		// Database is ready
	case <-time.After(30 * time.Second):
		os.Stderr.WriteString("Timeout waiting for Neo4j to be ready\n")
		os.Exit(1)
	}

	// Clean database before running tests
	cleanTestDatabase()

	// Run tests
	code := m.Run()

	// Clean up
	cleanTestDatabase()
	testDB.Close()
	cancel()

	os.Exit(code)
}

// cleanTestDatabase removes all nodes and relationships, then re-initializes
func cleanTestDatabase() {
	ctx := context.Background()
	// Delete all nodes and relationships
	_, _ = testDB.ExecuteWrite(ctx, "MATCH (n) DETACH DELETE n", nil)
	// Re-apply schema (constraints and indexes)
	_ = testDB.applySchema(ctx)
	// Re-initialize serial counter
	_ = testDB.initSerialCounter()
}

// setupTestEvent creates a test event directly in Neo4j for testing queries
func setupTestEvent(t *testing.T, eventID, pubkey string, kind int64, tags string) {
	t.Helper()
	ctx := context.Background()

	cypher := `
		MERGE (a:NostrUser {pubkey: $pubkey})
		CREATE (e:Event {
			id: $eventId,
			serial: $serial,
			kind: $kind,
			created_at: $createdAt,
			content: $content,
			sig: $sig,
			pubkey: $pubkey,
			tags: $tags,
			expiration: 0
		})
		CREATE (e)-[:AUTHORED_BY]->(a)
	`

	params := map[string]any{
		"eventId":   eventID,
		"serial":    time.Now().UnixNano(),
		"kind":      kind,
		"createdAt": time.Now().Unix(),
		"content":   "test content",
		"sig": "0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000",
		"pubkey": pubkey,
		"tags":   tags,
	}

	_, err := testDB.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to setup test event: %v", err)
	}
}

// setupInvalidNostrUser creates a NostrUser with an invalid (binary) pubkey for testing migrations
func setupInvalidNostrUser(t *testing.T, invalidPubkey string) {
	t.Helper()
	ctx := context.Background()

	cypher := `CREATE (u:NostrUser {pubkey: $pubkey, created_at: timestamp()})`
	params := map[string]any{"pubkey": invalidPubkey}

	_, err := testDB.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to setup invalid NostrUser: %v", err)
	}
}

// setupInvalidEvent creates an Event with an invalid pubkey/ID for testing migrations
func setupInvalidEvent(t *testing.T, invalidID, invalidPubkey string) {
	t.Helper()
	ctx := context.Background()

	cypher := `
		CREATE (e:Event {
			id: $id,
			pubkey: $pubkey,
			kind: 1,
			created_at: timestamp(),
			content: 'test',
			sig: 'invalid',
			tags: '[]',
			serial: $serial,
			expiration: 0
		})
	`
	params := map[string]any{
		"id":     invalidID,
		"pubkey": invalidPubkey,
		"serial": time.Now().UnixNano(),
	}

	_, err := testDB.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to setup invalid Event: %v", err)
	}
}

// setupInvalidTag creates a Tag node with invalid value for testing migrations
func setupInvalidTag(t *testing.T, tagType string, invalidValue string) {
	t.Helper()
	ctx := context.Background()

	cypher := `CREATE (tag:Tag {type: $type, value: $value})`
	params := map[string]any{
		"type":  tagType,
		"value": invalidValue,
	}

	_, err := testDB.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to setup invalid Tag: %v", err)
	}
}

// countNodes counts nodes with a given label
func countNodes(t *testing.T, label string) int64 {
	t.Helper()
	ctx := context.Background()

	cypher := "MATCH (n:" + label + ") RETURN count(n) AS count"
	result, err := testDB.ExecuteRead(ctx, cypher, nil)
	if err != nil {
		t.Fatalf("Failed to count nodes: %v", err)
	}

	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			return count
		}
	}
	return 0
}

// countInvalidNostrUsers counts NostrUser nodes with invalid pubkeys
func countInvalidNostrUsers(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()

	cypher := `
		MATCH (u:NostrUser)
		WHERE size(u.pubkey) <> 64
		   OR NOT u.pubkey =~ '^[0-9a-f]{64}$'
		RETURN count(u) AS count
	`
	result, err := testDB.ExecuteRead(ctx, cypher, nil)
	if err != nil {
		t.Fatalf("Failed to count invalid NostrUsers: %v", err)
	}

	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			return count
		}
	}
	return 0
}

// countInvalidTags counts Tag nodes (e/p type) with invalid values
func countInvalidTags(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()

	cypher := `
		MATCH (t:Tag)
		WHERE t.type IN ['e', 'p']
		  AND (size(t.value) <> 64 OR NOT t.value =~ '^[0-9a-f]{64}$')
		RETURN count(t) AS count
	`
	result, err := testDB.ExecuteRead(ctx, cypher, nil)
	if err != nil {
		t.Fatalf("Failed to count invalid Tags: %v", err)
	}

	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			return count
		}
	}
	return 0
}
