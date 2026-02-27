import { describe, it, expect } from 'vitest'
import { Feed } from './Feed'
import { FeedType } from './FeedType'
import { ContentFilter } from './ContentFilter'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'
import { FeedSwitched, ContentFilterUpdated, FeedRefreshed } from './events'

describe('Feed', () => {
  // Test data
  const ownerPubkey = Pubkey.fromHex(
    'a'.repeat(64)
  )
  const relayUrl1 = RelayUrl.tryCreate('wss://relay1.example.com')!
  const relayUrl2 = RelayUrl.tryCreate('wss://relay2.example.com')!

  describe('factory methods', () => {
    it('creates a following feed', () => {
      const feed = Feed.following(ownerPubkey)

      expect(feed.owner).toEqual(ownerPubkey)
      expect(feed.type.value).toBe('following')
      expect(feed.isSocialFeed).toBe(true)
      expect(feed.isRelayFeed).toBe(false)
      expect(feed.relayUrls).toEqual([])
      expect(feed.lastRefreshedAt).toBeNull()
    })

    it('creates a pinned feed', () => {
      const feed = Feed.pinned(ownerPubkey)

      expect(feed.owner).toEqual(ownerPubkey)
      expect(feed.type.value).toBe('pinned')
      expect(feed.isSocialFeed).toBe(true)
      expect(feed.isRelayFeed).toBe(false)
    })

    it('creates a relay set feed', () => {
      const relays = [relayUrl1, relayUrl2]
      const feed = Feed.relays(ownerPubkey, 'my-set', relays)

      expect(feed.owner).toEqual(ownerPubkey)
      expect(feed.type.value).toBe('relays')
      expect(feed.type.relaySetId).toBe('my-set')
      expect(feed.isSocialFeed).toBe(false)
      expect(feed.isRelayFeed).toBe(true)
      expect(feed.relayUrls).toHaveLength(2)
      expect(feed.hasRelayUrls).toBe(true)
    })

    it('creates a single relay feed', () => {
      const feed = Feed.singleRelay(relayUrl1)

      expect(feed.owner).toBeNull()
      expect(feed.type.value).toBe('relay')
      expect(feed.type.relayUrl).toBe(relayUrl1.value) // Use the actual normalized URL
      expect(feed.isSocialFeed).toBe(false)
      expect(feed.isRelayFeed).toBe(true)
      expect(feed.relayUrls).toHaveLength(1)
    })

    it('creates an empty feed', () => {
      const feed = Feed.empty()

      expect(feed.owner).toBeNull()
      expect(feed.type.value).toBe('following')
      expect(feed.relayUrls).toEqual([])
    })
  })

  describe('switchTo', () => {
    it('switches from following to relay feed', () => {
      const feed = Feed.following(ownerPubkey)
      const newType = FeedType.relay(relayUrl1.value)

      const event = feed.switchTo(newType, [relayUrl1])

      expect(event).toBeInstanceOf(FeedSwitched)
      expect(event.fromType?.value).toBe('following')
      expect(event.toType.value).toBe('relay')
      expect(feed.type.value).toBe('relay')
      expect(feed.relayUrls).toHaveLength(1)
      expect(feed.lastRefreshedAt).not.toBeNull()
    })

    it('switches to relay set feed', () => {
      const feed = Feed.following(ownerPubkey)
      const newType = FeedType.relays('my-set')
      const relays = [relayUrl1, relayUrl2]

      const event = feed.switchTo(newType, relays)

      expect(event.toType.value).toBe('relays')
      expect(event.relaySetId).toBe('my-set')
      expect(feed.relayUrls).toHaveLength(2)
    })

    it('switches to social feed and clears relay URLs', () => {
      const feed = Feed.singleRelay(relayUrl1)
      const newType = FeedType.following()

      feed.switchTo(newType)

      expect(feed.type.value).toBe('following')
      expect(feed.relayUrls).toEqual([])
    })
  })

  describe('updateContentFilter', () => {
    it('updates content filter and returns event', () => {
      const feed = Feed.following(ownerPubkey)
      const newFilter = ContentFilter.default().withHideReplies(true)

      const event = feed.updateContentFilter(newFilter)

      expect(event).toBeInstanceOf(ContentFilterUpdated)
      expect(feed.contentFilter.hideReplies).toBe(true)
    })
  })

  describe('refresh', () => {
    it('marks feed as refreshed and returns event', () => {
      const feed = Feed.following(ownerPubkey)
      expect(feed.lastRefreshedAt).toBeNull()

      const event = feed.refresh()

      expect(event).toBeInstanceOf(FeedRefreshed)
      expect(event.feedType.value).toBe('following')
      expect(feed.lastRefreshedAt).not.toBeNull()
    })
  })

  describe('buildTimelineQuery', () => {
    it('returns null for social feed without authors', () => {
      const feed = Feed.following(ownerPubkey)
      feed.setResolvedRelayUrls([relayUrl1])

      const query = feed.buildTimelineQuery()

      expect(query).toBeNull()
    })

    it('builds query for social feed with authors', () => {
      const feed = Feed.following(ownerPubkey)
      feed.setResolvedRelayUrls([relayUrl1])
      const author = Pubkey.fromHex('b'.repeat(64))

      const query = feed.buildTimelineQuery({ authors: [author] })

      expect(query).not.toBeNull()
      expect(query!.authors).toHaveLength(1)
    })

    it('returns null when no relay URLs are resolved', () => {
      const feed = Feed.following(ownerPubkey)

      const query = feed.buildTimelineQuery({ authors: [ownerPubkey] })

      expect(query).toBeNull()
    })

    it('builds query for relay feed', () => {
      const feed = Feed.singleRelay(relayUrl1)

      const query = feed.buildTimelineQuery()

      expect(query).not.toBeNull()
      expect(query!.relays).toHaveLength(1)
    })
  })

  describe('toState/fromState', () => {
    it('serializes and deserializes following feed', () => {
      const feed = Feed.following(ownerPubkey)
      feed.setResolvedRelayUrls([relayUrl1])
      feed.refresh()

      const state = feed.toState()
      const restored = Feed.fromState(state, ownerPubkey)

      expect(restored.type.value).toBe('following')
      expect(restored.relayUrls).toHaveLength(1)
      expect(restored.lastRefreshedAt).not.toBeNull()
    })

    it('serializes and deserializes relay set feed', () => {
      const feed = Feed.relays(ownerPubkey, 'test-set', [relayUrl1, relayUrl2])

      const state = feed.toState()
      const restored = Feed.fromState(state, ownerPubkey)

      expect(restored.type.value).toBe('relays')
      expect(restored.type.relaySetId).toBe('test-set')
      expect(restored.relayUrls).toHaveLength(2)
    })

    it('handles invalid state gracefully', () => {
      const invalidState = {
        feedType: 'invalid',
        relayUrls: [],
        contentFilter: {
          hideMutedUsers: true,
          hideContentMentioningMuted: false,
          hideUntrustedUsers: false,
          hideReplies: false,
          hideReposts: false,
          allowedKinds: [],
          nsfwPolicy: 'hide'
        }
      }

      const restored = Feed.fromState(invalidState)

      expect(restored.type.value).toBe('following') // Falls back to empty/following
    })
  })

  describe('withOwner', () => {
    it('creates a copy with new owner', () => {
      const feed = Feed.singleRelay(relayUrl1)
      const newOwner = Pubkey.fromHex('c'.repeat(64))

      const feedWithOwner = feed.withOwner(newOwner)

      expect(feedWithOwner.owner).toEqual(newOwner)
      expect(feedWithOwner.type.value).toBe('relay')
      expect(feedWithOwner.relayUrls).toHaveLength(1)
    })
  })

  describe('equals', () => {
    it('returns true for identical feeds', () => {
      const feed1 = Feed.following(ownerPubkey)
      const feed2 = Feed.following(ownerPubkey)

      expect(feed1.equals(feed2)).toBe(true)
    })

    it('returns false for different feed types', () => {
      const feed1 = Feed.following(ownerPubkey)
      const feed2 = Feed.pinned(ownerPubkey)

      expect(feed1.equals(feed2)).toBe(false)
    })

    it('returns false for different relay URLs', () => {
      const feed1 = Feed.relays(ownerPubkey, 'set', [relayUrl1])
      const feed2 = Feed.relays(ownerPubkey, 'set', [relayUrl1, relayUrl2])

      expect(feed1.equals(feed2)).toBe(false)
    })
  })
})
