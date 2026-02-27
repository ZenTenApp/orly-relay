import { ApplicationDataKey } from '@/constants'
import { createDeletedMessagesDraftEvent } from '@/lib/draft-event'
import dmService, {
  clearPlaintextCache,
  decryptMessagesInBatches,
  getGlobalDeleteCutoff,
  IDMEncryption,
  isConversationDeleted,
  isMessageDeleted
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
  sendMessage: (content: string, customRelayUrls?: string[]) => Promise<void>
  refreshConversations: () => Promise<void>
  reloadConversation: () => void
  loadMoreConversations: () => Promise<void>
  hasMoreConversations: boolean
  preferNip44: boolean
  setPreferNip44: (prefer: boolean) => void
  isNewConversation: boolean
  clearNewConversationFlag: () => void
  dismissProvisionalConversation: () => void
  // Unread tracking
  totalUnreadCount: number
  hasNewMessages: boolean
  markInboxAsSeen: () => void
  // Selection mode
  selectedMessages: Set<string>
  isSelectionMode: boolean
  toggleMessageSelection: (messageId: string) => void
  selectAllMessages: () => void
  clearSelection: () => void
  // Deletion
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
  const [conversationMessages, setConversationMessages] = useState<Map<string, TDirectMessage[]>>(
    () => new Map()
  )
  const [loadedConversations, setLoadedConversations] = useState<Set<string>>(() => new Set())
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
  const CONVERSATIONS_PER_PAGE = 100

  // Track which conversation load is in progress to prevent race conditions
  const loadingConversationRef = useRef<string | null>(null)
  // Track if we've already initialized to avoid reloading on navigation
  const hasInitializedRef = useRef(false)
  const lastPubkeyRef = useRef<string | null>(null)
  // Background subscription for real-time DM updates
  const dmSubscriptionRef = useRef<{ close: () => void } | null>(null)
  // Track newest message timestamp from subscription (to update hasNewMessages)
  const [newestIncomingTimestamp, setNewestIncomingTimestamp] = useState(0)

  // Create encryption wrapper object for dm.service
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

  // Load deleted state and conversations when user is logged in
  useEffect(() => {
    if (pubkey && encryption) {
      // Skip if already initialized for this pubkey (e.g., navigating back)
      if (hasInitializedRef.current && lastPubkeyRef.current === pubkey) {
        return
      }

      // Mark as initialized for this pubkey
      hasInitializedRef.current = true
      lastPubkeyRef.current = pubkey

      // Reload lastSeenTimestamp from storage (useState initializer may have run when pubkey was null)
      const savedTimestamp = storage.getDMLastSeenTimestamp(pubkey)
      if (savedTimestamp > 0) {
        setLastSeenTimestamp(savedTimestamp)
      }

      // Load deleted state FIRST before anything else
      const loadDeletedStateAndConversations = async () => {
        // Step 1: Load deleted state from IndexedDB
        let currentDeletedState: TDMDeletedState = { deletedIds: [], deletedRanges: {} }
        const cached = await indexedDb.getDeletedMessagesState(pubkey)
        if (cached) {
          currentDeletedState = cached
          setDeletedState(cached)
        } else {
          setDeletedState(currentDeletedState)
        }

        // Step 2: Fetch from relays (kind 30078 Application Specific Data) - this takes priority
        try {
          const relayUrls = relayList?.read.length ? relayList.read : client.currentRelays
          const events = await client.fetchEvents(relayUrls, {
            kinds: [kinds.Application],
            authors: [pubkey],
            '#d': [ApplicationDataKey.DM_DELETED_MESSAGES],
            limit: 1
          })
          if (events.length > 0) {
            const event = events[0]
            try {
              const parsedState = JSON.parse(event.content) as TDMDeletedState
              currentDeletedState = parsedState
              setDeletedState(parsedState)
              await indexedDb.putDeletedMessagesState(pubkey, parsedState)
            } catch {
              // Invalid JSON, ignore
            }
          }
        } catch {
          // Relay fetch failed, use cached
        }

        // Step 3: Load cached conversations (filtered by deleted state)
        const cachedConvs = await indexedDb.getDMConversations(pubkey)
        if (cachedConvs.length > 0) {
          const conversations: TConversation[] = cachedConvs
            .filter((c) => c.partnerPubkey && typeof c.partnerPubkey === 'string')
            .filter((c) => !isConversationDeleted(c.partnerPubkey, c.lastMessageAt, currentDeletedState))
            .map((c) => ({
              partnerPubkey: c.partnerPubkey,
              lastMessageAt: c.lastMessageAt,
              lastMessagePreview: c.lastMessagePreview || '',
              unreadCount: 0,
              preferredEncryption: c.encryptionType
            }))
          setAllConversations(conversations)
          setConversations(conversations.slice(0, CONVERSATIONS_PER_PAGE))
          setHasMoreConversations(conversations.length > CONVERSATIONS_PER_PAGE)
        }

        // Step 4: Background refresh from network (don't clear existing data)
        backgroundRefreshConversations()

        // Step 5: Start real-time subscription for new DMs
        if (dmSubscriptionRef.current) {
          dmSubscriptionRef.current.close()
        }
        const relayUrls = relayList?.read || []
        // Use the newest known conversation timestamp to avoid gaps between fetch and subscription
        const newestKnownTimestamp = cachedConvs.length > 0
          ? Math.max(...cachedConvs.map((c) => c.lastMessageAt))
          : undefined
        dmSubscriptionRef.current = dmService.subscribeToDMs(pubkey, relayUrls, (event) => {
          // New DM event received - update timestamp to trigger hasNewMessages
          setNewestIncomingTimestamp(event.created_at)
        }, newestKnownTimestamp)
      }

      loadDeletedStateAndConversations()
    } else {
      // Clear all state on logout
      setConversations([])
      setAllConversations([])
      setMessages([])
      setConversationMessages(new Map())
      setLoadedConversations(new Set())
      setCurrentConversation(null)
      setDeletedState(null)
      setSelectedMessages(new Set())
      setIsSelectionMode(false)
      // Clear in-memory plaintext cache
      clearPlaintextCache()
      // Stop DM subscription
      if (dmSubscriptionRef.current) {
        dmSubscriptionRef.current.close()
        dmSubscriptionRef.current = null
      }
      // Reset initialization flag so we reload on next login
      hasInitializedRef.current = false
      lastPubkeyRef.current = null
    }
  }, [pubkey, encryption, relayList])

  // Load full conversation when selected
  useEffect(() => {
    if (!currentConversation || !pubkey || !encryption) {
      setMessages([])
      loadingConversationRef.current = null
      return
    }

    // Capture the conversation we're loading to detect stale updates
    const targetConversation = currentConversation
    loadingConversationRef.current = targetConversation

    // Check if we already have messages in memory for this conversation
    const existing = conversationMessages.get(targetConversation)
    if (existing && existing.length > 0) {
      setMessages(existing)
      // If already fully loaded and data is present, don't fetch again
      if (loadedConversations.has(targetConversation)) {
        return
      }
    }

    // Load full conversation history
    const loadConversation = async () => {
      setIsLoadingConversation(true)
      try {
        // First, try to load from IndexedDB cache for instant display
        const cached = await indexedDb.getConversationMessages(pubkey, targetConversation)
        if (cached && cached.length > 0 && loadingConversationRef.current === targetConversation) {
          const cachedMessages: TDirectMessage[] = cached
            .filter(
              (m) => !isMessageDeleted(m.id, targetConversation, m.createdAt, deletedState)
            )
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
          setConversationMessages((prev) => new Map(prev).set(targetConversation, cachedMessages))
        }

        // Then fetch fresh from relays
        const relayUrls = relayList?.read || []
        const events = await dmService.fetchConversationEvents(pubkey, targetConversation, relayUrls)

        // Check if user switched to a different conversation while we were loading
        if (loadingConversationRef.current !== targetConversation) {
          return // Abort - user switched conversations
        }

        // Pre-filter events: skip gift wraps older than global delete cutoff (before decryption)
        const deleteCutoff = getGlobalDeleteCutoff(deletedState)
        const filteredEvents = deleteCutoff > 0
          ? events.filter((e) => e.kind !== 1059 || e.created_at > deleteCutoff)
          : events

        // Decrypt messages in batches to avoid blocking UI
        // Progressive updates: show messages as they're decrypted
        const allDecrypted: TDirectMessage[] = []
        const seenIds = new Set<string>()

        await decryptMessagesInBatches(
          filteredEvents,
          encryption,
          pubkey,
          10, // batch size
          (batchMessages) => {
            // Check if still on same conversation before updating
            if (loadingConversationRef.current !== targetConversation) return

            // Filter to only messages in this conversation (excluding deleted and duplicates)
            const validMessages = batchMessages.filter((message) => {
              if (seenIds.has(message.id)) return false
              const partner =
                message.senderPubkey === pubkey ? message.recipientPubkey : message.senderPubkey
              if (partner !== targetConversation) return false
              if (isMessageDeleted(message.id, targetConversation, message.createdAt, deletedState)) return false
              seenIds.add(message.id)
              return true
            })

            allDecrypted.push(...validMessages)

            // Sort and update progressively
            const sorted = [...allDecrypted].sort((a, b) => a.createdAt - b.createdAt)
            setMessages(sorted)
            setConversationMessages((prev) => new Map(prev).set(targetConversation, sorted))
          }
        )

        // Check again after decryption (which can take time)
        if (loadingConversationRef.current !== targetConversation) {
          return // Abort - user switched conversations
        }

        // Final sort
        const sorted = allDecrypted.sort((a, b) => a.createdAt - b.createdAt)

        // Update state only if still on same conversation
        setConversationMessages((prev) => new Map(prev).set(targetConversation, sorted))
        setLoadedConversations((prev) => new Set(prev).add(targetConversation))
        setMessages(sorted)

        // Cache messages to IndexedDB (without the full event object)
        const toCache = sorted.map((m) => ({
          id: m.id,
          senderPubkey: m.senderPubkey,
          recipientPubkey: m.recipientPubkey,
          content: m.decryptedContent || m.content,
          createdAt: m.createdAt,
          encryptionType: m.encryptionType,
          seenOnRelays: m.seenOnRelays
        }))
        await indexedDb.putConversationMessages(pubkey, targetConversation, toCache)
      } catch {
        // Failed to load conversation
      } finally {
        // Only clear loading state if this is still the active load
        if (loadingConversationRef.current === targetConversation) {
          setIsLoadingConversation(false)
        }
      }
    }

    loadConversation()
  }, [currentConversation, pubkey, encryption, relayList, deletedState])

  // Background refresh - merges new data without clearing existing cache
  const backgroundRefreshConversations = useCallback(async () => {
    if (!pubkey || !encryption) return

    try {
      // Get relay URLs
      const relayUrls = relayList?.read || []

      // Fetch recent DM events (raw, not decrypted)
      const events = await dmService.fetchRecentDMEvents(pubkey, relayUrls)

      // Separate NIP-04 events and gift wraps
      const nip04Events = events.filter((e) => e.kind === 4)
      const giftWraps = events.filter((e) => e.kind === 1059)

      // Build conversation map from existing conversations
      const conversationMap = new Map<string, TConversation>()
      allConversations.forEach((c) => conversationMap.set(c.partnerPubkey, c))

      // Add NIP-04 conversations
      const nip04Convs = dmService.groupEventsIntoConversations(nip04Events, pubkey)
      nip04Convs.forEach((conv, key) => {
        const existing = conversationMap.get(key)
        if (!existing || conv.lastMessageAt > existing.lastMessageAt) {
          conversationMap.set(key, conv)
        }
      })

      // Update UI with NIP-04 data (filtered by deleted state)
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

      // Process gift wraps in background (progressive, no UI blocking)
      const sortedGiftWraps = giftWraps.sort((a, b) => b.created_at - a.created_at)

      // Calculate global delete cutoff for pre-filtering (skip decryption for old deleted messages)
      const deleteCutoff = getGlobalDeleteCutoff(deletedState)

      for (const giftWrap of sortedGiftWraps) {
        // Skip gift wraps older than global delete cutoff (no decryption needed)
        if (deleteCutoff > 0 && giftWrap.created_at <= deleteCutoff) {
          continue
        }

        try {
          const message = await dmService.decryptMessage(giftWrap, encryption, pubkey)
          if (message && message.senderPubkey && message.recipientPubkey) {
            const partnerPubkey =
              message.senderPubkey === pubkey ? message.recipientPubkey : message.senderPubkey

            if (!partnerPubkey || partnerPubkey === '__reaction__') continue

            const existing = conversationMap.get(partnerPubkey)
            if (!existing || message.createdAt > existing.lastMessageAt) {
              const preview = (message.content ?? '').substring(0, 100)
              conversationMap.set(partnerPubkey, {
                partnerPubkey,
                lastMessageAt: message.createdAt,
                lastMessagePreview: preview,
                unreadCount: 0,
                preferredEncryption: 'nip17'
              })
              updateAndShowConversations()
            }

            // Cache conversation metadata
            indexedDb
              .putDMConversation(
                pubkey,
                partnerPubkey,
                message.createdAt,
                (message.content ?? '').substring(0, 100),
                'nip17'
              )
              .catch(() => {})
          }
        } catch {
          // Skip failed decryptions silently
        }
      }

      // Final update and cache all conversations
      updateAndShowConversations()
      const finalConversations = Array.from(conversationMap.values())
      Promise.all(
        finalConversations.map((conv) =>
          indexedDb.putDMConversation(
            pubkey,
            conv.partnerPubkey,
            conv.lastMessageAt,
            conv.lastMessagePreview,
            conv.preferredEncryption
          )
        )
      ).catch(() => {})
    } catch {
      // Background refresh failed silently - cached data still shown
    }
  }, [pubkey, encryption, relayList, deletedState, allConversations])

  // Full refresh - fetches fresh data from network (manual action)
  const refreshConversations = useCallback(async () => {
    if (!pubkey || !encryption) return

    setIsLoading(true)
    setError(null)

    try {
      // Get relay URLs
      const relayUrls = relayList?.read || []

      // Fetch recent DM events (raw, not decrypted)
      const events = await dmService.fetchRecentDMEvents(pubkey, relayUrls)

      // Separate NIP-04 events and gift wraps
      const nip04Events = events.filter((e) => e.kind === 4)
      const giftWraps = events.filter((e) => e.kind === 1059)

      // Build conversation map from existing conversations (merge, don't replace)
      const conversationMap = new Map<string, TConversation>()
      allConversations.forEach((c) => conversationMap.set(c.partnerPubkey, c))

      // Add NIP-04 conversations
      const nip04Convs = dmService.groupEventsIntoConversations(nip04Events, pubkey)
      nip04Convs.forEach((conv, key) => {
        const existing = conversationMap.get(key)
        if (!existing || conv.lastMessageAt > existing.lastMessageAt) {
          conversationMap.set(key, conv)
        }
      })

      // Show NIP-04 conversations immediately (filtered by deleted state)
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
      setIsLoading(false) // Stop spinner, but continue processing in background

      // Sort gift wraps by created_at descending (newest first)
      const sortedGiftWraps = giftWraps.sort((a, b) => b.created_at - a.created_at)

      // Process gift wraps one by one in the background (progressive loading)
      for (const giftWrap of sortedGiftWraps) {
        try {
          const message = await dmService.decryptMessage(giftWrap, encryption, pubkey)
          if (message && message.senderPubkey && message.recipientPubkey) {
            const partnerPubkey =
              message.senderPubkey === pubkey ? message.recipientPubkey : message.senderPubkey

            if (!partnerPubkey || partnerPubkey === '__reaction__') continue

            const existing = conversationMap.get(partnerPubkey)
            if (!existing || message.createdAt > existing.lastMessageAt) {
              const preview = (message.content ?? '').substring(0, 100)
              conversationMap.set(partnerPubkey, {
                partnerPubkey,
                lastMessageAt: message.createdAt,
                lastMessagePreview: preview,
                unreadCount: 0,
                preferredEncryption: 'nip17'
              })
              // Update UI progressively
              updateAndShowConversations()
            }

            // Cache conversation metadata
            indexedDb
              .putDMConversation(
                pubkey,
                partnerPubkey,
                message.createdAt,
                (message.content ?? '').substring(0, 100),
                'nip17'
              )
              .catch(() => {})
          }
        } catch {
          // Skip failed decryptions silently
        }
      }

      // Final update and cache all conversations
      updateAndShowConversations()
      const finalConversations = Array.from(conversationMap.values())
      Promise.all(
        finalConversations.map((conv) =>
          indexedDb.putDMConversation(
            pubkey,
            conv.partnerPubkey,
            conv.lastMessageAt,
            conv.lastMessagePreview,
            conv.preferredEncryption
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
      // Clear messages immediately to prevent showing old conversation
      if (partnerPubkey !== currentConversation) {
        setMessages([])
      }
      setCurrentConversation(partnerPubkey)
    },
    [currentConversation]
  )

  // Start a new conversation - marks it as new for UI effects (pulsing settings button)
  // Creates a provisional conversation that appears in the list immediately
  const startConversation = useCallback(
    (partnerPubkey: string) => {
      // Check if this is a new conversation (not in existing list)
      const existingConversation = allConversations.find(
        (c) => c.partnerPubkey === partnerPubkey
      )
      if (!existingConversation) {
        setIsNewConversation(true)
        setProvisionalPubkey(partnerPubkey)
        // Add a provisional conversation to the list so it appears immediately
        // Default to nip17 (modern encryption) to avoid dual-send on first message
        const provisionalConversation: TConversation = {
          partnerPubkey,
          lastMessageAt: Math.floor(Date.now() / 1000),
          lastMessagePreview: '',
          unreadCount: 0,
          preferredEncryption: 'nip17'
        }
        // Add to front of both lists
        setAllConversations((prev) => [provisionalConversation, ...prev])
        setConversations((prev) => [provisionalConversation, ...prev])
      }
      // Clear messages and select the conversation
      setMessages([])
      setCurrentConversation(partnerPubkey)
    },
    [allConversations]
  )

  const clearNewConversationFlag = useCallback(() => {
    setIsNewConversation(false)
  }, [])

  // Dismiss a provisional conversation (remove from list without sending any messages)
  const dismissProvisionalConversation = useCallback(() => {
    if (!provisionalPubkey) return

    // Remove from conversation lists
    setAllConversations((prev) => prev.filter((c) => c.partnerPubkey !== provisionalPubkey))
    setConversations((prev) => prev.filter((c) => c.partnerPubkey !== provisionalPubkey))

    // Clear provisional state
    setProvisionalPubkey(null)
    setIsNewConversation(false)

    // Deselect if this was the current conversation
    if (currentConversation === provisionalPubkey) {
      setCurrentConversation(null)
      setMessages([])
    }
  }, [provisionalPubkey, currentConversation])

  // Reload the current conversation by clearing its cached state
  const reloadConversation = useCallback(() => {
    if (!currentConversation) return

    // Clear the loaded state and cached messages for this conversation
    setLoadedConversations((prev) => {
      const next = new Set(prev)
      next.delete(currentConversation)
      return next
    })
    setConversationMessages((prev) => {
      const next = new Map(prev)
      next.delete(currentConversation)
      return next
    })
    // Clear current messages to trigger a reload
    setMessages([])
  }, [currentConversation])

  const sendMessage = useCallback(
    async (content: string, customRelayUrls?: string[]) => {
      if (!pubkey || !encryption || !currentConversation) {
        throw new Error('Cannot send message: not logged in or no conversation selected')
      }

      // Use custom relays if provided, otherwise fall back to user's write relays
      const relayUrls = customRelayUrls && customRelayUrls.length > 0
        ? customRelayUrls
        : (relayList?.write || [])

      // Find existing encryption type for this conversation
      const conversation = conversations.find((c) => c.partnerPubkey === currentConversation)
      const existingEncryptionType: TDMEncryptionType | null =
        conversation?.preferredEncryption ?? null

      // Check for conversation-specific encryption preference
      const encryptionPref = await indexedDb.getConversationEncryptionPreference(
        pubkey,
        currentConversation
      )

      // Determine the encryption to use based on preference
      let effectiveEncryption: TDMEncryptionType | null = existingEncryptionType

      if (encryptionPref === 'nip04') {
        effectiveEncryption = 'nip04'
      } else if (encryptionPref === 'nip17') {
        effectiveEncryption = 'nip17'
      }
      // 'auto' keeps the existing behavior (match conversation or send both)

      // Send the message
      const sentEvents = await dmService.sendDM(
        currentConversation,
        content,
        encryption,
        relayUrls,
        preferNip44,
        effectiveEncryption
      )

      // Create local message for immediate display
      const now = Math.floor(Date.now() / 1000)
      // Determine the actual encryption type used for the message
      const usedEncryptionType: TDMEncryptionType =
        effectiveEncryption || (preferNip44 ? 'nip17' : 'nip04')
      const newMessage: TDirectMessage = {
        id: sentEvents[0]?.id || `local-${now}`,
        senderPubkey: pubkey,
        recipientPubkey: currentConversation,
        content,
        createdAt: now,
        encryptionType: usedEncryptionType,
        event: sentEvents[0] || ({} as Event),
        decryptedContent: content
      }

      // Add to messages for this conversation
      setConversationMessages((prev) => {
        const existing = prev.get(currentConversation) || []
        return new Map(prev).set(currentConversation, [...existing, newMessage])
      })
      setMessages((prev) => [...prev, newMessage])

      // Update conversation
      setConversations((prev) => {
        const existing = prev.find((c) => c.partnerPubkey === currentConversation)
        if (existing) {
          return prev.map((c) =>
            c.partnerPubkey === currentConversation
              ? {
                  ...c,
                  lastMessageAt: now,
                  lastMessagePreview: content.substring(0, 100),
                  preferredEncryption: usedEncryptionType
                }
              : c
          )
        } else {
          return [
            {
              partnerPubkey: currentConversation,
              lastMessageAt: now,
              lastMessagePreview: content.substring(0, 100),
              unreadCount: 0,
              preferredEncryption: usedEncryptionType
            },
            ...prev
          ]
        }
      })

      // Clear provisional state - conversation is now permanent
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

  // Selection mode methods
  const toggleMessageSelection = useCallback((messageId: string) => {
    setSelectedMessages((prev) => {
      const next = new Set(prev)
      if (next.has(messageId)) {
        next.delete(messageId)
        // Exit selection mode if nothing selected
        if (next.size === 0) {
          setIsSelectionMode(false)
        }
      } else {
        next.add(messageId)
        // Enter selection mode when first message selected
        if (!isSelectionMode) {
          setIsSelectionMode(true)
        }
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

  // Helper to publish deleted state to relays
  const publishDeletedState = useCallback(
    async (newState: TDMDeletedState) => {
      if (!pubkey || !encryption) return

      // Save to IndexedDB
      await indexedDb.putDeletedMessagesState(pubkey, newState)

      // Publish to relays
      const relayUrls = relayList?.write.length ? relayList.write : client.currentRelays
      const draftEvent = createDeletedMessagesDraftEvent(newState)
      const signedEvent = await encryption.signEvent(draftEvent)
      await client.publishEvent(relayUrls, signedEvent)
    },
    [pubkey, encryption, relayList]
  )

  // Delete selected messages (soft delete only - no kind 5, so undelete always works)
  const deleteSelectedMessages = useCallback(async () => {
    if (!pubkey || selectedMessages.size === 0) return

    const messageIds = Array.from(selectedMessages)

    // Update deleted state
    const newDeletedState: TDMDeletedState = {
      deletedIds: [...(deletedState?.deletedIds || []), ...messageIds],
      deletedRanges: deletedState?.deletedRanges || {}
    }
    setDeletedState(newDeletedState)

    // Remove from UI
    setMessages((prev) => prev.filter((m) => !selectedMessages.has(m.id)))
    if (currentConversation) {
      setConversationMessages((prev) => {
        const existing = prev.get(currentConversation) || []
        return new Map(prev).set(
          currentConversation,
          existing.filter((m) => !selectedMessages.has(m.id))
        )
      })
    }

    // Clear selection
    setSelectedMessages(new Set())
    setIsSelectionMode(false)

    // Publish to relays
    await publishDeletedState(newDeletedState)
  }, [pubkey, selectedMessages, deletedState, currentConversation, publishDeletedState])

  // Delete all messages in current conversation (timestamp range)
  const deleteAllInConversation = useCallback(async () => {
    if (!pubkey || !currentConversation) return

    const now = Math.floor(Date.now() / 1000)
    const newRange = { start: 0, end: now }

    // Update deleted state with new range
    const newDeletedState: TDMDeletedState = {
      deletedIds: deletedState?.deletedIds || [],
      deletedRanges: {
        ...(deletedState?.deletedRanges || {}),
        [currentConversation]: [
          ...(deletedState?.deletedRanges[currentConversation] || []),
          newRange
        ]
      }
    }
    setDeletedState(newDeletedState)

    // Clear messages from UI
    setMessages([])
    setConversationMessages((prev) => {
      const next = new Map(prev)
      next.delete(currentConversation)
      return next
    })

    // Remove conversation from list
    setConversations((prev) => prev.filter((c) => c.partnerPubkey !== currentConversation))
    setAllConversations((prev) => prev.filter((c) => c.partnerPubkey !== currentConversation))

    // Clear selection and close conversation
    setSelectedMessages(new Set())
    setIsSelectionMode(false)
    setCurrentConversation(null)

    // Publish to relays
    await publishDeletedState(newDeletedState)
  }, [pubkey, currentConversation, deletedState, publishDeletedState])

  // Undelete all messages in current conversation (remove delete markers)
  const undeleteAllInConversation = useCallback(async () => {
    if (!pubkey || !currentConversation) return

    // Remove all delete markers for this conversation
    const newDeletedState: TDMDeletedState = {
      deletedIds: deletedState?.deletedIds || [],
      deletedRanges: {
        ...(deletedState?.deletedRanges || {}),
        [currentConversation]: [] // Clear all ranges for this conversation
      }
    }
    setDeletedState(newDeletedState)

    // Clear cached messages to force reload
    setConversationMessages((prev) => {
      const next = new Map(prev)
      next.delete(currentConversation)
      return next
    })
    setLoadedConversations((prev) => {
      const next = new Set(prev)
      next.delete(currentConversation)
      return next
    })

    // Publish to relays
    await publishDeletedState(newDeletedState)

    // Trigger a background refresh of conversations
    await backgroundRefreshConversations()
  }, [pubkey, currentConversation, deletedState, publishDeletedState, backgroundRefreshConversations])

  // Filter out deleted conversations from the list
  const filteredConversations = useMemo(() => {
    if (!deletedState) return conversations
    return conversations.filter(
      (c) => !isConversationDeleted(c.partnerPubkey, c.lastMessageAt, deletedState)
    )
  }, [conversations, deletedState])

  // Calculate total unread count across all conversations
  const totalUnreadCount = useMemo(() => {
    return filteredConversations.reduce((sum, c) => sum + c.unreadCount, 0)
  }, [filteredConversations])

  // Check if there are new messages since last seen
  const newestMessageTimestamp = useMemo(() => {
    const fromConversations = filteredConversations.length === 0
      ? 0
      : Math.max(...filteredConversations.map((c) => c.lastMessageAt))
    // Also consider real-time incoming messages
    return Math.max(fromConversations, newestIncomingTimestamp)
  }, [filteredConversations, newestIncomingTimestamp])

  // Only show notification dot if:
  // 1. User has a saved lastSeenTimestamp AND there are newer messages
  // 2. OR we received a real-time message during this session (newestIncomingTimestamp > 0)
  // This prevents the dot from appearing on first load when lastSeenTimestamp is 0
  const hasNewMessages = lastSeenTimestamp > 0
    ? newestMessageTimestamp > lastSeenTimestamp
    : newestIncomingTimestamp > 0

  // Mark inbox as seen (update last seen timestamp)
  const markInboxAsSeen = useCallback(() => {
    if (!pubkey) return
    // Always clear incoming timestamp first to remove notification dot
    setNewestIncomingTimestamp(0)
    // Only update storage if we have a valid timestamp
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
        // Unread tracking
        totalUnreadCount,
        hasNewMessages,
        markInboxAsSeen,
        // Selection mode
        selectedMessages,
        isSelectionMode,
        toggleMessageSelection,
        selectAllMessages,
        clearSelection,
        // Deletion
        deleteSelectedMessages,
        deleteAllInConversation,
        undeleteAllInConversation
      }}
    >
      {children}
    </DMContext.Provider>
  )
}
