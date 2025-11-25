# Event-Driven Vertex Management Implementation Summary

## What Was Implemented

This document summarizes the event-driven vertex management system for maintaining NostrUser nodes and social graph relationships in the ORLY Neo4j backend.

## Architecture Overview

### Dual Node System

The implementation uses two separate node types:

1. **Event and Author nodes** (NIP-01 Support)
   - Created by `SaveEvent()` for ALL events
   - Used for standard Nostr REQ filter queries
   - Supports kinds, authors, tags, time ranges, etc.
   - **Always maintained** - this is the core relay functionality

2. **NostrUser nodes** (Social Graph / WoT)
   - Created by `SocialEventProcessor` for kinds 0, 3, 1984, 10000
   - Used for social graph queries and Web of Trust metrics
   - Connected by FOLLOWS, MUTES, REPORTS relationships
   - **Event traceable** - every relationship links back to the event that created it

This dual approach ensures **NIP-01 queries continue to work** while adding social graph capabilities.

## Files Created/Modified

### New Files

1. **[EVENT_PROCESSING_SPEC.md](./EVENT_PROCESSING_SPEC.md)** (120 KB)
   - Complete specification for event-driven vertex management
   - Processing workflows for each event kind
   - Diff computation algorithms
   - Cypher query patterns
   - Implementation architecture with Go code examples
   - Testing strategy and performance considerations

2. **[social-event-processor.go](./social-event-processor.go)** (18 KB)
   - `SocialEventProcessor` struct and methods
   - `processContactList()` - handles kind 3 (follows)
   - `processMuteList()` - handles kind 10000 (mutes)
   - `processReport()` - handles kind 1984 (reports)
   - `processProfileMetadata()` - handles kind 0 (profiles)
   - Helper functions for diff computation and p-tag extraction
   - Batch processing support

3. **[WOT_SPEC.md](./WOT_SPEC.md)** (40 KB)
   - Web of Trust data model specification
   - Trust metrics definitions (GrapeRank, PageRank, etc.)
   - Deployment modes (lean vs. full)
   - Example Cypher queries

4. **[ADDITIONAL_REQUIREMENTS.md](./ADDITIONAL_REQUIREMENTS.md)** (30 KB)
   - 50+ missing implementation details identified
   - Organized into 12 categories
   - Phased implementation roadmap
   - Research needed items flagged

### Modified Files

1. **[schema.go](./schema.go)**
   - Added `ProcessedSocialEvent` node constraint
   - Added indexes for social event processing
   - Maintained all existing NIP-01 schema (Event, Author, Tag, Marker)
   - Added WoT schema (NostrUser, WoT metrics nodes)
   - Updated `dropAll()` to handle new schema elements

2. **[save-event.go](./save-event.go)**
   - Integrated `SocialEventProcessor` call after base event save
   - Social processing is **non-blocking** (errors logged but don't fail save)
   - Processes kinds 0, 3, 1984, 10000 automatically
   - NIP-01 functionality preserved

3. **[README.md](./README.md)**
   - Added WoT features to feature list
   - Documented new specification files
   - Added file structure section

## Data Model

### Node Types

#### ProcessedSocialEvent (Tracking Node)
```cypher
(:ProcessedSocialEvent {
  event_id: string,           // Hex event ID
  event_kind: int,            // 0, 3, 1984, or 10000
  pubkey: string,             // Author pubkey
  created_at: timestamp,      // Event timestamp
  processed_at: timestamp,    // When relay processed it
  relationship_count: int,    // How many relationships created
  superseded_by: string|null  // If replaced by newer event
})
```

#### NostrUser (Social Graph Vertex)
```cypher
(:NostrUser {
  pubkey: string,             // Hex pubkey (unique)
  npub: string,               // Bech32 npub
  name: string,               // Profile name
  about: string,              // Profile about
  picture: string,            // Profile picture URL
  nip05: string,              // NIP-05 identifier
  // ... other profile fields
  // ... trust metrics (future)
})
```

### Relationship Types (With Event Traceability)

#### FOLLOWS
```cypher
(:NostrUser)-[:FOLLOWS {
  created_by_event: string,   // Event ID that created this
  created_at: timestamp,      // Event timestamp
  relay_received_at: timestamp
}]->(:NostrUser)
```

#### MUTES
```cypher
(:NostrUser)-[:MUTES {
  created_by_event: string,
  created_at: timestamp,
  relay_received_at: timestamp
}]->(:NostrUser)
```

#### REPORTS
```cypher
(:NostrUser)-[:REPORTS {
  created_by_event: string,
  created_at: timestamp,
  relay_received_at: timestamp,
  report_type: string         // NIP-56 type (spam, illegal, etc.)
}]->(:NostrUser)
```

## Event Processing Flow

### Kind 3 (Contact List) - Follow Relationships

```
1. Receive kind 3 event
2. Check if event already exists (base Event node)
   └─ If exists: return (already processed)
3. Save base Event + Author nodes (NIP-01)
4. Social processing:
   a. Check for existing kind 3 from this pubkey
   b. If new event is older: skip (don't replace newer with older)
   c. Extract p-tags from new event → new_follows[]
   d. Query old FOLLOWS relationships → old_follows[]
   e. Compute diff:
      - added_follows = new_follows - old_follows
      - removed_follows = old_follows - new_follows
   f. Transaction:
      - Mark old ProcessedSocialEvent as superseded
      - Create new ProcessedSocialEvent
      - DELETE removed FOLLOWS relationships
      - CREATE added FOLLOWS relationships
   g. Log: "processed contact list: added=X, removed=Y"
```

**Example:**
```
Event 1: Alice follows [Bob, Charlie]
  → Creates FOLLOWS to Bob and Charlie

Event 2: Alice follows [Bob, Dave] (newer)
  → Diff: added=[Dave], removed=[Charlie]
  → Marks Event 1 as superseded
  → Deletes FOLLOWS to Charlie
  → Creates FOLLOWS to Dave
  → Result: Alice follows [Bob, Dave]
```

### Kind 10000 (Mute List) - Mute Relationships

Same pattern as kind 3, but creates/updates MUTES relationships.

### Kind 1984 (Reporting) - Report Relationships

**Different:** Kind 1984 is NOT replaceable.

```
1. Receive kind 1984 event
2. Save base Event + Author nodes
3. Social processing:
   a. Extract p-tag (reported user) and report type
   b. Create ProcessedSocialEvent node
   c. Create REPORTS relationship (new, not replacing old)
```

**Multiple Reports:** Same user can report same target multiple times. Each creates a separate REPORTS relationship.

### Kind 0 (Profile Metadata) - User Properties

```
1. Receive kind 0 event
2. Save base Event + Author nodes
3. Social processing:
   a. Parse JSON content
   b. MERGE NostrUser (create if not exists)
   c. SET profile properties (name, about, picture, etc.)
```

**Note:** Kind 0 is replaceable, but we don't diff - just update all profile fields.

## Key Design Decisions

### 1. Event Traceability

**All relationships include `created_by_event` property.**

This enables:
- Diff-based updates (know which relationships to remove when event is replaced)
- Event deletion (remove associated relationships)
- Auditing (track provenance of all data)
- Temporal queries (state of graph at specific time)

### 2. Separate Event Tracking Node (ProcessedSocialEvent)

Instead of marking Event nodes as processed, we use a separate ProcessedSocialEvent node.

**Why?**
- Event node represents the raw Nostr event (immutable)
- ProcessedSocialEvent tracks our processing state (mutable)
- Can have multiple processing states for same event (future: different algorithms)
- Clean separation of concerns

### 3. Superseded Chain

When a replaceable event is updated:
- Old ProcessedSocialEvent.superseded_by = new_event_id
- New ProcessedSocialEvent.superseded_by = null

This creates a chain: Event1 → Event2 → Event3 (current)

**Benefits:**
- Can query history of user's follows over time
- Can reconstruct graph at any point in time
- Can debug issues ("why was this relationship removed?")

### 4. Non-Blocking Social Processing

Social event processing errors are logged but don't fail the base event save.

**Rationale:**
- NIP-01 queries are the core relay function
- Social graph is supplementary (for WoT, filtering, etc.)
- Relay should continue operating even if social processing fails
- Can reprocess events later if social processing was fixed

### 5. Dual Node System (Event/Author vs. NostrUser)

**Why not merge Author and NostrUser?**
- Event and Author are for NIP-01 queries (fast, simple)
- NostrUser is for social graph (complex relationships)
- Different query patterns and optimization strategies
- Keeps social graph overhead separate from core relay performance
- Future: May merge, but keep separate for now (pre-alpha)

## Query Examples

### Get Current Follows for User

```cypher
MATCH (user:NostrUser {pubkey: $pubkey})-[f:FOLLOWS]->(followed:NostrUser)
WHERE NOT EXISTS {
  MATCH (old:ProcessedSocialEvent {event_id: f.created_by_event})
  WHERE old.superseded_by IS NOT NULL
}
RETURN followed.pubkey, followed.name
```

### Get Mutual Follows (Friends)

```cypher
MATCH (user:NostrUser {pubkey: $pubkey})-[:FOLLOWS]->(friend:NostrUser)
-[:FOLLOWS]->(user)
RETURN friend.pubkey, friend.name
```

### Count Reports by Type

```cypher
MATCH (reported:NostrUser {pubkey: $pubkey})<-[r:REPORTS]-()
RETURN r.report_type, count(*) as count
ORDER BY count DESC
```

### Follow Graph History

```cypher
MATCH (evt:ProcessedSocialEvent {pubkey: $pubkey, event_kind: 3})
RETURN evt.event_id, evt.created_at, evt.relationship_count, evt.superseded_by
ORDER BY evt.created_at ASC
```

## Testing Plan

### Unit Tests

1. **Diff computation**
   - Empty lists
   - All new, all removed
   - Mixed additions and removals
   - Duplicate handling

2. **Event ordering**
   - Newer replaces older
   - Older rejected
   - Same timestamp

3. **P-tag extraction**
   - Valid tags
   - Invalid pubkeys
   - Empty lists

### Integration Tests

1. **Contact list updates**
   - Initial list
   - Add follows
   - Remove follows
   - Replace entire list
   - Empty list (unfollow all)

2. **Multiple users**
   - Alice follows Bob
   - Bob follows Charlie
   - Verify independent updates

3. **Replaceable event semantics**
   - Older event arrives after newer
   - Verify rejection

4. **Report accumulation**
   - Multiple reports from same user
   - Multiple users reporting same target
   - Different report types

### Performance Tests

1. **Large contact lists**
   - 1000+ follows
   - Diff computation time
   - Transaction time

2. **High volume**
   - 1000 events/sec
   - Graph write throughput
   - Query performance

## Next Steps

### Phase 1: Basic Testing (Current)
- [ ] Unit tests for social-event-processor.go
- [ ] Integration tests with live Neo4j instance
- [ ] Test with real Nostr events

### Phase 2: Optimization
- [ ] Batch processing for initial sync
- [ ] Index tuning based on query patterns
- [ ] Transaction optimization

### Phase 3: Trust Metrics (Future)
- [ ] Implement hops calculation (shortest path)
- [ ] Research/implement GrapeRank algorithm
- [ ] Implement Personalized PageRank
- [ ] Compute and store trust metrics

### Phase 4: Query Extensions (Future)
- [ ] REQ filter extensions (max_hops, min_influence)
- [ ] Trust metrics query API
- [ ] Multi-tenant support

## Configuration

Currently **no configuration needed** - social event processing is automatic for kinds 0, 3, 1984, 10000.

Future configuration options:
```bash
# Enable/disable social graph processing
ORLY_SOCIAL_GRAPH_ENABLED=true

# Which event kinds to process
ORLY_SOCIAL_GRAPH_KINDS=0,3,1984,10000

# Batch size for initial sync
ORLY_SOCIAL_GRAPH_BATCH_SIZE=1000

# Enable WoT metrics computation (future)
ORLY_WOT_ENABLED=false
```

## Monitoring

### Metrics to Track

- Events processed per second (by kind)
- Average diff size (added/removed counts)
- Transaction durations
- Error rates by error type
- Graph size (node/relationship counts)

### Logs to Monitor

```
INFO: processed contact list: author=abc123, added=5, removed=2, total=50
INFO: processed mute list: author=abc123, added=1, removed=0
INFO: processed report: reporter=abc123, reported=def456, type=spam
ERROR: failed to process social event kind 3: <error details>
```

## Known Limitations

1. **No event deletion support yet**
   - Kind 5 (event deletion) not implemented
   - Relationships persist even if event deleted

2. **No encrypted tag support**
   - Kind 10000 can have encrypted tags (private mutes)
   - Currently only processes public tags

3. **No temporal queries yet**
   - Can't query "who did Alice follow on 2024-01-01?"
   - Superseded chain exists but query API not implemented

4. **No batch import tool**
   - Processing events one at a time
   - Need tool for initial sync from existing relay

5. **No trust metrics computation**
   - NostrUser nodes created but metrics not calculated
   - Requires Phase 3 implementation

## Performance Characteristics

### Expected Performance

- **Small contact lists** (< 100 follows): < 100ms per event
- **Medium contact lists** (100-500 follows): 100-500ms per event
- **Large contact lists** (500-1000 follows): 500-1000ms per event
- **Very large contact lists** (> 1000 follows): May need optimization

### Bottlenecks

1. **Diff computation** - O(n) where n = max(old_size, new_size)
2. **Graph writes** - Neo4j transaction overhead
3. **Multiple network round trips** - Could batch queries

### Optimization Opportunities

1. Use APOC procedures for batch operations
2. Cache frequently accessed user data
3. Parallelize independent social event processing
4. Use Neo4j Graph Data Science library for trust metrics

## Summary

This implementation provides a **solid foundation** for social graph management in the ORLY Neo4j backend:

✅ **Event traceability** - All relationships link to source events
✅ **Diff-based updates** - Efficient replaceable event handling
✅ **NIP-01 compatible** - Standard queries still work
✅ **Extensible** - Ready for trust metrics computation
✅ **Well-documented** - Comprehensive specs and code comments

The system is **ready for testing and deployment** in a pre-alpha environment.
