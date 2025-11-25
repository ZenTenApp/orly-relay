# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ORLY is a high-performance Nostr relay written in Go, designed for personal relays, small communities, and business deployments. It emphasizes low latency, custom cryptography optimizations, and embedded database performance.

**Key Technologies:**
- **Language**: Go 1.25.3+
- **Database**: Badger v4 (embedded key-value store) or DGraph (distributed graph database)
- **Cryptography**: Custom p8k library using purego for secp256k1 operations (no CGO)
- **Web UI**: Svelte frontend embedded in the binary
- **WebSocket**: gorilla/websocket for Nostr protocol
- **Performance**: SIMD-accelerated SHA256 and hex encoding, query result caching with zstd compression

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

# Note: libsecp256k1.so is automatically downloaded by test.sh if needed
# It can also be manually downloaded from the nostr repository:
# wget https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so
# export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)"
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

# Database backend selection (badger or dgraph)
export ORLY_DB_TYPE=badger
export ORLY_DGRAPH_URL=localhost:9080  # Only for dgraph backend

# Query cache configuration (improves REQ response times)
export ORLY_QUERY_CACHE_SIZE_MB=512      # Default: 512MB
export ORLY_QUERY_CACHE_MAX_AGE=5m       # Cache expiry time

# Database cache tuning (for Badger backend)
export ORLY_DB_BLOCK_CACHE_MB=512        # Block cache size
export ORLY_DB_INDEX_CACHE_MB=256        # Index cache size
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
- `factory.go` - Database backend selection (Badger or DGraph)
- `database.go` - Badger implementation with cache tuning and query cache
- `save-event.go` - Event storage with index updates
- `query-events.go` - Main query execution engine with filter normalization
- `query-for-*.go` - Specialized query builders for different filter patterns
- `indexes/` - Index key construction for efficient lookups
- `export.go` / `import.go` - Event export/import in JSONL format
- `subscriptions.go` - Active subscription tracking
- `identity.go` - Relay identity key management
- `migrations.go` - Database schema migration runner

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
  - `libsecp256k1.so` - Downloaded from nostr repository at runtime/build time
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
- See `docs/POLICY_USAGE_GUIDE.md` for configuration examples

**`pkg/sync/`** - Distributed synchronization
- `cluster_manager.go` - Active replication between relay peers
- `relay_group_manager.go` - Relay group configuration (NIP-XX)
- `manager.go` - Distributed directory consensus

**`pkg/spider/`** - Event syncing from other relays
- `spider.go` - Spider manager for "follows" mode
- Fetches events from admin relays for followed pubkeys

**`pkg/utils/`** - Shared utilities
- `atomic/` - Extended atomic operations
- `interrupt/` - Signal handling and graceful shutdown
- `apputil/` - Application-level utilities

**Web UI (`app/web/`):**
- Svelte-based admin interface
- Embedded in binary via `go:embed`
- Features: event browser, sprocket management, user admin, settings

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
- `libsecp256k1.so` is automatically downloaded by build/test scripts from the nostr repository
- Manual download: `wget https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so`
- Library must be in `LD_LIBRARY_PATH` or same directory as binary for runtime loading

**Database Backend Selection:**
- Supports multiple backends via `ORLY_DB_TYPE` environment variable
- **Badger** (default): Embedded key-value store with custom indexing, ideal for single-instance deployments
- **DGraph**: Distributed graph database for larger, multi-node deployments
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

**Configuration System:**
- Uses `go-simpler.org/env` for struct tags
- All config in `app/config/config.go` with `ORLY_` prefix
- Supports XDG directories via `github.com/adrg/xdg`
- Default data directory: `~/.local/share/ORLY`

**Event Publishing:**
- `pkg/protocol/publish/` manages publisher registry
- Each WebSocket connection registers its subscriptions
- `publishers.Publish(event)` broadcasts to matching subscribers
- Efficient filter matching without re-querying database

**Embedded Assets:**
- Web UI built to `app/web/dist/`
- Embedded via `//go:embed` directive in `app/web.go`
- Served at root path `/` with API at `/api/*`

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
1. Installs Go 1.25.0 if needed
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

## Release Process

1. Update version in `pkg/version/version` file (e.g., v1.2.3)
2. Create and push tag:
   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```
3. GitHub Actions workflow builds binaries for multiple platforms
4. Release created automatically with binaries and checksums
