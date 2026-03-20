// smesh2 service worker — thin orchestrator
// delegates to crypto.js, db.js, pool.js, dm.js

import { signEvent, schnorr, bytesToHex } from './crypto.js'
import { saveEvent, queryEvents, getConversationList, queryDMs } from './db.js'
import {
  setPoolState, setCallbacks, pool, subs,
  sendToRelay, handleProxy, handleEvent, handleRelayInfo,
} from './pool.js'
import {
  setDMState, processIncomingDM, sendDM, handleDMSub,
  handleExtDMResult, handleCryptoResult,
} from './dm.js'

const CACHE_NAME = 'smesh2-v52'
const CACHE_URLS = ['./', './favicon.ico', './favicon.png', './favicon-96x96.png', './apple-touch-icon.png']

// ─── module state ───────────────────────────────────────────────────

let secretKey = null
let secretKeyHex = null
let myPubkey = null
let writeRelays = []

function syncState() {
  setPoolState({ writeRelays, secretKey, secretKeyHex, myPubkey })
  setDMState({ secretKey, secretKeyHex, myPubkey })
}

// ─── lifecycle ──────────────────────────────────────────────────────

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(CACHE_URLS))
      .catch((err) => console.warn('cache addAll failed, continuing:', err))
  )
  self.skipWaiting()
})

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((names) =>
      Promise.all(
        names
          .filter((n) => n !== CACHE_NAME)
          .map((n) => caches.delete(n))
      )
    )
  )
  self.clients.claim()
})

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url)
  if (url.origin !== self.location.origin) return
  if (e.request.mode === 'navigate') {
    e.respondWith(
      fetch(e.request).catch(() => caches.match(e.request))
    )
    return
  }
  e.respondWith(
    caches.match(e.request).then((cached) => cached || fetch(e.request))
  )
})

// ─── subscriptions ──────────────────────────────────────────────────

async function handleReq(clientId, subId, filter) {
  subs.set(subId, { filter, clientId })
  const events = await queryEvents(filter)
  const client = await self.clients.get(clientId)
  if (!client) return
  for (const ev of events) client.postMessage(['EVENT', subId, ev])
  client.postMessage(['EOSE', subId])
}

function handleClose(subId) {
  subs.delete(subId)
}

async function pushToMatchingSubs(event) {
  for (const [subId, { filter, clientId }] of subs) {
    if (matchesFilter(event, filter)) {
      const client = await self.clients.get(clientId)
      if (client) client.postMessage(['EVENT', subId, event])
    }
  }
}

function matchesFilter(ev, f) {
  if (f.ids && !f.ids.includes(ev.id)) return false
  if (f.authors && !f.authors.includes(ev.pubkey)) return false
  if (f.kinds && !f.kinds.includes(ev.kind)) return false
  if (f.since && ev.created_at < f.since) return false
  if (f.until && ev.created_at > f.until) return false
  for (const [k, v] of Object.entries(f)) {
    if (k.startsWith('#') && k.length === 2) {
      const tagName = k[1]
      if (!ev.tags?.some((t) => t[0] === tagName && v.includes(t[1]))) return false
    }
  }
  return true
}

// ─── identity broadcast ─────────────────────────────────────────────

async function broadcastIdentity(clientId, pubkey, relayUrls) {
  const events = await queryEvents({ authors: [pubkey], kinds: [0, 3, 10002, 10050, 10051] })
  const byKind = new Map()
  for (const ev of events) {
    const prev = byKind.get(ev.kind)
    if (!prev || ev.created_at > prev.created_at) byKind.set(ev.kind, ev)
  }

  const relayEvent = byKind.get(10002)
  const userRelays = relayEvent
    ? relayEvent.tags.filter((t) => t[0] === 'r').map((t) => t[1])
    : writeRelays.length ? writeRelays : []

  if (!byKind.has(10050) && secretKey && userRelays.length) {
    const ev = signEvent({
      pubkey,
      created_at: Math.floor(Date.now() / 1000),
      kind: 10050,
      tags: userRelays.map((r) => ['relay', r]),
      content: ''
    }, secretKey)
    await saveEvent(ev)
    byKind.set(10050, ev)
  }

  if (!byKind.has(10051) && secretKey && userRelays.length) {
    const ev = signEvent({
      pubkey,
      created_at: Math.floor(Date.now() / 1000),
      kind: 10051,
      tags: userRelays.map((r) => ['relay', r]),
      content: ''
    }, secretKey)
    await saveEvent(ev)
    byKind.set(10051, ev)
  }

  const toSend = [...byKind.values()]
  if (!toSend.length) {
    const client = await self.clients.get(clientId)
    if (client) client.postMessage(['BROADCAST_DONE', 0, 0])
    return
  }

  for (const ev of toSend) {
    for (const url of relayUrls) sendToRelay(url, ['EVENT', ev])
  }
  const client = await self.clients.get(clientId)
  if (client) client.postMessage(['BROADCAST_DONE', toSend.length, relayUrls.length])
}

// ─── wire up pool callbacks ─────────────────────────────────────────

async function broadcastToClients(msg) {
  const clients = await self.clients.matchAll()
  for (const client of clients) client.postMessage(msg)
}

setCallbacks({
  onEvent: pushToMatchingSubs,
  onDMEvent: processIncomingDM,
  broadcastToClients,
})

// ─── message dispatch ───────────────────────────────────────────────

self.addEventListener('message', (e) => {
  const [type, ...args] = e.data
  const clientId = e.source?.id

  switch (type) {
    case 'REQ':        handleReq(clientId, args[0], args[1]); break
    case 'CLOSE':      handleClose(args[0]); break
    case 'EVENT':      handleEvent(clientId, args[0]); break
    case 'PROXY':      handleProxy(clientId, args[0], args[1], ...args.slice(2)); break
    case 'RELAY_INFO': handleRelayInfo(clientId, args[0]); break
    case 'SKIP_WAITING': self.skipWaiting(); break

    case 'SET_KEY': {
      secretKey = new Uint8Array(args[0])
      secretKeyHex = Array.from(secretKey, (b) => b.toString(16).padStart(2, '0')).join('')
      myPubkey = bytesToHex(schnorr.getPublicKey(secretKey))
      syncState()
      self.clients.get(clientId).then((c) => c?.postMessage(['KEY_SET']))
      break
    }
    case 'SET_PUBKEY': {
      myPubkey = args[0]
      syncState()
      break
    }
    case 'SIGN': {
      const [requestId, unsignedEvent] = args
      if (!secretKey) break
      try {
        const signed = signEvent(unsignedEvent, secretKey)
        self.clients.get(clientId).then((c) => {
          if (c) c.postMessage(['SIGNED', requestId, signed])
        })
      } catch (err) {
        self.clients.get(clientId).then((c) => {
          if (c) c.postMessage(['SIGN_ERROR', requestId, err.message])
        })
      }
      break
    }
    case 'CLEAR_KEY': {
      secretKey = null
      secretKeyHex = null
      myPubkey = null
      writeRelays = []
      syncState()
      break
    }
    case 'SET_WRITE_RELAYS': {
      writeRelays = args[0] || []
      syncState()
      break
    }
    case 'BROADCAST': {
      broadcastIdentity(clientId, args[0], args[1])
      break
    }
    case 'SEND_DM': {
      sendDM(clientId, args[0], args[1], args[2])
      break
    }
    case 'DM_EXT_RESULT': {
      const [peer, content, success, errMsg] = args
      handleExtDMResult(clientId, peer, content, success, errMsg)
      break
    }
    case 'DM_SUB': {
      handleDMSub(clientId, args[0])
      break
    }
    case 'DM_LIST': {
      getConversationList().then((list) => {
        self.clients.get(clientId).then((c) => {
          if (c) c.postMessage(['DM_LIST', list])
        })
      })
      break
    }
    case 'DM_HISTORY': {
      const [peer, limit, until] = args
      queryDMs(peer, limit || 50, until).then((messages) => {
        self.clients.get(clientId).then((c) => {
          if (c) c.postMessage(['DM_HISTORY', peer, messages])
        })
      })
      break
    }
    case 'CRYPTO_RESULT': {
      const [requestId, result, error] = args
      handleCryptoResult(requestId, result, error)
      break
    }
  }
})
