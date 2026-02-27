/**
 * Adapter functions for gradual migration from legacy code to Relay domain objects.
 */

import { Event } from 'nostr-tools'
import { RelayUrl, tryToRelayUrl } from '../shared'
import { RelayList, RelayScope } from './RelayList'
import { RelaySet } from './RelaySet'
import { FavoriteRelays } from './FavoriteRelays'

// ============================================================================
// RelayList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a RelayList domain object
 */
export const toRelayList = (event: Event, filterOutOnion = false): RelayList => {
  return RelayList.fromEvent(event, filterOutOnion)
}

/**
 * Try to create a RelayList from an event, returns null if invalid
 */
export const tryToRelayList = (
  event: Event | null | undefined,
  filterOutOnion = false
): RelayList | null => {
  if (!event) return null
  try {
    return RelayList.fromEvent(event, filterOutOnion)
  } catch {
    return null
  }
}

/**
 * Convert a RelayList to the legacy TRelayList format
 */
export const fromRelayListToLegacy = (
  relayList: RelayList
): {
  read: string[]
  write: string[]
  originalRelays: Array<{ url: string; scope: RelayScope }>
} => {
  return relayList.toLegacyFormat()
}

/**
 * Create a RelayList from legacy format
 */
export const toRelayListFromLegacy = (
  ownerHex: string,
  legacy: {
    read: string[]
    write: string[]
    originalRelays?: Array<{ url: string; scope: RelayScope }>
  }
): RelayList | null => {
  const { Pubkey } = require('../shared')
  const owner = Pubkey.tryFromString(ownerHex)
  if (!owner) return null

  const relayList = RelayList.empty(owner)

  if (legacy.originalRelays) {
    for (const { url, scope } of legacy.originalRelays) {
      relayList.setRelayUrl(url, scope)
    }
  } else {
    // Reconstruct from read/write arrays
    const readSet = new Set(legacy.read)
    const writeSet = new Set(legacy.write)
    const allUrls = new Set([...legacy.read, ...legacy.write])

    for (const url of allUrls) {
      const isRead = readSet.has(url)
      const isWrite = writeSet.has(url)
      let scope: RelayScope = 'both'
      if (isRead && !isWrite) scope = 'read'
      else if (!isRead && isWrite) scope = 'write'

      relayList.setRelayUrl(url, scope)
    }
  }

  return relayList
}

// ============================================================================
// RelaySet Adapters
// ============================================================================

/**
 * Convert a Nostr event to a RelaySet domain object
 */
export const toRelaySet = (event: Event): RelaySet => {
  return RelaySet.fromEvent(event)
}

/**
 * Try to create a RelaySet from an event, returns null if invalid
 */
export const tryToRelaySet = (event: Event | null | undefined): RelaySet | null => {
  if (!event) return null
  try {
    return RelaySet.fromEvent(event)
  } catch {
    return null
  }
}

/**
 * Convert a RelaySet to the legacy TRelaySet format
 */
export const fromRelaySetToLegacy = (
  relaySet: RelaySet,
  pubkey: string
): {
  id: string
  aTag: string[]
  name: string
  relayUrls: string[]
} => {
  return {
    id: relaySet.id,
    aTag: relaySet.toATag(pubkey),
    name: relaySet.name,
    relayUrls: relaySet.getRelayUrls()
  }
}

/**
 * Create a RelaySet from legacy format
 */
export const toRelaySetFromLegacy = (legacy: {
  id: string
  name: string
  relayUrls: string[]
}): RelaySet => {
  return RelaySet.createWithRelays(legacy.name, legacy.relayUrls, legacy.id)
}

// ============================================================================
// FavoriteRelays Adapters
// ============================================================================

/**
 * Convert events to a FavoriteRelays domain object
 */
export const toFavoriteRelays = (
  event: Event,
  relaySetEvents: Event[] = []
): FavoriteRelays => {
  const relaySets = relaySetEvents.map((e) => tryToRelaySet(e)).filter(Boolean) as RelaySet[]
  return FavoriteRelays.fromEvent(event, relaySets)
}

/**
 * Try to create FavoriteRelays from an event
 */
export const tryToFavoriteRelays = (
  event: Event | null | undefined,
  relaySetEvents: Event[] = []
): FavoriteRelays | null => {
  if (!event) return null
  try {
    return toFavoriteRelays(event, relaySetEvents)
  } catch {
    return null
  }
}

// ============================================================================
// Utility Adapters
// ============================================================================

/**
 * Convert an array of URL strings to RelayUrl objects
 * Skips invalid URLs
 */
export const urlsToRelayUrls = (urls: string[]): RelayUrl[] => {
  return urls.map((u) => tryToRelayUrl(u)).filter((r): r is RelayUrl => r !== null)
}

/**
 * Convert an array of RelayUrl objects to URL strings
 */
export const relayUrlsToStrings = (relays: RelayUrl[]): string[] => {
  return relays.map((r) => r.value)
}

/**
 * Normalize a relay URL string
 * Returns null if invalid
 */
export const normalizeRelayUrl = (url: string): string | null => {
  const relay = tryToRelayUrl(url)
  return relay ? relay.value : null
}

/**
 * Check if a string is a valid WebSocket URL
 */
export const isValidRelayUrl = (url: string): boolean => {
  return RelayUrl.isValid(url)
}
