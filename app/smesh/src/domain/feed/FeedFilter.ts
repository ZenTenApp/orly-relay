import { Event } from 'nostr-tools'
import { ContentFilter, FilterContext, FilterResult, FilterReason } from './ContentFilter'

/**
 * Interface for checking if a pubkey is muted
 */
export interface MuteChecker {
  isMuted(pubkey: string): boolean
  getMutedPubkeys(): Set<string>
}

/**
 * Interface for checking if a pubkey is trusted
 */
export interface TrustChecker {
  isTrusted(pubkey: string): boolean
  getTrustedPubkeys(): Set<string>
}

/**
 * Interface for checking if an event is deleted
 */
export interface DeletionChecker {
  isDeleted(eventId: string): boolean
  getDeletedEventIds(): Set<string>
}

/**
 * Interface for checking if an event is pinned
 */
export interface PinnedChecker {
  isPinned(eventId: string): boolean
  getPinnedEventIds(): Set<string>
}

/**
 * Result of filtering with the original event
 */
export interface FilteredEvent {
  event: Event
  result: FilterResult
}

/**
 * Statistics about filtering results
 */
export interface FilterStats {
  total: number
  shown: number
  hidden: number
  byReason: Map<FilterReason, number>
}

/**
 * FeedFilter Domain Service
 *
 * Coordinates filtering of timeline events using ContentFilter and various
 * checkers (mute, trust, deletion). This is a domain service because it
 * requires coordination between multiple domain concepts.
 *
 * Usage:
 * - Inject checkers that provide mute/trust/deletion data
 * - Call filterEvents() to filter a batch of events
 * - Call shouldDisplay() to check a single event
 */
export class FeedFilter {
  constructor(
    private readonly muteChecker: MuteChecker,
    private readonly trustChecker?: TrustChecker,
    private readonly deletionChecker?: DeletionChecker,
    private readonly pinnedChecker?: PinnedChecker,
    private readonly currentUserPubkey?: string
  ) {}

  /**
   * Create a filter context from the current checker state
   */
  private buildContext(): FilterContext {
    return {
      mutedPubkeys: this.muteChecker.getMutedPubkeys(),
      trustedPubkeys: this.trustChecker?.getTrustedPubkeys(),
      deletedEventIds: this.deletionChecker?.getDeletedEventIds(),
      pinnedEventIds: this.pinnedChecker?.getPinnedEventIds(),
      currentUserPubkey: this.currentUserPubkey
    }
  }

  /**
   * Filter a batch of events, returning only those that should be shown
   */
  filterEvents(events: Event[], filter: ContentFilter): Event[] {
    const context = this.buildContext()
    return events.filter((event) => filter.shouldShow(event, context).shouldShow)
  }

  /**
   * Filter events and return both shown and hidden with reasons
   */
  filterEventsWithDetails(events: Event[], filter: ContentFilter): FilteredEvent[] {
    const context = this.buildContext()
    return events.map((event) => ({
      event,
      result: filter.shouldShow(event, context)
    }))
  }

  /**
   * Get only events that should be shown with their filter results
   */
  getShownEvents(events: Event[], filter: ContentFilter): FilteredEvent[] {
    return this.filterEventsWithDetails(events, filter).filter((fe) => fe.result.shouldShow)
  }

  /**
   * Get only events that were hidden with their reasons
   */
  getHiddenEvents(events: Event[], filter: ContentFilter): FilteredEvent[] {
    return this.filterEventsWithDetails(events, filter).filter((fe) => !fe.result.shouldShow)
  }

  /**
   * Check if a single event should be displayed
   */
  shouldDisplay(event: Event, filter: ContentFilter): FilterResult {
    const context = this.buildContext()
    return filter.shouldShow(event, context)
  }

  /**
   * Get statistics about filtering a batch of events
   */
  getFilterStats(events: Event[], filter: ContentFilter): FilterStats {
    const results = this.filterEventsWithDetails(events, filter)
    const byReason = new Map<FilterReason, number>()

    let shown = 0
    let hidden = 0

    for (const { result } of results) {
      if (result.shouldShow) {
        shown++
      } else {
        hidden++
        if (result.reason) {
          byReason.set(result.reason, (byReason.get(result.reason) ?? 0) + 1)
        }
      }
    }

    return {
      total: events.length,
      shown,
      hidden,
      byReason
    }
  }

  /**
   * Create a new FeedFilter with an updated mute checker
   */
  withMuteChecker(muteChecker: MuteChecker): FeedFilter {
    return new FeedFilter(
      muteChecker,
      this.trustChecker,
      this.deletionChecker,
      this.pinnedChecker,
      this.currentUserPubkey
    )
  }

  /**
   * Create a new FeedFilter with an updated trust checker
   */
  withTrustChecker(trustChecker: TrustChecker): FeedFilter {
    return new FeedFilter(
      this.muteChecker,
      trustChecker,
      this.deletionChecker,
      this.pinnedChecker,
      this.currentUserPubkey
    )
  }

  /**
   * Create a new FeedFilter with an updated deletion checker
   */
  withDeletionChecker(deletionChecker: DeletionChecker): FeedFilter {
    return new FeedFilter(
      this.muteChecker,
      this.trustChecker,
      deletionChecker,
      this.pinnedChecker,
      this.currentUserPubkey
    )
  }

  /**
   * Create a new FeedFilter with an updated pinned checker
   */
  withPinnedChecker(pinnedChecker: PinnedChecker): FeedFilter {
    return new FeedFilter(
      this.muteChecker,
      this.trustChecker,
      this.deletionChecker,
      pinnedChecker,
      this.currentUserPubkey
    )
  }

  /**
   * Create a new FeedFilter with an updated current user
   */
  withCurrentUser(pubkey: string): FeedFilter {
    return new FeedFilter(
      this.muteChecker,
      this.trustChecker,
      this.deletionChecker,
      this.pinnedChecker,
      pubkey
    )
  }
}

/**
 * Simple in-memory implementation of MuteChecker for testing
 */
export class SimpleMuteChecker implements MuteChecker {
  constructor(private readonly mutedPubkeys: Set<string> = new Set()) {}

  isMuted(pubkey: string): boolean {
    return this.mutedPubkeys.has(pubkey)
  }

  getMutedPubkeys(): Set<string> {
    return this.mutedPubkeys
  }
}

/**
 * Simple in-memory implementation of TrustChecker for testing
 */
export class SimpleTrustChecker implements TrustChecker {
  constructor(private readonly trustedPubkeys: Set<string> = new Set()) {}

  isTrusted(pubkey: string): boolean {
    return this.trustedPubkeys.has(pubkey)
  }

  getTrustedPubkeys(): Set<string> {
    return this.trustedPubkeys
  }
}

/**
 * Simple in-memory implementation of DeletionChecker for testing
 */
export class SimpleDeletionChecker implements DeletionChecker {
  constructor(private readonly deletedEventIds: Set<string> = new Set()) {}

  isDeleted(eventId: string): boolean {
    return this.deletedEventIds.has(eventId)
  }

  getDeletedEventIds(): Set<string> {
    return this.deletedEventIds
  }
}

/**
 * Simple in-memory implementation of PinnedChecker for testing
 */
export class SimplePinnedChecker implements PinnedChecker {
  constructor(private readonly pinnedEventIds: Set<string> = new Set()) {}

  isPinned(eventId: string): boolean {
    return this.pinnedEventIds.has(eventId)
  }

  getPinnedEventIds(): Set<string> {
    return this.pinnedEventIds
  }
}
