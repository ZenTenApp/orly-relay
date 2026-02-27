import { Event, kinds } from 'nostr-tools'
import { Pubkey, RelayUrl, Timestamp } from '../shared'

/**
 * The scope of a relay in a relay list (read, write, or both)
 */
export type RelayScope = 'read' | 'write' | 'both'

/**
 * A relay entry with its scope
 */
export type RelayEntry = {
  relay: RelayUrl
  scope: RelayScope
}

/**
 * Result of a relay list modification
 */
export type RelayListChange =
  | { type: 'added'; relay: RelayUrl; scope: RelayScope }
  | { type: 'removed'; relay: RelayUrl }
  | { type: 'scope_changed'; relay: RelayUrl; from: RelayScope; to: RelayScope }
  | { type: 'no_change' }

/**
 * RelayList Aggregate
 *
 * Represents a user's mailbox relay preferences (kind 10002 in Nostr, NIP-65).
 * Defines which relays the user reads from and writes to.
 *
 * Invariants:
 * - No duplicate relay URLs
 * - All URLs must be valid WebSocket URLs
 * - At least one read and one write relay is recommended (but not enforced)
 */
export class RelayList {
  private readonly _relays: Map<string, RelayEntry>

  private constructor(
    private readonly _owner: Pubkey,
    entries: RelayEntry[]
  ) {
    this._relays = new Map()
    for (const entry of entries) {
      this._relays.set(entry.relay.value, entry)
    }
  }

  /**
   * Create an empty RelayList for a user
   */
  static empty(owner: Pubkey): RelayList {
    return new RelayList(owner, [])
  }

  /**
   * Create a RelayList with initial relays (all set to 'both')
   */
  static fromUrls(owner: Pubkey, urls: string[]): RelayList {
    const entries: RelayEntry[] = []
    for (const url of urls) {
      const relay = RelayUrl.tryCreate(url)
      if (relay) {
        entries.push({ relay, scope: 'both' })
      }
    }
    return new RelayList(owner, entries)
  }

  /**
   * Reconstruct a RelayList from a Nostr kind 10002 event
   *
   * @param event The relay list event
   * @param filterOutOnion Whether to filter out .onion addresses
   */
  static fromEvent(event: Event, filterOutOnion = false): RelayList {
    if (event.kind !== kinds.RelayList) {
      throw new Error(`Expected kind ${kinds.RelayList}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const entries: RelayEntry[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'r' && tag[1]) {
        const relay = RelayUrl.tryCreate(tag[1])
        if (!relay) continue
        if (filterOutOnion && relay.isOnion) continue

        let scope: RelayScope = 'both'
        if (tag[2] === 'read') {
          scope = 'read'
        } else if (tag[2] === 'write') {
          scope = 'write'
        }

        entries.push({ relay, scope })
      }
    }

    return new RelayList(owner, entries)
  }

  /**
   * The owner of this relay list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Total number of relays
   */
  get count(): number {
    return this._relays.size
  }

  /**
   * Get all relay entries
   */
  getEntries(): RelayEntry[] {
    return Array.from(this._relays.values())
  }

  /**
   * Get all relays (regardless of scope)
   */
  getAllRelays(): RelayUrl[] {
    return Array.from(this._relays.values()).map((e) => e.relay)
  }

  /**
   * Get all relay URLs as strings
   */
  getAllUrls(): string[] {
    return Array.from(this._relays.keys())
  }

  /**
   * Get read relays (scope is 'read' or 'both')
   */
  getReadRelays(): RelayUrl[] {
    return Array.from(this._relays.values())
      .filter((e) => e.scope === 'read' || e.scope === 'both')
      .map((e) => e.relay)
  }

  /**
   * Get read relay URLs as strings
   */
  getReadUrls(): string[] {
    return this.getReadRelays().map((r) => r.value)
  }

  /**
   * Get write relays (scope is 'write' or 'both')
   */
  getWriteRelays(): RelayUrl[] {
    return Array.from(this._relays.values())
      .filter((e) => e.scope === 'write' || e.scope === 'both')
      .map((e) => e.relay)
  }

  /**
   * Get write relay URLs as strings
   */
  getWriteUrls(): string[] {
    return this.getWriteRelays().map((r) => r.value)
  }

  /**
   * Check if a relay is in this list
   */
  hasRelay(relay: RelayUrl): boolean {
    return this._relays.has(relay.value)
  }

  /**
   * Get the scope for a relay
   */
  getScope(relay: RelayUrl): RelayScope | null {
    const entry = this._relays.get(relay.value)
    return entry ? entry.scope : null
  }

  /**
   * Add or update a relay with a specific scope
   *
   * @returns RelayListChange indicating what changed
   */
  setRelay(relay: RelayUrl, scope: RelayScope): RelayListChange {
    const existing = this._relays.get(relay.value)

    if (existing) {
      if (existing.scope === scope) {
        return { type: 'no_change' }
      }
      const oldScope = existing.scope
      this._relays.set(relay.value, { relay, scope })
      return { type: 'scope_changed', relay, from: oldScope, to: scope }
    }

    this._relays.set(relay.value, { relay, scope })
    return { type: 'added', relay, scope }
  }

  /**
   * Add a relay by URL string
   *
   * @returns RelayListChange or null if URL is invalid
   */
  setRelayUrl(url: string, scope: RelayScope): RelayListChange | null {
    const relay = RelayUrl.tryCreate(url)
    if (!relay) return null
    return this.setRelay(relay, scope)
  }

  /**
   * Remove a relay from this list
   *
   * @returns RelayListChange indicating what changed
   */
  removeRelay(relay: RelayUrl): RelayListChange {
    if (!this._relays.has(relay.value)) {
      return { type: 'no_change' }
    }

    this._relays.delete(relay.value)
    return { type: 'removed', relay }
  }

  /**
   * Remove a relay by URL string
   *
   * @returns RelayListChange or null if URL is invalid
   */
  removeRelayUrl(url: string): RelayListChange | null {
    const relay = RelayUrl.tryCreate(url)
    if (!relay) return null
    return this.removeRelay(relay)
  }

  /**
   * Replace all relays with a new list
   */
  setEntries(entries: RelayEntry[]): void {
    this._relays.clear()
    for (const entry of entries) {
      this._relays.set(entry.relay.value, entry)
    }
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(): string[][] {
    return Array.from(this._relays.values()).map((entry) => {
      if (entry.scope === 'both') {
        return ['r', entry.relay.value]
      }
      return ['r', entry.relay.value, entry.scope]
    })
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(): { kind: number; content: string; created_at: number; tags: string[][] } {
    return {
      kind: kinds.RelayList,
      content: '',
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }

  /**
   * Convert to the legacy TRelayList format
   */
  toLegacyFormat(): {
    read: string[]
    write: string[]
    originalRelays: Array<{ url: string; scope: RelayScope }>
  } {
    return {
      read: this.getReadUrls(),
      write: this.getWriteUrls(),
      originalRelays: Array.from(this._relays.values()).map((e) => ({
        url: e.relay.value,
        scope: e.scope
      }))
    }
  }
}
