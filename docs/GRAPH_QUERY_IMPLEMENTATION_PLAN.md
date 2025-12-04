# Graph Query Implementation Plan

> **Status**: Phases 0-4 complete. See `GRAPH_IMPLEMENTATION_PHASES.md` for current status.

## Overview

This plan describes the implementation of:
1. **E-Tag Graph Index** (`eeg`/`gee`) - Event-to-event reference tracking ✅ COMPLETE
2. **Graph Query Extension** - REQ filter `_graph` field for multi-hop traversals ✅ COMPLETE
3. **Helper Functions** - Pure index-based graph traversal without event decoding ✅ COMPLETE
4. **Relay-Signed Responses** - Results returned as signed Nostr events ✅ COMPLETE

### Response Kinds (Implemented)

| Kind | Name | Description |
|------|------|-------------|
| 39000 | Graph Follows | Response for follows/followers queries |
| 39001 | Graph Mentions | Response for mentions queries |
| 39002 | Graph Thread | Response for thread traversal queries |

### Response Format (Implemented)

```json
{
    "kind": 39000,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "follows"],
        ["seed", "<seed_pubkey_hex>"],
        ["depth", "2"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"pk1\",\"pk2\"],[\"pk3\",\"pk4\"]],\"total_pubkeys\":4}",
    "sig": "<relay_signature>"
}
```

## Architecture

### Current State

The relay has pubkey graph indexes (`epg`/`peg`) that track:
- `epg|event_serial|pubkey_serial|kind|direction` - Event → Pubkey edges
- `peg|pubkey_serial|kind|direction|event_serial` - Pubkey → Event edges

**Direction values:**
- `0` = Author (bidirectional)
- `1` = P-Tag Out (event references pubkey)
- `2` = P-Tag In (pubkey referenced by event)

### Proposed Addition: E-Tag Graph

```
Event-Event Graph (Forward): eeg
  eeg|source_event_serial(5)|target_event_serial(5)|kind(2)|direction(1) = 16 bytes

Event-Event Graph (Reverse): gee
  gee|target_event_serial(5)|kind(2)|direction(1)|source_event_serial(5) = 16 bytes
```

**Direction values:**
- `0` = References (source event has e-tag pointing to target)
- `1` = Referenced-by (target event is referenced by source)

### Combined Query Capability

With both indexes, we can answer questions like:

1. **"Who follows Alice?"** (kind-3 events with p-tag to Alice)
   - Use `peg|alice_serial|0003|2|*`

2. **"What kind-7 reactions reference events authored by Alice?"**
   - Step 1: Find Alice's events via `peg|alice_serial|*|0|*`
   - Step 2: For each event, find kind-7 reactions via `gee|event_serial|0007|1|*`

3. **"Traverse Alice's follow graph 2 hops deep"**
   - Step 1: Get Alice's kind-3 event serial
   - Step 2: Iterate `epg|k3_serial|*|*|1` to get followed pubkey serials
   - Step 3: For each, repeat step 1-2

---

## Phase 1: E-Tag Graph Index ✅ COMPLETE

> Implemented in `pkg/database/indexes/keys.go`, `pkg/database/indexes/types/letter.go`, and `pkg/database/save-event.go`

### 1.1 Add Index Key Definitions

**File: `pkg/database/indexes/keys.go`**

```go
// EventEventGraph creates edges between events (e-tag references)
// Source event has e-tag pointing to target event
// Direction: 0=references, 1=referenced-by
//
//   3 prefix|5 source_event_serial|5 target_event_serial|2 kind|1 direction
var EventEventGraph = next()

func EventEventGraphVars() (
    sourceSer *types.Uint40, targetSer *types.Uint40,
    kind *types.Uint16, direction *types.Letter,
) {
    return new(types.Uint40), new(types.Uint40), new(types.Uint16), new(types.Letter)
}

func EventEventGraphEnc(
    sourceSer *types.Uint40, targetSer *types.Uint40,
    kind *types.Uint16, direction *types.Letter,
) (enc *T) {
    return New(NewPrefix(EventEventGraph), sourceSer, targetSer, kind, direction)
}

// GraphEventEvent is the reverse index for efficient lookups
// "What events reference this event?"
//
//   3 prefix|5 target_event_serial|2 kind|1 direction|5 source_event_serial
var GraphEventEvent = next()

func GraphEventEventVars() (
    targetSer *types.Uint40, kind *types.Uint16,
    direction *types.Letter, sourceSer *types.Uint40,
) {
    return new(types.Uint40), new(types.Uint16), new(types.Letter), new(types.Uint40)
}

func GraphEventEventEnc(
    targetSer *types.Uint40, kind *types.Uint16,
    direction *types.Letter, sourceSer *types.Uint40,
) (enc *T) {
    return New(NewPrefix(GraphEventEvent), targetSer, kind, direction, sourceSer)
}
```

### 1.2 Add Direction Constants

**File: `pkg/database/indexes/types/direction.go`**

```go
package types

const (
    // Pubkey graph directions (existing)
    EdgeDirectionAuthor  = byte(0) // Event author relationship
    EdgeDirectionPTagOut = byte(1) // Event references pubkey (outbound)
    EdgeDirectionPTagIn  = byte(2) // Pubkey is referenced (inbound)

    // Event graph directions (new)
    EdgeDirectionETagOut = byte(0) // Event references another event (outbound)
    EdgeDirectionETagIn  = byte(1) // Event is referenced by another (inbound)
)
```

### 1.3 Populate E-Tag Graph in SaveEvent

**File: `pkg/database/save-event.go`** (additions)

```go
// After existing pubkey graph creation, add e-tag graph:

// Extract e-tag event IDs for graph
eTags := ev.Tags.GetAll([]byte("e"))
for _, eTag := range eTags {
    if eTag.Len() >= 2 {
        // Get referenced event ID, handling binary/hex formats
        var targetEventID []byte
        if targetEventID, err = hex.Dec(string(eTag.ValueHex())); err == nil && len(targetEventID) == 32 {
            // Look up target event's serial (may not exist if we haven't seen it)
            var targetSerial *types.Uint40
            if targetSerial, err = d.GetEventSerialByID(targetEventID); err == nil && targetSerial != nil {
                // Create forward edge: source -> target
                keyBuf := new(bytes.Buffer)
                directionOut := new(types.Letter)
                directionOut.Set(types.EdgeDirectionETagOut)
                if err = indexes.EventEventGraphEnc(ser, targetSerial, eventKind, directionOut).MarshalWrite(keyBuf); chk.E(err) {
                    return
                }
                if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
                    return
                }

                // Create reverse edge: target <- source
                keyBuf.Reset()
                directionIn := new(types.Letter)
                directionIn.Set(types.EdgeDirectionETagIn)
                if err = indexes.GraphEventEventEnc(targetSerial, eventKind, directionIn, ser).MarshalWrite(keyBuf); chk.E(err) {
                    return
                }
                if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
                    return
                }
            }
            // Note: If target event doesn't exist yet, we could queue for later backfill
            // or just skip - the edge will be created when querying if needed
        }
    }
}
```

### 1.4 Helper: Get Event Serial by ID

**File: `pkg/database/event-serial-lookup.go`** (new)

```go
package database

import (
    "bytes"

    "github.com/dgraph-io/badger/v4"
    "next.orly.dev/pkg/database/indexes"
    "next.orly.dev/pkg/database/indexes/types"
)

// GetEventSerialByID looks up an event's serial by its 32-byte ID
// Returns nil if event not found
func (d *D) GetEventSerialByID(eventID []byte) (*types.Uint40, error) {
    if len(eventID) != 32 {
        return nil, nil
    }

    // Use the existing ID index: eid|id_hash(8)|serial(5)
    idHash := new(types.IdHash)
    idHash.Set(eventID)

    var serial *types.Uint40
    err := d.View(func(txn *badger.Txn) error {
        // Create prefix for ID lookup
        prefix := new(bytes.Buffer)
        indexes.IdEnc(idHash, nil).MarshalWrite(prefix)

        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        it.Seek(prefix.Bytes())
        if it.ValidForPrefix(prefix.Bytes()) {
            // Decode the serial from the key
            key := it.Item().Key()
            ser := new(types.Uint40)
            dec := indexes.IdDec(new(types.IdHash), ser)
            if err := dec.UnmarshalRead(bytes.NewReader(key)); err == nil {
                serial = ser
            }
        }
        return nil
    })

    return serial, err
}
```

---

## Phase 2: Graph Traversal Functions

### 2.1 Core Traversal Primitives

**File: `pkg/database/graph-traversal.go`** (new)

```go
package database

import (
    "bytes"

    "github.com/dgraph-io/badger/v4"
    "next.orly.dev/pkg/database/indexes"
    "next.orly.dev/pkg/database/indexes/types"
    "git.mleku.dev/mleku/nostr/encoders/hex"
)

// GraphResult contains the results of a graph traversal
type GraphResult struct {
    // PubkeysByDepth maps depth level to pubkeys discovered at that depth
    // Used for simple queries - pubkeys ordered by first-seen depth (ascending)
    PubkeysByDepth map[int][]string `json:"pubkeys_by_depth,omitempty"`

    // EventsByDepth maps depth level to event IDs discovered at that depth
    EventsByDepth map[int][]string `json:"events_by_depth,omitempty"`

    // FirstSeen tracks which depth each pubkey/event was first discovered
    FirstSeen map[string]int `json:"first_seen"`

    // InboundRefs holds aggregated inbound reference data
    // Map: ref_kind -> target_event_id -> []referencing_event_ids
    InboundRefs map[uint16]map[string][]string `json:"inbound_refs,omitempty"`

    // OutboundRefs holds aggregated outbound reference data
    // Map: ref_kind -> source_event_id -> []referenced_event_ids
    OutboundRefs map[uint16]map[string][]string `json:"outbound_refs,omitempty"`

    // TargetAuthors maps event IDs to their author pubkeys (for response generation)
    TargetAuthors map[string]string `json:"target_authors,omitempty"`

    // TargetDepths maps event IDs to the depth at which their author was discovered
    TargetDepths map[string]int `json:"target_depths,omitempty"`

    // Counts for quick summaries
    TotalPubkeys int `json:"total_pubkeys"`
    TotalEvents  int `json:"total_events"`
}

// RefAggregation represents a single target with its reference count
// Used for sorting results by count descending
type RefAggregation struct {
    TargetEventID string   // The event being referenced
    TargetAuthor  string   // Author of the target event
    TargetDepth   int      // Depth at which target's author was discovered
    RefKind       uint16   // Kind of the referencing events
    RefCount      int      // Number of references
    RefEventIDs   []string // IDs of referencing events
}

// GetInboundRefsSorted returns inbound refs for a kind, sorted by count descending
func (r *GraphResult) GetInboundRefsSorted(kind uint16) []RefAggregation {
    refs, ok := r.InboundRefs[kind]
    if !ok {
        return nil
    }

    var aggs []RefAggregation
    for targetID, refIDs := range refs {
        aggs = append(aggs, RefAggregation{
            TargetEventID: targetID,
            TargetAuthor:  r.TargetAuthors[targetID],
            TargetDepth:   r.TargetDepths[targetID],
            RefKind:       kind,
            RefCount:      len(refIDs),
            RefEventIDs:   refIDs,
        })
    }

    // Sort by RefCount descending
    sort.Slice(aggs, func(i, j int) bool {
        return aggs[i].RefCount > aggs[j].RefCount
    })

    return aggs
}

// GetOutboundRefsSorted returns outbound refs for a kind, sorted by count descending
func (r *GraphResult) GetOutboundRefsSorted(kind uint16) []RefAggregation {
    refs, ok := r.OutboundRefs[kind]
    if !ok {
        return nil
    }

    var aggs []RefAggregation
    for sourceID, refIDs := range refs {
        aggs = append(aggs, RefAggregation{
            TargetEventID: sourceID, // For outbound, "target" is the source event
            TargetAuthor:  r.TargetAuthors[sourceID],
            TargetDepth:   r.TargetDepths[sourceID],
            RefKind:       kind,
            RefCount:      len(refIDs),
            RefEventIDs:   refIDs,
        })
    }

    // Sort by RefCount descending
    sort.Slice(aggs, func(i, j int) bool {
        return aggs[i].RefCount > aggs[j].RefCount
    })

    return aggs
}

// GetDepthsSorted returns depth levels in ascending order
func (r *GraphResult) GetDepthsSorted() []int {
    var depths []int
    for d := range r.PubkeysByDepth {
        depths = append(depths, d)
    }
    sort.Ints(depths)
    return depths
}

// GetPTagsFromEventSerial returns all pubkey serials referenced by an event's p-tags
// Uses: epg|event_serial|*|*|1 (direction=1 = outbound p-tag)
func (d *D) GetPTagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error) {
    var pubkeySerials []*types.Uint40

    err := d.View(func(txn *badger.Txn) error {
        // Build prefix: epg|event_serial
        prefix := new(bytes.Buffer)
        indexes.EventPubkeyGraphEnc(eventSerial, nil, nil, nil).MarshalWrite(prefix)

        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Seek(prefix.Bytes()); it.ValidForPrefix(prefix.Bytes()); it.Next() {
            key := it.Item().Key()

            // Decode key to get pubkey serial and direction
            eventSer, pubkeySer, _, direction := indexes.EventPubkeyGraphVars()
            dec := indexes.EventPubkeyGraphDec(eventSer, pubkeySer, new(types.Uint16), direction)
            if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                continue
            }

            // Only include p-tag references (direction=1), not author (direction=0)
            if direction.Get() == types.EdgeDirectionPTagOut {
                ser := new(types.Uint40)
                ser.Set(pubkeySer.Uint64())
                pubkeySerials = append(pubkeySerials, ser)
            }
        }
        return nil
    })

    return pubkeySerials, err
}

// GetETagsFromEventSerial returns all event serials referenced by an event's e-tags
// Uses: eeg|event_serial|*|*|0 (direction=0 = outbound e-tag)
func (d *D) GetETagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error) {
    var eventSerials []*types.Uint40

    err := d.View(func(txn *badger.Txn) error {
        prefix := new(bytes.Buffer)
        indexes.EventEventGraphEnc(eventSerial, nil, nil, nil).MarshalWrite(prefix)

        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Seek(prefix.Bytes()); it.ValidForPrefix(prefix.Bytes()); it.Next() {
            key := it.Item().Key()

            sourceSer, targetSer, _, direction := indexes.EventEventGraphVars()
            dec := indexes.EventEventGraphDec(sourceSer, targetSer, new(types.Uint16), direction)
            if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                continue
            }

            if direction.Get() == types.EdgeDirectionETagOut {
                ser := new(types.Uint40)
                ser.Set(targetSer.Uint64())
                eventSerials = append(eventSerials, ser)
            }
        }
        return nil
    })

    return eventSerials, err
}

// GetReferencingEvents returns events that reference a target event with e-tags
// Optionally filtered by kind (e.g., kind-7 for reactions, kind-1 for replies)
// Uses: gee|target_event_serial|kind|1|* (direction=1 = inbound reference)
func (d *D) GetReferencingEvents(targetSerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
    var eventSerials []*types.Uint40

    err := d.View(func(txn *badger.Txn) error {
        if len(kinds) > 0 {
            // Scan specific kinds
            for _, k := range kinds {
                kind := new(types.Uint16)
                kind.Set(k)
                direction := new(types.Letter)
                direction.Set(types.EdgeDirectionETagIn)

                prefix := new(bytes.Buffer)
                indexes.GraphEventEventEnc(targetSerial, kind, direction, nil).MarshalWrite(prefix)

                opts := badger.DefaultIteratorOptions
                opts.PrefetchValues = false
                it := txn.NewIterator(opts)

                for it.Seek(prefix.Bytes()); it.ValidForPrefix(prefix.Bytes()); it.Next() {
                    key := it.Item().Key()

                    _, _, _, sourceSer := indexes.GraphEventEventVars()
                    dec := indexes.GraphEventEventDec(new(types.Uint40), new(types.Uint16), new(types.Letter), sourceSer)
                    if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                        continue
                    }

                    ser := new(types.Uint40)
                    ser.Set(sourceSer.Uint64())
                    eventSerials = append(eventSerials, ser)
                }
                it.Close()
            }
        } else {
            // Scan all kinds for this target
            prefix := new(bytes.Buffer)
            indexes.GraphEventEventEnc(targetSerial, nil, nil, nil).MarshalWrite(prefix)

            opts := badger.DefaultIteratorOptions
            opts.PrefetchValues = false
            it := txn.NewIterator(opts)
            defer it.Close()

            for it.Seek(prefix.Bytes()); it.ValidForPrefix(prefix.Bytes()); it.Next() {
                key := it.Item().Key()

                _, _, direction, sourceSer := indexes.GraphEventEventVars()
                dec := indexes.GraphEventEventDec(new(types.Uint40), new(types.Uint16), direction, sourceSer)
                if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                    continue
                }

                // Only inbound references
                if direction.Get() == types.EdgeDirectionETagIn {
                    ser := new(types.Uint40)
                    ser.Set(sourceSer.Uint64())
                    eventSerials = append(eventSerials, ser)
                }
            }
        }
        return nil
    })

    return eventSerials, err
}

// FindEventByAuthorAndKind finds the most recent event of a kind by author
// Uses: peg|pubkey_serial|kind|0|* (direction=0 = author)
func (d *D) FindEventByAuthorAndKind(authorSerial *types.Uint40, k uint16) (*types.Uint40, error) {
    var eventSerial *types.Uint40

    err := d.View(func(txn *badger.Txn) error {
        kind := new(types.Uint16)
        kind.Set(k)
        direction := new(types.Letter)
        direction.Set(types.EdgeDirectionAuthor)

        prefix := new(bytes.Buffer)
        indexes.PubkeyEventGraphEnc(authorSerial, kind, direction, nil).MarshalWrite(prefix)

        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        opts.Reverse = true // Get most recent (highest serial)
        it := txn.NewIterator(opts)
        defer it.Close()

        // Seek to end of prefix range
        endKey := make([]byte, len(prefix.Bytes())+5)
        copy(endKey, prefix.Bytes())
        for i := len(prefix.Bytes()); i < len(endKey); i++ {
            endKey[i] = 0xFF
        }

        it.Seek(endKey)
        if it.ValidForPrefix(prefix.Bytes()) {
            key := it.Item().Key()

            _, _, _, evSer := indexes.PubkeyEventGraphVars()
            dec := indexes.PubkeyEventGraphDec(new(types.Uint40), new(types.Uint16), new(types.Letter), evSer)
            if err := dec.UnmarshalRead(bytes.NewReader(key)); err == nil {
                eventSerial = new(types.Uint40)
                eventSerial.Set(evSer.Uint64())
            }
        }
        return nil
    })

    return eventSerial, err
}

// GetPubkeyHexFromSerial resolves a pubkey serial to hex string
// Uses: spk|serial -> 32-byte pubkey value
func (d *D) GetPubkeyHexFromSerial(serial *types.Uint40) (string, error) {
    var pubkeyHex string

    err := d.View(func(txn *badger.Txn) error {
        keyBuf := new(bytes.Buffer)
        indexes.SerialPubkeyEnc(serial).MarshalWrite(keyBuf)

        item, err := txn.Get(keyBuf.Bytes())
        if err != nil {
            return err
        }

        return item.Value(func(val []byte) error {
            if len(val) == 32 {
                pubkeyHex = hex.Enc(val)
            }
            return nil
        })
    })

    return pubkeyHex, err
}
```

### 2.2 High-Level Traversal: Follow Graph

**File: `pkg/database/graph-follows.go`** (new)

```go
package database

import (
    "git.mleku.dev/mleku/nostr/encoders/hex"
    "next.orly.dev/pkg/database/indexes/types"
)

// TraverseFollows performs BFS traversal of the follow graph
// Returns pubkeys grouped by discovery depth
// Terminates early if two consecutive depths yield no new pubkeys
func (d *D) TraverseFollows(seedPubkey []byte, maxDepth int) (*GraphResult, error) {
    result := &GraphResult{
        PubkeysByDepth: make(map[int][]string),
        FirstSeen:      make(map[string]int),
    }

    // Get seed pubkey serial
    seedSerial, err := d.GetPubkeySerial(seedPubkey)
    if err != nil {
        return nil, err
    }
    if seedSerial == nil {
        return result, nil // Pubkey not in database
    }

    // Track pubkeys to process at each depth level
    currentLevel := []*types.Uint40{seedSerial}
    visited := make(map[uint64]bool)
    visited[seedSerial.Uint64()] = true

    consecutiveEmptyDepths := 0

    for currentDepth := 1; currentDepth <= maxDepth; currentDepth++ {
        var nextLevel []*types.Uint40
        newPubkeysAtDepth := 0

        // Process all pubkeys at current level
        for _, pubkeySerial := range currentLevel {
            // Find this pubkey's kind-3 (follow list) event
            k3Serial, err := d.FindEventByAuthorAndKind(pubkeySerial, 3)
            if err != nil || k3Serial == nil {
                continue // No follow list for this pubkey
            }

            // Get all p-tags from the follow list (without decoding the event!)
            followSerials, err := d.GetPTagsFromEventSerial(k3Serial)
            if err != nil {
                continue
            }

            for _, followSerial := range followSerials {
                followVal := followSerial.Uint64()
                if visited[followVal] {
                    continue
                }
                visited[followVal] = true

                // Resolve serial to pubkey hex
                pubkeyHex, err := d.GetPubkeyHexFromSerial(followSerial)
                if err != nil || pubkeyHex == "" {
                    continue
                }

                // Record discovery
                result.FirstSeen[pubkeyHex] = currentDepth
                result.PubkeysByDepth[currentDepth] = append(result.PubkeysByDepth[currentDepth], pubkeyHex)
                result.TotalPubkeys++
                newPubkeysAtDepth++

                // Add to next level for further traversal
                nextLevel = append(nextLevel, followSerial)
            }
        }

        // Early termination: if two consecutive depths yield no new pubkeys, stop
        if newPubkeysAtDepth == 0 {
            consecutiveEmptyDepths++
            if consecutiveEmptyDepths >= 2 {
                // Graph exhausted - no new pubkeys for 2 consecutive depths
                break
            }
        } else {
            consecutiveEmptyDepths = 0
        }

        // Move to next level
        currentLevel = nextLevel
        if len(currentLevel) == 0 && consecutiveEmptyDepths < 2 {
            // No more pubkeys to process but haven't hit termination condition yet
            consecutiveEmptyDepths++
            if consecutiveEmptyDepths >= 2 {
                break
            }
        }
    }

    return result, nil
}

// AddInboundRefsToResult finds events that reference events/pubkeys at the given depth
// and adds them to the result. Uses the gee (GraphEventEvent) index.
// This enables finding reactions, replies, reposts, etc. without decoding events.
func (d *D) AddInboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
    if result.EventsByDepth == nil {
        result.EventsByDepth = make(map[int][]string)
    }

    seenEvents := make(map[string]bool)
    for _, eventID := range result.AllEvents() {
        seenEvents[eventID] = true
    }

    // For pubkeys at this depth, find their authored events, then find inbound refs
    pubkeys := result.PubkeysByDepth[depth]
    for _, pubkeyHex := range pubkeys {
        pubkeyBytes, err := hex.Dec(pubkeyHex)
        if err != nil {
            continue
        }

        pubkeySerial, err := d.GetPubkeySerial(pubkeyBytes)
        if err != nil || pubkeySerial == nil {
            continue
        }

        // Find all events authored by this pubkey
        // Scan: peg|pubkey_serial|*|0|* (direction=0 for author)
        authoredSerials, err := d.GetEventsAuthoredByPubkey(pubkeySerial, nil)
        if err != nil {
            continue
        }

        // For each authored event, find inbound references of specified kinds
        for _, eventSerial := range authoredSerials {
            refSerials, err := d.GetReferencingEvents(eventSerial, kinds)
            if err != nil {
                continue
            }

            for _, refSerial := range refSerials {
                eventID, err := d.GetEventIDFromSerial(refSerial)
                if err != nil || eventID == "" {
                    continue
                }

                if !seenEvents[eventID] {
                    seenEvents[eventID] = true
                    result.EventsByDepth[depth] = append(result.EventsByDepth[depth], eventID)
                    result.TotalEvents++
                }
            }
        }
    }

    return nil
}

// AddOutboundRefsToResult finds events referenced BY events at the given depth
// and adds them to the result. Uses the eeg (EventEventGraph) index.
// This enables following e-tag chains to find what posts are replying to.
func (d *D) AddOutboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
    if result.EventsByDepth == nil {
        result.EventsByDepth = make(map[int][]string)
    }

    seenEvents := make(map[string]bool)
    for _, eventID := range result.AllEvents() {
        seenEvents[eventID] = true
    }

    // Get events at this depth
    eventIDs := result.EventsByDepth[depth]
    for _, eventIDHex := range eventIDs {
        eventIDBytes, err := hex.Dec(eventIDHex)
        if err != nil {
            continue
        }

        eventSerial, err := d.GetEventSerialByID(eventIDBytes)
        if err != nil || eventSerial == nil {
            continue
        }

        // Find outbound e-tag references
        // Scan: eeg|event_serial|*|*|0 (direction=0 for outbound)
        referencedSerials, err := d.GetETagsFromEventSerial(eventSerial)
        if err != nil {
            continue
        }

        for _, refSerial := range referencedSerials {
            // Optionally filter by kind if specified
            if len(kinds) > 0 {
                // Would need to look up event kind - for now add all
                // Future: add kind check here
            }

            refEventID, err := d.GetEventIDFromSerial(refSerial)
            if err != nil || refEventID == "" {
                continue
            }

            if !seenEvents[refEventID] {
                seenEvents[refEventID] = true
                result.EventsByDepth[depth] = append(result.EventsByDepth[depth], refEventID)
                result.TotalEvents++
            }
        }
    }

    return nil
}

// AllPubkeys returns all pubkeys discovered across all depths
func (r *GraphResult) AllPubkeys() []string {
    var all []string
    for _, pubkeys := range r.PubkeysByDepth {
        all = append(all, pubkeys...)
    }
    return all
}

// AllEvents returns all event IDs discovered across all depths
func (r *GraphResult) AllEvents() []string {
    var all []string
    for _, events := range r.EventsByDepth {
        all = append(all, events...)
    }
    return all
}

// GetEventsAuthoredByPubkey returns event serials for events authored by a pubkey
// Optionally filtered by kinds
func (d *D) GetEventsAuthoredByPubkey(pubkeySerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
    // Implementation similar to GetReferencingEvents but using peg index
    // peg|pubkey_serial|kind|0|event_serial (direction=0 for author)
    // ...
}

// GetEventIDFromSerial resolves an event serial to hex event ID
func (d *D) GetEventIDFromSerial(serial *types.Uint40) (string, error) {
    // Use sei|serial -> 32-byte event ID
    // ...
}
```

---

## Phase 3: REQ Filter Extension

### 3.1 Graph Query Specification

The `_graph` field in a REQ filter enables graph traversal queries:

```json
["REQ", "sub1", {
    "_graph": {
        "method": "follows",
        "seed": "<pubkey_hex>",
        "depth": 2,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1},
            {"kinds": [6], "from_depth": 1}
        ]
    },
    "kinds": [0]  // Return kind-0 (profile) events for discovered pubkeys
}]
```

**Methods:**
- `follows` - Traverse outbound follow relationships (kind-3 p-tags)
- `followers` - Traverse inbound follow relationships (kind-3 events mentioning seed)
- `mentions` - Find events mentioning seed pubkey (any kind with p-tag)
- `thread` - Traverse reply thread via e-tags

**Parameters:**
- `seed` - Starting pubkey (hex) or event ID (hex) for traversal
- `depth` - Maximum traversal depth (1-5, default 1)
- `inbound_refs` - Array of inbound reference filters (AND semantics between items):
  - `kinds` - Event kinds that reference discovered events (e.g., [7] for reactions, [6] for reposts, [1] for replies)
  - `from_depth` - Only include refs from this depth onwards (0 = include refs to seed)
- `outbound_refs` - Array of outbound reference filters (AND semantics between items):
  - `kinds` - Event kinds referenced by discovered events (e.g., follow e-tag chains)
  - `from_depth` - Only include refs from this depth onwards

**Semantics:**
- Items within `inbound_refs` array are AND'd together (all conditions must match)
- Items within `outbound_refs` array are AND'd together
- `inbound_refs` and `outbound_refs` can be used together for bidirectional traversal

### 3.1.1 Response Format

The response format depends on whether `inbound_refs`/`outbound_refs` are specified.

#### Simple Query Response (no refs)

For queries without `inbound_refs` or `outbound_refs`, the response is a series of events
sent in **ascending depth order**, with pubkeys grouped by the depth at which they were
first discovered:

```
["EVENT", "sub1", <kind-0 event for pubkey at depth 1>]
["EVENT", "sub1", <kind-0 event for pubkey at depth 1>]
["EVENT", "sub1", <kind-0 event for pubkey at depth 1>]
... (all depth 1 pubkeys)
["EVENT", "sub1", <kind-0 event for pubkey at depth 2>]
["EVENT", "sub1", <kind-0 event for pubkey at depth 2>]
... (all depth 2 pubkeys)
["EOSE", "sub1"]
```

Optionally, a metadata event can precede the results:

```json
["EVENT", "sub1", {
    "kind": 30078,
    "content": "",
    "tags": [
        ["d", "graph-meta"],
        ["method", "follows"],
        ["seed", "<pubkey>"],
        ["depth", "2"],
        ["count", "1", "150"],
        ["count", "2", "3420"]
    ]
}]
```

The `count` tags indicate: `["count", "<depth>", "<pubkey_count>"]`

#### Query Response with Refs (inbound_refs and/or outbound_refs)

When `inbound_refs` or `outbound_refs` are specified, the response includes aggregated
reference data. Results are sent as arrays **sorted in descending order by count** -
events with the most references come first, down to events with only one reference.

**Response structure:**

1. **Graph metadata event** (describes the query and aggregate counts)
2. **Inbound ref results** (if `inbound_refs` specified) - sorted by ref count descending
3. **Outbound ref results** (if `outbound_refs` specified) - sorted by ref count descending
4. **EOSE**

**Metadata event format:**

```json
["EVENT", "sub1", {
    "kind": 30078,
    "content": "",
    "tags": [
        ["d", "graph-meta"],
        ["method", "follows"],
        ["seed", "<pubkey>"],
        ["depth", "2"],
        ["count", "1", "150"],
        ["count", "2", "3420"],
        ["inbound", "7", "total", "8932"],
        ["inbound", "7", "unique_targets", "2100"],
        ["inbound", "6", "total", "342"],
        ["inbound", "6", "unique_targets", "89"],
        ["outbound", "1", "total", "1205"],
        ["outbound", "1", "unique_targets", "890"]
    ]
}]
```

**Ref result event format:**

Each target event (the event being referenced) is sent with its reference count and
the referencing event IDs:

```json
["EVENT", "sub1", {
    "kind": 30079,
    "content": "",
    "tags": [
        ["d", "inbound-refs"],
        ["target", "<event_id_hex>"],
        ["target_author", "<pubkey_hex>"],
        ["target_depth", "1"],
        ["ref_kind", "7"],
        ["ref_count", "523"],
        ["refs", "<ref_event_id_1>", "<ref_event_id_2>", "..."]
    ]
}]
```

**Ordering:** Results are sent in **descending order by `ref_count`**:
- First: events with highest reference counts (most popular)
- Last: events with only 1 reference

**Example response for reactions query:**

```
// Metadata
["EVENT", "sub1", {kind: 30078, tags: [["d","graph-meta"], ["inbound","7","total","1500"], ...]}]

// Most reacted post (523 reactions)
["EVENT", "sub1", {kind: 30079, tags: [["target","abc..."], ["ref_kind","7"], ["ref_count","523"], ...]}]

// Second most reacted (312 reactions)
["EVENT", "sub1", {kind: 30079, tags: [["target","def..."], ["ref_kind","7"], ["ref_count","312"], ...]}]

// ... continues in descending order ...

// Least reacted posts (1 reaction each)
["EVENT", "sub1", {kind: 30079, tags: [["target","xyz..."], ["ref_kind","7"], ["ref_count","1"], ...]}]

["EOSE", "sub1"]
```

**Multiple ref types:** When multiple `inbound_refs` or `outbound_refs` are specified,
each type gets its own sorted sequence:

```
// Metadata with all ref type summaries
["EVENT", "sub1", {kind: 30078, tags: [...]}]

// All kind-7 (reactions) sorted by count descending
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","7"], ["ref_count","523"], ...]}]
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","7"], ["ref_count","312"], ...]}]
...
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","7"], ["ref_count","1"], ...]}]

// All kind-6 (reposts) sorted by count descending
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","6"], ["ref_count","89"], ...]}]
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","6"], ["ref_count","45"], ...]}]
...
["EVENT", "sub1", {kind: 30079, tags: [["ref_kind","6"], ["ref_count","1"], ...]}]

// All outbound refs (if specified) sorted by count descending
["EVENT", "sub1", {kind: 30080, tags: [["ref_kind","1"], ["ref_count","15"], ...]}]
...

["EOSE", "sub1"]
```

**Note:** The reference aggregation response format described above is planned for a future phase. The current implementation returns graph results as relay-signed events with kinds:

- `39000` - Graph Follows (follows/followers queries)
- `39001` - Graph Mentions (mentions queries)
- `39002` - Graph Thread (thread traversal queries)

These are application-specific kinds in the 39000-39999 range.

### 3.2 Filter Extension Struct

**File: `pkg/protocol/graph/query.go`** (new)

```go
package graph

import (
    "errors"
    "fmt"
)

// Query represents a graph traversal query embedded in a REQ filter
type Query struct {
    Method      string     `json:"method"`
    Seed        string     `json:"seed"`
    Depth       int        `json:"depth,omitempty"`
    InboundRefs []RefSpec  `json:"inbound_refs,omitempty"`
    OutboundRefs []RefSpec `json:"outbound_refs,omitempty"`
}

// RefSpec specifies a reference filter for graph traversal
// Multiple RefSpecs in an array have AND semantics
type RefSpec struct {
    Kinds     []uint16 `json:"kinds"`               // Event kinds to match
    FromDepth int      `json:"from_depth,omitempty"` // Only apply from this depth (0 = include seed)
}

// Validate checks that the query parameters are valid
func (q *Query) Validate() error {
    if q.Method == "" {
        return errors.New("_graph.method is required")
    }
    if q.Seed == "" {
        return errors.New("_graph.seed is required")
    }
    if len(q.Seed) != 64 {
        return errors.New("_graph.seed must be 64-character hex (pubkey or event ID)")
    }
    if q.Depth < 1 {
        q.Depth = 1
    }
    if q.Depth > 16 {
        return errors.New("_graph.depth cannot exceed 16")
    }

    validMethods := map[string]bool{
        "follows": true, "followers": true, "mentions": true, "thread": true,
    }
    if !validMethods[q.Method] {
        return fmt.Errorf("_graph.method '%s' is not valid", q.Method)
    }

    // Validate ref specs
    for i, ref := range q.InboundRefs {
        if len(ref.Kinds) == 0 {
            return fmt.Errorf("_graph.inbound_refs[%d].kinds cannot be empty", i)
        }
        if ref.FromDepth < 0 || ref.FromDepth > q.Depth {
            return fmt.Errorf("_graph.inbound_refs[%d].from_depth must be 0-%d", i, q.Depth)
        }
    }
    for i, ref := range q.OutboundRefs {
        if len(ref.Kinds) == 0 {
            return fmt.Errorf("_graph.outbound_refs[%d].kinds cannot be empty", i)
        }
        if ref.FromDepth < 0 || ref.FromDepth > q.Depth {
            return fmt.Errorf("_graph.outbound_refs[%d].from_depth must be 0-%d", i, q.Depth)
        }
    }

    return nil
}

// HasInboundRefs returns true if any inbound reference filters are specified
func (q *Query) HasInboundRefs() bool {
    return len(q.InboundRefs) > 0
}

// HasOutboundRefs returns true if any outbound reference filters are specified
func (q *Query) HasOutboundRefs() bool {
    return len(q.OutboundRefs) > 0
}

// InboundKindsAtDepth returns the kinds to query for inbound refs at a given depth
// Returns nil if no inbound refs apply at this depth
func (q *Query) InboundKindsAtDepth(depth int) []uint16 {
    var kinds []uint16
    for _, ref := range q.InboundRefs {
        if depth >= ref.FromDepth {
            kinds = append(kinds, ref.Kinds...)
        }
    }
    return kinds
}

// OutboundKindsAtDepth returns the kinds to query for outbound refs at a given depth
func (q *Query) OutboundKindsAtDepth(depth int) []uint16 {
    var kinds []uint16
    for _, ref := range q.OutboundRefs {
        if depth >= ref.FromDepth {
            kinds = append(kinds, ref.Kinds...)
        }
    }
    return kinds
}
```

### 3.3 Handle Graph Query in REQ Handler

**File: `app/handle-req.go`** (additions)

```go
import "next.orly.dev/pkg/protocol/graph"

func (l *Listener) HandleReq(msg []byte) (err error) {
    // ... existing code ...

    // Check for graph queries in filters
    for _, f := range *env.Filters {
        if f != nil {
            // Check for _graph extension field
            graphQuery := l.extractGraphQuery(f)
            if graphQuery != nil {
                // Execute graph query instead of normal filter
                if err = l.handleGraphQuery(env.Subscription, graphQuery, f); err != nil {
                    log.E.F("graph query failed: %v", err)
                }
                // Send EOSE and return - graph queries don't create subscriptions
                if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
                    return
                }
                return nil
            }
        }
    }

    // ... rest of existing HandleReq ...
}

func (l *Listener) extractGraphQuery(f *filter.F) *graph.Query {
    // The filter has an Extra map for unknown fields
    // We need to check if _graph was preserved during parsing
    // This may require changes to the filter package in the nostr library
    // to preserve unknown fields

    // For now, check if there's a custom extension mechanism
    // ...
    return nil
}

func (l *Listener) handleGraphQuery(subID []byte, q *graph.Query, f *filter.F) error {
    if err := q.Validate(); err != nil {
        return err
    }

    seedBytes, err := hex.Dec(q.Seed)
    if err != nil {
        return fmt.Errorf("invalid seed: %w", err)
    }

    var result *database.GraphResult

    switch q.Method {
    case "follows":
        // Traverse follow graph via kind-3 p-tags
        result, err = l.DB.TraverseFollows(seedBytes, q.Depth)

    case "followers":
        // Find who follows this pubkey (kind-3 events with p-tag to seed)
        result, err = l.DB.TraverseFollowers(seedBytes, q.Depth)

    case "mentions":
        // Find events mentioning this pubkey (p-tag references)
        var kinds []uint16
        if f.Kinds != nil {
            kinds = f.Kinds.ToUint16()
        }
        result, err = l.DB.FindMentions(seedBytes, kinds)

    case "thread":
        // Traverse reply thread via e-tags (seed is event ID)
        result, err = l.DB.TraverseThread(seedBytes, q.Depth)
    }

    if err != nil {
        return err
    }

    // Apply inbound_refs filters at each depth level
    if q.HasInboundRefs() {
        for depth := 0; depth <= q.Depth; depth++ {
            inboundKinds := q.InboundKindsAtDepth(depth)
            if len(inboundKinds) == 0 {
                continue
            }

            // For each event/pubkey discovered at this depth, find inbound references
            // This adds reaction events, replies, reposts, etc. to the result
            if err = l.DB.AddInboundRefsToResult(result, depth, inboundKinds); err != nil {
                log.W.F("failed to add inbound refs at depth %d: %v", depth, err)
            }
        }
    }

    // Apply outbound_refs filters at each depth level
    if q.HasOutboundRefs() {
        for depth := 0; depth <= q.Depth; depth++ {
            outboundKinds := q.OutboundKindsAtDepth(depth)
            if len(outboundKinds) == 0 {
                continue
            }

            // For each event discovered at this depth, find outbound references
            // This follows e-tag chains to referenced events
            if err = l.DB.AddOutboundRefsToResult(result, depth, outboundKinds); err != nil {
                log.W.F("failed to add outbound refs at depth %d: %v", depth, err)
            }
        }
    }

    // Generate response based on query type
    if q.HasInboundRefs() || q.HasOutboundRefs() {
        // Response with refs: send metadata + sorted aggregations
        l.sendGraphResponseWithRefs(subID, q, result)
    } else {
        // Simple response: send events in ascending depth order
        l.sendGraphResponseSimple(subID, q, result, f)
    }

    return nil
}

// sendGraphResponseSimple sends pubkey events in ascending depth order
func (l *Listener) sendGraphResponseSimple(subID []byte, q *graph.Query, result *database.GraphResult, f *filter.F) {
    // Optional: send metadata event first
    metaEvent := l.createGraphMetaEvent(q, result)
    if res, err := eventenvelope.NewResultWith(subID, metaEvent); err == nil {
        res.Write(l)
    }

    // Send events in ascending depth order
    for _, depth := range result.GetDepthsSorted() {
        pubkeys := result.PubkeysByDepth[depth]
        for _, pubkeyHex := range pubkeys {
            pubkeyBytes, _ := hex.Dec(pubkeyHex)

            // Fetch requested event kinds for this pubkey
            if f.Kinds != nil && f.Kinds.Len() > 0 {
                events, _ := l.DB.QueryEventsForAuthor(pubkeyBytes, f.Kinds.ToUint16(), f.Limit)
                for _, ev := range events {
                    res, _ := eventenvelope.NewResultWith(subID, ev)
                    res.Write(l)
                }
            }
        }
    }
}

// sendGraphResponseWithRefs sends metadata + ref aggregations sorted by count descending
func (l *Listener) sendGraphResponseWithRefs(subID []byte, q *graph.Query, result *database.GraphResult) {
    // 1. Send metadata event with summary counts
    metaEvent := l.createGraphMetaEventWithRefs(q, result)
    if res, err := eventenvelope.NewResultWith(subID, metaEvent); err == nil {
        res.Write(l)
    }

    // 2. Send inbound refs, grouped by kind, sorted by count descending
    for _, refSpec := range q.InboundRefs {
        for _, kind := range refSpec.Kinds {
            aggs := result.GetInboundRefsSorted(kind)
            for _, agg := range aggs {
                refEvent := l.createInboundRefEvent(agg)
                if res, err := eventenvelope.NewResultWith(subID, refEvent); err == nil {
                    res.Write(l)
                }
            }
        }
    }

    // 3. Send outbound refs, grouped by kind, sorted by count descending
    for _, refSpec := range q.OutboundRefs {
        for _, kind := range refSpec.Kinds {
            aggs := result.GetOutboundRefsSorted(kind)
            for _, agg := range aggs {
                refEvent := l.createOutboundRefEvent(agg)
                if res, err := eventenvelope.NewResultWith(subID, refEvent); err == nil {
                    res.Write(l)
                }
            }
        }
    }
}

// createGraphMetaEvent creates kind-30078 metadata event for simple queries
func (l *Listener) createGraphMetaEvent(q *graph.Query, result *database.GraphResult) *event.E {
    ev := &event.E{
        Kind: 30078,
        Tags: tag.NewS(
            tag.NewFromAny("d", "graph-meta"),
            tag.NewFromAny("method", q.Method),
            tag.NewFromAny("seed", q.Seed),
            tag.NewFromAny("depth", fmt.Sprintf("%d", q.Depth)),
        ),
    }

    // Add count tags for each depth
    for _, depth := range result.GetDepthsSorted() {
        count := len(result.PubkeysByDepth[depth])
        ev.Tags.Append(tag.NewFromAny("count", fmt.Sprintf("%d", depth), fmt.Sprintf("%d", count)))
    }

    // Sign with relay identity
    l.Server.signRelayEvent(ev)
    return ev
}

// createGraphMetaEventWithRefs creates kind-30078 metadata with ref summaries
func (l *Listener) createGraphMetaEventWithRefs(q *graph.Query, result *database.GraphResult) *event.E {
    ev := l.createGraphMetaEvent(q, result)

    // Add inbound ref summaries
    for kind, refs := range result.InboundRefs {
        total := 0
        for _, refIDs := range refs {
            total += len(refIDs)
        }
        ev.Tags.Append(tag.NewFromAny("inbound", fmt.Sprintf("%d", kind), "total", fmt.Sprintf("%d", total)))
        ev.Tags.Append(tag.NewFromAny("inbound", fmt.Sprintf("%d", kind), "unique_targets", fmt.Sprintf("%d", len(refs))))
    }

    // Add outbound ref summaries
    for kind, refs := range result.OutboundRefs {
        total := 0
        for _, refIDs := range refs {
            total += len(refIDs)
        }
        ev.Tags.Append(tag.NewFromAny("outbound", fmt.Sprintf("%d", kind), "total", fmt.Sprintf("%d", total)))
        ev.Tags.Append(tag.NewFromAny("outbound", fmt.Sprintf("%d", kind), "unique_targets", fmt.Sprintf("%d", len(refs))))
    }

    // Re-sign after adding tags
    l.Server.signRelayEvent(ev)
    return ev
}

// createInboundRefEvent creates kind-30079 event for an inbound ref aggregation
func (l *Listener) createInboundRefEvent(agg database.RefAggregation) *event.E {
    ev := &event.E{
        Kind: 30079,
        Tags: tag.NewS(
            tag.NewFromAny("d", "inbound-refs"),
            tag.NewFromAny("target", agg.TargetEventID),
            tag.NewFromAny("target_author", agg.TargetAuthor),
            tag.NewFromAny("target_depth", fmt.Sprintf("%d", agg.TargetDepth)),
            tag.NewFromAny("ref_kind", fmt.Sprintf("%d", agg.RefKind)),
            tag.NewFromAny("ref_count", fmt.Sprintf("%d", agg.RefCount)),
        ),
    }

    // Add refs tag with all referencing event IDs
    refsTag := tag.NewFromAny("refs")
    for _, refID := range agg.RefEventIDs {
        refsTag.Append([]byte(refID))
    }
    ev.Tags.Append(refsTag)

    l.Server.signRelayEvent(ev)
    return ev
}

// createOutboundRefEvent creates kind-30080 event for an outbound ref aggregation
func (l *Listener) createOutboundRefEvent(agg database.RefAggregation) *event.E {
    ev := &event.E{
        Kind: 30080,
        Tags: tag.NewS(
            tag.NewFromAny("d", "outbound-refs"),
            tag.NewFromAny("source", agg.TargetEventID),
            tag.NewFromAny("source_author", agg.TargetAuthor),
            tag.NewFromAny("source_depth", fmt.Sprintf("%d", agg.TargetDepth)),
            tag.NewFromAny("ref_kind", fmt.Sprintf("%d", agg.RefKind)),
            tag.NewFromAny("ref_count", fmt.Sprintf("%d", agg.RefCount)),
        ),
    }

    // Add refs tag with all referenced event IDs
    refsTag := tag.NewFromAny("refs")
    for _, refID := range agg.RefEventIDs {
        refsTag.Append([]byte(refID))
    }
    ev.Tags.Append(refsTag)

    l.Server.signRelayEvent(ev)
    return ev
}
```

### 3.4 Filter Package Extension

The nostr library's `filter.F` struct needs to preserve unknown JSON fields. This requires a change to `git.mleku.dev/mleku/nostr/encoders/filter`:

**Option A: Add Extra field to filter.F**

```go
type F struct {
    Ids     *tag.T           `json:"ids,omitempty"`
    Kinds   *kind.S          `json:"kinds,omitempty"`
    Authors *tag.T           `json:"authors,omitempty"`
    Tags    *TagFilter       `json:"#,omitempty"`
    Since   *timestamp.T     `json:"since,omitempty"`
    Until   *timestamp.T     `json:"until,omitempty"`
    Limit   *int             `json:"limit,omitempty"`
    Search  string           `json:"search,omitempty"`

    // Extra preserves unknown fields for extensions like _graph
    Extra   map[string]json.RawMessage `json:"-"`
}

func (f *F) UnmarshalJSON(data []byte) error {
    // First unmarshal known fields
    type Alias F
    aux := &struct{ *Alias }{Alias: (*Alias)(f)}
    if err := json.Unmarshal(data, aux); err != nil {
        return err
    }

    // Then capture unknown fields
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }

    // Remove known fields
    knownFields := []string{"ids", "kinds", "authors", "since", "until", "limit", "search"}
    for _, k := range knownFields {
        delete(raw, k)
    }
    // Remove tag fields (start with #)
    for k := range raw {
        if strings.HasPrefix(k, "#") {
            delete(raw, k)
        }
    }

    f.Extra = raw
    return nil
}
```

**Option B: Handle at relay level**

Parse the raw JSON before passing to filter.Unmarshal and extract `_graph` first.

---

## Phase 4: Response Format ✅ COMPLETE

### 4.1 Implemented Response Format

Graph queries return relay-signed events with the following structure:

**Kind 39000 (follows/followers):**
```json
{
    "kind": 39000,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "follows"],
        ["seed", "<seed_pubkey_hex>"],
        ["depth", "2"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"pk1\",\"pk2\"],[\"pk3\",\"pk4\"]],\"total_pubkeys\":4}",
    "sig": "<relay_signature>"
}
```

**Kind 39001 (mentions):**
```json
{
    "kind": 39001,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "mentions"],
        ["seed", "<seed_pubkey_hex>"],
        ["depth", "1"]
    ],
    "content": "{\"events_by_depth\":[[\"ev1\",\"ev2\"]],\"total_events\":2}",
    "sig": "<relay_signature>"
}
```

**Kind 39002 (thread):**
```json
{
    "kind": 39002,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "thread"],
        ["seed", "<seed_event_id_hex>"],
        ["depth", "10"]
    ],
    "content": "{\"events_by_depth\":[[\"reply1\"],[\"reply2\"]],\"total_events\":2}",
    "sig": "<relay_signature>"
}
```

### 4.2 Legacy Response Format (Not Implemented)

The following format was originally planned but NOT implemented:

---

## Phase 5: Migration

### 5.1 Backfill E-Tag Graph

**File: `pkg/database/migrations.go`** (addition)

```go
func (d *D) MigrateETagGraph() error {
    log.I.F("Starting e-tag graph backfill migration...")

    var processed, edges int

    // Iterate all events
    err := d.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        it := txn.NewIterator(opts)
        defer it.Close()

        // Scan compact events
        prefix := []byte(indexes.CompactEventPrefix)

        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            // Decode event
            // Extract e-tags
            // Create graph edges
            processed++
            if processed%10000 == 0 {
                log.I.F("Migration progress: %d events, %d edges", processed, edges)
            }
        }
        return nil
    })

    log.I.F("E-tag graph migration complete: %d events, %d edges", processed, edges)
    return err
}
```

---

## Implementation Order

### Sprint 1: E-Tag Graph Index ✅ COMPLETE
1. Add index key definitions (`indexes/keys.go`)
2. Add direction constants (`indexes/types/letter.go`)
3. Implement `GetEventSerialByID` helper
4. Update `SaveEvent` to create e-tag graph edges
5. Write tests for e-tag graph creation

### Sprint 2: Graph Traversal Functions ✅ COMPLETE
1. Implement `GetPTagsFromEventSerial`
2. Implement `GetETagsFromEventSerial`
3. Implement `GetReferencingEvents`
4. Implement `GetFollowsFromPubkeySerial` / `GetFollowersOfPubkeySerial`
5. Implement `GetPubkeyHexFromSerial` / `GetEventIDFromSerial`
6. Write comprehensive tests

### Sprint 3: High-Level Traversals ✅ COMPLETE
1. Implement `TraverseFollows` with early termination
2. Implement `TraverseFollowers`
3. Implement `FindMentions`, `TraverseThread`
4. Implement `GraphResult` struct with depth arrays

### Sprint 4: REQ Extension & Response Generation ✅ COMPLETE
1. Create `graph.Query` struct and validation (`pkg/protocol/graph/query.go`)
2. Implement `graph.Executor` for query execution (`pkg/protocol/graph/executor.go`)
3. Create `GraphAdapter` for database interface (`pkg/database/graph-adapter.go`)
4. Implement graph query handling in `handle-req.go`
5. Generate relay-signed response events (kinds 39000, 39001, 39002)

### Sprint 5: Migration & Configuration (Pending)
1. Implement e-tag graph backfill migration
2. Add configuration flags (`ORLY_GRAPH_QUERIES_ENABLED`, `ORLY_GRAPH_MAX_DEPTH`)
3. NIP-11 advertisement of graph query support
4. Performance testing on large datasets

---

## Example Queries

### 1. Find Alice's 2-hop follow network

```json
["REQ", "follows", {
    "_graph": {
        "method": "follows",
        "seed": "alice_pubkey_hex",
        "depth": 2
    },
    "kinds": [0]
}]
```

**Returns**: Kind-0 (profile metadata) events for all pubkeys in Alice's 2-hop follow network.

### 2. Find reactions AND reposts to posts by Alice's follows

```json
["REQ", "reactions", {
    "_graph": {
        "method": "follows",
        "seed": "alice_pubkey_hex",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1},
            {"kinds": [6], "from_depth": 1}
        ]
    },
    "kinds": [0]
}]
```

**Returns**:
- Kind-0 profiles for Alice's follows (depth 1)
- Kind-7 reaction events referencing posts by those follows
- Kind-6 repost events referencing posts by those follows

The AND semantics mean: find events that are BOTH reactions (kind-7) AND reposts (kind-6) to the discovered posts. If you want OR semantics, combine the kinds in a single RefSpec:

```json
"inbound_refs": [
    {"kinds": [6, 7], "from_depth": 1}
]
```

### 3. Find who follows Alice (inbound follows)

```json
["REQ", "followers", {
    "_graph": {
        "method": "followers",
        "seed": "alice_pubkey_hex",
        "depth": 1
    },
    "kinds": [0]
}]
```

**Returns**: Kind-0 profile events for everyone who has Alice in their follow list.

### 4. Traverse a thread with all replies

```json
["REQ", "thread", {
    "_graph": {
        "method": "thread",
        "seed": "root_event_id_hex",
        "depth": 10,
        "inbound_refs": [
            {"kinds": [1], "from_depth": 0}
        ]
    }
}]
```

**Returns**: All kind-1 reply events in the thread, up to 10 levels deep.

### 5. Complex query: Follow network + reactions + replies

```json
["REQ", "social", {
    "_graph": {
        "method": "follows",
        "seed": "alice_pubkey_hex",
        "depth": 2,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1},
            {"kinds": [1], "from_depth": 2}
        ],
        "outbound_refs": [
            {"kinds": [1], "from_depth": 1}
        ]
    },
    "kinds": [0, 1]
}]
```

**Returns**:
- Kind-0 profiles for Alice's 2-hop follow network
- Kind-1 notes authored by those pubkeys
- Kind-7 reactions to posts at depth 1+ (Alice's direct follows)
- Kind-1 replies to posts at depth 2+ (friends of friends)
- Kind-1 events referenced BY posts at depth 1+ (what they're replying to)

---

## Performance Expectations

| Query Type | Depth | Expected Latency | Without Graph Index |
|------------|-------|------------------|---------------------|
| follows    | 1     | ~10ms            | ~100ms (decode k3)  |
| follows    | 2     | ~100ms           | ~5-10s              |
| follows    | 3     | ~500ms           | ~minutes            |
| reactions  | 1     | ~50ms            | ~500ms              |
| thread     | 10    | ~100ms           | ~1s                 |

The key insight: **No event decoding required** for graph traversal. All operations are pure index scans with 5-byte serial lookups.
