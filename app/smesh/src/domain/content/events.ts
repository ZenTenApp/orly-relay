import { Pubkey, EventId, DomainEvent } from '../shared'

// ============================================================================
// Bookmark Events
// ============================================================================

/**
 * Raised when an event is bookmarked
 */
export class EventBookmarked extends DomainEvent {
  readonly eventType = 'content.event_bookmarked'

  constructor(
    readonly actor: Pubkey,
    readonly bookmarkedEventId: string,
    readonly bookmarkType: 'event' | 'replaceable'
  ) {
    super()
  }
}

/**
 * Raised when an event is removed from bookmarks
 */
export class EventUnbookmarked extends DomainEvent {
  readonly eventType = 'content.event_unbookmarked'

  constructor(
    readonly actor: Pubkey,
    readonly unbookmarkedEventId: string
  ) {
    super()
  }
}

/**
 * Raised when a bookmark list is published
 */
export class BookmarkListPublished extends DomainEvent {
  readonly eventType = 'content.bookmark_list_published'

  constructor(
    readonly owner: Pubkey,
    readonly bookmarkCount: number
  ) {
    super()
  }
}

// ============================================================================
// Pin Events
// ============================================================================

/**
 * Raised when a note is pinned
 */
export class NotePinned extends DomainEvent {
  readonly eventType = 'content.note_pinned'

  constructor(
    readonly actor: Pubkey,
    readonly pinnedEventId: EventId
  ) {
    super()
  }
}

/**
 * Raised when a note is unpinned
 */
export class NoteUnpinned extends DomainEvent {
  readonly eventType = 'content.note_unpinned'

  constructor(
    readonly actor: Pubkey,
    readonly unpinnedEventId: string
  ) {
    super()
  }
}

/**
 * Raised when old pins are removed due to limit
 */
export class PinsLimitExceeded extends DomainEvent {
  readonly eventType = 'content.pins_limit_exceeded'

  constructor(
    readonly actor: Pubkey,
    readonly removedEventIds: string[]
  ) {
    super()
  }
}

/**
 * Raised when a pin list is published
 */
export class PinListPublished extends DomainEvent {
  readonly eventType = 'content.pin_list_published'

  constructor(
    readonly owner: Pubkey,
    readonly pinCount: number
  ) {
    super()
  }
}

// ============================================================================
// Reaction Events
// ============================================================================

/**
 * Raised when a reaction is added to content
 */
export class ReactionAdded extends DomainEvent {
  readonly eventType = 'content.reaction_added'

  constructor(
    readonly actor: Pubkey,
    readonly targetEventId: EventId,
    readonly targetAuthor: Pubkey,
    readonly emoji: string,
    readonly isLike: boolean
  ) {
    super()
  }
}

// ============================================================================
// Repost Events
// ============================================================================

/**
 * Raised when content is reposted
 */
export class ContentReposted extends DomainEvent {
  readonly eventType = 'content.reposted'

  constructor(
    readonly actor: Pubkey,
    readonly originalEventId: EventId,
    readonly originalAuthor: Pubkey
  ) {
    super()
  }
}

// ============================================================================
// Event Types Union
// ============================================================================

/**
 * Union type of all content domain events
 */
export type ContentDomainEvent =
  | EventBookmarked
  | EventUnbookmarked
  | BookmarkListPublished
  | NotePinned
  | NoteUnpinned
  | PinsLimitExceeded
  | PinListPublished
  | ReactionAdded
  | ContentReposted
