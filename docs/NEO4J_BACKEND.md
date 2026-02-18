# Neo4j Database Backend for ORLY Relay

## Overview

The Neo4j database backend provides a graph-native storage solution for the ORLY Nostr relay. Unlike traditional key-value or document stores, Neo4j is optimized for relationship-heavy queries, making it an ideal fit for Nostr's social graph and event reference patterns.

## Architecture

### Core Components

1. **Main Database File** ([pkg/neo4j/neo4j.go](../pkg/neo4j/neo4j.go))
   - Implements the `database.Database` interface
   - Manages Neo4j driver connection and lifecycle
   - Uses Badger for metadata storage (markers, identity, subscriptions)
   - Registers with the database factory via `init()`

2. **Schema Management** ([pkg/neo4j/schema.go](../pkg/neo4j/schema.go))
   - Defines Neo4j constraints and indexes using Cypher
   - Creates unique constraints on Event IDs and Author pubkeys
   - Indexes for optimal query performance (kind, created_at, tags)

3. **Query Engine** ([pkg/neo4j/query-events.go](../pkg/neo4j/query-events.go))
   - Translates Nostr REQ filters to Cypher queries
   - Leverages graph traversal for tag relationships
   - Supports prefix matching for IDs and pubkeys
   - Parameterized queries for security and performance

4. **Event Storage** ([pkg/neo4j/save-event.go](../pkg/neo4j/save-event.go))
   - Stores events as nodes with properties
   - Creates graph relationships:
     - `AUTHORED_BY`: Event → Author
     - `REFERENCES`: Event → Event (e-tags)
     - `MENTIONS`: Event → Author (p-tags)
     - `TAGGED_WITH`: Event → Tag

## Graph Schema

### Node Types

**Event Node**
```cypher
(:Event {
  id: string,           // Hex-encoded event ID (32 bytes)
  serial: int,          // Sequential serial number
  kind: int,            // Event kind
  created_at: int,      // Unix timestamp
  content: string,      // Event content
  sig: string,          // Hex-encoded signature
  pubkey: string,       // Hex-encoded author pubkey
  tags: string          // JSON-encoded tags array
})
```

**Author Node**
```cypher
(:Author {
  pubkey: string        // Hex-encoded pubkey (unique)
})
```

**Tag Node**
```cypher
(:Tag {
  type: string,         // Tag type (e.g., "t", "d")
  value: string         // Tag value
})
```

**Word Node** (NIP-50 search index)
```cypher
(:Word {
  hash: string,         // 8-byte truncated SHA-256, hex-encoded (16 chars)
  text: string          // Normalized lowercase word (e.g., "bitcoin")
})
```

**Marker Node** (for metadata)
```cypher
(:Marker {
  key: string,          // Unique key
  value: string         // Hex-encoded value
})
```

### Relationships

- `(:Event)-[:AUTHORED_BY]->(:Author)` - Event authorship
- `(:Event)-[:REFERENCES]->(:Event)` - Event references (e-tags)
- `(:Event)-[:MENTIONS]->(:Author)` - Author mentions (p-tags)
- `(:Event)-[:TAGGED_WITH]->(:Tag)` - Generic tag associations
- `(:Event)-[:HAS_WORD]->(:Word)` - Word search index (NIP-50)

## How Nostr REQ Messages Are Implemented

### Filter to Cypher Translation

The query engine in [query-events.go](../pkg/neo4j/query-events.go) translates Nostr filters to Cypher queries:

#### 1. ID Filters
```json
{"ids": ["abc123..."]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.id = $id_0
```

For prefix matching (partial IDs):
```cypher
WHERE e.id STARTS WITH $id_0
```

#### 2. Author Filters
```json
{"authors": ["pubkey1...", "pubkey2..."]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.pubkey IN $authors
```

#### 3. Kind Filters
```json
{"kinds": [1, 7]}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.kind IN $kinds
```

#### 4. Time Range Filters
```json
{"since": 1234567890, "until": 1234567900}
```
Becomes:
```cypher
MATCH (e:Event)
WHERE e.created_at >= $since AND e.created_at <= $until
```

#### 5. Tag Filters (Graph Advantage!)
```json
{"#t": ["bitcoin", "nostr"]}
```
Becomes:
```cypher
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t0:Tag)
WHERE t0.type = $tagType_0 AND t0.value IN $tagValues_0
```

This leverages Neo4j's native graph traversal for efficient tag queries!

#### 6. NIP-50 Word Search
```json
{"search": "bitcoin lightning"}
```
Becomes:
```cypher
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE w.hash IN $wordHashes
WITH e, count(DISTINCT w) AS matchCount
RETURN e.id, e.kind, e.created_at, e.content, e.sig, e.pubkey, e.tags, matchCount
ORDER BY matchCount DESC, e.created_at DESC
LIMIT $limit
```

Search can be combined with any other filter (kinds, authors, since/until, tags). See the [NIP-50 Search](#nip-50-word-search-1) section below for full details.

#### 7. Combined Filters
```json
{
  "kinds": [1],
  "authors": ["abc..."],
  "#p": ["xyz..."],
  "limit": 50
}
```
Becomes:
```cypher
MATCH (e:Event)
OPTIONAL MATCH (e)-[:TAGGED_WITH]->(t0:Tag)
WHERE e.kind IN $kinds
  AND e.pubkey IN $authors
  AND t0.type = $tagType_0
  AND t0.value IN $tagValues_0
RETURN e.id, e.kind, e.created_at, e.content, e.sig, e.pubkey, e.tags
ORDER BY e.created_at DESC
LIMIT $limit
```

### Query Execution Flow

1. **Parse Filter**: Extract IDs, authors, kinds, times, tags
2. **Build Cypher**: Construct parameterized query with MATCH/WHERE clauses
3. **Execute**: Run via `ExecuteRead()` with read-only session
4. **Parse Results**: Convert Neo4j records to Nostr events
5. **Return**: Send events back to client

## Configuration

All configuration is centralized in `app/config/config.go` and visible via `./orly help`.

> **Important:** All environment variables must be defined in `app/config/config.go`. Do not use `os.Getenv()` directly in package code. Database backends receive configuration via the `database.DatabaseConfig` struct.

### Environment Variables

```bash
# Neo4j Connection
ORLY_NEO4J_URI="bolt://localhost:7687"
ORLY_NEO4J_USER="neo4j"
ORLY_NEO4J_PASSWORD="password"

# Database Type Selection
ORLY_DB_TYPE="neo4j"

# Data Directory (for Badger metadata storage)
ORLY_DATA_DIR="~/.local/share/ORLY"

# Neo4j Driver Tuning (Memory Management)
ORLY_NEO4J_MAX_CONN_POOL=25       # Max connections (default: 25, driver default: 100)
ORLY_NEO4J_FETCH_SIZE=1000        # Records per fetch batch (default: 1000, -1=all)
ORLY_NEO4J_MAX_TX_RETRY_SEC=30    # Max transaction retry time in seconds
ORLY_NEO4J_QUERY_RESULT_LIMIT=10000  # Max results per query (0=unlimited)
```

### Example Docker Compose Setup

```yaml
version: '3.8'
services:
  neo4j:
    image: neo4j:5.15
    ports:
      - "7474:7474"  # HTTP
      - "7687:7687"  # Bolt
    environment:
      - NEO4J_AUTH=neo4j/password
      - NEO4J_PLUGINS=["apoc"]
      # Memory tuning for production
      - NEO4J_server_memory_heap_initial__size=512m
      - NEO4J_server_memory_heap_max__size=1g
      - NEO4J_server_memory_pagecache_size=512m
      # Transaction memory limits (prevent runaway queries)
      - NEO4J_dbms_memory_transaction_total__max=256m
      - NEO4J_dbms_memory_transaction_max=64m
      # Query timeout
      - NEO4J_dbms_transaction_timeout=30s
    volumes:
      - neo4j_data:/data
      - neo4j_logs:/logs

  orly:
    build: .
    ports:
      - "3334:3334"
    environment:
      - ORLY_DB_TYPE=neo4j
      - ORLY_NEO4J_URI=bolt://neo4j:7687
      - ORLY_NEO4J_USER=neo4j
      - ORLY_NEO4J_PASSWORD=password
      # Driver tuning for memory management
      - ORLY_NEO4J_MAX_CONN_POOL=25
      - ORLY_NEO4J_FETCH_SIZE=1000
      - ORLY_NEO4J_QUERY_RESULT_LIMIT=10000
    depends_on:
      - neo4j

volumes:
  neo4j_data:
  neo4j_logs:
```

## Performance Considerations

### Advantages Over Badger/DGraph

1. **Native Graph Queries**: Tag relationships and social graph traversals are native operations
2. **Optimized Indexes**: Automatic index usage for constrained properties
3. **Efficient Joins**: Relationship traversals are O(1) lookups
4. **Query Planner**: Neo4j's query planner optimizes complex multi-filter queries

### Tuning Recommendations

1. **Indexes**: The schema creates indexes for:
   - Event ID (unique constraint + index)
   - Event kind
   - Event created_at
   - Composite: kind + created_at
   - Tag type + value

2. **Cache Configuration**: Configure Neo4j's page cache and heap size (see Memory Tuning below)

3. **Query Limits**: The relay automatically enforces `ORLY_NEO4J_QUERY_RESULT_LIMIT` (default: 10000) to prevent unbounded queries from exhausting memory

## Memory Tuning

Neo4j runs as a separate process (typically in Docker), so memory management involves both the relay driver settings and Neo4j server configuration.

### Understanding Memory Layers

1. **ORLY Relay Process** (~35MB RSS typical)
   - Go driver connection pool
   - Query result buffering
   - Controlled by `ORLY_NEO4J_*` environment variables

2. **Neo4j Server Process** (512MB-4GB+ depending on data)
   - JVM heap for Java objects
   - Page cache for graph data
   - Transaction memory for query execution
   - Controlled by `NEO4J_*` environment variables

### Relay Driver Tuning (ORLY side)

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEO4J_MAX_CONN_POOL` | 25 | Max connections in pool. Lower = less memory, but may bottleneck under high load. Driver default is 100. |
| `ORLY_NEO4J_FETCH_SIZE` | 1000 | Records fetched per batch. Lower = less memory per query, more round trips. Set to -1 for all (risky). |
| `ORLY_NEO4J_MAX_TX_RETRY_SEC` | 30 | Max seconds to retry failed transactions. |
| `ORLY_NEO4J_QUERY_RESULT_LIMIT` | 10000 | Hard cap on results per query. Prevents unbounded queries. Set to 0 for unlimited (not recommended). |

**Recommended settings for memory-constrained environments:**
```bash
ORLY_NEO4J_MAX_CONN_POOL=10
ORLY_NEO4J_FETCH_SIZE=500
ORLY_NEO4J_QUERY_RESULT_LIMIT=5000
```

### Neo4j Server Tuning (Docker/neo4j.conf)

**JVM Heap Memory** - For Java objects and query processing:
```bash
# Docker environment variables
NEO4J_server_memory_heap_initial__size=512m
NEO4J_server_memory_heap_max__size=1g

# neo4j.conf equivalent
server.memory.heap.initial_size=512m
server.memory.heap.max_size=1g
```

**Page Cache** - For caching graph data from disk:
```bash
# Docker
NEO4J_server_memory_pagecache_size=512m

# neo4j.conf
server.memory.pagecache.size=512m
```

**Transaction Memory Limits** - Prevent runaway queries:
```bash
# Docker
NEO4J_dbms_memory_transaction_total__max=256m   # Global limit across all transactions
NEO4J_dbms_memory_transaction_max=64m           # Per-transaction limit

# neo4j.conf
dbms.memory.transaction.total.max=256m
db.memory.transaction.max=64m
```

**Query Timeout** - Kill long-running queries:
```bash
# Docker
NEO4J_dbms_transaction_timeout=30s

# neo4j.conf
dbms.transaction.timeout=30s
```

### Memory Sizing Guidelines

| Deployment Size | Heap | Page Cache | Total Neo4j | ORLY Pool |
|-----------------|------|------------|-------------|-----------|
| Development | 512m | 256m | ~1GB | 10 |
| Small relay (<100k events) | 1g | 512m | ~2GB | 25 |
| Medium relay (<1M events) | 2g | 1g | ~4GB | 50 |
| Large relay (>1M events) | 4g | 2g | ~8GB | 100 |

**Formula for Page Cache:**
```
Page Cache = Data Size on Disk × 1.2
```

Use `neo4j-admin server memory-recommendation` inside the container to get tailored recommendations.

### Monitoring Memory Usage

**Check Neo4j memory from relay logs:**
```bash
# Driver config is logged at startup
grep "connecting to neo4j" /path/to/orly.log
# Output: connecting to neo4j at bolt://... (pool=25, fetch=1000, txRetry=30s)
```

**Check Neo4j server memory:**
```bash
# Inside Neo4j container
docker exec neo4j neo4j-admin server memory-recommendation

# Or query via Cypher
CALL dbms.listPools() YIELD pool, heapMemoryUsed, heapMemoryUsedBytes
RETURN pool, heapMemoryUsed
```

**Monitor transaction memory:**
```cypher
CALL dbms.listTransactions()
YIELD transactionId, currentQuery, allocatedBytes
RETURN transactionId, currentQuery, allocatedBytes
ORDER BY allocatedBytes DESC
```

## Implementation Details

### Replaceable Events

Replaceable events (kinds 0, 3, 10000-19999) are handled in `WouldReplaceEvent()`:

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})
WHERE e.created_at < $createdAt
RETURN e.serial, e.created_at
```

Older events are deleted before saving the new one.

### Parameterized Replaceable Events

For kinds 30000-39999, we also match on the d-tag:

```cypher
MATCH (e:Event {kind: $kind, pubkey: $pubkey})-[:TAGGED_WITH]->(t:Tag {type: 'd', value: $dValue})
WHERE e.created_at < $createdAt
RETURN e.serial
```

### Event Deletion (NIP-09)

Delete events (kind 5) are processed via graph traversal:

```cypher
MATCH (target:Event {id: $targetId})
MATCH (delete:Event {kind: 5})-[:REFERENCES]->(target)
WHERE delete.pubkey = $pubkey OR delete.pubkey IN $admins
RETURN delete.id
```

Only same-author or admin deletions are allowed.

## Comparison with Other Backends

| Feature | Badger | DGraph | Neo4j |
|---------|--------|--------|-------|
| **Storage Type** | Key-value | Graph (distributed) | Graph (native) |
| **Query Language** | Custom indexes | DQL | Cypher |
| **Tag Queries** | Index lookups | Graph traversal | Native relationships |
| **Scaling** | Single-node | Distributed | Cluster/Causal cluster |
| **Memory Usage** | Low | Medium | High |
| **Setup Complexity** | Minimal | Medium | Medium |
| **Best For** | Small relays | Large distributed | Relationship-heavy |

## Development Guide

### Adding New Indexes

1. Update [schema.go](../pkg/neo4j/schema.go) with new index definition
2. Add to `applySchema()` function
3. Restart relay to apply schema changes

Example:
```cypher
CREATE INDEX event_content_fulltext IF NOT EXISTS
FOR (e:Event) ON (e.content)
OPTIONS {indexConfig: {`fulltext.analyzer`: 'english'}}
```

### Custom Queries

To add custom query methods:

1. Add method to [query-events.go](../pkg/neo4j/query-events.go)
2. Build Cypher query with parameterization
3. Use `ExecuteRead()` or `ExecuteWrite()` as appropriate
4. Parse results with `parseEventsFromResult()`

### Testing

Due to Neo4j dependency, tests require a running Neo4j instance:

```bash
# Start Neo4j via Docker
docker run -d --name neo4j-test \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/test \
  neo4j:5.15

# Run tests
ORLY_NEO4J_URI="bolt://localhost:7687" \
ORLY_NEO4J_USER="neo4j" \
ORLY_NEO4J_PASSWORD="test" \
go test ./pkg/neo4j/...

# Cleanup
docker rm -f neo4j-test
```

## Bolt+S External Access (Remote Cypher Queries)

### What Is Bolt+S?

Bolt+S is Neo4j's encrypted Bolt protocol — the same wire protocol used by `cypher-shell`, Neo4j Browser, and client drivers, but wrapped in TLS. By default, the ORLY relay's Neo4j instance only listens on `localhost` and is not accessible from the network. Enabling bolt+s exposes Neo4j on a public port with TLS encryption, allowing external tools to run Cypher queries against the relay's graph database.

This is useful for:
- Running ad-hoc Cypher queries from Neo4j Browser or Desktop
- Connecting knowledge graph tools (e.g., Brainstorm) to the relay's social graph
- Exploring the event graph, author relationships, and tag networks
- Running analytics queries that aren't exposed through the Nostr protocol

### How It Works

The ORLY relay manages Neo4j's bolt+s configuration through its web admin UI:

1. The relay reads and writes `neo4j.conf` directly (path configured via `ORLY_NEO4J_CONF_PATH`)
2. When an owner toggles bolt+s on/off, the relay modifies the relevant TLS and connector settings in `neo4j.conf`
3. The relay then restarts Neo4j via a configurable shell command (default: `sudo systemctl restart neo4j`)
4. The web UI displays the resulting `bolt+s://` connection URI

The admin UI is only visible when `ORLY_DB_TYPE=neo4j` and only accessible to users with owner-level permissions.

### Prerequisites

Before enabling bolt+s, you need:

1. **Neo4j installed and running** as a systemd service (or equivalent)
2. **TLS certificates** — typically from Let's Encrypt, but any valid cert/key pair works
3. **The relay user must be able to restart Neo4j** — via passwordless sudo
4. **Neo4j must be able to read the TLS certificates** — file permissions
5. **The bolt port must be open** in your firewall

### Step-by-Step Setup

#### 1. Set the TLS Certificate Directory

Tell the relay where your TLS certificates live. For Let's Encrypt:

```bash
ORLY_NEO4J_TLS_CERT_DIR=/etc/letsencrypt/live/relay.example.com
```

This directory must contain:
- `privkey.pem` — the private key
- `fullchain.pem` — the certificate chain

If you're using Let's Encrypt with Caddy or certbot, these files are created automatically. The relay configures Neo4j to use these exact filenames.

#### 2. Grant the Relay User Permission to Restart Neo4j

The relay needs to run `sudo systemctl restart neo4j` after modifying the config. Create a sudoers rule so this works without a password:

```bash
echo 'mleku ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart neo4j' | sudo tee /etc/sudoers.d/orly-neo4j
sudo chmod 440 /etc/sudoers.d/orly-neo4j
```

Replace `mleku` with the user running the relay. This grants permission only for `systemctl restart neo4j` — nothing else.

#### 3. Ensure Neo4j Can Read the TLS Certificates

Let's Encrypt certificates are typically owned by root with restricted permissions. Neo4j (which runs as the `neo4j` user) needs read access:

```bash
sudo chmod -R 755 /etc/letsencrypt/live/ /etc/letsencrypt/archive/
```

Alternatively, copy the certificates to a directory Neo4j owns:

```bash
sudo mkdir -p /var/lib/neo4j/certificates/bolt
sudo cp /etc/letsencrypt/live/relay.example.com/privkey.pem /var/lib/neo4j/certificates/bolt/
sudo cp /etc/letsencrypt/live/relay.example.com/fullchain.pem /var/lib/neo4j/certificates/bolt/
sudo chown -R neo4j:neo4j /var/lib/neo4j/certificates/bolt
```

If you copy certificates, set `ORLY_NEO4J_TLS_CERT_DIR=/var/lib/neo4j/certificates/bolt` and remember to update the copies when certificates renew.

#### 4. Open the Bolt Port in Your Firewall

The default bolt port is 7687. If you're using `ufw`:

```bash
sudo ufw allow 7687/tcp
```

If you're using a cloud provider's firewall (AWS security groups, DigitalOcean firewall, etc.), add an inbound rule for TCP port 7687.

To use a non-default port:

```bash
ORLY_NEO4J_BOLT_PORT=7688
```

#### 5. Restart the Relay

The relay reads `ORLY_NEO4J_TLS_CERT_DIR` at startup. After setting it, restart the relay:

```bash
sudo systemctl restart orly
```

#### 6. Enable Bolt+S via the Web UI

1. Log in to the relay's admin dashboard with an owner account
2. Click the **Neo4j** tab in the sidebar
3. Click **Load Configuration** to see the current bolt+s status
4. Toggle the **Bolt+S** switch to enabled
5. Click **Apply & Restart Neo4j**
6. The connection URI (e.g., `bolt+s://relay.example.com:7687`) appears once enabled

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_NEO4J_CONF_PATH` | `/etc/neo4j/neo4j.conf` | Path to neo4j.conf. The relay reads and writes this file to manage bolt+s settings. |
| `ORLY_NEO4J_RESTART_CMD` | `sudo systemctl restart neo4j` | Shell command to restart Neo4j after config changes. Must succeed without interactive input. |
| `ORLY_NEO4J_TLS_CERT_DIR` | *(empty)* | Directory containing `privkey.pem` and `fullchain.pem`. Must be set before bolt+s can be enabled. |
| `ORLY_NEO4J_BOLT_PORT` | `7687` | The external port Neo4j listens on for bolt connections. Used in both `neo4j.conf` and the displayed connection URI. |

### What the Relay Modifies in neo4j.conf

When enabling bolt+s, the relay sets these keys in `neo4j.conf`:

```ini
# Require TLS for all bolt connections
dbms.connector.bolt.tls_level=REQUIRED

# Listen on all interfaces (not just localhost)
dbms.connector.bolt.listen_address=0.0.0.0:7687

# Enable the bolt SSL policy
dbms.ssl.policy.bolt.enabled=true
dbms.ssl.policy.bolt.base_directory=/etc/letsencrypt/live/relay.example.com
dbms.ssl.policy.bolt.private_key=privkey.pem
dbms.ssl.policy.bolt.public_certificate=fullchain.pem
dbms.ssl.policy.bolt.client_auth=NONE
```

When disabling, `tls_level` is set to `DISABLED`, `bolt.enabled` to `false`, and the certificate settings are commented out. The relay handles both Neo4j 4.x (`dbms.connector.bolt.*`) and 5.x (`server.bolt.*`) config key formats.

### Connecting to the Database

Once bolt+s is enabled, connect with any Neo4j client:

**cypher-shell:**
```bash
cypher-shell -a bolt+s://relay.example.com:7687 -u neo4j -p <password>
```

**Neo4j Browser:**
1. Open Neo4j Browser (Desktop or web)
2. Enter connection URL: `bolt+s://relay.example.com:7687`
3. Enter credentials (same as `ORLY_NEO4J_USER` / `ORLY_NEO4J_PASSWORD`)

**Neo4j Python driver:**
```python
from neo4j import GraphDatabase

driver = GraphDatabase.driver(
    "bolt+s://relay.example.com:7687",
    auth=("neo4j", "password")
)

with driver.session() as session:
    result = session.run("MATCH (e:Event) RETURN count(e) AS total")
    print(result.single()["total"])
```

**Neo4j JavaScript driver:**
```javascript
import neo4j from "neo4j-driver";

const driver = neo4j.driver(
    "bolt+s://relay.example.com:7687",
    neo4j.auth.basic("neo4j", "password")
);

const session = driver.session();
const result = await session.run("MATCH (a:Author) RETURN count(a) AS total");
console.log(result.records[0].get("total"));
await session.close();
```

### Example Queries

Once connected, you can query the relay's graph directly:

```cypher
-- Count all events
MATCH (e:Event) RETURN count(e) AS total;

-- Find the most referenced authors
MATCH (e:Event)-[:MENTIONS]->(a:Author)
RETURN a.pubkey, count(e) AS mentions
ORDER BY mentions DESC LIMIT 20;

-- Social graph: who follows whom (kind 3 contact lists)
MATCH (e:Event {kind: 3})-[:AUTHORED_BY]->(a:Author)
MATCH (e)-[:MENTIONS]->(followed:Author)
RETURN a.pubkey AS follower, followed.pubkey AS follows
LIMIT 100;

-- Event references (reply chains)
MATCH path = (reply:Event)-[:REFERENCES*1..5]->(root:Event)
WHERE root.id = $eventId
RETURN path;

-- Tag popularity
MATCH (e:Event)-[:TAGGED_WITH]->(t:Tag {type: 't'})
RETURN t.value, count(e) AS usage
ORDER BY usage DESC LIMIT 50;
```

### Security Considerations

- **Neo4j credentials are separate from Nostr identities.** Anyone with the Neo4j username and password has full read/write access to the database. Do not share credentials publicly.
- **Bolt+s encrypts the connection** but does not restrict who can connect. Use firewall rules to limit access by IP if needed.
- **The relay's owner toggle is protected by NIP-98 authentication.** Only users with owner-level ACL permissions can enable or disable bolt+s.
- **Consider read-only access.** Neo4j supports role-based access control. You can create a read-only user for external clients:

```cypher
CREATE USER reader SET PASSWORD 'readonly123' SET PASSWORD CHANGE NOT REQUIRED;
GRANT ROLE reader TO reader;
GRANT READ {*} ON GRAPH * TO reader;
```

### Troubleshooting

#### "TLS cert dir must be set before enabling bolt+s"

Set `ORLY_NEO4J_TLS_CERT_DIR` in your environment and restart the relay. The toggle is disabled until this is configured.

#### "Config updated but Neo4j restart failed"

The relay modified `neo4j.conf` successfully but couldn't restart Neo4j. Check:

1. **Sudoers**: Verify the relay user can run the restart command:
   ```bash
   sudo -n systemctl restart neo4j
   ```
   If this prompts for a password, the sudoers rule is missing or incorrect.

2. **Neo4j service exists**:
   ```bash
   systemctl status neo4j
   ```

3. **Restart command override**: If Neo4j isn't managed by systemd, set a custom command:
   ```bash
   ORLY_NEO4J_RESTART_CMD="sudo /usr/local/bin/restart-neo4j.sh"
   ```

#### Cannot connect after enabling bolt+s

1. **Check Neo4j is running**:
   ```bash
   systemctl status neo4j
   journalctl -u neo4j -n 50
   ```

2. **Check Neo4j logs for TLS errors** — common causes:
   - Certificate files not readable by the `neo4j` user
   - Certificate and key don't match
   - Certificate expired

3. **Check the port is open**:
   ```bash
   ss -tlnp | grep 7687
   ```

4. **Check firewall**:
   ```bash
   sudo ufw status | grep 7687
   ```

5. **Test connectivity from the client machine**:
   ```bash
   openssl s_client -connect relay.example.com:7687
   ```

#### Certificate renewal

If you're using Let's Encrypt, certificates renew automatically but Neo4j needs a restart to pick up the new files. Add a renewal hook:

```bash
# /etc/letsencrypt/renewal-hooks/deploy/restart-neo4j.sh
#!/bin/bash
systemctl restart neo4j
```

```bash
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/restart-neo4j.sh
```

#### Rolling back (disabling bolt+s)

Toggle bolt+s off in the web UI and click **Apply & Restart Neo4j**. This sets `tls_level=DISABLED`, comments out the certificate settings, and restarts Neo4j. Bolt connections will revert to unencrypted localhost-only access.

You can also manually revert by editing `neo4j.conf`:

```bash
sudo sed -i 's/^dbms.connector.bolt.tls_level=REQUIRED/dbms.connector.bolt.tls_level=DISABLED/' /etc/neo4j/neo4j.conf
sudo systemctl restart neo4j
```

### API Endpoints

The bolt+s management feature exposes three HTTP endpoints:

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/neo4j/config` | GET | None | Returns `{ "db_type": "neo4j" }`. Used by the UI to decide whether to show the Neo4j tab. |
| `/api/neo4j/bolt` | GET | Owner (NIP-98) | Returns current bolt+s status, config path, port, cert dir, and connection URI. |
| `/api/neo4j/bolt/toggle` | POST | Owner (NIP-98) | Accepts `{ "enabled": true/false }`. Modifies neo4j.conf and restarts Neo4j. |

## NIP-50 Word Search

The Neo4j backend supports full-text word search via [NIP-50](https://github.com/nostr-protocol/nips/blob/master/50.md), using a graph-based inverted index. This is the same approach used by the Badger backend — words are stored as graph nodes linked to events, enabling efficient set-intersection queries via graph traversal.

### How It Works

When an event is saved, its content and tag values are tokenized into individual words. Each unique word becomes a `(:Word)` node in the graph, linked to the event via a `(:Event)-[:HAS_WORD]->(:Word)` relationship. Searching for words is then a graph traversal from Word nodes back to Events.

**Indexing pipeline:**
1. Event content is split into words (Unicode-aware word boundaries)
2. Tag values from all tags (subjects, hashtags, etc.) are also tokenized
3. Each word is normalized to lowercase ASCII (decorative Unicode → ASCII via NFKD)
4. Words shorter than 2 characters are discarded
5. URLs, Nostr URIs (`nostr:...`), and 64-character hex strings are skipped entirely
6. Each word is hashed with SHA-256, truncated to 8 bytes, and hex-encoded (16 chars)
7. A `(:Word {hash, text})` node is MERGE'd (created if new, reused if existing)
8. A `[:HAS_WORD]` relationship is created from the event to the word

**The hash serves as the unique key** (~10^19 possible values — far beyond all human language words). The `text` property stores the readable word for debugging and direct Cypher queries.

### Deployment: Upgrading to v0.60.5+

To enable word search on an existing Neo4j deployment:

1. **Update the relay binary** to v0.60.5 or later
2. **Restart the relay** — no config changes needed
3. **Wait for migration v6** to complete

On startup, the relay automatically:
- Creates the `Word` uniqueness constraint and index
- Runs migration v6 to backfill Word nodes for all existing events

**Monitoring migration progress** — watch the relay logs:
```
[INFO] applying migration v6: Backfill Word nodes and HAS_WORD relationships for NIP-50 search
[INFO] backfilling word index for 50000 events...
[INFO] backfilled word index: 500/50000 events processed
[INFO] backfilled word index: 1000/50000 events processed
...
[INFO] word index backfill complete: 50000 events processed
[INFO] migration v6 completed successfully
```

The migration processes events in batches of 500. For a relay with 50,000 events, expect it to take a few minutes. For a relay with millions of events, it may take longer but runs non-blocking — the relay serves requests during migration, search results will simply be incomplete until it finishes.

**Verifying migration completion** — check for the Migration marker node:
```cypher
MATCH (m:Migration {version: "v6"})
RETURN m.version, m.description, m.applied_at
```

**Verifying word index health:**
```cypher
-- Count indexed events and unique words
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
RETURN count(DISTINCT e) AS events_indexed,
       count(DISTINCT w) AS unique_words

-- Check a specific word exists
MATCH (w:Word {text: "bitcoin"})
RETURN w.hash, w.text
```

### Querying via NIP-50 (Nostr Protocol)

Clients send a standard NIP-50 `REQ` message with a `search` field in the filter:

```json
["REQ", "sub-id", {"search": "bitcoin lightning", "limit": 20}]
```

The relay tokenizes the search string using the same pipeline as indexing (lowercase, ASCII normalization, hash lookup), then traverses the word graph to find matching events.

**Search can be combined with any standard NIP-01 filter:**

```json
["REQ", "sub-id", {
  "search": "bitcoin",
  "kinds": [1],
  "authors": ["abc123..."],
  "since": 1700000000,
  "#t": ["cryptocurrency"],
  "limit": 50
}]
```

All filter clauses are applied as additional `WHERE` conditions on the search query, so the index is used for both word matching and filter narrowing in a single graph traversal.

#### Relevance Scoring

Results are ranked using a **50/50 blend** of match relevance and recency:

```
score = 0.5 × (matchCount / maxMatchCount) + 0.5 × (recencyRatio)
```

Where:
- `matchCount` = number of search terms matched by this event
- `maxMatchCount` = highest match count across all results
- `recencyRatio` = `(event.created_at - oldest) / (newest - oldest)`

This means an event matching all search terms AND being recent ranks highest, while an event matching fewer terms or being older ranks lower. Events with equal scores are ordered by `created_at DESC`.

#### Example: Multi-Term Search

For `"bitcoin lightning network"`:
- An event containing all 3 words → matchCount = 3 (highest relevance)
- An event containing "bitcoin" and "lightning" → matchCount = 2
- An event containing only "bitcoin" → matchCount = 1
- An event containing none → not returned

### Querying via Bolt+S (Direct Cypher)

If bolt+s is enabled (see [Bolt+S External Access](#bolts-external-access-remote-cypher-queries)), you can query the word index directly with Cypher. This is useful for analytics, debugging, and building custom search experiences outside the Nostr protocol.

#### Find Events Containing a Word

```cypher
MATCH (e:Event)-[:HAS_WORD]->(w:Word {text: "bitcoin"})
RETURN e.id, e.content, e.created_at
ORDER BY e.created_at DESC
LIMIT 20
```

#### Multi-Word Intersection (AND Logic)

Find events containing ALL specified words:

```cypher
WITH ["bitcoin", "lightning", "network"] AS searchTerms
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE w.text IN searchTerms
WITH e, count(DISTINCT w) AS matchCount, size(searchTerms) AS totalTerms
WHERE matchCount = totalTerms
RETURN e.id, e.content, e.created_at
ORDER BY e.created_at DESC
LIMIT 20
```

#### Multi-Word Union with Ranking (OR Logic)

Find events containing ANY of the words, ranked by how many they match:

```cypher
WITH ["bitcoin", "lightning", "network"] AS searchTerms
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE w.text IN searchTerms
WITH e, count(DISTINCT w) AS matchCount
RETURN e.id, e.content, e.created_at, matchCount
ORDER BY matchCount DESC, e.created_at DESC
LIMIT 50
```

#### Word Co-Occurrence Analysis

Find words that frequently appear alongside "bitcoin":

```cypher
MATCH (e:Event)-[:HAS_WORD]->(w1:Word {text: "bitcoin"})
MATCH (e)-[:HAS_WORD]->(w2:Word)
WHERE w2.text <> "bitcoin"
RETURN w2.text AS related_word, count(e) AS co_occurrences
ORDER BY co_occurrences DESC
LIMIT 20
```

#### Author's Vocabulary Profile

Find the most-used words by a specific author:

```cypher
MATCH (e:Event {pubkey: $pubkey})-[:HAS_WORD]->(w:Word)
RETURN w.text, count(e) AS usage
ORDER BY usage DESC
LIMIT 50
```

#### Trending Words Over Time

Find words that appeared most in the last 24 hours:

```cypher
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE e.created_at > (timestamp() / 1000 - 86400)
RETURN w.text, count(e) AS mentions
ORDER BY mentions DESC
LIMIT 30
```

#### Hash-Based Queries

For programmatic access, you can query by hash directly (avoiding normalization edge cases):

```cypher
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE w.hash IN ["a1b2c3d4e5f6a7b8", "f8e7d6c5b4a39281"]
RETURN e.id, e.content, count(DISTINCT w) AS matchCount
ORDER BY matchCount DESC
LIMIT 20
```

The hash is the first 8 bytes of `SHA-256(lowercase_word)`, hex-encoded to 16 characters.

### What Gets Indexed (and What Doesn't)

| Content Type | Indexed? | Example |
|-------------|----------|---------|
| Regular words | Yes | "Bitcoin is great" → `bitcoin`, `is`, `great` |
| Tag values | Yes | `["t", "cryptocurrency"]` → `cryptocurrency` |
| Subject tags | Yes | `["subject", "decentralized finance"]` → `decentralized`, `finance` |
| URLs | **No** | `https://example.com/page` → skipped entirely |
| Nostr URIs | **No** | `nostr:npub1abc...` → skipped entirely |
| Hex strings (64 chars) | **No** | Event IDs, pubkeys → skipped |
| Single characters | **No** | "a", "I" → too short (< 2 chars) |
| Mixed case | Normalized | "BITCOIN", "Bitcoin" → `bitcoin` |
| Decorative Unicode | Normalized | "𝗕𝗶𝘁𝗰𝗼𝗶𝗻" → `bitcoin` |
| Emoji | **No** | Emoji are not alphabetic words |

### Performance Characteristics

The word search index uses Neo4j's native graph traversal rather than a Lucene-based fulltext index. This has specific trade-offs:

**Strengths:**
- Consistent with the Badger backend's approach (same tokenization, same scoring)
- Word nodes are naturally deduplicated — "bitcoin" is one node regardless of how many events reference it
- Combining search with graph traversals (authors, tags, kinds) is a single query
- Adding new events is O(words) — each word is a MERGE + relationship CREATE

**Considerations:**
- Each word adds a node and relationship to the graph, so the database size grows proportional to total word count
- Very common words (stopwords like "the", "is") create high-degree Word nodes — Neo4j handles these well but they add little search value
- The 8-byte hash provides ~10^19 possible values, sufficient for all human languages combined

### Migration Details (v6)

The v6 migration backfills Word nodes for all events that existed before upgrading:

1. Finds all events without any `HAS_WORD` relationships
2. Processes them in batches of 500
3. For each event: tokenizes content + all tag values
4. Creates Word nodes and HAS_WORD relationships using `UNWIND` batch pattern
5. Logs progress every batch

The migration is **idempotent** — it only processes events missing word relationships, so it's safe to restart mid-migration. If the relay crashes during migration, it will resume from where it left off on the next startup.

The migration marker is stored as a `(:Migration {version: "v6"})` node. Once this node exists, the migration is skipped on subsequent startups.

## Future Enhancements

1. ~~**Full-text Search**: Leverage Neo4j's full-text indexes for content search~~ — **Done in v0.60.5** (graph-based word index via NIP-50)
2. **Graph Analytics**: Implement social graph metrics (centrality, communities)
3. ~~**Advanced Queries**: Support NIP-50 search via Cypher full-text capabilities~~ — **Done in v0.60.5**
4. **Clustering**: Deploy Neo4j cluster for high availability
5. **APOC Procedures**: Utilize APOC library for advanced graph algorithms
6. **Caching Layer**: Implement query result caching similar to Badger backend
7. **Stopword Filtering**: Optionally skip indexing high-frequency words ("the", "is", "and") to reduce graph size

## Troubleshooting

### Connection Issues

```bash
# Test connectivity
cypher-shell -a bolt://localhost:7687 -u neo4j -p password

# Check Neo4j logs
docker logs neo4j
```

### Performance Issues

```cypher
// View query execution plan
EXPLAIN MATCH (e:Event) WHERE e.kind = 1 RETURN e LIMIT 10

// Profile query performance
PROFILE MATCH (e:Event)-[:AUTHORED_BY]->(a:Author) RETURN e, a LIMIT 10
```

### Schema Issues

```cypher
// List all constraints
SHOW CONSTRAINTS

// List all indexes
SHOW INDEXES

// Drop and recreate schema
DROP CONSTRAINT event_id_unique IF EXISTS
CREATE CONSTRAINT event_id_unique FOR (e:Event) REQUIRE e.id IS UNIQUE
```

## References

- [Neo4j Documentation](https://neo4j.com/docs/)
- [Cypher Query Language](https://neo4j.com/docs/cypher-manual/current/)
- [Neo4j Go Driver](https://neo4j.com/docs/go-manual/current/)
- [Graph Database Patterns](https://neo4j.com/developer/graph-db-vs-rdbms/)
- [Nostr Protocol (NIP-01)](https://github.com/nostr-protocol/nips/blob/master/01.md)

## License

This Neo4j backend implementation follows the same license as the ORLY relay project.
