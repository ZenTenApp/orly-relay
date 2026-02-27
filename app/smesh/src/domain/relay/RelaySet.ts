import { Event, kinds } from 'nostr-tools'
import { RelayUrl, Timestamp } from '../shared'

/**
 * Result of a relay set modification
 */
export type RelaySetChange =
  | { type: 'added'; relay: RelayUrl }
  | { type: 'removed'; relay: RelayUrl }
  | { type: 'no_change' }

/**
 * RelaySet Aggregate
 *
 * Represents a named collection of relays (kind 30002 in Nostr).
 * Used for organizing relays into groups like "fast relays", "paid relays", etc.
 *
 * Invariants:
 * - Name is required and non-empty
 * - No duplicate relay URLs
 * - All URLs must be valid WebSocket URLs
 */
export class RelaySet {
  private readonly _relays: Map<string, RelayUrl>

  private constructor(
    private readonly _id: string,
    private _name: string,
    relays: RelayUrl[]
  ) {
    this._relays = new Map()
    for (const relay of relays) {
      this._relays.set(relay.value, relay)
    }
  }

  /**
   * Create a new empty RelaySet with a generated ID
   */
  static create(name: string, id?: string): RelaySet {
    const setId = id || crypto.randomUUID().replace(/-/g, '').slice(0, 12)
    return new RelaySet(setId, name.trim() || 'Unnamed Set', [])
  }

  /**
   * Create a RelaySet with initial relays
   */
  static createWithRelays(name: string, relayUrls: string[], id?: string): RelaySet {
    const set = RelaySet.create(name, id)
    for (const url of relayUrls) {
      const relay = RelayUrl.tryCreate(url)
      if (relay) {
        set._relays.set(relay.value, relay)
      }
    }
    return set
  }

  /**
   * Reconstruct a RelaySet from a Nostr kind 30002 event
   */
  static fromEvent(event: Event): RelaySet {
    if (event.kind !== kinds.Relaysets) {
      throw new Error(`Expected kind ${kinds.Relaysets}, got ${event.kind}`)
    }

    let id = ''
    let name = ''
    const relays: RelayUrl[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'd' && tag[1]) {
        id = tag[1]
      } else if (tag[0] === 'title' && tag[1]) {
        name = tag[1]
      } else if (tag[0] === 'relay' && tag[1]) {
        const relay = RelayUrl.tryCreate(tag[1])
        if (relay) {
          relays.push(relay)
        }
      }
    }

    return new RelaySet(id || 'unknown', name || 'Unnamed Set', relays)
  }

  /**
   * The unique identifier for this relay set
   */
  get id(): string {
    return this._id
  }

  /**
   * The display name of this relay set
   */
  get name(): string {
    return this._name
  }

  /**
   * Number of relays in this set
   */
  get count(): number {
    return this._relays.size
  }

  /**
   * Check if the set is empty
   */
  get isEmpty(): boolean {
    return this._relays.size === 0
  }

  /**
   * Get all relays in this set
   */
  getRelays(): RelayUrl[] {
    return Array.from(this._relays.values())
  }

  /**
   * Get all relay URLs as strings
   */
  getRelayUrls(): string[] {
    return Array.from(this._relays.keys())
  }

  /**
   * Check if a relay is in this set
   */
  hasRelay(relay: RelayUrl): boolean {
    return this._relays.has(relay.value)
  }

  /**
   * Check if a relay URL string is in this set
   */
  hasRelayUrl(url: string): boolean {
    const relay = RelayUrl.tryCreate(url)
    return relay ? this._relays.has(relay.value) : false
  }

  /**
   * Rename this relay set
   */
  rename(newName: string): void {
    this._name = newName.trim() || 'Unnamed Set'
  }

  /**
   * Add a relay to this set
   *
   * @returns RelaySetChange indicating what changed
   */
  addRelay(relay: RelayUrl): RelaySetChange {
    if (this._relays.has(relay.value)) {
      return { type: 'no_change' }
    }

    this._relays.set(relay.value, relay)
    return { type: 'added', relay }
  }

  /**
   * Add a relay by URL string
   *
   * @returns RelaySetChange or null if URL is invalid
   */
  addRelayUrl(url: string): RelaySetChange | null {
    const relay = RelayUrl.tryCreate(url)
    if (!relay) return null
    return this.addRelay(relay)
  }

  /**
   * Remove a relay from this set
   *
   * @returns RelaySetChange indicating what changed
   */
  removeRelay(relay: RelayUrl): RelaySetChange {
    if (!this._relays.has(relay.value)) {
      return { type: 'no_change' }
    }

    this._relays.delete(relay.value)
    return { type: 'removed', relay }
  }

  /**
   * Remove a relay by URL string
   *
   * @returns RelaySetChange or null if URL is invalid
   */
  removeRelayUrl(url: string): RelaySetChange | null {
    const relay = RelayUrl.tryCreate(url)
    if (!relay) return null
    return this.removeRelay(relay)
  }

  /**
   * Replace all relays with a new list
   */
  setRelays(relays: RelayUrl[]): void {
    this._relays.clear()
    for (const relay of relays) {
      this._relays.set(relay.value, relay)
    }
  }

  /**
   * Convert to the 'a' tag reference format
   */
  toATag(pubkey: string): string[] {
    return ['a', `${kinds.Relaysets}:${pubkey}:${this._id}`]
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(): string[][] {
    const tags: string[][] = [
      ['d', this._id],
      ['title', this._name]
    ]

    for (const relay of this._relays.values()) {
      tags.push(['relay', relay.value])
    }

    return tags
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(): { kind: number; content: string; created_at: number; tags: string[][] } {
    return {
      kind: kinds.Relaysets,
      content: '',
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }
}
