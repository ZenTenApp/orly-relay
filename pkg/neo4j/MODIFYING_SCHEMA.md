# Modifying the Neo4j Schema

This document provides a comprehensive guide to the Neo4j database schema used by ORLY for Nostr event storage and Web of Trust (WoT) calculations. It is intended to help external developers understand and modify the schema for their applications.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Code Locations](#code-locations)
3. [NIP-01 Mandatory Schema](#nip-01-mandatory-schema)
4. [NIP-01 Query Construction](#nip-01-query-construction)
5. [Optional Social Graph Schema](#optional-social-graph-schema)
6. [Web of Trust (WoT) Schema](#web-of-trust-wot-schema)
7. [Modifying the Schema](#modifying-the-schema)

---

## Architecture Overview

The Neo4j implementation uses a **dual-node architecture** to separate concerns:

1. **NIP-01 Base Layer**: Stores Nostr events with `Event`, `Author`, and `Tag` nodes for standard relay operations
2. **WoT Extension Layer**: Stores social graph data with `NostrUser` nodes and relationship types (`FOLLOWS`, `MUTES`, `REPORTS`) for trust calculations

This separation allows the WoT extension to be modified independently without affecting NIP-01 compliance.

### Data Model Summary

From the specification document:

**Node Labels:**
- `NostrUser` - User identity for social graph (WoT layer)
- `NostrEvent` - Event storage (maps to `Event` in current implementation)
- `NostrEventTag` - Tag data (maps to `Tag` in current implementation)
- `NostrRelay` - Relay metadata
- `NostrUserWotMetricsCard` - Trust metrics per observer/observee pair
- `SetOfNostrUserWotMetricsCards` - Container for metrics cards (optional)

**Relationship Types:**
- NIP-01: `AUTHORED_BY`, `HAS_TAG` (current: `TAGGED_WITH`), `REFERENCES`, `SUGGESTED_RELAY`
- NIP-02: `FOLLOWS` (with timestamp)
- NIP-51: `MUTES` (with timestamp)
- NIP-56: `REPORTS` (with timestamp, report_type)
- Content relationships: `IS_A_REPLY_TO`, `IS_A_REACTION_TO`, `IS_A_REPOST_OF`, `IS_A_COMMENT_ON`

---

## Code Locations

### Core Files

| File | Purpose |
|------|---------|
| [`schema.go`](schema.go) | **Schema definitions** - All constraints and indexes are defined here |
| [`neo4j.go`](neo4j.go) | Database connection and initialization |
| [`save-event.go`](save-event.go) | Event storage with node/relationship creation |
| [`query-events.go`](query-events.go) | NIP-01 filter → Cypher query translation |
| [`social-event-processor.go`](social-event-processor.go) | WoT relationship management (FOLLOWS, MUTES, REPORTS) |

### Supporting Files

| File | Purpose |
|------|---------|
| [`fetch-event.go`](fetch-event.go) | Event retrieval by serial/ID |
| [`delete.go`](delete.go) | Event deletion and NIP-09 handling |
| [`serial.go`](serial.go) | Serial number generation using Marker nodes |
| [`markers.go`](markers.go) | General key-value metadata storage |
| [`identity.go`](identity.go) | Relay identity management |

---

## NIP-01 Mandatory Schema

These elements are **required** for a NIP-01 compliant relay.

### Constraints (schema.go:30-43)

```cypher
-- Event ID uniqueness (for "ids" filter)
CREATE CONSTRAINT event_id_unique IF NOT EXISTS
FOR (e:Event) REQUIRE e.id IS UNIQUE

-- Author pubkey uniqueness (for "authors" filter)
CREATE CONSTRAINT author_pubkey_unique IF NOT EXISTS
FOR (a:Author) REQUIRE a.pubkey IS UNIQUE
```

### Indexes (schema.go:84-108)

```cypher
-- "kinds" filter
CREATE INDEX event_kind IF NOT EXISTS FOR (e:Event) ON (e.kind)

-- "since"/"until" filters
CREATE INDEX event_created_at IF NOT EXISTS FOR (e:Event) ON (e.created_at)

-- "#<tag>" filters (e.g., #e, #p, #t)
CREATE INDEX tag_type IF NOT EXISTS FOR (t:Tag) ON (t.type)
CREATE INDEX tag_value IF NOT EXISTS FOR (t:Tag) ON (t.value)
CREATE INDEX tag_type_value IF NOT EXISTS FOR (t:Tag) ON (t.type, t.value)
```

### Event Node Properties

Created in `save-event.go:buildEventCreationCypher()`:

```go
// Event node structure
(e:Event {
  id: string,          // 64-char hex event ID
  serial: int64,       // Internal monotonic serial number
  kind: int64,         // Event kind (0, 1, 3, 7, etc.)
  created_at: int64,   // Unix timestamp
  content: string,     // Event content
  sig: string,         // 128-char hex signature
  pubkey: string,      // 64-char hex author pubkey
  tags: string         // JSON-serialized tags array
})
```

### Relationship Types (NIP-01)

Created in `save-event.go:buildEventCreationCypher()`:

```cypher
-- Event → Author relationship
(e:Event)-[:AUTHORED_BY]->(a:Author {pubkey: ...})

-- Event → Event reference (e-tags)
(e:Event)-[:REFERENCES]->(ref:Event)

-- Event → Author mention (p-tags)
(e:Event)-[:MENTIONS]->(mentioned:Author)

-- Event → Tag (other tags like #t, #d, etc.)
(e:Event)-[:TAGGED_WITH]->(t:Tag {type: ..., value: ...})
```

---

## NIP-01 Query Construction

The `query-events.go` file translates Nostr REQ filters into Cypher queries.

### Filter to Cypher Mapping

| NIP-01 Filter | Cypher Translation | Index Used |
|---------------|-------------------|------------|
| `ids: ["abc..."]` | `e.id = $id_0` or `e.id STARTS WITH $id_0` | `event_id_unique` |
| `authors: ["def..."]` | `e.pubkey = $author_0` or `e.pubkey STARTS WITH $author_0` | `author_pubkey_unique` |
| `kinds: [1, 7]` | `e.kind IN $kinds` | `event_kind` |
| `since: 1234567890` | `e.created_at >= $since` | `event_created_at` |
| `until: 1234567890` | `e.created_at <= $until` | `event_created_at` |
| `#p: ["pubkey1"]` | Tag join with `type='p' AND value IN $tagValues` | `tag_type_value` |
| `limit: 100` | `LIMIT $limit` | N/A |

### Query Builder (query-events.go:49-182)

```go
func (n *N) buildCypherQuery(f *filter.F, includeDeleteEvents bool) (string, map[string]any) {
    // Base match clause
    matchClause := "MATCH (e:Event)"

    // IDs filter - supports prefix matching
    if len(f.Ids.T) > 0 {
        // Full ID: e.id = $id_0
        // Prefix:  e.id STARTS WITH $id_0
    }

    // Authors filter - supports prefix matching
    if len(f.Authors.T) > 0 {
        // Same pattern as IDs
    }

    // Kinds filter
    if len(f.Kinds.K) > 0 {
        whereClauses = append(whereClauses, "e.kind IN $kinds")
    }

    // Time range filters
    if f.Since != nil {
        whereClauses = append(whereClauses, "e.created_at >= $since")
    }
    if f.Until != nil {
        whereClauses = append(whereClauses, "e.created_at <= $until")
    }

    // Tag filters - joins with Tag nodes via TAGGED_WITH
    for _, tagValues := range *f.Tags {
        matchClause += fmt.Sprintf(" OPTIONAL MATCH (e)-[:TAGGED_WITH]->(%s:Tag)", tagVarName)
        // WHERE conditions for tag type and values
    }
}
```

---

## Optional Social Graph Schema

These elements support social graph processing but are **not required** for NIP-01.

### Processed Event Tracking (schema.go:59-61)

Tracks which social events (kinds 0, 3, 1984, 10000) have been processed:

```cypher
CREATE CONSTRAINT processedSocialEvent_event_id IF NOT EXISTS
FOR (e:ProcessedSocialEvent) REQUIRE e.event_id IS UNIQUE

CREATE INDEX processedSocialEvent_pubkey_kind IF NOT EXISTS
FOR (e:ProcessedSocialEvent) ON (e.pubkey, e.event_kind)

CREATE INDEX processedSocialEvent_superseded IF NOT EXISTS
FOR (e:ProcessedSocialEvent) ON (e.superseded_by)
```

### Social Event Processing (social-event-processor.go)

The `SocialEventProcessor` handles:

1. **Kind 0 (Profile Metadata)**: Updates `NostrUser` node with profile data
2. **Kind 3 (Contact List)**: Creates/updates `FOLLOWS` relationships
3. **Kind 10000 (Mute List)**: Creates/updates `MUTES` relationships
4. **Kind 1984 (Reports)**: Creates `REPORTS` relationships

**FOLLOWS Relationship** (social-event-processor.go:294-357):
```cypher
-- Contact list diff-based update
MERGE (author:NostrUser {pubkey: $author_pubkey})

-- Update unchanged follows to new event
MATCH (author)-[unchanged:FOLLOWS]->(followed:NostrUser)
WHERE unchanged.created_by_event = $old_event_id
  AND NOT followed.pubkey IN $removed_follows
SET unchanged.created_by_event = $new_event_id

-- Remove old follows
MATCH (author)-[old_follows:FOLLOWS]->(followed:NostrUser)
WHERE old_follows.created_by_event = $old_event_id
  AND followed.pubkey IN $removed_follows
DELETE old_follows

-- Create new follows
UNWIND $added_follows AS followed_pubkey
MERGE (followed:NostrUser {pubkey: followed_pubkey})
CREATE (author)-[:FOLLOWS {
  created_by_event: $new_event_id,
  created_at: $created_at,
  relay_received_at: timestamp()
}]->(followed)
```

---

## Web of Trust (WoT) Schema

These elements support trust metrics calculations and are managed by an **external application**.

### WoT Constraints (schema.go:69-80)

```cypher
-- NostrUser uniqueness
CREATE CONSTRAINT nostrUser_pubkey IF NOT EXISTS
FOR (n:NostrUser) REQUIRE n.pubkey IS UNIQUE

-- Metrics card container
CREATE CONSTRAINT setOfNostrUserWotMetricsCards_observee_pubkey IF NOT EXISTS
FOR (n:SetOfNostrUserWotMetricsCards) REQUIRE n.observee_pubkey IS UNIQUE

-- Unique metrics card per customer+observee
CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF NOT EXISTS
FOR (n:NostrUserWotMetricsCard) REQUIRE (n.customer_id, n.observee_pubkey) IS UNIQUE

-- Unique metrics card per observer+observee
CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF NOT EXISTS
FOR (n:NostrUserWotMetricsCard) REQUIRE (n.observer_pubkey, n.observee_pubkey) IS UNIQUE
```

### WoT Indexes (schema.go:145-164)

```cypher
-- NostrUser trust metrics
CREATE INDEX nostrUser_hops IF NOT EXISTS FOR (n:NostrUser) ON (n.hops)
CREATE INDEX nostrUser_personalizedPageRank IF NOT EXISTS FOR (n:NostrUser) ON (n.personalizedPageRank)
CREATE INDEX nostrUser_influence IF NOT EXISTS FOR (n:NostrUser) ON (n.influence)
CREATE INDEX nostrUser_verifiedFollowerCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedFollowerCount)
CREATE INDEX nostrUser_verifiedMuterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedMuterCount)
CREATE INDEX nostrUser_verifiedReporterCount IF NOT EXISTS FOR (n:NostrUser) ON (n.verifiedReporterCount)
CREATE INDEX nostrUser_followerInput IF NOT EXISTS FOR (n:NostrUser) ON (n.followerInput)

-- NostrUserWotMetricsCard indexes
CREATE INDEX nostrUserWotMetricsCard_customer_id IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.customer_id)
CREATE INDEX nostrUserWotMetricsCard_observer_pubkey IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.observer_pubkey)
CREATE INDEX nostrUserWotMetricsCard_observee_pubkey IF NOT EXISTS FOR (n:NostrUserWotMetricsCard) ON (n.observee_pubkey)
-- ... additional metric indexes
```

### NostrUser Node Properties

From the specification:

```cypher
(:NostrUser {
  pubkey: string,                 -- 64-char hex public key
  name: string,                   -- Profile name (from kind 0)
  about: string,                  -- Profile bio (from kind 0)
  picture: string,                -- Profile picture URL (from kind 0)
  nip05: string,                  -- NIP-05 identifier (from kind 0)
  lud16: string,                  -- Lightning address (from kind 0)
  display_name: string,           -- Display name (from kind 0)
  npub: string,                   -- Bech32 encoded pubkey

  -- WoT metrics (populated by external application)
  hops: int,                      -- Distance from observer
  personalizedPageRank: float,    -- PageRank score
  influence: float,               -- Influence score
  verifiedFollowerCount: int,     -- Count of verified followers
  verifiedMuterCount: int,        -- Count of verified muters
  verifiedReporterCount: int,     -- Count of verified reporters
  followerInput: float            -- Follower input score
})
```

### NostrUserWotMetricsCard Properties

```cypher
(:NostrUserWotMetricsCard {
  customer_id: string,            -- Customer identifier
  observer_pubkey: string,        -- Observer's pubkey
  observee_pubkey: string,        -- Observee's pubkey
  hops: int,                      -- Distance from observer to observee
  influence: float,               -- Influence score
  average: float,                 -- Average metric
  input: float,                   -- Input score
  confidence: float,              -- Confidence level
  personalizedPageRank: float,    -- Personalized PageRank
  verifiedFollowerCount: int,     -- Verified follower count
  verifiedMuterCount: int,        -- Verified muter count
  verifiedReporterCount: int,     -- Verified reporter count
  followerInput: float,           -- Follower input
  muterInput: float,              -- Muter input
  reporterInput: float            -- Reporter input
})
```

### WoT Relationship Properties

```cypher
-- FOLLOWS relationship (from kind 3 events)
[:FOLLOWS {
  created_by_event: string,       -- Event ID that created this follow
  created_at: int64,              -- Unix timestamp from event
  relay_received_at: int64,       -- When relay received the event
  timestamp: string               -- (spec format)
}]

-- MUTES relationship (from kind 10000 events)
[:MUTES {
  created_by_event: string,
  created_at: int64,
  relay_received_at: int64,
  timestamp: string
}]

-- REPORTS relationship (from kind 1984 events)
[:REPORTS {
  created_by_event: string,
  created_at: int64,
  relay_received_at: int64,
  timestamp: string,
  report_type: string             -- Report reason (spam, nudity, etc.)
}]

-- WOT_METRICS_CARD relationship
[:WOT_METRICS_CARD]->(NostrUserWotMetricsCard)
```

---

## Modifying the Schema

### Adding New Indexes

1. **Edit `schema.go`**: Add your index to the `indexes` slice in `applySchema()`
2. **Add corresponding DROP**: Add the index name to `dropAll()` for clean wipes
3. **Document**: Update this file with the new index

Example:
```go
// In applySchema() indexes slice:
"CREATE INDEX nostrUser_myNewField IF NOT EXISTS FOR (n:NostrUser) ON (n.myNewField)",

// In dropAll() indexes slice:
"DROP INDEX nostrUser_myNewField IF EXISTS",
```

### Adding New Constraints

1. **Edit `schema.go`**: Add your constraint to the `constraints` slice
2. **Add corresponding DROP**: Add to `dropAll()`
3. **Update node creation**: Ensure the constrained field is populated in `save-event.go` or `social-event-processor.go`

### Adding New Node Labels

1. **Define constraints/indexes** in `schema.go`
2. **Create nodes** in appropriate handler (e.g., `social-event-processor.go` for social nodes)
3. **Update queries** in `query-events.go` if the nodes participate in NIP-01 queries

### Adding New Relationship Types

For new relationship types like `IS_A_REPLY_TO`, `IS_A_REACTION_TO`, etc.:

1. **Process in `save-event.go`**: Detect the event kind and create appropriate relationships
2. **Add indexes** if needed for traversal performance
3. **Document** the relationship properties

Example for replies (NIP-10):
```go
// In buildEventCreationCypher(), add handling for kind 1 events with reply markers:
if ev.Kind == 1 {
    // Check for e-tags with "reply" or "root" markers
    for _, tag := range *ev.Tags {
        if string(tag.T[0]) == "e" && len(tag.T) >= 4 {
            marker := string(tag.T[3])
            if marker == "reply" || marker == "root" {
                cypher += `
                OPTIONAL MATCH (parent:Event {id: $parentId})
                FOREACH (ignoreMe IN CASE WHEN parent IS NOT NULL THEN [1] ELSE [] END |
                  CREATE (e)-[:IS_A_REPLY_TO {marker: $marker}]->(parent)
                )`
            }
        }
    }
}
```

### Adding NostrEventTag → NostrUser REFERENCES

Per the specification update, p-tags should create `REFERENCES` relationships to `NostrUser` nodes:

```go
// In save-event.go buildEventCreationCypher(), modify p-tag handling:
case "p":
    // Current implementation: creates MENTIONS to Author
    cypher += fmt.Sprintf(`
    MERGE (mentioned%d:Author {pubkey: $%s})
    CREATE (e)-[:MENTIONS]->(mentioned%d)
    `, pTagIndex, paramName, pTagIndex)

    // NEW: Also reference NostrUser for WoT traversal
    cypher += fmt.Sprintf(`
    MERGE (user%d:NostrUser {pubkey: $%s})
    // Create a Tag node for the p-tag
    MERGE (pTag%d:NostrEventTag {tag_name: 'p', tag_value: $%s})
    CREATE (e)-[:HAS_TAG]->(pTag%d)
    CREATE (pTag%d)-[:REFERENCES]->(user%d)
    `, pTagIndex, paramName, pTagIndex, paramName, pTagIndex, pTagIndex, pTagIndex)
```

---

## Testing Schema Changes

1. **Unit tests**: Run `go test ./pkg/neo4j/...`
2. **Schema application**: Test with a fresh Neo4j instance
3. **Query performance**: Use `EXPLAIN` and `PROFILE` in Neo4j Browser
4. **Migration**: For existing databases, create a migration script

```bash
# Test schema application
CGO_ENABLED=0 go test -v ./pkg/neo4j -run TestSchema
```

---

## References

- [NIP-01: Basic Protocol](https://github.com/nostr-protocol/nips/blob/master/01.md)
- [NIP-02: Follow List](https://github.com/nostr-protocol/nips/blob/master/02.md)
- [NIP-51: Lists](https://github.com/nostr-protocol/nips/blob/master/51.md)
- [NIP-56: Reporting](https://github.com/nostr-protocol/nips/blob/master/56.md)
- [Neo4j Data Modeling](https://neo4j.com/docs/getting-started/data-modeling/)
- [NosFabrica Data Model Specification](https://notion.so/Data-Model-for-a-Neo4j-Nostr-Relay-2b30dd16b665800fb16df4756ed3f3ad)
