# Smesh Signer

NIP-07 browser extension for Nostr identity management and event signing. Manages multiple identities in an Argon2id-encrypted vault. Exposes `window.nostr` to web applications for signing without revealing private keys.

## What It Does

- **NIP-07**: `getPublicKey()`, `signEvent()`, `getRelays()`
- **NIP-04**: `nip04.encrypt()`, `nip04.decrypt()` (legacy DMs)
- **NIP-44**: `nip44.encrypt()`, `nip44.decrypt()` (modern DMs, Marmot MLS)
- **WebLN**: Lightning payments via NWC (Nostr Wallet Connect)
- **Vault**: Argon2id (256MB, 4 threads, 8 iter) + AES-256-GCM per entry

## Build From Source

### Prerequisites

[Bun](https://bun.sh) (or npm/node 20+).

### Local Build

```sh
cd next/signer
bun install
bun run build:firefox    # dist/firefox/  (unpacked extension)
bun run xpi              # dist/smesh_signer-*.zip
```

### Docker Build

No bun/node required on host:

```sh
cd next/signer
docker build -t smesh-signer-build .
mkdir -p out
docker run --rm -v $(pwd)/out:/out smesh-signer-build
```

Output lands in `out/firefox/` (unpacked) and `out/smesh_signer-*.zip` (packaged).

## Install

### Firefox

**From XPI** (permanent):
1. `about:addons` > gear icon > "Install Add-on From File..."
2. Select `smesh_signer-*.zip`

**From source** (temporary, for development):
1. `about:debugging` > "This Firefox" > "Load Temporary Add-on..."
2. Select any file inside `dist/firefox/`
3. Extension persists until browser restart

### Chrome / Chromium

1. `chrome://extensions` > enable "Developer mode"
2. "Load unpacked" > select `dist/firefox/` directory
3. The MV3 manifest is compatible with both browsers

## Architecture

Three-layer message bridge between web pages and the encrypted vault:

```
Web page                    Extension
--------                    ---------
window.nostr.signEvent()
  |
  v
[Injected Script]          smesh-signer-extension.js
  | window.postMessage()     (runs in page context)
  v
[Content Script]           smesh-signer-content-script.js
  | browser.runtime.sendMessage()
  v
[Background SW]            background.js
  | vault decrypt + BIP340 sign
  v
  (response bubbles back through same chain)
```

### Message Flow Detail

1. **Page** calls `window.nostr.signEvent(event)`
2. **Injected script** wraps call as `{type: "smesh-signer-request", method, params}`, posts to window
3. **Content script** catches window message, forwards via `browser.runtime.sendMessage()`
4. **Background** receives, checks permission policy, performs crypto, returns result
5. Reverse path: background -> content script -> injected script -> Promise resolves

### Permission Model

Per-identity, per-host, per-method granularity:
- `allow` / `deny` policies stored in vault
- Kind-specific policies for `signEvent` (e.g. allow kind 1 but prompt for kind 30023)
- **Reckless mode**: auto-approve everything (global toggle)
- **Whitelisted hosts**: auto-approve from specific origins

### Vault Encryption

```
password
  |
  v  Argon2id (256MB mem, 4 threads, 8 iterations, ~3s)
  |
  v  32-byte derived key
  |
  v  AES-256-GCM (random 12-byte IV per entry)
  |
  v  encrypted vault blob  -->  browser.storage.local
```

Key cached in `browser.storage.session` while vault is unlocked. Cleared on lock or browser close.

## Crypto Proxy Integration

When smesh detects the signer extension, it delegates all crypto to the extension instead of requiring an nsec. The Marmot SW sends crypto requests through the bus:

```
Marmot SW  -->  Bus WS  -->  Shell SW  -->  Page  -->  window.nostr.nip44Encrypt()
                                                          |
                                                    Signer Extension
                                                          |
                                                    (vault decrypt + ECDH + encrypt)
```

This is the `ProxyCrypto` path in `pkg/nostr/protocol/marmot/crypto.go`. The alternative `LocalCrypto` path holds the key directly (bridge/bridgebot mode).

## Development

Watch mode for iterative development:

```sh
bun run watch:firefox
```

Rebuilds on source change. Reload extension in `about:debugging` after each rebuild.

Debug logging: uncomment `console.log` inside `debug()` at `src/smesh-signer-extension.ts:248`.

## Project Structure

```
next/signer/
  src/
    background.ts               Background SW (vault, crypto, dispatch)
    smesh-signer-extension.ts   Injected into page (window.nostr API)
    smesh-signer-content-script.ts  Content script (message bridge)
    prompt.ts                   Permission prompt popup
    options.ts                  Extension options page
    unlock.ts                   Vault unlock page
    app/                        Angular popup UI (identity management)
  public/
    manifest.json               MV3 manifest (Firefox + Chrome compatible)
    *.html                      Popup/options/prompt/unlock pages
  common/                       Shared Angular library (@common)
  custom-webpack.config.ts      Extra webpack entries (background, content script, etc.)
```
