// db.js — IndexedDB operations for events and DMs

import { sha256, bytesToHex } from './crypto.js'

const DB_NAME = 'smesh2'
const DB_VERSION = 2

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
      if (!db.objectStoreNames.contains('dms')) {
        const dms = db.createObjectStore('dms', { keyPath: 'id' })
        dms.createIndex('peer', 'peer', { unique: false })
        dms.createIndex('peer_ts', ['peer', 'created_at'], { unique: false })
      }
    }
    req.onsuccess = () => resolve(patchDBClose(req.result))
    req.onerror = () => reject(req.error)
  })
}

let dbPromise = openDB()

export async function getDB() {
  const db = await dbPromise
  if (db._closed) {
    dbPromise = openDB()
    return dbPromise
  }
  return db
}

function patchDBClose(db) {
  db._closed = false
  db.onclose = () => { db._closed = true }
  db.addEventListener('close', () => { db._closed = true })
  return db
}

// ─── events ─────────────────────────────────────────────────────────

export async function saveEvent(event) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('events', 'readwrite')
    const store = tx.objectStore('events')
    const req = store.put(event)
    req.onsuccess = () => resolve(true)
    req.onerror = () => {
      if (req.error?.name === 'ConstraintError') resolve(false)
      else reject(req.error)
    }
  })
}

export async function queryEvents(filter) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('events', 'readonly')
    const store = tx.objectStore('events')
    const results = []

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

// ─── DMs ────────────────────────────────────────────────────────────

export function dmDedupId(peerPubkey, content, createdAt) {
  const contentHash = bytesToHex(sha256(new TextEncoder().encode(content)))
  const timeWindow = Math.floor(createdAt / 300)
  return bytesToHex(sha256(new TextEncoder().encode(peerPubkey + contentHash + timeWindow)))
}

export async function saveDM(dm) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('dms', 'readwrite')
    const store = tx.objectStore('dms')
    const getReq = store.get(dm.id)
    getReq.onsuccess = () => {
      const existing = getReq.result
      if (existing) {
        if (dm.protocol === 'nip17' && existing.protocol === 'nip04') {
          store.put(dm)
          resolve('upgraded')
        } else {
          resolve('duplicate')
        }
      } else {
        store.put(dm)
        resolve('saved')
      }
    }
    getReq.onerror = () => reject(getReq.error)
  })
}

export async function queryDMs(peer, limit = 50, until = null) {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('dms', 'readonly')
    const store = tx.objectStore('dms')
    const idx = store.index('peer_ts')
    const results = []

    const upper = until ? [peer, until] : [peer, Date.now() / 1000 + 86400]
    const lower = [peer, 0]
    const range = IDBKeyRange.bound(lower, upper)
    const req = idx.openCursor(range, 'prev')

    req.onsuccess = (e) => {
      const cursor = e.target.result
      if (!cursor || results.length >= limit) { resolve(results); return }
      results.push(cursor.value)
      cursor.continue()
    }
    req.onerror = () => reject(req.error)
  })
}

export async function getConversationList() {
  const db = await getDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('dms', 'readonly')
    const store = tx.objectStore('dms')
    const idx = store.index('peer')
    const conversations = new Map()

    const req = idx.openCursor(null, 'prev')
    req.onsuccess = (e) => {
      const cursor = e.target.result
      if (!cursor) {
        resolve([...conversations.values()].sort((a, b) => b.lastTs - a.lastTs))
        return
      }
      const dm = cursor.value
      if (!conversations.has(dm.peer)) {
        conversations.set(dm.peer, {
          peer: dm.peer,
          lastMessage: dm.content.slice(0, 80),
          lastTs: dm.created_at,
          from: dm.from,
        })
      } else {
        const existing = conversations.get(dm.peer)
        if (dm.created_at > existing.lastTs) {
          existing.lastMessage = dm.content.slice(0, 80)
          existing.lastTs = dm.created_at
          existing.from = dm.from
        }
      }
      cursor.continue()
    }
    req.onerror = () => reject(req.error)
  })
}
