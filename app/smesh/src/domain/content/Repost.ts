import { Event, kinds } from 'nostr-tools'
import { EventId, Pubkey, Timestamp } from '../shared'

/**
 * Repost Entity
 *
 * Represents a repost (kind 6 or 16) of another Nostr event.
 */
export class Repost {
  private readonly _embeddedEvent: Event | null

  private constructor(
    private readonly _event: Event,
    private readonly _targetEventId: EventId,
    private readonly _targetAuthor: Pubkey,
    embeddedEvent: Event | null
  ) {
    this._embeddedEvent = embeddedEvent
  }

  /**
   * Create a Repost from a Nostr Event
   */
  static fromEvent(event: Event): Repost {
    if (event.kind !== kinds.Repost && event.kind !== kinds.GenericRepost) {
      throw new Error(`Expected kind ${kinds.Repost} or ${kinds.GenericRepost}, got ${event.kind}`)
    }

    // Find the target event (first 'e' tag)
    const eTag = event.tags.find((t) => t[0] === 'e')
    if (!eTag?.[1]) {
      throw new Error('Repost must have an e tag')
    }

    // Find the target author (first 'p' tag)
    const pTag = event.tags.find((t) => t[0] === 'p')
    if (!pTag?.[1]) {
      throw new Error('Repost must have a p tag')
    }

    const targetEventId = EventId.fromHex(eTag[1])
    const targetAuthor = Pubkey.fromHex(pTag[1])

    // Try to parse embedded event from content
    let embeddedEvent: Event | null = null
    if (event.content) {
      try {
        embeddedEvent = JSON.parse(event.content) as Event
      } catch {
        // Content is not valid JSON, that's fine
      }
    }

    return new Repost(event, targetEventId, targetAuthor, embeddedEvent)
  }

  /**
   * Try to create a Repost from an Event, returns null if invalid
   */
  static tryFromEvent(event: Event | null | undefined): Repost | null {
    if (!event) return null
    try {
      return Repost.fromEvent(event)
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
   * The repost's event ID
   */
  get id(): EventId {
    return EventId.fromHex(this._event.id)
  }

  /**
   * The author who reposted
   */
  get author(): Pubkey {
    return Pubkey.fromHex(this._event.pubkey)
  }

  /**
   * The event ID being reposted
   */
  get targetEventId(): EventId {
    return this._targetEventId
  }

  /**
   * The author of the original event
   */
  get targetAuthor(): Pubkey {
    return this._targetAuthor
  }

  /**
   * When the repost was created
   */
  get createdAt(): Timestamp {
    return Timestamp.fromUnix(this._event.created_at)
  }

  /**
   * Whether this is a standard repost (kind 6)
   */
  get isStandardRepost(): boolean {
    return this._event.kind === kinds.Repost
  }

  /**
   * Whether this is a generic repost (kind 16)
   */
  get isGenericRepost(): boolean {
    return this._event.kind === kinds.GenericRepost
  }

  /**
   * The embedded/quoted event (if included in content)
   */
  get embeddedEvent(): Event | null {
    return this._embeddedEvent
  }

  /**
   * Whether the repost includes the embedded event
   */
  get hasEmbeddedEvent(): boolean {
    return this._embeddedEvent !== null
  }

  /**
   * Get the kind of the reposted event (from k tag)
   */
  get targetKind(): number | undefined {
    const kTag = this._event.tags.find((t) => t[0] === 'k')
    return kTag?.[1] ? parseInt(kTag[1], 10) : undefined
  }
}
