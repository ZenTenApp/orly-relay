# Pubkey Graph System

## Overview

The pubkey graph system provides efficient social graph queries by creating bidirectional, direction-aware edges between events and pubkeys in the ORLY relay.

## Architecture

### 1. Pubkey Serial Assignment

**Purpose**: Compress 32-byte pubkeys to 5-byte serials for space efficiency.

**Tables**:
- `pks|pubkey_hash(8)|serial(5)` - Hash-to-serial lookup (16 bytes)
- `spk|serial(5)` → 32-byte pubkey (value) - Serial-to-pubkey reverse lookup

**Space Savings**: Each graph edge saves 27 bytes per pubkey reference (32 → 5 bytes).

### 2. Graph Edge Storage

**Bidirectional edges with metadata**:

#### EventPubkeyGraph (Forward)
```
epg|event_serial(5)|pubkey_serial(5)|kind(2)|direction(1) = 16 bytes
```

#### PubkeyEventGraph (Reverse)
```
peg|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5) = 16 bytes
```

### 3. Direction Byte

The direction byte distinguishes relationship types:

| Value | Direction | From Event Perspective | From Pubkey Perspective |
|-------|-----------|------------------------|-------------------------|
| `0` | Author | This pubkey is the event author | I am the author of this event |
| `1` | P-Tag Out | Event references this pubkey | *(not used in reverse)* |
| `2` | P-Tag In | *(not used in forward)* | I am referenced by this event |

**Location in keys**:
- **EventPubkeyGraph**: Byte 13 (after 3+5+5)
- **PubkeyEventGraph**: Byte 10 (after 3+5+2)

## Graph Edge Creation

When an event is saved:

1. **Extract pubkeys**:
   - Event author: `ev.Pubkey`
   - P-tags: All `["p", "<hex-pubkey>", ...]` tags

2. **Get or create serials**: Each unique pubkey gets a monotonic 5-byte serial

3. **Create bidirectional edges**:

   For **author** (pubkey = event author):
   ```
   epg|event_serial|author_serial|kind|0  (author edge)
   peg|author_serial|kind|0|event_serial  (is-author edge)
   ```

   For each **p-tag** (referenced pubkey):
   ```
   epg|event_serial|ptag_serial|kind|1    (outbound reference)
   peg|ptag_serial|kind|2|event_serial    (inbound reference)
   ```

## Query Patterns

### Find all events authored by a pubkey
```
Prefix scan: peg|pubkey_serial|*|0|*
Filter: direction == 0 (author)
```

### Find all events mentioning a pubkey (inbound p-tags)
```
Prefix scan: peg|pubkey_serial|*|2|*
Filter: direction == 2 (p-tag inbound)
```

### Find all kind-1 events mentioning a pubkey
```
Prefix scan: peg|pubkey_serial|0x0001|2|*
Exact match: kind == 1, direction == 2
```

### Find all pubkeys referenced by an event (outbound p-tags)
```
Prefix scan: epg|event_serial|*|*|1
Filter: direction == 1 (p-tag outbound)
```

### Find the author of an event
```
Prefix scan: epg|event_serial|*|*|0
Filter: direction == 0 (author)
```

## Implementation Details

### Thread Safety

The `GetOrCreatePubkeySerial` function uses:
1. Read transaction to check for existing serial
2. If not found, get next sequence number
3. Write transaction with double-check to handle race conditions
4. Returns existing serial if another goroutine created it concurrently

### Deduplication

The save-event function deduplicates pubkeys before creating serials:
- Map keyed by hex-encoded pubkey
- Prevents duplicate edges when author is also in p-tags

### Edge Cases

1. **Author in p-tags**: Only creates author edge (direction=0), skips duplicate p-tag edge
2. **Invalid p-tags**: Silently skipped if hex decode fails or length != 32 bytes
3. **No p-tags**: Only author edge is created

## Performance Characteristics

### Space Efficiency

Per event with N unique pubkeys:
- **Old approach** (storing full pubkeys): N × 32 bytes = 32N bytes
- **New approach** (using serials): N × 5 bytes = 5N bytes
- **Savings**: 27N bytes per event (84% reduction)

Example: Event with author + 10 p-tags:
- Old: 11 × 32 = 352 bytes
- New: 11 × 5 = 55 bytes
- **Saved: 297 bytes (84%)**

### Query Performance

1. **Pubkey lookup**: O(1) hash lookup via 8-byte truncated hash
2. **Serial generation**: O(1) atomic increment
3. **Graph queries**: Sequential scan with prefix optimization
4. **Kind filtering**: Built into key ordering, no event decoding needed

## Testing

Comprehensive tests verify:
- ✅ Serial assignment and deduplication
- ✅ Bidirectional graph edge creation
- ✅ Multiple events sharing pubkeys
- ✅ Direction byte correctness
- ✅ Edge cases (invalid pubkeys, non-existent keys)

## Future Query APIs

The graph structure supports efficient queries for:

1. **Social Graph Queries**:
   - Who does Alice follow? (p-tags authored by Alice)
   - Who follows Bob? (p-tags referencing Bob)
   - Common connections between Alice and Bob

2. **Event Discovery**:
   - All replies to Alice's events (kind-1 events with p-tag to Alice)
   - All events Alice has replied to (kind-1 events by Alice with p-tags)
   - Quote reposts, mentions, reactions by event kind

3. **Analytics**:
   - Most-mentioned pubkeys (count p-tag-in edges)
   - Most active authors (count author edges)
   - Interaction patterns by kind

## Migration Notes

This is a **new index** that:
- Runs alongside existing event indexes
- Populated automatically for all new events
- Does NOT require reindexing existing events (yet)
- Can be backfilled via a migration if needed

To backfill existing events, run a migration that:
1. Iterates all events
2. Extracts pubkeys and creates serials
3. Creates graph edges for each event
