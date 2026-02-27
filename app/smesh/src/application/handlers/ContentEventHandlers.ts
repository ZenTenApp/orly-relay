import {
  EventBookmarked,
  EventUnbookmarked,
  BookmarkListPublished,
  NotePinned,
  NoteUnpinned,
  PinsLimitExceeded,
  PinListPublished,
  ReactionAdded,
  ContentReposted
} from '@/domain/content'
import { EventHandler, eventDispatcher } from '@/domain/shared'

/**
 * Handlers for content domain events
 *
 * These handlers coordinate cross-context updates when content events occur.
 * They enable real-time UI updates and cross-context coordination.
 */

/**
 * Callback for updating reaction counts in UI
 */
export type UpdateReactionCountCallback = (eventId: string, emoji: string, delta: number) => void

/**
 * Callback for updating repost counts in UI
 */
export type UpdateRepostCountCallback = (eventId: string, delta: number) => void

/**
 * Callback for creating notifications
 */
export type CreateNotificationCallback = (
  type: 'reaction' | 'repost' | 'mention' | 'reply',
  actorPubkey: string,
  targetEventId: string
) => void

/**
 * Callback for showing toast messages
 */
export type ShowToastCallback = (message: string, type: 'info' | 'warning' | 'error') => void

/**
 * Callback for updating profile pinned notes
 */
export type UpdateProfilePinsCallback = (pubkey: string) => void

/**
 * Service callbacks for cross-context coordination
 */
export interface ContentHandlerCallbacks {
  onUpdateReactionCount?: UpdateReactionCountCallback
  onUpdateRepostCount?: UpdateRepostCountCallback
  onCreateNotification?: CreateNotificationCallback
  onShowToast?: ShowToastCallback
  onUpdateProfilePins?: UpdateProfilePinsCallback
}

let callbacks: ContentHandlerCallbacks = {}

/**
 * Set the callbacks for cross-context coordination
 * Call this during provider initialization
 */
export function setContentHandlerCallbacks(newCallbacks: ContentHandlerCallbacks): void {
  callbacks = { ...callbacks, ...newCallbacks }
}

/**
 * Clear all callbacks (for cleanup/testing)
 */
export function clearContentHandlerCallbacks(): void {
  callbacks = {}
}

/**
 * Handler for event bookmarked
 * Can be used to:
 * - Update bookmark count displays
 * - Prefetch bookmarked content for offline access
 */
export const handleEventBookmarked: EventHandler<EventBookmarked> = async (event) => {
  console.debug('[ContentEventHandler] Event bookmarked:', {
    actor: event.actor.formatted,
    bookmarkedEventId: event.bookmarkedEventId,
    type: event.bookmarkType
  })

  // Future: Trigger prefetch of bookmarked content
}

/**
 * Handler for event unbookmarked
 */
export const handleEventUnbookmarked: EventHandler<EventUnbookmarked> = async (event) => {
  console.debug('[ContentEventHandler] Event unbookmarked:', {
    actor: event.actor.formatted,
    unbookmarkedEventId: event.unbookmarkedEventId
  })
}

/**
 * Handler for bookmark list published
 */
export const handleBookmarkListPublished: EventHandler<BookmarkListPublished> = async (event) => {
  console.debug('[ContentEventHandler] Bookmark list published:', {
    owner: event.owner.formatted,
    bookmarkCount: event.bookmarkCount
  })
}

/**
 * Handler for note pinned
 * Coordinates with:
 * - Profile context: Update pinned notes display
 * - Cache context: Ensure pinned content is cached
 */
export const handleNotePinned: EventHandler<NotePinned> = async (event) => {
  console.debug('[ContentEventHandler] Note pinned:', {
    actor: event.actor.formatted,
    pinnedEventId: event.pinnedEventId.hex
  })

  // Update profile display to show new pinned note
  if (callbacks.onUpdateProfilePins) {
    callbacks.onUpdateProfilePins(event.actor.hex)
  }
}

/**
 * Handler for note unpinned
 * Coordinates with:
 * - Profile context: Update pinned notes display
 */
export const handleNoteUnpinned: EventHandler<NoteUnpinned> = async (event) => {
  console.debug('[ContentEventHandler] Note unpinned:', {
    actor: event.actor.formatted,
    unpinnedEventId: event.unpinnedEventId
  })

  // Update profile display to remove unpinned note
  if (callbacks.onUpdateProfilePins) {
    callbacks.onUpdateProfilePins(event.actor.hex)
  }
}

/**
 * Handler for pins limit exceeded
 * Coordinates with:
 * - UI context: Show toast notification about removed pins
 */
export const handlePinsLimitExceeded: EventHandler<PinsLimitExceeded> = async (event) => {
  console.debug('[ContentEventHandler] Pins limit exceeded:', {
    actor: event.actor.formatted,
    removedCount: event.removedEventIds.length
  })

  // Show toast notification about removed pins
  if (callbacks.onShowToast) {
    callbacks.onShowToast(
      `Pin limit reached. ${event.removedEventIds.length} older pin(s) were removed.`,
      'warning'
    )
  }
}

/**
 * Handler for pin list published
 */
export const handlePinListPublished: EventHandler<PinListPublished> = async (event) => {
  console.debug('[ContentEventHandler] Pin list published:', {
    owner: event.owner.formatted,
    pinCount: event.pinCount
  })
}

/**
 * Handler for reaction added
 * Coordinates with:
 * - UI context: Update reaction counts in real-time
 * - Notification context: Create notification for content author
 */
export const handleReactionAdded: EventHandler<ReactionAdded> = async (event) => {
  console.debug('[ContentEventHandler] Reaction added:', {
    actor: event.actor.formatted,
    targetEventId: event.targetEventId.hex,
    targetAuthor: event.targetAuthor.formatted,
    emoji: event.emoji,
    isLike: event.isLike
  })

  // Update reaction count in UI
  if (callbacks.onUpdateReactionCount) {
    callbacks.onUpdateReactionCount(event.targetEventId.hex, event.emoji, 1)
  }

  // Create notification for the content author (if not self)
  if (callbacks.onCreateNotification && event.actor.hex !== event.targetAuthor.hex) {
    callbacks.onCreateNotification('reaction', event.actor.hex, event.targetEventId.hex)
  }
}

/**
 * Handler for content reposted
 * Coordinates with:
 * - UI context: Update repost counts in real-time
 * - Notification context: Create notification for original author
 */
export const handleContentReposted: EventHandler<ContentReposted> = async (event) => {
  console.debug('[ContentEventHandler] Content reposted:', {
    actor: event.actor.formatted,
    originalEventId: event.originalEventId.hex,
    originalAuthor: event.originalAuthor.formatted
  })

  // Update repost count in UI
  if (callbacks.onUpdateRepostCount) {
    callbacks.onUpdateRepostCount(event.originalEventId.hex, 1)
  }

  // Create notification for the original author (if not self)
  if (callbacks.onCreateNotification && event.actor.hex !== event.originalAuthor.hex) {
    callbacks.onCreateNotification('repost', event.actor.hex, event.originalEventId.hex)
  }
}

/**
 * Register all content event handlers with the event dispatcher
 */
export function registerContentEventHandlers(): void {
  eventDispatcher.on('content.event_bookmarked', handleEventBookmarked)
  eventDispatcher.on('content.event_unbookmarked', handleEventUnbookmarked)
  eventDispatcher.on('content.bookmark_list_published', handleBookmarkListPublished)
  eventDispatcher.on('content.note_pinned', handleNotePinned)
  eventDispatcher.on('content.note_unpinned', handleNoteUnpinned)
  eventDispatcher.on('content.pins_limit_exceeded', handlePinsLimitExceeded)
  eventDispatcher.on('content.pin_list_published', handlePinListPublished)
  eventDispatcher.on('content.reaction_added', handleReactionAdded)
  eventDispatcher.on('content.reposted', handleContentReposted)
}

/**
 * Unregister all content event handlers
 */
export function unregisterContentEventHandlers(): void {
  eventDispatcher.off('content.event_bookmarked', handleEventBookmarked)
  eventDispatcher.off('content.event_unbookmarked', handleEventUnbookmarked)
  eventDispatcher.off('content.bookmark_list_published', handleBookmarkListPublished)
  eventDispatcher.off('content.note_pinned', handleNotePinned)
  eventDispatcher.off('content.note_unpinned', handleNoteUnpinned)
  eventDispatcher.off('content.pins_limit_exceeded', handlePinsLimitExceeded)
  eventDispatcher.off('content.pin_list_published', handlePinListPublished)
  eventDispatcher.off('content.reaction_added', handleReactionAdded)
  eventDispatcher.off('content.reposted', handleContentReposted)
}
