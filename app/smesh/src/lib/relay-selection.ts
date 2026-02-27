import { RelayList } from '@/domain/relay/RelayList'
import relayListCacheService from '@/services/relay-list-cache.service'
import { Event } from 'nostr-tools'

/**
 * Context for relay selection decisions
 */
export interface RelaySelectionContext {
  /** Current user's relay list */
  userRelayList: RelayList | null
  /** Current user's pubkey */
  userPubkey: string | null
}

/**
 * Options for publish relay selection
 */
export interface PublishRelayOptions {
  /** Include recipients' (p-tag) write relays */
  includeRecipients?: boolean
  /** Additional relay hints from nprofile/nevent */
  hints?: string[]
}

/**
 * Select relays for publishing an event
 *
 * Strategy:
 * 1. Always include user's write relays
 * 2. If includeRecipients, add recipients' write relays
 * 3. Add any relay hints provided
 */
export async function selectPublishRelays(
  ctx: RelaySelectionContext,
  event: Partial<Event>,
  options: PublishRelayOptions = {}
): Promise<string[]> {
  const relays = new Set<string>()

  // Add user's write relays
  if (ctx.userRelayList) {
    for (const relay of ctx.userRelayList.getWriteUrls()) {
      relays.add(relay)
    }
  }

  // Add hints
  if (options.hints) {
    for (const hint of options.hints) {
      relays.add(hint)
    }
  }

  // Add recipients' write relays if requested
  if (options.includeRecipients && event.tags) {
    const pTags = event.tags.filter((t) => t[0] === 'p' && t[1])

    for (const tag of pTags) {
      const pubkey = tag[1]
      const hint = tag[2] // Optional relay hint in p-tag

      // Try to get cached relay list
      const recipientRelays = await relayListCacheService.getRelayList(pubkey)
      if (recipientRelays) {
        for (const relay of recipientRelays.write) {
          relays.add(relay)
        }
      } else if (hint && hint.startsWith('wss://')) {
        // Use hint as fallback
        relays.add(hint)
      }
    }
  }

  return Array.from(relays)
}

/**
 * Select relays for fetching events by author
 *
 * Strategy:
 * 1. Use author's read relays if known
 * 2. Fall back to hints if provided
 * 3. Last resort: user's own relays
 */
export async function selectReadRelays(
  ctx: RelaySelectionContext,
  authorPubkey: string,
  hints?: string[]
): Promise<string[]> {
  // Try cached relay list first
  const authorRelays = await relayListCacheService.getRelayList(authorPubkey)
  if (authorRelays && authorRelays.read.length > 0) {
    return authorRelays.read
  }

  // Use hints if provided
  if (hints && hints.length > 0) {
    return hints
  }

  // Last resort: user's own relays
  if (ctx.userRelayList) {
    return ctx.userRelayList.getReadUrls()
  }

  return []
}

/**
 * Select relays for fetching a specific event by ID
 *
 * Strategy:
 * 1. Use hints first (most likely to have the event)
 * 2. Add author's read relays if author is known
 * 3. Add user's read relays as fallback
 */
export async function selectRelaysForEvent(
  ctx: RelaySelectionContext,
  _eventId: string,
  hints?: string[],
  authorPubkey?: string
): Promise<string[]> {
  const relays = new Set<string>()

  // Use hints first (highest priority)
  if (hints) {
    for (const hint of hints) {
      relays.add(hint)
    }
  }

  // Add author's relays if known
  if (authorPubkey) {
    const authorRelays = await relayListCacheService.getRelayList(authorPubkey)
    if (authorRelays) {
      for (const relay of authorRelays.read) {
        relays.add(relay)
      }
    }
  }

  // Add user's relays as fallback
  if (ctx.userRelayList) {
    for (const relay of ctx.userRelayList.getReadUrls()) {
      relays.add(relay)
    }
  }

  return Array.from(relays)
}

/**
 * Select relays for fetching a thread
 *
 * Collects relays from:
 * - Root event author's relays
 * - All reply authors' relays
 * - Any hints in e-tags
 * - User's own relays
 */
export async function selectThreadRelays(
  ctx: RelaySelectionContext,
  _rootEventId: string,
  rootAuthorPubkey?: string,
  replyAuthorPubkeys?: string[],
  hints?: string[]
): Promise<string[]> {
  const relays = new Set<string>()

  // Add hints
  if (hints) {
    for (const hint of hints) {
      relays.add(hint)
    }
  }

  // Add root author's relays
  if (rootAuthorPubkey) {
    const rootRelays = await relayListCacheService.getRelayList(rootAuthorPubkey)
    if (rootRelays) {
      for (const relay of rootRelays.read) {
        relays.add(relay)
      }
    }
  }

  // Add reply authors' relays
  if (replyAuthorPubkeys && replyAuthorPubkeys.length > 0) {
    const authorRelays = await relayListCacheService.fetchRelayLists(replyAuthorPubkeys)
    for (const cached of authorRelays.values()) {
      for (const relay of cached.read) {
        relays.add(relay)
      }
    }
  }

  // Add user's relays
  if (ctx.userRelayList) {
    for (const relay of ctx.userRelayList.getReadUrls()) {
      relays.add(relay)
    }
  }

  return Array.from(relays)
}

/**
 * Select relays for DM operations
 *
 * For DMs, we need to ensure the message reaches the recipient's relays
 */
export async function selectDMRelays(
  ctx: RelaySelectionContext,
  recipientPubkey: string
): Promise<{ send: string[]; receive: string[] }> {
  const recipientRelays = await relayListCacheService.getRelayList(recipientPubkey)

  const sendRelays = new Set<string>()
  const receiveRelays = new Set<string>()

  // Send to recipient's write relays (where they check for incoming)
  // and our own write relays
  if (recipientRelays) {
    for (const relay of recipientRelays.write) {
      sendRelays.add(relay)
    }
  }

  if (ctx.userRelayList) {
    for (const relay of ctx.userRelayList.getWriteUrls()) {
      sendRelays.add(relay)
    }
    // Receive from our own read relays
    for (const relay of ctx.userRelayList.getReadUrls()) {
      receiveRelays.add(relay)
    }
  }

  return {
    send: Array.from(sendRelays),
    receive: Array.from(receiveRelays)
  }
}

/**
 * Extract relay hints from an nprofile, nevent, or naddr
 */
export function extractRelayHints(decoded: {
  type: string
  data: { relays?: string[] }
}): string[] {
  if ('relays' in decoded.data && Array.isArray(decoded.data.relays)) {
    return decoded.data.relays.filter((r) => typeof r === 'string' && r.startsWith('wss://'))
  }
  return []
}

/**
 * Extract relay hints from event e-tags
 */
export function extractRelayHintsFromTags(tags: string[][]): string[] {
  const hints: string[] = []

  for (const tag of tags) {
    if ((tag[0] === 'e' || tag[0] === 'a') && tag[2] && tag[2].startsWith('wss://')) {
      hints.push(tag[2])
    }
    if (tag[0] === 'p' && tag[2] && tag[2].startsWith('wss://')) {
      hints.push(tag[2])
    }
  }

  return [...new Set(hints)]
}
