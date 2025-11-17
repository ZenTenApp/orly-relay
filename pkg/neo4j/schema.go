package neo4j

import (
	"context"
	"fmt"
)

// applySchema creates Neo4j constraints and indexes for Nostr events
// Neo4j uses Cypher queries to define schema constraints and indexes
func (n *N) applySchema(ctx context.Context) error {
	n.Logger.Infof("applying Nostr schema to neo4j")

	// Create constraints and indexes using Cypher queries
	// Constraints ensure uniqueness and are automatically indexed
	constraints := []string{
		// Unique constraint on Event.id (event ID must be unique)
		"CREATE CONSTRAINT event_id_unique IF NOT EXISTS FOR (e:Event) REQUIRE e.id IS UNIQUE",

		// Unique constraint on Author.pubkey (author public key must be unique)
		"CREATE CONSTRAINT author_pubkey_unique IF NOT EXISTS FOR (a:Author) REQUIRE a.pubkey IS UNIQUE",

		// Unique constraint on Marker.key (marker key must be unique)
		"CREATE CONSTRAINT marker_key_unique IF NOT EXISTS FOR (m:Marker) REQUIRE m.key IS UNIQUE",
	}

	// Additional indexes for query optimization
	indexes := []string{
		// Index on Event.kind for kind-based queries
		"CREATE INDEX event_kind IF NOT EXISTS FOR (e:Event) ON (e.kind)",

		// Index on Event.created_at for time-range queries
		"CREATE INDEX event_created_at IF NOT EXISTS FOR (e:Event) ON (e.created_at)",

		// Index on Event.serial for serial-based lookups
		"CREATE INDEX event_serial IF NOT EXISTS FOR (e:Event) ON (e.serial)",

		// Composite index for common query patterns (kind + created_at)
		"CREATE INDEX event_kind_created_at IF NOT EXISTS FOR (e:Event) ON (e.kind, e.created_at)",

		// Index on Tag.type for tag-type queries
		"CREATE INDEX tag_type IF NOT EXISTS FOR (t:Tag) ON (t.type)",

		// Index on Tag.value for tag-value queries
		"CREATE INDEX tag_value IF NOT EXISTS FOR (t:Tag) ON (t.value)",

		// Composite index for tag queries (type + value)
		"CREATE INDEX tag_type_value IF NOT EXISTS FOR (t:Tag) ON (t.type, t.value)",
	}

	// Execute all constraint creation queries
	for _, constraint := range constraints {
		if _, err := n.ExecuteWrite(ctx, constraint, nil); err != nil {
			return fmt.Errorf("failed to create constraint: %w", err)
		}
	}

	// Execute all index creation queries
	for _, index := range indexes {
		if _, err := n.ExecuteWrite(ctx, index, nil); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	n.Logger.Infof("schema applied successfully")
	return nil
}

// dropAll drops all data from neo4j (useful for testing)
func (n *N) dropAll(ctx context.Context) error {
	n.Logger.Warningf("dropping all data from neo4j")

	// Delete all nodes and relationships
	_, err := n.ExecuteWrite(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("failed to drop all data: %w", err)
	}

	// Drop all constraints
	constraints := []string{
		"DROP CONSTRAINT event_id_unique IF EXISTS",
		"DROP CONSTRAINT author_pubkey_unique IF EXISTS",
		"DROP CONSTRAINT marker_key_unique IF EXISTS",
	}

	for _, constraint := range constraints {
		_, _ = n.ExecuteWrite(ctx, constraint, nil)
		// Ignore errors as constraints may not exist
	}

	// Drop all indexes
	indexes := []string{
		"DROP INDEX event_kind IF EXISTS",
		"DROP INDEX event_created_at IF EXISTS",
		"DROP INDEX event_serial IF EXISTS",
		"DROP INDEX event_kind_created_at IF EXISTS",
		"DROP INDEX tag_type IF EXISTS",
		"DROP INDEX tag_value IF EXISTS",
		"DROP INDEX tag_type_value IF EXISTS",
	}

	for _, index := range indexes {
		_, _ = n.ExecuteWrite(ctx, index, nil)
		// Ignore errors as indexes may not exist
	}

	// Reapply schema after dropping
	return n.applySchema(ctx)
}
