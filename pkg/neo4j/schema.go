package neo4j

import (
	"context"
	"fmt"
)

// applySchema creates Neo4j constraints and indexes for Nostr events
// Neo4j uses Cypher queries to define schema constraints and indexes
// Includes both base Nostr relay schema and optional WoT extensions
//
// Schema categories:
//   - MANDATORY (NIP-01): Required for basic REQ filter support per NIP-01 spec
//   - OPTIONAL (Internal): Used for relay internal operations, not required by NIP-01
//   - OPTIONAL (WoT): Web of Trust extensions, relay-specific functionality
//
// NIP-01 REQ filter fields that require indexing:
//   - ids: array of event IDs -> Event.id (MANDATORY)
//   - authors: array of pubkeys -> Author.pubkey (MANDATORY)
//   - kinds: array of integers -> Event.kind (MANDATORY)
//   - #<tag>: tag queries like #e, #p -> Tag.type + Tag.value (MANDATORY)
//   - since: unix timestamp -> Event.created_at (MANDATORY)
//   - until: unix timestamp -> Event.created_at (MANDATORY)
//   - limit: integer -> no index needed, just result limiting
func (n *N) applySchema(ctx context.Context) error {
	n.Logger.Infof("applying Nostr schema to neo4j")

	// Create constraints and indexes using Cypher queries
	// Constraints ensure uniqueness and are automatically indexed
	constraints := []string{
		// ============================================================
		// === MANDATORY: NIP-01 REQ Query Support ===
		// These constraints are required for basic Nostr relay operation
		// ============================================================

		// MANDATORY (NIP-01): Event.id uniqueness for "ids" filter
		// REQ filters can specify: {"ids": ["<event_id>", ...]}
		"CREATE CONSTRAINT event_id_unique IF NOT EXISTS FOR (e:Event) REQUIRE e.id IS UNIQUE",

		// MANDATORY (NIP-01): Author.pubkey uniqueness for "authors" filter
		// REQ filters can specify: {"authors": ["<pubkey>", ...]}
		// Events are linked to Author nodes via AUTHORED_BY relationship
		"CREATE CONSTRAINT author_pubkey_unique IF NOT EXISTS FOR (a:Author) REQUIRE a.pubkey IS UNIQUE",

		// ============================================================
		// === OPTIONAL: Internal Relay Operations ===
		// These are used for relay state management, not NIP-01 queries
		// ============================================================

		// OPTIONAL (Internal): Marker nodes for tracking relay state
		// Used for serial number generation, sync markers, etc.
		"CREATE CONSTRAINT marker_key_unique IF NOT EXISTS FOR (m:Marker) REQUIRE m.key IS UNIQUE",

		// ============================================================
		// === OPTIONAL: Social Graph Event Processing ===
		// Tracks processing of social events for graph updates
		// ============================================================

		// OPTIONAL (Social Graph): Tracks which social events have been processed
		// Used to build/update WoT graph from kinds 0, 3, 1984, 10000
		"CREATE CONSTRAINT processedSocialEvent_event_id IF NOT EXISTS FOR (e:ProcessedSocialEvent) REQUIRE e.event_id IS UNIQUE",

		// ============================================================
		// === OPTIONAL: Web of Trust (WoT) Extension Schema ===
		// These support trust metrics and social graph analysis
		// Not required for NIP-01 compliance
		// ============================================================

		// OPTIONAL (WoT): NostrUser nodes for social graph/trust metrics
		// Separate from Author nodes - Author is for NIP-01, NostrUser for WoT
		"CREATE CONSTRAINT nostrUser_pubkey IF NOT EXISTS FOR (n:NostrUser) REQUIRE n.pubkey IS UNIQUE",

		// OPTIONAL (WoT): Container for WoT metrics cards per observee
		"CREATE CONSTRAINT setOfNostrUserWotMetricsCards_observee_pubkey IF NOT EXISTS FOR (n:SetOfNostrUserWotMetricsCards) REQUIRE n.observee_pubkey IS UNIQUE",

		// OPTIONAL (WoT): Unique WoT metrics card per customer+observee pair
		"CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) REQUIRE (n.customer_id, n.observee_pubkey) IS UNIQUE",

		// OPTIONAL (WoT): Unique WoT metrics card per observer+observee pair
		"CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) REQUIRE (n.observer_pubkey, n.observee_pubkey) IS UNIQUE",
	}

	// Additional indexes for query optimization
	indexes := []string{
		// ============================================================
		// === MANDATORY: NIP-01 REQ Query Indexes ===
		// These indexes are required for efficient NIP-01 filter execution
		// ============================================================

		// MANDATORY (NIP-01): Event.kind index for "kinds" filter
		// REQ filters can specify: {"kinds": [1, 7, ...]}
		"CREATE INDEX event_kind IF NOT EXISTS FOR (e:Event) ON (e.kind)",

		// MANDATORY (NIP-01): Event.created_at index for "since"/"until" filters
		// REQ filters can specify: {"since": <timestamp>, "until": <timestamp>}
		"CREATE INDEX event_created_at IF NOT EXISTS FOR (e:Event) ON (e.created_at)",

		// MANDATORY (NIP-01): Tag.type index for "#<tag>" filter queries
		// REQ filters can specify: {"#e": ["<event_id>"], "#p": ["<pubkey>"], ...}
		"CREATE INDEX tag_type IF NOT EXISTS FOR (t:Tag) ON (t.type)",

		// MANDATORY (NIP-01): Tag.value index for "#<tag>" filter queries
		// Used in conjunction with tag_type for efficient tag lookups
		"CREATE INDEX tag_value IF NOT EXISTS FOR (t:Tag) ON (t.value)",

		// MANDATORY (NIP-01): Composite tag index for "#<tag>" filter queries
		// Most efficient for queries like: {"#p": ["<pubkey>"]}
		"CREATE INDEX tag_type_value IF NOT EXISTS FOR (t:Tag) ON (t.type, t.value)",

		// ============================================================
		// === RECOMMENDED: Performance Optimization Indexes ===
		// These improve query performance but aren't strictly required
		// ============================================================

		// RECOMMENDED: Composite index for common query patterns (kind + created_at)
		// Optimizes queries like: {"kinds": [1], "since": <ts>, "until": <ts>}
		"CREATE INDEX event_kind_created_at IF NOT EXISTS FOR (e:Event) ON (e.kind, e.created_at)",

		// ============================================================
		// === OPTIONAL: Internal Relay Operation Indexes ===
		// Used for relay-internal operations, not NIP-01 queries
		// ============================================================

		// OPTIONAL (Internal): Event.serial for internal serial-based lookups
		// Used for cursor-based pagination and sync operations
		"CREATE INDEX event_serial IF NOT EXISTS FOR (e:Event) ON (e.serial)",

		// OPTIONAL (NIP-40): Event.expiration for expired event cleanup
		// Used by DeleteExpired to efficiently find events past their expiration time
		"CREATE INDEX event_expiration IF NOT EXISTS FOR (e:Event) ON (e.expiration)",

		// ============================================================
		// === OPTIONAL: Social Graph Event Processing Indexes ===
		// Support tracking of processed social events for graph updates
		// ============================================================

		// OPTIONAL (Social Graph): Quick lookup of processed events by pubkey+kind
		"CREATE INDEX processedSocialEvent_pubkey_kind IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.pubkey, e.event_kind)",

		// OPTIONAL (Social Graph): Filter for active (non-superseded) events
		"CREATE INDEX processedSocialEvent_superseded IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.superseded_by)",

		// ============================================================
		// === OPTIONAL: Web of Trust (WoT) Extension Indexes ===
		// These support trust metrics and social graph analysis
		// Not required for NIP-01 compliance
		// ============================================================

		// OPTIONAL (WoT): NostrUser trust metric indexes
		"CREATE INDEX nostrUser_hops IF NOT EXISTS FOR (n:NostrUser) ON (n.hops)",
		"CREATE INDEX nostrUser_personalizedPageRank IF NOT EXISTS FOR (n:NostrUser) ON (n.personalizedPageRank)",
		"CREATE INDEX nostrUser_influence IF NOT EXISTS FOR (n:NostrUser) ON (n.influence)",
		"CREATE INDEX nostrUser_verifiedFollowerCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedFollowerCount)",
		"CREATE INDEX nostrUser_verifiedMuterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedMuterCount)",
		"CREATE INDEX nostrUser_verifiedReporterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedReporterCount)",
		"CREATE INDEX nostrUser_followerInput IF NOT EXISTS FOR (n:NostrUser) ON (n.followerInput)",

		// OPTIONAL (WoT): NostrUserWotMetricsCard indexes for trust card lookups
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

	// Drop all constraints (MANDATORY + OPTIONAL)
	constraints := []string{
		// MANDATORY (NIP-01) constraints
		"DROP CONSTRAINT event_id_unique IF EXISTS",
		"DROP CONSTRAINT author_pubkey_unique IF EXISTS",

		// OPTIONAL (Internal) constraints
		"DROP CONSTRAINT marker_key_unique IF EXISTS",

		// OPTIONAL (Social Graph) constraints
		"DROP CONSTRAINT processedSocialEvent_event_id IF EXISTS",

		// OPTIONAL (WoT) constraints
		"DROP CONSTRAINT nostrUser_pubkey IF EXISTS",
		"DROP CONSTRAINT setOfNostrUserWotMetricsCards_observee_pubkey IF EXISTS",
		"DROP CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF EXISTS",
		"DROP CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF EXISTS",
	}

	for _, constraint := range constraints {
		_, _ = n.ExecuteWrite(ctx, constraint, nil)
		// Ignore errors as constraints may not exist
	}

	// Drop all indexes (MANDATORY + RECOMMENDED + OPTIONAL)
	indexes := []string{
		// MANDATORY (NIP-01) indexes
		"DROP INDEX event_kind IF EXISTS",
		"DROP INDEX event_created_at IF EXISTS",
		"DROP INDEX tag_type IF EXISTS",
		"DROP INDEX tag_value IF EXISTS",
		"DROP INDEX tag_type_value IF EXISTS",

		// RECOMMENDED (Performance) indexes
		"DROP INDEX event_kind_created_at IF EXISTS",

		// OPTIONAL (Internal) indexes
		"DROP INDEX event_serial IF EXISTS",
		"DROP INDEX event_expiration IF EXISTS",

		// OPTIONAL (Social Graph) indexes
		"DROP INDEX processedSocialEvent_pubkey_kind IF EXISTS",
		"DROP INDEX processedSocialEvent_superseded IF EXISTS",

		// OPTIONAL (WoT) indexes
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
