# CLAUDE.md

ORLY is a high-performance Nostr relay in Go with Badger/Neo4j/WasmDB backends, Svelte web UI, and purego-based secp256k1 crypto.

## CRITICAL: Server-Side Changes Prohibited

**DO NOT modify the relay (ORLY) code (Go files in `app/`, `pkg/`, `cmd/`) unless the user EXPLICITLY states the changes should go into "the relay" or "orly".**

The relay (ORLY) implements standard Nostr protocol and Blossom blob storage. It should:
- Store and serve events via WebSocket (NIP-01)
- Store and serve blobs via HTTP (Blossom BUD-01/02)
- **Nothing else**

All application logic belongs in the **client** (Svelte web UI in `app/web/`):
- Use `nostrClient.publish()` for publishing events via WebSocket
- Use Blossom HTTP endpoints only for blob upload/download/delete
- Query events via nostr protocol, not custom HTTP endpoints
- No adding HTTP endpoints to the relay
- No server-side event processing or filtering logic

If you think server changes are needed, **ASK FIRST** - the answer is probably "do it client-side".

## CRITICAL: NIP-42 Authentication in Client Code

**The client MUST handle NIP-42 AUTH challenges automatically.** When a relay sends an AUTH challenge (for reads or writes), the client should:

1. Detect the AUTH challenge from the relay
2. Sign the AUTH event using the user's signer
3. Send the signed AUTH event back to the relay
4. Retry the original operation

**Never ask the user whether to authenticate** - if a relay requires auth, the operation simply won't work without it. Users connecting to auth-required relays expect authentication to happen automatically.

This applies to:
- Publishing events (kind 30063 binding events, etc.)
- Querying events that require auth
- Any relay interaction that receives an "auth-required" response

The `nostrClient` in `app/web/src/nostr.js` must implement automatic AUTH handling for all relay connections.

## Quick Reference

```bash
# Build - IMPORTANT: Use cmd/orly for unified binary with subcommands
CGO_ENABLED=0 go build -o orly ./cmd/orly    # Unified binary (launcher, db, acl, relay)
CGO_ENABLED=0 go build -o orly .              # Relay-only (NO subcommand support)
./scripts/update-embedded-web.sh              # Build with embedded web UI

# Test
./scripts/test.sh
go test -v -run TestName ./pkg/package

# Run (unified binary)
./orly                    # Start relay (default subcommand)
./orly launcher           # Start with process supervisor (split IPC mode)
./orly db --driver=badger # Start database server
./orly acl --driver=follows # Start ACL server
./orly identity           # Show relay pubkey
./orly version            # Show version
./orly help               # Show all subcommands

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
| `ORLY_DB_TYPE` | badger | badger/neo4j/wasmdb/grpc |
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

**REMINDER: Cloudron Deployment Script** — When making changes to the Neo4j driver (`pkg/neo4j/`), remind mleku to check the cloudron-orly deployment at `https://git.nostrdev.com/stuff/cloudron-orly`. The Dockerfile downloads specific ORLY binaries by version tag, and changes to Neo4j schema, config, or driver behavior may require corresponding updates to `neo4j.conf`, `start.sh`, `supervisord.conf`, or environment variables in that repo.

## Architecture

```
main.go              → Relay-only entry point (no subcommands)
cmd/
  orly/              → Unified binary entry point (WITH subcommands)
    main.go          → Subcommand router (db, acl, sync, launcher, relay)
    db/              → Database server subcommand
    acl/             → ACL server subcommand
    sync/            → Sync service subcommand
    launcher/        → Process supervisor (self-exec pattern)
    relay/           → Main relay subcommand
  nurl/              → NIP-98 HTTP debugging tool
  vainstr/           → Vanity npub generator
  relay-tester/      → Protocol compliance testing
  benchmark/         → Performance testing
app/
  server.go          → HTTP/WebSocket server
  handle-*.go        → Nostr message handlers (EVENT, REQ, AUTH, etc.)
  config/            → Environment configuration (go-simpler.org/env)
  web/               → Svelte frontend (embedded via go:embed)
pkg/
  interfaces/
    transport/       → Transport interface (pluggable network transports)
  transport/
    manager.go       → Transport lifecycle manager (ordered start/stop)
    tcp/             → Plain HTTP transport
    tls/             → TLS/ACME transport (autocert + manual certs)
    tor/             → Tor hidden service transport (wraps pkg/tor)
  database/          → Database interface + Badger implementation
  neo4j/             → Neo4j backend with WoT extensions
  wasmdb/            → WebAssembly IndexedDB backend
  tor/               → Tor subprocess management and hostname watching
  protocol/          → Nostr protocol (ws/, auth/, publish/)
  encoders/          → Optimized JSON encoding with buffer pools
  policy/            → Event filtering/validation
  acl/               → Access control (none/follows/managed)
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
- Existing: `acl/`, `neterr/`, `resultiter/`, `store/`, `publisher/`, `transport/`, `typer/`

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
| **Neo4j** | Social graph, WoT queries | `ORLY_DB_TYPE=neo4j` |
| **WasmDB** | Browser/WebAssembly | `GOOS=js GOARCH=wasm` |
| **gRPC** | Remote database (IPC split mode) | `ORLY_DB_TYPE=grpc` |

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

**Migration Between Backends**
```bash
# Migrate from Badger to Neo4j
./orly migrate --from badger --to neo4j

# Migrate with custom target path
./orly migrate --from badger --to neo4j --target-path /mnt/ssd/orly-neo4j
```

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

## Versioning

**The version file `pkg/version/version` must be updated when tagging releases.**

```bash
# When releasing a new version:
echo "v0.58.15" > pkg/version/version  # Update to match the git tag
git add pkg/version/version
git commit -m "Bump version to v0.58.15"
git tag v0.58.15
git push origin main --tags
```

The web UI reads this file to display the relay version. Forgetting to update it will show stale version info.

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
| Neo4j backend guide | `docs/NEO4J_BACKEND.md` |
| Neo4j bolt+s setup | `docs/NEO4J_BACKEND.md#bolts-external-access-remote-cypher-queries` |
| Neo4j Cypher proxy | `docs/NEO4J_BACKEND.md#cypher-query-proxy-http-endpoint` |
| HTTP guard (bot/rate) | `docs/HTTP_GUARD.md` |
| Neo4j WoT schema | `pkg/neo4j/WOT_SPEC.md` |
| Neo4j schema changes | `pkg/neo4j/MODIFYING_SCHEMA.md` |
| Event kinds database | `app/web/src/eventKinds.js` |
| Nsec encryption | `app/web/src/nsec-crypto.js` |

## Transport System

Network transports are pluggable via `pkg/interfaces/transport.Transport`:

```go
type Transport interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Addresses() []string
}
```

**Current transports**: `tcp`, `tls`, `tor`. TCP and TLS are mutually exclusive (TLS replaces TCP when `ORLY_TLS_DOMAINS` is set). Tor runs in parallel.

**Adding a new transport** (e.g., QUIC):
1. Create `pkg/transport/quic/quic.go` implementing the interface
2. Add `l.transportMgr.Add(quicTransport)` in `app/main.go`

The transport manager handles ordered startup (Start fails fast, rolls back) and reverse-order shutdown. Addresses from all transports are aggregated for NIP-11 relay info.

## Deploying to relay.orly.dev

- **Architecture**: **x86_64 (amd64)** — NOT arm64, always use `GOARCH=amd64`
- **OS**: Ubuntu 24.04 LTS
- **SSH**: `ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@69.164.249.71`
- **Service**: `systemctl {start|stop|restart|status} orly`
- **Logs**: `journalctl -u orly -f`
- **Binary**: `/home/mleku/.local/bin/orly` (unified binary with subcommands)
- **Mode**: Split IPC via `orly launcher` (self-exec spawns db, acl, relay subprocesses)

### Build & Deploy

**CRITICAL**: Build from `./cmd/orly` for the unified binary. Building from root (`go build .`) creates a relay-only binary WITHOUT the launcher subcommand, causing deployment failures.

```bash
# 1. Build unified binary for amd64 (includes web UI)
./scripts/update-embedded-web.sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orly ./cmd/orly

# 2. Stop service
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@69.164.249.71 'systemctl stop orly'

# 3. Deploy binary
rsync -avz --compress -e "ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes" \
  orly root@69.164.249.71:/home/mleku/.local/bin/

# 4. Fix ownership and start
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@69.164.249.71 \
  'chown mleku:mleku /home/mleku/.local/bin/orly && systemctl start orly'

# 5. Verify (should show launcher + db + acl + relay subprocesses)
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@69.164.249.71 \
  'sleep 5 && systemctl status orly'
```

### Launcher Mode (Split IPC)

The systemd service runs `orly launcher` which uses self-exec to spawn:
- `orly db --driver=badger` (gRPC database server on :50051)
- `orly acl --driver=follows` (gRPC ACL server on :50052)
- `orly` (main relay connecting to both via gRPC)

This provides process isolation and allows independent restarts. The unified binary eliminates ~100MB of duplicate Go runtime compared to separate binaries.

**Future improvements**: Build on VPS directly (git pull + go build) to avoid slow binary transfers.

## Git Remotes

- **origin**: `ssh://git@git.nostrdev.com:29418/mleku/next.orly.dev.git` (contract work)
- **gitea**: `ssh://mleku@git.mleku.dev:2222/mleku/next.orly.dev.git` (primary, mleku's own host)

Push to both remotes. Use `GIT_SSH_COMMAND="ssh -i ~/.ssh/id_ed25519"` for gitea.

## Dependencies

- `github.com/dgraph-io/badger/v4` - Badger DB (LSM, SSD-optimized)
- `github.com/neo4j/neo4j-go-driver/v5` - Neo4j
- `github.com/gorilla/websocket` - WebSocket
- `github.com/ebitengine/purego` - CGO-free C loading
- `github.com/minio/sha256-simd` - SIMD SHA256
- `go-simpler.org/env` - Config
- `lol.mleku.dev` - Logging
