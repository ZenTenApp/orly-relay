// sm3sh service worker — SSE-driven hot reload.
//
// On install: caches all app files, activates immediately.
// On activate: connects to /__sse for version updates.
// On version change: purges cache, refetches, reloads all clients.

const CACHE = 'sm3sh';
const APP_FILES = [
  './',
  './index.html',
  './$entry.mjs',
  './smesh3.mjs',
  './common_crypto_sha256.mjs',
  './common_helpers.mjs',
  './common_jsbridge_crypto.mjs',
  './common_jsbridge_dom.mjs',
  './common_jsbridge_localstorage.mjs',
  './common_jsbridge_ws.mjs',
  './common_nostr.mjs',
  './common_relay.mjs',
  './$runtime/index.mjs',
  './$runtime/runtime.mjs',
  './$runtime/goroutine.mjs',
  './$runtime/channel.mjs',
  './$runtime/builtin.mjs',
  './$runtime/types.mjs',
  './$runtime/sync.mjs',
  './$runtime/dom.mjs',
  './$runtime/ws.mjs',
  './$runtime/localstorage.mjs',
  './$runtime/crypto.mjs',
  './$wasm/secp256k1.mjs',
  './$wasm/secp256k1.wasm',
];

let currentVersion = null;
let sse = null;

// --- Lifecycle ---

self.addEventListener('install', (event) => {
  event.waitUntil(
    fetch('/__version')
      .then(r => r.text())
      .then(v => {
        currentVersion = v.trim();
        return caches.open(CACHE);
      })
      .then(cache => cache.addAll(APP_FILES))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    self.clients.claim().then(() => connectSSE())
  );
});

// --- Fetch: cache-first, network fallback ---

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  if (url.pathname === '/__sse' || url.pathname === '/__version') return;

  event.respondWith(
    caches.match(event.request).then(cached => cached || fetch(event.request))
  );
});

// --- SSE: version change → purge + reload ---

function connectSSE() {
  if (sse) sse.close();
  sse = new EventSource('/__sse');

  sse.onmessage = (event) => {
    const v = event.data.trim();
    if (!currentVersion) {
      currentVersion = v;
      return;
    }
    if (v !== currentVersion) {
      currentVersion = v;
      refreshAndReload();
    }
  };
}

async function refreshAndReload() {
  try {
    const cache = await caches.open(CACHE);
    const bust = '?v=' + currentVersion;
    await Promise.all(
      APP_FILES.map(file =>
        fetch(file + bust).then(resp => {
          if (resp.ok) return cache.put(new Request(file), resp);
        })
      )
    );
  } catch (err) {
    console.error('sm3sh sw: refresh failed', err);
  }
  // Reload all clients.
  const all = await self.clients.matchAll({ type: 'window' });
  for (const client of all) {
    client.navigate(client.url);
  }
}
