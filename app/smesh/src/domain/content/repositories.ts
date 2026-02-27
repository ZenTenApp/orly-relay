import { Pubkey } from '../shared'
import { BookmarkList } from './BookmarkList'
import { PinList } from './PinList'

/**
 * Repository interface for BookmarkList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - Event publishing
 */
export interface BookmarkListRepository {
  /**
   * Find the bookmark list for a user
   * Should check cache first, then fetch from relays if not found
   */
  findByOwner(pubkey: Pubkey): Promise<BookmarkList | null>

  /**
   * Save a bookmark list
   * Should publish to relays and update local cache
   */
  save(bookmarkList: BookmarkList): Promise<void>
}

/**
 * Repository interface for PinList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - Event publishing
 */
export interface PinListRepository {
  /**
   * Find the pin list for a user
   * Should check cache first, then fetch from relays if not found
   */
  findByOwner(pubkey: Pubkey): Promise<PinList | null>

  /**
   * Save a pin list
   * Should publish to relays and update local cache
   */
  save(pinList: PinList): Promise<void>
}
