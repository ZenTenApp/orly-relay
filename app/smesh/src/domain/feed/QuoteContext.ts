import { Event, nip19 } from 'nostr-tools'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

/**
 * QuoteContext Value Object
 *
 * Encapsulates the context needed for quoting another note.
 * Handles NIP-27 (nostr: URI) generation and proper tagging.
 *
 * Unlike replies, quotes are standalone posts that reference
 * another event. The referenced event is shown inline in the
 * quoting post.
 *
 * Tags used:
 * - 'q' tag: The quoted event ID (NIP-18)
 * - 'p' tag: The quoted author's pubkey
 *
 * The quote is inserted into content using nostr:nevent1... format.
 */
export class QuoteContext {
  private constructor(
    private readonly _quotedEventId: string,
    private readonly _quotedAuthor: Pubkey,
    private readonly _relayHints: readonly RelayUrl[],
    private readonly _quotedKind?: number
  ) {}

  /**
   * Create a quote context from an event
   */
  static fromEvent(event: Event, relayHints: RelayUrl[] = []): QuoteContext {
    const authorPubkey = Pubkey.tryFromString(event.pubkey)
    if (!authorPubkey) {
      throw new Error('Invalid pubkey in event being quoted')
    }

    return new QuoteContext(event.id, authorPubkey, relayHints, event.kind)
  }

  /**
   * Create a quote context from components
   */
  static create(
    eventId: string,
    author: Pubkey,
    relayHints: RelayUrl[] = [],
    kind?: number
  ): QuoteContext {
    return new QuoteContext(eventId, author, relayHints, kind)
  }

  // Getters
  get quotedEventId(): string {
    return this._quotedEventId
  }

  get quotedAuthor(): Pubkey {
    return this._quotedAuthor
  }

  get relayHints(): readonly RelayUrl[] {
    return this._relayHints
  }

  get quotedKind(): number | undefined {
    return this._quotedKind
  }

  /**
   * Generate the nostr:nevent1... URI for embedding in content
   *
   * Uses NIP-19 nevent encoding which includes:
   * - Event ID
   * - Relay hints (for fetching)
   * - Author pubkey (for verification)
   * - Kind (optional, for context)
   */
  toNostrUri(): string {
    const nevent = nip19.neventEncode({
      id: this._quotedEventId,
      relays: this._relayHints.map((r) => r.value),
      author: this._quotedAuthor.hex,
      kind: this._quotedKind
    })
    return `nostr:${nevent}`
  }

  /**
   * Generate the simple note reference (nostr:note1...)
   * Use this for simpler clients that don't support nevent
   */
  toSimpleNostrUri(): string {
    const note = nip19.noteEncode(this._quotedEventId)
    return `nostr:${note}`
  }

  /**
   * Generate tags for a quote post
   *
   * Returns:
   * - ['q', eventId, relayHint?] - The quoted event
   * - ['p', pubkey] - The quoted author
   */
  toTags(): string[][] {
    const tags: string[][] = []

    // Quote tag (NIP-18)
    const quoteTag = ['q', this._quotedEventId]
    if (this._relayHints.length > 0) {
      quoteTag.push(this._relayHints[0].value)
    }
    tags.push(quoteTag)

    // Pubkey tag for the quoted author
    tags.push(['p', this._quotedAuthor.hex])

    return tags
  }

  /**
   * Append the quote to content
   *
   * Adds a newline and the nostr: URI to the end of the content.
   * Returns the modified content string.
   */
  appendToContent(content: string): string {
    const uri = this.toNostrUri()
    const trimmed = content.trim()

    if (trimmed.length === 0) {
      return uri
    }

    // Check if content already ends with the URI
    if (trimmed.endsWith(uri)) {
      return trimmed
    }

    return `${trimmed}\n\n${uri}`
  }

  /**
   * Check if content already contains this quote
   */
  isInContent(content: string): boolean {
    // Check for both nevent and note formats
    return (
      content.includes(this.toNostrUri()) ||
      content.includes(this.toSimpleNostrUri()) ||
      content.includes(this._quotedEventId)
    )
  }

  /**
   * Add a relay hint
   */
  withRelayHint(relay: RelayUrl): QuoteContext {
    const existingUrls = new Set(this._relayHints.map((r) => r.value))
    if (existingUrls.has(relay.value)) {
      return this
    }
    return new QuoteContext(
      this._quotedEventId,
      this._quotedAuthor,
      [...this._relayHints, relay],
      this._quotedKind
    )
  }

  /**
   * Check equality
   */
  equals(other: QuoteContext): boolean {
    return this._quotedEventId === other._quotedEventId
  }
}
