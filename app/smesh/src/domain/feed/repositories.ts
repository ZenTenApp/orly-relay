import { Event } from 'nostr-tools'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { Feed, FeedState } from './Feed'
import { TimelineQuery } from './TimelineQuery'

/**
 * Repository for persisting Feed configuration
 *
 * The Feed aggregate represents user preferences for feed display,
 * not the actual timeline events. This repository handles persistence
 * of feed configuration state.
 */
export interface FeedRepository {
  /**
   * Get the feed configuration for a user
   * Returns null if no saved configuration exists
   */
  getByOwner(owner: Pubkey): Promise<Feed | null>

  /**
   * Save the current feed configuration
   */
  save(feed: Feed): Promise<void>

  /**
   * Delete saved feed configuration for a user
   */
  delete(owner: Pubkey): Promise<void>

  /**
   * Get the raw state for a user (for debugging/migration)
   */
  getState(owner: Pubkey): Promise<FeedState | null>
}

/**
 * Result of a timeline query
 */
export interface TimelineResult {
  events: Event[]
  eose: boolean
  queryId: string
}

/**
 * Subscription handle for timeline updates
 */
export interface TimelineSubscription {
  readonly queryId: string

  /**
   * Close this subscription
   */
  close(): void

  /**
   * Check if subscription is still active
   */
  isActive(): boolean
}

/**
 * Callback for timeline event updates
 */
export type TimelineEventCallback = (event: Event) => void

/**
 * Callback for end-of-stored-events signal
 */
export type TimelineEOSECallback = (queryId: string) => void

/**
 * Repository for fetching timeline events
 *
 * This repository handles the query side - fetching events from relays
 * based on timeline queries. It's responsible for relay communication
 * but not event storage (that's handled by event caching infrastructure).
 */
export interface TimelineRepository {
  /**
   * Subscribe to a timeline query
   * Returns a subscription handle for management
   */
  subscribe(
    query: TimelineQuery,
    onEvent: TimelineEventCallback,
    onEOSE?: TimelineEOSECallback
  ): TimelineSubscription

  /**
   * Fetch events matching a query (one-shot)
   * Waits for EOSE from all relays before returning
   */
  fetch(query: TimelineQuery): Promise<TimelineResult>

  /**
   * Fetch newer events than the most recent in the query
   * Useful for refreshing a timeline
   */
  fetchNewer(query: TimelineQuery, since: number): Promise<TimelineResult>

  /**
   * Fetch older events for pagination
   */
  fetchOlder(query: TimelineQuery, until: number): Promise<TimelineResult>

  /**
   * Close all active subscriptions
   */
  closeAll(): void
}

/**
 * Repository for accessing cached timeline events
 *
 * Separate from TimelineRepository to allow for different caching strategies
 * and to keep the query/fetch concerns separate from storage.
 */
export interface TimelineCacheRepository {
  /**
   * Get cached events for a query
   * Returns events sorted by created_at descending
   */
  getEvents(queryKey: string, limit?: number): Promise<Event[]>

  /**
   * Store events in cache
   */
  storeEvents(queryKey: string, events: Event[]): Promise<void>

  /**
   * Get the most recent event timestamp for a query
   * Used to determine the "since" for refresh queries
   */
  getMostRecentTimestamp(queryKey: string): Promise<number | null>

  /**
   * Get the oldest event timestamp for a query
   * Used to determine the "until" for pagination queries
   */
  getOldestTimestamp(queryKey: string): Promise<number | null>

  /**
   * Clear cache for a specific query
   */
  clearCache(queryKey: string): Promise<void>

  /**
   * Clear all cached timeline data
   */
  clearAll(): Promise<void>

  /**
   * Merge new events with existing cache
   * Handles deduplication and sorting
   */
  mergeEvents(queryKey: string, newEvents: Event[]): Promise<Event[]>
}

/**
 * Repository for draft note storage
 *
 * Handles persistence of unsent notes for recovery.
 */
export interface DraftRepository {
  /**
   * Save a draft note
   */
  save(draftId: string, content: string, metadata: DraftMetadata): Promise<void>

  /**
   * Get a specific draft
   */
  get(draftId: string): Promise<Draft | null>

  /**
   * Get all drafts for a user
   */
  getAll(owner: Pubkey): Promise<Draft[]>

  /**
   * Delete a draft
   */
  delete(draftId: string): Promise<void>

  /**
   * Clear all drafts for a user
   */
  clearAll(owner: Pubkey): Promise<void>
}

/**
 * Metadata for a draft note
 */
export interface DraftMetadata {
  owner: Pubkey
  createdAt: number
  updatedAt: number
  replyToEventId?: string
  quoteEventId?: string
}

/**
 * A saved draft note
 */
export interface Draft {
  id: string
  content: string
  metadata: DraftMetadata
}
