// crypto.js — signing + NIP-04 + NIP-44 encryption
// all noble-* imports centralized here

import { schnorr, secp256k1 } from 'https://esm.sh/@noble/curves@1.8.2/secp256k1'
import { sha256 } from 'https://esm.sh/@noble/hashes@1.7.2/sha256'
import { extract as hkdfExtract, expand as hkdfExpand } from 'https://esm.sh/@noble/hashes@1.7.2/hkdf'
import { hmac } from 'https://esm.sh/@noble/hashes@1.7.2/hmac'
import { bytesToHex, hexToBytes, concatBytes, utf8ToBytes } from 'https://esm.sh/@noble/hashes@1.7.2/utils'
import { chacha20 } from 'https://esm.sh/@noble/ciphers@1.2.1/chacha'

// ─── base64 helpers (no spread overflow) ────────────────────────────

export function toBase64(bytes) {
  let binary = ''
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

export function fromBase64(b64) {
  return Uint8Array.from(atob(b64), c => c.charCodeAt(0))
}

// ─── re-exports for other modules ───────────────────────────────────

export { schnorr, secp256k1, sha256, bytesToHex, hexToBytes, concatBytes }

// ─── signing ────────────────────────────────────────────────────────

export function signEvent(event, secretKey) {
  const serialized = JSON.stringify([
    0, event.pubkey, event.created_at, event.kind, event.tags, event.content
  ])
  const hash = sha256(new TextEncoder().encode(serialized))
  const id = bytesToHex(hash)
  const sig = bytesToHex(schnorr.sign(hash, secretKey))
  return { ...event, id, sig }
}

export function signEventWithKey(event, privkey) {
  return signEvent(event, privkey)
}

export function randomizeTimestamp(baseTime) {
  return baseTime - Math.floor(Math.random() * 2 * 24 * 60 * 60)
}

// ─── NIP-04 (AES-256-CBC via Web Crypto + secp256k1 ECDH) ──────────

function nip04SharedKey(privkeyHex, pubkeyHex) {
  const shared = secp256k1.getSharedSecret(privkeyHex, '02' + pubkeyHex)
  return shared.slice(1, 33)
}

export async function nip04Encrypt(privkeyHex, pubkeyHex, plaintext) {
  const sharedKey = nip04SharedKey(privkeyHex, pubkeyHex)
  const iv = crypto.getRandomValues(new Uint8Array(16))
  const key = await crypto.subtle.importKey('raw', sharedKey, { name: 'AES-CBC' }, false, ['encrypt'])
  const enc = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, key, new TextEncoder().encode(plaintext))
  return toBase64(new Uint8Array(enc)) + '?iv=' + toBase64(iv)
}

export async function nip04Decrypt(privkeyHex, pubkeyHex, ciphertext) {
  const [encB64, ivB64] = ciphertext.split('?iv=')
  const enc = fromBase64(encB64)
  const iv = fromBase64(ivB64)
  const sharedKey = nip04SharedKey(privkeyHex, pubkeyHex)
  const key = await crypto.subtle.importKey('raw', sharedKey, { name: 'AES-CBC' }, false, ['decrypt'])
  const dec = await crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, enc)
  return new TextDecoder().decode(dec)
}

// ─── NIP-44 v2 (ChaCha20 + HMAC-SHA256 + HKDF) ────────────────────

const NIP44_SALT = utf8ToBytes('nip44-v2')

export function nip44ConversationKey(privkeyHex, pubkeyHex) {
  const shared = secp256k1.getSharedSecret(privkeyHex, '02' + pubkeyHex)
  const sharedX = shared.slice(1, 33)
  return hkdfExtract(sha256, sharedX, NIP44_SALT)
}

function nip44MessageKeys(conversationKey, nonce) {
  const keys = hkdfExpand(sha256, conversationKey, nonce, 76)
  return {
    chachaKey: keys.slice(0, 32),
    chaChaNonce: keys.slice(32, 44),
    hmacKey: keys.slice(44, 76),
  }
}

function nip44CalcPadding(len) {
  if (len <= 32) return 32
  const nextPow = 1 << (Math.floor(Math.log2(len - 1)) + 1)
  const chunk = nextPow <= 256 ? 32 : nextPow / 8
  return chunk * (Math.floor((len - 1) / chunk) + 1)
}

function nip44Pad(plaintext) {
  const unpadded = utf8ToBytes(plaintext)
  const len = unpadded.length
  if (len < 1 || len > 65535) throw new Error('invalid plaintext length')
  const paddedLen = nip44CalcPadding(len)
  const out = new Uint8Array(2 + paddedLen)
  out[0] = (len >> 8) & 0xff
  out[1] = len & 0xff
  out.set(unpadded, 2)
  return out
}

function nip44Unpad(padded) {
  const len = (padded[0] << 8) | padded[1]
  if (len < 1 || len > padded.length - 2) throw new Error('invalid padding')
  const plainBytes = padded.slice(2, 2 + len)
  for (let i = 2 + len; i < padded.length; i++) {
    if (padded[i] !== 0) throw new Error('invalid padding: non-zero')
  }
  return new TextDecoder().decode(plainBytes)
}

export function nip44Encrypt(plaintext, conversationKey) {
  const nonce = crypto.getRandomValues(new Uint8Array(32))
  const { chachaKey, chaChaNonce, hmacKey } = nip44MessageKeys(conversationKey, nonce)
  const padded = nip44Pad(plaintext)
  const ciphertext = chacha20(chachaKey, chaChaNonce, padded)
  const mac = hmac(sha256, hmacKey, concatBytes(nonce, ciphertext))
  const payload = concatBytes(new Uint8Array([2]), nonce, ciphertext, mac)
  return toBase64(payload)
}

export function nip44Decrypt(b64payload, conversationKey) {
  const raw = fromBase64(b64payload)
  if (raw.length < 99) throw new Error('payload too short')
  const version = raw[0]
  if (version !== 2) throw new Error('unsupported nip44 version: ' + version)
  const nonce = raw.slice(1, 33)
  const ciphertext = raw.slice(33, raw.length - 32)
  const mac = raw.slice(raw.length - 32)
  const { chachaKey, chaChaNonce, hmacKey } = nip44MessageKeys(conversationKey, nonce)
  const expectedMac = hmac(sha256, hmacKey, concatBytes(nonce, ciphertext))
  let macOk = true
  for (let i = 0; i < 32; i++) { if (mac[i] !== expectedMac[i]) macOk = false }
  if (!macOk) throw new Error('nip44: invalid MAC')
  const padded = chacha20(chachaKey, chaChaNonce, ciphertext)
  return nip44Unpad(padded)
}

// ─── gift-wrap ──────────────────────────────────────────────────────

export async function giftWrap(seal, recipientPubkey, baseTime) {
  const ephemeralPriv = crypto.getRandomValues(new Uint8Array(32))
  const ephemeralPrivHex = bytesToHex(ephemeralPriv)
  const ephemeralPub = bytesToHex(schnorr.getPublicKey(ephemeralPriv))

  const ck = nip44ConversationKey(ephemeralPrivHex, recipientPubkey)
  const wrapContent = nip44Encrypt(JSON.stringify(seal), ck)

  const wrap = {
    kind: 1059,
    content: wrapContent,
    tags: [['p', recipientPubkey]],
    created_at: randomizeTimestamp(baseTime),
    pubkey: ephemeralPub,
  }
  return signEventWithKey(wrap, ephemeralPriv)
}
