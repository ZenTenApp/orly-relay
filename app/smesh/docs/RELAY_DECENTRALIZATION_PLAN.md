# Relay Decentralization Plan

## Problem Statement

The codebase uses `BIG_RELAY_URLS` (~80 occurrences) as a lazy fallback for relay selection. This is an anti-pattern because:

1. **Privacy violation**: User data leaks to relays they didn't explicitly choose
2. **Centralization pressure**: Creates dependency on a handful of "big" relays
3. **Ignores user preferences**: Bypasses NIP-65 relay lists that users configured
4. **Poor delivery**: Events may not reach intended recipients who use different relays

## Design Principles

1. **User sovereignty**: Only publish to relays the user explicitly chose
2. **Recipient-aware publishing**: When interacting with others, use THEIR relay preferences
3. **Cached relay lists**: Store other users' relay lists for quick access
4. **Graceful degradation**: Handle missing relay lists without falling back to hardcoded relays
5. **Bootstrap via hints**: Use relay hints from nprofile/nevent URIs, not hardcoded lists

## Architecture

### 1. Relay List Cache Service

Create `src/services/relay-list-cache.service.ts`:

```typescript
interface CachedRelayList {
  pubkey: string
  read: string[]
  write: string[]
  fetchedAt: number
  event?: NostrEvent // Original kind 10002 event
}

class RelayListCacheService {
  // In-memory LRU cache
  private cache: LRUCache<string, CachedRelayList>

  // Persist to IndexedDB
  async getRelayList(pubkey: string): Promise<CachedRelayList | null>
  async setRelayList(pubkey: string, relayList: CachedRelayList): Promise<void>

  // Fetch from network if not cached or stale
  async fetchRelayList(pubkey: string, hints?: string[]): Promise<CachedRelayList | null>

  // Batch fetch for multiple pubkeys (e.g., thread participants)
  async fetchRelayLists(pubkeys: string[], hints?: string[]): Promise<Map<string, CachedRelayList>>

  // Get combined write relays for multiple recipients
  async getWriteRelaysForRecipients(pubkeys: string[]): Promise<string[]>
}
```

### 2. Relay Selection Strategy

Create `src/lib/relay-selection.ts`:

```typescript
interface RelaySelectionContext {
  // Current user's relay list
  userRelayList: RelayList
  // Cached relay lists for other users
  relayListCache: RelayListCacheService
}

// For publishing events
async function selectPublishRelays(
  ctx: RelaySelectionContext,
  event: NostrEvent,
  options?: { includeRecipients?: boolean }
): Promise<string[]> {
  const relays = new Set<string>()

  // Always include user's write relays
  ctx.userRelayList.write.forEach(r => relays.add(r))

  // If event has p-tags (mentions/replies), include their write relays
  if (options?.includeRecipients) {
    const pTags = event.tags.filter(t => t[0] === 'p')
    for (const [, pubkey, hint] of pTags) {
      const recipientRelays = await ctx.relayListCache.getRelayList(pubkey)
      if (recipientRelays) {
        recipientRelays.write.forEach(r => relays.add(r))
      } else if (hint) {
        // Use hint as fallback
        relays.add(hint)
      }
    }
  }

  return Array.from(relays)
}

// For fetching events by author
async function selectReadRelays(
  ctx: RelaySelectionContext,
  authorPubkey: string,
  hints?: string[]
): Promise<string[]> {
  // Try cached relay list first
  const authorRelays = await ctx.relayListCache.getRelayList(authorPubkey)
  if (authorRelays && authorRelays.read.length > 0) {
    return authorRelays.read
  }

  // Use hints if provided (from nprofile, nevent, etc.)
  if (hints && hints.length > 0) {
    return hints
  }

  // Last resort: user's own relays (they might have the event)
  return ctx.userRelayList.read
}

// For fetching events by ID with hints
async function selectRelaysForEvent(
  ctx: RelaySelectionContext,
  eventId: string,
  hints?: string[],
  authorPubkey?: string
): Promise<string[]> {
  const relays = new Set<string>()

  // Use hints first
  hints?.forEach(r => relays.add(r))

  // Add author's relays if known
  if (authorPubkey) {
    const authorRelays = await ctx.relayListCache.getRelayList(authorPubkey)
    authorRelays?.read.forEach(r => relays.add(r))
  }

  // Add user's relays
  ctx.userRelayList.read.forEach(r => relays.add(r))

  return Array.from(relays)
}
```

### 3. Cache Propagation (Republish to User's Relays)

When fetching profiles (kind 0) and relay lists (kind 10002) from other users, republish them to the current user's write relays. This provides:

1. **Faster future fetches**: Data is already on your preferred relays
2. **Network propagation**: Helps spread data across the decentralized network
3. **Offline resilience**: Your relays become a personal cache of relevant data
4. **Reduced latency**: No need to query distant relays for frequently-accessed profiles

**Implementation: Service Worker with Queue**

The propagation runs in a service worker to avoid blocking the UI and to continue processing even when the app tab is closed.

```typescript
// src/service-worker/propagation-queue.ts

interface PropagationJob {
  id: string
  event: NostrEvent
  targetRelays: string[]
  addedAt: number
  attempts: number
}

// Stored in IndexedDB, processed by service worker
const PROPAGATION_QUEUE_STORE = 'propagation-queue'
const PROPAGATED_EVENTS_STORE = 'propagated-events' // Track what we've sent

// Main thread: Add to queue
async function queueForPropagation(event: NostrEvent, targetRelays: string[]): Promise<void> {
  // Skip if not a cacheable kind
  if (![0, 10002].includes(event.kind)) return

  // Skip own events
  if (event.pubkey === currentUserPubkey) return

  // Check if recently propagated (in IndexedDB)
  const recentlyPropagated = await db.get(PROPAGATED_EVENTS_STORE, event.id)
  if (recentlyPropagated && Date.now() - recentlyPropagated.timestamp < 24 * 60 * 60 * 1000) {
    return
  }

  // Add to queue in IndexedDB
  const job: PropagationJob = {
    id: event.id,
    event,
    targetRelays,
    addedAt: Date.now(),
    attempts: 0
  }
  await db.put(PROPAGATION_QUEUE_STORE, job)

  // Wake up service worker to process
  if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
    navigator.serviceWorker.controller.postMessage({ type: 'PROCESS_PROPAGATION_QUEUE' })
  }
}

// Service worker: Process queue
self.addEventListener('message', async (event) => {
  if (event.data.type === 'PROCESS_PROPAGATION_QUEUE') {
    await processPropagationQueue()
  }
})

// Also process on periodic sync (if supported)
self.addEventListener('periodicsync', async (event) => {
  if (event.tag === 'propagation-queue') {
    event.waitUntil(processPropagationQueue())
  }
})

async function processPropagationQueue(): Promise<void> {
  const db = await openDB()
  const jobs = await db.getAll(PROPAGATION_QUEUE_STORE)

  for (const job of jobs) {
    try {
      // Create WebSocket connections to target relays
      const results = await publishToRelays(job.targetRelays, job.event)

      // If at least one relay accepted, mark as propagated
      if (results.some(r => r.success)) {
        await db.put(PROPAGATED_EVENTS_STORE, {
          id: job.event.id,
          timestamp: Date.now()
        })
        await db.delete(PROPAGATION_QUEUE_STORE, job.id)
      } else {
        // Retry later (with backoff)
        job.attempts++
        if (job.attempts < 3) {
          await db.put(PROPAGATION_QUEUE_STORE, job)
        } else {
          // Give up after 3 attempts
          await db.delete(PROPAGATION_QUEUE_STORE, job.id)
        }
      }
    } catch (err) {
      console.warn('Propagation failed:', job.id, err)
      job.attempts++
      if (job.attempts < 3) {
        await db.put(PROPAGATION_QUEUE_STORE, job)
      }
    }
  }

  // Clean up old propagated entries (older than 7 days)
  const allPropagated = await db.getAll(PROPAGATED_EVENTS_STORE)
  const weekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000
  for (const entry of allPropagated) {
    if (entry.timestamp < weekAgo) {
      await db.delete(PROPAGATED_EVENTS_STORE, entry.id)
    }
  }
}
```

**Service Worker Benefits:**
- Runs in background, doesn't block UI interactions
- Continues processing even when app tab is closed
- Can use Periodic Background Sync for regular processing
- Survives page refreshes
- Centralized queue management across all tabs
```

**Important considerations:**

- Only propagate replaceable events (kinds 0, 10002, 10000, etc.) where newer replaces older
- Check timestamps to avoid overwriting newer data with older
- Debounce propagation to avoid spamming relays during bulk fetches
- Don't propagate your own events (they're already on your relays)
- Consider relay policies - some relays may reject events from non-authors
- This is opt-in behavior controlled by user settings

### 4. Bootstrap Strategy

For new users or first-time queries where we have NO relay information:

```typescript
// User-configurable bootstrap relays (stored in settings)
interface BootstrapConfig {
  // User can configure their preferred bootstrap relays
  relays: string[]
  // Or use relay hints from the link/URI that brought them here
  useHintsFromUri: boolean
}

// When user clicks an nprofile:// or nostr: link
function extractRelayHints(uri: string): string[] {
  const decoded = nip19.decode(uri)
  if (decoded.type === 'nprofile') return decoded.data.relays || []
  if (decoded.type === 'nevent') return decoded.data.relays || []
  if (decoded.type === 'naddr') return decoded.data.relays || []
  return []
}
```

### 4. Migration Path

Phase 1: Add infrastructure (non-breaking)
- Create RelayListCacheService
- Create relay selection utilities
- Add IndexedDB schema for relay list cache
- Fetch and cache relay lists opportunistically

Phase 2: Gradual replacement
- Replace BIG_RELAY_URLS usage one service at a time
- Start with least critical paths (profile fetching)
- Add metrics/logging to track relay selection

Phase 3: Remove fallbacks
- Remove BIG_RELAY_URLS constant
- Make relay selection explicit everywhere
- Add user-configurable bootstrap relays in settings

## Implementation Tasks

### Epic: Remove BIG_RELAY_URLS Centralization

#### Phase 1: Infrastructure

- [ ] Create RelayListCacheService with LRU + IndexedDB
- [ ] Add relay list fetching with batch support
- [ ] Create relay-selection.ts utilities
- [ ] Add IndexedDB schema migration for relay lists
- [ ] Opportunistically cache relay lists when fetching profiles

#### Phase 2: Publishing Path

- [ ] Update client.publishEvent to use relay selection
- [ ] Update reply/quote to include recipient relays
- [ ] Update DM service to use recipient inbox relays
- [ ] Update media-upload.service binding event publish
- [ ] Update settings sync to use user's relays only

#### Phase 3: Fetching Path

- [ ] Update profile fetching to use author's relays
- [ ] Update thread loading to use relay hints from tags
- [ ] Update notification fetching to use user's relays
- [ ] Update timeline fetching to use followed users' relays
- [ ] Update search to use user-configured search relays

#### Phase 4: Cleanup

- [ ] Remove BIG_RELAY_URLS constant
- [ ] Remove SEARCHABLE_RELAY_URLS (make user-configurable)
- [ ] Add bootstrap relay configuration in settings
- [ ] Update new user onboarding to set initial relays
- [ ] Add relay health monitoring (mark slow/dead relays)

## Files to Modify

### High Priority (Publishing)
- `src/services/client.service.ts` - Core publish/fetch logic
- `src/services/media-upload.service.ts` - Binding event publish
- `src/services/dm.service.ts` - DM relay selection
- `src/providers/NostrProvider/index.tsx` - Initial setup

### Medium Priority (Fetching)
- `src/services/thread.service.ts` - Thread loading
- `src/providers/NotificationProvider.tsx` - Notifications
- `src/providers/DMProvider.tsx` - DM fetching
- `src/components/Profile/ProfileFeed.tsx` - Profile content

### Lower Priority (UI/Settings)
- `src/constants.ts` - Remove BIG_RELAY_URLS
- `src/components/Settings/index.tsx` - Bootstrap relay config
- Various components using BIG_RELAY_URLS for queries

## Success Criteria

1. No hardcoded relay URLs in codebase (except user-configurable defaults)
2. Events are published to user's write relays + recipients' write relays
3. Fetching uses author's read relays with relay hints as backup
4. Relay lists are cached and updated periodically
5. New users can configure their bootstrap relays
6. Privacy: No data sent to relays user didn't choose

## Notes

- This is a breaking change in behavior but not API
- Users with no relay list configured will have degraded experience
- Consider showing warning if user has no relay list
- May need to handle edge cases where no relays are available
