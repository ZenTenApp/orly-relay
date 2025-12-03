# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ORLY is a high-performance Nostr relay written in Go, designed for personal relays, small communities, and business deployments. It emphasizes low latency, custom cryptography optimizations, and embedded database performance.

**Key Technologies:**
- **Language**: Go 1.25.3+
- **Database**: Badger v4 (embedded) or Neo4j (social graph)
- **Cryptography**: Custom p8k library using purego for secp256k1 operations (no CGO)
- **Web UI**: Svelte frontend embedded in the binary
- **WebSocket**: gorilla/websocket for Nostr protocol
- **Performance**: SIMD-accelerated SHA256 and hex encoding, query result caching with zstd compression
- **Social Graph**: Neo4j backend with Web of Trust (WoT) extensions for trust metrics

## Build Commands

### Basic Build
```bash
# Build relay binary only
go build -o orly

# Pure Go build (no CGO) - this is the standard approach
CGO_ENABLED=0 go build -o orly
```

### Build with Web UI
```bash
# Recommended: Use the provided script
./scripts/update-embedded-web.sh

# Manual build
cd app/web
bun install
bun run build
cd ../../
go build -o orly
```

### Development Mode (Web UI Hot Reload)
```bash
# Terminal 1: Start relay with dev proxy
export ORLY_WEB_DISABLE=true
export ORLY_WEB_DEV_PROXY_URL=http://localhost:5173
./orly &

# Terminal 2: Start dev server
cd app/web && bun run dev
```

## Testing

### Run All Tests
```bash
# Standard test run
./scripts/test.sh

# Or manually with purego setup
CGO_ENABLED=0 go test ./...

# Note: libsecp256k1.so is included in the repository root
# Set LD_LIBRARY_PATH to use it: export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)"
```

### Run Specific Package Tests
```bash
# Test database package
cd pkg/database && go test -v ./...

# Test protocol package
cd pkg/protocol && go test -v ./...

# Test with specific test function
go test -v -run TestSaveEvent ./pkg/database
```

### Relay Protocol Testing
```bash
# Test relay protocol compliance
go run cmd/relay-tester/main.go -url ws://localhost:3334

# List available tests
go run cmd/relay-tester/main.go -list

# Run specific test
go run cmd/relay-tester/main.go -url ws://localhost:3334 -test "Basic Event"
```

### Benchmarking
```bash
# Run Go benchmarks in specific package
go test -bench=. -benchmem ./pkg/database

# Note: Crypto benchmarks are now in the external nostr library at:
# https://git.mleku.dev/mleku/nostr

# Run full relay benchmark suite
cd cmd/benchmark
go run main.go -data-dir /tmp/bench-db -events 10000 -workers 4

# Benchmark reports are saved to cmd/benchmark/reports/
# The benchmark tool tests event storage, queries, and subscription performance
```

## Running the Relay

### Basic Run
```bash
# Build and run
go build -o orly && ./orly

# With environment variables
export ORLY_LOG_LEVEL=debug
export ORLY_PORT=3334
./orly
```

### Get Relay Identity
```bash
# Print relay identity secret and pubkey
./orly identity
```

### Common Configuration
```bash
# TLS with Let's Encrypt
export ORLY_TLS_DOMAINS=relay.example.com

# Admin configuration
export ORLY_ADMINS=npub1...

# Follows ACL mode
export ORLY_ACL_MODE=follows

# Enable sprocket event processing
export ORLY_SPROCKET_ENABLED=true

# Enable policy system
export ORLY_POLICY_ENABLED=true

# Database backend selection (badger or neo4j)
export ORLY_DB_TYPE=badger

# Neo4j configuration (only when ORLY_DB_TYPE=neo4j)
export ORLY_NEO4J_URI=bolt://localhost:7687
export ORLY_NEO4J_USER=neo4j
export ORLY_NEO4J_PASSWORD=password

# Query cache configuration (improves REQ response times)
export ORLY_QUERY_CACHE_SIZE_MB=512      # Default: 512MB
export ORLY_QUERY_CACHE_MAX_AGE=5m       # Cache expiry time

# Database cache tuning (for Badger backend)
export ORLY_DB_BLOCK_CACHE_MB=512        # Block cache size
export ORLY_DB_INDEX_CACHE_MB=256        # Index cache size
export ORLY_INLINE_EVENT_THRESHOLD=1024  # Inline storage threshold (bytes)

# Directory Spider (metadata sync from other relays)
export ORLY_DIRECTORY_SPIDER=true        # Enable directory spider
export ORLY_DIRECTORY_SPIDER_INTERVAL=24h # How often to run
export ORLY_DIRECTORY_SPIDER_HOPS=3      # Max hops for relay discovery

# NIP-43 Relay Access Metadata
export ORLY_NIP43_ENABLED=true           # Enable invite system
export ORLY_NIP43_INVITE_EXPIRY=24h      # Invite code validity

# Authentication modes
export ORLY_AUTH_REQUIRED=false          # Require auth for all requests
export ORLY_AUTH_TO_WRITE=false          # Require auth only for writes
```

## Code Architecture

### Repository Structure

**Root Entry Point:**
- `main.go` - Application entry point with signal handling, profiling setup, and database initialization
- `app/main.go` - Core relay server initialization and lifecycle management

**Core Packages:**

**`app/`** - HTTP/WebSocket server and handlers
- `server.go` - Main Server struct and HTTP request routing
- `handle-*.go` - Nostr protocol message handlers (EVENT, REQ, COUNT, CLOSE, AUTH, DELETE)
- `handle-policy-config.go` - Kind 12345 policy updates and kind 3 admin follow list handling
- `handle-websocket.go` - WebSocket connection lifecycle and frame handling
- `listener.go` - Network listener setup
- `sprocket.go` - External event processing script manager
- `publisher.go` - Event broadcast to active subscriptions
- `payment_processor.go` - NWC integration for subscription payments
- `blossom.go` - Blob storage service initialization
- `web.go` - Embedded web UI serving and dev proxy
- `config/` - Environment variable configuration using go-simpler.org/env

**`pkg/database/`** - Database abstraction layer with multiple backend support
- `interface.go` - Database interface definition for pluggable backends
- `factory.go` - Database backend selection (Badger or Neo4j)
- `database.go` - Badger implementation with cache tuning and query cache
- `save-event.go` - Event storage with index updates
- `query-events.go` - Main query execution engine with filter normalization
- `query-for-*.go` - Specialized query builders for different filter patterns
- `indexes/` - Index key construction for efficient lookups
- `export.go` / `import.go` - Event export/import in JSONL format
- `subscriptions.go` - Active subscription tracking
- `identity.go` - Relay identity key management
- `migrations.go` - Database schema migration runner

**`pkg/neo4j/`** - Neo4j graph database backend with social graph support
- `neo4j.go` - Main database implementation
- `schema.go` - Graph schema and index definitions (includes WoT extensions)
- `query-events.go` - REQ filter to Cypher translation
- `save-event.go` - Event storage with relationship creation
- `social-event-processor.go` - Processes kinds 0, 3, 1984, 10000 for social graph
- `WOT_SPEC.md` - Web of Trust data model specification (NostrUser nodes, trust metrics)
- `MODIFYING_SCHEMA.md` - Guide for schema modifications

**`pkg/protocol/`** - Nostr protocol implementation
- `ws/` - WebSocket message framing and parsing
- `auth/` - NIP-42 authentication challenge/response
- `publish/` - Event publisher for broadcasting to subscriptions
- `relayinfo/` - NIP-11 relay information document
- `directory/` - Distributed directory service (NIP-XX)
- `nwc/` - Nostr Wallet Connect client
- `blossom/` - Blob storage protocol

**`pkg/encoders/`** - Optimized Nostr data encoding/decoding
- `event/` - Event JSON marshaling/unmarshaling with buffer pooling
- `filter/` - Filter parsing and validation
- `bech32encoding/` - npub/nsec/note encoding
- `hex/` - SIMD-accelerated hex encoding using templexxx/xhex
- `timestamp/`, `kind/`, `tag/` - Specialized field encoders

**Cryptographic operations** (from `git.mleku.dev/mleku/nostr` library)
- Pure Go secp256k1 using purego (no CGO) to dynamically load libsecp256k1.so
  - Schnorr signature operations (NIP-01)
  - ECDH for encrypted DMs (NIP-04, NIP-44)
  - Public key recovery from signatures
  - `libsecp256k1.so` - Included in repository root for runtime loading
- Key derivation and conversion utilities
- SIMD-accelerated SHA256 using minio/sha256-simd
- SIMD-accelerated hex encoding using templexxx/xhex

**`pkg/acl/`** - Access control systems
- `acl.go` - ACL registry and interface
- `follows.go` - Follows-based whitelist (admins + their follows can write)
- `managed.go` - NIP-86 managed relay with role-based permissions
- `none.go` - Open relay (no restrictions)

**`pkg/policy/`** - Event filtering and validation policies
- Policy configuration loaded from `~/.config/ORLY/policy.json`
- Per-kind size limits, age restrictions, custom scripts
- **Write-Only Validation**: Size, age, tag, and expiry validations apply ONLY to write operations
- **Read-Only Filtering**: `read_allow`, `read_deny`, `privileged` apply ONLY to read operations
- See `docs/POLICY_CONFIGURATION_REFERENCE.md` for authoritative read vs write applicability
- **Dynamic Policy Hot Reload via Kind 12345 Events:**
  - Policy admins can update policy configuration without relay restart
  - Kind 12345 events contain JSON policy in content field
  - Validation-first approach: JSON validated before pausing message processing
  - Message processing uses RWMutex: RLock for normal ops, Lock for policy updates
  - Policy admin follow lists (kind 3) trigger immediate cache refresh
  - `WriteAllowFollows` rule grants both read+write access to admin follows
  - Tag validation supports regex patterns per tag type
- **Policy Rule Fields:**
  - `max_expiry_duration`: ISO-8601 duration format (e.g., "P7D", "PT1H30M") for event expiry limits
  - `protected_required`: Requires NIP-70 protected events (must have "-" tag)
  - `identifier_regex`: Regex pattern for validating "d" tag identifiers
  - `follows_whitelist_admins`: Per-rule admin pubkeys whose follows are whitelisted
  - `write_allow` / `write_deny`: Pubkey whitelist/blacklist for writing (write-only)
  - `read_allow` / `read_deny`: Pubkey whitelist/blacklist for reading (read-only)
  - `privileged`: Party-involved access control (read-only)
- See `docs/POLICY_USAGE_GUIDE.md` for configuration examples
- See `pkg/policy/README.md` for quick reference

**`pkg/sync/`** - Distributed synchronization
- `cluster_manager.go` - Active replication between relay peers
- `relay_group_manager.go` - Relay group configuration (NIP-XX)
- `manager.go` - Distributed directory consensus

**`pkg/spider/`** - Event syncing from other relays
- `spider.go` - Spider manager for "follows" mode
- Fetches events from admin relays for followed pubkeys
- **Directory Spider** (`directory.go`):
  - Discovers relays by crawling kind 10002 (relay list) events
  - Expands outward from seed pubkeys (whitelisted users) via hop distance
  - Fetches metadata events (kinds 0, 3, 10000, 10002) from discovered relays
  - Self-detection prevents querying own relay
  - Configurable interval and max hops via `ORLY_DIRECTORY_SPIDER_*` env vars

**`pkg/utils/`** - Shared utilities
- `atomic/` - Extended atomic operations
- `interrupt/` - Signal handling and graceful shutdown
- `apputil/` - Application-level utilities

**Web UI (`app/web/`):**
- Svelte-based admin interface
- Embedded in binary via `go:embed`
- Features: event browser, sprocket management, policy management, user admin, settings
- **Policy Management Tab:** JSON editor with validation, save publishes kind 12345 event

**Command-line Tools (`cmd/`):**
- `relay-tester/` - Nostr protocol compliance testing
- `benchmark/` - Multi-relay performance comparison
- `stresstest/` - Load testing tool
- `aggregator/` - Event aggregation utility
- `convert/` - Data format conversion
- `policytest/` - Policy validation testing

### Important Patterns

**Pure Go with Purego:**
- All builds use `CGO_ENABLED=0`
- The p8k crypto library (from `git.mleku.dev/mleku/nostr`) uses `github.com/ebitengine/purego` to dynamically load `libsecp256k1.so` at runtime
- This avoids CGO complexity while maintaining C library performance
- `libsecp256k1.so` is included in the repository root
- Library must be in `LD_LIBRARY_PATH` or same directory as binary for runtime loading

**Database Backend Selection:**
- Supports multiple backends via `ORLY_DB_TYPE` environment variable
- **Badger** (default): Embedded key-value store with custom indexing, ideal for single-instance deployments
- **Neo4j**: Graph database with social graph and Web of Trust (WoT) extensions
  - Processes kinds 0 (profile), 3 (contacts), 1984 (reports), 10000 (mute list) for social graph
  - NostrUser nodes with trust metrics (influence, PageRank)
  - FOLLOWS, MUTES, REPORTS relationships for WoT analysis
  - See `pkg/neo4j/WOT_SPEC.md` for full schema specification
- Backend selected via factory pattern in `pkg/database/factory.go`
- All backends implement the same `Database` interface defined in `pkg/database/interface.go`

**Database Query Pattern:**
- Filters are analyzed in `get-indexes-from-filter.go` to determine optimal query strategy
- Filters are normalized before cache lookup, ensuring identical queries with different field ordering hit the cache
- Different query builders (`query-for-kinds.go`, `query-for-authors.go`, etc.) handle specific filter patterns
- All queries return event serials (uint64) for efficient joining
- Query results cached with zstd level 9 compression (configurable size and TTL)
- Final events fetched via `fetch-events-by-serials.go`

**WebSocket Message Flow:**
1. `handle-websocket.go` accepts connection and spawns goroutine
2. Incoming frames parsed by `pkg/protocol/ws/`
3. Routed to handlers: `handle-event.go`, `handle-req.go`, `handle-count.go`, etc.
4. Events stored via `database.SaveEvent()`
5. Active subscriptions notified via `publishers.Publish()`

**Configuration System - CRITICAL RULES:**
- Uses `go-simpler.org/env` for struct tags
- **ALL environment variables MUST be defined in `app/config/config.go`**
  - **NEVER** use `os.Getenv()` directly in packages - always pass config via structs
  - **NEVER** parse environment variables outside of `app/config/`
  - This ensures all config options appear in `./orly help` output
  - Database backends receive config via `database.DatabaseConfig` struct
  - Use `GetDatabaseConfigValues()` helper to extract DB config from app config
- All config fields use `ORLY_` prefix with struct tags defining defaults and usage
- Supports XDG directories via `github.com/adrg/xdg`
- Default data directory: `~/.local/share/ORLY`
- Database-specific config (Neo4j, Badger) is passed via `DatabaseConfig` struct in `pkg/database/factory.go`

**Constants - CRITICAL RULES:**
- **ALWAYS** define named constants for values used more than a few times
- **ALWAYS** define named constants if multiple packages depend on the same value
- Constants shared across packages should be in a dedicated package (e.g., `pkg/constants/`)
- Magic numbers and strings are forbidden - use named constants with clear documentation
- Example:
  ```go
  // BAD - magic number
  if timeout > 30 {

  // GOOD - named constant
  const DefaultTimeoutSeconds = 30
  if timeout > DefaultTimeoutSeconds {
  ```

**Event Publishing:**
- `pkg/protocol/publish/` manages publisher registry
- Each WebSocket connection registers its subscriptions
- `publishers.Publish(event)` broadcasts to matching subscribers
- Efficient filter matching without re-querying database

**Embedded Assets:**
- Web UI built to `app/web/dist/`
- Embedded via `//go:embed` directive in `app/web.go`
- Served at root path `/` with API at `/api/*`

**Domain Boundaries & Encapsulation:**
- Library packages (e.g., `pkg/policy`) should NOT export internal state variables
- Use unexported fields (lowercase) for internal state to enforce encapsulation at compile time
- Provide public API methods (e.g., `IsEnabled()`, `CheckPolicy()`) instead of exposing internals
- When JSON unmarshalling is needed for unexported fields, use a shadow struct with custom `UnmarshalJSON`
- External packages (e.g., `app/`) should ONLY use public API methods, never access internal fields
- **DO NOT** change unexported fields to exported when fixing bugs - this breaks the domain boundary

**Binary-Optimized Tag Storage (CRITICAL - Read Carefully):**

The nostr library (`git.mleku.dev/mleku/nostr/encoders/tag`) uses binary optimization for `e` and `p` tags. This is a common source of bugs when working with pubkeys and event IDs.

**How Binary Encoding Works:**
- When events are unmarshaled from JSON, 64-character hex values in e/p tags are converted to 33-byte binary format (32 bytes hash + null terminator)
- The `tag.T` field contains `[][]byte` where each element may be binary or hex depending on tag type
- `event.E.ID`, `event.E.Pubkey`, and `event.E.Sig` are always stored as fixed-size byte arrays (`[32]byte` or `[64]byte`)

**NEVER Do This:**
```go
// WRONG: tag.T[1] may be 33-byte binary, not 64-char hex!
pubkey := string(tag.T[1])  // Results in garbage for binary-encoded tags

// WRONG: Will fail for binary-encoded e/p tags
pt, err := hex.Dec(string(pTag.Value()))
```

**ALWAYS Do This:**
```go
// CORRECT: Use ValueHex() which handles both binary and hex formats
pubkey := string(pTag.ValueHex())  // Always returns lowercase hex

// CORRECT: For decoding to bytes
pt, err := hex.Dec(string(pTag.ValueHex()))

// CORRECT: For event.E fields (always binary, use hex.Enc)
pubkeyHex := hex.Enc(ev.Pubkey[:])  // Always produces lowercase hex
eventIDHex := hex.Enc(ev.ID[:])
sigHex := hex.Enc(ev.Sig[:])
```

**Tag Methods Reference:**
- `tag.ValueHex()` - Returns hex string regardless of storage format (handles both binary and hex)
- `tag.ValueBinary()` - Returns 32-byte binary if stored in binary format, nil otherwise
- `tag.Value()` - Returns raw bytes **DANGEROUS for e/p tags** - may be binary

**Hex Case Sensitivity:**
- The hex encoder (`git.mleku.dev/mleku/nostr/encoders/hex`) **always produces lowercase hex**
- External sources may send uppercase hex (e.g., `"ABCD..."` instead of `"abcd..."`)
- When storing pubkeys/event IDs (especially in Neo4j), **always normalize to lowercase**
- Mixed case causes duplicate entities in graph databases

**Neo4j-Specific Helpers (pkg/neo4j/hex_utils.go):**
```go
// ExtractPTagValue handles binary encoding and normalizes to lowercase
pubkey := ExtractPTagValue(pTag)

// ExtractETagValue handles binary encoding and normalizes to lowercase
eventID := ExtractETagValue(eTag)

// NormalizePubkeyHex handles both binary and uppercase hex
normalized := NormalizePubkeyHex(rawValue)

// IsValidHexPubkey validates 64-char hex
if IsValidHexPubkey(pubkey) { ... }
```

**Files Most Affected by These Rules:**
- `pkg/neo4j/save-event.go` - Event storage with e/p tag handling
- `pkg/neo4j/social-event-processor.go` - Social graph with p-tag extraction
- `pkg/neo4j/query-events.go` - Filter queries with tag matching
- `pkg/database/save-event.go` - Badger event storage
- `pkg/database/filter_utils.go` - Tag normalization utilities
- `pkg/find/parser.go` - FIND protocol parser with p-tag extraction

This optimization saves memory and enables faster comparisons in the database layer.

**Interface Design - CRITICAL RULES:**

**Rule 1: ALL interfaces MUST be defined in `pkg/interfaces/<name>/`**
- Interfaces provide isolation between packages and enable dependency inversion
- Keeping interfaces in a dedicated package prevents circular dependencies
- Each interface package should be minimal (just the interface, no implementations)

**Rule 2: NEVER use type assertions with interface literals**
- **NEVER** write `.(interface{ Method() Type })` - this is non-idiomatic and unmaintainable
- Interface literals cannot be documented, tested for satisfaction, or reused
- Example of WRONG approach:
  ```go
  // BAD - interface literal in type assertion
  if checker, ok := obj.(interface{ Check() bool }); ok {
      checker.Check()
  }
  ```
- Example of CORRECT approach:
  ```go
  // GOOD - use defined interface from pkg/interfaces/
  import "next.orly.dev/pkg/interfaces/checker"

  if c, ok := obj.(checker.Checker); ok {
      c.Check()
  }
  ```

**Rule 3: Resolving Circular Dependencies**
- If a circular dependency occurs when adding an interface, move the interface to `pkg/interfaces/`
- The implementing type stays in its original package
- The consuming code imports only the interface package
- This pattern:
  ```
  pkg/interfaces/foo/   <- interface definition (no dependencies)
       ↑           ↑
  pkg/bar/         pkg/baz/
  (implements)     (consumes via interface)
  ```

**Existing interfaces in `pkg/interfaces/`:**
- `acl/` - ACL and PolicyChecker interfaces
- `neterr/` - TimeoutError interface for network errors
- `resultiter/` - Neo4jResultIterator for database results
- `store/` - Storage-related interfaces
- `publisher/` - Event publishing interfaces
- `typer/` - Type identification interface

## Development Workflow

### Making Changes to Web UI
1. Edit files in `app/web/src/`
2. For hot reload: `cd app/web && bun run dev` (with `ORLY_WEB_DISABLE=true` and `ORLY_WEB_DEV_PROXY_URL=http://localhost:5173`)
3. For production build: `./scripts/update-embedded-web.sh`

### Adding New Nostr Protocol Handlers
1. Create `app/handle-<message-type>.go`
2. Add case in `app/handle-message.go` message router
3. Implement handler following existing patterns
4. Add tests in `app/<handler>_test.go`

### Adding Database Indexes
1. Define index in `pkg/database/indexes/`
2. Add migration in `pkg/database/migrations.go`
3. Update `save-event.go` to populate index
4. Add query builder in `pkg/database/query-for-<index>.go`
5. Update `get-indexes-from-filter.go` to use new index

### Environment Variables for Development
```bash
# Verbose logging
export ORLY_LOG_LEVEL=trace
export ORLY_DB_LOG_LEVEL=debug

# Enable profiling
export ORLY_PPROF=cpu
export ORLY_PPROF_HTTP=true  # Serves on :6060

# Health check endpoint
export ORLY_HEALTH_PORT=8080
```

### Profiling
```bash
# CPU profiling
export ORLY_PPROF=cpu
./orly
# Profile written on shutdown

# HTTP pprof server
export ORLY_PPROF_HTTP=true
./orly
# Visit http://localhost:6060/debug/pprof/

# Memory profiling
export ORLY_PPROF=memory
export ORLY_PPROF_PATH=/tmp/profiles
```

## Deployment

### Automated Deployment
```bash
# Deploy with systemd service
./scripts/deploy.sh
```

This script:
1. Installs Go 1.25.3 if needed
2. Builds relay with embedded web UI
3. Installs to `~/.local/bin/orly`
4. Creates systemd service
5. Sets capabilities for port 443 binding

### systemd Service Management
```bash
# Start/stop/restart
sudo systemctl start orly
sudo systemctl stop orly
sudo systemctl restart orly

# Enable on boot
sudo systemctl enable orly

# View logs
sudo journalctl -u orly -f
```

### Manual Deployment
```bash
# Build for production
./scripts/update-embedded-web.sh

# Or build all platforms
./scripts/build-all-platforms.sh
```

## Key Dependencies

- `github.com/dgraph-io/badger/v4` - Embedded database
- `github.com/gorilla/websocket` - WebSocket server
- `github.com/minio/sha256-simd` - SIMD SHA256
- `github.com/templexxx/xhex` - SIMD hex encoding
- `github.com/ebitengine/purego` - CGO-free C library loading
- `go-simpler.org/env` - Environment variable configuration
- `lol.mleku.dev` - Custom logging library

## Testing Guidelines

- Test files use `_test.go` suffix
- Use `github.com/stretchr/testify` for assertions
- Database tests require temporary database setup (see `pkg/database/testmain_test.go`)
- WebSocket tests should use `relay-tester` package
- Always clean up resources in tests (database, connections, goroutines)

## Performance Considerations

- **Query Cache**: 512MB query result cache (configurable via `ORLY_QUERY_CACHE_SIZE_MB`) with zstd level 9 compression reduces database load for repeated queries
- **Filter Normalization**: Filters are normalized before cache lookup, so identical queries with different field ordering produce cache hits
- **Database Caching**: Tune `ORLY_DB_BLOCK_CACHE_MB` and `ORLY_DB_INDEX_CACHE_MB` for workload (Badger backend only)
- **Query Optimization**: Add indexes for common filter patterns; multiple specialized query builders optimize different filter combinations
- **Batch Operations**: ID lookups and event fetching use batch operations via `GetSerialsByIds` and `FetchEventsBySerials`
- **Memory Pooling**: Use buffer pools in encoders (see `pkg/encoders/event/`)
- **SIMD Operations**: Leverage minio/sha256-simd and templexxx/xhex for cryptographic operations
- **Goroutine Management**: Each WebSocket connection runs in its own goroutine

## Recent Optimizations

ORLY has received several significant performance improvements in recent updates:

### Query Cache System (Latest)
- 512MB query result cache with zstd level 9 compression
- Filter normalization ensures cache hits regardless of filter field ordering
- Configurable size (`ORLY_QUERY_CACHE_SIZE_MB`) and TTL (`ORLY_QUERY_CACHE_MAX_AGE`)
- Dramatically reduces database load for repeated queries (common in Nostr clients)
- Cache key includes normalized filter representation for optimal hit rate

### Badger Cache Tuning
- Optimized block cache (default 512MB, tune via `ORLY_DB_BLOCK_CACHE_MB`)
- Optimized index cache (default 256MB, tune via `ORLY_DB_INDEX_CACHE_MB`)
- Resulted in 10-15% improvement in most benchmark scenarios
- See git history for cache tuning evolution

### Query Execution Improvements
- Multiple specialized query builders for different filter patterns:
  - `query-for-kinds.go` - Kind-based queries
  - `query-for-authors.go` - Author-based queries
  - `query-for-tags.go` - Tag-based queries
  - Combination builders for `kinds+authors`, `kinds+tags`, `kinds+authors+tags`
- Batch operations for ID lookups via `GetSerialsByIds`
- Serial-based event fetching for efficiency
- Filter analysis in `get-indexes-from-filter.go` selects optimal strategy

## Git Commit Message Format

When asked to "make a commit comment", generate a commit message following this standard format:

**Structure:**
- **First line**: 72 characters maximum, imperative mood summary
- **Second line**: Empty line
- **Body**: Bullet points describing each change in detail
- **Optional**: "Files modified:" section listing affected files

**Example:**
```
Fix directory spider tag loss: size limits and validation

- Increase WebSocket message size limit from 500KB to 10MB to prevent
  truncation of large kind 3 follow list events (8000+ follows)
- Add validation in SaveEvent to reject kind 3 events without p tags
  before storage, preventing malformed events from buggy relays
- Implement CleanupKind3WithoutPTags() to remove existing malformed
  kind 3 events at startup
- Add enhanced logging showing tag count and event ID when rejecting
  invalid kind 3 events for better observability

Files modified:
- app/handle-websocket.go: Increase DefaultMaxMessageSize to 10MB
- pkg/database/save-event.go: Add kind 3 validation with logging
- pkg/database/cleanup-kind3.go: New cleanup function
- app/main.go: Invoke cleanup at startup
```

## Release Process

1. Update version in `pkg/version/version` file (e.g., v1.2.3)
2. Create and push tag:
   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```
3. GitHub Actions workflow builds binaries for multiple platforms
4. Release created automatically with binaries and checksums

## Recent Features (v0.31.x)

### Directory Spider
The directory spider (`pkg/spider/directory.go`) automatically discovers and syncs metadata from other relays:
- Crawls kind 10002 (relay list) events to discover relays
- Expands outward from seed pubkeys (whitelisted users) via configurable hop distance
- Fetches essential metadata events (kinds 0, 3, 10000, 10002)
- Self-detection prevents querying own relay
- Enable with `ORLY_DIRECTORY_SPIDER=true`

### Neo4j Social Graph Backend
The Neo4j backend (`pkg/neo4j/`) includes Web of Trust (WoT) extensions:
- **Social Event Processor**: Handles kinds 0, 3, 1984, 10000 for social graph management
- **NostrUser nodes**: Store profile data and trust metrics (influence, PageRank)
- **Relationships**: FOLLOWS, MUTES, REPORTS for social graph analysis
- **WoT Schema**: See `pkg/neo4j/WOT_SPEC.md` for full specification
- **Schema Modifications**: See `pkg/neo4j/MODIFYING_SCHEMA.md` for how to update

### Policy System Enhancements
- **Write-Only Validation**: Size, age, tag validations apply ONLY to writes
- **Read-Only Filtering**: `read_allow`, `read_deny`, `privileged` apply ONLY to reads
- **Scripts**: Policy scripts execute ONLY for write operations
- **Reference Documentation**: `docs/POLICY_CONFIGURATION_REFERENCE.md` provides authoritative read vs write applicability
- See also: `pkg/policy/README.md` for quick reference

### Authentication Modes
- `ORLY_AUTH_REQUIRED=true`: Require authentication for ALL requests
- `ORLY_AUTH_TO_WRITE=true`: Require authentication only for writes (allow anonymous reads)

### NIP-43 Relay Access Metadata
Invite-based access control system:
- `ORLY_NIP43_ENABLED=true`: Enable invite system
- Publishes kind 8000/8001 events for member changes
- Publishes kind 13534 membership list events
- Configurable invite expiry via `ORLY_NIP43_INVITE_EXPIRY`

## Documentation Index

| Document | Purpose |
|----------|---------|
| `docs/POLICY_CONFIGURATION_REFERENCE.md` | Authoritative policy config reference with read/write applicability |
| `docs/POLICY_USAGE_GUIDE.md` | Comprehensive policy system user guide |
| `pkg/policy/README.md` | Policy system quick reference |
| `pkg/neo4j/README.md` | Neo4j backend overview |
| `pkg/neo4j/WOT_SPEC.md` | Web of Trust schema specification |
| `pkg/neo4j/MODIFYING_SCHEMA.md` | How to modify Neo4j schema |
| `pkg/neo4j/TESTING.md` | Neo4j testing guide |
| `readme.adoc` | Project README with feature overview |
