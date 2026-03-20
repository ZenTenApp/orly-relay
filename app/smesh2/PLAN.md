# smesh2 cleanup plan

## Phase 0: Channel primitive

A ~30-line module that replaces callback maps, timeout cleanup, and stale closures with linear async flow.

### chan.js

```js
// broadcast channel: multiple listeners, each gets every message
function chan() {
  const waiters = []
  return {
    send(val) {
      for (const w of waiters.splice(0)) w(val)
    },
    recv() {
      return new Promise(resolve => waiters.push(resolve))
    }
  }
}

// multiplexed channel: keyed by request ID, one-shot per key
function mux() {
  const pending = new Map()
  return {
    send(id, val) {
      const resolve = pending.get(id)
      if (resolve) { pending.delete(id); resolve(val) }
    },
    recv(id, timeoutMs = 15000) {
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => { pending.delete(id); reject(new Error('timeout')) }, timeoutMs)
        pending.set(id, (val) => { clearTimeout(timer); resolve(val) })
      })
    }
  }
}

export { chan, mux }
```

Used in both SW and UI:

SW side (dm.js):
```js
const cryptoMux = mux()
// requestCrypto becomes:
async function requestCrypto(type, pubkey, text) {
  const id = ++cryptoRequestId
  broadcastToClients([type, id, pubkey, text])
  return cryptoMux.recv(id)
}
// CRYPTO_RESULT handler becomes:
cryptoMux.send(requestId, { result, error })
```

UI side (app.js):
```js
const signMux = mux()
const swChan = chan()  // broadcast: every SW message goes here

// signViaSW becomes:
async function signViaSW(event) {
  const id = ++signCounter
  send(['SIGN', id, event])
  return signMux.recv(id)
}

// message handler — one line, no switch statement for routing
navigator.serviceWorker.addEventListener('message', e => {
  const [type, ...args] = e.data
  swChan.send({ type, args })      // broadcast to all listeners
  if (type === 'SIGNED') signMux.send(args[0], args[1])
  // etc for other muxed channels
})

// DM listener — reads state at consumption time, never stale
async function listenDMs() {
  while (true) {
    const msg = await swChan.recv()
    if (msg.type !== 'DM_RECEIVED') continue
    const dm = msg.args[0]
    dispatch({ type: 'ADD_CONVERSATION', conversation: { peer: dm.peer, lastMessage: dm.content.slice(0, 80), lastTs: dm.created_at, from: dm.from } })
    dispatch({ type: 'ADD_DM_MESSAGE', message: dm })
  }
}
```

The broadcast channel (`chan`) replaces useEffect message handlers. The multiplexed channel (`mux`) replaces callback maps with timeouts. Both are tiny, zero-dependency, and eliminate the stale closure disease at the root.

## Phase 1: Split sw.js into modules

Create 5 new files, keeping sw.js as the orchestrator.

### crypto.js (~170 lines)
- signEvent, signEventWithKey, randomizeTimestamp
- nip04SharedKey, nip04EncryptRaw, nip04DecryptRaw
- NIP44_SALT, nip44ConversationKey, nip44MessageKeys
- nip44CalcPadding, nip44Pad, nip44Unpad, nip44EncryptRaw, nip44DecryptRaw
- giftWrap (needs crypto + signing)
- toBase64 chunked encoder (no spread overflow)
- FIX #1: replace `btoa(String.fromCharCode(...payload))` with chunked encoder

Exports: all functions above. Imports noble-curves/hashes/ciphers.

### db.js (~140 lines)
- DB_NAME, DB_VERSION, openDB, getDB, patchDBClose
- saveEvent, queryEvents
- saveDM, queryDMs, getConversationList, dmDedupId

Exports: getDB, saveEvent, queryEvents, saveDM, queryDMs, getConversationList, dmDedupId.

### pool.js (~220 lines)
- MAX_CONNECTIONS, pool map, getConnection
- sendToRelay, sanitizeFilter, HEX_FILTER_KEYS
- handleRelayMessage (EVENT, EOSE, OK, NOTICE, AUTH)
- proxySubs, handleProxy, cleanupProxy
- handleEvent, handleRelayInfo, relayInfoCache
- reconnect handler registry
- FIX #2: on WebSocket open after reconnect, call registered handlers (dm.js re-sends subs)
- FIX #3: handleEvent publishes to writeRelays only, not all pool connections
- FIX #7: sanitizeFilter drops entire filter object if required array field becomes empty after sanitization
- FIX #11: handle AUTH challenge — sign with secretKey, send AUTH event back

pool.js exports `onReconnect(url, fn)`. dm.js registers its re-sub handler.

### dm.js (~180 lines)
- Uses mux() from chan.js for crypto request/response
- decryptNip04, encryptNip04, decryptNip44, encryptNip44 wrappers
- processIncomingDM, processNip04DM, processNip17DM
- sendDM, sendNip04DM, sendNip17DM
- handleDMSub, dmSubIds
- FIX #8: processNip17DM logs errors via console.warn instead of empty catch
- Registers reconnect handler with pool.js to re-send DM subs

### sw.js (~100 lines, down from 1139)
- imports from ./chan.js, ./crypto.js, ./db.js, ./pool.js, ./dm.js
- install/activate/fetch lifecycle handlers
- message dispatch switch (thin — delegates to modules)
- broadcastIdentity
- module-level state: secretKey, secretKeyHex, myPubkey, writeRelays
- subs map, handleReq, handleClose, pushToMatchingSubs, matchesFilter

## Phase 2: Remove Preact, go vanilla JS

Replace Preact+hooks+htm with plain DOM rendering + channel-driven message handling.

### Architecture
- Global `state` object (same shape as current useState initial value)
- `dispatch(action)` calls reducer, updates state, calls `render()`
- `render()` reads `state.activeTab` and calls the matching view function
- View functions build DOM via innerHTML or document.createElement
- Event delegation on `#app` for clicks, inputs, submits, keydowns
- SW messages flow through chan/mux — read state at consumption time, never captured

### style.css (~190 lines)
- Extract the `<style>` block verbatim. No changes to CSS.

### helpers.js (~140 lines)
- send()
- handleCryptoRequest() (extension mode crypto bridge)
- deriveKeyPBKDF2, encryptNsec, decryptNsec, decodeNsec, pubkeyFromSecret
- shortId, relativeTime, parseProfile, parseContent, profileRelays
- PROFILE_RELAYS, DEFAULT_RELAYS
- toBase64 chunked encoder

Imports: nip19Decode from nostr-tools, schnorr + bytesToHex from noble.

### state.js (~180 lines)
- reducer function (same logic as current, just cleaner)
- initialState factory
- dispatch() — runs reducer, stores result, triggers render

### views.js (~550 lines)
All view rendering functions:
- renderSidebar, renderFeed, renderNote, renderNoteInner, renderRichContent
- renderCompose, renderDMView, renderDMList, renderDMChat
- renderThread, buildThread, renderHashtagFeed, renderRelayFeed
- renderSettings, renderProfile, renderLogin, renderPasswordPrompt
- renderSnackbar, renderUpdateSnackbar, renderLightbox, renderSmeshLoader
- renderEmbeddedNote

Each function takes state (or relevant slice) and returns HTML string. `render()` in state.js sets innerHTML on the appropriate container, then binds any interactive elements.

For hot-path updates (new DM bubble, new feed note), use targeted DOM insertion instead of full re-render:
```js
// append DM bubble without re-rendering the entire chat
function appendDMBubble(dm) {
  const container = document.querySelector('.dm-messages')
  if (!container) return
  const el = document.createElement('div')
  el.innerHTML = renderDMBubble(dm)
  container.querySelector('.dm-end-marker')?.before(el.firstChild)
  el.firstChild?.scrollIntoView({ behavior: 'smooth' })
}
```

### app.js (~150 lines)
- boot(): register SW, setup message listeners, initial render
- signViaSW() using mux — linear async, no callback maps
- SW message router: feeds chan/mux, calls dispatch for state mutations
- Async listener loops for continuous streams (DM_RECEIVED, EVENT, etc.)
- Event delegation setup for #app
- FIX #5: DM_HISTORY reads state.activeDM at dispatch time
- FIX #6: RELAY_INFO dispatched to reducer which has current state
- FIX #10: no closures capture state — listeners read it when they wake
- FIX #15: both logout paths clear loginMode

### index.html (~10 lines)
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>smesh</title>
<link rel="icon" href="./favicon.ico" sizes="48x48" />
<link rel="icon" href="./favicon.png" sizes="256x256" type="image/png" />
<link rel="icon" href="./favicon-96x96.png" sizes="96x96" type="image/png" />
<link rel="apple-touch-icon" href="./apple-touch-icon.png" />
<link rel="stylesheet" href="./style.css" />
</head>
<body>
<div id="app"><div class="loading">loading...</div></div>
<script type="module" src="./app.js"></script>
</body>
</html>
```

## Phase 3: Deploy and verify

- Bump CACHE_NAME to v52
- Build Go binary (embeds new files automatically via `//go:embed smesh2`)
- Deploy to relay.orly.dev
- Deploy static files to next.smesh.mleku.dev
- Verify: login (both modes), feed, DMs (send + receive live), threads, hashtags, settings, reconnect behavior

## Bug fix summary

| # | Bug | Fix location | Fix |
|---|-----|-------------|-----|
| 1 | Spread overflow on large payloads | crypto.js | Chunked base64 encoder |
| 2 | DM subs die on WS reconnect | pool.js + dm.js | onReconnect callback, re-send subs |
| 3 | Publish to all relays not writeRelays | pool.js handleEvent | Use writeRelays array |
| 5 | DM_HISTORY stale closure | app.js | No closure — reads state at await |
| 6 | RELAY_INFO stale closure | app.js + state.js | Handled in reducer |
| 7 | sanitizeFilter empty required fields | pool.js | Drop filter if required array empty |
| 8 | processNip17DM swallows errors | dm.js | console.warn the error |
| 10 | useEffect stale closures | app.js | No useEffect exists — channel listeners |
| 11 | No NIP-42 AUTH | pool.js | Sign AUTH challenge, send AUTH event |
| 15 | Settings logout inconsistent | views.js | Clear loginMode in both logout paths |

## What's NOT changing

- IndexedDB schema (v2, events + dms stores)
- SW <-> UI postMessage protocol (all message types stay the same)
- CSS (moved to file, no visual changes)
- nostr-tools/nip19 import (still needed for bech32 decode, nothing else)
- All NIP-04/NIP-44 crypto logic (just relocated)
- Relay connection pool behavior (better reconnect + auth handling)

## File inventory

Before: 2 files (sw.js 1139 lines, index.html 1925 lines) = 3064 lines
After: 12 files, estimated ~1880 lines total

```
app/smesh2/
  index.html    ~10 lines    (shell)
  style.css     ~190 lines   (extracted verbatim)
  chan.js        ~30 lines    (channel primitives)
  app.js        ~150 lines   (boot, message routing, event delegation)
  state.js      ~180 lines   (reducer, dispatch, initial state)
  views.js      ~550 lines   (all view render functions)
  helpers.js    ~140 lines   (nostr utils, crypto helpers, content parser)
  sw.js         ~100 lines   (lifecycle, dispatch, identity broadcast)
  crypto.js     ~170 lines   (NIP-04, NIP-44, signing)
  db.js         ~140 lines   (IndexedDB operations)
  pool.js       ~220 lines   (WebSocket pool, relay messaging, AUTH, reconnect)
  dm.js         ~180 lines   (DM processing, sending, subscriptions)
```
