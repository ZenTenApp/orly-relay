import { Event, kinds } from 'nostr-tools'
import { Pubkey, RelayUrl, Timestamp } from '../shared'
import { RelaySet } from './RelaySet'

/**
 * Result of a favorite relays modification
 */
export type FavoriteRelaysChange =
  | { type: 'relay_added'; relay: RelayUrl }
  | { type: 'relay_removed'; relay: RelayUrl }
  | { type: 'set_added'; set: RelaySet }
  | { type: 'set_removed'; setId: string }
  | { type: 'no_change' }

/**
 * FavoriteRelays Aggregate
 *
 * Represents a user's favorite relays collection (kind 10012 in Nostr).
 * Combines individual relay URLs and references to relay sets.
 *
 * This is the user's curated list of relays they want quick access to,
 * separate from their mailbox relays (kind 10002).
 */
export class FavoriteRelays {
  private readonly _relays: Map<string, RelayUrl>
  private readonly _sets: Map<string, RelaySet>
  private readonly _setOrder: string[]

  private constructor(
    private readonly _owner: Pubkey,
    relays: RelayUrl[],
    sets: RelaySet[]
  ) {
    this._relays = new Map()
    this._sets = new Map()
    this._setOrder = []

    for (const relay of relays) {
      this._relays.set(relay.value, relay)
    }
    for (const set of sets) {
      this._sets.set(set.id, set)
      this._setOrder.push(set.id)
    }
  }

  /**
   * Create an empty FavoriteRelays for a user
   */
  static empty(owner: Pubkey): FavoriteRelays {
    return new FavoriteRelays(owner, [], [])
  }

  /**
   * Create FavoriteRelays from URLs only
   */
  static fromUrls(owner: Pubkey, urls: string[]): FavoriteRelays {
    const relays: RelayUrl[] = []
    for (const url of urls) {
      const relay = RelayUrl.tryCreate(url)
      if (relay && !relays.some((r) => r.value === relay.value)) {
        relays.push(relay)
      }
    }
    return new FavoriteRelays(owner, relays, [])
  }

  /**
   * Reconstruct FavoriteRelays from a Nostr kind 10012 event
   *
   * @param event The favorite relays event
   * @param relaySets The relay set events referenced by 'a' tags
   */
  static fromEvent(event: Event, relaySets: RelaySet[] = []): FavoriteRelays {
    const owner = Pubkey.fromHex(event.pubkey)
    const relays: RelayUrl[] = []
    const setIds: string[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'relay' && tag[1]) {
        const relay = RelayUrl.tryCreate(tag[1])
        if (relay && !relays.some((r) => r.value === relay.value)) {
          relays.push(relay)
        }
      } else if (tag[0] === 'a' && tag[1]) {
        const [kind, , setId] = tag[1].split(':')
        if (kind === kinds.Relaysets.toString() && setId && !setIds.includes(setId)) {
          setIds.push(setId)
        }
      }
    }

    // Match relay sets to their IDs in order
    const orderedSets: RelaySet[] = []
    for (const id of setIds) {
      const set = relaySets.find((s) => s.id === id)
      if (set) {
        orderedSets.push(set)
      }
    }

    return new FavoriteRelays(owner, relays, orderedSets)
  }

  /**
   * The owner of this favorite relays list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Number of individual favorite relays
   */
  get relayCount(): number {
    return this._relays.size
  }

  /**
   * Number of relay sets
   */
  get setCount(): number {
    return this._sets.size
  }

  /**
   * Get all individual favorite relays
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
   * Get all relay sets in order
   */
  getSets(): RelaySet[] {
    return this._setOrder.map((id) => this._sets.get(id)!).filter(Boolean)
  }

  /**
   * Get a relay set by ID
   */
  getSet(id: string): RelaySet | undefined {
    return this._sets.get(id)
  }

  /**
   * Get all unique relays (from both individual relays and sets)
   */
  getAllUniqueRelays(): RelayUrl[] {
    const all = new Map<string, RelayUrl>()

    for (const relay of this._relays.values()) {
      all.set(relay.value, relay)
    }

    for (const set of this._sets.values()) {
      for (const relay of set.getRelays()) {
        all.set(relay.value, relay)
      }
    }

    return Array.from(all.values())
  }

  /**
   * Check if a relay is in the favorites
   */
  hasRelay(relay: RelayUrl): boolean {
    return this._relays.has(relay.value)
  }

  /**
   * Check if a relay set is in the favorites
   */
  hasSet(id: string): boolean {
    return this._sets.has(id)
  }

  /**
   * Add a relay to favorites
   *
   * @returns FavoriteRelaysChange indicating what changed
   */
  addRelay(relay: RelayUrl): FavoriteRelaysChange {
    if (this._relays.has(relay.value)) {
      return { type: 'no_change' }
    }

    this._relays.set(relay.value, relay)
    return { type: 'relay_added', relay }
  }

  /**
   * Add multiple relays to favorites
   */
  addRelays(relays: RelayUrl[]): FavoriteRelaysChange[] {
    return relays.map((r) => this.addRelay(r))
  }

  /**
   * Add a relay by URL string
   */
  addRelayUrl(url: string): FavoriteRelaysChange | null {
    const relay = RelayUrl.tryCreate(url)
    if (!relay) return null
    return this.addRelay(relay)
  }

  /**
   * Remove a relay from favorites
   *
   * @returns FavoriteRelaysChange indicating what changed
   */
  removeRelay(relay: RelayUrl): FavoriteRelaysChange {
    if (!this._relays.has(relay.value)) {
      return { type: 'no_change' }
    }

    this._relays.delete(relay.value)
    return { type: 'relay_removed', relay }
  }

  /**
   * Remove multiple relays from favorites
   */
  removeRelays(relays: RelayUrl[]): FavoriteRelaysChange[] {
    return relays.map((r) => this.removeRelay(r))
  }

  /**
   * Add a relay set to favorites
   *
   * @returns FavoriteRelaysChange indicating what changed
   */
  addSet(set: RelaySet): FavoriteRelaysChange {
    if (this._sets.has(set.id)) {
      return { type: 'no_change' }
    }

    this._sets.set(set.id, set)
    this._setOrder.push(set.id)
    return { type: 'set_added', set }
  }

  /**
   * Remove a relay set from favorites
   *
   * @returns FavoriteRelaysChange indicating what changed
   */
  removeSet(id: string): FavoriteRelaysChange {
    if (!this._sets.has(id)) {
      return { type: 'no_change' }
    }

    this._sets.delete(id)
    const index = this._setOrder.indexOf(id)
    if (index !== -1) {
      this._setOrder.splice(index, 1)
    }
    return { type: 'set_removed', setId: id }
  }

  /**
   * Update a relay set
   */
  updateSet(set: RelaySet): boolean {
    if (!this._sets.has(set.id)) {
      return false
    }
    this._sets.set(set.id, set)
    return true
  }

  /**
   * Reorder the favorite relays
   */
  reorderRelays(newOrder: RelayUrl[]): void {
    this._relays.clear()
    for (const relay of newOrder) {
      this._relays.set(relay.value, relay)
    }
  }

  /**
   * Reorder the relay sets
   */
  reorderSets(newOrder: RelaySet[]): void {
    this._setOrder.length = 0
    for (const set of newOrder) {
      if (this._sets.has(set.id)) {
        this._setOrder.push(set.id)
      }
    }
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(pubkey: string): string[][] {
    const tags: string[][] = []

    for (const relay of this._relays.values()) {
      tags.push(['relay', relay.value])
    }

    for (const id of this._setOrder) {
      const set = this._sets.get(id)
      if (set) {
        tags.push(['a', `${kinds.Relaysets}:${pubkey}:${id}`])
      }
    }

    return tags
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(pubkey: string): {
    kind: number
    content: string
    created_at: number
    tags: string[][]
  } {
    return {
      kind: 10012, // ExtendedKind.FAVORITE_RELAYS
      content: '',
      created_at: Timestamp.now().unix,
      tags: this.toTags(pubkey)
    }
  }
}
