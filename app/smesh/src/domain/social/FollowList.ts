import { Event, kinds } from 'nostr-tools'
import { Pubkey, Timestamp } from '../shared'
import { CannotFollowSelfError } from './errors'

/**
 * Represents a petname entry with relay hint
 */
export type FollowEntry = {
  pubkey: Pubkey
  relayHint?: string
  petname?: string
}

/**
 * Result of a follow/unfollow operation
 */
export type FollowListChange =
  | { type: 'added'; pubkey: Pubkey }
  | { type: 'removed'; pubkey: Pubkey }
  | { type: 'no_change' }

/**
 * FollowList Aggregate
 *
 * Represents a user's contact list (kind 3 event in Nostr).
 * Encapsulates all business rules for following/unfollowing users.
 *
 * Invariants:
 * - Cannot follow self
 * - No duplicate entries
 * - Pubkeys must be valid
 */
export class FollowList {
  private readonly _entries: Map<string, FollowEntry>
  private readonly _content: string

  private constructor(
    private readonly _owner: Pubkey,
    entries: FollowEntry[],
    content: string = ''
  ) {
    this._entries = new Map()
    for (const entry of entries) {
      this._entries.set(entry.pubkey.hex, entry)
    }
    this._content = content
  }

  /**
   * Create an empty FollowList for a user
   */
  static empty(owner: Pubkey): FollowList {
    return new FollowList(owner, [])
  }

  /**
   * Reconstruct a FollowList from a Nostr kind 3 event
   */
  static fromEvent(event: Event): FollowList {
    if (event.kind !== kinds.Contacts) {
      throw new Error(`Expected kind ${kinds.Contacts}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const entries: FollowEntry[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          entries.push({
            pubkey,
            relayHint: tag[2] || undefined,
            petname: tag[3] || undefined
          })
        }
      }
    }

    return new FollowList(owner, entries, event.content)
  }

  /**
   * The owner of this follow list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Number of users being followed
   */
  get count(): number {
    return this._entries.size
  }

  /**
   * The raw content field (may contain relay preferences in legacy format)
   */
  get content(): string {
    return this._content
  }

  /**
   * Get all followed pubkeys
   */
  getFollowing(): Pubkey[] {
    return Array.from(this._entries.values()).map((e) => e.pubkey)
  }

  /**
   * Get all follow entries with metadata
   */
  getEntries(): FollowEntry[] {
    return Array.from(this._entries.values())
  }

  /**
   * Check if a user is being followed
   */
  isFollowing(pubkey: Pubkey): boolean {
    return this._entries.has(pubkey.hex)
  }

  /**
   * Get the entry for a followed user
   */
  getEntry(pubkey: Pubkey): FollowEntry | undefined {
    return this._entries.get(pubkey.hex)
  }

  /**
   * Follow a user
   *
   * @throws CannotFollowSelfError if attempting to follow self
   * @returns FollowListChange indicating what changed
   */
  follow(pubkey: Pubkey, relayHint?: string, petname?: string): FollowListChange {
    if (pubkey.equals(this._owner)) {
      throw new CannotFollowSelfError()
    }

    if (this._entries.has(pubkey.hex)) {
      return { type: 'no_change' }
    }

    this._entries.set(pubkey.hex, { pubkey, relayHint, petname })
    return { type: 'added', pubkey }
  }

  /**
   * Unfollow a user
   *
   * @returns FollowListChange indicating what changed
   */
  unfollow(pubkey: Pubkey): FollowListChange {
    if (!this._entries.has(pubkey.hex)) {
      return { type: 'no_change' }
    }

    this._entries.delete(pubkey.hex)
    return { type: 'removed', pubkey }
  }

  /**
   * Update petname for a followed user
   *
   * @returns true if updated, false if user not found
   */
  setPetname(pubkey: Pubkey, petname: string | undefined): boolean {
    const entry = this._entries.get(pubkey.hex)
    if (!entry) {
      return false
    }

    this._entries.set(pubkey.hex, { ...entry, petname })
    return true
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(): string[][] {
    return Array.from(this._entries.values()).map((entry) => {
      const tag = ['p', entry.pubkey.hex]
      if (entry.relayHint) {
        tag.push(entry.relayHint)
        if (entry.petname) {
          tag.push(entry.petname)
        }
      } else if (entry.petname) {
        tag.push('', entry.petname)
      }
      return tag
    })
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(): { kind: number; content: string; created_at: number; tags: string[][] } {
    return {
      kind: kinds.Contacts,
      content: this._content,
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }

  /**
   * Create a new FollowList with the same entries but different owner
   * Useful for importing someone else's follow list
   */
  cloneFor(newOwner: Pubkey): FollowList {
    const entries = this.getEntries().filter((e) => !e.pubkey.equals(newOwner))
    return new FollowList(newOwner, entries, this._content)
  }

  /**
   * Merge another follow list into this one (union of both)
   */
  merge(other: FollowList): void {
    for (const entry of other.getEntries()) {
      if (!entry.pubkey.equals(this._owner) && !this._entries.has(entry.pubkey.hex)) {
        this._entries.set(entry.pubkey.hex, entry)
      }
    }
  }
}
