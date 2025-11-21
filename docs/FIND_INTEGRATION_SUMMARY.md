# FIND Name Binding System - Integration Summary

## Overview

The Free Internet Name Daemon (FIND) protocol has been integrated into ORLY, enabling human-readable name-to-npub bindings that are discoverable through standard Nostr queries. This document summarizes the implementation and provides guidance for using the system.

## What Was Implemented

### Core Components

1. **Consensus Engine** ([pkg/find/consensus.go](../pkg/find/consensus.go))
   - Implements trust-weighted consensus algorithm for name registrations
   - Validates proposals against renewal windows and ownership rules
   - Computes consensus scores from attestations
   - Enforces mandatory 1-year registration period with 30-day preferential renewal

2. **Trust Graph Manager** ([pkg/find/trust.go](../pkg/find/trust.go))
   - Manages web-of-trust relationships between registry services
   - Calculates direct and inherited trust (0-3 hops)
   - Applies hop-based decay factors (1.0, 0.8, 0.6, 0.4)
   - Provides metrics and analytics

3. **Registry Service** ([pkg/find/registry.go](../pkg/find/registry.go))
   - Monitors registration proposals (kind 30100)
   - Collects attestations from other registry services (kind 20100)
   - Publishes name state after consensus (kind 30102)
   - Manages pending proposals and attestation windows

4. **Event Parsers** ([pkg/find/parser.go](../pkg/find/parser.go))
   - Parses all FIND event types (30100-30105)
   - Validates event structure and required tags
   - Already complete - no changes needed

5. **Event Builders** ([pkg/find/builder.go](../pkg/find/builder.go))
   - Creates FIND events (registration proposals, attestations, name states, records)
   - Already complete - no changes needed

6. **Validators** ([pkg/find/validation.go](../pkg/find/validation.go))
   - DNS-style name format validation
   - IPv4/IPv6 address validation
   - Record type and value validation
   - Already complete - no changes needed

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      ORLY Relay                              │
│                                                               │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐  │
│  │  WebSocket     │  │  Registry      │  │   Database   │  │
│  │  Handler       │  │  Service       │  │   (Badger/   │  │
│  │                │  │                │  │    DGraph)   │  │
│  │ - Receives     │  │ - Monitors     │  │              │  │
│  │   proposals    │  │   proposals    │  │ - Stores     │  │
│  │ - Stores       │──│ - Computes     │──│   all FIND   │  │
│  │   events       │  │   consensus    │  │   events     │  │
│  │                │  │ - Publishes    │  │              │  │
│  │                │  │   name state   │  │              │  │
│  └────────────────┘  └────────────────┘  └──────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ Nostr Events
                            ▼
        ┌─────────────────────────────────────┐
        │    Clients & Other Registry Services │
        │                                       │
        │ - Query name state (kind 30102)      │
        │ - Query records (kind 30103)         │
        │ - Submit proposals (kind 30100)      │
        └─────────────────────────────────────┘
```

## How It Works

### Name Registration Flow

1. **User submits registration proposal**
   ```
   User → Create kind 30100 event → Publish to relay
   ```

2. **Relay stores proposal**
   ```
   Relay → Database → Store event
   ```

3. **Registry service processes proposal**
   ```
   Registry Service → Validate proposal
                    → Wait for attestation window (60-120s)
                    → Collect attestations from other services
                    → Compute trust-weighted consensus
   ```

4. **Consensus reached**
   ```
   Registry Service → Create name state (kind 30102)
                    → Publish to database
   ```

5. **Clients query ownership**
   ```
   Client → Query kind 30102 for name → Relay returns name state
   ```

### Name Resolution Flow

1. **Client queries name state**
   ```javascript
   // Query kind 30102 events with d tag = name
   const nameStates = await relay.list([{
     kinds: [30102],
     '#d': ['example.nostr']
   }])
   ```

2. **Client queries DNS records**
   ```javascript
   // Query kind 30103 events for records
   const records = await relay.list([{
     kinds: [30103],
     '#name': ['example.nostr'],
     '#type': ['A'],
     authors: [nameOwnerPubkey]
   }])
   ```

3. **Client uses resolved data**
   ```javascript
   // Extract IP addresses
   const ips = records.map(e =>
     e.tags.find(t => t[0] === 'value')[1]
   )
   // Connect to service at IP
   ```

## Event Types

| Kind | Name | Description | Persistence |
|------|------|-------------|-------------|
| 30100 | Registration Proposal | User submits name claim | Parameterized replaceable |
| 20100 | Attestation | Registry service votes | Ephemeral (3 min) |
| 30101 | Trust Graph | Service trust relationships | Parameterized replaceable (30 days) |
| 30102 | Name State | Current ownership | Parameterized replaceable (1 year) |
| 30103 | Name Records | DNS-style records | Parameterized replaceable (tied to name) |
| 30104 | Certificate | TLS-style certificates | Parameterized replaceable (90 days) |
| 30105 | Witness Service | Certificate witnesses | Parameterized replaceable (180 days) |

## Integration Status

### ✅ Completed

- [x] Consensus algorithm implementation
- [x] Trust graph calculation with multi-hop support
- [x] Registry service core logic
- [x] Event parsers for all FIND types
- [x] Event builders for creating FIND events
- [x] Validation functions (DNS names, IPs, etc.)
- [x] Implementation documentation
- [x] Client integration examples

### 🔨 Integration Points (Next Steps)

To complete the integration, the following work remains:

1. **Configuration** ([app/config/config.go](../app/config/config.go))
   ```go
   // Add these fields to config.C:
   FindEnabled           bool     `env:"ORLY_FIND_ENABLED" default:"false"`
   FindServicePubkey     string   `env:"ORLY_FIND_SERVICE_PUBKEY"`
   FindServicePrivkey    string   `env:"ORLY_FIND_SERVICE_PRIVKEY"`
   FindAttestationDelay  string   `env:"ORLY_FIND_ATTESTATION_DELAY" default:"60s"`
   FindBootstrapServices []string `env:"ORLY_FIND_BOOTSTRAP_SERVICES"`
   ```

2. **Database Query Helpers** ([pkg/database/](../pkg/database/))
   ```go
   // Add helper methods:
   func (d *Database) QueryNameState(name string) (*find.NameState, error)
   func (d *Database) QueryNameRecords(name, recordType string) ([]*find.NameRecord, error)
   func (d *Database) IsNameAvailable(name string) (bool, error)
   ```

3. **Server Integration** ([app/main.go](../app/main.go))
   ```go
   // Initialize registry service if enabled:
   if cfg.FindEnabled {
       registryService, err := find.NewRegistryService(ctx, db, signer, &find.RegistryConfig{
           Enabled: true,
           AttestationDelay: 60 * time.Second,
       })
       if err != nil {
           return err
       }
       if err := registryService.Start(); err != nil {
           return err
       }
       defer registryService.Stop()
   }
   ```

4. **HTTP API Endpoints** ([app/handle-find-api.go](../app/handle-find-api.go) - new file)
   ```go
   // Add REST endpoints:
   GET  /api/find/names/:name          // Query name state
   GET  /api/find/names/:name/records  // Query all records
   POST /api/find/register             // Submit proposal
   ```

5. **WebSocket Event Routing** ([app/handle-websocket.go](../app/handle-websocket.go))
   ```go
   // Route FIND events to registry service:
   if cfg.FindEnabled && registryService != nil {
       if ev.Kind >= 30100 && ev.Kind <= 30105 {
           registryService.HandleEvent(ev)
       }
   }
   ```

## Usage Examples

### Register a Name (Client)

```javascript
import { finalizeEvent, getPublicKey } from 'nostr-tools'

async function registerName(relay, privkey, name) {
  const pubkey = getPublicKey(privkey)

  // Create registration proposal
  const event = {
    kind: 30100,
    pubkey,
    created_at: Math.floor(Date.now() / 1000),
    tags: [
      ['d', name],
      ['action', 'register'],
      ['expiration', String(Math.floor(Date.now() / 1000) + 300)]
    ],
    content: ''
  }

  const signedEvent = finalizeEvent(event, privkey)
  await relay.publish(signedEvent)

  console.log('Proposal submitted, waiting for consensus...')

  // Wait 2 minutes for consensus
  await new Promise(r => setTimeout(r, 120000))

  // Check if registration succeeded
  const nameState = await relay.get({
    kinds: [30102],
    '#d': [name]
  })

  if (nameState && nameState.tags.find(t => t[0] === 'owner')[1] === pubkey) {
    console.log('Registration successful!')
    return true
  } else {
    console.log('Registration failed')
    return false
  }
}
```

### Publish DNS Records (Client)

```javascript
async function publishARecord(relay, privkey, name, ipAddress) {
  const pubkey = getPublicKey(privkey)

  // Verify we own the name first
  const nameState = await relay.get({
    kinds: [30102],
    '#d': [name]
  })

  if (!nameState || nameState.tags.find(t => t[0] === 'owner')[1] !== pubkey) {
    throw new Error('You do not own this name')
  }

  // Create A record
  const event = {
    kind: 30103,
    pubkey,
    created_at: Math.floor(Date.now() / 1000),
    tags: [
      ['d', `${name}:A:1`],
      ['name', name],
      ['type', 'A'],
      ['value', ipAddress],
      ['ttl', '3600']
    ],
    content: ''
  }

  const signedEvent = finalizeEvent(event, privkey)
  await relay.publish(signedEvent)

  console.log(`Published A record: ${name} → ${ipAddress}`)
}
```

### Resolve Name to IP (Client)

```javascript
async function resolveNameToIP(relay, name) {
  // 1. Get name state (ownership info)
  const nameState = await relay.get({
    kinds: [30102],
    '#d': [name]
  })

  if (!nameState) {
    throw new Error('Name not registered')
  }

  // Check if expired
  const expirationTag = nameState.tags.find(t => t[0] === 'expiration')
  if (expirationTag) {
    const expiration = parseInt(expirationTag[1])
    if (Date.now() / 1000 > expiration) {
      throw new Error('Name expired')
    }
  }

  const owner = nameState.tags.find(t => t[0] === 'owner')[1]

  // 2. Get A records
  const records = await relay.list([{
    kinds: [30103],
    '#name': [name],
    '#type': ['A'],
    authors: [owner]
  }])

  if (records.length === 0) {
    throw new Error('No A records found')
  }

  // 3. Extract IP addresses
  const ips = records.map(event => {
    return event.tags.find(t => t[0] === 'value')[1]
  })

  console.log(`${name} → ${ips.join(', ')}`)
  return ips
}
```

### Run Registry Service (Operator)

```bash
# Set environment variables
export ORLY_FIND_ENABLED=true
export ORLY_FIND_SERVICE_PUBKEY="your_service_pubkey_hex"
export ORLY_FIND_SERVICE_PRIVKEY="your_service_privkey_hex"
export ORLY_FIND_ATTESTATION_DELAY="60s"
export ORLY_FIND_BOOTSTRAP_SERVICES="pubkey1,pubkey2,pubkey3"

# Start relay
./orly
```

The registry service will:
- Monitor for registration proposals
- Validate proposals against rules
- Publish attestations for valid proposals
- Compute consensus with other services
- Publish name state events

## Key Features

### ✅ Implemented

1. **Trust-Weighted Consensus**
   - Services vote on proposals with weighted attestations
   - Multi-hop trust inheritance (0-3 hops)
   - Hop-based decay factors prevent infinite trust chains

2. **Renewal Window Enforcement**
   - Names expire after exactly 1 year
   - 30-day preferential renewal window for owners
   - Automatic expiration handling

3. **Subdomain Authority**
   - Only parent domain owners can register subdomains
   - TLDs can be registered by anyone (first-come-first-served)
   - Hierarchical ownership validation

4. **DNS-Compatible Records**
   - A, AAAA, CNAME, MX, TXT, NS, SRV record types
   - Per-type record limits
   - TTL-based caching

5. **Sparse Attestation**
   - Optional probabilistic attestation to reduce network load
   - Deterministic sampling based on proposal hash
   - Configurable sampling rates

### 🔮 Future Enhancements

1. **Certificate System** (Defined in NIP, not yet implemented)
   - Challenge-response verification
   - Threshold witnessing (3+ signatures)
   - TLS replacement capabilities

2. **Economic Incentives** (Designed but not implemented)
   - Optional registration fees via Lightning
   - Reputation scoring for registry services
   - Subscription models

3. **Advanced Features**
   - Noise protocol for secure transport
   - Browser integration
   - DNS gateway (traditional DNS → FIND)

## Testing

### Unit Tests

Run existing tests:
```bash
cd pkg/find
go test -v ./...
```

Tests cover:
- Name validation (validation_test.go)
- Parser functions (parser_test.go)
- Builder functions (builder_test.go)

### Integration Tests (To Be Added)

Recommended test scenarios:
1. **Single proposal registration**
2. **Competing proposals with consensus**
3. **Renewal window validation**
4. **Subdomain authority checks**
5. **Trust graph calculation**
6. **Multi-hop trust inheritance**

## Documentation

- **[Implementation Plan](FIND_IMPLEMENTATION_PLAN.md)** - Detailed architecture and phases
- **[NIP Specification](names.md)** - Complete protocol specification
- **[Usage Guide](FIND_USER_GUIDE.md)** - End-user documentation (to be created)
- **[Operator Guide](FIND_OPERATOR_GUIDE.md)** - Registry operator documentation (to be created)

## Security Considerations

### Attack Mitigations

1. **Sybil Attacks**: Trust-weighted consensus prevents new services from dominating
2. **Censorship**: Diverse trust graphs make network-wide censorship difficult
3. **Name Squatting**: Mandatory 1-year expiration with preferential renewal window
4. **Renewal DoS**: 30-day window, multiple retry opportunities
5. **Transfer Fraud**: Cryptographic signature from previous owner required

### Privacy Considerations

- Registration proposals are public (necessary for consensus)
- Ownership history is permanently visible on relays
- Clients can use Tor or private relays for sensitive queries

## Performance Characteristics

- **Registration Finality**: 1-2 minutes (60-120s attestation window)
- **Name Resolution**: < 100ms (database query)
- **Trust Calculation**: O(n) where n = number of services (with 3-hop limit)
- **Consensus Computation**: O(p×a) where p = proposals, a = attestations

## Support & Feedback

- **Issues**: https://github.com/orly-dev/orly/issues
- **Discussions**: https://github.com/orly-dev/orly/discussions
- **Nostr**: nostr:npub1... (relay operator npub)

## Next Steps

To complete the integration:

1. ✅ Review this summary
2. 🔨 Add configuration fields to config.C
3. 🔨 Implement database query helpers
4. 🔨 Integrate registry service in app/main.go
5. 🔨 Add HTTP API endpoints (optional)
6. 🔨 Write integration tests
7. 🔨 Create operator documentation
8. 🔨 Create user guide with examples

The core FIND protocol logic is complete and ready for integration!
