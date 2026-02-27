import { nip19 } from 'nostr-tools'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

/**
 * Mention type indicating how the user was referenced
 */
export type MentionType = 'tag' | 'inline' | 'reply_author' | 'quote_author'

/**
 * Mention Value Object
 *
 * Represents a user mention in a note.
 * Handles different mention types and tag generation.
 *
 * Mention types:
 * - tag: Explicit p tag mention
 * - inline: nostr:npub or nostr:nprofile in content
 * - reply_author: Author of the note being replied to
 * - quote_author: Author of the note being quoted
 */
export class Mention {
  private constructor(
    private readonly _pubkey: Pubkey,
    private readonly _type: MentionType,
    private readonly _relayHint: RelayUrl | null,
    private readonly _displayName: string | null
  ) {}

  /**
   * Create a tag mention (from p tag)
   */
  static tag(pubkey: Pubkey, relayHint?: RelayUrl): Mention {
    return new Mention(pubkey, 'tag', relayHint ?? null, null)
  }

  /**
   * Create an inline mention (from content)
   */
  static inline(pubkey: Pubkey, displayName?: string): Mention {
    return new Mention(pubkey, 'inline', null, displayName ?? null)
  }

  /**
   * Create a reply author mention
   */
  static replyAuthor(pubkey: Pubkey, relayHint?: RelayUrl): Mention {
    return new Mention(pubkey, 'reply_author', relayHint ?? null, null)
  }

  /**
   * Create a quote author mention
   */
  static quoteAuthor(pubkey: Pubkey, relayHint?: RelayUrl): Mention {
    return new Mention(pubkey, 'quote_author', relayHint ?? null, null)
  }

  /**
   * Parse mentions from content text
   * Extracts nostr:npub and nostr:nprofile references
   */
  static parseFromContent(content: string): Mention[] {
    const mentions: Mention[] = []
    const seenPubkeys = new Set<string>()

    // Match nostr:npub1... and nostr:nprofile1...
    const regex = /nostr:(npub1[a-z0-9]+|nprofile1[a-z0-9]+)/gi
    const matches = content.matchAll(regex)

    for (const match of matches) {
      try {
        const { type, data } = nip19.decode(match[1])

        if (type === 'npub') {
          const pubkey = Pubkey.tryFromString(data)
          if (pubkey && !seenPubkeys.has(pubkey.hex)) {
            seenPubkeys.add(pubkey.hex)
            mentions.push(Mention.inline(pubkey))
          }
        } else if (type === 'nprofile') {
          const pubkey = Pubkey.tryFromString(data.pubkey)
          if (pubkey && !seenPubkeys.has(pubkey.hex)) {
            seenPubkeys.add(pubkey.hex)
            const relayHint = data.relays?.[0]
              ? RelayUrl.tryCreate(data.relays[0])
              : null
            mentions.push(new Mention(pubkey, 'inline', relayHint, null))
          }
        }
      } catch {
        // Skip invalid bech32
      }
    }

    return mentions
  }

  // Getters
  get pubkey(): Pubkey {
    return this._pubkey
  }

  get type(): MentionType {
    return this._type
  }

  get relayHint(): RelayUrl | null {
    return this._relayHint
  }

  get displayName(): string | null {
    return this._displayName
  }

  get isExplicitTag(): boolean {
    return this._type === 'tag'
  }

  get isInline(): boolean {
    return this._type === 'inline'
  }

  get isFromContext(): boolean {
    return this._type === 'reply_author' || this._type === 'quote_author'
  }

  /**
   * Generate the nostr:npub or nostr:nprofile URI for this mention
   */
  toNostrUri(): string {
    if (this._relayHint) {
      const nprofile = nip19.nprofileEncode({
        pubkey: this._pubkey.hex,
        relays: [this._relayHint.value]
      })
      return `nostr:${nprofile}`
    }
    return `nostr:${this._pubkey.npub}`
  }

  /**
   * Generate the p tag for this mention
   */
  toTag(): string[] {
    const tag = ['p', this._pubkey.hex]
    if (this._relayHint) {
      tag.push(this._relayHint.value)
    }
    return tag
  }

  /**
   * Add a relay hint
   */
  withRelayHint(relay: RelayUrl): Mention {
    return new Mention(this._pubkey, this._type, relay, this._displayName)
  }

  /**
   * Add display name
   */
  withDisplayName(name: string): Mention {
    return new Mention(this._pubkey, this._type, this._relayHint, name)
  }

  /**
   * Check equality (by pubkey only)
   */
  equals(other: Mention): boolean {
    return this._pubkey.hex === other._pubkey.hex
  }

  /**
   * Check if this mention has the same pubkey as another
   */
  hasSamePubkey(pubkey: Pubkey): boolean {
    return this._pubkey.hex === pubkey.hex
  }
}

/**
 * Collection of mentions with deduplication
 */
export class MentionList {
  private constructor(private readonly _mentions: readonly Mention[]) {}

  /**
   * Create empty mention list
   */
  static empty(): MentionList {
    return new MentionList([])
  }

  /**
   * Create from array of mentions (deduplicates)
   */
  static from(mentions: Mention[]): MentionList {
    const seen = new Set<string>()
    const unique: Mention[] = []

    for (const mention of mentions) {
      if (!seen.has(mention.pubkey.hex)) {
        seen.add(mention.pubkey.hex)
        unique.push(mention)
      }
    }

    return new MentionList(unique)
  }

  get mentions(): readonly Mention[] {
    return this._mentions
  }

  get length(): number {
    return this._mentions.length
  }

  get isEmpty(): boolean {
    return this._mentions.length === 0
  }

  /**
   * Get all pubkeys
   */
  get pubkeys(): Pubkey[] {
    return this._mentions.map((m) => m.pubkey)
  }

  /**
   * Add a mention (returns new list)
   */
  add(mention: Mention): MentionList {
    if (this.contains(mention.pubkey)) {
      return this
    }
    return new MentionList([...this._mentions, mention])
  }

  /**
   * Remove a mention by pubkey (returns new list)
   */
  remove(pubkey: Pubkey): MentionList {
    return new MentionList(
      this._mentions.filter((m) => m.pubkey.hex !== pubkey.hex)
    )
  }

  /**
   * Check if a pubkey is mentioned
   */
  contains(pubkey: Pubkey): boolean {
    return this._mentions.some((m) => m.pubkey.hex === pubkey.hex)
  }

  /**
   * Generate all p tags
   */
  toTags(): string[][] {
    return this._mentions.map((m) => m.toTag())
  }

  /**
   * Merge with another mention list
   */
  merge(other: MentionList): MentionList {
    return MentionList.from([...this._mentions, ...other._mentions])
  }
}
