#!/usr/bin/env bun
/**
 * cypher.ts — Interactive Cypher query tool for ORLY's Neo4j backend.
 *
 * Connect to a Neo4j instance (local or remote bolt+s) and run Cypher
 * queries against the Nostr event graph.
 *
 * Usage:
 *   bun run cypher.ts                                     # interactive REPL
 *   bun run cypher.ts --uri "bolt+s://relay.example.com:7687"  # remote
 *   echo "MATCH (e:Event) RETURN count(e)" | bun run cypher.ts # piped
 *   bun run cypher.ts --example kinds                     # run built-in example
 *   bun run cypher.ts --file my_query.cypher              # run from file
 */

import neo4j, { type Record as Neo4jRecord } from "neo4j-driver";
import { createInterface } from "readline";
import { parseArgs } from "util";

// ── Built-in example queries ────────────────────────────────────
const EXAMPLES: Record<string, { name: string; description: string; query: string }> = {
  count: {
    name: "Count everything",
    description: "How many events are in the database?",
    query: "MATCH (e:Event) RETURN count(e) AS events",
  },
  kinds: {
    name: "Event kind distribution",
    description: "What kinds of Nostr events are stored?",
    query: `
      MATCH (e:Event)
      RETURN e.kind AS kind, count(e) AS total
      ORDER BY total DESC LIMIT 15`,
  },
  "top-authors": {
    name: "Most active authors",
    description: "Who has published the most events?",
    query: `
      MATCH (e:Event)-[:AUTHORED_BY]->(a:NostrUser)
      RETURN a.pubkey AS pubkey, count(e) AS events
      ORDER BY events DESC LIMIT 10`,
  },
  "popular-tags": {
    name: "Popular hashtags",
    description: "Which #t hashtags are used most often?",
    query: `
      MATCH (e:Event)-[:TAGGED_WITH]->(t:Tag {type: 't'})
      RETURN t.value AS hashtag, count(e) AS usage
      ORDER BY usage DESC LIMIT 20`,
  },
  "most-mentioned": {
    name: "Most mentioned users",
    description: "Which pubkeys get mentioned (p-tagged) the most?",
    query: `
      MATCH (e:Event)-[:MENTIONS]->(u:NostrUser)
      RETURN u.pubkey AS pubkey, count(e) AS mentions
      ORDER BY mentions DESC LIMIT 10`,
  },
  "reply-chains": {
    name: "Reply chain depth",
    description: "How deep do reply chains go? (max 5 hops)",
    query: `
      MATCH path = (reply:Event)-[:REFERENCES*1..5]->(root:Event)
      WHERE NOT (root)-[:REFERENCES]->()
      RETURN length(path) AS depth, count(path) AS chains
      ORDER BY depth`,
  },
  "social-graph": {
    name: "Follow graph (kind 3 contact lists)",
    description: "Who follows the most people?",
    query: `
      MATCH (e:Event {kind: 3})-[:AUTHORED_BY]->(follower:NostrUser)
      MATCH (e)-[:MENTIONS]->(followed:NostrUser)
      RETURN follower.pubkey AS follower,
             count(DISTINCT followed) AS following
      ORDER BY following DESC LIMIT 10`,
  },
  recent: {
    name: "Most recent events",
    description: "The newest events by created_at timestamp.",
    query: `
      MATCH (e:Event)-[:AUTHORED_BY]->(a:NostrUser)
      RETURN e.id AS id,
             e.kind AS kind,
             a.pubkey AS author,
             e.created_at AS timestamp,
             substring(e.content, 0, 60) AS preview
      ORDER BY e.created_at DESC LIMIT 10`,
  },
  schema: {
    name: "Database labels",
    description: "What node types exist in the graph?",
    query: "CALL db.labels() YIELD label RETURN label ORDER BY label",
  },
  relationships: {
    name: "Relationship types and counts",
    description: "What relationship types exist and how many of each?",
    query: `
      MATCH ()-[r]->()
      RETURN type(r) AS relationship, count(r) AS total
      ORDER BY total DESC`,
  },
};

// ── Formatting ──────────────────────────────────────────────────
function formatValue(val: any): string {
  if (val === null || val === undefined) return "<null>";
  // neo4j integers
  if (neo4j.isInt(val)) return val.toString();
  if (Array.isArray(val) || typeof val === "object") {
    return JSON.stringify(val);
  }
  const s = String(val);
  return s.length > 80 ? s.slice(0, 77) + "..." : s;
}

function printResults(records: Neo4jRecord[]) {
  if (!records.length) {
    console.log("  (no results)");
    return;
  }

  const keys = records[0].keys;
  if (!keys.length) {
    console.log("  (empty result set)");
    return;
  }

  const rows = records.map((r) => keys.map((k) => formatValue(r.get(k))));
  const widths = keys.map((k, i) => {
    let w = k.length;
    for (const row of rows) w = Math.max(w, row[i].length);
    return Math.min(w, 60);
  });

  const header = keys.map((k, i) => String(k).padEnd(widths[i])).join(" | ");
  const sep = widths.map((w) => "-".repeat(w)).join("-+-");
  console.log(`  ${header}`);
  console.log(`  ${sep}`);

  for (const row of rows) {
    const cells = row.map((cell, i) => {
      if (cell.length > widths[i]) cell = cell.slice(0, widths[i] - 3) + "...";
      return cell.padEnd(widths[i]);
    });
    console.log(`  ${cells.join(" | ")}`);
  }
  console.log(`\n  (${records.length} row${records.length !== 1 ? "s" : ""})`);
}

// ── Query execution ─────────────────────────────────────────────
async function runQuery(session: neo4j.Session, query: string) {
  query = query.trim();
  if (!query) return;

  const t0 = Date.now();
  try {
    const result = await session.run(query);
    printResults(result.records);
  } catch (e: any) {
    const msg = e.message || String(e);
    if (msg.includes("SyntaxError") || msg.includes("Invalid input")) {
      console.log(`\n  Syntax error: ${msg}`);
      console.log("  Tip: Labels are case-sensitive — use Event, not event.");
    } else if (msg.includes("write") || msg.includes("read only")) {
      console.log(`\n  Write error: ${msg}`);
      console.log("  Tip: Use --write to allow write queries.");
    } else {
      console.log(`\n  Error: ${msg}`);
    }
    return;
  }
  const elapsed = ((Date.now() - t0) / 1000).toFixed(3);
  console.log(`  Query took ${elapsed}s`);
}

// ── CLI argument parsing ────────────────────────────────────────
const { values: cliArgs } = parseArgs({
  options: {
    uri:      { type: "string", default: "bolt://localhost:7687" },
    user:     { type: "string", default: "neo4j" },
    password: { type: "string", default: "nostr-demo-2024" },
    write:    { type: "boolean", default: false },
    file:     { type: "string", short: "f" },
    example:  { type: "string", short: "e" },
    "list-examples": { type: "boolean", default: false },
    help:     { type: "boolean", default: false },
  },
});

function showExamples() {
  console.log("\n  Built-in example queries:");
  console.log("  " + "=".repeat(60));
  for (const [key, ex] of Object.entries(EXAMPLES)) {
    console.log(`\n  [${key}] ${ex.name}`);
    console.log(`    ${ex.description}`);
  }
  console.log(`\n  Run one:  bun run cypher.ts --example kinds`);
  console.log(`  Or in REPL: type 'run kinds'\n`);
}

function showHelp() {
  console.log(`
  cypher.ts — Interactive Cypher query tool for ORLY Neo4j

  Usage:
    bun run cypher.ts [options]

  Options:
    --uri <uri>          Neo4j bolt URI (default: bolt://localhost:7687)
    --user <user>        Username (default: neo4j)
    --password <pass>    Password (default: nostr-demo-2024)
    --write              Allow write queries (default: read-only)
    --file, -f <path>    Run query from a .cypher file
    --example, -e <name> Run a built-in example
    --list-examples      List built-in examples and exit
    --help               Show this help

  Interactive commands:
    examples             List built-in example queries
    run <name>           Run a built-in example
    show <name>          Print an example query without running it
    help                 Show interactive help
    quit                 Exit
  `);
}

// ── Interactive REPL ────────────────────────────────────────────
async function repl(session: neo4j.Session) {
  const mode = cliArgs.write ? "read-write" : "read-only";
  console.log(`
  ┌──────────────────────────────────────────────┐
  │  ORLY Cypher Bridge — Interactive Query Tool  │
  │                                               │
  │  Mode: ${mode.padEnd(10)}                            │
  │  Type 'help' for commands                     │
  │  Type 'examples' for sample queries           │
  │  Type 'quit' to exit                          │
  └──────────────────────────────────────────────┘
  `);

  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: process.stdin.isTTY ?? false,
  });

  let buffer: string[] = [];
  let prompt = "cypher> ";

  const askLine = () =>
    new Promise<string | null>((resolve) => {
      rl.question(prompt, (answer) => resolve(answer));
      rl.once("close", () => resolve(null));
    });

  while (true) {
    const line = await askLine();
    if (line === null) {
      console.log("\nBye!");
      break;
    }

    const stripped = line.trim();

    // Handle commands when buffer is empty
    if (!buffer.length) {
      const lower = stripped.toLowerCase();

      if (lower === "quit" || lower === "exit" || lower === "q") {
        console.log("Bye!");
        break;
      }
      if (lower === "help") {
        showHelp();
        continue;
      }
      if (lower === "examples") {
        showExamples();
        continue;
      }
      if (lower.startsWith("run ")) {
        const name = stripped.slice(4).trim();
        const ex = EXAMPLES[name];
        if (ex) {
          console.log(`\n  Running: ${ex.name}`);
          console.log(`  ${ex.description}\n`);
          await runQuery(session, ex.query);
          console.log();
        } else {
          console.log(`  Unknown example: '${name}'. Type 'examples' to see available ones.`);
        }
        continue;
      }
      if (lower.startsWith("show ")) {
        const name = stripped.slice(5).trim();
        const ex = EXAMPLES[name];
        if (ex) {
          console.log(`\n  [${name}] ${ex.name}`);
          console.log(`  ${ex.description}\n`);
          for (const qline of ex.query.trim().split("\n")) {
            console.log(`    ${qline.trim()}`);
          }
          console.log();
        } else {
          console.log(`  Unknown example: '${name}'.`);
        }
        continue;
      }
      if (!stripped) continue;
    }

    // Accumulate multi-line queries
    buffer.push(line);
    const full = buffer.join("\n");

    // Query is complete if it ends with ; or is a single short statement
    const MULTILINE_STARTERS = [
      "MATCH", "WITH", "OPTIONAL", "CALL", "UNWIND",
      "CREATE", "MERGE", "DELETE", "SET", "REMOVE", "FOREACH", "LOAD",
    ];
    const isMultilineStart = MULTILINE_STARTERS.some((kw) =>
      stripped.toUpperCase().startsWith(kw)
    );

    if (
      stripped.endsWith(";") ||
      (buffer.length === 1 && !isMultilineStart)
    ) {
      const query = full.replace(/;\s*$/, "");
      console.log();
      await runQuery(session, query);
      console.log();
      buffer = [];
      prompt = "cypher> ";
    } else {
      prompt = "   ...> ";
    }
  }

  rl.close();
}

// ── Main ────────────────────────────────────────────────────────
async function main() {
  if (cliArgs.help) {
    showHelp();
    process.exit(0);
  }

  if (cliArgs["list-examples"]) {
    showExamples();
    process.exit(0);
  }

  // Connect
  let driver: neo4j.Driver;
  try {
    driver = neo4j.driver(
      cliArgs.uri!,
      neo4j.auth.basic(cliArgs.user!, cliArgs.password!)
    );
    await driver.verifyConnectivity();
  } catch (e: any) {
    console.error(`Error: Cannot connect to Neo4j at ${cliArgs.uri}`);
    console.error(`  ${e.message}`);
    console.error();
    console.error("Troubleshooting:");
    console.error("  1. Is Neo4j running?  docker ps | grep neo4j");
    console.error("  2. Correct URI?       bolt://localhost:7687 (local)");
    console.error("                        bolt+s://relay.example.com:7687 (remote)");
    console.error("  3. Correct password?  Check ORLY_NEO4J_PASSWORD");
    process.exit(1);
  }

  const session = driver.session();

  try {
    // --example: run a built-in example
    if (cliArgs.example) {
      const ex = EXAMPLES[cliArgs.example];
      if (!ex) {
        console.error(`Unknown example: '${cliArgs.example}'`);
        console.error(`Available: ${Object.keys(EXAMPLES).join(", ")}`);
        process.exit(1);
      }
      console.log(`\n  ${ex.name}: ${ex.description}\n`);
      await runQuery(session, ex.query);
      console.log();
      return;
    }

    // --file: run from .cypher file
    if (cliArgs.file) {
      const file = Bun.file(cliArgs.file);
      if (!(await file.exists())) {
        console.error(`File not found: ${cliArgs.file}`);
        process.exit(1);
      }
      const query = await file.text();
      console.log(`  Running query from ${cliArgs.file}\n`);
      await runQuery(session, query);
      console.log();
      return;
    }

    // Piped input
    if (!process.stdin.isTTY) {
      const chunks: Buffer[] = [];
      for await (const chunk of process.stdin) {
        chunks.push(chunk as Buffer);
      }
      const query = Buffer.concat(chunks).toString().trim();
      if (query) {
        await runQuery(session, query);
        console.log();
      }
      return;
    }

    // Interactive REPL
    await repl(session);
  } finally {
    await session.close();
    await driver.close();
  }
}

main();
