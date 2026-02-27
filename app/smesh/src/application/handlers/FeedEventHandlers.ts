import {
  FeedSwitched,
  ContentFilterUpdated,
  FeedRefreshed,
  NoteCreated,
  NoteDeleted,
  NoteReplied,
  UsersMentioned,
  TimelineEventsReceived,
  TimelineEOSED
} from '@/domain/feed/events'
import { EventHandler, eventDispatcher } from '@/domain/shared'

/**
 * Handlers for Feed domain events
 *
 * These handlers coordinate cross-context updates when feed events occur.
 * They enable coordination between Feed, Social, Content, and UI contexts.
 */

/**
 * Handler for feed switched events
 * Can be used to:
 * - Clear timeline caches for the old feed
 * - Prefetch content for the new feed
 * - Update URL/navigation state
 * - Log analytics
 */
export const handleFeedSwitched: EventHandler<FeedSwitched> = async (event) => {
  console.debug('[FeedEventHandler] Feed switched:', {
    owner: event.owner?.formatted,
    fromType: event.fromType?.value ?? 'none',
    toType: event.toType.value,
    relaySetId: event.relaySetId
  })

  // Future: Clear old timeline cache
  // Future: Trigger new timeline fetch
  // Future: Update analytics
}

/**
 * Handler for content filter updated events
 * Can be used to:
 * - Re-filter current timeline with new settings
 * - Persist filter preferences
 * - Update filter indicators in UI
 */
export const handleContentFilterUpdated: EventHandler<ContentFilterUpdated> = async (event) => {
  console.debug('[FeedEventHandler] Content filter updated:', {
    owner: event.owner.formatted,
    hideRepliesChanged: event.previousFilter.hideReplies !== event.newFilter.hideReplies,
    hideRepostsChanged: event.previousFilter.hideReposts !== event.newFilter.hideReposts,
    nsfwPolicyChanged: event.previousFilter.nsfwPolicy !== event.newFilter.nsfwPolicy
  })

  // Future: Trigger timeline re-filter
  // Future: Persist filter preferences
}

/**
 * Handler for feed refreshed events
 * Can be used to:
 * - Update last refresh timestamp display
 * - Trigger background data fetch
 * - Reset scroll position indicators
 */
export const handleFeedRefreshed: EventHandler<FeedRefreshed> = async (event) => {
  console.debug('[FeedEventHandler] Feed refreshed:', {
    owner: event.owner?.formatted,
    feedType: event.feedType.value
  })

  // Future: Update refresh timestamp in UI
  // Future: Trigger stale data cleanup
}

/**
 * Handler for note created events
 * Can be used to:
 * - Add note to local timeline immediately (optimistic UI)
 * - Create notifications for mentioned users
 * - Update post count displays
 */
export const handleNoteCreated: EventHandler<NoteCreated> = async (event) => {
  console.debug('[FeedEventHandler] Note created:', {
    author: event.author.formatted,
    noteId: event.noteId.hex,
    mentionCount: event.mentions.length,
    isReply: event.isReply,
    isQuote: event.isQuote
  })

  // Future: Add to local timeline if author is self
  // Future: Create mention notifications
}

/**
 * Handler for note deleted events
 * Can be used to:
 * - Remove note from all timelines
 * - Update reply counts on parent notes
 * - Clean up cached data
 */
export const handleNoteDeleted: EventHandler<NoteDeleted> = async (event) => {
  console.debug('[FeedEventHandler] Note deleted:', {
    author: event.author.formatted,
    noteId: event.noteId.hex
  })

  // Future: Remove from timeline display
  // Future: Remove from caches
}

/**
 * Handler for note replied events
 * Can be used to:
 * - Increment reply count on parent note
 * - Create notification for parent note author
 * - Update thread view if open
 */
export const handleNoteReplied: EventHandler<NoteReplied> = async (event) => {
  console.debug('[FeedEventHandler] Note replied:', {
    replier: event.replier.formatted,
    replyNoteId: event.replyNoteId.hex,
    originalNoteId: event.originalNoteId.hex,
    originalAuthor: event.originalAuthor.formatted
  })

  // Future: Increment reply count
  // Future: Create reply notification for parent author
  // Future: Update thread view
}

/**
 * Handler for users mentioned events
 * Can be used to:
 * - Create mention notifications for each mentioned user
 * - Highlight mentions in the source note
 */
export const handleUsersMentioned: EventHandler<UsersMentioned> = async (event) => {
  console.debug('[FeedEventHandler] Users mentioned:', {
    author: event.author.formatted,
    noteId: event.noteId.hex,
    mentionedCount: event.mentionedPubkeys.length
  })

  // Future: Create mention notifications
}

/**
 * Handler for timeline events received
 * Can be used to:
 * - Update event cache
 * - Trigger profile/metadata fetches for new authors
 * - Update unread counts
 */
export const handleTimelineEventsReceived: EventHandler<TimelineEventsReceived> = async (event) => {
  console.debug('[FeedEventHandler] Timeline events received:', {
    feedType: event.feedType.value,
    eventCount: event.eventCount,
    newestTimestamp: event.newestTimestamp.unix,
    isHistorical: event.isHistorical
  })

  // Future: Prefetch profiles for new authors
  // Future: Update new post indicators
}

/**
 * Handler for timeline EOSE (end of stored events)
 * Can be used to:
 * - Mark initial load as complete
 * - Switch from loading to live mode
 * - Update loading indicators
 */
export const handleTimelineEOSED: EventHandler<TimelineEOSED> = async (event) => {
  console.debug('[FeedEventHandler] Timeline EOSE:', {
    feedType: event.feedType.value,
    totalEvents: event.totalEvents
  })

  // Future: Update loading state
  // Future: Show "up to date" indicator
}

/**
 * Register all feed event handlers with the event dispatcher
 */
export function registerFeedEventHandlers(): void {
  eventDispatcher.on('feed.switched', handleFeedSwitched)
  eventDispatcher.on('feed.content_filter_updated', handleContentFilterUpdated)
  eventDispatcher.on('feed.refreshed', handleFeedRefreshed)
  eventDispatcher.on('feed.note_created', handleNoteCreated)
  eventDispatcher.on('feed.note_deleted', handleNoteDeleted)
  eventDispatcher.on('feed.note_replied', handleNoteReplied)
  eventDispatcher.on('feed.users_mentioned', handleUsersMentioned)
  eventDispatcher.on('feed.timeline_events_received', handleTimelineEventsReceived)
  eventDispatcher.on('feed.timeline_eosed', handleTimelineEOSED)
}

/**
 * Unregister all feed event handlers
 */
export function unregisterFeedEventHandlers(): void {
  eventDispatcher.off('feed.switched', handleFeedSwitched)
  eventDispatcher.off('feed.content_filter_updated', handleContentFilterUpdated)
  eventDispatcher.off('feed.refreshed', handleFeedRefreshed)
  eventDispatcher.off('feed.note_created', handleNoteCreated)
  eventDispatcher.off('feed.note_deleted', handleNoteDeleted)
  eventDispatcher.off('feed.note_replied', handleNoteReplied)
  eventDispatcher.off('feed.users_mentioned', handleUsersMentioned)
  eventDispatcher.off('feed.timeline_events_received', handleTimelineEventsReceived)
  eventDispatcher.off('feed.timeline_eosed', handleTimelineEOSED)
}
