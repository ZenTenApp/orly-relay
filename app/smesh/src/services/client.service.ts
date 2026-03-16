import { DEPLOYMENT, ExtendedKind } from '@/constants'
import { Pubkey } from '@/domain'
import {
  compareEvents,
  getReplaceableCoordinate,
  getReplaceableCoordinateFromEvent,
  isReplaceableEvent
} from '@/lib/event'
import { getProfileFromEvent, getRelayListFromEvent } from '@/lib/event-metadata'
import { getPubkeysFromPTags, getServersFromServerTags, tagNameEquals } from '@/lib/tag'
import { isLocalNetworkUrl, isWebsocketUrl, normalizeUrl } from '@/lib/url'
import { isSafari } from '@/lib/utils'
import { ISigner, TProfile, TPublishOptions, TRelayList, TSubRequestFilter } from '@/types'
import { sha256 } from '@noble/hashes/sha2'
import DataLoader from 'dataloader'
import dayjs from 'dayjs'
import FlexSearch from 'flexsearch'
import { LRUCache } from 'lru-cache'
import {
  EventTemplate,
  Filter,
  kinds,
  matchFilters,
  Event as NEvent,
  nip19,
  SimplePool,
  VerifiedEvent
} from 'nostr-tools'
import { AbstractRelay } from 'nostr-tools/abstract-relay'
import indexedDb from './indexed-db.service'
import storage from './local-storage.service'
import managedOutboxService from './managed-outbox.service'
import nrcCacheRelayService from './nrc/nrc-cache-relay.service'
import relayDiscoveryService from './relay-discovery.service'
import relayListCacheService from './relay-list-cache.service'
import relayStatsService from './relay-stats.service'

/**
 * Bootstrap relays used when no user relays are available yet.
 * Essential for initial login when we need to fetch the user's relay list.
 */
const BOOTSTRAP_RELAYS = DEPLOYMENT.defaultRelay
  ? [DEPLOYMENT.defaultRelay]
  : [
      'wss://relay.orly.dev/',
      'wss://relay.damus.io/',
      'wss://relay.nostr.band/',
      'wss://nos.lol/',
      'wss://nostr.wine/'
    ]

type TTimelineRef = [string, number]

class ClientService extends EventTarget {
  static instance: ClientService

  signer?: ISigner
  pubkey?: string
  currentRelays: string[] = []
  /** After publishEvent resolves, this promise resolves to URLs of relays that failed */
  lastPublishFailedRelays: Promise<string[]> = Promise.resolve([])
  private pool: SimplePool

  private timelines: Record<
    string,
    | {
        refs: TTimelineRef[]
        filter: TSubRequestFilter
        urls: string[]
      }
    | string[]
    | undefined
  > = {}
  private replaceableEventCacheMap = new Map<string, NEvent>()
  private eventCacheMap = new Map<string, Promise<NEvent | undefined>>()
  private eventDataLoader = new DataLoader<string, NEvent | undefined>(
    (ids) => Promise.all(ids.map((id) => this._fetchEvent(id))),
    { cacheMap: this.eventCacheMap }
  )
  private fetchEventFromBigRelaysDataloader = new DataLoader<string, NEvent | undefined>(
    this.fetchEventsFromBigRelays.bind(this),
    { cache: false, batchScheduleFn: (callback) => setTimeout(callback, 50) }
  )

  private userIndex = new FlexSearch.Index({
    tokenize: 'forward'
  })

  constructor() {
    super()
    this.pool = new SimplePool()
    this.pool.trackRelays = true
  }

  public static getInstance(): ClientService {
    if (!ClientService.instance) {
      ClientService.instance = new ClientService()
      ClientService.instance.init()
    }
    return ClientService.instance
  }

  async init() {
    await relayStatsService.init()
    await indexedDb.iterateProfileEvents((profileEvent) => this.addUsernameToIndex(profileEvent))
  }

  /**
   * Get fallback relays: discovered relays (if cached) or hardcoded bootstrap relays.
   * This ensures we always have relays available even before discovery has run.
   */
  private getFallbackRelays(): string[] {
    const count = storage.getFallbackRelayCount()
    const discovered = relayDiscoveryService.getTopRelays(count)
    return discovered.length > 0 ? discovered : BOOTSTRAP_RELAYS
  }

  async determineTargetRelays(
    event: NEvent,
    { specifiedRelayUrls, additionalRelayUrls }: TPublishOptions = {}
  ) {
    if (event.kind === kinds.Report) {
      const targetEventId = event.tags.find(tagNameEquals('e'))?.[1]
      if (targetEventId) {
        return this.getSeenEventRelayUrls(targetEventId)
      }
    }

    // NRC-only config sync: don't publish config events to relays, only sync via NRC
    const CONFIG_KINDS = [
      kinds.Contacts, // 3
      kinds.Mutelist, // 10000
      kinds.RelayList, // 10002
      30002, // Relay sets
      ExtendedKind.FAVORITE_RELAYS, // 10012
      30078 // Application data (settings sync)
    ]
    if (storage.getNrcOnlyConfigSync() && CONFIG_KINDS.includes(event.kind)) {
      return [] // No relays - NRC will sync this event to paired devices
    }

    const relaySet = new Set<string>()
    if (specifiedRelayUrls?.length) {
      specifiedRelayUrls.forEach((url) => relaySet.add(url))
    } else {
      additionalRelayUrls?.forEach((url) => relaySet.add(url))

      // Get user's relay list
      const userRelayList = await this.fetchRelayList(event.pubkey)

      // Add user's write relays
      userRelayList.write.forEach((url) => relaySet.add(url))

      // For events that mention others, add recipients' write relays
      if (![kinds.Contacts, kinds.Mutelist, ExtendedKind.PINNED_USERS].includes(event.kind)) {
        const mentions: string[] = []
        event.tags.forEach(([tagName, tagValue]) => {
          if (
            ['p', 'P'].includes(tagName) &&
            !!tagValue &&
            Pubkey.isValidHex(tagValue) &&
            !mentions.includes(tagValue)
          ) {
            mentions.push(tagValue)
          }
        })
        if (mentions.length > 0) {
          // Use cached relay lists for recipients
          const recipientRelays = await relayListCacheService.getWriteRelaysForRecipients(mentions)
          recipientRelays.slice(0, 10).forEach((url) => relaySet.add(url))
        }
      }

      // For comments on external content, use relay hints from tags
      if (event.kind === ExtendedKind.COMMENT) {
        const rootITag = event.tags.find(tagNameEquals('I'))
        if (rootITag) {
          // Extract relay hints from e-tags or a-tags in the event
          event.tags.forEach((tag) => {
            if ((tag[0] === 'e' || tag[0] === 'a' || tag[0] === 'E' || tag[0] === 'A') && tag[2]?.startsWith('wss://')) {
              relaySet.add(tag[2])
            }
          })
        }
      }
    }

    // If no relays found, fall back to user's current relays (not hardcoded big relays)
    if (!relaySet.size && this.currentRelays.length > 0) {
      this.currentRelays.forEach((url) => relaySet.add(url))
    }

    // Gate through managed outbox service — user's own write relays bypass gating
    const ownRelays = new Set(this.currentRelays)
    return managedOutboxService.filterRelayUrls(Array.from(relaySet), 'outbox', ownRelays)
  }

  async determineRelaysByFilter(filter: Filter) {
    const ownRelays = new Set(this.currentRelays)
    if (filter.search) {
      return storage.getSearchRelays()
    } else if (filter.authors?.length) {
      const relayLists = await this.fetchRelayLists(filter.authors)
      const relays = Array.from(new Set(relayLists.flatMap((list) => list.write.slice(0, 5))))
      const filtered = managedOutboxService.filterRelayUrls(relays, 'inbox', ownRelays)
      return filtered.length > 0 ? filtered : this.currentRelays
    } else if (filter['#p']?.length) {
      const relayLists = await this.fetchRelayLists(filter['#p'])
      const relays = Array.from(new Set(relayLists.flatMap((list) => list.read.slice(0, 5))))
      const filtered = managedOutboxService.filterRelayUrls(relays, 'inbox', ownRelays)
      return filtered.length > 0 ? filtered : this.currentRelays
    }
    // Use current relays, falling back to discovered/bootstrap relays
    return this.currentRelays.length > 0 ? this.currentRelays : this.getFallbackRelays()
  }

  async publishEvent(
    relayUrls: string[],
    event: NEvent
  ) {
    const uniqueRelayUrls = Array.from(new Set(relayUrls))
    const TIMEOUT_MS = 3_000

    // Track per-relay outcomes: 'success' | Error
    const results = new Map<string, 'pending' | 'success' | Error>(
      uniqueRelayUrls.map((url) => [url, 'pending'])
    )

    // Resolves as soon as ONE relay ACKs, or rejects on timeout / all-failed
    let allSettledPromise: Promise<unknown>

    await new Promise<void>((resolve, reject) => {
      let resolved = false

      const timer = setTimeout(() => {
        if (!resolved) {
          resolved = true
          reject(new Error('Publish timed out: no relay acknowledged within 3 seconds'))
        }
      }, TIMEOUT_MS)

      const onSuccess = (url: string) => {
        results.set(url, 'success')
        relayStatsService.recordPublishSuccess(url)
        if (!resolved) {
          resolved = true
          clearTimeout(timer)
          this.emitNewEvent(event)
          resolve()
        }
      }

      const onError = (url: string, error: Error) => {
        results.set(url, error)
        relayStatsService.recordPublishFailure(url)
        // If all relays have finished and none succeeded, reject
        const pending = [...results.values()].filter((v) => v === 'pending')
        if (pending.length === 0 && !resolved) {
          resolved = true
          clearTimeout(timer)
          const failed = [...results.entries()].filter(([, v]) => v !== 'success')
          reject(
            new AggregateError(
              failed.map(
                ([u, err]) =>
                  new Error(`${u}: ${err instanceof Error ? err.message : String(err)}`)
              )
            )
          )
        }
      }

      allSettledPromise = Promise.allSettled(
        uniqueRelayUrls.map(async (url) => {
          // eslint-disable-next-line @typescript-eslint/no-this-alias
          const that = this
          const relay = await this.pool
            .ensureRelay(url, { connectionTimeout: TIMEOUT_MS })
            .catch((err) => {
              console.debug(`[publishEvent] Failed to connect to ${url}:`, err?.message || err)
              return undefined
            })
          if (!relay) {
            onError(url, new Error('Cannot connect to relay'))
            return
          }

          relay.publishTimeout = TIMEOUT_MS
          let hasAuthed = false

          const attemptPublish = async (): Promise<void> => {
            try {
              await relay.publish(event)
              that.trackEventSeenOn(event.id, relay)
              console.debug(`[publishEvent] Success on ${url}`)
              onSuccess(url)
            } catch (error) {
              if (
                !hasAuthed &&
                error instanceof Error &&
                error.message.startsWith('auth-required') &&
                !!that.signer
              ) {
                console.debug(`[publishEvent] Auth required on ${url}, authenticating...`)
                try {
                  await relay.auth((authEvt: EventTemplate) => that.signer!.signEvent(authEvt))
                  hasAuthed = true
                  return await attemptPublish()
                } catch (authError) {
                  console.debug(`[publishEvent] Auth failed on ${url}:`, authError)
                  onError(url, authError instanceof Error ? authError : new Error(String(authError)))
                }
              } else {
                console.debug(
                  `[publishEvent] Failed on ${url}:`,
                  error instanceof Error ? error.message : error
                )
                onError(url, error instanceof Error ? error : new Error(String(error)))
              }
            }
          }

          return attemptPublish()
        })
      )
    })

    // Store background promise: resolves to failed relay URLs once all relays finish
    this.lastPublishFailedRelays = allSettledPromise!.then(() => {
      return [...results.entries()]
        .filter(([, v]) => v !== 'success')
        .map(([url]) => url)
    })
  }

  emitNewEvent(event: NEvent) {
    this.dispatchEvent(new CustomEvent('newEvent', { detail: event }))
  }

  async signHttpAuth(url: string, method: string, description = '') {
    if (!this.signer) {
      throw new Error('Please login first to sign the event')
    }
    const event = await this.signer?.signEvent({
      content: description,
      kind: kinds.HTTPAuth,
      created_at: dayjs().unix(),
      tags: [
        ['u', url],
        ['method', method]
      ]
    })
    return 'Nostr ' + btoa(JSON.stringify(event))
  }

  /** =========== Timeline =========== */

  private generateTimelineKey(urls: string[], filter: Filter) {
    const stableFilter: any = {}
    Object.entries(filter)
      .sort()
      .forEach(([key, value]) => {
        if (key === 'limit') return
        if (Array.isArray(value)) {
          stableFilter[key] = [...value].sort()
        }
        stableFilter[key] = value
      })
    const paramsStr = JSON.stringify({
      urls: [...urls].sort(),
      filter: stableFilter
    })
    const encoder = new TextEncoder()
    const data = encoder.encode(paramsStr)
    const hashBuffer = sha256(data)
    const hashArray = Array.from(new Uint8Array(hashBuffer))
    return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('')
  }

  private generateMultipleTimelinesKey(subRequests: { urls: string[]; filter: Filter }[]) {
    const keys = subRequests.map(({ urls, filter }) => this.generateTimelineKey(urls, filter))
    const encoder = new TextEncoder()
    const data = encoder.encode(JSON.stringify(keys.sort()))
    const hashBuffer = sha256(data)
    const hashArray = Array.from(new Uint8Array(hashBuffer))
    return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('')
  }

  async subscribeTimeline(
    subRequests: { urls: string[]; filter: TSubRequestFilter }[],
    {
      onEvents,
      onNew,
      onClose
    }: {
      onEvents: (events: NEvent[], eosed: boolean) => void
      onNew: (evt: NEvent) => void
      onClose?: (url: string, reason: string) => void
    },
    {
      startLogin,
      needSort = true
    }: {
      startLogin?: () => void
      needSort?: boolean
    } = {}
  ) {
    const newEventIdSet = new Set<string>()
    const requestCount = subRequests.length
    const threshold = Math.floor(requestCount / 2)
    let events: NEvent[] = []
    let eosedCount = 0

    const subs = await Promise.all(
      subRequests.map(({ urls, filter }) => {
        return this._subscribeTimeline(
          urls,
          filter,
          {
            onEvents: (_events, _eosed) => {
              if (_eosed) {
                eosedCount++
              }

              events = this.mergeTimelines(events, _events)

              if (eosedCount >= threshold) {
                onEvents(events, eosedCount >= requestCount)
              }
            },
            onNew: (evt) => {
              if (newEventIdSet.has(evt.id)) return
              newEventIdSet.add(evt.id)
              onNew(evt)
            },
            onClose
          },
          { startLogin, needSort }
        )
      })
    )

    const key = this.generateMultipleTimelinesKey(subRequests)
    this.timelines[key] = subs.map((sub) => sub.timelineKey)

    return {
      closer: () => {
        onEvents = () => {}
        onNew = () => {}
        subs.forEach((sub) => {
          sub.closer()
        })
      },
      timelineKey: key
    }
  }

  private mergeTimelines(a: NEvent[], b: NEvent[]): NEvent[] {
    if (a.length === 0) return [...b]
    if (b.length === 0) return [...a]

    const result: NEvent[] = []
    let i = 0
    let j = 0
    while (i < a.length && j < b.length) {
      const cmp = compareEvents(a[i], b[j])
      if (cmp > 0) {
        result.push(a[i])
        i++
      } else if (cmp < 0) {
        result.push(b[j])
        j++
      } else {
        result.push(a[i])
        i++
        j++
      }
    }

    return result
  }

  async loadMoreTimeline(key: string, until: number, limit: number) {
    const timeline = this.timelines[key]
    if (!timeline) return []

    if (!Array.isArray(timeline)) {
      return this._loadMoreTimeline(key, until, limit)
    }
    const timelines = await Promise.all(
      timeline.map((key) => this._loadMoreTimeline(key, until, limit))
    )

    const eventIdSet = new Set<string>()
    const events: NEvent[] = []
    timelines.forEach((timeline) => {
      timeline.forEach((evt) => {
        if (eventIdSet.has(evt.id)) return
        eventIdSet.add(evt.id)
        events.push(evt)
      })
    })
    return events.sort((a, b) => b.created_at - a.created_at).slice(0, limit)
  }

  subscribe(
    urls: string[],
    filter: Filter | Filter[],
    {
      onevent,
      oneose,
      onclose,
      startLogin,
      onAllClose
    }: {
      onevent?: (evt: NEvent) => void
      oneose?: (eosed: boolean) => void
      onclose?: (url: string, reason: string) => void
      startLogin?: () => void
      onAllClose?: (reasons: string[]) => void
    }
  ) {
    const relays = Array.from(new Set(urls))
    const filters = Array.isArray(filter) ? filter : [filter]

    // eslint-disable-next-line @typescript-eslint/no-this-alias
    const that = this
    const _knownIds = new Set<string>()
    let startedCount = relays.length
    let eosedCount = 0
    let eosed = false
    let closedCount = 0
    const closeReasons: string[] = []
    const subPromises: Promise<{ close: () => void }>[] = []
    relays.forEach((url) => {
      let hasAuthed = false

      subPromises.push(startSub())

      async function startSub() {
        const relay = await that.pool.ensureRelay(url, { connectionTimeout: 5_000 }).catch(() => {
          return undefined
        })
        // cannot connect to relay
        if (!relay) {
          relayStatsService.recordFetchFailure(url)
          if (!eosed) {
            eosedCount++
            eosed = eosedCount >= startedCount
            oneose?.(eosed)
          }
          return {
            close: () => {}
          }
        }

        return relay.subscribe(filters, {
          receivedEvent: (relay, id) => {
            that.trackEventSeenOn(id, relay)
          },
          alreadyHaveEvent: (id: string) => {
            const have = _knownIds.has(id)
            if (have) {
              return true
            }
            _knownIds.add(id)
            return false
          },
          onevent: (evt: NEvent) => {
            onevent?.(evt)
          },
          oneose: () => {
            relayStatsService.recordFetchSuccess(url)
            // make sure eosed is not called multiple times
            if (eosed) return

            eosedCount++
            eosed = eosedCount >= startedCount
            oneose?.(eosed)
          },
          onclose: (reason: string) => {
            // auth-required
            if (reason.startsWith('auth-required') && !hasAuthed) {
              // already logged in
              if (that.signer) {
                relay
                  .auth(async (authEvt: EventTemplate) => {
                    const evt = await that.signer!.signEvent(authEvt)
                    if (!evt) {
                      throw new Error('sign event failed')
                    }
                    return evt as VerifiedEvent
                  })
                  .then(() => {
                    hasAuthed = true
                    if (!eosed) {
                      startedCount++
                      subPromises.push(startSub())
                    }
                  })
                  .catch(() => {
                    // ignore
                  })
                return
              }

              // open login dialog
              if (startLogin) {
                startLogin()
                return
              }
            }

            // close the subscription — record as fetch failure
            relayStatsService.recordFetchFailure(url)
            closedCount++
            closeReasons.push(reason)
            onclose?.(url, reason)
            if (closedCount >= startedCount) {
              onAllClose?.(closeReasons)
            }
            return
          },
          eoseTimeout: 10_000 // 10s
        })
      }
    })

    const handleNewEventFromInternal = (data: Event) => {
      const customEvent = data as CustomEvent<NEvent>
      const evt = customEvent.detail
      if (!matchFilters(filters, evt)) return

      const id = evt.id
      const have = _knownIds.has(id)
      if (have) return

      _knownIds.add(id)
      onevent?.(evt)
    }

    this.addEventListener('newEvent', handleNewEventFromInternal)

    return {
      close: () => {
        this.removeEventListener('newEvent', handleNewEventFromInternal)
        subPromises.forEach((subPromise) => {
          subPromise
            .then((sub) => {
              sub.close()
            })
            .catch((err) => {
              console.error(err)
            })
        })
      }
    }
  }

  private async _subscribeTimeline(
    urls: string[],
    filter: TSubRequestFilter, // filter with limit,
    {
      onEvents,
      onNew,
      onClose
    }: {
      onEvents: (events: NEvent[], eosed: boolean) => void
      onNew: (evt: NEvent) => void
      onClose?: (url: string, reason: string) => void
    },
    {
      startLogin,
      needSort = true
    }: {
      startLogin?: () => void
      needSort?: boolean
    } = {}
  ) {
    const relays = Array.from(new Set(urls))
    const key = this.generateTimelineKey(relays, filter)
    const timeline = this.timelines[key]
    let cachedEvents: NEvent[] = []
    let since: number | undefined
    if (timeline && !Array.isArray(timeline) && timeline.refs.length && needSort) {
      cachedEvents = (await this.eventDataLoader.loadMany(timeline.refs.map(([id]) => id))).filter(
        (evt) => !!evt && !(evt instanceof Error)
      ) as NEvent[]
      if (cachedEvents.length) {
        onEvents([...cachedEvents], false)
        since = cachedEvents[0].created_at + 1
      }
    }

    // eslint-disable-next-line @typescript-eslint/no-this-alias
    const that = this
    let events: NEvent[] = []
    let eosedAt: number | null = null
    let lastFlushedCount = 0
    let flushTimer: ReturnType<typeof setTimeout> | null = null

    // Progressive flush: emit accumulated events to onEvents before EOSE
    // so the UI can start rendering content as it arrives from the relay
    const scheduleFlush = () => {
      if (flushTimer !== null) return
      flushTimer = setTimeout(() => {
        flushTimer = null
        if (eosedAt || events.length === lastFlushedCount) return
        lastFlushedCount = events.length
        const sorted = needSort
          ? [...events].sort((a, b) => b.created_at - a.created_at).slice(0, filter.limit)
          : [...events]
        const merged = cachedEvents.length > 0
          ? sorted.concat(cachedEvents).slice(0, filter.limit)
          : sorted
        onEvents(merged, false)
      }, 150)
    }

    const subCloser = this.subscribe(relays, since ? { ...filter, since } : filter, {
      startLogin,
      onevent: (evt: NEvent) => {
        that.addEventToCache(evt)
        // not eosed yet, push to events and schedule progressive render
        if (!eosedAt) {
          events.push(evt)
          scheduleFlush()
          return
        }
        // new event
        if (evt.created_at > eosedAt) {
          onNew(evt)
        }

        const timeline = that.timelines[key]
        if (!timeline || Array.isArray(timeline) || !timeline.refs.length) {
          return
        }

        // find the right position to insert
        let idx = 0
        for (const ref of timeline.refs) {
          if (evt.created_at > ref[1] || (evt.created_at === ref[1] && evt.id < ref[0])) {
            break
          }
          // the event is already in the cache
          if (evt.created_at === ref[1] && evt.id === ref[0]) {
            return
          }
          idx++
        }
        // the event is too old, ignore it
        if (idx >= timeline.refs.length) return

        // insert the event to the right position
        timeline.refs.splice(idx, 0, [evt.id, evt.created_at])
      },
      oneose: (eosed) => {
        // Cancel any pending progressive flush — the final sorted result replaces it
        if (flushTimer !== null) {
          clearTimeout(flushTimer)
          flushTimer = null
        }
        if (eosed && !eosedAt) {
          eosedAt = dayjs().unix()
        }
        // (algo feeds) no need to sort and cache
        if (!needSort) {
          return onEvents([...events], !!eosedAt)
        }
        if (!eosed) {
          events = events.sort((a, b) => b.created_at - a.created_at).slice(0, filter.limit)
          return onEvents([...events.concat(cachedEvents).slice(0, filter.limit)], false)
        }

        events = events.sort((a, b) => b.created_at - a.created_at).slice(0, filter.limit)
        const timeline = that.timelines[key]
        // no cache yet
        if (!timeline || Array.isArray(timeline) || !timeline.refs.length) {
          that.timelines[key] = {
            refs: events.map((evt) => [evt.id, evt.created_at]),
            filter,
            urls
          }
          return onEvents([...events], true)
        }

        // Prevent concurrent requests from duplicating the same event
        const firstRefCreatedAt = timeline.refs[0][1]
        const newRefs = events
          .filter((evt) => evt.created_at > firstRefCreatedAt)
          .map((evt) => [evt.id, evt.created_at] as TTimelineRef)

        if (events.length >= filter.limit) {
          // if new refs are more than limit, means old refs are too old, replace them
          timeline.refs = newRefs
          onEvents([...events], true)
        } else {
          // merge new refs with old refs
          timeline.refs = newRefs.concat(timeline.refs)
          onEvents([...events.concat(cachedEvents).slice(0, filter.limit)], true)
        }
      },
      onclose: onClose
    })

    return {
      timelineKey: key,
      closer: () => {
        if (flushTimer !== null) {
          clearTimeout(flushTimer)
          flushTimer = null
        }
        onEvents = () => {}
        onNew = () => {}
        subCloser.close()
      }
    }
  }

  private async _loadMoreTimeline(key: string, until: number, limit: number) {
    const timeline = this.timelines[key]
    if (!timeline || Array.isArray(timeline)) return []

    const { filter, urls, refs } = timeline
    const startIdx = refs.findIndex(([, createdAt]) => createdAt <= until)
    const cachedEvents =
      startIdx >= 0
        ? ((
            await this.eventDataLoader.loadMany(
              refs.slice(startIdx, startIdx + limit).map(([id]) => id)
            )
          ).filter((evt) => !!evt && !(evt instanceof Error)) as NEvent[])
        : []
    if (cachedEvents.length >= limit) {
      return cachedEvents
    }

    until = cachedEvents.length ? cachedEvents[cachedEvents.length - 1].created_at - 1 : until
    limit = limit - cachedEvents.length
    let events = await this.query(urls, { ...filter, until, limit })
    events.forEach((evt) => {
      this.addEventToCache(evt)
    })
    events = events.sort((a, b) => b.created_at - a.created_at).slice(0, limit)

    // Prevent concurrent requests from duplicating the same event
    const lastRefCreatedAt = refs.length > 0 ? refs[refs.length - 1][1] : dayjs().unix()
    timeline.refs.push(
      ...events
        .filter((evt) => evt.created_at < lastRefCreatedAt)
        .map((evt) => [evt.id, evt.created_at] as TTimelineRef)
    )
    return [...cachedEvents, ...events]
  }

  /** =========== Event =========== */

  getSeenEventRelays(eventId: string) {
    return Array.from(this.pool.seenOn.get(eventId)?.values() || [])
  }

  getSeenEventRelayUrls(eventId: string) {
    return this.getSeenEventRelays(eventId).map((relay) => relay.url)
  }

  getEventHints(eventId: string) {
    return this.getSeenEventRelayUrls(eventId).filter((url) => !isLocalNetworkUrl(url))
  }

  getEventHint(eventId: string) {
    return this.getSeenEventRelayUrls(eventId).find((url) => !isLocalNetworkUrl(url)) ?? ''
  }

  trackEventSeenOn(eventId: string, relay: AbstractRelay) {
    let set = this.pool.seenOn.get(eventId)
    if (!set) {
      set = new Set()
      this.pool.seenOn.set(eventId, set)
    }
    set.add(relay)
  }

  private async query(urls: string[], filter: Filter | Filter[], onevent?: (evt: NEvent) => void) {
    return await new Promise<NEvent[]>((resolve) => {
      const events: NEvent[] = []
      const sub = this.subscribe(urls, filter, {
        onevent(evt) {
          onevent?.(evt)
          events.push(evt)
        },
        oneose: (eosed) => {
          if (eosed) {
            sub.close()
            resolve(events)
          }
        },
        onAllClose: () => {
          resolve(events)
        }
      })
    })
  }

  /**
   * Query with NRC cache relay integration
   *
   * Query flow:
   * 1. Check IndexedDB cache first
   * 2. Query NRC cache relays with 400ms timeout
   * 3. Query regular relays for remaining events
   * 4. Push loaded events to cache relays in background
   */
  async queryWithCacheRelays(
    urls: string[],
    filter: Filter | Filter[],
    {
      onevent,
      skipCache = false
    }: {
      onevent?: (evt: NEvent) => void
      skipCache?: boolean
    } = {}
  ): Promise<NEvent[]> {
    const filters = Array.isArray(filter) ? filter : [filter]
    const seenIds = new Set<string>()
    const allEvents: NEvent[] = []

    const addEvent = (evt: NEvent) => {
      if (!seenIds.has(evt.id)) {
        seenIds.add(evt.id)
        allEvents.push(evt)
        onevent?.(evt)
      }
    }

    // Step 1: Check IndexedDB cache
    if (!skipCache) {
      try {
        const cachedEvents = await indexedDb.queryCachedEvents(filters)
        for (const evt of cachedEvents) {
          addEvent(evt as NEvent)
        }
        if (cachedEvents.length > 0) {
          console.log(`[ClientService] Found ${cachedEvents.length} events in IndexedDB cache`)
        }
      } catch (err) {
        console.warn('[ClientService] IndexedDB cache query failed:', err)
      }
    }

    // Step 2: Query NRC cache relays (400ms timeout)
    const cacheRelayResult = await nrcCacheRelayService.queryWithTimeout(filters, 400)
    if (cacheRelayResult.fromCache && cacheRelayResult.events.length > 0) {
      for (const evt of cacheRelayResult.events) {
        addEvent(evt as NEvent)
      }
      console.log(
        `[ClientService] Got ${cacheRelayResult.events.length} events from NRC cache relay`
      )
    }

    // Step 3: Query regular relays for remaining events
    const regularEvents = await this.query(urls, filter)
    const newEvents: NEvent[] = []
    for (const evt of regularEvents) {
      if (!seenIds.has(evt.id)) {
        addEvent(evt)
        newEvents.push(evt)
      }
    }

    // Step 4: Cache new events and push to NRC cache relays in background
    if (newEvents.length > 0) {
      // Cache in IndexedDB
      indexedDb.putCachedEvents(newEvents).catch((err) => {
        console.warn('[ClientService] Failed to cache events:', err)
      })

      // Push to NRC cache relays
      nrcCacheRelayService.queueEventsForPush(newEvents)
    }

    return allEvents
  }

  async fetchEvents(
    urls: string[],
    filter: Filter | Filter[],
    {
      onevent,
      cache = false,
      useCacheRelay = false
    }: {
      onevent?: (evt: NEvent) => void
      cache?: boolean
      useCacheRelay?: boolean
    } = {}
  ) {
    const relays = Array.from(new Set(urls))
    // Use provided relays, or fall back to current user's relays, or bootstrap relays
    let targetRelays = relays.length > 0 ? relays : this.currentRelays

    // Fall back to discovered/bootstrap relays if no relays available (essential for initial login)
    if (targetRelays.length === 0) {
      targetRelays = this.getFallbackRelays()
    }

    // Use cache relay integration if enabled
    if (useCacheRelay) {
      const events = await this.queryWithCacheRelays(targetRelays, filter, { onevent })
      if (cache) {
        events.forEach((evt) => {
          this.addEventToCache(evt)
        })
      }
      return events
    }

    // Standard query path
    const events = await this.query(targetRelays, filter, onevent)
    if (cache) {
      events.forEach((evt) => {
        this.addEventToCache(evt)
      })
    }
    return events
  }

  async fetchEvent(id: string): Promise<NEvent | undefined> {
    if (!/^[0-9a-f]{64}$/.test(id)) {
      let eventId: string | undefined
      let coordinate: string | undefined
      const { type, data } = nip19.decode(id)
      switch (type) {
        case 'note':
          eventId = data
          break
        case 'nevent':
          eventId = data.id
          break
        case 'naddr':
          coordinate = getReplaceableCoordinate(data.kind, data.pubkey, data.identifier)
          break
      }
      if (coordinate) {
        const cache = this.replaceableEventCacheMap.get(coordinate)
        if (cache) {
          return cache
        }
        const indexedDbCache = await indexedDb.getReplaceableEventByCoordinate(coordinate)
        if (indexedDbCache) {
          this.replaceableEventCacheMap.set(coordinate, indexedDbCache)
          return indexedDbCache
        }
      } else if (eventId) {
        const cache = this.eventCacheMap.get(eventId)
        if (cache) {
          return cache
        }
      }
    }
    return this.eventDataLoader.load(id)
  }

  addEventToCache(event: NEvent) {
    this.eventDataLoader.prime(event.id, Promise.resolve(event))
    if (isReplaceableEvent(event.kind)) {
      const coordinate = getReplaceableCoordinateFromEvent(event)
      const cachedEvent = this.replaceableEventCacheMap.get(coordinate)
      if (!cachedEvent || compareEvents(event, cachedEvent) > 0) {
        this.replaceableEventCacheMap.set(coordinate, event)
      }
    }
  }

  getReplaeableEventFromCache(coordinate: string): NEvent | undefined {
    return this.replaceableEventCacheMap.get(coordinate)
  }

  private async fetchEventById(relayUrls: string[], id: string): Promise<NEvent | undefined> {
    const event = await this.fetchEventFromBigRelaysDataloader.load(id)
    if (event) {
      return event
    }

    return this.fetchEventFromRelays(relayUrls, { ids: [id], limit: 1 })
  }

  private async _fetchEvent(id: string): Promise<NEvent | undefined> {
    let filter: Filter | undefined
    let relays: string[] = []
    let author: string | undefined
    if (/^[0-9a-f]{64}$/.test(id)) {
      filter = { ids: [id] }
    } else {
      const { type, data } = nip19.decode(id)
      switch (type) {
        case 'note':
          filter = { ids: [data] }
          break
        case 'nevent':
          filter = { ids: [data.id] }
          if (data.relays) relays = data.relays
          if (data.author) author = data.author
          break
        case 'naddr':
          filter = {
            authors: [data.pubkey],
            kinds: [data.kind],
            limit: 1
          }
          author = data.pubkey
          if (data.identifier) {
            filter['#d'] = [data.identifier]
          }
          if (data.relays) relays = data.relays
      }
    }
    if (!filter) {
      throw new Error('Invalid id')
    }

    let event: NEvent | undefined
    if (filter.ids?.length) {
      event = await this.fetchEventById(relays, filter.ids[0])
    }

    if (!event && author) {
      const relayList = await this.fetchRelayList(author)
      event = await this.fetchEventFromRelays(relayList.write.slice(0, 5), filter)
    }

    // Phase 4: Progressive querying through discovered relays
    if (!event) {
      const alreadyTried = new Set<string>([
        ...this.currentRelays,
        ...this.getFallbackRelays(),
        ...relays
      ])

      const batches = relayDiscoveryService.getRelayBatches(10, 0, 50, alreadyTried)

      for (const batch of batches) {
        if (batch.length === 0) continue

        const batchEvents = await Promise.race([
          this.query(batch, filter),
          new Promise<NEvent[]>((resolve) => setTimeout(() => resolve([]), 5000))
        ])

        if (batchEvents.length > 0) {
          event = batchEvents.sort((a, b) => b.created_at - a.created_at)[0]
          if (event) {
            this.addEventToCache(event)
            break
          }
        }
      }
    }

    if (event && event.id !== id) {
      this.addEventToCache(event)
    }

    // Don't cache failed lookups - allow retry on next request
    if (!event) {
      this.eventCacheMap.delete(id)
    }

    return event
  }

  private async fetchEventFromRelays(relayUrls: string[], filter: Filter) {
    if (!relayUrls.length) return

    const events = await this.query(relayUrls, filter)
    return events.sort((a, b) => b.created_at - a.created_at)[0]
  }

  private async fetchEventsFromBigRelays(ids: readonly string[]) {
    // Use current relays, falling back to discovered/bootstrap relays
    const relays = this.currentRelays.length > 0 ? this.currentRelays : this.getFallbackRelays()

    const events = await this.query(relays, {
      ids: Array.from(new Set(ids)),
      limit: ids.length
    })
    const eventsMap = new Map<string, NEvent>()
    for (const event of events) {
      eventsMap.set(event.id, event)
    }

    return ids.map((id) => eventsMap.get(id))
  }

  /** =========== Following favorite relays =========== */

  private followingFavoriteRelaysCache = new LRUCache<string, Promise<[string, string[]][]>>({
    max: 10,
    fetchMethod: this._fetchFollowingFavoriteRelays.bind(this)
  })

  async fetchFollowingFavoriteRelays(pubkey: string) {
    return this.followingFavoriteRelaysCache.fetch(pubkey)
  }

  private async _fetchFollowingFavoriteRelays(pubkey: string) {
    const fetchNewData = async () => {
      const followings = await this.fetchFollowings(pubkey)
      // Fetch from authors' relays instead of hardcoded big relays
      const events = await this.fetchEvents([], {
        authors: followings,
        kinds: [ExtendedKind.FAVORITE_RELAYS, kinds.Relaysets],
        limit: 1000
      })
      const alreadyExistsFavoriteRelaysPubkeySet = new Set<string>()
      const alreadyExistsRelaySetsPubkeySet = new Set<string>()
      const uniqueEvents: NEvent[] = []
      events
        .sort((a, b) => b.created_at - a.created_at)
        .forEach((event) => {
          if (event.kind === ExtendedKind.FAVORITE_RELAYS) {
            if (alreadyExistsFavoriteRelaysPubkeySet.has(event.pubkey)) return
            alreadyExistsFavoriteRelaysPubkeySet.add(event.pubkey)
          } else if (event.kind === kinds.Relaysets) {
            if (alreadyExistsRelaySetsPubkeySet.has(event.pubkey)) return
            alreadyExistsRelaySetsPubkeySet.add(event.pubkey)
          } else {
            return
          }
          uniqueEvents.push(event)
        })

      const relayMap = new Map<string, Set<string>>()
      uniqueEvents.forEach((event) => {
        event.tags.forEach(([tagName, tagValue]) => {
          if (tagName === 'relay' && tagValue && isWebsocketUrl(tagValue)) {
            const url = normalizeUrl(tagValue)
            relayMap.set(url, (relayMap.get(url) || new Set()).add(event.pubkey))
          }
        })
      })
      const relayMapEntries = Array.from(relayMap.entries())
        .sort((a, b) => b[1].size - a[1].size)
        .map(([url, pubkeys]) => [url, Array.from(pubkeys)]) as [string, string[]][]

      indexedDb.putFollowingFavoriteRelays(pubkey, relayMapEntries)
      return relayMapEntries
    }

    const cached = await indexedDb.getFollowingFavoriteRelays(pubkey)
    if (cached) {
      fetchNewData()
      return cached
    }
    return fetchNewData()
  }

  /** =========== Followings =========== */

  async initUserIndexFromFollowings(pubkey: string, signal: AbortSignal) {
    const followings = await this.fetchFollowings(pubkey, false)
    for (let i = 0; i * 20 < followings.length; i++) {
      if (signal.aborted) return
      await Promise.all(
        followings
          .slice(i * 20, (i + 1) * 20)
          .map((pubkey) => this.fetchProfile(pubkey, false, false))
      )
      await new Promise((resolve) => setTimeout(resolve, 1000))
    }
  }

  /** =========== Profile =========== */

  async searchProfiles(relayUrls: string[], filter: Filter): Promise<TProfile[]> {
    const events = await this.query(relayUrls, {
      ...filter,
      kinds: [kinds.Metadata]
    })

    const profileEvents = events.sort((a, b) => b.created_at - a.created_at)
    await Promise.allSettled(profileEvents.map((profile) => this.addUsernameToIndex(profile)))
    profileEvents.forEach((profile) => this.updateProfileEventCache(profile))
    return profileEvents.map((profileEvent) => getProfileFromEvent(profileEvent))
  }

  async searchNpubsFromLocal(query: string, limit: number = 100) {
    const result = await this.userIndex.searchAsync(query, { limit })
    return result
      .map((pubkey) => Pubkey.tryFromString(pubkey as string)?.npub)
      .filter(Boolean) as string[]
  }

  async searchProfilesFromLocal(query: string, limit: number = 100) {
    const npubs = await this.searchNpubsFromLocal(query, limit)
    const profiles = await Promise.all(npubs.map((npub) => this.fetchProfile(npub)))
    return profiles.filter((profile) => !!profile) as TProfile[]
  }

  private async addUsernameToIndex(profileEvent: NEvent) {
    try {
      const profileObj = JSON.parse(profileEvent.content)
      const text = [
        profileObj.display_name?.trim() ?? '',
        profileObj.name?.trim() ?? '',
        profileObj.nip05
          ?.split('@')
          .map((s: string) => s.trim())
          .join(' ') ?? ''
      ].join(' ')
      if (!text) return

      await this.userIndex.addAsync(profileEvent.pubkey, text)
    } catch {
      return
    }
  }

  private async _fetchProfileEvent(id: string): Promise<NEvent | undefined> {
    let pubkey: string | undefined
    let hintRelays: string[] = []
    if (/^[0-9a-f]{64}$/.test(id)) {
      pubkey = id
    } else {
      const { data, type } = nip19.decode(id)
      switch (type) {
        case 'npub':
          pubkey = data
          break
        case 'nprofile':
          pubkey = data.pubkey
          if (data.relays) hintRelays = data.relays
          break
      }
    }

    if (!pubkey) {
      throw new Error('Invalid id')
    }

    // Phase 3: Improved relay selection for profile fetching
    // 1. Try relay hints from nprofile first (most specific)
    if (hintRelays.length > 0) {
      const profileEvent = await this.fetchEventFromRelays(hintRelays.slice(0, 3), {
        authors: [pubkey],
        kinds: [kinds.Metadata],
        limit: 1
      })
      if (profileEvent) {
        this.addUsernameToIndex(profileEvent)
        indexedDb.putReplaceableEvent(profileEvent)
        return profileEvent
      }
    }

    // 2. Try author's relays from cache/network
    const authorRelayList = await relayListCacheService.getRelayList(pubkey)
    if (authorRelayList && authorRelayList.write.length > 0) {
      const profileEvent = await this.fetchEventFromRelays(authorRelayList.write.slice(0, 5), {
        authors: [pubkey],
        kinds: [kinds.Metadata],
        limit: 1
      })
      if (profileEvent) {
        this.addUsernameToIndex(profileEvent)
        indexedDb.putReplaceableEvent(profileEvent)
        return profileEvent
      }
    }

    // 3. Fall back to current relays (batched for efficiency)
    const profileFromCurrentRelays = await this.replaceableEventFromBigRelaysDataloader.load({
      pubkey,
      kind: kinds.Metadata
    })
    if (profileFromCurrentRelays) {
      this.addUsernameToIndex(profileFromCurrentRelays)
      return profileFromCurrentRelays
    }

    return undefined
  }

  private profileDataloader = new DataLoader<string, TProfile | null, string>(async (ids) => {
    const results = await Promise.allSettled(ids.map((id) => this._fetchProfile(id)))
    return results.map((res) => (res.status === 'fulfilled' ? res.value : null))
  })

  async fetchProfile(
    id: string,
    skipCache = false,
    updateCacheInBackground = true
  ): Promise<TProfile | null> {
    if (skipCache) {
      return this._fetchProfile(id)
    }

    const pk = Pubkey.tryFromString(id)
    if (!pk) throw new Error('Invalid id')
    const localProfileEvent = await indexedDb.getReplaceableEvent(pk.hex, kinds.Metadata)
    if (localProfileEvent) {
      if (updateCacheInBackground) {
        this.profileDataloader.load(id) // update cache in background
      }
      const localProfile = getProfileFromEvent(localProfileEvent)
      return localProfile
    }
    return await this.profileDataloader.load(id)
  }

  private async _fetchProfile(id: string): Promise<TProfile | null> {
    const profileEvent = await this._fetchProfileEvent(id)
    if (profileEvent) {
      return getProfileFromEvent(profileEvent)
    }

    const pk = Pubkey.tryFromString(id)
    if (!pk) return null
    return { pubkey: pk.hex, npub: pk.npub, username: pk.formatNpub(12) }
  }

  async updateProfileEventCache(event: NEvent) {
    await this.updateReplaceableEventFromBigRelaysCache(event)
  }

  /** =========== Relay list =========== */

  async fetchRelayList(pubkey: string): Promise<TRelayList> {
    const [relayList] = await this.fetchRelayLists([pubkey])
    return relayList
  }

  async fetchRelayLists(pubkeys: string[]): Promise<TRelayList[]> {
    const relayEvents = await this.fetchReplaceableEventsFromBigRelays(pubkeys, kinds.RelayList)

    return relayEvents.map((event) => {
      if (event) {
        // Cache the relay list for future use
        relayListCacheService.setRelayList(event)
        return getRelayListFromEvent(event, storage.getFilterOutOnionRelays())
      }
      // No relay list found - use discovered/bootstrap relays as fallback
      const fallback = this.getFallbackRelays()
      return {
        write: fallback,
        read: fallback,
        originalRelays: []
      }
    })
  }

  async forceUpdateRelayListEvent(pubkey: string) {
    await this.replaceableEventFromBigRelaysBatchLoadFn([{ pubkey, kind: kinds.RelayList }])
  }

  async updateRelayListCache(event: NEvent) {
    return await this.updateReplaceableEventFromBigRelaysCache(event)
  }

  /** =========== Replaceable event from big relays dataloader =========== */

  private replaceableEventFromBigRelaysDataloader = new DataLoader<
    { pubkey: string; kind: number },
    NEvent | null,
    string
  >(this.replaceableEventFromBigRelaysBatchLoadFn.bind(this), {
    batchScheduleFn: (callback) => setTimeout(callback, 50),
    maxBatchSize: 500,
    cacheKeyFn: ({ pubkey, kind }) => `${pubkey}:${kind}`
  })

  private async replaceableEventFromBigRelaysBatchLoadFn(
    params: readonly { pubkey: string; kind: number }[]
  ) {
    const groups = new Map<number, string[]>()
    params.forEach(({ pubkey, kind }) => {
      if (!groups.has(kind)) {
        groups.set(kind, [])
      }
      groups.get(kind)!.push(pubkey)
    })

    const eventsMap = new Map<string, NEvent>()
    // Use current relays, falling back to discovered/bootstrap relays
    const relays = this.currentRelays.length > 0 ? this.currentRelays : this.getFallbackRelays()

    await Promise.allSettled(
      Array.from(groups.entries()).map(async ([kind, pubkeys]) => {
        const events = await this.query(relays, {
          authors: pubkeys,
          kinds: [kind]
        })

        for (const event of events) {
          const key = `${event.pubkey}:${event.kind}`
          const existing = eventsMap.get(key)
          if (!existing || existing.created_at < event.created_at) {
            eventsMap.set(key, event)
          }
        }
      })
    )

    return params.map(({ pubkey, kind }) => {
      const key = `${pubkey}:${kind}`
      const event = eventsMap.get(key)
      if (event) {
        indexedDb.putReplaceableEvent(event)
        return event
      } else {
        indexedDb.putNullReplaceableEvent(pubkey, kind)
        return null
      }
    })
  }

  private async fetchReplaceableEventsFromBigRelays(pubkeys: string[], kind: number) {
    const events = await indexedDb.getManyReplaceableEvents(pubkeys, kind)
    const nonExistingPubkeyIndexMap = new Map<string, number>()
    const existingPubkeys: string[] = []
    pubkeys.forEach((pubkey, i) => {
      if (events[i] === undefined) {
        nonExistingPubkeyIndexMap.set(pubkey, i)
      } else {
        existingPubkeys.push(pubkey)
      }
    })
    const newEvents = await this.replaceableEventFromBigRelaysDataloader.loadMany(
      Array.from(nonExistingPubkeyIndexMap.keys()).map((pubkey) => ({ pubkey, kind }))
    )
    newEvents.forEach((event) => {
      if (event && !(event instanceof Error)) {
        const index = nonExistingPubkeyIndexMap.get(event.pubkey)
        if (index !== undefined) {
          events[index] = event
        }
      }
    })

    this.replaceableEventFromBigRelaysDataloader.loadMany(
      existingPubkeys.map((pubkey) => ({ pubkey, kind }))
    ) // update cache in background

    return events
  }

  private async updateReplaceableEventFromBigRelaysCache(event: NEvent) {
    const newEvent = await indexedDb.putReplaceableEvent(event)
    if (newEvent.id !== event.id) {
      return newEvent
    }

    this.replaceableEventFromBigRelaysDataloader.clear({ pubkey: event.pubkey, kind: event.kind })
    this.replaceableEventFromBigRelaysDataloader.prime(
      { pubkey: event.pubkey, kind: event.kind },
      Promise.resolve(event)
    )
    return newEvent
  }

  /** =========== Replaceable event dataloader =========== */

  private replaceableEventDataLoader = new DataLoader<
    { pubkey: string; kind: number; d?: string },
    NEvent | null,
    string
  >(this.replaceableEventBatchLoadFn.bind(this), {
    cacheKeyFn: ({ pubkey, kind, d }) => `${kind}:${pubkey}:${d ?? ''}`
  })

  private async replaceableEventBatchLoadFn(
    params: readonly { pubkey: string; kind: number; d?: string }[]
  ) {
    const groups = new Map<string, { kind: number; d?: string }[]>()
    params.forEach(({ pubkey, kind, d }) => {
      if (!groups.has(pubkey)) {
        groups.set(pubkey, [])
      }
      groups.get(pubkey)!.push({ kind: kind, d })
    })

    const eventMap = new Map<string, NEvent | null>()
    await Promise.allSettled(
      Array.from(groups.entries()).map(async ([pubkey, _params]) => {
        const groupByKind = new Map<number, string[]>()
        _params.forEach(({ kind, d }) => {
          if (!groupByKind.has(kind)) {
            groupByKind.set(kind, [])
          }
          if (d) {
            groupByKind.get(kind)!.push(d)
          }
        })
        const filters = Array.from(groupByKind.entries()).map(
          ([kind, dList]) =>
            (dList.length > 0
              ? {
                  authors: [pubkey],
                  kinds: [kind],
                  '#d': dList
                }
              : { authors: [pubkey], kinds: [kind] }) as Filter
        )
        const relayList = await this.fetchRelayList(pubkey)
        // Use author's relays, falling back to current relays, then discovered/bootstrap relays
        let authorRelays = relayList.write.length > 0 ? relayList.write : this.currentRelays
        if (authorRelays.length === 0) {
          authorRelays = this.getFallbackRelays()
        }
        const relays = authorRelays.slice(0, 5)
        const events = await this.query(relays, filters)

        for (const event of events) {
          const key = getReplaceableCoordinateFromEvent(event)
          const existing = eventMap.get(key)
          if (!existing || existing.created_at < event.created_at) {
            eventMap.set(key, event)
          }
        }
      })
    )

    return params.map(({ pubkey, kind, d }) => {
      const key = `${kind}:${pubkey}:${d ?? ''}`
      const event = eventMap.get(key)
      if (kind === kinds.Pinlist) return event ?? null

      if (event) {
        indexedDb.putReplaceableEvent(event)
        return event
      } else {
        indexedDb.putNullReplaceableEvent(pubkey, kind, d)
        return null
      }
    })
  }

  private async fetchReplaceableEvent(
    pubkey: string,
    kind: number,
    d?: string,
    updateCache = true
  ) {
    const storedEvent = await indexedDb.getReplaceableEvent(pubkey, kind, d)
    if (storedEvent !== undefined) {
      if (updateCache) {
        this.replaceableEventDataLoader.load({ pubkey, kind, d }) // update cache in background
      }
      return storedEvent
    }

    return await this.replaceableEventDataLoader.load({ pubkey, kind, d })
  }

  private async updateReplaceableEventCache(event: NEvent) {
    const newEvent = await indexedDb.putReplaceableEvent(event)
    if (newEvent.id !== event.id) {
      return
    }

    this.replaceableEventDataLoader.clear({ pubkey: event.pubkey, kind: event.kind })
    this.replaceableEventDataLoader.prime(
      { pubkey: event.pubkey, kind: event.kind },
      Promise.resolve(event)
    )
  }

  /** =========== Replaceable event =========== */

  async fetchFollowListEvent(pubkey: string, updateCache = true) {
    return await this.fetchReplaceableEvent(pubkey, kinds.Contacts, undefined, updateCache)
  }

  async fetchFollowings(pubkey: string, updateCache = true) {
    const followListEvent = await this.fetchFollowListEvent(pubkey, updateCache)
    return followListEvent ? getPubkeysFromPTags(followListEvent.tags) : []
  }

  async updateFollowListCache(evt: NEvent) {
    await this.updateReplaceableEventCache(evt)
  }

  async fetchMuteListEvent(pubkey: string) {
    return await this.fetchReplaceableEvent(pubkey, kinds.Mutelist)
  }

  async fetchBookmarkListEvent(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, kinds.BookmarkList)
  }

  async fetchBlossomServerListEvent(pubkey: string) {
    return await this.fetchReplaceableEvent(pubkey, ExtendedKind.BLOSSOM_SERVER_LIST)
  }

  async fetchBlossomServerList(pubkey: string) {
    const evt = await this.fetchBlossomServerListEvent(pubkey)
    return evt ? getServersFromServerTags(evt.tags) : []
  }

  async fetchPinListEvent(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, kinds.Pinlist)
  }

  async fetchRelayListEvent(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, kinds.RelayList)
  }

  async fetchFavoriteRelaysEvent(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, ExtendedKind.FAVORITE_RELAYS)
  }

  async fetchUserEmojiListEvent(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, kinds.UserEmojiList)
  }

  async fetchPinnedUsersList(pubkey: string) {
    return this.fetchReplaceableEvent(pubkey, ExtendedKind.PINNED_USERS)
  }

  async updateBlossomServerListEventCache(evt: NEvent) {
    await this.updateReplaceableEventCache(evt)
  }

  async fetchEmojiSetEvents(pointers: string[], updateCacheInBackground = true) {
    const params = pointers
      .map((pointer) => {
        const [kindStr, pubkey, d = ''] = pointer.split(':')
        if (!pubkey || !kindStr) return null

        const kind = parseInt(kindStr, 10)
        if (kind !== kinds.Emojisets) return null

        return { pubkey, kind, d }
      })
      .filter(Boolean) as { pubkey: string; kind: number; d: string }[]
    return await Promise.all(
      params.map(({ pubkey, kind, d }) =>
        this.fetchReplaceableEvent(pubkey, kind, d, updateCacheInBackground)
      )
    )
  }

  // ================= Utils =================

  async generateSubRequestsForPubkeys(pubkeys: string[], myPubkey?: string | null) {
    // If many websocket connections are initiated simultaneously, it will be
    // very slow on Safari (for unknown reason)
    if (isSafari()) {
      let urls = this.currentRelays
      if (myPubkey) {
        const relayList = await this.fetchRelayList(myPubkey)
        urls = relayList.read.length > 0 ? relayList.read.slice(0, 5) : this.currentRelays
      }
      return [{ urls, filter: { authors: pubkeys } }]
    }

    const relayLists = await this.fetchRelayLists(pubkeys)
    const group: Record<string, Set<string>> = {}
    relayLists.forEach((relayList, index) => {
      relayList.write.slice(0, 4).forEach((url) => {
        if (!group[url]) {
          group[url] = new Set()
        }
        group[url].add(pubkeys[index])
      })
    })

    const relayCount = Object.keys(group).length
    const coveredCount = new Map<string, number>()
    Object.entries(group)
      .sort(([, a], [, b]) => b.size - a.size)
      .forEach(([url, pubkeys]) => {
        if (
          relayCount > 10 &&
          pubkeys.size < 10 &&
          Array.from(pubkeys).every((pubkey) => (coveredCount.get(pubkey) ?? 0) >= 2)
        ) {
          delete group[url]
        } else {
          pubkeys.forEach((pubkey) => {
            coveredCount.set(pubkey, (coveredCount.get(pubkey) ?? 0) + 1)
          })
        }
      })

    return Object.entries(group).map(([url, authors]) => ({
      urls: [url],
      filter: { authors: Array.from(authors) }
    }))
  }
}

const instance = ClientService.getInstance()
export default instance
