// TinyJS Runtime — DOM Bridge
// Provides Go-callable browser DOM operations.
// Elements are tracked by integer handles.
// Function names match Go signatures (PascalCase).

const _elements = new Map();
let _nextId = 1;
const _callbacks = new Map();
let _nextCb = 1;

// Bootstrap: map handle 0 to document.body (available after DOMContentLoaded).
function ensureBody() {
  if (!_elements.has(0) && typeof document !== 'undefined' && document.body) {
    _elements.set(0, document.body);
  }
}

// Element creation and lookup.
export function CreateElement(tag) {
  const el = document.createElement(tag);
  const id = _nextId++;
  _elements.set(id, el);
  return id;
}

export function CreateTextNode(text) {
  const node = document.createTextNode(text);
  const id = _nextId++;
  _elements.set(id, node);
  return id;
}

export function GetElementById(id) {
  const el = document.getElementById(id);
  if (!el) return -1;
  const handle = _nextId++;
  _elements.set(handle, el);
  return handle;
}

export function QuerySelector(sel) {
  const el = document.querySelector(sel);
  if (!el) return -1;
  const handle = _nextId++;
  _elements.set(handle, el);
  return handle;
}

export function Body() {
  ensureBody();
  return 0;
}

// Tree manipulation.
export function AppendChild(parentId, childId) {
  const parent = _elements.get(parentId);
  const child = _elements.get(childId);
  if (parent && child) parent.appendChild(child);
}

export function RemoveChild(parentId, childId) {
  const parent = _elements.get(parentId);
  const child = _elements.get(childId);
  if (parent && child) parent.removeChild(child);
}

export function FirstChild(parentId) {
  const parent = _elements.get(parentId);
  if (parent && parent.firstChild) {
    const id = _nextId++;
    _elements.set(id, parent.firstChild);
    return id;
  }
  return 0;
}

export function NextSibling(elId) {
  const el = _elements.get(elId);
  if (el && el.nextSibling) {
    const id = _nextId++;
    _elements.set(id, el.nextSibling);
    return id;
  }
  return 0;
}

export function InsertBefore(parentId, newId, refId) {
  const parent = _elements.get(parentId);
  const newEl = _elements.get(newId);
  const ref = refId >= 0 ? _elements.get(refId) : null;
  if (parent && newEl) parent.insertBefore(newEl, ref);
}

export function ReplaceChild(parentId, newId, oldId) {
  const parent = _elements.get(parentId);
  const newEl = _elements.get(newId);
  const oldEl = _elements.get(oldId);
  if (parent && newEl && oldEl) parent.replaceChild(newEl, oldEl);
}

// Properties and attributes.
export function SetAttribute(elId, name, value) {
  const el = _elements.get(elId);
  if (el) el.setAttribute(name, value);
}

export function RemoveAttribute(elId, name) {
  const el = _elements.get(elId);
  if (el) el.removeAttribute(name);
}

export function SetTextContent(elId, text) {
  const el = _elements.get(elId);
  if (el) el.textContent = text;
}

export function SetInnerHTML(elId, html) {
  const el = _elements.get(elId);
  if (el) el.innerHTML = html;
}

export function SetStyle(elId, prop, value) {
  const el = _elements.get(elId);
  if (el) el.style[prop] = value;
}

export function SetProperty(elId, prop, value) {
  const el = _elements.get(elId);
  if (el) el[prop] = value;
}

export function GetProperty(elId, prop) {
  const el = _elements.get(elId);
  if (el) return String(el[prop] ?? '');
  return '';
}

export function AddClass(elId, cls) {
  const el = _elements.get(elId);
  if (el && el.classList) el.classList.add(cls);
}

export function RemoveClass(elId, cls) {
  const el = _elements.get(elId);
  if (el && el.classList) el.classList.remove(cls);
}

// Events.
export function AddEventListener(elId, event, callbackId) {
  const el = _elements.get(elId);
  const cb = _callbacks.get(callbackId);
  if (el && cb) el.addEventListener(event, cb);
}

export function RemoveEventListener(elId, event, callbackId) {
  const el = _elements.get(elId);
  const cb = _callbacks.get(callbackId);
  if (el && cb) el.removeEventListener(event, cb);
}

// Register a Go function as a JS callback. Returns callback ID.
export function RegisterCallback(fn) {
  const id = _nextCb++;
  _callbacks.set(id, fn);
  return id;
}

export function ReleaseCallback(id) {
  _callbacks.delete(id);
}

// Scheduling.
export function RequestAnimationFrame(fn) {
  if (typeof window !== 'undefined') {
    window.requestAnimationFrame(fn);
  } else {
    setTimeout(fn, 16);
  }
}

export function SetTimeout(fn, ms) {
  return setTimeout(fn, ms);
}

export function SetInterval(fn, ms) {
  return setInterval(fn, ms);
}

export function ClearInterval(id) {
  clearInterval(id);
}

export function ClearTimeout(id) {
  clearTimeout(id);
}

// Cleanup: release element handle.
export function ReleaseElement(id) {
  _elements.delete(id);
}

// Fetch a URL as text, call fn with result.
export function FetchText(url, fn) {
  fetch(url).then(r => r.text()).then(t => { if (fn) fn(t); });
}

// Fetch NIP-11 relay info document with Accept header.
export function FetchRelayInfo(url, fn) {
  fetch(url, { headers: { 'Accept': 'application/nostr+json' } })
    .then(r => r.text())
    .then(t => { if (fn) fn(t); })
    .catch(() => { if (fn) fn(''); });
}

// --- IndexedDB ---

let _db = null;
let _dbReady = [];

function ensureDB(cb) {
  if (_db) { cb(_db); return; }
  _dbReady.push(cb);
  if (_dbReady.length > 1) return; // already opening
  const req = indexedDB.open('sm3sh', 3);
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
  req.onblocked = () => { console.warn('[sm3sh] IDB upgrade blocked — close other tabs'); };
  req.onsuccess = (e) => {
    _db = e.target.result;
    _db.onversionchange = () => { _db.close(); _db = null; };
    for (const fn of _dbReady) fn(_db);
    _dbReady = [];
  };
  req.onerror = () => { _dbReady = []; };
}

export function IDBGet(store, key, fn) {
  ensureDB((db) => {
    try {
      const tx = db.transaction(store, 'readonly');
      const req = tx.objectStore(store).get(key);
      req.onsuccess = () => { fn(req.result ?? ''); };
      req.onerror = () => { fn(''); };
    } catch(e) { fn(''); }
  });
}

export function IDBPut(store, key, value) {
  ensureDB((db) => {
    try {
      const tx = db.transaction(store, 'readwrite');
      tx.objectStore(store).put(value, key);
    } catch(e) {}
  });
}

export function IDBGetAll(store, fn, done) {
  ensureDB((db) => {
    try {
      const tx = db.transaction(store, 'readonly');
      const req = tx.objectStore(store).openCursor();
      req.onsuccess = (e) => {
        const cursor = e.target.result;
        if (cursor) {
          fn(String(cursor.key), String(cursor.value ?? ''));
          cursor.continue();
        } else {
          done();
        }
      };
      req.onerror = () => { done(); };
    } catch(e) { done(); }
  });
}

// Check if browser prefers dark color scheme.
export function PrefersDark() {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }
  return false;
}

// Log a message to the browser console.
export function ConsoleLog(msg) {
  console.log('[sm3sh]', msg);
}

// Send a raw JSON string to the service worker controller.
// Messages sent before the SW is active are queued and flushed on controllerchange.
let _swQueue = null;
export function PostToSW(msg) {
  const sw = navigator.serviceWorker;
  if (!sw) return;
  if (sw.controller) {
    sw.controller.postMessage(msg);
  } else {
    if (!_swQueue) {
      _swQueue = [];
      sw.addEventListener('controllerchange', () => {
        if (sw.controller) {
          for (const m of _swQueue) sw.controller.postMessage(m);
        }
        _swQueue = null;
      }, { once: true });
    }
    _swQueue.push(msg);
  }
}

// Register a handler for non-bus messages from the service worker.
// Bus relay (shell SW → satellite SWs) is handled in index.html inline script
// so it's active before WASM loads.
export function OnSWMessage(fn) {
  if (!navigator.serviceWorker) return;
  navigator.serviceWorker.addEventListener('message', (event) => {
    const d = event.data;
    // Bus messages handled by index.html — skip here.
    if (typeof d === 'string' && d.length > 0 && d[0] === '{') return;
    if (typeof d === 'string') {
      fn(d);
    } else if (Array.isArray(d) && d.length > 0) {
      fn(JSON.stringify(d));
    }
  });
}

// --- History API ---

export function PushState(path) {
  history.pushState(null, '', path);
}

export function ReplaceState(path) {
  history.replaceState(null, '', path);
}

export function LocationReload() {
  location.reload();
}

export function GetPath() {
  return location.pathname + location.hash;
}

export function Hostname() {
  return location.hostname;
}

export function Port() {
  return location.port;
}

export function OnPopState(fn) {
  window.addEventListener('popstate', () => {
    fn(location.pathname + location.hash);
  });
}

// Get raw element (for advanced use within JS runtime only).
export function getRawElement(id) {
  return _elements.get(id);
}
