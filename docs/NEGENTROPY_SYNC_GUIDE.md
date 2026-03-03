# Negentropy Sync Configuration Guide

ORLY implements [NIP-77](https://github.com/nostr-protocol/nips/blob/master/77.md) negentropy-based set reconciliation for efficient relay synchronization. This guide covers configuration for all deployment modes.

## How It Works

NIP-77 allows clients and relays to efficiently determine which events they have in common and which they're missing, without transferring full event lists. A client sends a `NEG-OPEN` message with a filter and a compact fingerprint of its local event set. The relay compares this against its own events and responds with `NEG-MSG` containing the diff. After one or more round-trips the reconciliation completes and the relay sends the missing `EVENT` messages.

ORLY supports two negentropy modes:

- **Embedded mode** (standalone relay): The relay handles NIP-77 directly in-process. No extra services needed.
- **gRPC mode** (split IPC via launcher): A separate negentropy service process handles reconciliation, communicating with the relay over gRPC.

The mode is determined by `ORLY_SYNC_TYPE`:
- `local` (default) = embedded mode
- `grpc` = gRPC mode (used automatically when running via `orly launcher`)

## Quick Start: Standalone Relay

The simplest setup. One env var, one process:

```bash
ORLY_NEGENTROPY_ENABLED=true ./orly
```

This starts the relay with an embedded negentropy handler. Clients can immediately use `NEG-OPEN`/`NEG-MSG`/`NEG-CLOSE` messages.

**Verify it works** — you should see this in the logs:

```
initializing embedded negentropy handler
embedded negentropy handler initialized (NIP-77 enabled)
```

If you don't see these lines, negentropy is not active. Check that `ORLY_NEGENTROPY_ENABLED=true` is actually set (not `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` — that's a different variable for the launcher).

**Verify via NIP-11** — the relay info document should list NIP-77 support:

```bash
curl -s -H "Accept: application/nostr+json" https://your-relay.example.com | jq '.supported_nips'
```

## Quick Start: Launcher Mode (Split IPC)

When running via `orly launcher`, negentropy runs as a separate subprocess. The launcher automatically sets all the env vars the relay needs (`ORLY_SYNC_TYPE=grpc`, `ORLY_NEGENTROPY_ENABLED=true`, `ORLY_GRPC_SYNC_NEGENTROPY=...`).

You only need to tell the **launcher** to enable it:

```bash
ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true ./orly launcher
```

**Verify it works** — you should see these log lines in order:

```
starting negentropy service on 127.0.0.1:50064
negentropy service is ready
connecting to gRPC negentropy server at 127.0.0.1:50064
gRPC negentropy client connected
```

If you see `negentropy client is nil` when a client sends `NEG-OPEN`, the gRPC connection failed. See Troubleshooting below.

## Relay-to-Relay Peer Sync

To sync events with other relays in the background:

```bash
# Standalone
ORLY_NEGENTROPY_ENABLED=true \
ORLY_SYNC_NEGENTROPY_PEERS=wss://relay1.example.com,wss://relay2.example.com \
./orly

# Launcher mode
ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true \
ORLY_SYNC_NEGENTROPY_PEERS=wss://relay1.example.com,wss://relay2.example.com \
./orly launcher
```

The relay periodically reconciles with each peer using negentropy, pulling and pushing events as needed.

## Configuration Reference

### Relay Variables

These control the relay's NIP-77 behavior. In launcher mode, most are set automatically by the launcher.

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEGENTROPY_ENABLED` | `false` | Enable NIP-77 negentropy support. **Required.** |
| `ORLY_SYNC_TYPE` | `local` | `local` = embedded handler, `grpc` = connect to gRPC service. The launcher sets this automatically. |
| `ORLY_GRPC_SYNC_NEGENTROPY` | `127.0.0.1:50056` | gRPC address of negentropy service. Only used when `ORLY_SYNC_TYPE=grpc`. |
| `ORLY_GRPC_SYNC_TIMEOUT` | `10s` | Timeout for gRPC connection to negentropy service. |

### Launcher Variables

These control whether and how the launcher spawns the negentropy subprocess.

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` | `false` | Enable negentropy subprocess in launcher mode. |
| `ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN` | `127.0.0.1:50064` | gRPC listen address for the negentropy service. |

### Negentropy Service Variables

These configure the negentropy service itself (the subprocess, not the relay).

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_NEGENTROPY_PEERS` | | Comma-separated WebSocket URLs of peer relays to sync with. |
| `ORLY_SYNC_NEGENTROPY_INTERVAL` | `60s` | How often to sync with peers. |
| `ORLY_SYNC_NEGENTROPY_FRAME_SIZE` | `4096` | Negentropy protocol frame size in bytes. |
| `ORLY_SYNC_NEGENTROPY_ID_SIZE` | `16` | Event ID truncation size (bytes). 16 = first 16 bytes of 32-byte ID. |
| `ORLY_SYNC_NEGENTROPY_SESSION_TIMEOUT` | `5m` | How long a client reconciliation session stays open before timeout. |

### Filter Variables

Limit which events are included in negentropy reconciliation:

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_SYNC_NEGENTROPY_FILTER_KINDS` | | Comma-separated kind numbers (e.g., `0,1,3,6,7`). |
| `ORLY_SYNC_NEGENTROPY_FILTER_AUTHORS` | | Comma-separated hex pubkeys. |
| `ORLY_SYNC_NEGENTROPY_FILTER_SINCE` | | Unix timestamp: only events after this time. |
| `ORLY_SYNC_NEGENTROPY_FILTER_UNTIL` | | Unix timestamp: only events before this time. |

## Deployment Examples

### systemd (Standalone)

```ini
[Unit]
Description=ORLY Nostr Relay
After=network.target

[Service]
ExecStart=/usr/local/bin/orly
Environment=ORLY_NEGENTROPY_ENABLED=true
Environment=ORLY_PORT=3334

[Install]
WantedBy=multi-user.target
```

### systemd (Launcher with Negentropy)

```ini
[Unit]
Description=ORLY Nostr Relay (Launcher)
After=network.target

[Service]
ExecStart=/usr/local/bin/orly launcher
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
Environment=ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN=127.0.0.1:50064
Environment=ORLY_PORT=3334

[Install]
WantedBy=multi-user.target
```

### Docker/Cloudron (supervisord)

```ini
[program:orly-db]
command=/app/code/orly db --driver=badger
environment=ORLY_DATA_DIR="/app/data/ORLY"
priority=15

[program:orly-negentropy]
command=/app/code/orly sync --driver=negentropy
environment=ORLY_SYNC_NEGENTROPY_LISTEN="127.0.0.1:50064",ORLY_SYNC_NEGENTROPY_DB_TYPE="grpc",ORLY_SYNC_NEGENTROPY_DB_SERVER="127.0.0.1:50051"
priority=20

[program:orly]
command=/app/code/orly
environment=ORLY_DB_TYPE="grpc",ORLY_GRPC_SERVER="127.0.0.1:50051",ORLY_NEGENTROPY_ENABLED="true",ORLY_SYNC_TYPE="grpc",ORLY_GRPC_SYNC_NEGENTROPY="127.0.0.1:50064"
priority=25
```

Note: when running the relay process directly (not via launcher), you must set BOTH `ORLY_NEGENTROPY_ENABLED=true` AND `ORLY_SYNC_TYPE=grpc` to use the gRPC negentropy service. Missing `ORLY_SYNC_TYPE=grpc` causes the relay to silently use the embedded handler instead.

## strfry Interoperability

ORLY's NIP-77 implementation is compatible with strfry's `sync` command:

```bash
# Pull events from ORLY into strfry
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir down

# Push events from strfry to ORLY
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir up

# Bidirectional
strfry sync wss://your-orly-relay.com --dir both
```

## Troubleshooting

### "negentropy client is nil" in logs

The relay received a `NEG-OPEN` from a client but no negentropy handler was registered. Causes:

1. **`ORLY_NEGENTROPY_ENABLED` not set to `true`** — The relay skips negentropy init entirely. No log output at all for negentropy.

2. **Launcher mode but `ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED` not set** — The launcher won't spawn the negentropy subprocess and won't pass `ORLY_NEGENTROPY_ENABLED=true` to the relay.

3. **gRPC connection failed** — Look for `failed to connect to gRPC negentropy server` in logs. The negentropy service isn't running or the address doesn't match.

4. **gRPC timeout** — Look for `timeout waiting for gRPC negentropy server`. The service started but didn't become ready within 30 seconds. May indicate a database connectivity issue in the negentropy service.

5. **Wrong `ORLY_SYNC_TYPE` in manual (non-launcher) gRPC setup** — If you run the relay process manually with a separate negentropy service, you MUST set `ORLY_SYNC_TYPE=grpc`. Without it, the relay defaults to `local` and uses the embedded handler, ignoring the gRPC address entirely.

### No log output about negentropy at startup

`ORLY_NEGENTROPY_ENABLED` is not `true`. The init function returns immediately without logging anything. Set it and restart.

### "embedded negentropy handler initialized" but expected gRPC mode

`ORLY_SYNC_TYPE` is `local` (the default). The relay used the embedded handler instead of connecting to your gRPC service. Set `ORLY_SYNC_TYPE=grpc` or use `orly launcher` which sets it automatically.

### Peer sync not working

- Verify peer URLs are reachable: `curl -s -H "Accept: application/nostr+json" wss://peer-relay.com`
- Verify peer relay supports NIP-77: check `supported_nips` in relay info
- Enable debug logging: `ORLY_LOG_LEVEL=debug`
- Check filter config: over-restrictive filters may exclude all events

### High memory during sync

- Reduce `ORLY_SYNC_NEGENTROPY_FRAME_SIZE` to `2048` or lower
- Use filter variables to limit the reconciliation set
- Increase `ORLY_SYNC_NEGENTROPY_INTERVAL` to reduce frequency

## Architecture

### Standalone Mode

```
Client ──NEG-OPEN──► ┌────────────────────────────┐
Client ◄──NEG-MSG─── │       orly (relay)          │
Client ──NEG-MSG───► │                              │
Client ◄──EVENT───── │  embedded negentropy handler │
                      │           ↕                  │
                      │     badger/neo4j DB          │
                      └────────────────────────────────┘
```

### Launcher / Split IPC Mode

```
Client ──NEG-OPEN──► ┌──────────┐  gRPC   ┌───────────────────┐
Client ◄──NEG-MSG─── │  orly    │ ──────► │ orly sync          │
Client ──NEG-MSG───► │ (relay)  │ ◄────── │ --driver=negentropy│
Client ◄──EVENT───── └──────────┘         └─────────┬──────────┘
                           │                          │
                           │ gRPC                     │ gRPC
                           ▼                          ▼
                      ┌──────────────────────────────────┐
                      │       orly db --driver=badger     │
                      └──────────────────────────────────┘
```

## See Also

- [IPC Sync Services Architecture](./IPC_SYNC_SERVICES.md)
- [NIP-77 Specification](https://github.com/nostr-protocol/nips/blob/master/77.md)
- [strfry sync documentation](https://github.com/hoytech/strfry/blob/master/docs/sync.md)
