# Web of Trust (WoT) Data Model Specification for Neo4j

This document describes the Web of Trust graph data model extensions for the ORLY Neo4j backend, based on the [Brainstorm prototype](https://straycat.brainstorm.social).

## Overview

The WoT data model extends the base Nostr relay functionality with trust metrics computation using graph algorithms (GrapeRank, Personalized PageRank) to enable:

- **Social graph-based filtering**: Filter events based on web of trust relationships
- **Personalized trust scores**: Compute trust metrics personalized to each user/customer
- **Multi-tenant support**: Track separate trust metrics for multiple customers/observers
- **Spam and moderation**: Use social graph signals (follows, mutes, reports) for content filtering

## Reference Implementation

- **Live instance**: https://straycat.brainstorm.social (32 GB RAM, 8 vCPU, 100 GB SSD)
- **Repository**: https://github.com/Pretty-Good-Freedom-Tech/brainstorm
- **Neo4j browser**: http://straycat.brainstorm.social:7474/browser/
- **Relay**: https://straycat.brainstorm.social/relay

## Data Model Architecture

The WoT model adds specialized nodes and relationships to track social graph structure and compute trust metrics.

### Node Labels

#### 1. NostrUser

Represents a Nostr user (identified by pubkey) with computed trust metrics.

**Properties:**
- `pubkey` (string, unique) - Hex-encoded public key
- `npub` (string) - Bech32-encoded npub

**Trust Metrics (Owner-Personalized):**
- `hops` (integer) - Distance from owner node via FOLLOWS relationships
- `personalizedPageRank` (float) - PageRank score personalized to owner
- `influence` (float) - GrapeRank influence score
- `average` (float) - GrapeRank average score
- `input` (float) - GrapeRank input score
- `confidence` (float) - GrapeRank confidence score

**Social Graph Counts:**
- `followingCount` (integer) - Total number of users this user follows
- `followedByCount` (integer) - Total number of followers
- `mutingCount` (integer) - Total number of users this user mutes
- `mutedByCount` (integer) - Total number of users who mute this user
- `reportingCount` (integer) - Total number of reports filed by this user
- `reportedByCount` (integer) - Total number of reports filed against this user

**Verified Counts (GrapeRank-weighted):**
- `verifiedFollowerCount` (integer) - Count of followers with influence above threshold
- `verifiedMuterCount` (integer) - Count of muters with influence above threshold
- `verifiedReporterCount` (integer) - Count of reporters with influence above threshold

**Input Scores (Sum of Influence):**
- `followerInput` (float) - Sum of influence scores of all followers
- `muterInput` (float) - Sum of influence scores of all muters
- `reporterInput` (float) - Sum of influence scores of all reporters

**NIP-56 Report Types:**

For each report type (impersonator, spam, illegal, malware, nsfw, etc.), the following metrics are tracked:
- `{reportType}Count` (integer) - Total count of this report type
- `{reportType}VerifiedCount` (integer) - Count from verified reporters
- `{reportType}Input` (float) - Sum of influence scores of reporters

Note: NIP-56 metrics may be better modeled as separate nodes to avoid property explosion.

**Indexes:**
- Unique constraint on `pubkey`
- Index on `hops`
- Index on `personalizedPageRank`
- Index on `influence`
- Index on `verifiedFollowerCount`
- Index on `verifiedMuterCount`
- Index on `verifiedReporterCount`
- Index on `followerInput`

#### 2. SetOfNostrUserWotMetricsCards

Organizational node that groups all WoT metric cards for a single observee (user being scored). This design pattern keeps WoT metric cards partitioned from other NostrUser relationships.

**Properties:**
- `observee_pubkey` (string, unique) - Pubkey of the user being scored

**Purpose:** Acts as an intermediary to minimize direct relationships on NostrUser nodes, which may have many other relationships in a full relay implementation.

**Indexes:**
- Unique constraint on `observee_pubkey`

#### 3. NostrUserWotMetricsCard

Stores personalized trust metrics for a specific (observer, observee) pair. Each card corresponds to a NIP-85 Trusted Assertion (kind 30382) event.

**Properties:**
- `customer_id` (string) - Identifier for the customer/service instance
- `observer_pubkey` (string) - Pubkey of the observer (the customer)
- `observee_pubkey` (string) - Pubkey of the user being scored

**Trust Metrics (Observer-Personalized):**
All the same metrics as NostrUser node, but personalized to the observer:
- `hops`, `personalizedPageRank`
- `influence`, `average`, `input`, `confidence`
- `verifiedFollowerCount`, `verifiedMuterCount`, `verifiedReporterCount`
- `followerInput`, `muterInput`, `reporterInput`

**Indexes:**
- Unique constraint on `(customer_id, observee_pubkey)`
- Unique constraint on `(observer_pubkey, observee_pubkey)`
- Index on `customer_id`
- Index on `observer_pubkey`
- Index on `observee_pubkey`
- Index on `hops`
- Index on `personalizedPageRank`
- Index on `influence`
- Index on `verifiedFollowerCount`
- Index on `verifiedMuterCount`
- Index on `verifiedReporterCount`
- Index on `followerInput`

#### 4. Set (Deprecated)

Legacy node label that is redundant with SetOfNostrUserWotMetricsCards. Should be removed in new implementations.

### Relationship Types

#### 1. FOLLOWS

Represents a follow relationship between users (derived from kind 3 events).

**Direction:** `(follower:NostrUser)-[:FOLLOWS]->(followed:NostrUser)`

**Properties:** None (or optionally timestamp)

**Source:** Created/updated from kind 3 (contact list) events

#### 2. MUTES

Represents a mute relationship between users (derived from kind 10000 events).

**Direction:** `(muter:NostrUser)-[:MUTES]->(muted:NostrUser)`

**Properties:** None (or optionally timestamp)

**Source:** Created/updated from kind 10000 (mute list) events

#### 3. REPORTS

Represents a report filed against a user (derived from kind 1984 events).

**Direction:** `(reporter:NostrUser)-[:REPORTS]->(reported:NostrUser)`

**Properties:**
- `reportType` (string) - NIP-56 report type (impersonation, spam, illegal, malware, nsfw, etc.)
- `timestamp` (integer) - When the report was filed

**Source:** Created from kind 1984 (reporting) events

#### 4. WOT_METRICS_CARDS

Links a NostrUser to their SetOfNostrUserWotMetricsCards organizational node.

**Direction:** `(user:NostrUser)-[:WOT_METRICS_CARDS]->(set:SetOfNostrUserWotMetricsCards)`

**Properties:** None

**Cardinality:** One-to-one (each NostrUser has at most one SetOfNostrUserWotMetricsCards)

#### 5. SPECIFIC_INSTANCE

Links a SetOfNostrUserWotMetricsCards to individual NostrUserWotMetricsCard nodes for each observer.

**Direction:** `(set:SetOfNostrUserWotMetricsCards)-[:SPECIFIC_INSTANCE]->(card:NostrUserWotMetricsCard)`

**Properties:** None

**Cardinality:** One-to-many (one set has many cards, one per observer)

**Note:** May be renamed to `WOT_METRICS_CARD` for clarity.

## Nostr Event Kinds

The WoT model processes the following Nostr event kinds:

| Kind | Name | Purpose | Graph Action |
|------|------|---------|--------------|
| 0 | Profile Metadata | User profile information | Update NostrUser properties (npub, name, etc.) |
| 3 | Contact List | Follow list | Create/update FOLLOWS relationships |
| 1984 | Reporting | Report users/content | Create REPORTS relationships with reportType |
| 10000 | Mute List | Mute list | Create/update MUTES relationships |
| 30382 | Trusted Assertion (NIP-85) | Published trust metrics | Create/update NostrUserWotMetricsCard nodes |

## Trust Metrics Computation

### User Tracking Criteria

Trust metrics are computed for users who meet any of these criteria:
1. Connected to the owner/observer by a finite number of FOLLOWS relationships (e.g., within N hops)
2. Muted by a trusted user (user with sufficient influence)
3. Reported by a trusted user

This typically results in ~300k tracked users out of millions in the network.

### GrapeRank Algorithm

GrapeRank is a trust scoring algorithm that computes:
- **Influence**: Primary trust score based on social graph structure
- **Average**: Average trust received from neighbors
- **Input**: Total trust input from all connections
- **Confidence**: Confidence level in the score

**Note:** Implementation details for GrapeRank are not included in the specification.

### Personalized PageRank

Computes a personalized PageRank score for each user relative to an owner/observer, using the FOLLOWS graph as the link structure.

**Note:** Implementation details are not included in the specification.

### Verified Counts

Users with `influence` above a configurable threshold are considered "verified" for counting purposes. This provides a quality-weighted count of followers/muters/reporters.

### Input Scores

Alternative to verified counts: sum the influence scores of all followers/muters/reporters to get a weighted measure of social signals.

## Deployment Modes

### Lean Mode (Baseline)

Minimal WoT implementation suitable for resource-constrained deployments:
- NostrUser, NostrUserWotMetricsCard, SetOfNostrUserWotMetricsCards nodes
- FOLLOWS, MUTES, REPORTS, WOT_METRICS_CARDS, SPECIFIC_INSTANCE relationships
- Process kinds: 0, 3, 1984, 10000
- Compute baseline trust metrics

**Hardware:** Can run on smaller instances (e.g., 8 GB RAM, 2 vCPU)

### Full Relay Mode (Extended)

Comprehensive implementation with additional features:
- All lean mode features
- NostrEvent nodes with full event storage
- Additional relationships:
  - `IS_A_REACTION_TO` (kind 7 reactions)
  - `IS_A_RESPONSE_TO` (kind 1 replies)
  - `IS_A_REPOST_OF` (kind 6, kind 16 reposts)
  - `P_TAGGED` (p-tag mentions from events to users)
  - `E_TAGGED` (e-tag references from events to events)
- NostrRelay, CashuMint nodes for ecosystem mapping
- Enhanced GrapeRank incorporating zaps, replies, reactions

**Hardware:** Requires larger instances (e.g., 32 GB RAM, 8 vCPU, 100+ GB SSD)

## Cypher Schema Definitions

```cypher
-- NostrUser node constraint and indexes
CREATE CONSTRAINT nostrUser_pubkey IF NOT EXISTS
  FOR (n:NostrUser) REQUIRE n.pubkey IS UNIQUE;

CREATE INDEX nostrUser_hops IF NOT EXISTS
  FOR (n:NostrUser) ON (n.hops);

CREATE INDEX nostrUser_personalizedPageRank IF NOT EXISTS
  FOR (n:NostrUser) ON (n.personalizedPageRank);

CREATE INDEX nostrUser_influence IF NOT EXISTS
  FOR (n:NostrUser) ON (n.influence);

CREATE INDEX nostrUser_verifiedFollowerCount IF NOT EXISTS
  FOR (n:NostrUser) ON (n.verifiedFollowerCount);

CREATE INDEX nostrUser_verifiedMuterCount IF NOT EXISTS
  FOR (n:NostrUser) ON (n.verifiedMuterCount);

CREATE INDEX nostrUser_verifiedReporterCount IF NOT EXISTS
  FOR (n:NostrUser) ON (n.verifiedReporterCount);

CREATE INDEX nostrUser_followerInput IF NOT EXISTS
  FOR (n:NostrUser) ON (n.followerInput);

-- SetOfNostrUserWotMetricsCards constraint
CREATE CONSTRAINT SetOfNostrUserWotMetricsCards_observee_pubkey IF NOT EXISTS
  FOR (n:SetOfNostrUserWotMetricsCards) REQUIRE n.observee_pubkey IS UNIQUE;

-- NostrUserWotMetricsCard constraints and indexes
CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_1 IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) REQUIRE (n.customer_id, n.observee_pubkey) IS UNIQUE;

CREATE CONSTRAINT nostrUserWotMetricsCard_unique_combination_2 IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) REQUIRE (n.observer_pubkey, n.observee_pubkey) IS UNIQUE;

CREATE INDEX nostrUserWotMetricsCard_customer_id IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.customer_id);

CREATE INDEX nostrUserWotMetricsCard_observer_pubkey IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.observer_pubkey);

CREATE INDEX nostrUserWotMetricsCard_observee_pubkey IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.observee_pubkey);

CREATE INDEX nostrUserWotMetricsCard_hops IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.hops);

CREATE INDEX nostrUserWotMetricsCard_personalizedPageRank IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.personalizedPageRank);

CREATE INDEX nostrUserWotMetricsCard_influence IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.influence);

CREATE INDEX nostrUserWotMetricsCard_verifiedFollowerCount IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.verifiedFollowerCount);

CREATE INDEX nostrUserWotMetricsCard_verifiedMuterCount IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.verifiedMuterCount);

CREATE INDEX nostrUserWotMetricsCard_verifiedReporterCount IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.verifiedReporterCount);

CREATE INDEX nostrUserWotMetricsCard_followerInput IF NOT EXISTS
  FOR (n:NostrUserWotMetricsCard) ON (n.followerInput);
```

## Example Queries

### Find users followed by owner within N hops

```cypher
MATCH path = (owner:NostrUser {pubkey: $ownerPubkey})-[:FOLLOWS*1..3]->(user:NostrUser)
WHERE user.hops <= 3
RETURN user.pubkey, user.hops, user.influence
ORDER BY user.influence DESC
LIMIT 100
```

### Get trust metrics for a specific observer-observee pair

```cypher
MATCH (card:NostrUserWotMetricsCard {
  observer_pubkey: $observerPubkey,
  observee_pubkey: $observeePubkey
})
RETURN card.hops, card.influence, card.personalizedPageRank
```

### Find highly trusted users (high influence, many verified followers)

```cypher
MATCH (user:NostrUser)
WHERE user.influence > $threshold
  AND user.verifiedFollowerCount > $minFollowers
RETURN user.pubkey, user.influence, user.verifiedFollowerCount
ORDER BY user.influence DESC
LIMIT 50
```

### Find reported users with high reporter influence

```cypher
MATCH (reporter:NostrUser)-[r:REPORTS]->(reported:NostrUser)
WHERE reporter.influence > $threshold
RETURN reported.pubkey,
       r.reportType,
       COUNT(reporter) AS reportCount,
       SUM(reporter.influence) AS totalInfluence
ORDER BY totalInfluence DESC
```

## Integration with ORLY Relay

### Configuration

```bash
# Enable Neo4j backend
export ORLY_DB_TYPE=neo4j
export ORLY_NEO4J_URI=bolt://localhost:7687
export ORLY_NEO4J_USER=neo4j
export ORLY_NEO4J_PASSWORD=password

# Enable WoT processing
export ORLY_WOT_ENABLED=true
export ORLY_WOT_OWNER_PUBKEY=<hex-pubkey>
export ORLY_WOT_INFLUENCE_THRESHOLD=0.5
export ORLY_WOT_MAX_HOPS=3

# Enable multi-tenant support
export ORLY_WOT_MULTI_TENANT=true
```

### Event Processing Flow

1. **Kind 0 (Profile)**: Update NostrUser node properties
2. **Kind 3 (Follows)**: Parse p-tags, create/update FOLLOWS relationships
3. **Kind 1984 (Reports)**: Parse p-tags and report type, create REPORTS relationships
4. **Kind 10000 (Mutes)**: Parse p-tags, create/update MUTES relationships
5. **Background Job**: Periodically run GrapeRank and PageRank algorithms
6. **Kind 30382 (Trusted Assertion)**: Update NostrUserWotMetricsCard nodes

### Query Filtering

Extend REQ filters with WoT parameters:

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

## Performance Considerations

- **Index Strategy**: Heavy indexing on trust metric fields for fast filtering
- **Batch Updates**: Process social graph events in batches to minimize graph writes
- **Cached Metrics**: Store computed trust metrics as node properties (denormalized)
- **Incremental Computation**: Update metrics incrementally when graph changes
- **Query Optimization**: Use Cypher query plans (EXPLAIN/PROFILE) to optimize complex traversals

## Future Enhancements

- NIP-56 report type nodes (separate from NostrUser properties)
- Full relay mode with NostrEvent nodes
- Zap-weighted trust metrics
- Reply/reaction-weighted trust metrics
- Distributed trust computation across multiple relay instances
- Real-time trust metric updates (streaming)

## References

- NIP-56 (Reporting): https://github.com/nostr-protocol/nips/blob/master/56.md
- NIP-85 (Trusted Assertions): https://nostrhub.io/naddr1qvzqqqrcvypzq3svyhng9ld8sv44950j957j9vchdktj7cxumsep9mvvjthc2pjuqyt8wumn8ghj7un9d3shjtnswf5k6ctv9ehx2aqqzf68yatnw3jkgttpwdek2un5d9hkuuctys9zn
- Brainstorm Prototype: https://github.com/Pretty-Good-Freedom-Tech/brainstorm
- NIP-56 Metrics Dashboard: https://straycat.brainstorm.social/nip56.html
