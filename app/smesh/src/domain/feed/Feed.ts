import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'
import { Timestamp } from '../shared/value-objects/Timestamp'
import { FeedType } from './FeedType'
import { ContentFilter } from './ContentFilter'
import { RelayStrategy } from './RelayStrategy'
import { TimelineQuery } from './TimelineQuery'
import { FeedSwitched, ContentFilterUpdated, FeedRefreshed } from './events'

/**
 * Options for switching feeds
 */
export interface FeedSwitchOptions {
  relaySetId?: string
  relayUrl?: string
}

/**
 * Options for building timeline queries
 */
export interface TimelineQueryOptions {
  authors?: Pubkey[]
  kinds?: number[]
  limit?: number
}

/**
 * Serializable state for persistence
 */
export interface FeedState {
  feedType: string
  relaySetId?: string
  relayUrl?: string
  relayUrls: string[]
  contentFilter: {
    hideMutedUsers: boolean
    hideContentMentioningMuted: boolean
    hideUntrustedUsers: boolean
    hideReplies: boolean
    hideReposts: boolean
    allowedKinds: number[]
    nsfwPolicy: string
  }
  lastRefreshedAt?: number
}

/**
 * Feed Aggregate
 *
 * Represents the user's active feed configuration and state.
 * This is the aggregate root for the Feed bounded context's query side.
 *
 * Invariants:
 * - Must have a valid feed type
 * - For relay feeds, must have resolved relay URLs
 * - Content filter is always present with sensible defaults
 */
export class Feed {
  private constructor(
    private readonly _owner: Pubkey | null,
    private _feedType: FeedType,
    private _relayStrategy: RelayStrategy,
    private _resolvedRelayUrls: RelayUrl[],
    private _contentFilter: ContentFilter,
    private _lastRefreshedAt: Timestamp | null
  ) {}

  // ============================================================================
  // Factory Methods
  // ============================================================================

  /**
   * Create a following feed (shows posts from followed users)
   */
  static following(owner: Pubkey): Feed {
    return new Feed(
      owner,
      FeedType.following(),
      RelayStrategy.authorWriteRelays(),
      [],
      ContentFilter.default(),
      null
    )
  }

  /**
   * Create a pinned users feed
   */
  static pinned(owner: Pubkey): Feed {
    return new Feed(
      owner,
      FeedType.pinned(),
      RelayStrategy.authorWriteRelays(),
      [],
      ContentFilter.default(),
      null
    )
  }

  /**
   * Create a relay set feed
   */
  static relays(owner: Pubkey, setId: string, relayUrls: RelayUrl[]): Feed {
    return new Feed(
      owner,
      FeedType.relays(setId),
      RelayStrategy.specific(relayUrls, setId),
      relayUrls,
      ContentFilter.default(),
      null
    )
  }

  /**
   * Create a single relay feed
   */
  static singleRelay(relayUrl: RelayUrl): Feed {
    return new Feed(
      null,
      FeedType.relay(relayUrl.value),
      RelayStrategy.single(relayUrl),
      [relayUrl],
      ContentFilter.default(),
      null
    )
  }

  /**
   * Create an empty/uninitialized feed
   */
  static empty(): Feed {
    return new Feed(
      null,
      FeedType.following(),
      RelayStrategy.bigRelays(),
      [],
      ContentFilter.default(),
      null
    )
  }

  /**
   * Restore from persisted state
   */
  static fromState(state: FeedState, owner?: Pubkey): Feed {
    const feedType = FeedType.tryFromString(
      state.feedType,
      state.relaySetId ?? state.relayUrl
    )

    if (!feedType) {
      return Feed.empty()
    }

    const relayUrls = state.relayUrls
      .map((url) => RelayUrl.tryCreate(url))
      .filter((r): r is RelayUrl => r !== null)

    let relayStrategy: RelayStrategy
    if (feedType.value === 'relay' && relayUrls.length > 0) {
      relayStrategy = RelayStrategy.single(relayUrls[0])
    } else if (feedType.value === 'relays' && relayUrls.length > 0) {
      relayStrategy = RelayStrategy.specific(relayUrls, state.relaySetId)
    } else if (feedType.isSocialFeed) {
      relayStrategy = RelayStrategy.authorWriteRelays()
    } else {
      relayStrategy = RelayStrategy.bigRelays()
    }

    const contentFilter = ContentFilter.fromPreferences({
      hideMutedUsers: state.contentFilter.hideMutedUsers,
      hideContentMentioningMuted: state.contentFilter.hideContentMentioningMuted,
      hideUntrustedUsers: state.contentFilter.hideUntrustedUsers,
      hideReplies: state.contentFilter.hideReplies,
      hideReposts: state.contentFilter.hideReposts,
      allowedKinds: state.contentFilter.allowedKinds,
      nsfwPolicy: state.contentFilter.nsfwPolicy as 'hide' | 'hide_content' | 'show'
    })

    return new Feed(
      owner ?? null,
      feedType,
      relayStrategy,
      relayUrls,
      contentFilter,
      state.lastRefreshedAt ? Timestamp.fromUnix(state.lastRefreshedAt) : null
    )
  }

  // ============================================================================
  // Queries
  // ============================================================================

  get owner(): Pubkey | null {
    return this._owner
  }

  get type(): FeedType {
    return this._feedType
  }

  get relayStrategy(): RelayStrategy {
    return this._relayStrategy
  }

  get relayUrls(): readonly RelayUrl[] {
    return this._resolvedRelayUrls
  }

  get contentFilter(): ContentFilter {
    return this._contentFilter
  }

  get lastRefreshedAt(): Timestamp | null {
    return this._lastRefreshedAt
  }

  /**
   * Check if this is a social feed (following or pinned)
   */
  get isSocialFeed(): boolean {
    return this._feedType.isSocialFeed
  }

  /**
   * Check if this is a relay-based feed
   */
  get isRelayFeed(): boolean {
    return this._feedType.isRelayFeed
  }

  /**
   * Check if the feed has resolved relay URLs
   */
  get hasRelayUrls(): boolean {
    return this._resolvedRelayUrls.length > 0
  }

  /**
   * Get relay URLs as strings for compatibility
   */
  get relayUrlStrings(): string[] {
    return this._resolvedRelayUrls.map((r) => r.value)
  }

  // ============================================================================
  // Commands
  // ============================================================================

  /**
   * Switch to a different feed type
   * Returns a domain event describing the change
   */
  switchTo(newType: FeedType, relayUrls: RelayUrl[] = []): FeedSwitched {
    const previousType = this._feedType

    this._feedType = newType

    // Update relay strategy based on new type
    if (newType.value === 'relay' && relayUrls.length > 0) {
      this._relayStrategy = RelayStrategy.single(relayUrls[0])
      this._resolvedRelayUrls = [relayUrls[0]]
    } else if (newType.value === 'relays' && relayUrls.length > 0) {
      this._relayStrategy = RelayStrategy.specific(relayUrls, newType.relaySetId ?? undefined)
      this._resolvedRelayUrls = relayUrls
    } else if (newType.isSocialFeed) {
      this._relayStrategy = RelayStrategy.authorWriteRelays()
      this._resolvedRelayUrls = []
    } else {
      this._relayStrategy = RelayStrategy.bigRelays()
      this._resolvedRelayUrls = []
    }

    this._lastRefreshedAt = Timestamp.now()

    return new FeedSwitched(
      this._owner,
      previousType,
      newType,
      newType.relaySetId ?? undefined
    )
  }

  /**
   * Update the resolved relay URLs (after resolution)
   */
  setResolvedRelayUrls(urls: RelayUrl[]): void {
    this._resolvedRelayUrls = [...urls]
  }

  /**
   * Update content filter settings
   * Returns a domain event describing the change
   */
  updateContentFilter(newFilter: ContentFilter): ContentFilterUpdated {
    const previousFilter = this._contentFilter
    this._contentFilter = newFilter

    return new ContentFilterUpdated(
      this._owner!,
      previousFilter,
      newFilter
    )
  }

  /**
   * Mark the feed as refreshed
   * Returns a domain event
   */
  refresh(): FeedRefreshed {
    this._lastRefreshedAt = Timestamp.now()

    return new FeedRefreshed(this._owner, this._feedType)
  }

  // ============================================================================
  // Timeline Query Building
  // ============================================================================

  /**
   * Build a timeline query for this feed configuration
   *
   * For social feeds, authors should be provided (followings or pinned users).
   * For relay feeds, the resolved relay URLs are used.
   */
  buildTimelineQuery(options: TimelineQueryOptions = {}): TimelineQuery | null {
    // Need relay URLs to build a query
    if (this._resolvedRelayUrls.length === 0) {
      return null
    }

    if (this.isSocialFeed) {
      // Social feeds need authors
      if (!options.authors || options.authors.length === 0) {
        return null
      }

      return TimelineQuery.forAuthors(
        options.authors,
        this._resolvedRelayUrls,
        {
          kinds: options.kinds,
          limit: options.limit
        }
      )
    }

    // Relay feeds - global query
    return TimelineQuery.forRelay(
      this._resolvedRelayUrls[0],
      {
        kinds: options.kinds,
        limit: options.limit
      }
    ).withRelays(this._resolvedRelayUrls)
  }

  // ============================================================================
  // Persistence
  // ============================================================================

  /**
   * Convert to serializable state for persistence
   */
  toState(): FeedState {
    return {
      feedType: this._feedType.value,
      relaySetId: this._feedType.relaySetId ?? undefined,
      relayUrl: this._feedType.relayUrl ?? undefined,
      relayUrls: this._resolvedRelayUrls.map((r) => r.value),
      contentFilter: {
        hideMutedUsers: this._contentFilter.hideMutedUsers,
        hideContentMentioningMuted: this._contentFilter.hideContentMentioningMuted,
        hideUntrustedUsers: this._contentFilter.hideUntrustedUsers,
        hideReplies: this._contentFilter.hideReplies,
        hideReposts: this._contentFilter.hideReposts,
        allowedKinds: [...this._contentFilter.allowedKinds],
        nsfwPolicy: this._contentFilter.nsfwPolicy
      },
      lastRefreshedAt: this._lastRefreshedAt?.unix
    }
  }

  /**
   * Create a copy of this feed with a new owner
   */
  withOwner(owner: Pubkey): Feed {
    return new Feed(
      owner,
      this._feedType,
      this._relayStrategy,
      [...this._resolvedRelayUrls],
      this._contentFilter,
      this._lastRefreshedAt
    )
  }

  /**
   * Check equality with another feed
   */
  equals(other: Feed): boolean {
    if (!this._feedType.equals(other._feedType)) return false
    if (this._resolvedRelayUrls.length !== other._resolvedRelayUrls.length) return false

    for (let i = 0; i < this._resolvedRelayUrls.length; i++) {
      if (!this._resolvedRelayUrls[i].equals(other._resolvedRelayUrls[i])) return false
    }

    return this._contentFilter.equals(other._contentFilter)
  }
}
