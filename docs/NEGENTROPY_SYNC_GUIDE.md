# Negentropy Sync Configuration Guide

ORLY implements [NIP-77](https://github.com/nostr-protocol/nips/blob/master/77.md) negentropy-based set reconciliation for efficient relay synchronization. This guide covers configuration for various deployment scenarios.

## Overview

Negentropy sync provides:

- **Efficient synchronization**: Only exchanges events that differ between relays
- **Bandwidth optimization**: Uses set reconciliation instead of transferring full event lists
- **NIP-77 client support**: Clients can sync directly with your relay using the negentropy protocol
- **Relay-to-relay sync**: Automatically sync with configured peer relays

## Quick Start

### Standalone Relay (Simplest)

For a standalone relay that accepts NIP-77 client connections:

```bash
export ORLY_NEGENTROPY_ENABLED=true
./orly
```

This enables the relay to respond to client `NEG-OPEN`, `NEG-MSG`, and `NEG-CLOSE` messages.

### Relay with Peer Sync

To sync events with other relays:

```bash
export ORLY_NEGENTROPY_ENABLED=true
export ORLY_SYNC_NEGENTROPY_PEERS=wss://relay1.example.com,wss://relay2.example.com
./orly
```

The relay will periodically sync with each peer relay.

## Deployment Scenarios

### 1. Split IPC Mode (Recommended for Production)

In split mode, negentropy runs as a separate gRPC service managed by `orly-launcher`:

```bash
# Build binaries
CGO_ENABLED=0 go build -o orly .
CGO_ENABLED=0 go build -o orly-db-badger ./cmd/orly-db-badger
CGO_ENABLED=0 go build -o orly-sync-negentropy ./cmd/orly-sync-negentropy
CGO_ENABLED=0 go build -o orly-launcher ./cmd/orly-launcher
```

**systemd service configuration:**

```ini
[Service]
ExecStart=/path/to/orly-launcher

# Database service
Environment=ORLY_LAUNCHER_DB_BACKEND=badger
Environment=ORLY_LAUNCHER_DB_BINARY=/path/to/orly-db-badger
Environment=ORLY_LAUNCHER_DB_LISTEN=127.0.0.1:50051

# Negentropy service
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY=/path/to/orly-sync-negentropy
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN=127.0.0.1:50064
Environment=ORLY_SYNC_NEGENTROPY_PEERS=wss://relay.orly.dev

# Relay configuration
Environment=ORLY_LAUNCHER_RELAY_BINARY=/path/to/orly
Environment=ORLY_DB_TYPE=grpc
Environment=ORLY_GRPC_SERVER=127.0.0.1:50051
Environment=ORLY_NEGENTROPY_ENABLED=true
Environment=ORLY_SYNC_TYPE=grpc
Environment=ORLY_GRPC_SYNC_NEGENTROPY=127.0.0.1:50064
```

### 2. Neo4j Backend with Negentropy

For Neo4j deployments, use `orly-db-neo4j`:

```bash
CGO_ENABLED=0 go build -o orly-db-neo4j ./cmd/orly-db-neo4j
```

**Configuration:**

```ini
# Database service (Neo4j)
Environment=ORLY_LAUNCHER_DB_BACKEND=neo4j
Environment=ORLY_LAUNCHER_DB_BINARY=/path/to/orly-db-neo4j
Environment=ORLY_LAUNCHER_DB_LISTEN=127.0.0.1:50051
Environment=ORLY_NEO4J_URI=bolt://localhost:7687
Environment=ORLY_NEO4J_USER=neo4j
Environment=ORLY_NEO4J_PASSWORD=your_password
Environment=ORLY_DATA_DIR=/var/lib/orly

# Negentropy and relay (same as Badger)
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
...
```

### 3. Docker/Cloudron Deployment

For containerized deployments with supervisord:

**supervisord.conf:**

```ini
[program:orly-db]
command=/app/code/orly-db-neo4j
environment=ORLY_DB_LISTEN="127.0.0.1:50051",ORLY_DATA_DIR="/app/data/ORLY",ORLY_DB_TYPE="neo4j",ORLY_NEO4J_URI="bolt://127.0.0.1:7687",ORLY_NEO4J_USER="neo4j",ORLY_NEO4J_PASSWORD="%(ENV_ORLY_NEO4J_PASSWORD)s"
priority=15

[program:orly]
command=/app/code/orly
environment=ORLY_DB_TYPE="grpc",ORLY_GRPC_SERVER="127.0.0.1:50051",ORLY_NEGENTROPY_ENABLED="true",ORLY_SYNC_TYPE="grpc",ORLY_GRPC_SYNC_NEGENTROPY="127.0.0.1:50064"
priority=20

[program:orly-negentropy]
command=/app/code/orly-sync-negentropy
environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN="127.0.0.1:50064",ORLY_DB_TYPE="grpc",ORLY_GRPC_SERVER="127.0.0.1:50051",ORLY_SYNC_NEGENTROPY_PEERS="wss://relay.orly.dev"
priority=25
```

## Environment Variables Reference

### Relay Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEGENTROPY_ENABLED` | false | Enable NIP-77 client support |
| `ORLY_SYNC_TYPE` | local | Sync backend: `local` or `grpc` |
| `ORLY_GRPC_SYNC_NEGENTROPY` | | gRPC address of negentropy service |

### Negentropy Service Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` | false | Enable negentropy service |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY` | | Path to orly-sync-negentropy binary |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN` | 127.0.0.1:50064 | gRPC listen address |
| `ORLY_SYNC_NEGENTROPY_PEERS` | | Comma-separated peer relay URLs |
| `ORLY_SYNC_NEGENTROPY_INTERVAL` | 60s | Background sync interval |
| `ORLY_SYNC_NEGENTROPY_FRAME_SIZE` | 4096 | Negentropy protocol frame size |
| `ORLY_SYNC_NEGENTROPY_ID_SIZE` | 16 | Event ID truncation size |
| `ORLY_SYNC_NEGENTROPY_SESSION_TIMEOUT` | 5m | Client session timeout |

### Filter Configuration

You can limit which events are synced using filter environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_NEGENTROPY_FILTER_KINDS` | | Comma-separated kind numbers to sync |
| `ORLY_SYNC_NEGENTROPY_FILTER_AUTHORS` | | Comma-separated author pubkeys to sync |
| `ORLY_SYNC_NEGENTROPY_FILTER_SINCE` | | Unix timestamp: only sync events after this time |
| `ORLY_SYNC_NEGENTROPY_FILTER_UNTIL` | | Unix timestamp: only sync events before this time |

**Example: Sync only social events**

```bash
export ORLY_SYNC_NEGENTROPY_FILTER_KINDS=0,1,3,6,7,1984,10000,30000
```

## strfry Interoperability

ORLY's negentropy implementation is compatible with strfry. You can sync events between ORLY and strfry relays.

### Syncing from strfry to ORLY

Configure ORLY to sync from a strfry relay:

```bash
export ORLY_SYNC_NEGENTROPY_PEERS=wss://strfry-relay.example.com
```

### Using strfry CLI to sync with ORLY

strfry can use ORLY as a sync target:

```bash
# Pull events from ORLY to strfry
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir down

# Push events from strfry to ORLY
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir up

# Bidirectional sync
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir both
```

### ORLY CLI Sync Command

ORLY includes a strfry-compatible CLI sync command:

```bash
# Build the sync binary
CGO_ENABLED=0 go build -o orly-sync ./cmd/orly

# Pull events from another relay
./orly-sync sync wss://relay.example.com --filter '{"kinds": [0, 1, 3]}' --dir down

# Push events to another relay
./orly-sync sync wss://relay.example.com --filter '{"kinds": [0, 1, 3]}' --dir up

# Specify local data directory
./orly-sync sync wss://relay.example.com --data-dir /path/to/db --dir down
```

## Testing

### Docker Compose Test Environment

A test environment for negentropy interop testing is available in `tests/negentropy/`:

```bash
cd tests/negentropy
docker compose build
docker compose up -d

# Run sync test
./test-sync.sh

# Clean up
docker compose down -v
```

### Verifying Negentropy Support

Check if a relay supports NIP-77:

```bash
curl -s https://your-relay.com | grep -o '"77"'
```

Or check the relay info:

```bash
curl -s -H "Accept: application/nostr+json" https://your-relay.com | jq '.supported_nips'
```

## Troubleshooting

### Common Issues

**1. "negentropy sync starting" but no events synced**

- Verify peer relay URL is correct and accessible
- Check if peer relay supports NIP-77
- Verify network connectivity between relays

**2. "failed to connect to gRPC server"**

- Ensure `orly-db` service is running and ready
- Check `ORLY_GRPC_SERVER` matches the database listen address
- Verify firewall allows connections on gRPC ports

**3. High memory usage during sync**

- Reduce `ORLY_SYNC_NEGENTROPY_FRAME_SIZE` to 2048 or lower
- Use filters to limit the event set being reconciled
- Increase sync interval to reduce concurrent operations

### Logging

Enable debug logging for negentropy:

```bash
export ORLY_LOG_LEVEL=debug
```

View sync-specific logs:

```bash
journalctl -u orly | grep negentropy
```

## Architecture

```
                     ┌──────────────────┐
                     │   Peer Relays    │
                     │  (wss://...)     │
                     └────────┬─────────┘
                              │
                              │ NIP-77 WebSocket
                              │
┌─────────────────────────────┼─────────────────────────────┐
│                             ▼                              │
│  ┌─────────────────────────────────────────────────────┐  │
│  │            orly-sync-negentropy                     │  │
│  │                 (gRPC :50064)                       │  │
│  │                                                     │  │
│  │  • Background sync with peers                       │  │
│  │  • Event reconciliation                             │  │
│  │  • Push/pull event exchange                         │  │
│  └──────────────────────┬──────────────────────────────┘  │
│                         │                                  │
│                         │ gRPC                             │
│                         │                                  │
│  ┌──────────────────────▼──────────────────────────────┐  │
│  │                  orly-db                             │  │
│  │              (gRPC :50051)                           │  │
│  │                                                      │  │
│  │  • Event storage                                     │  │
│  │  • Query execution                                   │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
│                         ORLY Server                        │
└────────────────────────────────────────────────────────────┘

NIP-77 Client Flow:
  Client ─NEG-OPEN──► Relay ──► Negentropy Service
  Client ◄──NEG-MSG── Relay ◄── (reconciliation)
  Client ─NEG-MSG───► Relay ──► (continue)
  Client ◄──EVENT──── Relay ◄── (missing events)
```

## See Also

- [IPC Sync Services Architecture](./IPC_SYNC_SERVICES.md) - Technical details of gRPC services
- [NIP-77 Specification](https://github.com/nostr-protocol/nips/blob/master/77.md) - Protocol specification
- [strfry sync documentation](https://github.com/hoytech/strfry/blob/master/docs/sync.md) - strfry sync reference
