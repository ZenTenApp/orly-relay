import {
  UserFollowed,
  UserUnfollowed,
  UserMuted,
  UserUnmuted,
  MuteVisibilityChanged,
  FollowListPublished,
  MuteListPublished
} from '@/domain/social/events'
import { EventHandler, eventDispatcher } from '@/domain/shared'

/**
 * Handlers for social domain events
 *
 * These handlers coordinate cross-context updates when social events occur.
 * They bridge the Social context with Feed, Notification, and Cache contexts.
 */

/**
 * Callback type for feed refresh requests
 */
export type FeedRefreshCallback = () => void

/**
 * Callback type for content refiltering requests
 */
export type RefilterCallback = () => void

/**
 * Callback type for profile prefetch requests
 */
export type PrefetchProfileCallback = (pubkey: string) => void

/**
 * Service callbacks that can be injected for cross-context coordination
 */
export interface SocialHandlerCallbacks {
  onFeedRefreshNeeded?: FeedRefreshCallback
  onRefilterNeeded?: RefilterCallback
  onPrefetchProfile?: PrefetchProfileCallback
}

let callbacks: SocialHandlerCallbacks = {}

/**
 * Set the callbacks for cross-context coordination
 * Call this during provider initialization
 */
export function setSocialHandlerCallbacks(newCallbacks: SocialHandlerCallbacks): void {
  callbacks = { ...callbacks, ...newCallbacks }
}

/**
 * Clear all callbacks (for cleanup/testing)
 */
export function clearSocialHandlerCallbacks(): void {
  callbacks = {}
}

/**
 * Handler for user followed events
 * Coordinates with:
 * - Feed context: Add followed user's content to timeline
 * - Cache context: Prefetch followed user's profile and notes
 */
export const handleUserFollowed: EventHandler<UserFollowed> = async (event) => {
  console.debug('[SocialEventHandler] User followed:', {
    actor: event.actor.formatted,
    followed: event.followed.formatted,
    petname: event.petname
  })

  // Prefetch the followed user's profile for better UX
  if (callbacks.onPrefetchProfile) {
    callbacks.onPrefetchProfile(event.followed.hex)
  }
}

/**
 * Handler for user unfollowed events
 * Can be used to:
 * - Update feed context to exclude unfollowed user's content
 * - Clean up cached data for unfollowed user
 */
export const handleUserUnfollowed: EventHandler<UserUnfollowed> = async (event) => {
  console.debug('[SocialEventHandler] User unfollowed:', {
    actor: event.actor.formatted,
    unfollowed: event.unfollowed.formatted
  })

  // Future: Dispatch to feed context to update content sources
}

/**
 * Handler for user muted events
 * Coordinates with:
 * - Feed context: Refilter timeline to hide muted user's content
 * - Notification context: Filter notifications from muted user
 * - DM context: Update DM filtering
 */
export const handleUserMuted: EventHandler<UserMuted> = async (event) => {
  console.debug('[SocialEventHandler] User muted:', {
    actor: event.actor.formatted,
    muted: event.muted.formatted,
    visibility: event.visibility
  })

  // Trigger immediate refiltering of current timeline
  if (callbacks.onRefilterNeeded) {
    callbacks.onRefilterNeeded()
  }
}

/**
 * Handler for user unmuted events
 * Coordinates with:
 * - Feed context: Refilter timeline to show unmuted user's content
 * - Notification context: Restore notifications from unmuted user
 */
export const handleUserUnmuted: EventHandler<UserUnmuted> = async (event) => {
  console.debug('[SocialEventHandler] User unmuted:', {
    actor: event.actor.formatted,
    unmuted: event.unmuted.formatted
  })

  // Trigger refiltering to restore unmuted user's content
  if (callbacks.onRefilterNeeded) {
    callbacks.onRefilterNeeded()
  }
}

/**
 * Handler for mute visibility changed events
 */
export const handleMuteVisibilityChanged: EventHandler<MuteVisibilityChanged> = async (event) => {
  console.debug('[SocialEventHandler] Mute visibility changed:', {
    actor: event.actor.formatted,
    target: event.target.formatted,
    from: event.from,
    to: event.to
  })
}

/**
 * Handler for follow list published events
 * Coordinates with:
 * - Feed context: Refresh following feed with new list
 * - Cache context: Invalidate author caches
 */
export const handleFollowListPublished: EventHandler<FollowListPublished> = async (event) => {
  console.debug('[SocialEventHandler] Follow list published:', {
    owner: event.owner.formatted,
    followingCount: event.followingCount
  })

  // Trigger feed refresh to reflect new following list
  if (callbacks.onFeedRefreshNeeded) {
    callbacks.onFeedRefreshNeeded()
  }
}

/**
 * Handler for mute list published events
 * Coordinates with:
 * - Feed context: Refilter timeline with new mute list
 * - Notification context: Update notification filtering
 */
export const handleMuteListPublished: EventHandler<MuteListPublished> = async (event) => {
  console.debug('[SocialEventHandler] Mute list published:', {
    owner: event.owner.formatted,
    publicMuteCount: event.publicMuteCount,
    privateMuteCount: event.privateMuteCount
  })

  // Trigger refiltering with updated mute list
  if (callbacks.onRefilterNeeded) {
    callbacks.onRefilterNeeded()
  }
}

/**
 * Register all social event handlers with the event dispatcher
 */
export function registerSocialEventHandlers(): void {
  eventDispatcher.on('social.user_followed', handleUserFollowed)
  eventDispatcher.on('social.user_unfollowed', handleUserUnfollowed)
  eventDispatcher.on('social.user_muted', handleUserMuted)
  eventDispatcher.on('social.user_unmuted', handleUserUnmuted)
  eventDispatcher.on('social.mute_visibility_changed', handleMuteVisibilityChanged)
  eventDispatcher.on('social.follow_list_published', handleFollowListPublished)
  eventDispatcher.on('social.mute_list_published', handleMuteListPublished)
}

/**
 * Unregister all social event handlers
 */
export function unregisterSocialEventHandlers(): void {
  eventDispatcher.off('social.user_followed', handleUserFollowed)
  eventDispatcher.off('social.user_unfollowed', handleUserUnfollowed)
  eventDispatcher.off('social.user_muted', handleUserMuted)
  eventDispatcher.off('social.user_unmuted', handleUserUnmuted)
  eventDispatcher.off('social.mute_visibility_changed', handleMuteVisibilityChanged)
  eventDispatcher.off('social.follow_list_published', handleFollowListPublished)
  eventDispatcher.off('social.mute_list_published', handleMuteListPublished)
}
