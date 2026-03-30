# CLAUDE.md — Smesh Project

## Output Discipline

No preamble. No summaries. No explaining what you're about to do before doing it. No recapping what you just did after doing it. No sycophantic or flattering language. No "great question", "excellent point", "absolutely right". No performative enthusiasm. No "I think", "I feel", "I believe", "I'm happy to", "I'm excited to", "Let me help you with". Be brief. Default to the shortest answer that is complete. Code blocks only for actual code. When in doubt, write code, not words.

Do not report on background tasks that failed or were superseded by newer runs. Only the current/active run matters.

## Hard Dependency Rules

**THIS IS THE MOST IMPORTANT SECTION. VIOLATIONS HERE WASTE HOURS.**

This project uses TinyGo compiled to WASM for frontend and TinyGo for backend. There is NO node ecosystem. There is NO npm. There is NO package.json. There are NO JS frameworks.

ALLOWED:
- TinyGo standard library
- Packages already imported in the existing codebase (check go.mod)
- Raw HTML, CSS, JS as static assets per-package

FORBIDDEN — DO NOT ADD UNDER ANY CIRCUMSTANCES:
- nostr-tools or any npm package
- Any node_modules
- Any JS framework (React, Vue, Svelte, etc.)
- Any Go package not already in go.mod without explicit approval
- Any build tool that requires node/npm/yarn/bun
- Any CDN-loaded JS library

Before writing ANY import statement, check go.mod and the existing imports in the package you're editing. If the import doesn't already exist in the project, STOP and ASK before adding it.

After ANY code change, verify:
1. No new dependencies were introduced
2. `go.mod` was not modified (unless explicitly approved)
3. No npm/node artifacts appeared anywhere
4. The code compiles with TinyGo

If you catch yourself reaching for a convenience library, implement it from scratch using stdlib. That is always the correct choice in this project.

## Architecture

Smesh is a Docker-deployable stack: Bitcoin/Lightning, Nostr, Git, project management, p2p marketplace. The web frontend is TinyGo compiled to WASM. HTML and JS dependencies are per-file packages — static assets, not a build pipeline.

Key architectural principles:
- One language (TinyGo) both sides
- Multi-command binaries or standalone TinyGo executables
- No complex layered stacks
- Concurrency via goroutines and channels, not callbacks or promises
- Hot reload via fsnotify before anything else
- Nostr protocol: BIP340 signatures, SHA256 — already implemented in the codebase
- Tagline: "eliding obstraction"

## Change Discipline

- Small commits, one feature at a time
- Verify it compiles AND runs before moving on
- Re-read modified files after changes to catch drift
- If a session produces 1000+ line changes, something went wrong — scope was too broad
- When fixing a bug, understand the root cause before writing code
- Do not refactor adjacent code while fixing a bug
- There is no such thing as an "irrelevant" warning. All console warnings, DOM warnings, and log noise must be fixed. They obscure real issues and increase the time needed to diagnose problems.

## Nostr Protocol Notes

- Event signing: BIP340 Schnorr signatures over SHA256 of serialized event
- Relay communication: WebSocket (primary), SSE (secondary/implemented previously)
- NIPs compliance: check existing implementation before assuming NIP requirements
- DO NOT use nostr-tools. The crypto is implemented natively in TinyGo.
- **DMs are MLS (marmot) only.** The smesh UI has NO NIP-04 (kind 4) and NO NIP-17 (kind 1059 gift wrap) support. All DMs go through MLS_SEND → marmot WASM → signer extension. The email bridge must accept MLS DMs from the browser — there is no fallback protocol.

## Developer Context

Mleku is the sole developer. Based in Zmajevac, Croatia. Works in Claude Code CLI for coding; the claude.ai chat interface is for thinking and debriefing between build sessions.

Coding style: direct, minimal, no abstraction for abstraction's sake. If a function does one thing and is called once, it doesn't need to be in its own file. Concise variable names. Comments only where the why isn't obvious from the what.

## Project Ecosystem

- **Smesh**: the main stack (this project)
- **Dendrite/Iskra**: lattice-based semantic translation engine, TinyGo compiled to WASM, separate project but same language and principles
- **Dragon's Dom**: community space in Zmajevac, website at github.com/DragonsDom/website
- **Dendrita**: hand-made furniture business

## Theoretical Framework (reference only — for when it's relevant)

Mleku's axiom:
- Zero: incoherence = nondeterminism
- Finite: chaos = coherence at insufficient resolution appears incoherent; deception = incoherence at insufficient resolution appears coherent
- Infinity: coherence = determinism

Obstraction: portmanteau of obstruction + abstraction. What layered JS frameworks, npm dependencies, and protocol middleware do to software. The thing this project exists to eliminate.

Wu Xing cycles (generating/overcoming) and I Ching hexagrams are operational tools used in decision-making, not decoration. If encountered in comments or docs, treat as meaningful.

## Physics Papers (summaries for reference)

**Spacetime Subdivision Hypothesis**: spacetime as substance recursively subdividing on Bethe lattice. Gravity=surface tension. Mass=EM-dark electron pair count (dissolves dark matter/energy). c=growth-decay equilibrium. Two experimental devices: resonant cavity (thrust) and Wu Xing pair-breaker (mass reduction via golden-ratio vortices). Closed topology (hall of mirrors). Connects to Dendrite lattice architecture.

**Imaginary Parameters**: i=temporal inversion in every fundamental law. Teleology (final causation) encoded as imaginary sector of physics. T=imaginary time. Absolute zero=infinite backward evolution. Hadamard rotation connects real/imaginary sectors. SVP hardness=third law. Links to hamadryad cryptosystem and Dendrite.

## Local Testing (default for dev)

Build and test locally before deploying to VPS. This is the default workflow.

```sh
# Build orly for local arch
go build -o /tmp/orly-local ./cmd/orly

# Compile all tinyjs targets
./build.sh

# Run with disk mode (fsnotify hot-reload + SSE)
ORLY_SMESH3_DIR=$PWD/app/smesh3 ORLY_SMESH3_PORT=8090 /tmp/orly-local
```

Access at `http://127.0.0.1:8090`. In dev mode the server binds `0.0.0.0` so satellite SWs run on separate origins for thread isolation:
- `127.0.0.1:8090` — main app + shell SW
- `127.0.0.2:8090` — marmot SW (via cross-origin iframe)
- `127.0.0.3:8090` — relay SW (via cross-origin iframe)

All 127.0.0.x are secure contexts (loopback), so SWs register without HTTPS.

After `./build.sh`: fsnotify detects changes → SSE pushes version → browser reloads. No rsync, no VPS.

## VPS Deployment (production)

VPS at 69.164.249.71 (Ubuntu), accessed via `ssh orly` (WireGuard tunnel to 10.0.0.1).

**Stack**: Caddy → orly binary (relay on :3334, smesh2 on :8089, smesh on :8090)
**Domains**: `smesh.lol` (primary), `sm3sh.mleku.dev` (legacy) — both point to :8090
**Subdomains**: `marmot.smesh.lol`, `relay.smesh.lol` for SW thread isolation
**Service**: systemd `orly.service`, binary at `/home/mleku/.local/bin/orly`
**smesh assets**: `/home/mleku/sm3sh/` — served in disk mode, fsnotify watches for changes, SSE pushes version to service worker which reloads all clients.

No Go on VPS. Cross-compile + rsync:
```sh
CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc GOOS=linux GOARCH=amd64 go build -o /tmp/orly-deploy ./cmd/orly
rsync -avz --progress /tmp/orly-deploy orly:/home/mleku/.local/bin/orly.new
rsync -avz --exclude='*.go' app/smesh3/ orly:/home/mleku/sm3sh/
ssh orly "chown mleku:mleku /home/mleku/.local/bin/orly.new && mv /home/mleku/.local/bin/orly{,.bak} && mv /home/mleku/.local/bin/orly{.new,} && chmod +x /home/mleku/.local/bin/orly && systemctl restart orly"
```

**IMPORTANT**: SSH to VPS connects as root. The orly service runs as `User=mleku`. All deployed files must be `chown mleku:mleku` before use — root-owned binaries cause systemd 203/EXEC failures. Use `rsync` not `scp` for the binary (scp can truncate large files over slow links).

Asset-only deploy (no binary change): just rsync, fsnotify triggers reload automatically.

## Version Locations

Two files to bump on every release:
- `pkg/version/version` — canonical version (plain text, embedded into binary)
- `next/sm3sh/main.go` line 13 — `version = "v0.65.44"` (smesh UI display)

Both must stay in sync.

## tinyjs Build

Source: `next/sm3sh/` (app) + `next/common/` (shared libs)
Compile: `cd next/sm3sh && tinyjs -o ../../app/smesh3 .`
Output: `app/smesh3/*.mjs` — module name maps to filename (`common/helpers` → `common_helpers.mjs`)
The .mjs files are gitignored — use `git add -f` when committing.
sw.js `APP_FILES` list must match actual output filenames after any module rename.

### Service Worker tinyjs targets

Each SW compiles to its OWN subdirectory — never to the main app dir. Each subdirectory has its own `$runtime/` with SW-specific APIs (`self.clients`, `Origin()`, etc.) that don't exist in the page runtime.

```sh
# Shell SW (main hub)
cd next/sw && tinyjs -o ../../app/smesh3/\$sw .

# Marmot SW (MLS DM proxy) — runs on marmot.* subdomain
cd next/sw-marmot && tinyjs -o ../../app/smesh3/\$sw-marmot .

# Relay SW (relay pool, IDB cache) — runs on relay.* subdomain
cd next/sw-relay && tinyjs -o ../../app/smesh3/\$sw-relay .
```

**CRITICAL**: Compiling a SW to the wrong directory (e.g. `app/smesh3/` instead of `app/smesh3/$sw-marmot/`) causes it to pick up the page runtime, which lacks SW APIs → `self.clients is undefined` errors.

## Signer Extension Build

Source: `next/signer/` — Angular 19 NIP-07 browser extension (provides `window.nostr` for signing).

```sh
cd next/signer && bun run build:firefox   # compile TS → dist/firefox/
cd next/signer && bun run xpi             # package → next/signer/dist/smesh_signer-*.zip
```

After build, reload in `about:debugging` → "This Firefox" → "Load Temporary Add-on" (pick any file in `dist/firefox/`).

Debug logging: `src/smesh-signer-extension.ts` line 248 — `debug()` function. Enable by uncommenting `console.log` inside it. Produces verbose "getPublicKey received", "nip44.decrypt received" etc. on every NIP-07 call.

## Client Tag

Published events (kind 1, 1111, etc.) should include a `client` tag: `["client","smesh.lol"]`. This is the domain where the app is served, so Nostr clients that display the client name show the actual URL.

Configurable via `ORLY_CLIENT_TAG` env var (default: `smesh.lol`). Server exposes it at `/__client-tag` so the frontend can read it at runtime without recompilation. Self-hosters set their own domain.

## Nostr-First Architecture

**THE VPS IS A DUMB NOSTR RELAY. NOTHING MORE.**

The VPS serves exactly two things:
1. Static files: SW JavaScript, HTML, CSS
2. Nostr relay protocol: EVENT, REQ, CLOSE over WebSocket

ALL application logic runs in the browser. No exceptions. No "smart backend." No custom WebSocket endpoints. No server-side crypto, MLS, message routing, or session management. If code can't compile to run client-side, the answer is to fix the compilation, not to put it on the server.

This is a Nostr app, not a web app. The server doesn't know anything about the user, their keys, their messages, or their state. The relay sees events — that's it.

### SW Mesh Architecture

Three service workers communicate via BroadcastChannel (client-local, no server):

```
Page ↔ Shell SW ($sw/) ↔ BroadcastChannel ↔ Marmot SW ($sw-marmot/)
                                           ↔ Relay SW ($sw-relay/)
```

- **Shell SW**: Message hub, SSE version monitor, cache-first fetch. Runs on main origin.
- **Marmot SW**: MLS DM engine, all MLS crypto runs here. Runs on `marmot.*` subdomain.
- **Relay SW**: Relay pool, subscriptions, IDB DM cache. Runs on `relay.*` subdomain.
- **BroadcastChannel**: Client-local message bus. Fire-and-forget. No server involvement.

Subdomains exist ONLY for browser thread isolation. They serve static SW JS files. They do NOT have custom endpoints.

### SW Logging

SW logs go via BroadcastChannel to the page console. Fire-and-forget — if no page is listening, logs are dropped silently. Never blocking. Never sent to the server.

### What NEVER belongs on the VPS

- `/__bus` or any inter-SW message routing
- `/__marmot` or any MLS/crypto backend
- `/__log` or any log collection
- Crypto proxy, session state, key material, message content
- Anything that makes the server aware of user activity beyond EVENT/REQ

## TinyJS Bridge Invariants

**These rules are derived from the working codebase. Violating them causes silent corruption, not compile errors. Every bug from the past week traces to one of these being broken.**

### 1. The `panic("jsbridge")` Contract

Every Go function in `next/common/jsbridge/*/` that panics with `"jsbridge"` MUST have a matching `export function` with the **exact same name** in the corresponding `$runtime/*.mjs` file. The tinyjs compiler replaces the Go call with a direct JS import at compile time.

- Go stub signature defines the contract; JS implements it
- If you add a Go bridge function, you MUST add the JS export in the same commit
- If you rename a Go bridge function, you MUST rename the JS export
- There are MULTIPLE runtime directories — each SW has its own `$runtime/`:
  - `app/smesh3/$runtime/dom.mjs` — page runtime (DOM, fetch, SW messaging)
  - `app/smesh3/$sw/$runtime/sw.mjs` — shell SW runtime
  - `app/smesh3/$sw-relay/$runtime/sw.mjs` — relay SW runtime
  - `app/smesh3/$sw-relay/$runtime/bc.mjs` — relay bus runtime
  - `app/smesh3/$sw/$runtime/bc.mjs` — shell bus runtime
- `$runtime/*.mjs` files are **hand-written**. All other `.mjs` files in `app/smesh3/` are **tinyjs compiler output** — never edit them by hand
- When adding a function to the shared jsbridge Go package (`next/common/jsbridge/sw/sw.go`), the JS implementation must be added to **every** `$runtime/sw.mjs` that uses it

### 2. Opaque Handle System

All browser objects cross the Go↔JS boundary as **integer handles**. Never pass DOM elements, Response objects, Cache objects, Client objects, or Events directly to Go.

```
Go type     | JS storage        | Meaning
------------|-------------------|------------------
dom.Element | _elements Map     | DOM node
sw.Event    | _events Map       | SW lifecycle event
sw.Client   | _clients Map      | SW client (tab)
sw.Cache    | _caches Map       | CacheStorage cache
sw.Response | _responses Map    | fetch Response
sw.SSE      | _sseConns Map     | EventSource
sw.Timer    | setTimeout return | Timer handle
```

- Handle 0 for `dom.Element` is `document.body` (special case)
- Handle 0 or -1 for other types means "not found" or "error"
- JS `_store*()` functions allocate monotonically increasing IDs
- Handles are never reused, never freed — acceptable for SW lifetime

### 3. Fire-and-Forget Callbacks

All async operations use **callbacks**, never Promises that return to Go. The pattern:

```go
// Go side — callback receives result
sw.Fetch(url, func(resp sw.Response, ok bool) {
    // handle result
})
```

```javascript
// JS side — Promise resolves into callback
export function Fetch(url, fn) {
  fetch(url).then(
    (resp) => fn(_storeResponse(resp), true),
    () => fn(0, false)
  );
}
```

Rules:
- JS async operations resolve into the Go callback, never return a Promise
- Callbacks must always be called exactly once (success or failure path)
- If a callback might never fire (network timeout, etc.), set a safety timeout in Go
- The `done func()` pattern signals completion of multi-step operations (CacheAddAll, ClaimClients)

### 4. String-Only Boundary Crossing

**Go and JS communicate exclusively via strings and integers.** No Go structs cross to JS. No JS objects cross to Go. Structured data crosses as JSON strings.

- `GetMessageData()` returns `typeof d === 'string' ? d : JSON.stringify(d)` — always a string to Go
- `PostToSW(msg)` sends a string from page to SW
- `PostMessageJSON(client, json)` **parses** JSON before posting — receiver gets a real JS array, not a string
- `PostMessage(client, msg)` sends a raw string — receiver gets a string

This distinction is critical:
- **SW→Page messages** use `PostMessageJSON` → page receives `Array` → `OnSWMessage` re-serializes to string for Go
- **Page→SW messages** use `PostToSW` → sends raw string → SW's `GetMessageData` returns it directly
- **Bus messages** are always strings (JSON objects starting with `{`)

### 5. Message Format Convention

Two message shapes coexist. They are distinguished by the first character:

| First char | Shape | Used by | Example |
|-----------|-------|---------|---------|
| `[` | JSON array (as string) | App messages: page↔shell SW | `["EVENT","sub-1",{...}]` |
| `{` | JSON object (as string) | Bus envelopes: inter-SW | `{"from":"relay","to":"shell","msg":[...]}` |

- `OnSWMessage` in `dom.mjs` **skips** messages starting with `{` — those are bus traffic handled by `index.html`
- Bus messages use envelope format: `{"from":"<origin>","to":"<dest>","msg":<payload>}`
- The `msg` field contains the inner array message — it is a JSON value, not a string-escaped JSON
- SW `message` event handlers check `d[0] === '{'` to route bus vs app messages

### 6. Bus Protocol

Shell SW ↔ Satellite SWs communicate via bus. Shell uses BroadcastChannel (`$sw/$runtime/bc.mjs`). Relay uses MessagePort (`$sw-relay/$runtime/bc.mjs`) for production, despite the Go import being `common/jsbridge/bc`.

**Envelope format:**
```json
{"from":"shell","to":"relay","msg":["PROXY","sub-1",{"kinds":[1],"limit":10},["wss://relay.example.com"]]}
```

**Readiness protocol:**
1. Shell sends `PING` with version on bus connect
2. Satellite responds with `READY` + version
3. Shell compares versions — if mismatch, sends `FORCE_UPDATE_SW` to page
4. Shell flushes queued messages for that satellite
5. Messages sent before READY are queued (max 256, oldest dropped)

**FWD batching (relay→shell):**
- Relay accumulates `FWD`/`FWD_ALL` into buffer
- 50ms `SetTimeout` flushes as single `FWD_BATCH` message
- Reduces BroadcastChannel pressure during subscription floods

### 7. Crypto Proxy Chain

Key material NEVER exists in any SW. All crypto routes through:

```
Relay SW → bus → Shell SW → PostMessageJSON → Page → Signer Extension → result
                                                   ← CRYPTO_RESULT ←
```

- Each request gets an incrementing ID
- Callbacks stored in `cryptoCBs` map
- 15-second timeout with cleanup
- The signer extension intercepts `CRYPTO_REQ` via `window.nostr` NIP-07 interface

### 8. JSON Construction — String Concatenation, Not Marshaling

All JSON messages in SWs are built by **string concatenation**, not `json.Marshal` or equivalent. This is intentional — TinyGo's `encoding/json` pulls in reflection.

```go
// Correct: string concatenation
busSend("relay", "[\"PROXY\","+jstr(subID)+","+filterRaw+","+strsJSON(relays)+"]")

// Wrong: never use json.Marshal in SW code
```

- `jstr(s)` wraps a string in quotes with proper escaping (calls `helpers.JsonString`)
- `strsJSON(ss)` serializes `[]string` to `["a","b","c"]`
- `raw` values (from `mw.raw()`) are already valid JSON — embed directly, don't re-quote
- The `mw` (message walker) type parses JSON arrays without deserialization — use `str()`, `num()`, `raw()`, `strs()`

### 9. $runtime Files Are Hand-Written

The `$runtime/` directories contain hand-written JavaScript that implements the Go bridge contracts. Everything else in `app/smesh3/` (and subdirs) with a `.mjs` extension is **tinyjs compiler output**.

- **Never edit** files like `app/smesh3/smesh3.mjs` or `app/smesh3/common_helpers.mjs` — they are regenerated on every compile
- **Always edit** `$runtime/*.mjs` files when adding/changing bridge functions
- When the tinyjs compiler has a bug, fix the compiler (`next/tinygo/`), don't hand-patch the output
- Other hand-written JS: `app/smesh3/mls-bridge.mjs`, `app/smesh3/index.html` inline scripts

### 10. Page-as-Router for Satellite SWs

In production (HTTPS), satellite SWs run on subdomains (`relay.smesh.lol`, `marmot.smesh.lol`). The page on the main origin acts as message router:

1. Page opens hidden iframe to subdomain
2. Iframe registers satellite SW
3. Iframe establishes MessagePort to satellite SW
4. Page bridges: shell SW ↔ page ↔ iframe ↔ satellite SW

In dev (127.0.0.x), same pattern but loopback addresses instead of subdomains.

The **page MUST relay bus messages** between shell and satellite SWs. SWs on different origins cannot communicate directly. The bus relay code lives in `index.html` inline script, NOT in the compiled WASM app, because it must be active before WASM loads.

### 11. Marmot Subscription Convention

MLS DM subscriptions use a prefix convention to route relay events back through the MLS pipeline:

- Shell creates subscription with ID `"marmot-sub-<n>"` where `<n>` is a bare number
- Relay SW treats it as a normal PROXY subscription
- When events arrive, shell checks for `["EVENT","marmot-sub-` prefix
- Matching events get converted to `["MLS_PROXY","deliverEvent",<n>,<eventJSON>]`
- `<n>` is a bare number (not quoted) — marmot WASM expects numeric type
- `<eventJSON>` is a JSON-escaped string — marmot WASM calls `args[1].String()`

### 12. Error Handling — Silent, Never Throwing

Errors do not propagate across the bridge. If a JS function fails, it returns a zero/empty value. If a Go callback receives unexpected input, it logs and returns.

- JS runtime functions catch errors silently — `if (!el) return;`, `if (!cache) { if (done) done(); return; }`
- Go bridge stubs cannot throw — they are replaced at compile time
- SW crash handlers (`self.addEventListener('error', ...)`) log via bus, never halt execution
- The `mw` walker returns empty strings for missing/malformed fields, never panics

### 13. No encoding/json, No reflect, No fmt

TinyGo SW binaries must be small and fast. These Go packages are **forbidden** in SW code:

- `encoding/json` — pulls in reflect, doubles binary size
- `reflect` — not fully supported in TinyGo
- `fmt` — pulls in reflect via `%v` formatting

Use instead:
- `helpers.JsonString(s)` for JSON string escaping
- `helpers.Itoa(n)` for int-to-string
- `mw` walker for JSON parsing
- String concatenation for JSON construction
- `sw.Log()` for debug output (not `fmt.Println`)

## What NOT To Do

- Never suggest sleep, rest, or taking a break
- Never add dependencies without explicit approval
- Never refactor working code that wasn't asked about
- Never introduce abstractions "for future flexibility"
- Never use frameworks when stdlib suffices
- Never explain code you're about to write — just write it
- Never summarize changes after making them unless asked
- Never create test files unless explicitly asked
- Never modify go.mod without approval
