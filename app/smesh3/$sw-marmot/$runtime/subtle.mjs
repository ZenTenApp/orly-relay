// TinyJS Runtime — SubtleCrypto bridge (AES-CBC + random bytes)
// Service worker version — identical to main app, SubtleCrypto available in SW scope.

import { Slice } from './builtin.mjs';

function sliceToU8(s) {
  if (s instanceof Uint8Array) return s;
  const u = new Uint8Array(s.$length);
  for (let i = 0; i < s.$length; i++) u[i] = s.$array[s.$offset + i];
  return u;
}

function u8ToSlice(u8) {
  const arr = new Array(u8.length);
  for (let i = 0; i < u8.length; i++) arr[i] = u8[i];
  return new Slice(arr, 0, u8.length, u8.length);
}

export function RandomBytes(dst) {
  const u8 = new Uint8Array(dst.$length);
  crypto.getRandomValues(u8);
  for (let i = 0; i < dst.$length; i++) dst.$array[dst.$offset + i] = u8[i];
}

export function AESCBCEncrypt(key, iv, plaintext, fn) {
  const k = sliceToU8(key), v = sliceToU8(iv), pt = sliceToU8(plaintext);
  crypto.subtle.importKey('raw', k, { name: 'AES-CBC' }, false, ['encrypt'])
    .then(ck => crypto.subtle.encrypt({ name: 'AES-CBC', iv: v }, ck, pt))
    .then(buf => fn(u8ToSlice(new Uint8Array(buf))));
}

export function AESCBCDecrypt(key, iv, ciphertext, fn) {
  const k = sliceToU8(key), v = sliceToU8(iv), ct = sliceToU8(ciphertext);
  crypto.subtle.importKey('raw', k, { name: 'AES-CBC' }, false, ['decrypt'])
    .then(ck => crypto.subtle.decrypt({ name: 'AES-CBC', iv: v }, ck, ct))
    .then(buf => fn(u8ToSlice(new Uint8Array(buf))));
}
