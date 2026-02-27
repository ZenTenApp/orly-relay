import { Pubkey, DomainEvent } from '../shared'
import { MuteVisibility } from './MuteList'

// Re-export DomainEvent for backward compatibility
export { DomainEvent }

// ============================================================================
// Follow List Events
// ============================================================================

/**
 * Raised when a user follows another user
 */
export class UserFollowed extends DomainEvent {
  readonly eventType = 'social.user_followed'

  constructor(
    readonly actor: Pubkey,
    readonly followed: Pubkey,
    readonly relayHint?: string,
    readonly petname?: string
  ) {
    super()
  }
}

/**
 * Raised when a user unfollows another user
 */
export class UserUnfollowed extends DomainEvent {
  readonly eventType = 'social.user_unfollowed'

  constructor(
    readonly actor: Pubkey,
    readonly unfollowed: Pubkey
  ) {
    super()
  }
}

/**
 * Raised when a follow list is published
 */
export class FollowListPublished extends DomainEvent {
  readonly eventType = 'social.follow_list_published'

  constructor(
    readonly owner: Pubkey,
    readonly followingCount: number
  ) {
    super()
  }
}

// ============================================================================
// Mute List Events
// ============================================================================

/**
 * Raised when a user mutes another user
 */
export class UserMuted extends DomainEvent {
  readonly eventType = 'social.user_muted'

  constructor(
    readonly actor: Pubkey,
    readonly muted: Pubkey,
    readonly visibility: MuteVisibility
  ) {
    super()
  }
}

/**
 * Raised when a user unmutes another user
 */
export class UserUnmuted extends DomainEvent {
  readonly eventType = 'social.user_unmuted'

  constructor(
    readonly actor: Pubkey,
    readonly unmuted: Pubkey
  ) {
    super()
  }
}

/**
 * Raised when mute visibility is changed (public to private or vice versa)
 */
export class MuteVisibilityChanged extends DomainEvent {
  readonly eventType = 'social.mute_visibility_changed'

  constructor(
    readonly actor: Pubkey,
    readonly target: Pubkey,
    readonly from: MuteVisibility,
    readonly to: MuteVisibility
  ) {
    super()
  }
}

/**
 * Raised when a mute list is published
 */
export class MuteListPublished extends DomainEvent {
  readonly eventType = 'social.mute_list_published'

  constructor(
    readonly owner: Pubkey,
    readonly publicMuteCount: number,
    readonly privateMuteCount: number
  ) {
    super()
  }
}

// ============================================================================
// Event Types Union
// ============================================================================

/**
 * Union type of all social domain events
 */
export type SocialDomainEvent =
  | UserFollowed
  | UserUnfollowed
  | FollowListPublished
  | UserMuted
  | UserUnmuted
  | MuteVisibilityChanged
  | MuteListPublished
