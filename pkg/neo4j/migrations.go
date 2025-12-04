package neo4j

import (
	"context"
	"fmt"
)

// Migration represents a database migration with a version identifier
type Migration struct {
	Version     string
	Description string
	Migrate     func(ctx context.Context, n *N) error
}

// migrations is the ordered list of database migrations
// Migrations are applied in order and tracked via Marker nodes
var migrations = []Migration{
	{
		Version:     "v1",
		Description: "Merge Author nodes into NostrUser nodes",
		Migrate:     migrateAuthorToNostrUser,
	},
	{
		Version:     "v2",
		Description: "Clean up binary-encoded pubkeys and event IDs to lowercase hex",
		Migrate:     migrateBinaryToHex,
	},
}

// RunMigrations executes all pending migrations
func (n *N) RunMigrations() {
	ctx := context.Background()

	for _, migration := range migrations {
		// Check if migration has already been applied
		if n.migrationApplied(ctx, migration.Version) {
			n.Logger.Infof("migration %s already applied, skipping", migration.Version)
			continue
		}

		n.Logger.Infof("applying migration %s: %s", migration.Version, migration.Description)

		if err := migration.Migrate(ctx, n); err != nil {
			n.Logger.Errorf("migration %s failed: %v", migration.Version, err)
			// Continue to next migration - don't fail startup
			continue
		}

		// Mark migration as complete
		if err := n.markMigrationComplete(ctx, migration.Version, migration.Description); err != nil {
			n.Logger.Warningf("failed to mark migration %s as complete: %v", migration.Version, err)
		}

		n.Logger.Infof("migration %s completed successfully", migration.Version)
	}
}

// migrationApplied checks if a migration has already been applied
func (n *N) migrationApplied(ctx context.Context, version string) bool {
	cypher := `
		MATCH (m:Migration {version: $version})
		RETURN m.version
	`
	result, err := n.ExecuteRead(ctx, cypher, map[string]any{"version": version})
	if err != nil {
		return false
	}
	return result.Next(ctx)
}

// markMigrationComplete marks a migration as applied
func (n *N) markMigrationComplete(ctx context.Context, version, description string) error {
	cypher := `
		CREATE (m:Migration {
			version: $version,
			description: $description,
			applied_at: timestamp()
		})
	`
	_, err := n.ExecuteWrite(ctx, cypher, map[string]any{
		"version":     version,
		"description": description,
	})
	return err
}

// migrateAuthorToNostrUser migrates Author nodes to NostrUser nodes
// This consolidates the separate Author (NIP-01) and NostrUser (WoT) labels
// into a unified NostrUser label for the social graph
func migrateAuthorToNostrUser(ctx context.Context, n *N) error {
	// Step 1: Check if there are any Author nodes to migrate
	countCypher := `MATCH (a:Author) RETURN count(a) AS count`
	countResult, err := n.ExecuteRead(ctx, countCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count Author nodes: %w", err)
	}

	var authorCount int64
	if countResult.Next(ctx) {
		record := countResult.Record()
		if count, ok := record.Values[0].(int64); ok {
			authorCount = count
		}
	}

	if authorCount == 0 {
		n.Logger.Infof("no Author nodes to migrate")
		return nil
	}

	n.Logger.Infof("migrating %d Author nodes to NostrUser", authorCount)

	// Step 2: For each Author node, merge into NostrUser with same pubkey
	// This uses MERGE to either match existing NostrUser or create new one
	// Then copies any relationships from Author to NostrUser
	mergeCypher := `
		// Match all Author nodes
		MATCH (a:Author)

		// For each Author, merge into NostrUser (creates if doesn't exist)
		MERGE (u:NostrUser {pubkey: a.pubkey})
		ON CREATE SET u.created_at = timestamp(), u.migrated_from_author = true

		// Return count for logging
		RETURN count(DISTINCT a) AS migrated
	`

	result, err := n.ExecuteWrite(ctx, mergeCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to merge Author nodes to NostrUser: %w", err)
	}

	// Log result (result consumption happens within the session)
	_ = result

	// Step 3: Migrate AUTHORED_BY relationships from Author to NostrUser
	// Events should now point to NostrUser instead of Author
	relationshipCypher := `
		// Find events linked to Author via AUTHORED_BY
		MATCH (e:Event)-[r:AUTHORED_BY]->(a:Author)

		// Get or create the corresponding NostrUser
		MATCH (u:NostrUser {pubkey: a.pubkey})

		// Create new relationship to NostrUser if it doesn't exist
		MERGE (e)-[:AUTHORED_BY]->(u)

		// Delete old relationship to Author
		DELETE r

		RETURN count(r) AS migrated_relationships
	`

	_, err = n.ExecuteWrite(ctx, relationshipCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to migrate AUTHORED_BY relationships: %w", err)
	}

	// Step 4: Migrate MENTIONS relationships from Author to NostrUser
	mentionsCypher := `
		// Find events with MENTIONS to Author
		MATCH (e:Event)-[r:MENTIONS]->(a:Author)

		// Get or create the corresponding NostrUser
		MATCH (u:NostrUser {pubkey: a.pubkey})

		// Create new relationship to NostrUser if it doesn't exist
		MERGE (e)-[:MENTIONS]->(u)

		// Delete old relationship to Author
		DELETE r

		RETURN count(r) AS migrated_mentions
	`

	_, err = n.ExecuteWrite(ctx, mentionsCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to migrate MENTIONS relationships: %w", err)
	}

	// Step 5: Delete orphaned Author nodes (no longer needed)
	deleteCypher := `
		// Find Author nodes with no remaining relationships
		MATCH (a:Author)
		WHERE NOT (a)<-[:AUTHORED_BY]-() AND NOT (a)<-[:MENTIONS]-()
		DETACH DELETE a
		RETURN count(a) AS deleted
	`

	_, err = n.ExecuteWrite(ctx, deleteCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to delete orphaned Author nodes: %w", err)
	}

	// Step 6: Drop the old Author constraint if it exists
	dropConstraintCypher := `DROP CONSTRAINT author_pubkey_unique IF EXISTS`
	_, _ = n.ExecuteWrite(ctx, dropConstraintCypher, nil)
	// Ignore error as constraint may not exist

	n.Logger.Infof("completed Author to NostrUser migration")
	return nil
}

// migrateBinaryToHex cleans up any binary-encoded pubkeys and event IDs
// The nostr library stores e/p tag values in binary format (33 bytes with null terminator),
// but Neo4j should store them as lowercase hex strings for consistent querying.
// This migration:
// 1. Finds NostrUser nodes with invalid (non-hex) pubkeys and deletes them
// 2. Finds Event nodes with invalid pubkeys/IDs and deletes them
// 3. Finds Tag nodes (type 'e' or 'p') with invalid values and deletes them
// 4. Cleans up MENTIONS relationships pointing to invalid NostrUser nodes
func migrateBinaryToHex(ctx context.Context, n *N) error {
	// Step 1: Count problematic nodes before cleanup
	n.Logger.Infof("scanning for binary-encoded values in Neo4j...")

	// Check for NostrUser nodes with invalid pubkeys (not 64 char hex)
	// A valid hex pubkey is exactly 64 lowercase hex characters
	countInvalidUsersCypher := `
		MATCH (u:NostrUser)
		WHERE size(u.pubkey) <> 64
		   OR NOT u.pubkey =~ '^[0-9a-f]{64}$'
		RETURN count(u) AS count
	`
	result, err := n.ExecuteRead(ctx, countInvalidUsersCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count invalid NostrUser nodes: %w", err)
	}

	var invalidUserCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			invalidUserCount = count
		}
	}
	n.Logger.Infof("found %d NostrUser nodes with invalid pubkeys", invalidUserCount)

	// Check for Event nodes with invalid pubkeys or IDs
	countInvalidEventsCypher := `
		MATCH (e:Event)
		WHERE (size(e.pubkey) <> 64 OR NOT e.pubkey =~ '^[0-9a-f]{64}$')
		   OR (size(e.id) <> 64 OR NOT e.id =~ '^[0-9a-f]{64}$')
		RETURN count(e) AS count
	`
	result, err = n.ExecuteRead(ctx, countInvalidEventsCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count invalid Event nodes: %w", err)
	}

	var invalidEventCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			invalidEventCount = count
		}
	}
	n.Logger.Infof("found %d Event nodes with invalid pubkeys or IDs", invalidEventCount)

	// Check for Tag nodes (e/p type) with invalid values
	countInvalidTagsCypher := `
		MATCH (t:Tag)
		WHERE t.type IN ['e', 'p']
		  AND (size(t.value) <> 64 OR NOT t.value =~ '^[0-9a-f]{64}$')
		RETURN count(t) AS count
	`
	result, err = n.ExecuteRead(ctx, countInvalidTagsCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count invalid Tag nodes: %w", err)
	}

	var invalidTagCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			invalidTagCount = count
		}
	}
	n.Logger.Infof("found %d Tag nodes (e/p type) with invalid values", invalidTagCount)

	// If nothing to clean up, we're done
	if invalidUserCount == 0 && invalidEventCount == 0 && invalidTagCount == 0 {
		n.Logger.Infof("no binary-encoded values found, migration complete")
		return nil
	}

	// Step 2: Delete invalid NostrUser nodes and their relationships
	if invalidUserCount > 0 {
		n.Logger.Infof("deleting %d invalid NostrUser nodes...", invalidUserCount)
		deleteInvalidUsersCypher := `
			MATCH (u:NostrUser)
			WHERE size(u.pubkey) <> 64
			   OR NOT u.pubkey =~ '^[0-9a-f]{64}$'
			DETACH DELETE u
		`
		_, err = n.ExecuteWrite(ctx, deleteInvalidUsersCypher, nil)
		if err != nil {
			return fmt.Errorf("failed to delete invalid NostrUser nodes: %w", err)
		}
		n.Logger.Infof("deleted %d invalid NostrUser nodes", invalidUserCount)
	}

	// Step 3: Delete invalid Event nodes and their relationships
	if invalidEventCount > 0 {
		n.Logger.Infof("deleting %d invalid Event nodes...", invalidEventCount)
		deleteInvalidEventsCypher := `
			MATCH (e:Event)
			WHERE (size(e.pubkey) <> 64 OR NOT e.pubkey =~ '^[0-9a-f]{64}$')
			   OR (size(e.id) <> 64 OR NOT e.id =~ '^[0-9a-f]{64}$')
			DETACH DELETE e
		`
		_, err = n.ExecuteWrite(ctx, deleteInvalidEventsCypher, nil)
		if err != nil {
			return fmt.Errorf("failed to delete invalid Event nodes: %w", err)
		}
		n.Logger.Infof("deleted %d invalid Event nodes", invalidEventCount)
	}

	// Step 4: Delete invalid Tag nodes (e/p type) and their relationships
	if invalidTagCount > 0 {
		n.Logger.Infof("deleting %d invalid Tag nodes...", invalidTagCount)
		deleteInvalidTagsCypher := `
			MATCH (t:Tag)
			WHERE t.type IN ['e', 'p']
			  AND (size(t.value) <> 64 OR NOT t.value =~ '^[0-9a-f]{64}$')
			DETACH DELETE t
		`
		_, err = n.ExecuteWrite(ctx, deleteInvalidTagsCypher, nil)
		if err != nil {
			return fmt.Errorf("failed to delete invalid Tag nodes: %w", err)
		}
		n.Logger.Infof("deleted %d invalid Tag nodes", invalidTagCount)
	}

	// Step 5: Clean up any orphaned MENTIONS/REFERENCES relationships
	// These would be relationships pointing to nodes we just deleted
	cleanupOrphanedCypher := `
		// Clean up any ProcessedSocialEvent nodes with invalid pubkeys
		MATCH (p:ProcessedSocialEvent)
		WHERE size(p.pubkey) <> 64
		   OR NOT p.pubkey =~ '^[0-9a-f]{64}$'
		DETACH DELETE p
	`
	_, _ = n.ExecuteWrite(ctx, cleanupOrphanedCypher, nil)
	// Ignore errors - best effort cleanup

	n.Logger.Infof("binary-to-hex migration completed successfully")
	return nil
}
