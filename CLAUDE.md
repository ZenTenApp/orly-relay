# CLAUDE.md

ORLY is a high-performance Nostr relay in Go with Badger/Neo4j/WasmDB backends, Svelte web UI, and purego-based secp256k1 crypto.

## Quick Reference

```bash
# Build
CGO_ENABLED=0 go build -o orly
./scripts/update-embedded-web.sh  # With web UI

# Test
./scripts/test.sh
go test -v -run TestName ./pkg/package

# Run
./orly                    # Start relay
./orly identity           # Show relay pubkey
./orly version            # Show version

# Web UI dev (hot reload)
ORLY_WEB_DISABLE=true ORLY_WEB_DEV_PROXY_URL=http://localhost:5173 ./orly &
cd app/web && bun run dev

# NIP-98 HTTP debugging (build: go build -o nurl ./cmd/nurl)
NOSTR_SECRET_KEY=nsec1... ./nurl https://relay.example.com/api/logs
NOSTR_SECRET_KEY=nsec1... ./nurl https://relay.example.com/api/logs/clear
./nurl help  # Show usage

# Vanity npub generator (build: go build -o vainstr ./cmd/vainstr)
./vainstr mleku end      # Find npub ending with "mleku"
./vainstr orly begin     # Find npub starting with "orly" (after npub1)
./vainstr foo contain    # Find npub containing "foo"
./vainstr --threads 4 xyz end  # Use 4 threads
```

## Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_PORT` | 3334 | Server port |
| `ORLY_LOG_LEVEL` | info | trace/debug/info/warn/error |
| `ORLY_DB_TYPE` | badger | badger/bbolt/neo4j/wasmdb |
| `ORLY_POLICY_ENABLED` | false | Enable policy system |
| `ORLY_ACL_MODE` | none | none/follows/managed |
| `ORLY_TLS_DOMAINS` | | Let's Encrypt domains |
| `ORLY_AUTH_TO_WRITE` | false | Require auth for writes |

**Neo4j Memory Tuning** (only when `ORLY_DB_TYPE=neo4j`):

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEO4J_MAX_CONN_POOL` | 25 | Max connections (lower = less memory) |
| `ORLY_NEO4J_FETCH_SIZE` | 1000 | Records per batch (-1=all) |
| `ORLY_NEO4J_QUERY_RESULT_LIMIT` | 10000 | Max results per query (0=unlimited) |

See `./orly help` for all options. **All env vars MUST be defined in `app/config/config.go`**.

## Architecture

```
main.go              → Entry point
app/
  server.go          → HTTP/WebSocket server
  handle-*.go        → Nostr message handlers (EVENT, REQ, AUTH, etc.)
  config/            → Environment configuration (go-simpler.org/env)
  web/               → Svelte frontend (embedded via go:embed)
pkg/
  database/          → Database interface + Badger implementation
  bbolt/             → BBolt backend (HDD-optimized, B+tree)
  neo4j/             → Neo4j backend with WoT extensions
  wasmdb/            → WebAssembly IndexedDB backend
  protocol/          → Nostr protocol (ws/, auth/, publish/)
  encoders/          → Optimized JSON encoding with buffer pools
  policy/            → Event filtering/validation
  acl/               → Access control (none/follows/managed)
cmd/
  relay-tester/      → Protocol compliance testing
  benchmark/         → Performance testing
```

## Critical Rules

### 1. Binary-Optimized Tag Storage (MUST READ)

The nostr library stores `e` and `p` tag values as 33-byte binary (not 64-char hex).

```go
// WRONG - may be binary garbage
pubkey := string(tag.T[1])
pt, err := hex.Dec(string(pTag.Value()))

// CORRECT - always use ValueHex()
pubkey := string(pTag.ValueHex())           // Returns lowercase hex
pt, err := hex.Dec(string(pTag.ValueHex()))

// For event.E fields (always binary)
pubkeyHex := hex.Enc(ev.Pubkey[:])
```

**Always normalize to lowercase hex** when storing in Neo4j to prevent duplicates.

### 2. Configuration System

- **ALL env vars in `app/config/config.go`** - never use `os.Getenv()` in packages
- Pass config via structs (e.g., `database.DatabaseConfig`)
- Use `ORLY_` prefix for all variables

### 3. Interface Design

- **Define interfaces in `pkg/interfaces/<name>/`** - prevents circular deps
- **Never use interface literals** in type assertions: `.(interface{ Method() })` is forbidden
- Existing: `acl/`, `neterr/`, `resultiter/`, `store/`, `publisher/`, `typer/`

### 4. Constants

Define named constants for repeated values. No magic numbers/strings.

```go
// BAD
if timeout > 30 {

// GOOD
const DefaultTimeoutSeconds = 30
if timeout > DefaultTimeoutSeconds {
```

### 5. Domain Encapsulation

- Use unexported fields for internal state
- Provide public API methods (`IsEnabled()`, `CheckPolicy()`)
- Never change unexported→exported to fix bugs

### 6. Auth-Required Configuration (CAUTION)

**Be extremely careful when modifying auth-related settings in deployment configs.**

The `ORLY_AUTH_REQUIRED` and `ORLY_AUTH_TO_WRITE` settings control whether clients must authenticate via NIP-42 before interacting with the relay. Changing these on a production relay can:

- **Lock out all existing clients** if they don't support NIP-42 auth
- **Break automated systems** (bots, bridges, scrapers) that depend on anonymous access
- **Cause data sync issues** if upstream relays can't push events

Before enabling auth-required on any deployment:
1. Verify all expected clients support NIP-42
2. Ensure the relay identity key is properly configured
3. Test with a non-production instance first

## Database Backends

| Backend | Use Case | Build |
|---------|----------|-------|
| **Badger** (default) | Single-instance, SSD, high performance | Standard |
| **BBolt** | HDD-optimized, large archives, lower memory | `ORLY_DB_TYPE=bbolt` |
| **Neo4j** | Social graph, WoT queries | `ORLY_DB_TYPE=neo4j` |
| **WasmDB** | Browser/WebAssembly | `GOOS=js GOARCH=wasm` |

All implement `pkg/database.Database` interface.

### Scaling for Large Archives

For archives with millions of events, consider:

**Option 1: Tune Badger (SSD recommended)**
```bash
# Increase caches for larger working set (requires more RAM)
ORLY_DB_BLOCK_CACHE_MB=2048      # 2GB block cache
ORLY_DB_INDEX_CACHE_MB=1024      # 1GB index cache
ORLY_SERIAL_CACHE_PUBKEYS=500000 # 500k pubkeys
ORLY_SERIAL_CACHE_EVENT_IDS=2000000  # 2M event IDs

# Higher compression to reduce disk IO
ORLY_DB_ZSTD_LEVEL=9             # Best compression ratio

# Enable storage GC with aggressive eviction
ORLY_GC_ENABLED=true
ORLY_GC_BATCH_SIZE=5000
ORLY_MAX_STORAGE_BYTES=107374182400  # 100GB cap
```

**Option 2: Use BBolt for HDD/Low-Memory Deployments**
```bash
ORLY_DB_TYPE=bbolt

# Tune for your HDD
ORLY_BBOLT_BATCH_MAX_EVENTS=10000   # Larger batches for HDD
ORLY_BBOLT_BATCH_MAX_MB=256          # 256MB batch buffer
ORLY_BBOLT_FLUSH_TIMEOUT_SEC=60      # Longer flush interval
ORLY_BBOLT_BLOOM_SIZE_MB=32          # Larger bloom filter
ORLY_BBOLT_MMAP_SIZE_MB=16384        # 16GB mmap (scales with DB size)
```

**Migration Between Backends**
```bash
# Migrate from Badger to BBolt
./orly migrate --from badger --to bbolt

# Migrate with custom target path
./orly migrate --from badger --to bbolt --target-path /mnt/hdd/orly-archive
```

**BBolt vs Badger Trade-offs:**
- BBolt: Lower memory, HDD-friendly, simpler (B+tree), slower random reads
- Badger: Higher memory, SSD-optimized (LSM), faster concurrent access

## Logging (lol.mleku.dev)

```go
import "lol.mleku.dev/log"
import "lol.mleku.dev/chk"

log.T.F("trace: %s", msg)  // T=Trace, D=Debug, I=Info, W=Warn, E=Error, F=Fatal
if chk.E(err) { return }   // Log + check error
```

## Development Workflows

**Add Nostr handler**: Create `app/handle-<type>.go` → add case in `handle-message.go`

**Add database index**: Define in `pkg/database/indexes/` → add migration → update `save-event.go` → add query builder

**Profiling**: `ORLY_PPROF=cpu ./orly` or `ORLY_PPROF_HTTP=true` for :6060

## Commit Format

```
Fix description in imperative mood (72 chars max)

- Bullet point details
- More details

Files modified:
- path/to/file.go: What changed
```

## Web UI Libraries

### nsec-crypto.js

Secure nsec encryption library at `app/web/src/nsec-crypto.js`. Uses Argon2id + AES-256-GCM.

```js
import { encryptNsec, decryptNsec, isValidNsec, deriveKey } from "./nsec-crypto.js";

// Encrypt nsec with password (~3 sec derivation)
const encrypted = await encryptNsec(nsec, password);

// Decrypt (validates bech32 checksum)
const nsec = await decryptNsec(encrypted, password);

// Validate nsec format and checksum
if (isValidNsec(nsec)) { ... }
```

**Argon2id parameters**: 4 threads, 8 iterations, 256MB memory, 32-byte output.

**Storage format**: Base64(salt[32] + iv[12] + ciphertext). Validates bech32 on encrypt/decrypt.

## Documentation

| Topic | Location |
|-------|----------|
| Policy config | `docs/POLICY_CONFIGURATION_REFERENCE.md` |
| Policy guide | `docs/POLICY_USAGE_GUIDE.md` |
| Neo4j WoT schema | `pkg/neo4j/WOT_SPEC.md` |
| Neo4j schema changes | `pkg/neo4j/MODIFYING_SCHEMA.md` |
| Event kinds database | `app/web/src/eventKinds.js` |
| Nsec encryption | `app/web/src/nsec-crypto.js` |

## Dependencies

- `github.com/dgraph-io/badger/v4` - Badger DB (LSM, SSD-optimized)
- `go.etcd.io/bbolt` - BBolt DB (B+tree, HDD-optimized)
- `github.com/neo4j/neo4j-go-driver/v5` - Neo4j
- `github.com/gorilla/websocket` - WebSocket
- `github.com/ebitengine/purego` - CGO-free C loading
- `github.com/minio/sha256-simd` - SIMD SHA256
- `go-simpler.org/env` - Config
- `lol.mleku.dev` - Logging
