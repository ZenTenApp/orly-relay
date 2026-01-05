# Graph Queries: Remaining Implementation Plan

> Consolidated plan based on NIP-XX-GRAPH-QUERIES.md spec, existing implementation plans, and codebase analysis.

## Current Status Summary

| Component | Status | Notes |
|-----------|--------|-------|
| Filter extension parsing (`_graph`) | ✅ COMPLETE | Phase 0 |
| E-tag graph index (eeg/gee) | ✅ COMPLETE | Phase 1 - indexes populated on new events |
| Graph traversal primitives | ✅ COMPLETE | Phase 2 |
| High-level traversals (follows, followers, mentions, thread) | ✅ COMPLETE | Phase 3 |
| Query handler + relay-signed responses | ✅ COMPLETE | Phase 4 |
| **Reference aggregation (Badger)** | ✅ COMPLETE | Phase 5C - `pkg/database/graph-refs.go` |
| **Reference aggregation (Neo4j)** | ✅ COMPLETE | Phase 5C - `pkg/neo4j/graph-refs.go` |
| **E-tag graph backfill migration** | ✅ COMPLETE | Phase 5B - Migration v8 in `pkg/database/migrations.go` |
| **Configuration options** | ✅ COMPLETE | Phase 5A - `app/config/config.go` |
| **NIP-11 advertisement** | ✅ COMPLETE | Phase 5A - `app/handle-relayinfo.go` |
| **P-tag graph query optimization** | ❌ NOT IMPLEMENTED | Enhancement - lower priority |

---

## Phase 5A: Configuration & NIP-11 Advertisement

**Goal**: Allow relays to configure graph query support and advertise capabilities.

### 5A.1 Configuration Options

**File**: `app/config/config.go` or environment variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ORLY_GRAPH_QUERIES_ENABLED` | bool | true | Enable/disable graph queries |
| `ORLY_GRAPH_MAX_DEPTH` | int | 16 | Maximum traversal depth |
| `ORLY_GRAPH_RATE_LIMIT` | int | 10 | Queries per minute per connection |
| `ORLY_GRAPH_MAX_RESULTS` | int | 10000 | Maximum pubkeys/events per response |

**Implementation**:

```go
// pkg/config/graph.go
type GraphConfig struct {
    Enabled    bool `env:"ORLY_GRAPH_QUERIES_ENABLED" default:"true"`
    MaxDepth   int  `env:"ORLY_GRAPH_MAX_DEPTH" default:"16"`
    RateLimit  int  `env:"ORLY_GRAPH_RATE_LIMIT" default:"10"`
    MaxResults int  `env:"ORLY_GRAPH_MAX_RESULTS" default:"10000"`
}
```

**Server integration** (`app/server.go`):

```go
// Check config before initializing executor
if config.Graph.Enabled {
    l.graphExecutor, err = graph.NewExecutor(graphAdapter, relaySecretKey, config.Graph)
}
```

### 5A.2 NIP-11 Advertisement

**File**: `app/handle-relayinfo.go`

**Changes required**:

```go
// In buildRelayInfo():
if s.graphExecutor != nil {
    info.SupportedNips = append(info.SupportedNips, "XX") // Or appropriate NIP number
    info.Limitation.GraphQueryMaxDepth = config.Graph.MaxDepth
}

// Add to RelayInfo struct or use Software field:
type RelayInfo struct {
    // ... existing fields
    Limitation struct {
        // ... existing limitations
        GraphQueryMaxDepth int `json:"graph_query_max_depth,omitempty"`
    } `json:"limitation,omitempty"`
}
```

**Example NIP-11 output**:

```json
{
    "supported_nips": [1, "XX"],
    "limitation": {
        "graph_query_max_depth": 16
    }
}
```

---

## Phase 5B: E-Tag Graph Backfill Migration

**Goal**: Populate e-tag graph indexes (eeg/gee) for events stored before graph feature was added.

### 5B.1 Migration Implementation

**File**: `pkg/database/migration-etag-graph.go`

```go
package database

import (
    "bytes"

    "github.com/dgraph-io/badger/v4"
    "next.orly.dev/pkg/database/indexes"
    "next.orly.dev/pkg/database/indexes/types"
)

// MigrateETagGraph backfills e-tag graph edges for existing events
// This is safe to run multiple times (idempotent)
func (d *D) MigrateETagGraph() error {
    log.I.F("Starting e-tag graph backfill migration...")

    var processed, edges, skipped int
    batchSize := 1000
    batch := make([]eTagEdge, 0, batchSize)

    // Iterate all events using serial-event index (sei)
    err := d.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = true
        it := txn.NewIterator(opts)
        defer it.Close()

        prefix := indexes.NewPrefix(indexes.SerialEvent).Bytes()

        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            item := it.Item()

            // Decode event serial from key
            key := item.Key()
            sourceSer := new(types.Uint40)
            dec := indexes.SerialEventDec(sourceSer)
            if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                skipped++
                continue
            }

            // Get event data
            var ev event.T
            if err := item.Value(func(val []byte) error {
                return ev.UnmarshalCompact(val)
            }); err != nil {
                skipped++
                continue
            }

            // Get event kind
            eventKind := new(types.Uint16)
            eventKind.Set(uint16(ev.Kind))

            // Extract e-tags
            for _, eTag := range ev.Tags.GetAll([]byte("e")) {
                if eTag.Len() < 2 {
                    continue
                }

                targetID, err := hex.Dec(string(eTag.Value()))
                if err != nil || len(targetID) != 32 {
                    continue
                }

                // Look up target event serial
                targetSer, err := d.GetEventSerialByID(targetID)
                if err != nil || targetSer == nil {
                    // Target event doesn't exist in our relay - skip
                    continue
                }

                batch = append(batch, eTagEdge{
                    sourceSer: sourceSer,
                    targetSer: targetSer,
                    kind:      eventKind,
                })
            }

            // Flush batch if full
            if len(batch) >= batchSize {
                if err := d.writeETagEdges(batch); err != nil {
                    return err
                }
                edges += len(batch)
                batch = batch[:0]
            }

            processed++
            if processed%10000 == 0 {
                log.I.F("Migration progress: %d events, %d edges, %d skipped", processed, edges, skipped)
            }
        }
        return nil
    })

    // Flush remaining batch
    if len(batch) > 0 {
        if err := d.writeETagEdges(batch); err != nil {
            return err
        }
        edges += len(batch)
    }

    log.I.F("E-tag graph migration complete: %d events, %d edges, %d skipped", processed, edges, skipped)
    return err
}

type eTagEdge struct {
    sourceSer *types.Uint40
    targetSer *types.Uint40
    kind      *types.Uint16
}

func (d *D) writeETagEdges(edges []eTagEdge) error {
    return d.Update(func(txn *badger.Txn) error {
        for _, edge := range edges {
            // Forward edge: eeg|source|target|kind|direction(0)
            keyBuf := new(bytes.Buffer)
            dirOut := new(types.Letter)
            dirOut.Set(types.EdgeDirectionETagOut)
            if err := indexes.EventEventGraphEnc(edge.sourceSer, edge.targetSer, edge.kind, dirOut).MarshalWrite(keyBuf); err != nil {
                return err
            }
            if err := txn.Set(keyBuf.Bytes(), nil); err != nil {
                return err
            }

            // Reverse edge: gee|target|kind|direction(1)|source
            keyBuf.Reset()
            dirIn := new(types.Letter)
            dirIn.Set(types.EdgeDirectionETagIn)
            if err := indexes.GraphEventEventEnc(edge.targetSer, edge.kind, dirIn, edge.sourceSer).MarshalWrite(keyBuf); err != nil {
                return err
            }
            if err := txn.Set(keyBuf.Bytes(), nil); err != nil {
                return err
            }
        }
        return nil
    })
}
```

### 5B.2 CLI Integration

**File**: `cmd/migrate.go` (or add to existing migration command)

```go
// Add migration subcommand
func runETagGraphMigration(cmd *cobra.Command, args []string) error {
    db, err := database.Open(cfg.DataDir)
    if err != nil {
        return err
    }
    defer db.Close()

    return db.MigrateETagGraph()
}
```

**Usage**:

```bash
./orly migrate --backfill-etag-graph
# OR
./orly migrate etag-graph
```

---

## Phase 5C: Reference Aggregation (inbound_refs / outbound_refs)

**Goal**: Implement the `inbound_refs` and `outbound_refs` query parameters per NIP spec.

### Spec Requirements

From NIP-XX-GRAPH-QUERIES.md:

```json
{
    "_graph": {
        "method": "follows",
        "seed": "<pubkey>",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1}
        ]
    }
}
```

**Semantics**:
- `inbound_refs`: Find events that **reference** discovered events (reactions, replies, reposts)
- `outbound_refs`: Find events **referenced by** discovered events (what posts are replying to)
- Multiple `ref_spec` items have **AND** semantics
- Multiple `kinds` within a single `ref_spec` have **OR** semantics
- Results sorted by count **descending** (most referenced first)

### 5C.1 Extend Query Struct

**File**: `pkg/protocol/graph/query.go`

Already defined but needs execution:

```go
type RefSpec struct {
    Kinds     []uint16 `json:"kinds"`
    FromDepth int      `json:"from_depth,omitempty"`
}

type Query struct {
    // ... existing fields
    InboundRefs  []RefSpec `json:"inbound_refs,omitempty"`
    OutboundRefs []RefSpec `json:"outbound_refs,omitempty"`
}
```

### 5C.2 Implement Reference Collection

**File**: `pkg/database/graph-refs.go`

```go
// CollectInboundRefs finds events that reference events authored by the discovered pubkeys
// For each depth level, finds inbound e-tag references of specified kinds
// Returns aggregated counts sorted by popularity (descending)
func (d *D) CollectInboundRefs(
    result *GraphResult,
    refSpecs []RefSpec,
    maxDepth int,
) error {
    if result.InboundRefs == nil {
        result.InboundRefs = make(map[uint16]map[string][]string)
    }

    // For each depth level
    for depth := 0; depth <= maxDepth; depth++ {
        // Determine which ref specs apply at this depth
        var applicableKinds []uint16
        for _, spec := range refSpecs {
            if depth >= spec.FromDepth {
                applicableKinds = append(applicableKinds, spec.Kinds...)
            }
        }

        if len(applicableKinds) == 0 {
            continue
        }

        // Get pubkeys at this depth
        var pubkeySerials []*types.Uint40
        if depth == 0 {
            // depth 0 = seed pubkey
            seedSerial, _ := d.PubkeyHexToSerial(result.SeedPubkey)
            if seedSerial != nil {
                pubkeySerials = []*types.Uint40{seedSerial}
            }
        } else {
            pubkeys := result.PubkeysByDepth[depth]
            for _, pkHex := range pubkeys {
                ser, _ := d.PubkeyHexToSerial(pkHex)
                if ser != nil {
                    pubkeySerials = append(pubkeySerials, ser)
                }
            }
        }

        // For each pubkey, find their events, then find references to those events
        for _, pkSerial := range pubkeySerials {
            // Get events authored by this pubkey
            authoredEvents, err := d.GetEventsAuthoredByPubkey(pkSerial, nil)
            if err != nil {
                continue
            }

            for _, eventSerial := range authoredEvents {
                eventIDHex, _ := d.GetEventIDFromSerial(eventSerial)
                if eventIDHex == "" {
                    continue
                }

                // Find inbound references (events that reference this event)
                refSerials, err := d.GetReferencingEvents(eventSerial, applicableKinds)
                if err != nil {
                    continue
                }

                for _, refSerial := range refSerials {
                    refIDHex, _ := d.GetEventIDFromSerial(refSerial)
                    if refIDHex == "" {
                        continue
                    }

                    // Get the kind of the referencing event
                    refKind, _ := d.GetEventKindFromSerial(refSerial)

                    // Add to aggregation
                    if result.InboundRefs[refKind] == nil {
                        result.InboundRefs[refKind] = make(map[string][]string)
                    }
                    result.InboundRefs[refKind][eventIDHex] = append(
                        result.InboundRefs[refKind][eventIDHex],
                        refIDHex,
                    )
                }
            }
        }
    }

    return nil
}

// CollectOutboundRefs finds events referenced BY events at each depth
// (following e-tag chains to find what posts are being replied to)
func (d *D) CollectOutboundRefs(
    result *GraphResult,
    refSpecs []RefSpec,
    maxDepth int,
) error {
    if result.OutboundRefs == nil {
        result.OutboundRefs = make(map[uint16]map[string][]string)
    }

    // Similar implementation to CollectInboundRefs but using GetETagsFromEventSerial
    // to follow outbound references instead of GetReferencingEvents for inbound
    // ...

    return nil
}
```

### 5C.3 Response Generation with Refs

**File**: `pkg/protocol/graph/executor.go`

Add ref aggregation support to response:

```go
func (e *Executor) Execute(q *Query) (*event.T, error) {
    // ... existing traversal code ...

    // Collect references if specified
    if len(q.InboundRefs) > 0 {
        if err := e.db.CollectInboundRefs(result, q.InboundRefs, q.Depth); err != nil {
            return nil, fmt.Errorf("inbound refs: %w", err)
        }
    }

    if len(q.OutboundRefs) > 0 {
        if err := e.db.CollectOutboundRefs(result, q.OutboundRefs, q.Depth); err != nil {
            return nil, fmt.Errorf("outbound refs: %w", err)
        }
    }

    // Build response content with refs included
    content := ResponseContent{
        PubkeysByDepth: result.ToDepthArrays(),
        TotalPubkeys:   result.TotalPubkeys,
    }

    // Add ref summaries if present
    if len(result.InboundRefs) > 0 || len(result.OutboundRefs) > 0 {
        content.InboundRefSummary = buildRefSummary(result.InboundRefs)
        content.OutboundRefSummary = buildRefSummary(result.OutboundRefs)
    }

    // ... rest of response generation ...
}

type ResponseContent struct {
    PubkeysByDepth     [][]string `json:"pubkeys_by_depth,omitempty"`
    EventsByDepth      [][]string `json:"events_by_depth,omitempty"`
    TotalPubkeys       int        `json:"total_pubkeys,omitempty"`
    TotalEvents        int        `json:"total_events,omitempty"`
    InboundRefSummary  []RefSummary `json:"inbound_refs,omitempty"`
    OutboundRefSummary []RefSummary `json:"outbound_refs,omitempty"`
}

type RefSummary struct {
    Kind          uint16   `json:"kind"`
    TargetEventID string   `json:"target"`
    RefCount      int      `json:"count"`
    RefEventIDs   []string `json:"refs,omitempty"` // Optional: include actual ref IDs
}

func buildRefSummary(refs map[uint16]map[string][]string) []RefSummary {
    var summaries []RefSummary

    for kind, targets := range refs {
        for targetID, refIDs := range targets {
            summaries = append(summaries, RefSummary{
                Kind:          kind,
                TargetEventID: targetID,
                RefCount:      len(refIDs),
                RefEventIDs:   refIDs,
            })
        }
    }

    // Sort by count descending
    sort.Slice(summaries, func(i, j int) bool {
        return summaries[i].RefCount > summaries[j].RefCount
    })

    return summaries
}
```

---

## Phase 5D: P-Tag Graph Query Optimization

**Goal**: Use the pubkey graph index (peg) for faster `#p` tag queries.

### Spec

From PTAG_GRAPH_OPTIMIZATION.md:

When a filter has `#p` tags (mentions/references), use the `peg` index instead of the `tkc` (TagKind) index for:
- 41% smaller index size
- No hash collisions (exact serial match vs 8-byte hash)
- Kind-indexed in key structure

### 5D.1 Query Optimization

**File**: `pkg/database/query-for-ptag-graph.go`

```go
package database

// canUsePTagGraph checks if a filter can use graph optimization
func canUsePTagGraph(f *filter.F) bool {
    // Has p-tags?
    if f.Tags == nil || f.Tags.Len() == 0 {
        return false
    }

    hasPTags := false
    for _, t := range *f.Tags {
        if len(t.Key()) >= 1 && t.Key()[0] == 'p' {
            hasPTags = true
            break
        }
    }
    if !hasPTags {
        return false
    }

    // No authors filter (use different optimization for that)
    if f.Authors != nil && f.Authors.Len() > 0 {
        return false
    }

    return true
}

// QueryPTagGraph uses the pubkey graph index for efficient p-tag queries
func (d *D) QueryPTagGraph(f *filter.F) (serials types.Uint40s, err error) {
    // Extract p-tags from filter
    var ptagPubkeys [][]byte
    for _, t := range *f.Tags {
        if len(t.Key()) >= 1 && t.Key()[0] == 'p' {
            for _, val := range t.Values() {
                pubkeyBytes, err := hex.Dec(string(val))
                if err == nil && len(pubkeyBytes) == 32 {
                    ptagPubkeys = append(ptagPubkeys, pubkeyBytes)
                }
            }
        }
    }

    if len(ptagPubkeys) == 0 {
        return nil, nil
    }

    // Resolve pubkeys to serials
    var pubkeySerials []*types.Uint40
    for _, pk := range ptagPubkeys {
        ser, err := d.GetPubkeySerial(pk)
        if err == nil && ser != nil {
            pubkeySerials = append(pubkeySerials, ser)
        }
    }

    // Query kinds (optional)
    var kinds []uint16
    if f.Kinds != nil {
        kinds = f.Kinds.ToUint16()
    }

    // Scan peg index for each pubkey
    seen := make(map[uint64]bool)
    for _, pkSerial := range pubkeySerials {
        // peg|pubkey_serial|kind|direction(2)|event_serial
        // direction=2 means "inbound p-tag" (this pubkey is referenced)
        eventSerials, err := d.GetEventsReferencingPubkey(pkSerial, kinds)
        if err != nil {
            continue
        }

        for _, evSer := range eventSerials {
            if !seen[evSer.Uint64()] {
                seen[evSer.Uint64()] = true
                serials = append(serials, evSer)
            }
        }
    }

    return serials, nil
}

// GetEventsReferencingPubkey finds events that have p-tag referencing this pubkey
// Uses peg index with direction=2 (p-tag inbound)
func (d *D) GetEventsReferencingPubkey(pubkeySerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
    var eventSerials []*types.Uint40

    err := d.View(func(txn *badger.Txn) error {
        if len(kinds) > 0 {
            // Scan specific kinds
            for _, k := range kinds {
                kind := new(types.Uint16)
                kind.Set(k)
                direction := new(types.Letter)
                direction.Set(types.EdgeDirectionPTagIn) // direction=2

                prefix := new(bytes.Buffer)
                indexes.PubkeyEventGraphEnc(pubkeySerial, kind, direction, nil).MarshalWrite(prefix)

                opts := badger.DefaultIteratorOptions
                opts.PrefetchValues = false
                it := txn.NewIterator(opts)

                for it.Seek(prefix.Bytes()); it.ValidForPrefix(prefix.Bytes()); it.Next() {
                    key := it.Item().Key()

                    _, _, _, evSer := indexes.PubkeyEventGraphVars()
                    dec := indexes.PubkeyEventGraphDec(new(types.Uint40), new(types.Uint16), new(types.Letter), evSer)
                    if err := dec.UnmarshalRead(bytes.NewReader(key)); err != nil {
                        continue
                    }

                    ser := new(types.Uint40)
                    ser.Set(evSer.Uint64())
                    eventSerials = append(eventSerials, ser)
                }
                it.Close()
            }
        } else {
            // Scan all kinds (direction=2 only)
            direction := new(types.Letter)
            direction.Set(types.EdgeDirectionPTagIn)

            // Need to scan all kinds for this pubkey with direction=2
            // This is less efficient - recommend always filtering by kinds
            // ... implementation
        }
        return nil
    })

    return eventSerials, err
}
```

### 5D.2 Query Dispatcher Integration

**File**: `pkg/database/query.go` (or equivalent)

```go
func (d *D) GetSerialsFromFilter(f *filter.F) (sers types.Uint40s, err error) {
    // Try p-tag graph optimization first
    if canUsePTagGraph(f) {
        if sers, err = d.QueryPTagGraph(f); err == nil && len(sers) > 0 {
            return sers, nil
        }
        // Fall through to traditional indexes on error or empty result
    }

    // Existing index selection logic...
}
```

---

## Implementation Priority Order

### Critical Path (Completes NIP Spec)

1. **Phase 5C: Reference Aggregation** - Required by NIP spec for full feature parity
   - Estimated: Medium complexity
   - Value: High (enables reaction/reply counts, popular post discovery)

2. **Phase 5A: Configuration & NIP-11** - Needed for relay discoverability
   - Estimated: Low complexity
   - Value: Medium (allows clients to detect support)

### Enhancement Path (Performance & Operations)

3. **Phase 5B: E-Tag Graph Backfill** - Enables graph queries on historical data
   - Estimated: Medium complexity
   - Value: Medium (relays with existing data need this)

4. **Phase 5D: P-Tag Graph Optimization** - Performance improvement
   - Estimated: Low-Medium complexity
   - Value: Medium (3-5x faster for mention queries)

---

## Testing Plan

### Unit Tests

```go
// Reference aggregation
func TestCollectInboundRefs(t *testing.T)
func TestCollectOutboundRefs(t *testing.T)
func TestRefSummarySorting(t *testing.T)

// Configuration
func TestGraphConfigDefaults(t *testing.T)
func TestGraphConfigEnvOverrides(t *testing.T)

// Migration
func TestETagGraphMigration(t *testing.T)
func TestMigrationIdempotency(t *testing.T)

// P-tag optimization
func TestCanUsePTagGraph(t *testing.T)
func TestQueryPTagGraph(t *testing.T)
```

### Integration Tests

```go
// Full round-trip with refs
func TestGraphQueryWithInboundRefs(t *testing.T)
func TestGraphQueryWithOutboundRefs(t *testing.T)
func TestGraphQueryRefsSortedByCount(t *testing.T)

// NIP-11
func TestRelayInfoAdvertisesGraphQueries(t *testing.T)
```

### Performance Tests

```go
// Benchmark ref collection
func BenchmarkCollectInboundRefs(b *testing.B)
func BenchmarkPTagGraphVsTagIndex(b *testing.B)
```

---

## Summary

| Phase | Description | Complexity | Priority | Status |
|-------|-------------|------------|----------|--------|
| **5A** | Configuration & NIP-11 | Low | High | ✅ COMPLETE |
| **5B** | E-tag graph migration | Medium | Medium | ✅ COMPLETE |
| **5C** | Reference aggregation | Medium | High | ✅ COMPLETE |
| **5D** | P-tag optimization | Low-Medium | Medium | ❌ Not started |

**Completed 2025-01-05:**
- Phase 5A: Configuration options (`ORLY_GRAPH_*` env vars) and NIP-11 `graph_query` field
- Phase 5B: E-tag graph backfill migration (v8) runs automatically on startup
- Phase 5C: Inbound/outbound ref collection for both Badger and Neo4j backends

**Remaining:**
- Phase 5D: P-tag graph query optimization (enhancement, not critical)

**Implementation Files:**
- `app/config/config.go` - Graph configuration options
- `app/handle-relayinfo.go` - NIP-11 advertisement
- `pkg/database/migrations.go` - E-tag graph backfill (v8)
- `pkg/database/graph-refs.go` - Badger ref collection
- `pkg/neo4j/graph-refs.go` - Neo4j ref collection
- `pkg/protocol/graph/executor.go` - Query execution with ref support
