import chatService, {
  TAccessMode,
  TChannel,
  TChannelMessage,
  TMemberEntry
} from '@/services/chat.service'
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
  createChannel: (name: string, about: string, accessMode?: TAccessMode) => Promise<void>
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
  channelMembers: TMemberEntry[]
  channelBlocked: TMemberEntry[]
  channelInvited: TMemberEntry[]
  channelRequested: string[]
  channelRejected: string[]
  channelAccessMode: TAccessMode
  hiddenMessages: Set<string>
  isOwnerOrMod: boolean
  isMember: boolean
  addMod: (pubkey: string) => Promise<void>
  removeMod: (pubkey: string) => Promise<void>
  approveMember: (pubkey: string) => Promise<void>
  removeMember: (pubkey: string) => Promise<void>
  hideMessage: (messageId: string) => Promise<void>
  blockUser: (pubkey: string) => Promise<void>
  unblockUser: (pubkey: string) => Promise<void>
  updateAccessMode: (mode: TAccessMode) => Promise<void>
  sendInvite: (pubkey: string) => Promise<void>
  revokeInvite: (pubkey: string) => Promise<void>
  acceptRequest: (pubkey: string) => Promise<void>
  rejectRequest: (pubkey: string) => Promise<void>
  revokeRejection: (pubkey: string) => Promise<void>
  // Participants (for @ mentions and member list)
  channelParticipants: string[]
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
  const [channelMembers, setChannelMembers] = useState<TMemberEntry[]>([])
  const [channelBlocked, setChannelBlocked] = useState<TMemberEntry[]>([])
  const [channelInvited, setChannelInvited] = useState<TMemberEntry[]>([])
  const [channelRequested, setChannelRequested] = useState<string[]>([])
  const [channelRejected, setChannelRejected] = useState<string[]>([])
  const [channelAccessMode, setChannelAccessMode] = useState<TAccessMode>('whitelist')
  const [hiddenMessages, setHiddenMessages] = useState<Set<string>>(new Set())

  // Keep ref in sync
  useEffect(() => {
    currentChannelRef.current = currentChannel
  }, [currentChannel])

  // Load notification prefs from localStorage on login
  useEffect(() => {
    if (!pubkey) return
    loadJsonMap(`nirc:lastSeen:${pubkey}`)
    setMutedChannels(loadStringSet(`nirc:muted:${pubkey}`))
  }, [pubkey])

  const isOwnerOrMod = useMemo(() => {
    if (!pubkey || !currentChannel) return false
    if (currentChannel.creator === pubkey) return true
    return channelMods.includes(pubkey)
  }, [pubkey, currentChannel, channelMods])

  const isMember = useMemo(() => {
    if (!pubkey || !currentChannel) return false
    if (channelAccessMode === 'open') return true
    if (currentChannel.creator === pubkey) return true
    if (channelMods.includes(pubkey)) return true
    if (channelAccessMode === 'whitelist') {
      return (
        channelMembers.some((m) => m.pubkey === pubkey) ||
        channelInvited.some((m) => m.pubkey === pubkey)
      )
    }
    if (channelAccessMode === 'blacklist') {
      return !channelBlocked.some((m) => m.pubkey === pubkey)
    }
    return false
  }, [pubkey, currentChannel, channelAccessMode, channelMods, channelMembers, channelInvited, channelBlocked])

  // Collect unique participants from messages + member list for @ mentions
  const channelParticipants = useMemo(() => {
    const pks = new Set<string>()
    for (const msg of messages) pks.add(msg.pubkey)
    for (const m of channelMembers) pks.add(m.pubkey)
    for (const m of channelInvited) pks.add(m.pubkey)
    for (const pk of channelMods) pks.add(pk)
    if (currentChannel) pks.add(currentChannel.creator)
    return [...pks]
  }, [messages, channelMembers, channelInvited, channelMods, currentChannel])

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
      let members: TMemberEntry[] = []
      let blocked: TMemberEntry[] = []
      let invited: TMemberEntry[] = []
      let requested: string[] = []
      let rejected: string[] = []
      let accessMode: TAccessMode = channel.accessMode
      if (meta) {
        mods = meta.mods
        members = meta.members
        blocked = meta.blocked
        invited = meta.invited
        requested = meta.requested
        rejected = meta.rejected
        accessMode = meta.accessMode
        // Update channel's accessMode from latest metadata
        channel.accessMode = accessMode
      }
      // Owner is always a mod
      if (!mods.includes(ownerPk)) mods = [ownerPk, ...mods]
      setChannelMods(mods)
      setChannelMembers(members)
      setChannelBlocked(blocked)
      setChannelInvited(invited)
      setChannelRequested(requested)
      setChannelRejected(rejected)
      setChannelAccessMode(accessMode)

      // Fetch hidden messages and blocked users from mod actions
      const allMods = mods
      const hidden = await chatService.fetchHiddenMessageIds(relayUrl, channel.id, allMods)
      setHiddenMessages(hidden)

      const blockedFromActions = await chatService.fetchBlockedUsers(relayUrl, channel.id, allMods)
      if (blockedFromActions.size > 0) {
        setChannelBlocked((prev) => {
          const existingPks = new Set(prev.map((e) => e.pubkey))
          const newEntries = [...blockedFromActions]
            .filter((pk) => !existingPks.has(pk))
            .map((pk) => ({ pubkey: pk, addedBy: '' }))
          return [...prev, ...newEntries]
        })
      }
    },
    [relayUrl]
  )

  const selectChannel = useCallback(
    async (channel: TChannel | null) => {
      subCloserRef.current?.close()
      subCloserRef.current = null
      seenIdsRef.current.clear()

      setCurrentChannel(channel)
      setMessages([])
      setChannelMods([])
      setChannelMembers([])
      setChannelBlocked([])
      setChannelInvited([])
      setChannelRequested([])
      setChannelRejected([])
      setChannelAccessMode('whitelist')
      setHiddenMessages(new Set())

      if (!channel) return

      markChannelAsSeen(channel.id)

      setIsLoadingMessages(true)
      try {
        const [msgs] = await Promise.all([
          chatService.fetchMessages(relayUrl, channel.id),
          loadModState(channel)
        ])
        setMessages(msgs)
        msgs.forEach((m) => seenIdsRef.current.add(m.id))
      } finally {
        setIsLoadingMessages(false)
      }

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

  const pendingChannelIdRef = useRef<string | null>(null)

  const selectChannelById = useCallback(
    (channelId: string | null) => {
      if (!channelId) {
        pendingChannelIdRef.current = null
        selectChannel(null)
        return
      }
      const ch = channels.find((c) => c.id === channelId)
      if (ch) {
        pendingChannelIdRef.current = null
        selectChannel(ch)
      } else {
        pendingChannelIdRef.current = channelId
      }
    },
    [channels, selectChannel]
  )

  useEffect(() => {
    if (pendingChannelIdRef.current && channels.length > 0) {
      const ch = channels.find((c) => c.id === pendingChannelIdRef.current)
      if (ch) {
        pendingChannelIdRef.current = null
        selectChannel(ch)
      }
    }
  }, [channels, selectChannel])

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

    const globalSub = client.subscribe(
      [relayUrl],
      {
        kinds: [42],
        '#e': channelIds,
        since: Math.floor(Date.now() / 1000)
      },
      {
        onevent: (event: any) => {
          if (event.pubkey === pubkey) return
          const eTag = event.tags?.find(
            (t: string[]) => t[0] === 'e' && (t[3] === 'root' || t.length === 2)
          )
          if (!eTag) return
          const chId = eTag[1]
          if (currentChannelRef.current?.id === chId) return
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
    async (name: string, about: string, accessMode: TAccessMode = 'whitelist') => {
      if (!pubkey) return
      const draft = chatService.createChannelDraft(name, about, accessMode)
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
      members: TMemberEntry[],
      blocked: TMemberEntry[],
      invited: TMemberEntry[],
      requested: string[],
      rejected: string[],
      accessMode?: TAccessMode
    ) => {
      if (!currentChannel || !pubkey) return
      const meta: Record<string, unknown> = {
        name: currentChannel.name,
        about: currentChannel.about,
        access_mode: accessMode ?? channelAccessMode
      }

      const draft = chatService.createMetadataUpdateDraft(
        currentChannel.id,
        relayUrl,
        meta as any,
        mods.filter((pk) => pk !== currentChannel.creator),
        members,
        blocked,
        invited,
        requested,
        rejected
      )
      const signed = await signEvent(draft)
      await client.publishEvent([relayUrl], signed)
    },
    [currentChannel, relayUrl, pubkey, signEvent, channelAccessMode]
  )

  const addMod = useCallback(
    async (pk: string) => {
      const newMods = [...channelMods, pk]
      setChannelMods(newMods)
      await publishMetadataUpdate(newMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const removeMod = useCallback(
    async (pk: string) => {
      // Cascade: remove all members/blocked/invited that this mod added
      const newMods = channelMods.filter((m) => m !== pk)
      const newMembers = channelMembers.filter((m) => m.addedBy !== pk)
      const newBlocked = channelBlocked.filter((m) => m.addedBy !== pk)
      const newInvited = channelInvited.filter((m) => m.addedBy !== pk)
      setChannelMods(newMods)
      setChannelMembers(newMembers)
      setChannelBlocked(newBlocked)
      setChannelInvited(newInvited)
      await publishMetadataUpdate(newMods, newMembers, newBlocked, newInvited, channelRequested, channelRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const approveMember = useCallback(
    async (pk: string) => {
      if (!pubkey) return
      const entry: TMemberEntry = { pubkey: pk, addedBy: pubkey }
      const newMembers = [...channelMembers, entry]
      // Remove from requested if present
      const newRequested = channelRequested.filter((r) => r !== pk)
      setChannelMembers(newMembers)
      setChannelRequested(newRequested)
      await publishMetadataUpdate(channelMods, newMembers, channelBlocked, channelInvited, newRequested, channelRejected)
    },
    [pubkey, channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const removeMember = useCallback(
    async (pk: string) => {
      const newMembers = channelMembers.filter((m) => m.pubkey !== pk)
      setChannelMembers(newMembers)
      await publishMetadataUpdate(channelMods, newMembers, channelBlocked, channelInvited, channelRequested, channelRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
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
      const entry: TMemberEntry = { pubkey: targetPubkey, addedBy: pubkey }
      setChannelBlocked((prev) => [...prev, entry])
    },
    [currentChannel, relayUrl, pubkey, signEvent]
  )

  const unblockUser = useCallback(
    async (targetPubkey: string) => {
      const newBlocked = channelBlocked.filter((e) => e.pubkey !== targetPubkey)
      setChannelBlocked(newBlocked)
      await publishMetadataUpdate(channelMods, channelMembers, newBlocked, channelInvited, channelRequested, channelRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const updateAccessMode = useCallback(
    async (mode: TAccessMode) => {
      setChannelAccessMode(mode)
      setCurrentChannel((prev) => (prev ? { ...prev, accessMode: mode } : null))
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, mode)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const sendInvite = useCallback(
    async (targetPubkey: string) => {
      if (!currentChannel || !pubkey) return
      const entry: TMemberEntry = { pubkey: targetPubkey, addedBy: pubkey }
      const newInvited = [...channelInvited, entry]
      setChannelInvited(newInvited)
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, newInvited, channelRequested, channelRejected)

      // Send DM with channel link
      const link = `https://smesh.mleku.dev/#/chat/${currentChannel.id}`
      const dmContent = `You've been invited to #${currentChannel.name} on NIRC:\n${link}`
      const dmDraft = {
        kind: 4,
        created_at: Math.floor(Date.now() / 1000),
        tags: [['p', targetPubkey]],
        content: dmContent
      }
      try {
        const signed = await signEvent(dmDraft)
        await client.publishEvent([relayUrl], signed)
      } catch {
        // DM send failure is non-fatal — invite is already in metadata
      }
    },
    [currentChannel, pubkey, channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate, signEvent, relayUrl]
  )

  const revokeInvite = useCallback(
    async (targetPubkey: string) => {
      const newInvited = channelInvited.filter((e) => e.pubkey !== targetPubkey)
      setChannelInvited(newInvited)
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, newInvited, channelRequested, channelRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const acceptRequest = useCallback(
    async (pk: string) => {
      // Move from requested to member
      await approveMember(pk)
    },
    [approveMember]
  )

  const rejectRequest = useCallback(
    async (pk: string) => {
      const newRequested = channelRequested.filter((r) => r !== pk)
      const newRejected = [...channelRejected, pk]
      setChannelRequested(newRequested)
      setChannelRejected(newRejected)
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, channelInvited, newRequested, newRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  const revokeRejection = useCallback(
    async (pk: string) => {
      const newRejected = channelRejected.filter((r) => r !== pk)
      setChannelRejected(newRejected)
      await publishMetadataUpdate(channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, newRejected)
    },
    [channelMods, channelMembers, channelBlocked, channelInvited, channelRequested, channelRejected, publishMetadataUpdate]
  )

  // Filter messages: hide hidden messages and blocked users
  const filteredMessages = useMemo(() => {
    const blockedSet = new Set(channelBlocked.map((e) => e.pubkey))
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
        channelInvited,
        channelRequested,
        channelRejected,
        channelAccessMode,
        hiddenMessages,
        isOwnerOrMod,
        isMember,
        addMod,
        removeMod,
        approveMember,
        removeMember,
        hideMessage,
        blockUser,
        unblockUser,
        updateAccessMode,
        sendInvite,
        revokeInvite,
        acceptRequest,
        rejectRequest,
        revokeRejection,
        channelParticipants
      }}
    >
      {children}
    </ChatContext.Provider>
  )
}
