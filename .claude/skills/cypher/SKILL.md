---
name: cypher
description: This skill should be used when writing, debugging, or discussing Neo4j Cypher queries. Provides comprehensive knowledge of Cypher syntax, query patterns, performance optimization, and common mistakes. Particularly useful for translating between domain models and graph queries.
---

# Neo4j Cypher Query Language

## Purpose

This skill provides expert-level guidance for writing Neo4j Cypher queries, including syntax, patterns, performance optimization, and common pitfalls. It is particularly tuned for the patterns used in this ORLY Nostr relay codebase.

## When to Use

Activate this skill when:
- Writing Cypher queries for Neo4j
- Debugging Cypher syntax errors
- Optimizing query performance
- Translating Nostr filter queries to Cypher
- Working with graph relationships and traversals
- Creating or modifying schema (indexes, constraints)

## Core Cypher Syntax

### Clause Order (CRITICAL)

Cypher requires clauses in a specific order. Violating this causes syntax errors:

```cypher
// CORRECT order of clauses
MATCH (n:Label)           // 1. Pattern matching
WHERE n.prop = value      // 2. Filtering
WITH n, count(*) AS cnt   // 3. Intermediate results (resets scope)
OPTIONAL MATCH (n)-[r]-() // 4. Optional patterns
CREATE (m:NewNode)        // 5. Node/relationship creation
SET n.prop = value        // 6. Property updates
DELETE r                  // 7. Deletions
RETURN n.prop AS result   // 8. Return clause
ORDER BY result DESC      // 9. Ordering
SKIP 10 LIMIT 20          // 10. Pagination
```

### The WITH Clause (CRITICAL)

The `WITH` clause is required to transition between certain operations:

**Rule: Cannot use MATCH after CREATE without WITH**

```cypher
// WRONG - MATCH after CREATE without WITH
CREATE (e:Event {id: $id})
MATCH (ref:Event {id: $refId})  // ERROR!
CREATE (e)-[:REFERENCES]->(ref)

// CORRECT - Use WITH to carry variables forward
CREATE (e:Event {id: $id})
WITH e
MATCH (ref:Event {id: $refId})
CREATE (e)-[:REFERENCES]->(ref)
```

**Rule: WITH resets the scope**

Variables not included in WITH are no longer accessible:

```cypher
// WRONG - 'a' is lost after WITH
MATCH (a:Author), (e:Event)
WITH e
WHERE a.pubkey = $pubkey  // ERROR: 'a' not defined

// CORRECT - Include all needed variables
MATCH (a:Author), (e:Event)
WITH a, e
WHERE a.pubkey = $pubkey
```

### Node and Relationship Patterns

```cypher
// Nodes
(n)                     // Anonymous node
(n:Label)               // Labeled node
(n:Label {prop: value}) // Node with properties
(n:Label:OtherLabel)    // Multiple labels

// Relationships
-[r]->                  // Directed, anonymous
-[r:TYPE]->             // Typed relationship
-[r:TYPE {prop: value}]-> // With properties
-[r:TYPE|OTHER]->       // Multiple types (OR)
-[*1..3]->              // Variable length (1 to 3 hops)
-[*]->                  // Any number of hops
```

### MERGE vs CREATE

**CREATE**: Always creates new nodes/relationships (may create duplicates)

```cypher
CREATE (n:Event {id: $id})  // Creates even if id exists
```

**MERGE**: Finds or creates (idempotent)

```cypher
MERGE (n:Event {id: $id})   // Finds existing or creates new
ON CREATE SET n.created = timestamp()
ON MATCH SET n.accessed = timestamp()
```

**Best Practice**: Use MERGE for reference nodes, CREATE for unique events

```cypher
// Reference nodes - use MERGE (idempotent)
MERGE (author:Author {pubkey: $pubkey})

// Unique events - use CREATE (after checking existence)
CREATE (e:Event {id: $eventId, ...})
```

### OPTIONAL MATCH

Returns NULL for non-matching patterns (like LEFT JOIN):

```cypher
// Find events, with or without tags
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t:Tag)
RETURN e.id, collect(t.value) AS tags
```

### Conditional Creation with FOREACH

To conditionally create relationships:

```cypher
// FOREACH trick for conditional operations
OPTIONAL MATCH (ref:Event {id: $refId})
FOREACH (ignoreMe IN CASE WHEN ref IS NOT NULL THEN [1] ELSE [] END |
  CREATE (e)-[:REFERENCES]->(ref)
)
```

### Aggregation Functions

```cypher
count(*)              // Count all rows
count(n)              // Count non-null values
count(DISTINCT n)     // Count unique values
collect(n)            // Collect into list
collect(DISTINCT n)   // Collect unique values
sum(n.value)          // Sum values
avg(n.value)          // Average
min(n.value), max(n.value)  // Min/max
```

### String Operations

```cypher
// String matching
WHERE n.name STARTS WITH 'prefix'
WHERE n.name ENDS WITH 'suffix'
WHERE n.name CONTAINS 'substring'
WHERE n.name =~ 'regex.*pattern'  // Regex

// String functions
toLower(str), toUpper(str)
trim(str), ltrim(str), rtrim(str)
substring(str, start, length)
replace(str, search, replacement)
```

### List Operations

```cypher
// IN clause
WHERE n.kind IN [1, 7, 30023]
WHERE n.pubkey IN $pubkeyList

// List comprehension
[x IN list WHERE x > 0 | x * 2]

// UNWIND - expand list into rows
UNWIND $pubkeys AS pubkey
MERGE (u:User {pubkey: pubkey})
```

### Parameters

Always use parameters for values (security + performance):

```cypher
// CORRECT - parameterized
MATCH (e:Event {id: $eventId})
WHERE e.kind IN $kinds

// WRONG - string interpolation (SQL injection risk!)
MATCH (e:Event {id: '" + eventId + "'})
```

## Schema Management

### Constraints

```cypher
// Uniqueness constraint (also creates index)
CREATE CONSTRAINT event_id_unique IF NOT EXISTS
FOR (e:Event) REQUIRE e.id IS UNIQUE

// Composite uniqueness
CREATE CONSTRAINT card_unique IF NOT EXISTS
FOR (c:Card) REQUIRE (c.customer_id, c.observee_pubkey) IS UNIQUE

// Drop constraint
DROP CONSTRAINT event_id_unique IF EXISTS
```

### Indexes

```cypher
// Single property index
CREATE INDEX event_kind IF NOT EXISTS FOR (e:Event) ON (e.kind)

// Composite index
CREATE INDEX event_kind_created IF NOT EXISTS
FOR (e:Event) ON (e.kind, e.created_at)

// Drop index
DROP INDEX event_kind IF EXISTS
```

## Common Query Patterns

### Find with Filter

```cypher
// Multiple conditions with OR
MATCH (e:Event)
WHERE e.kind IN $kinds
  AND (e.id = $id1 OR e.id = $id2)
  AND e.created_at >= $since
RETURN e
ORDER BY e.created_at DESC
LIMIT $limit
```

### Graph Traversal

```cypher
// Find events by author
MATCH (e:Event)-[:AUTHORED_BY]->(a:Author {pubkey: $pubkey})
RETURN e

// Find followers of a user
MATCH (follower:NostrUser)-[:FOLLOWS]->(user:NostrUser {pubkey: $pubkey})
RETURN follower.pubkey

// Find mutual follows (friends)
MATCH (a:NostrUser {pubkey: $pubkeyA})-[:FOLLOWS]->(b:NostrUser)
WHERE (b)-[:FOLLOWS]->(a)
RETURN b.pubkey AS mutual_friend
```

### Upsert Pattern

```cypher
MERGE (n:Node {key: $key})
ON CREATE SET
  n.created_at = timestamp(),
  n.value = $value
ON MATCH SET
  n.updated_at = timestamp(),
  n.value = $value
RETURN n
```

### Batch Processing with UNWIND

```cypher
// Create multiple nodes from list
UNWIND $items AS item
CREATE (n:Node {id: item.id, value: item.value})

// Create relationships from list
UNWIND $follows AS followed_pubkey
MERGE (followed:NostrUser {pubkey: followed_pubkey})
MERGE (author)-[:FOLLOWS]->(followed)
```

## Performance Optimization

### Index Usage

1. **Start with indexed properties** - Begin MATCH with most selective indexed field
2. **Use composite indexes** - For queries filtering on multiple properties
3. **Profile queries** - Use `PROFILE` prefix to see execution plan

```cypher
PROFILE MATCH (e:Event {kind: 1})
WHERE e.created_at > $since
RETURN e LIMIT 100
```

### Query Optimization Tips

1. **Filter early** - Put WHERE conditions close to MATCH
2. **Limit early** - Use LIMIT as early as possible
3. **Avoid Cartesian products** - Connect patterns or use WITH
4. **Use parameters** - Enables query plan caching

```cypher
// GOOD - Filter and limit early
MATCH (e:Event)
WHERE e.kind IN $kinds AND e.created_at >= $since
WITH e ORDER BY e.created_at DESC LIMIT 100
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t:Tag)
RETURN e, collect(t)

// BAD - Late filtering
MATCH (e:Event), (t:Tag)
WHERE e.kind IN $kinds
RETURN e, t LIMIT 100
```

## Reference Materials

For detailed information, consult the reference files:

- **references/syntax-reference.md** - Complete Cypher syntax guide with all clause types, operators, and functions
- **references/common-patterns.md** - Project-specific patterns for ORLY Nostr relay including event storage, tag queries, and social graph traversals
- **references/common-mistakes.md** - Frequent Cypher errors and how to avoid them

## ORLY-Specific Patterns

This codebase uses these specific Cypher patterns:

### Event Storage Pattern

```cypher
// Create event with author relationship
MERGE (a:Author {pubkey: $pubkey})
CREATE (e:Event {
  id: $eventId,
  serial: $serial,
  kind: $kind,
  created_at: $createdAt,
  content: $content,
  sig: $sig,
  pubkey: $pubkey,
  tags: $tags
})
CREATE (e)-[:AUTHORED_BY]->(a)
```

### Tag Query Pattern

```cypher
// Query events by tag (Nostr #<tag> filter)
MATCH (e:Event)-[:TAGGED_WITH]->(t:Tag {type: $tagType})
WHERE t.value IN $tagValues
RETURN e
ORDER BY e.created_at DESC
LIMIT $limit
```

### Social Graph Pattern

```cypher
// Process contact list with diff-based updates
// Mark old as superseded
OPTIONAL MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
SET old.superseded_by = $new_event_id

// Create tracking node
CREATE (new:ProcessedSocialEvent {
  event_id: $new_event_id,
  event_kind: 3,
  pubkey: $author_pubkey,
  created_at: $created_at,
  processed_at: timestamp()
})

// Update relationships
MERGE (author:NostrUser {pubkey: $author_pubkey})
WITH author
UNWIND $added_follows AS followed_pubkey
MERGE (followed:NostrUser {pubkey: followed_pubkey})
MERGE (author)-[:FOLLOWS]->(followed)
```

## Official Resources

- Neo4j Cypher Manual: https://neo4j.com/docs/cypher-manual/current/
- Cypher Cheat Sheet: https://neo4j.com/docs/cypher-cheat-sheet/current/
- Query Tuning: https://neo4j.com/docs/cypher-manual/current/query-tuning/