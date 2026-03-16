# CLAUDE.md

ORLY is a high-performance Nostr relay in Go with Badger/Neo4j/WasmDB backends, Svelte web UI, purego-based secp256k1 crypto, and a Nostr-to-Email bridge (Marmot).

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
CGO_ENABLED=0 go build -o orly ./cmd/orly    # Unified binary (launcher, db, acl, bridge, relay)
CGO_ENABLED=0 go build -o orly .              # Relay-only (NO subcommand support)
./scripts/update-embedded-web.sh              # Build with embedded web UI

# Test
./scripts/test.sh
go test -v -run TestName ./pkg/package

# Run (unified binary)
./orly                    # Start relay (default subcommand)
./orly launcher           # Start with process supervisor (split IPC mode)
./orly db --driver=badger # Start database server
./orly acl --driver=paid  # Start ACL server
./orly bridge             # Start Marmot email bridge
./orly sync --driver=negentropy  # Start sync service
./orly test-subscribe     # Test paid subscription flow end-to-end
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

# Proto generation
cd proto && buf generate
```

## Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_PORT` | 3334 | Server port |
| `ORLY_LOG_LEVEL` | info | trace/debug/info/warn/error |
| `ORLY_DB_TYPE` | badger | badger/neo4j/wasmdb/grpc |
| `ORLY_POLICY_ENABLED` | false | Enable policy system |
| `ORLY_ACL_MODE` | none | none/follows/managed/curating/paid |
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
    main.go          → Subcommand router (db, acl, sync, bridge, launcher, relay, test-subscribe)
    db/              → Database server subcommand
    acl/             → ACL server subcommand
    sync/            → Sync service subcommand (distributed, cluster, relaygroup, negentropy)
    bridge/          → Email bridge subcommand
    launcher/        → Process supervisor (self-exec pattern)
    relay/           → Main relay subcommand
    testsubscribe/   → Test paid subscription flow (NWC loopback)
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
  nostr/             → Vendored nostr library (events, encoders, crypto, protocol, signer)
  p256k1/            → Vendored secp256k1 crypto (Schnorr, ECDSA, purego, asm)
  lol/               → Vendored logging library (log levels, chk.E pattern)
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
    nwc/             → NWC (Nostr Wallet Connect) client
  encoders/          → Optimized JSON encoding with buffer pools
  policy/            → Event filtering/validation
  acl/               → Access control (none/follows/managed/curating/paid)
  bridge/            → Marmot Email Bridge (DM↔SMTP, NIP-17 gift-wrap)
proto/
  orlydb/v1/         → Database gRPC service (100+ RPCs)
  orlyacl/v1/        → ACL gRPC service (50+ RPCs)
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

## ACL Drivers

| Driver | Description | Registration |
|--------|-------------|-------------|
| `none` | Open relay, no restrictions | Default/built-in |
| `follows` | Whitelist from admin follow lists | `RegisterDriver("follows", ...)` |
| `managed` | NIP-86 fine-grained access control | `RegisterDriver("managed", ...)` |
| `curating` | Rate-limited trust tier system | `RegisterDriver("curating", ...)` |
| `paid` | Lightning payment-gated access | `RegisterDriver("paid", ...)` |

Set via `ORLY_ACL_MODE`. The `paid` driver stores subscriptions and aliases through the Database interface (works in both embedded and gRPC split-IPC modes). It exposes methods via the ACL gRPC service: `SubscribePubkey`, `UnsubscribePubkey`, `IsSubscribed`, `ClaimAlias`, etc.

## Marmot Email Bridge

The bridge (`pkg/bridge/`) provides bidirectional Nostr DM ↔ SMTP email. Users DM the bridge's Nostr pubkey to subscribe, send emails, and receive inbound mail as DMs.

### DM Protocols

The bridge supports both legacy and modern DM protocols:
- **Kind 4 (NIP-04)**: Legacy encrypted DMs, single-layer NIP-44 encryption
- **Kind 1059 (NIP-17)**: Modern gift-wrapped DMs — three-layer encryption (1059 → 13 → 14) with ephemeral sender key for privacy

The bridge subscribes to both kinds and tracks which protocol each sender uses. Replies are sent in the same format the sender used, preventing duplicate messages in clients that support both.

### Bridge Files

| File | Purpose |
|------|---------|
| `bridge.go` | Main Bridge struct — identity, relay connection, DM handling, format tracking |
| `config.go` | Config struct (domain, NSEC, relay URL, SMTP, DKIM, NWC, ACL gRPC) |
| `giftwrap.go` | NIP-17 gift-wrap: `wrapGiftWrap()`, `unwrapGiftWrap()`, timestamp randomization |
| `identity.go` | 3-tier identity resolution: config NSEC → database → file fallback |
| `relay.go` | WebSocket relay connection with auto-reconnect and NIP-42 auth |
| `router.go` | DM router — dispatches to SubscriptionHandler or OutboundProcessor |
| `parser.go` | DM classification: subscribe/status commands, outbound email detection |
| `subscription_handler.go` | Subscribe flow: invoice creation, payment poll (up to 10 min), ACL activation |
| `subscription.go` | FileSubscriptionStore: persists subscription state as JSON |
| `payment.go` | NWC payment processor wrapper for subscription invoices |
| `inbound.go` | Email → DM: converts inbound emails to Nostr DMs via Blossom upload |
| `outbound.go` | DM → Email: converts outbound DMs to SMTP messages |
| `serve.go` | HTTP handlers for /compose and /decrypt web pages |
| `attachments.go` | ChaCha20-Poly1305 attachment encryption |
| `ratelimit.go` | Sliding window rate limiter for outbound emails |
| `zip.go` | Zip bundling for HTML + attachments (max 25MB) |

### Bridge Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_BRIDGE_ENABLED` | false | Enable Marmot email bridge |
| `ORLY_BRIDGE_DOMAIN` | | Email domain (e.g., relay.example.com) |
| `ORLY_BRIDGE_NSEC` | | Bridge identity nsec (default: use relay identity) |
| `ORLY_BRIDGE_RELAY_URL` | | WebSocket relay URL for standalone mode |
| `ORLY_BRIDGE_SMTP_PORT` | 2525 | SMTP server listen port |
| `ORLY_BRIDGE_SMTP_HOST` | 0.0.0.0 | SMTP server listen address |
| `ORLY_BRIDGE_DATA_DIR` | | Bridge data directory (default: $ORLY_DATA_DIR/bridge) |
| `ORLY_BRIDGE_DKIM_KEY` | | Path to DKIM private key PEM file |
| `ORLY_BRIDGE_DKIM_SELECTOR` | marmot | DKIM selector for DNS TXT record |
| `ORLY_BRIDGE_NWC_URI` | | NWC connection string (falls back to `ORLY_NWC_URI`) |
| `ORLY_BRIDGE_MONTHLY_PRICE_SATS` | 2100 | Monthly subscription price (sats) |
| `ORLY_BRIDGE_ALIAS_PRICE_SATS` | 4200 | Monthly alias email price (sats) |
| `ORLY_BRIDGE_COMPOSE_URL` | | Public URL of compose form |
| `ORLY_BRIDGE_SMTP_RELAY_HOST` | | SMTP smarthost for outbound (e.g., smtp.migadu.com) |
| `ORLY_BRIDGE_SMTP_RELAY_PORT` | 587 | SMTP smarthost port (STARTTLS) |
| `ORLY_BRIDGE_SMTP_RELAY_USERNAME` | | SMTP smarthost AUTH username |
| `ORLY_BRIDGE_SMTP_RELAY_PASSWORD` | | SMTP smarthost AUTH password |
| `ORLY_BRIDGE_ACL_GRPC_SERVER` | | gRPC address of ACL server for paid subscription management |

### Bridge Message Flow

```
Inbound DM (kind 4 or 1059)
  → unwrap (NIP-04 or NIP-17 gift-wrap)
  → record sender format (kind4 / giftwrap)
  → ClassifyDM → subscribe / status / outbound email / help
  → Router dispatches to handler

Reply DM
  → check sender's recorded format
  → send in same format (kind 4 or NIP-17 gift-wrap)
```

### Marmot SDK Note

A separate MLS-based Marmot protocol SDK exists in the nostr library at `pkg/nostr/protocol/marmot/` (kinds 443, 445, 1059). It provides forward secrecy via MLS key ratcheting. The email bridge does NOT use this SDK — it uses NIP-17 gift-wrapping instead, which is compatible with standard Nostr clients like smesh.

## NWC Client

The NWC (Nostr Wallet Connect) client lives in `pkg/protocol/nwc/`:

| File | Purpose |
|------|---------|
| `uri.go` | `ConnectionParams` struct, `ParseConnectionURI()` |
| `client.go` | `Client` struct with `Request()` and `SubscribeNotifications()` |

```go
client, err := nwc.NewClient(nwcURI)
err = client.Request(ctx, "make_invoice", params, &result)
err = client.SubscribeNotifications(ctx, handler)
```

Used by the bridge's `PaymentProcessor` for Lightning invoice creation and payment polling.

## Database Backends

| Backend | Use Case | Build |
|---------|----------|-------|
| **Badger** (default) | Single-instance, SSD, high performance | Standard |
| **Neo4j** | Social graph, WoT queries | `ORLY_DB_TYPE=neo4j` |
| **WasmDB** | Browser/WebAssembly | `GOOS=js GOARCH=wasm` |
| **gRPC** | Remote database (IPC split mode) | `ORLY_DB_TYPE=grpc` |

All implement `pkg/database.Database` interface.

### Database Interface Method Groups

The `Database` interface (`pkg/database/interface.go`) contains 100+ methods organized as:

- **Core Lifecycle**: Path, Init, Sync, Close, Wipe, Ready, SetLogLevel
- **Event Storage**: SaveEvent, GetSerialsFromFilter, WouldReplaceEvent
- **Event Queries**: QueryEvents, QueryAllVersions, QueryEventsWithOptions, CountEvents, QueryForSerials, QueryForIds
- **Event Fetch**: FetchEventBySerial, FetchEventsBySerials, GetSerialById, GetSerialsByIds, GetFullIdPubkeyBySerial
- **Event Deletion**: DeleteEvent, DeleteEventBySerial, DeleteExpired, ProcessDelete, CheckForDeleted
- **Import/Export**: Import, Export, ImportEventsFromReader, ImportEventsFromStrings
- **Relay Identity**: GetRelayIdentitySecret, SetRelayIdentitySecret, GetOrCreateRelayIdentitySecret
- **Markers**: SetMarker, GetMarker, HasMarker, DeleteMarker
- **Subscriptions**: GetSubscription, IsSubscriptionActive, ExtendSubscription, RecordPayment, GetPaymentHistory, IsFirstTimeUser
- **Paid ACL**: SavePaidSubscription, GetPaidSubscription, DeletePaidSubscription, ListPaidSubscriptions, ClaimAlias, GetAliasByPubkey, GetPubkeyByAlias, IsAliasTaken
- **NIP-43**: AddNIP43Member, RemoveNIP43Member, IsNIP43Member, StoreInviteCode, ValidateInviteCode
- **Query Cache**: GetCachedJSON, CacheMarshaledJSON, GetCachedEvents, CacheEvents, InvalidateQueryCache
- **Access Tracking**: RecordEventAccess, GetEventAccessInfo, GetLeastAccessedEvents
- **Blob Storage**: SaveBlob, GetBlob, HasBlob, DeleteBlob, ListBlobs, ListAllBlobs, GetThumbnail, SaveThumbnail
- **NRC**: CreateNRCConnection, GetNRCConnection, SaveNRCConnection, DeleteNRCConnection

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

## gRPC Proto Services

Proto definitions in `proto/` with buf generation. Two services:

### DatabaseService (`proto/orlydb/v1/service.proto`)

100+ RPCs covering: lifecycle, event storage/queries/fetch/deletion, import/export, relay identity, markers, subscriptions, paid ACL, NIP-43, query cache, access tracking, blob storage, thumbnails, cypher queries, migrations.

### ACLService (`proto/orlyacl/v1/acl.proto`)

50+ RPCs covering: core access checks, follows management, managed (ban/allow pubkeys/events/IPs/kinds), curating (trust tiers, rate limiting, spam), paid (subscribe/unsubscribe, aliases).

## Logging (pkg/lol)

```go
import "next.orly.dev/pkg/lol/log"
import "next.orly.dev/pkg/lol/chk"

log.T.F("trace: %s", msg)  // T=Trace, D=Debug, I=Info, W=Warn, E=Error, F=Fatal
if chk.E(err) { return }   // Log + check error
```

## Development Workflows

**Add Nostr handler**: Create `app/handle-<type>.go` → add case in `handle-message.go`

**Add database index**: Define in `pkg/database/indexes/` → add migration → update `save-event.go` → add query builder

**Add ACL driver**: Create `pkg/acl/<name>.go` + `pkg/acl/register_<name>.go` → use `RegisterDriver("<name>", desc, factory)`

**Add bridge command**: Add case in `pkg/bridge/parser.go` `ClassifyDM()` → handle in `pkg/bridge/router.go` `RouteDM()`

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

## CRITICAL: Three Separate Web UIs

There are three independent web UIs in this repo. They are **separate codebases** with different frameworks, build tools, and deployment targets. Changes to one do NOT affect the others.

| UI | Path | Framework | Build | Service Worker | Deploy Target |
|----|------|-----------|-------|----------------|---------------|
| Relay Dashboard | `app/web/` | Svelte/Rollup | `bun run build` → `dist/` | Manual `CACHE_VERSION` in `public/sw.js` | Embedded in Go binary (`app/web.go`) |
| Smesh Client | `app/smesh/` | React/Vite | `bun run build` → `dist/` | Workbox auto-hashed (content-addressed) | Embedded (`app/smesh.go`) + rsync to `/home/mleku/smesh/dist/` on VPS |
| Launcher Admin | `cmd/orly-launcher/web/` | Svelte/Rollup | `bun run build` → `dist/` | None | Embedded (`cmd/orly-launcher/web.go`) |

**Common mistakes to avoid:**

1. **Rebuilding is not changing.** Rebuilding a UI without source changes just produces the same app with new chunk hashes. If the user reports visual issues (wrong colors, missing features), the **source** needs changing, not just a rebuild.
2. **Theme defaults are per-UI.** The relay dashboard defines its theme in `app/web/src/App.svelte` (CSS variables). The smesh client defines its theme in `app/smesh/src/constants.ts` (PRIMARY_COLORS), `app/smesh/src/providers/ThemeProvider.tsx` (default theme setting), and `app/smesh/src/index.css` (CSS variable defaults). These must be changed independently.
3. **Smesh has two deploy targets.** The embedded copy (served on port 8088 via Go binary) AND the static copy at smesh.mleku.dev (rsync to VPS) both need updating.
4. **Service worker cache invalidation differs.** Relay dashboard requires manually bumping `CACHE_VERSION` in `app/web/public/sw.js`. Smesh uses Workbox which auto-hashes chunk filenames — but the SW itself must still be re-fetched by the browser (Caddy cache headers matter).
5. **localStorage persists user preferences.** Theme/color changes to defaults only affect new users or cleared storage. Existing users keep their saved preferences.

**Current theme defaults (smesh):** pure-black background, amber primary (`38 92% 50%` ≈ `#F59E0B`), matching the relay dashboard.

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
- **Mode**: Split IPC via `orly launcher` (self-exec spawns db, acl, bridge, relay subprocesses)
- **Bridge pubkey**: `cf1ae33ad5f229dabd7d733ce37b0165126aebf581e4094df9373f77e00cb696`

### Build & Deploy

**CRITICAL**: Build from `./cmd/orly` for the unified binary. Building from root (`go build .`) creates a relay-only binary WITHOUT the launcher subcommand, causing deployment failures.

**CRITICAL**: When deploying web UI changes, bump `CACHE_VERSION` in `app/web/public/sw.js` before building (e.g., `orly-v2` → `orly-v3`). The service worker caches `bundle.js` by filename (no content hashing), so without a version bump, users will be served the stale cached bundle indefinitely.

```bash
# 1. Bump CACHE_VERSION in app/web/public/sw.js (required for frontend changes!)
# 2. Build unified binary for amd64 (includes web UI)
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

# 5. Verify (should show launcher + db + acl + bridge + relay subprocesses)
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@69.164.249.71 \
  'sleep 5 && systemctl status orly'
```

### Full-Stack Deploy Checklist

When deploying changes across all web UIs (relay dashboard, smesh client, launcher admin):

```bash
# 1. Bump CACHE_VERSION in app/web/public/sw.js (smesh uses Workbox auto-hashing, no manual bump)
# 2. Build all web UIs
cd app/web && bun install && bun run build
cd app/smesh && bun install && bun run build
cd cmd/orly-launcher/web && bun install && bun run build
# 3. Commit dist/ changes
# 4. Build amd64 binary (embeds all three UIs)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orly ./cmd/orly
# 5. Deploy binary to relay.orly.dev (stop → rsync → chown → start)
# 6. Deploy smesh static files to smesh.mleku.dev
rsync -avz --delete -e "ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes" \
  app/smesh/dist/ root@69.164.249.71:/home/mleku/smesh/dist/
```

Note: `scripts/update-embedded-web.sh` only builds relay dashboard + smesh (not launcher admin) and runs `go install` (local arch, not amd64 cross-compile). For full deployment, use the manual steps above.

### Launcher Mode (Split IPC)

The systemd service runs `orly launcher` which uses self-exec to spawn:
- `orly db --driver=badger` (gRPC database server on :50051)
- `orly acl --driver=<mode>` (gRPC ACL server on :50052, if `ORLY_LAUNCHER_ACL_ENABLED=true`)
- `orly bridge` (Marmot email bridge, if `ORLY_BRIDGE_ENABLED=true`)
- `orly sync --driver=<type>` (sync services, if enabled: distributed, cluster, relaygroup, negentropy)
- `orly` (main relay connecting to db/acl via gRPC)

This provides process isolation and allows independent restarts. The unified binary eliminates ~100MB of duplicate Go runtime compared to separate binaries.

### Launcher Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_LAUNCHER_DB_DRIVER` | badger | Database driver (badger/neo4j) |
| `ORLY_LAUNCHER_DB_LISTEN` | 127.0.0.1:50051 | Database gRPC listen address |
| `ORLY_LAUNCHER_ACL_ENABLED` | false | Enable ACL subprocess |
| `ORLY_LAUNCHER_ACL_LISTEN` | 127.0.0.1:50052 | ACL gRPC listen address |
| `ORLY_LAUNCHER_DB_READY_TIMEOUT` | 30s | Wait for DB to become ready |
| `ORLY_LAUNCHER_ACL_READY_TIMEOUT` | 120s | Wait for ACL to become ready |
| `ORLY_LAUNCHER_STOP_TIMEOUT` | 30s | Graceful shutdown timeout |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` | false | Enable negentropy sync |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN` | 127.0.0.1:50064 | Negentropy gRPC address |
| `ORLY_LAUNCHER_SERVICES_ENABLED` | true | Enable auxiliary services |
| `ORLY_LAUNCHER_ADMIN_ENABLED` | true | Enable admin interface |
| `ORLY_LAUNCHER_ADMIN_PORT` | 8080 | Admin HTTP port |
| `ORLY_LAUNCHER_OWNERS` | | Comma-separated owner pubkeys |

**Future improvements**: Build on VPS directly (git pull + go build) to avoid slow binary transfers.

## Git Remotes

- **origin**: `ssh://git@git.nostrdev.com:29418/mleku/next.orly.dev.git`

## git.mleku.dev Self-Hosted Git

Bare git repos on mleku.dev with HTTP read + SSH write. No forge (no Gitea/Forgejo/cgit).

- **Server**: `mleku.dev` (same VPS as relay.orly.dev, `root@mleku.dev`)
- **Repos**: `/home/git/` (bare repos, e.g. `reshetka.git`)
- **HTTP clone** (public, read-only): `https://git.mleku.dev/reshetka.git`
- **SSH clone** (authenticated, read+write): `git@mleku.dev:reshetka.git`
- **Go import**: `git.mleku.dev/reshetka` (meta tag served by Caddy)
- **LFS**: Supported via `git-lfs-transfer` (SSH-native, charmbracelet)

### Architecture

- **Shell**: `git` user has `/usr/local/bin/git-shell-lfs` (allows only `git-receive-pack`, `git-upload-pack`, `git-lfs-transfer`)
- **HTTP backend**: `fcgiwrap` → `git-http-backend` (served by Caddy, `GIT_HTTP_EXPORT_ALL=1`)
- **Post-receive hook**: runs `git update-server-info` for dumb HTTP client compatibility
- **Update hook**: branch protection — only `GIT_USER=owner` can push to `main`; all users can push tags and feature branches

### User Management

Manual editing of `/home/git/.ssh/authorized_keys`. Each line sets `GIT_USER` via SSH environment variable.

Roles:
- `owner` — can push to `main`, tags, and all branches
- `contributor` — can push tags and feature branches only (not `main`)

### Adding a Contributor

```bash
# SSH into the server
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@mleku.dev

# Append their public key to authorized_keys
echo 'environment="GIT_USER=contributor" ssh-ed25519 AAAAC3...theirkey user@description' \
  >> /home/git/.ssh/authorized_keys
```

The contributor can then: `git clone git@mleku.dev:reshetka.git`, push feature branches, and submit PRs by telling the owner to merge.

### Adding a New Repo

```bash
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes root@mleku.dev
cd /home/git
git init --bare newrepo.git
chown -R git:git newrepo.git
touch newrepo.git/git-daemon-export-ok
# Copy hooks from existing repo
cp reshetka.git/hooks/post-receive newrepo.git/hooks/post-receive
cp reshetka.git/hooks/update newrepo.git/hooks/update
# Then update Caddy config to add HTTP route for the new repo
```

## Dependencies

### Internal (monorepo packages)

- `next.orly.dev/pkg/nostr/` — Nostr library (crypto, events, encoders, protocol, signer)
- `next.orly.dev/pkg/p256k1/` — secp256k1 elliptic curve crypto (Schnorr, ECDSA, purego)
- `next.orly.dev/pkg/lol/` — Logging (log levels, chk.E error pattern)

These were originally separate modules (git.mleku.dev/mleku/nostr, p256k1.mleku.dev, lol.mleku.dev) that have been fully merged into the monorepo as standard Go packages.

### External

- `github.com/dgraph-io/badger/v4` - Badger DB (LSM, SSD-optimized)
- `github.com/neo4j/neo4j-go-driver/v5` - Neo4j
- `github.com/gorilla/websocket` - WebSocket
- `github.com/ebitengine/purego` - CGO-free C loading
- `github.com/minio/sha256-simd` - SIMD SHA256
- `go-simpler.org/env` - Config
