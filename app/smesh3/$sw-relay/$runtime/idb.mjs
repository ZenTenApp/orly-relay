// TinyJS Runtime — IndexedDB store for events and DMs
// Port of smesh2/db.js with same schema and query logic.

const DB_NAME = 'sm3sh';
const DB_VERSION = 4;

let _db = null;
let _dbPromise = null;
let _expectedVersion = '';

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
      if (!db.objectStoreNames.contains('meta')) db.createObjectStore('meta');
    };
    req.onblocked = () => { console.warn('[sm3sh-sw] IDB upgrade blocked'); };
    req.onsuccess = () => {
      const db = req.result;
      db._closed = false;
      db.onclose = () => { db._closed = true; };
      db.onversionchange = () => { db.close(); db._closed = true; _db = null; };
      if (_expectedVersion) {
        _epochCheck(db).then(() => resolve(db)).catch(() => resolve(db));
      } else {
        resolve(db);
      }
    };
    req.onerror = () => reject(req.error);
  });
}

function _epochCheck(db) {
  return new Promise((resolve) => {
    const tx = db.transaction('meta', 'readonly');
    const store = tx.objectStore('meta');
    const req = store.get('_version');
    req.onsuccess = () => {
      const stored = req.result;
      if (stored === _expectedVersion) { resolve(); return; }
      console.warn('[sm3sh-sw] version epoch mismatch: stored=' + (stored || 'none') + ' running=' + _expectedVersion + ' — flushing IDB');
      _flushAndStamp(db).then(resolve);
    };
    req.onerror = () => { _flushAndStamp(db).then(resolve); };
  });
}

function _flushAndStamp(db) {
  return new Promise((resolve) => {
    const names = ['profiles', 'relays', 'events', 'dms'];
    const tx = db.transaction([...names, 'meta'], 'readwrite');
    for (const name of names) tx.objectStore(name).clear();
    tx.objectStore('meta').put(_expectedVersion, '_version');
    tx.oncomplete = () => { resolve(); };
    tx.onerror = () => { resolve(); };
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
  let event;
  try { event = JSON.parse(eventJSON); } catch (e) {
    console.error('SaveEvent JSON parse error:', e.message);
    fn(false); return;
  }
  getDB().then(db => {
    try {
      const tx = db.transaction('events', 'readwrite');
      const store = tx.objectStore('events');
      const req = store.put(event);
      req.onsuccess = () => { try { fn(true); } catch(e) { _busErr('SaveEvent cb', e); } };
      req.onerror = () => {
        if (req.error?.name === 'ConstraintError') fn(false);
        else { console.warn('SaveEvent error:', req.error); fn(false); }
      };
    } catch (e) {
      console.error('SaveEvent tx error:', e.message);
      _busErr('SaveEvent tx', e);
      fn(false);
    }
  }).catch(e => { console.error('SaveEvent getDB error:', e.message); _busErr('SaveEvent getDB', e); fn(false); });
}

function _busErr(ctx, e) {
  if (self._busPort) {
    self._busPort.postMessage('{"from":"relay","to":"shell","msg":["LOG","relay","IDB CRASH ' + ctx + ': ' + String(e.message).replace(/"/g, '\\"').replace(/\n/g, ' ') + '"]}');
  }
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
    const idx = store.index('peer_ts');
    const list = [];
    const seen = new Set();

    // Reverse cursor on [peer, created_at] — for each peer, the last
    // entry in sort order is the newest message. After grabbing it,
    // advance past the rest of that peer's messages.
    const req = idx.openCursor(null, 'prev');
    req.onsuccess = (e) => {
      const cursor = e.target.result;
      if (!cursor) {
        list.sort((a, b) => b.lastTs - a.lastTs);
        fn(JSON.stringify(list));
        return;
      }
      const dm = cursor.value;
      if (!seen.has(dm.peer)) {
        seen.add(dm.peer);
        list.push({
          peer: dm.peer,
          lastMessage: dm.content.slice(0, 80),
          lastTs: dm.created_at,
          from: dm.from,
        });
        // Skip to the previous peer — advance cursor past all entries
        // for this peer by setting upper bound to [peer, 0].
        cursor.continue([dm.peer, 0]);
      } else {
        // Still in same peer range (shouldn't happen with continue skip,
        // but handle gracefully).
        cursor.continue();
      }
    };
    req.onerror = () => { console.warn('GetConversationList error:', req.error); fn('[]'); };
  });
}

export function SetVersion(v) {
  _expectedVersion = v;
}
