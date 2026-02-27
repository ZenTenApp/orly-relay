/**
 * Adapter functions for gradual migration from legacy code to Content domain objects.
 */

import { Event, kinds } from 'nostr-tools'
import { Note } from './Note'
import { Reaction } from './Reaction'
import { Repost } from './Repost'
import { BookmarkList } from './BookmarkList'
import { PinList } from './PinList'

// ============================================================================
// Note Adapters
// ============================================================================

/**
 * Convert a Nostr event to a Note domain object
 */
export const toNote = (event: Event): Note => {
  return Note.fromEvent(event)
}

/**
 * Try to create a Note from an event, returns null if invalid
 */
export const tryToNote = (event: Event | null | undefined): Note | null => {
  return Note.tryFromEvent(event)
}

/**
 * Check if an event is a short text note
 */
export const isNoteEvent = (event: Event): boolean => {
  return event.kind === kinds.ShortTextNote
}

/**
 * Convert multiple events to Notes
 * Filters out non-note events
 */
export const toNotes = (events: Event[]): Note[] => {
  return events
    .filter(isNoteEvent)
    .map((e) => tryToNote(e))
    .filter((n): n is Note => n !== null)
}

// ============================================================================
// Reaction Adapters
// ============================================================================

/**
 * Convert a Nostr event to a Reaction domain object
 */
export const toReaction = (event: Event): Reaction => {
  return Reaction.fromEvent(event)
}

/**
 * Try to create a Reaction from an event, returns null if invalid
 */
export const tryToReaction = (event: Event | null | undefined): Reaction | null => {
  return Reaction.tryFromEvent(event)
}

/**
 * Check if an event is a reaction
 */
export const isReactionEvent = (event: Event): boolean => {
  return event.kind === kinds.Reaction
}

/**
 * Convert multiple events to Reactions
 */
export const toReactions = (events: Event[]): Reaction[] => {
  return events
    .filter(isReactionEvent)
    .map((e) => tryToReaction(e))
    .filter((r): r is Reaction => r !== null)
}

/**
 * Group reactions by emoji
 */
export const groupReactionsByEmoji = (
  reactions: Reaction[]
): Map<string, Reaction[]> => {
  const groups = new Map<string, Reaction[]>()

  for (const reaction of reactions) {
    const emoji = reaction.emoji
    const existing = groups.get(emoji) || []
    existing.push(reaction)
    groups.set(emoji, existing)
  }

  return groups
}

/**
 * Count likes for an event
 */
export const countLikes = (reactions: Reaction[]): number => {
  return reactions.filter((r) => r.isLike).length
}

// ============================================================================
// Repost Adapters
// ============================================================================

/**
 * Convert a Nostr event to a Repost domain object
 */
export const toRepost = (event: Event): Repost => {
  return Repost.fromEvent(event)
}

/**
 * Try to create a Repost from an event, returns null if invalid
 */
export const tryToRepost = (event: Event | null | undefined): Repost | null => {
  return Repost.tryFromEvent(event)
}

/**
 * Check if an event is a repost
 */
export const isRepostEvent = (event: Event): boolean => {
  return event.kind === kinds.Repost || event.kind === kinds.GenericRepost
}

/**
 * Convert multiple events to Reposts
 */
export const toReposts = (events: Event[]): Repost[] => {
  return events
    .filter(isRepostEvent)
    .map((e) => tryToRepost(e))
    .filter((r): r is Repost => r !== null)
}

// ============================================================================
// Content Type Detection
// ============================================================================

/**
 * Determine the content type of an event
 */
export const getContentType = (
  event: Event
): 'note' | 'reaction' | 'repost' | 'other' => {
  if (event.kind === kinds.ShortTextNote) return 'note'
  if (event.kind === kinds.Reaction) return 'reaction'
  if (event.kind === kinds.Repost || event.kind === kinds.GenericRepost) return 'repost'
  return 'other'
}

/**
 * Parse any content event into its appropriate domain object
 */
export const parseContentEvent = (
  event: Event
): Note | Reaction | Repost | null => {
  const type = getContentType(event)

  switch (type) {
    case 'note':
      return tryToNote(event)
    case 'reaction':
      return tryToReaction(event)
    case 'repost':
      return tryToRepost(event)
    default:
      return null
  }
}

// ============================================================================
// BookmarkList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a BookmarkList domain object
 */
export const toBookmarkList = (event: Event): BookmarkList => {
  return BookmarkList.fromEvent(event)
}

/**
 * Try to create a BookmarkList from an event, returns null if invalid
 */
export const tryToBookmarkList = (event: Event | null | undefined): BookmarkList | null => {
  return BookmarkList.tryFromEvent(event)
}

/**
 * Check if an event is a bookmark list
 */
export const isBookmarkListEvent = (event: Event): boolean => {
  return event.kind === kinds.BookmarkList
}

// ============================================================================
// PinList Adapters
// ============================================================================

/**
 * Convert a Nostr event to a PinList domain object
 */
export const toPinList = (event: Event): PinList => {
  return PinList.fromEvent(event)
}

/**
 * Try to create a PinList from an event, returns null if invalid
 */
export const tryToPinList = (event: Event | null | undefined): PinList | null => {
  return PinList.tryFromEvent(event)
}

/**
 * Check if an event is a pin list
 */
export const isPinListEvent = (event: Event): boolean => {
  return event.kind === kinds.Pinlist
}
