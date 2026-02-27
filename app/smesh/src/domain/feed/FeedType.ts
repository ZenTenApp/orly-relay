/**
 * FeedType Value Object
 *
 * Represents the type of feed being displayed.
 * Immutable and self-validating.
 */

export type FeedTypeValue = 'following' | 'pinned' | 'relays' | 'relay'

export class FeedType {
  private constructor(
    private readonly _value: FeedTypeValue,
    private readonly _relaySetId: string | null,
    private readonly _relayUrl: string | null
  ) {}

  /**
   * Create a following feed type (shows posts from followed users)
   */
  static following(): FeedType {
    return new FeedType('following', null, null)
  }

  /**
   * Create a pinned feed type (shows posts from pinned users)
   */
  static pinned(): FeedType {
    return new FeedType('pinned', null, null)
  }

  /**
   * Create a relay set feed type (shows posts from a group of relays)
   */
  static relays(setId: string): FeedType {
    if (!setId || setId.trim() === '') {
      throw new Error('Relay set ID cannot be empty')
    }
    return new FeedType('relays', setId, null)
  }

  /**
   * Create a single relay feed type (shows posts from one relay)
   */
  static relay(url: string): FeedType {
    if (!url || url.trim() === '') {
      throw new Error('Relay URL cannot be empty')
    }
    return new FeedType('relay', null, url)
  }

  /**
   * Parse from string representation
   */
  static tryFromString(value: string, id?: string): FeedType | null {
    switch (value) {
      case 'following':
        return FeedType.following()
      case 'pinned':
        return FeedType.pinned()
      case 'relays':
        return id ? FeedType.relays(id) : null
      case 'relay':
        return id ? FeedType.relay(id) : null
      default:
        return null
    }
  }

  get value(): FeedTypeValue {
    return this._value
  }

  get relaySetId(): string | null {
    return this._relaySetId
  }

  get relayUrl(): string | null {
    return this._relayUrl
  }

  /**
   * Check if this is a social feed (following or pinned)
   */
  get isSocialFeed(): boolean {
    return this._value === 'following' || this._value === 'pinned'
  }

  /**
   * Check if this is a relay-based feed
   */
  get isRelayFeed(): boolean {
    return this._value === 'relays' || this._value === 'relay'
  }

  equals(other: FeedType): boolean {
    if (this._value !== other._value) return false
    if (this._relaySetId !== other._relaySetId) return false
    if (this._relayUrl !== other._relayUrl) return false
    return true
  }

  toString(): string {
    switch (this._value) {
      case 'following':
        return 'following'
      case 'pinned':
        return 'pinned'
      case 'relays':
        return `relays:${this._relaySetId}`
      case 'relay':
        return `relay:${this._relayUrl}`
    }
  }
}
