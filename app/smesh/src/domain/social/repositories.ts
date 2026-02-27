import { Pubkey } from '../shared'
import { FollowList } from './FollowList'
import { MuteList } from './MuteList'
import { PinnedUsersList } from './PinnedUsersList'

/**
 * Repository interface for FollowList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - Event publishing
 */
export interface FollowListRepository {
  /**
   * Find the follow list for a user
   * Should check cache first, then fetch from relays if not found
   */
  findByOwner(pubkey: Pubkey): Promise<FollowList | null>

  /**
   * Save a follow list
   * Should publish to relays and update local cache
   */
  save(followList: FollowList): Promise<void>
}

/**
 * Repository interface for MuteList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - NIP-04 encryption/decryption for private mutes
 * - Event publishing
 */
export interface MuteListRepository {
  /**
   * Find the mute list for a user
   * Should check cache first, then fetch from relays if not found
   * Private mutes should be decrypted automatically
   */
  findByOwner(pubkey: Pubkey): Promise<MuteList | null>

  /**
   * Save a mute list
   * Should encrypt private mutes and publish to relays
   */
  save(muteList: MuteList): Promise<void>
}

/**
 * Repository interface for PinnedUsersList aggregate
 *
 * Implementations should handle:
 * - Local caching (IndexedDB)
 * - Remote fetching from relays
 * - NIP-04 encryption/decryption for private pins
 * - Event publishing
 */
export interface PinnedUsersListRepository {
  /**
   * Find the pinned users list for a user
   * Should check cache first, then fetch from relays if not found
   * Private pins should be decrypted automatically
   */
  findByOwner(pubkey: Pubkey): Promise<PinnedUsersList | null>

  /**
   * Save a pinned users list
   * Should encrypt private pins and publish to relays
   */
  save(pinnedUsersList: PinnedUsersList): Promise<void>
}
