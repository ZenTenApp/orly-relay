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
    const id = _storeEvent(event);
    fn(id);
  });
}

export function OnActivate(fn) {
  self.addEventListener('activate', (event) => {
    const id = _storeEvent(event);
    fn(id);
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
    _events.delete(id);
  });
}

export function OnMessage(fn) {
  self.addEventListener('message', (event) => {
    const id = _storeEvent(event);
    fn(id);
    _events.delete(id);
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
  return ev?.source?.id || '';
}

// --- SW globals ---

export function Origin() {
  return self.location.origin;
}

export function SkipWaiting() {
  self.skipWaiting();
}

export function ClaimClients(done) {
  self.clients.claim().then(() => { if (done) done(); });
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
  if (c) c.postMessage(JSON.parse(json));
}

export function GetClientByID(id, fn) {
  self.clients.get(id).then(c => {
    if (c) fn(_storeClient(c), true);
    else fn(0, false);
  });
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
  });
}

export function CacheAddAll(cacheId, urls, done) {
  const cache = _caches.get(cacheId);
  if (!cache) { if (done) done(); return; }
  // Convert Go slice to native JS array for Cache API.
  const arr = urls.$array.slice(urls.$offset, urls.$offset + urls.$length);
  cache.addAll(arr).then(() => { if (done) done(); });
}

export function CachePut(cacheId, url, respId, done) {
  const cache = _caches.get(cacheId);
  const resp = _responses.get(respId);
  _responses.delete(respId);
  if (!cache || !resp) { if (done) done(); return; }
  cache.put(new Request(url), resp).then(() => { if (done) done(); });
}

export function CacheMatch(url, fn) {
  caches.match(new Request(url)).then((resp) => {
    fn(_storeResponse(resp));
  });
}

export function CacheDelete(name, done) {
  caches.delete(name).then(() => { if (done) done(); });
}

// --- Fetch ---

export function Fetch(url, fn) {
  fetch(url).then(
    (resp) => fn(_storeResponse(resp), true),
    () => fn(0, false)
  );
}

export function ResponseOK(respId) {
  const resp = _responses.get(respId);
  return resp ? resp.ok : false;
}

// --- SSE ---

export function SSEConnect(url, onMessage) {
  const id = _nextSseId++;
  const es = new EventSource(url);
  _sseConns.set(id, es);
  es.onmessage = (event) => {
    if (onMessage) onMessage(event.data);
  };
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

const _timers = new Map();
let _nextTimerId = 1;

export function SetTimeout(ms, fn) {
  const id = _nextTimerId++;
  _timers.set(id, setTimeout(() => { _timers.delete(id); fn(); }, ms));
  return id;
}

export function ClearTimeout(timerId) {
  const t = _timers.get(timerId);
  if (t !== undefined) { clearTimeout(t); _timers.delete(timerId); }
}

// --- Time ---

export function NowSeconds() {
  return Math.floor(Date.now() / 1000);
}

export function NowMillis() {
  return Date.now();
}

// --- Logging ---

function _rlog(level, msg) {
  fetch('/__log', {method:'POST', body: level + ' [marmot] ' + msg}).catch(()=>{});
}

export function Log(msg) {
  console.log('sw:', msg);
  _rlog('I', msg);
}

export function Warn(msg) {
  console.warn('sw:', msg);
  _rlog('W', msg);
}
