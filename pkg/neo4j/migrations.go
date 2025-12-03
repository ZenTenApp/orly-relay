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
