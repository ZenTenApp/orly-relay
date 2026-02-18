package neo4j

import (
	"context"
	"encoding/hex"
	"fmt"

	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database"
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
	{
		Version:     "v3",
		Description: "Convert direct REFERENCES/MENTIONS relationships to Tag-based model",
		Migrate:     migrateToTagBasedReferences,
	},
	{
		Version:     "v4",
		Description: "Deduplicate REPORTS relationships by (reporter, reported, report_type)",
		Migrate:     migrateDeduplicateReports,
	},
	{
		Version:     "v5",
		Description: "Add naddr property to addressable Event nodes (kinds 30000-39999)",
		Migrate:     migrateAddNaddr,
	},
	{
		Version:     "v6",
		Description: "Backfill Word nodes and HAS_WORD relationships for NIP-50 search",
		Migrate:     migrateBackfillWords,
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

// migrateToTagBasedReferences converts direct REFERENCES and MENTIONS relationships
// to the new Tag-based model where:
// - Event-[:REFERENCES]->Event becomes Event-[:TAGGED_WITH]->Tag-[:REFERENCES]->Event
// - Event-[:MENTIONS]->NostrUser becomes Event-[:TAGGED_WITH]->Tag-[:REFERENCES]->NostrUser
//
// This enables unified tag querying via #e and #p filters while maintaining graph traversal.
func migrateToTagBasedReferences(ctx context.Context, n *N) error {
	// Step 1: Count existing direct REFERENCES relationships (Event->Event)
	countRefCypher := `
		MATCH (source:Event)-[r:REFERENCES]->(target:Event)
		RETURN count(r) AS count
	`
	result, err := n.ExecuteRead(ctx, countRefCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count REFERENCES relationships: %w", err)
	}

	var refCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			refCount = count
		}
	}
	n.Logger.Infof("found %d direct Event-[:REFERENCES]->Event relationships to migrate", refCount)

	// Step 2: Count existing direct MENTIONS relationships (Event->NostrUser)
	countMentionsCypher := `
		MATCH (source:Event)-[r:MENTIONS]->(target:NostrUser)
		RETURN count(r) AS count
	`
	result, err = n.ExecuteRead(ctx, countMentionsCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count MENTIONS relationships: %w", err)
	}

	var mentionsCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			mentionsCount = count
		}
	}
	n.Logger.Infof("found %d direct Event-[:MENTIONS]->NostrUser relationships to migrate", mentionsCount)

	// If nothing to migrate, we're done
	if refCount == 0 && mentionsCount == 0 {
		n.Logger.Infof("no direct relationships to migrate, migration complete")
		return nil
	}

	// Step 3: Migrate REFERENCES relationships to Tag-based model
	// Process in batches to avoid memory issues with large datasets
	if refCount > 0 {
		n.Logger.Infof("migrating %d REFERENCES relationships to Tag-based model...", refCount)

		// This query:
		// 1. Finds Event->Event REFERENCES relationships
		// 2. Creates/merges Tag node with type='e' and value=target event ID
		// 3. Creates TAGGED_WITH from source Event to Tag
		// 4. Creates REFERENCES from Tag to target Event
		// 5. Deletes the old direct REFERENCES relationship
		migrateRefCypher := `
			MATCH (source:Event)-[r:REFERENCES]->(target:Event)
			WITH source, r, target LIMIT 1000
			MERGE (t:Tag {type: 'e', value: target.id})
			CREATE (source)-[:TAGGED_WITH]->(t)
			MERGE (t)-[:REFERENCES]->(target)
			DELETE r
			RETURN count(r) AS migrated
		`

		// Run migration in batches until no more relationships exist
		totalMigrated := int64(0)
		for {
			result, err := n.ExecuteWrite(ctx, migrateRefCypher, nil)
			if err != nil {
				return fmt.Errorf("failed to migrate REFERENCES batch: %w", err)
			}

			var batchMigrated int64
			if result.Next(ctx) {
				if count, ok := result.Record().Values[0].(int64); ok {
					batchMigrated = count
				}
			}

			if batchMigrated == 0 {
				break
			}
			totalMigrated += batchMigrated
			n.Logger.Infof("migrated %d REFERENCES relationships (total: %d)", batchMigrated, totalMigrated)
		}

		n.Logger.Infof("completed migrating %d REFERENCES relationships", totalMigrated)
	}

	// Step 4: Migrate MENTIONS relationships to Tag-based model
	if mentionsCount > 0 {
		n.Logger.Infof("migrating %d MENTIONS relationships to Tag-based model...", mentionsCount)

		// This query:
		// 1. Finds Event->NostrUser MENTIONS relationships
		// 2. Creates/merges Tag node with type='p' and value=target pubkey
		// 3. Creates TAGGED_WITH from source Event to Tag
		// 4. Creates REFERENCES from Tag to target NostrUser
		// 5. Deletes the old direct MENTIONS relationship
		migrateMentionsCypher := `
			MATCH (source:Event)-[r:MENTIONS]->(target:NostrUser)
			WITH source, r, target LIMIT 1000
			MERGE (t:Tag {type: 'p', value: target.pubkey})
			CREATE (source)-[:TAGGED_WITH]->(t)
			MERGE (t)-[:REFERENCES]->(target)
			DELETE r
			RETURN count(r) AS migrated
		`

		// Run migration in batches until no more relationships exist
		totalMigrated := int64(0)
		for {
			result, err := n.ExecuteWrite(ctx, migrateMentionsCypher, nil)
			if err != nil {
				return fmt.Errorf("failed to migrate MENTIONS batch: %w", err)
			}

			var batchMigrated int64
			if result.Next(ctx) {
				if count, ok := result.Record().Values[0].(int64); ok {
					batchMigrated = count
				}
			}

			if batchMigrated == 0 {
				break
			}
			totalMigrated += batchMigrated
			n.Logger.Infof("migrated %d MENTIONS relationships (total: %d)", batchMigrated, totalMigrated)
		}

		n.Logger.Infof("completed migrating %d MENTIONS relationships", totalMigrated)
	}

	n.Logger.Infof("Tag-based references migration completed successfully")
	return nil
}

// migrateDeduplicateReports removes duplicate REPORTS relationships
// Prior to this migration, processReport() used CREATE which allowed multiple
// REPORTS relationships with the same report_type between the same two users.
// This migration keeps only the most recent report (by created_at) for each
// (reporter, reported, report_type) combination.
func migrateDeduplicateReports(ctx context.Context, n *N) error {
	// Step 1: Count duplicate REPORTS relationships
	// Duplicates are defined as multiple REPORTS with the same (reporter, reported, report_type)
	countDuplicatesCypher := `
		MATCH (reporter:NostrUser)-[r:REPORTS]->(reported:NostrUser)
		WITH reporter, reported, r.report_type AS type, collect(r) AS rels
		WHERE size(rels) > 1
		RETURN sum(size(rels) - 1) AS duplicate_count
	`
	result, err := n.ExecuteRead(ctx, countDuplicatesCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count duplicate REPORTS: %w", err)
	}

	var duplicateCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			duplicateCount = count
		}
	}

	if duplicateCount == 0 {
		n.Logger.Infof("no duplicate REPORTS relationships found, migration complete")
		return nil
	}

	n.Logger.Infof("found %d duplicate REPORTS relationships to remove", duplicateCount)

	// Step 2: Delete duplicate REPORTS, keeping the one with the highest created_at
	// This query:
	// 1. Groups REPORTS by (reporter, reported, report_type)
	// 2. Finds the maximum created_at for each group
	// 3. Deletes all relationships in the group except the newest one
	deleteDuplicatesCypher := `
		MATCH (reporter:NostrUser)-[r:REPORTS]->(reported:NostrUser)
		WITH reporter, reported, r.report_type AS type,
		     collect(r) AS rels, max(r.created_at) AS maxCreatedAt
		WHERE size(rels) > 1
		UNWIND rels AS rel
		WITH rel, maxCreatedAt
		WHERE rel.created_at < maxCreatedAt
		DELETE rel
		RETURN count(*) AS deleted
	`

	writeResult, err := n.ExecuteWrite(ctx, deleteDuplicatesCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to delete duplicate REPORTS: %w", err)
	}

	var deletedCount int64
	if writeResult.Next(ctx) {
		if count, ok := writeResult.Record().Values[0].(int64); ok {
			deletedCount = count
		}
	}

	n.Logger.Infof("deleted %d duplicate REPORTS relationships", deletedCount)

	// Step 3: Mark superseded ProcessedSocialEvent nodes for deleted reports
	// Find ProcessedSocialEvent nodes (kind 1984) whose event IDs are no longer
	// referenced by any REPORTS relationship's created_by_event
	markSupersededCypher := `
		MATCH (evt:ProcessedSocialEvent {event_kind: 1984})
		WHERE evt.superseded_by IS NULL
		  AND NOT EXISTS {
		      MATCH ()-[r:REPORTS]->()
		      WHERE r.created_by_event = evt.event_id
		  }
		SET evt.superseded_by = 'migration_v4_dedupe'
		RETURN count(evt) AS superseded
	`

	markResult, err := n.ExecuteWrite(ctx, markSupersededCypher, nil)
	if err != nil {
		// Non-fatal - just log warning
		n.Logger.Warningf("failed to mark superseded ProcessedSocialEvent nodes: %v", err)
	} else {
		var supersededCount int64
		if markResult.Next(ctx) {
			if count, ok := markResult.Record().Values[0].(int64); ok {
				supersededCount = count
			}
		}
		if supersededCount > 0 {
			n.Logger.Infof("marked %d ProcessedSocialEvent nodes as superseded", supersededCount)
		}
	}

	n.Logger.Infof("REPORTS deduplication migration completed successfully")
	return nil
}

// migrateAddNaddr adds the naddr property to existing addressable Event nodes (kinds 30000-39999).
// The naddr format is: pubkey:kind:dtag (colon-delimited coordinate)
// This enables direct lookups and uniqueness constraints for parameterized replaceable events.
func migrateAddNaddr(ctx context.Context, n *N) error {
	// Step 1: Count addressable events without naddr
	countCypher := `
		MATCH (e:Event)
		WHERE e.kind >= 30000 AND e.kind < 40000
		  AND e.naddr IS NULL
		RETURN count(e) AS count
	`
	result, err := n.ExecuteRead(ctx, countCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count addressable events: %w", err)
	}

	var eventCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			eventCount = count
		}
	}

	if eventCount == 0 {
		n.Logger.Infof("no addressable events without naddr found, migration complete")
		return nil
	}

	n.Logger.Infof("found %d addressable events to update with naddr", eventCount)

	// Step 2: Update events in batches
	// For each event, compute naddr from pubkey + kind + d-tag
	// The d-tag value is obtained from the Tag node via TAGGED_WITH relationship
	updateCypher := `
		MATCH (e:Event)
		WHERE e.kind >= 30000 AND e.kind < 40000
		  AND e.naddr IS NULL
		WITH e LIMIT 1000

		// Get d-tag value via TAGGED_WITH relationship
		OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t:Tag {type: 'd'})
		WITH e, COALESCE(t.value, '') AS dValue

		// Build naddr: pubkey:kind:dValue
		SET e.naddr = e.pubkey + ':' + toString(e.kind) + ':' + dValue

		RETURN count(e) AS updated
	`

	// Run migration in batches until no more events to update
	totalUpdated := int64(0)
	for {
		writeResult, err := n.ExecuteWrite(ctx, updateCypher, nil)
		if err != nil {
			return fmt.Errorf("failed to update addressable events batch: %w", err)
		}

		var batchUpdated int64
		if writeResult.Next(ctx) {
			if count, ok := writeResult.Record().Values[0].(int64); ok {
				batchUpdated = count
			}
		}

		if batchUpdated == 0 {
			break
		}
		totalUpdated += batchUpdated
		n.Logger.Infof("updated %d addressable events with naddr (total: %d)", batchUpdated, totalUpdated)
	}

	n.Logger.Infof("naddr migration completed: updated %d addressable events", totalUpdated)
	return nil
}

// migrateBackfillWords creates Word nodes and HAS_WORD relationships for all
// existing Event nodes that don't have them yet. This enables NIP-50 word search
// for events saved before word indexing was added to SaveEvent.
func migrateBackfillWords(ctx context.Context, n *N) error {
	// Count events without word index
	countCypher := `
		MATCH (e:Event)
		WHERE NOT (e)-[:HAS_WORD]->(:Word)
		RETURN count(e) AS count
	`
	result, err := n.ExecuteRead(ctx, countCypher, nil)
	if err != nil {
		return fmt.Errorf("failed to count events: %w", err)
	}

	var eventCount int64
	if result.Next(ctx) {
		if count, ok := result.Record().Values[0].(int64); ok {
			eventCount = count
		}
	}

	if eventCount == 0 {
		n.Logger.Infof("no events to backfill word index for")
		return nil
	}

	n.Logger.Infof("backfilling word index for %d events...", eventCount)

	// Process events in batches: fetch content+tags, tokenize in Go, write Word nodes back
	const fetchBatchSize = 500
	totalProcessed := int64(0)

	for {
		fetchCypher := `
			MATCH (e:Event)
			WHERE NOT (e)-[:HAS_WORD]->(:Word)
			RETURN e.id AS id, e.content AS content, e.tags AS tags
			LIMIT $batchSize
		`
		batchResult, err := n.ExecuteRead(ctx, fetchCypher, map[string]any{
			"batchSize": fetchBatchSize,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch events batch: %w", err)
		}

		var batchCount int64
		for batchResult.Next(ctx) {
			record := batchResult.Record()
			if record == nil {
				continue
			}

			idRaw, _ := record.Get("id")
			contentRaw, _ := record.Get("content")
			tagsRaw, _ := record.Get("tags")

			eventID, _ := idRaw.(string)
			content, _ := contentRaw.(string)
			tagsStr, _ := tagsRaw.(string)

			// Collect unique word tokens from content and tags
			seen := make(map[string]struct{})
			var words []database.WordToken

			if len(content) > 0 {
				for _, wt := range database.TokenWords([]byte(content)) {
					hashHex := hex.EncodeToString(wt.Hash)
					if _, ok := seen[hashHex]; !ok {
						seen[hashHex] = struct{}{}
						words = append(words, wt)
					}
				}
			}

			if tagsStr != "" {
				tags := tag.NewS()
				if err := tags.UnmarshalJSON([]byte(tagsStr)); err == nil && tags != nil {
					for _, t := range *tags {
						for _, field := range t.T {
							if len(field) == 0 {
								continue
							}
							for _, wt := range database.TokenWords(field) {
								hashHex := hex.EncodeToString(wt.Hash)
								if _, ok := seen[hashHex]; !ok {
									seen[hashHex] = struct{}{}
									words = append(words, wt)
								}
							}
						}
					}
				}
			}

			if len(words) > 0 {
				if err := n.addWordsInBatches(ctx, eventID, words); err != nil {
					n.Logger.Warningf("failed to backfill words for event %s: %v",
						safePrefix(eventID, 16), err)
				}
			}

			batchCount++
		}

		if batchCount == 0 {
			break
		}

		totalProcessed += batchCount
		n.Logger.Infof("backfilled word index: %d/%d events processed", totalProcessed, eventCount)
	}

	n.Logger.Infof("word index backfill complete: %d events processed", totalProcessed)
	return nil
}
