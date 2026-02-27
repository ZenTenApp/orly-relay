import { Event } from 'nostr-tools'
import { ExtendedKind } from '@/constants'
import { Pubkey, Timestamp } from '../shared'

/**
 * Represents a pinned user entry
 */
export type PinnedUserEntry = {
  pubkey: Pubkey
  isPrivate: boolean
}

/**
 * Result of a pin/unpin operation
 */
export type PinnedUsersListChange =
  | { type: 'pinned'; pubkey: Pubkey }
  | { type: 'unpinned'; pubkey: Pubkey }
  | { type: 'no_change' }

/**
 * PinnedUsersList Aggregate
 *
 * Represents a user's pinned users list (kind 10003 in Nostr).
 * Supports both public (in tags) and private (encrypted content) pins.
 *
 * Invariants:
 * - Cannot pin self
 * - No duplicate entries
 * - Pubkeys must be valid
 */
export class PinnedUsersList {
  private readonly _publicPins: Map<string, PinnedUserEntry>
  private readonly _privatePins: Map<string, PinnedUserEntry>
  private _encryptedContent: string

  private constructor(
    private readonly _owner: Pubkey,
    publicPins: PinnedUserEntry[],
    privatePins: PinnedUserEntry[],
    encryptedContent: string = ''
  ) {
    this._publicPins = new Map()
    this._privatePins = new Map()
    this._encryptedContent = encryptedContent

    for (const pin of publicPins) {
      this._publicPins.set(pin.pubkey.hex, pin)
    }
    for (const pin of privatePins) {
      this._privatePins.set(pin.pubkey.hex, pin)
    }
  }

  /**
   * Create an empty PinnedUsersList for a user
   */
  static empty(owner: Pubkey): PinnedUsersList {
    return new PinnedUsersList(owner, [], [])
  }

  /**
   * Reconstruct a PinnedUsersList from a Nostr event (public pins only)
   * Private pins must be added separately after decryption
   */
  static fromEvent(event: Event): PinnedUsersList {
    if (event.kind !== ExtendedKind.PINNED_USERS) {
      throw new Error(`Expected kind ${ExtendedKind.PINNED_USERS}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const publicPins: PinnedUserEntry[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          publicPins.push({ pubkey, isPrivate: false })
        }
      }
    }

    return new PinnedUsersList(owner, publicPins, [], event.content)
  }

  /**
   * The owner of this pinned users list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Total number of pinned users (public + private)
   */
  get count(): number {
    return this._publicPins.size + this._privatePins.size
  }

  /**
   * Number of public pins
   */
  get publicCount(): number {
    return this._publicPins.size
  }

  /**
   * Number of private pins
   */
  get privateCount(): number {
    return this._privatePins.size
  }

  /**
   * The encrypted content (private pins)
   */
  get encryptedContent(): string {
    return this._encryptedContent
  }

  /**
   * Set decrypted private pins
   */
  setPrivatePins(privateTags: string[][]): void {
    this._privatePins.clear()
    for (const tag of privateTags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          this._privatePins.set(pubkey.hex, { pubkey, isPrivate: true })
        }
      }
    }
  }

  /**
   * Get all pinned pubkeys
   */
  getPinnedPubkeys(): Pubkey[] {
    const all = new Map(this._publicPins)
    for (const [hex, entry] of this._privatePins) {
      all.set(hex, entry)
    }
    return Array.from(all.values()).map((e) => e.pubkey)
  }

  /**
   * Get all pinned entries
   */
  getEntries(): PinnedUserEntry[] {
    const all = new Map(this._publicPins)
    for (const [hex, entry] of this._privatePins) {
      all.set(hex, entry)
    }
    return Array.from(all.values())
  }

  /**
   * Get public entries only
   */
  getPublicEntries(): PinnedUserEntry[] {
    return Array.from(this._publicPins.values())
  }

  /**
   * Get private entries only
   */
  getPrivateEntries(): PinnedUserEntry[] {
    return Array.from(this._privatePins.values())
  }

  /**
   * Check if a user is pinned
   */
  isPinned(pubkey: Pubkey): boolean {
    return this._publicPins.has(pubkey.hex) || this._privatePins.has(pubkey.hex)
  }

  /**
   * Pin a user publicly
   *
   * @throws Error if attempting to pin self
   * @returns PinnedUsersListChange indicating what changed
   */
  pin(pubkey: Pubkey): PinnedUsersListChange {
    if (pubkey.equals(this._owner)) {
      throw new Error('Cannot pin self')
    }

    if (this.isPinned(pubkey)) {
      return { type: 'no_change' }
    }

    this._publicPins.set(pubkey.hex, { pubkey, isPrivate: false })
    return { type: 'pinned', pubkey }
  }

  /**
   * Unpin a user
   *
   * @returns PinnedUsersListChange indicating what changed
   */
  unpin(pubkey: Pubkey): PinnedUsersListChange {
    const wasPublic = this._publicPins.delete(pubkey.hex)
    const wasPrivate = this._privatePins.delete(pubkey.hex)

    if (wasPublic || wasPrivate) {
      return { type: 'unpinned', pubkey }
    }

    return { type: 'no_change' }
  }

  /**
   * Convert public pins to Nostr event tags format
   */
  toTags(): string[][] {
    return Array.from(this._publicPins.values()).map((entry) => ['p', entry.pubkey.hex])
  }

  /**
   * Convert private pins to tags for encryption
   */
  toPrivateTags(): string[][] {
    return Array.from(this._privatePins.values()).map((entry) => ['p', entry.pubkey.hex])
  }

  /**
   * Set encrypted content (after encrypting private tags)
   */
  setEncryptedContent(content: string): void {
    this._encryptedContent = content
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(): { kind: number; content: string; created_at: number; tags: string[][] } {
    return {
      kind: ExtendedKind.PINNED_USERS,
      content: this._encryptedContent,
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }
}

/**
 * Try to create a PinnedUsersList from an event
 * Returns null if the event is not a valid pinned users event
 */
export function tryToPinnedUsersList(event: Event | null | undefined): PinnedUsersList | null {
  if (!event || event.kind !== ExtendedKind.PINNED_USERS) {
    return null
  }
  try {
    return PinnedUsersList.fromEvent(event)
  } catch {
    return null
  }
}
