// pool.js — WebSocket connection pool, relay messaging, AUTH, reconnect

import { signEvent, schnorr, sha256, bytesToHex } from './crypto.js'
import { saveEvent } from './db.js'

const MAX_CONNECTIONS = 16

// url -> { ws, lastUsed, queue, subCount }
export const pool = new Map()

// module-level state set by sw.js
let _writeRelays = []
let _secretKey = null
let _secretKeyHex = null
let _myPubkey = null

export function setPoolState({ writeRelays, secretKey, secretKeyHex, myPubkey }) {
  if (writeRelays !== undefined) _writeRelays = writeRelays
  if (secretKey !== undefined) _secretKey = secretKey
  if (secretKeyHex !== undefined) _secretKeyHex = secretKeyHex
  if (myPubkey !== undefined) _myPubkey = myPubkey
}

// ─── reconnect handlers ─────────────────────────────────────────────

const reconnectHandlers = []

export function onReconnect(fn) {
  reconnectHandlers.push(fn)
}

function fireReconnect(url) {
  for (const fn of reconnectHandlers) {
    try { fn(url) } catch (e) { console.warn('reconnect handler error:', e) }
  }
}

// ─── event/message callbacks (set by sw.js) ─────────────────────────

let _onEvent = null
let _onDMEvent = null
let _broadcastToClients = null

export function setCallbacks({ onEvent, onDMEvent, broadcastToClients }) {
  if (onEvent) _onEvent = onEvent
  if (onDMEvent) _onDMEvent = onDMEvent
  if (broadcastToClients) _broadcastToClients = broadcastToClients
}

function broadcast(msg) {
  if (_broadcastToClients) _broadcastToClients(msg)
}

// ─── connection pool ────────────────────────────────────────────────

function getConnection(url) {
  if (pool.has(url)) {
    const conn = pool.get(url)
    conn.lastUsed = Date.now()
    return conn.ws
  }

  if (pool.size >= MAX_CONNECTIONS) {
    let oldest = null
    let oldestTime = Infinity
    for (const [u, c] of pool) {
      if (c.lastUsed < oldestTime) { oldest = u; oldestTime = c.lastUsed }
    }
    if (oldest) {
      pool.get(oldest).ws.close()
      pool.delete(oldest)
    }
  }

  const ws = new WebSocket(url)
  const conn = { ws, lastUsed: Date.now(), queue: null }

  ws.onopen = () => {
    if (conn.queue) {
      for (const msg of conn.queue) ws.send(msg)
      conn.queue = null
    }
    fireReconnect(url)
  }

  ws.onmessage = (e) => {
    let msg
    try { msg = JSON.parse(e.data) } catch { return }
    handleRelayMessage(url, msg)
  }

  ws.onclose = () => pool.delete(url)
  ws.onerror = () => pool.delete(url)

  pool.set(url, conn)
  return ws
}

// ─── filter sanitizer ───────────────────────────────────────────────

const HEX_FILTER_KEYS = new Set(['ids', 'authors', '#e', '#p', '#a', '#d'])

function sanitizeFilter(msg) {
  if (msg[0] !== 'REQ') return msg
  const out = [msg[0], msg[1]]
  for (let i = 2; i < msg.length; i++) {
    const filter = msg[i]
    if (!filter || typeof filter !== 'object') { out.push(filter); continue }
    const clean = {}
    let hasRequiredField = false
    for (const [k, v] of Object.entries(filter)) {
      if (HEX_FILTER_KEYS.has(k) && Array.isArray(v)) {
        const valid = v.filter((s) => {
          if (typeof s !== 'string') return false
          if (s.length % 2 !== 0) { console.warn('dropped odd-len hex', k, s.length, s.slice(0, 16)); return false }
          if (!/^[0-9a-f]+$/i.test(s)) { console.warn('dropped non-hex', k, s.slice(0, 16)); return false }
          return true
        })
        if (valid.length) {
          clean[k] = valid
          hasRequiredField = true
        }
        // FIX #7: if ALL hex values were invalid, skip this key entirely
        // but track that we had a required field that's now empty
      } else {
        clean[k] = v
        if (k === 'kinds' || k === 'since' || k === 'until' || k === 'limit') {
          // these are constraints, not required fields
        } else if (!k.startsWith('_')) {
          hasRequiredField = true
        }
      }
    }
    // only include filter if it has at least one field that can match something
    if (hasRequiredField || Object.keys(clean).some(k => k === 'kinds')) {
      out.push(clean)
    } else {
      console.warn('dropped empty filter after sanitization')
    }
  }
  // if no filters remain after sanitization, don't send the REQ
  if (out.length <= 2) return null
  return out
}

// ─── send to relay ──────────────────────────────────────────────────

export function sendToRelay(url, msg) {
  msg = sanitizeFilter(msg)
  if (!msg) return // sanitizer dropped all filters
  const ws = getConnection(url)
  const data = JSON.stringify(msg)
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(data)
  } else {
    const conn = pool.get(url)
    if (conn) {
      conn.queue = conn.queue || []
      conn.queue.push(data)
    }
  }
}

// ─── proxy subscriptions ────────────────────────────────────────────

// proxyId -> { remoteSubIds, relayCount, eoseCount, timeout }
export const proxySubs = new Map()
// sub_id -> { filter, clientId }
export const subs = new Map()

export async function handleProxy(clientId, subId, filter, ...relayUrls) {
  if (proxySubs.has(subId)) cleanupProxy(subId)

  subs.set(subId, { filter, clientId })

  const remoteSubIds = new Set()
  const remoteSubId = 'p_' + subId + '_' + Math.random().toString(36).slice(2, 6)

  proxySubs.set(subId, {
    remoteSubIds,
    relayCount: relayUrls.length,
    eoseCount: 0,
    timeout: setTimeout(() => cleanupProxy(subId), 10000),
  })

  for (const url of relayUrls) {
    const rSubId = remoteSubId + '_' + url.replace(/\W/g, '').slice(-8)
    remoteSubIds.add(rSubId)
    sendToRelay(url, ['REQ', rSubId, filter])
  }
}

export async function cleanupProxy(proxyId) {
  const info = proxySubs.get(proxyId)
  if (!info) return
  clearTimeout(info.timeout)
  const sub = subs.get(proxyId)
  if (sub) {
    const client = await self.clients.get(sub.clientId)
    if (client) client.postMessage(['EOSE', proxyId])
  }
  for (const rSubId of info.remoteSubIds) {
    for (const [url, conn] of pool) {
      if (conn.ws.readyState === WebSocket.OPEN) {
        conn.ws.send(JSON.stringify(['CLOSE', rSubId]))
      }
    }
  }
  proxySubs.delete(proxyId)
  subs.delete(proxyId)
}

// ─── relay message handler ──────────────────────────────────────────

async function handleRelayMessage(relayUrl, msg) {
  const [type, ...args] = msg

  if (type === 'EVENT') {
    const [subId, event] = args
    const saved = await saveEvent(event)
    if (saved && _onEvent) await _onEvent(event, subId)
    // propagate to write relays (skip source, skip DMs)
    // FIX #3: use writeRelays, not all pool connections
    if (saved && event.kind !== 4 && event.kind !== 1059) {
      for (const wr of _writeRelays) {
        if (wr !== relayUrl) sendToRelay(wr, ['EVENT', event])
      }
    }
    // process DMs regardless of saved (might already have event but need to decrypt)
    if ((event.kind === 4 || event.kind === 1059) && _onDMEvent) {
      _onDMEvent(event)
    }
  }

  if (type === 'EOSE') {
    const [subId] = args
    for (const [proxyId, info] of proxySubs) {
      if (info.remoteSubIds?.has(subId)) {
        info.eoseCount = (info.eoseCount || 0) + 1
        if (info.eoseCount >= info.relayCount) {
          cleanupProxy(proxyId)
        }
      }
    }
  }

  if (type === 'OK') {
    const [eventId, success, message] = args
    broadcast(['OK', eventId, success, message || ''])
  }

  if (type === 'NOTICE') {
    const [message] = args
    broadcast(['NOTICE', `[${relayUrl}] ${message}`])
  }

  // FIX #11: NIP-42 AUTH challenge
  if (type === 'AUTH') {
    const [challenge] = args
    if (_secretKey && _myPubkey) {
      const authEvent = signEvent({
        kind: 22242,
        content: '',
        tags: [
          ['relay', relayUrl],
          ['challenge', challenge],
        ],
        created_at: Math.floor(Date.now() / 1000),
        pubkey: _myPubkey,
      }, _secretKey)
      const conn = pool.get(relayUrl)
      if (conn?.ws?.readyState === WebSocket.OPEN) {
        conn.ws.send(JSON.stringify(['AUTH', authEvent]))
      }
    }
  }
}

// ─── publish event ──────────────────────────────────────────────────

// FIX #3: publish to writeRelays only, not all connected relays
export async function handleEvent(clientId, event) {
  const saved = await saveEvent(event)
  if (saved && _onEvent) await _onEvent(event, null)

  for (const url of _writeRelays) {
    sendToRelay(url, ['EVENT', event])
  }

  const client = await self.clients.get(clientId)
  if (client) client.postMessage(['OK', event.id, true, ''])
}

// ─── relay info (NIP-11) ────────────────────────────────────────────

const relayInfoCache = new Map()
const RELAY_INFO_TTL = 3600000

export async function handleRelayInfo(clientId, relayUrl) {
  const cached = relayInfoCache.get(relayUrl)
  if (cached && Date.now() - cached.ts < RELAY_INFO_TTL) {
    const client = await self.clients.get(clientId)
    if (client) client.postMessage(['RELAY_INFO', relayUrl, cached.info])
    return
  }

  try {
    const httpUrl = relayUrl.replace('wss://', 'https://').replace('ws://', 'http://')
    const resp = await fetch(httpUrl, {
      headers: { 'Accept': 'application/nostr+json' }
    })
    const info = await resp.json()
    relayInfoCache.set(relayUrl, { info, ts: Date.now() })
    const client = await self.clients.get(clientId)
    if (client) client.postMessage(['RELAY_INFO', relayUrl, info])
  } catch (err) {
    const client = await self.clients.get(clientId)
    if (client) client.postMessage(['RELAY_INFO', relayUrl, null])
  }
}
