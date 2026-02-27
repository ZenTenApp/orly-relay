/**
 * Adapter functions for gradual migration from legacy code to Social domain objects.
 *
 * These functions allow existing providers and services to continue working
 * while new code can use the rich domain objects.
 */

import { Event } from 'nostr-tools'
import { tryToPubkey } from '../shared'
import { FollowList } from './FollowList'
import { MuteList, MuteVisibility } from './MuteList'
import { PinnedUsersList } from './PinnedUsersList'

// ============================================================================
// FollowList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a FollowList domain object
 */
export const toFollowList = (event: Event): FollowList => {
  return FollowList.fromEvent(event)
}

/**
 * Try to create a FollowList from an event, returns null if invalid
 */
export const tryToFollowList = (event: Event | null | undefined): FollowList | null => {
  if (!event) return null
  try {
    return FollowList.fromEvent(event)
  } catch {
    return null
  }
}

/**
 * Convert a FollowList to a legacy hex string set
 */
export const fromFollowListToHexSet = (followList: FollowList): Set<string> => {
  return new Set(followList.getFollowing().map((p) => p.hex))
}

/**
 * Convert a FollowList to a legacy hex string array
 */
export const fromFollowListToHexArray = (followList: FollowList): string[] => {
  return followList.getFollowing().map((p) => p.hex)
}

/**
 * Check if a hex pubkey is in a FollowList
 */
export const isFollowingHex = (followList: FollowList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  return pubkey ? followList.isFollowing(pubkey) : false
}

/**
 * Add a hex pubkey to a FollowList
 * @returns true if added, false if already following or invalid
 */
export const followByHex = (
  followList: FollowList,
  hex: string,
  relayHint?: string,
  petname?: string
): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  try {
    const change = followList.follow(pubkey, relayHint, petname)
    return change.type === 'added'
  } catch {
    return false
  }
}

/**
 * Remove a hex pubkey from a FollowList
 * @returns true if removed, false if not following or invalid
 */
export const unfollowByHex = (followList: FollowList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  const change = followList.unfollow(pubkey)
  return change.type === 'removed'
}

// ============================================================================
// MuteList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a MuteList domain object
 *
 * @param event The mute list event
 * @param decryptedPrivateTags The decrypted private tags (from NIP-04 content)
 */
export const toMuteList = (
  event: Event,
  decryptedPrivateTags: string[][] = []
): MuteList => {
  return MuteList.fromEvent(event, decryptedPrivateTags)
}

/**
 * Try to create a MuteList from an event, returns null if invalid
 */
export const tryToMuteList = (
  event: Event | null | undefined,
  decryptedPrivateTags: string[][] = []
): MuteList | null => {
  if (!event) return null
  try {
    return MuteList.fromEvent(event, decryptedPrivateTags)
  } catch {
    return null
  }
}

/**
 * Convert a MuteList to a legacy hex string set (all mutes)
 */
export const fromMuteListToHexSet = (muteList: MuteList): Set<string> => {
  return new Set(muteList.getAllMuted().map((p) => p.hex))
}

/**
 * Convert a MuteList's public mutes to a legacy hex string set
 */
export const fromMuteListToPublicHexSet = (muteList: MuteList): Set<string> => {
  return new Set(muteList.getPublicMuted().map((p) => p.hex))
}

/**
 * Convert a MuteList's private mutes to a legacy hex string set
 */
export const fromMuteListToPrivateHexSet = (muteList: MuteList): Set<string> => {
  return new Set(muteList.getPrivateMuted().map((p) => p.hex))
}

/**
 * Check if a hex pubkey is muted
 */
export const isMutedHex = (muteList: MuteList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  return pubkey ? muteList.isMuted(pubkey) : false
}

/**
 * Get the mute visibility for a hex pubkey
 */
export const getMuteVisibilityByHex = (
  muteList: MuteList,
  hex: string
): MuteVisibility | null => {
  const pubkey = tryToPubkey(hex)
  return pubkey ? muteList.getMuteVisibility(pubkey) : null
}

/**
 * Mute a hex pubkey publicly
 * @returns true if muted, false if already muted or invalid
 */
export const mutePubliclyByHex = (muteList: MuteList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  try {
    const change = muteList.mutePublicly(pubkey)
    return change.type !== 'no_change'
  } catch {
    return false
  }
}

/**
 * Mute a hex pubkey privately
 * @returns true if muted, false if already muted or invalid
 */
export const mutePrivatelyByHex = (muteList: MuteList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  try {
    const change = muteList.mutePrivately(pubkey)
    return change.type !== 'no_change'
  } catch {
    return false
  }
}

/**
 * Unmute a hex pubkey
 * @returns true if unmuted, false if not muted or invalid
 */
export const unmuteByHex = (muteList: MuteList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  const change = muteList.unmute(pubkey)
  return change.type === 'unmuted'
}

// ============================================================================
// PinnedUsersList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a PinnedUsersList domain object
 *
 * @param event The pinned users list event
 * @param decryptedPrivateTags The decrypted private tags (from NIP-04 content)
 */
export const toPinnedUsersList = (
  event: Event,
  decryptedPrivateTags: string[][] = []
): PinnedUsersList => {
  const list = PinnedUsersList.fromEvent(event)
  if (decryptedPrivateTags.length > 0) {
    list.setPrivatePins(decryptedPrivateTags)
  }
  return list
}

/**
 * Convert a PinnedUsersList to a legacy hex string set (all pins)
 */
export const fromPinnedUsersListToHexSet = (pinnedUsersList: PinnedUsersList): Set<string> => {
  return new Set(pinnedUsersList.getPinnedPubkeys().map((p) => p.hex))
}

/**
 * Check if a hex pubkey is pinned
 */
export const isPinnedHex = (pinnedUsersList: PinnedUsersList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  return pubkey ? pinnedUsersList.isPinned(pubkey) : false
}

/**
 * Pin a hex pubkey
 * @returns true if pinned, false if already pinned or invalid
 */
export const pinByHex = (pinnedUsersList: PinnedUsersList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  try {
    const change = pinnedUsersList.pin(pubkey)
    return change.type === 'pinned'
  } catch {
    return false
  }
}

/**
 * Unpin a hex pubkey
 * @returns true if unpinned, false if not pinned or invalid
 */
export const unpinByHex = (pinnedUsersList: PinnedUsersList, hex: string): boolean => {
  const pubkey = tryToPubkey(hex)
  if (!pubkey) return false
  const change = pinnedUsersList.unpin(pubkey)
  return change.type === 'unpinned'
}

// ============================================================================
// Combined Adapters
// ============================================================================

/**
 * Create a function to check if a pubkey should be filtered out
 * (either muted or not following, depending on context)
 */
export const createMuteFilter = (
  muteList: MuteList
): ((hex: string) => boolean) => {
  const mutedSet = fromMuteListToHexSet(muteList)
  return (hex: string) => mutedSet.has(hex)
}

/**
 * Create a function to check if a pubkey is being followed
 */
export const createFollowFilter = (
  followList: FollowList
): ((hex: string) => boolean) => {
  const followingSet = fromFollowListToHexSet(followList)
  return (hex: string) => followingSet.has(hex)
}

/**
 * Create a function to check if a pubkey is pinned
 */
export const createPinnedFilter = (
  pinnedUsersList: PinnedUsersList
): ((hex: string) => boolean) => {
  const pinnedSet = fromPinnedUsersListToHexSet(pinnedUsersList)
  return (hex: string) => pinnedSet.has(hex)
}
