import { Pubkey } from '../shared/value-objects/Pubkey'
import { EventId } from '../shared/value-objects/EventId'
import { ReplyContext } from './ReplyContext'
import { QuoteContext } from './QuoteContext'
import { NoteCreated, NoteReplied, UsersMentioned } from './events'

/**
 * Options for note composition
 */
export interface NoteComposerOptions {
  isNsfw?: boolean
  addClientTag?: boolean
  isProtected?: boolean
}

/**
 * Poll option for poll notes
 */
export interface PollOption {
  id: string
  text: string
}

/**
 * Poll configuration
 */
export interface PollConfig {
  isMultipleChoice: boolean
  options: PollOption[]
  endsAt?: number
  relays: string[]
}

/**
 * Validation result for note composition
 */
export interface ValidationResult {
  isValid: boolean
  errors: string[]
}

/**
 * Extracted content elements from note text
 */
export interface ExtractedContent {
  hashtags: string[]
  mentionedPubkeys: Pubkey[]
  quotedEventIds: string[]
  imageUrls: string[]
}

/**
 * NoteComposer Aggregate
 *
 * Represents the state and business logic for composing a new note.
 * This is the write-side aggregate for the Feed bounded context.
 *
 * The NoteComposer handles:
 * - Text content with mentions, hashtags, and embedded media
 * - Reply threading (using ReplyContext)
 * - Quote posts (using QuoteContext)
 * - Poll creation
 * - Content warnings (NSFW)
 * - Validation before publishing
 *
 * Note: The NoteComposer does NOT handle actual publishing or signing.
 * Those are infrastructure concerns handled by the application layer.
 *
 * Invariants:
 * - Author must be set before publishing
 * - Content must not be empty (unless it's a repost)
 * - Poll must have at least 2 options if enabled
 */
export class NoteComposer {
  private constructor(
    private readonly _author: Pubkey,
    private _content: string,
    private _replyContext: ReplyContext | null,
    private _quoteContext: QuoteContext | null,
    private _additionalMentions: readonly Pubkey[],
    private _options: NoteComposerOptions,
    private _pollConfig: PollConfig | null
  ) {}

  // ============================================================================
  // Factory Methods
  // ============================================================================

  /**
   * Create a new note composer for a fresh post
   */
  static create(author: Pubkey): NoteComposer {
    return new NoteComposer(
      author,
      '',
      null,
      null,
      [],
      {
        isNsfw: false,
        addClientTag: false,
        isProtected: false
      },
      null
    )
  }

  /**
   * Create a note composer for a reply
   */
  static reply(author: Pubkey, replyTo: ReplyContext): NoteComposer {
    return new NoteComposer(
      author,
      '',
      replyTo,
      null,
      [],
      {
        isNsfw: false,
        addClientTag: false,
        isProtected: false
      },
      null
    )
  }

  /**
   * Create a note composer for a quote post
   */
  static quote(author: Pubkey, quoteNote: QuoteContext): NoteComposer {
    return new NoteComposer(
      author,
      '',
      null,
      quoteNote,
      [],
      {
        isNsfw: false,
        addClientTag: false,
        isProtected: false
      },
      null
    )
  }

  /**
   * Create a note composer for a poll
   */
  static poll(author: Pubkey): NoteComposer {
    return new NoteComposer(
      author,
      '',
      null,
      null,
      [],
      {
        isNsfw: false,
        addClientTag: false,
        isProtected: false
      },
      {
        isMultipleChoice: false,
        options: [],
        relays: []
      }
    )
  }

  // ============================================================================
  // Queries
  // ============================================================================

  get author(): Pubkey {
    return this._author
  }

  get content(): string {
    return this._content
  }

  get replyContext(): ReplyContext | null {
    return this._replyContext
  }

  get quoteContext(): QuoteContext | null {
    return this._quoteContext
  }

  get additionalMentions(): readonly Pubkey[] {
    return this._additionalMentions
  }

  get options(): NoteComposerOptions {
    return { ...this._options }
  }

  get pollConfig(): PollConfig | null {
    return this._pollConfig ? { ...this._pollConfig } : null
  }

  get isReply(): boolean {
    return this._replyContext !== null
  }

  get isQuote(): boolean {
    return this._quoteContext !== null
  }

  get isPoll(): boolean {
    return this._pollConfig !== null
  }

  get isNsfw(): boolean {
    return this._options.isNsfw ?? false
  }

  /**
   * Get all mentioned pubkeys (from reply context + additional mentions)
   */
  get allMentions(): Pubkey[] {
    const mentions: Pubkey[] = []
    const seenHexes = new Set<string>()

    // Add mentions from reply context
    if (this._replyContext) {
      for (const pk of this._replyContext.mentionedPubkeys) {
        if (!seenHexes.has(pk.hex)) {
          mentions.push(pk)
          seenHexes.add(pk.hex)
        }
      }
    }

    // Add mentions from quote context
    if (this._quoteContext) {
      const quotedAuthor = this._quoteContext.quotedAuthor
      if (!seenHexes.has(quotedAuthor.hex)) {
        mentions.push(quotedAuthor)
        seenHexes.add(quotedAuthor.hex)
      }
    }

    // Add additional mentions
    for (const pk of this._additionalMentions) {
      if (!seenHexes.has(pk.hex)) {
        mentions.push(pk)
        seenHexes.add(pk.hex)
      }
    }

    return mentions
  }

  /**
   * Extract hashtags from content
   */
  get hashtags(): string[] {
    const matches = this._content.match(/#[\p{L}\p{N}\p{M}]+/gu)
    if (!matches) return []
    return matches.map((m) => m.slice(1).toLowerCase()).filter(Boolean)
  }

  /**
   * Get the effective content for publishing
   * (includes quote URI if quoting)
   */
  get effectiveContent(): string {
    if (this._quoteContext) {
      return this._quoteContext.appendToContent(this._content)
    }
    return this._content
  }

  // ============================================================================
  // Commands (Immutable - return new instances)
  // ============================================================================

  /**
   * Set the content text
   */
  setContent(content: string): NoteComposer {
    return new NoteComposer(
      this._author,
      content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      this._options,
      this._pollConfig
    )
  }

  /**
   * Add a mention
   */
  addMention(pubkey: Pubkey): NoteComposer {
    // Check if already mentioned
    if (this._additionalMentions.some((p) => p.hex === pubkey.hex)) {
      return this
    }

    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      [...this._additionalMentions, pubkey],
      this._options,
      this._pollConfig
    )
  }

  /**
   * Remove a mention
   */
  removeMention(pubkey: Pubkey): NoteComposer {
    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions.filter((p) => p.hex !== pubkey.hex),
      this._options,
      this._pollConfig
    )
  }

  /**
   * Set content warning (NSFW)
   */
  setContentWarning(isNsfw: boolean): NoteComposer {
    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      { ...this._options, isNsfw },
      this._pollConfig
    )
  }

  /**
   * Set client tag option
   */
  setClientTag(addClientTag: boolean): NoteComposer {
    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      { ...this._options, addClientTag },
      this._pollConfig
    )
  }

  /**
   * Set protected event option
   */
  setProtected(isProtected: boolean): NoteComposer {
    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      { ...this._options, isProtected },
      this._pollConfig
    )
  }

  /**
   * Enable poll mode with configuration
   */
  enablePoll(config: PollConfig): NoteComposer {
    // Polls can't be replies
    return new NoteComposer(
      this._author,
      this._content,
      null, // Clear reply context
      this._quoteContext,
      this._additionalMentions,
      this._options,
      config
    )
  }

  /**
   * Disable poll mode
   */
  disablePoll(): NoteComposer {
    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      this._options,
      null
    )
  }

  /**
   * Update poll options
   */
  setPollOptions(options: PollOption[]): NoteComposer {
    if (!this._pollConfig) return this

    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      this._options,
      { ...this._pollConfig, options }
    )
  }

  /**
   * Set poll multiple choice mode
   */
  setPollMultipleChoice(isMultipleChoice: boolean): NoteComposer {
    if (!this._pollConfig) return this

    return new NoteComposer(
      this._author,
      this._content,
      this._replyContext,
      this._quoteContext,
      this._additionalMentions,
      this._options,
      { ...this._pollConfig, isMultipleChoice }
    )
  }

  // ============================================================================
  // Validation
  // ============================================================================

  /**
   * Validate the note is ready for publishing
   */
  validate(): ValidationResult {
    const errors: string[] = []

    // Content must not be empty (for regular posts)
    if (!this._content.trim() && !this.isPoll) {
      errors.push('Content cannot be empty')
    }

    // Poll validation
    if (this._pollConfig) {
      const validOptions = this._pollConfig.options.filter((opt) => opt.text.trim())
      if (validOptions.length < 2) {
        errors.push('Poll must have at least 2 options')
      }
      if (!this._content.trim()) {
        errors.push('Poll question cannot be empty')
      }
    }

    return {
      isValid: errors.length === 0,
      errors
    }
  }

  /**
   * Check if the note can be published
   */
  canPublish(): boolean {
    return this.validate().isValid
  }

  // ============================================================================
  // Domain Events
  // ============================================================================

  /**
   * Create the NoteCreated domain event
   * Call this after successful publishing
   */
  createNoteCreatedEvent(noteId: EventId): NoteCreated {
    return new NoteCreated(
      this._author,
      noteId,
      this._replyContext?.replyToEvent
        ? EventId.tryFromString(this._replyContext.replyToEvent.eventId)
        : null,
      this._quoteContext
        ? EventId.tryFromString(this._quoteContext.quotedEventId)
        : null,
      this.allMentions,
      this.hashtags
    )
  }

  /**
   * Create the NoteReplied domain event (if this is a reply)
   * Call this after successful publishing
   */
  createNoteRepliedEvent(replyNoteId: EventId): NoteReplied | null {
    if (!this._replyContext) return null

    const originalNoteId = EventId.tryFromString(this._replyContext.replyToEvent.eventId)
    if (!originalNoteId) return null

    return new NoteReplied(
      originalNoteId,
      this._replyContext.replyToEvent.pubkey,
      replyNoteId,
      this._author
    )
  }

  /**
   * Create the UsersMentioned domain event (if there are mentions)
   * Call this after successful publishing
   */
  createUsersMentionedEvent(noteId: EventId): UsersMentioned | null {
    const mentions = this.allMentions
    if (mentions.length === 0) return null

    return new UsersMentioned(noteId, this._author, mentions)
  }
}
