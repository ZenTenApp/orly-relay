# next.orly.dev

---

![orly.dev](./docs/orly.png)

![Version v0.24.1](https://img.shields.io/badge/version-v0.24.1-blue.svg)
[![Documentation](https://img.shields.io/badge/godoc-documentation-blue.svg)](https://pkg.go.dev/next.orly.dev)
[![Support this project](https://img.shields.io/badge/donate-geyser_crowdfunding_project_page-orange.svg)](https://geyser.fund/project/orly)

zap me: �mlekudev@getalby.com

follow me on [nostr](https://jumble.social/users/npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku)

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

To build the relay binary only:

```bash
git clone <repository-url>
cd next.orly.dev
go build -o orly
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
go build -o orly
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
go build -o orly

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

### Follows ACL

The follows ACL (Access Control List) system provides flexible relay access control based on social relationships in the Nostr network.

```bash
export ORLY_ACL_MODE=follows
export ORLY_ADMINS=npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku
./orly
```

The system grants write access to users followed by designated admins, with read-only access for others. Follow lists update dynamically as admins modify their relationships.

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
