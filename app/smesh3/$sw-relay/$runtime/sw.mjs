// TinyJS Runtime — Service Worker Bridge
// Provides Go-callable Service Worker, Cache, Fetch, and SSE operations.

// --- Internal state ---

const _events = new Map();
let _nextEventId = 1;
const _caches = new Map();
let _nextCacheId = 1;
const _responses = new Map();
let _nextRespId = 1;
const _clients = new Map();
let _nextClientId = 1;
const _sseConns = new Map();
let _nextSseId = 1;

function _storeEvent(ev) {
  const id = _nextEventId++;
  _events.set(id, ev);
  return id;
}

function _storeResponse(resp) {
  if (!resp) return 0;
  const id = _nextRespId++;
  _responses.set(id, resp);
  return id;
}

function _storeClient(client) {
  const id = _nextClientId++;
  _clients.set(id, client);
  return id;
}

// --- Lifecycle ---

export function OnInstall(fn) {
  self.addEventListener('install', (event) => {
    fn(_storeEvent(event));
  });
}

export function OnActivate(fn) {
  self.addEventListener('activate', (event) => {
    fn(_storeEvent(event));
  });
}

// Fetch events use a deferred promise pattern:
// Go calls RespondWithCache or RespondWithNetwork synchronously to pick strategy.
// The runtime creates the promise and calls event.respondWith() synchronously.
const _fetchResolvers = new Map();

export function OnFetch(fn) {
  self.addEventListener('fetch', (event) => {
    const id = _storeEvent(event);
    fn(id);
    // Check if Go code set up a response strategy.
    const resolver = _fetchResolvers.get(id);
    if (resolver) {
      event.respondWith(resolver);
      _fetchResolvers.delete(id);
    }
    // If no resolver was set, browser handles the fetch normally.
  });
}

export function OnMessage(fn) {
  self.addEventListener('message', (event) => {
    fn(_storeEvent(event));
  });
}

// --- Event methods ---

export function WaitUntil(eventId, fn) {
  const ev = _events.get(eventId);
  if (!ev) return;
  ev.waitUntil(new Promise((resolve) => {
    fn(resolve);
  }));
}

export function RespondWith(eventId, respId) {
  const resp = _responses.get(respId);
  if (resp) {
    _fetchResolvers.set(eventId, Promise.resolve(resp));
  }
}

export function RespondWithNetwork(eventId) {
  // Don't set a resolver — browser handles the fetch.
}

// RespondWithCacheFirst tries cache, falls back to network.
// This is the common pattern and must be called synchronously from onFetch.
export function RespondWithCacheFirst(eventId) {
  const ev = _events.get(eventId);
  if (!ev) return;
  _fetchResolvers.set(eventId,
    caches.match(ev.request).then(cached => cached || fetch(ev.request))
  );
}

export function GetRequestURL(eventId) {
  const ev = _events.get(eventId);
  return ev ? ev.request.url : '';
}

export function GetRequestPath(eventId) {
  const ev = _events.get(eventId);
  if (!ev) return '';
  return new URL(ev.request.url).pathname;
}

export function GetMessageData(eventId) {
  const ev = _events.get(eventId);
  if (!ev) return '';
  const d = ev.data;
  return typeof d === 'string' ? d : JSON.stringify(d);
}

export function GetMessageClientID(eventId) {
  const ev = _events.get(eventId);
  if (!ev) return '';
  return ev.source ? ev.source.id || '' : '';
}

// --- SW globals ---

export function Origin() {
  return self.location.origin;
}

export function SkipWaiting() {
  self.skipWaiting();
}

export function ClaimClients(done) {
  self.clients.claim().then(() => { if (done) done(); }).catch(() => { if (done) done(); });
}

export function MatchClients(fn) {
  self.clients.matchAll({ type: 'window' }).then((all) => {
    for (const c of all) {
      fn(_storeClient(c));
    }
  });
}

export function PostMessage(clientId, msg) {
  const c = _clients.get(clientId);
  if (c) c.postMessage(msg);
}

export function PostMessageJSON(clientId, json) {
  const c = _clients.get(clientId);
  if (c) {
    try { c.postMessage(JSON.parse(json)); }
    catch { c.postMessage(json); }
  }
}

export function GetClientByID(id, fn) {
  self.clients.get(id).then((client) => {
    if (client) {
      fn(_storeClient(client), true);
    } else {
      fn(0, false);
    }
  }).catch(() => fn(0, false));
}

export function Navigate(clientId, url) {
  const c = _clients.get(clientId);
  if (c) c.navigate(url || c.url);
}

// --- Cache ---

export function CacheOpen(name, fn) {
  caches.open(name).then((cache) => {
    const id = _nextCacheId++;
    _caches.set(id, cache);
    fn(id);
  }).catch(() => fn(0));
}

export function CacheAddAll(cacheId, urls, done) {
  const cache = _caches.get(cacheId);
  if (!cache) { if (done) done(); return; }
  cache.addAll(urls).then(() => { if (done) done(); }).catch(() => { if (done) done(); });
}

export function CachePut(cacheId, url, respId, done) {
  const cache = _caches.get(cacheId);
  const resp = _responses.get(respId);
  if (!cache || !resp) { if (done) done(); return; }
  cache.put(new Request(url), resp).then(() => { if (done) done(); }).catch(() => { if (done) done(); });
}

export function CacheMatch(url, fn) {
  caches.match(new Request(url)).then((resp) => {
    fn(_storeResponse(resp));
  }).catch(() => fn(0));
}

export function CacheDelete(name, done) {
  caches.delete(name).then(() => { if (done) done(); }).catch(() => { if (done) done(); });
}

// --- Fetch ---

export function Fetch(url, fn) {
  fetch(url).then(
    (resp) => fn(_storeResponse(resp), true),
    () => fn(0, false)
  );
}

export function FetchAll(urls, onEach, onDone) {
  const arr = urls.$array
    ? urls.$array.slice(urls.$offset, urls.$offset + urls.$length)
    : urls;
  if (arr.length === 0) { if (onDone) onDone(); return; }
  let remaining = arr.length;
  for (let i = 0; i < arr.length; i++) {
    ((idx) => {
      fetch(arr[idx]).then(
        (resp) => { onEach(idx, _storeResponse(resp), true); if (--remaining === 0 && onDone) onDone(); },
        ()     => { onEach(idx, 0, false);                   if (--remaining === 0 && onDone) onDone(); }
      );
    })(i);
  }
}

export function ResponseOK(respId) {
  const resp = _responses.get(respId);
  return resp ? resp.ok : false;
}

// --- SSE ---

export function SSEConnect(url, onMessage) {
  const id = _nextSseId++;
  if (typeof EventSource !== 'undefined') {
    const es = new EventSource(url);
    _sseConns.set(id, es);
    es.onmessage = (event) => {
      if (onMessage) onMessage(event.data);
    };
  } else {
    // SW scope: EventSource not available. Poll with fetch.
    let active = true;
    _sseConns.set(id, { close() { active = false; } });
    (async function poll() {
      while (active) {
        try {
          const resp = await fetch(url, { headers: { 'Accept': 'text/event-stream' } });
          const reader = resp.body.getReader();
          const decoder = new TextDecoder();
          let buf = '';
          while (active) {
            const { value, done } = await reader.read();
            if (done) break;
            buf += decoder.decode(value, { stream: true });
            let idx;
            while ((idx = buf.indexOf('\n\n')) !== -1) {
              const msg = buf.substring(0, idx);
              buf = buf.substring(idx + 2);
              for (const line of msg.split('\n')) {
                if (line.startsWith('data: ') && onMessage) {
                  onMessage(line.substring(6));
                }
              }
            }
          }
        } catch (e) {
          if (active) await new Promise(r => setTimeout(r, 3000));
        }
      }
    })();
  }
  return id;
}

export function SSEClose(sseId) {
  const es = _sseConns.get(sseId);
  if (es) {
    es.close();
    _sseConns.delete(sseId);
  }
}

// --- Timers ---

export function SetTimeout(ms, fn) {
  return setTimeout(fn, ms);
}

export function ClearTimeout(t) {
  clearTimeout(t);
}

export function NowSeconds() {
  return Math.floor(Date.now() / 1000);
}

export function NowMillis() {
  return Date.now();
}

// --- Logging ---

export function Log(msg) {
  console.log('sw:', msg);
}
