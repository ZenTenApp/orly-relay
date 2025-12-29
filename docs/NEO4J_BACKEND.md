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

All configuration is centralized in `app/config/config.go` and visible via `./orly help`.

> **Important:** All environment variables must be defined in `app/config/config.go`. Do not use `os.Getenv()` directly in package code. Database backends receive configuration via the `database.DatabaseConfig` struct.

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

# Neo4j Driver Tuning (Memory Management)
ORLY_NEO4J_MAX_CONN_POOL=25       # Max connections (default: 25, driver default: 100)
ORLY_NEO4J_FETCH_SIZE=1000        # Records per fetch batch (default: 1000, -1=all)
ORLY_NEO4J_MAX_TX_RETRY_SEC=30    # Max transaction retry time in seconds
ORLY_NEO4J_QUERY_RESULT_LIMIT=10000  # Max results per query (0=unlimited)
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
      # Memory tuning for production
      - NEO4J_server_memory_heap_initial__size=512m
      - NEO4J_server_memory_heap_max__size=1g
      - NEO4J_server_memory_pagecache_size=512m
      # Transaction memory limits (prevent runaway queries)
      - NEO4J_dbms_memory_transaction_total__max=256m
      - NEO4J_dbms_memory_transaction_max=64m
      # Query timeout
      - NEO4J_dbms_transaction_timeout=30s
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
      # Driver tuning for memory management
      - ORLY_NEO4J_MAX_CONN_POOL=25
      - ORLY_NEO4J_FETCH_SIZE=1000
      - ORLY_NEO4J_QUERY_RESULT_LIMIT=10000
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

2. **Cache Configuration**: Configure Neo4j's page cache and heap size (see Memory Tuning below)

3. **Query Limits**: The relay automatically enforces `ORLY_NEO4J_QUERY_RESULT_LIMIT` (default: 10000) to prevent unbounded queries from exhausting memory

## Memory Tuning

Neo4j runs as a separate process (typically in Docker), so memory management involves both the relay driver settings and Neo4j server configuration.

### Understanding Memory Layers

1. **ORLY Relay Process** (~35MB RSS typical)
   - Go driver connection pool
   - Query result buffering
   - Controlled by `ORLY_NEO4J_*` environment variables

2. **Neo4j Server Process** (512MB-4GB+ depending on data)
   - JVM heap for Java objects
   - Page cache for graph data
   - Transaction memory for query execution
   - Controlled by `NEO4J_*` environment variables

### Relay Driver Tuning (ORLY side)

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEO4J_MAX_CONN_POOL` | 25 | Max connections in pool. Lower = less memory, but may bottleneck under high load. Driver default is 100. |
| `ORLY_NEO4J_FETCH_SIZE` | 1000 | Records fetched per batch. Lower = less memory per query, more round trips. Set to -1 for all (risky). |
| `ORLY_NEO4J_MAX_TX_RETRY_SEC` | 30 | Max seconds to retry failed transactions. |
| `ORLY_NEO4J_QUERY_RESULT_LIMIT` | 10000 | Hard cap on results per query. Prevents unbounded queries. Set to 0 for unlimited (not recommended). |

**Recommended settings for memory-constrained environments:**
```bash
ORLY_NEO4J_MAX_CONN_POOL=10
ORLY_NEO4J_FETCH_SIZE=500
ORLY_NEO4J_QUERY_RESULT_LIMIT=5000
```

### Neo4j Server Tuning (Docker/neo4j.conf)

**JVM Heap Memory** - For Java objects and query processing:
```bash
# Docker environment variables
NEO4J_server_memory_heap_initial__size=512m
NEO4J_server_memory_heap_max__size=1g

# neo4j.conf equivalent
server.memory.heap.initial_size=512m
server.memory.heap.max_size=1g
```

**Page Cache** - For caching graph data from disk:
```bash
# Docker
NEO4J_server_memory_pagecache_size=512m

# neo4j.conf
server.memory.pagecache.size=512m
```

**Transaction Memory Limits** - Prevent runaway queries:
```bash
# Docker
NEO4J_dbms_memory_transaction_total__max=256m   # Global limit across all transactions
NEO4J_dbms_memory_transaction_max=64m           # Per-transaction limit

# neo4j.conf
dbms.memory.transaction.total.max=256m
db.memory.transaction.max=64m
```

**Query Timeout** - Kill long-running queries:
```bash
# Docker
NEO4J_dbms_transaction_timeout=30s

# neo4j.conf
dbms.transaction.timeout=30s
```

### Memory Sizing Guidelines

| Deployment Size | Heap | Page Cache | Total Neo4j | ORLY Pool |
|-----------------|------|------------|-------------|-----------|
| Development | 512m | 256m | ~1GB | 10 |
| Small relay (<100k events) | 1g | 512m | ~2GB | 25 |
| Medium relay (<1M events) | 2g | 1g | ~4GB | 50 |
| Large relay (>1M events) | 4g | 2g | ~8GB | 100 |

**Formula for Page Cache:**
```
Page Cache = Data Size on Disk × 1.2
```

Use `neo4j-admin server memory-recommendation` inside the container to get tailored recommendations.

### Monitoring Memory Usage

**Check Neo4j memory from relay logs:**
```bash
# Driver config is logged at startup
grep "connecting to neo4j" /path/to/orly.log
# Output: connecting to neo4j at bolt://... (pool=25, fetch=1000, txRetry=30s)
```

**Check Neo4j server memory:**
```bash
# Inside Neo4j container
docker exec neo4j neo4j-admin server memory-recommendation

# Or query via Cypher
CALL dbms.listPools() YIELD pool, heapMemoryUsed, heapMemoryUsedBytes
RETURN pool, heapMemoryUsed
```

**Monitor transaction memory:**
```cypher
CALL dbms.listTransactions()
YIELD transactionId, currentQuery, allocatedBytes
RETURN transactionId, currentQuery, allocatedBytes
ORDER BY allocatedBytes DESC
```

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
