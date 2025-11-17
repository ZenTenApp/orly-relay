# Neo4j Database Backend for ORLY Relay

## Overview

The Neo4j database backend provides a graph-native storage solution for the ORLY Nostr relay. Unlike traditional key-value or document stores, Neo4j is optimized for relationship-heavy queries, making it an ideal fit for Nostr's social graph and event reference patterns.

## Architecture

### Core Components

1. **Main Database File** ([pkg/neo4j/neo4j.go](../pkg/neo4j/neo4j.go))
   - Implements the `database.Database` interface
   - Manages Neo4j driver connection and lifecycle
   - Uses Badger for metadata storage (markers, identity, subscriptions)
   - Registers with the database factory via `init()`

2. **Schema Management** ([pkg/neo4j/schema.go](../pkg/neo4j/schema.go))
   - Defines Neo4j constraints and indexes using Cypher
   - Creates unique constraints on Event IDs and Author pubkeys
   - Indexes for optimal query performance (kind, created_at, tags)

3. **Query Engine** ([pkg/neo4j/query-events.go](../pkg/neo4j/query-events.go))
   - Translates Nostr REQ filters to Cypher queries
   - Leverages graph traversal for tag relationships
   - Supports prefix matching for IDs and pubkeys
   - Parameterized queries for security and performance

4. **Event Storage** ([pkg/neo4j/save-event.go](../pkg/neo4j/save-event.go))
   - Stores events as nodes with properties
   - Creates graph relationships:
     - `AUTHORED_BY`: Event → Author
     - `REFERENCES`: Event → Event (e-tags)
     - `MENTIONS`: Event → Author (p-tags)
     - `TAGGED_WITH`: Event → Tag

## Graph Schema

### Node Types

**Event Node**
```cypher
(:Event {
  id: string,           // Hex-encoded event ID (32 bytes)
  serial: int,          // Sequential serial number
  kind: int,            // Event kind
  created_at: int,      // Unix timestamp
  content: string,      // Event content
  sig: string,          // Hex-encoded signature
  pubkey: string,       // Hex-encoded author pubkey
  tags: string          // JSON-encoded tags array
})
```

**Author Node**
```cypher
(:Author {
  pubkey: string        // Hex-encoded pubkey (unique)
})
```

**Tag Node**
```cypher
(:Tag {
  type: string,         // Tag type (e.g., "t", "d")
  value: string         // Tag value
})
```

**Marker Node** (for metadata)
```cypher
(:Marker {
  key: string,          // Unique key
  value: string         // Hex-encoded value
})
```

### Relationships

- `(:Event)-[:AUTHORED_BY]->(:Author)` - Event authorship
- `(:Event)-[:REFERENCES]->(:Event)` - Event references (e-tags)
- `(:Event)-[:MENTIONS]->(:Author)` - Author mentions (p-tags)
- `(:Event)-[:TAGGED_WITH]->(:Tag)` - Generic tag associations

## How Nostr REQ Messages Are Implemented

### Filter to Cypher Translation

The query engine in [query-events.go](../pkg/neo4j/query-events.go) translates Nostr filters to Cypher queries:

#### 1. ID Filters
```json
{"ids": ["abc123..."]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.id = $id_0
```

For prefix matching (partial IDs):
```cypher
WHERE e.id STARTS WITH $id_0
```

#### 2. Author Filters
```json
{"authors": ["pubkey1...", "pubkey2..."]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.pubkey IN $authors
```

#### 3. Kind Filters
```json
{"kinds": [1, 7]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.kind IN $kinds
```

#### 4. Time Range Filters
```json
{"since": 1234567890, "until": 1234567900}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.created_at >= $since AND e.created_at <= $until
```

#### 5. Tag Filters (Graph Advantage!)
```json
{"#t": ["bitcoin", "nostr"]}
```
Becomes:
```cypher
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t0:Tag)
WHERE t0.type = $tagType_0 AND t0.value IN $tagValues_0
```

This leverages Neo4j's native graph traversal for efficient tag queries!

#### 6. Combined Filters
```json
{
  "kinds": [1],
  "authors": ["abc..."],
  "#p": ["xyz..."],
  "limit": 50
}
```
Becomes:
```cypher
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t0:Tag)
WHERE e.kind IN $kinds
  AND e.pubkey IN $authors
  AND t0.type = $tagType_0
  AND t0.value IN $tagValues_0
RETURN e.id, e.kind, e.created_at, e.content, e.sig, e.pubkey, e.tags
ORDER BY e.created_at DESC
LIMIT $limit
```

### Query Execution Flow

1. **Parse Filter**: Extract IDs, authors, kinds, times, tags
2. **Build Cypher**: Construct parameterized query with MATCH/WHERE clauses
3. **Execute**: Run via `ExecuteRead()` with read-only session
4. **Parse Results**: Convert Neo4j records to Nostr events
5. **Return**: Send events back to client

## Configuration

### Environment Variables

```bash
# Neo4j Connection
ORLY_NEO4J_URI="bolt://localhost:7687"
ORLY_NEO4J_USER="neo4j"
ORLY_NEO4J_PASSWORD="password"

# Database Type Selection
ORLY_DB_TYPE="neo4j"

# Data Directory (for Badger metadata storage)
ORLY_DATA_DIR="~/.local/share/ORLY"
```

### Example Docker Compose Setup

```yaml
version: '3.8'
services:
  neo4j:
    image: neo4j:5.15
    ports:
      - "7474:7474"  # HTTP
      - "7687:7687"  # Bolt
    environment:
      - NEO4J_AUTH=neo4j/password
      - NEO4J_PLUGINS=["apoc"]
    volumes:
      - neo4j_data:/data
      - neo4j_logs:/logs

  orly:
    build: .
    ports:
      - "3334:3334"
    environment:
      - ORLY_DB_TYPE=neo4j
      - ORLY_NEO4J_URI=bolt://neo4j:7687
      - ORLY_NEO4J_USER=neo4j
      - ORLY_NEO4J_PASSWORD=password
    depends_on:
      - neo4j

volumes:
  neo4j_data:
  neo4j_logs:
```

## Performance Considerations

### Advantages Over Badger/DGraph

1. **Native Graph Queries**: Tag relationships and social graph traversals are native operations
2. **Optimized Indexes**: Automatic index usage for constrained properties
3. **Efficient Joins**: Relationship traversals are O(1) lookups
4. **Query Planner**: Neo4j's query planner optimizes complex multi-filter queries

### Tuning Recommendations

1. **Indexes**: The schema creates indexes for:
   - Event ID (unique constraint + index)
   - Event kind
   - Event created_at
   - Composite: kind + created_at
   - Tag type + value

2. **Cache Configuration**: Configure Neo4j's page cache and heap size:
```conf
# neo4j.conf
dbms.memory.heap.initial_size=2G
dbms.memory.heap.max_size=4G
dbms.memory.pagecache.size=4G
```

3. **Query Limits**: Always use LIMIT in queries to prevent memory exhaustion

## Implementation Details

### Replaceable Events

Replaceable events (kinds 0, 3, 10000-19999) are handled in `WouldReplaceEvent()`:

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})
WHERE e.created_at < $createdAt
RETURN e.serial, e.created_at
```

Older events are deleted before saving the new one.

### Parameterized Replaceable Events

For kinds 30000-39999, we also match on the d-tag:

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})-[:TAGGED_WITH]->(t:Tag {type: 'd', value: $dValue})
WHERE e.created_at < $createdAt
RETURN e.serial
```

### Event Deletion (NIP-09)

Delete events (kind 5) are processed via graph traversal:

```cypher
MATCH (target:Event {id: $targetId})
MATCH (delete:Event {kind: 5})-[:REFERENCES]->(target)
WHERE delete.pubkey = $pubkey OR delete.pubkey IN $admins
RETURN delete.id
```

Only same-author or admin deletions are allowed.

## Comparison with Other Backends

| Feature | Badger | DGraph | Neo4j |
|---------|--------|--------|-------|
| **Storage Type** | Key-value | Graph (distributed) | Graph (native) |
| **Query Language** | Custom indexes | DQL | Cypher |
| **Tag Queries** | Index lookups | Graph traversal | Native relationships |
| **Scaling** | Single-node | Distributed | Cluster/Causal cluster |
| **Memory Usage** | Low | Medium | High |
| **Setup Complexity** | Minimal | Medium | Medium |
| **Best For** | Small relays | Large distributed | Relationship-heavy |

## Development Guide

### Adding New Indexes

1. Update [schema.go](../pkg/neo4j/schema.go) with new index definition
2. Add to `applySchema()` function
3. Restart relay to apply schema changes

Example:
```cypher
CREATE INDEX event_content_fulltext IF NOT EXISTS
FOR (e:Event) ON (e.content)
OPTIONS {indexConfig: {`fulltext.analyzer`: 'english'}}
```

### Custom Queries

To add custom query methods:

1. Add method to [query-events.go](../pkg/neo4j/query-events.go)
2. Build Cypher query with parameterization
3. Use `ExecuteRead()` or `ExecuteWrite()` as appropriate
4. Parse results with `parseEventsFromResult()`

### Testing

Due to Neo4j dependency, tests require a running Neo4j instance:

```bash
# Start Neo4j via Docker
docker run -d --name neo4j-test \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/test \
  neo4j:5.15

# Run tests
ORLY_NEO4J_URI="bolt://localhost:7687" \
ORLY_NEO4J_USER="neo4j" \
ORLY_NEO4J_PASSWORD="test" \
go test ./pkg/neo4j/...

# Cleanup
docker rm -f neo4j-test
```

## Future Enhancements

1. **Full-text Search**: Leverage Neo4j's full-text indexes for content search
2. **Graph Analytics**: Implement social graph metrics (centrality, communities)
3. **Advanced Queries**: Support NIP-50 search via Cypher full-text capabilities
4. **Clustering**: Deploy Neo4j cluster for high availability
5. **APOC Procedures**: Utilize APOC library for advanced graph algorithms
6. **Caching Layer**: Implement query result caching similar to Badger backend

## Troubleshooting

### Connection Issues

```bash
# Test connectivity
cypher-shell -a bolt://localhost:7687 -u neo4j -p password

# Check Neo4j logs
docker logs neo4j
```

### Performance Issues

```cypher
// View query execution plan
EXPLAIN MATCH (e:Event) WHERE e.kind = 1 RETURN e LIMIT 10

// Profile query performance
PROFILE MATCH (e:Event)-[:AUTHORED_BY]->(a:Author) RETURN e, a LIMIT 10
```

### Schema Issues

```cypher
// List all constraints
SHOW CONSTRAINTS

// List all indexes
SHOW INDEXES

// Drop and recreate schema
DROP CONSTRAINT event_id_unique IF EXISTS
CREATE CONSTRAINT event_id_unique FOR (e:Event) REQUIRE e.id IS UNIQUE
```

## References

- [Neo4j Documentation](https://neo4j.com/docs/)
- [Cypher Query Language](https://neo4j.com/docs/cypher-manual/current/)
- [Neo4j Go Driver](https://neo4j.com/docs/go-manual/current/)
- [Graph Database Patterns](https://neo4j.com/developer/graph-db-vs-rdbms/)
- [Nostr Protocol (NIP-01)](https://github.com/nostr-protocol/nips/blob/master/01.md)

## License

This Neo4j backend implementation follows the same license as the ORLY relay project.
