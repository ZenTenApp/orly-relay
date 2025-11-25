# Additional Requirements for WoT Implementation

This document identifies features and implementation details that are mentioned in the Brainstorm specification but lack detailed documentation. These items require further research, design decisions, or implementation details before the WoT system can be fully implemented in ORLY.

## 1. Algorithm Implementations

### 1.1 GrapeRank Algorithm

**Status:** Mentioned but not documented

**What's Specified:**
- Computes 4 metrics: `influence`, `average`, `input`, `confidence`
- Used to determine "verified" status (influence above threshold)
- Applied to social graph structure (FOLLOWS, MUTES, REPORTS)

**What's Missing:**
- [ ] Mathematical definition of the GrapeRank algorithm
- [ ] How influence is calculated from graph structure
- [ ] How average, input, and confidence are derived
- [ ] Convergence criteria and iteration limits
- [ ] Initialization values for new nodes
- [ ] Handling of disconnected components in the graph
- [ ] Edge weight calculations (are all FOLLOWS equal weight?)
- [ ] Integration of MUTES and REPORTS into the algorithm
- [ ] Parameter tuning (damping factors, iteration counts, etc.)

**Research Needed:**
- Review academic papers or source code for GrapeRank
- Determine if GrapeRank is a proprietary algorithm or based on existing graph algorithms
- Investigate whether it's related to PageRank, EigenTrust, or other trust propagation algorithms

**Implementation Questions:**
- Should this be implemented in Neo4j using Cypher queries or as an external computation?
- Can Neo4j's Graph Data Science library be used?
- How frequently should GrapeRank be recomputed?

### 1.2 Personalized PageRank

**Status:** Mentioned but not documented

**What's Specified:**
- Computes `personalizedPageRank` score for each user
- Personalized relative to an owner/observer node
- Uses FOLLOWS graph as link structure

**What's Missing:**
- [ ] Random walk restart probability (alpha parameter)
- [ ] Convergence tolerance
- [ ] Maximum iteration count
- [ ] Handling of dangling nodes (users with no outgoing FOLLOWS)
- [ ] Teleportation strategy (restart only to owner, or distributed?)
- [ ] Edge weight normalization
- [ ] Incremental update strategy when graph changes

**Implementation Questions:**
- Should we use Neo4j's built-in PageRank algorithm or implement custom Cypher?
- How to efficiently compute personalized PageRank for multiple observers?
- Can results be cached and updated incrementally?

### 1.3 Hops Calculation

**Status:** Partially specified

**What's Specified:**
- `hops` = distance from owner node via FOLLOWS relationships
- Used as a simpler alternative to PageRank

**What's Missing:**
- [ ] Handling of multiple paths (shortest path? all paths?)
- [ ] Maximum hop distance to compute (performance limit)
- [ ] Behavior for users unreachable from owner
- [ ] Update strategy when FOLLOWS relationships change

**Implementation Questions:**
- Use Cypher shortest path algorithm?
- Compute eagerly or lazily?
- Cache hop distances?

## 2. Event Processing Logic

### 2.1 Kind 3 (Contact List) Processing

**Status:** Mentioned but not fully specified

**What's Specified:**
- Creates/updates FOLLOWS relationships
- Source of social graph structure

**What's Missing:**
- [ ] Handling of replaceable event semantics (newer kind 3 replaces older)
- [ ] Should we delete old FOLLOWS relationships not in new list?
- [ ] Or only add new FOLLOWS relationships?
- [ ] Handling of relay hints in p-tags (ignore? store?)
- [ ] Petname support (3rd element of p-tag)
- [ ] Timestamp tracking on FOLLOWS relationships
- [ ] Event validation (signature verification, kind check)

**Implementation Questions:**
- Full replacement or incremental update?
- How to handle unfollow actions?
- Should FOLLOWS relationships have timestamps?

### 2.2 Kind 10000 (Mute List) Processing

**Status:** Mentioned but not fully specified

**What's Specified:**
- Creates/updates MUTES relationships
- Used in trust metrics computation

**What's Missing:**
- [ ] Same replaceable event handling questions as kind 3
- [ ] Handling of 'private' vs 'public' tags
- [ ] Support for encrypted mute lists
- [ ] Timestamp tracking
- [ ] Validation logic

**Implementation Questions:**
- Should mute lists be publicly visible in the graph?
- How to handle encrypted mute lists?

### 2.3 Kind 1984 (Reporting) Processing

**Status:** Partially specified

**What's Specified:**
- Creates REPORTS relationships
- Includes `reportType` property from NIP-56

**What's Missing:**
- [ ] Full enumeration of valid NIP-56 report types
- [ ] Parsing logic for report type from event tags
- [ ] Should multiple reports from same user create multiple edges or update one edge?
- [ ] Expiration/time-decay of reports
- [ ] Report validation (is reported pubkey in tags?)
- [ ] Support for reporting events (e-tags) vs users (p-tags)
- [ ] Handling of report reason/evidence fields

**Implementation Questions:**
- One REPORTS edge per report, or aggregate multiple reports?
- Should REPORTS edges have timestamps and decay over time?
- Store report evidence/reason in edge properties?

### 2.4 Kind 0 (Profile Metadata) Processing

**Status:** Mentioned but minimal detail

**What's Specified:**
- Updates NostrUser node properties (npub, name, etc.)

**What's Missing:**
- [ ] Which profile fields to store? (name, about, picture, nip05, etc.)
- [ ] Replaceable event handling
- [ ] Validation of profile data
- [ ] Size limits for profile fields
- [ ] Handling of malformed or malicious profile data

**Implementation Questions:**
- Store all profile fields as node properties?
- Or store profile JSON as single property?

### 2.5 Kind 30382 (Trusted Assertion - NIP-85) Processing

**Status:** Mentioned but no specification provided

**What's Specified:**
- Each NostrUserWotMetricsCard corresponds to a kind 30382 event
- Presumably used to publish trust metrics

**What's Missing:**
- [ ] Complete NIP-85 specification (link provided but not documented here)
- [ ] Event tag structure for trust metrics
- [ ] How trust metrics are encoded in the event
- [ ] Which metrics are published (all? subset?)
- [ ] Who creates these events? (relay owner? customers?)
- [ ] How to handle conflicts (multiple sources of trust metrics)
- [ ] Validation and signature verification
- [ ] Privacy considerations (publishing trust scores)

**Research Needed:**
- Review NIP-85 specification in detail
- Determine if ORLY should generate these events or only consume them

## 3. Multi-Tenant Support

### 3.1 Customer Management

**Status:** Mentioned but not specified

**What's Specified:**
- Support for multiple customers/observers
- Each customer gets their own NostrUserWotMetricsCard nodes
- `customer_id` field identifies customers

**What's Missing:**
- [ ] Customer registration/onboarding process
- [ ] Customer authentication
- [ ] Customer pubkey management (is customer_id == observer_pubkey?)
- [ ] API for customers to query their trust metrics
- [ ] Customer-specific configuration (threshold, max_hops, etc.)
- [ ] Rate limiting per customer
- [ ] Customer data isolation and privacy
- [ ] Billing/subscription model (if applicable)

**Implementation Questions:**
- Is this a paid service or open to all relay users?
- How do customers authenticate to query their metrics?
- REST API, WebSocket extension, or separate service?

### 3.2 Metric Computation Scheduling

**Status:** Not specified

**What's Missing:**
- [ ] When are trust metrics computed? (on-demand, periodic, triggered by events?)
- [ ] How often to recompute GrapeRank and PageRank?
- [ ] Full recomputation vs. incremental updates
- [ ] Priority system for computation (owner first, then customers?)
- [ ] Resource limits and queue management
- [ ] Handling of computation failures or timeouts
- [ ] Progress tracking and status reporting

**Implementation Questions:**
- Background job scheduler? (e.g., cron, queue system)
- Compute in relay process or separate service?
- How to handle computation for thousands of customers?

## 4. NIP-56 Report Types

### 4.1 Report Type Enumeration

**Status:** Mentioned with link to dashboard but not enumerated

**What's Specified:**
- Report types include: impersonation, spam, illegal, malware, nsfw
- Each type tracked separately in NostrUser properties
- Link to dashboard: https://straycat.brainstorm.social/nip56.html

**What's Missing:**
- [ ] Complete list of valid NIP-56 report types
- [ ] Standardized spelling/capitalization
- [ ] Mapping from event tags to report types
- [ ] Handling of unknown/custom report types
- [ ] Report type categories or groupings
- [ ] Deprecated or legacy report types

**Research Needed:**
- Review NIP-56 specification for canonical list
- Check Brainstorm dashboard for implementation-specific types

### 4.2 Report Type Data Model

**Status:** Under consideration

**What's Specified:**
- Current approach: Properties on NostrUser node (`{reportType}Count`, etc.)
- Acknowledged as potential "property explosion"
- Alternative: Separate nodes for NIP-56 metrics

**What's Missing:**
- [ ] Decision on data model approach
- [ ] If using separate nodes, what's the schema?
- [ ] Relationship types for report type nodes
- [ ] Query patterns for report type data
- [ ] Migration strategy if changing approach

**Design Question:**
- Keep as properties (simpler, faster queries) or separate nodes (more flexible, avoids explosion)?

## 5. Configuration and Deployment

### 5.1 Deployment Mode Selection

**Status:** Two modes described conceptually

**What's Specified:**
- "Lean mode": Minimal WoT for baseline trust metrics
- "Full relay mode": Comprehensive with event storage and additional relationships

**What's Missing:**
- [ ] Configuration flags to select mode
- [ ] Feature toggles for individual full-mode features
- [ ] Resource requirement specifications for each mode
- [ ] Performance benchmarks for each mode
- [ ] Migration path from lean to full mode
- [ ] Hybrid modes (some full features, not all)

**Implementation Questions:**
- Single binary with runtime configuration?
- Or separate builds for lean vs. full?

### 5.2 WoT Configuration Parameters

**Status:** Not specified

**What's Missing:**
- [ ] Influence threshold for "verified" status (default? per-customer?)
- [ ] Maximum hops to compute (performance vs. coverage tradeoff)
- [ ] GrapeRank parameters (damping, iterations, etc.)
- [ ] PageRank parameters (alpha, tolerance, iterations)
- [ ] Metric update frequency (how often to recompute?)
- [ ] Graph pruning rules (remove inactive users?)
- [ ] Memory and performance limits

**Suggested Environment Variables:**
```bash
ORLY_WOT_ENABLED=true
ORLY_WOT_MODE=lean|full
ORLY_WOT_OWNER_PUBKEY=<hex>
ORLY_WOT_INFLUENCE_THRESHOLD=0.5
ORLY_WOT_MAX_HOPS=3
ORLY_WOT_GRAPERANK_ITERATIONS=100
ORLY_WOT_PAGERANK_ALPHA=0.85
ORLY_WOT_UPDATE_INTERVAL=1h
ORLY_WOT_MULTI_TENANT=false
```

## 6. Query Extensions

### 6.1 REQ Filter Extensions

**Status:** Example provided but not fully specified

**Example from spec:**
```json
{
  "kinds": [1],
  "wot": {
    "max_hops": 2,
    "min_influence": 0.5,
    "observer": "<pubkey>"
  }
}
```

**What's Missing:**
- [ ] Complete specification of `wot` filter syntax
- [ ] Filtering by verified counts
- [ ] Filtering by report status (exclude reported users)
- [ ] Filtering by mute status
- [ ] Combining multiple WoT filters (AND, OR logic)
- [ ] Support in existing filter parsing code
- [ ] Translation to Cypher queries
- [ ] Performance implications
- [ ] Error handling for invalid WoT filters

**Implementation Questions:**
- Should WoT filters be part of standard Nostr filter or extension?
- How to handle clients that don't understand WoT filters?
- Return empty results or ignore WoT parameters?

### 6.2 Trust Metrics Query API

**Status:** Not specified

**What's Missing:**
- [ ] API endpoint for querying trust metrics
- [ ] Request/response format
- [ ] Batch queries (multiple users)
- [ ] Filtering and sorting options
- [ ] Pagination for large result sets
- [ ] Authentication and authorization
- [ ] Rate limiting
- [ ] Caching strategy

**Suggested API:**
```
GET /api/wot/metrics?observer=<pubkey>&observee=<pubkey>
GET /api/wot/metrics?observer=<pubkey>&min_influence=0.5&limit=100
POST /api/wot/metrics/batch (with list of observee pubkeys)
```

## 7. Full Relay Mode Features

### 7.1 Additional Relationship Types

**Status:** Mentioned but not specified

**What's Specified:**
- `IS_A_REACTION_TO` (kind 7 reactions)
- `IS_A_RESPONSE_TO` (kind 1 replies)
- `IS_A_REPOST_OF` (kind 6, kind 16 reposts)
- `P_TAGGED` (p-tag mentions)
- `E_TAGGED` (e-tag references)

**What's Missing:**
- [ ] Schema for each relationship type
- [ ] Processing logic for each event kind
- [ ] How these relationships affect trust metrics
- [ ] Query patterns using these relationships
- [ ] Performance implications of storing all events
- [ ] Data retention and pruning strategies

### 7.2 NostrEvent Nodes

**Status:** Mentioned but not specified

**What's Missing:**
- [ ] Schema for NostrEvent nodes
- [ ] Which events to store as nodes (all kinds? subset?)
- [ ] Relationship to existing Event nodes in base ORLY schema
- [ ] Migration from base schema to full relay schema
- [ ] Query patterns for event-based relationships
- [ ] Storage optimization for large event graphs

### 7.3 Ecosystem Nodes

**Status:** Mentioned but not specified

**What's Specified:**
- NostrRelay nodes
- CashuMint nodes

**What's Missing:**
- [ ] Schema for these node types
- [ ] Purpose and use cases
- [ ] How they integrate with WoT metrics
- [ ] Data sources for these nodes
- [ ] Relationship types to other nodes

### 7.4 Enhanced Trust Metrics

**Status:** Mentioned but not specified

**What's Specified:**
- Incorporate zaps into trust metrics
- Incorporate replies and reactions into trust metrics

**What's Missing:**
- [ ] How zaps affect influence calculations
- [ ] Weight of zaps vs. follows in trust scoring
- [ ] Handling of zap amounts (larger zaps = more weight?)
- [ ] How replies and reactions are weighted
- [ ] Preventing gaming/manipulation of metrics
- [ ] Sybil attack resistance

## 8. Performance and Scalability

### 8.1 Graph Size Limits

**Status:** Example given (300k tracked users out of millions)

**What's Missing:**
- [ ] Hard limits on node/relationship counts
- [ ] Performance degradation curves
- [ ] Memory usage projections
- [ ] Disk space requirements
- [ ] Neo4j heap and pagecache tuning
- [ ] Sharding or partitioning strategies for very large graphs

### 8.2 Query Performance

**Status:** Not specified

**What's Missing:**
- [ ] Query time SLAs/targets
- [ ] Slow query identification and optimization
- [ ] Index tuning strategy
- [ ] Caching layer for frequently accessed metrics
- [ ] Query result pagination and cursors
- [ ] Monitoring and alerting for performance issues

### 8.3 Incremental Updates

**Status:** Mentioned as preferred approach

**What's Missing:**
- [ ] Algorithm for incremental GrapeRank updates
- [ ] Algorithm for incremental PageRank updates
- [ ] When to trigger incremental vs. full recomputation
- [ ] Handling of cascading updates (one change affects many nodes)
- [ ] Correctness guarantees for incremental updates
- [ ] Testing strategy for incremental vs. full computation equivalence

## 9. Security and Privacy

### 9.1 Privacy Considerations

**Status:** Not addressed

**What's Missing:**
- [ ] Privacy implications of publishing trust metrics
- [ ] User consent for trust metric computation
- [ ] Anonymization or aggregation of sensitive metrics
- [ ] GDPR compliance (right to be forgotten, data export)
- [ ] Encryption of sensitive graph data
- [ ] Access control for trust metric queries

### 9.2 Attack Resistance

**Status:** Not addressed

**What's Missing:**
- [ ] Sybil attack detection and mitigation
- [ ] Graph manipulation detection (fake follows, spam reports)
- [ ] Rate limiting on relationship creation
- [ ] Honeypot/trap accounts
- [ ] Adversarial testing procedures
- [ ] Recovery from successful attacks

### 9.3 Data Validation

**Status:** Minimal specification

**What's Missing:**
- [ ] Event signature verification
- [ ] Pubkey format validation
- [ ] Tag structure validation
- [ ] Duplicate detection
- [ ] Malformed data handling
- [ ] Logging and alerting for validation failures

## 10. Testing and Validation

### 10.1 Test Data

**Status:** Not specified

**What's Missing:**
- [ ] Sample graph data for testing
- [ ] Expected trust metric values for test data
- [ ] Test cases for edge cases (disconnected graphs, cycles, etc.)
- [ ] Performance benchmarks with realistic graph sizes
- [ ] Stress tests for large graph operations

### 10.2 Validation

**Status:** Not specified

**What's Missing:**
- [ ] How to validate correctness of GrapeRank implementation
- [ ] How to validate correctness of PageRank implementation
- [ ] Regression testing for metric changes
- [ ] Comparison with reference implementations (Brainstorm, others)
- [ ] Monitoring and alerting for anomalous metric values

## 11. Migration and Compatibility

### 11.1 Migration from Base Schema

**Status:** Not addressed

**What's Missing:**
- [ ] Migration path from existing ORLY Neo4j backend
- [ ] Backward compatibility with existing Event/Author schema
- [ ] Data migration scripts
- [ ] Downtime requirements
- [ ] Rollback procedures

### 11.2 Interoperability

**Status:** Not addressed

**What's Missing:**
- [ ] Compatibility with standard Nostr clients (ignore WoT filters gracefully)
- [ ] Import/export of trust metrics in standard format
- [ ] Federation of trust metrics across multiple relays
- [ ] Integration with existing WoT implementations (Brainstorm, others)

## 12. Documentation and Examples

### 12.1 User Documentation

**Status:** Minimal

**What's Missing:**
- [ ] User guide for relay operators
- [ ] Configuration guide with examples
- [ ] Troubleshooting guide
- [ ] Performance tuning guide
- [ ] FAQ

### 12.2 Developer Documentation

**Status:** Minimal

**What's Missing:**
- [ ] Architecture documentation
- [ ] Code structure and module organization
- [ ] API documentation (trust metrics query API)
- [ ] Contributing guide
- [ ] Testing guide

### 12.3 Example Queries

**Status:** Some examples in spec

**What's Missing:**
- [ ] More comprehensive query examples
- [ ] Query cookbook for common use cases
- [ ] Performance notes for each query pattern
- [ ] Cypher query optimization tips

## Prioritization Recommendations

### Phase 1: Core WoT (Minimal Viable Product)
1. Hops calculation (simpler than PageRank)
2. Kind 3 (follows) processing
3. NostrUser node creation and management
4. Basic query filtering by hops
5. Configuration system for owner pubkey and max hops

### Phase 2: Trust Metrics
1. GrapeRank algorithm implementation (research and adapt)
2. Personalized PageRank implementation
3. Verified count calculations
4. Kind 10000 (mutes) and kind 1984 (reports) processing
5. WoT filter extension for REQ queries

### Phase 3: Multi-Tenant
1. NostrUserWotMetricsCard node creation
2. Customer management system
3. Trust metrics API
4. Per-customer metric computation
5. NIP-85 Trusted Assertion generation

### Phase 4: Full Relay Mode
1. Additional relationship types
2. NostrEvent nodes
3. Enhanced trust metrics with zaps/replies
4. Ecosystem nodes (relays, mints)

## Summary

This document identifies **50+ specific implementation details** that are mentioned in the Brainstorm specification but lack sufficient detail for implementation. The most critical missing pieces are:

1. **Algorithm implementations** (GrapeRank, PageRank) - requires research or reverse engineering
2. **Event processing logic** - requires detailed design for each event kind
3. **Multi-tenant architecture** - requires customer management system design
4. **NIP-56 and NIP-85 integration** - requires NIP specification review
5. **Configuration system** - requires parameter identification and default values
6. **Query API** - requires API design and authentication model
7. **Performance optimization** - requires benchmarking and tuning
8. **Testing strategy** - requires test data and validation methodology

These areas should be addressed systematically to build a complete WoT implementation for ORLY.
