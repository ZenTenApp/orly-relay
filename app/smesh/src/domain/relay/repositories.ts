import { Pubkey } from '../shared'
import { RelayList } from './RelayList'
import { RelaySet } from './RelaySet'
import { FavoriteRelays } from './FavoriteRelays'

/**
 * Repository interface for RelayList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - Event publishing
 */
export interface RelayListRepository {
  /**
   * Find the relay list for a user
   */
  findByOwner(pubkey: Pubkey): Promise<RelayList | null>

  /**
   * Save a relay list
   */
  save(relayList: RelayList): Promise<void>
}

/**
 * Repository interface for RelaySet aggregate
 */
export interface RelaySetRepository {
  /**
   * Find a relay set by owner and ID
   */
  findById(pubkey: Pubkey, id: string): Promise<RelaySet | null>

  /**
   * Find all relay sets for a user
   */
  findByOwner(pubkey: Pubkey): Promise<RelaySet[]>

  /**
   * Save a relay set
   */
  save(pubkey: Pubkey, relaySet: RelaySet): Promise<void>

  /**
   * Delete a relay set
   */
  delete(pubkey: Pubkey, id: string): Promise<void>
}

/**
 * Repository interface for FavoriteRelays aggregate
 */
export interface FavoriteRelaysRepository {
  /**
   * Find the favorite relays for a user
   */
  findByOwner(pubkey: Pubkey): Promise<FavoriteRelays | null>

  /**
   * Save favorite relays
   */
  save(favoriteRelays: FavoriteRelays): Promise<void>
}
