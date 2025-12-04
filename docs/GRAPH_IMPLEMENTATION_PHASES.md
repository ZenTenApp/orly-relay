# Graph Query Implementation Phases

This document provides a clear breakdown of implementation phases for NIP-XX Graph Queries.

---

## Phase 0: Filter Extension Parsing (Foundation) ✅ COMPLETE

**Goal**: Enable the nostr library to correctly "ignore" unknown filter fields per NIP-01, while preserving them for relay-level processing.

### Deliverables (Completed)
- [x] Modified `filter.F` struct with `Extra` field
- [x] Modified `Unmarshal()` to skip unknown keys
- [x] `skipJSONValue()` helper function
- [x] `graph.ExtractFromFilter()` function
- [x] Integration in `handle-req.go`
- [x] Rate limiter with token bucket for graph queries

---

## Phase 1: E-Tag Graph Index ✅ COMPLETE

**Goal**: Create bidirectional indexes for event-to-event references (e-tags).

### Index Key Structure

```
Event-Event Graph (Forward): eeg
  eeg|source_event_serial(5)|target_event_serial(5)|kind(2)|direction(1) = 16 bytes

Event-Event Graph (Reverse): gee
  gee|target_event_serial(5)|kind(2)|direction(1)|source_event_serial(5) = 16 bytes
```

### Direction Constants
- `EdgeDirectionETagOut = 0` - Event references another event (outbound)
- `EdgeDirectionETagIn = 1` - Event is referenced by another (inbound)

### Deliverables (Completed)
- [x] Index key definitions for eeg/gee (`pkg/database/indexes/keys.go`)
- [x] Direction constants for e-tags (`pkg/database/indexes/types/letter.go`)
- [x] E-tag graph creation in SaveEvent (`pkg/database/save-event.go`)
- [x] Tests for e-tag graph creation (`pkg/database/etag-graph_test.go`)

**Key Bug Fix**: Buffer reuse in transaction required copying key bytes before writing second key to prevent overwrite.

---

## Phase 2: Graph Traversal Primitives ✅ COMPLETE

**Goal**: Implement pure index-based graph traversal functions.

### 2.1 Core traversal functions

**File**: `pkg/database/graph-traversal.go`

```go
// Core primitives (no event decoding required)
func (d *D) GetPTagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error)
func (d *D) GetETagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error)
func (d *D) GetReferencingEvents(targetSerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error)
func (d *D) GetFollowsFromPubkeySerial(pubkeySerial *types.Uint40) ([]*types.Uint40, error)
func (d *D) GetFollowersOfPubkeySerial(pubkeySerial *types.Uint40) ([]*types.Uint40, error)
func (d *D) GetPubkeyHexFromSerial(serial *types.Uint40) (string, error)
func (d *D) GetEventIDFromSerial(serial *types.Uint40) (string, error)
```

### 2.2 GraphResult struct

**File**: `pkg/database/graph-result.go`

```go
// GraphResult contains depth-organized traversal results
type GraphResult struct {
    PubkeysByDepth  map[int][]string      // depth -> pubkeys first discovered at that depth
    EventsByDepth   map[int][]string      // depth -> events discovered at that depth
    FirstSeenPubkey map[string]int        // pubkey hex -> depth where first seen
    FirstSeenEvent  map[string]int        // event hex -> depth where first seen
    TotalPubkeys    int
    TotalEvents     int
    InboundRefs     map[uint16]map[string][]string  // kind -> target -> []referencing_ids
    OutboundRefs    map[uint16]map[string][]string  // kind -> source -> []referenced_ids
}

func (r *GraphResult) ToDepthArrays() [][]string       // For pubkey results
func (r *GraphResult) ToEventDepthArrays() [][]string  // For event results
func (r *GraphResult) GetInboundRefsSorted(kind uint16) []RefAggregation
func (r *GraphResult) GetOutboundRefsSorted(kind uint16) []RefAggregation
```

### Deliverables (Completed)
- [x] Core traversal functions in `graph-traversal.go`
- [x] GraphResult struct with ToDepthArrays() and ToEventDepthArrays()
- [x] RefAggregation struct with sorted accessors
- [x] Tests in `graph-result_test.go` and `graph-traversal_test.go`

---

## Phase 3: High-Level Traversals ✅ COMPLETE

**Goal**: Implement the graph query methods (follows, followers, mentions, thread).

### 3.1 Follow graph traversal

**File**: `pkg/database/graph-follows.go`

```go
// TraverseFollows performs BFS traversal of the follow graph
// Returns pubkeys grouped by first-discovered depth (no duplicates across depths)
func (d *D) TraverseFollows(seedPubkey []byte, maxDepth int) (*GraphResult, error)

// TraverseFollowers performs BFS traversal to find who follows the seed pubkey
func (d *D) TraverseFollowers(seedPubkey []byte, maxDepth int) (*GraphResult, error)

// Hex convenience wrappers
func (d *D) TraverseFollowsFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error)
func (d *D) TraverseFollowersFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error)
```

### 3.2 Other traversals

**File**: `pkg/database/graph-mentions.go`
```go
func (d *D) FindMentions(pubkey []byte, kinds []uint16) (*GraphResult, error)
func (d *D) FindMentionsFromHex(pubkeyHex string, kinds []uint16) (*GraphResult, error)
func (d *D) FindMentionsByPubkeys(pubkeySerials []*types.Uint40, kinds []uint16) (*GraphResult, error)
```

**File**: `pkg/database/graph-thread.go`
```go
func (d *D) TraverseThread(seedEventID []byte, maxDepth int, direction string) (*GraphResult, error)
func (d *D) TraverseThreadFromHex(seedEventIDHex string, maxDepth int, direction string) (*GraphResult, error)
func (d *D) GetThreadReplies(eventID []byte, kinds []uint16) (*GraphResult, error)
func (d *D) GetThreadParents(eventID []byte) (*GraphResult, error)
```

### 3.3 Ref aggregation

**File**: `pkg/database/graph-refs.go`
```go
func (d *D) AddInboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error
func (d *D) AddOutboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error
func (d *D) CollectRefsForPubkeys(pubkeySerials []*types.Uint40, refKinds []uint16, eventKinds []uint16) (*GraphResult, error)
```

### Deliverables (Completed)
- [x] TraverseFollows with early termination (2 consecutive empty depths)
- [x] TraverseFollowers
- [x] FindMentions and FindMentionsByPubkeys
- [x] TraverseThread with bidirectional traversal
- [x] Inbound/Outbound ref aggregation
- [x] Tests in `graph-follows_test.go`

---

## Phase 4: Graph Query Handler and Response Generation ✅ COMPLETE

**Goal**: Wire up the REQ handler to execute graph queries and generate relay-signed response events.

### 4.1 Response Event Generation

**Key Design Decision**: All graph query responses are returned as **relay-signed events**, enabling:
- Standard client validation (no special handling)
- Result caching and storage on relays
- Cryptographic proof of origin

### 4.2 Response Kinds (Implemented)

| Kind | Name | Description |
|------|------|-------------|
| 39000 | Graph Follows | Response for follows/followers queries |
| 39001 | Graph Mentions | Response for mentions queries |
| 39002 | Graph Thread | Response for thread traversal queries |

### 4.3 Implementation Files

**New files:**
- `pkg/protocol/graph/executor.go` - Executes graph queries and generates signed responses
- `pkg/database/graph-adapter.go` - Adapts database to `graph.GraphDatabase` interface

**Modified files:**
- `app/server.go` - Added `graphExecutor` field
- `app/main.go` - Initialize graph executor on startup
- `app/handle-req.go` - Execute graph queries and return results

### 4.4 Response Format (Implemented)

The response is a relay-signed event with JSON content:

```go
type ResponseContent struct {
    PubkeysByDepth [][]string `json:"pubkeys_by_depth,omitempty"`
    EventsByDepth  [][]string `json:"events_by_depth,omitempty"`
    TotalPubkeys   int        `json:"total_pubkeys,omitempty"`
    TotalEvents    int        `json:"total_events,omitempty"`
}
```

**Example response event:**
```json
{
    "kind": 39000,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": 1704067200,
    "tags": [
        ["method", "follows"],
        ["seed", "<seed_pubkey_hex>"],
        ["depth", "2"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"pk1\",\"pk2\"],[\"pk3\",\"pk4\"]],\"total_pubkeys\":4}",
    "sig": "<relay_signature>"
}

### Deliverables (Completed)
- [x] Graph executor with query routing (`pkg/protocol/graph/executor.go`)
- [x] Response event generation with relay signature
- [x] GraphDatabase interface and adapter
- [x] Integration in `handle-req.go`
- [x] All tests passing

---

## Phase 5: Migration & Configuration

**Goal**: Enable backfilling and configuration.

### 5.1 E-tag graph backfill migration

**File**: `pkg/database/migrations.go`

```go
func (d *D) MigrateETagGraph() error {
    // Iterate all events
    // Extract e-tags
    // Create eeg/gee edges for targets that exist
}
```

### 5.2 Configuration

**File**: `app/config/config.go`

Add:
- `ORLY_GRAPH_QUERIES_ENABLED` - enable/disable feature
- `ORLY_GRAPH_MAX_DEPTH` - maximum traversal depth (default 16)
- `ORLY_GRAPH_RATE_LIMIT` - queries per minute per connection

### 5.3 NIP-11 advertisement

Update relay info document to advertise support and limits.

### Deliverables
- [ ] Backfill migration
- [ ] Configuration options
- [ ] NIP-11 advertisement
- [ ] Documentation updates

---

## Summary: Implementation Order

| Phase | Description | Status | Dependencies |
|-------|-------------|--------|--------------|
| **0** | Filter extension parsing | ✅ Complete | None |
| **1** | E-tag graph index | ✅ Complete | Phase 0 |
| **2** | Graph traversal primitives | ✅ Complete | Phase 1 |
| **3** | High-level traversals | ✅ Complete | Phase 2 |
| **4** | Graph query handler | ✅ Complete | Phase 3 |
| **5** | Migration & configuration | Pending | Phase 4 |

---

## Response Format Summary

### Graph-Only Query (no kinds filter)

**Request:**
```json
["REQ", "sub", {"_graph": {"method": "follows", "seed": "abc...", "depth": 2}}]
```

**Response:** Single signed event with depth arrays
```json
["EVENT", "sub", {
    "kind": 39000,
    "pubkey": "<relay_pubkey>",
    "content": "[[\"depth1_pk1\",\"depth1_pk2\"],[\"depth2_pk3\",\"depth2_pk4\"]]",
    "tags": [["d","follows:abc...:2"],["method","follows"],["seed","abc..."],["depth","2"]],
    "sig": "..."
}]
["EOSE", "sub"]
```

### Query with Event Filters

**Request:**
```json
["REQ", "sub", {"_graph": {"method": "follows", "seed": "abc...", "depth": 2}, "kinds": [0]}]
```

**Response:** Graph result + events in depth order
```
["EVENT", "sub", <kind-39000 graph result>]
["EVENT", "sub", <kind-0 for depth-1 pubkey>]
["EVENT", "sub", <kind-0 for depth-1 pubkey>]
["EVENT", "sub", <kind-0 for depth-2 pubkey>]
...
["EOSE", "sub"]
```

### Query with Reference Aggregation

**Request:**
```json
["REQ", "sub", {"_graph": {"method": "follows", "seed": "abc...", "depth": 1, "inbound_refs": [{"kinds": [7]}]}}]
```

**Response:** Graph result + refs sorted by count (descending)
```
["EVENT", "sub", <kind-39000 with ref summaries>]
["EVENT", "sub", <kind-39001 target with 523 refs>]
["EVENT", "sub", <kind-39001 target with 312 refs>]
["EVENT", "sub", <kind-39001 target with 1 ref>]
["EOSE", "sub"]
```

---

## Testing Strategy

### Unit Tests
- Filter parsing with unknown fields
- Index key encoding/decoding
- Traversal primitives
- Result depth array generation
- Reference sorting

### Integration Tests
- Full graph query round-trip
- Response format validation
- Signature verification
- Backward compatibility (non-graph REQs still work)

### Performance Tests
- Traversal latency at various depths
- Memory usage for large graphs
- Comparison with event-decoding approach
