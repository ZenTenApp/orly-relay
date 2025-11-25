# Neo4j Social Event Processor

A graph-native implementation for managing Nostr social relationships in Neo4j, providing Web of Trust (WoT) capabilities for the ORLY relay.

## Overview

The Social Event Processor automatically processes Nostr events that define social relationships and stores them as a navigable graph in Neo4j. This enables powerful social graph queries, trust metrics computation, and relationship-aware content filtering.

When events are saved to the relay, the processor intercepts social event types and maintains a parallel graph of `NostrUser` nodes connected by relationship edges (`FOLLOWS`, `MUTES`, `REPORTS`). This graph is separate from the standard NIP-01 event storage, optimized specifically for social graph operations.

## Supported Event Kinds

### Kind 0 - Profile Metadata

Creates or updates `NostrUser` nodes with profile information extracted from the event content:
- Display name
- About/bio
- Profile picture URL
- NIP-05 identifier
- Lightning address (lud16)

Profile updates are applied whenever a newer kind 0 event is received for a pubkey.

### Kind 3 - Contact Lists (Follows)

Manages `FOLLOWS` relationships between users using an efficient diff-based approach:
- When a new contact list arrives, the processor compares it to the previous list
- Only changed relationships are modified (added or removed)
- Unchanged follows are preserved with updated event traceability
- Older events (by timestamp) are automatically rejected

This approach minimizes graph operations for large follow lists where only a few changes occur.

### Kind 10000 - Mute Lists

Manages `MUTES` relationships using the same diff-based approach as contact lists:
- Tracks which users have muted which other users
- Supports incremental updates
- Enables mute-aware content filtering

### Kind 1984 - Reports

Creates `REPORTS` relationships with additional metadata:
- Report type (spam, illegal, impersonation, etc.)
- Accumulative - multiple reports from different users are preserved
- Enables trust/reputation scoring based on community reports

## Key Features

### Event Traceability

Every relationship in the graph is linked back to the Nostr event that created it via a `created_by_event` property. This provides:
- Full audit trail of social graph changes
- Ability to verify relationships against signed events
- Support for event deletion (future)

### Replaceable Event Handling

For replaceable event kinds (0, 3, 10000), the processor:
- Automatically rejects events older than the current state
- Marks superseded events for historical tracking
- Updates relationship pointers to the newest event

### Idempotent Operations

All graph operations are designed to be safely repeatable:
- Duplicate events don't create duplicate relationships
- Reprocessing events produces the same graph state
- Safe for use in distributed/replicated relay setups

### Integration with Event Storage

The social processor is called automatically by `SaveEvent()` for supported event kinds. No additional code is needed - simply save events normally and the social graph is maintained alongside standard event storage.

## Use Cases

### Web of Trust Queries

Find users within N degrees of separation from a trusted seed set:
- "Show me posts from people my follows follow"
- "Find users who are followed by multiple people I trust"

### Reputation Scoring

Compute trust metrics based on the social graph:
- PageRank-style influence scores
- Report-based reputation penalties
- Verified follower counts

### Content Filtering

Filter content based on social relationships:
- Only show posts from follows and their follows
- Hide content from muted users
- Flag content from reported users

### Social Graph Analysis

Analyze community structure:
- Find clusters of highly connected users
- Identify influential community members
- Detect potential sybil networks

## Testing

The implementation includes comprehensive tests covering:
- Profile metadata creation and updates
- Contact list initial creation
- Contact list incremental updates (add/remove follows)
- Older event rejection
- Mute list processing
- Report accumulation
- Final graph state verification

To run the tests:

```bash
# Start Neo4j
cd pkg/neo4j
docker-compose up -d

# Set environment variables
export ORLY_NEO4J_URI="bolt://localhost:7687"
export ORLY_NEO4J_USER="neo4j"
export ORLY_NEO4J_PASSWORD="testpass123"

# Run tests
go test -v -run TestSocialEventProcessor
```

See [TESTING.md](./TESTING.md) for detailed test documentation.

## Graph Model

The social graph consists of:

**Nodes:**
- `NostrUser` - Represents a Nostr user with their pubkey and profile data
- `ProcessedSocialEvent` - Tracks which events have been processed and their status

**Relationships:**
- `FOLLOWS` - User A follows User B
- `MUTES` - User A has muted User B
- `REPORTS` - User A has reported User B (with report type)

All relationships include properties for event traceability and timestamps.

## Configuration

The social event processor is enabled by default when using the Neo4j database backend. No additional configuration is required.

To use Neo4j as the database backend:

```bash
export ORLY_DB_TYPE=neo4j
export ORLY_NEO4J_URI=bolt://localhost:7687
export ORLY_NEO4J_USER=neo4j
export ORLY_NEO4J_PASSWORD=your_password
```

## Related Documentation

- [WOT_SPEC.md](./WOT_SPEC.md) - Complete Web of Trust data model specification
- [EVENT_PROCESSING_SPEC.md](./EVENT_PROCESSING_SPEC.md) - Detailed event processing logic
- [ADDITIONAL_REQUIREMENTS.md](./ADDITIONAL_REQUIREMENTS.md) - Future enhancements and algorithm details
- [TESTING.md](./TESTING.md) - Test documentation and troubleshooting

## Future Enhancements

Planned features include:
- GrapeRank and Personalized PageRank algorithms
- Multi-tenant trust metrics (per-user WoT views)
- Encrypted mute list support (NIP-59)
- Event deletion handling (Kind 5)
- Large-scale follow list optimization
- Trust score caching and incremental updates
