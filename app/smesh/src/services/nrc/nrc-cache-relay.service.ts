/**
 * NRC Cache Relay Service
 *
 * Manages NRC connections that act as "cache relays":
 * - Query first with 400ms timeout before falling back to regular relays
 * - Push loaded events to cache relays in background
 *
 * Cache relays are private relays accessible via NRC that can store
 * a user's viewed events for faster subsequent access.
 */

import { Event, Filter } from 'nostr-tools'
import { NRCClient, SyncProgress } from './nrc-client.service'

/**
 * Configuration for an NRC cache relay
 */
export interface NRCCacheRelayConfig {
  id: string
  uri: string // nostr+relayconnect:// URI
  label: string
  enabled: boolean
  queryFirst: boolean // Query before regular relays with 400ms timeout
  pushEvents: boolean // Push loaded events in background
  lastConnected?: number
  lastError?: string
}

/**
 * Cache relay query result
 */
export interface CacheRelayQueryResult {
  events: Event[]
  fromCache: boolean
  relayId?: string
}

// Storage key for cache relay configs
const STORAGE_KEY = 'nrc-cache-relays'

// Default query timeout for cache relays (400ms)
const DEFAULT_CACHE_QUERY_TIMEOUT = 400

// Maximum events per push batch
const MAX_PUSH_BATCH_SIZE = 50

// Debounce time for push batching (ms)
const PUSH_DEBOUNCE_MS = 100

class NRCCacheRelayService extends EventTarget {
  private configs: NRCCacheRelayConfig[] = []
  private pushQueue: Event[] = []
  private pushInProgress = false
  private pushTimeout: ReturnType<typeof setTimeout> | null = null
  private seenEventIds: Set<string> = new Set()

  constructor() {
    super()
    this.loadConfigs()
  }

  /**
   * Load configurations from storage
   */
  private loadConfigs(): void {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY)
      if (stored) {
        this.configs = JSON.parse(stored)
      }
    } catch (err) {
      console.error('[NRC Cache] Failed to load configs:', err)
      this.configs = []
    }
  }

  /**
   * Save configurations to storage
   */
  private saveConfigs(): void {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(this.configs))
    } catch (err) {
      console.error('[NRC Cache] Failed to save configs:', err)
    }
  }

  /**
   * Get all cache relay configurations
   */
  getAll(): NRCCacheRelayConfig[] {
    return [...this.configs]
  }

  /**
   * Get enabled cache relays that should be queried first
   */
  getQueryFirstRelays(): NRCCacheRelayConfig[] {
    return this.configs.filter((c) => c.enabled && c.queryFirst)
  }

  /**
   * Get enabled cache relays that should receive pushed events
   */
  getPushRelays(): NRCCacheRelayConfig[] {
    return this.configs.filter((c) => c.enabled && c.pushEvents)
  }

  /**
   * Add a new cache relay configuration
   */
  add(config: Omit<NRCCacheRelayConfig, 'id'>): NRCCacheRelayConfig {
    const newConfig: NRCCacheRelayConfig = {
      ...config,
      id: crypto.randomUUID()
    }
    this.configs.push(newConfig)
    this.saveConfigs()
    this.dispatchEvent(new CustomEvent('configsChanged'))
    return newConfig
  }

  /**
   * Update a cache relay configuration
   */
  update(id: string, updates: Partial<NRCCacheRelayConfig>): void {
    const index = this.configs.findIndex((c) => c.id === id)
    if (index >= 0) {
      this.configs[index] = { ...this.configs[index], ...updates }
      this.saveConfigs()
      this.dispatchEvent(new CustomEvent('configsChanged'))
    }
  }

  /**
   * Remove a cache relay configuration
   */
  remove(id: string): void {
    this.configs = this.configs.filter((c) => c.id !== id)
    this.saveConfigs()
    this.dispatchEvent(new CustomEvent('configsChanged'))
  }

  /**
   * Query cache relays with timeout
   *
   * Returns events from the first cache relay that responds within the timeout.
   * If no cache relay responds in time, returns an empty array.
   *
   * @param filters - Nostr filters to query
   * @param timeoutMs - Maximum time to wait for response (default: 400ms)
   * @returns Events from cache relay, or empty array if none respond in time
   */
  async queryWithTimeout(
    filters: Filter[],
    timeoutMs: number = DEFAULT_CACHE_QUERY_TIMEOUT
  ): Promise<CacheRelayQueryResult> {
    const queryRelays = this.getQueryFirstRelays()

    if (queryRelays.length === 0) {
      return { events: [], fromCache: false }
    }

    // Race all cache relays against a timeout
    const queryPromises = queryRelays.map(async (config) => {
      try {
        const client = new NRCClient(config.uri)
        const events = await client.sync(filters, undefined, timeoutMs + 5000) // Add buffer to timeout

        // Update last connected
        this.update(config.id, {
          lastConnected: Date.now(),
          lastError: undefined
        })

        return { events, relayId: config.id }
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : String(err)
        console.warn(`[NRC Cache] Query failed for ${config.label}:`, errorMsg)

        // Update error state
        this.update(config.id, {
          lastError: errorMsg
        })

        throw err
      }
    })

    // Create timeout promise
    const timeoutPromise = new Promise<never>((_, reject) => {
      setTimeout(() => reject(new Error('Cache query timeout')), timeoutMs)
    })

    try {
      // Race: first successful response wins, or timeout
      const result = await Promise.race([Promise.any(queryPromises), timeoutPromise])

      if (result && 'events' in result) {
        console.log(
          `[NRC Cache] Got ${result.events.length} events from cache relay in <${timeoutMs}ms`
        )
        return {
          events: result.events,
          fromCache: true,
          relayId: result.relayId
        }
      }
    } catch (err) {
      // All queries failed or timed out
      console.log('[NRC Cache] No cache relay responded in time')
    }

    return { events: [], fromCache: false }
  }

  /**
   * Queue an event for background push to cache relays
   *
   * Events are batched and pushed in the background to avoid
   * blocking the main thread.
   */
  queueEventForPush(event: Event): void {
    // Skip if already seen
    if (this.seenEventIds.has(event.id)) {
      return
    }
    this.seenEventIds.add(event.id)

    // Add to queue
    this.pushQueue.push(event)

    // Schedule batch push if not already scheduled
    if (!this.pushTimeout && !this.pushInProgress) {
      this.pushTimeout = setTimeout(() => {
        this.pushTimeout = null
        this.processPushQueue()
      }, PUSH_DEBOUNCE_MS)
    }
  }

  /**
   * Queue multiple events for background push
   */
  queueEventsForPush(events: Event[]): void {
    for (const event of events) {
      this.queueEventForPush(event)
    }
  }

  /**
   * Process the push queue in batches
   */
  private async processPushQueue(): Promise<void> {
    if (this.pushQueue.length === 0 || this.pushInProgress) {
      return
    }

    const pushRelays = this.getPushRelays()
    if (pushRelays.length === 0) {
      // No push relays configured, clear the queue
      this.pushQueue = []
      return
    }

    this.pushInProgress = true

    // Take a batch from the queue
    const batch = this.pushQueue.splice(0, MAX_PUSH_BATCH_SIZE)
    console.log(`[NRC Cache] Pushing ${batch.length} events to ${pushRelays.length} cache relays`)

    // Push to all configured relays in parallel
    const pushPromises = pushRelays.map(async (config) => {
      try {
        const client = new NRCClient(config.uri)
        const sentCount = await client.sendEvents(batch, (progress: SyncProgress) => {
          // Optional: track progress
          if (progress.phase === 'error') {
            console.warn(`[NRC Cache] Push error to ${config.label}: ${progress.message}`)
          }
        })

        console.log(`[NRC Cache] Pushed ${sentCount}/${batch.length} events to ${config.label}`)

        // Update last connected
        this.update(config.id, {
          lastConnected: Date.now(),
          lastError: undefined
        })

        return sentCount
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : String(err)
        console.warn(`[NRC Cache] Push failed to ${config.label}:`, errorMsg)

        // Update error state
        this.update(config.id, {
          lastError: errorMsg
        })

        return 0
      }
    })

    await Promise.allSettled(pushPromises)

    this.pushInProgress = false

    // If there are more events in the queue, schedule another batch
    if (this.pushQueue.length > 0) {
      this.pushTimeout = setTimeout(() => {
        this.pushTimeout = null
        this.processPushQueue()
      }, PUSH_DEBOUNCE_MS)
    }
  }

  /**
   * Test connection to a cache relay
   */
  async testConnection(
    uri: string,
    onProgress?: (progress: SyncProgress) => void
  ): Promise<boolean> {
    try {
      const client = new NRCClient(uri)
      // Request just one profile event to test the full round-trip
      const events = await client.sync([{ kinds: [0], limit: 1 }], onProgress, 15000)
      console.log(`[NRC Cache] Test connection successful, received ${events.length} events`)
      return true
    } catch (err) {
      console.error('[NRC Cache] Test connection failed:', err)
      throw err
    }
  }

  /**
   * Clear the seen event IDs cache
   * Call this periodically to prevent memory growth
   */
  clearSeenCache(): void {
    // Keep a reasonable size limit
    if (this.seenEventIds.size > 10000) {
      this.seenEventIds.clear()
    }
  }

  /**
   * Get push queue status
   */
  getPushQueueStatus(): { queueSize: number; inProgress: boolean } {
    return {
      queueSize: this.pushQueue.length,
      inProgress: this.pushInProgress
    }
  }
}

// Singleton instance
const instance = new NRCCacheRelayService()
export default instance

export { NRCCacheRelayService }
