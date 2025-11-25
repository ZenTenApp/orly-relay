# Event-Driven Vertex Management Specification

This document specifies how Nostr events (specifically kind 0, 3, 1984, and 10000) are processed to maintain NostrUser vertices and social graph relationships (FOLLOWS, MUTES, REPORTS) in Neo4j, with full event traceability for diff-based updates.

## Overview

The event processing system must:
1. **Capture** Nostr events that define social relationships
2. **Generate/Update** NostrUser vertices from these events
3. **Trace** relationships back to their source events
4. **Diff** old vs new events to update the graph correctly
5. **Handle** replaceable event semantics (newer replaces older)

## Core Principle: Event Traceability

Every relationship in the graph must be traceable to the event that created it. This enables:
- **Diff-based updates**: When a replaceable event is updated, compare old vs new to determine which relationships to add/remove
- **Event deletion**: When an event is deleted, remove associated relationships
- **Auditing**: Track provenance of all social graph data
- **Temporal queries**: Query the state of the graph at a specific point in time

## Data Model Extensions

### Relationship Properties for Traceability

All social graph relationships must include these properties:

```cypher
// FOLLOWS relationship properties
(:NostrUser)-[:FOLLOWS {
  created_by_event: "event_id_hex",      // Event ID that created this relationship
  created_at: timestamp,                  // Event created_at timestamp
  relay_received_at: timestamp            // When relay received the event
}]->(:NostrUser)

// MUTES relationship properties
(:NostrUser)-[:MUTES {
  created_by_event: "event_id_hex",
  created_at: timestamp,
  relay_received_at: timestamp
}]->(:NostrUser)

// REPORTS relationship properties
(:NostrUser)-[:REPORTS {
  created_by_event: "event_id_hex",
  created_at: timestamp,
  relay_received_at: timestamp,
  report_type: "spam|impersonation|illegal|..."  // NIP-56 report type
}]->(:NostrUser)
```

### Event Tracking Node

Create a node to track which events have been processed and what relationships they created:

```cypher
(:ProcessedSocialEvent {
  event_id: "hex_id",                     // Event ID
  event_kind: 3|1984|10000,               // Event kind
  pubkey: "author_pubkey",                // Event author
  created_at: timestamp,                  // Event timestamp
  processed_at: timestamp,                // When we processed it
  relationship_count: integer,            // How many relationships created
  superseded_by: "newer_event_id"|null   // If replaced by newer event
})
```

## Event Processing Workflows

### Kind 3 (Contact List / Follows)

**Event Structure:**
```json
{
  "kind": 3,
  "pubkey": "user_pubkey",
  "created_at": 1234567890,
  "tags": [
    ["p", "followed_pubkey_1", "relay_hint", "petname"],
    ["p", "followed_pubkey_2", "relay_hint", "petname"],
    ...
  ]
}
```

**Processing Steps:**

1. **Check if replaceable event already exists**
   ```cypher
   MATCH (existing:ProcessedSocialEvent {
     event_kind: 3,
     pubkey: $pubkey
   })
   WHERE existing.superseded_by IS NULL
   RETURN existing
   ```

2. **If existing event found and new event is older**: Reject the event
   ```
   if existing.created_at >= new_event.created_at:
       return EventRejected("Older event")
   ```

3. **Extract p-tags from new event**
   ```
   new_follows = set(tag[1] for tag in tags if tag[0] == 'p')
   ```

4. **If replacing existing event, get old follows**
   ```cypher
   MATCH (author:NostrUser {pubkey: $pubkey})-[r:FOLLOWS]->()
   WHERE r.created_by_event = $existing_event_id
   RETURN collect(endNode(r).pubkey) as old_follows
   ```

5. **Compute diff**
   ```python
   added_follows = new_follows - old_follows
   removed_follows = old_follows - new_follows
   ```

6. **Transaction: Update graph atomically**
   ```cypher
   // Begin transaction

   // A. Mark old event as superseded
   MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
   SET old.superseded_by = $new_event_id

   // B. Create new event tracking node
   CREATE (new:ProcessedSocialEvent {
     event_id: $new_event_id,
     event_kind: 3,
     pubkey: $pubkey,
     created_at: $created_at,
     processed_at: timestamp(),
     relationship_count: $new_follows_count,
     superseded_by: null
   })

   // C. Remove old FOLLOWS relationships
   MATCH (author:NostrUser {pubkey: $pubkey})-[r:FOLLOWS]->(followed:NostrUser)
   WHERE r.created_by_event = $old_event_id
     AND followed.pubkey IN $removed_follows
   DELETE r

   // D. Create new FOLLOWS relationships
   MERGE (author:NostrUser {pubkey: $pubkey})
   WITH author
   UNWIND $added_follows AS followed_pubkey
   MERGE (followed:NostrUser {pubkey: followed_pubkey})
   CREATE (author)-[:FOLLOWS {
     created_by_event: $new_event_id,
     created_at: $created_at,
     relay_received_at: $now
   }]->(followed)

   // Commit transaction
   ```

**Edge Cases:**
- **Empty contact list**: User unfollows everyone (remove all FOLLOWS relationships)
- **First contact list**: No existing event, create all relationships
- **Duplicate p-tags**: Deduplicate before processing
- **Invalid pubkeys**: Skip malformed pubkeys, log warning

### Kind 10000 (Mute List)

**Event Structure:**
```json
{
  "kind": 10000,
  "pubkey": "user_pubkey",
  "created_at": 1234567890,
  "tags": [
    ["p", "muted_pubkey_1"],
    ["p", "muted_pubkey_2"],
    ...
  ]
}
```

**Processing Steps:**

Same pattern as kind 3, but with MUTES relationships:

1. Check for existing kind 10000 from this pubkey
2. If new event is older, reject
3. Extract p-tags to get new mutes list
4. Get old mutes list (if replacing)
5. Compute diff: `added_mutes`, `removed_mutes`
6. Transaction:
   - Mark old event as superseded
   - Create new ProcessedSocialEvent node
   - Delete removed MUTES relationships
   - Create added MUTES relationships

**Note on Privacy:**
- Kind 10000 supports both public and encrypted tags
- For encrypted tags, relationship tracking is limited (can't see who is muted)
- Consider: Store relationship but set `muted_pubkey = "encrypted"` placeholder

### Kind 1984 (Reporting)

**Event Structure:**
```json
{
  "kind": 1984,
  "pubkey": "reporter_pubkey",
  "created_at": 1234567890,
  "tags": [
    ["p", "reported_pubkey", "report_type"]
  ],
  "content": "Optional reason"
}
```

**Processing Steps:**

Kind 1984 is **NOT replaceable**, so each report creates a separate relationship:

1. **Extract report data**
   ```python
   for tag in tags:
       if tag[0] == 'p':
           reported_pubkey = tag[1]
           report_type = tag[2] if len(tag) > 2 else "other"
   ```

2. **Create REPORTS relationship**
   ```cypher
   // Transaction

   // Create event tracking node
   CREATE (evt:ProcessedSocialEvent {
     event_id: $event_id,
     event_kind: 1984,
     pubkey: $reporter_pubkey,
     created_at: $created_at,
     processed_at: timestamp(),
     relationship_count: 1,
     superseded_by: null
   })

   // Create or get reporter and reported users
   MERGE (reporter:NostrUser {pubkey: $reporter_pubkey})
   MERGE (reported:NostrUser {pubkey: $reported_pubkey})

   // Create REPORTS relationship
   CREATE (reporter)-[:REPORTS {
     created_by_event: $event_id,
     created_at: $created_at,
     relay_received_at: timestamp(),
     report_type: $report_type
   }]->(reported)
   ```

**Multiple Reports:**
- Same user can report same target multiple times (different events)
- Each creates a separate REPORTS relationship
- Query aggregation needed to count total reports

**Report Types (NIP-56):**
- `nudity` - Depictions of nudity, porn, etc
- `profanity` - Profanity, hateful speech, etc
- `illegal` - Illegal content
- `spam` - Spam
- `impersonation` - Someone pretending to be someone else
- `malware` - Links to malware
- `other` - Other reasons

### Kind 0 (Profile Metadata)

**Event Structure:**
```json
{
  "kind": 0,
  "pubkey": "user_pubkey",
  "created_at": 1234567890,
  "content": "{\"name\":\"Alice\",\"about\":\"...\",\"picture\":\"...\"}"
}
```

**Processing Steps:**

1. **Parse profile JSON**
   ```python
   profile = json.loads(event.content)
   ```

2. **Update NostrUser properties**
   ```cypher
   MERGE (user:NostrUser {pubkey: $pubkey})
   ON CREATE SET
     user.created_at = $now,
     user.first_seen_event = $event_id
   ON MATCH SET
     user.last_profile_update = $created_at
   SET
     user.name = $profile.name,
     user.about = $profile.about,
     user.picture = $profile.picture,
     user.nip05 = $profile.nip05,
     user.lud16 = $profile.lud16,
     user.display_name = $profile.display_name
   ```

**Note:** Kind 0 is replaceable, but we typically keep only latest profile data (not diffing relationships).

## Implementation Architecture

### Event Processor Interface

```go
package neo4j

import (
    "context"
    "git.mleku.dev/mleku/nostr/encoders/event"
)

// SocialEventProcessor handles kind 0, 3, 1984, 10000 events
type SocialEventProcessor struct {
    db *N
}

// ProcessSocialEvent routes events to appropriate handlers
func (p *SocialEventProcessor) ProcessSocialEvent(ctx context.Context, ev *event.E) error {
    switch ev.Kind {
    case 0:
        return p.processProfileMetadata(ctx, ev)
    case 3:
        return p.processContactList(ctx, ev)
    case 1984:
        return p.processReport(ctx, ev)
    case 10000:
        return p.processMuteList(ctx, ev)
    default:
        return fmt.Errorf("unsupported social event kind: %d", ev.Kind)
    }
}
```

### Contact List Processor (Kind 3)

```go
func (p *SocialEventProcessor) processContactList(ctx context.Context, ev *event.E) error {
    authorPubkey := hex.Enc(ev.Pubkey[:])
    eventID := hex.Enc(ev.ID[:])

    // 1. Check for existing contact list
    existingEvent, err := p.getLatestSocialEvent(ctx, authorPubkey, 3)
    if err != nil {
        return err
    }

    // 2. Reject if older
    if existingEvent != nil && existingEvent.CreatedAt >= ev.CreatedAt {
        return fmt.Errorf("older contact list event rejected")
    }

    // 3. Extract p-tags
    newFollows := extractPTags(ev)

    // 4. Get old follows (if replacing)
    var oldFollows []string
    if existingEvent != nil {
        oldFollows, err = p.getFollowsForEvent(ctx, existingEvent.EventID)
        if err != nil {
            return err
        }
    }

    // 5. Compute diff
    added, removed := diffStringSlices(oldFollows, newFollows)

    // 6. Update graph in transaction
    return p.updateContactListGraph(ctx, UpdateContactListParams{
        AuthorPubkey:   authorPubkey,
        NewEventID:     eventID,
        OldEventID:     existingEvent.EventID,
        CreatedAt:      ev.CreatedAt,
        AddedFollows:   added,
        RemovedFollows: removed,
    })
}

type UpdateContactListParams struct {
    AuthorPubkey   string
    NewEventID     string
    OldEventID     string
    CreatedAt      int64
    AddedFollows   []string
    RemovedFollows []string
}

func (p *SocialEventProcessor) updateContactListGraph(ctx context.Context, params UpdateContactListParams) error {
    // Build complex Cypher transaction
    cypher := `
        // Mark old event as superseded (if exists)
        OPTIONAL MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
        SET old.superseded_by = $new_event_id

        // Create new event tracking node
        CREATE (new:ProcessedSocialEvent {
            event_id: $new_event_id,
            event_kind: 3,
            pubkey: $author_pubkey,
            created_at: $created_at,
            processed_at: timestamp(),
            relationship_count: $follows_count,
            superseded_by: null
        })

        // Get or create author node
        MERGE (author:NostrUser {pubkey: $author_pubkey})

        // Remove old FOLLOWS relationships
        WITH author
        OPTIONAL MATCH (author)-[old_follows:FOLLOWS]->(followed:NostrUser)
        WHERE old_follows.created_by_event = $old_event_id
          AND followed.pubkey IN $removed_follows
        DELETE old_follows

        // Create new FOLLOWS relationships
        WITH author
        UNWIND $added_follows AS followed_pubkey
        MERGE (followed:NostrUser {pubkey: followed_pubkey})
        CREATE (author)-[:FOLLOWS {
            created_by_event: $new_event_id,
            created_at: $created_at,
            relay_received_at: timestamp()
        }]->(followed)
    `

    cypherParams := map[string]any{
        "author_pubkey":    params.AuthorPubkey,
        "new_event_id":     params.NewEventID,
        "old_event_id":     params.OldEventID,
        "created_at":       params.CreatedAt,
        "follows_count":    len(params.AddedFollows) + len(params.RemovedFollows),
        "added_follows":    params.AddedFollows,
        "removed_follows":  params.RemovedFollows,
    }

    _, err := p.db.ExecuteWrite(ctx, cypher, cypherParams)
    return err
}
```

### Helper Functions

```go
// extractPTags extracts unique pubkeys from p-tags
func extractPTags(ev *event.E) []string {
    seen := make(map[string]bool)
    var pubkeys []string

    for _, tag := range *ev.Tags {
        if len(tag.T) >= 2 && string(tag.T[0]) == "p" {
            pubkey := string(tag.T[1])
            if !seen[pubkey] {
                seen[pubkey] = true
                pubkeys = append(pubkeys, pubkey)
            }
        }
    }

    return pubkeys
}

// diffStringSlices computes added and removed elements
func diffStringSlices(old, new []string) (added, removed []string) {
    oldSet := make(map[string]bool)
    for _, s := range old {
        oldSet[s] = true
    }

    newSet := make(map[string]bool)
    for _, s := range new {
        newSet[s] = true
        if !oldSet[s] {
            added = append(added, s)
        }
    }

    for _, s := range old {
        if !newSet[s] {
            removed = append(removed, s)
        }
    }

    return
}
```

## Integration with SaveEvent

The social event processor should be called from the existing `SaveEvent` method:

```go
// In pkg/neo4j/save-event.go

func (n *N) SaveEvent(c context.Context, ev *event.E) (exists bool, err error) {
    // ... existing event save logic ...

    // After saving base event, process social graph updates
    if ev.Kind == 0 || ev.Kind == 3 || ev.Kind == 1984 || ev.Kind == 10000 {
        processor := &SocialEventProcessor{db: n}
        if err := processor.ProcessSocialEvent(c, ev); err != nil {
            n.Logger.Errorf("failed to process social event: %v", err)
            // Decide: fail the whole save or just log error?
        }
    }

    return false, nil
}
```

## Schema Updates Required

Add ProcessedSocialEvent node to schema.go:

```go
// In applySchema, add:
constraints = append(constraints,
    // Unique constraint on ProcessedSocialEvent.event_id
    "CREATE CONSTRAINT processedSocialEvent_event_id IF NOT EXISTS FOR (e:ProcessedSocialEvent) REQUIRE e.event_id IS UNIQUE",
)

indexes = append(indexes,
    // Index on ProcessedSocialEvent for quick lookup
    "CREATE INDEX processedSocialEvent_pubkey_kind IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.pubkey, e.event_kind)",
    "CREATE INDEX processedSocialEvent_superseded IF NOT EXISTS FOR (e:ProcessedSocialEvent) ON (e.superseded_by)",
)
```

## Query Patterns

### Get User's Current Follows

```cypher
// Get all users followed by a user (from most recent event)
MATCH (user:NostrUser {pubkey: $pubkey})-[f:FOLLOWS]->(followed:NostrUser)
WHERE NOT EXISTS {
    MATCH (old:ProcessedSocialEvent {event_id: f.created_by_event})
    WHERE old.superseded_by IS NOT NULL
}
RETURN followed.pubkey, f.created_at
ORDER BY f.created_at DESC
```

### Get User's Current Mutes

```cypher
MATCH (user:NostrUser {pubkey: $pubkey})-[m:MUTES]->(muted:NostrUser)
WHERE NOT EXISTS {
    MATCH (old:ProcessedSocialEvent {event_id: m.created_by_event})
    WHERE old.superseded_by IS NOT NULL
}
RETURN muted.pubkey
```

### Count Reports Against User

```cypher
MATCH (reporter:NostrUser)-[r:REPORTS]->(reported:NostrUser {pubkey: $pubkey})
RETURN r.report_type, count(*) as report_count
ORDER BY report_count DESC
```

### Get Social Graph History

```cypher
// Get all contact list events for a user, in order
MATCH (evt:ProcessedSocialEvent {pubkey: $pubkey, event_kind: 3})
RETURN evt.event_id, evt.created_at, evt.relationship_count, evt.superseded_by
ORDER BY evt.created_at DESC
```

## Testing Strategy

### Unit Tests

1. **Diff calculation tests**
   - Empty lists
   - No changes
   - All new follows
   - All removed follows
   - Mixed additions and removals

2. **Event ordering tests**
   - Newer event replaces older
   - Older event rejected
   - Same timestamp handling

3. **P-tag extraction tests**
   - Valid tags
   - Duplicate pubkeys
   - Malformed tags
   - Empty tag lists

### Integration Tests

1. **Contact list update flow**
   - Create initial contact list
   - Update with additions
   - Update with removals
   - Verify graph state at each step

2. **Multiple users**
   - Alice follows Bob
   - Bob follows Charlie
   - Alice unfollows Bob
   - Verify relationships

3. **Concurrent updates**
   - Multiple events for same user
   - Verify transaction isolation

## Performance Considerations

### Batch Processing

For initial graph population from existing events:

```go
func (p *SocialEventProcessor) BatchProcessContactLists(ctx context.Context, events []*event.E) error {
    // Group by author
    byAuthor := make(map[string][]*event.E)
    for _, ev := range events {
        pubkey := hex.Enc(ev.Pubkey[:])
        byAuthor[pubkey] = append(byAuthor[pubkey], ev)
    }

    // Process each author's events in order
    for pubkey, authorEvents := range byAuthor {
        // Sort by created_at
        sort.Slice(authorEvents, func(i, j int) bool {
            return authorEvents[i].CreatedAt < authorEvents[j].CreatedAt
        })

        // Process in order (older to newer)
        for _, ev := range authorEvents {
            if err := p.processContactList(ctx, ev); err != nil {
                return fmt.Errorf("batch process failed for %s: %w", pubkey, err)
            }
        }
    }

    return nil
}
```

### Index Strategy

- Index on `(pubkey, event_kind)` for fast lookup of latest event
- Index on `superseded_by` to filter active relationships
- Index on relationship `created_by_event` for diff operations

### Memory Management

- Process events in batches to avoid loading all into memory
- Use streaming queries for large result sets
- Set reasonable limits on relationship counts per user

## Error Handling

### Validation Errors

- Malformed pubkeys → skip, log warning
- Invalid JSON in kind 0 → skip profile update
- Missing required tags → skip event

### Graph Errors

- Neo4j connection failure → retry with backoff
- Transaction timeout → reduce batch size
- Constraint violation → likely race condition, retry

### Recovery Strategies

- Failed event processing → mark event for retry
- Partial transaction → rollback and retry
- Data inconsistency → repair tool to rebuild from events

## Monitoring and Observability

### Metrics to Track

- Events processed per second (by kind)
- Average relationships per contact list
- Diff operation sizes (added/removed counts)
- Transaction durations
- Error rates by error type

### Logging

```go
n.Logger.Infof("processed contact list: author=%s, event=%s, added=%d, removed=%d",
    authorPubkey, eventID, len(added), len(removed))
```

## Future Enhancements

1. **Temporal snapshots**: Store full graph state at regular intervals
2. **Event sourcing**: Replay event log to reconstruct graph
3. **Analytics**: Track follow/unfollow patterns over time
4. **Recommendations**: Suggest users to follow based on graph
5. **Privacy**: Encrypted relationship support (NIP-17, NIP-59)

## Summary

This specification provides a complete event-driven vertex management system that:
- ✅ Captures social graph events (kinds 0, 3, 1984, 10000)
- ✅ Maintains NostrUser vertices with traceability
- ✅ Diffs replaceable events to update relationships correctly
- ✅ Supports relationship queries with event provenance
- ✅ Handles edge cases and error conditions
- ✅ Provides clear implementation path with Go code examples

The system is ready to be implemented as the foundation for WoT trust metrics computation.
