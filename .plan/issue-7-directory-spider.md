# Implementation Plan: Directory Spider (Issue #7)

## Overview

Add a new "directory spider" that discovers relays by crawling kind 10002 (relay list) events, expanding outward in hops from whitelisted users, and then fetches essential metadata events (kinds 0, 3, 10000, 10002) from the discovered network.

**Key Characteristics:**
- Runs once per day (configurable)
- Single-threaded, serial operations to minimize load
- 3-hop relay discovery from whitelisted users
- Fetches: kind 0 (profile), 3 (follow list), 10000 (mute list), 10002 (relay list)

---

## Architecture

### New Package Structure

```
pkg/spider/
├── spider.go           # Existing follows spider
├── directory.go        # NEW: Directory spider implementation
├── directory_test.go   # NEW: Tests
└── common.go           # NEW: Shared utilities (extract from spider.go)
```

### Core Components

```go
// DirectorySpider manages the daily relay discovery and metadata sync
type DirectorySpider struct {
    ctx        context.Context
    cancel     context.CancelFunc
    db         *database.D
    pub        publisher.I

    // Configuration
    interval   time.Duration  // Default: 24h
    maxHops    int            // Default: 3

    // State
    running    atomic.Bool
    lastRun    time.Time

    // Relay discovery
    discoveredRelays map[string]int  // URL -> hop distance
    processedRelays  map[string]bool // Already fetched from

    // Callbacks for integration
    getSeedPubkeys func() [][]byte  // Whitelisted users (from ACL)
}
```

---

## Implementation Phases

### Phase 1: Core Directory Spider Structure

**File:** `pkg/spider/directory.go`

1. **Create DirectorySpider struct** with:
   - Context management for cancellation
   - Database and publisher references
   - Configuration (interval, max hops)
   - State tracking (discovered relays, processed relays)

2. **Constructor:** `NewDirectorySpider(ctx, db, pub, interval, maxHops)`
   - Initialize maps and state
   - Set defaults (24h interval, 3 hops)

3. **Lifecycle methods:**
   - `Start()` - Launch main goroutine
   - `Stop()` - Cancel context and wait for shutdown
   - `TriggerNow()` - Force immediate run (for testing/admin)

### Phase 2: Relay Discovery (3-Hop Expansion)

**Algorithm:**

```
Round 1: Get relay lists from whitelisted users
  - Query local DB for kind 10002 events from seed pubkeys
  - Extract relay URLs from "r" tags
  - Mark as hop 0 relays

Round 2-4 (3 iterations):
  - For each relay at current hop level (in serial):
    1. Connect to relay
    2. Query for ALL kind 10002 events (limit: 5000)
    3. Extract new relay URLs
    4. Mark as hop N+1 relays
    5. Close connection
    6. Sleep briefly between relays (rate limiting)
```

**Key Methods:**

```go
// discoverRelays performs the 3-hop relay expansion
func (ds *DirectorySpider) discoverRelays(ctx context.Context) error

// fetchRelayListsFromRelay connects to a relay and fetches kind 10002 events
func (ds *DirectorySpider) fetchRelayListsFromRelay(ctx context.Context, relayURL string) ([]*event.T, error)

// extractRelaysFromEvents parses kind 10002 events and extracts relay URLs
func (ds *DirectorySpider) extractRelaysFromEvents(events []*event.T) []string
```

### Phase 3: Metadata Fetching

After relay discovery, fetch essential metadata from all discovered relays:

**Kinds to fetch:**
- Kind 0: Profile metadata (replaceable)
- Kind 3: Follow lists (replaceable)
- Kind 10000: Mute lists (replaceable)
- Kind 10002: Relay lists (already have many, but get latest)

**Fetch Strategy:**

```go
// fetchMetadataFromRelays iterates through discovered relays serially
func (ds *DirectorySpider) fetchMetadataFromRelays(ctx context.Context) error {
    for relayURL := range ds.discoveredRelays {
        // Skip if already processed
        if ds.processedRelays[relayURL] {
            continue
        }

        // Fetch each kind type
        for _, k := range []int{0, 3, 10000, 10002} {
            events, err := ds.fetchKindFromRelay(ctx, relayURL, k)
            // Store events...
        }

        ds.processedRelays[relayURL] = true

        // Rate limiting sleep
        time.Sleep(500 * time.Millisecond)
    }
}
```

**Query Filters:**
- For replaceable events (0, 3, 10000, 10002): No time filter, let relay return latest
- Limit per query: 1000-5000 events
- Use pagination if relay supports it

### Phase 4: WebSocket Client for Fetching

**Reuse existing patterns from spider.go:**

```go
// fetchFromRelay handles connection, query, and cleanup
func (ds *DirectorySpider) fetchFromRelay(ctx context.Context, relayURL string, f *filter.F) ([]*event.T, error) {
    // Create timeout context (30 seconds per relay)
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Connect using ws.Client (from pkg/protocol/ws)
    client, err := ws.NewClient(ctx, relayURL)
    if err != nil {
        return nil, err
    }
    defer client.Close()

    // Subscribe with filter
    sub, err := client.Subscribe(ctx, f)
    if err != nil {
        return nil, err
    }

    // Collect events until EOSE or timeout
    var events []*event.T
    for ev := range sub.Events {
        events = append(events, ev)
    }

    return events, nil
}
```

### Phase 5: Event Storage

**Storage Strategy:**

```go
func (ds *DirectorySpider) storeEvents(ctx context.Context, events []*event.T) (saved, duplicates int) {
    for _, ev := range events {
        _, err := ds.db.SaveEvent(ctx, ev)
        if err != nil {
            if errors.Is(err, database.ErrDuplicate) {
                duplicates++
                continue
            }
            // Log other errors but continue
            log.W.F("failed to save event %s: %v", ev.ID.String(), err)
            continue
        }
        saved++

        // Publish to active subscribers
        ds.pub.Deliver(ev)
    }
    return
}
```

### Phase 6: Main Loop

```go
func (ds *DirectorySpider) mainLoop() {
    // Calculate time until next run
    ticker := time.NewTicker(ds.interval)
    defer ticker.Stop()

    // Run immediately on start
    ds.runOnce()

    for {
        select {
        case <-ds.ctx.Done():
            return
        case <-ticker.C:
            ds.runOnce()
        }
    }
}

func (ds *DirectorySpider) runOnce() {
    if !ds.running.CompareAndSwap(false, true) {
        log.I.F("directory spider already running, skipping")
        return
    }
    defer ds.running.Store(false)

    log.I.F("starting directory spider run")
    start := time.Now()

    // Reset state
    ds.discoveredRelays = make(map[string]int)
    ds.processedRelays = make(map[string]bool)

    // Phase 1: Discover relays via 3-hop expansion
    if err := ds.discoverRelays(ds.ctx); err != nil {
        log.E.F("relay discovery failed: %v", err)
        return
    }
    log.I.F("discovered %d relays", len(ds.discoveredRelays))

    // Phase 2: Fetch metadata from all relays
    if err := ds.fetchMetadataFromRelays(ds.ctx); err != nil {
        log.E.F("metadata fetch failed: %v", err)
        return
    }

    ds.lastRun = time.Now()
    log.I.F("directory spider completed in %v", time.Since(start))
}
```

### Phase 7: Configuration

**New environment variables:**

```go
// In app/config/config.go
DirectorySpiderEnabled  bool          `env:"ORLY_DIRECTORY_SPIDER" default:"false" usage:"enable directory spider for metadata sync"`
DirectorySpiderInterval time.Duration `env:"ORLY_DIRECTORY_SPIDER_INTERVAL" default:"24h" usage:"how often to run directory spider"`
DirectorySpiderMaxHops  int           `env:"ORLY_DIRECTORY_SPIDER_HOPS" default:"3" usage:"maximum hops for relay discovery"`
```

### Phase 8: Integration with app/main.go

```go
// After existing spider initialization
if badgerDB, ok := db.(*database.D); ok && cfg.DirectorySpiderEnabled {
    l.directorySpider, err = spider.NewDirectorySpider(
        ctx,
        badgerDB,
        l.publishers,
        cfg.DirectorySpiderInterval,
        cfg.DirectorySpiderMaxHops,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create directory spider: %w", err)
    }

    // Set callback to get seed pubkeys from ACL
    l.directorySpider.SetSeedCallback(func() [][]byte {
        // Get whitelisted users from all ACLs
        var pubkeys [][]byte
        for _, aclInstance := range acl.Registry.ACL {
            if follows, ok := aclInstance.(*acl.Follows); ok {
                pubkeys = append(pubkeys, follows.GetFollowedPubkeys()...)
            }
        }
        return pubkeys
    })

    l.directorySpider.Start()
}
```

---

## Self-Relay Detection

Reuse the existing `isSelfRelay()` pattern from spider.go:

```go
func (ds *DirectorySpider) isSelfRelay(relayURL string) bool {
    // Use NIP-11 to get relay pubkey
    // Compare against our relay identity pubkey
    // Cache results to avoid repeated requests
}
```

---

## Error Handling & Resilience

1. **Connection Timeouts:** 30 seconds per relay
2. **Query Timeouts:** 60 seconds per query
3. **Graceful Degradation:** Continue to next relay on failure
4. **Rate Limiting:** 500ms sleep between relays
5. **Memory Limits:** Process events in batches of 1000
6. **Context Cancellation:** Check at each step for shutdown

---

## Testing Strategy

### Unit Tests

```go
// pkg/spider/directory_test.go

func TestExtractRelaysFromEvents(t *testing.T)
func TestDiscoveryHopTracking(t *testing.T)
func TestSelfRelayFiltering(t *testing.T)
```

### Integration Tests

```go
func TestDirectorySpiderE2E(t *testing.T) {
    // Start test relay
    // Populate with kind 10002 events
    // Run directory spider
    // Verify events fetched and stored
}
```

---

## Logging

Use existing `lol.mleku.dev` logging patterns:

```go
log.I.F("directory spider: starting relay discovery")
log.D.F("directory spider: hop %d, discovered %d new relays", hop, count)
log.W.F("directory spider: failed to connect to %s: %v", url, err)
log.E.F("directory spider: critical error: %v", err)
```

---

## Implementation Order

1. **Phase 1:** Core struct and lifecycle (1-2 hours)
2. **Phase 2:** Relay discovery with hop expansion (2-3 hours)
3. **Phase 3:** Metadata fetching (1-2 hours)
4. **Phase 4:** WebSocket client integration (1 hour)
5. **Phase 5:** Event storage (30 min)
6. **Phase 6:** Main loop and scheduling (1 hour)
7. **Phase 7:** Configuration (30 min)
8. **Phase 8:** Integration with main.go (30 min)
9. **Testing:** Unit and integration tests (2-3 hours)

**Total Estimate:** 10-14 hours

---

## Future Enhancements (Out of Scope)

- Web UI status page for directory spider
- Metrics/stats collection (relays discovered, events fetched)
- Configurable kind list to fetch
- Priority ordering of relays (closer hops first)
- Persistent relay discovery cache between runs

---

## Dependencies

**Existing packages to use:**
- `pkg/protocol/ws` - WebSocket client
- `pkg/database` - Event storage
- `pkg/encoders/filter` - Query filter construction
- `pkg/acl` - Get whitelisted users
- `pkg/sync` - NIP-11 cache for self-detection (if needed)

**No new external dependencies required.**

---

## Follow-up Items (Post-Implementation)

### TODO: Verify Connection Behavior is Not Overly Aggressive

**Issue:** The current implementation creates a **new WebSocket connection for each kind query** when fetching metadata. For each relay, this means:
1. Connect → fetch kind 0 → disconnect
2. Connect → fetch kind 3 → disconnect
3. Connect → fetch kind 10000 → disconnect
4. Connect → fetch kind 10002 → disconnect

This could be seen as aggressive by remote relays and may trigger rate limiting or IP bans.

**Verification needed:**
- [ ] Monitor logs with `ORLY_LOG_LEVEL=debug` to see per-kind fetch results
- [ ] Check if relays are returning events for all 4 kinds or just kind 0
- [ ] Look for WARNING logs about connection failures or rate limiting
- [ ] Verify the 500ms delay between relays is sufficient

**Potential optimization (if needed):**
- Refactor `fetchMetadataFromRelays()` to use a single connection per relay
- Fetch all 4 kinds using multiple subscriptions on one connection
- Example pattern:
  ```go
  client, err := ws.RelayConnect(ctx, relayURL)
  defer client.Close()

  for _, k := range kindsToFetch {
      events, _ := fetchKindOnConnection(client, k)
      // ...
  }
  ```

**Priority:** Medium - only optimize if monitoring shows issues with the current approach
