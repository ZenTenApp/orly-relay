/**
 * NIP-46 Bunker Signer
 *
 * Implements remote signing via NIP-46 protocol.
 * The signer connects to a bunker WebSocket and
 * requests signing operations.
 */

import { ISigner, TDraftEvent } from '@/types'
import * as utils from '@noble/curves/abstract/utils'
import { secp256k1 } from '@noble/curves/secp256k1'
import { Event, VerifiedEvent, getPublicKey as nGetPublicKey, nip04, finalizeEvent } from 'nostr-tools'

// NIP-46 methods
const NIP46_METHOD = {
  CONNECT: 'connect',
  GET_PUBLIC_KEY: 'get_public_key',
  SIGN_EVENT: 'sign_event',
  NIP04_ENCRYPT: 'nip04_encrypt',
  NIP04_DECRYPT: 'nip04_decrypt',
  NIP44_ENCRYPT: 'nip44_encrypt',
  NIP44_DECRYPT: 'nip44_decrypt',
  PING: 'ping'
} as const

type NIP46Method = (typeof NIP46_METHOD)[keyof typeof NIP46_METHOD]

// NIP-46 request format
interface NIP46Request {
  id: string
  method: NIP46Method
  params: string[]
}

// NIP-46 response format
interface NIP46Response {
  id: string
  result?: string
  error?: string
}

// Pending request tracker
interface PendingRequest {
  resolve: (value: string) => void
  reject: (error: Error) => void
  timeout: ReturnType<typeof setTimeout>
}

/**
 * Generate a random request ID.
 */
function generateRequestId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  return utils.bytesToHex(bytes)
}

/**
 * Parse a bunker URL (bunker://<pubkey>?relay=<url>&secret=<secret>).
 */
export function parseBunkerUrl(url: string): {
  pubkey: string
  relays: string[]
  secret?: string
} {
  if (!url.startsWith('bunker://')) {
    throw new Error('Invalid bunker URL: must start with bunker://')
  }

  const withoutPrefix = url.slice('bunker://'.length)
  const [pubkeyPart, queryPart] = withoutPrefix.split('?')

  if (!pubkeyPart || pubkeyPart.length !== 64) {
    throw new Error('Invalid bunker URL: missing or invalid pubkey')
  }

  const params = new URLSearchParams(queryPart || '')
  const relays = params.getAll('relay')
  const secret = params.get('secret') || undefined

  if (relays.length === 0) {
    throw new Error('Invalid bunker URL: no relay specified')
  }

  return {
    pubkey: pubkeyPart,
    relays,
    secret
  }
}

/**
 * Parse a nostr+connect URL (nostr+connect://<relay-url>?pubkey=<client-pubkey>&secret=<secret>).
 * This is the format that signers (like Amber) scan to connect to a client.
 */
export function parseNostrConnectUrl(url: string): {
  relay: string
  pubkey?: string
  secret?: string
} {
  if (!url.startsWith('nostr+connect://')) {
    throw new Error('Invalid nostr+connect URL: must start with nostr+connect://')
  }

  const withoutPrefix = url.slice('nostr+connect://'.length)
  const [relayPart, queryPart] = withoutPrefix.split('?')

  if (!relayPart) {
    throw new Error('Invalid nostr+connect URL: missing relay')
  }

  const params = new URLSearchParams(queryPart || '')
  const pubkey = params.get('pubkey') || undefined
  const secret = params.get('secret') || undefined

  return {
    relay: relayPart,
    pubkey,
    secret
  }
}

/**
 * Build a nostr+connect URL for signers to scan.
 * @param relay - The relay URL (without ws:// prefix, will be added)
 * @param pubkey - The client's ephemeral pubkey for this session
 * @param secret - Optional secret for the handshake
 */
export function buildNostrConnectUrl(relay: string, pubkey: string, secret?: string): string {
  // Ensure relay URL uses the relay host without protocol
  let relayHost = relay
    .replace('wss://', '')
    .replace('ws://', '')
    .replace('https://', '')
    .replace('http://', '')
    .replace(/\/$/, '')

  const params = new URLSearchParams()
  params.set('pubkey', pubkey)
  if (secret) {
    params.set('secret', secret)
  }
  return `nostr+connect://${relayHost}?${params.toString()}`
}

/**
 * Build a bunker URL from components.
 */
export function buildBunkerUrl(pubkey: string, relays: string[], secret?: string): string {
  const params = new URLSearchParams()
  relays.forEach((relay) => params.append('relay', relay))
  if (secret) {
    params.set('secret', secret)
  }
  return `bunker://${pubkey}?${params.toString()}`
}

export class BunkerSigner implements ISigner {
  private bunkerPubkey: string
  private relayUrls: string[]
  private connectionSecret?: string
  private localPrivkey: Uint8Array
  private localPubkey: string
  private remotePubkey: string | null = null
  private ws: WebSocket | null = null
  private pendingRequests = new Map<string, PendingRequest>()
  private connected = false
  private requestTimeout = 30000 // 30 seconds

  // Whether we're waiting for signer to connect (reverse flow)
  private awaitingConnection = false
  private connectionResolve: ((pubkey: string) => void) | null = null

  /**
   * Create a BunkerSigner.
   * @param bunkerPubkey - The bunker's public key (hex)
   * @param relayUrls - Relay URLs to connect to
   * @param connectionSecret - Optional connection secret for initial handshake
   */
  constructor(bunkerPubkey: string, relayUrls: string[], connectionSecret?: string) {
    this.bunkerPubkey = bunkerPubkey
    this.relayUrls = relayUrls
    this.connectionSecret = connectionSecret

    // Generate local ephemeral keypair for NIP-46 communication
    this.localPrivkey = secp256k1.utils.randomPrivateKey()
    this.localPubkey = nGetPublicKey(this.localPrivkey)
  }

  /**
   * Create a BunkerSigner that waits for a signer (like Amber) to connect.
   * Returns the nostr+connect URL to display as QR code and a promise for the connected signer.
   *
   * @param relayUrl - The relay URL for the connection
   * @param secret - Optional secret for the handshake
   * @param timeout - Connection timeout in ms (default 120000 = 2 minutes)
   */
  static async awaitSignerConnection(
    relayUrl: string,
    secret?: string,
    timeout = 120000
  ): Promise<{ connectUrl: string; signer: Promise<BunkerSigner> }> {
    // Generate ephemeral keypair for this session
    const localPrivkey = secp256k1.utils.randomPrivateKey()
    const localPubkey = nGetPublicKey(localPrivkey)

    // Generate secret if not provided
    const connectionSecret = secret || generateRequestId()

    // Build the nostr+connect URL for signer to scan
    const connectUrl = buildNostrConnectUrl(relayUrl, localPubkey, connectionSecret)

    // Create signer instance (bunkerPubkey will be set when signer connects)
    const signer = new BunkerSigner('', [relayUrl], connectionSecret)
    signer.localPrivkey = localPrivkey
    signer.localPubkey = localPubkey
    signer.awaitingConnection = true

    // Return URL immediately, signer promise resolves when connected
    const signerPromise = new Promise<BunkerSigner>((resolve, reject) => {
      signer.connectionResolve = (signerPubkey: string) => {
        signer.bunkerPubkey = signerPubkey
        // Do NOT set remotePubkey here - it must be fetched via get_public_key
        // The signerPubkey from the connect event is the bunker's communication pubkey,
        // not necessarily the user's signing pubkey
        signer.awaitingConnection = false
        resolve(signer)
      }
      // Set timeout
      setTimeout(() => {
        if (signer.awaitingConnection) {
          signer.disconnect()
          reject(new Error('Connection timeout waiting for signer'))
        }
      }, timeout)

      // Connect to relay and wait
      signer.connectAndWait(relayUrl).catch(reject)
    })

    return { connectUrl, signer: signerPromise }
  }

  /**
   * Connect to relay and wait for signer to initiate connection.
   */
  private async connectAndWait(relayUrl: string): Promise<void> {
    await this.connectToRelayAndListen(relayUrl)
  }

  /**
   * Connect to relay and listen for incoming connect requests.
   */
  private async connectToRelayAndListen(relayUrl: string): Promise<void> {
    return new Promise((resolve, reject) => {
      let wsUrl = relayUrl
      if (relayUrl.startsWith('http://')) {
        wsUrl = 'ws://' + relayUrl.slice(7)
      } else if (relayUrl.startsWith('https://')) {
        wsUrl = 'wss://' + relayUrl.slice(8)
      } else if (!relayUrl.startsWith('ws://') && !relayUrl.startsWith('wss://')) {
        wsUrl = 'wss://' + relayUrl
      }

      const ws = new WebSocket(wsUrl)

      const timeout = setTimeout(() => {
        ws.close()
        reject(new Error('Connection timeout'))
      }, 10000)

      ws.onopen = () => {
        clearTimeout(timeout)
        this.ws = ws
        this.connected = true

        // Subscribe to events for our local pubkey
        const subId = generateRequestId()
        ws.send(
          JSON.stringify([
            'REQ',
            subId,
            {
              kinds: [24133],
              '#p': [this.localPubkey],
              since: Math.floor(Date.now() / 1000) - 60
            }
          ])
        )

        resolve()
      }

      ws.onerror = () => {
        clearTimeout(timeout)
        reject(new Error('WebSocket error'))
      }

      ws.onclose = () => {
        this.connected = false
        this.ws = null
      }

      ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
    })
  }

  /**
   * Get the local public key (for displaying in nostr+connect URL).
   */
  getLocalPubkey(): string {
    return this.localPubkey
  }

  /**
   * Initialize connection to the bunker.
   */
  async init(): Promise<void> {
    // Connect to first available relay
    for (const relayUrl of this.relayUrls) {
      try {
        await this.connectToRelay(relayUrl)
        break
      } catch (err) {
        console.warn(`Failed to connect to ${relayUrl}:`, err)
      }
    }

    if (!this.connected) {
      throw new Error('Failed to connect to any bunker relay')
    }

    // Perform NIP-46 connect handshake
    await this.connect()
  }

  /**
   * Connect to a relay WebSocket.
   */
  private async connectToRelay(relayUrl: string): Promise<void> {
    return new Promise((resolve, reject) => {
      // Convert ws:// or wss:// URL
      let wsUrl = relayUrl
      if (relayUrl.startsWith('http://')) {
        wsUrl = 'ws://' + relayUrl.slice(7)
      } else if (relayUrl.startsWith('https://')) {
        wsUrl = 'wss://' + relayUrl.slice(8)
      } else if (!relayUrl.startsWith('ws://') && !relayUrl.startsWith('wss://')) {
        wsUrl = 'wss://' + relayUrl
      }

      const ws = new WebSocket(wsUrl)

      const timeout = setTimeout(() => {
        ws.close()
        reject(new Error('Connection timeout'))
      }, 10000)

      ws.onopen = () => {
        clearTimeout(timeout)
        this.ws = ws
        this.connected = true

        // Subscribe to responses for our local pubkey
        const subId = generateRequestId()
        ws.send(
          JSON.stringify([
            'REQ',
            subId,
            {
              kinds: [24133], // NIP-46 response kind
              '#p': [this.localPubkey],
              since: Math.floor(Date.now() / 1000) - 60
            }
          ])
        )

        resolve()
      }

      ws.onerror = () => {
        clearTimeout(timeout)
        reject(new Error('WebSocket error'))
      }

      ws.onclose = () => {
        this.connected = false
        this.ws = null
      }

      ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
    })
  }

  /**
   * Handle incoming WebSocket messages.
   */
  private async handleMessage(data: string): Promise<void> {
    try {
      const msg = JSON.parse(data)
      if (!Array.isArray(msg)) return

      const [type, ...rest] = msg

      if (type === 'EVENT') {
        const [, event] = rest as [string, Event]
        if (event.kind === 24133) {
          await this.handleNIP46Response(event)
        }
      } else if (type === 'OK') {
        // Event published confirmation
      } else if (type === 'NOTICE') {
        console.warn('Relay notice:', rest[0])
      }
    } catch (err) {
      console.error('Failed to parse message:', err)
    }
  }

  /**
   * Handle NIP-46 response event.
   */
  private async handleNIP46Response(event: Event): Promise<void> {
    try {
      // Decrypt the content with NIP-04
      const decrypted = await nip04.decrypt(this.localPrivkey, event.pubkey, event.content)
      const parsed = JSON.parse(decrypted)

      // Check if this is an incoming connect request (signer initiating connection)
      if (this.awaitingConnection && parsed.method === 'connect') {
        const request = parsed as NIP46Request
        console.log('Received connect request from signer:', event.pubkey)

        // Verify secret if we have one
        if (this.connectionSecret) {
          const providedSecret = request.params[1] // Second param is the secret
          if (providedSecret !== this.connectionSecret) {
            console.warn('Connect request has wrong secret, ignoring')
            return
          }
        }

        // Send ack response
        const response: NIP46Response = {
          id: request.id,
          result: 'ack'
        }
        const encrypted = await nip04.encrypt(this.localPrivkey, event.pubkey, JSON.stringify(response))
        const responseEvent: TDraftEvent = {
          kind: 24133,
          created_at: Math.floor(Date.now() / 1000),
          content: encrypted,
          tags: [['p', event.pubkey]]
        }
        const signedResponse = finalizeEvent(responseEvent, this.localPrivkey)
        this.ws?.send(JSON.stringify(['EVENT', signedResponse]))

        // Resolve the connection promise
        if (this.connectionResolve) {
          this.connectionResolve(event.pubkey)
        }
        return
      }

      // Handle as normal response
      const response = parsed as NIP46Response
      const pending = this.pendingRequests.get(response.id)

      if (pending) {
        clearTimeout(pending.timeout)
        this.pendingRequests.delete(response.id)

        if (response.error) {
          pending.reject(new Error(response.error))
        } else if (response.result !== undefined) {
          pending.resolve(response.result)
        } else {
          pending.reject(new Error('Empty response'))
        }
      }
    } catch (err) {
      console.error('Failed to handle NIP-46 response:', err)
    }
  }

  /**
   * Send a NIP-46 request and wait for response.
   */
  private async sendRequest(method: NIP46Method, params: string[] = []): Promise<string> {
    if (!this.ws || !this.connected) {
      throw new Error('Not connected to bunker')
    }

    const request: NIP46Request = {
      id: generateRequestId(),
      method,
      params
    }

    // Encrypt with NIP-04 to the bunker's pubkey
    const encrypted = await nip04.encrypt(this.localPrivkey, this.bunkerPubkey, JSON.stringify(request))

    // Create NIP-46 request event
    const draftEvent: TDraftEvent = {
      kind: 24133,
      created_at: Math.floor(Date.now() / 1000),
      content: encrypted,
      tags: [['p', this.bunkerPubkey]]
    }

    const signedEvent = finalizeEvent(draftEvent, this.localPrivkey)

    // Send to relay
    this.ws.send(JSON.stringify(['EVENT', signedEvent]))

    // Wait for response
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pendingRequests.delete(request.id)
        reject(new Error('Request timeout'))
      }, this.requestTimeout)

      this.pendingRequests.set(request.id, { resolve, reject, timeout })
    })
  }

  /**
   * Perform NIP-46 connect handshake.
   */
  private async connect(): Promise<void> {
    const params: string[] = [this.localPubkey]
    if (this.connectionSecret) {
      params.push(this.connectionSecret)
    }

    const result = await this.sendRequest(NIP46_METHOD.CONNECT, params)
    if (result !== 'ack') {
      throw new Error(`Connect failed: ${result}`)
    }
  }

  /**
   * Get the public key of the user (from the bunker).
   */
  async getPublicKey(): Promise<string> {
    if (this.remotePubkey) {
      return this.remotePubkey
    }

    const pubkey = await this.sendRequest(NIP46_METHOD.GET_PUBLIC_KEY)
    this.remotePubkey = pubkey
    return pubkey
  }

  /**
   * Sign an event via the bunker.
   */
  async signEvent(draftEvent: TDraftEvent): Promise<VerifiedEvent> {
    const eventJson = JSON.stringify({
      ...draftEvent,
      pubkey: await this.getPublicKey()
    })

    const signedEventJson = await this.sendRequest(NIP46_METHOD.SIGN_EVENT, [eventJson])
    const signedEvent = JSON.parse(signedEventJson) as VerifiedEvent

    return signedEvent
  }

  /**
   * Encrypt a message with NIP-04 via the bunker.
   */
  async nip04Encrypt(pubkey: string, plainText: string): Promise<string> {
    return this.sendRequest(NIP46_METHOD.NIP04_ENCRYPT, [pubkey, plainText])
  }

  /**
   * Decrypt a message with NIP-04 via the bunker.
   */
  async nip04Decrypt(pubkey: string, cipherText: string): Promise<string> {
    return this.sendRequest(NIP46_METHOD.NIP04_DECRYPT, [pubkey, cipherText])
  }

  /**
   * Encrypt a message with NIP-44 via the bunker.
   */
  async nip44Encrypt(pubkey: string, plainText: string): Promise<string> {
    return this.sendRequest(NIP46_METHOD.NIP44_ENCRYPT, [pubkey, plainText])
  }

  /**
   * Decrypt a message with NIP-44 via the bunker.
   */
  async nip44Decrypt(pubkey: string, cipherText: string): Promise<string> {
    return this.sendRequest(NIP46_METHOD.NIP44_DECRYPT, [pubkey, cipherText])
  }

  /**
   * Check if connected to the bunker.
   */
  isConnected(): boolean {
    return this.connected
  }

  /**
   * Disconnect from the bunker.
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.connected = false
    this.pendingRequests.forEach((pending) => {
      clearTimeout(pending.timeout)
      pending.reject(new Error('Disconnected'))
    })
    this.pendingRequests.clear()
  }

  /**
   * Get the bunker's public key.
   */
  getBunkerPubkey(): string {
    return this.bunkerPubkey
  }

  /**
   * Get the relay URLs.
   */
  getRelayUrls(): string[] {
    return this.relayUrls
  }

  /**
   * Get the bunker URL for sharing.
   */
  getBunkerUrl(): string {
    return buildBunkerUrl(this.bunkerPubkey, this.relayUrls)
  }
}
