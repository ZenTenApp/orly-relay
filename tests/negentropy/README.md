# Comprehensive Negentropy Sync Test Suite

Tests NIP-77 negentropy synchronization between ORLY and strfry relays.

## Test Scenarios

### 1. strfry as Client, ORLY as Server (Phases 1-6)
Uses `strfry sync` command to test:
- **Push**: strfry → orly-relay-1
- **Pull**: strfry ← orly-relay-1
- **Bidirectional**: strfry ↔ orly-relay-1

### 2. ORLY as Client, ORLY as Server (Phases 7-10)
Uses `orly sync` CLI with a temporary Badger DB as a bridge:
- **relay-1 → relay-2**: orly CLI pulls from relay-1, pushes to relay-2
- **relay-2 → relay-1**: orly CLI pushes relay-2 events to relay-1
- **Three-way consistency**: strfry, relay-1, relay-2 converge

## Infrastructure

```
┌─────────────┐  NIP-77   ┌──────────────┐
│   strfry    │◄─────────►│ orly-relay-1 │
│  (7777)     │           │   (3334)     │
└─────────────┘           └──────┬───────┘
                                 │
                          orly sync CLI
                          (bridge DB in
                           test-runner)
                                 │
                          ┌──────┴───────┐
                          │ orly-relay-2 │
                          │   (3335)     │
                          └──────────────┘
```

The `orly sync` CLI runs inside the test-runner container. It opens a
temporary Badger database and uses NIP-77 negentropy to sync with each
relay, effectively bridging events between the two ORLY instances.

## Quick Start

### Prerequisites
- Docker and Docker Compose

### Run All Tests

```bash
cd tests/negentropy

# Build all images
docker compose build

# Start infrastructure
docker compose up -d

# Run comprehensive tests
./comprehensive-test.sh

# Or with verbose output
./comprehensive-test.sh --verbose

# Clean up
docker compose down -v
```

### Manual Operations

```bash
# Generate events
docker compose exec -T test-runner event-generator -count 500 -relay ws://strfry:7777

# strfry sync (strfry as client, ORLY as server)
docker compose exec -T strfry /app/strfry --config=/etc/strfry.conf sync ws://orly-relay-1:3334 --dir down

# orly sync (orly CLI as client, any relay as server)
docker compose exec -T test-runner orly sync ws://orly-relay-1:3334 --data-dir /tmp/sync-db

# Manual event inspection
echo '["REQ", "test", {"limit": 10}]' | websocat ws://localhost:7777
echo '["REQ", "test", {"limit": 10}]' | websocat ws://localhost:3334
echo '["REQ", "test", {"limit": 10}]' | websocat ws://localhost:3335
```

## Test Parameters

- **Seed Events**: 200 per relay
- **Extra Events**: 100
- **Event Kinds**: 0, 1, 3, 1984, 10000, 10001, 30023
- **Batch Size**: 50 events per batch
- **Authors**: 3 test keypairs (alice, bob, carol)

### Event Distribution

| Kind | Percentage | Description |
|------|------------|-------------|
| 1    | 60%        | Short text notes |
| 0    | 15%        | Metadata |
| 3    | 10%        | Contacts |
| 1984 | 5%         | Reports |
| 10000| 5%         | Mute lists |
| 10001| 3%         | Pin lists |
| 30023| 2%         | Long-form articles |

## Troubleshooting

### Check service health
```bash
docker compose ps
docker compose logs -f strfry
docker compose logs -f orly-relay-1
docker compose logs -f orly-relay-2
```

### Reset test data
```bash
docker compose down -v
docker compose up -d
```

## Architecture Details

### Strfry
- Image: Built from source (Dockerfile.strfry)
- Port: 7777
- Features: Full NIP-77 negentropy support, `strfry sync` CLI

### ORLY Relay
- Image: Built from project (Dockerfile.orly)
- Ports: 3334 (relay-1), 3335 (relay-2)
- Features: NIP-77 negentropy via embedded handler
- Note: `ORLY_QUERY_RESULT_LIMIT=10000` set for testing (default is 256)

### Test Runner
- Image: Alpine with test tools (Dockerfile.test-runner)
- Features: event-generator, orly CLI, websocat, curl, jq
