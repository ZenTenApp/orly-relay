// MLS Bridge — routes MLS commands between shell SW and signer extension.
// Loaded by index.html. Does not require TinyGo recompilation.

// Relay URLs from the most recent mls.init — used for publish/subscribe routing.
let _mlsRelays = [];

// --- Fix 1b: Queue MLS_PROXY messages until window.nostr.mls is available ---
let _proxyQueue = [];
let _proxyReady = false;
let _proxyPoll = null;

function checkProxyReady() {
  if (window.nostr?.mls) {
    _proxyReady = true;
    if (_proxyPoll) { clearInterval(_proxyPoll); _proxyPoll = null; }
    // Drain queued messages.
    const q = _proxyQueue.splice(0);
    for (const d of q) handleMlsProxy(d);
  }
}

function handleMlsProxy(d) {
  if (!_proxyReady) {
    _proxyQueue.push(d);
    if (!_proxyPoll) _proxyPoll = setInterval(checkProxyReady, 200);
    return;
  }
  const method = d[1];
  (async () => {
    try {
      switch (method) {
        case 'init': {
          _mlsRelays = d[2] || [];
          const lastTS = parseInt(localStorage.getItem('marmot_last_event_ts') || '0', 10) || 0;
          await window.nostr.mls.init(d[2], lastTS);
          break;
        }
        case 'sendDM':
          await window.nostr.mls.sendDM(d[2], d[3]);
          break;
        case 'subscribe':
          await window.nostr.mls.subscribe();
          break;
        case 'publishKP':
          await window.nostr.mls.publishKP();
          break;
        case 'listGroups': {
          const groups = await window.nostr.mls.listGroups();
          postToSW('["MLS_GROUPS",' + JSON.stringify(groups) + ']');
          break;
        }
        case 'deliverEvent':
          await window.nostr.mls.deliverEvent(d[2], d[3]);
          break;
        case 'backupGroups':
          await window.nostr.mls.backupGroups();
          break;
        case 'restoreGroups':
          await window.nostr.mls.restoreGroups();
          break;
        case 'ratchetGroup':
          await window.nostr.mls.ratchetGroup(d[2]);
          break;
      }
    } catch (err) {
      console.error('mls-bridge: ' + method + ' failed:', err);
    }
  })();
}

// Handle MLS_PROXY messages from the shell SW.
if (navigator.serviceWorker) {
  navigator.serviceWorker.addEventListener('message', async (event) => {
    const d = event.data;
    if (!Array.isArray(d) || d[0] !== 'MLS_PROXY') return;
    handleMlsProxy(d);
  });
}

// Handle MLS push events from the signer extension.
// These are dispatched as 'nostr-mls' CustomEvents by the injected script.
window.addEventListener('nostr-mls', (event) => {
  const data = event.detail;
  if (!data) return;
  switch (data.cmd) {
    case 'publish':
      postToSW('["MLS_PUBLISH",' + JSON.stringify(data.event) + ',' + JSON.stringify(_mlsRelays) + ']');
      break;
    case 'subscribe':
      postToSW('["MLS_SUBSCRIBE",' + JSON.stringify(String(data.subId)) + ',' + data.filter + ',' + JSON.stringify(_mlsRelays) + ']');
      break;
    case 'dm': {
      const rec = JSON.stringify({
        peer: data.peer, sender: data.sender, content: data.content,
        ts: data.ts, source: data.source, eventId: data.eventId
      });
      postToSW('["MLS_DM",' + rec + ']');
      break;
    }
    case 'status': {
      const msg = data.msg;
      postToSW('["MLS_STATUS",' + JSON.stringify(msg) + ']');
      // Ratchet completion — clear DM history for the peer.
      if (typeof msg === 'string' && msg.startsWith('ratchet ok:')) {
        const peer = msg.slice('ratchet ok:'.length);
        postToSW('["CLEAR_DM_HISTORY",' + JSON.stringify(peer) + ']');
      }
      break;
    }
    case 'relays':
      _mlsRelays = data.relays || [];
      break;
    case 'mls_ts':
      if (data.ts > 0) localStorage.setItem('marmot_last_event_ts', String(data.ts));
      break;
  }
});

// --- Fix 1a: Check for relay URLs that were set before this script loaded ---
if (window._nostrMlsRelays && window._nostrMlsRelays.length > 0) {
  _mlsRelays = window._nostrMlsRelays;
}

let _mlsQueue = null;
function postToSW(msg) {
  const sw = navigator.serviceWorker;
  if (!sw) return;
  if (sw.controller) {
    sw.controller.postMessage(msg);
  } else {
    if (!_mlsQueue) {
      _mlsQueue = [];
      // Fix 1e: removed { once: true } — second controller swap must be detected.
      sw.addEventListener('controllerchange', () => {
        if (sw.controller && _mlsQueue) {
          for (const m of _mlsQueue) sw.controller.postMessage(m);
        }
        _mlsQueue = null;
      });
    }
    _mlsQueue.push(msg);
  }
}
