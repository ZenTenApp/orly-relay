# Common Cypher Patterns for ORLY Nostr Relay

This reference contains project-specific Cypher patterns used in the ORLY Nostr relay's Neo4j backend.

## Schema Overview

### Node Types

| Label | Purpose | Key Properties |
|-------|---------|----------------|
| `Event` | Nostr events (NIP-01) | `id`, `kind`, `pubkey`, `created_at`, `content`, `sig`, `tags`, `serial` |
| `Author` | Event authors (for NIP-01 queries) | `pubkey` |
| `Tag` | Generic tags | `type`, `value` |
| `NostrUser` | Social graph users (WoT) | `pubkey`, `name`, `about`, `picture`, `nip05` |
| `ProcessedSocialEvent` | Social event tracking | `event_id`, `event_kind`, `pubkey`, `superseded_by` |
| `Marker` | Internal state markers | `key`, `value` |

### Relationship Types

| Type | From | To | Purpose |
|------|------|-----|---------|
| `AUTHORED_BY` | Event | Author | Links event to author |
| `TAGGED_WITH` | Event | Tag | Links event to tags |
| `REFERENCES` | Event | Event | e-tag references |
| `MENTIONS` | Event | Author | p-tag mentions |
| `FOLLOWS` | NostrUser | NostrUser | Contact list (kind 3) |
| `MUTES` | NostrUser | NostrUser | Mute list (kind 10000) |
| `REPORTS` | NostrUser | NostrUser | Reports (kind 1984) |

## Event Storage Patterns

### Create Event with Full Relationships

This pattern creates an event and all related nodes/relationships atomically:

```cypher
// 1. Create or get author
MERGE (a:Author {pubkey: $pubkey})

// 2. Create event node
CREATE (e:Event {
  id: $eventId,
  serial: $serial,
  kind: $kind,
  created_at: $createdAt,
  content: $content,
  sig: $sig,
  pubkey: $pubkey,
  tags: $tagsJson  // JSON string for full tag data
})

// 3. Link to author
CREATE (e)-[:AUTHORED_BY]->(a)

// 4. Process e-tags (event references)
WITH e, a
OPTIONAL MATCH (ref0:Event {id: $eTag_0})
FOREACH (_ IN CASE WHEN ref0 IS NOT NULL THEN [1] ELSE [] END |
  CREATE (e)-[:REFERENCES]->(ref0)
)

// 5. Process p-tags (mentions)
WITH e, a
MERGE (mentioned0:Author {pubkey: $pTag_0})
CREATE (e)-[:MENTIONS]->(mentioned0)

// 6. Process other tags
WITH e, a
MERGE (tag0:Tag {type: $tagType_0, value: $tagValue_0})
CREATE (e)-[:TAGGED_WITH]->(tag0)

RETURN e.id AS id
```

### Check Event Existence

```cypher
MATCH (e:Event {id: $id})
RETURN e.id AS id
LIMIT 1
```

### Get Next Serial Number

```cypher
MERGE (m:Marker {key: 'serial'})
ON CREATE SET m.value = 1
ON MATCH SET m.value = m.value + 1
RETURN m.value AS serial
```

## Query Patterns

### Basic Filter Query (NIP-01)

```cypher
MATCH (e:Event)
WHERE e.kind IN $kinds
  AND e.pubkey IN $authors
  AND e.created_at >= $since
  AND e.created_at <= $until
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags,
       e.serial AS serial
ORDER BY e.created_at DESC
LIMIT $limit
```

### Query by Event ID (with prefix support)

```cypher
// Exact match
MATCH (e:Event {id: $id})
RETURN e

// Prefix match
MATCH (e:Event)
WHERE e.id STARTS WITH $idPrefix
RETURN e
```

### Query by Tag (#<tag> filter)

```cypher
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t:Tag)
WHERE t.type = $tagType AND t.value IN $tagValues
RETURN DISTINCT e
ORDER BY e.created_at DESC
LIMIT $limit
```

### Count Events

```cypher
MATCH (e:Event)
WHERE e.kind IN $kinds
RETURN count(e) AS count
```

### Query Delete Events Targeting an Event

```cypher
MATCH (target:Event {id: $targetId})
MATCH (e:Event {kind: 5})-[:REFERENCES]->(target)
RETURN e
ORDER BY e.created_at DESC
```

### Replaceable Event Check (kinds 0, 3, 10000-19999)

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})
WHERE e.created_at < $newCreatedAt
RETURN e.serial AS serial
ORDER BY e.created_at DESC
```

### Parameterized Replaceable Event Check (kinds 30000-39999)

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})-[:TAGGED_WITH]->(t:Tag {type: 'd', value: $dValue})
WHERE e.created_at < $newCreatedAt
RETURN e.serial AS serial
ORDER BY e.created_at DESC
```

## Social Graph Patterns

### Update Profile (Kind 0)

```cypher
MERGE (user:NostrUser {pubkey: $pubkey})
ON CREATE SET
  user.created_at = timestamp(),
  user.first_seen_event = $event_id
ON MATCH SET
  user.last_profile_update = $created_at
SET
  user.name = $name,
  user.about = $about,
  user.picture = $picture,
  user.nip05 = $nip05,
  user.lud16 = $lud16,
  user.display_name = $display_name
```

### Contact List Update (Kind 3) - Diff-Based

```cypher
// Mark old event as superseded
OPTIONAL MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
SET old.superseded_by = $new_event_id

// Create new event tracking
CREATE (new:ProcessedSocialEvent {
  event_id: $new_event_id,
  event_kind: 3,
  pubkey: $author_pubkey,
  created_at: $created_at,
  processed_at: timestamp(),
  relationship_count: $total_follows,
  superseded_by: null
})

// Get or create author
MERGE (author:NostrUser {pubkey: $author_pubkey})

// Update unchanged relationships to new event
WITH author
OPTIONAL MATCH (author)-[unchanged:FOLLOWS]->(followed:NostrUser)
WHERE unchanged.created_by_event = $old_event_id
  AND NOT followed.pubkey IN $removed_follows
SET unchanged.created_by_event = $new_event_id,
    unchanged.created_at = $created_at

// Remove old relationships for removed follows
WITH author
OPTIONAL MATCH (author)-[old_follows:FOLLOWS]->(followed:NostrUser)
WHERE old_follows.created_by_event = $old_event_id
  AND followed.pubkey IN $removed_follows
DELETE old_follows

// Create new relationships for added follows
WITH author
UNWIND $added_follows AS followed_pubkey
MERGE (followed:NostrUser {pubkey: followed_pubkey})
MERGE (author)-[new_follows:FOLLOWS]->(followed)
ON CREATE SET
  new_follows.created_by_event = $new_event_id,
  new_follows.created_at = $created_at,
  new_follows.relay_received_at = timestamp()
ON MATCH SET
  new_follows.created_by_event = $new_event_id,
  new_follows.created_at = $created_at
```

### Create Report (Kind 1984)

```cypher
// Create tracking node
CREATE (evt:ProcessedSocialEvent {
  event_id: $event_id,
  event_kind: 1984,
  pubkey: $reporter_pubkey,
  created_at: $created_at,
  processed_at: timestamp(),
  relationship_count: 1,
  superseded_by: null
})

// Create users and relationship
MERGE (reporter:NostrUser {pubkey: $reporter_pubkey})
MERGE (reported:NostrUser {pubkey: $reported_pubkey})
CREATE (reporter)-[:REPORTS {
  created_by_event: $event_id,
  created_at: $created_at,
  relay_received_at: timestamp(),
  report_type: $report_type
}]->(reported)
```

### Get Latest Social Event for Pubkey

```cypher
MATCH (evt:ProcessedSocialEvent {pubkey: $pubkey, event_kind: $kind})
WHERE evt.superseded_by IS NULL
RETURN evt.event_id AS event_id,
       evt.created_at AS created_at,
       evt.relationship_count AS relationship_count
ORDER BY evt.created_at DESC
LIMIT 1
```

### Get Follows for Event

```cypher
MATCH (author:NostrUser)-[f:FOLLOWS]->(followed:NostrUser)
WHERE f.created_by_event = $event_id
RETURN collect(followed.pubkey) AS pubkeys
```

## WoT Query Patterns

### Find Mutual Follows

```cypher
MATCH (a:NostrUser {pubkey: $pubkeyA})-[:FOLLOWS]->(b:NostrUser)
WHERE (b)-[:FOLLOWS]->(a)
RETURN b.pubkey AS mutual_friend
```

### Find Followers

```cypher
MATCH (follower:NostrUser)-[:FOLLOWS]->(user:NostrUser {pubkey: $pubkey})
RETURN follower.pubkey, follower.name
```

### Find Following

```cypher
MATCH (user:NostrUser {pubkey: $pubkey})-[:FOLLOWS]->(following:NostrUser)
RETURN following.pubkey, following.name
```

### Hop Distance (Trust Path)

```cypher
MATCH (start:NostrUser {pubkey: $startPubkey})
MATCH (end:NostrUser {pubkey: $endPubkey})
MATCH path = shortestPath((start)-[:FOLLOWS*..6]->(end))
RETURN length(path) AS hops, [n IN nodes(path) | n.pubkey] AS path
```

### Second-Degree Connections

```cypher
MATCH (me:NostrUser {pubkey: $myPubkey})-[:FOLLOWS]->(:NostrUser)-[:FOLLOWS]->(suggested:NostrUser)
WHERE NOT (me)-[:FOLLOWS]->(suggested)
  AND suggested.pubkey <> $myPubkey
RETURN suggested.pubkey, count(*) AS commonFollows
ORDER BY commonFollows DESC
LIMIT 20
```

## Schema Management Patterns

### Create Constraint

```cypher
CREATE CONSTRAINT event_id_unique IF NOT EXISTS
FOR (e:Event) REQUIRE e.id IS UNIQUE
```

### Create Index

```cypher
CREATE INDEX event_kind IF NOT EXISTS
FOR (e:Event) ON (e.kind)
```

### Create Composite Index

```cypher
CREATE INDEX event_kind_created_at IF NOT EXISTS
FOR (e:Event) ON (e.kind, e.created_at)
```

### Drop All Data (Testing Only)

```cypher
MATCH (n) DETACH DELETE n
```

## Performance Patterns

### Use EXPLAIN/PROFILE

```cypher
// See query plan without running
EXPLAIN MATCH (e:Event) WHERE e.kind = 1 RETURN e

// Run and see actual metrics
PROFILE MATCH (e:Event) WHERE e.kind = 1 RETURN e
```

### Batch Import with UNWIND

```cypher
UNWIND $events AS evt
CREATE (e:Event {
  id: evt.id,
  kind: evt.kind,
  pubkey: evt.pubkey,
  created_at: evt.created_at,
  content: evt.content,
  sig: evt.sig,
  tags: evt.tags
})
```

### Efficient Pagination

```cypher
// Use indexed ORDER BY with WHERE for cursor-based pagination
MATCH (e:Event)
WHERE e.kind = 1 AND e.created_at < $cursor
RETURN e
ORDER BY e.created_at DESC
LIMIT 20
```
