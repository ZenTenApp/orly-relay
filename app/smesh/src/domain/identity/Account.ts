import { Pubkey } from '../shared'
import { SignerType, SignerTypeValue } from './SignerType'

/**
 * Credentials for different signer types
 */
export type AccountCredentials = {
  ncryptsec?: string
  nsec?: string
}

/**
 * Account Entity
 *
 * Represents a user account in the application.
 * An account is identified by a public key and a signer type.
 *
 * Note: The same pubkey with different signer types are considered
 * different accounts (e.g., you might have nsec and nip-07 for the same key).
 */
export class Account {
  private constructor(
    private readonly _pubkey: Pubkey,
    private readonly _signerType: SignerType,
    private readonly _credentials: AccountCredentials
  ) {}

  /**
   * Create a new account
   */
  static create(
    pubkey: Pubkey,
    signerType: SignerType,
    credentials: AccountCredentials = {}
  ): Account {
    return new Account(pubkey, signerType, { ...credentials })
  }

  /**
   * Create an account from raw values
   */
  static fromRaw(
    pubkeyHex: string,
    signerTypeValue: SignerTypeValue,
    credentials: AccountCredentials = {}
  ): Account {
    const pubkey = Pubkey.fromHex(pubkeyHex)
    const signerType = SignerType.fromString(signerTypeValue)
    return new Account(pubkey, signerType, { ...credentials })
  }

  /**
   * Try to create an account from raw values
   */
  static tryFromRaw(
    pubkeyHex: string,
    signerTypeValue: string,
    credentials: AccountCredentials = {}
  ): Account | null {
    try {
      const pubkey = Pubkey.tryFromString(pubkeyHex)
      if (!pubkey) return null

      const signerType = SignerType.tryFromString(signerTypeValue)
      if (!signerType) return null

      return new Account(pubkey, signerType, { ...credentials })
    } catch {
      return null
    }
  }

  /**
   * Create from legacy TAccount format
   */
  static fromLegacy(legacy: {
    pubkey: string
    signerType: SignerTypeValue
    ncryptsec?: string
    nsec?: string
    npub?: string
  }): Account | null {
    const pubkey = Pubkey.tryFromString(legacy.pubkey)
    if (!pubkey) return null

    const signerType = SignerType.tryFromString(legacy.signerType)
    if (!signerType) return null

    return new Account(pubkey, signerType, {
      ncryptsec: legacy.ncryptsec,
      nsec: legacy.nsec
    })
  }

  /**
   * The account's public key
   */
  get pubkey(): Pubkey {
    return this._pubkey
  }

  /**
   * The signer type used by this account
   */
  get signerType(): SignerType {
    return this._signerType
  }

  /**
   * Whether this account can sign events
   */
  get canSign(): boolean {
    return this._signerType.canSign
  }

  /**
   * Whether this is a view-only account
   */
  get isViewOnly(): boolean {
    return this._signerType.isViewOnly
  }

  /**
   * Get the ncryptsec credential (if available)
   */
  get ncryptsec(): string | undefined {
    return this._credentials.ncryptsec
  }

  /**
   * Create a unique identifier for this account
   * Combination of pubkey and signer type
   */
  get id(): string {
    return `${this._pubkey.hex}:${this._signerType.value}`
  }

  /**
   * Check if this is the same account (same pubkey and signer type)
   */
  equals(other: Account): boolean {
    return this._pubkey.equals(other._pubkey) && this._signerType.equals(other._signerType)
  }

  /**
   * Check if this account has the same pubkey as another
   */
  hasSamePubkey(other: Account): boolean {
    return this._pubkey.equals(other._pubkey)
  }

  /**
   * Convert to legacy TAccount format
   */
  toLegacy(): {
    pubkey: string
    signerType: SignerTypeValue
    ncryptsec?: string
    nsec?: string
    npub?: string
  } {
    return {
      pubkey: this._pubkey.hex,
      signerType: this._signerType.value,
      ncryptsec: this._credentials.ncryptsec,
      nsec: this._credentials.nsec,
      npub: this._signerType.isViewOnly ? this._pubkey.npub : undefined
    }
  }

  /**
   * Create an account pointer (pubkey + signer type, no credentials)
   */
  toPointer(): { pubkey: string; signerType: SignerTypeValue } {
    return {
      pubkey: this._pubkey.hex,
      signerType: this._signerType.value
    }
  }

  /**
   * For JSON serialization (excludes sensitive credentials)
   */
  toJSON(): { pubkey: string; signerType: string } {
    return {
      pubkey: this._pubkey.hex,
      signerType: this._signerType.value
    }
  }
}
