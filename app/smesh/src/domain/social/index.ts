/**
 * Social Bounded Context
 *
 * Handles following, muting, and other social graph relationships.
 */

// Aggregates
export { FollowList } from './FollowList'
export type { FollowEntry, FollowListChange } from './FollowList'

export { MuteList } from './MuteList'
export type { MuteEntry, MuteVisibility, MuteListChange } from './MuteList'

export { PinnedUsersList, tryToPinnedUsersList } from './PinnedUsersList'
export type { PinnedUserEntry, PinnedUsersListChange } from './PinnedUsersList'

// Domain Events
export {
  DomainEvent,
  UserFollowed,
  UserUnfollowed,
  FollowListPublished,
  UserMuted,
  UserUnmuted,
  MuteVisibilityChanged,
  MuteListPublished
} from './events'
export type { SocialDomainEvent } from './events'

// Errors
export {
  CannotFollowSelfError,
  CannotMuteSelfError,
  NotAuthenticatedError,
  FollowListOperationError,
  MuteListOperationError
} from './errors'

// Repository Interfaces
export type { FollowListRepository, MuteListRepository, PinnedUsersListRepository } from './repositories'

// Adapters for migration
export {
  // FollowList adapters
  toFollowList,
  tryToFollowList,
  fromFollowListToHexSet,
  fromFollowListToHexArray,
  isFollowingHex,
  followByHex,
  unfollowByHex,
  // MuteList adapters
  toMuteList,
  tryToMuteList,
  fromMuteListToHexSet,
  fromMuteListToPublicHexSet,
  fromMuteListToPrivateHexSet,
  isMutedHex,
  getMuteVisibilityByHex,
  mutePubliclyByHex,
  mutePrivatelyByHex,
  unmuteByHex,
  // PinnedUsersList adapters
  toPinnedUsersList,
  fromPinnedUsersListToHexSet,
  isPinnedHex,
  pinByHex,
  unpinByHex,
  // Combined adapters
  createMuteFilter,
  createFollowFilter,
  createPinnedFilter
} from './adapters'
