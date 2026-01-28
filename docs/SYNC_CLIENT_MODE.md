# ORLY Sync Client Mode

## Overview

ORLY now supports two modes for the `orly sync` command, similar to how strfry's sync command works but with the added flexibility of gRPC-based IPC:

1. **Direct Mode** (like strfry): Opens the database directly
2. **Client Mode** (unique to ORLY): Connects to a running `orly-sync-negentropy` service via gRPC

## Why Client Mode?

### The Problem with Direct Database Access

Both strfry and ORLY's direct mode require:
- Filesystem access to the database files
- The database to support multiple readers (or reader + writer)
- Running on the same machine as the database

### The gRPC IPC Solution

ORLY's IPC architecture allows the sync command to:
- Connect to a running service over the network
- No filesystem access required
- Sync from a remote machine
- Multiple clients can sync simultaneously using the same relay's database

## Architecture Comparison

### strfry sync (Direct Mode Only)
```
┌─────────────────┐         ┌─────────────────┐
│   strfry sync   │────────▶│   LMDB (file)   │
│   (CLI tool)    │  read   │                 │
└────────┬────────┘         └─────────────────┘
         │
         │ WebSocket
         ▼
┌─────────────────┐
│  Remote Relay   │
└─────────────────┘
```

### ORLY sync (Direct Mode - same as strfry)
```
┌─────────────────┐         ┌─────────────────┐
│   orly sync     │────────▶│  Badger/Neo4j   │
│   (CLI tool)    │  read   │   (database)    │
└────────┬────────┘         └─────────────────┘
         │
         │ WebSocket
         ▼
┌─────────────────┐
│  Remote Relay   │
└─────────────────┘
```

### ORLY sync (Client Mode - gRPC IPC)
```
┌─────────────────┐         ┌─────────────────────────────┐
│   orly sync     │────────▶│   orly-sync-negentropy      │
│   (CLI tool)    │  gRPC   │   (gRPC service)            │
│                 │         │                             │
│  No DB access   │         │  • Manages negentropy       │
│  needed!        │         │  • Opens database           │
└─────────────────┘         │  • Handles NIP-77 protocol  │
                            └──────────────┬──────────────┘
                                           │
                            ┌──────────────┴──────────────┐
                            │                             │
                   ┌────────▼────────┐         ┌─────────▼─────────┐
                   │  Badger/Neo4j   │         │   Remote Relay    │
                   │  (database)     │         │   (WebSocket)     │
                   └─────────────────┘         └───────────────────┘
```

## Usage

### Direct Mode (strfry-style)

```bash
# Sync using local database (must have filesystem access)
orly sync wss://relay.example.com --filter '{"kinds": [0, 3, 1984]}' --dir both

# Options:
#   --data-dir=PATH    Path to database (default: ~/.local/share/ORLY)
#   --filter=JSON      Nostr filter to limit sync scope
#   --dir=DIR          Direction: down, up, both (default: down)
```

### Client Mode (gRPC IPC)

```bash
# Sync using a running ORLY service (no filesystem access needed!)
orly sync wss://relay.example.com --server 127.0.0.1:50064 --filter '{"kinds": [1]}'

# Options:
#   --server=ADDR      gRPC address of orly-sync-negentropy service
#   --filter=JSON      Nostr filter to limit sync scope
#   --dir=DIR          Direction: down, up, both (default: down)
```

## Setting Up the Server

### Option 1: Using orly-launcher (Recommended)

The launcher manages all services automatically:

```bash
# Enable negentropy service
export ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
export ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN=127.0.0.1:50064

# Run launcher
./orly-launcher
```

### Option 2: Standalone Service

Run the sync service manually:

```bash
# Start database first
ORLY_DB_LISTEN=127.0.0.1:50051 ./orly-db-badger &

# Start sync service
ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN=127.0.0.1:50064 \
ORLY_DB_TYPE=grpc \
ORLY_GRPC_SERVER=127.0.0.1:50051 \
./orly-sync-negentropy
```

## Real-World Scenarios

### Scenario 1: Sync from Your Laptop to Production Relay

**Problem:** Your production ORLY relay runs on a server. You want to sync events from another relay using your laptop, but the database is on the server.

**Solution:**
```bash
# On server: ensure gRPC is accessible (bind to 0.0.0.0 or use SSH tunnel)
export ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN=0.0.0.0:50064

# On laptop: sync via gRPC
orly sync wss://remote-relay.com \
  --server your-server.com:50064 \
  --filter '{"kinds": [0, 1, 3]}' \
  --dir both
```

### Scenario 2: Multiple Admins Sharing a Relay

**Problem:** Multiple administrators need to sync content to the same relay, but you don't want to give them all filesystem access.

**Solution:**
- Run `orly-sync-negentropy` as a service
- Each admin uses `orly sync --server` to sync
- No direct database access required

### Scenario 3: Automated Sync Scripts

**Problem:** You want to run sync scripts from a CI/CD pipeline or cron job.

**Solution:**
```bash
# Cron job that syncs daily
0 2 * * * /usr/local/bin/orly sync wss://backup-relay.com \
  --server 127.0.0.1:50064 \
  --filter '{"kinds": [0, 1, 3, 1984]}' \
  --dir both \
  >> /var/log/orly-sync.log 2>&1
```

## Comparison with strfry

| Feature | strfry sync | ORLY sync (Direct) | ORLY sync (Client) |
|---------|-------------|-------------------|-------------------|
| Direct DB access | Yes | Yes | No |
| Network IPC | No | No | Yes (gRPC) |
| Remote sync | No | No | Yes |
| Multiple readers | LMDB | Badger/Neo4j | Yes (via service) |
| Requires relay running | No | No | Yes |

## Technical Details

### gRPC API

The client mode uses the `NegentropyService.SyncWithPeer` RPC:

```protobuf
rpc SyncWithPeer(SyncPeerRequest) returns (stream SyncProgress);

message SyncPeerRequest {
  string peer_url = 1;
  Filter filter = 2;
  int64 since = 3;
}

message SyncProgress {
  string peer_url = 1;
  int32 round = 2;
  int64 have_count = 3;
  int64 need_count = 4;
  int64 fetched_count = 5;
  int64 sent_count = 6;
  bool complete = 7;
  string error = 8;
}
```

### Security Considerations

When exposing the gRPC service over the network:

1. **Bind to localhost only** (default) for single-machine use
2. **Use SSH tunnels** for remote access:
   ```bash
   ssh -L 50064:localhost:50064 user@relay-server
   orly sync wss://... --server 127.0.0.1:50064
   ```
3. **Use TLS/mTLS** for production gRPC (future enhancement)
4. **Firewall rules** to restrict access to trusted IPs

## Troubleshooting

### "Failed to connect to sync service"

- Verify the service is running: `ps aux | grep orly-sync-negentropy`
- Check the address matches: `netstat -tlnp | grep 50064`
- Check firewall rules

### "Sync service not ready"

- The service might still be initializing
- Check that the database service is running
- Look at service logs for errors

### "Permission denied" (Direct Mode)

- Ensure you have read access to the database directory
- Check file ownership: `ls -la ~/.local/share/ORLY/`

## Future Enhancements

1. **TLS/mTLS support** for secure remote connections
2. **Authentication** for client connections
3. **Rate limiting** to prevent abuse
4. **Audit logging** for compliance
