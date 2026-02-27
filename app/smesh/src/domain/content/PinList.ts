import { Event, kinds } from 'nostr-tools'
import { EventId, Pubkey, Timestamp } from '../shared'

/**
 * Maximum number of pinned notes allowed
 */
export const MAX_PINNED_NOTES = 5

/**
 * A pinned note entry
 */
export type PinEntry = {
  eventId: EventId
  pubkey?: Pubkey
  relayHint?: string
}

/**
 * Result of a pin operation
 */
export type PinListChange =
  | { type: 'pinned'; entry: PinEntry }
  | { type: 'unpinned'; eventId: string }
  | { type: 'no_change' }
  | { type: 'limit_exceeded'; removed: PinEntry[] }

/**
 * Error thrown when trying to pin non-own content
 */
export class CannotPinOthersContentError extends Error {
  constructor() {
    super('Cannot pin content from other users')
    this.name = 'CannotPinOthersContentError'
  }
}

/**
 * Error thrown when trying to pin non-note content
 */
export class CanOnlyPinNotesError extends Error {
  constructor() {
    super('Can only pin short text notes')
    this.name = 'CanOnlyPinNotesError'
  }
}

/**
 * PinList Aggregate
 *
 * Represents a user's pinned notes list (kind 10001 in Nostr).
 * Users can pin their own short text notes to highlight them on their profile.
 *
 * Invariants:
 * - Can only pin own notes (same pubkey)
 * - Can only pin short text notes (kind 1)
 * - Maximum of MAX_PINNED_NOTES entries (oldest removed when exceeded)
 * - No duplicate entries
 */
export class PinList {
  private readonly _entries: Map<string, PinEntry>
  private readonly _order: string[] // Maintains insertion order
  private readonly _content: string

  private constructor(
    private readonly _owner: Pubkey,
    entries: PinEntry[],
    content: string = ''
  ) {
    this._entries = new Map()
    this._order = []
    for (const entry of entries) {
      this._entries.set(entry.eventId.hex, entry)
      this._order.push(entry.eventId.hex)
    }
    this._content = content
  }

  /**
   * Create an empty PinList for a user
   */
  static empty(owner: Pubkey): PinList {
    return new PinList(owner, [])
  }

  /**
   * Reconstruct a PinList from a Nostr kind 10001 event
   */
  static fromEvent(event: Event): PinList {
    if (event.kind !== kinds.Pinlist) {
      throw new Error(`Expected kind ${kinds.Pinlist}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const entries: PinEntry[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'e' && tag[1]) {
        const eventId = EventId.tryFromString(tag[1])
        if (eventId && !entries.some((e) => e.eventId.hex === eventId.hex)) {
          const pubkey = tag[2] ? Pubkey.tryFromString(tag[2]) : undefined
          entries.push({
            eventId,
            pubkey: pubkey || undefined,
            relayHint: tag[3] || undefined
          })
        }
      }
    }

    return new PinList(owner, entries, event.content)
  }

  /**
   * Try to create a PinList from an event, returns null if invalid
   */
  static tryFromEvent(event: Event | null | undefined): PinList | null {
    if (!event) return null
    try {
      return PinList.fromEvent(event)
    } catch {
      return null
    }
  }

  /**
   * The owner of this pin list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Number of pinned notes
   */
  get count(): number {
    return this._entries.size
  }

  /**
   * Whether the pin list is at maximum capacity
   */
  get isFull(): boolean {
    return this._entries.size >= MAX_PINNED_NOTES
  }

  /**
   * The raw content field
   */
  get content(): string {
    return this._content
  }

  /**
   * Get all pinned entries in order
   */
  getEntries(): PinEntry[] {
    return this._order.map((id) => this._entries.get(id)!).filter(Boolean)
  }

  /**
   * Get all pinned event IDs
   */
  getEventIds(): string[] {
    return [...this._order]
  }

  /**
   * Get pinned event IDs as a Set for fast lookup
   */
  getEventIdSet(): Set<string> {
    return new Set(this._order)
  }

  /**
   * Check if a note is pinned
   */
  isPinned(eventId: string): boolean {
    return this._entries.has(eventId)
  }

  /**
   * Pin a note
   *
   * @throws CannotPinOthersContentError if note is from another user
   * @throws CanOnlyPinNotesError if event is not a short text note
   * @returns PinListChange indicating what changed
   */
  pin(event: Event): PinListChange {
    // Validate: only own notes
    if (event.pubkey !== this._owner.hex) {
      throw new CannotPinOthersContentError()
    }

    // Validate: only short text notes
    if (event.kind !== kinds.ShortTextNote) {
      throw new CanOnlyPinNotesError()
    }

    const eventId = EventId.fromHex(event.id)

    // Check for duplicate
    if (this._entries.has(eventId.hex)) {
      return { type: 'no_change' }
    }

    const entry: PinEntry = {
      eventId,
      pubkey: this._owner,
      relayHint: undefined
    }

    // Check capacity and remove oldest if needed
    const removed: PinEntry[] = []
    while (this._entries.size >= MAX_PINNED_NOTES) {
      const oldestId = this._order.shift()
      if (oldestId) {
        const oldEntry = this._entries.get(oldestId)
        if (oldEntry) {
          removed.push(oldEntry)
        }
        this._entries.delete(oldestId)
      }
    }

    // Add new pin
    this._entries.set(eventId.hex, entry)
    this._order.push(eventId.hex)

    if (removed.length > 0) {
      return { type: 'limit_exceeded', removed }
    }

    return { type: 'pinned', entry }
  }

  /**
   * Unpin a note
   *
   * @returns PinListChange indicating what changed
   */
  unpin(eventId: string): PinListChange {
    if (!this._entries.has(eventId)) {
      return { type: 'no_change' }
    }

    this._entries.delete(eventId)
    const index = this._order.indexOf(eventId)
    if (index !== -1) {
      this._order.splice(index, 1)
    }

    return { type: 'unpinned', eventId }
  }

  /**
   * Unpin by event
   */
  unpinEvent(event: Event): PinListChange {
    return this.unpin(event.id)
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(): string[][] {
    const tags: string[][] = []

    for (const id of this._order) {
      const entry = this._entries.get(id)
      if (entry) {
        const tag = ['e', entry.eventId.hex]
        if (entry.pubkey) {
          tag.push(entry.pubkey.hex)
          if (entry.relayHint) {
            tag.push(entry.relayHint)
          }
        } else if (entry.relayHint) {
          tag.push('', entry.relayHint)
        }
        tags.push(tag)
      }
    }

    return tags
  }

  /**
   * Convert to a draft event for publishing
   */
  toDraftEvent(): { kind: number; content: string; created_at: number; tags: string[][] } {
    return {
      kind: kinds.Pinlist,
      content: this._content,
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }
}
