import { Event, kinds } from 'nostr-tools'
import { EventId, Pubkey, Timestamp } from '../shared'

/**
 * Types of notes based on their relationship to other content
 */
export type NoteType = 'root' | 'reply' | 'quote'

/**
 * Mention extracted from note tags
 */
export type NoteMention = {
  pubkey: Pubkey
  relayHint?: string
  marker?: 'reply' | 'root' | 'mention'
}

/**
 * Reference to another note
 */
export type NoteReference = {
  eventId: EventId
  relayHint?: string
  marker?: 'reply' | 'root' | 'mention'
  author?: Pubkey
}

/**
 * Note Entity
 *
 * Represents a short text note (kind 1) in Nostr.
 * Wraps the raw Event with rich domain behavior.
 *
 * This is a read-only entity - it represents a published note.
 * For creating notes, use NoteBuilder.
 */
export class Note {
  private readonly _mentions: NoteMention[]
  private readonly _references: NoteReference[]
  private readonly _hashtags: string[]

  private constructor(
    private readonly _event: Event,
    mentions: NoteMention[],
    references: NoteReference[],
    hashtags: string[]
  ) {
    this._mentions = mentions
    this._references = references
    this._hashtags = hashtags
  }

  /**
   * Create a Note from a Nostr Event
   */
  static fromEvent(event: Event): Note {
    if (event.kind !== kinds.ShortTextNote) {
      throw new Error(`Expected kind ${kinds.ShortTextNote}, got ${event.kind}`)
    }

    const mentions: NoteMention[] = []
    const references: NoteReference[] = []
    const hashtags: string[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          mentions.push({
            pubkey,
            relayHint: tag[2] || undefined,
            marker: tag[3] as NoteMention['marker']
          })
        }
      } else if (tag[0] === 'e' && tag[1]) {
        const eventId = EventId.tryFromString(tag[1])
        if (eventId) {
          const author = tag[4] ? Pubkey.tryFromString(tag[4]) : undefined
          references.push({
            eventId,
            relayHint: tag[2] || undefined,
            marker: tag[3] as NoteReference['marker'],
            author: author || undefined
          })
        }
      } else if (tag[0] === 't' && tag[1]) {
        hashtags.push(tag[1].toLowerCase())
      }
    }

    return new Note(event, mentions, references, hashtags)
  }

  /**
   * Try to create a Note from an Event, returns null if invalid
   */
  static tryFromEvent(event: Event | null | undefined): Note | null {
    if (!event) return null
    try {
      return Note.fromEvent(event)
    } catch {
      return null
    }
  }

  /**
   * The underlying Nostr event
   */
  get event(): Event {
    return this._event
  }

  /**
   * The note's event ID
   */
  get id(): EventId {
    return EventId.fromHex(this._event.id)
  }

  /**
   * The author's public key
   */
  get author(): Pubkey {
    return Pubkey.fromHex(this._event.pubkey)
  }

  /**
   * The note content
   */
  get content(): string {
    return this._event.content
  }

  /**
   * When the note was created
   */
  get createdAt(): Timestamp {
    return Timestamp.fromUnix(this._event.created_at)
  }

  /**
   * All mentioned users
   */
  get mentions(): NoteMention[] {
    return [...this._mentions]
  }

  /**
   * All referenced notes
   */
  get references(): NoteReference[] {
    return [...this._references]
  }

  /**
   * All hashtags in the note
   */
  get hashtags(): string[] {
    return [...this._hashtags]
  }

  /**
   * Get the type of note based on its references
   */
  get noteType(): NoteType {
    const rootRef = this._references.find((r) => r.marker === 'root')
    const replyRef = this._references.find((r) => r.marker === 'reply')
    const quoteRef = this._event.tags.find((t) => t[0] === 'q')

    if (rootRef || replyRef) {
      return 'reply'
    }
    if (quoteRef) {
      return 'quote'
    }
    return 'root'
  }

  /**
   * Whether this is a root note (not a reply)
   */
  get isRoot(): boolean {
    return this.noteType === 'root'
  }

  /**
   * Whether this is a reply to another note
   */
  get isReply(): boolean {
    return this.noteType === 'reply'
  }

  /**
   * Get the root note reference (if this is a reply)
   */
  get rootReference(): NoteReference | undefined {
    return this._references.find((r) => r.marker === 'root')
  }

  /**
   * Get the parent note reference (if this is a reply)
   */
  get parentReference(): NoteReference | undefined {
    return this._references.find((r) => r.marker === 'reply')
  }

  /**
   * Whether this note has a content warning
   */
  get hasContentWarning(): boolean {
    return this._event.tags.some((t) => t[0] === 'content-warning')
  }

  /**
   * Get the content warning reason (if any)
   */
  get contentWarning(): string | undefined {
    const tag = this._event.tags.find((t) => t[0] === 'content-warning')
    return tag?.[1]
  }

  /**
   * Whether this is an NSFW note
   */
  get isNsfw(): boolean {
    const cwTag = this._event.tags.find((t) => t[0] === 'content-warning')
    return cwTag?.[1]?.toLowerCase().includes('nsfw') ?? false
  }

  /**
   * Whether this note mentions a specific user
   */
  mentionsUser(pubkey: Pubkey): boolean {
    return this._mentions.some((m) => m.pubkey.equals(pubkey))
  }

  /**
   * Whether this note references a specific event
   */
  referencesNote(eventId: EventId): boolean {
    return this._references.some((r) => r.eventId.equals(eventId))
  }

  /**
   * Whether this note includes a specific hashtag
   */
  hasHashtag(hashtag: string): boolean {
    return this._hashtags.includes(hashtag.toLowerCase())
  }

  /**
   * Get mentioned pubkeys as hex strings (for legacy compatibility)
   */
  getMentionedPubkeysHex(): string[] {
    return this._mentions.map((m) => m.pubkey.hex)
  }
}
