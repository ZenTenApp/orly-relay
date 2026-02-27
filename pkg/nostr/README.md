# Nostr Library

A comprehensive, high-performance Go implementation of the Nostr protocol providing encoding/decoding, cryptography, WebSocket client, and relay utilities. This library offers a complete toolkit for building Nostr clients and relays with optimized performance through SIMD acceleration and zero-copy operations.

## Features

- **Pure Go with Purego** - No CGO dependencies, uses `ebitengine/purego` to dynamically load `libsecp256k1.so`
- **SIMD Optimization** - Accelerated SHA256 (`minio/sha256-simd`) and hex encoding (`templexxx/xhex`)
- **Zero-Copy Operations** - Efficient buffer pooling and reuse
- **Complete NIP Support** - Events, filters, tags, envelopes, and cryptographic operations
- **WebSocket Client** - Full-featured client for connecting to Nostr relays
- **Relay Information** - NIP-11 relay information document handling
- **HTTP Authentication** - NIP-98 HTTP auth for REST APIs
- **Multiple Encodings** - JSON, binary, and canonical encodings for events
- **Comprehensive Crypto** - Schnorr signatures, ECDH, NIP-04/NIP-44 encryption, MuSig2

## Installation

```bash
go get git.mleku.dev/mleku/nostr
```

## Quick Start

### Connect to a Relay and Subscribe

```go
package main

import (
    "context"
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/filter"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()

    // Connect to relay
    relay, err := ws.RelayConnect(ctx, "wss://relay.damus.io")
    if err != nil {
        panic(err)
    }
    defer relay.Close()

    // Create filter
    f := filter.New()
    f.Kinds = kind.NewS(kind.TextNote)
    limit := 10
    f.Limit = &limit

    // Subscribe to events
    sub, err := relay.Subscribe(ctx, filter.NewS(f))
    if err != nil {
        panic(err)
    }

    // Process events
    for event := range sub.Events {
        fmt.Printf("Event: %s\n", string(event.Content))
    }
}
```

## Table of Contents

- [WebSocket Client](#websocket-client) - Connect to Nostr relays
- [Event Package](#event-package) - Event encoding/decoding and manipulation
- [Filter Package](#filter-package) - Query filters for event subscriptions
- [Tag Package](#tag-package) - Tag encoding and manipulation
- [Kind Package](#kind-package) - Event kind constants and utilities
- [Envelope Packages](#envelope-packages) - WebSocket message envelopes
- [Relay Information](#relay-information) - NIP-11 relay info documents
- [HTTP Authentication](#http-authentication) - NIP-98 HTTP auth
- [Cryptography](#cryptography) - Signatures, encryption, and key management
- [Protocol Helpers](#protocol-helpers) - NIP-42 auth and utilities
- [Utilities](#utilities) - Helpers and tools

---

## WebSocket Client

The `ws` package provides a complete WebSocket client implementation for connecting to Nostr relays.

### Features

- Connect to relays with context-based lifecycle management
- Subscribe to events with filters
- Publish events with OK callbacks
- NIP-42 authentication support
- Automatic EOSE (End Of Stored Events) detection
- Duplicate event checking
- Replaceable event handling
- Custom message handlers
- Notice handling

### Connecting to a Relay

```go
package main

import (
    "context"

    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()

    // Connect to relay
    relay, err := ws.RelayConnect(ctx, "wss://relay.damus.io")
    if err != nil {
        panic(err)
    }
    defer relay.Close()

    // Relay is now ready to use
}
```

### Subscribing to Events

```go
package main

import (
    "context"
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/filter"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()
    relay, _ := ws.RelayConnect(ctx, "wss://relay.damus.io")
    defer relay.Close()

    // Create filters
    f := filter.New()
    f.Kinds = kind.NewS(kind.TextNote, kind.Reaction)
    limit := 100
    f.Limit = &limit

    // Subscribe with options
    sub, err := relay.Subscribe(ctx, filter.NewS(f),
        ws.WithLabel("my-subscription"),
    )
    if err != nil {
        panic(err)
    }

    // Process events as they arrive
    for event := range sub.Events {
        fmt.Printf("Event %x: %s\n", event.ID, string(event.Content))
    }

    // Check if EOSE was received
    if sub.EndOfStoredEvents.Load() {
        fmt.Println("Received all stored events")
    }
}
```

### Publishing Events

```go
package main

import (
    "context"
    "fmt"
    "time"

    "git.mleku.dev/mleku/nostr/encoders/event"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()
    relay, _ := ws.RelayConnect(ctx, "wss://relay.damus.io")
    defer relay.Close()

    // Create signer
    signer, _ := p8k.New()
    signer.Generate()

    // Create event
    ev := event.New()
    ev.Kind = kind.TextNote.K
    ev.CreatedAt = time.Now().Unix()
    ev.Content = []byte("Hello Nostr!")
    ev.Sign(signer)

    // Publish event
    err := relay.Publish(ctx, ev)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Published event: %x\n", ev.ID)
}
```

### NIP-42 Authentication

```go
package main

import (
    "context"

    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()
    relay, _ := ws.RelayConnect(ctx, "wss://relay.example.com")
    defer relay.Close()

    // Create signer
    signer, _ := p8k.New()
    signer.Generate()

    // Authenticate with relay
    err := relay.Auth(ctx, signer)
    if err != nil {
        panic(err)
    }
}
```

### Subscription Options

```go
// WithLabel sets the subscription label
sub, _ := relay.Subscribe(ctx, filters, ws.WithLabel("my-sub"))

// Subscription automatically closes when context is cancelled
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
sub, _ := relay.Subscribe(ctx, filters)
```

---

## Event Package

The `event` package provides the core Nostr event type with JSON, binary, and canonical encodings.

### Types

```go
type E struct {
    ID        []byte  // SHA256 hash of canonical encoding
    Pubkey    []byte  // Public key (32 bytes)
    CreatedAt int64   // UNIX timestamp
    Kind      uint16  // Event kind
    Tags      *tag.S  // Tag list
    Content   []byte  // Arbitrary content
    Sig       []byte  // Schnorr signature (64 bytes)
}

type S []*E  // Slice of events (sortable by CreatedAt)
type C chan *E  // Channel for event streaming
```

### Constructor

```go
func New() *E
```

### Methods

#### Lifecycle

```go
func (ev *E) Free()                    // Nil all fields for GC
func (ev *E) Clone() *E                // Deep copy with independent memory
func (ev *E) EstimateSize() int        // Estimate serialized size
```

#### Encoding/Decoding

```go
func (ev *E) Marshal(dst []byte) []byte           // Marshal to JSON
func (ev *E) MarshalJSON() ([]byte, error)        // Standard JSON marshaler
func (ev *E) Serialize() []byte                   // Marshal to new buffer
func (ev *E) Unmarshal(b []byte) ([]byte, error)  // Unmarshal from JSON
func (ev *E) UnmarshalJSON(b []byte) error        // Standard JSON unmarshaler
```

#### Binary Encoding

```go
func (ev *E) MarshalBinary(w io.Writer)              // Write binary format
func (ev *E) MarshalBinaryToBytes(dst []byte) []byte // Binary to bytes
func (ev *E) UnmarshalBinary(r io.Reader) error      // Read binary format
```

#### Canonical Encoding

```go
func (ev *E) ToCanonical(dst []byte) []byte  // Canonical encoding for ID
func (ev *E) GetIDBytes() []byte             // Compute event ID
func Hash(in []byte) []byte                  // SHA256 hash
```

#### Signatures

```go
func (ev *E) Sign(keys signer.I) error           // Sign event with key pair
func (ev *E) Verify() (bool, error)              // Verify signature
```

### Usage Example

```go
package main

import (
    "fmt"
    "time"

    "git.mleku.dev/mleku/nostr/encoders/event"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/encoders/tag"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    // Create a signer
    signer, _ := p8k.New()
    signer.Generate()

    // Create an event
    ev := event.New()
    ev.Kind = kind.TextNote.K
    ev.CreatedAt = time.Now().Unix()
    ev.Content = []byte("Hello Nostr!")

    // Add tags
    ev.Tags = tag.NewS(
        tag.NewFromBytesSlice([]byte("t"), []byte("nostr")),
        tag.NewFromBytesSlice([]byte("p"), signer.Pub()),
    )

    // Sign the event
    if err := ev.Sign(signer); err != nil {
        panic(err)
    }

    // Verify signature
    valid, err := ev.Verify()
    if err != nil || !valid {
        panic("invalid signature")
    }

    // Marshal to JSON
    jsonBytes := ev.Serialize()
    fmt.Printf("Event JSON: %s\n", jsonBytes)

    // Unmarshal from JSON
    ev2 := event.New()
    _, err = ev2.Unmarshal(jsonBytes)
    if err != nil {
        panic(err)
    }

    // Clone for async processing
    clone := ev.Clone()
    go processEvent(clone)

    // Free original
    ev.Free()
}

func processEvent(ev *event.E) {
    defer ev.Free()
    // Process event...
}
```

---

## Filter Package

The `filter` package implements Nostr subscription filters for querying events.

### Type

```go
type F struct {
    IDs     *schnorr.Bytes  // Event IDs (hex-encoded in JSON)
    Authors *schnorr.Bytes  // Author pubkeys (hex-encoded in JSON)
    Kinds   *kind.S         // Event kinds
    Tags    *tag.S          // Tag filters (#e, #p, etc.)
    Since   *int64          // UNIX timestamp
    Until   *int64          // UNIX timestamp
    Limit   *int            // Max results
}
```

### Constructor

```go
func New() *F
```

### Methods

```go
func (f *F) Sort()                                    // Sort fields for deterministic output
func (f *F) Matches(ev *event.E) bool                 // Check if event matches filter
func (f *F) MatchesIgnoringTimestampConstraints(ev *event.E) bool
func (f *F) EstimateSize() int                        // Estimate serialized size
func (f *F) Marshal(dst []byte) []byte                // Marshal to JSON
func (f *F) Serialize() []byte                        // Marshal to new buffer
func (f *F) Unmarshal(b []byte) ([]byte, error)       // Unmarshal from JSON
```

### Usage Example

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
    "git.mleku.dev/mleku/nostr/encoders/event"
    "git.mleku.dev/mleku/nostr/encoders/filter"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/encoders/tag"
)

func main() {
    // Create a filter
    f := filter.New()

    // Filter by kinds
    f.Kinds = kind.NewS(kind.TextNote, kind.EncryptedDirectMessage)

    // Filter by authors
    pubkey, _ := schnorr.ParseBytes("...")
    f.Authors = schnorr.NewBytes(pubkey)

    // Time constraints
    since := int64(1234567890)
    until := int64(9999999999)
    limit := 100
    f.Since = &since
    f.Until = &until
    f.Limit = &limit

    // Tag filters (e.g., #p tag)
    f.Tags = tag.NewS(
        tag.NewFromBytesSlice([]byte("#p"), pubkey),
    )

    // Sort for deterministic output
    f.Sort()

    // Marshal to JSON
    jsonBytes := f.Serialize()
    fmt.Printf("Filter: %s\n", jsonBytes)

    // Check if event matches
    ev := event.New()
    ev.Kind = kind.TextNote.K
    if f.Matches(ev) {
        fmt.Println("Event matches filter")
    }
}
```

---

## Tag Package

The `tag` package provides tag encoding and manipulation.

### Types

```go
type T struct {
    T [][]byte  // Tag elements (first is key, rest are values)
}

type S []*T  // Slice of tags
```

### Constructors

```go
func New() *T
func NewWithCap(c int) *T
func NewFromBytesSlice(t ...[]byte) *T
func NewFromAny(t ...any) *T

func NewS(t ...*T) *S
func NewSWithCap(c int) *S
```

### Tag Methods

```go
func (t *T) Free()                                  // Nil all fields
func (t *T) Len() int                               // Number of elements
func (t *T) Less(i, j int) bool                     // Compare elements
func (t *T) Swap(i, j int)                          // Swap elements
func (t *T) Contains(s []byte) bool                 // Check if contains element
func (t *T) Marshal(dst []byte) []byte              // Marshal to JSON
func (t *T) MarshalJSON() ([]byte, error)           // Standard marshaler
func (t *T) Unmarshal(b []byte) ([]byte, error)     // Unmarshal from JSON
func (t *T) UnmarshalJSON(b []byte) error           // Standard unmarshaler
func (t *T) Key() []byte                            // Get first element (key)
func (t *T) Value() []byte                          // Get second element (value)
func (t *T) Relay() []byte                          // Get third element (relay)
func (t *T) ValueHex() []byte                       // Get value as hex
func (t *T) ValueBinary() []byte                    // Get value as binary
func (t *T) ToSliceOfStrings() []string             // Convert to string slice
func (t *T) Equals(other *T) bool                   // Compare tags
```

### Tag Slice Methods

```go
func (s *S) Len() int
func (s *S) Less(i, j int) bool
func (s *S) Swap(i, j int)
func (s *S) Append(t ...*T)
func (s *S) Marshal(dst []byte) []byte
func (s *S) Unmarshal(b []byte) ([]byte, error)
func (s *S) GetFirst(tagName []byte) *T             // Get first tag with key
func (s *S) GetAll(tagName []byte) []*T             // Get all tags with key
func (s *S) FilterOut(tagName []byte) *S            // Remove tags with key
```

### Usage Example

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/hex"
    "git.mleku.dev/mleku/nostr/encoders/tag"
)

func main() {
    // Create individual tags
    eTag := tag.NewFromBytesSlice(
        []byte("e"),
        hex.Dec("...event-id..."),
        []byte("wss://relay.example.com"),
        []byte("reply"),
    )

    pTag := tag.NewFromBytesSlice(
        []byte("p"),
        hex.Dec("...pubkey..."),
    )

    tTag := tag.NewFromAny("t", "nostr", "protocol")

    // Create tag collection
    tags := tag.NewS(eTag, pTag, tTag)

    // Access tag elements
    fmt.Printf("Key: %s\n", eTag.Key())        // "e"
    fmt.Printf("Value: %s\n", eTag.ValueHex()) // event-id as hex (handles binary storage)
    fmt.Printf("Relay: %s\n", eTag.Relay())    // relay URL

    // Find tags - use ValueHex() for e/p tags (may be binary-encoded internally)
    pTags := tags.GetAll([]byte("p"))
    for _, pt := range pTags {
        fmt.Printf("P tag: %s\n", pt.ValueHex()) // Always returns hex regardless of storage
    }

    // Filter tags
    withoutE := tags.FilterOut([]byte("e"))

    // Marshal to JSON
    jsonBytes := tags.Marshal(nil)
    fmt.Printf("Tags: %s\n", jsonBytes)
}
```

---

## Kind Package

The `kind` package provides event kind constants and utilities.

### Type

```go
type K struct {
    K uint16  // Kind number
}

type S struct {
    K []*K  // Slice of kinds (sortable)
}
```

### Constructors

```go
func New(k uint16) *K
func NewS(k ...*K) *S
func NewWithCap(c int) *S
func FromIntSlice(is []int) *S
```

### Constants

```go
var (
    Metadata                = New(0)     // NIP-01
    TextNote                = New(1)     // NIP-01
    RecommendRelay          = New(2)     // NIP-01
    Contacts                = New(3)     // NIP-02
    EncryptedDirectMessage  = New(4)     // NIP-04
    EventDeletion           = New(5)     // NIP-09
    Repost                  = New(6)     // NIP-18
    Reaction                = New(7)     // NIP-25
    BadgeAward              = New(8)     // NIP-58
    ChannelCreation         = New(40)    // NIP-28
    ChannelMetadata         = New(41)    // NIP-28
    ChannelMessage          = New(42)    // NIP-28
    ChannelHideMessage      = New(43)    // NIP-28
    ChannelMuteUser         = New(44)    // NIP-28
    FileMetadata            = New(1063)  // NIP-94
    LiveChatMessage         = New(1311)  // NIP-53

    // Replaceable events (10000-19999)
    ProfileBadges           = New(30008) // NIP-58
    BadgeDefinition         = New(30009) // NIP-58

    // Ephemeral events (20000-29999)
    Auth                    = New(22242) // NIP-42

    // ... and 200+ more kinds
)
```

### Methods

```go
func (k *S) Len() int
func (k *S) Less(i, j int) bool
func (k *S) Swap(i, j int)
func (k *S) ToUint16() []uint16
func (k *S) Clone() *S
func (k *S) Contains(s uint16) bool
func (k *S) Equals(t1 *S) bool
func (k *S) Marshal(dst []byte) []byte
func (k *S) Unmarshal(b []byte) ([]byte, error)
func (k *S) IsPrivileged() bool                     // Check if contains privileged kinds
func (k *K) IsRegular() bool                        // 1000 <= k < 10000
func (k *K) IsReplaceable() bool                    // 10000 <= k < 20000 or k in [0,3]
func (k *K) IsEphemeral() bool                      // 20000 <= k < 30000
func (k *K) IsAddressable() bool                    // 30000 <= k < 40000
```

### Usage Example

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/kind"
)

func main() {
    // Use predefined constants
    textNote := kind.TextNote
    fmt.Printf("Text note kind: %d\n", textNote.K)

    // Create custom kind
    customKind := kind.New(30023)

    // Check kind properties
    if customKind.IsAddressable() {
        fmt.Println("This is an addressable event")
    }

    // Create kind filter
    kinds := kind.NewS(
        kind.TextNote,
        kind.Reaction,
        kind.Repost,
    )

    // Check if contains kind
    if kinds.Contains(1) {
        fmt.Println("Contains text notes")
    }

    // Convert to uint16 slice
    kindNumbers := kinds.ToUint16()
    fmt.Printf("Kinds: %v\n", kindNumbers)
}
```

---

## Envelope Packages

Envelope packages provide WebSocket message framing for the Nostr protocol.

### Available Envelopes

- `eventenvelope` - EVENT messages (client to relay)
- `reqenvelope` - REQ messages (subscription requests)
- `closeenvelope` - CLOSE messages (close subscription)
- `eoseenvelope` - EOSE messages (end of stored events)
- `okenvelope` - OK messages (command results)
- `noticeenvelope` - NOTICE messages (human-readable messages)
- `authenvelope` - AUTH messages (NIP-42 authentication)
- `closedenvelope` - CLOSED messages (subscription closed by relay)
- `countenvelope` - COUNT messages (event counting)

### Common Pattern

All envelopes follow a similar pattern:

```go
// Create envelope
env := eventenvelope.NewSubmission()

// Unmarshal from wire format
remainder, err := env.Unmarshal(wireBytes)

// Access data
event := env.E

// Marshal to wire format
wireBytes = env.Marshal(nil)
```

### Event Envelope Example

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
    "git.mleku.dev/mleku/nostr/encoders/event"
)

func main() {
    // Client submitting event
    ev := event.New()
    // ... populate event ...

    submission := eventenvelope.NewSubmission()
    submission.E = ev
    wireBytes := submission.Marshal(nil)

    // Send wireBytes over WebSocket

    // Server receiving event
    received := eventenvelope.NewSubmission()
    _, err := received.Unmarshal(wireBytes)
    if err != nil {
        panic(err)
    }

    // Process received.E
    fmt.Printf("Received event: %x\n", received.E.ID)
}
```

### REQ Envelope Example

```go
package main

import (
    "git.mleku.dev/mleku/nostr/encoders/envelopes/reqenvelope"
    "git.mleku.dev/mleku/nostr/encoders/filter"
)

func main() {
    // Create subscription request
    req := reqenvelope.New()
    req.Label = []byte("my-subscription")

    // Add filters
    f1 := filter.New()
    // ... configure filter ...
    req.Filters = append(req.Filters, f1)

    // Marshal
    wireBytes := req.Marshal(nil)

    // Send over WebSocket
}
```

### OK Envelope Example

```go
package main

import (
    "git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
    "git.mleku.dev/mleku/nostr/encoders/reason"
)

func main() {
    // Create OK response
    ok := okenvelope.New()
    ok.ID = eventID
    ok.OK = true
    ok.Reason = reason.Duplicate.With(": event already exists")

    // Marshal
    wireBytes := ok.Marshal(nil)

    // Send to client
}
```

---

## Relay Information

The `relayinfo` package implements NIP-11 relay information document handling.

### Features

- Fetch relay information from HTTP endpoint
- Comprehensive NIP database (80+ NIPs with descriptions)
- Relay limits and restrictions
- Payment/fee structures
- Save/load relay info to/from JSON files

### Types

```go
type T struct {
    Name           []byte
    Description    []byte
    Pubkey         []byte
    Contact        []byte
    Nips           []int
    Software       []byte
    Version        []byte
    Limitation     *Limitation
    Payments_url   []byte
    Fees           *Fees
    Icon           []byte
    // ... additional fields
}

type Limitation struct {
    MaxMessageLength      *int
    MaxSubscriptions      *int
    MaxFilters            *int
    MaxLimit              *int
    MaxSubidLength        *int
    MaxEventTags          *int
    MaxContentLength      *int
    MinPowDifficulty      *int
    AuthRequired          *bool
    PaymentRequired       *bool
    RestrictedWrites      *bool
    CreatedAtLowerLimit   *int64
    CreatedAtUpperLimit   *int64
}
```

### Methods

```go
func Fetch(ctx context.Context, u []byte) (*T, error)    // Fetch from relay
func Load(filePath []byte) (*T, error)                   // Load from file
func (t *T) Save(filePath []byte) error                  // Save to file
func (t *T) Marshal(dst []byte) []byte                   // Marshal to JSON
func (t *T) Unmarshal(b []byte) ([]byte, error)          // Unmarshal from JSON
```

### Usage Example

```go
package main

import (
    "context"
    "fmt"

    "git.mleku.dev/mleku/nostr/relayinfo"
)

func main() {
    ctx := context.Background()

    // Fetch relay information
    info, err := relayinfo.Fetch(ctx, []byte("wss://relay.damus.io"))
    if err != nil {
        panic(err)
    }

    // Display relay info
    fmt.Printf("Relay: %s\n", info.Name)
    fmt.Printf("Description: %s\n", info.Description)
    fmt.Printf("NIPs supported: %v\n", info.Nips)
    fmt.Printf("Software: %s %s\n", info.Software, info.Version)

    // Check limitations
    if info.Limitation != nil {
        if info.Limitation.MaxMessageLength != nil {
            fmt.Printf("Max message length: %d\n", *info.Limitation.MaxMessageLength)
        }
        if info.Limitation.AuthRequired != nil && *info.Limitation.AuthRequired {
            fmt.Println("Authentication required")
        }
    }

    // Save to file
    err = info.Save([]byte("relay-info.json"))
    if err != nil {
        panic(err)
    }

    // Load from file
    loaded, err := relayinfo.Load([]byte("relay-info.json"))
    if err != nil {
        panic(err)
    }
}
```

---

## HTTP Authentication

The `httpauth` package implements NIP-98 HTTP authentication for REST APIs.

### Features

- Create NIP-98 authentication events
- Add Authorization headers to HTTP requests
- Support for payload hashing
- Expiration support
- Custom JWT-style delegation (kind 13004)

### Functions

```go
func AddNIP98Header(req *http.Request, u *url.URL, method, payload string,
    signer signer.I, expiration int64) error

func CreateAuthEvent(u *url.URL, method, payload string,
    signer signer.I, expiration int64) (*event.E, error)
```

### Usage Example

```go
package main

import (
    "net/http"
    "net/url"
    "time"

    "git.mleku.dev/mleku/nostr/httpauth"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    // Create signer
    signer, _ := p8k.New()
    signer.Generate()

    // Create HTTP request
    u, _ := url.Parse("https://api.example.com/upload")
    req, _ := http.NewRequest("POST", u.String(), nil)

    // Add NIP-98 auth header (expires in 1 hour)
    expiration := time.Now().Add(1 * time.Hour).Unix()
    err := httpauth.AddNIP98Header(req, u, "POST", "", signer, expiration)
    if err != nil {
        panic(err)
    }

    // Make request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
}
```

---

## Cryptography

### Signer Interface

The `signer.I` interface abstracts key management and cryptographic operations.

```go
type I interface {
    Generate() error                              // Generate new key pair
    InitSec(sec []byte) error                     // Load secret key
    InitPub(pub []byte) error                     // Load public key
    Sec() []byte                                  // Get secret key
    Pub() []byte                                  // Get public key (x-only)
    Sign(msg []byte) ([]byte, error)              // Sign message
    Verify(msg, sig []byte) (bool, error)         // Verify signature
    Zero()                                        // Wipe secret key
    ECDH(pub []byte) ([]byte, error)              // Derive shared secret
    ECDHRaw(pub []byte) ([]byte, error)           // Raw ECDH (for NIP-44)
}
```

### P8K Implementation

The `p8k` package provides implementations using both libsecp256k1 (via purego) and pure Go fallback.

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    // Create new signer
    signer, err := p8k.New()
    if err != nil {
        panic(err)
    }

    // Generate random key pair
    if err := signer.Generate(); err != nil {
        panic(err)
    }

    fmt.Printf("Public key: %x\n", signer.Pub())
    fmt.Printf("Secret key: %x\n", signer.Sec())

    // Sign message
    msg := []byte("hello")
    sig, err := signer.Sign(msg)
    if err != nil {
        panic(err)
    }

    // Verify signature
    valid, err := signer.Verify(msg, sig)
    if err != nil || !valid {
        panic("invalid signature")
    }

    // ECDH for encryption
    recipientPub := []byte{/* 32 bytes */}
    sharedSecret, err := signer.ECDH(recipientPub)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Shared secret: %x\n", sharedSecret)

    // Wipe keys when done
    defer signer.Zero()
}
```

### Loading Existing Keys

```go
package main

import (
    "git.mleku.dev/mleku/nostr/encoders/hex"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    signer, _ := p8k.New()

    // Load from hex secret key
    secHex := "..."
    secBytes := hex.Dec(secHex)

    if err := signer.InitSec(secBytes); err != nil {
        panic(err)
    }

    // Public key is automatically derived
    fmt.Printf("Loaded pubkey: %x\n", signer.Pub())
}
```

### Schnorr Signatures

The `schnorr` package provides low-level signature operations.

```go
package main

import (
    "git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
)

func main() {
    // Parse public key
    pubBytes, err := schnorr.ParseBytes("hex-pubkey")
    if err != nil {
        panic(err)
    }

    // Verify signature
    msg := []byte("message hash")
    sigBytes := []byte{/* 64 bytes */}

    if !schnorr.Verify(pubBytes, msg, sigBytes) {
        panic("invalid signature")
    }
}
```

### Encryption (NIP-04 and NIP-44)

```go
package main

import (
    "git.mleku.dev/mleku/nostr/crypto/encryption"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    // Sender keys
    sender, _ := p8k.New()
    sender.Generate()

    // Recipient keys
    recipient, _ := p8k.New()
    recipient.Generate()

    plaintext := "Secret message"

    // NIP-44 encryption (recommended)
    ciphertext, err := encryption.Nip44Encrypt(
        plaintext,
        sender,
        recipient.Pub(),
    )
    if err != nil {
        panic(err)
    }

    // NIP-44 decryption
    decrypted, err := encryption.Nip44Decrypt(
        ciphertext,
        recipient,
        sender.Pub(),
    )
    if err != nil {
        panic(err)
    }

    // NIP-04 encryption (legacy)
    ciphertext04, err := encryption.Nip04Encrypt(
        plaintext,
        sender,
        recipient.Pub(),
    )
    if err != nil {
        panic(err)
    }

    // NIP-04 decryption
    decrypted04, err := encryption.Nip04Decrypt(
        ciphertext04,
        recipient,
        sender.Pub(),
    )
}
```

### Bech32 Encoding

The `bech32encoding` package provides npub/nsec/note encoding.

```go
package main

import (
    "fmt"

    "git.mleku.dev/mleku/nostr/encoders/bech32encoding"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func main() {
    signer, _ := p8k.New()
    signer.Generate()

    // Encode public key as npub
    npub := bech32encoding.PubkeyToNpub(signer.Pub())
    fmt.Printf("npub: %s\n", npub)

    // Encode secret key as nsec
    nsec := bech32encoding.SecToNsec(signer.Sec())
    fmt.Printf("nsec: %s\n", nsec)

    // Decode npub
    pubBytes, err := bech32encoding.NpubToPubkey(npub)
    if err != nil {
        panic(err)
    }

    // Decode nsec
    secBytes, err := bech32encoding.NsecToSec(nsec)
    if err != nil {
        panic(err)
    }

    // Encode event ID as note
    eventID := []byte{/* 32 bytes */}
    note := bech32encoding.EventIDToNote(eventID)
    fmt.Printf("note: %s\n", note)

    // Decode note
    decodedID, err := bech32encoding.NoteToEventID(note)
    if err != nil {
        panic(err)
    }
}
```

---

## Protocol Helpers

### NIP-42 Authentication

The `protocol/auth` package provides utilities for NIP-42 authentication.

```go
package main

import (
    "time"

    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
    "git.mleku.dev/mleku/nostr/protocol/auth"
)

func main() {
    signer, _ := p8k.New()
    signer.Generate()

    // Create auth event
    challenge := []byte("challenge-from-relay")
    relayURL := []byte("wss://relay.example.com")

    authEvent, err := auth.CreateAuthEvent(challenge, relayURL, signer)
    if err != nil {
        panic(err)
    }

    // Validate auth event
    valid, err := auth.ValidateAuthEvent(authEvent, challenge, relayURL, time.Minute)
    if err != nil || !valid {
        panic("invalid auth event")
    }
}
```

---

## Utilities

### Buffer Pool

The `bufpool` package provides efficient buffer pooling for zero-copy operations.

```go
package main

import (
    "git.mleku.dev/mleku/nostr/utils/bufpool"
)

func main() {
    // Get buffer from pool
    buf := bufpool.Get()
    defer bufpool.Put(buf)

    // Use buffer
    buf = append(buf, []byte("data")...)

    // Buffer is returned to pool on Put
}
```

### URL Normalization

The `normalize` package provides URL and message normalization utilities.

```go
package main

import (
    "git.mleku.dev/mleku/nostr/utils/normalize"
)

func main() {
    // Normalize WebSocket URL
    normalized := normalize.URL([]byte("relay.damus.io"))
    // Returns: "wss://relay.damus.io"

    // HTTP to WebSocket conversion
    wsURL := normalize.URL([]byte("https://relay.example.com"))
    // Returns: "wss://relay.example.com"

    // Port handling
    withPort := normalize.URL([]byte("relay.example.com:443"))
    // Returns: "wss://relay.example.com"
}
```

### Hex Encoding

The `hex` package provides SIMD-accelerated hex encoding.

```go
package main

import (
    "git.mleku.dev/mleku/nostr/encoders/hex"
)

func main() {
    // Encode to hex
    data := []byte{0x01, 0x02, 0x03}
    hexStr := hex.Enc(data)

    // Append to buffer
    buf := make([]byte, 0, 64)
    buf = hex.EncAppend(buf, data)

    // Decode from hex
    decoded := hex.Dec("010203")
}
```

### Text Utilities

The `text` package provides JSON escaping and text processing.

```go
package main

import (
    "git.mleku.dev/mleku/nostr/encoders/text"
)

func main() {
    // Escape for JSON
    raw := []byte(`Hello "world"`)
    escaped := text.EscapeJSONString(nil, raw)

    // Unescape from JSON
    unescaped := text.UnescapeJSONString(nil, escaped)

    // Check if needs escaping
    if text.NeedsEscape(raw) {
        // Handle escaping
    }
}
```

---

## Performance Optimizations

### SIMD Acceleration

- **SHA256**: Uses `minio/sha256-simd` for hardware-accelerated hashing
- **Hex Encoding**: Uses `templexxx/xhex` for SIMD hex encoding

### Zero-Copy Operations

- Buffer pooling via `bufpool` package
- Reusable byte slices in all encoders
- `Marshal(dst []byte)` methods append to existing buffers

### Memory Management

```go
// Efficient encoding pattern
buf := bufpool.Get()
defer bufpool.Put(buf)

buf = event.Marshal(buf)  // Append to buffer
buf = filter.Marshal(buf) // Append more
```

### Binary Encoding

For maximum performance, use binary encoding instead of JSON:

```go
// Binary encoding (more compact, faster)
var buf bytes.Buffer
ev.MarshalBinary(&buf)

// Or to bytes
binBytes := ev.MarshalBinaryToBytes(nil)
```

---

## Testing

Run tests with:

```bash
go test ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./encoders/event
go test -bench=. -benchmem ./encoders/filter
```

---

## Dependencies

### Required

- `github.com/minio/sha256-simd` - SIMD-accelerated SHA256
- `github.com/templexxx/xhex` - SIMD hex encoding
- `github.com/ebitengine/purego` - CGO-free library loading
- `github.com/gorilla/websocket` - WebSocket implementation
- `lol.mleku.dev` - Logging and error handling
- `golang.org/x/crypto` - Cryptographic primitives
- `golang.org/x/exp` - Experimental packages (constraints)
- `lukechampine.com/frand` - Fast random number generation
- `github.com/puzpuzpuz/xsync/v3` - Concurrent data structures

### System Requirements

- `libsecp256k1.so` is embedded in the p8k package for Linux
- Go 1.25.3 or later

---

## Architecture

### Package Structure

```
git.mleku.dev/mleku/nostr/
├── crypto/              # Cryptographic operations
│   ├── ec/              # Elliptic curve crypto
│   │   ├── schnorr/     # Schnorr signatures
│   │   ├── secp256k1/   # secp256k1 curve
│   │   ├── bech32/      # Bech32 encoding
│   │   ├── musig2/      # MuSig2 multi-sig
│   │   ├── base58/      # Base58 encoding
│   │   ├── ecdsa/       # ECDSA signatures
│   │   ├── taproot/     # Taproot utilities
│   │   └── ...
│   ├── encryption/      # NIP-04/NIP-44
│   ├── keys/            # Key generation and conversion
│   └── p8k/             # Purego secp256k1
├── encoders/            # Nostr protocol encoders
│   ├── event/           # Event encoding
│   ├── filter/          # Filter encoding
│   ├── tag/             # Tag encoding
│   ├── kind/            # Kind constants
│   ├── envelopes/       # WebSocket envelopes
│   ├── hex/             # Hex encoding
│   ├── text/            # Text utilities
│   ├── bech32encoding/  # Bech32 encoding
│   ├── ints/            # Integer encoding
│   ├── reason/          # Reason codes
│   ├── timestamp/       # Timestamp handling
│   └── varint/          # Varint encoding
├── httpauth/            # NIP-98 HTTP auth
├── interfaces/          # Abstract interfaces
│   ├── signer/          # Signer interface
│   │   └── p8k/         # P8K implementation
│   └── codec/           # Codec interfaces
├── protocol/            # Protocol helpers
│   └── auth/            # NIP-42 auth
├── relayinfo/           # NIP-11 relay info
├── utils/               # Utility packages
│   ├── bufpool/         # Buffer pooling
│   ├── normalize/       # URL normalization
│   ├── constraints/     # Type constraints
│   ├── number/          # Number utilities
│   ├── pointers/        # Pointer utilities
│   ├── units/           # Size units
│   └── values/          # Value utilities
└── ws/                  # WebSocket client
```

### Design Principles

1. **Zero-Copy**: All `Marshal` methods accept `dst []byte` to append to existing buffers
2. **Buffer Pooling**: Use `bufpool.Get/Put` for temporary buffers
3. **Pure Go**: No CGO, uses purego for C library interop
4. **Performance**: SIMD acceleration where available
5. **Context-Based**: Lifecycle management with context.Context
6. **Correctness**: Extensive test coverage

---

## Complete Examples

### Full Relay Client Example

```go
package main

import (
    "context"
    "fmt"
    "time"

    "git.mleku.dev/mleku/nostr/encoders/event"
    "git.mleku.dev/mleku/nostr/encoders/filter"
    "git.mleku.dev/mleku/nostr/encoders/kind"
    "git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
    "git.mleku.dev/mleku/nostr/ws"
)

func main() {
    ctx := context.Background()

    // Initialize keys
    signer, _ := p8k.New()
    signer.Generate()

    // Connect to relay
    relay, err := ws.RelayConnect(ctx, "wss://relay.damus.io")
    if err != nil {
        panic(err)
    }
    defer relay.Close()

    // Create and publish event
    ev := event.New()
    ev.Kind = kind.TextNote.K
    ev.CreatedAt = time.Now().Unix()
    ev.Content = []byte("Hello Nostr from mleku/nostr!")
    ev.Sign(signer)

    // Publish event
    err = relay.Publish(ctx, ev)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Published event: %x\n", ev.ID)

    // Subscribe to events
    f := filter.New()
    f.Kinds = kind.NewS(kind.TextNote)
    limit := 10
    f.Limit = &limit

    sub, err := relay.Subscribe(ctx, filter.NewS(f))
    if err != nil {
        panic(err)
    }

    // Process events
    for event := range sub.Events {
        fmt.Printf("Event: %s\n", string(event.Content))
    }
}
```

---

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass: `go test ./...`
2. Code is formatted: `go fmt ./...`
3. Benchmarks show no regressions
4. Documentation is updated

---

## License

[Insert license information]

---

## Credits

This library is extracted from the [ORLY](https://next.orly.dev) relay implementation, designed for high-performance Nostr protocol handling.

### Key Technologies

- **Cryptography**: Uses Bitcoin Core's `libsecp256k1` via purego
- **SIMD**: MinIO's SHA256 and templexxx's hex encoder
- **Logging**: lol.mleku.dev logging framework
- **WebSocket**: Gorilla WebSocket

---

## See Also

- [Nostr Protocol](https://github.com/nostr-protocol/nostr)
- [NIPs](https://github.com/nostr-protocol/nips)
- [ORLY Relay](https://next.orly.dev)
