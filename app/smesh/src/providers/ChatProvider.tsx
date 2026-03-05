import chatService, { TChannel, TChannelMessage } from '@/services/chat.service'
import client from '@/services/client.service'
import { useNostr } from '@/providers/NostrProvider'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState
} from 'react'

// --- localStorage helpers ---

function loadJsonMap(key: string): Record<string, number> {
  try {
    return JSON.parse(localStorage.getItem(key) || '{}')
  } catch {
    return {}
  }
}

function saveJsonMap(key: string, map: Record<string, number>) {
  localStorage.setItem(key, JSON.stringify(map))
}

function loadStringSet(key: string): Set<string> {
  try {
    return new Set(JSON.parse(localStorage.getItem(key) || '[]'))
  } catch {
    return new Set()
  }
}

function saveStringSet(key: string, set: Set<string>) {
  localStorage.setItem(key, JSON.stringify([...set]))
}

// --- Types ---

type TChatContext = {
  // Channel state
  channels: TChannel[]
  currentChannel: TChannel | null
  messages: TChannelMessage[]
  isLoadingChannels: boolean
  isLoadingMessages: boolean
  relayUrl: string
  setRelayUrl: (url: string) => void
  selectChannel: (channel: TChannel | null) => void
  selectChannelById: (channelId: string | null) => void
  sendMessage: (content: string) => Promise<void>
  createChannel: (name: string, about: string) => Promise<void>
  refreshChannels: () => Promise<void>
  loadMoreMessages: () => Promise<void>
  // Notifications
  unreadCounts: Record<string, number>
  hasUnreadChannels: boolean
  mutedChannels: Set<string>
  markChannelAsSeen: (channelId: string) => void
  toggleMuteChannel: (channelId: string) => void
  // Moderation
  channelMods: string[]
  channelMembers: string[]
  channelBlocked: string[]
  hiddenMessages: Set<string>
  isOwnerOrMod: boolean
  addMod: (pubkey: string) => Promise<void>
  removeMod: (pubkey: string) => Promise<void>
  approveMember: (pubkey: string) => Promise<void>
  removeMember: (pubkey: string) => Promise<void>
  hideMessage: (messageId: string) => Promise<void>
  blockUser: (pubkey: string) => Promise<void>
  unblockUser: (pubkey: string) => Promise<void>
  updateChannelSettings: (inviteOnly: boolean) => Promise<void>
}

const ChatContext = createContext<TChatContext | undefined>(undefined)

export function useChat() {
  const ctx = useContext(ChatContext)
  if (!ctx) throw new Error('useChat must be used within ChatProvider')
  return ctx
}

const DEFAULT_RELAY = 'wss://relay.orly.dev/'

export function ChatProvider({ children }: { children: React.ReactNode }) {
  const { pubkey, signEvent } = useNostr()
  const [relayUrl, setRelayUrl] = useState(DEFAULT_RELAY)
  const [channels, setChannels] = useState<TChannel[]>([])
  const [currentChannel, setCurrentChannel] = useState<TChannel | null>(null)
  const [messages, setMessages] = useState<TChannelMessage[]>([])
  const [isLoadingChannels, setIsLoadingChannels] = useState(false)
  const [isLoadingMessages, setIsLoadingMessages] = useState(false)
  const subCloserRef = useRef<{ close: () => void } | null>(null)
  const seenIdsRef = useRef(new Set<string>())

  // Notification state
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({})
  const [mutedChannels, setMutedChannels] = useState<Set<string>>(new Set())
  const [, setLastSeenTimestamps] = useState<Record<string, number>>({})
  const currentChannelRef = useRef<TChannel | null>(null)

  // Moderation state (for current channel)
  const [channelMods, setChannelMods] = useState<string[]>([])
  const [channelMembers, setChannelMembers] = useState<string[]>([])
  const [channelBlocked, setChannelBlocked] = useState<string[]>([])
  const [hiddenMessages, setHiddenMessages] = useState<Set<string>>(new Set())

  // Keep ref in sync
  useEffect(() => {
    currentChannelRef.current = currentChannel
  }, [currentChannel])

  // Load notification prefs from localStorage on login
  useEffect(() => {
    if (!pubkey) return
    loadJsonMap(`nirc:lastSeen:${pubkey}`) // Warm localStorage, used by markChannelAsSeen
    setMutedChannels(loadStringSet(`nirc:muted:${pubkey}`))
  }, [pubkey])

  const isOwnerOrMod = useMemo(() => {
    if (!pubkey || !currentChannel) return false
    if (currentChannel.creator === pubkey) return true
    return channelMods.includes(pubkey)
  }, [pubkey, currentChannel, channelMods])

  const hasUnreadChannels = useMemo(() => {
    return Object.entries(unreadCounts).some(
      ([chId, count]) => count > 0 && !mutedChannels.has(chId)
    )
  }, [unreadCounts, mutedChannels])

  const markChannelAsSeen = useCallback(
    (channelId: string) => {
      setUnreadCounts((prev) => {
        if (!prev[channelId]) return prev
        const next = { ...prev }
        delete next[channelId]
        return next
      })
      const now = Math.floor(Date.now() / 1000)
      setLastSeenTimestamps((prev) => {
        const next = { ...prev, [channelId]: now }
        if (pubkey) saveJsonMap(`nirc:lastSeen:${pubkey}`, next)
        return next
      })
    },
    [pubkey]
  )

  const toggleMuteChannel = useCallback(
    (channelId: string) => {
      setMutedChannels((prev) => {
        const next = new Set(prev)
        if (next.has(channelId)) {
          next.delete(channelId)
        } else {
          next.add(channelId)
        }
        if (pubkey) saveStringSet(`nirc:muted:${pubkey}`, next)
        return next
      })
    },
    [pubkey]
  )

  const refreshChannels = useCallback(async () => {
    setIsLoadingChannels(true)
    try {
      const chs = await chatService.fetchChannels(relayUrl)
      setChannels(chs)
    } finally {
      setIsLoadingChannels(false)
    }
  }, [relayUrl])

  // Load channels on mount and relay change
  useEffect(() => {
    refreshChannels()
  }, [refreshChannels])

  // Fetch moderation state for a channel
  const loadModState = useCallback(
    async (channel: TChannel) => {
      const meta = await chatService.fetchChannelMeta(relayUrl, channel.id)
      const ownerPk = channel.creator
      let mods: string[] = []
      let members: string[] = []
      let blocked: string[] = []
      if (meta) {
        mods = meta.mods
        members = meta.members
        blocked = meta.blocked
        // Update channel's inviteOnly from latest metadata
        channel.inviteOnly = meta.inviteOnly
      }
      // Owner is always a mod
      if (!mods.includes(ownerPk)) mods = [ownerPk, ...mods]
      setChannelMods(mods)
      setChannelMembers(members)
      setChannelBlocked(blocked)

      // Fetch hidden messages and blocked users from mod actions
      const allMods = mods
      const hidden = await chatService.fetchHiddenMessageIds(relayUrl, channel.id, allMods)
      setHiddenMessages(hidden)

      const blockedFromActions = await chatService.fetchBlockedUsers(relayUrl, channel.id, allMods)
      if (blockedFromActions.size > 0) {
        setChannelBlocked((prev) => {
          const combined = new Set([...prev, ...blockedFromActions])
          return [...combined]
        })
      }
    },
    [relayUrl]
  )

  const selectChannel = useCallback(
    async (channel: TChannel | null) => {
      // Close previous subscription
      subCloserRef.current?.close()
      subCloserRef.current = null
      seenIdsRef.current.clear()

      setCurrentChannel(channel)
      setMessages([])
      setChannelMods([])
      setChannelMembers([])
      setChannelBlocked([])
      setHiddenMessages(new Set())

      if (!channel) return

      // Mark as seen
      markChannelAsSeen(channel.id)

      setIsLoadingMessages(true)
      try {
        // Load messages and mod state in parallel
        const [msgs] = await Promise.all([
          chatService.fetchMessages(relayUrl, channel.id),
          loadModState(channel)
        ])
        setMessages(msgs)
        msgs.forEach((m) => seenIdsRef.current.add(m.id))
      } finally {
        setIsLoadingMessages(false)
      }

      // Subscribe for new messages
      subCloserRef.current = chatService.subscribeMessages(
        relayUrl,
        channel.id,
        (msg) => {
          if (seenIdsRef.current.has(msg.id)) return
          seenIdsRef.current.add(msg.id)
          setMessages((prev) => [...prev, msg])
        }
      )
    },
    [relayUrl, markChannelAsSeen, loadModState]
  )

  const selectChannelById = useCallback(
    (channelId: string | null) => {
      if (!channelId) {
        selectChannel(null)
        return
      }
      const ch = channels.find((c) => c.id === channelId)
      if (ch) selectChannel(ch)
    },
    [channels, selectChannel]
  )

  // Cleanup subscription on unmount
  useEffect(() => {
    return () => {
      subCloserRef.current?.close()
    }
  }, [])

  // Global subscription for unread tracking across all channels
  useEffect(() => {
    if (!pubkey || channels.length === 0) return

    const channelIds = channels.map((ch) => ch.id)

    // Subscribe to all channel messages for unread tracking
    const globalSub = client.subscribe(
      [relayUrl],
      {
        kinds: [42],
        '#e': channelIds,
        since: Math.floor(Date.now() / 1000)
      },
      {
        onevent: (event: any) => {
          if (event.pubkey === pubkey) return // Own messages don't count
          const eTag = event.tags?.find(
            (t: string[]) => t[0] === 'e' && (t[3] === 'root' || t.length === 2)
          )
          if (!eTag) return
          const chId = eTag[1]
          // Don't increment if this is the currently viewed channel
          if (currentChannelRef.current?.id === chId) return
          // Don't increment if muted
          if (mutedChannels.has(chId)) return

          setUnreadCounts((prev) => ({
            ...prev,
            [chId]: (prev[chId] || 0) + 1
          }))
        }
      }
    )

    return () => {
      globalSub.close()
    }
  }, [pubkey, channels, relayUrl, mutedChannels])

  const sendMessage = useCallback(
    async (content: string) => {
      if (!currentChannel || !pubkey) return
      const draft = chatService.createMessageDraft(currentChannel.id, relayUrl, content)
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
    },
    [currentChannel, relayUrl, pubkey, signEvent]
  )

  const createChannel = useCallback(
    async (name: string, about: string) => {
      if (!pubkey) return
      const draft = chatService.createChannelDraft(name, about)
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
      await refreshChannels()
    },
    [relayUrl, pubkey, signEvent, refreshChannels]
  )

  const loadMoreMessages = useCallback(async () => {
    if (!currentChannel || messages.length === 0) return
    const oldest = messages[0]
    const older = await chatService.fetchMessages(
      relayUrl,
      currentChannel.id,
      50,
      oldest.createdAt - 1
    )
    older.forEach((m) => seenIdsRef.current.add(m.id))
    setMessages((prev) => [...older, ...prev])
  }, [currentChannel, messages, relayUrl])

  // --- Moderation actions ---

  const publishMetadataUpdate = useCallback(
    async (
      mods: string[],
      members: string[],
      blocked: string[],
      inviteOnly?: boolean
    ) => {
      if (!currentChannel || !pubkey) return
      const meta: Record<string, unknown> = {
        name: currentChannel.name,
        about: currentChannel.about
      }
      if (inviteOnly !== undefined) meta.invite_only = inviteOnly
      else meta.invite_only = currentChannel.inviteOnly

      const draft = chatService.createMetadataUpdateDraft(
        currentChannel.id,
        relayUrl,
        meta as any,
        mods.filter((pk) => pk !== currentChannel.creator), // Don't list owner as mod in tags
        members,
        blocked
      )
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
    },
    [currentChannel, relayUrl, pubkey, signEvent]
  )

  const addMod = useCallback(
    async (pk: string) => {
      const newMods = [...channelMods, pk]
      setChannelMods(newMods)
      await publishMetadataUpdate(newMods, channelMembers, channelBlocked)
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  const removeMod = useCallback(
    async (pk: string) => {
      const newMods = channelMods.filter((m) => m !== pk)
      setChannelMods(newMods)
      await publishMetadataUpdate(newMods, channelMembers, channelBlocked)
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  const approveMember = useCallback(
    async (pk: string) => {
      const newMembers = [...channelMembers, pk]
      setChannelMembers(newMembers)
      await publishMetadataUpdate(channelMods, newMembers, channelBlocked)
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  const removeMember = useCallback(
    async (pk: string) => {
      const newMembers = channelMembers.filter((m) => m !== pk)
      setChannelMembers(newMembers)
      await publishMetadataUpdate(channelMods, newMembers, channelBlocked)
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  const hideMessage = useCallback(
    async (messageId: string) => {
      if (!pubkey) return
      const draft = chatService.createHideMessageDraft(messageId, relayUrl)
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
      setHiddenMessages((prev) => new Set([...prev, messageId]))
    },
    [relayUrl, pubkey, signEvent]
  )

  const blockUser = useCallback(
    async (targetPubkey: string) => {
      if (!currentChannel || !pubkey) return
      const draft = chatService.createBlockUserDraft(
        currentChannel.id,
        targetPubkey,
        relayUrl
      )
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
      setChannelBlocked((prev) => [...prev, targetPubkey])
    },
    [currentChannel, relayUrl, pubkey, signEvent]
  )

  const unblockUser = useCallback(
    async (targetPubkey: string) => {
      const newBlocked = channelBlocked.filter((pk) => pk !== targetPubkey)
      setChannelBlocked(newBlocked)
      await publishMetadataUpdate(channelMods, channelMembers, newBlocked)
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  const updateChannelSettings = useCallback(
    async (inviteOnly: boolean) => {
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, inviteOnly)
      setCurrentChannel((prev) => (prev ? { ...prev, inviteOnly } : null))
    },
    [channelMods, channelMembers, channelBlocked, publishMetadataUpdate]
  )

  // Filter messages: hide hidden messages and blocked users
  const filteredMessages = useMemo(() => {
    const blockedSet = new Set(channelBlocked)
    return messages.filter(
      (msg) => !hiddenMessages.has(msg.id) && !blockedSet.has(msg.pubkey)
    )
  }, [messages, hiddenMessages, channelBlocked])

  return (
    <ChatContext.Provider
      value={{
        channels,
        currentChannel,
        messages: filteredMessages,
        isLoadingChannels,
        isLoadingMessages,
        relayUrl,
        setRelayUrl,
        selectChannel,
        selectChannelById,
        sendMessage,
        createChannel,
        refreshChannels,
        loadMoreMessages,
        // Notifications
        unreadCounts,
        hasUnreadChannels,
        mutedChannels,
        markChannelAsSeen,
        toggleMuteChannel,
        // Moderation
        channelMods,
        channelMembers,
        channelBlocked,
        hiddenMessages,
        isOwnerOrMod,
        addMod,
        removeMod,
        approveMember,
        removeMember,
        hideMessage,
        blockUser,
        unblockUser,
        updateChannelSettings
      }}
    >
      {children}
    </ChatContext.Provider>
  )
}
