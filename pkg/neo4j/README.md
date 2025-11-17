# Neo4j Database Backend

A graph database backend implementation for the ORLY Nostr relay using Neo4j.

## Quick Start

### 1. Start Neo4j

```bash
docker run -d --name neo4j \
  -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:5.15
```

### 2. Configure Environment

```bash
export ORLY_DB_TYPE=neo4j
export ORLY_NEO4J_URI=bolt://localhost:7687
export ORLY_NEO4J_USER=neo4j
export ORLY_NEO4J_PASSWORD=password
```

### 3. Run ORLY

```bash
./orly
```

## Features

- **Graph-Native Storage**: Events, authors, and tags stored as nodes and relationships
- **Efficient Queries**: Leverages Neo4j's native graph traversal for tag and social graph queries
- **Cypher Query Language**: Powerful, expressive query language for complex filters
- **Automatic Indexing**: Unique constraints and indexes for optimal performance
- **Relationship Queries**: Native support for event references, mentions, and tags

## Architecture

See [docs/NEO4J_BACKEND.md](../../docs/NEO4J_BACKEND.md) for comprehensive documentation on:
- Graph schema design
- How Nostr REQ messages are implemented in Cypher
- Performance tuning
- Development guide
- Comparison with other backends

## File Structure

- `neo4j.go` - Main database implementation
- `schema.go` - Graph schema and index definitions
- `query-events.go` - REQ filter to Cypher translation
- `save-event.go` - Event storage with relationship creation
- `fetch-event.go` - Event retrieval by serial/ID
- `serial.go` - Serial number management
- `markers.go` - Metadata key-value storage
- `identity.go` - Relay identity management
- `delete.go` - Event deletion (NIP-09)
- `subscriptions.go` - Subscription management
- `nip43.go` - Invite-based ACL (NIP-43)
- `import-export.go` - Event import/export
- `logger.go` - Logging infrastructure

## Testing

```bash
# Start Neo4j test instance
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

## Example Cypher Queries

### Find all events by an author
```cypher
MATCH (e:Event {pubkey: "abc123..."})
RETURN e
ORDER BY e.created_at DESC
```

### Find events with specific tags
```cypher
MATCH (e:Event)-[:TAGGED_WITH]->(t:Tag {type: "t", value: "bitcoin"})
RETURN e
```

### Social graph query
```cypher
MATCH (author:Author {pubkey: "abc123..."})
<-[:AUTHORED_BY]-(e:Event)
-[:MENTIONS]->(mentioned:Author)
RETURN author, e, mentioned
```

## Performance Tips

1. **Use Limits**: Always include LIMIT in queries
2. **Index Usage**: Ensure queries use indexed properties (id, kind, created_at)
3. **Parameterize**: Use parameterized queries to enable query plan caching
4. **Monitor**: Use `EXPLAIN` and `PROFILE` to analyze query performance

## Limitations

- Requires external Neo4j database (not embedded)
- Higher memory usage compared to Badger
- Metadata still uses Badger (markers, subscriptions)
- More complex deployment than single-binary solutions

## Why Neo4j for Nostr?

Nostr is inherently a social graph with heavy relationship queries:
- Event references (e-tags) → Graph edges
- Author mentions (p-tags) → Graph edges
- Follow relationships → Graph structure
- Thread traversal → Path queries

Neo4j excels at these patterns, making it a natural fit for relationship-heavy Nostr queries.

## License

Same as ORLY relay project.
