import { MuteListRepository, MuteList, Pubkey, tryToMuteList } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * Function to decrypt private mutes (NIP-04)
 */
export type DecryptFn = (ciphertext: string, pubkey: string) => Promise<string>

/**
 * Function to encrypt private mutes (NIP-04)
 */
export type EncryptFn = (plaintext: string, pubkey: string) => Promise<string>

/**
 * Extended dependencies for MuteList repository
 */
export interface MuteListRepositoryDependencies extends RepositoryDependencies {
  /**
   * NIP-04 decrypt function for private mutes
   */
  decrypt: DecryptFn

  /**
   * NIP-04 encrypt function for private mutes
   */
  encrypt: EncryptFn

  /**
   * The current user's pubkey (for encryption/decryption)
   */
  currentUserPubkey: string
}

/**
 * IndexedDB + Relay implementation of MuteListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Handles NIP-04 encryption/decryption for private mutes.
 */
export class MuteListRepositoryImpl implements MuteListRepository {
  constructor(private readonly deps: MuteListRepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<MuteList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.Mutelist)
    let event = cachedEvent

    // Fetch from relays if not cached
    if (!event) {
      event = await client.fetchMuteListEvent(pubkey.hex)
    }

    if (!event) return null

    // Decrypt private mutes if this is the current user's mute list
    let privateTags: string[][] = []
    if (event.pubkey === this.deps.currentUserPubkey && event.content) {
      try {
        // Try to get decrypted content from cache
        const cacheKey = `mute:${event.id}`
        let decryptedContent = await indexedDb.getDecryptedContent(cacheKey)

        if (!decryptedContent) {
          decryptedContent = await this.deps.decrypt(event.content, event.pubkey)
          await indexedDb.putDecryptedContent(cacheKey, decryptedContent)
        }

        privateTags = JSON.parse(decryptedContent)
      } catch {
        // Decryption failed, proceed with empty private tags
      }
    }

    return tryToMuteList(event, privateTags)
  }

  async save(muteList: MuteList): Promise<void> {
    // Encrypt private mutes
    const privateTags = muteList.toPrivateTags()
    let encryptedContent = ''

    if (privateTags.length > 0) {
      const plaintext = JSON.stringify(privateTags)
      encryptedContent = await this.deps.encrypt(plaintext, this.deps.currentUserPubkey)
    }

    const draftEvent = muteList.toDraftEvent(encryptedContent)
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)

    // Cache the decrypted content
    if (encryptedContent) {
      const cacheKey = `mute:${publishedEvent.id}`
      await indexedDb.putDecryptedContent(cacheKey, JSON.stringify(privateTags))
    }
  }
}
