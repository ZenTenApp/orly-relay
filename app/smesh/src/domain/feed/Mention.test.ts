import { describe, it, expect } from 'vitest'
import { Mention, MentionList } from './Mention'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

describe('Mention', () => {
  const pubkey1 = Pubkey.fromHex('a'.repeat(64))
  const pubkey2 = Pubkey.fromHex('b'.repeat(64))
  const relayUrl = RelayUrl.tryCreate('wss://relay.example.com')!

  describe('factory methods', () => {
    it('creates tag mention', () => {
      const mention = Mention.tag(pubkey1)

      expect(mention.pubkey).toEqual(pubkey1)
      expect(mention.type).toBe('tag')
      expect(mention.isExplicitTag).toBe(true)
      expect(mention.isInline).toBe(false)
      expect(mention.isFromContext).toBe(false)
    })

    it('creates tag mention with relay hint', () => {
      const mention = Mention.tag(pubkey1, relayUrl)

      expect(mention.relayHint).toEqual(relayUrl)
    })

    it('creates inline mention', () => {
      const mention = Mention.inline(pubkey1, 'Alice')

      expect(mention.type).toBe('inline')
      expect(mention.displayName).toBe('Alice')
      expect(mention.isInline).toBe(true)
    })

    it('creates reply author mention', () => {
      const mention = Mention.replyAuthor(pubkey1, relayUrl)

      expect(mention.type).toBe('reply_author')
      expect(mention.isFromContext).toBe(true)
    })

    it('creates quote author mention', () => {
      const mention = Mention.quoteAuthor(pubkey1)

      expect(mention.type).toBe('quote_author')
      expect(mention.isFromContext).toBe(true)
    })
  })

  describe('parseFromContent', () => {
    it('extracts npub mentions', () => {
      const npub = pubkey1.npub
      const content = `Hey nostr:${npub} check this out!`

      const mentions = Mention.parseFromContent(content)

      expect(mentions).toHaveLength(1)
      expect(mentions[0].pubkey.hex).toBe(pubkey1.hex)
      expect(mentions[0].isInline).toBe(true)
    })

    it('extracts multiple mentions', () => {
      const npub1 = pubkey1.npub
      const npub2 = pubkey2.npub
      const content = `nostr:${npub1} and nostr:${npub2} are cool`

      const mentions = Mention.parseFromContent(content)

      expect(mentions).toHaveLength(2)
    })

    it('deduplicates mentions of same pubkey', () => {
      const npub = pubkey1.npub
      const content = `nostr:${npub} says nostr:${npub} is great`

      const mentions = Mention.parseFromContent(content)

      expect(mentions).toHaveLength(1)
    })

    it('handles invalid bech32 gracefully', () => {
      const content = 'nostr:npub1invalid and nostr:npub1alsobad'

      const mentions = Mention.parseFromContent(content)

      expect(mentions).toHaveLength(0)
    })

    it('returns empty array for content without mentions', () => {
      const content = 'Just a regular post without mentions'

      const mentions = Mention.parseFromContent(content)

      expect(mentions).toHaveLength(0)
    })
  })

  describe('toNostrUri', () => {
    it('returns npub URI without relay hint', () => {
      const mention = Mention.inline(pubkey1)

      const uri = mention.toNostrUri()

      expect(uri).toBe(`nostr:${pubkey1.npub}`)
    })

    it('returns nprofile URI with relay hint', () => {
      const mention = Mention.tag(pubkey1, relayUrl)

      const uri = mention.toNostrUri()

      expect(uri).toContain('nostr:nprofile1')
    })
  })

  describe('toTag', () => {
    it('generates p tag without relay', () => {
      const mention = Mention.inline(pubkey1)

      const tag = mention.toTag()

      expect(tag).toEqual(['p', pubkey1.hex])
    })

    it('generates p tag with relay hint', () => {
      const mention = Mention.tag(pubkey1, relayUrl)

      const tag = mention.toTag()

      expect(tag).toEqual(['p', pubkey1.hex, relayUrl.value])
    })
  })

  describe('immutable modifications', () => {
    it('withRelayHint returns new instance', () => {
      const original = Mention.inline(pubkey1)
      const modified = original.withRelayHint(relayUrl)

      expect(original.relayHint).toBeNull()
      expect(modified.relayHint).toEqual(relayUrl)
    })

    it('withDisplayName returns new instance', () => {
      const original = Mention.inline(pubkey1)
      const modified = original.withDisplayName('Bob')

      expect(original.displayName).toBeNull()
      expect(modified.displayName).toBe('Bob')
    })
  })

  describe('equals', () => {
    it('returns true for same pubkey', () => {
      const a = Mention.tag(pubkey1)
      const b = Mention.inline(pubkey1) // Different type but same pubkey

      expect(a.equals(b)).toBe(true)
    })

    it('returns false for different pubkeys', () => {
      const a = Mention.tag(pubkey1)
      const b = Mention.tag(pubkey2)

      expect(a.equals(b)).toBe(false)
    })
  })

  describe('hasSamePubkey', () => {
    it('returns true for matching pubkey', () => {
      const mention = Mention.tag(pubkey1)

      expect(mention.hasSamePubkey(pubkey1)).toBe(true)
    })

    it('returns false for different pubkey', () => {
      const mention = Mention.tag(pubkey1)

      expect(mention.hasSamePubkey(pubkey2)).toBe(false)
    })
  })
})

describe('MentionList', () => {
  const pubkey1 = Pubkey.fromHex('a'.repeat(64))
  const pubkey2 = Pubkey.fromHex('b'.repeat(64))
  const pubkey3 = Pubkey.fromHex('c'.repeat(64))

  describe('factory methods', () => {
    it('creates empty list', () => {
      const list = MentionList.empty()

      expect(list.isEmpty).toBe(true)
      expect(list.length).toBe(0)
    })

    it('creates from mentions with deduplication', () => {
      const mentions = [
        Mention.tag(pubkey1),
        Mention.inline(pubkey2),
        Mention.tag(pubkey1) // Duplicate
      ]

      const list = MentionList.from(mentions)

      expect(list.length).toBe(2)
    })
  })

  describe('add', () => {
    it('adds new mention', () => {
      const list = MentionList.empty().add(Mention.tag(pubkey1))

      expect(list.length).toBe(1)
      expect(list.contains(pubkey1)).toBe(true)
    })

    it('does not add duplicate', () => {
      const list = MentionList.empty()
        .add(Mention.tag(pubkey1))
        .add(Mention.inline(pubkey1))

      expect(list.length).toBe(1)
    })
  })

  describe('remove', () => {
    it('removes mention by pubkey', () => {
      const list = MentionList.from([Mention.tag(pubkey1), Mention.tag(pubkey2)]).remove(
        pubkey1
      )

      expect(list.length).toBe(1)
      expect(list.contains(pubkey1)).toBe(false)
      expect(list.contains(pubkey2)).toBe(true)
    })
  })

  describe('contains', () => {
    it('returns true if pubkey is in list', () => {
      const list = MentionList.from([Mention.tag(pubkey1)])

      expect(list.contains(pubkey1)).toBe(true)
      expect(list.contains(pubkey2)).toBe(false)
    })
  })

  describe('pubkeys', () => {
    it('returns all pubkeys', () => {
      const list = MentionList.from([Mention.tag(pubkey1), Mention.tag(pubkey2)])

      const pubkeys = list.pubkeys

      expect(pubkeys).toHaveLength(2)
      expect(pubkeys.map((p) => p.hex)).toContain(pubkey1.hex)
      expect(pubkeys.map((p) => p.hex)).toContain(pubkey2.hex)
    })
  })

  describe('toTags', () => {
    it('generates p tags for all mentions', () => {
      const list = MentionList.from([Mention.tag(pubkey1), Mention.tag(pubkey2)])

      const tags = list.toTags()

      expect(tags).toHaveLength(2)
      expect(tags[0][0]).toBe('p')
      expect(tags[1][0]).toBe('p')
    })
  })

  describe('merge', () => {
    it('merges two lists with deduplication', () => {
      const list1 = MentionList.from([Mention.tag(pubkey1), Mention.tag(pubkey2)])
      const list2 = MentionList.from([Mention.tag(pubkey2), Mention.tag(pubkey3)])

      const merged = list1.merge(list2)

      expect(merged.length).toBe(3)
      expect(merged.contains(pubkey1)).toBe(true)
      expect(merged.contains(pubkey2)).toBe(true)
      expect(merged.contains(pubkey3)).toBe(true)
    })
  })
})
