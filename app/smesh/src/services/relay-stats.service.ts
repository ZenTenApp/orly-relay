import type {
  TRelayEntry,
  TNetworkRelayStats,
  TRelayDirection,
} from '@/types/relay-management'
import {
  AUTO_DISABLE_FAILURE_RATE,
  AUTO_DISABLE_MIN_ATTEMPTS,
  DIRECTION_BITS,
  STATUS_BITS,
  DIRECTION_FROM_BITS,
  STATUS_FROM_BITS,
} from '@/types/relay-management'
import { normalizeUrl } from '@/lib/url'
import networkIdentityService from './network-identity.service'
import indexedDb from './indexed-db.service'
import { ipToBytes, bytesToIp, ipToHex } from './network-identity.service'

const FLUSH_INTERVAL_MS = 10_000

interface StoredNetworkStats {
  publishSuccess: number
  publishFailure: number
  fetchSuccess: number
  fetchFailure: number
  lastSeen: number
}

interface StoredRelayEntry {
  url: string
  direction: TRelayDirection
  relayIp: string | null
  networkStats: Array<[string, StoredNetworkStats]>
  manualExclude: boolean
  status: 'pending' | 'approved' | 'rejected'
  reason: string
  addedAt: number
  updatedAt: number
}

class RelayStatsService {
  private entries: Map<string, TRelayEntry> = new Map()
  private dirtyUrls: Set<string> = new Set()
  private flushTimer: ReturnType<typeof setTimeout> | null = null
  private initPromise: Promise<void> | null = null

  async init(): Promise<void> {
    if (this.initPromise) return this.initPromise
    this.initPromise = this.loadFromDb()
    return this.initPromise
  }

  private async loadFromDb(): Promise<void> {
    const rows = await indexedDb.getAllRelayStats()
    for (const row of rows) {
      const stored = row.value as StoredRelayEntry
      if (!stored || !stored.url) continue
      const networkStats = new Map<string, TNetworkRelayStats>()
      if (Array.isArray(stored.networkStats)) {
        for (const [key, val] of stored.networkStats) {
          networkStats.set(key, { ...val })
        }
      }
      this.entries.set(stored.url, {
        url: stored.url,
        direction: stored.direction ?? 'outbox',
        relayIp: stored.relayIp ?? null,
        networkStats,
        manualExclude: stored.manualExclude ?? false,
        status: stored.status ?? 'pending',
        reason: stored.reason ?? '',
        addedAt: stored.addedAt ?? Date.now(),
        updatedAt: stored.updatedAt ?? Date.now(),
      })
    }
  }

  getOrCreateEntry(url: string): TRelayEntry {
    url = normalizeUrl(url)
    let entry = this.entries.get(url)
    if (!entry) {
      entry = {
        url,
        direction: 'outbox',
        relayIp: null,
        networkStats: new Map(),
        manualExclude: false,
        status: 'pending',
        reason: '',
        addedAt: Date.now(),
        updatedAt: Date.now(),
      }
      this.entries.set(url, entry)
    }
    return entry
  }

  recordPublishSuccess(url: string): void {
    this.recordStat(url, 'publishSuccess')
  }

  recordPublishFailure(url: string): void {
    this.recordStat(url, 'publishFailure')
  }

  recordFetchSuccess(url: string): void {
    this.recordStat(url, 'fetchSuccess')
  }

  recordFetchFailure(url: string): void {
    this.recordStat(url, 'fetchFailure')
  }

  private recordStat(
    url: string,
    field: keyof Pick<TNetworkRelayStats, 'publishSuccess' | 'publishFailure' | 'fetchSuccess' | 'fetchFailure'>
  ): void {
    const entry = this.getOrCreateEntry(url)
    const ipHex = networkIdentityService.getCurrentIpHex() ?? 'unknown'
    let stats = entry.networkStats.get(ipHex)
    if (!stats) {
      stats = {
        publishSuccess: 0,
        publishFailure: 0,
        fetchSuccess: 0,
        fetchFailure: 0,
        lastSeen: Date.now(),
      }
      entry.networkStats.set(ipHex, stats)
    }
    stats[field]++
    stats.lastSeen = Date.now()
    entry.updatedAt = Date.now()
    this.dirtyUrls.add(url)
    this.scheduleFlush()
  }

  getFailureRate(url: string): number {
    const entry = this.entries.get(normalizeUrl(url))
    if (!entry) return 0
    const ipHex = networkIdentityService.getCurrentIpHex() ?? 'unknown'
    const stats = entry.networkStats.get(ipHex)
    if (!stats) return 0
    const total =
      stats.publishSuccess + stats.publishFailure + stats.fetchSuccess + stats.fetchFailure
    if (total === 0) return 0
    return (stats.publishFailure + stats.fetchFailure) / total
  }

  isAutoDisabled(url: string): boolean {
    const entry = this.entries.get(normalizeUrl(url))
    if (!entry) return false
    const ipHex = networkIdentityService.getCurrentIpHex() ?? 'unknown'
    const stats = entry.networkStats.get(ipHex)
    if (!stats) return false
    const total =
      stats.publishSuccess + stats.publishFailure + stats.fetchSuccess + stats.fetchFailure
    if (total <= AUTO_DISABLE_MIN_ATTEMPTS) return false
    const failureRate = (stats.publishFailure + stats.fetchFailure) / total
    return failureRate >= AUTO_DISABLE_FAILURE_RATE
  }

  getEntry(url: string): TRelayEntry | undefined {
    return this.entries.get(normalizeUrl(url))
  }

  getAllEntries(): TRelayEntry[] {
    return Array.from(this.entries.values())
  }

  setRelayIp(url: string, ip: string): void {
    url = normalizeUrl(url)
    const entry = this.getOrCreateEntry(url)
    entry.relayIp = ip
    entry.updatedAt = Date.now()
    this.dirtyUrls.add(url)
    this.scheduleFlush()
  }

  updateEntry(
    url: string,
    updates: Partial<Pick<TRelayEntry, 'status' | 'direction' | 'manualExclude' | 'reason'>>
  ): void {
    const entry = this.getOrCreateEntry(url)
    if (updates.status !== undefined) entry.status = updates.status
    if (updates.direction !== undefined) entry.direction = updates.direction
    if (updates.manualExclude !== undefined) entry.manualExclude = updates.manualExclude
    if (updates.reason !== undefined) entry.reason = updates.reason
    entry.updatedAt = Date.now()
    this.dirtyUrls.add(url)
    this.scheduleFlush()
  }

  private scheduleFlush(): void {
    if (this.flushTimer) return
    this.flushTimer = setTimeout(() => {
      this.flushTimer = null
      this.flush()
    }, FLUSH_INTERVAL_MS)
  }

  private async flush(): Promise<void> {
    if (this.dirtyUrls.size === 0) return
    const urls = Array.from(this.dirtyUrls)
    this.dirtyUrls.clear()

    for (const url of urls) {
      const entry = this.entries.get(url)
      if (!entry) continue
      const serialized: StoredRelayEntry = {
        url: entry.url,
        direction: entry.direction,
        relayIp: entry.relayIp,
        networkStats: Array.from(entry.networkStats.entries()),
        manualExclude: entry.manualExclude,
        status: entry.status,
        reason: entry.reason,
        addedAt: entry.addedAt,
        updatedAt: entry.updatedAt,
      }
      await indexedDb.putRelayStats(url, serialized)
    }
  }

  encodeBinary(): Uint8Array {
    const entries = this.getAllEntries()
    const encoder = new TextEncoder()

    // Calculate total size
    let totalSize = 0
    const encodedUrls: Uint8Array[] = []
    for (const entry of entries) {
      const urlBytes = encoder.encode(entry.url)
      encodedUrls.push(urlBytes)
      // 1 (url len) + N (url) + 1 (flags) + 4 (relay ip) + 1 (stats count) + K*12 (stats)
      totalSize += 1 + urlBytes.length + 1 + 4 + 1 + entry.networkStats.size * 12
    }

    const buffer = new ArrayBuffer(totalSize)
    const view = new DataView(buffer)
    const bytes = new Uint8Array(buffer)
    let offset = 0

    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i]
      const urlBytes = encodedUrls[i]

      // URL length + URL bytes
      view.setUint8(offset, urlBytes.length)
      offset += 1
      bytes.set(urlBytes, offset)
      offset += urlBytes.length

      // Flags: direction(2 bits) | status(2 bits) | manualExclude(1 bit) | reserved(3 bits)
      const dirBits = DIRECTION_BITS[entry.direction] ?? 0
      const stsBits = STATUS_BITS[entry.status] ?? 0
      const exclBit = entry.manualExclude ? 1 : 0
      const flags = ((dirBits & 0x3) << 6) | ((stsBits & 0x3) << 4) | ((exclBit & 0x1) << 3)
      view.setUint8(offset, flags)
      offset += 1

      // Relay IPv4 (4 bytes)
      const relayIpBytes = ipToBytes(entry.relayIp ?? '0.0.0.0')
      bytes.set(relayIpBytes, offset)
      offset += 4

      // Network stats count
      view.setUint8(offset, Math.min(entry.networkStats.size, 255))
      offset += 1

      // Network stats entries
      let statsWritten = 0
      for (const [ipHex, stats] of entry.networkStats) {
        if (statsWritten >= 255) break
        // Client external IPv4 from hex (4 bytes)
        const clientIpBytes = ipHexToBytes(ipHex)
        bytes.set(clientIpBytes, offset)
        offset += 4

        view.setUint16(offset, Math.min(stats.publishSuccess, 65535))
        offset += 2
        view.setUint16(offset, Math.min(stats.publishFailure, 65535))
        offset += 2
        view.setUint16(offset, Math.min(stats.fetchSuccess, 65535))
        offset += 2
        view.setUint16(offset, Math.min(stats.fetchFailure, 65535))
        offset += 2
        statsWritten++
      }
    }

    return new Uint8Array(buffer, 0, offset)
  }

  decodeBinary(data: Uint8Array): void {
    const view = new DataView(data.buffer, data.byteOffset, data.byteLength)
    const decoder = new TextDecoder()
    let offset = 0

    while (offset < data.length) {
      if (offset + 1 > data.length) break

      // URL length + URL
      const urlLen = view.getUint8(offset)
      offset += 1
      if (offset + urlLen > data.length) break
      const url = decoder.decode(data.subarray(offset, offset + urlLen))
      offset += urlLen

      if (offset + 6 > data.length) break

      // Flags
      const flags = view.getUint8(offset)
      offset += 1
      const dirBits = (flags >> 6) & 0x3
      const stsBits = (flags >> 4) & 0x3
      const exclBit = (flags >> 3) & 0x1
      const direction = DIRECTION_FROM_BITS[dirBits]
      const status = STATUS_FROM_BITS[stsBits]
      const manualExclude = exclBit === 1

      // Relay IPv4
      const relayIpRaw = bytesToIp(data.subarray(offset, offset + 4))
      const relayIp = relayIpRaw === '0.0.0.0' ? null : relayIpRaw
      offset += 4

      // Network stats count
      const statsCount = view.getUint8(offset)
      offset += 1

      const incomingStats = new Map<string, TNetworkRelayStats>()
      for (let s = 0; s < statsCount; s++) {
        if (offset + 12 > data.length) break

        const clientIpBytes = data.subarray(offset, offset + 4)
        const clientIpHex = ipToHex(bytesToIp(clientIpBytes))
        offset += 4

        const publishSuccess = view.getUint16(offset)
        offset += 2
        const publishFailure = view.getUint16(offset)
        offset += 2
        const fetchSuccess = view.getUint16(offset)
        offset += 2
        const fetchFailure = view.getUint16(offset)
        offset += 2

        incomingStats.set(clientIpHex, {
          publishSuccess,
          publishFailure,
          fetchSuccess,
          fetchFailure,
          lastSeen: Date.now(),
        })
      }

      // Merge into existing entries
      const existing = this.entries.get(url)
      if (existing) {
        // Merge network stats with monotonic max
        for (const [ipHex, incoming] of incomingStats) {
          const local = existing.networkStats.get(ipHex)
          if (local) {
            local.publishSuccess = Math.max(local.publishSuccess, incoming.publishSuccess)
            local.publishFailure = Math.max(local.publishFailure, incoming.publishFailure)
            local.fetchSuccess = Math.max(local.fetchSuccess, incoming.fetchSuccess)
            local.fetchFailure = Math.max(local.fetchFailure, incoming.fetchFailure)
            local.lastSeen = Math.max(local.lastSeen, incoming.lastSeen)
          } else {
            existing.networkStats.set(ipHex, incoming)
          }
        }
        // Keep local flags for existing entries
        if (relayIp && !existing.relayIp) {
          existing.relayIp = relayIp
        }
        existing.updatedAt = Date.now()
      } else {
        this.entries.set(url, {
          url,
          direction,
          relayIp,
          networkStats: incomingStats,
          manualExclude,
          status,
          reason: '',
          addedAt: Date.now(),
          updatedAt: Date.now(),
        })
      }

      this.dirtyUrls.add(url)
    }

    if (this.dirtyUrls.size > 0) {
      this.scheduleFlush()
    }
  }
}

/** Convert 8-char hex IP to 4-byte Uint8Array. Handles 'unknown' and malformed input. */
function ipHexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(4)
  if (hex.length !== 8) return bytes
  for (let i = 0; i < 4; i++) {
    const n = parseInt(hex.slice(i * 2, i * 2 + 2), 16)
    bytes[i] = isNaN(n) ? 0 : n & 0xff
  }
  return bytes
}

const relayStatsService = new RelayStatsService()
export default relayStatsService
