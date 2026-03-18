// smesh2 service worker — local nostr relay
// speaks NIP-01 over postMessage to the UI thread

const CACHE_NAME = 'smesh2-v11'
const CACHE_URLS = ['./', './index.html']

// ─── lifecycle ───────────────────────────────────────────────────────

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(CACHE_URLS))
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
  // only handle same-origin requests
  if (url.origin !== self.location.origin) return
  e.respondWith(
    caches.match(e.request).then((cached) => cached || fetch(e.request))
  )
})

// ─── IndexedDB ───────────────────────────────────────────────────────

const DB_NAME = 'smesh2'
const DB_VERSION = 1

function openDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = (e) => {
      const db = e.target.result
      if (!db.objectStoreNames.contains('events')) {
        const store = db.createObjectStore('events', { keyPath: 'id' })
        store.createIndex('pubkey', 'pubkey', { unique: false })
        store.createIndex('kind', 'kind', { unique: false })
        store.createIndex('pubkey_kind', ['pubkey', 'kind'], { unique: false })
        store.createIndex('created_at', 'created_at', { unique: false })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

let dbPromise = openDB()

async function getDB() {
  return dbPromise
}

async function saveEvent(event) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('events', 'readwrite')
    const store = tx.objectStore('events')
    const req = store.put(event)
    req.onsuccess = () => resolve(true)
    req.onerror = () => {
      if (req.error?.name === 'ConstraintError') resolve(false) // duplicate
      else reject(req.error)
    }
  })
}

async function queryEvents(filter) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('events', 'readonly')
    const store = tx.objectStore('events')
    const results = []

    // pick best index
    let source
    if (filter.authors?.length === 1 && filter.kinds?.length === 1) {
      const idx = store.index('pubkey_kind')
      const key = IDBKeyRange.only([filter.authors[0], filter.kinds[0]])
      source = idx.openCursor(key, 'prev')
    } else if (filter.authors?.length === 1) {
      source = store.index('pubkey').openCursor(
        IDBKeyRange.only(filter.authors[0]), 'prev'
      )
    } else if (filter.kinds?.length === 1) {
      source = store.index('kind').openCursor(
        IDBKeyRange.only(filter.kinds[0]), 'prev'
      )
    } else {
      source = store.index('created_at').openCursor(null, 'prev')
    }

    source.onsuccess = (e) => {
      const cursor = e.target.result
      if (!cursor) { resolve(results); return }

      const ev = cursor.value
      let match = true

      if (filter.ids && !filter.ids.includes(ev.id)) match = false
      if (filter.authors?.length > 1 && !filter.authors.includes(ev.pubkey)) match = false
      if (filter.kinds?.length > 1 && !filter.kinds.includes(ev.kind)) match = false
      if (filter.since && ev.created_at < filter.since) match = false
      if (filter.until && ev.created_at > filter.until) match = false

      // tag filters (#e, #p, etc) — scan
      if (match) {
        for (const [k, v] of Object.entries(filter)) {
          if (k.startsWith('#') && k.length === 2) {
            const tagName = k[1]
            const hasTag = ev.tags?.some(
              (t) => t[0] === tagName && v.includes(t[1])
            )
            if (!hasTag) { match = false; break }
          }
        }
      }

      if (match) results.push(ev)
      if (filter.limit && results.length >= filter.limit) {
        resolve(results)
        return
      }
      cursor.continue()
    }
    source.onerror = () => reject(source.error)
  })
}

// ─── subscriptions ───────────────────────────────────────────────────

// sub_id -> { filter, clientId }
const subs = new Map()

async function handleReq(clientId, subId, filter) {
  subs.set(subId, { filter, clientId })
  // query local store
  const events = await queryEvents(filter)
  const client = await self.clients.get(clientId)
  if (!client) return
  for (const ev of events) {
    client.postMessage(['EVENT', subId, ev])
  }
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

// ─── WebSocket pool ──────────────────────────────────────────────────

const MAX_CONNECTIONS = 4
const pool = new Map() // url -> { ws, lastUsed, subCount }

function getConnection(url) {
  if (pool.has(url)) {
    const conn = pool.get(url)
    conn.lastUsed = Date.now()
    return conn.ws
  }

  // evict LRU if at capacity
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
  const conn = { ws, lastUsed: Date.now(), subCount: 0, pending: new Map() }

  ws.onopen = () => {
    // flush any queued messages
    if (conn.queue) {
      for (const msg of conn.queue) ws.send(msg)
      conn.queue = null
    }
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

function sendToRelay(url, msg) {
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

// proxy sub_id -> { relays, timeout, internalSubId }
const proxySubs = new Map()

async function handleRelayMessage(relayUrl, msg) {
  const [type, ...args] = msg

  if (type === 'EVENT') {
    const [subId, event] = args
    const saved = await saveEvent(event)
    if (saved) await pushToMatchingSubs(event)
  }

  if (type === 'EOSE') {
    const [subId] = args
    // check if this is a proxy sub that should close after EOSE
    for (const [proxyId, info] of proxySubs) {
      if (info.remoteSubIds?.has(subId)) {
        info.eoseCount = (info.eoseCount || 0) + 1
        if (info.eoseCount >= info.relayCount) {
          // all relays have sent EOSE, clean up
          cleanupProxy(proxyId)
        }
      }
    }
  }

  if (type === 'OK') {
    const [eventId, success, message] = args
    // forward OK to all clients
    const clients = await self.clients.matchAll()
    for (const client of clients) {
      client.postMessage(['OK', eventId, success, message || ''])
    }
  }

  if (type === 'NOTICE') {
    const [message] = args
    const clients = await self.clients.matchAll()
    for (const client of clients) {
      client.postMessage(['NOTICE', `[${relayUrl}] ${message}`])
    }
  }
}

async function handleProxy(clientId, subId, filter, ...relayUrls) {
  // clean up any existing proxy with this sub ID
  if (proxySubs.has(subId)) cleanupProxy(subId)

  // register a local subscription so incoming events get pushed to the UI
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

function cleanupProxy(proxyId) {
  const info = proxySubs.get(proxyId)
  if (!info) return
  clearTimeout(info.timeout)
  // send CLOSE for all remote subs
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

async function handleEvent(clientId, event) {
  // store locally
  const saved = await saveEvent(event)
  if (saved) await pushToMatchingSubs(event)

  // publish to write relays — for now just all connected relays
  for (const [url, conn] of pool) {
    if (conn.ws.readyState === WebSocket.OPEN) {
      conn.ws.send(JSON.stringify(['EVENT', event]))
    }
  }

  // send OK back to client
  const client = await self.clients.get(clientId)
  if (client) {
    client.postMessage(['OK', event.id, true, ''])
  }
}

// ─── relay info (NIP-11) ─────────────────────────────────────────────

const relayInfoCache = new Map() // url -> { info, ts }
const RELAY_INFO_TTL = 3600000   // 1 hour

async function handleRelayInfo(clientId, relayUrl) {
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

// ─── message dispatch ────────────────────────────────────────────────

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
  }
})
