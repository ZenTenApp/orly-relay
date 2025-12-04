# NIP-XX: Graph Queries

`draft` `optional`

This NIP defines an extension to the REQ message filter that enables efficient social graph traversal queries without requiring clients to fetch and decode large numbers of events.

## Motivation

Nostr's social graph is encoded in event tags:
- **Follow relationships**: Kind-3 events with `p` tags listing followed pubkeys
- **Event references**: `e` tags linking replies, reactions, reposts to their targets
- **Mentions**: `p` tags in any event kind referencing other users

Clients building social features (timelines, notifications, discovery) must currently:
1. Fetch kind-3 events for each user
2. Decode JSON to extract `p` tags
3. Recursively fetch more events for multi-hop queries
4. Aggregate and count references client-side

This is inefficient, especially for:
- **Multi-hop follow graphs** (friends-of-friends)
- **Reaction/reply counts** on posts
- **Thread traversal** for long conversations
- **Follower discovery** (who follows this user?)

Relays with graph-indexed storage can answer these queries orders of magnitude faster by traversing indexes directly without event decoding.

## Protocol Extension

### Filter Extension: `_graph`

The `_graph` field is added to REQ filters. Per NIP-01, unknown fields are ignored by relays that don't support this extension, ensuring backward compatibility.

```json
["REQ", "<subscription_id>", {
    "_graph": {
        "method": "<method>",
        "seed": "<hex>",
        "depth": <number>,
        "inbound_refs": [<ref_spec>, ...],
        "outbound_refs": [<ref_spec>, ...]
    },
    "kinds": [<kind>, ...]
}]
```

### Fields

#### `method` (required)

The graph traversal method to execute:

| Method | Seed Type | Description |
|--------|-----------|-------------|
| `follows` | pubkey | Traverse outbound follow relationships via kind-3 `p` tags |
| `followers` | pubkey | Find pubkeys whose kind-3 events contain `p` tag to seed |
| `mentions` | pubkey | Find events with `p` tag referencing seed pubkey |
| `thread` | event ID | Traverse reply chain via `e` tags |

#### `seed` (required)

64-character hex string. Interpretation depends on `method`:
- For `follows`, `followers`, `mentions`: pubkey hex
- For `thread`: event ID hex

#### `depth` (optional)

Maximum traversal depth. Integer from 1-16. Default: 1.

- `depth: 1` returns direct connections only
- `depth: 2` returns connections and their connections (friends-of-friends)
- Higher depths expand the graph further

**Early termination**: Traversal stops before reaching `depth` if two consecutive depth levels yield no new pubkeys. This prevents unnecessary work when the graph is exhausted.

#### `inbound_refs` (optional)

Array of reference specifications for finding events that **reference** discovered events (via `e` tags). Used to find reactions, replies, reposts, zaps, etc.

```json
"inbound_refs": [
    {"kinds": [7], "from_depth": 1},
    {"kinds": [1, 6], "from_depth": 0}
]
```

#### `outbound_refs` (optional)

Array of reference specifications for finding events **referenced by** discovered events (via `e` tags). Used to find what posts are being replied to, quoted, etc.

```json
"outbound_refs": [
    {"kinds": [1], "from_depth": 1}
]
```

#### Reference Specification (`ref_spec`)

```json
{
    "kinds": [<kind>, ...],
    "from_depth": <number>
}
```

- `kinds`: Event kinds to match (required, non-empty array)
- `from_depth`: Only apply this filter from this depth onwards (optional, default: 0)

**Semantics:**
- Multiple `ref_spec` objects in an array have **AND** semantics (all must match)
- Multiple kinds within a single `ref_spec` have **OR** semantics (any kind matches)
- `from_depth: 0` includes references to/from the seed itself
- `from_depth: 1` starts from first-hop connections

#### `kinds` (standard filter field)

When present alongside `_graph`, specifies which event kinds to return for discovered pubkeys (e.g., kind-0 profiles, kind-1 notes).

## Response Format

### Relay-Signed Result Events

All graph query responses are returned as **signed Nostr events** created by the relay using its identity key. This design provides several benefits:

1. **Standard validation**: Clients validate the response like any normal event - no special handling needed
2. **Caching**: Results can be stored on relays and retrieved later
3. **Transparency**: The relay's pubkey identifies who produced the result
4. **Cryptographic binding**: The signature proves the result came from a specific relay

### Response Kinds

| Kind | Name | Description |
|------|------|-------------|
| 39000 | Graph Follows | Response for follows/followers queries |
| 39001 | Graph Mentions | Response for mentions queries |
| 39002 | Graph Thread | Response for thread traversal queries |

These are application-specific kinds in the 39000-39999 range.

---

## Simple Query Response (graph-only filter)

When a REQ contains **only** the `_graph` field (no `kinds`, `authors`, or other filter fields), the relay returns a single signed event containing the graph traversal results organized by depth.

### Request Format

```json
["REQ", "<sub>", {
    "_graph": {
        "method": "follows",
        "seed": "<pubkey_hex>",
        "depth": 3
    }
}]
```

### Response: Kind 39000 Graph Result Event

```json
{
    "kind": 39000,
    "pubkey": "<relay_identity_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "follows"],
        ["seed", "<seed_hex>"],
        ["depth", "3"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"pubkey1\",\"pubkey2\"],[\"pubkey3\",\"pubkey4\"]],\"total_pubkeys\":4}",
    "id": "<event_id>",
    "sig": "<relay_signature>"
}
```

### Content Structure

The `content` field contains a JSON object with depth arrays:

```json
{
    "pubkeys_by_depth": [
        ["<pubkey_depth_1>", "<pubkey_depth_1>", ...],
        ["<pubkey_depth_2>", "<pubkey_depth_2>", ...],
        ["<pubkey_depth_3>", "<pubkey_depth_3>", ...]
    ],
    "total_pubkeys": 150
}
```

For event-based queries (mentions, thread), the structure is:

```json
{
    "events_by_depth": [
        ["<event_id_depth_1>", ...],
        ["<event_id_depth_2>", ...]
    ],
    "total_events": 42
}
```

**Key properties:**
- **Array index = depth - 1**: Index 0 contains depth-1 pubkeys (direct follows)
- **Unique per depth**: Each pubkey/event appears only at the depth where it was **first discovered**
- **No duplicates**: A pubkey in depth 1 will NOT appear in depth 2 or 3
- **Hex format**: All pubkeys and event IDs are 64-character lowercase hex strings

### Example

Alice follows Bob and Carol. Bob follows Dave. Carol follows Dave and Eve.

Request:
```json
["REQ", "follow-net", {
    "_graph": {
        "method": "follows",
        "seed": "<alice_pubkey>",
        "depth": 2
    }
}]
```

Response:
```json
["EVENT", "follow-net", {
    "kind": 39000,
    "pubkey": "<relay_pubkey>",
    "created_at": 1704067200,
    "tags": [
        ["method", "follows"],
        ["seed", "<alice_pubkey>"],
        ["depth", "2"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"<bob_pubkey>\",\"<carol_pubkey>\"],[\"<dave_pubkey>\",\"<eve_pubkey>\"]],\"total_pubkeys\":4}",
    "sig": "<signature>"
}]
["EOSE", "follow-net"]
```

**Interpretation:**
- Depth 1 (index 0): Bob, Carol (Alice's direct follows)
- Depth 2 (index 1): Dave, Eve (friends-of-friends, excluding Bob and Carol)
- Note: Dave appears only once even though both Bob and Carol follow Dave

---

## Query with Additional Filters

When the REQ includes both `_graph` AND other filter fields (like `kinds`), the relay:

1. Executes the graph traversal to discover pubkeys
2. Fetches the requested events for those pubkeys
3. Returns events in **ascending depth order**

### Request Format

```json
["REQ", "<sub>", {
    "_graph": {
        "method": "follows",
        "seed": "<pubkey_hex>",
        "depth": 2
    },
    "kinds": [0, 1]
}]
```

### Response

```
["EVENT", "<sub>", <kind-39000 graph result event>]
["EVENT", "<sub>", <kind-0 profile for depth-1 pubkey>]
["EVENT", "<sub>", <kind-1 note for depth-1 pubkey>]
... (all depth-1 events)
["EVENT", "<sub>", <kind-0 profile for depth-2 pubkey>]
["EVENT", "<sub>", <kind-1 note for depth-2 pubkey>]
... (all depth-2 events)
["EOSE", "<sub>"]
```

The graph result event (kind 39000) is sent first, allowing clients to know the complete graph structure before receiving individual events.

---

## Query with Reference Aggregation (Planned)

> **Note:** Reference aggregation is planned for a future implementation phase. The following describes the intended behavior.

When `inbound_refs` or `outbound_refs` are specified, the response will include aggregated reference data **sorted by count descending** (most referenced first).

### Request Format

```json
["REQ", "popular-posts", {
    "_graph": {
        "method": "follows",
        "seed": "<pubkey_hex>",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1}
        ]
    }
}]
```

### Response (Planned)

```
["EVENT", "popular-posts", <kind-39000 graph result with ref summaries>]
["EVENT", "popular-posts", <aggregated ref event with 523 reactions>]
["EVENT", "popular-posts", <aggregated ref event with 312 reactions>]
...
["EVENT", "popular-posts", <aggregated ref event with 1 reaction>]
["EOSE", "popular-posts"]
```

### Kind 39001: Graph Mentions Result

Used for `mentions` queries. Contains events that mention the seed pubkey:

```json
{
    "kind": 39001,
    "pubkey": "<relay_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "mentions"],
        ["seed", "<seed_pubkey_hex>"],
        ["depth", "1"]
    ],
    "content": "{\"events_by_depth\":[[\"<event_id_1>\",\"<event_id_2>\",...]],\"total_events\":42}",
    "sig": "<signature>"
}
```

### Kind 39002: Graph Thread Result

Used for `thread` queries. Contains events in a reply thread:

```json
{
    "kind": 39002,
    "pubkey": "<relay_pubkey>",
    "created_at": <timestamp>,
    "tags": [
        ["method", "thread"],
        ["seed", "<seed_event_id_hex>"],
        ["depth", "10"]
    ],
    "content": "{\"events_by_depth\":[[\"<reply_id_1>\",...],[\"<reply_id_2>\",...]],\"total_events\":156}",
    "sig": "<signature>"
}
```

### Reference Aggregation (Future)

When `inbound_refs` or `outbound_refs` are specified, the response includes aggregated reference data sorted by count descending. This feature is planned for a future implementation phase.

---

## Examples

### Example 1: Get Follow Network (Graph Only)

Get Alice's 2-hop follow network as a single signed event:

```json
["REQ", "follow-network", {
    "_graph": {
        "method": "follows",
        "seed": "abc123...def456",
        "depth": 2
    }
}]
```

**Response:**
```json
["EVENT", "follow-network", {
    "kind": 39000,
    "pubkey": "<relay_pubkey>",
    "tags": [
        ["method", "follows"],
        ["seed", "abc123...def456"],
        ["depth", "2"]
    ],
    "content": "{\"pubkeys_by_depth\":[[\"pub1\",\"pub2\",...150 pubkeys],[\"pub151\",\"pub152\",...3420 pubkeys]],\"total_pubkeys\":3570}",
    "sig": "<signature>"
}]
["EOSE", "follow-network"]
```

The content JSON object contains:
- `pubkeys_by_depth[0]`: 150 pubkeys (depth 1 - direct follows)
- `pubkeys_by_depth[1]`: 3420 pubkeys (depth 2 - friends-of-friends, excluding depth 1)
- `total_pubkeys`: 3570 (total unique pubkeys discovered)

### Example 2: Follow Network with Profiles

```json
["REQ", "follow-profiles", {
    "_graph": {
        "method": "follows",
        "seed": "abc123...def456",
        "depth": 2
    },
    "kinds": [0]
}]
```

**Response:**
```
["EVENT", "follow-profiles", <kind-39000 graph result>]
["EVENT", "follow-profiles", <kind-0 for depth-1 follow>]
... (150 depth-1 profiles)
["EVENT", "follow-profiles", <kind-0 for depth-2 follow>]
... (3420 depth-2 profiles)
["EOSE", "follow-profiles"]
```

### Example 3: Popular Posts by Reactions

Find reactions to posts by Alice's follows, sorted by popularity:

```json
["REQ", "popular-posts", {
    "_graph": {
        "method": "follows",
        "seed": "abc123...def456",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1}
        ]
    }
}]
```

**Response:** Most-reacted posts first, down to posts with only 1 reaction.

### Example 4: Thread Traversal

Fetch a complete reply thread:

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

### Example 5: Who Follows Me?

Find pubkeys that follow Alice:

```json
["REQ", "my-followers", {
    "_graph": {
        "method": "followers",
        "seed": "alice_pubkey_hex",
        "depth": 1
    }
}]
```

**Response:** Single kind-39000 event with follower pubkeys in content.

### Example 6: Reactions AND Reposts (AND semantics)

Find posts with both reactions and reposts:

```json
["REQ", "engaged-posts", {
    "_graph": {
        "method": "follows",
        "seed": "abc123...def456",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [7], "from_depth": 1},
            {"kinds": [6], "from_depth": 1}
        ]
    }
}]
```

This returns only posts that have **both** kind-7 reactions AND kind-6 reposts.

### Example 7: Reactions OR Reposts (OR semantics)

Find posts with either reactions or reposts:

```json
["REQ", "any-engagement", {
    "_graph": {
        "method": "follows",
        "seed": "abc123...def456",
        "depth": 1,
        "inbound_refs": [
            {"kinds": [6, 7], "from_depth": 1}
        ]
    }
}]
```

---

## Client Implementation Notes

### Validating Graph Results

Graph result events are signed by the relay's identity key. Clients should:

1. Verify the signature as with any event
2. Optionally verify the relay pubkey matches the connected relay
3. Parse the `content` JSON to extract depth-organized results

### Caching Results

Because graph results are standard signed events, clients can:

1. Store results locally for offline access
2. Optionally publish results to relays for sharing
3. Use the `method`, `seed`, and `depth` tags to identify equivalent queries
4. Compare `created_at` timestamps to determine freshness

### Trust Considerations

The relay is asserting "this is what the graph looks like from my perspective." Clients may want to:

1. Query multiple relays and compare results
2. Prefer relays they trust for graph queries
3. Use the response as a starting point and verify critical paths independently

---

## Relay Implementation Notes

### Index Requirements

Efficient implementation requires bidirectional graph indexes:

**Pubkey Graph:**
- Event → Pubkey edges (author relationship, `p` tag references)
- Pubkey → Event edges (reverse lookup)

**Event Graph:**
- Event → Event edges (`e` tag references)
- Event → Event reverse edges (what references this event)

Both indexes should include:
- Event kind (for filtering)
- Direction (author vs tag, inbound vs outbound)

### Query Execution

1. **Resolve seed**: Convert seed hex to internal identifier
2. **BFS traversal**: Traverse graph to specified depth, tracking first-seen depth
3. **Deduplication**: Each pubkey appears only at its first-discovered depth
4. **Collect refs**: If `inbound_refs`/`outbound_refs` specified, scan reference indexes
5. **Aggregate**: Group references by target/source, count occurrences
6. **Sort**: Order by count descending (for refs)
7. **Sign response**: Create and sign relay events with identity key

### Performance Considerations

- Use serial-based internal identifiers (5-byte) instead of full 32-byte IDs
- Pre-compute common aggregations if possible
- Set reasonable limits on depth (default max: 16) and result counts
- Consider caching frequent queries
- Use rate limiting to prevent abuse

---

## Backward Compatibility

- Relays not supporting this NIP will ignore the `_graph` field per NIP-01
- Clients should detect support via NIP-11 relay information document
- Response events (39000, 39001, 39002) are standard Nostr events

## NIP-11 Advertisement

Relays supporting this NIP should advertise it:

```json
{
    "supported_nips": [1, "XX"],
    "limitation": {
        "graph_query_max_depth": 16
    }
}
```

## Security Considerations

- **Rate limiting**: Graph queries can be expensive; relays should rate limit
- **Depth limits**: Maximum depth should be capped (recommended: 16)
- **Result limits**: Large follow graphs can return many results; consider size limits
- **Authentication**: Relays may require NIP-42 auth for graph queries

## References

- [NIP-01](https://github.com/nostr-protocol/nips/blob/master/01.md): Basic protocol
- [NIP-02](https://github.com/nostr-protocol/nips/blob/master/02.md): Follow lists (kind 3)
- [NIP-11](https://github.com/nostr-protocol/nips/blob/master/11.md): Relay information
- [NIP-33](https://github.com/nostr-protocol/nips/blob/master/33.md): Parameterized replaceable events
- [NIP-42](https://github.com/nostr-protocol/nips/blob/master/42.md): Authentication
