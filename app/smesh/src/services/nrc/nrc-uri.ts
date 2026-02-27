import * as utils from '@noble/curves/abstract/utils'
import { getPublicKey } from 'nostr-tools'
import * as nip44 from 'nostr-tools/nip44'
import { ParsedConnectionURI } from './nrc-types'

/**
 * Generate a random 32-byte secret as hex string
 */
export function generateSecret(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return utils.bytesToHex(bytes)
}

/**
 * Derive a keypair from a 32-byte secret
 * Returns the private key bytes and public key hex
 */
export function deriveKeypairFromSecret(secretHex: string): {
  privkey: Uint8Array
  pubkey: string
} {
  const privkey = utils.hexToBytes(secretHex)
  const pubkey = getPublicKey(privkey)
  return { privkey, pubkey }
}

/**
 * Derive conversation key for NIP-44 encryption
 */
export function deriveConversationKey(
  ourPrivkey: Uint8Array,
  theirPubkey: string
): Uint8Array {
  return nip44.v2.utils.getConversationKey(ourPrivkey, theirPubkey)
}

/**
 * Generate a secret-based NRC connection URI
 *
 * @param relayPubkey - The public key of the listening client/relay
 * @param rendezvousUrl - The URL of the rendezvous relay
 * @param secret - Optional 32-byte hex secret (generated if not provided)
 * @param deviceName - Optional device name for identification
 * @returns The connection URI and the secret used
 */
export function generateConnectionURI(
  relayPubkey: string,
  rendezvousUrl: string,
  secret?: string,
  deviceName?: string
): { uri: string; secret: string; clientPubkey: string } {
  const secretHex = secret || generateSecret()
  const { pubkey: clientPubkey } = deriveKeypairFromSecret(secretHex)

  // Build URI
  const params = new URLSearchParams()
  params.set('relay', rendezvousUrl)
  params.set('secret', secretHex)
  if (deviceName) {
    params.set('name', deviceName)
  }

  const uri = `nostr+relayconnect://${relayPubkey}?${params.toString()}`

  return { uri, secret: secretHex, clientPubkey }
}

/**
 * Parse an NRC connection URI
 *
 * @param uri - The nostr+relayconnect:// URI to parse
 * @returns Parsed connection parameters
 * @throws Error if URI is invalid
 */
export function parseConnectionURI(uri: string): ParsedConnectionURI {
  // Parse as URL
  let url: URL
  try {
    url = new URL(uri)
  } catch {
    throw new Error('Invalid URI format')
  }

  // Validate scheme
  if (url.protocol !== 'nostr+relayconnect:') {
    throw new Error('Invalid URI scheme, expected nostr+relayconnect://')
  }

  // Extract relay pubkey from host (should be 64 hex chars)
  const relayPubkey = url.hostname
  if (!/^[0-9a-fA-F]{64}$/.test(relayPubkey)) {
    throw new Error('Invalid relay pubkey in URI')
  }

  // Extract rendezvous relay URL
  const rendezvousUrl = url.searchParams.get('relay')
  if (!rendezvousUrl) {
    throw new Error('Missing relay parameter in URI')
  }

  // Validate rendezvous URL
  try {
    new URL(rendezvousUrl)
  } catch {
    throw new Error('Invalid rendezvous relay URL')
  }

  // Extract device name (optional)
  const deviceName = url.searchParams.get('name') || undefined

  // Secret-based auth
  const secret = url.searchParams.get('secret')
  if (!secret) {
    throw new Error('Missing secret parameter in URI')
  }

  // Validate secret format (64 hex chars = 32 bytes)
  if (!/^[0-9a-fA-F]{64}$/.test(secret)) {
    throw new Error('Invalid secret format, expected 64 hex characters')
  }

  // Derive keypair from secret
  const { privkey, pubkey } = deriveKeypairFromSecret(secret)

  return {
    relayPubkey,
    rendezvousUrl,
    secret,
    clientPubkey: pubkey,
    clientPrivkey: privkey,
    deviceName
  }
}

/**
 * Validate a connection URI without fully parsing it
 * Returns true if the URI appears valid, false otherwise
 */
export function isValidConnectionURI(uri: string): boolean {
  try {
    parseConnectionURI(uri)
    return true
  } catch {
    return false
  }
}
