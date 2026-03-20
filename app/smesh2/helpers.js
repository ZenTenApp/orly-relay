// helpers.js — nostr utils, crypto helpers, content parser

import { decode as nip19Decode } from 'https://esm.sh/nostr-tools@2.17.0/nip19'
import { schnorr } from 'https://esm.sh/@noble/curves@1.8.2/secp256k1'
import { bytesToHex } from 'https://esm.sh/@noble/hashes@1.7.2/utils'

// ─── SW communication ───────────────────────────────────────────────

export function send(msg) {
  if (navigator.serviceWorker.controller) {
    navigator.serviceWorker.controller.postMessage(msg)
  }
}

// ─── extension mode crypto bridge ───────────────────────────────────

export async function handleCryptoRequest(type, reqId, pubkey, text) {
  if (!window.nostr) {
    send(['CRYPTO_RESULT', reqId, null, 'no extension'])
    return
  }
  try {
    let result
    if (type === 'DECRYPT_NIP04') result = await window.nostr.nip04.decrypt(pubkey, text)
    else if (type === 'ENCRYPT_NIP04') result = await window.nostr.nip04.encrypt(pubkey, text)
    else if (type === 'DECRYPT_NIP44') result = await window.nostr.nip44.decrypt(pubkey, text)
    else if (type === 'ENCRYPT_NIP44') result = await window.nostr.nip44.encrypt(pubkey, text)
    send(['CRYPTO_RESULT', reqId, result, null])
  } catch (err) {
    send(['CRYPTO_RESULT', reqId, null, err.message])
  }
}

// ─── nsec encryption (PBKDF2 + AES-256-GCM) ────────────────────────

async function deriveKeyPBKDF2(password, salt) {
  const enc = new TextEncoder()
  const keyMaterial = await crypto.subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey'])
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: 600000, hash: 'SHA-256' },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  )
}

export async function encryptNsec(nsec, password) {
  const salt = crypto.getRandomValues(new Uint8Array(32))
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const key = await deriveKeyPBKDF2(password, salt)
  const encrypted = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, new TextEncoder().encode(nsec))
  const combined = new Uint8Array(salt.length + iv.length + encrypted.byteLength)
  combined.set(salt, 0)
  combined.set(iv, salt.length)
  combined.set(new Uint8Array(encrypted), salt.length + iv.length)
  let binary = ''
  for (let i = 0; i < combined.length; i++) binary += String.fromCharCode(combined[i])
  return btoa(binary)
}

export async function decryptNsec(encryptedData, password) {
  const combined = new Uint8Array(atob(encryptedData).split('').map((c) => c.charCodeAt(0)))
  const salt = combined.slice(0, 32)
  const iv = combined.slice(32, 44)
  const ciphertext = combined.slice(44)
  const key = await deriveKeyPBKDF2(password, salt)
  const decrypted = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ciphertext)
  return new TextDecoder().decode(decrypted)
}

export function decodeNsec(nsec) {
  const decoded = nip19Decode(nsec)
  if (decoded.type !== 'nsec') throw new Error('not an nsec')
  return decoded.data
}

export function pubkeyFromSecret(secretKeyBytes) {
  return bytesToHex(schnorr.getPublicKey(secretKeyBytes))
}

// ─── nostr helpers ──────────────────────────────────────────────────

export const PROFILE_RELAYS = [
  'wss://relay.damus.io', 'wss://relay.nostr.net', 'wss://nos.lol',
  'wss://purplepag.es', 'wss://relay.snort.social', 'wss://relay.primal.net',
  'wss://offchain.pub', 'wss://nostr.wine', 'wss://relay.noswhere.com',
  'wss://nostr-pub.wellorder.net',
]

export const DEFAULT_RELAYS = ['wss://relay.orly.dev', 'wss://relay.damus.io', 'wss://nos.lol', 'wss://relay.nostr.band']

export function profileRelays(userRelays) {
  return [...new Set([...userRelays.slice(0, 3), ...PROFILE_RELAYS])]
}

export function shortId(hex) {
  if (!hex) return '?'
  return hex.slice(0, 8) + '...' + hex.slice(-4)
}

export function relativeTime(ts) {
  const diff = Math.floor(Date.now() / 1000) - ts
  if (diff < 60) return 'now'
  if (diff < 3600) return Math.floor(diff / 60) + 'm'
  if (diff < 86400) return Math.floor(diff / 3600) + 'h'
  return Math.floor(diff / 86400) + 'd'
}

export function parseProfile(event) {
  try { return JSON.parse(event.content) } catch { return {} }
}

// ─── content parser ─────────────────────────────────────────────────

const IMAGE_RE = /\.(jpe?g|png|gif|webp|svg)(\?[^\s]*)?$/i
const VIDEO_RE = /\.(mp4|webm|mov)(\?[^\s]*)?$/i

export function parseContent(text) {
  if (!text) return [{ t: 'text', v: '' }]
  const TOKEN = /(https?:\/\/[^\s<>"]+)|(nostr:(npub1|note1|nevent1|nprofile1|naddr1)[a-z0-9]+)/gi
  const segments = []
  let last = 0, m
  while ((m = TOKEN.exec(text)) !== null) {
    if (m.index > last) segments.push({ t: 'text', v: text.slice(last, m.index) })
    if (m[1]) {
      const url = m[1].replace(/[.,;:!?)]+$/, '')
      segments.push(IMAGE_RE.test(url) ? { t: 'image', url } : VIDEO_RE.test(url) ? { t: 'video', url } : { t: 'link', url })
      last = m.index + url.length
      TOKEN.lastIndex = last
    } else if (m[2]) {
      const raw = m[2], bech32 = raw.slice(6)
      try {
        const d = nip19Decode(bech32)
        if (d.type === 'npub') segments.push({ t: 'mention', pubkey: d.data })
        else if (d.type === 'nprofile') segments.push({ t: 'mention', pubkey: d.data.pubkey, relays: d.data.relays })
        else if (d.type === 'note') segments.push({ t: 'noteref', id: d.data })
        else if (d.type === 'nevent') segments.push({ t: 'noteref', id: d.data.id, relays: d.data.relays })
        else if (d.type === 'naddr') segments.push({ t: 'addrref', kind: d.data.kind, pubkey: d.data.pubkey, d: d.data.identifier, relays: d.data.relays })
        else segments.push({ t: 'text', v: raw })
      } catch { segments.push({ t: 'text', v: raw }) }
      last = m.index + raw.length
      TOKEN.lastIndex = last
    }
  }
  if (last < text.length) segments.push({ t: 'text', v: text.slice(last) })
  return segments
}

// ─── HTML escaping ──────────────────────────────────────────────────

export function esc(str) {
  if (!str) return ''
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
