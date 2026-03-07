import client from './client.service'
import { Event as NEvent, EventTemplate } from 'nostr-tools'

// --- Access modes ---

export type TAccessMode = 'open' | 'whitelist' | 'blacklist'

// --- Message expiry ---

/** Default message expiry: 4 weeks in seconds */
export const DEFAULT_MESSAGE_EXPIRY = 4 * 7 * 24 * 60 * 60

/** Configurable expiry options for channel settings */
export const EXPIRY_OPTIONS = [
  { label: '1 week', value: 7 * 24 * 60 * 60 },
  { label: '2 weeks', value: 2 * 7 * 24 * 60 * 60 },
  { label: '4 weeks', value: 4 * 7 * 24 * 60 * 60 },
  { label: '2 months', value: 2 * 30 * 24 * 60 * 60 },
  { label: '3 months', value: 3 * 30 * 24 * 60 * 60 },
  { label: '6 months', value: 6 * 30 * 24 * 60 * 60 },
  { label: '9 months', value: 9 * 30 * 24 * 60 * 60 },
  { label: '12 months', value: 12 * 30 * 24 * 60 * 60 },
] as const

// --- Member entry with provenance tracking ---

export type TMemberEntry = {
  pubkey: string
  addedBy: string // pubkey of who added them (owner or mod)
}

// --- Channel types ---

export type TChannel = {
  id: string
  name: string
  about: string
  picture?: string
  creator: string
  createdAt: number
  accessMode: TAccessMode
  messageExpiry?: number // seconds until messages expire (NIP-40)
  mods: string[]
  members: TMemberEntry[]
  blocked: TMemberEntry[]
  invited: TMemberEntry[]
  requested: string[]
  rejected: string[]
}

export type TChannelMessage = {
  id: string
  channelId: string
  content: string
  pubkey: string
  createdAt: number
  event: NEvent
}

export type TModAction = {
  id: string
  kind: number
  pubkey: string
  channelId: string
  targetEventId?: string
  targetPubkey?: string
  reason: string
  createdAt: number
}

const CHANNEL_CREATE_KIND = 40
const CHANNEL_META_KIND = 41
const CHANNEL_MESSAGE_KIND = 42
const CHANNEL_HIDE_KIND = 43
const CHANNEL_MUTE_KIND = 44

/** Parse access_mode from kind 41 content, with backward compat for invite_only */
function parseAccessMode(meta: Record<string, unknown>): TAccessMode {
  if (meta.access_mode === 'open' || meta.access_mode === 'whitelist' || meta.access_mode === 'blacklist') {
    return meta.access_mode
  }
  // Backward compat: invite_only: true → whitelist, false → open
  if (meta.invite_only === false) return 'open'
  return 'whitelist'
}

function parseChannelFromEvent(event: NEvent): TChannel | null {
  try {
    const meta = JSON.parse(event.content)
    return {
      id: event.id,
      name: meta.name || 'unnamed',
      about: meta.about || '',
      picture: meta.picture,
      creator: event.pubkey,
      createdAt: event.created_at,
      accessMode: parseAccessMode(meta),
      messageExpiry: typeof meta.message_expiry === 'number' ? meta.message_expiry : undefined,
      mods: [],
      members: [],
      blocked: [],
      invited: [],
      requested: [],
      rejected: []
    }
  } catch {
    return null
  }
}

function parseMessageFromEvent(event: NEvent): TChannelMessage | null {
  const channelTag = event.tags.find(
    (t) => t[0] === 'e' && (t[3] === 'root' || t.length === 2)
  )
  if (!channelTag) return null

  return {
    id: event.id,
    channelId: channelTag[1],
    content: event.content,
    pubkey: event.pubkey,
    createdAt: event.created_at,
    event
  }
}

export type TChannelMeta = {
  mods: string[]
  members: TMemberEntry[]
  blocked: TMemberEntry[]
  invited: TMemberEntry[]
  requested: string[]
  rejected: string[]
  accessMode: TAccessMode
  messageExpiry?: number
}

class ChatService {
  async fetchChannels(relayUrl: string): Promise<TChannel[]> {
    const events = await client.fetchEvents([relayUrl], {
      kinds: [CHANNEL_CREATE_KIND],
      limit: 100
    })
    return events
      .map(parseChannelFromEvent)
      .filter((ch): ch is TChannel => ch !== null)
      .sort((a, b) => b.createdAt - a.createdAt)
  }

  async fetchMessages(
    relayUrl: string,
    channelId: string,
    limit = 50,
    until?: number
  ): Promise<TChannelMessage[]> {
    const filter: Record<string, unknown> = {
      kinds: [CHANNEL_MESSAGE_KIND],
      '#e': [channelId],
      limit
    }
    if (until) filter.until = until

    const events = await client.fetchEvents([relayUrl], filter as any)
    return events
      .map(parseMessageFromEvent)
      .filter((m): m is TChannelMessage => m !== null)
      .sort((a, b) => a.createdAt - b.createdAt)
  }

  async fetchChannelMeta(
    relayUrl: string,
    channelId: string
  ): Promise<TChannelMeta | null> {
    const events = await client.fetchEvents([relayUrl], {
      kinds: [CHANNEL_META_KIND],
      '#e': [channelId],
      limit: 1
    })
    if (events.length === 0) return null
    const ev = events[0]
    const mods: string[] = []
    const members: TMemberEntry[] = []
    const blocked: TMemberEntry[] = []
    const invited: TMemberEntry[] = []
    const requested: string[] = []
    const rejected: string[] = []
    for (const tag of ev.tags) {
      if (tag[0] !== 'p') continue
      const pk = tag[1]
      const role = tag[2]
      const addedBy = tag[3] || ''
      if (role === 'mod') mods.push(pk)
      else if (role === 'member') members.push({ pubkey: pk, addedBy })
      else if (role === 'blocked') blocked.push({ pubkey: pk, addedBy })
      else if (role === 'invited') invited.push({ pubkey: pk, addedBy })
      else if (role === 'requested') requested.push(pk)
      else if (role === 'rejected') rejected.push(pk)
    }
    let accessMode: TAccessMode = 'whitelist'
    let messageExpiry: number | undefined
    try {
      const meta = JSON.parse(ev.content)
      accessMode = parseAccessMode(meta)
      if (typeof meta.message_expiry === 'number') {
        messageExpiry = meta.message_expiry
      }
    } catch { /* keep default */ }
    return { mods, members, blocked, invited, requested, rejected, accessMode, messageExpiry }
  }

  async fetchHiddenMessageIds(
    relayUrl: string,
    _channelId: string,
    modPubkeys: string[]
  ): Promise<Set<string>> {
    if (modPubkeys.length === 0) return new Set()
    const events = await client.fetchEvents([relayUrl], {
      kinds: [CHANNEL_HIDE_KIND],
      authors: modPubkeys,
      limit: 500
    })
    const hidden = new Set<string>()
    for (const ev of events) {
      const eTag = ev.tags.find((t) => t[0] === 'e')
      if (eTag) hidden.add(eTag[1])
    }
    return hidden
  }

  async fetchBlockedUsers(
    relayUrl: string,
    channelId: string,
    modPubkeys: string[]
  ): Promise<Set<string>> {
    if (modPubkeys.length === 0) return new Set()
    const events = await client.fetchEvents([relayUrl], {
      kinds: [CHANNEL_MUTE_KIND],
      '#e': [channelId],
      authors: modPubkeys,
      limit: 500
    })
    const blocked = new Set<string>()
    for (const ev of events) {
      const pTag = ev.tags.find((t) => t[0] === 'p')
      if (pTag) blocked.add(pTag[1])
    }
    return blocked
  }

  subscribeMessages(
    relayUrl: string,
    channelId: string,
    onMessage: (msg: TChannelMessage) => void
  ) {
    return client.subscribe([relayUrl], {
      kinds: [CHANNEL_MESSAGE_KIND],
      '#e': [channelId],
      since: Math.floor(Date.now() / 1000)
    }, {
      onevent: (event: NEvent) => {
        const msg = parseMessageFromEvent(event)
        if (msg) onMessage(msg)
      }
    })
  }

  createChannelDraft(name: string, about: string, accessMode: TAccessMode = 'whitelist'): EventTemplate {
    return {
      kind: CHANNEL_CREATE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [],
      content: JSON.stringify({ name, about, access_mode: accessMode })
    }
  }

  createMessageDraft(channelId: string, relayUrl: string, content: string, expirySecs?: number): EventTemplate {
    const now = Math.floor(Date.now() / 1000)
    const expiry = expirySecs ?? DEFAULT_MESSAGE_EXPIRY
    return {
      kind: CHANNEL_MESSAGE_KIND,
      created_at: now,
      tags: [
        ['e', channelId, relayUrl, 'root'],
        ['expiration', String(now + expiry)]
      ],
      content
    }
  }

  createMetadataUpdateDraft(
    channelId: string,
    relayUrl: string,
    meta: { name?: string; about?: string; access_mode?: TAccessMode; message_expiry?: number },
    mods: string[],
    members: TMemberEntry[],
    blocked: TMemberEntry[],
    invited: TMemberEntry[],
    requested: string[],
    rejected: string[]
  ): EventTemplate {
    const tags: string[][] = [['e', channelId, relayUrl, 'root']]
    for (const pk of mods) tags.push(['p', pk, 'mod'])
    for (const m of members) tags.push(['p', m.pubkey, 'member', m.addedBy])
    for (const b of blocked) tags.push(['p', b.pubkey, 'blocked', b.addedBy])
    for (const inv of invited) tags.push(['p', inv.pubkey, 'invited', inv.addedBy])
    for (const pk of requested) tags.push(['p', pk, 'requested'])
    for (const pk of rejected) tags.push(['p', pk, 'rejected'])
    return {
      kind: CHANNEL_META_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags,
      content: JSON.stringify(meta)
    }
  }

  createHideMessageDraft(messageEventId: string, relayUrl: string, reason = ''): EventTemplate {
    const now = Math.floor(Date.now() / 1000)
    return {
      kind: CHANNEL_HIDE_KIND,
      created_at: now,
      tags: [
        ['e', messageEventId, relayUrl, 'root'],
        ['expiration', String(now + DEFAULT_MESSAGE_EXPIRY)]
      ],
      content: reason
    }
  }

  createBlockUserDraft(
    channelId: string,
    targetPubkey: string,
    relayUrl: string,
    reason = ''
  ): EventTemplate {
    const now = Math.floor(Date.now() / 1000)
    return {
      kind: CHANNEL_MUTE_KIND,
      created_at: now,
      tags: [
        ['e', channelId, relayUrl, 'root'],
        ['p', targetPubkey],
        ['expiration', String(now + DEFAULT_MESSAGE_EXPIRY)]
      ],
      content: reason
    }
  }
}

const chatService = new ChatService()
export default chatService
