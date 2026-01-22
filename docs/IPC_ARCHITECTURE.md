# ORLY IPC Architecture

ORLY implements a modular Inter-Process Communication (IPC) architecture where core functionality runs as independent gRPC services. This design provides resource isolation, fault tolerance, and flexible deployment options.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                orly-launcher                                     │
│                     (Process Supervisor & Lifecycle Manager)                     │
│                                                                                  │
│  Responsibilities:                                                               │
│  - Start services in dependency order                                            │
│  - Monitor process health and restart on failure                                 │
│  - Graceful shutdown in reverse order                                            │
│  - Pass environment configuration to child processes                             │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
        ┌───────────────────────────────┼───────────────────────────────┐
        │                               │                               │
        ▼                               ▼                               ▼
┌───────────────────┐       ┌───────────────────┐       ┌───────────────────────────┐
│    orly-db        │       │    orly-acl       │       │     Sync Services         │
│  (gRPC :50051)    │       │  (gRPC :50052)    │       │  (gRPC :50061-50064)      │
│                   │       │                   │       │                           │
│ Core Database:    │       │ Access Control:   │       │ orly-sync-distributed     │
│ ├─ Event storage  │       │ ├─ Permission     │       │ ├─ Peer-to-peer sync      │
│ ├─ Query engine   │       │ │   checks        │       │ └─ Serial-based polling   │
│ ├─ Index mgmt     │       │ ├─ Follow lists   │       │                           │
│ ├─ Serial counter │       │ ├─ Admin/owner    │       │ orly-sync-cluster         │
│ └─ Export/import  │       │ │   management    │       │ ├─ HA replication         │
│                   │       │ └─ Policy hooks   │       │ └─ Membership events      │
│ Backends:         │       │                   │       │                           │
│ ├─ Badger (SSD)   │       │ Modes:            │       │ orly-sync-relaygroup      │
│ ├─ Neo4j (graph)  │       │ ├─ none (open)    │       │ ├─ Kind 39105 config      │
│ └─ WasmDB (wasm)  │       │ ├─ follows        │       │ └─ Dynamic peer lists     │
│                   │       │ ├─ managed        │       │                           │
│                   │       │ └─ curating       │       │ orly-sync-negentropy      │
│                   │       │                   │       │ ├─ NIP-77 WebSocket       │
│                   │       │                   │       │ └─ Set reconciliation     │
└───────────────────┘       └───────────────────┘       └───────────────────────────┘
        │                           │                               │
        │                           │                               │
        └───────────────────────────┼───────────────────────────────┘
                                    │
                                    ▼
                        ┌───────────────────────┐
                        │        orly           │
                        │    (Main Relay)       │
                        │                       │
                        │ Client-Facing:        │
                        │ ├─ WebSocket server   │
                        │ ├─ NIP handling       │
                        │ ├─ REQ/EVENT/AUTH     │
                        │ ├─ NEG-* (NIP-77)     │
                        │ └─ HTTP API           │
                        │                       │
                        │ Internal:             │
                        │ ├─ gRPC DB client     │
                        │ ├─ gRPC ACL client    │
                        │ └─ gRPC sync clients  │
                        └───────────────────────┘
                                    │
                                    ▼
                            ┌───────────────┐
                            │    Caddy      │
                            │ (TLS Proxy)   │
                            └───────────────┘
                                    │
                                    ▼
                              Internet
```

## Service Ports

| Service | Default Port | Description |
|---------|--------------|-------------|
| orly-db | 50051 | Database server (Badger/Neo4j/WasmDB) |
| orly-acl | 50052 | Access control server |
| orly-sync-distributed | 50061 | Peer-to-peer sync |
| orly-sync-cluster | 50062 | Cluster replication |
| orly-sync-relaygroup | 50063 | Relay group config |
| orly-sync-negentropy | 50064 | NIP-77 negentropy |
| orly | 3334 | Main relay (WebSocket/HTTP) |

## Startup Sequence

The launcher starts services in strict dependency order:

```
1. orly-db          ──────► Wait for gRPC ready
                              │
2. orly-acl         ──────► Wait for gRPC ready (depends on DB)
                              │
3. Sync Services    ──────► Start in parallel, wait for all ready
   ├─ orly-sync-distributed     (depends on DB)
   ├─ orly-sync-cluster         (depends on DB)
   ├─ orly-sync-relaygroup      (depends on DB)
   └─ orly-sync-negentropy      (depends on DB)
                              │
4. orly             ──────► Connects to all services via gRPC
```

**Shutdown** happens in reverse order: relay → sync services → ACL → database.

## Benefits of IPC Architecture

### 1. Resource Isolation
- Database memory usage is isolated from relay
- ACL processing doesn't compete with query handling
- Sync services have dedicated resources

### 2. Fault Tolerance
- Database crash doesn't kill WebSocket connections immediately
- ACL service restart preserves event storage
- Individual sync service failures don't affect others

### 3. Independent Scaling
- Scale database for query-heavy workloads
- Scale relay for connection-heavy workloads
- Enable only needed sync services

### 4. Flexible Deployment
- Run services on different machines
- Mix local and remote services
- Deploy services incrementally

---

# Core Services

## 1. Database Service (orly-db)

The database service handles all event storage, indexing, and querying.

### Variants
- `orly-db-badger` - Badger LSM-tree database (SSD optimized)
- `orly-db-neo4j` - Neo4j graph database (social graph queries)

### gRPC API

```protobuf
service DatabaseService {
  // Readiness
  rpc Ready(Empty) returns (ReadyResponse);

  // Event operations
  rpc SaveEvent(SaveEventRequest) returns (SaveEventResponse);
  rpc QueryEvents(QueryEventsRequest) returns (QueryEventsResponse);
  rpc CountEvents(CountEventsRequest) returns (CountEventsResponse);
  rpc DeleteEvent(DeleteEventRequest) returns (DeleteEventResponse);
  rpc QueryAllVersions(QueryEventsRequest) returns (QueryEventsResponse);

  // Special queries
  rpc GetSerials(GetSerialsRequest) returns (GetSerialsResponse);
  rpc GetLastSerial(Empty) returns (GetLastSerialResponse);
  rpc CheckForDeleted(CheckDeletedRequest) returns (CheckDeletedResponse);

  // Admin operations
  rpc Export(ExportRequest, stream ExportResponse);
  rpc Import(stream ImportRequest) returns (ImportResponse);
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_DB_LISTEN` | 127.0.0.1:50051 | gRPC listen address |
| `ORLY_DATA_DIR` | ~/.local/share/ORLY | Database storage path |
| `ORLY_DB_LOG_LEVEL` | info | Logging level |
| `ORLY_DB_BLOCK_CACHE_MB` | 1024 | Badger block cache |
| `ORLY_DB_INDEX_CACHE_MB` | 512 | Badger index cache |
| `ORLY_DB_ZSTD_LEVEL` | 3 | Compression level (0-9) |

---

## 2. ACL Service (orly-acl)

The ACL service handles access control decisions for all relay operations.

### Variants
- `orly-acl-none` - Open relay (no restrictions)
- `orly-acl-follows` - Follow-list based access
- `orly-acl-managed` - NIP-86 managed access
- `orly-acl-curating` - Curator-based access

### gRPC API

```protobuf
service ACLService {
  // Readiness
  rpc Ready(Empty) returns (ReadyResponse);

  // Access checks
  rpc CheckAccess(CheckAccessRequest) returns (CheckAccessResponse);
  rpc GetAccessLevel(GetAccessLevelRequest) returns (GetAccessLevelResponse);
  rpc CheckPolicy(CheckPolicyRequest) returns (CheckPolicyResponse);

  // Admin operations
  rpc Configure(ConfigureRequest) returns (ConfigureResponse);
  rpc GetType(Empty) returns (TypeResponse);
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_ACL_LISTEN` | 127.0.0.1:50052 | gRPC listen address |
| `ORLY_ACL_MODE` | follows | ACL mode |
| `ORLY_ACL_DB_TYPE` | grpc | Database backend |
| `ORLY_ACL_GRPC_DB_SERVER` | 127.0.0.1:50051 | Database server |
| `ORLY_ACL_LOG_LEVEL` | info | Logging level |

---

## 3. Main Relay (orly)

The main relay handles all client-facing operations.

### Responsibilities
- WebSocket connections (NIP-01)
- REQ/EVENT/AUTH/CLOSE handling
- NEG-* messages (NIP-77, when enabled)
- HTTP API endpoints
- Web UI serving

### Connection Modes

**Local mode** (default):
```bash
ORLY_DB_TYPE=badger
ORLY_ACL_TYPE=local
```

**gRPC mode** (IPC):
```bash
ORLY_DB_TYPE=grpc
ORLY_GRPC_SERVER=127.0.0.1:50051
ORLY_ACL_TYPE=grpc
ORLY_GRPC_ACL_SERVER=127.0.0.1:50052
ORLY_NEGENTROPY_ENABLED=true
ORLY_GRPC_SYNC_NEGENTROPY=127.0.0.1:50064
```

---

# Sync Services

See [IPC_SYNC_SERVICES.md](./IPC_SYNC_SERVICES.md) for detailed sync service documentation.

## Quick Reference

| Service | Port | Purpose |
|---------|------|---------|
| orly-sync-distributed | 50061 | Serial-based peer sync |
| orly-sync-cluster | 50062 | HA cluster replication |
| orly-sync-relaygroup | 50063 | Kind 39105 relay groups |
| orly-sync-negentropy | 50064 | NIP-77 set reconciliation |

---

# Deployment Examples

## Minimal IPC (Database + Relay)

```bash
# Start database
ORLY_DB_LISTEN=127.0.0.1:50051 ./orly-db-badger &

# Start relay with gRPC database
ORLY_DB_TYPE=grpc \
ORLY_GRPC_SERVER=127.0.0.1:50051 \
./orly
```

## Full IPC with ACL

```bash
# Use launcher for automatic management
ORLY_LAUNCHER_DB_BACKEND=badger \
ORLY_LAUNCHER_ACL_ENABLED=true \
ORLY_ACL_MODE=follows \
./orly-launcher
```

## Full IPC with Negentropy

```bash
# Enable NIP-77 support
ORLY_LAUNCHER_DB_BACKEND=badger \
ORLY_LAUNCHER_ACL_ENABLED=true \
ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true \
ORLY_NEGENTROPY_ENABLED=true \
./orly-launcher
```

## systemd Service

See the example at `/etc/systemd/system/orly.service` on relay.orly.dev for a production configuration.

---

# Building Binaries

```bash
# Build all IPC binaries
make all-acl   # Builds orly-db-*, orly-acl-*, orly-launcher

# Build sync services
go build -o orly-sync-distributed ./cmd/orly-sync-distributed
go build -o orly-sync-cluster ./cmd/orly-sync-cluster
go build -o orly-sync-relaygroup ./cmd/orly-sync-relaygroup
go build -o orly-sync-negentropy ./cmd/orly-sync-negentropy

# Cross-compile for ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o orly-sync-negentropy ./cmd/orly-sync-negentropy
```

---

# Monitoring

## Health Checks

Each gRPC service implements a `Ready` RPC that returns when the service is accepting requests.

## Logs

All services log to stdout/stderr. Use journalctl for systemd deployments:

```bash
# Follow all ORLY logs
journalctl -u orly -f

# Filter by process
journalctl -u orly | grep "orly-db"
journalctl -u orly | grep "orly-sync-negentropy"
```

## pprof

Enable HTTP pprof for profiling:

```bash
ORLY_PPROF_HTTP=true ./orly-launcher
# Access at http://localhost:6060/debug/pprof/
```

---

# Troubleshooting

## Service won't start

1. Check if port is in use: `ss -tlnp | grep 5005`
2. Verify binary exists and is executable
3. Check logs: `journalctl -u orly -n 100`

## Database connection timeout

1. Ensure database started first
2. Check `ORLY_LAUNCHER_DB_READY_TIMEOUT` (default 30s)
3. Verify network connectivity

## Negentropy not working

1. Verify `ORLY_NEGENTROPY_ENABLED=true` in relay config
2. Check negentropy service is running: `ps aux | grep negentropy`
3. Verify gRPC connection: `ORLY_GRPC_SYNC_NEGENTROPY=127.0.0.1:50064`
