import { DomainEvent } from '../shared/events'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { EventId } from '../shared/value-objects/EventId'
import { Timestamp } from '../shared/value-objects/Timestamp'
import { FeedType } from './FeedType'
import { ContentFilter } from './ContentFilter'

// ============================================================================
// Feed Configuration Events
// ============================================================================

/**
 * Raised when the active feed is switched
 */
export class FeedSwitched extends DomainEvent {
  get eventType(): string {
    return 'feed.switched'
  }

  constructor(
    readonly owner: Pubkey | null,
    readonly fromType: FeedType | null,
    readonly toType: FeedType,
    readonly relaySetId?: string
  ) {
    super()
  }
}

/**
 * Raised when the content filter settings are updated
 */
export class ContentFilterUpdated extends DomainEvent {
  get eventType(): string {
    return 'feed.content_filter_updated'
  }

  constructor(
    readonly owner: Pubkey,
    readonly previousFilter: ContentFilter,
    readonly newFilter: ContentFilter
  ) {
    super()
  }
}

/**
 * Raised when a feed is manually refreshed
 */
export class FeedRefreshed extends DomainEvent {
  get eventType(): string {
    return 'feed.refreshed'
  }

  constructor(
    readonly owner: Pubkey | null,
    readonly feedType: FeedType
  ) {
    super()
  }
}

// ============================================================================
// Note Lifecycle Events
// ============================================================================

/**
 * Raised when a new note is created/published by the current user
 */
export class NoteCreated extends DomainEvent {
  get eventType(): string {
    return 'feed.note_created'
  }

  constructor(
    readonly author: Pubkey,
    readonly noteId: EventId,
    readonly replyTo: EventId | null,
    readonly quotedNote: EventId | null,
    readonly mentions: readonly Pubkey[],
    readonly hashtags: readonly string[]
  ) {
    super()
  }

  get isReply(): boolean {
    return this.replyTo !== null
  }

  get isQuote(): boolean {
    return this.quotedNote !== null
  }

  get hasMentions(): boolean {
    return this.mentions.length > 0
  }
}

/**
 * Raised when a note is deleted (deletion event received)
 */
export class NoteDeleted extends DomainEvent {
  get eventType(): string {
    return 'feed.note_deleted'
  }

  constructor(
    readonly author: Pubkey,
    readonly noteId: EventId
  ) {
    super()
  }
}

/**
 * Raised when a reply is received for a note the user cares about
 */
export class NoteReplied extends DomainEvent {
  get eventType(): string {
    return 'feed.note_replied'
  }

  constructor(
    readonly originalNoteId: EventId,
    readonly originalAuthor: Pubkey,
    readonly replyNoteId: EventId,
    readonly replier: Pubkey
  ) {
    super()
  }
}

/**
 * Raised when users are mentioned in a note
 */
export class UsersMentioned extends DomainEvent {
  get eventType(): string {
    return 'feed.users_mentioned'
  }

  constructor(
    readonly noteId: EventId,
    readonly author: Pubkey,
    readonly mentionedPubkeys: readonly Pubkey[]
  ) {
    super()
  }
}

// ============================================================================
// Timeline Events
// ============================================================================

/**
 * Raised when new events arrive in the active timeline
 */
export class TimelineEventsReceived extends DomainEvent {
  get eventType(): string {
    return 'feed.timeline_events_received'
  }

  constructor(
    readonly feedType: FeedType,
    readonly eventCount: number,
    readonly newestTimestamp: Timestamp,
    readonly isHistorical: boolean
  ) {
    super()
  }
}

/**
 * Raised when end-of-stored-events is received from all relays
 */
export class TimelineEOSED extends DomainEvent {
  get eventType(): string {
    return 'feed.timeline_eosed'
  }

  constructor(
    readonly feedType: FeedType,
    readonly totalEvents: number
  ) {
    super()
  }
}

/**
 * Raised when more events are loaded (pagination)
 */
export class TimelineLoadedMore extends DomainEvent {
  get eventType(): string {
    return 'feed.timeline_loaded_more'
  }

  constructor(
    readonly feedType: FeedType,
    readonly loadedCount: number,
    readonly oldestTimestamp: Timestamp
  ) {
    super()
  }
}
