/**
 * DM Service - Direct Message handling with NIP-04 and NIP-17 encryption support
 *
 * NIP-04: Kind 4 encrypted direct messages (legacy)
 * NIP-17: Kind 14 private direct messages with NIP-59 gift wrapping (modern)
 */

import { ExtendedKind } from '@/constants'
import { TConversation, TDirectMessage, TDMDeletedState, TDMEncryptionType, TDraftEvent } from '@/types'
import { Event, kinds, VerifiedEvent } from 'nostr-tools'
import client from './client.service'
import indexedDb from './indexed-db.service'

/** Check if a DM is an NIRC protocol message that should be hidden from the inbox */
export function isNircProtocolMessage(content: string): boolean {
  if (!content) return false
  if (content.startsWith('nirc:request:')) return true
  if (content.startsWith('nirc:')) return true
  return false
}

// In-memory plaintext cache for fast access (avoids async IndexedDB lookups on re-render)
const plaintextCache = new Map<string, string>()
const MAX_CACHE_SIZE = 1000

/**
 * Get plaintext from in-memory cache
 */
export function getCachedPlaintext(eventId: string): string | undefined {
  return plaintextCache.get(eventId)
}

/**
 * Set plaintext in in-memory cache (with LRU eviction)
 */
export function setCachedPlaintext(eventId: string, plaintext: string): void {
  // Simple LRU: if cache is full, delete oldest entries
  if (plaintextCache.size >= MAX_CACHE_SIZE) {
    const keysToDelete = Array.from(plaintextCache.keys()).slice(0, 100)
    keysToDelete.forEach(k => plaintextCache.delete(k))
  }
  plaintextCache.set(eventId, plaintext)
}

/**
 * Clear the plaintext cache (e.g., on logout)
 */
export function clearPlaintextCache(): void {
  plaintextCache.clear()
}

/**
 * Decrypt messages in batches to avoid blocking the UI
 * Yields control back to the event loop between batches
 */
export async function decryptMessagesInBatches(
  events: Event[],
  encryption: IDMEncryption,
  myPubkey: string,
  batchSize: number = 10,
  onBatchComplete?: (messages: TDirectMessage[], progress: number) => void
): Promise<TDirectMessage[]> {
  const allMessages: TDirectMessage[] = []
  const total = events.length

  for (let i = 0; i < events.length; i += batchSize) {
    const batch = events.slice(i, i + batchSize)

    // Process batch
    const batchResults = await Promise.all(
      batch.map((event) => dmService.decryptMessage(event, encryption, myPubkey))
    )

    const validMessages = batchResults.filter((m): m is TDirectMessage => m !== null)
    allMessages.push(...validMessages)

    // Report progress
    const progress = Math.min((i + batchSize) / total, 1)
    onBatchComplete?.(validMessages, progress)

    // Yield to event loop between batches (prevents UI blocking)
    if (i + batchSize < events.length) {
      await new Promise(resolve => setTimeout(resolve, 0))
    }
  }

  return allMessages
}

/**
 * Create and publish a kind 5 delete request for own messages
 * This requests relays to delete the original event
 */
export async function publishDeleteRequest(
  eventIds: string[],
  eventKind: number,
  encryption: IDMEncryption,
  relayUrls: string[]
): Promise<void> {
  if (eventIds.length === 0) return

  const draftEvent: TDraftEvent = {
    kind: kinds.EventDeletion, // 5
    created_at: Math.floor(Date.now() / 1000),
    content: 'Deleted by sender',
    tags: [
      ['k', eventKind.toString()],
      ...eventIds.map((id) => ['e', id])
    ]
  }

  const signedEvent = await encryption.signEvent(draftEvent)
  await client.publishEvent(relayUrls, signedEvent)
}

/**
 * Encryption methods interface for DM operations
 */
export interface IDMEncryption {
  nip04Encrypt: (pubkey: string, plainText: string) => Promise<string>
  nip04Decrypt: (pubkey: string, cipherText: string) => Promise<string>
  nip44Encrypt?: (pubkey: string, plainText: string) => Promise<string>
  nip44Decrypt?: (pubkey: string, cipherText: string) => Promise<string>
  signEvent: (draftEvent: TDraftEvent) => Promise<VerifiedEvent>
  getPublicKey: () => string
}

// NIP-04 uses kind 4
const KIND_ENCRYPTED_DM = kinds.EncryptedDirectMessage // 4

// NIP-17 uses kind 14 for chat messages, wrapped in gift wraps
const KIND_PRIVATE_DM = ExtendedKind.PRIVATE_DM // 14
const KIND_SEAL = ExtendedKind.SEAL // 13
const KIND_GIFT_WRAP = ExtendedKind.GIFT_WRAP // 1059
const KIND_REACTION = kinds.Reaction // 7

// 15 second timeout for DM fetches - if relays are dead, don't wait forever
const DM_FETCH_TIMEOUT_MS = 15000

/**
 * Wrap a promise with a timeout that returns empty array on timeout or error
 */
function withTimeout<T>(promise: Promise<T[]>, ms: number): Promise<T[]> {
  const timeoutPromise = new Promise<T[]>((resolve) => {
    setTimeout(() => resolve([]), ms)
  })
  const safePromise = promise.catch(() => [] as T[])
  return Promise.race([safePromise, timeoutPromise])
}

class DMService {
  /**
   * Fetch all DM events for a user from relays
   */
  async fetchDMEvents(pubkey: string, relayUrls: string[], limit = 500): Promise<Event[]> {
    // Use provided relays - no hardcoded fallback
    const allRelays = [...new Set(relayUrls)]

    // Fetch NIP-04 DMs (kind 4) and NIP-17 gift wraps in parallel
    const nip04Filter = {
      kinds: [KIND_ENCRYPTED_DM],
      limit
    }

    const [incomingNip04, outgoingNip04, giftWraps] = await Promise.all([
      // Fetch messages sent TO the user
      withTimeout(
        client.fetchEvents(allRelays, {
          ...nip04Filter,
          '#p': [pubkey]
        }),
        DM_FETCH_TIMEOUT_MS
      ),
      // Fetch messages sent BY the user
      withTimeout(
        client.fetchEvents(allRelays, {
          ...nip04Filter,
          authors: [pubkey]
        }),
        DM_FETCH_TIMEOUT_MS
      ),
      // Fetch NIP-17 gift wraps (kind 1059) - these are addressed to the user
      withTimeout(
        client.fetchEvents(allRelays, {
          kinds: [KIND_GIFT_WRAP],
          '#p': [pubkey],
          limit
        }),
        DM_FETCH_TIMEOUT_MS
      )
    ])

    // Combine all events
    const allEvents = [...incomingNip04, ...outgoingNip04, ...giftWraps]

    // Store in IndexedDB for caching
    await Promise.all(allEvents.map((event) => indexedDb.putDMEvent(event)))

    return allEvents
  }

  /**
   * Fetch recent DM events (limited) for building conversation list
   * Returns only most recent events to quickly show conversations
   */
  async fetchRecentDMEvents(pubkey: string, relayUrls: string[]): Promise<Event[]> {
    // Fetch with smaller limit for faster initial load
    return this.fetchDMEvents(pubkey, relayUrls, 100)
  }

  /**
   * Fetch all DM events for a specific conversation partner
   */
  async fetchConversationEvents(
    pubkey: string,
    partnerPubkey: string,
    relayUrls: string[]
  ): Promise<Event[]> {
    // Use provided relays - no hardcoded fallback
    const allRelays = [...new Set(relayUrls)]

    // Get partner's inbox relays for better NIP-17 discovery
    const partnerInboxRelays = await this.fetchPartnerInboxRelays(partnerPubkey)
    const inboxRelays = [...new Set([...relayUrls, ...partnerInboxRelays])]

    // Fetch NIP-04 messages between user and partner (with timeout)
    const [incomingNip04, outgoingNip04, giftWraps] = await Promise.all([
      // Messages FROM partner TO user
      withTimeout(
        client.fetchEvents(allRelays, {
          kinds: [KIND_ENCRYPTED_DM],
          authors: [partnerPubkey],
          '#p': [pubkey],
          limit: 500
        }),
        DM_FETCH_TIMEOUT_MS
      ),
      // Messages FROM user TO partner
      withTimeout(
        client.fetchEvents(allRelays, {
          kinds: [KIND_ENCRYPTED_DM],
          authors: [pubkey],
          '#p': [partnerPubkey],
          limit: 500
        }),
        DM_FETCH_TIMEOUT_MS
      ),
      // Gift wraps addressed to user - check both regular relays and inbox relays
      withTimeout(
        client.fetchEvents(inboxRelays, {
          kinds: [KIND_GIFT_WRAP],
          '#p': [pubkey],
          limit: 500
        }),
        DM_FETCH_TIMEOUT_MS
      )
    ])

    const allEvents = [...incomingNip04, ...outgoingNip04, ...giftWraps]

    // Store in IndexedDB for caching
    await Promise.all(allEvents.map((event) => indexedDb.putDMEvent(event)))

    return allEvents
  }

  /**
   * Decrypt a DM event and return a TDirectMessage
   */
  async decryptMessage(
    event: Event,
    encryption: IDMEncryption,
    myPubkey: string
  ): Promise<TDirectMessage | null> {
    try {
      if (event.kind === KIND_ENCRYPTED_DM) {
        // NIP-04 decryption - check in-memory cache first (fastest)
        const memCached = getCachedPlaintext(event.id)
        if (memCached) {
          return this.buildDirectMessage(event, memCached, myPubkey, 'nip04')
        }

        // Check IndexedDB cache (slower but persistent)
        const dbCached = await indexedDb.getDecryptedContent(event.id)
        if (dbCached) {
          // Populate in-memory cache for next access
          setCachedPlaintext(event.id, dbCached)
          return this.buildDirectMessage(event, dbCached, myPubkey, 'nip04')
        }

        const otherPubkey = this.getOtherPartyPubkey(event, myPubkey)
        if (!otherPubkey) return null

        let decryptedContent: string
        try {
          decryptedContent = await encryption.nip04Decrypt(otherPubkey, event.content)
        } catch {
          // NIP-04 failed — try NIP-44 fallback (some relays/bridges use NIP-44 on kind 4)
          if (encryption.nip44Decrypt) {
            decryptedContent = await encryption.nip44Decrypt(otherPubkey, event.content)
          } else {
            throw new Error('NIP-04 decrypt failed and no NIP-44 support')
          }
        }

        // Cache in both layers
        setCachedPlaintext(event.id, decryptedContent)
        indexedDb.putDecryptedContent(event.id, decryptedContent).catch(() => {})

        return this.buildDirectMessage(event, decryptedContent, myPubkey, 'nip04')
      } else if (event.kind === KIND_GIFT_WRAP) {
        // NIP-17 - check in-memory cache first
        const memCached = getCachedPlaintext(event.id)
        if (memCached) {
          // Stored as JSON: {s: senderPubkey, r: recipientPubkey, c: content, t: innerTimestamp, i: innerEventId}
          try {
            const parsed = JSON.parse(memCached) as { s: string; r: string; c: string; t?: number; i?: string }
            if (parsed.r === '__reaction__') return null
            const seenOnRelays = client.getSeenEventRelayUrls(event.id)
            return {
              id: event.id,
              innerEventId: parsed.i,
              senderPubkey: parsed.s,
              recipientPubkey: parsed.r,
              content: parsed.c,
              createdAt: parsed.t ?? event.created_at,
              encryptionType: 'nip17',
              event,
              decryptedContent: parsed.c,
              seenOnRelays: seenOnRelays.length > 0 ? seenOnRelays : undefined
            }
          } catch {
            // Invalid cache entry, fall through to re-decrypt
          }
        }

        // Check IndexedDB cache (includes sender info)
        const cachedUnwrapped = await indexedDb.getUnwrappedGiftWrap(event.id)
        if (cachedUnwrapped) {
          // Skip reactions in cache for now (they're stored but not returned as messages)
          if (cachedUnwrapped.recipientPubkey === '__reaction__') {
            return null
          }
          // Populate in-memory cache (include inner timestamp + inner event ID)
          setCachedPlaintext(event.id, JSON.stringify({ s: cachedUnwrapped.pubkey, r: cachedUnwrapped.recipientPubkey, c: cachedUnwrapped.content, t: cachedUnwrapped.createdAt, i: cachedUnwrapped.innerEventId }))
          const seenOnRelays = client.getSeenEventRelayUrls(event.id)
          return {
            id: event.id,
            innerEventId: cachedUnwrapped.innerEventId,
            senderPubkey: cachedUnwrapped.pubkey,
            recipientPubkey: cachedUnwrapped.recipientPubkey,
            content: cachedUnwrapped.content,
            createdAt: cachedUnwrapped.createdAt,
            encryptionType: 'nip17',
            event,
            decryptedContent: cachedUnwrapped.content,
            seenOnRelays: seenOnRelays.length > 0 ? seenOnRelays : undefined
          }
        }

        // Decrypt (unwrap gift wrap -> unseal -> decrypt)
        const unwrapped = await this.unwrapGiftWrap(event, encryption)
        if (!unwrapped) return null

        const innerEvent = unwrapped.innerEvent
        if (!innerEvent.tags) innerEvent.tags = []

        // Handle reactions - cache them but don't return as messages
        if (unwrapped.type === 'reaction') {
          // Cache the reaction for later display
          // TODO: Store reaction separately and associate with target message via 'e' tag
          indexedDb
            .putUnwrappedGiftWrap(event.id, {
              pubkey: innerEvent.pubkey,
              recipientPubkey: '__reaction__', // Marker for reactions
              content: unwrapped.content, // The emoji
              createdAt: innerEvent.created_at
            })
            .catch(() => {})
          // For now, just skip reactions (they're cached for future use)
          return null
        }

        const recipientPubkey = this.getRecipientFromTags(innerEvent.tags) || myPubkey
        const innerEventId = innerEvent.id

        // Cache in both layers (include inner timestamp + inner event ID for dedup)
        setCachedPlaintext(event.id, JSON.stringify({ s: innerEvent.pubkey, r: recipientPubkey, c: unwrapped.content, t: innerEvent.created_at, i: innerEventId }))
        indexedDb
          .putUnwrappedGiftWrap(event.id, {
            pubkey: innerEvent.pubkey,
            recipientPubkey,
            content: unwrapped.content,
            createdAt: innerEvent.created_at,
            innerEventId
          })
          .catch(() => {})

        const seenOnRelays = client.getSeenEventRelayUrls(event.id)
        return {
          id: event.id,
          innerEventId,
          senderPubkey: innerEvent.pubkey,
          recipientPubkey,
          content: unwrapped.content,
          createdAt: innerEvent.created_at,
          encryptionType: 'nip17',
          event,
          decryptedContent: unwrapped.content,
          seenOnRelays: seenOnRelays.length > 0 ? seenOnRelays : undefined
        }
      } else {
        return null
      }
    } catch (error) {
      console.warn('[DM] decryptMessage failed:', event.id,
        error instanceof Error ? error.message : 'Unknown error')
      return null
    }
  }

  /**
   * Unwrap a NIP-59 gift wrap to get the inner message or reaction
   */
  private async unwrapGiftWrap(
    giftWrap: Event,
    encryption: IDMEncryption
  ): Promise<{ content: string; innerEvent: Event; type: 'dm' | 'reaction' } | null> {
    try {
      // Step 1: Decrypt the gift wrap content using NIP-44
      if (!encryption.nip44Decrypt) {
        return null
      }

      const sealJson = await encryption.nip44Decrypt(giftWrap.pubkey, giftWrap.content)
      const seal = JSON.parse(sealJson) as Event

      if (seal.kind !== KIND_SEAL) {
        return null
      }

      // Step 2: Decrypt the seal content using NIP-44
      const innerEventJson = await encryption.nip44Decrypt(seal.pubkey, seal.content)
      const innerEvent = JSON.parse(innerEventJson) as Event

      if (innerEvent.kind === KIND_PRIVATE_DM) {
        return {
          content: innerEvent.content,
          innerEvent,
          type: 'dm'
        }
      } else if (innerEvent.kind === KIND_REACTION) {
        return {
          content: innerEvent.content, // The emoji
          innerEvent,
          type: 'reaction'
        }
      } else {
        // Silently ignore other event types (e.g., read receipts)
        return null
      }
    } catch (error) {
      console.warn('[DM] unwrapGiftWrap failed:', giftWrap.id,
        error instanceof Error ? error.message : 'Unknown error')
      return null
    }
  }

  /**
   * Build a TDirectMessage from an event
   */
  private buildDirectMessage(
    event: Event,
    decryptedContent: string,
    myPubkey: string,
    encryptionType: TDMEncryptionType = 'nip04'
  ): TDirectMessage {
    const recipient = this.getRecipientFromTags(event.tags)
    const isSender = event.pubkey === myPubkey
    const seenOnRelays = client.getSeenEventRelayUrls(event.id)

    return {
      id: event.id,
      senderPubkey: event.pubkey,
      recipientPubkey: recipient || (isSender ? '' : myPubkey),
      content: decryptedContent,
      createdAt: event.created_at,
      encryptionType,
      event,
      decryptedContent,
      seenOnRelays: seenOnRelays.length > 0 ? seenOnRelays : undefined
    }
  }

  /**
   * Send a DM to a recipient
   * When no existing conversation, sends in BOTH formats (NIP-04 and NIP-17)
   * expirationSeconds: optional TTL in seconds (0 = no expiration)
   */
  async sendDM(
    recipientPubkey: string,
    content: string,
    encryption: IDMEncryption,
    relayUrls: string[],
    _preferNip44: boolean,
    existingEncryption: TDMEncryptionType | null,
    expirationSeconds: number = 0
  ): Promise<Event[]> {
    const sentEvents: Event[] = []

    // Get recipient's relays for better delivery
    // Use inbox relays for NIP-17 (where recipient receives messages)
    // Use write relays for NIP-04 (where recipient publishes from)
    const [recipientInboxRelays, recipientWriteRelays] = await Promise.all([
      this.fetchPartnerInboxRelays(recipientPubkey),
      this.fetchPartnerRelays(recipientPubkey)
    ])
    const allRelays = [...new Set([...relayUrls, ...recipientWriteRelays])]
    // NIP-17: send only to recipient's inbox relays, not sender's general relays
    const inboxRelays = recipientInboxRelays.length > 0
      ? recipientInboxRelays
      : relayUrls // last resort fallback

    console.log('[DM] sendDM to', recipientPubkey.slice(0, 8) + '...')
    console.log('[DM] existingEncryption:', existingEncryption)
    console.log('[DM] user relays:', relayUrls)
    console.log('[DM] recipient inbox relays:', recipientInboxRelays)
    console.log('[DM] recipient write relays:', recipientWriteRelays)
    console.log('[DM] merged allRelays (NIP-04):', allRelays)
    console.log('[DM] inboxRelays (NIP-17):', inboxRelays)

    if (existingEncryption === null) {
      // No existing conversation — prefer NIP-17 if available, NIP-04 as fallback (never both)
      if (encryption.nip44Encrypt) {
        try {
          const nip17Event = await this.createAndPublishNip17DM(
            recipientPubkey,
            content,
            encryption,
            inboxRelays,
            relayUrls,
            expirationSeconds
          )
          sentEvents.push(nip17Event)
        } catch (error) {
          console.error('Failed to send NIP-17 DM:', error)
          throw error
        }
      } else {
        try {
          const nip04Event = await this.createAndPublishNip04DM(
            recipientPubkey,
            content,
            encryption,
            allRelays,
            expirationSeconds
          )
          sentEvents.push(nip04Event)
        } catch (error) {
          console.error('Failed to send NIP-04 DM:', error)
          throw error
        }
      }
    } else if (existingEncryption === 'nip04') {
      // Match existing NIP-04 encryption
      try {
        const nip04Event = await this.createAndPublishNip04DM(
          recipientPubkey,
          content,
          encryption,
          allRelays,
          expirationSeconds
        )
        sentEvents.push(nip04Event)
      } catch (error) {
        console.error('Failed to send NIP-04 DM:', error)
        throw error // Re-throw so caller knows it failed
      }
    } else if (existingEncryption === 'nip17') {
      // Match existing NIP-17 encryption - use inbox relays for recipient, sender relays for self
      if (!encryption.nip44Encrypt) {
        throw new Error('Encryption does not support NIP-44')
      }
      try {
        const nip17Event = await this.createAndPublishNip17DM(
          recipientPubkey,
          content,
          encryption,
          inboxRelays,
          relayUrls,
          expirationSeconds
        )
        sentEvents.push(nip17Event)
      } catch (error) {
        console.error('Failed to send NIP-17 DM:', error)
        throw error // Re-throw so caller knows it failed
      }
    }

    return sentEvents
  }

  /**
   * Create and publish a NIP-04 DM (kind 4)
   */
  private async createAndPublishNip04DM(
    recipientPubkey: string,
    content: string,
    encryption: IDMEncryption,
    relayUrls: string[],
    expirationSeconds: number = 0
  ): Promise<VerifiedEvent> {
    const encryptedContent = await encryption.nip04Encrypt(recipientPubkey, content)
    const now = Math.floor(Date.now() / 1000)
    const tags: string[][] = [['p', recipientPubkey]]
    if (expirationSeconds > 0) {
      tags.push(['expiration', String(now + expirationSeconds)])
    }

    const draftEvent: TDraftEvent = {
      kind: KIND_ENCRYPTED_DM,
      created_at: now,
      content: encryptedContent,
      tags
    }

    const signedEvent = await encryption.signEvent(draftEvent)
    await client.publishEvent(relayUrls, signedEvent)
    await indexedDb.putDMEvent(signedEvent)
    await indexedDb.putDecryptedContent(signedEvent.id, content)

    return signedEvent
  }

  /**
   * Create and publish a NIP-17 DM with gift wrapping (kind 14 -> 13 -> 1059).
   * Publishes two gift wraps: one for the recipient and one self-addressed copy
   * so the sender can recover sent messages from relays.
   */
  private async createAndPublishNip17DM(
    recipientPubkey: string,
    content: string,
    encryption: IDMEncryption,
    recipientRelayUrls: string[],
    senderRelayUrls: string[],
    expirationSeconds: number = 0
  ): Promise<VerifiedEvent> {
    if (!encryption.nip44Encrypt) {
      throw new Error('Encryption does not support NIP-44')
    }

    const senderPubkey = encryption.getPublicKey()
    const now = Math.floor(Date.now() / 1000)
    const expirationTag = expirationSeconds > 0
      ? ['expiration', String(now + expirationSeconds)]
      : null

    // Step 1: Create the inner chat message (kind 14) — shared by both wraps
    const innerTags: string[][] = [['p', recipientPubkey]]
    if (expirationTag) innerTags.push(expirationTag)
    const chatMessage: TDraftEvent = {
      kind: KIND_PRIVATE_DM,
      created_at: now,
      content,
      tags: innerTags
    }
    const signedChat = await encryption.signEvent(chatMessage)
    const signedChatJSON = JSON.stringify(signedChat)

    // --- Recipient path ---

    // Seal encrypted with (sender_secret, recipient_pubkey)
    const recipientSealContent = await encryption.nip44Encrypt(recipientPubkey, signedChatJSON)
    const recipientSeal: TDraftEvent = {
      kind: KIND_SEAL,
      created_at: this.randomizeTimestamp(signedChat.created_at),
      content: recipientSealContent,
      tags: []
    }
    const signedRecipientSeal = await encryption.signEvent(recipientSeal)

    // Gift wrap for recipient (expiration on outer lets relays GC)
    const recipientWrapContent = await encryption.nip44Encrypt(recipientPubkey, JSON.stringify(signedRecipientSeal))
    const recipientWrapTags: string[][] = [['p', recipientPubkey]]
    if (expirationTag) recipientWrapTags.push(expirationTag)
    const recipientGiftWrap: TDraftEvent = {
      kind: KIND_GIFT_WRAP,
      created_at: this.randomizeTimestamp(signedRecipientSeal.created_at),
      content: recipientWrapContent,
      tags: recipientWrapTags
    }
    const signedRecipientWrap = await encryption.signEvent(recipientGiftWrap)

    // --- Sender (self) path ---

    // Seal encrypted with (sender_secret, sender_pubkey)
    const senderSealContent = await encryption.nip44Encrypt(senderPubkey, signedChatJSON)
    const senderSeal: TDraftEvent = {
      kind: KIND_SEAL,
      created_at: this.randomizeTimestamp(signedChat.created_at),
      content: senderSealContent,
      tags: []
    }
    const signedSenderSeal = await encryption.signEvent(senderSeal)

    // Gift wrap for sender
    const senderWrapContent = await encryption.nip44Encrypt(senderPubkey, JSON.stringify(signedSenderSeal))
    const senderWrapTags: string[][] = [['p', senderPubkey]]
    if (expirationTag) senderWrapTags.push(expirationTag)
    const senderGiftWrap: TDraftEvent = {
      kind: KIND_GIFT_WRAP,
      created_at: this.randomizeTimestamp(signedSenderSeal.created_at),
      content: senderWrapContent,
      tags: senderWrapTags
    }
    const signedSenderWrap = await encryption.signEvent(senderGiftWrap)

    // Publish both copies
    await client.publishEvent(recipientRelayUrls, signedRecipientWrap)
    await client.publishEvent(senderRelayUrls, signedSenderWrap)

    // Cache both wraps in IndexedDB, including inner event ID for dedup
    const innerEventId = signedChat.id
    await indexedDb.putDMEvent(signedRecipientWrap)
    await indexedDb.putDecryptedContent(signedRecipientWrap.id, content)
    await indexedDb.putUnwrappedGiftWrap(signedRecipientWrap.id, {
      pubkey: senderPubkey, recipientPubkey, content, createdAt: now, innerEventId
    })
    await indexedDb.putDMEvent(signedSenderWrap)
    await indexedDb.putDecryptedContent(signedSenderWrap.id, content)
    await indexedDb.putUnwrappedGiftWrap(signedSenderWrap.id, {
      pubkey: senderPubkey, recipientPubkey, content, createdAt: now, innerEventId
    })

    // Attach innerEventId to the returned event for optimistic dedup
    ;(signedRecipientWrap as any)._innerEventId = innerEventId

    return signedRecipientWrap
  }

  /**
   * Randomize timestamp for privacy (NIP-59)
   */
  private randomizeTimestamp(baseTime: number): number {
    // Random offset between 0 and -2 days (never future, per NIP-59)
    const offset = -Math.floor(Math.random() * 2 * 24 * 60 * 60)
    return baseTime + offset
  }

  /**
   * Fetch partner's write relays for better DM delivery
   */
  async fetchPartnerRelays(pubkey: string): Promise<string[]> {
    try {
      // Try to get relay list from IndexedDB first
      const cachedEvent = await indexedDb.getReplaceableEvent(pubkey, kinds.RelayList)
      if (cachedEvent) {
        return this.parseWriteRelays(cachedEvent)
      }

      // Fetch from user's current relays (no hardcoded fallback to protect privacy)
      const relays = client.currentRelays.length > 0 ? client.currentRelays : []
      if (relays.length === 0) {
        // No relays configured - return empty to signal DM feature unavailable
        return []
      }

      const relayListEvents = await client.fetchEvents(relays, {
        kinds: [kinds.RelayList],
        authors: [pubkey],
        limit: 1
      })

      if (relayListEvents.length > 0) {
        const event = relayListEvents[0]
        await indexedDb.putReplaceableEvent(event)
        return this.parseWriteRelays(event)
      }

      // No relay list found - return empty (don't leak to third-party relay)
      return []
    } catch {
      return []
    }
  }

  /**
   * Fetch partner's inbox (read) relays for NIP-17 DM delivery
   * NIP-65: Inbox relays are where a user receives messages
   */
  async fetchPartnerInboxRelays(pubkey: string): Promise<string[]> {
    try {
      // 1. Try kind 10050 (NIP-17 DM inbox relays) from cache
      const cached10050 = await indexedDb.getReplaceableEvent(pubkey, kinds.DirectMessageRelaysList)
      if (cached10050) {
        const parsed = this.parseDMInboxRelays(cached10050)
        if (parsed.length > 0) return parsed
      }

      // 2. Try kind 10002 (general relay list) from cache as fallback
      const cached10002 = await indexedDb.getReplaceableEvent(pubkey, kinds.RelayList)
      if (cached10002) {
        const parsed = this.parseInboxRelays(cached10002)
        if (parsed.length > 0) return parsed
      }

      // 3. Fetch both from network
      const relays = client.currentRelays.length > 0 ? client.currentRelays : []
      if (relays.length === 0) return client.currentRelays

      const events = await client.fetchEvents(relays, {
        kinds: [kinds.DirectMessageRelaysList, kinds.RelayList],
        authors: [pubkey],
        limit: 2
      })

      let inbox10050: string[] = []
      let inbox10002: string[] = []

      for (const ev of events) {
        await indexedDb.putReplaceableEvent(ev)
        if (ev.kind === kinds.DirectMessageRelaysList) {
          inbox10050 = this.parseDMInboxRelays(ev)
        } else if (ev.kind === kinds.RelayList) {
          inbox10002 = this.parseInboxRelays(ev)
        }
      }

      // Prefer 10050 over 10002
      if (inbox10050.length > 0) return inbox10050
      if (inbox10002.length > 0) return inbox10002

      return client.currentRelays
    } catch {
      return client.currentRelays
    }
  }

  /**
   * Parse write (outbox) relays from kind 10002 event
   */
  private parseWriteRelays(event: Event): string[] {
    const writeRelays: string[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'r') {
        const url = tag[1]
        const scope = tag[2]
        // Include if it's a write relay or has no scope (both)
        if (!scope || scope === 'write') {
          writeRelays.push(url)
        }
      }
    }

    // Return empty if no write relays found (don't fall back to third-party relay)
    return writeRelays
  }

  /**
   * Parse inbox (read) relays from kind 10002 event
   * These are where the user receives DMs
   */
  /**
   * Parse relay URLs from a kind 10050 DM inbox relay list event.
   * NIP-17: tags are ["relay", "wss://..."]
   */
  private parseDMInboxRelays(event: Event): string[] {
    const relays: string[] = []
    for (const tag of event.tags) {
      if (tag[0] === 'relay' && tag[1]) {
        relays.push(tag[1])
      }
    }
    return relays
  }

  private parseInboxRelays(event: Event): string[] {
    const inboxRelays: string[] = []

    for (const tag of event.tags) {
      if (tag[0] === 'r') {
        const url = tag[1]
        const scope = tag[2]
        // Include if it's a read relay or has no scope (both)
        if (!scope || scope === 'read') {
          inboxRelays.push(url)
        }
      }
    }

    return inboxRelays.length > 0 ? inboxRelays : client.currentRelays
  }

  /**
   * Check other relays for an event and return which ones have it
   */
  async checkOtherRelaysForEvent(
    eventId: string,
    knownRelays: string[]
  ): Promise<string[]> {
    const knownSet = new Set(knownRelays.map((r) => r.replace(/\/$/, '')))
    // Check user's current relays that aren't already known
    const relaysToCheck = client.currentRelays.filter(
      (url) => !knownSet.has(url.replace(/\/$/, ''))
    )

    const foundOnRelays: string[] = []

    // Check each relay individually
    await Promise.all(
      relaysToCheck.map(async (relayUrl) => {
        try {
          const events = await client.fetchEvents([relayUrl], {
            ids: [eventId],
            limit: 1
          })
          if (events.length > 0) {
            foundOnRelays.push(relayUrl)
            // Track the event as seen on this relay
            client.trackEventSeenOn(eventId, { url: relayUrl } as any)
          }
        } catch {
          // Relay unreachable, ignore
        }
      })
    )

    return foundOnRelays
  }

  /**
   * Group messages into conversations
   */
  groupMessagesIntoConversations(
    messages: TDirectMessage[],
    myPubkey: string
  ): Map<string, TConversation> {
    const conversations = new Map<string, TConversation>()

    for (const message of messages) {
      // Skip NIRC protocol messages (access requests, invites, etc.)
      if (isNircProtocolMessage(message.content ?? '')) continue

      const partnerPubkey =
        message.senderPubkey === myPubkey ? message.recipientPubkey : message.senderPubkey

      if (!partnerPubkey) continue

      const existing = conversations.get(partnerPubkey)
      if (!existing || message.createdAt > existing.lastMessageAt) {
        conversations.set(partnerPubkey, {
          partnerPubkey,
          lastMessageAt: message.createdAt,
          lastMessagePreview: (message.content ?? '').substring(0, 100),
          unreadCount: 0,
          preferredEncryption: message.encryptionType
        })
      }
    }

    return conversations
  }

  /**
   * Build conversation list from raw events WITHOUT decryption (fast)
   * Only works for NIP-04 events - NIP-17 gift wraps need decryption
   */
  groupEventsIntoConversations(events: Event[], myPubkey: string): Map<string, TConversation> {
    const conversations = new Map<string, TConversation>()

    for (const event of events) {
      // Only process NIP-04 events (kind 4) - we can get metadata without decryption
      if (event.kind !== KIND_ENCRYPTED_DM) continue

      const recipient = this.getRecipientFromTags(event.tags)
      const partnerPubkey = event.pubkey === myPubkey ? recipient : event.pubkey

      if (!partnerPubkey) continue

      const existing = conversations.get(partnerPubkey)
      if (!existing || event.created_at > existing.lastMessageAt) {
        conversations.set(partnerPubkey, {
          partnerPubkey,
          lastMessageAt: event.created_at,
          lastMessagePreview: '', // Skip preview for speed - will be filled on conversation open
          unreadCount: 0,
          preferredEncryption: 'nip04'
        })
      }
    }

    return conversations
  }

  /**
   * Get messages for a specific conversation
   */
  getMessagesForConversation(
    messages: TDirectMessage[],
    partnerPubkey: string,
    myPubkey: string
  ): TDirectMessage[] {
    return messages
      .filter(
        (m) =>
          (m.senderPubkey === partnerPubkey && m.recipientPubkey === myPubkey) ||
          (m.senderPubkey === myPubkey && m.recipientPubkey === partnerPubkey)
      )
      .sort((a, b) => a.createdAt - b.createdAt)
  }

  /**
   * Get the other party's pubkey from a DM event
   */
  private getOtherPartyPubkey(event: Event, myPubkey: string): string | null {
    if (event.pubkey === myPubkey) {
      // I'm the sender, get recipient from tags
      return this.getRecipientFromTags(event.tags)
    } else {
      // I'm the recipient, sender is the pubkey
      return event.pubkey
    }
  }

  /**
   * Get recipient pubkey from event tags
   */
  private getRecipientFromTags(tags: string[][] | undefined): string | null {
    if (!tags) return null
    const pTag = tags.find((t) => t[0] === 'p')
    return pTag ? pTag[1] : null
  }

  /**
   * Subscribe to incoming DMs in real-time
   * Returns a close function to stop the subscription
   */
  subscribeToDMs(
    pubkey: string,
    relayUrls: string[],
    onEvent: (event: Event) => void,
    sinceTimestamp?: number
  ): { close: () => void } {
    // Use provided relays - no hardcoded fallback
    const allRelays = [...new Set(relayUrls)]
    // Use caller-provided timestamp (e.g., last fetched event time) or fall back to 5 minutes ago
    const since = sinceTimestamp ?? Math.floor(Date.now() / 1000) - 300

    // Subscribe to NIP-04 DMs (kind 4) addressed to user
    const nip04Sub = client.subscribe(
      allRelays,
      [
        { kinds: [KIND_ENCRYPTED_DM], '#p': [pubkey], since },
        { kinds: [KIND_ENCRYPTED_DM], authors: [pubkey], since }
      ],
      {
        onevent: (event) => {
          indexedDb.putDMEvent(event).catch(() => {})
          onEvent(event)
        }
      }
    )

    // Subscribe to NIP-17 gift wraps (kind 1059) addressed to user.
    // NIP-59 randomizes outer timestamps up to 2 days in the past,
    // so subtract 3 days to avoid missing fresh replies.
    const giftWrapSince = since - (3 * 24 * 60 * 60)
    const giftWrapSub = client.subscribe(
      allRelays,
      { kinds: [KIND_GIFT_WRAP], '#p': [pubkey], since: giftWrapSince },
      {
        onevent: (event) => {
          indexedDb.putDMEvent(event).catch(() => {})
          onEvent(event)
        }
      }
    )

    return {
      close: async () => {
        const [nip04, giftWrap] = await Promise.all([nip04Sub, giftWrapSub])
        nip04.close()
        giftWrap.close()
      }
    }
  }
}

const dmService = new DMService()
export default dmService

/**
 * Check if a message should be treated as deleted based on the deleted state
 * @param messageId - The event ID of the message
 * @param partnerPubkey - The conversation partner's pubkey
 * @param timestamp - The message timestamp (created_at)
 * @param deletedState - The user's deleted messages state
 * @returns true if the message should be hidden
 */
export function isMessageDeleted(
  messageId: string,
  partnerPubkey: string,
  timestamp: number,
  deletedState: TDMDeletedState | null
): boolean {
  if (!deletedState) return false

  // Check if message ID is explicitly deleted
  if (deletedState.deletedIds.includes(messageId)) {
    return true
  }

  // Check if timestamp falls within any deleted range for this conversation
  const ranges = deletedState.deletedRanges[partnerPubkey]
  if (ranges) {
    for (const range of ranges) {
      if (timestamp >= range.start && timestamp <= range.end) {
        return true
      }
    }
  }

  return false
}

/**
 * Check if a conversation should be hidden based on its last message timestamp
 * A conversation is deleted if its lastMessageAt falls within any deleted range
 * @param partnerPubkey - The conversation partner's pubkey
 * @param lastMessageAt - The timestamp of the last message in the conversation
 * @param deletedState - The user's deleted messages state
 * @returns true if the conversation should be hidden
 */
export function isConversationDeleted(
  partnerPubkey: string,
  lastMessageAt: number,
  deletedState: TDMDeletedState | null
): boolean {
  if (!deletedState) return false

  const ranges = deletedState.deletedRanges[partnerPubkey]
  if (!ranges || ranges.length === 0) return false

  // Check if lastMessageAt falls within any deleted range
  for (const range of ranges) {
    if (lastMessageAt >= range.start && lastMessageAt <= range.end) {
      return true
    }
  }

  return false
}

/**
 * Get the global delete cutoff timestamp.
 * Returns the maximum 'end' timestamp from all "delete all" ranges (where start=0).
 * Gift wraps with created_at <= this value can be skipped without decryption.
 * @param deletedState - The user's deleted messages state
 * @returns The cutoff timestamp, or 0 if no global cutoff exists
 */
export function getGlobalDeleteCutoff(deletedState: TDMDeletedState | null): number {
  if (!deletedState) return 0

  let maxCutoff = 0
  for (const ranges of Object.values(deletedState.deletedRanges)) {
    for (const range of ranges) {
      // Only consider "delete all" ranges (start=0) as global cutoffs
      if (range.start === 0 && range.end > maxCutoff) {
        maxCutoff = range.end
      }
    }
  }
  return maxCutoff
}
