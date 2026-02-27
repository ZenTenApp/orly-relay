import NewNotesButton from '@/components/NewNotesButton'
import { Button } from '@/components/ui/button'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import { getEventKey, getKeyFromTag, isMentioningMutedUsers, isReplyNoteEvent } from '@/lib/event'
import { tagNameEquals } from '@/lib/tag'
import { isTouchDevice } from '@/lib/utils'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import { useDeletedEvent } from '@/providers/DeletedEventProvider'
import { TNavigationColumn, useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { useMuteList } from '@/providers/MuteListProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useSocialGraphFilter } from '@/providers/SocialGraphFilterProvider'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { useUserTrust } from '@/providers/UserTrustProvider'
import client from '@/services/client.service'
import threadService from '@/services/thread.service'
import { TFeedSubRequest } from '@/types'
import dayjs from 'dayjs'
import { Event, kinds } from 'nostr-tools'
import { decode } from 'nostr-tools/nip19'
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState
} from 'react'
import { useTranslation } from 'react-i18next'
import PullToRefresh from 'react-simple-pull-to-refresh'
import { toast } from 'sonner'
import NoteCard, { NoteCardLoadingSkeleton } from '../NoteCard'
import NewNotesAboveIndicator from './NewNotesAboveIndicator'
import PinnedNoteCard from '../PinnedNoteCard'

const LIMIT = 200
const ALGO_LIMIT = 500
const SHOW_COUNT = 10

export type TNoteListRef = {
  scrollToTop: (behavior?: ScrollBehavior) => void
  refresh: () => void
}

const NoteList = forwardRef<
  TNoteListRef,
  {
    subRequests: TFeedSubRequest[]
    showKinds?: number[]
    filterMutedNotes?: boolean
    hideReplies?: boolean
    hideUntrustedNotes?: boolean
    hideSpam?: boolean
    areAlgoRelays?: boolean
    showRelayCloseReason?: boolean
    pinnedEventIds?: string[]
    filterFn?: (event: Event) => boolean
    showNewNotesDirectly?: boolean
    navColumn?: TNavigationColumn
    applySocialGraphFilter?: boolean
  }
>(
  (
    {
      subRequests,
      showKinds,
      filterMutedNotes = true,
      hideReplies = false,
      hideUntrustedNotes = false,
      hideSpam = false,
      areAlgoRelays = false,
      showRelayCloseReason = false,
      pinnedEventIds,
      filterFn,
      showNewNotesDirectly = false,
      navColumn = 1,
      applySocialGraphFilter = false
    },
    ref
  ) => {
    const { t } = useTranslation()
    const { startLogin } = useNostr()
    const { isUserTrusted, isSpammer } = useUserTrust()
    const { mutePubkeySet } = useMuteList()
    const { hideContentMentioningMutedUsers } = useContentPolicy()
    const { isEventDeleted } = useDeletedEvent()
    const { isPubkeyAllowed } = useSocialGraphFilter()
    const { autoInsertNewNotes } = useUserPreferences()
    const { offsetSelection, registerLoadMore, unregisterLoadMore } = useKeyboardNavigation()
    const effectiveAutoInsert = showNewNotesDirectly || autoInsertNewNotes
    const [events, setEvents] = useState<Event[]>([])
    const [newEvents, setNewEvents] = useState<Event[]>([])
    const [initialLoading, setInitialLoading] = useState(false)
    const [filtering, setFiltering] = useState(false)
    const [timelineKey, setTimelineKey] = useState<string | undefined>(undefined)
    const [filteredNotes, setFilteredNotes] = useState<
      { key: string; event: Event; reposters: string[] }[]
    >([])
    const [filteredNewEvents, setFilteredNewEvents] = useState<Event[]>([])
    const [refreshCount, setRefreshCount] = useState(0)
    const [newNotesAboveCount, setNewNotesAboveCount] = useState(0)
    const supportTouch = useMemo(() => isTouchDevice(), [])
    const topRef = useRef<HTMLDivElement | null>(null)
    const eventsRef = useRef(events)
    eventsRef.current = events
    const emptyRetryCountRef = useRef(0)
    const isAtTopRef = useRef(true)
    const effectiveAutoInsertRef = useRef(effectiveAutoInsert)
    effectiveAutoInsertRef.current = effectiveAutoInsert

    const shouldHideEvent = useCallback(
      (evt: Event) => {
        const pinnedEventHexIdSet = new Set()
        pinnedEventIds?.forEach((id) => {
          try {
            const { type, data } = decode(id)
            if (type === 'nevent') {
              pinnedEventHexIdSet.add(data.id)
            }
          } catch {
            // ignore
          }
        })

        if (pinnedEventHexIdSet.has(evt.id)) return true
        if (isEventDeleted(evt)) return true
        if (hideUntrustedNotes && !isUserTrusted(evt.pubkey)) return true
        if (filterMutedNotes && mutePubkeySet.has(evt.pubkey)) return true
        if (
          filterMutedNotes &&
          hideContentMentioningMutedUsers &&
          isMentioningMutedUsers(evt, mutePubkeySet)
        ) {
          return true
        }
        if (filterFn && !filterFn(evt)) {
          return true
        }
        // Social graph filter - only apply if enabled for this feed
        if (applySocialGraphFilter && !isPubkeyAllowed(evt.pubkey)) {
          return true
        }

        return false
      },
      [
        hideUntrustedNotes,
        filterMutedNotes,
        mutePubkeySet,
        hideContentMentioningMutedUsers,
        JSON.stringify(pinnedEventIds),
        isEventDeleted,
        filterFn,
        applySocialGraphFilter,
        isPubkeyAllowed
      ]
    )

    useEffect(() => {
      const processEvents = async () => {
        // Store processed event keys to avoid duplicates
        const keySet = new Set<string>()
        // Map to track reposters for each event key
        const repostersMap = new Map<string, Set<string>>()
        // Final list of filtered events
        const filteredEvents: Event[] = []
        const keys: string[] = []

        events.forEach((evt) => {
          const key = getEventKey(evt)
          if (keySet.has(key)) return
          keySet.add(key)

          if (shouldHideEvent(evt)) return
          if (hideReplies && isReplyNoteEvent(evt)) return
          if (evt.kind !== kinds.Repost && evt.kind !== kinds.GenericRepost) {
            filteredEvents.push(evt)
            keys.push(key)
            return
          }

          let targetEventKey: string | undefined
          let eventFromContent: Event | null = null
          const targetTag = evt.tags.find(tagNameEquals('a')) ?? evt.tags.find(tagNameEquals('e'))
          if (targetTag) {
            targetEventKey = getKeyFromTag(targetTag)
          } else {
            // Attempt to extract the target event from the repost content
            if (evt.content) {
              try {
                eventFromContent = JSON.parse(evt.content) as Event
              } catch {
                eventFromContent = null
              }
            }
            if (eventFromContent) {
              if (
                eventFromContent.kind === kinds.Repost ||
                eventFromContent.kind === kinds.GenericRepost
              ) {
                return
              }
              if (shouldHideEvent(evt)) return

              targetEventKey = getEventKey(eventFromContent)
            }
          }

          if (targetEventKey) {
            // Add to reposters map
            const reposters = repostersMap.get(targetEventKey)
            if (reposters) {
              reposters.add(evt.pubkey)
            } else {
              repostersMap.set(targetEventKey, new Set([evt.pubkey]))
            }

            // If the target event is not already included, add it now
            if (!keySet.has(targetEventKey)) {
              filteredEvents.push(evt)
              keys.push(targetEventKey)
              keySet.add(targetEventKey)
            }
          }
        })

        const _filteredNotes = (
          await Promise.all(
            filteredEvents.map(async (evt, i) => {
              if (hideSpam && (await isSpammer(evt.pubkey))) {
                return null
              }
              const key = keys[i]
              return { key, event: evt, reposters: Array.from(repostersMap.get(key) ?? []) }
            })
          )
        ).filter(Boolean) as {
          key: string
          event: Event
          reposters: string[]
        }[]

        setFilteredNotes(_filteredNotes)
      }

      setFiltering(true)
      processEvents().finally(() => setFiltering(false))
    }, [events, shouldHideEvent, hideReplies, isSpammer, hideSpam])

    useEffect(() => {
      const processNewEvents = async () => {
        const keySet = new Set<string>()
        const filteredEvents: Event[] = []

        newEvents.forEach((event) => {
          if (shouldHideEvent(event)) return
          if (hideReplies && isReplyNoteEvent(event)) return

          const key = getEventKey(event)
          if (keySet.has(key)) {
            return
          }
          keySet.add(key)
          filteredEvents.push(event)
        })

        const _filteredNotes = (
          await Promise.all(
            filteredEvents.map(async (evt) => {
              if (hideSpam && (await isSpammer(evt.pubkey))) {
                return null
              }
              return evt
            })
          )
        ).filter(Boolean) as Event[]
        setFilteredNewEvents(_filteredNotes)
      }
      processNewEvents()
    }, [newEvents, shouldHideEvent, isSpammer, hideSpam])

    const scrollToTop = (behavior: ScrollBehavior = 'instant') => {
      setTimeout(() => {
        topRef.current?.scrollIntoView({ behavior, block: 'start' })
      }, 20)
    }

    const refresh = () => {
      scrollToTop()
      setTimeout(() => {
        setRefreshCount((count) => count + 1)
      }, 500)
    }

    useImperativeHandle(ref, () => ({ scrollToTop, refresh }), [])

    useEffect(() => {
      if (!subRequests.length) return

      async function init() {
        setInitialLoading(true)
        setEvents([])
        setNewEvents([])

        if (showKinds?.length === 0 && subRequests.every(({ filter }) => !filter.kinds)) {
          return () => {}
        }

        const preprocessedSubRequests = await Promise.all(
          subRequests.map(async ({ urls, filter }) => {
            const relays = urls.length ? urls : await client.determineRelaysByFilter(filter)
            return {
              urls: relays,
              filter: {
                kinds: showKinds ?? [],
                ...filter,
                limit: areAlgoRelays ? ALGO_LIMIT : LIMIT
              }
            }
          })
        )

        const { closer, timelineKey } = await client.subscribeTimeline(
          preprocessedSubRequests,
          {
            onEvents: (events, eosed) => {
              if (events.length > 0) {
                setEvents(events)
              }
              if (eosed) {
                threadService.addRepliesToThread(events)
                setInitialLoading(false)
              }
            },
            onNew: (event) => {
              if (effectiveAutoInsertRef.current) {
                setEvents((oldEvents) =>
                  oldEvents.some((e) => e.id === event.id) ? oldEvents : [event, ...oldEvents]
                )
                if (!isAtTopRef.current) {
                  setNewNotesAboveCount((c) => c + 1)
                }
              } else {
                setNewEvents((oldEvents) =>
                  [event, ...oldEvents].sort((a, b) => b.created_at - a.created_at)
                )
              }
              threadService.addRepliesToThread([event])
            },
            onClose: (url, reason) => {
              if (!showRelayCloseReason) return
              // ignore reasons from nostr-tools
              if (
                [
                  'closed by caller',
                  'relay connection errored',
                  'relay connection closed',
                  'pingpong timed out',
                  'relay connection closed by us'
                ].includes(reason)
              ) {
                return
              }

              toast.error(`${url}: ${reason}`)
            }
          },
          {
            startLogin,
            needSort: !areAlgoRelays
          }
        )
        setTimelineKey(timelineKey)
        return closer
      }

      const promise = init()
      return () => {
        promise.then((closer) => closer())
      }
    }, [JSON.stringify(subRequests), refreshCount, JSON.stringify(showKinds)])

    const handleLoadMore = useCallback(async () => {
      if (!timelineKey || areAlgoRelays) return false
      const currentEvents = eventsRef.current
      const newEvents = await client.loadMoreTimeline(
        timelineKey,
        currentEvents.length ? currentEvents[currentEvents.length - 1].created_at - 1 : dayjs().unix(),
        LIMIT
      )
      if (newEvents.length === 0) {
        emptyRetryCountRef.current++
        // Allow up to 3 consecutive empty responses before giving up
        if (emptyRetryCountRef.current >= 3) {
          emptyRetryCountRef.current = 0
          return false
        }
        return true
      }
      emptyRetryCountRef.current = 0
      setEvents((oldEvents) => [...oldEvents, ...newEvents])
      return true
    }, [timelineKey, areAlgoRelays])

    const { visibleItems, shouldShowLoadingIndicator, bottomRef } = useInfiniteScroll({
      items: filteredNotes,
      showCount: SHOW_COUNT,
      onLoadMore: handleLoadMore,
      initialLoading
    })

    // Register load more callback for keyboard navigation
    useEffect(() => {
      registerLoadMore(navColumn, handleLoadMore)
      return () => unregisterLoadMore(navColumn)
    }, [navColumn, handleLoadMore, registerLoadMore, unregisterLoadMore])

    // Track whether user is at the top of the feed for live mode indicator
    useEffect(() => {
      const el = topRef.current
      if (!el) return

      const observer = new IntersectionObserver(
        ([entry]) => {
          const atTop = entry.isIntersecting
          isAtTopRef.current = atTop
          if (atTop) {
            setNewNotesAboveCount(0)
          }
        },
        { threshold: 0 }
      )
      observer.observe(el)
      return () => observer.disconnect()
    }, [])

    const showNewEvents = useCallback(() => {
      if (filteredNewEvents.length === 0) return
      // Offset the selection by the number of new items being added at the top
      offsetSelection(navColumn, filteredNewEvents.length)
      setEvents((oldEvents) => [...newEvents, ...oldEvents])
      setNewEvents([])
      setTimeout(() => {
        scrollToTop('smooth')
      }, 0)
    }, [filteredNewEvents.length, navColumn, newEvents, offsetSelection])

    // Shift+Enter to show new notes
    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.shiftKey && e.key === 'Enter' && filteredNewEvents.length > 0) {
          e.preventDefault()
          showNewEvents()
        }
      }
      window.addEventListener('keydown', handleKeyDown)
      return () => window.removeEventListener('keydown', handleKeyDown)
    }, [showNewEvents, filteredNewEvents.length])

    const list = (
      <div className="min-h-screen">
        {pinnedEventIds?.map((id) => <PinnedNoteCard key={id} eventId={id} className="w-full" />)}
        {visibleItems.map(({ key, event, reposters }, index) => (
          <NoteCard
            key={key}
            className="w-full"
            event={event}
            filterMutedNotes={filterMutedNotes}
            reposters={reposters}
            navColumn={navColumn}
            navIndex={index}
          />
        ))}
        <div ref={bottomRef} />
        {shouldShowLoadingIndicator || filtering || initialLoading ? (
          <NoteCardLoadingSkeleton />
        ) : events.length ? (
          <div className="text-center text-sm text-muted-foreground mt-2">{t('no more notes')}</div>
        ) : (
          <div className="flex justify-center w-full mt-2">
            <Button size="lg" onClick={() => setRefreshCount((count) => count + 1)}>
              {t('Reload')}
            </Button>
          </div>
        )}
      </div>
    )

    return (
      <div>
        <div ref={topRef} className="scroll-mt-[calc(6rem+1px)]" />
        {effectiveAutoInsert && (
          <NewNotesAboveIndicator
            count={newNotesAboveCount}
            onClick={() => scrollToTop('smooth')}
          />
        )}
        {supportTouch ? (
          <PullToRefresh
            onRefresh={async () => {
              refresh()
              await new Promise((resolve) => setTimeout(resolve, 1000))
            }}
            pullingContent=""
          >
            {list}
          </PullToRefresh>
        ) : (
          list
        )}
        <div className="h-20" />
        {!effectiveAutoInsert && filteredNewEvents.length > 0 && (
          <NewNotesButton newEvents={filteredNewEvents} onClick={showNewEvents} />
        )}
      </div>
    )
  }
)
NoteList.displayName = 'NoteList'
export default NoteList
