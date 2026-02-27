import { Pubkey, Timestamp } from '@/domain/shared'
import { FollowList } from '@/domain/social'
import { MuteList } from '@/domain/social'
import { RelayList } from '@/domain/relay'
import { kinds } from 'nostr-tools'

/**
 * Draft event structure (matches TDraftEvent)
 */
export type DraftEvent = {
  kind: number
  content: string
  created_at: number
  tags: string[][]
}

/**
 * Options for publishing a note
 */
export type PublishNoteOptions = {
  parentEventId?: string
  mentions?: Pubkey[]
  hashtags?: string[]
  isNsfw?: boolean
  addClientTag?: boolean
}

/**
 * PublishingService Domain Service
 *
 * Handles creation of draft events and publishing logic.
 * This service encapsulates the business rules for event creation.
 */
export class PublishingService {
  /**
   * Create a draft note event
   */
  createNoteDraft(
    content: string,
    options: PublishNoteOptions = {}
  ): DraftEvent {
    const tags: string[][] = []

    // Add mention tags
    if (options.mentions) {
      for (const pubkey of options.mentions) {
        tags.push(['p', pubkey.hex])
      }
    }

    // Add hashtags
    if (options.hashtags) {
      for (const hashtag of options.hashtags) {
        tags.push(['t', hashtag.toLowerCase()])
      }
    }

    // Add NSFW warning
    if (options.isNsfw) {
      tags.push(['content-warning', 'NSFW'])
    }

    // Add client tag
    if (options.addClientTag) {
      tags.push(['client', 'smesh'])
    }

    return {
      kind: kinds.ShortTextNote,
      content,
      created_at: Timestamp.now().unix,
      tags
    }
  }

  /**
   * Create a draft reaction event
   */
  createReactionDraft(
    targetEventId: string,
    targetPubkey: string,
    targetKind: number,
    emoji: string = '+'
  ): DraftEvent {
    const tags: string[][] = [
      ['e', targetEventId],
      ['p', targetPubkey]
    ]

    if (targetKind !== kinds.ShortTextNote) {
      tags.push(['k', targetKind.toString()])
    }

    return {
      kind: kinds.Reaction,
      content: emoji,
      created_at: Timestamp.now().unix,
      tags
    }
  }

  /**
   * Create a draft repost event
   */
  createRepostDraft(
    targetEventId: string,
    targetPubkey: string,
    embeddedContent?: string
  ): DraftEvent {
    return {
      kind: kinds.Repost,
      content: embeddedContent || '',
      created_at: Timestamp.now().unix,
      tags: [
        ['e', targetEventId],
        ['p', targetPubkey]
      ]
    }
  }

  /**
   * Create a draft follow list event from a FollowList aggregate
   */
  createFollowListDraft(followList: FollowList): DraftEvent {
    return followList.toDraftEvent()
  }

  /**
   * Create a draft mute list event from a MuteList aggregate
   * Note: The caller must handle encryption of private mutes
   */
  createMuteListDraft(muteList: MuteList, encryptedPrivateMutes: string = ''): DraftEvent {
    return muteList.toDraftEvent(encryptedPrivateMutes)
  }

  /**
   * Create a draft relay list event from a RelayList aggregate
   */
  createRelayListDraft(relayList: RelayList): DraftEvent {
    return relayList.toDraftEvent()
  }

  /**
   * Extract mentions from note content
   * Finds npub1, nprofile1, and @mentions
   */
  extractMentionsFromContent(content: string): Pubkey[] {
    const mentions: Pubkey[] = []
    const seen = new Set<string>()

    // Match npub1 and nprofile1
    const bech32Regex = /n(?:pub1|profile1)[0-9a-z]{58,}/gi
    const matches = content.match(bech32Regex) || []

    for (const match of matches) {
      const pubkey = Pubkey.tryFromString(match)
      if (pubkey && !seen.has(pubkey.hex)) {
        seen.add(pubkey.hex)
        mentions.push(pubkey)
      }
    }

    return mentions
  }

  /**
   * Extract hashtags from note content
   */
  extractHashtagsFromContent(content: string): string[] {
    const hashtags: string[] = []
    const matches = content.match(/#[\p{L}\p{N}\p{M}]+/gu) || []

    for (const match of matches) {
      const hashtag = match.slice(1).toLowerCase()
      if (hashtag && !hashtags.includes(hashtag)) {
        hashtags.push(hashtag)
      }
    }

    return hashtags
  }
}

/**
 * Singleton instance of the publishing service
 */
export const publishingService = new PublishingService()
