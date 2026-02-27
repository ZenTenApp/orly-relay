/**
 * SignerType Value Object
 *
 * Represents the type of signer/authentication method used for an account.
 */

const VALID_SIGNER_TYPES = ['nsec', 'nip-07', 'browser-nsec', 'ncryptsec', 'npub', 'bunker'] as const

export type SignerTypeValue = (typeof VALID_SIGNER_TYPES)[number]

/**
 * SignerType Value Object
 *
 * Represents how a user authenticates and signs events.
 * - nsec: Raw private key (stored securely)
 * - nip-07: Browser extension (nos2x, Alby, etc.)
 * - browser-nsec: Private key in browser storage (less secure)
 * - ncryptsec: Encrypted private key (NIP-49)
 * - npub: View-only mode (no signing capability)
 */
export class SignerType {
  private constructor(private readonly _value: SignerTypeValue) {}

  static readonly NSEC = new SignerType('nsec')
  static readonly NIP07 = new SignerType('nip-07')
  static readonly BROWSER_NSEC = new SignerType('browser-nsec')
  static readonly NCRYPTSEC = new SignerType('ncryptsec')
  static readonly NPUB = new SignerType('npub')
  static readonly BUNKER = new SignerType('bunker')

  /**
   * Create a SignerType from a string value
   */
  static fromString(value: string): SignerType {
    if (!SignerType.isValid(value)) {
      throw new Error(`Invalid signer type: ${value}`)
    }
    return new SignerType(value as SignerTypeValue)
  }

  /**
   * Try to create a SignerType from a string, returns null if invalid
   */
  static tryFromString(value: string): SignerType | null {
    try {
      return SignerType.fromString(value)
    } catch {
      return null
    }
  }

  /**
   * Check if a string is a valid signer type
   */
  static isValid(value: string): value is SignerTypeValue {
    return VALID_SIGNER_TYPES.includes(value as SignerTypeValue)
  }

  /**
   * Get all valid signer types
   */
  static all(): SignerType[] {
    return VALID_SIGNER_TYPES.map((v) => new SignerType(v))
  }

  /**
   * The raw string value
   */
  get value(): SignerTypeValue {
    return this._value
  }

  /**
   * Whether this signer can sign events
   */
  get canSign(): boolean {
    return this._value !== 'npub'
  }

  /**
   * Whether this signer stores keys locally
   */
  get storesKeysLocally(): boolean {
    return ['nsec', 'browser-nsec', 'ncryptsec'].includes(this._value)
  }

  /**
   * Whether this signer uses a remote/external service
   */
  get isRemote(): boolean {
    return this._value === 'nip-07' || this._value === 'bunker'
  }

  /**
   * Whether this is view-only mode
   */
  get isViewOnly(): boolean {
    return this._value === 'npub'
  }

  /**
   * Human-readable display name
   */
  get displayName(): string {
    switch (this._value) {
      case 'nsec':
        return 'Private Key'
      case 'nip-07':
        return 'Browser Extension'
      case 'browser-nsec':
        return 'Browser Key'
      case 'ncryptsec':
        return 'Encrypted Key'
      case 'npub':
        return 'View Only'
      case 'bunker':
        return 'Remote Signer'
    }
  }

  /**
   * Check equality with another SignerType
   */
  equals(other: SignerType): boolean {
    return this._value === other._value
  }

  /**
   * Returns the string value
   */
  toString(): string {
    return this._value
  }

  /**
   * For JSON serialization
   */
  toJSON(): string {
    return this._value
  }
}
