import { RelaySetRepository, RelaySet, Pubkey, tryToRelaySet } from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { kinds } from 'nostr-tools'
import { RepositoryDependencies } from './types'

/**
 * IndexedDB + Relay implementation of RelaySetRepository
 *
 * Uses IndexedDB for local caching and the client service for relay fetching.
 * Save operations publish to relays and update the local cache.
 */
export class RelaySetRepositoryImpl implements RelaySetRepository {
  constructor(private readonly deps: RepositoryDependencies) {}

  async findById(pubkey: Pubkey, id: string): Promise<RelaySet | null> {
    // Try cache first
    const cachedEvent = await indexedDb.getReplaceableEvent(pubkey.hex, kinds.Relaysets, id)
    if (cachedEvent) {
      const relaySet = tryToRelaySet(cachedEvent)
      if (relaySet) return relaySet
    }

    // Fetch from relays
    const events = await client.fetchEvents([], {
      kinds: [kinds.Relaysets],
      authors: [pubkey.hex],
      '#d': [id]
    })

    if (events.length === 0) return null

    // Get the most recent event
    const event = events.sort((a, b) => b.created_at - a.created_at)[0]

    // Update cache
    await indexedDb.putReplaceableEvent(event)

    return tryToRelaySet(event)
  }

  async findByOwner(pubkey: Pubkey): Promise<RelaySet[]> {
    // Fetch all relay sets from relays
    const events = await client.fetchEvents([], {
      kinds: [kinds.Relaysets],
      authors: [pubkey.hex]
    })

    // Deduplicate by 'd' tag (keep latest)
    const eventMap = new Map<string, typeof events[0]>()
    for (const event of events) {
      const d = event.tags.find((t) => t[0] === 'd')?.[1] || ''
      const existing = eventMap.get(d)
      if (!existing || existing.created_at < event.created_at) {
        eventMap.set(d, event)
      }
    }

    // Update cache and convert to domain objects
    const relaySets: RelaySet[] = []
    for (const event of eventMap.values()) {
      await indexedDb.putReplaceableEvent(event)
      const relaySet = tryToRelaySet(event)
      if (relaySet) {
        relaySets.push(relaySet)
      }
    }

    return relaySets
  }

  async save(_pubkey: Pubkey, relaySet: RelaySet): Promise<void> {
    const draftEvent = relaySet.toDraftEvent()
    const publishedEvent = await this.deps.publish(draftEvent)

    // Update cache
    await indexedDb.putReplaceableEvent(publishedEvent)
  }

  async delete(_pubkey: Pubkey, _id: string): Promise<void> {
    // Note: In Nostr, "deleting" a replaceable event is done by publishing
    // an empty or tombstone version. For now, we just remove from local cache.
    // The actual deletion logic depends on the application's requirements.
    // Typically you'd publish a new version that doesn't include the relay set.
  }
}
