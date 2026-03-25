// Content script — runs in ISOLATED world at document_start.
// Injects inject.mjs into the MAIN world with the extension URL so it can
// load the secp256k1 WASM for BIP-340 signing.
'use strict';

// Load the main inject module in page context.
// inject.mjs uses import.meta.url to self-locate its WASM files.
const mod = document.createElement('script');
mod.type = 'module';
mod.src = chrome.runtime.getURL('inject.mjs');
(document.head || document.documentElement).appendChild(mod);
