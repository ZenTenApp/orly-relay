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

ORLY demonstrates **mature DDD adoption** with sophisticated modular architecture. The codebase has evolved from a single-binary relay to a **multi-process system** with clear bounded context separation, gRPC-based inter-process communication, and pluggable implementations for database, ACL, and sync services.

**Strengths:**
- Clear separation between `app/` (application layer) and `pkg/` (domain/infrastructure)
- Repository pattern with four interchangeable backends (Badger, Neo4j, WasmDB, gRPC)
- Interface-based ACL system with four implementations (None, Follows, Managed, Curating)
- **Driver Registry pattern** for runtime component selection
- **Process Supervisor pattern** for split IPC mode deployment
- Per-connection aggregate isolation in `Listener`
- Strong use of Go interfaces for dependency inversion
- Immutable `EventRef` value object alongside legacy `IdPkTs`
- Comprehensive protocol extensions (NIP-43, NIP-77, NIP-86, Blossom, Graph Queries)
- **gRPC service layer** exposing all major interfaces for remote access

**Areas for Improvement:**
- Domain events are implicit rather than explicit types
- Some aggregates expose mutable state via public fields
- Handler methods mix application orchestration with domain logic
- Ubiquitous language is partially documented

**Overall DDD Maturity Score: 8/10** (improved from 7.5/10)

---

## Strategic Design Analysis

### Bounded Contexts

ORLY organizes code into distinct bounded contexts, each with its own model and language:

#### 1. Event Storage Context (`pkg/database/`)
- **Responsibility:** Persistent storage of Nostr events with indexing and querying
- **Key Abstractions:** `Database` interface (109 lines), `Subscription`, `Payment`, `NIP43Membership`
- **Implementations:** Badger (embedded), Neo4j (graph), WasmDB (browser), gRPC (remote)
- **gRPC Server:** `pkg/database/server/` - DatabaseService with 250+ RPC methods
- **File:** `pkg/database/interface.go:17-109`

#### 2. Access Control Context (`pkg/acl/`)
- **Responsibility:** Authorization decisions for read/write operations
- **Key Abstractions:** `I` interface, `Registry`, access levels (none/read/write/admin/owner)
- **Implementations:** `None`, `Follows`, `Managed`, `Curating`
- **gRPC Server:** `pkg/acl/server/` - ACLService
- **Files:** `pkg/acl/acl.go`, `pkg/interfaces/acl/acl.go:21-40`

#### 3. Event Policy Context (`pkg/policy/`)
- **Responsibility:** Event filtering, validation, rate limiting rules, follows-based whitelisting
- **Key Abstractions:** `Rule`, `Kinds`, `P` (PolicyManager)
- **Invariants:** Whitelist/blacklist precedence, size limits, tag requirements, protected events
- **File:** `pkg/policy/policy.go` (extensive, ~1000 lines)

#### 4. Connection Management Context (`app/`)
- **Responsibility:** WebSocket lifecycle, message routing, authentication, flow control
- **Key Abstractions:** `Listener`, `Server`, message handlers, `messageRequest`
- **Handlers:** 25+ message type handlers
- **File:** `app/listener.go:24-52`

#### 5. Protocol Extensions Context (`pkg/protocol/`)
- **Responsibility:** NIP implementations beyond core protocol
- **Subcontexts:**
  - **NIP-43 Membership** (`pkg/protocol/nip43/`): Invite-based access control (kinds 28934, 28936, 8000)
  - **Graph Queries** (`pkg/protocol/graph/`): BFS traversal for follows/followers/threads
  - **NWC Payments** (`pkg/protocol/nwc/`): Nostr Wallet Connect integration
  - **Blossom** (`pkg/protocol/blossom/`): BUD protocol definitions
  - **Directory** (`pkg/protocol/directory/`): Relay directory client

#### 6. Blob Storage Context (`pkg/blossom/`)
- **Responsibility:** Binary blob storage following BUD-09 specifications
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
- **Key Abstractions:** `Manager`, `Registry`, driver pattern
- **Implementations:**
  - **Negentropy** (`pkg/sync/negentropy/`): NIP-77 set reconciliation
  - **Cluster** (`pkg/sync/cluster/`): HTTP-based pull replication (kind 39108)
  - **Distributed** (`pkg/sync/distributed/`): Peer-to-peer sync
  - **RelayGroup** (`pkg/sync/relaygroup/`): Relay grouping (kind 39105)
- **gRPC Clients:** Each sync implementation has gRPC client for split mode
- **Files:** `pkg/sync/manager.go`, `pkg/sync/negentropy/`, `pkg/sync/cluster/`

#### 9. Spider Context (`pkg/spider/`)
- **Responsibility:** Syncing events from admin relays for followed pubkeys
- **Key Abstractions:** `Spider`, `RelayConnection`, `DirectorySpider`
- **Integration:** Batch subscriptions, rate limit backoff, blackout periods
- **File:** `pkg/spider/spider.go`

#### 10. Process Supervision Context (`cmd/orly-launcher/`) **NEW**
- **Responsibility:** Multi-process lifecycle management for split IPC mode
- **Key Abstractions:** `Supervisor`, `Config`, process state machine
- **Patterns:** Supervisor pattern with dependency ordering, crash recovery
- **Integration:** gRPC health checks, graceful shutdown ordering
- **Files:** `cmd/orly-launcher/supervisor.go`, `cmd/orly-launcher/config.go`

#### 11. Certificate Management Context (`cmd/orly-certs/`) **NEW**
- **Responsibility:** TLS certificate provisioning and renewal
- **Key Abstractions:** Certificate manager, DNS-01 challenge handler
- **Integration:** Independent service, communicates via filesystem
- **File:** `cmd/orly-certs/`

### Context Map

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                      Process Supervision (orly-launcher)                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│  │  Supervisor │───▶│ DB Process  │───▶│ ACL Process │───▶│Relay Process│       │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘       │
│         │                  │                  │                  │              │
│         │  [Dependency]    │   [gRPC]         │    [gRPC]        │              │
│         ▼                  ▼                  ▼                  ▼              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│  │Sync Services│    │   Certs     │    │ Admin Web   │    │   Health    │       │
│  │  (4 procs)  │    │  Service    │    │     UI      │    │   Checks    │       │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘       │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
         ┌───────────────────────────────┼───────────────────────────────┐
         │                               │                               │
         ▼                               ▼                               ▼
┌────────────────┐              ┌────────────────┐              ┌────────────────┐
│  Event Storage │              │  Access Control│              │  Event Policy  │
│ (pkg/database/)│              │   (pkg/acl/)   │              │  (pkg/policy/) │
│                │              │                │              │                │
│ Badger│Neo4j   │◀─[gRPC]─────▶│Follows│Managed │◀─[Conformist]│   Manager      │
│ WasmDB│gRPC    │              │Curating│None   │              │                │
└────────────────┘              └────────────────┘              └────────────────┘
         │                               │                               │
         │ [Shared Kernel]               │                               │
         ▼                               ▼                               │
┌────────────────────────────────────────────────────────────┐          │
│                      Event Entity                          │          │
│                 (git.mleku.dev/mleku/nostr)                │◀─────────┘
│            Filter, Tag, Subscription types                 │
└────────────────────────────────────────────────────────────┘
         │                               │
         │ [Customer-Supplier]           │ [Customer-Supplier]
         ▼                               ▼
┌────────────────┐              ┌────────────────┐              ┌────────────────┐
│  Blob Storage  │              │   Protocol     │              │  Rate Limiting │
│  (pkg/blossom) │              │   Extensions   │              │ (pkg/ratelimit)│
│                │              │ (pkg/protocol/)│              │                │
└────────────────┘              └────────────────┘              └────────────────┘
                                        │
        ┌───────────────────────────────┼───────────────────────────────┐
        ▼                               ▼                               ▼
┌────────────────┐              ┌────────────────┐              ┌────────────────┐
│   NIP-43       │              │   NIP-77       │              │ Graph Queries  │
│  Membership    │              │  Negentropy    │              │(pkg/protocol/  │
│(kinds 28934/36)│              │  Sync          │              │     graph/)    │
└────────────────┘              └────────────────┘              └────────────────┘
                                        │
        ┌───────────────────────────────┼───────────────────────────────┐
        ▼                               ▼                               ▼
┌────────────────┐              ┌────────────────┐              ┌────────────────┐
│   Cluster      │              │  Distributed   │              │  Relay Group   │
│   Sync         │              │     Sync       │              │   (kind 39105) │
│ (kind 39108)   │              │                │              │                │
└────────────────┘              └────────────────┘              └────────────────┘
```

**Integration Patterns Identified:**

| Upstream | Downstream | Pattern | Notes |
|----------|------------|---------|-------|
| nostr library | All contexts | Shared Kernel | Event, Filter, Tag types |
| Database | ACL, Policy, Blossom, Sync | Customer-Supplier | Query for follows, permissions, event storage |
| Policy | Handlers, Sync | Conformist | All respect policy decisions |
| ACL | Handlers, Blossom | Conformist | Handlers/Blossom respect access levels |
| Rate Limit | Database | Anti-Corruption | LoadMonitor abstraction |
| Sync Services | Database | Customer-Supplier | Serial-based event replication |
| Launcher | All Services | Supervisor | Process lifecycle management |
| gRPC Layer | DB, ACL, Sync | Published Language | Protocol buffer contracts |

### Subdomain Classification

| Subdomain | Type | Justification |
|-----------|------|---------------|
| Event Storage | **Core** | Central to relay's value proposition |
| Access Control | **Core** | Key differentiator (WoT, follows-based, managed, curating) |
| Event Policy | **Core** | Enables complex filtering rules |
| Graph Queries | **Core** | Unique social graph traversal capabilities |
| NIP-43 Membership | **Core** | Unique invite-based access model |
| Blob Storage (Blossom) | **Core** | Media hosting differentiator |
| NIP-77 Negentropy Sync | **Core** | Efficient relay-to-relay sync |
| Connection Management | **Supporting** | Standard WebSocket infrastructure |
| Rate Limiting | **Supporting** | Operational concern with PID controller |
| Cluster Sync | **Supporting** | Infrastructure for federation |
| Spider | **Supporting** | Data aggregation from external relays |
| Process Supervision | **Generic** | Standard process management |
| Certificate Management | **Generic** | Standard TLS infrastructure |

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
- **Lifecycle:** Created -> Valid -> Used/Expired
- **Invariants:** Cannot be reused once consumed

#### Subscription (Payment Entity)
```go
// pkg/database/interface.go (implied by methods)
// GetSubscription, ExtendSubscription, RecordPayment
```
- **Identity:** Pubkey
- **Lifecycle:** Trial -> Active -> Expired
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
- **Lifecycle:** Uploaded -> Active -> Deleted
- **Invariants:** Hash must match content; owner can delete

#### Process (Supervisor Entity) **NEW**
```go
// cmd/orly-launcher/supervisor.go (implied)
type Process struct {
    Name      string      // Identity: service name
    Cmd       *exec.Cmd   // Running process
    State     ProcessState
    Restarts  int
}
```
- **Identity:** Service name (orly-db, orly-acl, etc.)
- **Lifecycle:** Stopped -> Starting -> Running -> Stopping
- **Invariants:** Dependency ordering; health check before dependent startup

### Value Objects

Value objects are immutable and defined by their attributes, not identity.

#### EventRef (Immutable Event Reference)
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
- **Binary caches:** Performance optimization for hex->binary conversion

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

#### LauncherConfig (Configuration Value) **NEW**
```go
// cmd/orly-launcher/config.go
type Config struct {
    DBBackend     string  // badger, neo4j
    DBListen      string  // 127.0.0.1:50051
    ACLEnabled    bool
    ACLMode       string  // follows, managed, curation
    ACLListen     string  // 127.0.0.1:50052
    AdminEnabled  bool
    AdminPort     int
    AdminOwners   []string
    // ... sync service configs
}
```
- **Persistence:** JSON file at `~/.config/orly/launcher.json`
- **Environment Override:** All fields can be overridden via `ORLY_LAUNCHER_*`

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

#### Supervisor Aggregate **NEW**
- **Root:** `Supervisor`
- **Members:** Process map, config, dependency graph
- **Boundary:** All managed processes
- **Invariants:**
  - Dependency ordering (DB -> ACL -> Sync -> Relay)
  - Health checks before dependent startup
  - Reverse shutdown ordering
  - Crash recovery with restart limits

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
4. **gRPC** (`pkg/database/grpc/`): Remote database via gRPC **NEW**

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

#### Driver Registry Pattern **NEW**
```go
// pkg/database/ - Driver selection at runtime
database.HasDriver("badger")  // Check availability
database.NewFromDriver("badger", config)  // Create instance

// pkg/acl/ - ACL driver selection
acl.HasDriver("follows")
acl.NewACLFromDriver("follows", config)

// pkg/sync/ - Sync driver selection
sync.HasDriver("negentropy")
sync.NewSyncManager("negentropy", config)
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

#### Supervisor (Process Lifecycle Service) **NEW**
```go
// cmd/orly-launcher/supervisor.go
type Supervisor struct {
    config    *Config
    processes map[string]*Process
    mu        sync.Mutex
}
func (s *Supervisor) Start() error
func (s *Supervisor) Stop() error
func (s *Supervisor) Restart(name string) error
func (s *Supervisor) IsRunning() bool
```
- Manages process lifecycle with dependency ordering
- Health checks via gRPC before proceeding
- Crash recovery with configurable restart limits
- Graceful shutdown in reverse dependency order

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
| ProcessStarted | Supervisor start | Health check, dependency unlock |
| ProcessCrashed | Process exit | Restart attempt, alert |
| ConfigUpdated | Admin UI save | JSON persistence, reload |

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

type ProcessStateChanged struct {
    ServiceName string
    OldState    ProcessState
    NewState    ProcessState
    Timestamp   time.Time
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
| Supervisor | Process lifecycle manager | `supervisor.Supervisor` |
| Driver | Pluggable implementation | Registry pattern |
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
    ErrServiceUnavailable = &DomainError{Code: "SERVICE_UNAVAILABLE"}
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
- [x] Multiple repository implementations (Badger/Neo4j/WasmDB/gRPC)
- [x] Interface segregation prevents circular dependencies
- [x] Configuration centralized (`app/config/config.go`)
- [x] Per-connection aggregate isolation
- [x] Access control as pluggable strategy pattern
- [x] Value objects have immutable alternative (`EventRef`)
- [x] Context map documented
- [x] Driver registry pattern for runtime selection
- [x] gRPC layer for distributed deployment
- [x] Process supervision for split mode

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
| `app/server.go` | HTTP/WebSocket server setup |
| `app/listener.go` | Connection aggregate |
| `app/handle-event.go` | EVENT message handler |
| `app/handle-req.go` | REQ message handler |
| `app/handle-auth.go` | AUTH message handler |
| `app/handle-nip43.go` | NIP-43 membership handlers |
| `app/handle-nip86.go` | NIP-86 management handlers |
| `app/handle-negentropy.go` | NIP-77 negentropy sync |
| `app/handle-policy-config.go` | Policy configuration events |

### Infrastructure Files

| File | Purpose |
|------|---------|
| `pkg/database/database.go` | Badger implementation |
| `pkg/database/server/` | gRPC database server |
| `pkg/database/grpc/` | gRPC database client |
| `pkg/neo4j/` | Neo4j implementation |
| `pkg/wasmdb/` | WasmDB implementation |
| `pkg/blossom/server.go` | Blossom blob storage server |
| `pkg/ratelimit/limiter.go` | PID-based rate limiting |
| `pkg/sync/negentropy/` | NIP-77 negentropy sync |
| `pkg/sync/cluster/` | HTTP cluster replication |
| `pkg/spider/spider.go` | Event spider/aggregator |

### Command Binaries

| Binary | Purpose |
|--------|---------|
| `cmd/orly-launcher/` | Process supervisor with admin UI |
| `cmd/orly-db-badger/` | Standalone Badger database server |
| `cmd/orly-db-neo4j/` | Standalone Neo4j database server |
| `cmd/orly-acl-follows/` | Follows-based ACL server |
| `cmd/orly-acl-managed/` | NIP-86 managed ACL server |
| `cmd/orly-acl-curation/` | Trust-tier curation ACL server |
| `cmd/orly-sync-negentropy/` | NIP-77 negentropy sync service |
| `cmd/orly-sync-cluster/` | Cluster replication service |
| `cmd/orly-certs/` | Certificate management service |

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

### Protocol Buffer Definitions

| Package | Purpose |
|---------|---------|
| `pkg/proto/orlydb/v1/` | Database gRPC service (250+ methods) |
| `pkg/proto/orlyacl/v1/` | ACL gRPC service |
| `pkg/proto/orlysync/` | Sync services (negentropy, cluster, etc.) |

---

*Generated: 2026-01-24*
*Analysis based on ORLY codebase v0.56.4*
