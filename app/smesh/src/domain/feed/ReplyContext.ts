import { Event } from 'nostr-tools'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

/**
 * Information about a referenced event
 */
export interface EventReference {
  eventId: string
  pubkey: Pubkey
  relayHint?: RelayUrl
}

/**
 * ReplyContext Value Object
 *
 * Encapsulates the context needed for creating a reply to a note.
 * Handles NIP-10 compliant tagging for proper thread structure.
 *
 * NIP-10 Threading:
 * - Root tag: The original post that started the thread
 * - Reply tag: The immediate parent being replied to
 * - Mention tags: Other events referenced but not directly replied to
 *
 * This value object extracts the threading information from events
 * and generates proper tags for new replies.
 */
export class ReplyContext {
  private constructor(
    private readonly _rootEvent: EventReference | null,
    private readonly _replyToEvent: EventReference,
    private readonly _mentionedEvents: readonly EventReference[],
    private readonly _mentionedPubkeys: readonly Pubkey[]
  ) {}

  /**
   * Create a reply context from an event being replied to
   *
   * Extracts existing thread structure from the event's tags
   * to maintain proper threading.
   */
  static fromEvent(event: Event): ReplyContext {
    const replyToPubkey = Pubkey.tryFromString(event.pubkey)
    if (!replyToPubkey) {
      throw new Error('Invalid pubkey in event being replied to')
    }

    // Extract root and other thread info from existing tags
    let rootEvent: EventReference | null = null
    const mentionedEvents: EventReference[] = []
    const mentionedPubkeys: Pubkey[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'e' && tag[1]) {
        const marker = tag[3]
        const eventId = tag[1]
        const relayHint = tag[2] ? RelayUrl.tryCreate(tag[2]) : undefined

        // Find the event's author for this reference
        // We may not have it, so we'll just use the event pubkey as fallback
        const refPubkey = replyToPubkey // Fallback

        if (marker === 'root') {
          rootEvent = {
            eventId,
            pubkey: refPubkey,
            relayHint: relayHint ?? undefined
          }
        } else if (marker === 'mention') {
          mentionedEvents.push({
            eventId,
            pubkey: refPubkey,
            relayHint: relayHint ?? undefined
          })
        }
        // Skip 'reply' marker as we'll set the current event as the new reply target
      }

      if (tag[0] === 'p' && tag[1]) {
        const pk = Pubkey.tryFromString(tag[1])
        if (pk) {
          mentionedPubkeys.push(pk)
        }
      }
    }

    // The event being replied to becomes the new reply target
    const replyToEvent: EventReference = {
      eventId: event.id,
      pubkey: replyToPubkey
    }

    // If the event had no root, it's a top-level post, so it becomes the root
    if (!rootEvent) {
      rootEvent = replyToEvent
    }

    // Add the reply-to author to mentioned pubkeys if not already present
    const pubkeySet = new Set(mentionedPubkeys.map((p) => p.hex))
    if (!pubkeySet.has(replyToPubkey.hex)) {
      mentionedPubkeys.push(replyToPubkey)
    }

    return new ReplyContext(rootEvent, replyToEvent, mentionedEvents, mentionedPubkeys)
  }

  /**
   * Create a simple reply context (no existing thread)
   */
  static simple(eventId: string, authorPubkey: Pubkey, relayHint?: RelayUrl): ReplyContext {
    const ref: EventReference = {
      eventId,
      pubkey: authorPubkey,
      relayHint
    }
    return new ReplyContext(ref, ref, [], [authorPubkey])
  }

  // Getters
  get rootEvent(): EventReference | null {
    return this._rootEvent
  }

  get replyToEvent(): EventReference {
    return this._replyToEvent
  }

  get mentionedEvents(): readonly EventReference[] {
    return this._mentionedEvents
  }

  get mentionedPubkeys(): readonly Pubkey[] {
    return this._mentionedPubkeys
  }

  /**
   * Check if this is a reply to a top-level post (not nested)
   */
  get isDirectReply(): boolean {
    return (
      this._rootEvent !== null && this._rootEvent.eventId === this._replyToEvent.eventId
    )
  }

  /**
   * Check if this is a nested reply (reply to a reply)
   */
  get isNestedReply(): boolean {
    return (
      this._rootEvent !== null && this._rootEvent.eventId !== this._replyToEvent.eventId
    )
  }

  /**
   * Get the thread depth (0 for direct reply to root, 1+ for nested)
   */
  get depth(): number {
    return this._mentionedEvents.length
  }

  /**
   * Generate NIP-10 compliant tags for a reply
   *
   * Returns tags in the format:
   * - ['e', rootId, relayHint?, 'root']
   * - ['e', replyId, relayHint?, 'reply']
   * - ['p', pubkey, relayHint?] for each mentioned pubkey
   */
  toTags(): string[][] {
    const tags: string[][] = []

    // Root tag (the original post in the thread)
    if (this._rootEvent) {
      const rootTag = ['e', this._rootEvent.eventId]
      if (this._rootEvent.relayHint) {
        rootTag.push(this._rootEvent.relayHint.value)
      } else {
        rootTag.push('')
      }
      rootTag.push('root')
      tags.push(rootTag)
    }

    // Reply tag (the immediate parent)
    // Only add if different from root
    if (!this._rootEvent || this._rootEvent.eventId !== this._replyToEvent.eventId) {
      const replyTag = ['e', this._replyToEvent.eventId]
      if (this._replyToEvent.relayHint) {
        replyTag.push(this._replyToEvent.relayHint.value)
      } else {
        replyTag.push('')
      }
      replyTag.push('reply')
      tags.push(replyTag)
    } else if (this._rootEvent) {
      // If root and reply are the same, use 'reply' marker
      // (overwrite the root tag to be 'reply' for direct replies)
      tags[0][3] = 'reply'
    }

    // Pubkey tags for all mentioned authors
    const addedPubkeys = new Set<string>()
    for (const pubkey of this._mentionedPubkeys) {
      if (!addedPubkeys.has(pubkey.hex)) {
        tags.push(['p', pubkey.hex])
        addedPubkeys.add(pubkey.hex)
      }
    }

    return tags
  }

  /**
   * Add an additional pubkey mention
   */
  withMentionedPubkey(pubkey: Pubkey): ReplyContext {
    const existingHexes = new Set(this._mentionedPubkeys.map((p) => p.hex))
    if (existingHexes.has(pubkey.hex)) {
      return this
    }
    return new ReplyContext(
      this._rootEvent,
      this._replyToEvent,
      this._mentionedEvents,
      [...this._mentionedPubkeys, pubkey]
    )
  }

  /**
   * Check equality
   */
  equals(other: ReplyContext): boolean {
    if (this._replyToEvent.eventId !== other._replyToEvent.eventId) return false
    if (this._rootEvent?.eventId !== other._rootEvent?.eventId) return false
    return true
  }
}
