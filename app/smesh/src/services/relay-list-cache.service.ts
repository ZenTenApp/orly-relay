import { RelayList } from '@/domain/relay/RelayList'
import { Event, kinds } from 'nostr-tools'
import client from './client.service'
import indexedDb from './indexed-db.service'

/**
 * Cache entry for a user's relay list
 */
export interface CachedRelayList {
  pubkey: string
  read: string[]
  write: string[]
  fetchedAt: number
  event?: Event
}

/**
 * LRU Cache implementation for in-memory caching
 */
class LRUCache<K, V> {
  private cache = new Map<K, V>()

  constructor(private maxSize: number) {}

  get(key: K): V | undefined {
    const value = this.cache.get(key)
    if (value !== undefined) {
      // Move to end (most recently used)
      this.cache.delete(key)
      this.cache.set(key, value)
    }
    return value
  }

  set(key: K, value: V): void {
    if (this.cache.has(key)) {
      this.cache.delete(key)
    } else if (this.cache.size >= this.maxSize) {
      // Remove oldest (first) entry
      const firstKey = this.cache.keys().next().value
      if (firstKey !== undefined) {
        this.cache.delete(firstKey)
      }
    }
    this.cache.set(key, value)
  }

  has(key: K): boolean {
    return this.cache.has(key)
  }

  delete(key: K): boolean {
    return this.cache.delete(key)
  }

  clear(): void {
    this.cache.clear()
  }
}

/**
 * RelayListCacheService
 *
 * Caches NIP-65 relay lists for other users to enable proper relay selection
 * without falling back to hardcoded relay lists.
 *
 * Features:
 * - In-memory LRU cache for fast access
 * - IndexedDB persistence for cache survival across sessions
 * - Batch fetching for efficiency
 * - Stale-while-revalidate pattern
 */
class RelayListCacheService {
  private memoryCache: LRUCache<string, CachedRelayList>
  private pendingFetches = new Map<string, Promise<CachedRelayList | null>>()

  // Cache entries older than this are considered stale
  private staleAfterMs = 24 * 60 * 60 * 1000 // 24 hours

  constructor() {
    this.memoryCache = new LRUCache(500) // Keep up to 500 relay lists in memory
  }

  /**
   * Get a cached relay list for a pubkey
   * Returns null if not in cache
   */
  async getRelayList(pubkey: string): Promise<CachedRelayList | null> {
    // Check memory cache first
    const memCached = this.memoryCache.get(pubkey)
    if (memCached) {
      return memCached
    }

    // Check IndexedDB
    const event = await indexedDb.getReplaceableEvent(pubkey, kinds.RelayList)
    if (event) {
      const cached = this.eventToCachedRelayList(event)
      this.memoryCache.set(pubkey, cached)
      return cached
    }

    return null
  }

  /**
   * Check if a cached relay list is stale
   */
  isStale(cached: CachedRelayList): boolean {
    return Date.now() - cached.fetchedAt > this.staleAfterMs
  }

  /**
   * Fetch a relay list from the network
   * Uses relay hints if provided
   */
  async fetchRelayList(
    pubkey: string,
    hints?: string[]
  ): Promise<CachedRelayList | null> {
    // Dedupe concurrent fetches for the same pubkey
    const pending = this.pendingFetches.get(pubkey)
    if (pending) {
      return pending
    }

    const fetchPromise = this.doFetchRelayList(pubkey, hints)
    this.pendingFetches.set(pubkey, fetchPromise)

    try {
      return await fetchPromise
    } finally {
      this.pendingFetches.delete(pubkey)
    }
  }

  private async doFetchRelayList(
    pubkey: string,
    _hints?: string[]
  ): Promise<CachedRelayList | null> {
    try {
      // TODO: Use hints when fetching from network (requires adding hints support to fetchRelayListEvent)
      const event = await client.fetchRelayListEvent(pubkey)
      if (!event) {
        // Cache the miss to avoid repeated fetches
        await indexedDb.putNullReplaceableEvent(pubkey, kinds.RelayList)
        return null
      }

      // Store in both caches
      await indexedDb.putReplaceableEvent(event)
      const cached = this.eventToCachedRelayList(event)
      this.memoryCache.set(pubkey, cached)

      return cached
    } catch (error) {
      console.warn(`Failed to fetch relay list for ${pubkey}:`, error)
      return null
    }
  }

  /**
   * Fetch relay lists for multiple pubkeys efficiently
   */
  async fetchRelayLists(
    pubkeys: string[],
    hints?: string[]
  ): Promise<Map<string, CachedRelayList>> {
    const results = new Map<string, CachedRelayList>()
    const toFetch: string[] = []

    // Check caches first
    for (const pubkey of pubkeys) {
      const cached = await this.getRelayList(pubkey)
      if (cached && !this.isStale(cached)) {
        results.set(pubkey, cached)
      } else {
        toFetch.push(pubkey)
      }
    }

    // Batch fetch the rest
    if (toFetch.length > 0) {
      const fetchPromises = toFetch.map((pubkey) =>
        this.fetchRelayList(pubkey, hints).then((result) => ({
          pubkey,
          result
        }))
      )

      const fetched = await Promise.all(fetchPromises)
      for (const { pubkey, result } of fetched) {
        if (result) {
          results.set(pubkey, result)
        }
      }
    }

    return results
  }

  /**
   * Get combined write relays for multiple recipients
   * Used when publishing events that mention/reply to others
   */
  async getWriteRelaysForRecipients(pubkeys: string[]): Promise<string[]> {
    const relayLists = await this.fetchRelayLists(pubkeys)
    const writeRelays = new Set<string>()

    for (const cached of relayLists.values()) {
      for (const relay of cached.write) {
        writeRelays.add(relay)
      }
    }

    return Array.from(writeRelays)
  }

  /**
   * Store a relay list that was fetched elsewhere (opportunistic caching)
   */
  async setRelayList(event: Event): Promise<void> {
    if (event.kind !== kinds.RelayList) {
      return
    }

    await indexedDb.putReplaceableEvent(event)
    const cached = this.eventToCachedRelayList(event)
    this.memoryCache.set(event.pubkey, cached)
  }

  /**
   * Convert a kind 10002 event to a CachedRelayList
   */
  private eventToCachedRelayList(event: Event): CachedRelayList {
    const relayList = RelayList.fromEvent(event)
    return {
      pubkey: event.pubkey,
      read: relayList.getReadUrls(),
      write: relayList.getWriteUrls(),
      fetchedAt: Date.now(),
      event
    }
  }

  /**
   * Clear all cached relay lists
   */
  clearCache(): void {
    this.memoryCache.clear()
  }
}

const relayListCacheService = new RelayListCacheService()
export default relayListCacheService
