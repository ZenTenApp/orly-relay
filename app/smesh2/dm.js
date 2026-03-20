// dm.js — DM processing, sending, subscriptions

import { mux } from './chan.js'
import {
  nip04Encrypt, nip04Decrypt,
  nip44ConversationKey, nip44Encrypt, nip44Decrypt,
  signEvent, giftWrap, randomizeTimestamp, bytesToHex, schnorr,
} from './crypto.js'
import { saveDM, dmDedupId, saveEvent } from './db.js'
import { sendToRelay, pool, onReconnect } from './pool.js'

// ─── module state (set by sw.js) ────────────────────────────────────

let _secretKeyHex = null
let _secretKey = null
let _myPubkey = null

export function setDMState({ secretKeyHex, secretKey, myPubkey }) {
  if (secretKeyHex !== undefined) _secretKeyHex = secretKeyHex
  if (secretKey !== undefined) _secretKey = secretKey
  if (myPubkey !== undefined) _myPubkey = myPubkey
}

// ─── extension mode crypto bridge via mux ───────────────────────────

const cryptoMux = mux()
let cryptoRequestId = 0

export function handleCryptoResult(requestId, result, error) {
  cryptoMux.send(requestId, { result, error })
}

async function broadcastToClients(msg) {
  const clients = await self.clients.matchAll()
  for (const client of clients) client.postMessage(msg)
}

async function requestCrypto(type, pubkey, text) {
  const id = ++cryptoRequestId
  broadcastToClients([type, id, pubkey, text])
  const resp = await cryptoMux.recv(id)
  if (resp.error) throw new Error(resp.error)
  return resp.result
}

// ─── encrypt/decrypt wrappers ───────────────────────────────────────

async function decryptNip04(pubkey, ciphertext) {
  if (_secretKeyHex) return nip04Decrypt(_secretKeyHex, pubkey, ciphertext)
  return requestCrypto('DECRYPT_NIP04', pubkey, ciphertext)
}

async function encryptNip04(pubkey, plaintext) {
  if (_secretKeyHex) return nip04Encrypt(_secretKeyHex, pubkey, plaintext)
  return requestCrypto('ENCRYPT_NIP04', pubkey, plaintext)
}

async function decryptNip44(pubkey, ciphertext) {
  if (_secretKeyHex) {
    const ck = nip44ConversationKey(_secretKeyHex, pubkey)
    return nip44Decrypt(ciphertext, ck)
  }
  return requestCrypto('DECRYPT_NIP44', pubkey, ciphertext)
}

async function encryptNip44(pubkey, plaintext) {
  if (_secretKeyHex) {
    const ck = nip44ConversationKey(_secretKeyHex, pubkey)
    return nip44Encrypt(plaintext, ck)
  }
  return requestCrypto('ENCRYPT_NIP44', pubkey, plaintext)
}

// ─── incoming DM processing ─────────────────────────────────────────

export async function processIncomingDM(event) {
  if (!_myPubkey) return
  if (event.kind === 4) return processNip04DM(event)
  if (event.kind === 1059) return processNip17DM(event)
}

async function processNip04DM(event) {
  const pTag = event.tags?.find((t) => t[0] === 'p')
  if (!pTag) return
  const recipient = pTag[1]
  const isMine = event.pubkey === _myPubkey
  const isForMe = recipient === _myPubkey
  if (!isMine && !isForMe) return

  const peer = isMine ? recipient : event.pubkey

  try {
    const plaintext = await decryptNip04(peer, event.content)
    const dm = {
      id: dmDedupId(peer, plaintext, event.created_at),
      peer,
      from: event.pubkey,
      content: plaintext,
      created_at: event.created_at,
      protocol: 'nip04',
      eventId: event.id,
    }
    const result = await saveDM(dm)
    if (result !== 'duplicate') {
      broadcastToClients(['DM_RECEIVED', dm])
    }
  } catch (err) {
    console.warn('nip04 decrypt fail:', err.message)
  }
}

async function processNip17DM(event) {
  try {
    const sealJson = await decryptNip44(event.pubkey, event.content)
    const seal = JSON.parse(sealJson)
    if (seal.kind !== 13) return

    const innerJson = await decryptNip44(seal.pubkey, seal.content)
    const inner = JSON.parse(innerJson)
    if (inner.kind !== 14) return

    const senderPub = seal.pubkey
    const pTag = inner.tags?.find((t) => t[0] === 'p')
    const recipient = pTag?.[1]
    const isMine = senderPub === _myPubkey
    const peer = isMine ? (recipient || '') : senderPub

    if (!peer) return

    const dm = {
      id: dmDedupId(peer, inner.content, inner.created_at || event.created_at),
      peer,
      from: senderPub,
      content: inner.content,
      created_at: inner.created_at || event.created_at,
      protocol: 'nip17',
      eventId: event.id,
    }
    const result = await saveDM(dm)
    if (result !== 'duplicate') {
      broadcastToClients(['DM_RECEIVED', dm])
    }
  } catch (err) {
    // FIX #8: log instead of swallowing
    console.warn('nip17 unwrap fail:', err.message)
  }
}

// ─── DM sending ─────────────────────────────────────────────────────

export async function sendDM(clientId, recipientPubkey, content, relayUrls) {
  if (!_myPubkey) return
  const errors = []

  if (_secretKeyHex) {
    try { await sendNip04DM(recipientPubkey, content, relayUrls) }
    catch (err) { errors.push('nip04: ' + err.message) }
    try { await sendNip17DM(recipientPubkey, content, relayUrls) }
    catch (err) { errors.push('nip17: ' + err.message) }
  } else {
    const client = await self.clients.get(clientId)
    if (client) {
      client.postMessage(['DM_SEND_VIA_EXT', recipientPubkey, content, relayUrls])
      return
    }
  }

  const now = Math.floor(Date.now() / 1000)
  const dm = {
    id: dmDedupId(recipientPubkey, content, now),
    peer: recipientPubkey,
    from: _myPubkey,
    content,
    created_at: now,
    protocol: _secretKeyHex ? 'nip17' : 'nip04',
    eventId: '',
  }
  await saveDM(dm)

  const client = await self.clients.get(clientId)
  if (client) {
    if (errors.length) {
      client.postMessage(['DM_SENT', recipientPubkey, false, errors.join('; ')])
    } else {
      client.postMessage(['DM_SENT', recipientPubkey, true, ''])
    }
  }
  broadcastToClients(['DM_RECEIVED', dm])
}

async function sendNip04DM(recipientPubkey, content, relayUrls) {
  const ciphertext = await encryptNip04(recipientPubkey, content)
  const ev = signEvent({
    kind: 4,
    content: ciphertext,
    tags: [['p', recipientPubkey]],
    created_at: Math.floor(Date.now() / 1000),
    pubkey: _myPubkey,
  }, _secretKey)
  await saveEvent(ev)
  for (const url of relayUrls) sendToRelay(url, ['EVENT', ev])
}

async function sendNip17DM(recipientPubkey, content, relayUrls) {
  const now = Math.floor(Date.now() / 1000)

  const inner = signEvent({
    kind: 14,
    content,
    tags: [['p', recipientPubkey]],
    created_at: now,
    pubkey: _myPubkey,
  }, _secretKey)
  const innerJson = JSON.stringify(inner)

  // recipient copy
  const recipientSealContent = await encryptNip44(recipientPubkey, innerJson)
  const recipientSeal = signEvent({
    kind: 13,
    content: recipientSealContent,
    tags: [],
    created_at: randomizeTimestamp(now),
    pubkey: _myPubkey,
  }, _secretKey)
  const recipientWrap = await giftWrap(recipientSeal, recipientPubkey, now)
  for (const url of relayUrls) sendToRelay(url, ['EVENT', recipientWrap])

  // sender (self) copy
  const senderSealContent = await encryptNip44(_myPubkey, innerJson)
  const senderSeal = signEvent({
    kind: 13,
    content: senderSealContent,
    tags: [],
    created_at: randomizeTimestamp(now),
    pubkey: _myPubkey,
  }, _secretKey)
  const senderWrap = await giftWrap(senderSeal, _myPubkey, now)
  for (const url of relayUrls) sendToRelay(url, ['EVENT', senderWrap])
}

// ─── DM subscriptions ───────────────────────────────────────────────

const dmSubIds = new Set()
let _dmRelayUrls = []

export async function handleDMSub(clientId, relayUrls) {
  if (!_myPubkey || !relayUrls?.length) return
  _dmRelayUrls = relayUrls

  // close existing
  for (const rSubId of dmSubIds) {
    for (const [url, conn] of pool) {
      if (conn.ws?.readyState === WebSocket.OPEN) {
        conn.ws.send(JSON.stringify(['CLOSE', rSubId]))
      }
    }
  }
  dmSubIds.clear()

  openDMSubs(relayUrls)
}

function openDMSubs(relayUrls) {
  for (const url of relayUrls) {
    const suffix = url.replace(/\W/g, '').slice(-8)
    const id1 = 'dm4in_' + suffix
    const id2 = 'dm4out_' + suffix
    const id3 = 'dm17_' + suffix
    dmSubIds.add(id1)
    dmSubIds.add(id2)
    dmSubIds.add(id3)

    sendToRelay(url, ['REQ', id1, { kinds: [4], '#p': [_myPubkey], limit: 100 }])
    sendToRelay(url, ['REQ', id2, { kinds: [4], authors: [_myPubkey], limit: 100 }])
    sendToRelay(url, ['REQ', id3, { kinds: [1059], '#p': [_myPubkey], limit: 100 }])
  }
}

// FIX #2: re-send DM subs when a relay reconnects
onReconnect((url) => {
  if (!_myPubkey || !_dmRelayUrls.includes(url)) return
  // small delay to let the connection stabilize
  setTimeout(() => openDMSubs([url]), 500)
})

// ─── extension mode DM result ───────────────────────────────────────

export async function handleExtDMResult(clientId, peer, content, success, errMsg) {
  if (success && _myPubkey) {
    const now = Math.floor(Date.now() / 1000)
    const dm = {
      id: dmDedupId(peer, content, now),
      peer,
      from: _myPubkey,
      content,
      created_at: now,
      protocol: 'nip04',
      eventId: '',
    }
    await saveDM(dm)
    broadcastToClients(['DM_RECEIVED', dm])
  }
  const client = await self.clients.get(clientId)
  if (client) client.postMessage(['DM_SENT', peer, success, errMsg || ''])
}
