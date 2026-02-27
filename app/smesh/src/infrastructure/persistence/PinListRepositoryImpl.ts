import { PinListRepository, PinList, Pubkey, tryToPinList } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of PinListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class PinListRepositoryImpl implements PinListRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<PinList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.Pinlist)
    if (cachedEvent) {
      const pinList = tryToPinList(cachedEvent)
      if (pinList) return pinList
    }

    // Fetch from relays
    const event = await client.fetchPinListEvent(pubkey.hex)
    if (!event) return null

    // Update cache
    await indexedDb.putReplaceableEvent(event)

    return tryToPinList(event)
  }

  async save(pinList: PinList): Promise<void> {
    const draftEvent = pinList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)
  }
}
