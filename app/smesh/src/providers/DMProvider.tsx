import { ApplicationDataKey } from '@/constants'
import { createDeletedMessagesDraftEvent } from '@/lib/draft-event'
import dmService, {
  clearPlaintextCache,
  decryptMessagesInBatches,
  getGlobalDeleteCutoff,
  IDMEncryption,
  isConversationDeleted,
  isMessageDeleted,
  isNircProtocolMessage
} from '@/services/dm.service'
import indexedDb from '@/services/indexed-db.service'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import client from '@/services/client.service'
import { TConversation, TDirectMessage, TDMDeletedState, TDMEncryptionType } from '@/types'
import { Event, kinds } from 'nostr-tools'
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { useNostr } from './NostrProvider'

type TDMContext = {
  conversations: TConversation[]
  currentConversation: string | null
  messages: TDirectMessage[]
  isLoading: boolean
  isLoadingConversation: boolean
  error: string | null
  selectConversation: (partnerPubkey: string | null) => void
  startConversation: (partnerPubkey: string) => void
  sendMessage: (content: string, customRelayUrls?: string[], expirationSeconds?: number) => Promise<void>
  refreshConversations: () => Promise<void>
  reloadConversation: () => void
  loadMoreConversations: () => Promise<void>
  hasMoreConversations: boolean
  preferNip44: boolean
  setPreferNip44: (prefer: boolean) => void
  isNewConversation: boolean
  clearNewConversationFlag: () => void
  dismissProvisionalConversation: () => void
  totalUnreadCount: number
  hasNewMessages: boolean
  markInboxAsSeen: () => void
  selectedMessages: Set<string>
  isSelectionMode: boolean
  toggleMessageSelection: (messageId: string) => void
  selectAllMessages: () => void
  clearSelection: () => void
  deleteSelectedMessages: () => Promise<void>
  deleteAllInConversation: () => Promise<void>
  undeleteAllInConversation: () => Promise<void>
}

const DMContext = createContext<TDMContext | undefined>(undefined)

export const useDM = () => {
  const context = useContext(DMContext)
  if (!context) {
    throw new Error('useDM must be used within a DMProvider')
  }
  return context
}

// Merge two message arrays, dedupe by ID, sort by timestamp
function mergeAndDedupe(
  existing: TDirectMessage[],
  incoming: TDirectMessage[]
): TDirectMessage[] {
  const ids = new Set(existing.map((m) => m.id))
  const innerIds = new Set(existing.map((m) => m.innerEventId).filter(Boolean))
  const merged = [...existing]
  for (const msg of incoming) {
    if (ids.has(msg.id)) continue
    if (msg.innerEventId && innerIds.has(msg.innerEventId)) continue
    merged.push(msg)
    ids.add(msg.id)
    if (msg.innerEventId) innerIds.add(msg.innerEventId)
  }
  return merged.sort((a, b) => a.createdAt - b.createdAt)
}

export function DMProvider({ children }: { children: React.ReactNode }) {
  const {
    pubkey,
    relayList,
    nip04Encrypt,
    nip04Decrypt,
    nip44Encrypt,
    nip44Decrypt,
    hasNip44Support,
    signEvent
  } = useNostr()

  const [conversations, setConversations] = useState<TConversation[]>([])
  const [allConversations, setAllConversations] = useState<TConversation[]>([])
  const [currentConversation, setCurrentConversation] = useState<string | null>(null)
  const [messages, setMessages] = useState<TDirectMessage[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingConversation, setIsLoadingConversation] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [preferNip44, setPreferNip44State] = useState(() => storage.getPreferNip44())
  const [hasMoreConversations, setHasMoreConversations] = useState(false)
  const [isNewConversation, setIsNewConversation] = useState(false)
  const [provisionalPubkey, setProvisionalPubkey] = useState<string | null>(null)
  const [deletedState, setDeletedState] = useState<TDMDeletedState | null>(null)
  const [selectedMessages, setSelectedMessages] = useState<Set<string>>(new Set())
  const [isSelectionMode, setIsSelectionMode] = useState(false)
  const [lastSeenTimestamp, setLastSeenTimestamp] = useState<number>(() =>
    pubkey ? storage.getDMLastSeenTimestamp(pubkey) : 0
  )
  const [newestIncomingTimestamp, setNewestIncomingTimestamp] = useState(0)
  const [reloadTrigger, setReloadTrigger] = useState(0)
  const CONVERSATIONS_PER_PAGE = 100

  const loadingConversationRef = useRef<string | null>(null)
  const hasInitializedRef = useRef(false)
  const lastPubkeyRef = useRef<string | null>(null)
  const dmSubscriptionRef = useRef<{ close: () => void } | null>(null)
  const currentConversationRef = useRef<string | null>(null)

  // Keep ref in sync
  useEffect(() => {
    currentConversationRef.current = currentConversation
  }, [currentConversation])

  const encryption: IDMEncryption | null = useMemo(() => {
    if (!pubkey) return null
    return {
      nip04Encrypt,
      nip04Decrypt,
      nip44Encrypt: hasNip44Support ? nip44Encrypt : undefined,
      nip44Decrypt: hasNip44Support ? nip44Decrypt : undefined,
      signEvent,
      getPublicKey: () => pubkey
    }
  }, [pubkey, nip04Encrypt, nip04Decrypt, nip44Encrypt, nip44Decrypt, hasNip44Support, signEvent])

  // Initialize: load deleted state, conversations, start subscription
  useEffect(() => {
    if (pubkey && encryption) {
      if (hasInitializedRef.current && lastPubkeyRef.current === pubkey) {
        return
      }
      hasInitializedRef.current = true
      lastPubkeyRef.current = pubkey

      const savedTimestamp = storage.getDMLastSeenTimestamp(pubkey)
      if (savedTimestamp > 0) {
        setLastSeenTimestamp(savedTimestamp)
      }

      const initialize = async () => {
        // Load deleted state
        let currentDeletedState: TDMDeletedState = { deletedIds: [], deletedRanges: {} }
        const cached = await indexedDb.getDeletedMessagesState(pubkey)
        if (cached) {
          currentDeletedState = cached
          setDeletedState(cached)
        } else {
          setDeletedState(currentDeletedState)
        }

        try {
          const relayUrls = relayList?.read.length ? relayList.read : client.currentRelays
          const events = await client.fetchEvents(relayUrls, {
            kinds: [kinds.Application],
            authors: [pubkey],
            '#d': [ApplicationDataKey.DM_DELETED_MESSAGES],
            limit: 1
          })
          if (events.length > 0) {
            try {
              const parsedState = JSON.parse(events[0].content) as TDMDeletedState
              currentDeletedState = parsedState
              setDeletedState(parsedState)
              await indexedDb.putDeletedMessagesState(pubkey, parsedState)
            } catch {
              // Invalid JSON
            }
          }
        } catch {
          // Relay fetch failed
        }

        // Load cached conversations
        const cachedConvs = await indexedDb.getDMConversations(pubkey)
        if (cachedConvs.length > 0) {
          const convs: TConversation[] = cachedConvs
            .filter((c) => c.partnerPubkey && typeof c.partnerPubkey === 'string')
            .filter((c) => !isConversationDeleted(c.partnerPubkey, c.lastMessageAt, currentDeletedState))
            .map((c) => ({
              partnerPubkey: c.partnerPubkey,
              lastMessageAt: c.lastMessageAt,
              lastMessagePreview: c.lastMessagePreview || '',
              unreadCount: 0,
              preferredEncryption: c.encryptionType
            }))
          setAllConversations(convs)
          setConversations(convs.slice(0, CONVERSATIONS_PER_PAGE))
          setHasMoreConversations(convs.length > CONVERSATIONS_PER_PAGE)
        }

        // Background refresh
        backgroundRefreshConversations()

        // Start real-time subscription
        if (dmSubscriptionRef.current) {
          dmSubscriptionRef.current.close()
        }
        const relayUrls = relayList?.read || []
        const newestKnownTimestamp = cachedConvs.length > 0
          ? Math.max(...cachedConvs.map((c) => c.lastMessageAt))
          : undefined

        dmSubscriptionRef.current = dmService.subscribeToDMs(pubkey, relayUrls, async (event) => {
          setNewestIncomingTimestamp(event.created_at)

          if (encryption) {
            try {
              const message = await dmService.decryptMessage(event, encryption, pubkey)
              if (message && message.senderPubkey && message.recipientPubkey) {
                const partner = message.senderPubkey === pubkey
                  ? message.recipientPubkey
                  : message.senderPubkey
                const activeConv = currentConversationRef.current

                if (activeConv && partner === activeConv) {
                  // Append directly — dedup by outer ID and inner event ID (self-copy has same inner)
                  setMessages((prev) => {
                    if (prev.some((m) => m.id === message.id)) return prev
                    if (message.innerEventId && prev.some((m) => m.innerEventId === message.innerEventId)) return prev
                    return [...prev, message].sort((a, b) => a.createdAt - b.createdAt)
                  })
                }

                // Always update conversation list (for both active and non-active conversations)
                const preview = (message.content ?? '').substring(0, 100)
                const updateConvList = (prev: TConversation[]) => {
                  const existing = prev.find((c) => c.partnerPubkey === partner)
                  if (existing) {
                    if (message.createdAt <= existing.lastMessageAt) return prev
                    return prev.map((c) =>
                      c.partnerPubkey === partner
                        ? {
                            ...c,
                            lastMessageAt: message.createdAt,
                            lastMessagePreview: preview,
                            unreadCount: partner !== activeConv ? c.unreadCount + 1 : c.unreadCount
                          }
                        : c
                    ).sort((a, b) => b.lastMessageAt - a.lastMessageAt)
                  }
                  return [{
                    partnerPubkey: partner,
                    lastMessageAt: message.createdAt,
                    lastMessagePreview: preview,
                    unreadCount: 1,
                    preferredEncryption: message.encryptionType
                  }, ...prev]
                }
                setConversations(updateConvList)
                setAllConversations(updateConvList)

                // Persist to IndexedDB
                indexedDb.putDMConversation(
                  pubkey, partner, message.createdAt, preview, message.encryptionType
                ).catch(() => {})
              }
            } catch {
              // Decryption failed
            }
          }
        }, newestKnownTimestamp)
      }

      initialize()
    } else {
      // Logout
      setConversations([])
      setAllConversations([])
      setMessages([])
      setCurrentConversation(null)
      setDeletedState(null)
      setSelectedMessages(new Set())
      setIsSelectionMode(false)
      clearPlaintextCache()
      if (dmSubscriptionRef.current) {
        dmSubscriptionRef.current.close()
        dmSubscriptionRef.current = null
      }
      hasInitializedRef.current = false
      lastPubkeyRef.current = null
    }
  }, [pubkey, encryption, relayList])

  // Load conversation when selected (or reloadTrigger changes)
  useEffect(() => {
    if (!currentConversation || !pubkey || !encryption) {
      setMessages([])
      loadingConversationRef.current = null
      return
    }

    const targetConversation = currentConversation
    loadingConversationRef.current = targetConversation

    const loadConversation = async () => {
      setIsLoadingConversation(true)
      try {
        // IndexedDB cache — single setMessages, no progressive flashing
        const cached = await indexedDb.getConversationMessages(pubkey, targetConversation)
        if (cached && cached.length > 0 && loadingConversationRef.current === targetConversation) {
          const cachedMessages: TDirectMessage[] = cached
            .filter((m) => !isMessageDeleted(m.id, targetConversation, m.createdAt, deletedState))
            .map((m) => ({
              id: m.id,
              senderPubkey: m.senderPubkey,
              recipientPubkey: m.recipientPubkey,
              content: m.content,
              createdAt: m.createdAt,
              encryptionType: m.encryptionType,
              event: {} as Event,
              decryptedContent: m.content,
              seenOnRelays: m.seenOnRelays
            }))
          setMessages(cachedMessages)
        }

        // Network fetch
        const relayUrls = relayList?.read || []
        const events = await dmService.fetchConversationEvents(pubkey, targetConversation, relayUrls)

        if (loadingConversationRef.current !== targetConversation) return

        // Pre-filter gift wraps older than delete cutoff.
        // Gift wrap outer timestamps are randomized up to 2 days in the past (NIP-59),
        // so subtract 3 days buffer to avoid filtering out recent messages.
        const deleteCutoff = getGlobalDeleteCutoff(deletedState)
        const giftWrapCutoff = deleteCutoff > 0 ? deleteCutoff - 3 * 24 * 60 * 60 : 0
        const filteredEvents = giftWrapCutoff > 0
          ? events.filter((e) => e.kind !== 1059 || e.created_at > giftWrapCutoff)
          : events

        // Decrypt all at once — no progressive batch rendering
        const allDecrypted = await decryptMessagesInBatches(
          filteredEvents, encryption, pubkey, 10
        )

        if (loadingConversationRef.current !== targetConversation) return

        // Filter — deduplicate by inner event ID for gift wraps (recipient + self-copy
        // share the same inner DM but have different outer wrap IDs)
        const seenIds = new Set<string>()
        const seenInnerIds = new Set<string>()
        const validMessages = allDecrypted.filter((message) => {
          if (seenIds.has(message.id)) return false
          if (message.innerEventId && seenInnerIds.has(message.innerEventId)) return false
          if (isNircProtocolMessage(message.content ?? '')) return false
          const partner = message.senderPubkey === pubkey
            ? message.recipientPubkey
            : message.senderPubkey
          if (partner !== targetConversation) return false
          if (isMessageDeleted(message.id, targetConversation, message.createdAt, deletedState)) return false
          seenIds.add(message.id)
          if (message.innerEventId) seenInnerIds.add(message.innerEventId)
          return true
        })

        // Merge with existing messages (preserves subscription-appended messages)
        setMessages((prev) => mergeAndDedupe(prev, validMessages))

        // Cache to IndexedDB — merge with existing cache, never replace with fewer messages
        const toCache = validMessages.map((m) => ({
          id: m.id,
          senderPubkey: m.senderPubkey,
          recipientPubkey: m.recipientPubkey,
          content: m.decryptedContent || m.content,
          createdAt: m.createdAt,
          encryptionType: m.encryptionType,
          seenOnRelays: m.seenOnRelays
        }))
        type CachedMsg = typeof toCache[number]
        const existingCache = await indexedDb.getConversationMessages(pubkey, targetConversation)
        if (existingCache && existingCache.length > 0 && toCache.length > 0) {
          const ids = new Set(toCache.map((m) => m.id))
          const merged: CachedMsg[] = [...toCache]
          for (const m of existingCache) {
            if (!ids.has(m.id)) {
              merged.push(m as CachedMsg)
              ids.add(m.id)
            }
          }
          merged.sort((a, b) => a.createdAt - b.createdAt)
          await indexedDb.putConversationMessages(pubkey, targetConversation, merged)
        } else if (toCache.length > 0) {
          await indexedDb.putConversationMessages(pubkey, targetConversation, toCache)
        }
        // If toCache is empty, don't write — keep existing cache intact
      } catch {
        // Failed to load
      } finally {
        if (loadingConversationRef.current === targetConversation) {
          setIsLoadingConversation(false)
        }
      }
    }

    loadConversation()
  }, [currentConversation, pubkey, encryption, relayList, deletedState, reloadTrigger])

  // Background refresh conversations list
  const backgroundRefreshConversations = useCallback(async () => {
    if (!pubkey || !encryption) return

    try {
      const relayUrls = relayList?.read || []
      const events = await dmService.fetchRecentDMEvents(pubkey, relayUrls)

      const nip04Events = events.filter((e) => e.kind === 4)
      const giftWraps = events.filter((e) => e.kind === 1059)

      const conversationMap = new Map<string, TConversation>()
      allConversations.forEach((c) => conversationMap.set(c.partnerPubkey, c))

      const nip04Convs = dmService.groupEventsIntoConversations(nip04Events, pubkey)
      nip04Convs.forEach((conv, key) => {
        const existing = conversationMap.get(key)
        if (!existing || conv.lastMessageAt > existing.lastMessageAt) {
          conversationMap.set(key, conv)
        }
      })

      const updateAndShowConversations = () => {
        const validConversations = Array.from(conversationMap.values())
          .filter((conv) => conv.partnerPubkey && typeof conv.partnerPubkey === 'string')
          .filter((conv) => !isConversationDeleted(conv.partnerPubkey, conv.lastMessageAt, deletedState))
        const sortedConversations = validConversations.sort(
          (a, b) => b.lastMessageAt - a.lastMessageAt
        )
        setAllConversations(sortedConversations)
        setConversations(sortedConversations.slice(0, CONVERSATIONS_PER_PAGE))
        setHasMoreConversations(sortedConversations.length > CONVERSATIONS_PER_PAGE)
      }

      updateAndShowConversations()

      const sortedGiftWraps = giftWraps.sort((a, b) => b.created_at - a.created_at)
      const deleteCutoff = getGlobalDeleteCutoff(deletedState)

      for (const giftWrap of sortedGiftWraps) {
        if (deleteCutoff > 0 && giftWrap.created_at <= deleteCutoff) continue

        try {
          const message = await dmService.decryptMessage(giftWrap, encryption, pubkey)
          if (message && message.senderPubkey && message.recipientPubkey) {
            const partnerPubkey =
              message.senderPubkey === pubkey ? message.recipientPubkey : message.senderPubkey
            if (!partnerPubkey || partnerPubkey === '__reaction__') continue
            if (isNircProtocolMessage(message.content ?? '')) continue

            const existing = conversationMap.get(partnerPubkey)
            if (!existing || message.createdAt > existing.lastMessageAt) {
              conversationMap.set(partnerPubkey, {
                partnerPubkey,
                lastMessageAt: message.createdAt,
                lastMessagePreview: (message.content ?? '').substring(0, 100),
                unreadCount: 0,
                preferredEncryption: 'nip17'
              })
              updateAndShowConversations()
            }

            indexedDb.putDMConversation(
              pubkey, partnerPubkey, message.createdAt,
              (message.content ?? '').substring(0, 100), 'nip17'
            ).catch(() => {})
          }
        } catch {
          // Skip
        }
      }

      updateAndShowConversations()
      const finalConversations = Array.from(conversationMap.values())
      Promise.all(
        finalConversations.map((conv) =>
          indexedDb.putDMConversation(
            pubkey, conv.partnerPubkey, conv.lastMessageAt,
            conv.lastMessagePreview, conv.preferredEncryption
          )
        )
      ).catch(() => {})
    } catch {
      // Background refresh failed
    }
  }, [pubkey, encryption, relayList, deletedState, allConversations])

  // Manual refresh
  const refreshConversations = useCallback(async () => {
    if (!pubkey || !encryption) return

    setIsLoading(true)
    setError(null)

    try {
      const relayUrls = relayList?.read || []
      const events = await dmService.fetchRecentDMEvents(pubkey, relayUrls)

      const nip04Events = events.filter((e) => e.kind === 4)
      const giftWraps = events.filter((e) => e.kind === 1059)

      const conversationMap = new Map<string, TConversation>()
      allConversations.forEach((c) => conversationMap.set(c.partnerPubkey, c))

      const nip04Convs = dmService.groupEventsIntoConversations(nip04Events, pubkey)
      nip04Convs.forEach((conv, key) => {
        const existing = conversationMap.get(key)
        if (!existing || conv.lastMessageAt > existing.lastMessageAt) {
          conversationMap.set(key, conv)
        }
      })

      const updateAndShowConversations = () => {
        const validConversations = Array.from(conversationMap.values())
          .filter((conv) => conv.partnerPubkey && typeof conv.partnerPubkey === 'string')
          .filter((conv) => !isConversationDeleted(conv.partnerPubkey, conv.lastMessageAt, deletedState))
        const sortedConversations = validConversations.sort(
          (a, b) => b.lastMessageAt - a.lastMessageAt
        )
        setAllConversations(sortedConversations)
        setConversations(sortedConversations.slice(0, CONVERSATIONS_PER_PAGE))
        setHasMoreConversations(sortedConversations.length > CONVERSATIONS_PER_PAGE)
      }

      updateAndShowConversations()
      setIsLoading(false)

      const sortedGiftWraps = giftWraps.sort((a, b) => b.created_at - a.created_at)
      for (const giftWrap of sortedGiftWraps) {
        try {
          const message = await dmService.decryptMessage(giftWrap, encryption, pubkey)
          if (message && message.senderPubkey && message.recipientPubkey) {
            const partnerPubkey =
              message.senderPubkey === pubkey ? message.recipientPubkey : message.senderPubkey
            if (!partnerPubkey || partnerPubkey === '__reaction__') continue
            if (isNircProtocolMessage(message.content ?? '')) continue

            const existing = conversationMap.get(partnerPubkey)
            if (!existing || message.createdAt > existing.lastMessageAt) {
              conversationMap.set(partnerPubkey, {
                partnerPubkey,
                lastMessageAt: message.createdAt,
                lastMessagePreview: (message.content ?? '').substring(0, 100),
                unreadCount: 0,
                preferredEncryption: 'nip17'
              })
              updateAndShowConversations()
            }

            indexedDb.putDMConversation(
              pubkey, partnerPubkey, message.createdAt,
              (message.content ?? '').substring(0, 100), 'nip17'
            ).catch(() => {})
          }
        } catch {
          // Skip
        }
      }

      updateAndShowConversations()
      const finalConversations = Array.from(conversationMap.values())
      Promise.all(
        finalConversations.map((conv) =>
          indexedDb.putDMConversation(
            pubkey, conv.partnerPubkey, conv.lastMessageAt,
            conv.lastMessagePreview, conv.preferredEncryption
          )
        )
      ).catch(() => {})
    } catch {
      setError('Failed to load conversations')
      setIsLoading(false)
    }
  }, [pubkey, encryption, relayList, deletedState, allConversations])

  const loadMoreConversations = useCallback(async () => {
    if (!hasMoreConversations) return
    const currentCount = conversations.length
    const nextBatch = allConversations.slice(currentCount, currentCount + CONVERSATIONS_PER_PAGE)
    setConversations((prev) => [...prev, ...nextBatch])
    setHasMoreConversations(currentCount + nextBatch.length < allConversations.length)
  }, [conversations.length, allConversations, hasMoreConversations])

  const selectConversation = useCallback(
    (partnerPubkey: string | null) => {
      if (partnerPubkey !== currentConversation) {
        setMessages([])
      }
      setCurrentConversation(partnerPubkey)
    },
    [currentConversation]
  )

  const startConversation = useCallback(
    (partnerPubkey: string) => {
      const existingConversation = allConversations.find(
        (c) => c.partnerPubkey === partnerPubkey
      )
      if (!existingConversation) {
        setIsNewConversation(true)
        setProvisionalPubkey(partnerPubkey)
        const provisionalConversation: TConversation = {
          partnerPubkey,
          lastMessageAt: Math.floor(Date.now() / 1000),
          lastMessagePreview: '',
          unreadCount: 0,
          preferredEncryption: 'nip17'
        }
        setAllConversations((prev) => [provisionalConversation, ...prev])
        setConversations((prev) => [provisionalConversation, ...prev])
      }
      setMessages([])
      setCurrentConversation(partnerPubkey)
    },
    [allConversations]
  )

  const clearNewConversationFlag = useCallback(() => {
    setIsNewConversation(false)
  }, [])

  const dismissProvisionalConversation = useCallback(() => {
    if (!provisionalPubkey) return
    setAllConversations((prev) => prev.filter((c) => c.partnerPubkey !== provisionalPubkey))
    setConversations((prev) => prev.filter((c) => c.partnerPubkey !== provisionalPubkey))
    setProvisionalPubkey(null)
    setIsNewConversation(false)
    if (currentConversation === provisionalPubkey) {
      setCurrentConversation(null)
      setMessages([])
    }
  }, [provisionalPubkey, currentConversation])

  const reloadConversation = useCallback(() => {
    if (!currentConversation) return
    setMessages([])
    setReloadTrigger((prev) => prev + 1)
  }, [currentConversation])

  const sendMessage = useCallback(
    async (content: string, customRelayUrls?: string[], expirationSeconds?: number) => {
      if (!pubkey || !encryption || !currentConversation) {
        throw new Error('Cannot send message: not logged in or no conversation selected')
      }

      const relayUrls = customRelayUrls && customRelayUrls.length > 0
        ? customRelayUrls
        : (relayList?.write || [])

      const conversation = conversations.find((c) => c.partnerPubkey === currentConversation)
      const existingEncryptionType: TDMEncryptionType | null =
        conversation?.preferredEncryption ?? null

      const encryptionPref = await indexedDb.getConversationEncryptionPreference(
        pubkey, currentConversation
      )

      let effectiveEncryption: TDMEncryptionType | null = existingEncryptionType
      if (encryptionPref === 'nip04') {
        effectiveEncryption = 'nip04'
      } else if (encryptionPref === 'nip17') {
        effectiveEncryption = 'nip17'
      }

      // Read expiration preference from IndexedDB if not provided by caller
      let effectiveExpiration = expirationSeconds ?? 0
      if (effectiveExpiration === 0) {
        effectiveExpiration = await indexedDb.getConversationExpirationPreference(
          pubkey, currentConversation
        )
      }

      const sentEvents = await dmService.sendDM(
        currentConversation, content, encryption, relayUrls, preferNip44, effectiveEncryption,
        effectiveExpiration
      )

      const now = Math.floor(Date.now() / 1000)
      const usedEncryptionType: TDMEncryptionType =
        effectiveEncryption || (preferNip44 ? 'nip17' : 'nip04')
      const sentEvent = sentEvents[0]
      const innerEventId = (sentEvent as any)?._innerEventId as string | undefined
      const newMessage: TDirectMessage = {
        id: sentEvent?.id || `local-${now}`,
        innerEventId,
        senderPubkey: pubkey,
        recipientPubkey: currentConversation,
        content,
        createdAt: now,
        encryptionType: usedEncryptionType,
        event: sentEvent || ({} as Event),
        decryptedContent: content
      }

      setMessages((prev) => [...prev, newMessage])

      setConversations((prev) => {
        const existing = prev.find((c) => c.partnerPubkey === currentConversation)
        if (existing) {
          return prev.map((c) =>
            c.partnerPubkey === currentConversation
              ? { ...c, lastMessageAt: now, lastMessagePreview: content.substring(0, 100), preferredEncryption: usedEncryptionType }
              : c
          )
        } else {
          return [{
            partnerPubkey: currentConversation,
            lastMessageAt: now,
            lastMessagePreview: content.substring(0, 100),
            unreadCount: 0,
            preferredEncryption: usedEncryptionType
          }, ...prev]
        }
      })

      if (provisionalPubkey === currentConversation) {
        setProvisionalPubkey(null)
        setIsNewConversation(false)
      }
    },
    [pubkey, encryption, currentConversation, relayList, conversations, preferNip44, provisionalPubkey]
  )

  const setPreferNip44 = useCallback((prefer: boolean) => {
    setPreferNip44State(prefer)
    storage.setPreferNip44(prefer)
    dispatchSettingsChanged()
  }, [])

  const toggleMessageSelection = useCallback((messageId: string) => {
    setSelectedMessages((prev) => {
      const next = new Set(prev)
      if (next.has(messageId)) {
        next.delete(messageId)
        if (next.size === 0) setIsSelectionMode(false)
      } else {
        next.add(messageId)
        if (!isSelectionMode) setIsSelectionMode(true)
      }
      return next
    })
  }, [isSelectionMode])

  const selectAllMessages = useCallback(() => {
    const allIds = new Set(messages.map((m) => m.id))
    setSelectedMessages(allIds)
    setIsSelectionMode(true)
  }, [messages])

  const clearSelection = useCallback(() => {
    setSelectedMessages(new Set())
    setIsSelectionMode(false)
  }, [])

  const publishDeletedState = useCallback(
    async (newState: TDMDeletedState) => {
      if (!pubkey || !encryption) return
      await indexedDb.putDeletedMessagesState(pubkey, newState)
      const relayUrls = relayList?.write.length ? relayList.write : client.currentRelays
      const draftEvent = createDeletedMessagesDraftEvent(newState)
      const signedEvent = await encryption.signEvent(draftEvent)
      await client.publishEvent(relayUrls, signedEvent)
    },
    [pubkey, encryption, relayList]
  )

  const deleteSelectedMessages = useCallback(async () => {
    if (!pubkey || selectedMessages.size === 0) return

    const messageIds = Array.from(selectedMessages)
    const newDeletedState: TDMDeletedState = {
      deletedIds: [...(deletedState?.deletedIds || []), ...messageIds],
      deletedRanges: deletedState?.deletedRanges || {}
    }
    setDeletedState(newDeletedState)
    setMessages((prev) => prev.filter((m) => !selectedMessages.has(m.id)))
    setSelectedMessages(new Set())
    setIsSelectionMode(false)
    await publishDeletedState(newDeletedState)
  }, [pubkey, selectedMessages, deletedState, publishDeletedState])

  const deleteAllInConversation = useCallback(async () => {
    if (!pubkey || !currentConversation) return

    const now = Math.floor(Date.now() / 1000)
    const newDeletedState: TDMDeletedState = {
      deletedIds: deletedState?.deletedIds || [],
      deletedRanges: {
        ...(deletedState?.deletedRanges || {}),
        [currentConversation]: [
          ...(deletedState?.deletedRanges[currentConversation] || []),
          { start: 0, end: now }
        ]
      }
    }
    setDeletedState(newDeletedState)
    setMessages([])
    setConversations((prev) => prev.filter((c) => c.partnerPubkey !== currentConversation))
    setAllConversations((prev) => prev.filter((c) => c.partnerPubkey !== currentConversation))
    setSelectedMessages(new Set())
    setIsSelectionMode(false)
    setCurrentConversation(null)
    await publishDeletedState(newDeletedState)
  }, [pubkey, currentConversation, deletedState, publishDeletedState])

  const undeleteAllInConversation = useCallback(async () => {
    if (!pubkey || !currentConversation) return

    const newDeletedState: TDMDeletedState = {
      deletedIds: deletedState?.deletedIds || [],
      deletedRanges: {
        ...(deletedState?.deletedRanges || {}),
        [currentConversation]: []
      }
    }
    setDeletedState(newDeletedState)
    setMessages([])
    setReloadTrigger((prev) => prev + 1)
    await publishDeletedState(newDeletedState)
    await backgroundRefreshConversations()
  }, [pubkey, currentConversation, deletedState, publishDeletedState, backgroundRefreshConversations])

  const filteredConversations = useMemo(() => {
    if (!deletedState) return conversations
    return conversations.filter(
      (c) => !isConversationDeleted(c.partnerPubkey, c.lastMessageAt, deletedState)
    )
  }, [conversations, deletedState])

  const totalUnreadCount = useMemo(() => {
    return filteredConversations.reduce((sum, c) => sum + c.unreadCount, 0)
  }, [filteredConversations])

  const newestMessageTimestamp = useMemo(() => {
    const fromConversations = filteredConversations.length === 0
      ? 0
      : Math.max(...filteredConversations.map((c) => c.lastMessageAt))
    return Math.max(fromConversations, newestIncomingTimestamp)
  }, [filteredConversations, newestIncomingTimestamp])

  const hasNewMessages = lastSeenTimestamp > 0
    ? newestMessageTimestamp > lastSeenTimestamp
    : newestIncomingTimestamp > 0

  const markInboxAsSeen = useCallback(() => {
    if (!pubkey) return
    setNewestIncomingTimestamp(0)
    if (newestMessageTimestamp > 0) {
      setLastSeenTimestamp(newestMessageTimestamp)
      storage.setDMLastSeenTimestamp(pubkey, newestMessageTimestamp)
    }
  }, [pubkey, newestMessageTimestamp])

  return (
    <DMContext.Provider
      value={{
        conversations: filteredConversations,
        currentConversation,
        messages,
        isLoading,
        isLoadingConversation,
        error,
        selectConversation,
        startConversation,
        sendMessage,
        refreshConversations,
        reloadConversation,
        loadMoreConversations,
        hasMoreConversations,
        preferNip44,
        setPreferNip44,
        isNewConversation,
        clearNewConversationFlag,
        dismissProvisionalConversation,
        totalUnreadCount,
        hasNewMessages,
        markInboxAsSeen,
        selectedMessages,
        isSelectionMode,
        toggleMessageSelection,
        selectAllMessages,
        clearSelection,
        deleteSelectedMessages,
        deleteAllInConversation,
        undeleteAllInConversation
      }}
    >
      {children}
    </DMContext.Provider>
  )
}
