# next.orly.dev

---

![orly.dev](./docs/orly.png)

![Version v0.62.0](https://img.shields.io/badge/version-v0.62.0-blue.svg)
[![Documentation](https://img.shields.io/badge/godoc-documentation-blue.svg)](https://pkg.go.dev/next.orly.dev)
[![Support this project](https://img.shields.io/badge/donate-geyser_crowdfunding_project_page-orange.svg)](https://geyser.fund/project/orly)

zap me: �mlekudev@getalby.com

follow me on [nostr](https://jumble.social/users/npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku)

## Architecture Overview

ORLY supports a modular IPC architecture where core functionality runs as independent gRPC services:

```
orly launcher (process supervisor)
├── orly db              (gRPC :50051) - Event storage & queries
├── orly acl             (gRPC :50052) - Access control
├── orly bridge          (SMTP :2525)  - Marmot email bridge (DM ↔ SMTP)
├── orly sync distributed (gRPC :50061) - Peer-to-peer sync
├── orly sync cluster    (gRPC :50062) - Cluster replication
├── orly sync relaygroup (gRPC :50063) - Relay group config (Kind 39105)
├── orly sync negentropy (gRPC :50064) - NIP-77 set reconciliation
└── orly                 (WebSocket/HTTP) - Main relay
```

**Benefits**:
- **Resource isolation**: Database, ACL, and sync run in separate processes
- **Independent scaling**: Scale sync services independently from the relay
- **Fault tolerance**: Service crashes don't bring down the entire relay
- **Modular deployment**: Enable only the services you need

See [docs/IPC_SYNC_SERVICES.md](./docs/IPC_SYNC_SERVICES.md) for detailed API documentation.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Bug Reports & Feature Requests](#%EF%B8%8F-bug-reports--feature-requests)
- [System Requirements](#%EF%B8%8F-system-requirements)
- [About](#about)
- [Performance & Cryptography](#performance--cryptography)
- [Building](#building)
  - [Prerequisites](#prerequisites)
  - [Basic Build](#basic-build)
  - [Building with Web UI](#building-with-web-ui)
- [Core Features](#core-features)
  - [Web UI](#web-ui)
  - [Sprocket Event Processing](#sprocket-event-processing)
  - [Policy System](#policy-system)
- [Deployment](#deployment)
  - [Automated Deployment](#automated-deployment)
  - [TLS Configuration](#tls-configuration)
  - [systemd Service Management](#systemd-service-management)
  - [Remote Deployment](#remote-deployment)
  - [Configuration](#configuration)
  - [Firewall Configuration](#firewall-configuration)
  - [Monitoring](#monitoring)
- [Testing](#testing)
- [Command-Line Tools](#command-line-tools)
- [Access Control](#access-control)
  - [Follows ACL](#follows-acl)
  - [Curation ACL](#curation-acl)
  - [Cluster Replication](#cluster-replication)
- [Marmot Email Bridge](#marmot-email-bridge)
  - [Client Setup (White Noise, etc.)](#client-setup-white-noise-etc)
- [Docker Deployment](#docker-deployment)
- [Negentropy Sync (NIP-77)](#negentropy-sync-nip-77)
- [Documentation](#documentation)
- [Developer Notes](#developer-notes)

## ⚠️ Bug Reports & Feature Requests

**Bug reports and feature requests that do not follow the protocol will not be accepted.**

Before submitting any issue, you must read and follow [BUG_REPORTS_AND_FEATURE_REQUEST_PROTOCOL.md](./BUG_REPORTS_AND_FEATURE_REQUEST_PROTOCOL.md).

Requirements:
- **Bug reports**: Include environment details, reproduction steps, expected/actual behavior, and logs
- **Feature requests**: Include problem statement, proposed solution, and use cases
- **Both**: Search existing issues first, verify with latest version, provide minimal reproduction

Issues missing required information will be closed without review.

## ⚠️ System Requirements

> **IMPORTANT: ORLY requires a minimum of 500MB of free memory to operate.**
>
> The relay uses adaptive PID-controlled rate limiting to manage memory pressure. By default, it will:
> - Auto-detect available system memory at startup
> - Target 66% of available memory, capped at 1.5GB for optimal performance
> - **Fail to start** if less than 500MB is available
>
> You can override the memory target with `ORLY_RATE_LIMIT_TARGET_MB` (e.g., `ORLY_RATE_LIMIT_TARGET_MB=2000` for 2GB).
>
> To disable rate limiting (not recommended): `ORLY_RATE_LIMIT_ENABLED=false`

## About

ORLY is a nostr relay written from the ground up to be performant, low latency, and built with a number of features designed to make it well suited for:

- personal relays
- small community relays
- business deployments and RaaS (Relay as a Service) with a nostr-native NWC client to allow accepting payments through NWC capable lightning nodes
- high availability clusters for reliability and/or providing a unified data set across multiple regions

## Performance & Cryptography

ORLY leverages high-performance libraries and custom optimizations for exceptional speed:

- **SIMD Libraries**: Uses [minio/sha256-simd](https://github.com/minio/sha256-simd) for accelerated SHA256 hashing
- **p256k1 Cryptography**: Implements [p256k1.mleku.dev](https://github.com/p256k1/p256k1) for fast elliptic curve operations optimized for nostr
- **Fast Message Encoders**: High-performance encoding/decoding with [templexxx/xhex](https://github.com/templexxx/xhex) for SIMD-accelerated hex operations

The encoders achieve **24% faster JSON marshaling**, **16% faster canonical encoding**, and **54-91% reduction in memory allocations** through custom buffer pre-allocation and zero-allocation optimization techniques.

ORLY uses a fast embedded [badger](https://github.com/hypermodeinc/badger) database with a database designed for high performance querying and event storage.

## Building

ORLY is a standard Go application that can be built using the Go toolchain.

### Prerequisites

- Go 1.25.3 or later
- Git
- For web UI: [Bun](https://bun.sh/) JavaScript runtime

### Basic Build

To build the unified binary (relay + all subcommands):

```bash
git clone <repository-url>
cd next.orly.dev
go build -o orly ./cmd/orly
```

To build the relay-only binary (no subcommands):

```bash
go build -o orly .
```

### Building with Web UI

To build with the embedded web interface:

```bash
# Build the Svelte web application
cd app/web
bun install
bun run build

# Build the Go binary from project root
cd ../../
go build -o orly ./cmd/orly
```

The recommended way to build and embed the web UI is using the provided script:

```bash
./scripts/update-embedded-web.sh
```

This script will:
- Build the Svelte app in `app/web` to `app/web/dist` using Bun (preferred) or fall back to npm/yarn/pnpm
- Run `go install` from the repository root so the binary picks up the new embedded assets
- Automatically detect and use the best available JavaScript package manager

For manual builds, you can also use:

```bash
#!/bin/bash
# build.sh
echo "Building Svelte app..."
cd app/web
bun install
bun run build

echo "Building Go binary..."
cd ../../
go build -o orly ./cmd/orly

echo "Build complete!"
```

Make it executable with `chmod +x build.sh` and run with `./build.sh`.

## Core Features

### Web UI

ORLY includes a modern web-based user interface built with [Svelte](https://svelte.dev/) for relay management and monitoring.

- **Secure Authentication**: Nostr key pair authentication with challenge-response
- **Event Management**: Browse, export, import, and search events
- **User Administration**: Role-based permissions (guest, user, admin, owner)
- **Sprocket Management**: Upload and monitor event processing scripts
- **Real-time Updates**: Live event streaming and system monitoring
- **Responsive Design**: Works on desktop and mobile devices
- **Dark/Light Themes**: Persistent theme preferences

The web UI is embedded in the relay binary and accessible at the relay's root path.

#### Web UI Development

For development with hot-reloading, ORLY can proxy web requests to a local dev server while still handling WebSocket relay connections and API requests.

**Environment Variables:**

- `ORLY_WEB_DISABLE` - Set to `true` to disable serving the embedded web UI
- `ORLY_WEB_DEV_PROXY_URL` - URL of the dev server to proxy web requests to (e.g., `localhost:8080`)

**Setup:**

1. Start the dev server (in one terminal):

```bash
cd app/web
bun install
bun run dev
```

Note the port sirv is listening on (e.g., `http://localhost:8080`).

2. Start the relay with dev proxy enabled (in another terminal):

```bash
export ORLY_WEB_DISABLE=true
export ORLY_WEB_DEV_PROXY_URL=localhost:8080
./orly
```

The relay will:

- Handle WebSocket connections at `/` for Nostr protocol
- Handle API requests at `/api/*`
- Proxy all other requests (HTML, JS, CSS, assets) to the dev server

**With a reverse proxy/tunnel:**

If you're running behind a reverse proxy or tunnel (e.g., Caddy, nginx, Cloudflare Tunnel), the setup is the same. The relay listens locally and your reverse proxy forwards traffic to it:

```
Browser � Reverse Proxy � ORLY (port 3334) � Dev Server (port 8080)
                              �
                         WebSocket/API
```

Example with the relay on port 3334 and sirv on port 8080:

```bash
# Terminal 1: Dev server
cd app/web && bun run dev
# Output: Your application is ready~!
#         Local: http://localhost:8080

# Terminal 2: Relay
export ORLY_WEB_DISABLE=true
export ORLY_WEB_DEV_PROXY_URL=localhost:8080
export ORLY_PORT=3334
./orly
```

**Disabling the web UI without a proxy:**

If you only want to disable the embedded web UI (without proxying to a dev server), just set `ORLY_WEB_DISABLE=true` without setting `ORLY_WEB_DEV_PROXY_URL`. The relay will return 404 for web UI requests while still handling WebSocket and API requests.

### Sprocket Event Processing

ORLY includes a powerful sprocket system for external event processing scripts. Sprocket scripts enable custom filtering, validation, and processing logic for Nostr events before storage.

- **Real-time Processing**: Scripts receive events via stdin and respond with JSONL decisions
- **Three Actions**: `accept`, `reject`, or `shadowReject` events based on custom logic
- **Automatic Recovery**: Failed scripts are automatically disabled with periodic recovery attempts
- **Web UI Management**: Upload, configure, and monitor scripts through the admin interface

```bash
export ORLY_SPROCKET_ENABLED=true
export ORLY_APP_NAME="ORLY"
# Place script at ~/.config/ORLY/sprocket.sh
```

For detailed configuration and examples, see the [sprocket documentation](docs/sprocket/).

### Policy System

ORLY includes a comprehensive policy system for fine-grained control over event storage and retrieval. Configure custom validation rules, access controls, size limits, and age restrictions.

- **Access Control**: Allow/deny based on pubkeys, roles, or social relationships
- **Content Filtering**: Size limits, age validation, and custom rules
- **Script Integration**: Execute custom scripts for complex policy logic
- **Real-time Enforcement**: Policies applied to both read and write operations

```bash
export ORLY_POLICY_ENABLED=true
# Default policy file: ~/.config/ORLY/policy.json

# OPTIONAL: Use a custom policy file location
# WARNING: ORLY_POLICY_PATH MUST be an ABSOLUTE path (starting with /)
# Relative paths will be REJECTED and the relay will fail to start
export ORLY_POLICY_PATH=/etc/orly/policy.json
```

For detailed configuration and examples, see the [Policy Usage Guide](docs/POLICY_USAGE_GUIDE.md).

## Deployment

ORLY includes an automated deployment script that handles Go installation, dependency setup, building, and systemd service configuration.

### Automated Deployment

The deployment script (`scripts/deploy.sh`) provides a complete setup solution:

```bash
# Clone the repository
git clone <repository-url>
cd next.orly.dev

# Run the deployment script
./scripts/deploy.sh
```

The script will:

1. **Install Go 1.25.3** if not present (in `~/.local/go`)
2. **Configure environment** by creating `~/.goenv` and updating `~/.bashrc`
3. **Build the relay** with embedded web UI using `update-embedded-web.sh`
4. **Set capabilities** for port 443 binding (requires sudo)
5. **Install binary** to `~/.local/bin/orly`
6. **Create systemd service** and enable it

After deployment, reload your shell environment:

```bash
source ~/.bashrc
```

### TLS Configuration

ORLY supports automatic TLS certificate management with Let's Encrypt and custom certificates:

```bash
# Enable TLS with Let's Encrypt for specific domains
export ORLY_TLS_DOMAINS=relay.example.com,backup.relay.example.com

# Optional: Use custom certificates (will load .pem and .key files)
export ORLY_CERTS=/path/to/cert1,/path/to/cert2

# When TLS domains are configured, ORLY will:
# - Listen on port 443 for HTTPS/WSS
# - Listen on port 80 for ACME challenges
# - Ignore ORLY_PORT setting
```

Certificate files should be named with `.pem` and `.key` extensions:
- `/path/to/cert1.pem` (certificate)
- `/path/to/cert1.key` (private key)

### systemd Service Management

The deployment script creates a systemd service for easy management:

```bash
# Start the service
sudo systemctl start orly

# Stop the service
sudo systemctl stop orly

# Restart the service
sudo systemctl restart orly

# Enable service to start on boot
sudo systemctl enable orly --now

# Disable service from starting on boot
sudo systemctl disable orly --now

# Check service status
sudo systemctl status orly

# View service logs
sudo journalctl -u orly -f

# View recent logs
sudo journalctl -u orly --since "1 hour ago"
```

### Remote Deployment

You can deploy ORLY on a remote server using SSH:

```bash
# Deploy to a VPS with SSH key authentication
ssh user@your-server.com << 'EOF'
  # Clone and deploy
  git clone <repository-url>
  cd next.orly.dev
  ./scripts/deploy.sh

  # Configure your relay
  echo 'export ORLY_TLS_DOMAINS=relay.example.com' >> ~/.bashrc
  echo 'export ORLY_ADMINS=npub1your_admin_key_here' >> ~/.bashrc

  # Start the service
  sudo systemctl start orly --now
EOF

# Check deployment status
ssh user@your-server.com 'sudo systemctl status orly'
```

### Configuration

After deployment, configure your relay by setting environment variables in your shell profile:

```bash
# Add to ~/.bashrc or ~/.profile
export ORLY_TLS_DOMAINS=relay.example.com
export ORLY_ADMINS=npub1your_admin_key
export ORLY_ACL_MODE=follows
export ORLY_APP_NAME="MyRelay"
```

Then restart the service:

```bash
source ~/.bashrc
sudo systemctl restart orly
```

### Firewall Configuration

Ensure your firewall allows the necessary ports:

```bash
# For TLS-enabled relays
sudo ufw allow 80/tcp   # HTTP (ACME challenges)
sudo ufw allow 443/tcp  # HTTPS/WSS

# For non-TLS relays
sudo ufw allow 3334/tcp # Default ORLY port

# Enable firewall if not already enabled
sudo ufw enable
```

### Monitoring

Monitor your relay using systemd and standard Linux tools:

```bash
# Service status and logs
sudo systemctl status orly
sudo journalctl -u orly -f

# Resource usage
htop
sudo ss -tulpn | grep orly

# Disk usage (database grows over time)
du -sh ~/.local/share/ORLY/

# Check TLS certificates (if using Let's Encrypt)
ls -la ~/.local/share/ORLY/autocert/
```

## Testing

ORLY includes comprehensive testing tools for protocol validation and performance testing.

- **Protocol Testing**: Use `relay-tester` for Nostr protocol compliance validation
- **Stress Testing**: Performance testing under various load conditions
- **Benchmark Suite**: Comparative performance testing across relay implementations

For detailed testing instructions, multi-relay testing scenarios, and advanced usage, see the [Relay Testing Guide](docs/RELAY_TESTING_GUIDE.md).

The benchmark suite provides comprehensive performance testing and comparison across multiple relay implementations, including throughput, latency, and memory usage metrics.

## Command-Line Tools

ORLY includes several command-line utilities in the `cmd/` directory for testing, debugging, and administration.

### relay-tester

Nostr protocol compliance testing tool. Validates that a relay correctly implements the Nostr protocol specification.

```bash
# Run all protocol compliance tests
go run ./cmd/relay-tester -url ws://localhost:3334

# List available tests
go run ./cmd/relay-tester -list

# Run specific test
go run ./cmd/relay-tester -url ws://localhost:3334 -test "Basic Event"

# Output results as JSON
go run ./cmd/relay-tester -url ws://localhost:3334 -json
```

### benchmark

Comprehensive relay performance benchmarking tool. Tests event storage, queries, and subscription performance with detailed latency metrics (P90, P95, P99).

```bash
# Run benchmarks against local database
go run ./cmd/benchmark -data-dir /tmp/bench-db -events 10000 -workers 4

# Run benchmarks against a running relay
go run ./cmd/benchmark -relay ws://localhost:3334 -events 5000

# Use different database backends
go run ./cmd/benchmark -dgraph -events 10000
go run ./cmd/benchmark -neo4j -events 10000
```

The `cmd/benchmark/` directory also includes Docker Compose configurations for comparative benchmarks across multiple relay implementations (strfry, nostr-rs-relay, khatru, etc.).

### stresstest

Load testing tool for evaluating relay performance under sustained high-traffic conditions. Generates events with random content and tags to simulate realistic workloads.

```bash
# Run stress test with 10 concurrent workers
go run ./cmd/stresstest -url ws://localhost:3334 -workers 10 -duration 60s

# Generate events with random p-tags (up to 100 per event)
go run ./cmd/stresstest -url ws://localhost:3334 -workers 5
```

### blossomtest

Tests the Blossom blob storage protocol (BUD-01/BUD-02) implementation. Validates upload, download, and authentication flows.

```bash
# Test with generated key
go run ./cmd/blossomtest -url http://localhost:3334 -size 1024

# Test with specific nsec
go run ./cmd/blossomtest -url http://localhost:3334 -nsec nsec1...

# Test anonymous uploads (no authentication)
go run ./cmd/blossomtest -url http://localhost:3334 -no-auth
```

### aggregator

Event aggregation utility that fetches events from multiple relays using bloom filters for deduplication. Useful for syncing events across relays with memory-efficient duplicate detection.

```bash
go run ./cmd/aggregator -relays wss://relay1.com,wss://relay2.com -output events.jsonl
```

### convert

Key format conversion utility. Converts between hex and bech32 (npub/nsec) formats for Nostr keys.

```bash
# Convert npub to hex
go run ./cmd/convert npub1abc...

# Convert hex to npub
go run ./cmd/convert 0123456789abcdef...

# Convert secret key (nsec or hex) - outputs both nsec and derived npub
go run ./cmd/convert --secret nsec1xyz...
```

### FIND

Free Internet Name Daemon - CLI tool for the distributed naming system. Manages name registration, transfers, and certificate issuance.

```bash
# Validate a name format
go run ./cmd/FIND verify-name example.nostr

# Generate a new key pair
go run ./cmd/FIND generate-key

# Create a registration proposal
go run ./cmd/FIND register myname.nostr

# Transfer a name to a new owner
go run ./cmd/FIND transfer myname.nostr npub1newowner...
```

### policytest

Tests the policy system for event write control. Validates that policy rules correctly allow or reject events based on kind, pubkey, and other criteria.

```bash
go run ./cmd/policytest -url ws://localhost:3334 -type event -kind 4678
go run ./cmd/policytest -url ws://localhost:3334 -type req -kind 1
go run ./cmd/policytest -url ws://localhost:3334 -type publish-and-query -count 5
```

### policyfiltertest

Tests policy-based filtering with authorized and unauthorized pubkeys. Validates access control rules for specific users.

```bash
go run ./cmd/policyfiltertest -url ws://localhost:3334 \
  -allowed-pubkey <hex> -allowed-sec <hex> \
  -unauthorized-pubkey <hex> -unauthorized-sec <hex>
```

### subscription-test

Tests WebSocket subscription stability over extended periods. Monitors for dropped subscriptions and connection issues.

```bash
# Run subscription stability test for 60 seconds
go run ./cmd/subscription-test -url ws://localhost:3334 -duration 60 -kind 1

# With verbose output
go run ./cmd/subscription-test -url ws://localhost:3334 -duration 120 -v
```

### subscription-test-simple

Simplified subscription stability test that verifies subscriptions remain active without dropping over the test duration.

```bash
go run ./cmd/subscription-test-simple -url ws://localhost:3334 -duration 120
```

## Access Control

ORLY provides four ACL (Access Control List) modes to control who can publish events to your relay:

| Mode | Description | Best For |
|------|-------------|----------|
| `none` | Open relay, anyone can write | Public relays |
| `follows` | Write access based on admin follow lists | Personal/community relays |
| `managed` | Explicit allow/deny lists via NIP-86 API | Private relays |
| `curating` | Three-tier classification with rate limiting | Curated community relays |

```bash
export ORLY_ACL_MODE=follows  # or: none, managed, curating
```

### Follows ACL

The follows ACL system provides flexible relay access control based on social relationships in the Nostr network.

```bash
export ORLY_ACL_MODE=follows
export ORLY_ADMINS=npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku
./orly
```

The system grants write access to users followed by designated admins, with read-only access for others. Follow lists update dynamically as admins modify their relationships.

### Curation ACL

The curation ACL mode provides sophisticated content curation with a three-tier publisher classification system:

- **Trusted**: Unlimited publishing, bypass rate limits
- **Blacklisted**: Blocked from publishing, invisible to regular users
- **Unclassified**: Rate-limited publishing (default 50 events/day)

Key features:
- **Kind whitelisting**: Only allow specific event kinds (e.g., social, DMs, longform)
- **IP-based flood protection**: Auto-ban IPs that exceed rate limits
- **Spam flagging**: Mark events as spam without deleting
- **Web UI management**: Configure via the built-in curation interface

```bash
export ORLY_ACL_MODE=curating
export ORLY_OWNERS=npub1your_owner_key
./orly
```

After starting, publish a configuration event (kind 30078) to enable the relay. The web UI at `/#curation` provides a complete management interface.

For detailed configuration and API documentation, see the [Curation Mode Guide](docs/CURATION_MODE_GUIDE.md).

### Cluster Replication

ORLY supports distributed relay clusters using active replication. When configured with peer relays, ORLY will automatically synchronize events between cluster members using efficient HTTP polling.

```bash
export ORLY_RELAY_PEERS=https://peer1.example.com,https://peer2.example.com
export ORLY_CLUSTER_ADMINS=npub1cluster_admin_key
```

**Privacy Considerations:** By default, ORLY propagates all events including privileged events (DMs, gift wraps, etc.) to cluster peers for complete synchronization. This ensures no data loss but may expose private communications to other relay operators in your cluster.

To enhance privacy, you can disable propagation of privileged events:

```bash
export ORLY_CLUSTER_PROPAGATE_PRIVILEGED_EVENTS=false
```

**Important:** When disabled, privileged events will not be replicated to peer relays. This provides better privacy but means these events will only be available on the originating relay. Users should be aware that accessing their privileged events may require connecting directly to the relay where they were originally published.

## Marmot Email Bridge

ORLY includes a bidirectional Nostr DM to SMTP email bridge. Users DM the bridge's Nostr pubkey to subscribe (via Lightning payment), send outbound email, and receive inbound email as encrypted DMs.

### Features

- **Outbound email**: Send a DM with `To:` / `Subject:` headers to the bridge pubkey — it sends the email via SMTP
- **Inbound email**: Email sent to `npub@yourdomain` is delivered as an encrypted DM with a reply link
- **Lightning subscriptions**: Users pay via NWC (Nostr Wallet Connect) Lightning invoices for 30-day access
- **Attachment encryption**: Non-plaintext email parts are zipped, encrypted with ChaCha20-Poly1305, and uploaded to Blossom with fragment-key URLs
- **DKIM signing**: Outbound email is DKIM-signed for deliverability (or use an SMTP smarthost)
- **Dual DM protocol**: Supports both NIP-04 (kind 4) and NIP-17 gift-wrapped (kind 1059) DMs
- **Rate limiting**: Sliding-window rate limits per user and globally

### Quick Start

```bash
# 1. Build the unified binary
go build -o orly ./cmd/orly

# 2. Set environment
export ORLY_BRIDGE_ENABLED=true
export ORLY_BRIDGE_DOMAIN=yourdomain.com
export ORLY_BRIDGE_SMTP_PORT=2525
export ORLY_BRIDGE_NWC_URI="nostr+walletconnect://..."

# 3. Start (bridge runs alongside the relay)
./orly
```

Or run standalone against any Nostr relay:

```bash
export ORLY_BRIDGE_RELAY_URL=wss://relay.example.com
./orly bridge
```

### Bridge Profile

The bridge publishes a kind 0 (profile metadata) event on startup so Nostr clients can discover it. Create a `profile.txt` in the bridge data directory (or set `ORLY_BRIDGE_PROFILE`):

```
name: Marmot Bridge
about: Nostr-Email bridge at yourdomain.com. DM 'subscribe' to get started.
picture: https://yourdomain.com/avatar.png
nip05: bridge@yourdomain.com
lud16: tips@yourdomain.com
website: https://yourdomain.com
```

See [`profile.example.txt`](profile.example.txt) for a template.

### Client Setup (White Noise, etc.)

NIP-17 messaging clients like [White Noise](https://whitenoise.ing) need the bridge to have a **kind 10002** relay list event (NIP-65) to know where to send DMs. Without it, the client sees the bridge profile but reports the bridge "isn't on White Noise yet."

Until the bridge publishes kind 10002 automatically, publish one manually using [nak](https://github.com/fiatjaf/nak):

```bash
export NOSTR_SECRET_KEY=nsec1...  # Bridge identity
nak event --kind 10002 \
  --tag r='wss://your-relay.com/' \
  --tag r='wss://relay.damus.io/;read' \
  --tag r='wss://nos.lol/;read' \
  wss://your-relay.com wss://relay.damus.io wss://nos.lol
```

See [Bridge Deployment Guide — Client Setup: White Noise](docs/BRIDGE_DEPLOYMENT.md#client-setup-white-noise) for full instructions.

### Documentation

- [Bridge Deployment Guide](docs/BRIDGE_DEPLOYMENT.md) — DNS, DKIM, NWC, SMTP smarthost, port 25 workarounds, client setup, Migadu example

## Docker Deployment

ORLY ships a single Docker image that serves both the relay and the email bridge. The default entrypoint runs the relay; pass `bridge` as the command to run the email bridge.

### Build the Image

```bash
docker build -t orly .
```

### Run as Relay (default)

```bash
docker run --rm -p 3334:3334 -v orly-data:/data orly
```

### Run as Email Bridge

```bash
docker run --rm -p 2525:2525 --env-file .env.bridge orly bridge
```

### Docker Compose (Bridge)

A ready-made compose file is provided for bridge deployment:

```bash
cp .env.bridge.example .env.bridge   # edit with your values
docker compose -f docker-compose.bridge.yml up --build
```

See [`.env.bridge.example`](.env.bridge.example) for all available configuration variables.

## Negentropy Sync (NIP-77)

ORLY supports [NIP-77](https://github.com/nostr-protocol/nips/blob/master/77.md) negentropy-based set reconciliation for efficient relay synchronization.

### Quick Start

Enable negentropy client support:

```bash
export ORLY_NEGENTROPY_ENABLED=true
./orly
```

Enable peer relay synchronization:

```bash
export ORLY_NEGENTROPY_ENABLED=true
export ORLY_SYNC_NEGENTROPY_PEERS=wss://relay.orly.dev,wss://other-relay.com
./orly
```

### Split IPC Mode

For production deployments, run negentropy as a separate service:

```bash
# Build binaries
CGO_ENABLED=0 go build -o orly-sync-negentropy ./cmd/orly-sync-negentropy

# Configure launcher
export ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=true
export ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY=/path/to/orly-sync-negentropy
export ORLY_SYNC_NEGENTROPY_PEERS=wss://peer-relay.com
```

### strfry Compatibility

ORLY's negentropy implementation is compatible with strfry:

```bash
# Pull events from ORLY using strfry
strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1, 3]}' --dir down
```

For detailed configuration including Docker deployments, filtering options, and troubleshooting, see the [Negentropy Sync Guide](docs/NEGENTROPY_SYNC_GUIDE.md).

## Documentation

### Deployment & Operations

| Document | Description |
|----------|-------------|
| [Bridge Deployment Guide](docs/BRIDGE_DEPLOYMENT.md) | DNS, DKIM, NWC, SMTP for Marmot email bridge |
| [Deployment Testing](docs/DEPLOYMENT_TESTING.md) | Deployment verification procedures |
| [Build Platforms](docs/BUILD_PLATFORMS.md) | Multi-platform build guide (Linux, macOS, Windows, Android) |
| [Purego Build System](docs/PUREGO_BUILD_SYSTEM.md) | CGO-free build system with runtime library loading |
| [WASM/Mobile Builds](docs/WASM_MOBILE_BUILD_PLAN.md) | WebAssembly and mobile build targets |

### Configuration & Access Control

| Document | Description |
|----------|-------------|
| [Policy Usage Guide](docs/POLICY_USAGE_GUIDE.md) | Event filtering and validation rules |
| [Policy Configuration Reference](docs/POLICY_CONFIGURATION_REFERENCE.md) | Complete policy JSON schema |
| [Policy Troubleshooting](docs/POLICY_TROUBLESHOOTING.md) | Diagnosing policy issues |
| [Curation Mode Guide](docs/CURATION_MODE_GUIDE.md) | Three-tier publisher classification ACL |
| [HTTP Guard](docs/HTTP_GUARD.md) | Bot detection and HTTP rate limiting |
| [Branding Guide](docs/BRANDING_GUIDE.md) | Relay name, icon, NIP-11 customization |

### Architecture & Internals

| Document | Description |
|----------|-------------|
| [IPC Architecture](docs/IPC_ARCHITECTURE.md) | gRPC split-process design |
| [IPC Sync Services](docs/IPC_SYNC_SERVICES.md) | Sync service API reference |
| [Negentropy Sync Guide](docs/NEGENTROPY_SYNC_GUIDE.md) | NIP-77 set reconciliation setup |
| [Sync Client Mode](docs/SYNC_CLIENT_MODE.md) | Client-mode relay synchronization |
| [Neo4j Backend](docs/NEO4J_BACKEND.md) | Neo4j database driver setup and tuning |
| [NIP-77 Analysis](docs/NIP77_ANALYSIS_AND_FIX.md) | NIP-77 implementation details |

### Protocol Extensions

| Document | Description |
|----------|-------------|
| [FIND Names Spec](docs/names.md) | Free Internet Name Daemon protocol |
| [FIND Implementation](docs/FIND_IMPLEMENTATION_PLAN.md) | FIND integration architecture and status |
| [NIP-XX Graph Queries](docs/NIP-XX-GRAPH-QUERIES.md) | REQ filter extension for graph traversals |
| [NIP-XX Cluster Replication](docs/NIP-XX-Cluster-Replication.md) | HTTP polling-based cluster replication |
| [NIP-XX Responsive Images](docs/NIP-XX-responsive-images.md) | Image variant protocol extension |
| [NIP Curation](docs/NIP-CURATION.md) | Curation-mode protocol spec |
| [NIP NRC](docs/NIP-NRC.md) | Nostr Relay Connection protocol |

### Development & Reference

| Document | Description |
|----------|-------------|
| [Glossary](docs/GLOSSARY.md) | ORLY terminology and domain concepts |
| [Relay Testing Guide](docs/RELAY_TESTING_GUIDE.md) | Protocol compliance testing |
| [Web UI Event Templates](docs/WEB_UI_EVENT_TEMPLATES.md) | Event kind templates for the web UI |
| [Applesauce Reference](docs/applesauce-reference.md) | Applesauce library integration |
| [Graph Implementation Phases](docs/GRAPH_IMPLEMENTATION_PHASES.md) | Graph query feature tracker |
| [Graph Queries Remaining](docs/GRAPH_QUERIES_REMAINING_PLAN.md) | Outstanding graph query work |
| [Neo4j WoT Spec](pkg/neo4j/WOT_SPEC.md) | Web-of-Trust graph schema |
| [Neo4j Schema Changes](pkg/neo4j/MODIFYING_SCHEMA.md) | Guide for modifying the Neo4j schema |

## Developer Notes

### Binary-Optimized Tag Storage

The nostr library (`git.mleku.dev/mleku/nostr/encoders/tag`) uses binary optimization for `e` and `p` tags to reduce memory usage and improve comparison performance.

When events are unmarshaled from JSON, 64-character hex values in e/p tags are converted to 33-byte binary format (32 bytes hash + null terminator).

**Important:** When working with e/p tag values in code:

- **DO NOT** use `tag.Value()` directly - it returns raw bytes which may be binary, not hex
- **ALWAYS** use `tag.ValueHex()` to get a hex string regardless of storage format
- **Use** `tag.ValueBinary()` to get raw 32-byte binary (returns nil if not binary-encoded)

```go
// CORRECT: Use ValueHex() for hex decoding
pt, err := hex.Dec(string(pTag.ValueHex()))

// WRONG: Value() may return binary bytes, not hex
pt, err := hex.Dec(string(pTag.Value()))  // Will fail for binary-encoded tags!
```

### Release Process

The `/release` command pushes to multiple git remotes. To push to git.mleku.dev with the dedicated SSH key, ensure the `gitmlekudev` key is configured:

```bash
# SSH key should be at ~/.ssh/gitmlekudev
# The release command uses GIT_SSH_COMMAND to specify this key:
GIT_SSH_COMMAND="ssh -i ~/.ssh/gitmlekudev" git push ssh://mleku@git.mleku.dev:2222/mleku/next.orly.dev.git main --tags
```

Remotes pushed during release:
- `origin` - Primary remote
- `gitea` - Gitea mirror
- `git.mleku.dev` - Using `gitmlekudev` SSH key
