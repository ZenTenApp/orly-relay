/**
 * Relay Bounded Context
 *
 * Handles relay management, relay sets, and relay preferences.
 */

// Aggregates
export { RelaySet } from './RelaySet'
export type { RelaySetChange } from './RelaySet'

export { RelayList } from './RelayList'
export type { RelayScope, RelayEntry, RelayListChange } from './RelayList'

export { FavoriteRelays } from './FavoriteRelays'
export type { FavoriteRelaysChange } from './FavoriteRelays'

// Errors
export {
  RelaySetOperationError,
  RelayListOperationError,
  DuplicateRelayError,
  RelayNotFoundError
} from './errors'

// Domain Events
export {
  FavoriteRelayAdded,
  FavoriteRelayRemoved,
  FavoriteRelaysPublished,
  RelaySetCreated,
  RelaySetUpdated,
  RelaySetDeleted,
  MailboxRelayAdded,
  MailboxRelayRemoved,
  MailboxRelayScopeChanged,
  RelayListPublished,
  type RelaySetChanges
} from './events'

// Repository Interfaces
export type {
  RelayListRepository,
  RelaySetRepository,
  FavoriteRelaysRepository
} from './repositories'

// Adapters for migration
export {
  // RelayList adapters
  toRelayList,
  tryToRelayList,
  fromRelayListToLegacy,
  toRelayListFromLegacy,
  // RelaySet adapters
  toRelaySet,
  tryToRelaySet,
  fromRelaySetToLegacy,
  toRelaySetFromLegacy,
  // FavoriteRelays adapters
  toFavoriteRelays,
  tryToFavoriteRelays,
  // Utility adapters
  urlsToRelayUrls,
  relayUrlsToStrings,
  normalizeRelayUrl,
  isValidRelayUrl
} from './adapters'
