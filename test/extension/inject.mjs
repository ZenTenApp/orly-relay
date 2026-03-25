// Test NIP-07 signer — injected into MAIN world.
// Implements window.nostr with hardcoded test keys.
// Uses secp256k1 WASM for BIP-340 Schnorr signing.

const TEST_SK = '328615b6c92aa6527fc175a67670222daabc69fa2b84c2ded5f6907f78f2b0f8';
const TEST_PK = '5dbeb1d7e84d0a4fb3fac47868438dc9135b35f25e27ae933e306e3584bf69a8';

const extUrl = new URL('./', import.meta.url).href;

function hexToBytes(hex) {
  const b = new Uint8Array(hex.length / 2);
  for (let i = 0; i < b.length; i++) b[i] = parseInt(hex.substr(i * 2, 2), 16);
  return b;
}

function bytesToHex(bytes) {
  return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
}

const skBytes = hexToBytes(TEST_SK);

let secp = null;

async function getSecp() {
  if (secp) return secp;
  const { initSecp256k1 } = await import(extUrl + 'secp256k1.mjs');
  secp = await initSecp256k1(extUrl + 'secp256k1.wasm');
  return secp;
}

// NIP-01 canonical serialization for event ID computation.
function serializeEvent(ev) {
  return JSON.stringify([
    0,
    ev.pubkey,
    ev.created_at,
    ev.kind,
    ev.tags,
    ev.content,
  ]);
}

async function sha256hex(str) {
  const data = new TextEncoder().encode(str);
  const hash = await crypto.subtle.digest('SHA-256', data);
  return bytesToHex(new Uint8Array(hash));
}

window.nostr = {
  async getPublicKey() {
    return TEST_PK;
  },

  async signEvent(event) {
    const s = await getSecp();
    // Compute event ID per NIP-01.
    event.pubkey = TEST_PK;
    const id = await sha256hex(serializeEvent(event));
    event.id = id;
    // BIP-340 Schnorr sign the ID hash.
    const sig = s.sign(skBytes, hexToBytes(id));
    event.sig = bytesToHex(sig);
    return event;
  },

  nip04: {
    async encrypt(pubkey, plaintext) {
      return Promise.reject(new Error('nip04 not implemented in test signer'));
    },
    async decrypt(pubkey, ciphertext) {
      return Promise.reject(new Error('nip04 not implemented in test signer'));
    },
  },

  nip44: {
    async encrypt(pubkey, plaintext) {
      // NIP-44 requires ECDH which the secp256k1 WASM doesn't expose.
      // For tests that need NIP-44, use nsec auth mode instead.
      return Promise.reject(new Error('nip44 not implemented in test signer'));
    },
    async decrypt(pubkey, ciphertext) {
      return Promise.reject(new Error('nip44 not implemented in test signer'));
    },
  },
};

console.log('[test-signer] window.nostr installed (pk: ' + TEST_PK.slice(0, 8) + '...)');
