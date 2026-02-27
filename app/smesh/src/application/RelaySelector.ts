import { RelayUrl } from '@/domain/shared'
import { RelayList } from '@/domain/relay'

/**
 * Options for relay selection
 */
export type RelaySelectorOptions = {
  maxRelays?: number
  preferSecure?: boolean
  excludeOnion?: boolean
  includeDefaultRelays?: boolean
}

/**
 * RelaySelector Domain Service
 *
 * Handles intelligent selection of relays for various operations.
 * Implements relay selection strategies based on context.
 */
export class RelaySelector {
  constructor(
    private readonly defaultRelays: RelayUrl[] = []
  ) {}

  /**
   * Select relays for publishing an event
   * Prioritizes write relays from the user's relay list
   */
  selectForPublishing(
    relayList: RelayList | null,
    options: RelaySelectorOptions = {}
  ): RelayUrl[] {
    const { maxRelays = 5, preferSecure = true, excludeOnion = false } = options

    const candidates: RelayUrl[] = []

    // Add user's write relays
    if (relayList) {
      const writeRelays = relayList.getWriteRelays()
      candidates.push(...writeRelays)
    }

    // Add default relays if needed
    if (options.includeDefaultRelays || candidates.length === 0) {
      for (const relay of this.defaultRelays) {
        if (!candidates.some((c) => c.equals(relay))) {
          candidates.push(relay)
        }
      }
    }

    // Filter and sort
    let filtered = candidates
    if (excludeOnion) {
      filtered = filtered.filter((r) => !r.isOnion)
    }

    if (preferSecure) {
      filtered.sort((a, b) => {
        if (a.isSecure && !b.isSecure) return -1
        if (!a.isSecure && b.isSecure) return 1
        return 0
      })
    }

    return filtered.slice(0, maxRelays)
  }

  /**
   * Select relays for fetching events
   * Prioritizes read relays from the user's relay list
   */
  selectForFetching(
    relayList: RelayList | null,
    options: RelaySelectorOptions = {}
  ): RelayUrl[] {
    const { maxRelays = 5, preferSecure = true, excludeOnion = false } = options

    const candidates: RelayUrl[] = []

    // Add user's read relays
    if (relayList) {
      const readRelays = relayList.getReadRelays()
      candidates.push(...readRelays)
    }

    // Add default relays if needed
    if (options.includeDefaultRelays || candidates.length === 0) {
      for (const relay of this.defaultRelays) {
        if (!candidates.some((c) => c.equals(relay))) {
          candidates.push(relay)
        }
      }
    }

    // Filter and sort
    let filtered = candidates
    if (excludeOnion) {
      filtered = filtered.filter((r) => !r.isOnion)
    }

    if (preferSecure) {
      filtered.sort((a, b) => {
        if (a.isSecure && !b.isSecure) return -1
        if (!a.isSecure && b.isSecure) return 1
        return 0
      })
    }

    return filtered.slice(0, maxRelays)
  }

  /**
   * Select relays for publishing to specific users' inboxes
   * Includes mentioned users' read relays
   */
  selectForMentions(
    authorRelayList: RelayList | null,
    mentionedRelayLists: RelayList[],
    options: RelaySelectorOptions = {}
  ): RelayUrl[] {
    const { maxRelays = 8 } = options

    const relaySet = new Map<string, RelayUrl>()

    // Add author's write relays first
    if (authorRelayList) {
      for (const relay of authorRelayList.getWriteRelays()) {
        relaySet.set(relay.value, relay)
      }
    }

    // Add mentioned users' read relays
    for (const relayList of mentionedRelayLists) {
      for (const relay of relayList.getReadRelays()) {
        relaySet.set(relay.value, relay)
      }
    }

    // Add defaults if needed
    if (options.includeDefaultRelays || relaySet.size === 0) {
      for (const relay of this.defaultRelays) {
        relaySet.set(relay.value, relay)
      }
    }

    const candidates = Array.from(relaySet.values())

    // Filter onion if needed
    let filtered = candidates
    if (options.excludeOnion) {
      filtered = filtered.filter((r) => !r.isOnion)
    }

    return filtered.slice(0, maxRelays)
  }

  /**
   * Get relay URLs as strings (for compatibility with existing code)
   */
  selectForPublishingUrls(
    relayList: RelayList | null,
    options: RelaySelectorOptions = {}
  ): string[] {
    return this.selectForPublishing(relayList, options).map((r) => r.value)
  }

  /**
   * Get relay URLs as strings for fetching
   */
  selectForFetchingUrls(
    relayList: RelayList | null,
    options: RelaySelectorOptions = {}
  ): string[] {
    return this.selectForFetching(relayList, options).map((r) => r.value)
  }
}

/**
 * Create a RelaySelector with default relays
 */
export function createRelaySelector(defaultRelayUrls: string[]): RelaySelector {
  const defaultRelays = defaultRelayUrls
    .map((url) => RelayUrl.tryCreate(url))
    .filter((r): r is RelayUrl => r !== null)

  return new RelaySelector(defaultRelays)
}
