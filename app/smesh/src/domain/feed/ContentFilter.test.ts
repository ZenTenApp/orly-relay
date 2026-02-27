import { describe, it, expect } from 'vitest'
import { ContentFilter } from './ContentFilter'
import type { Event } from 'nostr-tools'

describe('ContentFilter', () => {
  // Helper to create mock events
  const createEvent = (overrides: Partial<Event> = {}): Event => ({
    id: 'a'.repeat(64),
    pubkey: 'b'.repeat(64),
    created_at: Math.floor(Date.now() / 1000),
    kind: 1,
    tags: [],
    content: 'test content',
    sig: 'c'.repeat(128),
    ...overrides
  })

  describe('factory methods', () => {
    it('creates default filter with sensible defaults', () => {
      const filter = ContentFilter.default()

      expect(filter.hideMutedUsers).toBe(true)
      expect(filter.hideContentMentioningMuted).toBe(true)
      expect(filter.hideUntrustedUsers).toBe(false)
      expect(filter.hideReplies).toBe(false)
      expect(filter.hideReposts).toBe(false)
      expect(filter.allowedKinds).toEqual([])
      expect(filter.nsfwPolicy).toBe('hide_content')
    })

    it('creates filter from preferences', () => {
      const filter = ContentFilter.fromPreferences({
        hideMutedUsers: false,
        hideReplies: true,
        nsfwPolicy: 'show'
      })

      expect(filter.hideMutedUsers).toBe(false)
      expect(filter.hideReplies).toBe(true)
      expect(filter.nsfwPolicy).toBe('show')
    })

    it('uses defaults for missing preferences', () => {
      const filter = ContentFilter.fromPreferences({})

      expect(filter.hideMutedUsers).toBe(true)
      expect(filter.nsfwPolicy).toBe('hide_content')
    })
  })

  describe('isKindAllowed', () => {
    it('allows all kinds when allowedKinds is empty', () => {
      const filter = ContentFilter.default()

      expect(filter.isKindAllowed(1)).toBe(true)
      expect(filter.isKindAllowed(6)).toBe(true)
      expect(filter.isKindAllowed(30023)).toBe(true)
    })

    it('only allows specified kinds', () => {
      const filter = ContentFilter.default().withAllowedKinds([1, 6])

      expect(filter.isKindAllowed(1)).toBe(true)
      expect(filter.isKindAllowed(6)).toBe(true)
      expect(filter.isKindAllowed(7)).toBe(false)
    })
  })

  describe('shouldShow', () => {
    const mutedPubkeys = new Set(['muted'.repeat(8)])
    const trustedPubkeys = new Set(['trusted'.repeat(8)])
    const deletedEventIds = new Set(['deleted'.repeat(8)])

    it('shows normal events', () => {
      const filter = ContentFilter.default()
      const event = createEvent()
      const context = { mutedPubkeys: new Set<string>() }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(true)
    })

    it('hides events from muted authors', () => {
      const filter = ContentFilter.default()
      const event = createEvent({ pubkey: 'muted'.repeat(8) })
      const context = { mutedPubkeys }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('muted_author')
    })

    it('shows events from muted authors when hideMutedUsers is false', () => {
      const filter = ContentFilter.default().withHideMutedUsers(false)
      const event = createEvent({ pubkey: 'muted'.repeat(8) })
      const context = { mutedPubkeys }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(true)
    })

    it('hides events mentioning muted users', () => {
      const filter = ContentFilter.default()
      const event = createEvent({
        tags: [['p', 'muted'.repeat(8)]]
      })
      const context = { mutedPubkeys }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('mentions_muted_user')
    })

    it('hides deleted events', () => {
      const filter = ContentFilter.default()
      const event = createEvent({ id: 'deleted'.repeat(8) })
      const context = { mutedPubkeys: new Set<string>(), deletedEventIds }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('deleted')
    })

    it('hides untrusted authors when enabled', () => {
      const filter = ContentFilter.default().withHideUntrustedUsers(true)
      const event = createEvent({ pubkey: 'stranger'.repeat(8) })
      const context = { mutedPubkeys: new Set<string>(), trustedPubkeys }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('untrusted_author')
    })

    it('shows trusted authors when hiding untrusted', () => {
      const filter = ContentFilter.default().withHideUntrustedUsers(true)
      const event = createEvent({ pubkey: 'trusted'.repeat(8) })
      const context = { mutedPubkeys: new Set<string>(), trustedPubkeys }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(true)
    })

    it('hides replies when enabled', () => {
      const filter = ContentFilter.default().withHideReplies(true)
      const event = createEvent({
        tags: [['e', 'someevent'.repeat(8), '', 'reply']]
      })
      const context = { mutedPubkeys: new Set<string>() }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('reply_filtered')
    })

    it('hides reposts when enabled', () => {
      const filter = ContentFilter.default().withHideReposts(true)
      const event = createEvent({ kind: 6 })
      const context = { mutedPubkeys: new Set<string>() }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('repost_filtered')
    })

    it('hides events with disallowed kinds', () => {
      const filter = ContentFilter.default().withAllowedKinds([1])
      const event = createEvent({ kind: 6 })
      const context = { mutedPubkeys: new Set<string>() }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(false)
      expect(result.reason).toBe('kind_not_allowed')
    })

    it('shows pinned events even from muted authors', () => {
      const filter = ContentFilter.default()
      const eventId = 'pinned'.repeat(8)
      const event = createEvent({ id: eventId, pubkey: 'muted'.repeat(8) })
      const context = {
        mutedPubkeys,
        pinnedEventIds: new Set([eventId])
      }

      const result = filter.shouldShow(event, context)

      expect(result.shouldShow).toBe(true)
    })
  })

  describe('immutable modifications', () => {
    it('withHideMutedUsers returns new instance', () => {
      const filter1 = ContentFilter.default()
      const filter2 = filter1.withHideMutedUsers(false)

      expect(filter1.hideMutedUsers).toBe(true)
      expect(filter2.hideMutedUsers).toBe(false)
    })

    it('withHideReplies returns new instance', () => {
      const filter1 = ContentFilter.default()
      const filter2 = filter1.withHideReplies(true)

      expect(filter1.hideReplies).toBe(false)
      expect(filter2.hideReplies).toBe(true)
    })

    it('withAllowedKinds returns new instance', () => {
      const filter1 = ContentFilter.default()
      const filter2 = filter1.withAllowedKinds([1, 6, 7])

      expect(filter1.allowedKinds).toEqual([])
      expect(filter2.allowedKinds).toEqual([1, 6, 7])
    })

    it('withNsfwPolicy returns new instance', () => {
      const filter1 = ContentFilter.default()
      const filter2 = filter1.withNsfwPolicy('show')

      expect(filter1.nsfwPolicy).toBe('hide_content')
      expect(filter2.nsfwPolicy).toBe('show')
    })
  })

  describe('equals', () => {
    it('returns true for identical filters', () => {
      const filter1 = ContentFilter.default()
      const filter2 = ContentFilter.default()

      expect(filter1.equals(filter2)).toBe(true)
    })

    it('returns false for different settings', () => {
      const filter1 = ContentFilter.default()
      const filter2 = ContentFilter.default().withHideReplies(true)

      expect(filter1.equals(filter2)).toBe(false)
    })

    it('returns false for different allowed kinds', () => {
      const filter1 = ContentFilter.default().withAllowedKinds([1])
      const filter2 = ContentFilter.default().withAllowedKinds([1, 6])

      expect(filter1.equals(filter2)).toBe(false)
    })
  })
})
