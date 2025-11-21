# FIND Name Binding Implementation Plan

## Overview

This document outlines the implementation plan for integrating the Free Internet Name Daemon (FIND) protocol with the ORLY relay. The FIND protocol provides decentralized name-to-npub bindings that are discoverable by any client using standard Nostr queries.

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                        ORLY Relay                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   WebSocket  │  │  FIND Daemon │  │   HTTP API       │  │
│  │   Handler    │  │  (Registry   │  │   (NIP-11, Web)  │  │
│  │              │  │   Service)   │  │                  │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │             │
│         └─────────────────┼────────────────────┘             │
│                          │                                   │
│                  ┌───────▼────────┐                          │
│                  │   Database     │                          │
│                  │   (Badger/     │                          │
│                  │    DGraph)     │                          │
│                  └────────────────┘                          │
└─────────────────────────────────────────────────────────────┘
         │                                        ▲
         │  Publish FIND events                   │  Query FIND events
         │  (kinds 30100-30105)                   │  (kinds 30102, 30103)
         ▼                                        │
┌─────────────────────────────────────────────────────────────┐
│                     Nostr Network                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Other      │  │   Other      │  │    Clients       │  │
│  │   Relays     │  │   Registry   │  │                  │  │
│  │              │  │   Services   │  │                  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Event Flow

1. **Name Registration:**
   ```
   User → FIND CLI → Registration Proposal (kind 30100) → Relay → Database
                                                           ↓
                                          Registry Service (attestation)
                                                           ↓
                                          Attestation (kind 20100) → Other Registry Services
                                                           ↓
                                          Consensus → Name State (kind 30102)
   ```

2. **Name Resolution:**
   ```
   Client → Query kind 30102 (name state) → Relay → Database → Response
   Client → Query kind 30103 (records) → Relay → Database → Response
   ```

## Implementation Phases

### Phase 1: Database Storage for FIND Events ✓ (Already Supported)

The relay already stores all parameterized replaceable events (kind 30xxx) and ephemeral events (kind 20xxx), which includes all FIND event types:

- ✓ Kind 30100: Registration Proposals
- ✓ Kind 20100: Attestations (ephemeral)
- ✓ Kind 30101: Trust Graphs
- ✓ Kind 30102: Name State
- ✓ Kind 30103: Name Records
- ✓ Kind 30104: Certificates
- ✓ Kind 30105: Witness Services

**Status:** No changes needed. The existing event storage system handles these automatically.

### Phase 2: Registry Service Implementation

Create a new registry service that runs within the ORLY relay process (optional, can be enabled via config).

**New Files:**
- `pkg/find/registry.go` - Core registry service
- `pkg/find/consensus.go` - Consensus algorithm implementation
- `pkg/find/trust.go` - Trust graph calculation
- `app/find-service.go` - Integration with relay server

**Key Components:**

```go
// Registry service that monitors proposals and computes consensus
type RegistryService struct {
    db              database.Database
    pubkey          []byte // Registry service identity
    trustGraph      *TrustGraph
    pendingProposals map[string]*ProposalState
    config          *RegistryConfig
}

type RegistryConfig struct {
    Enabled             bool
    ServicePubkey       string
    AttestationDelay    time.Duration // Default: 60s
    SparseAttestation   bool
    SamplingRate        int // For sparse attestation
}

// Proposal state tracking during attestation window
type ProposalState struct {
    Proposal      *RegistrationProposal
    Attestations  []*Attestation
    ReceivedAt    time.Time
    ProcessedAt   *time.Time
}
```

**Responsibilities:**
1. Subscribe to kind 30100 (registration proposals) from database
2. Validate proposals (name format, ownership, renewal window)
3. Check for conflicts (competing proposals)
4. After attestation window (60-120s):
   - Fetch attestations (kind 20100) from other registry services
   - Compute trust-weighted consensus
   - Publish name state (kind 30102) if consensus reached

### Phase 3: Client Query Handlers

Enhance existing query handlers to optimize FIND event queries.

**Enhancements:**
- Add specialized indexes for FIND events (already exists via `d` tag indexes)
- Implement name resolution helper functions
- Cache frequently queried name states

**New Helper Functions:**

```go
// Query name state for a given name
func (d *Database) QueryNameState(name string) (*find.NameState, error)

// Query all records for a name
func (d *Database) QueryNameRecords(name string, recordType string) ([]*find.NameRecord, error)

// Check if name is available for registration
func (d *Database) IsNameAvailable(name string) (bool, error)

// Get parent domain owner (for subdomain validation)
func (d *Database) GetParentDomainOwner(name string) (string, error)
```

### Phase 4: Configuration Integration

Add FIND-specific configuration options to `app/config/config.go`:

```go
type C struct {
    // ... existing fields ...

    // FIND registry service settings
    FindEnabled           bool     `env:"ORLY_FIND_ENABLED" default:"false" usage:"enable FIND registry service for name consensus"`
    FindServicePubkey     string   `env:"ORLY_FIND_SERVICE_PUBKEY" usage:"public key for this registry service (hex)"`
    FindServicePrivkey    string   `env:"ORLY_FIND_SERVICE_PRIVKEY" usage:"private key for signing attestations (hex)"`
    FindAttestationDelay  string   `env:"ORLY_FIND_ATTESTATION_DELAY" default:"60s" usage:"delay before publishing attestations"`
    FindSparseEnabled     bool     `env:"ORLY_FIND_SPARSE_ENABLED" default:"false" usage:"use sparse attestation (probabilistic)"`
    FindSamplingRate      int      `env:"ORLY_FIND_SAMPLING_RATE" default:"10" usage:"sampling rate for sparse attestation (1/K)"`
    FindBootstrapServices []string `env:"ORLY_FIND_BOOTSTRAP_SERVICES" usage:"comma-separated list of bootstrap registry service pubkeys"`
}
```

### Phase 5: FIND Daemon HTTP API

Add HTTP API endpoints for FIND operations (optional, for user convenience):

**New Endpoints:**
- `GET /api/find/names/:name` - Query name state
- `GET /api/find/names/:name/records` - Query all records for a name
- `GET /api/find/names/:name/records/:type` - Query specific record type
- `POST /api/find/register` - Submit registration proposal
- `POST /api/find/transfer` - Submit transfer proposal
- `GET /api/find/trust-graph` - Query this relay's trust graph

**Implementation:**
```go
// app/handle-find-api.go
func (s *Server) handleFindNameQuery(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")

    // Validate name format
    if err := find.ValidateName(name); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Query name state from database
    nameState, err := s.DB.QueryNameState(name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if nameState == nil {
        http.Error(w, "name not found", http.StatusNotFound)
        return
    }

    // Return as JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(nameState)
}
```

### Phase 6: Client Integration Examples

Provide example code for clients to use FIND:

**Example: Query name ownership**
```javascript
// JavaScript/TypeScript example using nostr-tools
import { SimplePool } from 'nostr-tools'

async function queryNameOwner(relays, name) {
  const pool = new SimplePool()

  // Query kind 30102 events with d tag = name
  const events = await pool.list(relays, [{
    kinds: [30102],
    '#d': [name],
    limit: 5
  }])

  if (events.length === 0) {
    return null // Name not registered
  }

  // Check for majority consensus among registry services
  const ownerCounts = {}
  for (const event of events) {
    const ownerTag = event.tags.find(t => t[0] === 'owner')
    if (ownerTag) {
      const owner = ownerTag[1]
      ownerCounts[owner] = (ownerCounts[owner] || 0) + 1
    }
  }

  // Return owner with most attestations
  let maxCount = 0
  let consensusOwner = null
  for (const [owner, count] of Object.entries(ownerCounts)) {
    if (count > maxCount) {
      maxCount = count
      consensusOwner = owner
    }
  }

  return consensusOwner
}

// Example: Resolve name to IP address
async function resolveNameToIP(relays, name) {
  const owner = await queryNameOwner(relays, name)
  if (!owner) {
    throw new Error('Name not registered')
  }

  // Query kind 30103 events for A records
  const pool = new SimplePool()
  const records = await pool.list(relays, [{
    kinds: [30103],
    '#name': [name],
    '#type': ['A'],
    authors: [owner], // Only records from name owner are valid
    limit: 5
  }])

  if (records.length === 0) {
    throw new Error('No A records found')
  }

  // Extract IP addresses from value tags
  const ips = records.map(event => {
    const valueTag = event.tags.find(t => t[0] === 'value')
    return valueTag ? valueTag[1] : null
  }).filter(Boolean)

  return ips
}
```

**Example: Register a name**
```javascript
import { finalizeEvent, getPublicKey } from 'nostr-tools'
import { find } from './find-helpers'

async function registerName(relays, privkey, name) {
  // Validate name format
  if (!find.validateName(name)) {
    throw new Error('Invalid name format')
  }

  const pubkey = getPublicKey(privkey)

  // Create registration proposal (kind 30100)
  const event = {
    kind: 30100,
    created_at: Math.floor(Date.now() / 1000),
    tags: [
      ['d', name],
      ['action', 'register'],
      ['expiration', String(Math.floor(Date.now() / 1000) + 300)] // 5 min expiry
    ],
    content: ''
  }

  const signedEvent = finalizeEvent(event, privkey)

  // Publish to relays
  const pool = new SimplePool()
  await Promise.all(relays.map(relay => pool.publish(relay, signedEvent)))

  // Wait for consensus (typically 1-2 minutes)
  console.log('Registration proposal submitted. Waiting for consensus...')
  await new Promise(resolve => setTimeout(resolve, 120000))

  // Check if registration succeeded
  const owner = await queryNameOwner(relays, name)
  if (owner === pubkey) {
    console.log('Registration successful!')
    return true
  } else {
    console.log('Registration failed - another proposal may have won consensus')
    return false
  }
}
```

## Testing Plan

### Unit Tests

1. **Name Validation Tests** (`pkg/find/validation_test.go` - already exists)
   - Valid names
   - Invalid names (too long, invalid characters, etc.)
   - Subdomain authority validation

2. **Consensus Algorithm Tests** (`pkg/find/consensus_test.go` - new)
   - Single proposal scenario
   - Competing proposals
   - Trust-weighted scoring
   - Attestation window expiry

3. **Trust Graph Tests** (`pkg/find/trust_test.go` - new)
   - Direct trust relationships
   - Multi-hop trust inheritance
   - Trust decay calculation

### Integration Tests

1. **End-to-End Registration** (`pkg/find/integration_test.go` - new)
   - Submit proposal
   - Generate attestations
   - Compute consensus
   - Verify name state

2. **Name Renewal** (`pkg/find/renewal_test.go` - new)
   - Renewal during preferential window
   - Rejection outside renewal window
   - Expiration handling

3. **Record Management** (`pkg/find/records_test.go` - new)
   - Publish DNS-style records
   - Verify owner authorization
   - Query records by type

### Performance Tests

1. **Concurrent Proposals** - Benchmark handling 1000+ simultaneous proposals
2. **Trust Graph Calculation** - Test with 10,000+ registry services
3. **Query Performance** - Measure name resolution latency

## Deployment Strategy

### Development Phase
1. Implement core registry service (Phase 2)
2. Add unit tests
3. Test with local relay and simulated registry services

### Testnet Phase
1. Deploy 5-10 test relays with FIND enabled
2. Simulate various attack scenarios (Sybil, censorship, etc.)
3. Tune consensus parameters based on results

### Production Rollout
1. Documentation and client libraries
2. Enable FIND on select relays (opt-in)
3. Monitor for issues and gather feedback
4. Gradual adoption across relay network

## Security Considerations

### Attack Mitigations

1. **Sybil Attacks**
   - Trust-weighted consensus prevents new services from dominating
   - Age-weighted trust (new services have reduced influence)

2. **Censorship**
   - Diverse trust graphs make network-wide censorship difficult
   - Users can query different registry services aligned with their values

3. **Name Squatting**
   - Mandatory 1-year expiration
   - Preferential 30-day renewal window
   - No indefinite holding

4. **Renewal Window DoS**
   - 30-day window reduces attack surface
   - Owner can submit multiple renewal attempts
   - Registry services filter by pubkey during renewal window

### Privacy Considerations

- Registration proposals are public (necessary for consensus)
- Ownership history is permanently visible
- Clients can use Tor or private relays for sensitive queries

## Documentation Updates

1. **User Guide** (`docs/FIND_USER_GUIDE.md` - new)
   - How to register a name
   - How to manage DNS records
   - How to renew registrations
   - Client integration examples

2. **Operator Guide** (`docs/FIND_OPERATOR_GUIDE.md` - new)
   - How to enable FIND registry service
   - Trust graph configuration
   - Monitoring and troubleshooting
   - Bootstrap recommendations

3. **Developer Guide** (`docs/FIND_DEVELOPER_GUIDE.md` - new)
   - API reference
   - Client library examples (JS, Python, Go)
   - Event schemas and validation
   - Consensus algorithm details

4. **Update CLAUDE.md**
   - Add FIND sections to project overview
   - Document new configuration options
   - Add testing instructions

## Success Metrics

- **Registration Finality:** < 2 minutes for 95% of registrations
- **Query Latency:** < 100ms for name lookups
- **Consensus Agreement:** > 99% agreement among honest registry services
- **Uptime:** Registry service availability > 99.9%
- **Adoption:** 100+ registered names within first month of testnet

## Future Enhancements

1. **Economic Incentives** - Optional registration fees via Lightning
2. **Reputation System** - Track registry service quality metrics
3. **Certificate System** - Implement NIP-XX certificate witnessing
4. **Noise Protocol** - Secure transport layer for TLS replacement
5. **Client Libraries** - Official libraries for popular languages
6. **Browser Integration** - Browser extension for name resolution
7. **DNS Gateway** - Traditional DNS server that queries FIND
