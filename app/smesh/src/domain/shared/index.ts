/**
 * Domain Shared Kernel
 *
 * Common value objects, errors, and adapters used across all bounded contexts.
 */

// Value Objects
export {
  Pubkey,
  RelayUrl,
  EventId,
  Timestamp,
  InvalidPubkeyError,
  InvalidRelayUrlError,
  InvalidEventIdError,
  InvalidTimestampError,
  DomainError,
} from './value-objects'

// Domain Events
export {
  DomainEvent,
  SimpleEventDispatcher,
  eventDispatcher,
} from './events'
export type { EventHandler, EventDispatcher } from './events'

// Adapters for migration
export {
  // Pubkey
  toPubkey,
  tryToPubkey,
  fromPubkey,
  toPubkeys,
  fromPubkeys,
  // RelayUrl
  toRelayUrl,
  tryToRelayUrl,
  fromRelayUrl,
  toRelayUrls,
  fromRelayUrls,
  // EventId
  toEventId,
  tryToEventId,
  fromEventId,
  toEventIds,
  fromEventIds,
  // Timestamp
  toTimestamp,
  tryToTimestamp,
  fromTimestamp,
  // Set helpers
  createPubkeySet,
  createRelayUrlSet,
} from './adapters'
