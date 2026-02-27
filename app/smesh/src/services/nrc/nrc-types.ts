import { Filter, Event } from 'nostr-tools'
import { ISigner } from '@/types'

// NRC Event Kinds
export const KIND_NRC_REQUEST = 24891
export const KIND_NRC_RESPONSE = 24892

// Session types
export interface NRCSession {
  id: string
  clientPubkey: string
  conversationKey?: Uint8Array // Optional - only set when using direct key access
  deviceName?: string
  createdAt: number
  lastActivity: number
  subscriptions: Map<string, NRCSubscription>
}

export interface NRCSubscription {
  id: string
  filters: Filter[]
  createdAt: number
  eventCount: number
  eoseSent: boolean
}

// Message types (encrypted content)
export interface RequestMessage {
  type: 'REQ' | 'CLOSE' | 'EVENT' | 'COUNT' | 'IDS'
  payload: unknown[]
}

export interface ResponseMessage {
  type: 'EVENT' | 'EOSE' | 'OK' | 'NOTICE' | 'CLOSED' | 'COUNT' | 'CHUNK' | 'IDS'
  payload: unknown[]
}

// ===== Sync Types =====

/**
 * Event manifest entry - describes an event we have
 * Used by IDS request/response for diffing
 */
export interface EventManifestEntry {
  kind: number
  id: string
  created_at: number
  d?: string // For parameterized replaceable events (kinds 30000-39999)
}

// Chunked message for large payloads
export interface ChunkMessage {
  type: 'CHUNK'
  messageId: string  // Unique ID for this chunked message
  index: number      // 0-based chunk index
  total: number      // Total number of chunks
  data: string       // Base64 encoded chunk data
}

// Helper to check if a message is a chunk
export function isChunkMessage(msg: ResponseMessage): msg is ResponseMessage & { payload: [ChunkMessage] } {
  return msg.type === 'CHUNK'
}

// Connection management
export interface NRCConnection {
  id: string
  label: string
  secret?: string // For secret-based auth
  clientPubkey?: string // Derived from secret
  createdAt: number
  lastUsed?: number
}

// Listener configuration
export interface NRCListenerConfig {
  rendezvousUrl: string
  signer: ISigner
  authorizedSecrets: Map<string, string> // clientPubkey → deviceName
  sessionTimeout?: number // Session inactivity timeout in ms (default 30 min)
  maxSubscriptionsPerSession?: number // Max subscriptions per session (default 100)
}

// Authorization result
export interface AuthResult {
  conversationKey?: Uint8Array // Optional - only set when using direct key access
  deviceName: string
}

// Parsed connection URI
export interface ParsedConnectionURI {
  relayPubkey: string // Hex pubkey of the listening relay/client
  rendezvousUrl: string // URL of the rendezvous relay
  // For secret-based auth
  secret?: string // 32-byte hex secret
  clientPubkey?: string // Derived pubkey from secret
  clientPrivkey?: Uint8Array // Derived private key from secret
  // Optional
  deviceName?: string
}

// Listener state for React context
export interface NRCListenerState {
  isEnabled: boolean
  isListening: boolean
  connections: NRCConnection[]
  activeSessions: number
  rendezvousUrl: string
}

// Event with simplified typing for storage queries
export type StoredEvent = Event

// Device-specific event check
export function isDeviceSpecificEvent(event: Event): boolean {
  const dTag = event.tags.find((t) => t[0] === 'd')?.[1]
  return dTag?.startsWith('device:') ?? false
}
