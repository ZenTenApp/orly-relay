/**
 * Adapter functions for gradual migration from primitives to value objects.
 *
 * These functions allow existing code to continue using string primitives
 * while new code can use value objects. Use these at the boundaries
 * between old and new code.
 */

import { Pubkey, RelayUrl, EventId, Timestamp } from './value-objects'

// ============================================================================
// Pubkey Adapters
// ============================================================================

/**
 * Convert a hex string to a Pubkey value object.
 * @throws InvalidPubkeyError if invalid
 */
export const toPubkey = (hex: string): Pubkey => Pubkey.fromHex(hex)

/**
 * Try to convert a string (hex, npub, nprofile) to a Pubkey.
 * Returns null if invalid.
 */
export const tryToPubkey = (input: string): Pubkey | null => Pubkey.tryFromString(input)

/**
 * Convert a Pubkey back to a hex string for interop with existing code.
 */
export const fromPubkey = (pubkey: Pubkey): string => pubkey.hex

/**
 * Convert an array of hex strings to Pubkeys.
 * Skips invalid entries.
 */
export const toPubkeys = (hexes: string[]): Pubkey[] =>
  hexes.map((h) => Pubkey.tryFromString(h)).filter((p): p is Pubkey => p !== null)

/**
 * Convert an array of Pubkeys to hex strings.
 */
export const fromPubkeys = (pubkeys: Pubkey[]): string[] => pubkeys.map((p) => p.hex)

// ============================================================================
// RelayUrl Adapters
// ============================================================================

/**
 * Convert a URL string to a RelayUrl value object.
 * @throws InvalidRelayUrlError if invalid
 */
export const toRelayUrl = (url: string): RelayUrl => RelayUrl.create(url)

/**
 * Try to convert a URL string to a RelayUrl.
 * Returns null if invalid.
 */
export const tryToRelayUrl = (url: string): RelayUrl | null => RelayUrl.tryCreate(url)

/**
 * Convert a RelayUrl back to a string for interop with existing code.
 */
export const fromRelayUrl = (relay: RelayUrl): string => relay.value

/**
 * Convert an array of URL strings to RelayUrls.
 * Skips invalid entries.
 */
export const toRelayUrls = (urls: string[]): RelayUrl[] =>
  urls.map((u) => RelayUrl.tryCreate(u)).filter((r): r is RelayUrl => r !== null)

/**
 * Convert an array of RelayUrls to strings.
 */
export const fromRelayUrls = (relays: RelayUrl[]): string[] => relays.map((r) => r.value)

// ============================================================================
// EventId Adapters
// ============================================================================

/**
 * Convert a hex string to an EventId value object.
 * @throws InvalidEventIdError if invalid
 */
export const toEventId = (hex: string): EventId => EventId.fromHex(hex)

/**
 * Try to convert a string (hex, note1, nevent1) to an EventId.
 * Returns null if invalid.
 */
export const tryToEventId = (input: string): EventId | null => EventId.tryFromString(input)

/**
 * Convert an EventId back to a hex string for interop with existing code.
 */
export const fromEventId = (eventId: EventId): string => eventId.hex

/**
 * Convert an array of hex strings to EventIds.
 * Skips invalid entries.
 */
export const toEventIds = (hexes: string[]): EventId[] =>
  hexes.map((h) => EventId.tryFromString(h)).filter((e): e is EventId => e !== null)

/**
 * Convert an array of EventIds to hex strings.
 */
export const fromEventIds = (eventIds: EventId[]): string[] => eventIds.map((e) => e.hex)

// ============================================================================
// Timestamp Adapters
// ============================================================================

/**
 * Convert a Unix timestamp (seconds) to a Timestamp value object.
 * @throws InvalidTimestampError if invalid
 */
export const toTimestamp = (unix: number): Timestamp => Timestamp.fromUnix(unix)

/**
 * Try to convert a Unix timestamp to a Timestamp.
 * Returns null if invalid.
 */
export const tryToTimestamp = (unix: number): Timestamp | null => Timestamp.tryFromUnix(unix)

/**
 * Convert a Timestamp back to a Unix number for interop with existing code.
 */
export const fromTimestamp = (timestamp: Timestamp): number => timestamp.unix

// ============================================================================
// Set Adapters (for mute lists, follow lists, etc.)
// ============================================================================

/**
 * Convert a Set of hex strings to a Set wrapper that uses Pubkey for lookups.
 */
export const createPubkeySet = (
  hexSet: Set<string>
): {
  has: (pubkey: Pubkey) => boolean
  add: (pubkey: Pubkey) => void
  delete: (pubkey: Pubkey) => boolean
  toHexSet: () => Set<string>
  toPubkeys: () => Pubkey[]
} => ({
  has: (pubkey: Pubkey) => hexSet.has(pubkey.hex),
  add: (pubkey: Pubkey) => hexSet.add(pubkey.hex),
  delete: (pubkey: Pubkey) => hexSet.delete(pubkey.hex),
  toHexSet: () => hexSet,
  toPubkeys: () => Array.from(hexSet).map((h) => Pubkey.fromHex(h)),
})

/**
 * Convert a Set of URL strings to a Set wrapper that uses RelayUrl for lookups.
 */
export const createRelayUrlSet = (
  urlSet: Set<string>
): {
  has: (relay: RelayUrl) => boolean
  add: (relay: RelayUrl) => void
  delete: (relay: RelayUrl) => boolean
  toStringSet: () => Set<string>
  toRelayUrls: () => RelayUrl[]
} => ({
  has: (relay: RelayUrl) => urlSet.has(relay.value),
  add: (relay: RelayUrl) => urlSet.add(relay.value),
  delete: (relay: RelayUrl) => urlSet.delete(relay.value),
  toStringSet: () => urlSet,
  toRelayUrls: () =>
    Array.from(urlSet)
      .map((u) => RelayUrl.tryCreate(u))
      .filter((r): r is RelayUrl => r !== null),
})
