# Domain-Driven Design Analysis: ORLY Relay

This document provides a comprehensive Domain-Driven Design (DDD) analysis of the ORLY Nostr relay codebase, evaluating its alignment with DDD principles and identifying opportunities for improvement.

---

## Key Recommendations Summary

| # | Recommendation | Impact | Effort |
|---|----------------|--------|--------|
| 1 | [Formalize Domain Events](#1-formalize-domain-events) | High | Medium |
| 2 | [Strengthen Aggregate Boundaries](#2-strengthen-aggregate-boundaries) | High | Medium |
| 3 | [Extract Application Services](#3-extract-application-services) | Medium | High |
| 4 | [Establish Ubiquitous Language Glossary](#4-establish-ubiquitous-language-glossary) | Medium | Low |
| 5 | [Add Domain-Specific Error Types](#5-add-domain-specific-error-types) | Medium | Low |
| 6 | [Enforce Value Object Immutability](#6-enforce-value-object-immutability) | Low | Low |
| 7 | [Document Context Map](#7-document-context-map) | Medium | Low |

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
- Interface-based ACL system with pluggable implementations
- Per-connection aggregate isolation in `Listener`
- Strong use of Go interfaces for dependency inversion

**Areas for Improvement:**
- Domain events are implicit rather than explicit types
- Some aggregates expose mutable state via public fields
- Handler methods mix application orchestration with domain logic
- Ubiquitous language is partially documented

**Overall DDD Maturity Score: 7/10**

---

## Strategic Design Analysis

### Bounded Contexts

ORLY organizes code into distinct bounded contexts, each with its own model and language:

#### 1. Event Storage Context (`pkg/database/`)
- **Responsibility:** Persistent storage of Nostr events
- **Key Abstractions:** `Database` interface, `Subscription`, `Payment`
- **Implementations:** Badger (embedded), Neo4j (graph), WasmDB (browser)
- **File:** `pkg/database/interface.go:17-109`

#### 2. Access Control Context (`pkg/acl/`)
- **Responsibility:** Authorization decisions for read/write operations
- **Key Abstractions:** `I` interface, `Registry`, access levels
- **Implementations:** `None`, `Follows`, `Managed`
- **Files:** `pkg/acl/acl.go`, `pkg/interfaces/acl/acl.go:21-34`

#### 3. Event Policy Context (`pkg/policy/`)
- **Responsibility:** Event filtering, validation, and rate limiting rules
- **Key Abstractions:** `Rule`, `Kinds`, `PolicyManager`
- **Invariants:** Whitelist/blacklist precedence, size limits, tag requirements
- **File:** `pkg/policy/policy.go:58-180`

#### 4. Connection Management Context (`app/`)
- **Responsibility:** WebSocket lifecycle, message routing, authentication
- **Key Abstractions:** `Listener`, `Server`, message handlers
- **File:** `app/listener.go:24-52`

#### 5. Protocol Extensions Context (`pkg/protocol/`)
- **Responsibility:** NIP implementations beyond core protocol
- **Subcontexts:**
  - NIP-43 Membership (`pkg/protocol/nip43/`)
  - Graph queries (`pkg/protocol/graph/`)
  - NWC payments (`pkg/protocol/nwc/`)
  - Sync/replication (`pkg/sync/`)

#### 6. Rate Limiting Context (`pkg/ratelimit/`)
- **Responsibility:** Adaptive throttling based on system load
- **Key Abstractions:** `Limiter`, `Monitor`, PID controller
- **Integration:** Memory pressure from database backends

### Context Map

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Connection Management (app/)                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                  │
│  │   Server    │───▶│  Listener   │───▶│  Handlers   │                  │
│  └─────────────┘    └─────────────┘    └─────────────┘                  │
└────────┬────────────────────┬────────────────────┬──────────────────────┘
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
┌────────────────┐   ┌────────────────┐
│  Rate Limiting │   │   Protocol     │
│ (pkg/ratelimit)│   │   Extensions   │
│                │   │ (pkg/protocol/)│
└────────────────┘   └────────────────┘
```

**Integration Patterns Identified:**

| Upstream | Downstream | Pattern | Notes |
|----------|------------|---------|-------|
| nostr library | All contexts | Shared Kernel | Event, Filter, Tag types |
| Database | ACL, Policy | Customer-Supplier | Query for follow lists, permissions |
| Policy | Handlers | Conformist | Handlers respect policy decisions |
| ACL | Handlers | Conformist | Handlers respect access levels |
| Rate Limit | Database | Anti-Corruption | Load monitor abstraction |

### Subdomain Classification

| Subdomain | Type | Justification |
|-----------|------|---------------|
| Event Storage | **Core** | Central to relay's value proposition |
| Access Control | **Core** | Key differentiator (WoT, follows-based) |
| Event Policy | **Core** | Enables complex filtering rules |
| Connection Management | **Supporting** | Standard WebSocket infrastructure |
| Rate Limiting | **Supporting** | Operational concern, not domain-specific |
| NIP-43 Membership | **Core** | Unique invite-based access model |
| Sync/Replication | **Supporting** | Infrastructure for federation |

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
    // ... more fields
}
```
- **Identity:** WebSocket connection pointer
- **Lifecycle:** Created on connect, destroyed on disconnect
- **Invariants:** Only one authenticated pubkey per connection

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

### Value Objects

Value objects are immutable and defined by their attributes, not identity.

#### IdPkTs (Event Reference)
```go
// pkg/interfaces/store/store_interface.go:63-68
type IdPkTs struct {
    Id  []byte   // Event ID
    Pub []byte   // Pubkey
    Ts  int64    // Timestamp
    Ser uint64   // Serial number
}
```
- **Equality:** By all fields
- **Issue:** Should be immutable but uses mutable slices

#### Kinds (Policy Specification)
```go
// pkg/policy/policy.go:55-63
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
    Description       string
    WriteAllow        []string
    WriteDeny         []string
    MaxExpiry         *int64
    SizeLimit         *int64
    // ... 25+ fields
}
```
- **Issue:** Very large, could benefit from decomposition
- **Binary caches:** Performance optimization for hex→binary conversion

#### WriteRequest (Message Value)
```go
// pkg/protocol/publish/types.go (implied)
type WriteRequest struct {
    Data      []byte
    MsgType   int
    IsControl bool
    Deadline  time.Time
}
```

### Aggregates

Aggregates are clusters of entities/value objects with consistency boundaries.

#### Listener Aggregate
- **Root:** `Listener`
- **Members:** Subscriptions map, auth state, write channel
- **Boundary:** Per-connection isolation
- **Invariants:**
  - Subscriptions must exist before receiving matching events
  - AUTH must complete before other messages check authentication
  - Message processing is serialized within connection

```go
// app/listener.go:226-238 - Aggregate consistency enforcement
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
  - Timestamp must be within bounds
- **Validation:** `app/handle-event.go:348-390`

#### InviteCode Aggregate
- **Root:** `InviteCode`
- **Members:** Code, expiry, usage tracking
- **Invariants:**
  - Code uniqueness
  - Single-use enforcement
  - Expiry validation

### Repositories

The Repository pattern abstracts persistence for aggregate roots.

#### Database Interface (Primary Repository)
```go
// pkg/database/interface.go:17-109
type Database interface {
    // Event persistence
    SaveEvent(c context.Context, ev *event.E) (exists bool, err error)
    QueryEvents(c context.Context, f *filter.F) (evs event.S, err error)
    DeleteEvent(c context.Context, eid []byte) error

    // Subscription management
    GetSubscription(pubkey []byte) (*Subscription, error)
    ExtendSubscription(pubkey []byte, days int) error

    // NIP-43 membership
    AddNIP43Member(pubkey []byte, inviteCode string) error
    IsNIP43Member(pubkey []byte) (isMember bool, err error)

    // ... 50+ methods
}
```

**Repository Implementations:**
1. **Badger** (`pkg/database/database.go`): Embedded key-value store
2. **Neo4j** (`pkg/neo4j/`): Graph database for social queries
3. **WasmDB** (`pkg/wasmdb/`): Browser IndexedDB for WASM builds

**Interface Segregation:**
```go
// pkg/interfaces/store/store_interface.go:21-37
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
    // ...
}
```

### Domain Services

Domain services encapsulate logic that doesn't belong to any single entity.

#### ACL Registry (Access Decision Service)
```go
// pkg/acl/acl.go:40-48
func (s *S) GetAccessLevel(pub []byte, address string) (level string) {
    for _, i := range s.ACL {
        if i.Type() == s.Active.Load() {
            level = i.GetAccessLevel(pub, address)
            break
        }
    }
    return
}
```
- Delegates to active ACL implementation
- Stateless decision based on pubkey and IP

#### Policy Manager (Event Validation Service)
```go
// pkg/policy/policy.go (P type, CheckPolicy method)
// Evaluates rule chains, scripts, whitelist/blacklist logic
```
- Complex rule evaluation logic
- Script execution for custom validation

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

### Domain Events

**Current State:** Domain events are implicit in message flow, not explicit types.

**Implicit Events Identified:**

| Event | Trigger | Effect |
|-------|---------|--------|
| EventPublished | `SaveEvent()` success | `publishers.Deliver()` |
| EventDeleted | Kind 5 processing | Cascade delete targets |
| UserAuthenticated | AUTH envelope accepted | `authedPubkey` set |
| SubscriptionCreated | REQ envelope | Query + stream setup |
| MembershipAdded | NIP-43 join request | ACL update |
| PolicyUpdated | Policy config event | `messagePauseMutex.Lock()` |

---

## Anti-Patterns Identified

### 1. Large Handler Methods (Partial Anemic Domain Model)

**Location:** `app/handle-event.go:183-783` (600+ lines)

**Issue:** The `HandleEvent` method contains:
- Input validation
- Policy checking
- ACL verification
- Signature verification
- Persistence
- Event delivery
- Special case handling (delete, ephemeral, NIP-43)

**Impact:** Difficult to test, maintain, and understand. Business rules are embedded in orchestration code.

### 2. Mutable Value Object Fields

**Location:** `pkg/interfaces/store/store_interface.go:63-68`

```go
type IdPkTs struct {
    Id  []byte   // Mutable slice
    Pub []byte   // Mutable slice
    Ts  int64
    Ser uint64
}
```

**Impact:** Value objects should be immutable. Callers could accidentally mutate shared state.

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

The `Rule` struct has 25+ fields, suggesting it might benefit from decomposition into smaller, focused value objects:
- `AccessRule` (allow/deny lists)
- `SizeRule` (limits)
- `TimeRule` (expiry, age)
- `ValidationRule` (tags, regex)

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

// Simple dispatcher
type Dispatcher struct {
    handlers map[reflect.Type][]func(DomainEvent)
}
```

**Benefits:**
- Decoupled side effects
- Easier testing
- Audit trail capability
- Foundation for event sourcing if needed

**Files to Modify:**
- Create `pkg/domain/events/`
- Update `app/handle-event.go` to emit events
- Update `app/handle-nip43.go` for membership events

### 2. Strengthen Aggregate Boundaries

**Problem:** Aggregate internals are exposed via public fields.

**Solution:** Use unexported fields with behavior methods.

```go
// Before (current)
type Listener struct {
    authedPubkey atomicutils.Bytes  // Accessible from outside
}

// After (recommended)
type Listener struct {
    authedPubkey atomicutils.Bytes  // Keep as is (already using atomic wrapper)
}

// Add behavior methods
func (l *Listener) IsAuthenticated() bool {
    return len(l.authedPubkey.Load()) > 0
}

func (l *Listener) AuthenticatedPubkey() []byte {
    return l.authedPubkey.Load()
}

func (l *Listener) Authenticate(pubkey []byte) error {
    if l.IsAuthenticated() {
        return ErrAlreadyAuthenticated
    }
    l.authedPubkey.Store(pubkey)
    return nil
}
```

**Benefits:**
- Enforces invariants
- Clear API surface
- Easier refactoring

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

func (s *EventService) ProcessIncomingEvent(ctx context.Context, ev *event.E, authedPubkey []byte) (*EventResult, error) {
    // 1. Validate event structure
    if err := s.validateEventStructure(ev); err != nil {
        return nil, err
    }

    // 2. Check policy
    if !s.policyMgr.IsAllowed("write", ev, authedPubkey) {
        return &EventResult{Blocked: true, Reason: "policy"}, nil
    }

    // 3. Check ACL
    if !s.aclRegistry.CanWrite(authedPubkey) {
        return &EventResult{Blocked: true, Reason: "acl"}, nil
    }

    // 4. Persist
    exists, err := s.db.SaveEvent(ctx, ev)
    if err != nil {
        return nil, err
    }

    // 5. Publish domain event
    s.eventPublisher.Publish(events.EventPublished{...})

    return &EventResult{Saved: !exists}, nil
}
```

**Benefits:**
- Testable business logic
- Handlers become thin orchestrators
- Reusable across different entry points (WebSocket, HTTP API, CLI)

### 4. Establish Ubiquitous Language Glossary

**Problem:** Terminology is inconsistent across the codebase.

**Current Inconsistencies:**
- "subscription" (payment) vs "subscription" (REQ filter)
- "monitor" (rate limit) vs "spider" (sync)
- "pub" vs "pubkey" vs "author"

**Solution:** Add a `GLOSSARY.md` and enforce terms in code reviews.

```markdown
# ORLY Ubiquitous Language

## Core Domain Terms

| Term | Definition | Code Symbol |
|------|------------|-------------|
| Event | A signed Nostr message | `event.E` |
| Relay | This server | `Server` |
| Connection | WebSocket session | `Listener` |
| Filter | Query criteria for events | `filter.F` |
| **Event Subscription** | Active filter receiving events | `subscriptions map` |
| **Payment Subscription** | Paid access tier | `database.Subscription` |
| Access Level | Permission tier (none/read/write/admin/owner) | `acl.Level` |
| Policy | Event validation rules | `policy.Rule` |
```

### 5. Add Domain-Specific Error Types

**Problem:** Errors are strings or generic types, making error handling imprecise.

**Solution:** Create typed domain errors.

```go
// pkg/domain/errors/errors.go
package errors

type DomainError struct {
    Code    string
    Message string
    Cause   error
}

var (
    ErrEventInvalid       = &DomainError{Code: "EVENT_INVALID"}
    ErrEventBlocked       = &DomainError{Code: "EVENT_BLOCKED"}
    ErrAuthRequired       = &DomainError{Code: "AUTH_REQUIRED"}
    ErrQuotaExceeded      = &DomainError{Code: "QUOTA_EXCEEDED"}
    ErrInviteCodeInvalid  = &DomainError{Code: "INVITE_INVALID"}
    ErrInviteCodeExpired  = &DomainError{Code: "INVITE_EXPIRED"}
    ErrInviteCodeUsed     = &DomainError{Code: "INVITE_USED"}
)
```

**Benefits:**
- Precise error handling in handlers
- Better error messages to clients
- Easier testing

### 6. Enforce Value Object Immutability

**Problem:** Value objects use mutable slices.

**Solution:** Return copies from accessors.

```go
// pkg/interfaces/store/store_interface.go
type IdPkTs struct {
    id  []byte  // unexported
    pub []byte  // unexported
    ts  int64
    ser uint64
}

func NewIdPkTs(id, pub []byte, ts int64, ser uint64) *IdPkTs {
    return &IdPkTs{
        id:  append([]byte(nil), id...),   // Copy
        pub: append([]byte(nil), pub...),  // Copy
        ts:  ts,
        ser: ser,
    }
}

func (i *IdPkTs) ID() []byte  { return append([]byte(nil), i.id...) }
func (i *IdPkTs) Pub() []byte { return append([]byte(nil), i.pub...) }
func (i *IdPkTs) Ts() int64   { return i.ts }
func (i *IdPkTs) Ser() uint64 { return i.ser }
```

### 7. Document Context Map

**Problem:** Context relationships are implicit.

**Solution:** Add a `CONTEXT_MAP.md` documenting boundaries and integration patterns.

The diagram in the [Context Map](#context-map) section above should be maintained as living documentation.

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

### Needs Attention

- [ ] Ubiquitous language documented and used consistently
- [ ] Context map documenting integration patterns
- [ ] Domain events capture important state changes
- [ ] Entities have behavior, not just data
- [ ] Value objects are fully immutable
- [ ] No business logic in application services (orchestration only)
- [ ] No infrastructure concerns in domain layer

---

## Appendix: File References

### Core Domain Files

| File | Purpose |
|------|---------|
| `pkg/database/interface.go` | Repository interface (50+ methods) |
| `pkg/interfaces/acl/acl.go` | ACL interface definition |
| `pkg/interfaces/store/store_interface.go` | Store sub-interfaces |
| `pkg/policy/policy.go` | Policy rules and evaluation |
| `pkg/protocol/nip43/types.go` | NIP-43 invite management |

### Application Layer Files

| File | Purpose |
|------|---------|
| `app/server.go` | HTTP/WebSocket server setup |
| `app/listener.go` | Connection aggregate |
| `app/handle-event.go` | EVENT message handler |
| `app/handle-req.go` | REQ message handler |
| `app/handle-auth.go` | AUTH message handler |
| `app/handle-nip43.go` | NIP-43 membership handlers |

### Infrastructure Files

| File | Purpose |
|------|---------|
| `pkg/database/database.go` | Badger implementation |
| `pkg/neo4j/` | Neo4j implementation |
| `pkg/wasmdb/` | WasmDB implementation |
| `pkg/ratelimit/limiter.go` | Rate limiting |
| `pkg/sync/manager.go` | Distributed sync |

---

*Generated: 2025-12-23*
*Analysis based on ORLY codebase v0.36.10*
