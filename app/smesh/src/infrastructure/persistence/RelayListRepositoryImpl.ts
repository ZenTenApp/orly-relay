import { RelayListRepository, RelayList, Pubkey, tryToRelayList } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of RelayListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class RelayListRepositoryImpl implements RelayListRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<RelayList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.RelayList)
    if (cachedEvent) {
      const relayList = tryToRelayList(cachedEvent)
      if (relayList) return relayList
    }

    // Fetch from relays
    const event = await client.fetchRelayListEvent(pubkey.hex)
    if (!event) return null

    // Update cache
    await indexedDb.putReplaceableEvent(event)

    return tryToRelayList(event)
  }

  async save(relayList: RelayList): Promise<void> {
    const draftEvent = relayList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)
  }
}
