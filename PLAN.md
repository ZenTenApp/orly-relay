# ORLY Strategic Plan

Private planning document. Not for public distribution until the mandate is clear.

## Premise

Nostr's adoption stalls because defection rate tracks with adoption. People arrive
from hostile platforms, encounter slop and trolling, and leave. The relay ecosystem
is fragmented and nobody is doing content quality at the infrastructure level.

ORLY becomes the relay that solves this — not through gatekeeping, but through
automated classification that makes the network cleaner for everyone who connects
to it. The strategy is stealth-first: demonstrate value through action, let adoption
create demand, then open-source when the mandate is undeniable.

## Architecture Overview

ORLY becomes an all-in-one deployment:
- Nostr relay (existing)
- Email bridge / Marmot (existing)
- Smesh web client (to be integrated)
- Git hosting (to be built)
- Corpus crawler (partially built — DirectorySpider + negentropy exist)
- AI content classifier (to be built)
- Reputation ACL driver (to be built)
- Flag note publisher (to be built)

Anyone who runs `orly` gets all of this. One binary. One deployment step.


## Phase 0: Housekeeping

### 0.1 — VPS Disk Recovery
The VPS is at 100% (199G/199G). 180GB is consumed by /var/log.
- Truncate or rotate /var/log (179GB syslog alone)
- Configure logrotate with aggressive limits to prevent recurrence
- Configure journald MaxRetentionSec and SystemMaxUse
- Target: recover ~170GB for corpus storage

### 0.2 — Full Logging Audit
**Root cause of 0.1**: The relay logs at info level things that should be trace/debug.
The worst offender is `app/publisher.go:180` which logs "ephemeral event kind X
did NOT match subscription Y" at `log.I` for EVERY subscription on EVERY ephemeral
event. With 40+ active subscriptions, one ephemeral event produces 40+ log lines.
This alone generated 179GB of syslog.

Audit rules:
- **Info level**: Significant state changes only. Server start/stop, config loaded,
  client connect/disconnect, sync completed, errors that affect service. Target:
  a few lines per minute under normal operation.
- **Debug level**: Per-request information useful for troubleshooting. Individual
  event saves, subscription matches, filter evaluations, auth checks.
- **Trace level**: Hot-path internals. Subscription non-matches, ephemeral event
  routing details, serialization paths, cache hits/misses.

Audit scope (every file with log.I, log.D, log.T calls):
- `app/` — all handlers, publisher, server
- `pkg/protocol/` — websocket, auth, publish
- `pkg/database/` — query execution, saves, deletes
- `pkg/sync/` — sync operations
- `pkg/acl/` — access checks
- `pkg/bridge/` — email bridge operations
- `pkg/spider/` — crawler operations

Specific known issues:
- `app/publisher.go:180`: ephemeral non-match at I → move to T or delete entirely
- Full codebase grep for `log.I.F` in hot paths needed

### 0.3 — Repository Setup
- Create private working repo at /home/mleku/orly.dev (this directory)
- Source continues to live at /home/mleku/src/next.orly.dev
- This directory holds planning documents and operational artifacts
- The go module path remains next.orly.dev until the public pivot

### 0.4 — Symlink Update
- Update /m/orly symlink if needed
- Ensure build scripts work from either path


## Phase 1: Integrate Smesh Into ORLY

### 1.1 — Embed Smesh as a Subcommand or Service
Smesh is a React/Vite SPA at /home/mleku/src/git.mleku.dev/mleku/smesh.
Build output is ~5MB static files.

Approach:
- Copy smesh source into the ORLY monorepo (app/smesh/ or similar)
- Add a build step that produces dist/ from the smesh source
- Embed dist/ via go:embed (same pattern as existing app/web/)
- Serve on a configurable port or path
- Add ORLY_SMESH_ENABLED and ORLY_SMESH_PORT env vars
- Launcher spawns smesh HTTP server as another subprocess (or serve inline)

The existing app/web/ (Svelte relay dashboard) remains separate.
Smesh is the full client. Two different UIs, two different purposes.

### 1.2 — Relay Defaults in Smesh
- Hardcode relay.orly.dev as the primary bootstrap relay
- Add branding configuration so each ORLY instance can customize
- Consider: smesh reads relay NIP-11 info for branding


## Phase 2: Corpus Crawler

### 2.1 — Activate and Extend DirectorySpider
The spider already exists at pkg/spider/directory.go. It:
- Crawls kind 10002 events from seed relays
- Does multi-hop relay discovery
- Fetches metadata (kinds 0, 3, 10000, 10002)

Extend it to:
- Rank discovered relays by event count / mention frequency
- Track relay health (uptime, latency, event density)
- Persist relay rankings in Badger via markers

### 2.2 — Negentropy Bulk Sync
The negentropy implementation exists at pkg/sync/negentropy/.
Use it to:
- Sync from top-ranked relays discovered in 2.1
- Start with broad filters (all kinds), then narrow based on storage budget
- Implement storage budget manager:
  - Monitor Badger disk usage
  - Stop syncing when approaching 50% of available disk
  - Prioritize newer content over older
  - Prioritize content from pubkeys with higher relay presence

### 2.3 — Crawler Orchestration
- DirectorySpider discovers relays and ranks them
- Negentropy manager maintains sync sessions with top relays
- Storage budget manager gates new sync sessions
- All configurable via ORLY_CRAWLER_* env vars:
  - ORLY_CRAWLER_ENABLED (default: false)
  - ORLY_CRAWLER_MAX_STORAGE_PERCENT (default: 50)
  - ORLY_CRAWLER_SEED_RELAYS (comma-separated bootstrap list)
  - ORLY_CRAWLER_SYNC_INTERVAL (default: 1h)
  - ORLY_CRAWLER_MAX_RELAYS (default: 100)

### 2.4 — Outgrow Primal/Damus
Target: accumulate more events than any single relay in the network.
The key insight is that most relays only store events from their own users.
By aggregating from all of them, ORLY becomes the most complete archive.
100GB of Badger-compressed events is a LOT of nostr content.


## Phase 3: AI Content Classifier

### 3.1 — Classifier Architecture
- Runs as a pipeline stage on incoming events (and batch on corpus)
- Initial version: statistical / heuristic (no GPU dependency)
  - Token distribution analysis
  - Repetition patterns
  - Stylistic uniformity scores
  - Context-awareness markers
- Later version: small language model fine-tuned on labeled corpus
- Output: confidence score 0.0-1.0 for "AI-generated"

### 3.2 — Training Corpus
- Use the synced corpus from Phase 2
- Manual labeling of known AI accounts (there are obvious ones)
- Bootstrap with known slop patterns
- Progressive refinement: high-confidence flags get verified, expand training set

### 3.3 — Integration Points
- Batch classifier: runs over existing corpus, tags events in DB
- Inline classifier: runs on new events during SaveEvent
- Expose scores via custom tag or internal metadata
- Never delete — only flag. Deletion decisions are for the ACL layer.


## Phase 4: Reputation ACL Driver

### 4.1 — New ACL Driver: "reputation"
Register via existing RegisterDriver("reputation", ...) pattern.

Per-pubkey state:
- trust_score: float64 (0.0 = untrusted, 1.0 = fully trusted)
- check_interval: duration (how often to re-classify this pubkey's content)
- last_check: timestamp
- flag_count: int (number of AI-flagged events)
- clean_count: int (number of clean events)
- status: enum (normal, probation, blacklisted)

### 4.2 — Progressive Backoff
- New pubkeys: every event checked
- After N clean events: check every 10th event
- After M clean events: check every 100th event
- Any flag resets to higher checking frequency
- Persistent offenders (>X% flagged over Y events): automatic blacklist
- Blacklist is soft: events still stored but not served to queries by default

### 4.3 — Troll Detection (Phase 2 of classifier)
- Harder problem, built on foundation of AI detection
- Behavioral patterns: ratio of replies to original content, inflammatory language
- Graph patterns: who a pubkey interacts with, response patterns
- Community feedback: allow trusted pubkeys to flag content
- Same progressive backoff applies


## Phase 5: Flag Note Publisher

### 5.1 — Automated Publishing
- Relay has its own npub (already exists: relay identity key)
- Publishes kind 1 notes flagging serial offenders
- Content format: clear, factual, verifiable
  - "This pubkey [npub...] has been flagged as an automated content generator.
     X of Y recent events scored above 0.9 on AI content detection.
     Add wss://relay.orly.dev to your relay list to see everything, minus slop."
- Include NIP-19 nevent/nprofile with relay hint pointing to relay.orly.dev

### 5.2 — Distribution Strategy
- Publish to all relays mentioned in the flagged pubkey's kind 10002
- Publish to relays of users who interacted with the flagged content
- This embeds relay.orly.dev into the relay hint graph across the network
- Users seeing these notes and following up propagate the relay reference further

### 5.3 — Reputation of Flag Notes
- If the classifier is accurate, flag notes build credibility
- Users who find them useful add relay.orly.dev to their relay list
- More users → more events synced → better corpus → better classifier


## Phase 6: Git Hosting

### 6.1 — Minimal Git HTTP Service
Purpose: host the ORLY source code at git.orly.dev when ready to go public.

Architecture:
- Go HTTP server wrapping /usr/lib/git-core/git-http-backend as CGI
- git-http-backend handles smart HTTP protocol (clone/fetch/push)
- Thin HTML layer for browsing: repository list, commit log, file viewer, diff viewer
- No JavaScript required — server-rendered HTML
- Auth: read is public, push requires NIP-98 HTTP auth (nostr-native)

### 6.2 — Integration Into ORLY Binary
- Add as subcommand: `orly git-server`
- Or as integrated service via launcher
- Env vars: ORLY_GIT_ENABLED, ORLY_GIT_PORT, ORLY_GIT_ROOT
- Serves bare git repos from ORLY_GIT_ROOT directory

### 6.3 — Timeline
This is the last piece. It activates only when:
- The classifier is proven accurate
- Users are organically discovering relay.orly.dev
- Demand for the software is visible
- The mandate from the community is clear


## Phase 7: Smesh Deep Integration

### 7.1 — Classifier Visibility
- Smesh displays AI confidence scores on posts (optional, per-user setting)
- Visual indicator: subtle marker on flagged content
- User can choose to hide flagged content or show with warning

### 7.2 — Relay Promotion
- Smesh instances served by ORLY relays show relay branding
- "Powered by ORLY — clean nostr" footer or similar
- One-click "add this relay" button

### 7.3 — Smesh as Default Client
- Every ORLY relay serves smesh at its root URL
- Users who visit the relay URL in a browser get a full nostr client
- No separate deployment needed
- Smesh reads NIP-11 relay info for configuration


## Execution Order

```
Phase 0 → VPS housekeeping, repo setup
    ↓
Phase 1 → Smesh integration (one binary, one deploy)
    ↓
Phase 2 → Corpus crawler (bulk sync, outgrow incumbents)
    ↓
Phase 3 → AI classifier (train on corpus, flag AI slop)
    ↓
Phase 4 → Reputation ACL (progressive trust, auto-moderation)
    ↓
Phase 5 → Flag publisher (organic growth via relay hints)
    ↓
Phase 6 → Git hosting (open source when mandate is clear)
    ↓
Phase 7 → Deep integration (smesh + classifier UI)
```

Each phase is independently valuable. Phase 2 alone makes relay.orly.dev
the most complete nostr archive. Phase 3 alone makes it the cleanest.
Phase 5 alone drives adoption. They compound.


## Infrastructure Notes

### VPS: relay.orly.dev
- AMD64, Ubuntu 24.04
- 200GB disk (currently full — needs /var/log cleanup)
- Target: 100GB for corpus, 50GB headroom, 50GB for growth
- Caddy reverse proxy handles TLS

### Domain: orly.dev
- relay.orly.dev — nostr relay (existing)
- smesh.mleku.dev → becomes smesh.orly.dev or just orly.dev root
- git.orly.dev — git hosting (Phase 6)

### Source Code
- Lives at /home/mleku/src/next.orly.dev (private)
- Module path: next.orly.dev (unchanged until public pivot)
- Planning docs: /home/mleku/orly.dev/


## Risk Factors

1. **Classifier accuracy**: False positives on humans quoting AI output.
   Mitigation: high threshold, progressive trust, never hard-delete.

2. **Storage limits**: 100GB may not be enough for "all of nostr".
   Mitigation: prioritize by recency and relay rank, evict old low-value content.

3. **Relay blocking**: Large relays might rate-limit or block aggressive sync.
   Mitigation: respectful sync intervals, negentropy is bandwidth-efficient.

4. **Community reception**: Flag notes could be seen as spam themselves.
   Mitigation: accuracy is everything. Only flag high-confidence cases.
   Transparent methodology. Verifiable claims.

5. **Hostile response**: Incumbents might actively counter.
   Mitigation: stealth-first. By the time it's visible, value is demonstrated.
