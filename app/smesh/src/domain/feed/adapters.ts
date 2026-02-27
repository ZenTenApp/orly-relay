import { TFeedInfo, TFeedType } from '@/types'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'
import { Feed, FeedState } from './Feed'
import { FeedType } from './FeedType'

/**
 * Adapters for converting between Feed domain model and legacy types
 *
 * These adapters provide backward compatibility during the migration
 * from raw state management to domain-driven design.
 */

// ============================================================================
// FeedType Adapters
// ============================================================================

/**
 * Convert legacy TFeedType to domain FeedType
 */
export function toFeedType(feedType: TFeedType, id?: string): FeedType {
  switch (feedType) {
    case 'following':
      return FeedType.following()
    case 'pinned':
      return FeedType.pinned()
    case 'relays':
      if (!id) throw new Error('Relay set ID required for relays feed type')
      return FeedType.relays(id)
    case 'relay':
      if (!id) throw new Error('Relay URL required for relay feed type')
      return FeedType.relay(id)
    default:
      return FeedType.following()
  }
}

/**
 * Try to convert legacy TFeedType to domain FeedType
 * Returns null if conversion fails
 */
export function tryToFeedType(feedType: TFeedType, id?: string): FeedType | null {
  try {
    return toFeedType(feedType, id)
  } catch {
    return null
  }
}

/**
 * Convert domain FeedType to legacy TFeedType
 */
export function fromFeedType(feedType: FeedType): TFeedType {
  return feedType.value
}

// ============================================================================
// FeedInfo Adapters
// ============================================================================

/**
 * Convert legacy TFeedInfo to domain Feed aggregate
 */
export function toFeed(
  feedInfo: TFeedInfo,
  owner?: Pubkey,
  relayUrls?: RelayUrl[]
): Feed {
  if (!feedInfo) {
    return Feed.empty()
  }

  const feedType = tryToFeedType(feedInfo.feedType, feedInfo.id)
  if (!feedType) {
    return Feed.empty()
  }

  switch (feedInfo.feedType) {
    case 'following':
      return owner ? Feed.following(owner) : Feed.empty()
    case 'pinned':
      return owner ? Feed.pinned(owner) : Feed.empty()
    case 'relays':
      if (!owner || !feedInfo.id) return Feed.empty()
      return Feed.relays(owner, feedInfo.id, relayUrls ?? [])
    case 'relay':
      if (!feedInfo.id) return Feed.empty()
      const relayUrl = RelayUrl.tryCreate(feedInfo.id)
      if (!relayUrl) return Feed.empty()
      return Feed.singleRelay(relayUrl)
    default:
      return Feed.empty()
  }
}

/**
 * Convert domain Feed aggregate to legacy TFeedInfo
 */
export function fromFeed(feed: Feed): TFeedInfo {
  const feedType = feed.type

  if (feedType.value === 'following' || feedType.value === 'pinned') {
    return { feedType: feedType.value }
  }

  if (feedType.value === 'relays' && feedType.relaySetId) {
    return { feedType: 'relays', id: feedType.relaySetId }
  }

  if (feedType.value === 'relay' && feedType.relayUrl) {
    return { feedType: 'relay', id: feedType.relayUrl }
  }

  return null
}

// ============================================================================
// FeedState Adapters
// ============================================================================

/**
 * Convert legacy storage format to FeedState
 */
export function toFeedState(feedInfo: TFeedInfo, relayUrls: string[] = []): FeedState | null {
  if (!feedInfo) return null

  return {
    feedType: feedInfo.feedType,
    relaySetId: feedInfo.feedType === 'relays' ? feedInfo.id : undefined,
    relayUrl: feedInfo.feedType === 'relay' ? feedInfo.id : undefined,
    relayUrls,
    contentFilter: {
      hideMutedUsers: true,
      hideContentMentioningMuted: true,
      hideUntrustedUsers: false,
      hideReplies: false,
      hideReposts: false,
      allowedKinds: [],
      nsfwPolicy: 'hide_content'
    },
    lastRefreshedAt: undefined
  }
}

/**
 * Convert FeedState to legacy storage format
 */
export function fromFeedState(state: FeedState): { feedInfo: TFeedInfo; relayUrls: string[] } {
  let feedInfo: TFeedInfo = null

  if (state.feedType === 'following' || state.feedType === 'pinned') {
    feedInfo = { feedType: state.feedType as TFeedType }
  } else if (state.feedType === 'relays' && state.relaySetId) {
    feedInfo = { feedType: 'relays', id: state.relaySetId }
  } else if (state.feedType === 'relay' && state.relayUrl) {
    feedInfo = { feedType: 'relay', id: state.relayUrl }
  }

  return {
    feedInfo,
    relayUrls: state.relayUrls
  }
}

// ============================================================================
// Relay URL Adapters
// ============================================================================

/**
 * Convert string URLs to RelayUrl value objects
 * Filters out invalid URLs
 */
export function toRelayUrls(urls: string[]): RelayUrl[] {
  return urls
    .map((url) => RelayUrl.tryCreate(url))
    .filter((r): r is RelayUrl => r !== null)
}

/**
 * Convert RelayUrl value objects to strings
 */
export function fromRelayUrls(relayUrls: readonly RelayUrl[]): string[] {
  return relayUrls.map((r) => r.value)
}

// ============================================================================
// Comparison Utilities
// ============================================================================

/**
 * Check if two TFeedInfo objects represent the same feed
 */
export function isSameFeedInfo(a: TFeedInfo, b: TFeedInfo): boolean {
  if (a === null && b === null) return true
  if (a === null || b === null) return false
  if (a.feedType !== b.feedType) return false
  return a.id === b.id
}

/**
 * Check if a Feed matches a TFeedInfo
 */
export function feedMatchesInfo(feed: Feed, feedInfo: TFeedInfo): boolean {
  const converted = fromFeed(feed)
  return isSameFeedInfo(converted, feedInfo)
}
