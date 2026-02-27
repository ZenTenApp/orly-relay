import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

/**
 * Strategy types for relay selection
 */
export type RelayStrategyType =
  | 'user_write_relays' // Use owner's write relays
  | 'user_read_relays' // Use owner's read relays
  | 'author_write_relays' // Use each author's write relays (NIP-65 optimization)
  | 'specific_relays' // Use a specific relay set
  | 'single_relay' // Use a single relay
  | 'big_relays' // Use fallback big relays

/**
 * Interface for resolving relay lists for pubkeys
 */
export interface RelayListResolver {
  getWriteRelays(pubkey: Pubkey): Promise<RelayUrl[]>
  getReadRelays(pubkey: Pubkey): Promise<RelayUrl[]>
  getBigRelays(): RelayUrl[]
}

/**
 * RelayStrategy Value Object
 *
 * Determines which relays to query for a given feed configuration.
 * Immutable and encapsulates relay selection logic.
 */
export class RelayStrategy {
  private constructor(
    private readonly _type: RelayStrategyType,
    private readonly _relays: readonly RelayUrl[],
    private readonly _relaySetId: string | null
  ) {}

  /**
   * Use the current user's write relays
   */
  static userWriteRelays(): RelayStrategy {
    return new RelayStrategy('user_write_relays', [], null)
  }

  /**
   * Use the current user's read relays
   */
  static userReadRelays(): RelayStrategy {
    return new RelayStrategy('user_read_relays', [], null)
  }

  /**
   * Use each author's write relays (for optimized following feeds)
   */
  static authorWriteRelays(): RelayStrategy {
    return new RelayStrategy('author_write_relays', [], null)
  }

  /**
   * Use specific relays from a relay set
   */
  static specific(relays: RelayUrl[], setId?: string): RelayStrategy {
    if (relays.length === 0) {
      throw new Error('Specific relay strategy requires at least one relay')
    }
    return new RelayStrategy('specific_relays', [...relays], setId ?? null)
  }

  /**
   * Use a single relay
   */
  static single(relay: RelayUrl): RelayStrategy {
    return new RelayStrategy('single_relay', [relay], null)
  }

  /**
   * Use fallback big relays
   */
  static bigRelays(): RelayStrategy {
    return new RelayStrategy('big_relays', [], null)
  }

  /**
   * Create from relay URLs (convenience factory)
   */
  static fromUrls(urls: string[], setId?: string): RelayStrategy {
    const relays = urls
      .map((url) => RelayUrl.tryCreate(url))
      .filter((r): r is RelayUrl => r !== null)

    if (relays.length === 0) {
      return RelayStrategy.bigRelays()
    }

    if (relays.length === 1) {
      return RelayStrategy.single(relays[0])
    }

    return RelayStrategy.specific(relays, setId)
  }

  get type(): RelayStrategyType {
    return this._type
  }

  get relays(): readonly RelayUrl[] {
    return this._relays
  }

  get relaySetId(): string | null {
    return this._relaySetId
  }

  /**
   * Check if this strategy has static relays (doesn't need resolution)
   */
  get hasStaticRelays(): boolean {
    return (
      this._type === 'specific_relays' ||
      this._type === 'single_relay' ||
      this._type === 'big_relays'
    )
  }

  /**
   * Check if this strategy requires per-author relay resolution
   */
  get requiresPerAuthorResolution(): boolean {
    return this._type === 'author_write_relays'
  }

  /**
   * Resolve relay URLs based on the strategy
   *
   * For static strategies, returns the configured relays.
   * For dynamic strategies, uses the resolver to look up relays.
   */
  async resolve(
    resolver: RelayListResolver,
    ownerPubkey?: Pubkey
  ): Promise<RelayUrl[]> {
    switch (this._type) {
      case 'specific_relays':
      case 'single_relay':
        return [...this._relays]

      case 'big_relays':
        return resolver.getBigRelays()

      case 'user_write_relays':
        if (!ownerPubkey) {
          return resolver.getBigRelays()
        }
        return resolver.getWriteRelays(ownerPubkey)

      case 'user_read_relays':
        if (!ownerPubkey) {
          return resolver.getBigRelays()
        }
        return resolver.getReadRelays(ownerPubkey)

      case 'author_write_relays':
        // This requires per-author resolution, return empty
        // The caller should use resolveForAuthors instead
        return []
    }
  }

  /**
   * Resolve relay URLs for multiple authors (for optimized subscriptions)
   *
   * Returns a map of relay URL -> list of pubkeys to query at that relay.
   * This enables NIP-65 mailbox-style optimized queries.
   */
  async resolveForAuthors(
    resolver: RelayListResolver,
    authors: Pubkey[]
  ): Promise<Map<string, Pubkey[]>> {
    const relayToAuthors = new Map<string, Pubkey[]>()

    if (this._type !== 'author_write_relays') {
      // For non-author strategies, resolve once and map all authors
      const relays = await this.resolve(resolver)
      for (const relay of relays) {
        relayToAuthors.set(relay.value, [...authors])
      }
      return relayToAuthors
    }

    // For author_write_relays, resolve per author
    const bigRelays = resolver.getBigRelays()

    for (const author of authors) {
      let authorRelays = await resolver.getWriteRelays(author)

      // Fall back to big relays if no write relays found
      if (authorRelays.length === 0) {
        authorRelays = bigRelays
      }

      for (const relay of authorRelays) {
        const existing = relayToAuthors.get(relay.value)
        if (existing) {
          existing.push(author)
        } else {
          relayToAuthors.set(relay.value, [author])
        }
      }
    }

    return relayToAuthors
  }

  equals(other: RelayStrategy): boolean {
    if (this._type !== other._type) return false
    if (this._relaySetId !== other._relaySetId) return false
    if (this._relays.length !== other._relays.length) return false
    for (let i = 0; i < this._relays.length; i++) {
      if (!this._relays[i].equals(other._relays[i])) return false
    }
    return true
  }

  toString(): string {
    switch (this._type) {
      case 'user_write_relays':
        return 'user_write_relays'
      case 'user_read_relays':
        return 'user_read_relays'
      case 'author_write_relays':
        return 'author_write_relays'
      case 'big_relays':
        return 'big_relays'
      case 'single_relay':
        return `single:${this._relays[0]?.value}`
      case 'specific_relays':
        return this._relaySetId
          ? `set:${this._relaySetId}`
          : `specific:[${this._relays.map((r) => r.value).join(',')}]`
    }
  }
}
