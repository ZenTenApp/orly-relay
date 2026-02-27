import { BookmarkListRepository, BookmarkList, Pubkey, tryToBookmarkList } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of BookmarkListRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class BookmarkListRepositoryImpl implements BookmarkListRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<BookmarkList | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.BookmarkList)
    if (cachedEvent) {
      const bookmarkList = tryToBookmarkList(cachedEvent)
      if (bookmarkList) return bookmarkList
    }

    // Fetch from relays
    const event = await client.fetchBookmarkListEvent(pubkey.hex)
    if (!event) return null

    // Update cache
    await indexedDb.putReplaceableEvent(event)

    return tryToBookmarkList(event)
  }

  async save(bookmarkList: BookmarkList): Promise<void> {
    const draftEvent = bookmarkList.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)
  }
}
