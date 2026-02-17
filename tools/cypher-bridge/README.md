# Cypher Bridge — Query ORLY's Neo4j Graph

A friendly command-line tool for running [Cypher](https://neo4j.com/docs/cypher-manual/current/) queries against ORLY's Neo4j Nostr event graph. Comes with a seed script to populate a local Neo4j instance with real Nostr events, and an interactive REPL so you can explore the data immediately.

## What This Is

When ORLY runs with `ORLY_DB_TYPE=neo4j`, it stores Nostr events as a graph:

```
(Event)──AUTHORED_BY──>(NostrUser)     "Alice wrote this note"
(Event)──REFERENCES───>(Event)          "This note replies to that note"
(Event)──MENTIONS─────>(NostrUser)     "This note mentions Bob"
(Event)──TAGGED_WITH──>(Tag)            "This note is tagged #bitcoin"
```

This tool lets you query that graph directly using Cypher — Neo4j's query language. Think of it like SQL, but for graphs.

## Quick Start

### 1. Get Neo4j Running

The fastest way is Docker:

```bash
docker run -d --name neo4j-demo \
  -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/nostr-demo-2024 \
  neo4j:5.15
```

Wait ~15 seconds for it to start. You can check with:

```bash
docker logs neo4j-demo 2>&1 | tail -3
# Should show "Started." when ready
```

### 2. Install Dependencies

```bash
cd tools/cypher-bridge
bun install
```

### 3. Seed the Database

This loads 500 real Nostr events into Neo4j and creates the full graph structure (authors, tags, references, mentions):

```bash
bun run seed
```

Want more data? Use `--limit 2000` or `--all` for the full ~11,600 event dataset.

### 4. Start Querying

```bash
bun run cypher
```

You'll see:

```
  ┌──────────────────────────────────────────────┐
  │  ORLY Cypher Bridge — Interactive Query Tool  │
  │                                               │
  │  Mode: read-only                              │
  │  Type 'help' for commands                     │
  │  Type 'examples' for sample queries           │
  │  Type 'quit' to exit                          │
  └──────────────────────────────────────────────┘

cypher>
```

Type `examples` to see built-in queries, or just start typing Cypher.

## Usage Modes

### Interactive REPL

```bash
bun run cypher
```

Type Cypher queries directly. Multi-line queries end with a semicolon (`;`).

```
cypher> MATCH (e:Event)
   ...> WHERE e.kind = 1
   ...> RETURN count(e) AS text_notes;

  text_notes
  ----------
  372

  (1 row)
  Query took 0.070s
```

### Built-in Examples

List them:

```bash
bun run cypher -- --list-examples
```

Run one directly:

```bash
bun run cypher -- --example kinds
bun run cypher -- --example popular-tags
bun run cypher -- --example top-authors
```

Or inside the REPL:

```
cypher> run kinds
cypher> run popular-tags
```

### Piped Queries

```bash
echo "MATCH (e:Event) RETURN count(e) AS total" | bun run cypher
```

### Query Files

Save a query as `.cypher` and run it:

```bash
bun run cypher -- --file my_query.cypher
```

## Connecting to a Remote Relay

If a relay has bolt+s enabled (see the Neo4j tab in the admin UI), you can connect directly:

```bash
bun run cypher -- --uri "bolt+s://relay.example.com:7687" --user neo4j --password <password>
```

This connects over TLS to the relay's live Neo4j database. Be careful — this is the real thing.

## The Graph Schema

Here's what's in the database and how it all connects:

### Nodes

| Label | Properties | What Is It? |
|-------|-----------|-------------|
| **Event** | `id`, `kind`, `created_at`, `content`, `sig`, `pubkey`, `tags` | A Nostr event (note, repost, reaction, contact list, etc.) |
| **NostrUser** | `pubkey` | A Nostr user, identified by their public key |
| **Tag** | `type`, `value` | A tag value (like `t`/`bitcoin` or `d`/`my-article`) |

### Relationships

| Relationship | From → To | What It Means |
|-------------|-----------|---------------|
| **AUTHORED_BY** | Event → NostrUser | "This event was written by this user" |
| **REFERENCES** | Event → Event | "This event references that event" (e-tags: replies, quotes) |
| **MENTIONS** | Event → NostrUser | "This event mentions this user" (p-tags) |
| **TAGGED_WITH** | Event → Tag | "This event has this tag" (t, d, r, and other tag types) |

### Event Kinds

The `kind` field tells you what type of event it is:

| Kind | What It Is |
|------|-----------|
| 0 | Profile metadata (name, picture, about) |
| 1 | Text note (the main "tweet-like" thing) |
| 3 | Contact list (who someone follows) |
| 4 | Encrypted DM |
| 5 | Deletion request |
| 6 | Repost |
| 7 | Reaction (like, dislike, emoji) |
| 9735 | Zap receipt (lightning payment) |
| 10000 | Mute list |
| 10002 | Relay list |

## Example Queries to Try

### "How many text notes are there?"

```cypher
MATCH (e:Event {kind: 1})
RETURN count(e) AS notes
```

### "Show me the 5 most active authors and their note counts"

```cypher
MATCH (e:Event {kind: 1})-[:AUTHORED_BY]->(a:NostrUser)
RETURN a.pubkey AS author, count(e) AS notes
ORDER BY notes DESC
LIMIT 5
```

### "What hashtags are trending?"

```cypher
MATCH (e:Event)-[:TAGGED_WITH]->(t:Tag {type: 't'})
RETURN t.value AS hashtag, count(e) AS usage
ORDER BY usage DESC
LIMIT 10
```

### "Who replies to whom the most?"

```cypher
MATCH (reply:Event)-[:AUTHORED_BY]->(replier:NostrUser),
      (reply)-[:REFERENCES]->(original:Event)-[:AUTHORED_BY]->(op:NostrUser)
WHERE reply.kind = 1
RETURN replier.pubkey AS replier,
       op.pubkey AS original_poster,
       count(reply) AS replies
ORDER BY replies DESC
LIMIT 10
```

### "Find all events that mention a specific user"

```cypher
MATCH (e:Event)-[:MENTIONS]->(u:NostrUser {pubkey: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"})
RETURN e.id, e.kind, substring(e.content, 0, 80) AS preview
ORDER BY e.created_at DESC
LIMIT 20
```

### "Map the social graph — who follows whom?"

```cypher
MATCH (followList:Event {kind: 3})-[:AUTHORED_BY]->(follower:NostrUser),
      (followList)-[:MENTIONS]->(followed:NostrUser)
RETURN follower.pubkey AS follower,
       collect(followed.pubkey)[0..5] AS follows_sample,
       count(followed) AS total_following
ORDER BY total_following DESC
LIMIT 10
```

### "Find events with no replies (lonely notes)"

```cypher
MATCH (e:Event {kind: 1})
WHERE NOT ()-[:REFERENCES]->(e)
RETURN e.id, substring(e.content, 0, 80) AS content
ORDER BY e.created_at DESC
LIMIT 10
```

## Seed Script Options

```bash
# Seed 500 events (default)
bun run seed

# Seed more events
bun run seed -- --limit 2000

# Seed everything (~11,600 events)
bun run seed -- --all

# Wipe and re-seed
bun run seed -- --clean

# Custom Neo4j connection
bun run seed -- --uri bolt://myhost:7687 --user neo4j --password secret
```

The seed data comes from `~/src/git.mleku.dev/mleku/nostr/encoders/event/examples/out.jsonl` — a collection of ~11,600 real Nostr events covering text notes, reposts, reactions, contact lists, and more.

## CLI Options

```bash
bun run cypher -- --help

Options:
  --uri <uri>          Neo4j bolt URI (default: bolt://localhost:7687)
  --user <user>        Username (default: neo4j)
  --password <pass>    Password
  --write              Allow write queries (default: read-only)
  --file, -f <path>    Run query from a .cypher file
  --example, -e <name> Run a built-in example
  --list-examples      List built-in examples and exit
  --help               Show help
```

## How This Relates to ORLY

This tool talks to Neo4j the same way ORLY does — via the Bolt protocol. The graph schema here matches what ORLY creates in `pkg/neo4j/schema.go`.

In production, you'd connect to a live relay's Neo4j via bolt+s (encrypted Bolt). The relay owner enables this through the admin UI's Neo4j tab, which modifies `neo4j.conf` to expose the Bolt port with TLS.

The full flow:

1. ORLY relay receives Nostr events via WebSocket
2. ORLY stores them in Neo4j as graph nodes with relationships
3. You connect via bolt+s and run Cypher queries against the live graph
4. This tool is how you do step 3

## Troubleshooting

**"Cannot connect to Neo4j"**
- Is the container running? `docker ps | grep neo4j`
- Is it fully started? `docker logs neo4j-demo 2>&1 | tail -5`
- Port conflict? Check if something else is on 7687: `ss -tlnp | grep 7687`

**"Syntax error" on a query**
- Labels are case-sensitive: `Event` not `event`, `NostrUser` not `nostruser`
- Property names are case-sensitive: `created_at` not `Created_At`
- Strings use single quotes in Cypher: `{kind: 1}` not `{kind: "1"}`

**"Write error" when trying to create/delete**
- The tool runs in read-only mode by default. Use `--write` to allow writes.

**Seed script is slow**
- The bottleneck is creating individual tag relationships. 500 events takes ~20s, the full 11.6k dataset takes a few minutes. This is a one-time cost.
