import { FollowListRepository, FollowList, Pubkey, tryToFollowList } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { Event as NostrEvent, kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of FollowListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class FollowListRepositoryImpl implements FollowListRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<FollowList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.Contacts)
    if (cachedEvent) {
      const followList = tryToFollowList(cachedEvent)
      if (followList) return followList
    }

    // Fetch from relays
    const event = await client.fetchFollowListEvent(pubkey.hex, true)
    if (!event) return null

    return tryToFollowList(event)
  }

  async save(followList: FollowList): Promise<void> {
    const draftEvent = followList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)

    // Update client's follow list cache for filtering
    await client.updateFollowListCache(publishedEvent)
  }

  /**
   * Save and return the published event
   */
  async saveAndGetEvent(followList: FollowList): Promise<NostrEvent> {
    const draftEvent = followList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    await indexedDb.putReplaceableEvent(publishedEvent)
    await client.updateFollowListCache(publishedEvent)

    return publishedEvent
  }
}
