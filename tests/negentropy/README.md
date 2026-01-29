# Comprehensive Negentropy Sync Test Suite

This test suite validates NIP-77 negentropy synchronization between ORLY and strfry relays in all possible configurations.

## Test Scenarios

### 1. Orly as Relay, Strfry as Client
Uses `strfry sync` command to test:
- **Push**: strfry → orly-relay-1
- **Pull**: strfry ← orly-relay-1
- **Bidirectional**: strfry ↔ orly-relay-1

### 2. Strfry as Relay, Orly as Client
Uses `orly sync` with gRPC client mode:
- **Push**: orly-relay-2 → strfry
- **Pull**: orly-relay-2 ← strfry
- **Bidirectional**: orly-relay-2 ↔ strfry

### 3. Dual Orly with gRPC Control
Two ORLY relays synchronized via gRPC sync services:
- **orly-relay-1** ↔ **orly-relay-2** via gRPC-controlled sync

## Infrastructure

```
┌─────────────┐         ┌─────────────┐
│   strfry    │◄───────►│ orly-relay-1│
│  (7777)     │  NIP-77 │   (3334)    │
└──────┬──────┘         └──────┬──────┘
       │                       │
       │                       │ gRPC
       │                 ┌─────┴──────┐
       │                 │ orly-sync-1│
       │                 │ (50064)    │
       │                 └────────────┘
       │
       │ NIP-77          ┌─────────────┐
       └────────────────►│ orly-relay-2│
                         │   (3335)    │
                         └──────┬──────┘
                                │ gRPC
                          ┌─────┴──────┐
                          │ orly-sync-2│
                          │ (50064)    │
                          └────────────┘
```

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.24+ (for local event generator builds)

### Run All Tests

```bash
cd tests/negentropy

# Build all images
docker compose build

# Start infrastructure
docker compose up -d

# Run comprehensive tests
docker compose exec test-runner /tests/comprehensive-test.sh

# Or with verbose output
docker compose exec test-runner /tests/comprehensive-test.sh --verbose

# Clean up
docker compose down -v
```

### Run Individual Test Phases

```bash
# Enter test runner container
docker compose exec test-runner bash

# Check relay status
echo "Strfry events: $(count_events ws://strfry:7777 '{"limit": 1000}')"
echo "Orly-1 events: $(count_events ws://orly-relay-1:3334 '{"limit": 1000}')"
echo "Orly-2 events: $(count_events ws://orly-relay-2:3335 '{"limit": 1000}')"

# Generate events manually
event-generator -count 500 -relay ws://strfry:7777

# Test strfry as client (pull)
docker compose exec strfry /app/strfry sync ws://orly-relay-1:3334 --dir down

# Test orly as client via gRPC (pull)
orly sync ws://strfry:7777 --server orly-sync-2:50064 --dir down --verbose
```

## Test Parameters

- **Total Events**: 1200+ per seed operation
- **Event Kinds**: 0, 1, 3, 1984, 10000, 10001, 30023, 30078
- **Batch Size**: 100 events per batch
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

## Filter Testing

The test suite validates sync with various filters:

```bash
# Kind filter
'{"kinds": [1, 3]}'

# Time range
'{"since": 1700000000, "until": 1800000000}'

# Limit
'{"limit": 100}'

# Combined
'{"kinds": [1], "since": 1700000000, "limit": 500}'
```

## Verification

Tests verify:
1. Event counts match expected values
2. Bidirectional sync achieves consistency
3. Filtered sync respects constraints
4. Different event kinds sync correctly
5. No data corruption during sync

## Troubleshooting

### Check service health
```bash
docker compose ps
docker compose logs -f strfry
docker compose logs -f orly-relay-1
```

### Manual event inspection
```bash
# Get events from strfry
echo '["REQ", "test", {"limit": 10}]' | websocat ws://localhost:7777

# Get events from orly
echo '["REQ", "test", {"limit": 10}]' | websocat ws://localhost:3334
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
- Features: Full NIP-77 negentropy support

### Orly Relay
- Image: Built from project (Dockerfile.orly)
- Ports: 3334, 3335
- Features: NIP-77 negentropy + gRPC database interface

### Orly Sync Service
- Image: Built from cmd/orly-sync-negentropy
- Ports: 50064, 50065
- Features: gRPC-controlled negentropy sync

### Test Runner
- Image: Built with all test tools
- Features: event-generator, websocat, orly CLI
