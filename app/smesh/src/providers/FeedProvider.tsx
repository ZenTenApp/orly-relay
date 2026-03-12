import { getRelaySetFromEvent } from '@/lib/event-metadata'
import { isWebsocketUrl, normalizeUrl } from '@/lib/url'
import indexedDb from '@/services/indexed-db.service'
import storage from '@/services/local-storage.service'
import { TFeedInfo, TFeedType } from '@/types'
import { kinds } from 'nostr-tools'
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { useFavoriteRelays } from './FavoriteRelaysProvider'
import { useNostr } from './NostrProvider'

// Domain imports
import {
  Feed,
  FeedType,
  ContentFilter,
  fromFeed,
  toRelayUrls,
  fromRelayUrls,
  FeedSwitched
} from '@/domain/feed'
import { Pubkey } from '@/domain/shared/value-objects/Pubkey'
import { RelayUrl } from '@/domain/shared/value-objects/RelayUrl'
import { eventDispatcher } from '@/domain/shared'
import { setSocialHandlerCallbacks } from '@/application/handlers/SocialEventHandlers'

/**
 * Feed context type
 *
 * Provides both legacy TFeedInfo for backward compatibility
 * and new domain model access.
 */
type TFeedContext = {
  // Legacy interface (for backward compatibility)
  feedInfo: TFeedInfo
  relayUrls: string[]
  isReady: boolean
  switchFeed: (
    feedType: TFeedType | null,
    options?: { activeRelaySetId?: string; pubkey?: string; relay?: string | null }
  ) => Promise<void>
  markFeedLoaded: () => void

  // Domain model interface
  feed: Feed | null
  contentFilter: ContentFilter
  updateContentFilter: (filter: ContentFilter) => void
  refresh: () => void
}

const FeedContext = createContext<TFeedContext | undefined>(undefined)

export const useFeed = () => {
  const context = useContext(FeedContext)
  if (!context) {
    throw new Error('useFeed must be used within a FeedProvider')
  }
  return context
}

export function FeedProvider({ children }: { children: React.ReactNode }) {
  const { pubkey, isInitialized } = useNostr()
  const { relaySets } = useFavoriteRelays()

  // Domain state
  const [feed, setFeed] = useState<Feed | null>(null)
  const [contentFilter, setContentFilter] = useState<ContentFilter>(ContentFilter.default())

  // Legacy state (derived from domain state)
  const [isReady, setIsReady] = useState(false)
  const feedRef = useRef<Feed | null>(feed)

  // Derive legacy feedInfo from domain Feed
  const feedInfo = useMemo<TFeedInfo>(() => {
    return feed ? fromFeed(feed) : null
  }, [feed])

  // Derive relayUrls from domain Feed
  const relayUrls = useMemo<string[]>(() => {
    return feed ? fromRelayUrls(feed.relayUrls) : []
  }, [feed])

  // Get owner Pubkey from string
  const ownerPubkey = useMemo(() => {
    return pubkey ? Pubkey.tryFromString(pubkey) : null
  }, [pubkey])

  // Initialize feed on mount
  useEffect(() => {
    const init = async () => {
      if (!isInitialized) {
        return
      }

      let storedFeedInfo: TFeedInfo = null
      if (pubkey) {
        const retrieved = storage.getFeedInfo(pubkey)
        storedFeedInfo = retrieved ?? null
        if (!storedFeedInfo) {
          storedFeedInfo = { feedType: 'following' }
        }
      }

      if (storedFeedInfo?.feedType === 'relays') {
        return await switchFeed('relays', { activeRelaySetId: storedFeedInfo.id })
      }

      if (storedFeedInfo?.feedType === 'relay') {
        return await switchFeed('relay', { relay: storedFeedInfo.id })
      }

      if (storedFeedInfo?.feedType === 'following' && pubkey) {
        return await switchFeed('following', { pubkey })
      }

      if (storedFeedInfo?.feedType === 'pinned' && pubkey) {
        return await switchFeed('pinned', { pubkey })
      }

      setIsReady(true)
    }

    init()
  }, [pubkey, isInitialized])

  // Retry 'relays' feed when relay sets become available after initial load
  useEffect(() => {
    if (!isInitialized || !pubkey || !relaySets.length) return
    // Only retry if we don't have a feed or have a 'relays' stored but no active feed
    const storedFeedInfo = storage.getFeedInfo(pubkey)
    if (storedFeedInfo?.feedType === 'relays' && !feed) {
      switchFeed('relays', { activeRelaySetId: storedFeedInfo.id })
    }
  }, [relaySets, isInitialized, pubkey, feed])

  // Wire up event handler callbacks
  useEffect(() => {
    setSocialHandlerCallbacks({
      onFeedRefreshNeeded: () => {
        // Trigger feed refresh when follow list changes
        if (feed) {
          const event = feed.refresh()
          eventDispatcher.dispatch(event)
        }
      },
      onRefilterNeeded: () => {
        // Content filter hasn't changed, but mute list has
        // The filter will pick up new mutes on next render
        setContentFilter((prev) => prev)
      }
    })
  }, [feed])

  /**
   * Switch to a different feed type
   */
  const switchFeed = useCallback(async (
    feedType: TFeedType | null,
    options: {
      activeRelaySetId?: string | null
      pubkey?: string | null
      relay?: string | null
    } = {}
  ) => {
    const previousFeed = feedRef.current

    if (!feedType) {
      setFeed(null)
      feedRef.current = null
      setIsReady(true)
      return
    }

    setIsReady(false)

    let newFeed: Feed | null = null
    let newFeedType: FeedType | null = null

    if (feedType === 'relay') {
      const normalizedUrl = normalizeUrl(options.relay ?? '')
      if (!normalizedUrl || !isWebsocketUrl(normalizedUrl)) {
        setIsReady(true)
        return
      }

      const relayUrl = RelayUrl.tryCreate(normalizedUrl)
      if (!relayUrl) {
        setIsReady(true)
        return
      }

      newFeed = Feed.singleRelay(relayUrl)
      newFeedType = FeedType.relay(normalizedUrl)
    } else if (feedType === 'relays') {
      const relaySetId = options.activeRelaySetId ?? (relaySets.length > 0 ? relaySets[0].id : null)
      if (!relaySetId || !pubkey || !ownerPubkey) {
        setIsReady(true)
        return
      }

      let relaySet =
        relaySets.find((set) => set.id === relaySetId) ??
        (relaySets.length > 0 ? relaySets[0] : null)

      if (!relaySet) {
        const storedRelaySetEvent = await indexedDb.getReplaceableEvent(
          pubkey,
          kinds.Relaysets,
          relaySetId
        )
        if (storedRelaySetEvent) {
          relaySet = getRelaySetFromEvent(storedRelaySetEvent)
        }
      }

      if (relaySet) {
        const relayUrlObjects = toRelayUrls(relaySet.relayUrls)
        newFeed = Feed.relays(ownerPubkey, relaySet.id, relayUrlObjects)
        newFeedType = FeedType.relays(relaySet.id)
      }
    } else if (feedType === 'following') {
      if (!options.pubkey || !ownerPubkey) {
        setIsReady(true)
        return
      }
      newFeed = Feed.following(ownerPubkey)
      newFeedType = FeedType.following()
    } else if (feedType === 'pinned') {
      if (!options.pubkey || !ownerPubkey) {
        setIsReady(true)
        return
      }
      newFeed = Feed.pinned(ownerPubkey)
      newFeedType = FeedType.pinned()
    }

    if (newFeed && newFeedType) {
      // Update state
      setFeed(newFeed)
      feedRef.current = newFeed

      // Persist to storage
      const newFeedInfo = fromFeed(newFeed)
      storage.setFeedInfo(newFeedInfo, pubkey)

      // Dispatch domain event
      const event = new FeedSwitched(
        ownerPubkey,
        previousFeed?.type ?? null,
        newFeedType,
        newFeedType.relaySetId ?? undefined
      )
      eventDispatcher.dispatch(event)
      setIsReady(true)
    } else {
      // No feed could be created — mark ready immediately
      setIsReady(true)
    }
  }, [pubkey, ownerPubkey, relaySets])

  /**
   * Signal that the feed's initial data has loaded (called by NoteList)
   */
  const markFeedLoaded = useCallback(() => {
    setIsReady(true)
  }, [])

  /**
   * Update content filter settings
   */
  const updateContentFilter = useCallback((newFilter: ContentFilter) => {
    setContentFilter(newFilter)

    // If we have a feed, emit the domain event
    if (feed && ownerPubkey) {
      const event = feed.updateContentFilter(newFilter)
      eventDispatcher.dispatch(event)
    }
  }, [feed, ownerPubkey])

  /**
   * Refresh the current feed
   */
  const refresh = useCallback(() => {
    if (feed) {
      const event = feed.refresh()
      eventDispatcher.dispatch(event)
    }
  }, [feed])

  const value = useMemo<TFeedContext>(() => ({
    // Legacy interface
    feedInfo,
    relayUrls,
    isReady,
    switchFeed,
    markFeedLoaded,

    // Domain model interface
    feed,
    contentFilter,
    updateContentFilter,
    refresh
  }), [feedInfo, relayUrls, isReady, switchFeed, markFeedLoaded, feed, contentFilter, updateContentFilter, refresh])

  return (
    <FeedContext.Provider value={value}>
      {children}
    </FeedContext.Provider>
  )
}
