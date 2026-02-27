/**
 * Content Bounded Context
 *
 * Handles notes, reactions, reposts, bookmarks, pins, and other content types.
 */

// Entities
export { Note } from './Note'
export type { NoteType, NoteMention, NoteReference } from './Note'

export { Reaction } from './Reaction'
export type { ReactionType, CustomEmoji } from './Reaction'

export { Repost } from './Repost'

// Aggregates
export { BookmarkList } from './BookmarkList'
export type { BookmarkType, BookmarkEntry, BookmarkListChange } from './BookmarkList'

export { PinList, MAX_PINNED_NOTES, CannotPinOthersContentError, CanOnlyPinNotesError } from './PinList'
export type { PinEntry, PinListChange } from './PinList'

// Errors
export {
  InvalidContentError,
  NoteOperationError,
  CannotReactToOwnContentError,
  NoteNotFoundError,
  ContentTooLargeError
} from './errors'

// Domain Events
export {
  EventBookmarked,
  EventUnbookmarked,
  BookmarkListPublished,
  NotePinned,
  NoteUnpinned,
  PinsLimitExceeded,
  PinListPublished,
  ReactionAdded,
  ContentReposted
} from './events'
export type { ContentDomainEvent } from './events'

// Repositories
export type { BookmarkListRepository, PinListRepository } from './repositories'

// Adapters for migration
export {
  // Note adapters
  toNote,
  tryToNote,
  isNoteEvent,
  toNotes,
  // Reaction adapters
  toReaction,
  tryToReaction,
  isReactionEvent,
  toReactions,
  groupReactionsByEmoji,
  countLikes,
  // Repost adapters
  toRepost,
  tryToRepost,
  isRepostEvent,
  toReposts,
  // BookmarkList adapters
  toBookmarkList,
  tryToBookmarkList,
  isBookmarkListEvent,
  // PinList adapters
  toPinList,
  tryToPinList,
  isPinListEvent,
  // Content type detection
  getContentType,
  parseContentEvent
} from './adapters'
