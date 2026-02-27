import { ISigner, TDraftEvent } from '@/types'
import * as utils from '@noble/curves/abstract/utils'
import { bech32 } from '@scure/base'
import { finalizeEvent, getPublicKey as nGetPublicKey, nip04 } from 'nostr-tools'
import * as nip44 from 'nostr-tools/nip44'

/**
 * Convert nsec (bech32) to hex string
 */
function nsecToHex(nsec: string): string {
  const { prefix, words } = bech32.decode(nsec as `${string}1${string}`, 5000)
  if (prefix !== 'nsec') {
    throw new Error('Invalid nsec prefix')
  }
  const data = new Uint8Array(bech32.fromWords(words))
  return utils.bytesToHex(data)
}

/**
 * Normalize a private key to hex format.
 * Accepts nsec (bech32) or hex string.
 */
function normalizeToHex(privateKey: string): string {
  if (privateKey.startsWith('nsec')) {
    return nsecToHex(privateKey)
  }
  return privateKey
}

/**
 * Validate that a hex key is exactly 64 characters.
 */
function validateHexKey(hex: string): void {
  if (!/^[0-9a-fA-F]{64}$/.test(hex)) {
    throw new Error('Private key must be 64 hex characters')
  }
}

export class NsecSigner implements ISigner {
  private privkey: Uint8Array | null = null
  private pubkey: string | null = null

  login(nsecOrPrivkey: string | Uint8Array) {
    let privkey: Uint8Array

    if (typeof nsecOrPrivkey === 'string') {
      try {
        const hex = normalizeToHex(nsecOrPrivkey)
        validateHexKey(hex)
        privkey = utils.hexToBytes(hex)
      } catch (error) {
        throw new Error(
          `Invalid private key: ${error instanceof Error ? error.message : 'unknown error'}`
        )
      }
    } else {
      privkey = nsecOrPrivkey
    }

    this.privkey = privkey
    this.pubkey = nGetPublicKey(privkey)
    return this.pubkey
  }

  async getPublicKey() {
    if (!this.pubkey) {
      throw new Error('Not logged in')
    }
    return this.pubkey
  }

  async signEvent(draftEvent: TDraftEvent) {
    if (!this.privkey) {
      throw new Error('Not logged in')
    }

    return finalizeEvent(draftEvent, this.privkey)
  }

  async nip04Encrypt(pubkey: string, plainText: string) {
    if (!this.privkey) {
      throw new Error('Not logged in')
    }
    return nip04.encrypt(this.privkey, pubkey, plainText)
  }

  async nip04Decrypt(pubkey: string, cipherText: string) {
    if (!this.privkey) {
      throw new Error('Not logged in')
    }
    return nip04.decrypt(this.privkey, pubkey, cipherText)
  }

  async nip44Encrypt(pubkey: string, plainText: string) {
    if (!this.privkey) {
      throw new Error('Not logged in')
    }
    const conversationKey = nip44.v2.utils.getConversationKey(this.privkey, pubkey)
    return nip44.v2.encrypt(plainText, conversationKey)
  }

  async nip44Decrypt(pubkey: string, cipherText: string) {
    if (!this.privkey) {
      throw new Error('Not logged in')
    }
    const conversationKey = nip44.v2.utils.getConversationKey(this.privkey, pubkey)
    return nip44.v2.decrypt(cipherText, conversationKey)
  }
}
