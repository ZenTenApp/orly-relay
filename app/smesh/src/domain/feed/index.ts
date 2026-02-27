/**
 * Feed Bounded Context
 *
 * Handles timeline management, feed configuration, note composition,
 * and content filtering.
 */

// Aggregates
export { Feed, type FeedState, type FeedSwitchOptions, type TimelineQueryOptions } from './Feed'
export {
  NoteComposer,
  type NoteComposerOptions,
  type PollOption,
  type PollConfig,
  type ValidationResult,
  type ExtractedContent
} from './NoteComposer'

// Value Objects
export { FeedType, type FeedTypeValue } from './FeedType'
export {
  MediaAttachment,
  type MediaType,
  type UploadStatus,
  type ImageMetadata
} from './MediaAttachment'
export {
  Mention,
  MentionList,
  type MentionType
} from './Mention'
export {
  ContentFilter,
  type NsfwDisplayPolicy,
  type FilterContext,
  type FilterResult,
  type FilterReason
} from './ContentFilter'
export {
  RelayStrategy,
  type RelayStrategyType,
  type RelayListResolver
} from './RelayStrategy'
export {
  TimelineQuery,
  DEFAULT_TIMELINE_KINDS,
  DEFAULT_TIMELINE_LIMIT,
  type TimelineQueryParams
} from './TimelineQuery'
export { ReplyContext, type EventReference } from './ReplyContext'
export { QuoteContext } from './QuoteContext'

// Domain Services
export {
  FeedFilter,
  SimpleMuteChecker,
  SimpleTrustChecker,
  SimpleDeletionChecker,
  SimplePinnedChecker,
  type MuteChecker,
  type TrustChecker,
  type DeletionChecker,
  type PinnedChecker,
  type FilteredEvent,
  type FilterStats
} from './FeedFilter'

// Domain Events
export {
  // Feed configuration events
  FeedSwitched,
  ContentFilterUpdated,
  FeedRefreshed,
  // Note lifecycle events
  NoteCreated,
  NoteDeleted,
  NoteReplied,
  UsersMentioned,
  // Timeline events
  TimelineEventsReceived,
  TimelineEOSED,
  TimelineLoadedMore
} from './events'

// Repository Interfaces
export type {
  FeedRepository,
  TimelineRepository,
  TimelineSubscription,
  TimelineResult,
  TimelineEventCallback,
  TimelineEOSECallback,
  TimelineCacheRepository,
  DraftRepository,
  Draft,
  DraftMetadata
} from './repositories'

// Adapters for migration
export {
  // FeedType adapters
  toFeedType,
  tryToFeedType,
  fromFeedType,
  // Feed adapters
  toFeed,
  fromFeed,
  // FeedState adapters
  toFeedState,
  fromFeedState,
  // RelayUrl adapters
  toRelayUrls,
  fromRelayUrls,
  // Comparison utilities
  isSameFeedInfo,
  feedMatchesInfo
} from './adapters'
