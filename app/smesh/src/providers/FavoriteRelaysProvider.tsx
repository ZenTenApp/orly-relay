import {
  FavoriteRelays,
  RelaySet,
  tryToFavoriteRelays,
  tryToRelaySet,
  fromRelaySetToLegacy,
  Pubkey,
  RelayUrl
} from '@/domain'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import storage from '@/services/local-storage.service'
import { TRelaySet } from '@/types'
import { Event, kinds } from 'nostr-tools'
import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { useNostr } from './NostrProvider'

type TFavoriteRelaysContext = {
  favoriteRelays: string[]
  addFavoriteRelays: (relayUrls: string[]) => Promise<void>
  deleteFavoriteRelays: (relayUrls: string[]) => Promise<void>
  reorderFavoriteRelays: (reorderedRelays: string[]) => Promise<void>
  relaySets: TRelaySet[]
  createRelaySet: (relaySetName: string, relayUrls?: string[]) => Promise<void>
  addRelaySets: (newRelaySetEvents: Event[]) => Promise<void>
  deleteRelaySet: (id: string) => Promise<void>
  updateRelaySet: (newSet: TRelaySet) => Promise<void>
  reorderRelaySets: (reorderedSets: TRelaySet[]) => Promise<void>
}

const FavoriteRelaysContext = createContext<TFavoriteRelaysContext | undefined>(undefined)

export const useFavoriteRelays = () => {
  const context = useContext(FavoriteRelaysContext)
  if (!context) {
    throw new Error('useFavoriteRelays must be used within a FavoriteRelaysProvider')
  }
  return context
}

export function FavoriteRelaysProvider({ children }: { children: React.ReactNode }) {
  const { favoriteRelaysEvent, updateFavoriteRelaysEvent, pubkey, relayList, publish } = useNostr()
  const [relaySetEvents, setRelaySetEvents] = useState<Event[]>([])

  // Create domain FavoriteRelays from event and relay set events
  const favoriteRelaysAggregate = useMemo(() => {
    if (!favoriteRelaysEvent || !pubkey) return null
    return tryToFavoriteRelays(favoriteRelaysEvent, relaySetEvents)
  }, [favoriteRelaysEvent, relaySetEvents, pubkey])

  // Legacy compatibility: expose relays as string[] for existing consumers
  const favoriteRelays = useMemo(() => {
    if (!favoriteRelaysAggregate) {
      // Fall back to storage-based relay sets
      const storedRelaySets = storage.getRelaySets()
      const relays: string[] = []
      storedRelaySets.forEach(({ relayUrls }) => {
        relayUrls.forEach((url) => {
          if (!relays.includes(url)) {
            relays.push(url)
          }
        })
      })
      return relays
    }
    return favoriteRelaysAggregate.getRelayUrls()
  }, [favoriteRelaysAggregate])

  // Legacy compatibility: expose relay sets as TRelaySet[] for existing consumers
  const relaySets = useMemo((): TRelaySet[] => {
    if (!favoriteRelaysAggregate || !pubkey) return []
    return favoriteRelaysAggregate.getSets().map((set) => fromRelaySetToLegacy(set, pubkey))
  }, [favoriteRelaysAggregate, pubkey])

  // Initialize relay sets from event
  useEffect(() => {
    if (!favoriteRelaysEvent || !pubkey) {
      setRelaySetEvents([])
      return
    }

    const init = async () => {
      // Extract relay set IDs from event
      const relaySetIds: string[] = []
      favoriteRelaysEvent.tags.forEach(([tagName, tagValue]) => {
        if (tagName === 'a' && tagValue) {
          const [kind, author, relaySetId] = tagValue.split(':')
          if (kind !== kinds.Relaysets.toString()) return
          if (author !== pubkey) return // TODO: support others relay sets
          if (!relaySetId || relaySetIds.includes(relaySetId)) return
          relaySetIds.push(relaySetId)
        }
      })

      if (!relaySetIds.length) {
        setRelaySetEvents([])
        return
      }

      // Load from cache first
      const storedRelaySetEvents = await Promise.all(
        relaySetIds.map((id) => indexedDb.getReplaceableEvent(pubkey, kinds.Relaysets, id))
      )
      setRelaySetEvents(storedRelaySetEvents.filter(Boolean) as Event[])

      // Fetch latest from relays
      const newRelaySetEvents = await client.fetchEvents(
        (relayList?.write ?? []).concat(client.currentRelays).slice(0, 5),
        {
          kinds: [kinds.Relaysets],
          authors: [pubkey],
          '#d': relaySetIds
        }
      )

      // Deduplicate by keeping latest version
      const relaySetEventMap = new Map<string, Event>()
      newRelaySetEvents.forEach((event) => {
        const d = event.tags.find((t) => t[0] === 'd')?.[1]
        if (!d) return
        const old = relaySetEventMap.get(d)
        if (!old || old.created_at < event.created_at) {
          relaySetEventMap.set(d, event)
        }
      })

      // Maintain order from relay set IDs
      const uniqueNewRelaySetEvents = relaySetIds
        .map((id, index) => relaySetEventMap.get(id) || storedRelaySetEvents[index])
        .filter(Boolean) as Event[]

      setRelaySetEvents(uniqueNewRelaySetEvents)

      // Cache the events
      await Promise.all(
        uniqueNewRelaySetEvents.map((event) => indexedDb.putReplaceableEvent(event))
      )
    }
    init()
  }, [favoriteRelaysEvent, pubkey, relayList?.write])

  const addFavoriteRelays = async (relayUrls: string[]) => {
    if (!pubkey) return

    const ownerPubkey = Pubkey.fromHex(pubkey)
    const currentAggregate = favoriteRelaysAggregate ?? FavoriteRelays.empty(ownerPubkey)

    // Use domain aggregate to add relays
    const changes = relayUrls
      .map((url) => currentAggregate.addRelayUrl(url))
      .filter((c) => c && c.type !== 'no_change')

    if (changes.length === 0) return

    // Publish the updated favorite relays
    const draftEvent = currentAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(draftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const deleteFavoriteRelays = async (relayUrls: string[]) => {
    if (!pubkey || !favoriteRelaysAggregate) return

    // Use domain aggregate to remove relays
    const changes = relayUrls
      .map((url) => {
        const relay = RelayUrl.tryCreate(url)
        return relay ? favoriteRelaysAggregate.removeRelay(relay) : null
      })
      .filter((c) => c && c.type !== 'no_change')

    if (changes.length === 0) return

    // Publish the updated favorite relays
    const draftEvent = favoriteRelaysAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(draftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const createRelaySet = async (relaySetName: string, relayUrls: string[] = []) => {
    if (!pubkey) return

    // Create relay set using domain aggregate
    const newRelaySet = RelaySet.createWithRelays(relaySetName, relayUrls)

    // Publish the relay set event
    const relaySetDraftEvent = newRelaySet.toDraftEvent()
    const newRelaySetEvent = await publish(relaySetDraftEvent)
    await indexedDb.putReplaceableEvent(newRelaySetEvent)

    // Add the set to favorites
    const ownerPubkey = Pubkey.fromHex(pubkey)
    const currentAggregate = favoriteRelaysAggregate ?? FavoriteRelays.empty(ownerPubkey)
    currentAggregate.addSet(newRelaySet)

    // Publish the updated favorite relays
    const favoriteRelaysDraftEvent = currentAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(favoriteRelaysDraftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const addRelaySets = async (newRelaySetEvents: Event[]) => {
    if (!pubkey) return

    const ownerPubkey = Pubkey.fromHex(pubkey)
    const currentAggregate = favoriteRelaysAggregate ?? FavoriteRelays.empty(ownerPubkey)

    // Convert events to domain objects and add them
    for (const event of newRelaySetEvents) {
      const relaySet = tryToRelaySet(event)
      if (relaySet) {
        currentAggregate.addSet(relaySet)
      }
    }

    // Publish the updated favorite relays
    const favoriteRelaysDraftEvent = currentAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(favoriteRelaysDraftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const deleteRelaySet = async (id: string) => {
    if (!pubkey || !favoriteRelaysAggregate) return

    const change = favoriteRelaysAggregate.removeSet(id)
    if (change.type === 'no_change') return

    // Publish the updated favorite relays
    const draftEvent = favoriteRelaysAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(draftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const updateRelaySet = async (newSet: TRelaySet) => {
    if (!pubkey) return

    // Create domain object from legacy format and publish
    const relaySet = RelaySet.createWithRelays(newSet.name, newSet.relayUrls, newSet.id)
    const draftEvent = relaySet.toDraftEvent()
    const newRelaySetEvent = await publish(draftEvent)
    await indexedDb.putReplaceableEvent(newRelaySetEvent)

    // Update the local relay set events
    setRelaySetEvents((prev) => {
      return prev.map((event) => {
        const d = event.tags.find((t) => t[0] === 'd')?.[1]
        if (d === newSet.id) {
          return newRelaySetEvent
        }
        return event
      })
    })
  }

  const reorderFavoriteRelays = async (reorderedRelays: string[]) => {
    if (!pubkey || !favoriteRelaysAggregate) return

    // Reorder using domain aggregate
    const relayUrls = reorderedRelays
      .map((url) => RelayUrl.tryCreate(url))
      .filter((r): r is RelayUrl => r !== null)
    favoriteRelaysAggregate.reorderRelays(relayUrls)

    // Publish the updated favorite relays
    const draftEvent = favoriteRelaysAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(draftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  const reorderRelaySets = async (reorderedSets: TRelaySet[]) => {
    if (!pubkey || !favoriteRelaysAggregate) return

    // Convert to domain objects and reorder
    const domainSets = reorderedSets
      .map((s) => favoriteRelaysAggregate.getSet(s.id))
      .filter((s): s is RelaySet => s !== undefined)
    favoriteRelaysAggregate.reorderSets(domainSets)

    // Publish the updated favorite relays
    const draftEvent = favoriteRelaysAggregate.toDraftEvent(pubkey)
    const newFavoriteRelaysEvent = await publish(draftEvent)
    updateFavoriteRelaysEvent(newFavoriteRelaysEvent)
  }

  return (
    <FavoriteRelaysContext.Provider
      value={{
        favoriteRelays,
        addFavoriteRelays,
        deleteFavoriteRelays,
        reorderFavoriteRelays,
        relaySets,
        createRelaySet,
        addRelaySets,
        deleteRelaySet,
        updateRelaySet,
        reorderRelaySets
      }}
    >
      {children}
    </FavoriteRelaysContext.Provider>
  )
}
