import { nip19 } from 'nostr-tools'
import { InvalidPubkeyError } from '../errors'

/**
 * Value object representing a Nostr public key.
 * Immutable, self-validating, with equality by value.
 */
export class Pubkey {
  private constructor(private readonly _value: string) {}

  /**
   * Create a Pubkey from a 64-character hex string.
   * @throws InvalidPubkeyError if the hex is invalid
   */
  static fromHex(hex: string): Pubkey {
    if (!/^[0-9a-f]{64}$/.test(hex)) {
      throw new InvalidPubkeyError(hex)
    }
    return new Pubkey(hex)
  }

  /**
   * Create a Pubkey from an npub bech32 string.
   * @throws InvalidPubkeyError if the npub is invalid
   */
  static fromNpub(npub: string): Pubkey {
    try {
      const { type, data } = nip19.decode(npub)
      if (type !== 'npub') {
        throw new InvalidPubkeyError(npub)
      }
      return new Pubkey(data)
    } catch (e) {
      if (e instanceof InvalidPubkeyError) throw e
      throw new InvalidPubkeyError(npub)
    }
  }

  /**
   * Create a Pubkey from an nprofile bech32 string.
   * @throws InvalidPubkeyError if the nprofile is invalid
   */
  static fromNprofile(nprofile: string): Pubkey {
    try {
      const { type, data } = nip19.decode(nprofile)
      if (type !== 'nprofile') {
        throw new InvalidPubkeyError(nprofile)
      }
      return new Pubkey(data.pubkey)
    } catch (e) {
      if (e instanceof InvalidPubkeyError) throw e
      throw new InvalidPubkeyError(nprofile)
    }
  }

  /**
   * Try to create a Pubkey from any string format (hex, npub, nprofile).
   * Returns null if the string is invalid.
   */
  static tryFromString(input: string): Pubkey | null {
    try {
      if (input.startsWith('npub1')) {
        return Pubkey.fromNpub(input)
      }
      if (input.startsWith('nprofile1')) {
        return Pubkey.fromNprofile(input)
      }
      return Pubkey.fromHex(input)
    } catch {
      return null
    }
  }

  /**
   * Check if a string is a valid pubkey (hex format only).
   */
  static isValidHex(value: string): boolean {
    return /^[0-9a-f]{64}$/.test(value)
  }

  /** The raw 64-character hex value */
  get hex(): string {
    return this._value
  }

  /** The bech32-encoded npub representation */
  get npub(): string {
    return nip19.npubEncode(this._value)
  }

  /** A shortened display format: first 8 + ... + last 4 characters */
  get formatted(): string {
    return `${this._value.slice(0, 8)}...${this._value.slice(-4)}`
  }

  /** A shorter display format for the npub */
  formatNpub(length = 12): string {
    const npub = this.npub
    if (length < 12) length = 12
    if (length >= 63) return npub

    const prefixLength = Math.floor((length - 5) / 2) + 5
    const suffixLength = length - prefixLength
    return npub.slice(0, prefixLength) + '...' + npub.slice(-suffixLength)
  }

  /** Check equality with another Pubkey */
  equals(other: Pubkey): boolean {
    return this._value === other._value
  }

  /** Returns the hex representation */
  toString(): string {
    return this._value
  }

  /** For JSON serialization */
  toJSON(): string {
    return this._value
  }
}
