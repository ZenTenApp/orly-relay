#!/usr/bin/env bun
/**
 * seed.ts — Seed a Neo4j database with Nostr events from JSONL.
 *
 * Creates the same graph structure that ORLY uses:
 *   Event nodes ──AUTHORED_BY──> NostrUser
 *   Event nodes ──REFERENCES───> Event      (e-tags)
 *   Event nodes ──MENTIONS─────> NostrUser  (p-tags)
 *   Event nodes ──TAGGED_WITH──> Tag        (t, d, r, etc.)
 *
 * Usage:
 *   bun run seed.ts                              # defaults: localhost, 500 events
 *   bun run seed.ts --uri bolt://host:7687       # custom URI
 *   bun run seed.ts --limit 2000                 # seed more events
 *   bun run seed.ts --all                        # seed the entire dataset (~11k events)
 *   bun run seed.ts --clean                      # wipe DB first
 */

import neo4j from "neo4j-driver";
import { parseArgs } from "util";
import { resolve } from "path";
import { homedir } from "os";

// ── Defaults ────────────────────────────────────────────────────
const DEFAULT_DATA = resolve(
  homedir(),
  "src/git.smesh.lol/orly/pkg/nostr/encoders/event/examples/out.jsonl"
);

// ── Schema (matches ORLY's pkg/neo4j/schema.go) ────────────────
const CONSTRAINTS = [
  "CREATE CONSTRAINT event_id_unique IF NOT EXISTS FOR (e:Event) REQUIRE e.id IS UNIQUE",
  "CREATE CONSTRAINT nostrUser_pubkey IF NOT EXISTS FOR (n:NostrUser) REQUIRE n.pubkey IS UNIQUE",
];

const INDEXES = [
  "CREATE INDEX event_kind IF NOT EXISTS FOR (e:Event) ON (e.kind)",
  "CREATE INDEX event_created_at IF NOT EXISTS FOR (e:Event) ON (e.created_at)",
  "CREATE INDEX tag_type IF NOT EXISTS FOR (t:Tag) ON (t.type)",
  "CREATE INDEX tag_value IF NOT EXISTS FOR (t:Tag) ON (t.value)",
  "CREATE INDEX tag_type_value IF NOT EXISTS FOR (t:Tag) ON (t.type, t.value)",
  "CREATE INDEX event_kind_created_at IF NOT EXISTS FOR (e:Event) ON (e.kind, e.created_at)",
];

// ── Parse CLI args ──────────────────────────────────────────────
const { values: args } = parseArgs({
  options: {
    uri:      { type: "string", default: "bolt://localhost:7687" },
    user:     { type: "string", default: "neo4j" },
    password: { type: "string", default: "nostr-demo-2024" },
    data:     { type: "string", default: DEFAULT_DATA },
    limit:    { type: "string", default: "500" },
    all:      { type: "boolean", default: false },
    clean:    { type: "boolean", default: false },
    help:     { type: "boolean", default: false },
  },
});

if (args.help) {
  console.log(`
  seed.ts — Seed Neo4j with Nostr events

  Options:
    --uri <uri>          Neo4j bolt URI (default: bolt://localhost:7687)
    --user <user>        Neo4j username (default: neo4j)
    --password <pass>    Neo4j password (default: nostr-demo-2024)
    --data <path>        Path to JSONL event data
    --limit <n>          Max events to seed (default: 500)
    --all                Seed ALL events (ignores --limit)
    --clean              Wipe the database before seeding
    --help               Show this help
  `);
  process.exit(0);
}

const limit = args.all ? Infinity : parseInt(args.limit!, 10);

// ── Load events from JSONL ──────────────────────────────────────
interface NostrEvent {
  id: string;
  pubkey: string;
  kind: number;
  created_at: number;
  content: string;
  sig: string;
  tags: string[][];
}

async function loadEvents(path: string, max: number): Promise<NostrEvent[]> {
  const file = Bun.file(path);
  if (!(await file.exists())) {
    console.error(`Error: Data file not found: ${path}`);
    process.exit(1);
  }

  const text = await file.text();
  const lines = text.trim().split("\n");
  const events: NostrEvent[] = [];
  for (let i = 0; i < lines.length && i < max; i++) {
    events.push(JSON.parse(lines[i]));
  }
  return events;
}

// ── Main ────────────────────────────────────────────────────────
async function main() {
  console.log(`Connecting to Neo4j at ${args.uri} ...`);
  const driver = neo4j.driver(
    args.uri!,
    neo4j.auth.basic(args.user!, args.password!)
  );

  try {
    await driver.verifyConnectivity();
  } catch (e: any) {
    console.error(`Cannot connect: ${e.message}`);
    console.error("Is Neo4j running? Try: docker ps | grep neo4j");
    process.exit(1);
  }
  console.log("  Connected.");

  const session = driver.session();

  try {
    // Clean if requested
    if (args.clean) {
      console.log("Wiping database ...");
      await session.run("MATCH (n) DETACH DELETE n");
      console.log("  Done.");
    }

    // Apply schema
    console.log("Applying schema ...");
    for (const cypher of [...CONSTRAINTS, ...INDEXES]) {
      await session.run(cypher);
    }
    console.log("  Schema applied (constraints + indexes).");

    // Load events
    console.log(`Loading events from ${args.data} ...`);
    const events = await loadEvents(args.data!, limit);
    console.log(`  Loaded ${events.length} events.`);

    // Seed in batches
    console.log("Seeding graph ...");
    const t0 = Date.now();
    const batchSize = 50;

    for (let i = 0; i < events.length; i += batchSize) {
      const batch = events.slice(i, i + batchSize);
      await session.executeWrite(async (tx) => {
        // Create Event + NostrUser nodes with AUTHORED_BY
        const batchData = batch.map((ev) => ({
          id: ev.id,
          kind: neo4j.int(ev.kind),
          created_at: neo4j.int(ev.created_at),
          content: ev.content || "",
          sig: ev.sig || "",
          pubkey: ev.pubkey,
          tags: JSON.stringify(ev.tags || []),
        }));

        await tx.run(
          `UNWIND $events AS ev
           MERGE (e:Event {id: ev.id})
           SET e.kind = ev.kind,
               e.created_at = ev.created_at,
               e.content = ev.content,
               e.sig = ev.sig,
               e.pubkey = ev.pubkey,
               e.tags = ev.tags
           MERGE (a:NostrUser {pubkey: ev.pubkey})
           MERGE (e)-[:AUTHORED_BY]->(a)`,
          { events: batchData }
        );

        // Create tag relationships for each event
        for (const ev of batch) {
          for (const tag of ev.tags || []) {
            if (tag.length < 2 || !tag[1]) continue;
            const [tagType, tagValue] = tag;

            if (tagType === "e") {
              await tx.run(
                `MATCH (src:Event {id: $srcId})
                 MERGE (tgt:Event {id: $refId})
                 MERGE (src)-[:REFERENCES]->(tgt)`,
                { srcId: ev.id, refId: tagValue }
              );
            } else if (tagType === "p") {
              await tx.run(
                `MATCH (src:Event {id: $srcId})
                 MERGE (mentioned:NostrUser {pubkey: $pubkey})
                 MERGE (src)-[:MENTIONS]->(mentioned)`,
                { srcId: ev.id, pubkey: tagValue }
              );
            } else {
              await tx.run(
                `MATCH (src:Event {id: $srcId})
                 MERGE (t:Tag {type: $type, value: $value})
                 MERGE (src)-[:TAGGED_WITH]->(t)`,
                { srcId: ev.id, type: tagType, value: tagValue }
              );
            }
          }
        }
      });

      const done = Math.min(i + batchSize, events.length);
      const pct = Math.floor((done * 100) / events.length);
      process.stdout.write(`\r  Progress: ${done}/${events.length} (${pct}%)`);
    }

    const elapsed = ((Date.now() - t0) / 1000).toFixed(1);
    console.log(`\n  Seeded ${events.length} events in ${elapsed}s`);

    // Print summary
    const stats = await session.run(`
      MATCH (e:Event) WITH count(e) AS events
      MATCH (a:NostrUser) WITH events, count(a) AS users
      MATCH (t:Tag) WITH events, users, count(t) AS tags
      RETURN events, users, tags
    `);
    const row = stats.records[0];
    console.log(`\n  Graph summary:`);
    console.log(`    Events:    ${row.get("events")}`);
    console.log(`    Users:     ${row.get("users")}`);
    console.log(`    Tags:      ${row.get("tags")}`);

    const rels = await session.run(`
      MATCH ()-[r:AUTHORED_BY]->() WITH count(r) AS authored
      MATCH ()-[r:REFERENCES]->() WITH authored, count(r) AS refs
      MATCH ()-[r:MENTIONS]->() WITH authored, refs, count(r) AS mentions
      MATCH ()-[r:TAGGED_WITH]->() WITH authored, refs, mentions, count(r) AS tagged
      RETURN authored, refs, mentions, tagged
    `);
    const rrow = rels.records[0];
    console.log(`    AUTHORED_BY: ${rrow.get("authored")}`);
    console.log(`    REFERENCES:  ${rrow.get("refs")}`);
    console.log(`    MENTIONS:    ${rrow.get("mentions")}`);
    console.log(`    TAGGED_WITH: ${rrow.get("tagged")}`);

    console.log("\nDone! The database is ready for Cypher queries.");
  } finally {
    await session.close();
    await driver.close();
  }
}

main();
