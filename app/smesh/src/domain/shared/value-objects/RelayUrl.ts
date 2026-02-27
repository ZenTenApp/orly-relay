import { InvalidRelayUrlError } from '../errors'

/**
 * Value object representing a normalized Nostr relay WebSocket URL.
 * Immutable, self-validating, with equality by value.
 */
export class RelayUrl {
  private constructor(private readonly _value: string) {}

  /**
   * Create a RelayUrl from a WebSocket URL string.
   * Normalizes the URL (lowercases, removes trailing slash).
   * @throws InvalidRelayUrlError if the URL is invalid
   */
  static create(url: string): RelayUrl {
    const normalized = RelayUrl.normalize(url)
    if (!normalized) {
      throw new InvalidRelayUrlError(url)
    }
    return new RelayUrl(normalized)
  }

  /**
   * Try to create a RelayUrl. Returns null if invalid.
   */
  static tryCreate(url: string): RelayUrl | null {
    try {
      return RelayUrl.create(url)
    } catch {
      return null
    }
  }

  /**
   * Check if a string is a valid WebSocket URL.
   */
  static isValid(url: string): boolean {
    return RelayUrl.normalize(url) !== null
  }

  private static normalize(url: string): string | null {
    try {
      const trimmed = url.trim()
      if (!trimmed) return null

      const parsed = new URL(trimmed)
      if (!['ws:', 'wss:'].includes(parsed.protocol)) {
        return null
      }

      // Normalize: lowercase host, remove trailing slash, keep path
      let normalized = `${parsed.protocol}//${parsed.host.toLowerCase()}`
      if (parsed.pathname && parsed.pathname !== '/') {
        normalized += parsed.pathname.replace(/\/$/, '')
      }

      return normalized
    } catch {
      return null
    }
  }

  /** The normalized URL string */
  get value(): string {
    return this._value
  }

  /** The URL without the protocol prefix */
  get shortForm(): string {
    return this._value.replace(/^wss?:\/\//, '')
  }

  /** Whether this is a secure (wss://) connection */
  get isSecure(): boolean {
    return this._value.startsWith('wss:')
  }

  /** Whether this is a Tor onion address */
  get isOnion(): boolean {
    return this._value.includes('.onion')
  }

  /** Whether this is a local network address */
  get isLocalNetwork(): boolean {
    return (
      this._value.includes('localhost') ||
      this._value.includes('127.0.0.1') ||
      this._value.includes('192.168.') ||
      this._value.includes('10.') ||
      this._value.includes('172.16.')
    )
  }

  /** Extract the hostname from the URL */
  get hostname(): string {
    try {
      return new URL(this._value).hostname
    } catch {
      return this._value
    }
  }

  /** Check equality with another RelayUrl */
  equals(other: RelayUrl): boolean {
    return this._value === other._value
  }

  /** Returns the URL string */
  toString(): string {
    return this._value
  }

  /** For JSON serialization */
  toJSON(): string {
    return this._value
  }
}
