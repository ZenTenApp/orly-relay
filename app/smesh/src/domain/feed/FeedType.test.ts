import { describe, it, expect } from 'vitest'
import { FeedType } from './FeedType'

describe('FeedType', () => {
  describe('factory methods', () => {
    it('creates a following feed type', () => {
      const feedType = FeedType.following()

      expect(feedType.value).toBe('following')
      expect(feedType.relaySetId).toBeNull()
      expect(feedType.relayUrl).toBeNull()
      expect(feedType.isSocialFeed).toBe(true)
      expect(feedType.isRelayFeed).toBe(false)
    })

    it('creates a pinned feed type', () => {
      const feedType = FeedType.pinned()

      expect(feedType.value).toBe('pinned')
      expect(feedType.isSocialFeed).toBe(true)
      expect(feedType.isRelayFeed).toBe(false)
    })

    it('creates a relay set feed type', () => {
      const feedType = FeedType.relays('my-set-id')

      expect(feedType.value).toBe('relays')
      expect(feedType.relaySetId).toBe('my-set-id')
      expect(feedType.relayUrl).toBeNull()
      expect(feedType.isSocialFeed).toBe(false)
      expect(feedType.isRelayFeed).toBe(true)
    })

    it('creates a single relay feed type', () => {
      const feedType = FeedType.relay('wss://relay.example.com')

      expect(feedType.value).toBe('relay')
      expect(feedType.relaySetId).toBeNull()
      expect(feedType.relayUrl).toBe('wss://relay.example.com')
      expect(feedType.isSocialFeed).toBe(false)
      expect(feedType.isRelayFeed).toBe(true)
    })

    it('throws for empty relay set ID', () => {
      expect(() => FeedType.relays('')).toThrow('Relay set ID cannot be empty')
      expect(() => FeedType.relays('   ')).toThrow('Relay set ID cannot be empty')
    })

    it('throws for empty relay URL', () => {
      expect(() => FeedType.relay('')).toThrow('Relay URL cannot be empty')
      expect(() => FeedType.relay('   ')).toThrow('Relay URL cannot be empty')
    })
  })

  describe('tryFromString', () => {
    it('parses following', () => {
      const feedType = FeedType.tryFromString('following')

      expect(feedType).not.toBeNull()
      expect(feedType!.value).toBe('following')
    })

    it('parses pinned', () => {
      const feedType = FeedType.tryFromString('pinned')

      expect(feedType).not.toBeNull()
      expect(feedType!.value).toBe('pinned')
    })

    it('parses relays with ID', () => {
      const feedType = FeedType.tryFromString('relays', 'set-id')

      expect(feedType).not.toBeNull()
      expect(feedType!.value).toBe('relays')
      expect(feedType!.relaySetId).toBe('set-id')
    })

    it('returns null for relays without ID', () => {
      const feedType = FeedType.tryFromString('relays')

      expect(feedType).toBeNull()
    })

    it('parses relay with URL', () => {
      const feedType = FeedType.tryFromString('relay', 'wss://relay.example.com')

      expect(feedType).not.toBeNull()
      expect(feedType!.value).toBe('relay')
      expect(feedType!.relayUrl).toBe('wss://relay.example.com')
    })

    it('returns null for relay without URL', () => {
      const feedType = FeedType.tryFromString('relay')

      expect(feedType).toBeNull()
    })

    it('returns null for unknown type', () => {
      const feedType = FeedType.tryFromString('unknown')

      expect(feedType).toBeNull()
    })
  })

  describe('equals', () => {
    it('returns true for identical following types', () => {
      const type1 = FeedType.following()
      const type2 = FeedType.following()

      expect(type1.equals(type2)).toBe(true)
    })

    it('returns false for different types', () => {
      const type1 = FeedType.following()
      const type2 = FeedType.pinned()

      expect(type1.equals(type2)).toBe(false)
    })

    it('returns true for same relay set ID', () => {
      const type1 = FeedType.relays('same-id')
      const type2 = FeedType.relays('same-id')

      expect(type1.equals(type2)).toBe(true)
    })

    it('returns false for different relay set IDs', () => {
      const type1 = FeedType.relays('id-1')
      const type2 = FeedType.relays('id-2')

      expect(type1.equals(type2)).toBe(false)
    })

    it('returns true for same relay URL', () => {
      const type1 = FeedType.relay('wss://same.relay.com')
      const type2 = FeedType.relay('wss://same.relay.com')

      expect(type1.equals(type2)).toBe(true)
    })

    it('returns false for different relay URLs', () => {
      const type1 = FeedType.relay('wss://relay1.com')
      const type2 = FeedType.relay('wss://relay2.com')

      expect(type1.equals(type2)).toBe(false)
    })
  })

  describe('toString', () => {
    it('returns simple string for following', () => {
      expect(FeedType.following().toString()).toBe('following')
    })

    it('returns simple string for pinned', () => {
      expect(FeedType.pinned().toString()).toBe('pinned')
    })

    it('returns relays:id format for relay sets', () => {
      expect(FeedType.relays('my-set').toString()).toBe('relays:my-set')
    })

    it('returns relay:url format for single relay', () => {
      expect(FeedType.relay('wss://relay.com').toString()).toBe('relay:wss://relay.com')
    })
  })
})
