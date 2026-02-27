/**
 * Domain Event Handlers
 *
 * Application-level handlers that coordinate cross-context updates
 * when domain events occur.
 */

// Social Event Handlers
export {
  registerSocialEventHandlers,
  unregisterSocialEventHandlers,
  setSocialHandlerCallbacks,
  clearSocialHandlerCallbacks,
  handleUserFollowed,
  handleUserUnfollowed,
  handleUserMuted,
  handleUserUnmuted,
  handleMuteVisibilityChanged,
  handleFollowListPublished,
  handleMuteListPublished,
  type SocialHandlerCallbacks,
  type FeedRefreshCallback,
  type RefilterCallback,
  type PrefetchProfileCallback
} from './SocialEventHandlers'

// Content Event Handlers
export {
  registerContentEventHandlers,
  unregisterContentEventHandlers,
  setContentHandlerCallbacks,
  clearContentHandlerCallbacks,
  handleEventBookmarked,
  handleEventUnbookmarked,
  handleBookmarkListPublished,
  handleNotePinned,
  handleNoteUnpinned,
  handlePinsLimitExceeded,
  handlePinListPublished,
  handleReactionAdded,
  handleContentReposted,
  type ContentHandlerCallbacks,
  type UpdateReactionCountCallback,
  type UpdateRepostCountCallback,
  type CreateNotificationCallback,
  type ShowToastCallback,
  type UpdateProfilePinsCallback
} from './ContentEventHandlers'

// Feed Event Handlers
export {
  registerFeedEventHandlers,
  unregisterFeedEventHandlers,
  handleFeedSwitched,
  handleContentFilterUpdated,
  handleFeedRefreshed,
  handleNoteCreated,
  handleNoteDeleted,
  handleNoteReplied,
  handleUsersMentioned,
  handleTimelineEventsReceived,
  handleTimelineEOSED
} from './FeedEventHandlers'

// Relay Event Handlers
export {
  registerRelayEventHandlers,
  unregisterRelayEventHandlers,
  handleFavoriteRelayAdded,
  handleFavoriteRelayRemoved,
  handleFavoriteRelaysPublished,
  handleRelaySetCreated,
  handleRelaySetUpdated,
  handleRelaySetDeleted,
  handleMailboxRelayAdded,
  handleMailboxRelayRemoved,
  handleMailboxRelayScopeChanged,
  handleRelayListPublished
} from './RelayEventHandlers'

/**
 * Initialize all domain event handlers
 *
 * Call this once during application startup to register all handlers
 * with the event dispatcher.
 */
export function initializeEventHandlers(): void {
  const { registerSocialEventHandlers } = require('./SocialEventHandlers')
  const { registerContentEventHandlers } = require('./ContentEventHandlers')
  const { registerFeedEventHandlers } = require('./FeedEventHandlers')
  const { registerRelayEventHandlers } = require('./RelayEventHandlers')

  registerSocialEventHandlers()
  registerContentEventHandlers()
  registerFeedEventHandlers()
  registerRelayEventHandlers()

  console.debug('[EventHandlers] All domain event handlers registered')
}

/**
 * Cleanup all domain event handlers
 *
 * Call this during application shutdown or for testing purposes.
 */
export function cleanupEventHandlers(): void {
  const { unregisterSocialEventHandlers, clearSocialHandlerCallbacks } = require('./SocialEventHandlers')
  const { unregisterContentEventHandlers, clearContentHandlerCallbacks } = require('./ContentEventHandlers')
  const { unregisterFeedEventHandlers } = require('./FeedEventHandlers')
  const { unregisterRelayEventHandlers } = require('./RelayEventHandlers')

  unregisterSocialEventHandlers()
  unregisterContentEventHandlers()
  unregisterFeedEventHandlers()
  unregisterRelayEventHandlers()

  clearSocialHandlerCallbacks()
  clearContentHandlerCallbacks()

  console.debug('[EventHandlers] All domain event handlers unregistered')
}
