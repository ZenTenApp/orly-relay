/**
 * Persistence Infrastructure Layer
 *
 * Repository implementations using IndexedDB for local caching
 * and the client service for relay communication.
 */

// Types
export type { PublishFn, RepositoryDependencies } from './types'

// Social context repositories
export { FollowListRepositoryImpl } from './FollowListRepositoryImpl'
export { MuteListRepositoryImpl } from './MuteListRepositoryImpl'
export type { MuteListRepositoryDependencies, DecryptFn, EncryptFn } from './MuteListRepositoryImpl'
export { PinnedUsersListRepositoryImpl } from './PinnedUsersListRepositoryImpl'
export type { PinnedUsersListRepositoryDependencies } from './PinnedUsersListRepositoryImpl'

// Relay context repositories
export { RelayListRepositoryImpl } from './RelayListRepositoryImpl'
export { RelaySetRepositoryImpl } from './RelaySetRepositoryImpl'
export { FavoriteRelaysRepositoryImpl } from './FavoriteRelaysRepositoryImpl'

// Content context repositories
export { BookmarkListRepositoryImpl } from './BookmarkListRepositoryImpl'
export { PinListRepositoryImpl } from './PinListRepositoryImpl'
