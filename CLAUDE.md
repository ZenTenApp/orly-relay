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

## Deployment

VPS at 69.164.249.71 (Ubuntu), accessed via `ssh orly` (WireGuard tunnel to 10.0.0.1).

**Stack**: Caddy → orly binary (relay on :3334, smesh2 on :8089, sm3sh on :8090)
**Service**: systemd `orly.service`, binary at `/home/mleku/.local/bin/orly`
**sm3sh assets**: `/home/mleku/sm3sh/` — served in disk mode, fsnotify watches for changes, SSE pushes version to service worker which reloads all clients.

No Go on VPS. Cross-compile + rsync:
```sh
CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc GOOS=linux GOARCH=amd64 go build -o /tmp/orly-deploy ./cmd/orly
scp /tmp/orly-deploy orly:/home/mleku/.local/bin/orly.new
rsync -avz --exclude='*.go' app/smesh3/ orly:/home/mleku/sm3sh/
ssh orly "mv /home/mleku/.local/bin/orly{,.bak} && mv /home/mleku/.local/bin/orly{.new,} && chmod +x /home/mleku/.local/bin/orly && systemctl restart orly"
```

Asset-only deploy (no binary change): just rsync, fsnotify triggers reload automatically.

## tinyjs Build

Source: `next/sm3sh/` (app) + `next/common/` (shared libs)
Compile: `cd next/sm3sh && tinyjs -o ../../app/smesh3 .`
Output: `app/smesh3/*.mjs` — module name maps to filename (`common/helpers` → `common_helpers.mjs`)
The .mjs files are gitignored — use `git add -f` when committing.
sw.js `APP_FILES` list must match actual output filenames after any module rename.

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
