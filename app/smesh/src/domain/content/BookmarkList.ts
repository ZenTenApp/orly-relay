import { Event, kinds } from 'nostr-tools'
import { EventId, Pubkey, Timestamp } from '../shared'

/**
 * Type of bookmarked item
 */
export type BookmarkType = 'event' | 'replaceable'

/**
 * A bookmarked item
 */
export type BookmarkEntry = {
  type: BookmarkType
  id: string // event id or 'a' tag coordinate
  pubkey?: Pubkey
  relayHint?: string
}

/**
 * Result of a bookmark operation
 */
export type BookmarkListChange =
  | { type: 'added'; entry: BookmarkEntry }
  | { type: 'removed'; id: string }
  | { type: 'no_change' }

/**
 * BookmarkList Aggregate
 *
 * Represents a user's bookmark list (kind 10003 in Nostr).
 * Supports both regular events (e tags) and replaceable events (a tags).
 *
 * Invariants:
 * - No duplicate entries
 * - Event IDs and coordinates must be valid
 */
export class BookmarkList {
  private readonly _entries: Map<string, BookmarkEntry>
  private readonly _content: string

  private constructor(
    private readonly _owner: Pubkey,
    entries: BookmarkEntry[],
    content: string = ''
  ) {
    this._entries = new Map()
    for (const entry of entries) {
      this._entries.set(entry.id, entry)
    }
    this._content = content
  }

  /**
   * Create an empty BookmarkList for a user
   */
  static empty(owner: Pubkey): BookmarkList {
    return new BookmarkList(owner, [])
  }

  /**
   * Reconstruct a BookmarkList from a Nostr kind 10003 event
   */
  static fromEvent(event: Event): BookmarkList {
    if (event.kind !== kinds.BookmarkList) {
      throw new Error(`Expected kind ${kinds.BookmarkList}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const entries: BookmarkEntry[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'e' && tag[1]) {
        const eventId = EventId.tryFromString(tag[1])
        if (eventId) {
          const pubkey = tag[2] ? Pubkey.tryFromString(tag[2]) : undefined
          entries.push({
            type: 'event',
            id: eventId.hex,
            pubkey: pubkey || undefined,
            relayHint: tag[3] || undefined
          })
        }
      } else if (tag[0] === 'a' && tag[1]) {
        entries.push({
          type: 'replaceable',
          id: tag[1],
          relayHint: tag[2] || undefined
        })
      }
    }

    return new BookmarkList(owner, entries, event.content)
  }

  /**
   * Try to create a BookmarkList from an event, returns null if invalid
   */
  static tryFromEvent(event: Event | null | undefined): BookmarkList | null {
    if (!event) return null
    try {
      return BookmarkList.fromEvent(event)
    } catch {
      return null
    }
  }

  /**
   * The owner of this bookmark list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Number of bookmarked items
   */
  get count(): number {
    return this._entries.size
  }

  /**
   * The raw content field
   */
  get content(): string {
    return this._content
  }

  /**
   * Get all bookmark entries
   */
  getEntries(): BookmarkEntry[] {
    return Array.from(this._entries.values())
  }

  /**
   * Get all bookmarked event IDs (e tags only)
   */
  getEventIds(): string[] {
    return Array.from(this._entries.values())
      .filter((e) => e.type === 'event')
      .map((e) => e.id)
  }

  /**
   * Get all bookmarked replaceable coordinates (a tags only)
   */
  getReplaceableCoordinates(): string[] {
    return Array.from(this._entries.values())
      .filter((e) => e.type === 'replaceable')
      .map((e) => e.id)
  }

  /**
   * Check if an item is bookmarked by event ID
   */
  hasEventId(eventId: string): boolean {
    return this._entries.has(eventId)
  }

  /**
   * Check if a replaceable event is bookmarked by coordinate
   */
  hasCoordinate(coordinate: string): boolean {
    return this._entries.has(coordinate)
  }

  /**
   * Check if any form of the item is bookmarked
   */
  isBookmarked(idOrCoordinate: string): boolean {
    return this._entries.has(idOrCoordinate)
  }

  /**
   * Add an event bookmark
   *
   * @returns BookmarkListChange indicating what changed
   */
  addEvent(eventId: EventId, pubkey?: Pubkey, relayHint?: string): BookmarkListChange {
    const id = eventId.hex

    if (this._entries.has(id)) {
      return { type: 'no_change' }
    }

    const entry: BookmarkEntry = {
      type: 'event',
      id,
      pubkey,
      relayHint
    }
    this._entries.set(id, entry)
    return { type: 'added', entry }
  }

  /**
   * Add a replaceable event bookmark by coordinate
   *
   * @param coordinate The 'a' tag coordinate (kind:pubkey:d-tag)
   * @returns BookmarkListChange indicating what changed
   */
  addReplaceable(coordinate: string, relayHint?: string): BookmarkListChange {
    if (this._entries.has(coordinate)) {
      return { type: 'no_change' }
    }

    const entry: BookmarkEntry = {
      type: 'replaceable',
      id: coordinate,
      relayHint
    }
    this._entries.set(coordinate, entry)
    return { type: 'added', entry }
  }

  /**
   * Add a bookmark from a Nostr event
   *
   * @returns BookmarkListChange indicating what changed
   */
  addFromEvent(event: Event): BookmarkListChange {
    // Check if replaceable event
    if (this.isReplaceableKind(event.kind)) {
      const dTag = event.tags.find((t) => t[0] === 'd')?.[1] || ''
      const coordinate = `${event.kind}:${event.pubkey}:${dTag}`
      return this.addReplaceable(coordinate)
    }

    // Regular event
    const eventId = EventId.tryFromString(event.id)
    if (!eventId) return { type: 'no_change' }

    const pubkey = Pubkey.tryFromString(event.pubkey)
    return this.addEvent(eventId, pubkey || undefined)
  }

  /**
   * Remove a bookmark by ID or coordinate
   *
   * @returns BookmarkListChange indicating what changed
   */
  remove(idOrCoordinate: string): BookmarkListChange {
    if (!this._entries.has(idOrCoordinate)) {
      return { type: 'no_change' }
    }

    this._entries.delete(idOrCoordinate)
    return { type: 'removed', id: idOrCoordinate }
  }

  /**
   * Remove a bookmark by event
   */
  removeFromEvent(event: Event): BookmarkListChange {
    // Check if replaceable event
    if (this.isReplaceableKind(event.kind)) {
      const dTag = event.tags.find((t) => t[0] === 'd')?.[1] || ''
      const coordinate = `${event.kind}:${event.pubkey}:${dTag}`
      return this.remove(coordinate)
    }

    return this.remove(event.id)
  }

  /**
   * Check if a kind is replaceable
   */
  private isReplaceableKind(kind: number): boolean {
    return (kind >= 10000 && kind < 20000) || (kind >= 30000 && kind < 40000)
  }

  /**
   * Convert to Nostr event tags format
   */
  toTags(): string[][] {
    const tags: string[][] = []

    for (const entry of this._entries.values()) {
      if (entry.type === 'event') {
        const tag = ['e', entry.id]
        if (entry.pubkey) {
          tag.push(entry.pubkey.hex)
          if (entry.relayHint) {
            tag.push(entry.relayHint)
          }
        } else if (entry.relayHint) {
          tag.push('', entry.relayHint)
        }
        tags.push(tag)
      } else {
        const tag = ['a', entry.id]
        if (entry.relayHint) {
          tag.push(entry.relayHint)
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
      kind: kinds.BookmarkList,
      content: this._content,
      created_at: Timestamp.now().unix,
      tags: this.toTags()
    }
  }
}
