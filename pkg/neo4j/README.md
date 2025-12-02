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

All Neo4j configuration is defined in `app/config/config.go` and visible via `./orly help`:

```bash
export ORLY_DB_TYPE=neo4j
export ORLY_NEO4J_URI=bolt://localhost:7687
export ORLY_NEO4J_USER=neo4j
export ORLY_NEO4J_PASSWORD=password
```

> **Note:** Configuration is centralized in `app/config/config.go`. Do not use `os.Getenv()` directly in package code - all environment variables should be passed via the `database.DatabaseConfig` struct.

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
- **Web of Trust (WoT) Extensions**: Optional support for trust metrics, social graph analysis, and content filtering (see [WOT_SPEC.md](./WOT_SPEC.md))

## Architecture

See [docs/NEO4J_BACKEND.md](../../docs/NEO4J_BACKEND.md) for comprehensive documentation on:
- Graph schema design
- How Nostr REQ messages are implemented in Cypher
- Performance tuning
- Development guide
- Comparison with other backends

### Web of Trust (WoT) Extensions

This package includes schema support for Web of Trust trust metrics computation:

- **[WOT_SPEC.md](./WOT_SPEC.md)** - Complete specification of the WoT data model, based on the [Brainstorm prototype](https://github.com/Pretty-Good-Freedom-Tech/brainstorm)
  - NostrUser nodes with trust metrics (influence, PageRank, verified counts)
  - NostrUserWotMetricsCard nodes for personalized multi-tenant metrics
  - Social graph relationships (FOLLOWS, MUTES, REPORTS)
  - Cypher schema definitions and example queries

- **[ADDITIONAL_REQUIREMENTS.md](./ADDITIONAL_REQUIREMENTS.md)** - Implementation requirements and missing details
  - Algorithm implementations (GrapeRank, Personalized PageRank)
  - Event processing logic for kinds 0, 3, 1984, 10000
  - Multi-tenant architecture and configuration
  - Performance considerations and deployment modes

**Note:** The WoT schema is applied automatically but WoT features are not yet fully implemented. See ADDITIONAL_REQUIREMENTS.md for the roadmap.

## File Structure

### Core Implementation
- `neo4j.go` - Main database implementation
- `schema.go` - Graph schema and index definitions (includes WoT extensions)
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

### Documentation
- `README.md` - This file
- `WOT_SPEC.md` - Web of Trust data model specification
- `ADDITIONAL_REQUIREMENTS.md` - WoT implementation requirements and gaps
- `EVENT_PROCESSING_SPEC.md` - Event-driven vertex management specification
- `IMPLEMENTATION_SUMMARY.md` - Implementation overview and status
- `TESTING.md` - Test guide and troubleshooting
- `The Brainstorm prototype_ Neo4j Data Model.html` - Original Brainstorm specification document

### Tests
- `social-event-processor_test.go` - Comprehensive tests for kinds 0, 3, 1984, 10000

## Testing

### Quick Start

```bash
# Start Neo4j using docker-compose
cd pkg/neo4j
docker-compose up -d

# Wait for Neo4j to be ready (~30 seconds)
docker-compose logs -f neo4j  # Look for "Started."

# Set Neo4j connection
export ORLY_NEO4J_URI="bolt://localhost:7687"
export ORLY_NEO4J_USER="neo4j"
export ORLY_NEO4J_PASSWORD="testpass123"

# Run all tests
go test -v

# Run social event processor tests
go test -v -run TestSocialEventProcessor

# Cleanup
docker-compose down -v
```

### Test Coverage

The `social-event-processor_test.go` file contains comprehensive tests for:
- **Kind 0**: Profile metadata processing
- **Kind 3**: Contact list creation and diff-based updates
- **Kind 1984**: Report processing (multiple reports, different types)
- **Kind 10000**: Mute list processing
- **Event traceability**: Verifies all relationships link to source events
- **Graph state**: Validates final graph matches expected state
- **Helper functions**: Unit tests for diff computation and p-tag extraction

See [TESTING.md](./TESTING.md) for detailed test documentation, troubleshooting, and how to view the graph in Neo4j Browser.

### Viewing Test Results

After running tests, explore the graph at http://localhost:7474:

```cypher
// View all social relationships
MATCH path = (u1:NostrUser)-[r:FOLLOWS|MUTES|REPORTS]->(u2:NostrUser)
RETURN path

// View event processing history
MATCH (evt:ProcessedSocialEvent)
RETURN evt ORDER BY evt.created_at
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
