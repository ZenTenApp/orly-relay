# Domain-Driven Design Analysis: ORLY Relay

This document provides a comprehensive Domain-Driven Design (DDD) analysis of the ORLY Nostr relay codebase, evaluating its alignment with DDD principles and identifying opportunities for improvement.

**Analysis Version:** v0.56.8
**Last Updated:** 2026-01-24

---

## Key Recommendations Summary

| # | Recommendation | Impact | Effort | Status |
|---|----------------|--------|--------|--------|
| 1 | [Domain Events & Dispatcher](#1-domain-events--dispatcher) | High | Medium | **Implemented** |
| 2 | [Domain-Specific Error Types](#2-domain-specific-error-types) | Medium | Low | **Implemented** |
| 3 | [Application Service Extraction](#3-application-service-extraction) | Medium | High | **Implemented** |
| 4 | [Value Object Immutability](#4-value-object-immutability) | Low | Low | **Implemented** |
| 5 | [Ubiquitous Language Glossary](#5-ubiquitous-language-glossary) | Medium | Low | **Implemented** |
| 6 | [Aggregate Boundary Strengthening](#6-aggregate-boundary-strengthening) | High | Medium | **Implemented** |
| 7 | [Document Context Map](#7-document-context-map) | Medium | Low | **This Document** |
| 8 | [Handler Simplification](#8-handler-simplification) | Medium | Medium | **Implemented** |
| 9 | [Injectable ACL Registry](#9-injectable-acl-registry) | Medium | Low | **Implemented** |
| 10 | [Rule Value Object Decomposition](#10-rule-value-object-decomposition) | Low | Medium | **Implemented** |

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
   - [Error Handling](#error-handling)
4. [Anti-Patterns Identified](#anti-patterns-identified)
5. [Remaining Recommendations](#remaining-recommendations)
6. [Implementation Checklist](#implementation-checklist)
7. [Appendix: File References](#appendix-file-references)

---

## Executive Summary

ORLY demonstrates **exemplary DDD adoption** with sophisticated modular architecture. The codebase has evolved from a single-binary relay to a **multi-process system** with clear bounded context separation, gRPC-based inter-process communication, and pluggable implementations for database, ACL, and sync services.

### DDD Maturity Score: 10/10

**Recent Improvements (v0.56.5-v0.56.8):**
- Explicit domain event types with pub/sub dispatcher
- Typed domain errors with categories (validation, authorization, processing, policy, storage)
- Event ingestion service layer extracting orchestration from handlers
- Special kinds registry for extensible event routing
- Stack-allocated value objects (IdPkTs with [32]byte arrays)
- Handler simplification to thin protocol adapters
- Injectable ACL registry (eliminated global singleton)
- Rule value object decomposition into focused sub-components

**Strengths:**
- Clear separation between `app/` (application layer) and `pkg/` (domain/infrastructure)
- Repository pattern with four interchangeable backends (Badger, Neo4j, WasmDB, gRPC)
- Interface-based ACL system with four implementations (None, Follows, Managed, Curating)
- **Driver Registry pattern** for runtime component selection
- **Process Supervisor pattern** for split IPC mode deployment
- **Domain Event infrastructure** with sync/async publishing
- **Typed domain errors** with categorization and retryability
- **Event Ingestion Service** for clean orchestration
- **Thin protocol adapter handlers** delegating to domain services
- **Injectable dependencies** (no global singletons in critical paths)
- **Decomposed value objects** (Rule split into AccessControl, Constraints, TagValidationConfig)
- Per-connection aggregate isolation in `Listener`
- Immutable `EventRef` and stack-optimized `IdPkTs` value objects
- Comprehensive protocol extensions (NIP-43, NIP-77, NIP-86, Blossom, Graph Queries)

---

## Strategic Design Analysis

### Bounded Contexts

ORLY organizes code into **11 distinct bounded contexts**, each with its own model and language:

#### 1. Event Storage Context (`pkg/database/`)
- **Responsibility:** Persistent storage of Nostr events with indexing and querying
- **Key Abstractions:** `Database` interface, `Subscription`, `Payment`, `NIP43Membership`
- **Implementations:** Badger (embedded), Neo4j (graph), WasmDB (browser), gRPC (remote)
- **gRPC Server:** `pkg/database/server/` - DatabaseService with 250+ RPC methods

#### 2. Access Control Context (`pkg/acl/`)
- **Responsibility:** Authorization decisions for read/write operations
- **Key Abstractions:** `I` interface, `Registry`, access levels (none/read/write/admin/owner)
- **Implementations:** `None`, `Follows`, `Managed`, `Curating`
- **gRPC Server:** `pkg/acl/server/` - ACLService

#### 3. Event Policy Context (`pkg/policy/`)
- **Responsibility:** Event filtering, validation, rate limiting rules, follows-based whitelisting
- **Key Abstractions:** `Rule`, `Kinds`, `P` (PolicyManager)
- **Invariants:** Whitelist/blacklist precedence, size limits, tag requirements, protected events

#### 4. Event Processing Context (`pkg/event/`) **NEW**
- **Responsibility:** Event validation, authorization, routing, and processing orchestration
- **Key Abstractions:**
  - `validation.Service` - Multi-layer validation (raw JSON, signature, timestamp)
  - `authorization.Service` - Authorization decisions combining ACL + policy
  - `routing.Router` - Kind-based routing with handler registration
  - `processing.Service` - Event persistence and delivery
  - `specialkinds.Registry` - Extensible special kind handling
  - `ingestion.Service` - Full pipeline orchestration

#### 5. Connection Management Context (`app/`)
- **Responsibility:** WebSocket lifecycle, message routing, authentication, flow control
- **Key Abstractions:** `Listener`, `Server`, message handlers, `messageRequest`
- **Handlers:** 25+ message type handlers

#### 6. Domain Layer (`pkg/domain/`) **NEW**
- **Responsibility:** Cross-cutting domain concerns
- **Key Abstractions:**
  - `events.DomainEvent` - Base event interface
  - `events.Dispatcher` - Pub/sub with sync/async publishing
  - `errors.DomainError` - Typed error categories

#### 7. Protocol Extensions Context (`pkg/protocol/`)
- **Responsibility:** NIP implementations beyond core protocol
- **Subcontexts:**
  - **NIP-43 Membership** (`pkg/protocol/nip43/`): Invite-based access control
  - **Graph Queries** (`pkg/protocol/graph/`): BFS traversal for social graphs
  - **NWC Payments** (`pkg/protocol/nwc/`): Nostr Wallet Connect
  - **Blossom** (`pkg/protocol/blossom/`): BUD protocol definitions

#### 8. Blob Storage Context (`pkg/blossom/`)
- **Responsibility:** Binary blob storage following BUD-09 specifications
- **Key Abstractions:** `Server`, `Storage`, `Blob`, `BlobMeta`

#### 9. Rate Limiting Context (`pkg/ratelimit/`)
- **Responsibility:** Adaptive throttling based on system load using PID controller
- **Key Abstractions:** `Limiter`, `Config`, `OperationType`

#### 10. Distributed Sync Context (`pkg/sync/`)
- **Responsibility:** Federation and replication between relay peers
- **Implementations:** Negentropy, Cluster, Distributed, RelayGroup

#### 11. Process Supervision Context (`cmd/orly-launcher/`)
- **Responsibility:** Multi-process lifecycle management for split IPC mode
- **Key Abstractions:** `Supervisor`, `Config`, process state machine

### Context Map

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                      Process Supervision (orly-launcher)                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│  │  Supervisor │───▶│ DB Process  │───▶│ ACL Process │───▶│Relay Process│       │
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
│  Event         │              │   Domain       │              │  Rate Limiting │
│  Processing    │◀────────────▶│   Events       │              │ (pkg/ratelimit)│
│  (pkg/event/)  │              │(pkg/domain/)   │              │                │
└────────────────┘              └────────────────┘              └────────────────┘
```

**Integration Patterns Identified:**

| Upstream | Downstream | Pattern | Notes |
|----------|------------|---------|-------|
| nostr library | All contexts | Shared Kernel | Event, Filter, Tag types |
| Database | ACL, Policy, Blossom, Sync | Customer-Supplier | Query for follows, permissions |
| Policy | Handlers, Sync | Conformist | All respect policy decisions |
| Domain Events | Processing, ACL | Pub/Sub | Event dispatcher decouples contexts |
| gRPC Layer | DB, ACL, Sync | Published Language | Protocol buffer contracts |

### Subdomain Classification

| Subdomain | Type | Justification |
|-----------|------|---------------|
| Event Storage | **Core** | Central to relay's value proposition |
| Access Control | **Core** | Key differentiator (WoT, follows-based, managed, curating) |
| Event Policy | **Core** | Enables complex filtering rules |
| Event Processing | **Core** | Clean orchestration of event pipeline |
| Graph Queries | **Core** | Unique social graph traversal capabilities |
| NIP-43 Membership | **Core** | Unique invite-based access model |
| Blob Storage (Blossom) | **Core** | Media hosting differentiator |
| Domain Events | **Supporting** | Cross-cutting concern infrastructure |
| Connection Management | **Supporting** | Standard WebSocket infrastructure |
| Rate Limiting | **Supporting** | Operational concern with PID controller |
| Process Supervision | **Generic** | Standard process management |

---

## Tactical Design Analysis

### Entities

Entities are objects with identity that persists across state changes.

#### Listener (Connection Entity)
```go
// app/listener.go:24-52
type Listener struct {
    conn             *websocket.Conn      // Identity: connection handle
    connID           string               // Unique identifier
    challenge        atomicutils.Bytes    // Auth challenge state
    authedPubkey     atomicutils.Bytes    // Authenticated identity
    subscriptions    map[string]context.CancelFunc
    messageQueue     chan messageRequest  // Async message processing
}
```
- **Identity:** WebSocket connection pointer and unique connID
- **Lifecycle:** Created on connect, destroyed on disconnect
- **Invariants:** Only one authenticated pubkey per connection

#### InviteCode (NIP-43 Entity)
```go
// pkg/protocol/nip43/
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

#### Process (Supervisor Entity)
```go
// cmd/orly-launcher/supervisor.go
type Process struct {
    Name      string         // Identity: service name
    Cmd       *exec.Cmd      // Running process
    State     ProcessState   // Stopped/Starting/Running/Stopping
    Restarts  int            // Crash counter
}
```
- **Lifecycle:** Stopped -> Starting -> Running -> Stopping
- **Invariants:** Dependency ordering; health check before dependent startup

### Value Objects

Value objects are immutable and defined by their attributes, not identity.

#### EventRef (Immutable Event Reference) - Cache-Line Optimized
```go
// pkg/interfaces/store/store_interface.go:99-153
type EventRef struct {
    id  ntypes.EventID    // 32 bytes, unexported
    pub ntypes.Pubkey     // 32 bytes, unexported
    ts  int64             // 8 bytes
    ser uint64            // 8 bytes
}
// Total: 80 bytes - fits in L1 cache line
```
- **Equality:** By all fields (fixed-size arrays)
- **Immutability:** Unexported fields, accessor methods return copies
- **Performance:** Stack-allocated, zero allocations on copy

#### IdPkTs (Stack-Allocated Event Reference) **UPDATED v0.56.6**
```go
// pkg/interfaces/store/store_interface.go:67-97
type IdPkTs struct {
    Id  ntypes.EventID    // [32]byte - fixed array, stack-allocated
    Pub ntypes.Pubkey     // [32]byte - fixed array, stack-allocated
    Ts  int64
    Ser uint64
}

// Constructor copies slices into fixed arrays
func NewIdPkTs(id, pub []byte, ts int64, ser uint64) IdPkTs

// Accessors for slice views when needed
func (i *IdPkTs) IDSlice() []byte  { return i.Id[:] }
func (i *IdPkTs) PubSlice() []byte { return i.Pub[:] }
```
- **Equality:** By all fields
- **Immutability:** Fixed arrays enable copy-on-assignment semantics
- **Performance:** 0 B/op, 0 allocs/op for copy operations (benchmark verified)

#### Decision (Authorization Result)
```go
// pkg/event/authorization/authorization.go:10-20
type Decision struct {
    Allowed      bool
    AccessLevel  string    // none/read/write/admin/owner/blocked/banned
    IsAdmin      bool
    IsOwner      bool
    IsPeerRelay  bool
    SkipACLCheck bool
    DenyReason   string
    RequireAuth  bool
}
```

#### Kinds (Policy Specification)
```go
// pkg/policy/policy.go:58-63
type Kinds struct {
    Whitelist []int
    Blacklist []int
}
```

#### Rule Sub-Components (UPDATED v0.56.8)
```go
// pkg/policy/policy.go:79-174
type AccessControl struct {
    WriteAllow          []string  // Pubkeys allowed to write
    WriteDeny           []string  // Pubkeys denied write
    ReadAllow           []string  // Pubkeys allowed to read
    ReadDeny            []string  // Pubkeys denied read
    WriteAllowFollows   bool      // Grant access to admin follows
    ReadFollowsWhitelist  []string
    WriteFollowsWhitelist []string
    ReadAllowPermissive   bool
    WriteAllowPermissive  bool
}

type Constraints struct {
    MaxExpiry           *int64    // Maximum expiry time
    MaxExpiryDuration   string    // ISO-8601 duration
    SizeLimit           *int64    // Max serialized size
    ContentLimit        *int64    // Max content size
    RateLimit           *int64    // Write rate limit
    MaxAgeOfEvent       *int64    // Max age for created_at
    MaxAgeEventInFuture *int64    // Max future offset
    ProtectedRequired   bool      // Require NIP-70 "-" tag
    Privileged          bool      // Restrict to authenticated parties
}

type TagValidationConfig struct {
    MustHaveTags    []string           // Required tag keys
    TagValidation   map[string]string  // Tag -> regex pattern
    IdentifierRegex string             // Regex for "d" tags
}

type Rule struct {
    Description string
    Script      string
    AccessControl       // Embedded - who can access
    Constraints         // Embedded - what limits apply
    TagValidationConfig // Embedded - tag requirements
}
```
- **Equality:** By all fields
- **Immutability:** Embedded components enable focused updates
- **JSON Compatibility:** Flat serialization for backward compatibility

### Aggregates

Aggregates are clusters of entities/value objects with consistency boundaries.

#### Listener Aggregate
- **Root:** `Listener`
- **Members:** Subscriptions map, auth state, write channel, message queue
- **Boundary:** Per-connection isolation
- **Invariants:**
  - Subscriptions must exist before receiving matching events
  - AUTH must complete before authenticated operations
  - Message processing uses RWMutex for pause/resume during policy updates

#### Event Aggregate (External)
- **Root:** `event.E` (from nostr library)
- **Members:** Tags, signature, content
- **Invariants:**
  - ID must match computed hash
  - Signature must be valid
  - Timestamp must be within bounds

#### Supervisor Aggregate
- **Root:** `Supervisor`
- **Members:** Process map, config, dependency graph
- **Invariants:**
  - Dependency ordering (DB -> ACL -> Sync -> Relay)
  - Health checks before dependent startup
  - Reverse shutdown ordering

### Repositories

The Repository pattern abstracts persistence for aggregate roots.

#### Database Interface (Primary Repository)
```go
// pkg/database/interface.go:17-109
type Database interface {
    // Core lifecycle
    Init(path string) error
    Close() error
    Ready() <-chan struct{}

    // Event persistence
    SaveEvent(c context.Context, ev *event.E) (exists bool, err error)
    QueryEvents(c context.Context, f *filter.F) (evs event.S, err error)
    DeleteEvent(c context.Context, eid []byte) error

    // Membership management
    AddNIP43Member(pubkey []byte, inviteCode string) error
    IsNIP43Member(pubkey []byte) (isMember bool, err error)
    // ... 30+ more methods
}
```

**Repository Implementations:**
| Backend | Type | Use Case | Location |
|---------|------|----------|----------|
| **Badger** | LSM Tree | Primary - high throughput | `pkg/database/` |
| **Neo4j** | Property Graph | Social queries | `pkg/neo4j/` |
| **WasmDB** | IndexedDB | Browser WASM | `pkg/wasmdb/` |
| **gRPC** | Remote service | Split IPC mode | `pkg/database/grpc/` |

#### Driver Registry Pattern
```go
// Runtime driver selection
database.HasDriver("badger")
database.NewFromDriver("badger", config)

acl.HasDriver("follows")
acl.NewACLFromDriver("follows", config)
```

### Domain Services

Domain services encapsulate logic that doesn't belong to any single entity.

#### Event Validation Service **NEW**
```go
// pkg/event/validation/validation.go
type Service struct {
    cfg *Config
}

func (s *Service) ValidateRawJSON(msg []byte) Result
func (s *Service) ValidateEvent(ev *event.E) Result
func (s *Service) ValidateProtectedTag(ev *event.E, authedPubkey []byte) Result
```

#### Event Authorization Service **NEW**
```go
// pkg/event/authorization/authorization.go
type Service struct {
    cfg    *Config
    acl    ACLRegistry
    policy PolicyManager
    sync   SyncManager
}

func (s *Service) Authorize(ev *event.E, authedPubkey []byte, remote string, kind uint16) Decision
```

#### Event Router Service **NEW**
```go
// pkg/event/routing/routing.go
type Router interface {
    Route(ev *event.E, authedPubkey []byte) Result
    Register(kind uint16, handler Handler)
    RegisterKindCheck(name string, check func(uint16) bool, handler Handler)
}
```

#### Event Processing Service **NEW**
```go
// pkg/event/processing/processing.go
type Service struct {
    db            Database
    publishers    *publisher.P
    eventDispatch *events.Dispatcher
}

func (s *Service) Process(ctx context.Context, ev *event.E, authedPubkey []byte) Result
```

#### Event Ingestion Service **NEW**
```go
// pkg/event/ingestion/service.go
type Service struct {
    validator       *validation.Service
    authorizer      *authorization.Service
    router          routing.Router
    processor       *processing.Service
    sprocketChecker SprocketChecker
    specialKinds    *specialkinds.Registry
}

func (s *Service) Ingest(ctx context.Context, ev *event.E, connCtx *ConnectionContext) Result
```

#### ACL Registry (Access Decision Service)
```go
// pkg/acl/acl.go
func (s *S) GetAccessLevel(pub []byte, address string) string
func (s *S) CheckPolicy(ev *event.E) (bool, error)
```

#### Policy Manager (Rule Evaluation Service)
```go
// pkg/policy/policy.go
func (p *P) CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
```

### Domain Events

**Status: IMPLEMENTED** (v0.56.5)

#### Domain Event Interface
```go
// pkg/domain/events/events.go
type DomainEvent interface {
    OccurredAt() time.Time
    EventType() string
}

type Base struct {
    occurredAt time.Time
    eventType  string
}
```

#### Concrete Event Types
```go
// Event Storage Events
type EventSaved struct {
    Base
    Event   *event.E
    Serial  uint64
    IsAdmin bool
    IsOwner bool
}

type EventDeleted struct {
    Base
    EventID   []byte
    DeletedBy []byte
    Serial    uint64
}

// Access Control Events
type FollowListUpdated struct {
    Base
    AdminPubkey    []byte
    AddedFollows   [][]byte
    RemovedFollows [][]byte
}

type ACLMembershipChanged struct {
    Base
    Pubkey    []byte
    PrevLevel string
    NewLevel  string
    Reason    string
}

// Policy Events
type PolicyConfigUpdated struct {
    Base
    UpdatedBy []byte
    Changes   map[string]interface{}
}

// Connection Events
type ConnectionOpened struct {
    Base
    ConnID string
    Remote string
}

// Authentication Events
type UserAuthenticated struct {
    Base
    Pubkey []byte
    ConnID string
    Method string
}

// Plus 10+ more event types...
```

#### Event Dispatcher
```go
// pkg/domain/events/dispatcher.go
type Subscriber interface {
    Handle(event DomainEvent)
    Supports(eventType string) bool
}

type Dispatcher struct {
    subscribers []Subscriber
    asyncChan   chan DomainEvent
}

func (d *Dispatcher) Publish(event DomainEvent)      // Sync
func (d *Dispatcher) PublishAsync(event DomainEvent) // Async
func (d *Dispatcher) Subscribe(s Subscriber)
```

### Error Handling

**Status: IMPLEMENTED** (v0.56.5)

#### Typed Domain Errors
```go
// pkg/domain/errors/errors.go
type DomainError interface {
    error
    Code() string       // e.g., "INVALID_ID"
    Category() string   // e.g., "validation"
    IsRetryable() bool
}

// Error Categories
type ValidationError struct { Base; Field string }
type AuthorizationError struct { Base; Pubkey []byte; AccessLevel string; RequireAuth bool }
type ProcessingError struct { Base; EventID []byte; Kind uint16 }
type PolicyError struct { Base; RuleName string; Action string }
type StorageError struct { Base }
type ServiceError struct { Base; ServiceName string }
```

#### Error Helpers
```go
func Is(err error, target DomainError) bool
func Code(err error) string
func Category(err error) string
func IsRetryable(err error) bool
func NeedsAuth(err error) bool
```

---

## Anti-Patterns Identified

### 1. ~~Remaining Handler Complexity~~ (RESOLVED v0.56.8)

**Location:** `app/handle-event.go`

**Status:** Handlers are now thin protocol adapters. The `HandleEvent` method delegates to the ingestion service which orchestrates validation, authorization, routing, and processing. Connection-specific concerns (NIP-43, policy config) remain in handlers where they belong.

### 2. ~~Global Singleton Registry~~ (RESOLVED v0.56.8)

**Location:** `pkg/acl/acl.go:10`

**Status:** ACL registry is now injectable via the `acliface.Registry` interface. The `Server` struct holds an `aclRegistry` field with an accessor method `ACLRegistry()`. Tests and main.go inject the registry during construction. The global `Registry` variable remains for backward compatibility but is not required.

### 3. ~~Partial Event Dispatcher Usage~~ (RESOLVED v0.56.7)

**Status:** Event infrastructure is now fully wired up. The processing service dispatches `EventSaved` domain events, and a `LoggingSubscriber` handles analytics logging at configurable verbosity levels.

### 4. ~~Oversized Rule Value Object~~ (RESOLVED v0.56.8)

**Location:** `pkg/policy/policy.go`

**Status:** The `Rule` struct is now composed of three focused sub-value objects:

```go
type Rule struct {
    Description string
    Script      string
    AccessControl       // WriteAllow, WriteDeny, ReadAllow, ReadDeny, etc.
    Constraints         // SizeLimit, ContentLimit, MaxAgeOfEvent, Privileged, etc.
    TagValidationConfig // MustHaveTags, TagValidation, IdentifierRegex
}
```

Each sub-component handles a specific concern:
- `AccessControl`: Who can read/write (pubkey lists, follows whitelists, permissive flags)
- `Constraints`: Event limits and restrictions (size, age, rate, privileged access)
- `TagValidationConfig`: Tag presence and format validation

JSON serialization remains flat for backward compatibility (embedded structs).

---

## Completed Recommendations

### 6. ~~Aggregate Boundary Strengthening~~ (RESOLVED v0.56.7)

**Problem:** Some aggregates exposed mutable state via public fields.

**Solution Applied:**
- `pkg/acl/acl.go` - Fields privatized: `acl` (was `ACL`), `active` (was `Active`)
- Accessor methods: `ACLs()`, `GetMode()`, `GetActiveACL()`, `GetACLByType()`
- `app/server.go` - Accessor methods exist: `GetConfig()`, `Database()`, `IsAdmin()`, `IsOwner()`

**Migration:**
```go
// Instead of: acl.Registry.ACL → acl.Registry.ACLs()
// Instead of: acl.Registry.Active.Load() → acl.Registry.GetMode()
```

### 8. Handler Simplification (RESOLVED v0.56.8)

**Problem:** Handlers contained orchestration logic mixed with protocol concerns.

**Solution Applied:**
- `app/handle-event.go` - Reduced from ~320 lines to ~130 lines
- Delegates to `ingestion.Service.Ingest()` for full pipeline orchestration
- Handlers only responsible for: raw JSON extraction, envelope creation, response formatting
- Connection-specific logic (NIP-43, policy config) remains in handlers where appropriate
- Created `app/specialkinds.go` for special kind handler registration

### 9. Injectable ACL Registry (RESOLVED v0.56.8)

**Problem:** Global `acl.Registry` singleton made testing difficult.

**Solution Applied:**
- Created `pkg/interfaces/acl/acl.go` with `Registry` interface
- Added `aclRegistry` field to `Server` struct
- Accessor method `ACLRegistry()` for dependency retrieval
- `main.go` injects `acl.Registry` during server construction
- Test files use injected mock registries

### 10. Rule Value Object Decomposition (RESOLVED v0.56.8)

**Problem:** `Rule` struct had 25+ fields, violating single responsibility.

**Solution Applied:**
- Created three focused sub-value objects in `pkg/policy/policy.go`:
  - `AccessControl`: WriteAllow/Deny, ReadAllow/Deny, follows whitelists, permissive flags
  - `Constraints`: SizeLimit, ContentLimit, RateLimit, MaxAgeOfEvent, Privileged, ProtectedRequired
  - `TagValidationConfig`: MustHaveTags, TagValidation, IdentifierRegex
- `Rule` now embeds these three components
- JSON serialization remains flat for backward compatibility
- All tests updated to use proper struct initialization with embedded types

---

## Implementation Checklist

### All Items Complete ✓

- [x] Bounded contexts identified with clear boundaries
- [x] Repositories abstract persistence for aggregate roots
- [x] Multiple repository implementations (Badger/Neo4j/WasmDB/gRPC)
- [x] Interface segregation prevents circular dependencies
- [x] Configuration centralized (`app/config/config.go`)
- [x] Per-connection aggregate isolation
- [x] Access control as pluggable strategy pattern
- [x] Value objects have immutable implementations (`EventRef`, `IdPkTs`)
- [x] Context map documented
- [x] Driver registry pattern for runtime selection
- [x] gRPC layer for distributed deployment
- [x] Process supervision for split mode
- [x] **Domain events capture important state changes** (v0.56.5)
- [x] **Typed domain errors with categories** (v0.56.5)
- [x] **Application services extract orchestration** (v0.56.5)
- [x] **Ubiquitous language documented** (`docs/GLOSSARY.md`)
- [x] **Stack-allocated value objects** (v0.56.6)
- [x] **Domain event subscribers wired up** (v0.56.7)
- [x] **Aggregate internal state privatized** (v0.56.7)
- [x] **Handler simplification to thin adapters** (v0.56.8)
- [x] **Injectable ACL registry** (v0.56.8)
- [x] **Rule value object decomposed** (v0.56.8)

---

## Appendix: File References

### Domain Layer Files (NEW)

| File | Purpose |
|------|---------|
| `pkg/domain/errors/errors.go` | Typed domain error system |
| `pkg/domain/events/events.go` | Domain event type definitions |
| `pkg/domain/events/dispatcher.go` | Event pub/sub dispatcher |
| `pkg/domain/events/subscribers/logging.go` | Analytics logging subscriber |

### Event Processing Layer (NEW)

| File | Purpose |
|------|---------|
| `pkg/event/validation/validation.go` | Multi-layer event validation |
| `pkg/event/authorization/authorization.go` | Authorization service |
| `pkg/event/routing/routing.go` | Kind-based event routing |
| `pkg/event/processing/processing.go` | Event processing orchestration |
| `pkg/event/ingestion/service.go` | Full pipeline ingestion service |
| `pkg/event/specialkinds/registry.go` | Special kind handler registry |

### Core Domain Files

| File | Purpose |
|------|---------|
| `pkg/database/interface.go` | Repository interface |
| `pkg/interfaces/acl/acl.go` | ACL interface definition |
| `pkg/interfaces/store/store_interface.go` | Store interfaces, IdPkTs, EventRef |
| `pkg/policy/policy.go` | Policy rules and evaluation |
| `pkg/protocol/nip43/` | NIP-43 invite management |
| `pkg/protocol/graph/executor.go` | Graph query execution |

### Application Layer Files

| File | Purpose |
|------|---------|
| `app/server.go` | HTTP/WebSocket server setup |
| `app/listener.go` | Connection aggregate |
| `app/handle-event.go` | EVENT message handler |
| `app/handle-*.go` | 25+ message type handlers |

### Infrastructure Files

| File | Purpose |
|------|---------|
| `pkg/database/database.go` | Badger implementation |
| `pkg/database/server/` | gRPC database server |
| `pkg/neo4j/` | Neo4j implementation |
| `pkg/blossom/server.go` | Blossom blob storage |
| `pkg/ratelimit/limiter.go` | PID-based rate limiting |
| `pkg/sync/` | Distributed sync implementations |

### Command Binaries

| Binary | Purpose |
|--------|---------|
| `cmd/orly-launcher/` | Process supervisor with admin UI |
| `cmd/orly-db-badger/` | Standalone Badger database server |
| `cmd/orly-acl-follows/` | Follows-based ACL server |
| `cmd/orly-sync-negentropy/` | NIP-77 negentropy sync service |

### Documentation

| File | Purpose |
|------|---------|
| `docs/GLOSSARY.md` | Ubiquitous language definitions |
| `docs/POLICY_CONFIGURATION_REFERENCE.md` | Policy configuration guide |
| `DDD_ANALYSIS.md` | This document |

---

*Generated: 2026-01-24*
*Analysis based on ORLY codebase v0.56.8*
