import { Event, kinds } from 'nostr-tools'
import { EventId, Pubkey, Timestamp } from '../shared'

/**
 * Type of reaction
 */
export type ReactionType = 'like' | 'dislike' | 'emoji' | 'custom_emoji'

/**
 * Custom emoji data
 */
export type CustomEmoji = {
  shortcode: string
  url: string
}

/**
 * Reaction Entity
 *
 * Represents a reaction (kind 7) to a Nostr event.
 * Reactions can be likes (+), dislikes (-), emojis, or custom emojis.
 */
export class Reaction {
  private constructor(
    private readonly _event: Event,
    private readonly _targetEventId: EventId,
    private readonly _targetAuthor: Pubkey,
    private readonly _type: ReactionType,
    private readonly _emoji: string,
    private readonly _customEmoji?: CustomEmoji
  ) {}

  /**
   * Create a Reaction from a Nostr Event
   */
  static fromEvent(event: Event): Reaction {
    if (event.kind !== kinds.Reaction) {
      throw new Error(`Expected kind ${kinds.Reaction}, got ${event.kind}`)
    }

    // Find the target event (last 'e' tag)
    const eTags = event.tags.filter((t) => t[0] === 'e')
    const lastETag = eTags[eTags.length - 1]
    if (!lastETag?.[1]) {
      throw new Error('Reaction must have an e tag')
    }

    // Find the target author (last 'p' tag)
    const pTags = event.tags.filter((t) => t[0] === 'p')
    const lastPTag = pTags[pTags.length - 1]
    if (!lastPTag?.[1]) {
      throw new Error('Reaction must have a p tag')
    }

    const targetEventId = EventId.fromHex(lastETag[1])
    const targetAuthor = Pubkey.fromHex(lastPTag[1])

    // Determine reaction type
    const content = event.content
    let type: ReactionType
    let emoji = content
    let customEmoji: CustomEmoji | undefined

    if (content === '+' || content === '') {
      type = 'like'
      emoji = '+'
    } else if (content === '-') {
      type = 'dislike'
    } else if (content.startsWith(':') && content.endsWith(':')) {
      // Custom emoji format :shortcode:
      const emojiTag = event.tags.find(
        (t) => t[0] === 'emoji' && t[1] === content.slice(1, -1)
      )
      if (emojiTag?.[2]) {
        type = 'custom_emoji'
        customEmoji = {
          shortcode: emojiTag[1],
          url: emojiTag[2]
        }
      } else {
        type = 'emoji'
      }
    } else {
      type = 'emoji'
    }

    return new Reaction(event, targetEventId, targetAuthor, type, emoji, customEmoji)
  }

  /**
   * Try to create a Reaction from an Event, returns null if invalid
   */
  static tryFromEvent(event: Event | null | undefined): Reaction | null {
    if (!event) return null
    try {
      return Reaction.fromEvent(event)
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
   * The reaction's event ID
   */
  get id(): EventId {
    return EventId.fromHex(this._event.id)
  }

  /**
   * The author of the reaction
   */
  get author(): Pubkey {
    return Pubkey.fromHex(this._event.pubkey)
  }

  /**
   * The event ID being reacted to
   */
  get targetEventId(): EventId {
    return this._targetEventId
  }

  /**
   * The author of the event being reacted to
   */
  get targetAuthor(): Pubkey {
    return this._targetAuthor
  }

  /**
   * The type of reaction
   */
  get type(): ReactionType {
    return this._type
  }

  /**
   * The emoji or reaction content
   */
  get emoji(): string {
    return this._emoji
  }

  /**
   * Custom emoji data (if applicable)
   */
  get customEmoji(): CustomEmoji | undefined {
    return this._customEmoji
  }

  /**
   * When the reaction was created
   */
  get createdAt(): Timestamp {
    return Timestamp.fromUnix(this._event.created_at)
  }

  /**
   * Whether this is a like
   */
  get isLike(): boolean {
    return this._type === 'like'
  }

  /**
   * Whether this is a dislike
   */
  get isDislike(): boolean {
    return this._type === 'dislike'
  }

  /**
   * Whether this is a positive reaction (like or emoji, not dislike)
   */
  get isPositive(): boolean {
    return this._type !== 'dislike'
  }

  /**
   * Whether this uses a custom emoji
   */
  get hasCustomEmoji(): boolean {
    return this._type === 'custom_emoji' && !!this._customEmoji
  }

  /**
   * Get the display value for the reaction
   */
  get displayValue(): string {
    if (this._type === 'like') return '❤️'
    if (this._type === 'dislike') return '👎'
    if (this._customEmoji) return `:${this._customEmoji.shortcode}:`
    return this._emoji
  }
}
