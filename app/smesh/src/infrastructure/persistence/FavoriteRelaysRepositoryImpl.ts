import { FavoriteRelaysRepository, FavoriteRelays, Pubkey, tryToFavoriteRelays } from '@/domain'
import { ExtendedKind } from '@/constants'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds, Event } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of FavoriteRelaysRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class FavoriteRelaysRepositoryImpl implements FavoriteRelaysRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findByOwner(pubkey: Pubkey): Promise<FavoriteRelays | null> {
    // Try cache first for the favorite relays event
    let favoriteRelaysEvent = await indexedDb.getReplaceableEvent(
      pubkey.hex,
      ExtendedKind.FAVORITE_RELAYS
    )

    // Fetch from relays if not cached
    if (!favoriteRelaysEvent) {
      favoriteRelaysEvent = await client.fetchFavoriteRelaysEvent(pubkey.hex)
    }

    if (!favoriteRelaysEvent) return null

    // Extract relay set IDs from the event
    const relaySetIds: string[] = []
    favoriteRelaysEvent.tags.forEach(([tagName, tagValue]) => {
      if (tagName === 'a' && tagValue) {
        const [kind, author, relaySetId] = tagValue.split(':')
        if (kind !== kinds.Relaysets.toString()) return
        if (author !== pubkey.hex) return // Only own relay sets for now
        if (!relaySetId || relaySetIds.includes(relaySetId)) return
        relaySetIds.push(relaySetId)
      }
    })

    // Load relay set events
    const relaySetEvents: Event[] = []
    if (relaySetIds.length > 0) {
      // Try cache first
      const cachedEvents = await Promise.all(
        relaySetIds.map((id) => indexedDb.getReplaceableEvent(pubkey.hex, kinds.Relaysets, id))
      )

      // Collect cached events
      const cachedEventMap = new Map<string, Event>()
      const missingIds: string[] = []
      relaySetIds.forEach((id, index) => {
        const cached = cachedEvents[index]
        if (cached) {
          cachedEventMap.set(id, cached)
        } else {
          missingIds.push(id)
        }
      })

      // Fetch missing from relays
      if (missingIds.length > 0) {
        const fetchedEvents = await client.fetchEvents([], {
          kinds: [kinds.Relaysets],
          authors: [pubkey.hex],
          '#d': missingIds
        })

        // Deduplicate and cache
        for (const event of fetchedEvents) {
          const d = event.tags.find((t) => t[0] === 'd')?.[1]
          if (!d) continue
          const existing = cachedEventMap.get(d)
          if (!existing || existing.created_at < event.created_at) {
            cachedEventMap.set(d, event)
            await indexedDb.putReplaceableEvent(event)
          }
        }
      }

      // Collect in original order
      for (const id of relaySetIds) {
        const event = cachedEventMap.get(id)
        if (event) {
          relaySetEvents.push(event)
        }
      }
    }

    // Update favorite relays cache
    await indexedDb.putReplaceableEvent(favoriteRelaysEvent)

    return tryToFavoriteRelays(favoriteRelaysEvent, relaySetEvents)
  }

  async save(favoriteRelays: FavoriteRelays): Promise<void> {
    // First, publish all relay sets
    for (const relaySet of favoriteRelays.getSets()) {
      const relaySetDraftEvent = relaySet.toDraftEvent()
      const publishedRelaySetEvent = await this.deps.publish(relaySetDraftEvent)
      await indexedDb.putReplaceableEvent(publishedRelaySetEvent)
    }

    // Then publish the favorite relays event
    const draftEvent = favoriteRelays.toDraftEvent(favoriteRelays.owner.hex)
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)
  }
}
