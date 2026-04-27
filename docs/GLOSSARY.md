# ORLY Relay Domain Glossary

This glossary defines the ubiquitous language used throughout the ORLY codebase.
All contributors should use these terms consistently in code, comments, and documentation.

---

## Core Domain Concepts

### Event
A Nostr event as defined in NIP-01. The fundamental unit of data in the Nostr protocol.
Contains: id, pubkey, created_at, kind, tags, content, sig.

- **Code location**: `git.smesh.lol/orly/pkg/nostr/encoders/event`
- **Key type**: `event.E`

### Serial
A monotonically increasing 40-bit identifier assigned to each event upon storage.
Used for efficient range queries, synchronization, and garbage collection ordering.

- **Code location**: `pkg/database/indexes/types.Uint40`
- **Related types**: EventRef, IdPkTs

### Pubkey
A 32-byte secp256k1 public key identifying a Nostr user. Stored as binary internally,
displayed as 64-character lowercase hex or bech32 npub format externally.

- **Code location**: `git.smesh.lol/orly/pkg/nostr/types.Pubkey`

### Access Level
The permission tier granted to a pubkey. Determines what operations are allowed.

| Level | Description |
|-------|-------------|
| `none` | No access, authentication required |
| `read` | Read-only access (REQ allowed, EVENT denied) |
| `write` | Read and write access |
| `admin` | Write + import/export + arbitrary delete |
| `owner` | Admin + wipe + system configuration |
| `blocked` | IP address blocked |
| `banned` | Pubkey banned |

- **Code location**: `pkg/interfaces/acl/acl.go` constants

### ACL (Access Control List)
The authorization system that determines access levels for pubkeys and IP addresses.
Supports multiple modes with different authorization strategies.

- **Code location**: `pkg/acl/`
- **Interface**: `pkg/interfaces/acl/acl.go`

---

## Event Processing Pipeline

The event processing pipeline transforms incoming WebSocket messages into stored events.
Each stage has distinct responsibilities and produces typed results.

```
Raw JSON → Validation → Authorization → Routing → Processing → Delivery
```

### Validation
The process of verifying event structure, signature, and protocol compliance.

Checks performed:
- Raw JSON validation (hex case normalization)
- Event ID verification (hash matches content)
- Signature verification (schnorr signature valid)
- Timestamp validation (not too far in future)
- NIP-70 protected tag validation

- **Code location**: `pkg/event/validation/`
- **Result type**: `validation.Result` with `Valid`, `Code`, `Msg`

### Authorization
The decision process determining if an event is allowed based on ACL and policy.
Returns a structured decision with access level and deny reason.

- **Code location**: `pkg/event/authorization/`
- **Result type**: `authorization.Decision` with `Allowed`, `AccessLevel`, `DenyReason`, `RequireAuth`

### Routing
Dispatching events to specialized handlers based on event kind.
Determines whether events should be processed normally, delivered ephemerally, or handled specially.

Examples:
- Ephemeral events (kinds 20000-29999): Deliver without storage
- Delete events (kind 5): Trigger deletion cascade
- NIP-43 events (kinds 28934, 28936): Membership requests

- **Code location**: `pkg/event/routing/`
- **Result type**: `routing.Result` with `Action`, `Error`

### Processing
The final stage: persisting events, running post-save hooks, and delivering to subscribers.
Handles deduplication, replaceable event logic, and event delivery.

- **Code location**: `pkg/event/processing/`
- **Result type**: `processing.Result` with `Saved`, `Duplicate`, `Blocked`, `Error`

---

## ACL Modes

### None Mode
Open relay - all pubkeys have write access by default.
No authentication required unless explicitly configured via `ORLY_AUTH_REQUIRED`.

- **Code location**: `pkg/acl/none.go`

### Follows Mode
Whitelist based on admin/owner follow lists (kind 3 events).
Followed pubkeys get write access; others get read-only or denied based on configuration.
Supports progressive throttling for non-followed users.

- **Code location**: `pkg/acl/follows.go`
- **Config**: `ORLY_ACL_MODE=follows`

### Managed Mode
Fine-grained control via NIP-86 management API.
Supports pubkey bans, event bans, IP blocks, kind restrictions, and custom rules.
All management operations require NIP-98 HTTP authentication.

- **Code location**: `pkg/acl/managed.go`
- **Config**: `ORLY_ACL_MODE=managed`

### Curating Mode
Curator-based content moderation system.
Curators can approve/reject events from non-followed users.
Events from non-curated users are held pending approval.

- **Code location**: `pkg/acl/curating.go`
- **Config**: `ORLY_ACL_MODE=curating`

---

## Protocol Concepts

### NIP-42 Authentication
Challenge-response authentication for WebSocket connections.
Used to verify pubkey ownership before granting elevated access.

Flow:
1. Relay sends AUTH challenge with random string
2. Client signs challenge with private key
3. Relay verifies signature and grants access level

- **Code location**: `pkg/protocol/auth/`

### NIP-70 Protected Events
Events with `-` (protected) tag that can only be replaced/deleted by the author.
Prevents relays from accepting replacements from unauthorized pubkeys.

- **Tag format**: `["-"]` in tags array

### NIP-43 Relay Access
Invite-based membership system for restricted relays.
Supports join requests, leave requests, and membership tracking.

| Kind | Purpose |
|------|---------|
| 28934 | Join request with invite code |
| 28936 | Leave request |
| 8000 | Member added (relay-published) |

- **Code location**: `pkg/protocol/nip43/`

### NIP-86 Relay Management
HTTP JSON-RPC API for relay administration.
Requires NIP-98 HTTP authentication with admin/owner access level.

- **Endpoint**: `/api/v1/management`
- **Code location**: `app/handle-nip86.go`

### NIP-77 Negentropy
Set reconciliation protocol for efficient relay-to-relay synchronization.
Uses negentropy algorithm to identify missing events with minimal bandwidth.

- **Code location**: `pkg/sync/negentropy/`
- **Envelope types**: NEG-OPEN, NEG-MSG, NEG-CLOSE, NEG-ERR

---

## Infrastructure Concepts

### Publisher
The event delivery system that sends events to subscribers.
Composed of multiple publisher implementations (socket, internal, etc.).

- **Code location**: `pkg/protocol/publish/`
- **Interface**: `pkg/interfaces/publisher/publisher.go`

### Sprocket
External event processing plugin (JavaScript/Rhai script).
Can accept, reject, or shadow-reject events before normal processing.

| Action | Effect |
|--------|--------|
| accept | Event proceeds to normal processing |
| reject | Event rejected with error message |
| shadowReject | Event appears accepted but is not stored |

- **Code location**: `app/sprocket.go`

### Policy Manager
Rule-based event filtering system configured via JSON or kind 30078 events.
Evaluates events against configurable rules for allow/deny decisions.

- **Code location**: `pkg/policy/`
- **Config file**: `~/.config/orly/policy.json`

### Rate Limiter
PID controller-based adaptive throttling system.
Adjusts delays based on system load (memory pressure, write throughput).

- **Code location**: `pkg/ratelimit/`
- **Interface**: `pkg/interfaces/loadmonitor/`

### Supervisor
Process lifecycle manager for split IPC mode deployment.
Manages database, ACL, sync, and relay processes with dependency ordering.

- **Code location**: `cmd/orly-launcher/supervisor.go`

---

## Data Types

### EventRef
Stack-allocated event reference with fixed-size ID and pubkey arrays.
80 bytes total, fits in cache line, safe for concurrent use.
Immutable - all fields are unexported with accessor methods.

```go
type EventRef struct {
    id  ntypes.EventID // 32 bytes
    pub ntypes.Pubkey  // 32 bytes
    ts  int64          // 8 bytes
    ser uint64         // 8 bytes
}
```

- **Code location**: `pkg/interfaces/store/store_interface.go`

### IdPkTs
Event reference with slice-based fields for backward compatibility.
Mutable - use `ToEventRef()` for safe concurrent access.

```go
type IdPkTs struct {
    Id  []byte   // Event ID
    Pub []byte   // Pubkey
    Ts  int64    // Timestamp
    Ser uint64   // Serial number
}
```

- **Code location**: `pkg/interfaces/store/store_interface.go`

### Decision
Authorization result carrying allowed status, access level, and context.
Used to communicate authorization outcomes through the pipeline.

- **Code location**: `pkg/event/authorization/authorization.go`

### Result (Validation)
Validation outcome with Valid bool, ReasonCode enum, and message string.
Codes: ReasonNone, ReasonBlocked, ReasonInvalid, ReasonError.

- **Code location**: `pkg/event/validation/validation.go`

### Result (Processing)
Processing outcome indicating whether event was saved, duplicate, or blocked.
Includes error field for unexpected failures.

- **Code location**: `pkg/event/processing/processing.go`

---

## Subscription Terminology

Note: "Subscription" has two distinct meanings in the codebase:

### Event Subscription (REQ)
An active filter receiving matching events in real-time.
Created via REQ envelope, cancelled via CLOSE envelope.
Stored in `Listener.subscriptions` map with cancel function.

- **Code location**: `app/listener.go` subscriptions field

### Payment Subscription
Paid access tier granting elevated permissions for a time period.
Managed via NWC payments or manual extension.

- **Code location**: `pkg/database/interface.go` Subscription type

---

## Sync Terminology

### Spider
Event aggregator that fetches events from external relays.
Subscribes to events for followed pubkeys on configured relay lists.

- **Code location**: `pkg/spider/`

### Sync
Peer-to-peer replication between relay instances.
Multiple implementations: negentropy, cluster, distributed, relaygroup.

- **Code location**: `pkg/sync/`

### Cluster
Group of relay instances sharing events via HTTP-based pull replication.
Membership tracked via kind 39108 events.

- **Code location**: `pkg/sync/cluster/`

### Relay Group
Configuration of relay sets for synchronized operation.
Tracked via kind 39105 events.

- **Code location**: `pkg/sync/relaygroup/`

---

## Driver Pattern

### Driver
A pluggable implementation of a core interface.
Selected at runtime via configuration or command-line flags.

Examples:
- Database drivers: badger, neo4j, wasmdb, grpc
- ACL drivers: none, follows, managed, curating
- Sync drivers: negentropy, cluster, distributed, relaygroup

```go
// Check if driver is available
database.HasDriver("badger")

// Create instance from driver
db := database.NewFromDriver("badger", config)
```

- **Pattern location**: `pkg/database/`, `pkg/acl/`, `pkg/sync/`

---

*Last updated: 2026-01-24*
*Based on ORLY codebase v0.56.4*
