// sm3sh service worker — SSE-driven hot reload.
//
// Caches all app files on install. Maintains an SSE connection to /__sse
// for version updates. When the server signals a new version, fetches
// updated files into a staging cache, then notifies all clients.
// The new version activates only when the user clicks the snackbar.

const CACHE_PREFIX = 'sm3sh-v';
const APP_FILES = [
  './',
  './index.html',
  './$entry.mjs',
  './smesh3.mjs',
  './smesh3_crypto_sha256.mjs',
  './smesh3_helpers.mjs',
  './smesh3_jsbridge_crypto.mjs',
  './smesh3_jsbridge_dom.mjs',
  './smesh3_jsbridge_localstorage.mjs',
  './smesh3_jsbridge_ws.mjs',
  './smesh3_nostr.mjs',
  './smesh3_relay.mjs',
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
let pendingVersion = null;
let sse = null;

// --- Lifecycle ---

self.addEventListener('install', (event) => {
  event.waitUntil(
    fetch('/__version')
      .then(r => r.text())
      .then(v => {
        currentVersion = v.trim();
        return caches.open(CACHE_PREFIX + currentVersion);
      })
      .then(cache => cache.addAll(APP_FILES))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    purgeOldCaches().then(() => {
      connectSSE();
      return self.clients.claim();
    })
  );
});

// --- Fetch: cache-first ---

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Don't cache SSE or version endpoints.
  if (url.pathname === '/__sse' || url.pathname === '/__version') {
    return;
  }

  event.respondWith(
    caches.match(event.request).then(cached => {
      return cached || fetch(event.request);
    })
  );
});

// --- Messages from clients ---

self.addEventListener('message', (event) => {
  if (event.data === 'activate-update' && pendingVersion) {
    activateUpdate();
  }
  if (event.data === 'get-status') {
    event.source.postMessage({
      type: 'status',
      currentVersion,
      pendingVersion,
    });
  }
});

// --- SSE connection ---

function connectSSE() {
  if (sse) {
    sse.close();
  }

  sse = new EventSource('/__sse');

  sse.onmessage = (event) => {
    const serverVersion = event.data.trim();
    if (!currentVersion) {
      currentVersion = serverVersion;
      return;
    }
    if (serverVersion !== currentVersion && serverVersion !== pendingVersion) {
      stageUpdate(serverVersion);
    }
  };

  sse.onerror = () => {
    // EventSource auto-reconnects. Nothing to do.
  };
}

// --- Update staging ---

async function stageUpdate(newVersion) {
  try {
    const cache = await caches.open(CACHE_PREFIX + newVersion);
    // Fetch all files with cache-busting.
    const bust = '?v=' + newVersion;
    await Promise.all(
      APP_FILES.map(file =>
        fetch(file + bust).then(resp => {
          if (resp.ok) {
            // Store without the query string.
            return cache.put(new Request(file), resp);
          }
        })
      )
    );
    pendingVersion = newVersion;
    notifyClients({ type: 'update-available', version: newVersion });
  } catch (err) {
    console.error('sm3sh sw: staging failed', err);
  }
}

async function activateUpdate() {
  if (!pendingVersion) return;

  currentVersion = pendingVersion;
  pendingVersion = null;

  await purgeOldCaches();
  notifyClients({ type: 'update-activated' });
}

// --- Helpers ---

async function purgeOldCaches() {
  const keys = await caches.keys();
  const keep = CACHE_PREFIX + currentVersion;
  const pending = pendingVersion ? CACHE_PREFIX + pendingVersion : null;
  await Promise.all(
    keys
      .filter(k => k.startsWith(CACHE_PREFIX) && k !== keep && k !== pending)
      .map(k => caches.delete(k))
  );
}

async function notifyClients(msg) {
  const all = await self.clients.matchAll({ type: 'window' });
  for (const client of all) {
    client.postMessage(msg);
  }
}
