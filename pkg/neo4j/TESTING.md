# Testing the Neo4j Social Event Processor

This document explains how to run tests for the social event processor that manages NostrUser vertices and social graph relationships.

## Prerequisites

### 1. Neo4j Instance

You need a running Neo4j instance for integration tests. The easiest way is using Docker:

```bash
# Using docker-compose (recommended)
cd pkg/neo4j
docker-compose up -d
docker-compose logs -f neo4j  # Wait for "Started."

# Or manually:
docker run -d --name neo4j-test \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/testpass123 \
  neo4j:5.15

# Wait for Neo4j to start (check logs)
docker logs -f neo4j-test
```

Access the Neo4j browser at http://localhost:7474 (credentials: neo4j/testpass123)

### 2. Environment Variables

Set the Neo4j connection details:

```bash
export ORLY_NEO4J_URI="bolt://localhost:7687"
export ORLY_NEO4J_USER="neo4j"
export ORLY_NEO4J_PASSWORD="testpass123"
```

### 3. libsecp256k1.so

The tests require the secp256k1 library for signing events:

```bash
# Download from nostr repository
wget https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so -P /tmp/

# Add to library path
export LD_LIBRARY_PATH="/tmp:$LD_LIBRARY_PATH"
```

## Running Tests

### All Tests

```bash
cd pkg/neo4j
go test -v
```

### Specific Test

```bash
# Test profile metadata processing
go test -v -run TestSocialEventProcessor/Kind0_ProfileMetadata

# Test contact list initial creation
go test -v -run TestSocialEventProcessor/Kind3_ContactList_Initial

# Test contact list updates
go test -v -run TestSocialEventProcessor/Kind3_ContactList_Update

# Test mute list
go test -v -run TestSocialEventProcessor/Kind10000_MuteList

# Test reports
go test -v -run TestSocialEventProcessor/Kind1984_Reports
```

### Helper Function Tests

```bash
# Test diff computation
go test -v -run TestDiffComputation

# Test p-tag extraction
go test -v -run TestExtractPTags
```

### Benchmarks

```bash
# Benchmark diff computation with 1000-element lists
go test -bench=BenchmarkDiffComputation -benchmem
```

## Test Structure

### TestSocialEventProcessor

Comprehensive integration test that exercises the complete event processing flow:

1. **Kind 0 - Profile Metadata**
   - Creates a profile for Alice
   - Verifies NostrUser node has correct name, about, picture

2. **Kind 3 - Contact List (Initial)**
   - Alice follows Bob and Charlie
   - Verifies 2 FOLLOWS relationships created
   - Checks event traceability (created_by_event property)

3. **Kind 3 - Contact List (Add Follow)**
   - Alice adds Dave to follows (now: Bob, Charlie, Dave)
   - Verifies diff-based update (only Dave relationship added)
   - Checks old event marked as superseded

4. **Kind 3 - Contact List (Remove Follow)**
   - Alice unfollows Charlie (now: Bob, Dave)
   - Verifies Charlie's FOLLOWS relationship removed
   - Other relationships unchanged

5. **Kind 3 - Contact List (Older Event Rejected)**
   - Attempts to save an old contact list event
   - Verifies it's rejected (follows list unchanged)

6. **Kind 10000 - Mute List**
   - Alice mutes Eve
   - Verifies MUTES relationship created

7. **Kind 1984 - Reports**
   - Alice reports Eve for "spam"
   - Bob reports Eve for "illegal"
   - Verifies 2 REPORTS relationships created
   - Checks report types are correct

8. **Final Graph State Verification**
   - Alice follows: Bob, Dave
   - Alice mutes: Eve
   - Eve has 2 reports (from Alice and Bob)
   - All relationships have event traceability

### Test Helper Functions

- **generateTestKeypair()**: Creates test keypairs for users
- **queryFollows()**: Queries active FOLLOWS relationships
- **queryMutes()**: Queries active MUTES relationships
- **queryReports()**: Queries REPORTS relationships
- **diffStringSlices()**: Computes added/removed elements
- **extractPTags()**: Extracts p-tags from event
- **slicesEqual()**: Compares slices (order-independent)

## Expected Output

Successful test run:

```
=== RUN   TestSocialEventProcessor
=== RUN   TestSocialEventProcessor/Kind0_ProfileMetadata
✓ Profile metadata processed: name=Alice
=== RUN   TestSocialEventProcessor/Kind3_ContactList_Initial
✓ Initial contact list created: Alice follows [Bob, Charlie]
=== RUN   TestSocialEventProcessor/Kind3_ContactList_Update_AddFollow
✓ Contact list updated: Alice follows [Bob, Charlie, Dave]
=== RUN   TestSocialEventProcessor/Kind3_ContactList_Update_RemoveFollow
✓ Contact list updated: Alice unfollowed Charlie
=== RUN   TestSocialEventProcessor/Kind3_ContactList_OlderEventRejected
✓ Older contact list event rejected (follows unchanged)
=== RUN   TestSocialEventProcessor/Kind10000_MuteList
✓ Mute list processed: Alice mutes Eve
=== RUN   TestSocialEventProcessor/Kind1984_Reports
✓ Reports processed: Eve reported by Alice (spam) and Bob (illegal)
=== RUN   TestSocialEventProcessor/VerifyGraphState
✓ Final graph state verified
  - Alice follows: [bob_pubkey, dave_pubkey]
  - Alice mutes: [eve_pubkey]
  - Reports against Eve: 2
--- PASS: TestSocialEventProcessor (0.45s)
PASS
```

## Viewing Graph in Neo4j Browser

After running tests, you can explore the graph in Neo4j Browser (http://localhost:7474):

### View All NostrUser Nodes

```cypher
MATCH (u:NostrUser)
RETURN u
```

### View Social Graph

```cypher
MATCH path = (u1:NostrUser)-[r:FOLLOWS|MUTES|REPORTS]->(u2:NostrUser)
RETURN path
```

### View Alice's Follows

```cypher
MATCH (alice:NostrUser {name: "Alice"})-[:FOLLOWS]->(followed:NostrUser)
RETURN alice, followed
```

### View Event Processing History

```cypher
MATCH (evt:ProcessedSocialEvent)
RETURN evt.event_id, evt.event_kind, evt.created_at, evt.superseded_by
ORDER BY evt.created_at ASC
```

### View Event Traceability

```cypher
MATCH (u1:NostrUser)-[r:FOLLOWS]->(u2:NostrUser)
MATCH (evt:ProcessedSocialEvent {event_id: r.created_by_event})
RETURN u1.name, u2.name, evt.event_id, evt.created_at
```

## Cleanup

### Clear Test Data

```cypher
// In Neo4j Browser
MATCH (n)
DETACH DELETE n
```

Or use the database wipe function:

```bash
# In Go test
db.Wipe()  // Removes all data and reapplies schema
```

### Stop Neo4j Container

```bash
docker stop neo4j-test
docker rm neo4j-test
```

## Continuous Integration

To run tests in CI without Neo4j:

```bash
# Tests will be skipped if ORLY_NEO4J_URI is not set
go test ./pkg/neo4j/...
```

Output:
```
?       next.orly.dev/pkg/neo4j [no test files]
--- SKIP: TestSocialEventProcessor (0.00s)
    testmain_test.go:14: Neo4j not available (set ORLY_NEO4J_URI to enable tests)
```

## Debugging Failed Tests

### Enable Debug Logging

```bash
# Run with debug logs
go test -v -run TestSocialEventProcessor 2>&1 | tee test.log
```

The database logger is set to "debug" level in tests, showing all Cypher queries.

### Check Neo4j Logs

```bash
docker logs neo4j-test
```

### Inspect Graph State

After a failed test, connect to Neo4j Browser and run diagnostic queries:

```cypher
// Count nodes by label
MATCH (n)
RETURN labels(n), count(*)

// Count relationships by type
MATCH ()-[r]->()
RETURN type(r), count(*)

// Find relationships without event traceability
MATCH ()-[r:FOLLOWS|MUTES|REPORTS]->()
WHERE r.created_by_event IS NULL
RETURN r

// Find superseded events
MATCH (evt:ProcessedSocialEvent)
WHERE evt.superseded_by IS NOT NULL
RETURN evt
```

## Performance Benchmarks

Run benchmarks to measure diff computation performance:

```bash
go test -bench=. -benchmem
```

Expected output:
```
BenchmarkDiffComputation-8   	   50000	     30000 ns/op	   16384 B/op	      20 allocs/op
```

This benchmarks diff computation with:
- 1000-element lists
- 800 common elements
- 200 added elements
- 200 removed elements

## Test Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Target coverage:
- social-event-processor.go: >80%
- Helper functions: 100%

## Known Limitations

1. **No event deletion tests**: Kind 5 (event deletion) not implemented yet
2. **No encrypted tag tests**: Kind 10000 encrypted tags not supported yet
3. **No concurrent update tests**: Need to test race conditions
4. **No large list tests**: Should test with 1000+ follows

## Future Test Additions

- [ ] Test concurrent contact list updates
- [ ] Test very large follow lists (1000+ users)
- [ ] Test encrypted mute lists (NIP-59)
- [ ] Test event deletion (kind 5)
- [ ] Test malformed events (invalid pubkeys, etc.)
- [ ] Test Neo4j connection failures
- [ ] Test transaction rollbacks
- [ ] Load testing with realistic event stream

## Troubleshooting

### "Neo4j not available"

Ensure Neo4j is running and environment variables are set:
```bash
docker ps | grep neo4j
echo $ORLY_NEO4J_URI
```

### "Failed to create database"

Check Neo4j authentication:
```bash
docker exec -it neo4j-test cypher-shell -u neo4j -p test
```

### "libsecp256k1.so not found"

Download and set LD_LIBRARY_PATH:
```bash
wget https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so
export LD_LIBRARY_PATH="$(pwd):$LD_LIBRARY_PATH"
```

### "Constraint already exists"

The database wasn't cleaned between tests. Restart Neo4j:
```bash
docker restart neo4j-test
```

## Contact

For questions or issues with tests:
- File an issue: https://github.com/anthropics/orly/issues
- Check documentation: [EVENT_PROCESSING_SPEC.md](./EVENT_PROCESSING_SPEC.md)
