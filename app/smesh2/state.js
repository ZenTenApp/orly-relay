// state.js — reducer, dispatch, initial state

import { DEFAULT_RELAYS } from './helpers.js'

// ─── state ──────────────────────────────────────────────────────────

export let state = null
let _renderFn = null

export function initState(renderFn) {
  _renderFn = renderFn
  const storedPubkey = localStorage.getItem('smesh2-pubkey')
  const storedMode = localStorage.getItem('smesh2-loginMode') || 'extension'
  const hasEncrypted = !!localStorage.getItem('smesh2-enc')
  const canAutoRestore = storedPubkey && (storedMode === 'extension' || (storedMode === 'nsec' && !hasEncrypted))
  const needsPasswordPrompt = storedPubkey && storedMode === 'nsec' && hasEncrypted

  state = {
    pubkey: canAutoRestore ? storedPubkey : null,
    loginMode: canAutoRestore ? storedMode : null,
    hasStoredSession: !!needsPasswordPrompt,
    profile: {},
    profileTs: 0,
    profiles: new Map(),
    contacts: [],
    relays: JSON.parse(localStorage.getItem('smesh2-relays') || 'null') || DEFAULT_RELAYS,
    feed: [],
    pendingNotes: [],
    feedReady: false,
    feedLoading: false,
    feedPage: 0,
    feedExhausted: false,
    feedLenBefore: 0,
    activeTab: 'feed',
    snackbar: null,
    threadEventId: null,
    threadRootId: null,
    threadRelayHints: [],
    threadEvents: [],
    threadQueriedIds: [],
    orlyRelays: [],
    embeddedNotes: new Map(),
    lightboxUrl: null,
    hashtagQuery: '',
    hashtagFeed: [],
    hashtagLoading: false,
    hashtagPage: 0,
    hashtagExhausted: false,
    hashtagLenBefore: 0,
    relayFeed: [],
    relayFeedLoading: false,
    relayFeedPage: 0,
    relayFeedExhausted: false,
    relayFeedLenBefore: 0,
    conversations: [],
    activeDM: null,
    dmMessages: [],
    dmTab: 'list',
    sidebarOpen: false,
  }
}

export function dispatch(action) {
  const prev = state
  state = reducer(state, action)
  if (state !== prev && _renderFn) _renderFn()
}

// ─── reducer ────────────────────────────────────────────────────────

function reducer(state, action) {
  switch (action.type) {
    case 'LOGIN': {
      const mode = action.loginMode || 'extension'
      localStorage.setItem('smesh2-pubkey', action.pubkey)
      localStorage.setItem('smesh2-loginMode', mode)
      return { ...state, pubkey: action.pubkey, loginMode: mode, hasStoredSession: false, feedReady: false }
    }
    case 'CLEAR_STORED_SESSION':
      localStorage.removeItem('smesh2-pubkey')
      localStorage.removeItem('smesh2-loginMode')
      return { ...state, hasStoredSession: false }
    case 'SET_TAB':
      return { ...state, activeTab: action.tab }
    case 'SET_SIDEBAR':
      return { ...state, sidebarOpen: action.open }
    case 'SET_PROFILE': {
      if (state.profileTs && action.ts < state.profileTs) return state
      return { ...state, profile: action.profile, profileTs: action.ts }
    }
    case 'ADD_PROFILE': {
      const existing = state.profiles.get(action.pubkey)
      if (existing && existing._ts >= action.ts) return state
      const profiles = new Map(state.profiles)
      profiles.set(action.pubkey, { ...action.profile, _ts: action.ts })
      return { ...state, profiles }
    }
    case 'SET_CONTACTS':
      return { ...state, contacts: action.contacts }
    case 'SET_RELAYS': {
      localStorage.setItem('smesh2-relays', JSON.stringify(action.relays))
      return { ...state, relays: action.relays }
    }
    case 'ADD_RELAY': {
      if (state.relays.includes(action.url)) return state
      const relays = [...state.relays, action.url]
      localStorage.setItem('smesh2-relays', JSON.stringify(relays))
      return { ...state, relays }
    }
    case 'REMOVE_RELAY': {
      const relays = state.relays.filter((r) => r !== action.url)
      localStorage.setItem('smesh2-relays', JSON.stringify(relays))
      return { ...state, relays }
    }
    case 'ADD_EVENT': {
      if (state.feed.some((e) => e.id === action.event.id)) return state
      if (state.pendingNotes.some((e) => e.id === action.event.id)) return state
      return { ...state, feed: [...state.feed, action.event] }
    }
    case 'ADD_PENDING_NOTE': {
      if (state.feed.some((e) => e.id === action.event.id)) return state
      if (state.pendingNotes.some((e) => e.id === action.event.id)) return state
      return { ...state, pendingNotes: [...state.pendingNotes, action.event] }
    }
    case 'FLUSH_PENDING': {
      if (!state.pendingNotes.length) return state
      return { ...state, feed: [...state.feed, ...state.pendingNotes], pendingNotes: [] }
    }
    case 'SET_FEED_READY':
      return { ...state, feedReady: true }
    case 'SET_FEED_LOADING':
      return { ...state, feedLoading: true, feedLenBefore: state.feed.length }
    case 'FEED_LOADED_MORE': {
      const added = state.feed.length - state.feedLenBefore
      return { ...state, feedLoading: false, feedPage: state.feedPage + 1, feedExhausted: added === 0 }
    }
    case 'OPEN_THREAD': {
      const ev = action.event
      const eTags = (ev?.tags || []).filter((t) => t[0] === 'e')
      const rootTag = eTags.find((t) => t[3] === 'root') || (eTags.length > 0 ? eTags[0] : null)
      const rootId = rootTag ? rootTag[1] : action.eventId
      const hints = (ev?.tags || [])
        .filter((t) => t[0] === 'e' && t[2] && t[2].startsWith('wss://'))
        .map((t) => t[2])
      return { ...state, activeTab: 'thread', threadEventId: action.eventId, threadRootId: rootId, threadRelayHints: hints, threadEvents: ev ? [ev] : [], threadQueriedIds: [] }
    }
    case 'ADD_THREAD_EVENT': {
      if (state.threadEvents.some((e) => e.id === action.event.id)) return state
      return { ...state, threadEvents: [...state.threadEvents, action.event] }
    }
    case 'MARK_THREAD_QUERIED':
      return { ...state, threadQueriedIds: [...state.threadQueriedIds, ...action.ids] }
    case 'SET_ORLY_RELAYS':
      return { ...state, orlyRelays: action.relays }
    case 'SET_SNACKBAR':
      return { ...state, snackbar: action.message }
    case 'CACHE_EMBEDDED': {
      const embeddedNotes = new Map(state.embeddedNotes)
      embeddedNotes.set(action.eventId, action.event)
      return { ...state, embeddedNotes }
    }
    case 'OPEN_LIGHTBOX':
      return { ...state, lightboxUrl: action.url }
    case 'CLOSE_LIGHTBOX':
      return { ...state, lightboxUrl: null }
    case 'SET_DM_TAB':
      return { ...state, dmTab: action.tab }
    case 'OPEN_DM':
      return { ...state, activeTab: 'dms', dmTab: 'chat', activeDM: action.peer, dmMessages: [] }
    case 'SET_CONVERSATIONS':
      return { ...state, conversations: action.conversations }
    case 'SET_DM_MESSAGES':
      return { ...state, dmMessages: action.messages }
    case 'ADD_DM_MESSAGE': {
      const msg = action.message
      if (msg.peer !== state.activeDM) return state
      if (state.dmMessages.some((m) => m.id === msg.id)) return state
      return { ...state, dmMessages: [...state.dmMessages, msg] }
    }
    case 'ADD_CONVERSATION': {
      const c = action.conversation
      const existing = (state.conversations || []).filter((x) => x.peer !== c.peer)
      return { ...state, conversations: [c, ...existing].sort((a, b) => b.lastTs - a.lastTs) }
    }
    case 'SET_HASHTAG_QUERY':
      return { ...state, hashtagQuery: action.query, hashtagFeed: [], hashtagPage: 0, hashtagExhausted: false, hashtagLoading: false, hashtagLenBefore: 0 }
    case 'ADD_HASHTAG_EVENT': {
      if (state.hashtagFeed.some((e) => e.id === action.event.id)) return state
      return { ...state, hashtagFeed: [...state.hashtagFeed, action.event] }
    }
    case 'SET_HASHTAG_LOADING':
      return { ...state, hashtagLoading: true, hashtagLenBefore: state.hashtagFeed.length }
    case 'HASHTAG_LOADED_MORE': {
      const added = state.hashtagFeed.length - state.hashtagLenBefore
      return { ...state, hashtagLoading: false, hashtagPage: state.hashtagPage + 1, hashtagExhausted: added === 0 }
    }
    case 'ADD_RELAY_EVENT': {
      if (state.relayFeed.some((e) => e.id === action.event.id)) return state
      return { ...state, relayFeed: [...state.relayFeed, action.event] }
    }
    case 'SET_RELAY_LOADING':
      return { ...state, relayFeedLoading: true, relayFeedLenBefore: state.relayFeed.length }
    case 'RELAY_LOADED_MORE': {
      const added = state.relayFeed.length - state.relayFeedLenBefore
      return { ...state, relayFeedLoading: false, relayFeedPage: state.relayFeedPage + 1, relayFeedExhausted: added === 0 }
    }
    default:
      return state
  }
}
