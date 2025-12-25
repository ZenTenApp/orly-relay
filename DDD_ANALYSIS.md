# Domain-Driven Design Analysis: ORLY Relay

This document provides a comprehensive Domain-Driven Design (DDD) analysis of the ORLY Nostr relay codebase, evaluating its alignment with DDD principles and identifying opportunities for improvement.

---

## Key Recommendations Summary

| # | Recommendation | Impact | Effort | Status |
|---|----------------|--------|--------|--------|
| 1 | [Formalize Domain Events](#1-formalize-domain-events) | High | Medium | Pending |
| 2 | [Strengthen Aggregate Boundaries](#2-strengthen-aggregate-boundaries) | High | Medium | Partial |
| 3 | [Extract Application Services](#3-extract-application-services) | Medium | High | Pending |
| 4 | [Establish Ubiquitous Language Glossary](#4-establish-ubiquitous-language-glossary) | Medium | Low | Pending |
| 5 | [Add Domain-Specific Error Types](#5-add-domain-specific-error-types) | Medium | Low | Pending |
| 6 | [Enforce Value Object Immutability](#6-enforce-value-object-immutability) | Low | Low | **Addressed** |
| 7 | [Document Context Map](#7-document-context-map) | Medium | Low | **This Document** |

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Strategic Design Analysis](#strategic-design-analysis)
   - [Bounded Contexts](#bounded-contexts)
   - [Context Map](#context-map)
   - [Subdomain Classification](#subdomain-classification)
3. [Tactical Design Analysis](#tactical-design-analysis)
   - [Entities](#entities)
   - [Value Objects](#value-objects)
   - [Aggregates](#aggregates)
   - [Repositories](#repositories)
   - [Domain Services](#domain-services)
   - [Domain Events](#domain-events)
4. [Anti-Patterns Identified](#anti-patterns-identified)
5. [Detailed Recommendations](#detailed-recommendations)
6. [Implementation Checklist](#implementation-checklist)
7. [Appendix: File References](#appendix-file-references)

---

## Executive Summary

ORLY demonstrates **mature DDD adoption** for a system of its complexity. The codebase exhibits clear bounded context separation, proper repository patterns with multiple backend implementations, and well-designed interface segregation that prevents circular dependencies.

**Strengths:**
- Clear separation between `app/` (application layer) and `pkg/` (domain/infrastructure)
- Repository pattern with three interchangeable backends (Badger, Neo4j, WasmDB)
- Interface-based ACL system with pluggable implementations (None, Follows, Managed)
- Per-connection aggregate isolation in `Listener`
- Strong use of Go interfaces for dependency inversion
- **New:** Immutable `EventRef` value object alongside legacy `IdPkTs`
- **New:** Comprehensive protocol extensions (Blossom, Graph Queries, NIP-43, NIP-86)
- **New:** Distributed sync with cluster replication support

**Areas for Improvement:**
- Domain events are implicit rather than explicit types
- Some aggregates expose mutable state via public fields
- Handler methods mix application orchestration with domain logic
- Ubiquitous language is partially documented

**Overall DDD Maturity Score: 7.5/10** (improved from 7/10)

---

## Strategic Design Analysis

### Bounded Contexts

ORLY organizes code into distinct bounded contexts, each with its own model and language:

#### 1. Event Storage Context (`pkg/database/`)
- **Responsibility:** Persistent storage of Nostr events with indexing and querying
- **Key Abstractions:** `Database` interface (109 lines), `Subscription`, `Payment`, `NIP43Membership`
- **Implementations:** Badger (embedded), Neo4j (graph), WasmDB (browser)
- **File:** `pkg/database/interface.go:17-109`

#### 2. Access Control Context (`pkg/acl/`)
- **Responsibility:** Authorization decisions for read/write operations
- **Key Abstractions:** `I` interface, `Registry`, access levels (none/read/write/admin/owner)
- **Implementations:** `None`, `Follows`, `Managed`
- **Files:** `pkg/acl/acl.go`, `pkg/interfaces/acl/acl.go:21-40`

#### 3. Event Policy Context (`pkg/policy/`)
- **Responsibility:** Event filtering, validation, rate limiting rules, follows-based whitelisting
- **Key Abstractions:** `Rule`, `Kinds`, `P` (PolicyManager)
- **Invariants:** Whitelist/blacklist precedence, size limits, tag requirements, protected events
- **File:** `pkg/policy/policy.go` (extensive, ~1000 lines)

#### 4. Connection Management Context (`app/`)
- **Responsibility:** WebSocket lifecycle, message routing, authentication, flow control
- **Key Abstractions:** `Listener`, `Server`, message handlers, `messageRequest`
- **File:** `app/listener.go:24-52`

#### 5. Protocol Extensions Context (`pkg/protocol/`)
- **Responsibility:** NIP implementations beyond core protocol
- **Subcontexts:**
  - **NIP-43 Membership** (`pkg/protocol/nip43/`): Invite-based access control
  - **Graph Queries** (`pkg/protocol/graph/`): BFS traversal for follows/followers/threads
  - **NWC Payments** (`pkg/protocol/nwc/`): Nostr Wallet Connect integration
  - **Blossom** (`pkg/protocol/blossom/`): BUD protocol definitions
  - **Directory** (`pkg/protocol/directory/`): Relay directory client

#### 6. Blob Storage Context (`pkg/blossom/`)
- **Responsibility:** Binary blob storage following BUD specifications
- **Key Abstractions:** `Server`, `Storage`, `Blob`, `BlobMeta`
- **Invariants:** SHA-256 hash integrity, MIME type validation, quota enforcement
- **Files:** `pkg/blossom/server.go`, `pkg/blossom/storage.go`

#### 7. Rate Limiting Context (`pkg/ratelimit/`)
- **Responsibility:** Adaptive throttling based on system load using PID controller
- **Key Abstractions:** `Limiter`, `Config`, `OperationType` (Read/Write)
- **Integration:** Memory pressure from database backends via `loadmonitor` interface
- **File:** `pkg/ratelimit/limiter.go`

#### 8. Distributed Sync Context (`pkg/sync/`)
- **Responsibility:** Federation and replication between relay peers
- **Key Abstractions:** `Manager`, `ClusterManager`, `RelayGroupManager`, `NIP11Cache`
- **Integration:** Serial-number based sync protocol, NIP-11 peer discovery
- **Files:** `pkg/sync/manager.go`, `pkg/sync/cluster.go`, `pkg/sync/relaygroup.go`

#### 9. Spider Context (`pkg/spider/`)
- **Responsibility:** Syncing events from admin relays for followed pubkeys
- **Key Abstractions:** `Spider`, `RelayConnection`, `DirectorySpider`
- **Integration:** Batch subscriptions, rate limit backoff, blackout periods
- **File:** `pkg/spider/spider.go`

### Context Map

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Connection Management (app/)                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐   │
│  │   Server    │───▶│  Listener   │───▶│  Handlers   │◀──▶│  Publishers │   │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘   │
└────────┬────────────────────┬────────────────────┬──────────────────────────┘
         │                    │                    │
         │ [Conformist]       │ [Customer-Supplier]│ [Customer-Supplier]
         ▼                    ▼                    ▼
┌────────────────┐   ┌────────────────┐   ┌────────────────┐
│  Access Control│   │  Event Storage │   │  Event Policy  │
│   (pkg/acl/)   │   │ (pkg/database/)│   │  (pkg/policy/) │
│                │   │                │   │                │
│  Registry ◀────┼───┼────Conformist──┼───┼─▶ Manager      │
└────────────────┘   └────────────────┘   └────────────────┘
         │                    │                    │
         │                    │ [Shared Kernel]    │
         │                    ▼                    │
         │           ┌────────────────┐            │
         │           │  Event Entity  │            │
         │           │(git.mleku.dev/ │◀───────────┘
         │           │ mleku/nostr)   │
         │           └────────────────┘
         │                    │
         │ [Anti-Corruption]  │ [Customer-Supplier]
         ▼                    ▼
┌────────────────┐   ┌────────────────┐   ┌────────────────┐
│  Rate Limiting │   │   Protocol     │   │  Blob Storage  │
│ (pkg/ratelimit)│   │   Extensions   │   │  (pkg/blossom) │
│                │   │ (pkg/protocol/)│   │                │
└────────────────┘   └────────────────┘   └────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌────────────────┐   ┌────────────────┐   ┌────────────────┐
│  Distributed   │   │    Spider      │   │ Graph Queries  │
│     Sync       │   │   (pkg/spider) │   │(pkg/protocol/  │
│  (pkg/sync/)   │   │                │   │     graph/)    │
└────────────────┘   └────────────────┘   └────────────────┘
```

**Integration Patterns Identified:**

| Upstream | Downstream | Pattern | Notes |
|----------|------------|---------|-------|
| nostr library | All contexts | Shared Kernel | Event, Filter, Tag types |
| Database | ACL, Policy, Blossom | Customer-Supplier | Query for follow lists, permissions, blob storage |
| Policy | Handlers, Sync | Conformist | All respect policy decisions |
| ACL | Handlers, Blossom | Conformist | Handlers/Blossom respect access levels |
| Rate Limit | Database | Anti-Corruption | Load monitor abstraction |
| Sync | Database, Policy | Customer-Supplier | Serial-based event replication |

### Subdomain Classification

| Subdomain | Type | Justification |
|-----------|------|---------------|
| Event Storage | **Core** | Central to relay's value proposition |
| Access Control | **Core** | Key differentiator (WoT, follows-based, managed) |
| Event Policy | **Core** | Enables complex filtering rules |
| Graph Queries | **Core** | Unique social graph traversal capabilities |
| NIP-43 Membership | **Core** | Unique invite-based access model |
| Blob Storage (Blossom) | **Core** | Media hosting differentiator |
| Connection Management | **Supporting** | Standard WebSocket infrastructure |
| Rate Limiting | **Supporting** | Operational concern with PID controller |
| Distributed Sync | **Supporting** | Infrastructure for federation |
| Spider | **Supporting** | Data aggregation from external relays |

---

## Tactical Design Analysis

### Entities

Entities are objects with identity that persists across state changes.

#### Listener (Connection Entity)
```go
// app/listener.go:24-52
type Listener struct {
    conn             *websocket.Conn      // Identity: connection handle
    challenge        atomicutils.Bytes    // Auth challenge state
    authedPubkey     atomicutils.Bytes    // Authenticated identity
    subscriptions    map[string]context.CancelFunc
    messageQueue     chan messageRequest  // Async message processing
    droppedMessages  atomic.Int64         // Flow control counter
    // ... more fields
}
```
- **Identity:** WebSocket connection pointer
- **Lifecycle:** Created on connect, destroyed on disconnect
- **Invariants:** Only one authenticated pubkey per connection; AUTH processed synchronously

#### InviteCode (NIP-43 Entity)
```go
// pkg/protocol/nip43/types.go:26-31
type InviteCode struct {
    Code      string      // Identity: unique code
    ExpiresAt time.Time
    UsedBy    []byte      // Tracks consumption
    CreatedAt time.Time
}
```
- **Identity:** Unique code string
- **Lifecycle:** Created → Valid → Used/Expired
- **Invariants:** Cannot be reused once consumed

#### Subscription (Payment Entity)
```go
// pkg/database/interface.go (implied by methods)
// GetSubscription, ExtendSubscription, RecordPayment
```
- **Identity:** Pubkey
- **Lifecycle:** Trial → Active → Expired
- **Invariants:** Can only extend if not expired

#### Blob (Blossom Entity)
```go
// pkg/blossom/blob.go (implied)
type BlobMeta struct {
    SHA256    string    // Identity: content-addressable
    Size      int64
    Type      string    // MIME type
    Uploaded  time.Time
    Owner     []byte    // Uploader pubkey
}
```
- **Identity:** SHA-256 hash
- **Lifecycle:** Uploaded → Active → Deleted
- **Invariants:** Hash must match content; owner can delete

### Value Objects

Value objects are immutable and defined by their attributes, not identity.

#### EventRef (Immutable Event Reference) - **NEW**
```go
// pkg/interfaces/store/store_interface.go:99-107
type EventRef struct {
    id  ntypes.EventID // 32 bytes
    pub ntypes.Pubkey  // 32 bytes
    ts  int64          // 8 bytes
    ser uint64         // 8 bytes
}
```
- **Equality:** By all fields (fixed-size arrays)
- **Immutability:** Unexported fields, accessor methods return copies
- **Size:** 80 bytes, cache-line friendly, stack-allocated

#### IdPkTs (Legacy Event Reference)
```go
// pkg/interfaces/store/store_interface.go:67-72
type IdPkTs struct {
    Id  []byte   // Event ID
    Pub []byte   // Pubkey
    Ts  int64    // Timestamp
    Ser uint64   // Serial number
}
```
- **Equality:** By all fields
- **Issue:** Mutable slices (use `ToEventRef()` for immutable version)
- **Migration:** Has `ToEventRef()` and accessors `IDFixed()`, `PubFixed()`

#### Kinds (Policy Specification)
```go
// pkg/policy/policy.go:58-63
type Kinds struct {
    Whitelist []int `json:"whitelist,omitempty"`
    Blacklist []int `json:"blacklist,omitempty"`
}
```
- **Equality:** By whitelist/blacklist contents
- **Semantics:** Whitelist takes precedence over blacklist

#### Rule (Policy Rule)
```go
// pkg/policy/policy.go:75-180
type Rule struct {
    Description           string
    WriteAllow           []string
    WriteDeny            []string
    ReadFollowsWhitelist []string
    WriteFollowsWhitelist []string
    MaxExpiryDuration    string
    SizeLimit            *int64
    ContentLimit         *int64
    Privileged           bool
    ProtectedRequired    bool
    ReadAllowPermissive  bool
    WriteAllowPermissive bool
    // ... binary caches
}
```
- **Complexity:** 25+ fields, decomposition candidate
- **Binary caches:** Performance optimization for hex→binary conversion

#### WriteRequest (Message Value)
```go
// pkg/protocol/publish/types.go
type WriteRequest struct {
    Data      []byte
    MsgType   int
    IsControl bool
    IsPing    bool
    Deadline  time.Time
}
```

### Aggregates

Aggregates are clusters of entities/value objects with consistency boundaries.

#### Listener Aggregate
- **Root:** `Listener`
- **Members:** Subscriptions map, auth state, write channel, message queue
- **Boundary:** Per-connection isolation
- **Invariants:**
  - Subscriptions must exist before receiving matching events
  - AUTH must complete before other messages check authentication
  - Message processing uses RWMutex for pause/resume during policy updates

```go
// app/listener.go:226-249 - Aggregate consistency enforcement
l.authProcessing.Lock()
if isAuthMessage {
    // Process AUTH synchronously while holding lock
    l.HandleMessage(req.data, req.remote)
    l.authProcessing.Unlock()
} else {
    l.authProcessing.Unlock()
    // Process concurrently
}
```

#### Event Aggregate (External)
- **Root:** `event.E` (from nostr library)
- **Members:** Tags, signature, content
- **Invariants:**
  - ID must match computed hash
  - Signature must be valid
  - Timestamp must be within bounds (configurable per-kind)
- **Validation:** `app/handle-event.go`

#### InviteCode Aggregate
- **Root:** `InviteCode`
- **Members:** Code, expiry, usage tracking
- **Invariants:**
  - Code uniqueness
  - Single-use enforcement
  - Expiry validation

#### Blossom Blob Aggregate
- **Root:** `BlobMeta`
- **Members:** Content data, metadata, owner
- **Invariants:**
  - SHA-256 integrity
  - Size limits
  - MIME type restrictions
  - Owner-only deletion

### Repositories

The Repository pattern abstracts persistence for aggregate roots.

#### Database Interface (Primary Repository)
```go
// pkg/database/interface.go:17-109
type Database interface {
    // Core lifecycle
    Path() string
    Init(path string) error
    Sync() error
    Close() error
    Ready() <-chan struct{}

    // Event persistence (30+ methods)
    SaveEvent(c context.Context, ev *event.E) (exists bool, err error)
    QueryEvents(c context.Context, f *filter.F) (evs event.S, err error)
    DeleteEvent(c context.Context, eid []byte) error

    // Subscription management
    GetSubscription(pubkey []byte) (*Subscription, error)
    ExtendSubscription(pubkey []byte, days int) error

    // NIP-43 membership
    AddNIP43Member(pubkey []byte, inviteCode string) error
    IsNIP43Member(pubkey []byte) (isMember bool, err error)

    // Blossom integration
    ExtendBlossomSubscription(pubkey []byte, tier string, storageMB int64, daysExtended int) error
    GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error)

    // Query cache
    GetCachedJSON(f *filter.F) ([][]byte, bool)
    CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte)
}
```

**Repository Implementations:**
1. **Badger** (`pkg/database/database.go`): Embedded key-value store
2. **Neo4j** (`pkg/neo4j/`): Graph database for social queries
3. **WasmDB** (`pkg/wasmdb/`): Browser IndexedDB for WASM builds

**Interface Segregation:**
```go
// pkg/interfaces/store/store_interface.go:21-38
type I interface {
    Pather
    io.Closer
    Wiper
    Querier      // QueryForIds
    Querent      // QueryEvents
    Deleter      // DeleteEvent
    Saver        // SaveEvent
    Importer
    Exporter
    Syncer
    LogLeveler
    EventIdSerialer
    Initer
    SerialByIder
}
```

### Domain Services

Domain services encapsulate logic that doesn't belong to any single entity.

#### ACL Registry (Access Decision Service)
```go
// pkg/acl/acl.go:40-48
func (s *S) GetAccessLevel(pub []byte, address string) (level string)
func (s *S) CheckPolicy(ev *event.E) (allowed bool, err error)
func (s *S) AddFollow(pub []byte)
```
- Delegates to active ACL implementation
- Stateless decision based on pubkey and IP
- Optional `PolicyChecker` interface for custom validation

#### Policy Manager (Event Validation Service)
```go
// pkg/policy/policy.go (P type)
// CheckPolicy evaluates rule chains, scripts, whitelist/blacklist logic
// Supports per-kind rules with follows-based whitelisting
```
- Complex rule evaluation logic
- Script execution for custom validation
- Binary cache optimization for pubkey comparisons

#### InviteManager (Invite Lifecycle Service)
```go
// pkg/protocol/nip43/types.go:34-109
type InviteManager struct {
    codes  map[string]*InviteCode
    expiry time.Duration
}
func (im *InviteManager) GenerateCode() (code string, err error)
func (im *InviteManager) ValidateAndConsume(code string, pubkey []byte) (bool, string)
```
- Manages invite code lifecycle
- Thread-safe with mutex protection

#### Graph Executor (Query Execution Service)
```go
// pkg/protocol/graph/executor.go:56-60
type Executor struct {
    db           GraphDatabase
    relaySigner  signer.I
    relayPubkey  []byte
}
func (e *Executor) Execute(q *Query) (*event.E, error)
```
- BFS traversal for follows/followers/threads
- Generates relay-signed ephemeral response events

#### Rate Limiter (Throttling Service)
```go
// pkg/ratelimit/limiter.go
type Limiter struct { ... }
func (l *Limiter) Wait(ctx context.Context, op OperationType) error
```
- PID controller-based adaptive throttling
- Separate setpoints for read/write operations
- Emergency mode with hysteresis

### Domain Events

**Current State:** Domain events are implicit in message flow, not explicit types.

**Implicit Events Identified:**

| Event | Trigger | Effect |
|-------|---------|--------|
| EventPublished | `SaveEvent()` success | `publishers.Deliver()` |
| EventDeleted | Kind 5 processing | Cascade delete targets |
| UserAuthenticated | AUTH envelope accepted | `authedPubkey` set |
| SubscriptionCreated | REQ envelope | Query + stream setup |
| MembershipAdded | NIP-43 join request | ACL update, kind 8000 event |
| MembershipRemoved | NIP-43 leave request | ACL update, kind 8001 event |
| PolicyUpdated | Policy config event | `messagePauseMutex.Lock()` |
| BlobUploaded | Blossom PUT success | Quota updated |
| BlobDeleted | Blossom DELETE | Quota released |

---

## Anti-Patterns Identified

### 1. Large Handler Methods (Partial Anemic Domain Model)

**Location:** `app/handle-event.go` (600+ lines)

**Issue:** The event handling contains:
- Input validation (lowercase hex, JSON structure)
- Policy checking
- ACL verification
- Signature verification
- Persistence
- Event delivery
- Special case handling (delete, ephemeral, NIP-43, NIP-86)

**Impact:** Difficult to test, maintain, and understand. Business rules are embedded in orchestration code.

### 2. Mutable Value Object Fields (Partially Addressed)

**Location:** `pkg/interfaces/store/store_interface.go:67-72`

```go
type IdPkTs struct {
    Id  []byte   // Mutable slice
    Pub []byte   // Mutable slice
    Ts  int64
    Ser uint64
}
```

**Mitigation:** New `EventRef` type with unexported fields provides immutable alternative.
Use `ToEventRef()` method for safe conversion.

### 3. Global Singleton Registry

**Location:** `pkg/acl/acl.go:10`

```go
var Registry = &S{}
```

**Impact:** Global state makes testing difficult and hides dependencies. Should be injected.

### 4. Missing Domain Events

**Impact:** Side effects are coupled to primary operations. Adding new behaviors (logging, analytics, notifications) requires modifying core handlers.

### 5. Oversized Rule Value Object

**Location:** `pkg/policy/policy.go:75-180`

The `Rule` struct has 25+ fields with binary caches, suggesting decomposition into:
- `AccessRule` (allow/deny lists, follows whitelists)
- `SizeRule` (limits)
- `TimeRule` (expiry, age)
- `ValidationRule` (tags, regex, protected)

---

## Detailed Recommendations

### 1. Formalize Domain Events

**Problem:** Side effects are tightly coupled to primary operations.

**Solution:** Create explicit domain event types and a simple event dispatcher.

```go
// pkg/domain/events/events.go
package events

type DomainEvent interface {
    OccurredAt() time.Time
    AggregateID() []byte
}

type EventPublished struct {
    EventID   []byte
    Pubkey    []byte
    Kind      int
    Timestamp time.Time
}

type MembershipGranted struct {
    Pubkey     []byte
    InviteCode string
    Timestamp  time.Time
}

type BlobUploaded struct {
    SHA256    string
    Owner     []byte
    Size      int64
    Timestamp time.Time
}
```

### 2. Strengthen Aggregate Boundaries

**Problem:** Aggregate internals are exposed via public fields.

**Solution:** The Listener already uses behavior methods well. Extend pattern:

```go
func (l *Listener) IsAuthenticated() bool {
    return len(l.authedPubkey.Load()) > 0
}

func (l *Listener) AuthenticatedPubkey() []byte {
    return l.authedPubkey.Load()
}
```

### 3. Extract Application Services

**Problem:** Handler methods contain mixed concerns.

**Solution:** Extract domain logic into focused application services.

```go
// pkg/application/event_service.go
type EventService struct {
    db            database.Database
    policyMgr     *policy.P
    aclRegistry   *acl.S
    eventPublisher EventPublisher
}

func (s *EventService) ProcessIncomingEvent(ctx context.Context, ev *event.E, authedPubkey []byte) (*EventResult, error)
```

### 4. Establish Ubiquitous Language Glossary

**Problem:** Terminology is inconsistent across the codebase.

**Current Inconsistencies:**
- "subscription" (payment) vs "subscription" (REQ filter)
- "pub" vs "pubkey" vs "author"
- "spider" vs "sync" for relay federation

**Solution:** Maintain a `GLOSSARY.md`:

```markdown
# ORLY Ubiquitous Language

| Term | Definition | Code Symbol |
|------|------------|-------------|
| Event | A signed Nostr message | `event.E` |
| Relay | This server | `Server` |
| Connection | WebSocket session | `Listener` |
| Filter | Query criteria for events | `filter.F` |
| **Event Subscription** | Active filter receiving events | `subscriptions map` |
| **Payment Subscription** | Paid access tier | `database.Subscription` |
| Access Level | Permission tier | `acl.Level` |
| Policy | Event validation rules | `policy.Rule` |
| Blob | Binary content (images, media) | `blossom.BlobMeta` |
| Spider | Event aggregator from external relays | `spider.Spider` |
| Sync | Peer-to-peer replication | `sync.Manager` |
```

### 5. Add Domain-Specific Error Types

**Problem:** Errors are strings or generic types.

**Solution:** Create typed domain errors in `pkg/interfaces/neterr/` pattern:

```go
var (
    ErrEventInvalid       = &DomainError{Code: "EVENT_INVALID"}
    ErrEventBlocked       = &DomainError{Code: "EVENT_BLOCKED"}
    ErrAuthRequired       = &DomainError{Code: "AUTH_REQUIRED"}
    ErrQuotaExceeded      = &DomainError{Code: "QUOTA_EXCEEDED"}
    ErrInviteCodeInvalid  = &DomainError{Code: "INVITE_INVALID"}
    ErrBlobTooLarge       = &DomainError{Code: "BLOB_TOO_LARGE"}
)
```

### 6. Enforce Value Object Immutability - **ADDRESSED**

The `EventRef` type now provides an immutable alternative:

```go
// pkg/interfaces/store/store_interface.go:99-153
type EventRef struct {
    id  ntypes.EventID // unexported
    pub ntypes.Pubkey  // unexported
    ts  int64
    ser uint64
}

func (r EventRef) ID() ntypes.EventID { return r.id }   // Returns copy
func (r EventRef) IDHex() string { return r.id.Hex() }
func (i *IdPkTs) ToEventRef() EventRef                  // Migration path
```

### 7. Document Context Map - **THIS DOCUMENT**

The context map is now documented in this file with integration patterns.

---

## Implementation Checklist

### Currently Satisfied

- [x] Bounded contexts identified with clear boundaries
- [x] Repositories abstract persistence for aggregate roots
- [x] Multiple repository implementations (Badger/Neo4j/WasmDB)
- [x] Interface segregation prevents circular dependencies
- [x] Configuration centralized (`app/config/config.go`)
- [x] Per-connection aggregate isolation
- [x] Access control as pluggable strategy pattern
- [x] Value objects have immutable alternative (`EventRef`)
- [x] Context map documented

### Needs Attention

- [ ] Ubiquitous language documented and used consistently
- [ ] Domain events capture important state changes (explicit types)
- [ ] Entities have behavior, not just data (more encapsulation)
- [ ] No business logic in application services (handler decomposition)
- [ ] No infrastructure concerns in domain layer

---

## Appendix: File References

### Core Domain Files

| File | Purpose |
|------|---------|
| `pkg/database/interface.go` | Repository interface (109 lines) |
| `pkg/interfaces/acl/acl.go` | ACL interface definition with PolicyChecker |
| `pkg/interfaces/store/store_interface.go` | Store sub-interfaces, IdPkTs, EventRef |
| `pkg/policy/policy.go` | Policy rules and evaluation (~1000 lines) |
| `pkg/protocol/nip43/types.go` | NIP-43 invite management |
| `pkg/protocol/graph/executor.go` | Graph query execution |

### Application Layer Files

| File | Purpose |
|------|---------|
| `app/server.go` | HTTP/WebSocket server setup (1240 lines) |
| `app/listener.go` | Connection aggregate (297 lines) |
| `app/handle-event.go` | EVENT message handler |
| `app/handle-req.go` | REQ message handler |
| `app/handle-auth.go` | AUTH message handler |
| `app/handle-nip43.go` | NIP-43 membership handlers |
| `app/handle-nip86.go` | NIP-86 management handlers |
| `app/handle-policy-config.go` | Policy configuration events |

### Infrastructure Files

| File | Purpose |
|------|---------|
| `pkg/database/database.go` | Badger implementation |
| `pkg/neo4j/` | Neo4j implementation |
| `pkg/wasmdb/` | WasmDB implementation |
| `pkg/blossom/server.go` | Blossom blob storage server |
| `pkg/ratelimit/limiter.go` | PID-based rate limiting |
| `pkg/sync/manager.go` | Distributed sync manager |
| `pkg/sync/cluster.go` | Cluster replication |
| `pkg/spider/spider.go` | Event spider/aggregator |

### Interface Packages

| Package | Purpose |
|---------|---------|
| `pkg/interfaces/acl/` | ACL abstraction |
| `pkg/interfaces/loadmonitor/` | Load monitoring abstraction |
| `pkg/interfaces/neterr/` | Network error types |
| `pkg/interfaces/pid/` | PID controller interface |
| `pkg/interfaces/policy/` | Policy interface |
| `pkg/interfaces/publisher/` | Event publisher interface |
| `pkg/interfaces/resultiter/` | Result iterator interface |
| `pkg/interfaces/store/` | Store interface with IdPkTs, EventRef |
| `pkg/interfaces/typer/` | Type introspection interface |

---

*Generated: 2025-12-24*
*Analysis based on ORLY codebase v0.36.14*
