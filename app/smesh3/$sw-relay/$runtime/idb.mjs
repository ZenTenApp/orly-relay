// TinyJS Runtime — IndexedDB store for events and DMs
// Port of smesh2/db.js with same schema and query logic.

const DB_NAME = 'sm3sh';
const DB_VERSION = 3;

let _db = null;
let _dbPromise = null;

function openDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = (e) => {
      const db = e.target.result;
      if (!db.objectStoreNames.contains('profiles')) db.createObjectStore('profiles');
      if (!db.objectStoreNames.contains('relays')) db.createObjectStore('relays');
      if (!db.objectStoreNames.contains('events')) {
        const store = db.createObjectStore('events', { keyPath: 'id' });
        store.createIndex('pubkey', 'pubkey', { unique: false });
        store.createIndex('kind', 'kind', { unique: false });
        store.createIndex('pubkey_kind', ['pubkey', 'kind'], { unique: false });
        store.createIndex('created_at', 'created_at', { unique: false });
      }
      if (!db.objectStoreNames.contains('dms')) {
        const dms = db.createObjectStore('dms', { keyPath: 'id' });
        dms.createIndex('peer', 'peer', { unique: false });
        dms.createIndex('peer_ts', ['peer', 'created_at'], { unique: false });
      }
    };
    req.onblocked = () => { console.warn('[sm3sh-sw] IDB upgrade blocked'); };
    req.onsuccess = () => {
      const db = req.result;
      db._closed = false;
      db.onclose = () => { db._closed = true; };
      db.onversionchange = () => { db.close(); db._closed = true; _db = null; };
      resolve(db);
    };
    req.onerror = () => reject(req.error);
  });
}

async function getDB() {
  if (_db && !_db._closed) return _db;
  if (!_dbPromise) _dbPromise = openDB();
  _db = await _dbPromise;
  _dbPromise = null;
  return _db;
}

// --- Exports for Go jsbridge ---

export function Open(fn) {
  getDB().then(() => fn());
}

export function SaveEvent(eventJSON, fn) {
  const event = JSON.parse(eventJSON);
  getDB().then(db => {
    const tx = db.transaction('events', 'readwrite');
    const store = tx.objectStore('events');
    const req = store.put(event);
    req.onsuccess = () => fn(true);
    req.onerror = () => {
      if (req.error?.name === 'ConstraintError') fn(false);
      else { console.warn('SaveEvent error:', req.error); fn(false); }
    };
  });
}

export function QueryEvents(filterJSON, fn) {
  const filter = JSON.parse(filterJSON);
  getDB().then(db => {
    const tx = db.transaction('events', 'readonly');
    const store = tx.objectStore('events');
    const results = [];

    let source;
    if (filter.authors?.length === 1 && filter.kinds?.length === 1) {
      const idx = store.index('pubkey_kind');
      source = idx.openCursor(IDBKeyRange.only([filter.authors[0], filter.kinds[0]]), 'prev');
    } else if (filter.authors?.length === 1) {
      source = store.index('pubkey').openCursor(IDBKeyRange.only(filter.authors[0]), 'prev');
    } else if (filter.kinds?.length === 1) {
      source = store.index('kind').openCursor(IDBKeyRange.only(filter.kinds[0]), 'prev');
    } else {
      source = store.index('created_at').openCursor(null, 'prev');
    }

    source.onsuccess = (e) => {
      const cursor = e.target.result;
      if (!cursor) { fn(JSON.stringify(results)); return; }

      const ev = cursor.value;
      let match = true;

      if (filter.ids && !filter.ids.includes(ev.id)) match = false;
      if (filter.authors?.length > 1 && !filter.authors.includes(ev.pubkey)) match = false;
      if (filter.kinds?.length > 1 && !filter.kinds.includes(ev.kind)) match = false;
      if (filter.since && ev.created_at < filter.since) match = false;
      if (filter.until && ev.created_at > filter.until) match = false;

      if (match) {
        for (const [k, v] of Object.entries(filter)) {
          if (k.startsWith('#') && k.length === 2) {
            const tagName = k[1];
            if (!ev.tags?.some(t => t[0] === tagName && v.includes(t[1]))) { match = false; break; }
          }
        }
      }

      if (match) results.push(ev);
      if (filter.limit && results.length >= filter.limit) { fn(JSON.stringify(results)); return; }
      cursor.continue();
    };
    source.onerror = () => { console.warn('QueryEvents error:', source.error); fn('[]'); };
  });
}

export function SaveDM(dmJSON, fn) {
  const dm = JSON.parse(dmJSON);
  getDB().then(db => {
    const tx = db.transaction('dms', 'readwrite');
    const store = tx.objectStore('dms');
    const getReq = store.get(dm.id);
    getReq.onsuccess = () => {
      const existing = getReq.result;
      if (existing) {
        if (dm.protocol === 'nip17' && existing.protocol === 'nip04') {
          store.put(dm);
          fn('upgraded');
        } else {
          fn('duplicate');
        }
      } else {
        store.put(dm);
        fn('saved');
      }
    };
    getReq.onerror = () => { console.warn('SaveDM error:', getReq.error); fn('error'); };
  });
}

export function QueryDMs(peer, limit, until, fn) {
  getDB().then(db => {
    const tx = db.transaction('dms', 'readonly');
    const store = tx.objectStore('dms');
    const idx = store.index('peer_ts');
    const results = [];

    const upper = until > 0 ? [peer, until] : [peer, Date.now() / 1000 + 86400];
    const range = IDBKeyRange.bound([peer, 0], upper);
    const req = idx.openCursor(range, 'prev');

    req.onsuccess = (e) => {
      const cursor = e.target.result;
      if (!cursor || results.length >= limit) { fn(JSON.stringify(results)); return; }
      results.push(cursor.value);
      cursor.continue();
    };
    req.onerror = () => { console.warn('QueryDMs error:', req.error); fn('[]'); };
  });
}

export function GetConversationList(fn) {
  getDB().then(db => {
    const tx = db.transaction('dms', 'readonly');
    const store = tx.objectStore('dms');
    const idx = store.index('peer');
    const conversations = new Map();

    const req = idx.openCursor(null, 'prev');
    req.onsuccess = (e) => {
      const cursor = e.target.result;
      if (!cursor) {
        const list = [...conversations.values()].sort((a, b) => b.lastTs - a.lastTs);
        fn(JSON.stringify(list));
        return;
      }
      const dm = cursor.value;
      if (!conversations.has(dm.peer)) {
        conversations.set(dm.peer, {
          peer: dm.peer,
          lastMessage: dm.content.slice(0, 80),
          lastTs: dm.created_at,
          from: dm.from,
        });
      } else {
        const existing = conversations.get(dm.peer);
        if (dm.created_at > existing.lastTs) {
          existing.lastMessage = dm.content.slice(0, 80);
          existing.lastTs = dm.created_at;
          existing.from = dm.from;
        }
      }
      cursor.continue();
    };
    req.onerror = () => { console.warn('GetConversationList error:', req.error); fn('[]'); };
  });
}
