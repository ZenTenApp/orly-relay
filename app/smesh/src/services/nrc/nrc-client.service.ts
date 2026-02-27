/**
 * NRC (Nostr Relay Connect) Client Service
 *
 * Connects to a remote NRC listener and syncs events.
 * Uses the nostr+relayconnect:// URI scheme to establish encrypted
 * communication through a rendezvous relay.
 */

import { Event, Filter } from 'nostr-tools'
import * as nip44 from 'nostr-tools/nip44'
import * as utils from '@noble/curves/abstract/utils'
import { finalizeEvent } from 'nostr-tools'
import {
  KIND_NRC_REQUEST,
  KIND_NRC_RESPONSE,
  RequestMessage,
  ResponseMessage,
  ParsedConnectionURI,
  EventManifestEntry
} from './nrc-types'
import { parseConnectionURI, deriveConversationKey } from './nrc-uri'

/**
 * Generate a random subscription ID
 */
function generateSubId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(8))
  return utils.bytesToHex(bytes)
}

/**
 * Generate a random session ID
 */
function generateSessionId(): string {
  return crypto.randomUUID()
}

/**
 * Sync progress callback
 */
export interface SyncProgress {
  phase: 'connecting' | 'requesting' | 'receiving' | 'sending' | 'complete' | 'error'
  eventsReceived: number
  eventsSent?: number
  message?: string
}

/**
 * Remote connection state
 */
export interface RemoteConnection {
  id: string
  uri: string
  label: string
  relayPubkey: string
  rendezvousUrl: string
  lastSync?: number
  eventCount?: number
}

// Chunk buffer for reassembling large messages
interface ChunkBuffer {
  chunks: Map<number, string>
  total: number
  receivedAt: number
}

// Default sync timeout: 60 seconds
const DEFAULT_SYNC_TIMEOUT = 60000

/**
 * NRC Client for connecting to remote devices
 */
export class NRCClient {
  private uri: ParsedConnectionURI
  private ws: WebSocket | null = null
  private sessionId: string
  private connected = false
  private subId: string | null = null
  private pendingEvents: Event[] = []
  private onProgress?: (progress: SyncProgress) => void
  private resolveSync?: (events: Event[]) => void
  private rejectSync?: (error: Error) => void
  private chunkBuffers: Map<string, ChunkBuffer> = new Map()
  private syncTimeout: ReturnType<typeof setTimeout> | null = null
  private lastActivityTime: number = 0

  constructor(connectionUri: string) {
    this.uri = parseConnectionURI(connectionUri)
    this.sessionId = generateSessionId()
  }

  /**
   * Get the relay pubkey this client connects to
   */
  getRelayPubkey(): string {
    return this.uri.relayPubkey
  }

  /**
   * Get the rendezvous URL
   */
  getRendezvousUrl(): string {
    return this.uri.rendezvousUrl
  }

  /**
   * Connect to the rendezvous relay and sync events
   */
  async sync(
    filters: Filter[],
    onProgress?: (progress: SyncProgress) => void,
    timeout: number = DEFAULT_SYNC_TIMEOUT
  ): Promise<Event[]> {
    this.onProgress = onProgress
    this.pendingEvents = []
    this.chunkBuffers.clear()
    this.lastActivityTime = Date.now()

    return new Promise<Event[]>((resolve, reject) => {
      this.resolveSync = resolve
      this.rejectSync = reject

      // Set up sync timeout
      this.syncTimeout = setTimeout(() => {
        const timeSinceActivity = Date.now() - this.lastActivityTime
        if (timeSinceActivity > 30000) {
          // No activity for 30s, likely stalled
          console.error('[NRC Client] Sync timeout - no activity for 30s')
          this.disconnect()
          reject(new Error('Sync timeout - connection stalled'))
        } else {
          // Still receiving data, extend timeout
          console.log('[NRC Client] Sync still active, extending timeout')
          this.syncTimeout = setTimeout(() => {
            this.disconnect()
            reject(new Error('Sync timeout'))
          }, timeout)
        }
      }, timeout)

      this.connect()
        .then(() => {
          this.sendREQ(filters)
        })
        .catch((err) => {
          this.clearSyncTimeout()
          reject(err)
        })
    })
  }

  // State for IDS request
  private idsMode = false
  private resolveIDs?: (manifest: EventManifestEntry[]) => void
  private rejectIDs?: (error: Error) => void

  // State for sending events
  private sendingEvents = false
  private eventsSentCount = 0
  private eventsToSend: Event[] = []
  private resolveSend?: (count: number) => void

  /**
   * Request event IDs from remote (for diffing)
   */
  async requestIDs(
    filters: Filter[],
    onProgress?: (progress: SyncProgress) => void,
    timeout: number = DEFAULT_SYNC_TIMEOUT
  ): Promise<EventManifestEntry[]> {
    this.onProgress = onProgress
    this.chunkBuffers.clear()
    this.lastActivityTime = Date.now()
    this.idsMode = true

    return new Promise<EventManifestEntry[]>((resolve, reject) => {
      this.resolveIDs = resolve
      this.rejectIDs = reject

      this.syncTimeout = setTimeout(() => {
        this.disconnect()
        reject(new Error('IDS request timeout'))
      }, timeout)

      this.connect()
        .then(() => {
          this.sendIDSRequest(filters)
        })
        .catch((err) => {
          this.clearSyncTimeout()
          reject(err)
        })
    })
  }

  /**
   * Send IDS request
   */
  private sendIDSRequest(filters: Filter[]): void {
    if (!this.ws || !this.connected) {
      this.rejectIDs?.(new Error('Not connected'))
      return
    }

    this.onProgress?.({
      phase: 'requesting',
      eventsReceived: 0,
      message: 'Requesting event IDs...'
    })

    this.subId = generateSubId()

    const request: RequestMessage = {
      type: 'IDS',
      payload: ['IDS', this.subId, ...filters]
    }

    this.sendEncryptedRequest(request).catch((err) => {
      console.error('[NRC Client] Failed to send IDS:', err)
      this.rejectIDs?.(err)
    })
  }

  /**
   * Send events to remote device
   */
  async sendEvents(
    events: Event[],
    onProgress?: (progress: SyncProgress) => void,
    timeout: number = DEFAULT_SYNC_TIMEOUT
  ): Promise<number> {
    if (events.length === 0) return 0

    this.onProgress = onProgress
    this.chunkBuffers.clear()
    this.lastActivityTime = Date.now()
    this.sendingEvents = true
    this.eventsSentCount = 0
    this.eventsToSend = [...events]

    return new Promise<number>((resolve, reject) => {
      this.resolveSend = resolve

      this.syncTimeout = setTimeout(() => {
        this.disconnect()
        reject(new Error('Send events timeout'))
      }, timeout)

      this.connect()
        .then(() => {
          this.sendNextEvent()
        })
        .catch((err) => {
          this.clearSyncTimeout()
          reject(err)
        })
    })
  }

  /**
   * Send the next event in the queue
   */
  private sendNextEvent(): void {
    if (this.eventsToSend.length === 0) {
      // All done
      this.clearSyncTimeout()
      this.onProgress?.({
        phase: 'complete',
        eventsReceived: 0,
        eventsSent: this.eventsSentCount,
        message: `Sent ${this.eventsSentCount} events`
      })
      this.resolveSend?.(this.eventsSentCount)
      this.disconnect()
      return
    }

    const event = this.eventsToSend.shift()!
    this.onProgress?.({
      phase: 'sending',
      eventsReceived: 0,
      eventsSent: this.eventsSentCount,
      message: `Sending event ${this.eventsSentCount + 1}...`
    })

    const request: RequestMessage = {
      type: 'EVENT',
      payload: ['EVENT', event]
    }

    this.sendEncryptedRequest(request).catch((err) => {
      console.error('[NRC Client] Failed to send EVENT:', err)
      // Continue with next event even if this one failed
      this.sendNextEvent()
    })
  }

  /**
   * Clear the sync timeout
   */
  private clearSyncTimeout(): void {
    if (this.syncTimeout) {
      clearTimeout(this.syncTimeout)
      this.syncTimeout = null
    }
  }

  /**
   * Update last activity time (called when receiving data)
   */
  private updateActivity(): void {
    this.lastActivityTime = Date.now()
  }

  /**
   * Connect to the rendezvous relay
   */
  private async connect(): Promise<void> {
    if (this.connected) return

    this.onProgress?.({
      phase: 'connecting',
      eventsReceived: 0,
      message: 'Connecting to rendezvous relay...'
    })

    const relayUrl = this.uri.rendezvousUrl

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

      console.log(`[NRC Client] Connecting to: ${wsUrl}`)

      const ws = new WebSocket(wsUrl)

      const timeout = setTimeout(() => {
        ws.close()
        reject(new Error('Connection timeout'))
      }, 10000)

      ws.onopen = () => {
        clearTimeout(timeout)
        this.ws = ws
        this.connected = true

        // Subscribe to responses for our client pubkey
        const responseSubId = generateSubId()
        const clientPubkey = this.uri.clientPubkey

        if (!clientPubkey) {
          reject(new Error('Client pubkey not available'))
          return
        }

        ws.send(
          JSON.stringify([
            'REQ',
            responseSubId,
            {
              kinds: [KIND_NRC_RESPONSE],
              '#p': [clientPubkey],
              since: Math.floor(Date.now() / 1000) - 60
            }
          ])
        )

        console.log(`[NRC Client] Connected, subscribed for responses to ${clientPubkey.slice(0, 8)}...`)
        resolve()
      }

      ws.onerror = (error) => {
        clearTimeout(timeout)
        console.error('[NRC Client] WebSocket error:', error)
        reject(new Error('WebSocket error'))
      }

      ws.onclose = () => {
        this.connected = false
        this.ws = null
        console.log('[NRC Client] WebSocket closed')
      }

      ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
    })
  }

  /**
   * Send a REQ message to the remote listener
   */
  private sendREQ(filters: Filter[]): void {
    if (!this.ws || !this.connected) {
      this.rejectSync?.(new Error('Not connected'))
      return
    }

    console.log(`[NRC Client] Sending REQ to listener pubkey: ${this.uri.relayPubkey?.slice(0, 8)}...`)
    console.log(`[NRC Client] Our client pubkey: ${this.uri.clientPubkey?.slice(0, 8)}...`)
    console.log(`[NRC Client] Filters:`, JSON.stringify(filters))

    this.onProgress?.({
      phase: 'requesting',
      eventsReceived: 0,
      message: 'Requesting events...'
    })

    this.subId = generateSubId()

    const request: RequestMessage = {
      type: 'REQ',
      payload: ['REQ', this.subId, ...filters]
    }

    this.sendEncryptedRequest(request).catch((err) => {
      console.error('[NRC Client] Failed to send request:', err)
      this.rejectSync?.(err)
    })
  }

  /**
   * Send an encrypted request to the remote listener
   */
  private async sendEncryptedRequest(request: RequestMessage): Promise<void> {
    if (!this.ws) {
      throw new Error('Not connected')
    }

    if (!this.uri.clientPrivkey || !this.uri.clientPubkey) {
      throw new Error('Missing keys')
    }

    const plaintext = JSON.stringify(request)

    // Derive conversation key
    const conversationKey = deriveConversationKey(
      this.uri.clientPrivkey,
      this.uri.relayPubkey
    )

    const encrypted = nip44.v2.encrypt(plaintext, conversationKey)

    // Build the request event
    const unsignedEvent = {
      kind: KIND_NRC_REQUEST,
      content: encrypted,
      tags: [
        ['p', this.uri.relayPubkey],
        ['encryption', 'nip44_v2'],
        ['session', this.sessionId]
      ],
      created_at: Math.floor(Date.now() / 1000),
      pubkey: this.uri.clientPubkey
    }

    const signedEvent = finalizeEvent(unsignedEvent, this.uri.clientPrivkey)

    // Send to rendezvous relay
    this.ws.send(JSON.stringify(['EVENT', signedEvent]))
    console.log(`[NRC Client] Sent encrypted REQ, event id: ${signedEvent.id?.slice(0, 8)}..., p-tag: ${this.uri.relayPubkey?.slice(0, 8)}...`)
  }

  /**
   * Handle incoming WebSocket messages
   */
  private handleMessage(data: string): void {
    try {
      const msg = JSON.parse(data)
      if (!Array.isArray(msg)) return

      const [type, ...rest] = msg

      if (type === 'EVENT') {
        const [subId, event] = rest as [string, Event]
        console.log(`[NRC Client] Received EVENT on sub ${subId}, kind ${event.kind}, from ${event.pubkey?.slice(0, 8)}...`)

        if (event.kind === KIND_NRC_RESPONSE) {
          // Check p-tag to see who it's addressed to
          const pTag = event.tags.find(t => t[0] === 'p')?.[1]
          console.log(`[NRC Client] Response p-tag: ${pTag?.slice(0, 8)}..., our pubkey: ${this.uri.clientPubkey?.slice(0, 8)}...`)
          this.handleResponse(event)
        } else {
          console.log(`[NRC Client] Ignoring event kind ${event.kind}`)
        }
      } else if (type === 'EOSE') {
        console.log('[NRC Client] Received EOSE from relay subscription')
      } else if (type === 'OK') {
        console.log('[NRC Client] Event published:', rest)
      } else if (type === 'NOTICE') {
        console.log('[NRC Client] Relay notice:', rest[0])
      }
    } catch (err) {
      console.error('[NRC Client] Failed to parse message:', err)
    }
  }

  /**
   * Handle a response event from the remote listener
   */
  private handleResponse(event: Event): void {
    console.log(`[NRC Client] Attempting to decrypt response from ${event.pubkey?.slice(0, 8)}...`)

    this.decryptAndProcessResponse(event).catch((err) => {
      console.error('[NRC Client] Failed to handle response:', err)
    })
  }

  /**
   * Decrypt and process a response event
   */
  private async decryptAndProcessResponse(event: Event): Promise<void> {
    if (!this.uri.clientPrivkey) {
      throw new Error('Missing private key for decryption')
    }

    const conversationKey = deriveConversationKey(
      this.uri.clientPrivkey,
      this.uri.relayPubkey
    )
    const plaintext = nip44.v2.decrypt(event.content, conversationKey)

    const response: ResponseMessage = JSON.parse(plaintext)
    console.log(`[NRC Client] Received response: ${response.type}`)

    // Handle chunked messages
    if (response.type === 'CHUNK') {
      this.handleChunk(response)
      return
    }

    this.processResponse(response)
  }

  /**
   * Handle a chunk message and reassemble when complete
   */
  private handleChunk(response: ResponseMessage): void {
    const chunk = response.payload[0] as {
      type: 'CHUNK'
      messageId: string
      index: number
      total: number
      data: string
    }

    if (!chunk || chunk.type !== 'CHUNK') {
      console.error('[NRC Client] Invalid chunk message')
      return
    }

    const { messageId, index, total, data } = chunk

    // Get or create buffer for this message
    let buffer = this.chunkBuffers.get(messageId)
    if (!buffer) {
      buffer = {
        chunks: new Map(),
        total,
        receivedAt: Date.now()
      }
      this.chunkBuffers.set(messageId, buffer)
    }

    // Store the chunk
    buffer.chunks.set(index, data)
    this.updateActivity()
    console.log(`[NRC Client] Received chunk ${index + 1}/${total} for message ${messageId.slice(0, 8)}`)

    // Check if we have all chunks
    if (buffer.chunks.size === buffer.total) {
      // Reassemble the message
      const parts: string[] = []
      for (let i = 0; i < buffer.total; i++) {
        const part = buffer.chunks.get(i)
        if (!part) {
          console.error(`[NRC Client] Missing chunk ${i} for message ${messageId}`)
          this.chunkBuffers.delete(messageId)
          return
        }
        parts.push(part)
      }

      // Decode from base64
      const encoded = parts.join('')
      try {
        const plaintext = decodeURIComponent(escape(atob(encoded)))
        const reassembled: ResponseMessage = JSON.parse(plaintext)
        console.log(`[NRC Client] Reassembled chunked message: ${reassembled.type}`)
        this.processResponse(reassembled)
      } catch (err) {
        console.error('[NRC Client] Failed to reassemble chunked message:', err)
      }

      // Clean up buffer
      this.chunkBuffers.delete(messageId)
    }

    // Clean up old buffers (older than 60 seconds)
    const now = Date.now()
    for (const [id, buf] of this.chunkBuffers) {
      if (now - buf.receivedAt > 60000) {
        console.warn(`[NRC Client] Discarding stale chunk buffer: ${id}`)
        this.chunkBuffers.delete(id)
      }
    }
  }

  /**
   * Process a complete response message
   */
  private processResponse(response: ResponseMessage): void {
    this.updateActivity()

    switch (response.type) {
      case 'EVENT': {
        // Extract the event from payload: ["EVENT", subId, eventObject]
        const [, , syncedEvent] = response.payload as [string, string, Event]
        if (syncedEvent) {
          this.pendingEvents.push(syncedEvent)
          this.onProgress?.({
            phase: 'receiving',
            eventsReceived: this.pendingEvents.length,
            message: `Received ${this.pendingEvents.length} events...`
          })
        }
        break
      }
      case 'EOSE': {
        console.log(`[NRC Client] EOSE received, got ${this.pendingEvents.length} events`)
        this.complete()
        break
      }
      case 'NOTICE': {
        const [, message] = response.payload as [string, string]
        console.log(`[NRC Client] Notice: ${message}`)
        this.onProgress?.({
          phase: 'error',
          eventsReceived: this.pendingEvents.length,
          message: message
        })
        break
      }
      case 'OK': {
        // Response to EVENT publish
        if (this.sendingEvents) {
          const [, eventId, success, message] = response.payload as [string, string, boolean, string]
          if (success) {
            this.eventsSentCount++
            console.log(`[NRC Client] Event ${eventId?.slice(0, 8)} stored successfully`)
          } else {
            console.warn(`[NRC Client] Event ${eventId?.slice(0, 8)} failed: ${message}`)
          }
          // Send next event
          this.sendNextEvent()
        }
        break
      }
      case 'IDS': {
        // Response to IDS request - contains event manifest
        if (this.idsMode) {
          const [, , manifest] = response.payload as [string, string, EventManifestEntry[]]
          console.log(`[NRC Client] Received IDS response with ${manifest?.length || 0} entries`)
          this.clearSyncTimeout()
          this.resolveIDs?.(manifest || [])
          this.disconnect()
        }
        break
      }
      default:
        console.log(`[NRC Client] Unknown response type: ${response.type}`)
    }
  }

  /**
   * Complete the sync operation
   */
  private complete(): void {
    this.clearSyncTimeout()

    this.onProgress?.({
      phase: 'complete',
      eventsReceived: this.pendingEvents.length,
      message: `Synced ${this.pendingEvents.length} events`
    })

    this.resolveSync?.(this.pendingEvents)
    this.disconnect()
  }

  /**
   * Disconnect from the rendezvous relay
   */
  disconnect(): void {
    this.clearSyncTimeout()

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.connected = false
  }
}

/**
 * Sync events from a remote device
 *
 * @param connectionUri - The nostr+relayconnect:// URI
 * @param filters - Nostr filters for events to sync
 * @param onProgress - Optional progress callback
 * @returns Array of synced events
 */
export async function syncFromRemote(
  connectionUri: string,
  filters: Filter[],
  onProgress?: (progress: SyncProgress) => void
): Promise<Event[]> {
  const client = new NRCClient(connectionUri)
  return client.sync(filters, onProgress)
}

/**
 * Test connection to a remote device
 * Performs a minimal sync (kind 0 with limit 1) to verify the connection works
 *
 * @param connectionUri - The nostr+relayconnect:// URI
 * @param onProgress - Optional progress callback
 * @returns true if connection successful
 */
export async function testConnection(
  connectionUri: string,
  onProgress?: (progress: SyncProgress) => void
): Promise<boolean> {
  const client = new NRCClient(connectionUri)
  try {
    // Request just one profile event to test the full round-trip
    const events = await client.sync(
      [{ kinds: [0], limit: 1 }],
      onProgress,
      15000 // 15 second timeout for test
    )
    console.log(`[NRC] Test connection successful, received ${events.length} events`)
    return true
  } catch (err) {
    console.error('[NRC] Test connection failed:', err)
    throw err
  }
}

/**
 * Request event IDs from a remote device (for diffing)
 *
 * @param connectionUri - The nostr+relayconnect:// URI
 * @param filters - Filters to match events
 * @param onProgress - Optional progress callback
 * @returns Array of event manifest entries (id, kind, created_at, d)
 */
export async function requestRemoteIDs(
  connectionUri: string,
  filters: Filter[],
  onProgress?: (progress: SyncProgress) => void
): Promise<EventManifestEntry[]> {
  const client = new NRCClient(connectionUri)
  return client.requestIDs(filters, onProgress)
}

/**
 * Send events to a remote device
 *
 * @param connectionUri - The nostr+relayconnect:// URI
 * @param events - Events to send
 * @param onProgress - Optional progress callback
 * @returns Number of events successfully stored
 */
export async function sendEventsToRemote(
  connectionUri: string,
  events: Event[],
  onProgress?: (progress: SyncProgress) => void
): Promise<number> {
  const client = new NRCClient(connectionUri)
  return client.sendEvents(events, onProgress)
}
