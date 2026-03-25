// TinyJS Runtime — secp256k1 Crypto Bridge (WASM) for Marmot SW
// Non-blocking init: starts WASM loading but doesn't await it,
// so SW event handlers get registered promptly.

import { Slice } from './builtin.mjs';

let _secp = null;
let _loading = null;

// init starts WASM loading but returns immediately.
// The SW can register handlers while WASM loads in background.
export async function init() {
  _loading = loadWasm();
}

async function loadWasm() {
  try {
    const base = new URL('./', import.meta.url);
    const wasmPath = new URL('../$wasm/secp256k1.wasm', base).href;

    let ATU8, ATU32;
    const imports = {
      a: {
        a() { throw new Error('secp256k1: abort'); },
        f(dst, src, n) { ATU8.copyWithin(dst, src, src + n); },
        d() { throw new Error('secp256k1: out of memory'); },
        e() { return 52; },
        c() { return 70; },
        b(fd, iov, iovcnt, pnum) {
          let written = 0;
          for (let i = 0; i < iovcnt; i++) {
            written += ATU32[(iov + 4) >> 2];
            iov += 8;
          }
          ATU32[pnum >> 2] = written;
          return 0;
        },
      },
    };

    const resp = fetch(wasmPath);
    const { instance } = await WebAssembly.instantiateStreaming(resp, imports);
    const X = instance.exports;

    const mem = X['g'];
    ATU8 = new Uint8Array(mem.buffer);
    ATU32 = new Uint32Array(mem.buffer);
    X['h']();

    const malloc = X['i'];
    const pSK = malloc(32);
    const pEnt = malloc(32);
    const pMsg = malloc(32);
    const pPub = malloc(32);
    const pSig = malloc(64);
    const pKeypair = malloc(96);
    const pXonly = malloc(64);
    const ctx = X['o'](513 | 257);

    function withKeypair(sk, fn) {
      try {
        ATU8.set(sk, pSK);
        X['r'](ctx, pKeypair, pSK);
        return fn();
      } finally {
        ATU8.fill(0, pSK, pSK + 32);
        ATU8.fill(0, pKeypair, pKeypair + 96);
      }
    }

    _secp = {
      getPublicKey(sk) {
        if (1 !== withKeypair(sk, () => X['s'](ctx, pXonly, null, pKeypair))) {
          throw new Error('invalid secret key');
        }
        X['q'](ctx, pPub, pXonly);
        return ATU8.slice(pPub, pPub + 32);
      },
      sign(sk, msgHash, entropy) {
        ATU8.set(msgHash, pMsg);
        if (entropy) {
          ATU8.set(entropy, pEnt);
        } else {
          crypto.getRandomValues(ATU8.subarray(pEnt, pEnt + 32));
        }
        if (1 !== withKeypair(sk, () => X['t'](ctx, pSig, pMsg, pKeypair, pEnt))) {
          throw new Error('sign failed');
        }
        return ATU8.slice(pSig, pSig + 64);
      },
      verify(sig, msgHash, pk) {
        ATU8.set(sig, pSig);
        ATU8.set(msgHash, pMsg);
        ATU8.set(pk, pPub);
        if (1 !== X['p'](ctx, pXonly, pPub)) return false;
        return 1 === X['u'](ctx, pSig, pMsg, 32, pXonly);
      },
    };
    console.log('marmot-sw: crypto WASM ready');
  } catch (e) {
    console.error('marmot-sw: crypto init failed:', e);
  }
}

// ensure waits for WASM to finish loading (if still in progress).
async function ensure() {
  if (_loading) {
    await _loading;
    _loading = null;
  }
}

function sliceToU8(s) {
  if (s instanceof Uint8Array) return s;
  const u = new Uint8Array(s.$length);
  for (let i = 0; i < s.$length; i++) {
    u[i] = s.$array[s.$offset + i];
  }
  return u;
}

function u8ToSlice(u8) {
  if (u8 === null || u8 === undefined) return null;
  const arr = new Array(u8.length);
  for (let i = 0; i < u8.length; i++) arr[i] = u8[i];
  return new Slice(arr, 0, u8.length, u8.length);
}

export function PubKeyFromSecKey(seckey) {
  if (!_secp) return null;
  const result = _secp.getPublicKey(sliceToU8(seckey));
  return u8ToSlice(result);
}

export function SignSchnorr(seckey, msg, auxRand) {
  if (!_secp) return null;
  const result = _secp.sign(sliceToU8(seckey), sliceToU8(msg), sliceToU8(auxRand));
  if (result === null || result === undefined) return null;
  return u8ToSlice(result);
}

export function VerifySchnorr(pubkey, msg, sig) {
  if (!_secp) return false;
  return _secp.verify(sliceToU8(pubkey), sliceToU8(msg), sliceToU8(sig));
}
