package neo4j

import (
	"context"
	"fmt"
)

// applySchema creates Neo4j constraints and indexes for Nostr events
// Neo4j uses Cypher queries to define schema constraints and indexes
// Includes both base Nostr relay schema and optional WoT extensions
func (n *N) applySchema(ctx context.Context) error {
	n.Logger.Infof("applying Nostr schema to neo4j")

	// Create constraints and indexes using Cypher queries
	// Constraints ensure uniqueness and are automatically indexed
	constraints := []string{
		// === Base Nostr Relay Schema (NIP-01 Queries) ===

		// Unique constraint on Event.id (event ID must be unique)
		"CREATE CONSTRAINT event_id_unique IF NOT EXISTS FOR (e:Event) REQUIRE e.id IS UNIQUE",

		// Unique constraint on Author.pubkey (author public key must be unique)
		// Note: Author nodes are for NIP-01 query support (REQ filters)
		"CREATE CONSTRAINT author_pubkey_unique IF NOT EXISTS FOR (a:Author) REQUIRE a.pubkey IS UNIQUE",

		// Unique constraint on Marker.key (marker key must be unique)
		"CREATE CONSTRAINT marker_key_unique IF NOT EXISTS FOR (m:Marker) REQUIRE m.key IS UNIQUE",

		// === Social Graph Event Processing Schema ===

		// Unique constraint on ProcessedSocialEvent.event_id
		// Tracks which social events (kinds 0, 3, 1984, 10000) have been processed
		"CREATE CONSTRAINT processedSocialEvent_event_id IF NOT EXISTS FOR (e:ProcessedSocialEvent) REQUIRE e.event_id IS UNIQUE",

		// === WoT Extension Schema ===

		// Unique constraint on NostrUser.pubkey
		// Note: NostrUser nodes are for social graph/WoT (separate from Author nodes)
		"CREATE CONSTRAINT nostrUser_pubkey IF NOT EXISTS FOR (n:NostrUser) REQUIRE n.pubkey IS UNIQUE",

		// Unique constraint on SetOfNostrUserWotMetricsCards.observee_pubkey
		"CREATE CONSTRAINT setOfNostrUserWotMetricsCards_observee_pubkey IF NOT EXISTS FOR (n:SetOfNostrUserWotMetricsCards) REQUIRE n.observee_pubkey IS UNIQUE",

		// Unique constraint on NostrUserWotMetricsCard (customer_id, observee_pubkey)
		"CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) REQUIRE (n.customer_id, n.observee_pubkey) IS UNIQUE",

		// Unique constraint on NostrUserWotMetricsCard (observer_pubkey, observee_pubkey)
		"CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) REQUIRE (n.observer_pubkey, n.observee_pubkey) IS UNIQUE",
	}

	// Additional indexes for query optimization
	indexes := []string{
		// === Base Nostr Relay Indexes ===

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

		// === Social Graph Event Processing Indexes ===

		// Index on ProcessedSocialEvent for quick lookup by pubkey and kind
		"CREATE INDEX processedSocialEvent_pubkey_kind IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.pubkey, e.event_kind)",

		// Index on ProcessedSocialEvent.superseded_by to filter active events
		"CREATE INDEX processedSocialEvent_superseded IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.superseded_by)",

		// === WoT Extension Indexes ===

		// NostrUser indexes for trust metrics
		"CREATE INDEX nostrUser_hops IF NOT EXISTS FOR (n:NostrUser) ON (n.hops)",
		"CREATE INDEX nostrUser_personalizedPageRank IF NOT EXISTS FOR (n:NostrUser) ON (n.personalizedPageRank)",
		"CREATE INDEX nostrUser_influence IF NOT EXISTS FOR (n:NostrUser) ON (n.influence)",
		"CREATE INDEX nostrUser_verifiedFollowerCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedFollowerCount)",
		"CREATE INDEX nostrUser_verifiedMuterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedMuterCount)",
		"CREATE INDEX nostrUser_verifiedReporterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedReporterCount)",
		"CREATE INDEX nostrUser_followerInput IF NOT EXISTS FOR (n:NostrUser) ON (n.followerInput)",

		// NostrUserWotMetricsCard indexes
		"CREATE INDEX nostrUserWotMetricsCard_customer_id IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.customer_id)",
		"CREATE INDEX nostrUserWotMetricsCard_observer_pubkey IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.observer_pubkey)",
		"CREATE INDEX nostrUserWotMetricsCard_observee_pubkey IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.observee_pubkey)",
		"CREATE INDEX nostrUserWotMetricsCard_hops IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.hops)",
		"CREATE INDEX nostrUserWotMetricsCard_personalizedPageRank IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.personalizedPageRank)",
		"CREATE INDEX nostrUserWotMetricsCard_influence IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.influence)",
		"CREATE INDEX nostrUserWotMetricsCard_verifiedFollowerCount IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.verifiedFollowerCount)",
		"CREATE INDEX nostrUserWotMetricsCard_verifiedMuterCount IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.verifiedMuterCount)",
		"CREATE INDEX nostrUserWotMetricsCard_verifiedReporterCount IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.verifiedReporterCount)",
		"CREATE INDEX nostrUserWotMetricsCard_followerInput IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.followerInput)",
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

	// Drop all constraints (base + social graph + WoT)
	constraints := []string{
		// Base constraints
		"DROP CONSTRAINT event_id_unique IF EXISTS",
		"DROP CONSTRAINT author_pubkey_unique IF EXISTS",
		"DROP CONSTRAINT marker_key_unique IF EXISTS",

		// Social graph constraints
		"DROP CONSTRAINT processedSocialEvent_event_id IF EXISTS",

		// WoT constraints
		"DROP CONSTRAINT nostrUser_pubkey IF EXISTS",
		"DROP CONSTRAINT setOfNostrUserWotMetricsCards_observee_pubkey IF EXISTS",
		"DROP CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF EXISTS",
		"DROP CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF EXISTS",
	}

	for _, constraint := range constraints {
		_, _ = n.ExecuteWrite(ctx, constraint, nil)
		// Ignore errors as constraints may not exist
	}

	// Drop all indexes (base + social graph + WoT)
	indexes := []string{
		// Base indexes
		"DROP INDEX event_kind IF EXISTS",
		"DROP INDEX event_created_at IF EXISTS",
		"DROP INDEX event_serial IF EXISTS",
		"DROP INDEX event_kind_created_at IF EXISTS",
		"DROP INDEX tag_type IF EXISTS",
		"DROP INDEX tag_value IF EXISTS",
		"DROP INDEX tag_type_value IF EXISTS",

		// Social graph indexes
		"DROP INDEX processedSocialEvent_pubkey_kind IF EXISTS",
		"DROP INDEX processedSocialEvent_superseded IF EXISTS",

		// WoT indexes
		"DROP INDEX nostrUser_hops IF EXISTS",
		"DROP INDEX nostrUser_personalizedPageRank IF EXISTS",
		"DROP INDEX nostrUser_influence IF EXISTS",
		"DROP INDEX nostrUser_verifiedFollowerCount IF EXISTS",
		"DROP INDEX nostrUser_verifiedMuterCount IF EXISTS",
		"DROP INDEX nostrUser_verifiedReporterCount IF EXISTS",
		"DROP INDEX nostrUser_followerInput IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_customer_id IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_observer_pubkey IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_observee_pubkey IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_hops IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_personalizedPageRank IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_influence IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_verifiedFollowerCount IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_verifiedMuterCount IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_verifiedReporterCount IF EXISTS",
		"DROP INDEX nostrUserWotMetricsCard_followerInput IF EXISTS",
	}

	for _, index := range indexes {
		_, _ = n.ExecuteWrite(ctx, index, nil)
		// Ignore errors as indexes may not exist
	}

	// Reapply schema after dropping
	return n.applySchema(ctx)
}
