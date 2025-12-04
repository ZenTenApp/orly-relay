package neo4j

import (
	"context"
	"testing"
)

// TestMigrationV2_CleanupBinaryEncodedValues tests that migration v2 properly
// cleans up binary-encoded pubkeys and event IDs
func TestMigrationV2_CleanupBinaryEncodedValues(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Create some valid NostrUser nodes (should NOT be deleted)
	validPubkeys := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}
	for _, pk := range validPubkeys {
		setupInvalidNostrUser(t, pk) // Using setupInvalidNostrUser to create directly
	}

	// Create some invalid NostrUser nodes (should be deleted)
	invalidPubkeys := []string{
		"binary\x00garbage\x00data",                         // Binary garbage
		"ABCDEF",                                             // Too short
		"GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG", // Non-hex chars
		string(append(make([]byte, 32), 0)),                 // 33-byte binary format
	}
	for _, pk := range invalidPubkeys {
		setupInvalidNostrUser(t, pk)
	}

	// Verify invalid nodes exist before migration
	invalidCountBefore := countInvalidNostrUsers(t)
	if invalidCountBefore != 4 {
		t.Errorf("Expected 4 invalid NostrUsers before migration, got %d", invalidCountBefore)
	}

	totalBefore := countNodes(t, "NostrUser")
	if totalBefore != 6 {
		t.Errorf("Expected 6 total NostrUsers before migration, got %d", totalBefore)
	}

	// Run the migration
	err := migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify invalid nodes were deleted
	invalidCountAfter := countInvalidNostrUsers(t)
	if invalidCountAfter != 0 {
		t.Errorf("Expected 0 invalid NostrUsers after migration, got %d", invalidCountAfter)
	}

	// Verify valid nodes were NOT deleted
	totalAfter := countNodes(t, "NostrUser")
	if totalAfter != 2 {
		t.Errorf("Expected 2 valid NostrUsers after migration, got %d", totalAfter)
	}
}

// TestMigrationV2_CleanupInvalidEvents tests that migration v2 properly
// cleans up Event nodes with invalid pubkeys or IDs
func TestMigrationV2_CleanupInvalidEvents(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Create valid events
	validEventID := "1111111111111111111111111111111111111111111111111111111111111111"
	validPubkey := "0000000000000000000000000000000000000000000000000000000000000001"
	setupTestEvent(t, validEventID, validPubkey, 1, "[]")

	// Create invalid events directly
	setupInvalidEvent(t, "invalid_id", validPubkey)             // Invalid ID
	setupInvalidEvent(t, validEventID+"2", "invalid_pubkey")    // Invalid pubkey (different ID to avoid duplicate)
	setupInvalidEvent(t, "TOOSHORT", "binary\x00garbage")       // Both invalid

	// Count events before migration
	eventsBefore := countNodes(t, "Event")
	if eventsBefore != 4 {
		t.Errorf("Expected 4 Events before migration, got %d", eventsBefore)
	}

	// Run the migration
	err := migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify only valid event remains
	eventsAfter := countNodes(t, "Event")
	if eventsAfter != 1 {
		t.Errorf("Expected 1 valid Event after migration, got %d", eventsAfter)
	}
}

// TestMigrationV2_CleanupInvalidTags tests that migration v2 properly
// cleans up Tag nodes (e/p type) with invalid values
func TestMigrationV2_CleanupInvalidTags(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Create valid tags
	validHex := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	setupInvalidTag(t, "e", validHex) // Valid e-tag
	setupInvalidTag(t, "p", validHex) // Valid p-tag
	setupInvalidTag(t, "t", "topic")  // Non e/p tag (should not be affected)

	// Create invalid e/p tags
	setupInvalidTag(t, "e", "binary\x00garbage")   // Invalid e-tag
	setupInvalidTag(t, "p", "TOOSHORT")            // Invalid p-tag (too short)
	setupInvalidTag(t, "e", string(append(make([]byte, 32), 0))) // Binary encoded

	// Count tags before migration
	tagsBefore := countNodes(t, "Tag")
	if tagsBefore != 6 {
		t.Errorf("Expected 6 Tags before migration, got %d", tagsBefore)
	}

	invalidBefore := countInvalidTags(t)
	if invalidBefore != 3 {
		t.Errorf("Expected 3 invalid e/p Tags before migration, got %d", invalidBefore)
	}

	// Run the migration
	err := migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify invalid tags were deleted
	invalidAfter := countInvalidTags(t)
	if invalidAfter != 0 {
		t.Errorf("Expected 0 invalid e/p Tags after migration, got %d", invalidAfter)
	}

	// Verify valid tags remain (2 e/p valid + 1 t-tag)
	tagsAfter := countNodes(t, "Tag")
	if tagsAfter != 3 {
		t.Errorf("Expected 3 Tags after migration, got %d", tagsAfter)
	}
}

// TestMigrationV2_Idempotent tests that migration v2 can be run multiple times safely
func TestMigrationV2_Idempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Create only valid data
	validPubkey := "0000000000000000000000000000000000000000000000000000000000000001"
	validEventID := "1111111111111111111111111111111111111111111111111111111111111111"
	setupTestEvent(t, validEventID, validPubkey, 1, "[]")

	countBefore := countNodes(t, "Event")

	// Run migration first time
	err := migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("First migration run failed: %v", err)
	}

	countAfterFirst := countNodes(t, "Event")
	if countAfterFirst != countBefore {
		t.Errorf("First migration changed valid event count: before=%d, after=%d", countBefore, countAfterFirst)
	}

	// Run migration second time
	err = migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("Second migration run failed: %v", err)
	}

	countAfterSecond := countNodes(t, "Event")
	if countAfterSecond != countBefore {
		t.Errorf("Second migration changed valid event count: before=%d, after=%d", countBefore, countAfterSecond)
	}
}

// TestMigrationV2_NoDataDoesNotFail tests that migration v2 succeeds with empty database
func TestMigrationV2_NoDataDoesNotFail(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up completely
	cleanTestDatabase()

	ctx := context.Background()

	// Run migration on empty database - should not fail
	err := migrateBinaryToHex(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration on empty database failed: %v", err)
	}
}

// TestMigrationMarking tests that migrations are properly tracked
func TestMigrationMarking(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Verify migration v2 has not been applied
	if testDB.migrationApplied(ctx, "v2") {
		t.Error("Migration v2 should not be applied before test")
	}

	// Mark migration as complete
	err := testDB.markMigrationComplete(ctx, "v2", "Test migration")
	if err != nil {
		t.Fatalf("Failed to mark migration complete: %v", err)
	}

	// Verify migration is now marked as applied
	if !testDB.migrationApplied(ctx, "v2") {
		t.Error("Migration v2 should be applied after marking")
	}

	// Clean up
	cleanTestDatabase()
}

// TestMigrationV1_AuthorToNostrUserMerge tests the author migration
func TestMigrationV1_AuthorToNostrUserMerge(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	ctx := context.Background()

	// Create some Author nodes (legacy format)
	authorPubkeys := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
	}

	for _, pk := range authorPubkeys {
		cypher := `CREATE (a:Author {pubkey: $pubkey})`
		_, err := testDB.ExecuteWrite(ctx, cypher, map[string]any{"pubkey": pk})
		if err != nil {
			t.Fatalf("Failed to create Author node: %v", err)
		}
	}

	// Verify Author nodes exist
	authorCount := countNodes(t, "Author")
	if authorCount != 2 {
		t.Errorf("Expected 2 Author nodes, got %d", authorCount)
	}

	// Run migration
	err := migrateAuthorToNostrUser(ctx, testDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify NostrUser nodes were created
	nostrUserCount := countNodes(t, "NostrUser")
	if nostrUserCount != 2 {
		t.Errorf("Expected 2 NostrUser nodes after migration, got %d", nostrUserCount)
	}

	// Verify Author nodes were deleted (they should have no relationships after migration)
	authorCountAfter := countNodes(t, "Author")
	if authorCountAfter != 0 {
		t.Errorf("Expected 0 Author nodes after migration, got %d", authorCountAfter)
	}
}
