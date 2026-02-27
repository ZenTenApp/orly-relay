import { RECOMMENDED_BLOSSOM_SERVERS, RECOMMENDED_SEARCH_RELAYS } from '@/constants'
import client from '@/services/client.service'
import { getHashFromURL } from 'blossom-client-sdk'
import { parseResponsiveImageEvent, UploadedVariant, FILE_METADATA_KIND } from '@/lib/responsive-image-event'

// Timeout for HEAD request to check if server is responsive (ms)
const HEAD_TIMEOUT = 2000
// Timeout before trying mirrors in parallel (ms)
const MIRROR_FALLBACK_DELAY = 1500

interface CacheEntry {
  pubkey?: string
  resolve: (url: string) => void
  promise: Promise<string>
  tried: Set<string>
  validUrl?: string
  validating?: boolean
}

class BlossomService {
  static instance: BlossomService
  private cacheMap = new Map<string, CacheEntry>()

  constructor() {
    if (!BlossomService.instance) {
      BlossomService.instance = this
    }
    return BlossomService.instance
  }

  /**
   * Validates a URL by sending a HEAD request with timeout.
   * Returns true if the server responds quickly with 200/206.
   */
  private async validateUrl(url: string, timeoutMs: number = HEAD_TIMEOUT): Promise<boolean> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

    try {
      const response = await fetch(url, {
        method: 'HEAD',
        signal: controller.signal,
        mode: 'cors'
      })
      clearTimeout(timeoutId)
      return response.ok
    } catch {
      clearTimeout(timeoutId)
      return false
    }
  }

  /**
   * Builds a blob URL from a server base URL and hash.
   */
  private buildBlobUrl(serverUrl: string, hash: string, ext: string | null): string {
    try {
      const url = new URL(serverUrl)
      url.pathname = '/' + hash + (ext || '')
      return url.toString()
    } catch {
      return serverUrl + '/' + hash + (ext || '')
    }
  }

  /**
   * Gets all available Blossom servers for a pubkey.
   * Includes user's servers + recommended fallbacks.
   */
  private async getBlossomServers(pubkey: string): Promise<string[]> {
    const userServers = await client.fetchBlossomServerList(pubkey)
    const allServers = [...userServers]

    // Add recommended servers as fallbacks (if not already in list)
    for (const server of RECOMMENDED_BLOSSOM_SERVERS) {
      try {
        const serverUrl = new URL(server)
        if (!allServers.some((s) => new URL(s).hostname === serverUrl.hostname)) {
          allServers.push(server)
        }
      } catch {
        // Skip invalid URLs
      }
    }

    return allServers
  }

  /**
   * Races multiple URLs to find the first one that responds.
   * Returns the winning URL or null if none respond.
   */
  private async raceUrls(urls: string[], timeoutMs: number): Promise<string | null> {
    if (urls.length === 0) return null

    return new Promise((resolve) => {
      let resolved = false
      let pendingCount = urls.length

      const tryUrl = async (url: string) => {
        const isValid = await this.validateUrl(url, timeoutMs)
        if (isValid && !resolved) {
          resolved = true
          resolve(url)
        } else {
          pendingCount--
          if (pendingCount === 0 && !resolved) {
            resolve(null)
          }
        }
      }

      urls.forEach((url) => tryUrl(url))
    })
  }

  /**
   * Gets a valid URL for an image, proactively checking and racing mirrors.
   *
   * Strategy:
   * 1. If cached and valid, return immediately
   * 2. Start validating original URL
   * 3. After MIRROR_FALLBACK_DELAY, start racing mirrors in parallel
   * 4. Return first URL that responds successfully
   */
  async getValidUrl(url: string, pubkey: string): Promise<string> {
    // Check cache first
    const cache = this.cacheMap.get(url)
    if (cache?.validUrl) {
      return cache.validUrl
    }
    if (cache?.promise && cache.validating) {
      return cache.promise
    }

    // Parse URL to extract hash
    let hash: string | null = null
    let ext: string | null = null
    try {
      const parsedUrl = new URL(url)
      hash = getHashFromURL(parsedUrl)
      const extMatch = parsedUrl.pathname.match(/\.\w+$/i)
      ext = extMatch ? extMatch[0] : null
    } catch {
      return url
    }

    if (!hash) {
      return url
    }

    // Create cache entry with promise
    let resolveFunc: (url: string) => void
    const promise = new Promise<string>((resolve) => {
      resolveFunc = resolve
    })

    const entry: CacheEntry = {
      pubkey,
      resolve: resolveFunc!,
      promise,
      tried: new Set<string>(),
      validating: true
    }
    this.cacheMap.set(url, entry)

    // Start validation in background
    this.validateAndFindBestUrl(url, pubkey, hash, ext, entry)

    // Return promise that will resolve to the best URL
    return promise
  }

  /**
   * Background validation that races the original URL against mirrors.
   */
  private async validateAndFindBestUrl(
    originalUrl: string,
    pubkey: string,
    hash: string,
    ext: string | null,
    entry: CacheEntry
  ): Promise<void> {
    const originalHostname = new URL(originalUrl).hostname
    entry.tried.add(originalHostname)

    // Race: original URL validation vs timeout to try mirrors
    const originalValidPromise = this.validateUrl(originalUrl, HEAD_TIMEOUT)
    const delayPromise = new Promise<'timeout'>((resolve) =>
      setTimeout(() => resolve('timeout'), MIRROR_FALLBACK_DELAY)
    )

    const firstResult = await Promise.race([
      originalValidPromise.then((valid) => ({ type: 'original' as const, valid })),
      delayPromise.then(() => ({ type: 'timeout' as const }))
    ])

    // If original responded quickly and is valid, use it
    if (firstResult.type === 'original' && firstResult.valid) {
      entry.validUrl = originalUrl
      entry.validating = false
      entry.resolve(originalUrl)
      return
    }

    // Original is slow or failed, try mirrors in parallel
    const servers = await this.getBlossomServers(pubkey)
    const mirrorUrls = servers
      .map((server) => {
        try {
          const serverHostname = new URL(server).hostname
          if (entry.tried.has(serverHostname)) return null
          entry.tried.add(serverHostname)
          return this.buildBlobUrl(server, hash, ext)
        } catch {
          return null
        }
      })
      .filter((u): u is string => u !== null)

    // If original is still pending, include it in the race
    const urlsToRace = [...mirrorUrls]
    if (firstResult.type === 'timeout') {
      // Original is still pending, wait for it too
      const originalStillValid = await originalValidPromise
      if (originalStillValid) {
        entry.validUrl = originalUrl
        entry.validating = false
        entry.resolve(originalUrl)
        return
      }
    }

    // Race all mirrors
    const winningUrl = await this.raceUrls(urlsToRace, HEAD_TIMEOUT)
    if (winningUrl) {
      entry.validUrl = winningUrl
      entry.validating = false
      entry.resolve(winningUrl)
      return
    }

    // All failed, fall back to original (browser will handle the error)
    entry.validUrl = originalUrl
    entry.validating = false
    entry.resolve(originalUrl)
  }

  /**
   * Tries the next available URL when current one fails.
   * Called by Image component on load error.
   */
  async tryNextUrl(originalUrl: string): Promise<string | null> {
    const entry = this.cacheMap.get(originalUrl)
    if (!entry) {
      return null
    }

    if (entry.validUrl && entry.validUrl !== originalUrl) {
      return entry.validUrl
    }

    const { pubkey, tried, resolve } = entry
    let hash: string | null = null
    let ext: string | null = null
    try {
      const parsedUrl = new URL(originalUrl)
      hash = getHashFromURL(parsedUrl)
      const extMatch = parsedUrl.pathname.match(/\.\w+$/i)
      ext = extMatch ? extMatch[0] : null
    } catch {
      resolve(originalUrl)
      return null
    }

    if (!pubkey || !hash) {
      resolve(originalUrl)
      return null
    }

    const blossomServerList = await this.getBlossomServers(pubkey)
    const urls = blossomServerList
      .map((server) => {
        try {
          const serverUrl = new URL(server)
          if (tried.has(serverUrl.hostname)) return null
          tried.add(serverUrl.hostname)
          return this.buildBlobUrl(server, hash!, ext)
        } catch {
          return null
        }
      })
      .filter((u): u is string => u !== null)

    // Try each URL with validation
    for (const url of urls) {
      const isValid = await this.validateUrl(url, HEAD_TIMEOUT)
      if (isValid) {
        entry.validUrl = url
        resolve(url)
        return url
      }
    }

    resolve(originalUrl)
    return null
  }

  markAsSuccess(originalUrl: string, successUrl: string) {
    const entry = this.cacheMap.get(originalUrl)
    if (!entry) {
      this.cacheMap.set(originalUrl, {
        resolve: () => {},
        promise: Promise.resolve(successUrl),
        tried: new Set<string>(),
        validUrl: successUrl,
        validating: false
      })
      return
    }

    entry.resolve(successUrl)
    entry.validUrl = successUrl
    entry.validating = false
  }

  /**
   * Check if a string is a valid 64-character hex blob hash
   */
  isValidBlobHash(str: string): boolean {
    return /^[a-f0-9]{64}$/i.test(str)
  }

  /**
   * Query relays for a responsive image binding event (kind 1063) by blob hash.
   * Uses the user's NIP-65 relay list for discovery.
   *
   * @param blobHash - The 64-character SHA256 hash of any variant
   * @returns Parsed variants from the binding event, or null if not found
   */
  async queryBindingEvent(blobHash: string): Promise<UploadedVariant[] | null> {
    if (!this.isValidBlobHash(blobHash)) {
      return null
    }

    const hash = blobHash.toLowerCase()

    try {
      // Query for kind 1063 events with matching x tag
      // Use recommended search relays combined with user's current relays
      const relaysToQuery = Array.from(new Set([
        ...client.currentRelays,
        ...RECOMMENDED_SEARCH_RELAYS
      ]))

      const events = await client.fetchEvents(relaysToQuery, {
        kinds: [FILE_METADATA_KIND],
        '#x': [hash],
        limit: 1
      })

      if (events.length === 0) {
        return null
      }

      // Parse the first matching event
      const event = events[0]
      const variants = parseResponsiveImageEvent(event)

      return variants.length > 0 ? variants : null
    } catch {
      return null
    }
  }

  /**
   * Extract a blob hash from a Blossom URL.
   * Handles formats like:
   * - https://server.com/abc123...def456
   * - https://server.com/abc123...def456.jpg
   *
   * @param url - A Blossom blob URL
   * @returns The 64-character hash, or null if not found
   */
  extractHashFromUrl(url: string): string | null {
    try {
      const hash = getHashFromURL(new URL(url))
      return hash && this.isValidBlobHash(hash) ? hash.toLowerCase() : null
    } catch {
      return null
    }
  }
}

const instance = new BlossomService()
export default instance
