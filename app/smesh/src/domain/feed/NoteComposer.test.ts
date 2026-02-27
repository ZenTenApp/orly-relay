import { describe, it, expect } from 'vitest'
import { NoteComposer } from './NoteComposer'
import { ReplyContext } from './ReplyContext'
import { QuoteContext } from './QuoteContext'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { EventId } from '../shared/value-objects/EventId'
import { NoteCreated, NoteReplied, UsersMentioned } from './events'

describe('NoteComposer', () => {
  const authorPubkey = Pubkey.fromHex('a'.repeat(64))
  const otherPubkey = Pubkey.fromHex('b'.repeat(64))
  const eventIdHex = 'c'.repeat(64)

  describe('factory methods', () => {
    it('creates a new note composer', () => {
      const composer = NoteComposer.create(authorPubkey)

      expect(composer.author).toEqual(authorPubkey)
      expect(composer.content).toBe('')
      expect(composer.isReply).toBe(false)
      expect(composer.isQuote).toBe(false)
      expect(composer.isPoll).toBe(false)
      expect(composer.isNsfw).toBe(false)
    })

    it('creates a reply composer', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext)

      expect(composer.author).toEqual(authorPubkey)
      expect(composer.isReply).toBe(true)
      expect(composer.replyContext).not.toBeNull()
    })

    it('creates a quote composer', () => {
      const quoteContext = QuoteContext.create(eventIdHex, otherPubkey)
      const composer = NoteComposer.quote(authorPubkey, quoteContext)

      expect(composer.author).toEqual(authorPubkey)
      expect(composer.isQuote).toBe(true)
      expect(composer.quoteContext).not.toBeNull()
    })

    it('creates a poll composer', () => {
      const composer = NoteComposer.poll(authorPubkey)

      expect(composer.author).toEqual(authorPubkey)
      expect(composer.isPoll).toBe(true)
      expect(composer.pollConfig).not.toBeNull()
    })
  })

  describe('content management', () => {
    it('sets content immutably', () => {
      const composer1 = NoteComposer.create(authorPubkey)
      const composer2 = composer1.setContent('Hello, world!')

      expect(composer1.content).toBe('')
      expect(composer2.content).toBe('Hello, world!')
    })

    it('extracts hashtags from content', () => {
      const composer = NoteComposer.create(authorPubkey).setContent(
        'Hello #nostr and #bitcoin!'
      )

      expect(composer.hashtags).toEqual(['nostr', 'bitcoin'])
    })

    it('handles empty hashtags', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('No hashtags here')

      expect(composer.hashtags).toEqual([])
    })

    it('lowercases hashtags', () => {
      const composer = NoteComposer.create(authorPubkey).setContent(
        '#NOSTR #Bitcoin #Lightning'
      )

      expect(composer.hashtags).toEqual(['nostr', 'bitcoin', 'lightning'])
    })
  })

  describe('mentions', () => {
    it('adds mentions immutably', () => {
      const composer1 = NoteComposer.create(authorPubkey)
      const composer2 = composer1.addMention(otherPubkey)

      expect(composer1.additionalMentions).toHaveLength(0)
      expect(composer2.additionalMentions).toHaveLength(1)
    })

    it('prevents duplicate mentions', () => {
      const composer = NoteComposer.create(authorPubkey)
        .addMention(otherPubkey)
        .addMention(otherPubkey)

      expect(composer.additionalMentions).toHaveLength(1)
    })

    it('removes mentions', () => {
      const composer = NoteComposer.create(authorPubkey)
        .addMention(otherPubkey)
        .removeMention(otherPubkey)

      expect(composer.additionalMentions).toHaveLength(0)
    })

    it('includes reply context mentions in allMentions', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext)

      expect(composer.allMentions).toContainEqual(otherPubkey)
    })

    it('includes quote author in allMentions', () => {
      const quoteContext = QuoteContext.create(eventIdHex, otherPubkey)
      const composer = NoteComposer.quote(authorPubkey, quoteContext)

      expect(composer.allMentions).toContainEqual(otherPubkey)
    })

    it('deduplicates mentions from different sources', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext).addMention(
        otherPubkey
      )

      const mentions = composer.allMentions
      const pubkeyHexes = mentions.map((p) => p.hex)
      const uniqueHexes = new Set(pubkeyHexes)

      expect(uniqueHexes.size).toBe(pubkeyHexes.length)
    })
  })

  describe('options', () => {
    it('sets content warning', () => {
      const composer = NoteComposer.create(authorPubkey).setContentWarning(true)

      expect(composer.isNsfw).toBe(true)
      expect(composer.options.isNsfw).toBe(true)
    })

    it('sets client tag option', () => {
      const composer = NoteComposer.create(authorPubkey).setClientTag(true)

      expect(composer.options.addClientTag).toBe(true)
    })

    it('sets protected option', () => {
      const composer = NoteComposer.create(authorPubkey).setProtected(true)

      expect(composer.options.isProtected).toBe(true)
    })
  })

  describe('poll configuration', () => {
    it('enables poll with config', () => {
      const composer = NoteComposer.create(authorPubkey).enablePoll({
        isMultipleChoice: false,
        options: [
          { id: '1', text: 'Option 1' },
          { id: '2', text: 'Option 2' }
        ],
        relays: ['wss://relay.example.com']
      })

      expect(composer.isPoll).toBe(true)
      expect(composer.pollConfig?.options).toHaveLength(2)
    })

    it('disables poll', () => {
      const composer = NoteComposer.poll(authorPubkey).disablePoll()

      expect(composer.isPoll).toBe(false)
      expect(composer.pollConfig).toBeNull()
    })

    it('sets poll options', () => {
      const composer = NoteComposer.poll(authorPubkey).setPollOptions([
        { id: '1', text: 'Yes' },
        { id: '2', text: 'No' },
        { id: '3', text: 'Maybe' }
      ])

      expect(composer.pollConfig?.options).toHaveLength(3)
    })

    it('sets multiple choice mode', () => {
      const composer = NoteComposer.poll(authorPubkey).setPollMultipleChoice(true)

      expect(composer.pollConfig?.isMultipleChoice).toBe(true)
    })

    it('clears reply context when enabling poll', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext).enablePoll({
        isMultipleChoice: false,
        options: [],
        relays: []
      })

      expect(composer.isReply).toBe(false)
      expect(composer.replyContext).toBeNull()
    })
  })

  describe('effectiveContent', () => {
    it('returns plain content for regular notes', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('Hello')

      expect(composer.effectiveContent).toBe('Hello')
    })

    it('appends quote URI for quote posts', () => {
      const quoteContext = QuoteContext.create(eventIdHex, otherPubkey)
      const composer = NoteComposer.quote(authorPubkey, quoteContext).setContent('Check this out')

      expect(composer.effectiveContent).toContain('Check this out')
      expect(composer.effectiveContent).toContain('nostr:')
    })
  })

  describe('validation', () => {
    it('fails validation for empty content', () => {
      const composer = NoteComposer.create(authorPubkey)
      const result = composer.validate()

      expect(result.isValid).toBe(false)
      expect(result.errors).toContain('Content cannot be empty')
    })

    it('passes validation for non-empty content', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('Hello!')
      const result = composer.validate()

      expect(result.isValid).toBe(true)
      expect(result.errors).toHaveLength(0)
    })

    it('fails validation for poll without enough options', () => {
      const composer = NoteComposer.poll(authorPubkey)
        .setContent('Question?')
        .setPollOptions([{ id: '1', text: 'Only one option' }])

      const result = composer.validate()

      expect(result.isValid).toBe(false)
      expect(result.errors).toContain('Poll must have at least 2 options')
    })

    it('fails validation for poll without question', () => {
      const composer = NoteComposer.poll(authorPubkey).setPollOptions([
        { id: '1', text: 'Yes' },
        { id: '2', text: 'No' }
      ])

      const result = composer.validate()

      expect(result.isValid).toBe(false)
      expect(result.errors).toContain('Poll question cannot be empty')
    })

    it('passes validation for valid poll', () => {
      const composer = NoteComposer.poll(authorPubkey)
        .setContent('Do you like Nostr?')
        .setPollOptions([
          { id: '1', text: 'Yes' },
          { id: '2', text: 'No' }
        ])

      const result = composer.validate()

      expect(result.isValid).toBe(true)
    })

    it('canPublish reflects validation status', () => {
      const invalid = NoteComposer.create(authorPubkey)
      const valid = NoteComposer.create(authorPubkey).setContent('Hello!')

      expect(invalid.canPublish()).toBe(false)
      expect(valid.canPublish()).toBe(true)
    })
  })

  describe('domain events', () => {
    it('creates NoteCreated event', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('Hello #nostr')
      const noteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createNoteCreatedEvent(noteId)

      expect(event).toBeInstanceOf(NoteCreated)
      expect(event.author).toEqual(authorPubkey)
      expect(event.noteId).toEqual(noteId)
      expect(event.hashtags).toContain('nostr')
    })

    it('creates NoteCreated event with reply reference', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext).setContent('Reply')
      const noteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createNoteCreatedEvent(noteId)

      expect(event.replyTo).not.toBeNull()
    })

    it('creates NoteReplied event for replies', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext).setContent('Reply')
      const replyNoteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createNoteRepliedEvent(replyNoteId)

      expect(event).toBeInstanceOf(NoteReplied)
      expect(event?.replier).toEqual(authorPubkey)
    })

    it('returns null NoteReplied event for non-replies', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('Not a reply')
      const noteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createNoteRepliedEvent(noteId)

      expect(event).toBeNull()
    })

    it('creates UsersMentioned event when there are mentions', () => {
      const replyContext = ReplyContext.simple(eventIdHex, otherPubkey)
      const composer = NoteComposer.reply(authorPubkey, replyContext).setContent('Hey!')
      const noteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createUsersMentionedEvent(noteId)

      expect(event).toBeInstanceOf(UsersMentioned)
      expect(event?.mentionedPubkeys).toContainEqual(otherPubkey)
    })

    it('returns null UsersMentioned event when no mentions', () => {
      const composer = NoteComposer.create(authorPubkey).setContent('No mentions')
      const noteId = EventId.fromHex('d'.repeat(64))

      const event = composer.createUsersMentionedEvent(noteId)

      expect(event).toBeNull()
    })
  })
})
