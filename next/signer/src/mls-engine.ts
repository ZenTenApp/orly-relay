/* eslint-disable @typescript-eslint/no-explicit-any */
// mls-engine.ts — Loads marmot.wasm in the signer extension background.
// All MLS crypto happens here with direct access to vault keys.
// No bus round-trips for signing or encryption.

import { signEvent, nip44Encrypt, nip44Decrypt, nip04Encrypt, nip04Decrypt } from './background-common';
import browser from 'webextension-polyfill';

// --- State ---
let wasmReady = false;
const mlsTabIds = new Set<number>(); // Fix 2g: broadcast to all tabs
let currentPrivkey = '';
let mlsInitPromise: Promise<string> | null = null;
let wasmLoadingPromise: Promise<void> | null = null; // Fix 2d: guard concurrent instantiation
let lastEventTSInterval: ReturnType<typeof setInterval> | null = null; // Fix 2e: track interval

// Fix 2b: queue events arriving before init completes
let initDone = false;
const pendingDeliverEvents: Array<{ subId: number; eventJSON: string }> = [];

// --- IDB Group Store (same schema as the marmot SW used) ---
const GDB_NAME = 'marmot-groups';
const GDB_VER = 1;
const GDB_STORE = 'groups';

// Open DB for writes — creates the store if it doesn't exist yet.
function gdbOpenWrite(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(GDB_NAME, GDB_VER);
    req.onupgradeneeded = (e: any) => {
      const db = e.target.result as IDBDatabase;
      if (!db.objectStoreNames.contains(GDB_STORE)) {
        db.createObjectStore(GDB_STORE);
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

// Open DB for reads — returns null if the DB/store doesn't exist (no side-effects).
function gdbOpenRead(): Promise<IDBDatabase | null> {
  return new Promise((resolve) => {
    const req = indexedDB.open(GDB_NAME);
    req.onupgradeneeded = () => {
      // DB doesn't exist yet — abort so we don't create an empty one.
      req.result.close();
      req.transaction?.abort();
    };
    req.onsuccess = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(GDB_STORE)) {
        db.close();
        resolve(null);
        return;
      }
      resolve(db);
    };
    req.onerror = () => resolve(null);
    req.onblocked = () => resolve(null);
  });
}

function storeResult(id: number, data: string, err: string) {
  const m = (globalThis as any)._marmot;
  if (m?.storeResult) m.storeResult(id, data || '', err || '');
}

function setupStoreCallbacks() {
  const g = globalThis as any;
  g._marmot_store_save = (id: number, groupIDHex: string, stateHex: string) => {
    gdbOpenWrite().then(db => {
      const tx = db.transaction(GDB_STORE, 'readwrite');
      tx.objectStore(GDB_STORE).put(stateHex, groupIDHex);
      tx.oncomplete = () => storeResult(id, '', '');
      tx.onerror = () => storeResult(id, '', tx.error?.message || 'save failed');
    }).catch(e => storeResult(id, '', (e as Error).message));
  };
  g._marmot_store_load = (id: number, groupIDHex: string) => {
    gdbOpenRead().then(db => {
      if (!db) { storeResult(id, '', ''); return; }
      const tx = db.transaction(GDB_STORE, 'readonly');
      const req = tx.objectStore(GDB_STORE).get(groupIDHex);
      req.onsuccess = () => storeResult(id, req.result || '', '');
      req.onerror = () => storeResult(id, '', req.error?.message || 'load failed');
    }).catch(e => storeResult(id, '', (e as Error).message));
  };
  g._marmot_store_list = (id: number) => {
    gdbOpenRead().then(db => {
      if (!db) { storeResult(id, '', ''); return; }
      const tx = db.transaction(GDB_STORE, 'readonly');
      const req = tx.objectStore(GDB_STORE).getAllKeys();
      req.onsuccess = () => storeResult(id, (req.result || []).join(','), '');
      req.onerror = () => storeResult(id, '', req.error?.message || 'list failed');
    }).catch(e => storeResult(id, '', (e as Error).message));
  };
  g._marmot_store_delete = (id: number, groupIDHex: string) => {
    gdbOpenRead().then(db => {
      if (!db) { storeResult(id, '', ''); return; }
      const tx = db.transaction(GDB_STORE, 'readwrite');
      tx.objectStore(GDB_STORE).delete(groupIDHex);
      tx.oncomplete = () => storeResult(id, '', '');
      tx.onerror = () => storeResult(id, '', tx.error?.message || 'delete failed');
    }).catch(e => storeResult(id, '', (e as Error).message));
  };
}

// --- WASM Loading ---

// Fix 2d: guard against concurrent instantiation
async function loadWasm(): Promise<void> {
  if (wasmReady) return;
  if (wasmLoadingPromise) return wasmLoadingPromise;
  wasmLoadingPromise = doLoadWasm();
  try {
    await wasmLoadingPromise;
  } catch (e) {
    wasmLoadingPromise = null; // allow retry on failure
    throw e;
  }
}

async function doLoadWasm(): Promise<void> {
  // wasm_exec.js is loaded as a background script (manifest.json),
  // so globalThis.Go is already available.
  const GoClass = (globalThis as any).Go;
  if (!GoClass) throw new Error('Go WASM runtime not loaded (wasm_exec.js missing from background scripts)');

  const go = new GoClass();
  const wasmUrl = browser.runtime.getURL('marmot.wasm');
  const wasmResponse = await fetch(wasmUrl);
  const wasmBytes = await wasmResponse.arrayBuffer();
  const result = await WebAssembly.instantiate(wasmBytes, go.importObject);

  // Start the Go program. Don't await — it blocks forever (select {}).
  // The synchronous part sets up globalThis._marmot before yielding.
  go.run(result.instance).then(
    () => { console.error('[mls-engine] Go program exited unexpectedly'); wasmReady = false; },
    (err: any) => { console.error('[mls-engine] Go program crashed:', err); wasmReady = false; }
  );

  // Verify the Go side registered its API before we proceed.
  if (!(globalThis as any)._marmot) {
    throw new Error('Go WASM started but _marmot global not registered');
  }

  setupStoreCallbacks();
  wasmReady = true;
}

// --- Callback Bridge ---

function setupMarmotInit(lastEventTS: number) {
  const g = globalThis as any;

  // Override _marmot_init to wire our local callbacks.
  // Returns a Promise that resolves when NewClient completes (it runs in a
  // goroutine because store.ListGroups blocks on async IDB callbacks).
  g._marmot_init = async (pubkeyHex: string, ...relayURLs: string[]): Promise<string> => {
    const marmot = g._marmot;
    if (!marmot) return 'error: wasm not loaded';

    const publishFn = (eventJSON: string) => {
      pushToTab({ cmd: 'publish', event: eventJSON });
    };

    const subscribeFn = (subId: number, filterJSON: string) => {
      pushToTab({ cmd: 'subscribe', subId, filter: filterJSON });
    };

    const cryptoSendFn = (op: string, peerHex: string, data: string, id: number) => {
      handleCryptoLocal(op, peerHex, data, id);
    };

    const onDMFn = (senderHex: string, plaintext: string) => {
      const ts = Math.floor(Date.now() / 1000);
      pushToTab({
        cmd: 'dm',
        peer: senderHex,
        sender: senderHex,
        content: plaintext,
        ts,
        source: 'marmot',
        eventId: '',
      });
    };

    const onStatusFn = (msg: string) => {
      pushToTab({ cmd: 'status', msg });
    };

    // Fix 2e: clear previous interval before creating new.
    if (lastEventTSInterval) clearInterval(lastEventTSInterval);
    lastEventTSInterval = setInterval(() => {
      try {
        const ts = marmot.lastEventTS?.();
        if (ts > 0) pushToTab({ cmd: 'mls_ts', ts });
      } catch (_) {}
    }, 30000);

    return new Promise<string>((resolve) => {
      const onReadyFn = (result: string) => {
        resolve(result || 'ok');
      };
      marmot.init(pubkeyHex, publishFn, subscribeFn, cryptoSendFn, onDMFn, onStatusFn, onReadyFn, lastEventTS, ...relayURLs);
    });
  };
}

// --- Local Crypto (no round-trip!) ---

function handleCryptoLocal(op: string, peerHex: string, data: string, id: number) {
  const marmot = (globalThis as any)._marmot;
  if (!marmot) return;

  const resolve = (result: string, err: string) => {
    marmot.cryptoResult(id, result, err);
  };

  try {
    switch (op) {
      case 'signEvent': {
        const eventTemplate = JSON.parse(data);
        const signed = signEvent(eventTemplate, currentPrivkey);
        resolve(JSON.stringify(signed), '');
        break;
      }
      case 'nip44.encrypt':
      case 'nip44Encrypt':
        nip44Encrypt(currentPrivkey, peerHex, data).then(
          r => resolve(r, ''),
          e => resolve('', (e as Error).message)
        );
        break;
      case 'nip44.decrypt':
      case 'nip44Decrypt':
        nip44Decrypt(currentPrivkey, peerHex, data).then(
          r => resolve(r, ''),
          e => resolve('', (e as Error).message)
        );
        break;
      case 'nip04.encrypt':
      case 'nip04Encrypt':
        nip04Encrypt(currentPrivkey, peerHex, data).then(
          r => resolve(r, ''),
          e => resolve('', (e as Error).message)
        );
        break;
      case 'nip04.decrypt':
      case 'nip04Decrypt':
        nip04Decrypt(currentPrivkey, peerHex, data).then(
          r => resolve(r, ''),
          e => resolve('', (e as Error).message)
        );
        break;
      default:
        resolve('', 'unsupported crypto op: ' + op);
    }
  } catch (err) {
    resolve('', (err as Error).message);
  }
}

// --- Push to originating tab ---
// Broadcast to all smesh tabs. Re-discover on every send to avoid stale IDs
// after idle (tab refresh assigns new ID, old one stays in Set forever).

async function discoverTabs(): Promise<void> {
  try {
    const tabs = await browser.tabs.query({ url: ['*://smesh.lol/*', '*://127.0.0.1:*/*', '*://localhost:*/*'] });
    mlsTabIds.clear();
    for (const t of tabs) {
      if (t.id) mlsTabIds.add(t.id);
    }
  } catch (_) {}
}

async function pushToTab(data: any) {
  // Always re-discover — tabs.query is fast (<1ms) and prevents stale IDs.
  await discoverTabs();
  if (mlsTabIds.size === 0) return;
  const msg = { ext: 'smesh-signer', type: 'mls-push', data };
  for (const tabId of mlsTabIds) {
    browser.tabs.sendMessage(tabId, msg).catch(() => {
      mlsTabIds.delete(tabId);
    });
  }
}

// --- Public API (called from background.ts) ---

export async function mlsInit(
  privkey: string,
  pubkey: string,
  relayURLs: string[],
  tabId: number,
  lastEventTS: number = 0
): Promise<string> {
  mlsTabIds.add(tabId); // Fix 2g: add, don't overwrite
  currentPrivkey = privkey;
  // Fix 2f: reset on failure so retry is possible
  mlsInitPromise = (async () => {
    await loadWasm();
    setupMarmotInit(lastEventTS);
    const initFn = (globalThis as any)._marmot_init;
    if (!initFn) return 'error: wasm bridge not ready';
    const result = await initFn(pubkey, ...relayURLs);
    // Auto-bootstrap: publish key package and start subscription loop.
    if (!result || result === 'ok') {
      const m = (globalThis as any)._marmot;
      if (m) {
        m.publishKP();
        m.subscribe();
      }
    }
    // Fix 2b: mark init done and drain pending events.
    initDone = true;
    const pending = pendingDeliverEvents.splice(0);
    for (const p of pending) {
      const m = (globalThis as any)._marmot;
      if (m) m.deliverEvent(p.subId, p.eventJSON);
    }
    return String(result || 'ok');
  })().catch(e => {
    mlsInitPromise = null; // Fix 2f: allow retry
    throw e;
  });
  return mlsInitPromise;
}

export function mlsSetTab(tabId: number) {
  mlsTabIds.add(tabId); // Fix 2g
}

export async function mlsSendDM(recipient: string, content: string): Promise<string> {
  if (mlsInitPromise) await mlsInitPromise;
  const m = (globalThis as any)._marmot;
  if (!m) return 'error: not initialized';
  return String(m.sendDM(recipient, content) || 'ok');
}

export async function mlsSubscribe(): Promise<string> {
  if (mlsInitPromise) await mlsInitPromise;
  const m = (globalThis as any)._marmot;
  if (!m) return 'error: not initialized';
  return String(m.subscribe() || 'ok');
}

export async function mlsPublishKP(): Promise<string> {
  if (mlsInitPromise) await mlsInitPromise;
  const m = (globalThis as any)._marmot;
  if (!m) return 'error: not initialized';
  return String(m.publishKP() || 'ok');
}

export async function mlsListGroups(): Promise<string> {
  if (mlsInitPromise) await mlsInitPromise;
  const m = (globalThis as any)._marmot;
  if (!m) return '[]';
  return String(m.listGroups() || '[]');
}

// Fix 2b: queue events before init completes
export function mlsDeliverEvent(subId: number, eventJSON: string): void {
  if (!initDone) {
    pendingDeliverEvents.push({ subId, eventJSON });
    return;
  }
  const m = (globalThis as any)._marmot;
  if (!m) return;
  m.deliverEvent(subId, eventJSON);
}

export function mlsHandleEvent(eventJSON: string): void {
  const m = (globalThis as any)._marmot;
  if (!m) return;
  m.handleEvent(eventJSON);
}

export function isMlsMethod(method: string): boolean {
  return method.startsWith('mls.');
}
