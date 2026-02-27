/**
 * NRC (Nostr Relay Connect) Listener Service
 *
 * Listens for NRC requests (kind 24891) on a rendezvous relay and responds
 * with events from the local IndexedDB. This allows other user clients to
 * sync their data through this client.
 *
 * Protocol:
 * - Client sends kind 24891 request with encrypted REQ/CLOSE message
 * - This listener decrypts, queries local storage, and responds with kind 24892
 * - All content is NIP-44 encrypted end-to-end
 */

import { Event, Filter } from 'nostr-tools'
import * as utils from '@noble/curves/abstract/utils'
import indexedDb from '@/services/indexed-db.service'
import {
  KIND_NRC_REQUEST,
  KIND_NRC_RESPONSE,
  NRCListenerConfig,
  RequestMessage,
  ResponseMessage,
  AuthResult,
  NRCSession,
  isDeviceSpecificEvent,
  EventManifestEntry
} from './nrc-types'
import { NRCSessionManager } from './nrc-session'

/**
 * Generate a random subscription ID
 */
function generateSubId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(8))
  return utils.bytesToHex(bytes)
}

/**
 * NRC Listener Service
 *
 * Listens for incoming NRC requests and responds with local events.
 */
export class NRCListenerService {
  private config: NRCListenerConfig | null = null
  private sessions: NRCSessionManager
  private ws: WebSocket | null = null
  private subId: string | null = null
  private connected = false
  private running = false
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000 // Start with 1 second
  private maxReconnectDelay = 30000 // Max 30 seconds
  private listenerPubkey: string | null = null

  // Event callbacks
  private onSessionChange?: (count: number) => void

  constructor() {
    this.sessions = new NRCSessionManager()
  }

  /**
   * Set callback for session count changes
   */
  setOnSessionChange(callback: (count: number) => void): void {
    this.onSessionChange = callback
  }

  /**
   * Start listening for NRC requests
   */
  async start(config: NRCListenerConfig): Promise<void> {
    if (this.running) {
      console.warn('[NRC] Listener already running')
      return
    }

    this.config = config
    this.running = true

    // Get our public key
    this.listenerPubkey = await config.signer.getPublicKey()

    // Start session cleanup
    this.sessions.start()

    // Connect to rendezvous relay
    await this.connectToRelay()
  }

  /**
   * Stop listening
   */
  stop(): void {
    this.running = false

    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout)
      this.reconnectTimeout = null
    }

    if (this.ws) {
      // Unsubscribe
      if (this.subId) {
        try {
          this.ws.send(JSON.stringify(['CLOSE', this.subId]))
        } catch {
          // Ignore errors when closing
        }
      }
      this.ws.close()
      this.ws = null
    }

    this.sessions.stop()
    this.connected = false
    this.subId = null

    console.log('[NRC] Listener stopped')
  }

  /**
   * Check if listener is running
   */
  isRunning(): boolean {
    return this.running
  }

  /**
   * Check if connected to rendezvous relay
   */
  isConnected(): boolean {
    return this.connected
  }

  /**
   * Get active session count
   */
  getActiveSessionCount(): number {
    return this.sessions.getActiveSessionCount()
  }

  /**
   * Connect to the rendezvous relay
   */
  private async connectToRelay(): Promise<void> {
    if (!this.config || !this.running) return Promise.resolve()

    const relayUrl = this.config.rendezvousUrl

    return new Promise<void>((resolve, reject) => {
      // Normalize WebSocket URL
      let wsUrl = relayUrl
      if (relayUrl.startsWith('http://')) {
        wsUrl = 'ws://' + relayUrl.slice(7)
      } else if (relayUrl.startsWith('https://')) {
        wsUrl = 'wss://' + relayUrl.slice(8)
      } else if (!relayUrl.startsWith('ws://') && !relayUrl.startsWith('wss://')) {
        wsUrl = 'wss://' + relayUrl
      }

      console.log(`[NRC] Connecting to rendezvous relay: ${wsUrl}`)

      const ws = new WebSocket(wsUrl)

      const timeout = setTimeout(() => {
        ws.close()
        reject(new Error('Connection timeout'))
      }, 10000)

      ws.onopen = () => {
        clearTimeout(timeout)
        this.ws = ws
        this.connected = true
        this.reconnectDelay = 1000 // Reset reconnect delay on success

        // Subscribe to NRC requests for our pubkey
        this.subId = generateSubId()
        ws.send(
          JSON.stringify([
            'REQ',
            this.subId,
            {
              kinds: [KIND_NRC_REQUEST],
              '#p': [this.listenerPubkey],
              since: Math.floor(Date.now() / 1000) - 60
            }
          ])
        )

        console.log(`[NRC] Connected and subscribed with subId: ${this.subId}, listening for pubkey: ${this.listenerPubkey}`)
        resolve()
      }

      ws.onerror = (error) => {
        clearTimeout(timeout)
        console.error('[NRC] WebSocket error:', error)
        reject(new Error('WebSocket error'))
      }

      ws.onclose = () => {
        this.connected = false
        this.ws = null
        this.subId = null
        console.log('[NRC] WebSocket closed')

        // Attempt reconnection if still running
        if (this.running) {
          this.scheduleReconnect()
        }
      }

      ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
    }).catch((error) => {
      console.error('[NRC] Failed to connect:', error)
      if (this.running) {
        this.scheduleReconnect()
      }
    })
  }

  /**
   * Schedule reconnection with exponential backoff
   */
  private scheduleReconnect(): void {
    if (this.reconnectTimeout || !this.running) return

    console.log(`[NRC] Scheduling reconnect in ${this.reconnectDelay}ms`)

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null
      this.connectToRelay()
    }, this.reconnectDelay)

    // Exponential backoff
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
  }

  /**
   * Handle incoming WebSocket message
   */
  private handleMessage(data: string): void {
    try {
      const msg = JSON.parse(data)
      if (!Array.isArray(msg)) return

      const [type, ...rest] = msg

      if (type === 'EVENT') {
        const [, event] = rest as [string, Event]
        if (event.kind === KIND_NRC_REQUEST) {
          console.log('[NRC] Received NRC request from pubkey:', event.pubkey)
          this.handleRequest(event).catch((err) => {
            console.error('[NRC] Error handling request:', err)
          })
        }
      } else if (type === 'EOSE') {
        // End of stored events, listener is now live
        console.log('[NRC] Received EOSE, now listening for live events')
      } else if (type === 'NOTICE') {
        console.log('[NRC] Relay notice:', rest[0])
      } else if (type === 'OK') {
        // Event published successfully
      } else if (type === 'CLOSED') {
        console.log('[NRC] Subscription closed:', rest)
      }
    } catch (err) {
      console.error('[NRC] Failed to parse message:', err)
    }
  }

  /**
   * Handle an NRC request event
   */
  private async handleRequest(event: Event): Promise<void> {
    if (!this.config) return

    // Extract session ID from tags (used for correlation but we use pubkey-based sessions)
    const sessionTag = event.tags.find((t) => t[0] === 'session')
    const _sessionId = sessionTag?.[1]
    void _sessionId // Suppress unused variable warning

    try {
      // Authorize the request
      const authResult = await this.authorize(event)

      // Get or create session
      const session = this.sessions.getOrCreateSession(
        event.pubkey,
        undefined, // We use signer's nip44 methods instead of conversationKey
        authResult.deviceName
      )

      // Notify session change
      this.onSessionChange?.(this.sessions.getActiveSessionCount())

      // Decrypt the content using signer
      const plaintext = await this.decrypt(event.pubkey, event.content)
      const request: RequestMessage = JSON.parse(plaintext)
      console.log('[NRC] Received request:', request.type)

      // Handle the request based on type
      switch (request.type) {
        case 'REQ':
          await this.handleREQ(event, session, request.payload)
          break
        case 'CLOSE':
          await this.handleCLOSE(session, request.payload)
          break
        case 'EVENT':
          await this.handleEVENT(event, session, request.payload)
          break
        case 'IDS':
          // Return just event IDs matching filters (for diffing)
          await this.handleIDS(event, session, request.payload)
          break
        case 'COUNT':
          // Not implemented
          await this.sendError(event, session, 'COUNT not supported')
          break
        default:
          await this.sendError(event, session, `Unknown message type: ${request.type}`)
      }
    } catch (err) {
      console.error('[NRC] Request handling failed:', err)
      // Try to send error response (best effort)
      try {
        await this.sendErrorBestEffort(event, `Request failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
      } catch {
        // Ignore errors when sending error response
      }
    }
  }

  /**
   * Authorize an incoming request
   */
  private async authorize(event: Event): Promise<AuthResult> {
    if (!this.config) {
      throw new Error('Listener not configured')
    }

    // Secret-based auth: check if pubkey is authorized
    const deviceName = this.config.authorizedSecrets.get(event.pubkey)
    if (!deviceName) {
      console.log('[NRC] Unauthorized pubkey:', event.pubkey)
      console.log('[NRC] Authorized pubkeys:', Array.from(this.config.authorizedSecrets.keys()))
      console.log('[NRC] Authorized pubkeys (full):', JSON.stringify(Array.from(this.config.authorizedSecrets.entries())))
      throw new Error('Unauthorized: unknown client pubkey')
    }

    return {
      deviceName
    }
  }

  /**
   * Decrypt content using the signer's NIP-44 implementation
   */
  private async decrypt(clientPubkey: string, ciphertext: string): Promise<string> {
    if (!this.config) {
      throw new Error('Listener not configured')
    }

    if (!this.config.signer.nip44Decrypt) {
      throw new Error('Signer does not support NIP-44 decryption')
    }

    return this.config.signer.nip44Decrypt(clientPubkey, ciphertext)
  }

  /**
   * Encrypt content using the signer's NIP-44 implementation
   */
  private async encrypt(clientPubkey: string, plaintext: string): Promise<string> {
    if (!this.config) {
      throw new Error('Listener not configured')
    }

    if (!this.config.signer.nip44Encrypt) {
      throw new Error('Signer does not support NIP-44 encryption')
    }

    return this.config.signer.nip44Encrypt(clientPubkey, plaintext)
  }

  // Max chunk size (accounting for encryption overhead and event wrapper)
  // NIP-44 adds ~100 bytes overhead, plus base64 encoding increases size by ~33%
  private static readonly MAX_CHUNK_SIZE = 40000 // ~40KB chunks to stay safely under 65KB limit

  /**
   * Handle REQ message - query local storage and respond
   */
  private async handleREQ(
    reqEvent: Event,
    session: NRCSession,
    payload: unknown[]
  ): Promise<void> {
    // Parse REQ: ["REQ", subId, filter1, filter2, ...]
    if (payload.length < 2) {
      await this.sendError(reqEvent, session, 'Invalid REQ: missing subscription ID or filters')
      return
    }

    const [, subId, ...filterObjs] = payload as [string, string, ...Filter[]]

    // Add subscription to session
    const subscription = this.sessions.addSubscription(session.id, subId, filterObjs)
    if (!subscription) {
      await this.sendError(reqEvent, session, 'Too many subscriptions')
      return
    }

    // Query local events matching the filters
    const events = await this.queryLocalEvents(filterObjs)
    console.log(`[NRC] Found ${events.length} events matching filters`)

    // Send each matching event
    for (const evt of events) {
      const response: ResponseMessage = {
        type: 'EVENT',
        payload: ['EVENT', subId, evt]
      }

      try {
        await this.sendResponseChunked(reqEvent, session, response)
        this.sessions.incrementEventCount(session.id, subId)
      } catch (err) {
        console.error(`[NRC] Failed to send event ${evt.id?.slice(0, 8)}:`, err)
      }
    }

    // Send EOSE
    const eoseResponse: ResponseMessage = {
      type: 'EOSE',
      payload: ['EOSE', subId]
    }
    await this.sendResponse(reqEvent, session, eoseResponse)
    this.sessions.markEOSE(session.id, subId)
    console.log(`[NRC] Sent EOSE for subscription ${subId}`)
  }

  /**
   * Handle CLOSE message
   */
  private async handleCLOSE(session: NRCSession, payload: unknown[]): Promise<void> {
    // Parse CLOSE: ["CLOSE", subId]
    const [, subId] = payload as [string, string]
    if (subId) {
      this.sessions.removeSubscription(session.id, subId)
    }
  }

  /**
   * Handle EVENT message - store an event from the remote device
   */
  private async handleEVENT(
    reqEvent: Event,
    session: NRCSession,
    payload: unknown[]
  ): Promise<void> {
    // Parse EVENT: ["EVENT", eventObject]
    const [, eventToStore] = payload as [string, Event]

    if (!eventToStore || !eventToStore.id || !eventToStore.sig) {
      await this.sendError(reqEvent, session, 'Invalid EVENT: missing event data')
      return
    }

    try {
      // Store the event in IndexedDB
      await indexedDb.putReplaceableEvent(eventToStore)
      console.log(`[NRC] Stored event ${eventToStore.id.slice(0, 8)} kind ${eventToStore.kind} from ${session.deviceName}`)

      // Send OK response
      const response: ResponseMessage = {
        type: 'OK',
        payload: ['OK', eventToStore.id, true, '']
      }
      await this.sendResponse(reqEvent, session, response)
    } catch (err) {
      console.error('[NRC] Failed to store event:', err)
      const response: ResponseMessage = {
        type: 'OK',
        payload: ['OK', eventToStore.id, false, `Failed to store: ${err instanceof Error ? err.message : 'Unknown error'}`]
      }
      await this.sendResponse(reqEvent, session, response)
    }
  }

  /**
   * Handle IDS message - return event IDs matching filters (for diffing)
   * Similar to REQ but returns only IDs, not full events
   */
  private async handleIDS(
    reqEvent: Event,
    session: NRCSession,
    payload: unknown[]
  ): Promise<void> {
    // Parse IDS: ["IDS", subId, filter1, filter2, ...]
    if (payload.length < 2) {
      await this.sendError(reqEvent, session, 'Invalid IDS: missing subscription ID or filters')
      return
    }

    const [, subId, ...filterObjs] = payload as [string, string, ...Filter[]]

    // Query local events matching the filters
    const events = await this.queryLocalEvents(filterObjs)
    console.log(`[NRC] Found ${events.length} events for IDS request`)

    // Build manifest of event IDs with metadata for diffing
    const manifest: EventManifestEntry[] = events.map((evt) => ({
      kind: evt.kind,
      id: evt.id,
      created_at: evt.created_at,
      d: evt.tags.find((t) => t[0] === 'd')?.[1]
    }))

    // Send IDS response with the manifest
    const response: ResponseMessage = {
      type: 'IDS',
      payload: ['IDS', subId, manifest]
    }
    await this.sendResponseChunked(reqEvent, session, response)
    console.log(`[NRC] Sent IDS response with ${manifest.length} entries`)
  }

  /**
   * Query local IndexedDB for events matching filters
   */
  private async queryLocalEvents(filters: Filter[]): Promise<Event[]> {
    // Get all events from IndexedDB and filter
    const allEvents = await indexedDb.queryEventsForNRC(filters)

    // Filter out device-specific events
    return allEvents.filter((evt) => !isDeviceSpecificEvent(evt))
  }

  /**
   * Send an encrypted response
   */
  private async sendResponse(
    reqEvent: Event,
    session: NRCSession,
    response: ResponseMessage
  ): Promise<void> {
    if (!this.ws || !this.config || !this.listenerPubkey) {
      throw new Error('Not connected')
    }

    // Encrypt the response using signer
    const plaintext = JSON.stringify(response)
    const encrypted = await this.encrypt(session.clientPubkey, plaintext)

    // Build the response event
    const unsignedEvent = {
      kind: KIND_NRC_RESPONSE,
      content: encrypted,
      tags: [
        ['p', reqEvent.pubkey],
        ['encryption', 'nip44_v2'],
        ['session', session.id],
        ['e', reqEvent.id]
      ],
      created_at: Math.floor(Date.now() / 1000)
    }

    // Sign with our signer
    const signedEvent = await this.config.signer.signEvent(unsignedEvent)

    // Publish to rendezvous relay
    this.ws.send(JSON.stringify(['EVENT', signedEvent]))
  }

  /**
   * Send a response, chunking if necessary for large payloads
   */
  private async sendResponseChunked(
    reqEvent: Event,
    session: NRCSession,
    response: ResponseMessage
  ): Promise<void> {
    const plaintext = JSON.stringify(response)

    // If small enough, send directly
    if (plaintext.length <= NRCListenerService.MAX_CHUNK_SIZE) {
      await this.sendResponse(reqEvent, session, response)
      return
    }

    // Need to chunk - convert to base64 for safe transmission
    const encoded = btoa(unescape(encodeURIComponent(plaintext)))
    const chunks: string[] = []

    // Split into chunks
    for (let i = 0; i < encoded.length; i += NRCListenerService.MAX_CHUNK_SIZE) {
      chunks.push(encoded.slice(i, i + NRCListenerService.MAX_CHUNK_SIZE))
    }

    const messageId = crypto.randomUUID()
    console.log(`[NRC] Chunking large message (${plaintext.length} bytes) into ${chunks.length} chunks`)

    // Send each chunk
    for (let i = 0; i < chunks.length; i++) {
      const chunkResponse: ResponseMessage = {
        type: 'CHUNK',
        payload: [{
          type: 'CHUNK',
          messageId,
          index: i,
          total: chunks.length,
          data: chunks[i]
        }]
      }
      await this.sendResponse(reqEvent, session, chunkResponse)
    }
  }

  /**
   * Send an error response
   */
  private async sendError(
    reqEvent: Event,
    session: NRCSession,
    message: string
  ): Promise<void> {
    const response: ResponseMessage = {
      type: 'NOTICE',
      payload: ['NOTICE', message]
    }
    await this.sendResponse(reqEvent, session, response)
  }

  /**
   * Send error response with best-effort encryption
   */
  private async sendErrorBestEffort(reqEvent: Event, message: string): Promise<void> {
    if (!this.ws || !this.config || !this.listenerPubkey) {
      return
    }

    try {
      const response: ResponseMessage = {
        type: 'NOTICE',
        payload: ['NOTICE', message]
      }

      const plaintext = JSON.stringify(response)
      const encrypted = await this.encrypt(reqEvent.pubkey, plaintext)

      const unsignedEvent = {
        kind: KIND_NRC_RESPONSE,
        content: encrypted,
        tags: [
          ['p', reqEvent.pubkey],
          ['encryption', 'nip44_v2'],
          ['e', reqEvent.id]
        ],
        created_at: Math.floor(Date.now() / 1000)
      }

      const signedEvent = await this.config.signer.signEvent(unsignedEvent)
      this.ws.send(JSON.stringify(['EVENT', signedEvent]))
    } catch {
      // Best effort - ignore errors
    }
  }
}

// Singleton instance
let instance: NRCListenerService | null = null

export function getNRCListenerService(): NRCListenerService {
  if (!instance) {
    instance = new NRCListenerService()
  }
  return instance
}

export default getNRCListenerService()
