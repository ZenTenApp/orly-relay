import relayStatsService from './relay-stats.service'
import storage from './local-storage.service'
import type { TRelayDirection, TOutboxMode, TRelayEntry } from '@/types/relay-management'

class ManagedOutboxService {
  filterRelayUrls(
    urls: string[],
    direction: TRelayDirection,
    ownRelays?: Set<string>
  ): string[] {
    const mode = storage.getOutboxMode() as TOutboxMode
    const allowed: string[] = []

    for (const url of urls) {
      if (ownRelays?.has(url)) {
        allowed.push(url)
        continue
      }

      const entry = relayStatsService.getEntry(url)

      if (entry?.manualExclude) continue
      if (relayStatsService.isAutoDisabled(url)) continue

      if (mode === 'automatic') {
        if (!entry) {
          this.addPending(url, direction)
        }
        allowed.push(url)
        continue
      }

      // mode === 'managed'
      if (entry?.status === 'approved') {
        allowed.push(url)
      } else if (entry?.status === 'rejected') {
        // blocked
      } else {
        // pending or no entry
        this.addPending(url, direction)
      }
    }

    return allowed
  }

  addPending(url: string, direction: TRelayDirection, reason?: string): void {
    const existing = relayStatsService.getEntry(url)
    if (!existing) {
      const entry = relayStatsService.getOrCreateEntry(url)
      entry.direction = direction
      if (reason) entry.reason = reason
      // status defaults to 'pending' from getOrCreateEntry
      return
    }
    if (existing.direction !== direction && existing.direction !== 'both') {
      relayStatsService.updateEntry(url, { direction: 'both' })
    }
    if (reason && !existing.reason) {
      relayStatsService.updateEntry(url, { reason })
    }
  }

  approve(url: string): void {
    relayStatsService.updateEntry(url, { status: 'approved' })
  }

  reject(url: string): void {
    relayStatsService.updateEntry(url, { status: 'rejected' })
  }

  resetStatus(url: string): void {
    relayStatsService.updateEntry(url, { status: 'pending' })
  }

  setManualExclude(url: string, excluded: boolean): void {
    relayStatsService.updateEntry(url, { manualExclude: excluded })
  }

  bulkApprove(urls: string[]): void {
    for (const url of urls) this.approve(url)
  }

  bulkReject(urls: string[]): void {
    for (const url of urls) this.reject(url)
  }

  getPendingRelays(): TRelayEntry[] {
    return relayStatsService.getAllEntries().filter((e) => e.status === 'pending')
  }

  getApprovedRelays(): TRelayEntry[] {
    return relayStatsService.getAllEntries().filter((e) => e.status === 'approved')
  }

  getRejectedRelays(): TRelayEntry[] {
    return relayStatsService.getAllEntries().filter((e) => e.status === 'rejected')
  }

  getExcludedRelays(): TRelayEntry[] {
    return relayStatsService.getAllEntries().filter((e) => e.manualExclude)
  }

  getAutoDisabledRelays(): TRelayEntry[] {
    return relayStatsService
      .getAllEntries()
      .filter((e) => relayStatsService.isAutoDisabled(e.url))
  }
}

const managedOutboxService = new ManagedOutboxService()
export default managedOutboxService
