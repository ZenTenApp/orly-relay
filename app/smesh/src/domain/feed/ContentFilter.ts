import { Event } from 'nostr-tools'

/**
 * NSFW display policy options
 */
export type NsfwDisplayPolicy = 'hide' | 'hide_content' | 'show'

/**
 * Context required for filtering decisions
 */
export interface FilterContext {
  mutedPubkeys: Set<string>
  trustedPubkeys?: Set<string>
  deletedEventIds?: Set<string>
  currentUserPubkey?: string
  pinnedEventIds?: Set<string>
}

/**
 * Result of a filter check with reason
 */
export type FilterResult = {
  shouldShow: boolean
  reason?: FilterReason
}

/**
 * Reason why an event was filtered
 */
export type FilterReason =
  | 'muted_author'
  | 'mentions_muted_user'
  | 'untrusted_author'
  | 'deleted'
  | 'reply_filtered'
  | 'repost_filtered'
  | 'nsfw_hidden'
  | 'kind_not_allowed'

/**
 * ContentFilter Value Object
 *
 * Encapsulates all filtering criteria for timeline content.
 * Immutable - all modifications return new instances.
 */
export class ContentFilter {
  private constructor(
    private readonly _hideMutedUsers: boolean,
    private readonly _hideContentMentioningMuted: boolean,
    private readonly _hideUntrustedUsers: boolean,
    private readonly _hideReplies: boolean,
    private readonly _hideReposts: boolean,
    private readonly _allowedKinds: readonly number[],
    private readonly _nsfwPolicy: NsfwDisplayPolicy
  ) {}

  /**
   * Create default content filter with sensible defaults
   */
  static default(): ContentFilter {
    return new ContentFilter(
      true, // hideMutedUsers
      true, // hideContentMentioningMuted
      false, // hideUntrustedUsers
      false, // hideReplies
      false, // hideReposts
      [], // allowedKinds (empty = allow all)
      'hide_content' // nsfwPolicy
    )
  }

  /**
   * Create filter from user preferences
   */
  static fromPreferences(prefs: {
    hideMutedUsers?: boolean
    hideContentMentioningMuted?: boolean
    hideUntrustedUsers?: boolean
    hideReplies?: boolean
    hideReposts?: boolean
    allowedKinds?: number[]
    nsfwPolicy?: NsfwDisplayPolicy
  }): ContentFilter {
    return new ContentFilter(
      prefs.hideMutedUsers ?? true,
      prefs.hideContentMentioningMuted ?? true,
      prefs.hideUntrustedUsers ?? false,
      prefs.hideReplies ?? false,
      prefs.hideReposts ?? false,
      prefs.allowedKinds ?? [],
      prefs.nsfwPolicy ?? 'hide_content'
    )
  }

  // Getters
  get hideMutedUsers(): boolean {
    return this._hideMutedUsers
  }

  get hideContentMentioningMuted(): boolean {
    return this._hideContentMentioningMuted
  }

  get hideUntrustedUsers(): boolean {
    return this._hideUntrustedUsers
  }

  get hideReplies(): boolean {
    return this._hideReplies
  }

  get hideReposts(): boolean {
    return this._hideReposts
  }

  get allowedKinds(): readonly number[] {
    return this._allowedKinds
  }

  get nsfwPolicy(): NsfwDisplayPolicy {
    return this._nsfwPolicy
  }

  /**
   * Check if a kind is allowed by this filter
   */
  isKindAllowed(kind: number): boolean {
    // Empty array means all kinds allowed
    if (this._allowedKinds.length === 0) return true
    return this._allowedKinds.includes(kind)
  }

  /**
   * Check if an event should be shown based on this filter and context
   */
  shouldShow(event: Event, context: FilterContext): FilterResult {
    // Check kind filter first
    if (!this.isKindAllowed(event.kind)) {
      return { shouldShow: false, reason: 'kind_not_allowed' }
    }

    // Check if event is pinned (pinned events bypass most filters)
    if (context.pinnedEventIds?.has(event.id)) {
      return { shouldShow: true }
    }

    // Check deleted
    if (context.deletedEventIds?.has(event.id)) {
      return { shouldShow: false, reason: 'deleted' }
    }

    // Check muted author
    if (this._hideMutedUsers && context.mutedPubkeys.has(event.pubkey)) {
      return { shouldShow: false, reason: 'muted_author' }
    }

    // Check if content mentions muted users
    if (this._hideContentMentioningMuted) {
      const mentionedPubkeys = this.extractMentionedPubkeys(event)
      for (const pk of mentionedPubkeys) {
        if (context.mutedPubkeys.has(pk)) {
          return { shouldShow: false, reason: 'mentions_muted_user' }
        }
      }
    }

    // Check untrusted
    if (this._hideUntrustedUsers && context.trustedPubkeys) {
      if (!context.trustedPubkeys.has(event.pubkey)) {
        return { shouldShow: false, reason: 'untrusted_author' }
      }
    }

    // Check reply filter
    if (this._hideReplies && this.isReply(event)) {
      return { shouldShow: false, reason: 'reply_filtered' }
    }

    // Check repost filter
    if (this._hideReposts && this.isRepost(event)) {
      return { shouldShow: false, reason: 'repost_filtered' }
    }

    return { shouldShow: true }
  }

  /**
   * Extract pubkeys mentioned in an event
   */
  private extractMentionedPubkeys(event: Event): string[] {
    const pubkeys: string[] = []
    for (const tag of event.tags) {
      if (tag[0] === 'p' && tag[1]) {
        pubkeys.push(tag[1])
      }
    }
    return pubkeys
  }

  /**
   * Check if event is a reply
   */
  private isReply(event: Event): boolean {
    // Check for 'e' or 'E' tags with reply marker, or just any 'e' tag
    for (const tag of event.tags) {
      if ((tag[0] === 'e' || tag[0] === 'E') && tag[1]) {
        // If marker is 'reply' or 'root', it's a reply
        if (tag[3] === 'reply' || tag[3] === 'root') {
          return true
        }
        // Legacy: any 'e' tag indicates reply
        return true
      }
    }
    return false
  }

  /**
   * Check if event is a repost
   */
  private isRepost(event: Event): boolean {
    return event.kind === 6 || event.kind === 16
  }

  // Immutable modification methods
  withHideMutedUsers(hide: boolean): ContentFilter {
    return new ContentFilter(
      hide,
      this._hideContentMentioningMuted,
      this._hideUntrustedUsers,
      this._hideReplies,
      this._hideReposts,
      this._allowedKinds,
      this._nsfwPolicy
    )
  }

  withHideContentMentioningMuted(hide: boolean): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      hide,
      this._hideUntrustedUsers,
      this._hideReplies,
      this._hideReposts,
      this._allowedKinds,
      this._nsfwPolicy
    )
  }

  withHideUntrustedUsers(hide: boolean): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      this._hideContentMentioningMuted,
      hide,
      this._hideReplies,
      this._hideReposts,
      this._allowedKinds,
      this._nsfwPolicy
    )
  }

  withHideReplies(hide: boolean): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      this._hideContentMentioningMuted,
      this._hideUntrustedUsers,
      hide,
      this._hideReposts,
      this._allowedKinds,
      this._nsfwPolicy
    )
  }

  withHideReposts(hide: boolean): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      this._hideContentMentioningMuted,
      this._hideUntrustedUsers,
      this._hideReplies,
      hide,
      this._allowedKinds,
      this._nsfwPolicy
    )
  }

  withAllowedKinds(kinds: number[]): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      this._hideContentMentioningMuted,
      this._hideUntrustedUsers,
      this._hideReplies,
      this._hideReposts,
      [...kinds],
      this._nsfwPolicy
    )
  }

  withNsfwPolicy(policy: NsfwDisplayPolicy): ContentFilter {
    return new ContentFilter(
      this._hideMutedUsers,
      this._hideContentMentioningMuted,
      this._hideUntrustedUsers,
      this._hideReplies,
      this._hideReposts,
      this._allowedKinds,
      policy
    )
  }

  equals(other: ContentFilter): boolean {
    if (this._hideMutedUsers !== other._hideMutedUsers) return false
    if (this._hideContentMentioningMuted !== other._hideContentMentioningMuted) return false
    if (this._hideUntrustedUsers !== other._hideUntrustedUsers) return false
    if (this._hideReplies !== other._hideReplies) return false
    if (this._hideReposts !== other._hideReposts) return false
    if (this._nsfwPolicy !== other._nsfwPolicy) return false
    if (this._allowedKinds.length !== other._allowedKinds.length) return false
    for (let i = 0; i < this._allowedKinds.length; i++) {
      if (this._allowedKinds[i] !== other._allowedKinds[i]) return false
    }
    return true
  }
}
