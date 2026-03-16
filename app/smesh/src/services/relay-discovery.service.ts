/**
 * Relay Discovery Service
 *
 * Discovers all known relays on the Nostr network by:
 * 1. Starting with bootstrap relays
 * 2. Querying for NIP-65 relay list events (kind 10002)
 * 3. Extracting relay URLs and doing a second round of queries
 * 4. Compiling a frequency-sorted list of all discovered relays
 */

import { DEPLOYMENT } from '@/constants'
import { kinds, Event as NEvent } from 'nostr-tools'
import client from './client.service'

/** Bootstrap relays to seed the discovery */
const BOOTSTRAP_RELAYS = DEPLOYMENT.defaultRelay
  ? [DEPLOYMENT.defaultRelay]
  : [
      'wss://relay.orly.dev/',
      'wss://relay.damus.io/',
      'wss://relay.nostr.band/',
      'wss://nos.lol/',
      'wss://nostr.wine/',
      'wss://relay.snort.social/',
      'wss://purplepag.es/'
    ]

/** Cache key for localStorage */
const CACHE_KEY = 'relay-discovery-cache'
const CACHE_TTL = 24 * 60 * 60 * 1000 // 24 hours

export type RelayFrequency = {
  url: string
  count: number
  percentage: number
}

export type DiscoveryProgress = {
  phase: 'idle' | 'phase1' | 'phase2' | 'complete'
  relaysQueried: number
  totalRelays: number
  eventsFound: number
  uniqueRelaysFound: number
}

export type DiscoveryResult = {
  relays: RelayFrequency[]
  totalEvents: number
  timestamp: number
}

type CachedResult = DiscoveryResult & {
  cachedAt: number
}

class RelayDiscoveryService {
  private abortController: AbortController | null = null

  /**
   * Normalize a relay URL for consistent comparison
   */
  private normalizeUrl(url: string): string {
    try {
      const parsed = new URL(url.trim())
      // Ensure wss:// protocol and trailing slash
      let normalized = parsed.href
      if (!normalized.endsWith('/')) {
        normalized += '/'
      }
      return normalized.toLowerCase()
    } catch {
      return ''
    }
  }

  /**
   * Extract relay URLs from a NIP-65 relay list event
   */
  private extractRelaysFromEvent(event: NEvent): string[] {
    const relays: string[] = []
    for (const tag of event.tags) {
      if (tag[0] === 'r' && tag[1]) {
        const normalized = this.normalizeUrl(tag[1])
        if (normalized && normalized.startsWith('wss://')) {
          relays.push(normalized)
        }
      }
    }
    return relays
  }

  /**
   * Query relays for NIP-65 relay list events
   */
  private async queryRelayLists(
    relayUrls: string[],
    onProgress?: (queried: number, total: number, events: number) => void
  ): Promise<{ events: NEvent[]; relayFrequency: Map<string, number> }> {
    const events: NEvent[] = []
    const relayFrequency = new Map<string, number>()
    const seenEventIds = new Set<string>()

    // Query all relays in parallel with timeout
    const promises = relayUrls.map(async (relayUrl, index) => {
      if (this.abortController?.signal.aborted) return

      try {
        const relayEvents = await client.fetchEvents(
          [relayUrl],
          {
            kinds: [kinds.RelayList],
            limit: 500
          }
        )

        for (const event of relayEvents) {
          if (seenEventIds.has(event.id)) continue
          seenEventIds.add(event.id)
          events.push(event)

          // Extract and count relays
          const extractedRelays = this.extractRelaysFromEvent(event)
          for (const relay of extractedRelays) {
            relayFrequency.set(relay, (relayFrequency.get(relay) || 0) + 1)
          }
        }

        onProgress?.(index + 1, relayUrls.length, events.length)
      } catch (err) {
        console.warn(`[RelayDiscovery] Failed to query ${relayUrl}:`, err)
      }
    })

    await Promise.allSettled(promises)
    return { events, relayFrequency }
  }

  /**
   * Run the full two-phase discovery process
   */
  async discover(
    onProgress?: (progress: DiscoveryProgress) => void
  ): Promise<DiscoveryResult> {
    // Cancel any existing discovery
    this.abort()
    this.abortController = new AbortController()

    const allRelayFrequency = new Map<string, number>()
    let totalEvents = 0

    // Phase 1: Query bootstrap relays
    onProgress?.({
      phase: 'phase1',
      relaysQueried: 0,
      totalRelays: BOOTSTRAP_RELAYS.length,
      eventsFound: 0,
      uniqueRelaysFound: 0
    })

    const phase1Result = await this.queryRelayLists(
      BOOTSTRAP_RELAYS,
      (queried, total, events) => {
        onProgress?.({
          phase: 'phase1',
          relaysQueried: queried,
          totalRelays: total,
          eventsFound: events,
          uniqueRelaysFound: allRelayFrequency.size
        })
      }
    )

    if (this.abortController.signal.aborted) {
      return this.getEmptyResult()
    }

    // Merge phase 1 results
    for (const [relay, count] of phase1Result.relayFrequency) {
      allRelayFrequency.set(relay, (allRelayFrequency.get(relay) || 0) + count)
    }
    totalEvents += phase1Result.events.length

    // Phase 2: Query discovered relays (top 50 by frequency)
    const discoveredRelays = Array.from(allRelayFrequency.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 50)
      .map(([url]) => url)
      .filter(url => !BOOTSTRAP_RELAYS.includes(url))

    if (discoveredRelays.length > 0 && !this.abortController.signal.aborted) {
      onProgress?.({
        phase: 'phase2',
        relaysQueried: 0,
        totalRelays: discoveredRelays.length,
        eventsFound: totalEvents,
        uniqueRelaysFound: allRelayFrequency.size
      })

      const phase2Result = await this.queryRelayLists(
        discoveredRelays,
        (queried, total, events) => {
          onProgress?.({
            phase: 'phase2',
            relaysQueried: queried,
            totalRelays: total,
            eventsFound: totalEvents + events,
            uniqueRelaysFound: allRelayFrequency.size
          })
        }
      )

      // Merge phase 2 results
      for (const [relay, count] of phase2Result.relayFrequency) {
        allRelayFrequency.set(relay, (allRelayFrequency.get(relay) || 0) + count)
      }
      totalEvents += phase2Result.events.length
    }

    // Build final sorted result
    const relays: RelayFrequency[] = Array.from(allRelayFrequency.entries())
      .map(([url, count]) => ({
        url,
        count,
        percentage: Math.round((count / totalEvents) * 100 * 10) / 10
      }))
      .sort((a, b) => b.count - a.count)

    const result: DiscoveryResult = {
      relays,
      totalEvents,
      timestamp: Date.now()
    }

    // Cache the result
    this.saveToCache(result)

    onProgress?.({
      phase: 'complete',
      relaysQueried: BOOTSTRAP_RELAYS.length + discoveredRelays.length,
      totalRelays: BOOTSTRAP_RELAYS.length + discoveredRelays.length,
      eventsFound: totalEvents,
      uniqueRelaysFound: relays.length
    })

    return result
  }

  /**
   * Abort an in-progress discovery
   */
  abort(): void {
    if (this.abortController) {
      this.abortController.abort()
      this.abortController = null
    }
  }

  /**
   * Get cached discovery result if still valid
   */
  getCachedResult(): DiscoveryResult | null {
    try {
      const cached = localStorage.getItem(CACHE_KEY)
      if (!cached) return null

      const parsed: CachedResult = JSON.parse(cached)
      if (Date.now() - parsed.cachedAt > CACHE_TTL) {
        localStorage.removeItem(CACHE_KEY)
        return null
      }

      return {
        relays: parsed.relays,
        totalEvents: parsed.totalEvents,
        timestamp: parsed.timestamp
      }
    } catch {
      return null
    }
  }

  /**
   * Save result to cache
   */
  private saveToCache(result: DiscoveryResult): void {
    try {
      const cached: CachedResult = {
        ...result,
        cachedAt: Date.now()
      }
      localStorage.setItem(CACHE_KEY, JSON.stringify(cached))
    } catch (err) {
      console.warn('[RelayDiscovery] Failed to cache result:', err)
    }
  }

  /**
   * Get the top N relay URLs from the cached discovery result.
   * Returns empty array if no cached result exists or cache has expired.
   */
  getTopRelays(n: number): string[] {
    const cached = this.getCachedResult()
    if (!cached || cached.relays.length === 0) {
      return []
    }
    return cached.relays.slice(0, n).map(r => r.url)
  }

  /**
   * Get discovered relays in batches for progressive querying.
   * Returns arrays of relay URLs, each batch of `batchSize`,
   * starting after `offset` relays, up to `maxTotal` total relays.
   * Excludes relays in the `exclude` set.
   */
  getRelayBatches(
    batchSize: number,
    offset: number,
    maxTotal: number,
    exclude: Set<string>
  ): string[][] {
    const cached = this.getCachedResult()
    if (!cached || cached.relays.length === 0) {
      return []
    }

    const available = cached.relays
      .map(r => r.url)
      .filter(url => !exclude.has(url))
      .slice(offset, offset + maxTotal)

    const batches: string[][] = []
    for (let i = 0; i < available.length; i += batchSize) {
      batches.push(available.slice(i, i + batchSize))
    }
    return batches
  }

  /**
   * Run discovery if no valid cached result exists.
   * Intended for background auto-discovery on app startup.
   */
  async discoverIfNeeded(): Promise<void> {
    const cached = this.getCachedResult()
    if (cached && cached.relays.length > 0) {
      return // Cache is fresh, no discovery needed
    }

    console.log('[RelayDiscovery] No cached result, starting background discovery...')
    try {
      await this.discover()
      console.log('[RelayDiscovery] Background discovery complete')
    } catch (err) {
      console.warn('[RelayDiscovery] Background discovery failed:', err)
    }
  }

  /**
   * Clear the cache
   */
  clearCache(): void {
    localStorage.removeItem(CACHE_KEY)
  }

  /**
   * Export relay list as plaintext (one URL per line)
   */
  exportAsPlaintext(relays: RelayFrequency[]): string {
    return relays.map(r => r.url).join('\n')
  }

  /**
   * Download relay list as a text file
   */
  downloadAsFile(relays: RelayFrequency[], filename = 'nostr-relays.txt'): void {
    const content = this.exportAsPlaintext(relays)
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  private getEmptyResult(): DiscoveryResult {
    return {
      relays: [],
      totalEvents: 0,
      timestamp: Date.now()
    }
  }
}

const relayDiscoveryService = new RelayDiscoveryService()
export default relayDiscoveryService
