import { PinnedUsersListRepository, PinnedUsersList, Pubkey, tryToPinnedUsersList } from '@/domain'
import { ExtendedKind } from '@/constants'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { Event as NostrEvent } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * Function to decrypt private pins (NIP-04)
 */
export type DecryptFn = (ciphertext: string, pubkey: string) => Promise<string>

/**
 * Function to encrypt private pins (NIP-04)
 */
export type EncryptFn = (plaintext: string, pubkey: string) => Promise<string>

/**
 * Dependencies for PinnedUsersList repository
 */
export interface PinnedUsersListRepositoryDependencies extends RepositoryDependencies {
  /**
   * NIP-04 decrypt function for private pins
   */
  decrypt: DecryptFn

  /**
   * NIP-04 encrypt function for private pins
   */
  encrypt: EncryptFn

  /**
   * The current user's pubkey (for encryption/decryption)
   */
  currentUserPubkey: string
}

/**
 * IndexedDB + Relay implementation of PinnedUsersListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Handles NIP-04 encryption/decryption for private pins.
 */
export class PinnedUsersListRepositoryImpl implements PinnedUsersListRepository {
  constructor(private readonly deps: PinnedUsersListRepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<PinnedUsersList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, ExtendedKind.PINNED_USERS)
    let event = cachedEvent

    // Fetch from relays if not cached
    if (!event) {
      event = await client.fetchPinnedUsersList(pubkey.hex)
    }

    if (!event) return null

    // Create the aggregate from the event
    const pinnedUsersList = tryToPinnedUsersList(event)
    if (!pinnedUsersList) return null

    // Decrypt private pins if this is the current user's list
    if (event.pubkey === this.deps.currentUserPubkey && event.content) {
      try {
        // Try to get decrypted content from cache
        const cacheKey = `pinned:${event.id}`
        let decryptedContent = await indexedDb.getDecryptedContent(cacheKey)

        if (!decryptedContent) {
          decryptedContent = await this.deps.decrypt(event.content, event.pubkey)
          await indexedDb.putDecryptedContent(cacheKey, decryptedContent)
        }

        const privateTags = JSON.parse(decryptedContent)
        pinnedUsersList.setPrivatePins(privateTags)
      } catch {
        // Decryption failed, proceed with empty private pins
      }
    }

    return pinnedUsersList
  }

  async save(pinnedUsersList: PinnedUsersList): Promise<void> {
    // Encrypt private pins
    const privateTags = pinnedUsersList.toPrivateTags()
    let encryptedContent = ''

    if (privateTags.length > 0) {
      const plaintext = JSON.stringify(privateTags)
      encryptedContent = await this.deps.encrypt(plaintext, this.deps.currentUserPubkey)
    }

    // Set encrypted content on the aggregate before creating draft
    pinnedUsersList.setEncryptedContent(encryptedContent)

    const draftEvent = pinnedUsersList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)

    // Cache the decrypted content
    if (encryptedContent) {
      const cacheKey = `pinned:${publishedEvent.id}`
      await indexedDb.putDecryptedContent(cacheKey, JSON.stringify(privateTags))
    }
  }

  /**
   * Save and return the published event (for UI state updates)
   */
  async saveAndGetEvent(pinnedUsersList: PinnedUsersList): Promise<{ event: NostrEvent; privateTags: string[][] }> {
    const privateTags = pinnedUsersList.toPrivateTags()
    let encryptedContent = ''

    if (privateTags.length > 0) {
      const plaintext = JSON.stringify(privateTags)
      encryptedContent = await this.deps.encrypt(plaintext, this.deps.currentUserPubkey)
    }

    pinnedUsersList.setEncryptedContent(encryptedContent)

    const draftEvent = pinnedUsersList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    await indexedDb.putReplaceableEvent(publishedEvent)

    if (encryptedContent) {
      const cacheKey = `pinned:${publishedEvent.id}`
      await indexedDb.putDecryptedContent(cacheKey, JSON.stringify(privateTags))
    }

    return { event: publishedEvent, privateTags }
  }
}
