import { Filter } from 'nostr-tools'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'
import { Timestamp } from '../shared/value-objects/Timestamp'
import { TFeedSubRequest } from '@/types'

/**
 * Parameters for creating a timeline query
 */
export interface TimelineQueryParams {
  relays: RelayUrl[]
  authors?: Pubkey[]
  kinds?: number[]
  since?: Timestamp
  until?: Timestamp
  limit?: number
  hashtags?: string[]
  mentionedPubkeys?: Pubkey[]
  eventIds?: string[]
}

/**
 * Default kinds for timeline queries
 */
export const DEFAULT_TIMELINE_KINDS = [1, 6, 16] // notes, reposts, generic reposts

/**
 * Default limit for timeline queries
 */
export const DEFAULT_TIMELINE_LIMIT = 50

/**
 * TimelineQuery Value Object
 *
 * Represents the parameters needed to subscribe to a timeline.
 * Immutable and self-validating.
 */
export class TimelineQuery {
  private constructor(
    private readonly _relays: readonly RelayUrl[],
    private readonly _authors: readonly Pubkey[],
    private readonly _kinds: readonly number[],
    private readonly _since: Timestamp | null,
    private readonly _until: Timestamp | null,
    private readonly _limit: number,
    private readonly _hashtags: readonly string[],
    private readonly _mentionedPubkeys: readonly Pubkey[],
    private readonly _eventIds: readonly string[]
  ) {}

  /**
   * Create a timeline query from parameters
   */
  static create(params: TimelineQueryParams): TimelineQuery {
    if (params.relays.length === 0) {
      throw new Error('TimelineQuery requires at least one relay')
    }

    return new TimelineQuery(
      [...params.relays],
      params.authors ? [...params.authors] : [],
      params.kinds ?? DEFAULT_TIMELINE_KINDS,
      params.since ?? null,
      params.until ?? null,
      params.limit ?? DEFAULT_TIMELINE_LIMIT,
      params.hashtags ?? [],
      params.mentionedPubkeys ?? [],
      params.eventIds ?? []
    )
  }

  /**
   * Create a query for a specific author's timeline
   */
  static forAuthor(author: Pubkey, relays: RelayUrl[], options?: {
    kinds?: number[]
    limit?: number
  }): TimelineQuery {
    return TimelineQuery.create({
      relays,
      authors: [author],
      kinds: options?.kinds,
      limit: options?.limit
    })
  }

  /**
   * Create a query for multiple authors (following feed)
   */
  static forAuthors(authors: Pubkey[], relays: RelayUrl[], options?: {
    kinds?: number[]
    limit?: number
  }): TimelineQuery {
    return TimelineQuery.create({
      relays,
      authors,
      kinds: options?.kinds,
      limit: options?.limit
    })
  }

  /**
   * Create a query for a global relay feed
   */
  static forRelay(relay: RelayUrl, options?: {
    kinds?: number[]
    limit?: number
  }): TimelineQuery {
    return TimelineQuery.create({
      relays: [relay],
      kinds: options?.kinds,
      limit: options?.limit
    })
  }

  /**
   * Create a query for a hashtag
   */
  static forHashtag(hashtag: string, relays: RelayUrl[], options?: {
    kinds?: number[]
    limit?: number
  }): TimelineQuery {
    return TimelineQuery.create({
      relays,
      hashtags: [hashtag.replace(/^#/, '')],
      kinds: options?.kinds,
      limit: options?.limit
    })
  }

  // Getters
  get relays(): readonly RelayUrl[] {
    return this._relays
  }

  get authors(): readonly Pubkey[] {
    return this._authors
  }

  get kinds(): readonly number[] {
    return this._kinds
  }

  get since(): Timestamp | null {
    return this._since
  }

  get until(): Timestamp | null {
    return this._until
  }

  get limit(): number {
    return this._limit
  }

  get hashtags(): readonly string[] {
    return this._hashtags
  }

  get mentionedPubkeys(): readonly Pubkey[] {
    return this._mentionedPubkeys
  }

  get eventIds(): readonly string[] {
    return this._eventIds
  }

  /**
   * Check if this query has author filters
   */
  get hasAuthors(): boolean {
    return this._authors.length > 0
  }

  /**
   * Check if this query is a global relay query (no author filters)
   */
  get isGlobalQuery(): boolean {
    return this._authors.length === 0 && this._hashtags.length === 0
  }

  /**
   * Convert to Nostr filter format
   */
  toNostrFilter(): Filter {
    const filter: Filter = {}

    if (this._authors.length > 0) {
      filter.authors = this._authors.map((a) => a.hex)
    }

    if (this._kinds.length > 0) {
      filter.kinds = [...this._kinds]
    }

    if (this._since) {
      filter.since = this._since.unix
    }

    if (this._until) {
      filter.until = this._until.unix
    }

    if (this._limit > 0) {
      filter.limit = this._limit
    }

    if (this._hashtags.length > 0) {
      filter['#t'] = [...this._hashtags]
    }

    if (this._mentionedPubkeys.length > 0) {
      filter['#p'] = this._mentionedPubkeys.map((p) => p.hex)
    }

    if (this._eventIds.length > 0) {
      filter.ids = [...this._eventIds]
    }

    return filter
  }

  /**
   * Convert to subscription request format used by the application
   */
  toSubRequests(): TFeedSubRequest[] {
    const filter = this.toNostrFilter()
    // Remove since/until as those are handled by the subscription manager
    const { since, until, ...filterWithoutTime } = filter

    return [
      {
        urls: this._relays.map((r) => r.value),
        filter: filterWithoutTime
      }
    ]
  }

  /**
   * Convert to multiple subscription requests (for per-relay optimization)
   */
  toSubRequestsPerRelay(): TFeedSubRequest[] {
    const filter = this.toNostrFilter()
    const { since, until, ...filterWithoutTime } = filter

    return this._relays.map((relay) => ({
      urls: [relay.value],
      filter: filterWithoutTime
    }))
  }

  // Immutable modification methods
  withRelays(relays: RelayUrl[]): TimelineQuery {
    return new TimelineQuery(
      [...relays],
      this._authors,
      this._kinds,
      this._since,
      this._until,
      this._limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withAuthors(authors: Pubkey[]): TimelineQuery {
    return new TimelineQuery(
      this._relays,
      [...authors],
      this._kinds,
      this._since,
      this._until,
      this._limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withKinds(kinds: number[]): TimelineQuery {
    return new TimelineQuery(
      this._relays,
      this._authors,
      [...kinds],
      this._since,
      this._until,
      this._limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withSince(since: Timestamp): TimelineQuery {
    return new TimelineQuery(
      this._relays,
      this._authors,
      this._kinds,
      since,
      this._until,
      this._limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withUntil(until: Timestamp): TimelineQuery {
    return new TimelineQuery(
      this._relays,
      this._authors,
      this._kinds,
      this._since,
      until,
      this._limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withLimit(limit: number): TimelineQuery {
    if (limit <= 0) {
      throw new Error('Limit must be positive')
    }
    return new TimelineQuery(
      this._relays,
      this._authors,
      this._kinds,
      this._since,
      this._until,
      limit,
      this._hashtags,
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  withHashtags(hashtags: string[]): TimelineQuery {
    return new TimelineQuery(
      this._relays,
      this._authors,
      this._kinds,
      this._since,
      this._until,
      this._limit,
      hashtags.map((t) => t.replace(/^#/, '')),
      this._mentionedPubkeys,
      this._eventIds
    )
  }

  /**
   * Generate a cache key for this query
   */
  toCacheKey(): string {
    const parts = [
      this._relays
        .map((r) => r.value)
        .sort()
        .join(','),
      this._authors
        .map((a) => a.hex)
        .sort()
        .join(','),
      [...this._kinds].sort().join(','),
      [...this._hashtags].sort().join(',')
    ]
    return parts.join('|')
  }

  equals(other: TimelineQuery): boolean {
    if (this._limit !== other._limit) return false
    if (this._relays.length !== other._relays.length) return false
    if (this._authors.length !== other._authors.length) return false
    if (this._kinds.length !== other._kinds.length) return false
    if (this._hashtags.length !== other._hashtags.length) return false

    // Compare relays
    const thisRelaySet = new Set(this._relays.map((r) => r.value))
    for (const relay of other._relays) {
      if (!thisRelaySet.has(relay.value)) return false
    }

    // Compare authors
    const thisAuthorSet = new Set(this._authors.map((a) => a.hex))
    for (const author of other._authors) {
      if (!thisAuthorSet.has(author.hex)) return false
    }

    // Compare kinds
    const thisKindSet = new Set(this._kinds)
    for (const kind of other._kinds) {
      if (!thisKindSet.has(kind)) return false
    }

    // Compare hashtags
    const thisHashtagSet = new Set(this._hashtags)
    for (const hashtag of other._hashtags) {
      if (!thisHashtagSet.has(hashtag)) return false
    }

    return true
  }
}
