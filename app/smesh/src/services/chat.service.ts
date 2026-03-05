import client from './client.service'
import { Event as NEvent, EventTemplate } from 'nostr-tools'

export type TChannel = {
  id: string
  name: string
  about: string
  picture?: string
  creator: string
  createdAt: number
  inviteOnly: boolean
  mods: string[]
  members: string[]
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
      inviteOnly: meta.invite_only !== false,
      mods: [],
      members: []
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
  ): Promise<{ mods: string[]; members: string[]; blocked: string[]; inviteOnly: boolean } | null> {
    const events = await client.fetchEvents([relayUrl], {
      kinds: [CHANNEL_META_KIND],
      '#e': [channelId],
      limit: 1
    })
    if (events.length === 0) return null
    const ev = events[0]
    const mods: string[] = []
    const members: string[] = []
    const blocked: string[] = []
    for (const tag of ev.tags) {
      if (tag[0] === 'p' && tag[2] === 'mod') mods.push(tag[1])
      else if (tag[0] === 'p' && tag[2] === 'member') members.push(tag[1])
      else if (tag[0] === 'p' && tag[2] === 'blocked') blocked.push(tag[1])
    }
    let inviteOnly = true
    try {
      const meta = JSON.parse(ev.content)
      inviteOnly = meta.invite_only !== false
    } catch { /* keep default */ }
    return { mods, members, blocked, inviteOnly }
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

  createChannelDraft(name: string, about: string): EventTemplate {
    return {
      kind: CHANNEL_CREATE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [],
      content: JSON.stringify({ name, about, invite_only: true })
    }
  }

  createMessageDraft(channelId: string, relayUrl: string, content: string): EventTemplate {
    return {
      kind: CHANNEL_MESSAGE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [['e', channelId, relayUrl, 'root']],
      content
    }
  }

  createMetadataUpdateDraft(
    channelId: string,
    relayUrl: string,
    meta: { name?: string; about?: string; invite_only?: boolean },
    mods: string[],
    members: string[],
    blocked: string[]
  ): EventTemplate {
    const tags: string[][] = [['e', channelId, relayUrl, 'root']]
    for (const pk of mods) tags.push(['p', pk, 'mod'])
    for (const pk of members) tags.push(['p', pk, 'member'])
    for (const pk of blocked) tags.push(['p', pk, 'blocked'])
    return {
      kind: CHANNEL_META_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags,
      content: JSON.stringify(meta)
    }
  }

  createHideMessageDraft(messageEventId: string, relayUrl: string, reason = ''): EventTemplate {
    return {
      kind: CHANNEL_HIDE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [['e', messageEventId, relayUrl, 'root']],
      content: reason
    }
  }

  createBlockUserDraft(
    channelId: string,
    targetPubkey: string,
    relayUrl: string,
    reason = ''
  ): EventTemplate {
    return {
      kind: CHANNEL_MUTE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [
        ['e', channelId, relayUrl, 'root'],
        ['p', targetPubkey]
      ],
      content: reason
    }
  }
}

const chatService = new ChatService()
export default chatService
