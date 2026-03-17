import { Event, Filter, VerifiedEvent } from 'nostr-tools'
import {
  MEDIA_AUTO_LOAD_POLICY,
  NOTIFICATION_LIST_STYLE,
  NSFW_DISPLAY_POLICY,
  POLL_TYPE
} from '../constants'

export type TSubRequestFilter = Omit<Filter, 'since' | 'until'> & { limit: number }

export type TFeedSubRequest = {
  urls: string[]
  filter: Omit<Filter, 'since' | 'until'>
}

export type TProfile = {
  username: string
  pubkey: string
  npub: string
  original_username?: string
  banner?: string
  avatar?: string
  nip05?: string
  about?: string
  website?: string
  lud06?: string
  lud16?: string
  lightningAddress?: string
  created_at?: number
  emojis?: TEmoji[]
}
export type TMailboxRelayScope = 'read' | 'write' | 'both'
export type TMailboxRelay = {
  url: string
  scope: TMailboxRelayScope
}
export type TRelayList = {
  write: string[]
  read: string[]
  originalRelays: TMailboxRelay[]
}

export type TGraphQueryCapability = {
  enabled: boolean
  max_depth: number
  max_results: number
  methods: string[]
}

export type TRelayInfo = {
  url: string
  shortUrl: string
  name?: string
  description?: string
  icon?: string
  pubkey?: string
  contact?: string
  supported_nips?: number[]
  software?: string
  version?: string
  tags?: string[]
  payments_url?: string
  limitation?: {
    auth_required?: boolean
    payment_required?: boolean
  }
  graph_query?: TGraphQueryCapability
}

export type TWebMetadata = {
  title?: string | null
  description?: string | null
  image?: string | null
}

export type TRelaySet = {
  id: string
  aTag: string[]
  name: string
  relayUrls: string[]
}

export type TConfig = {
  relayGroups: TRelaySet[]
  theme: TThemeSetting
}

export type TThemeSetting = 'light' | 'dark' | 'system'
export type TTheme = 'light' | 'dark'

export type TDraftEvent = Pick<Event, 'content' | 'created_at' | 'kind' | 'tags'>

export type TNip07 = {
  getPublicKey: () => Promise<string>
  signEvent: (draftEvent: TDraftEvent) => Promise<VerifiedEvent>
  nip04?: {
    encrypt?: (pubkey: string, plainText: string) => Promise<string>
    decrypt?: (pubkey: string, cipherText: string) => Promise<string>
  }
  nip44?: {
    encrypt?: (pubkey: string, plainText: string) => Promise<string>
    decrypt?: (pubkey: string, cipherText: string) => Promise<string>
  }
}

export interface ISigner {
  getPublicKey: () => Promise<string>
  signEvent: (draftEvent: TDraftEvent) => Promise<VerifiedEvent>
  nip04Encrypt: (pubkey: string, plainText: string) => Promise<string>
  nip04Decrypt: (pubkey: string, cipherText: string) => Promise<string>
  nip44Encrypt?: (pubkey: string, plainText: string) => Promise<string>
  nip44Decrypt?: (pubkey: string, cipherText: string) => Promise<string>
}

export type TSignerType = 'nsec' | 'nip-07' | 'browser-nsec' | 'ncryptsec' | 'npub' | 'bunker'

export type TAccount = {
  pubkey: string
  signerType: TSignerType
  ncryptsec?: string
  nsec?: string
  npub?: string
  bunkerPubkey?: string
  bunkerRelays?: string[]
  bunkerSecret?: string
}

export type TAccountPointer = Pick<TAccount, 'pubkey' | 'signerType'>

export type TFeedType = 'following' | 'pinned' | 'relays' | 'relay'
export type TFeedInfo = { feedType: TFeedType; id?: string } | null

export type TLanguage = 'en' | 'zh' | 'pl'

export type TImetaInfo = {
  url: string
  sha256?: string
  blurHash?: string
  thumbHash?: Uint8Array
  dim?: { width: number; height: number }
  pubkey?: string
  /** Variant name from kind 1063 binding event (thumb, mobile, desktop, original) */
  variant?: string
}

export type TPublishOptions = {
  specifiedRelayUrls?: string[]
  additionalRelayUrls?: string[]
  minPow?: number
}

export type TNoteListMode = 'posts' | 'postsAndReplies' | 'you'

export type TNotificationType = 'all' | 'mentions' | 'reactions' | 'zaps'

export type TPageRef = { scrollToTop: (behavior?: ScrollBehavior) => void }

export type TEmoji = {
  shortcode: string
  url: string
}

export type TMediaUploadServiceConfig =
  | {
      type: 'nip96'
      service: string
    }
  | {
      type: 'blossom'
    }

export type TLlmConfig = {
  apiKey: string
  model: string
  systemPrompt: string
  autoRewrite: boolean
}

export type TPollType = (typeof POLL_TYPE)[keyof typeof POLL_TYPE]

export type TPollCreateData = {
  isMultipleChoice: boolean
  options: string[]
  relays: string[]
  endsAt?: number
}

export type TSearchType =
  | 'profile'
  | 'profiles'
  | 'notes'
  | 'note'
  | 'hashtag'
  | 'relay'
  | 'externalContent'
  | 'nak'

export type TSearchParams =
  | {
      type: Exclude<TSearchType, 'nak'>
      search: string
      input?: string
    }
  | {
      type: 'nak'
      search: string
      request: TFeedSubRequest
      input?: string
    }

export type TNotificationStyle =
  (typeof NOTIFICATION_LIST_STYLE)[keyof typeof NOTIFICATION_LIST_STYLE]

export type TAwesomeRelayCollection = {
  id: string
  name: string
  relays: string[]
}

export type TMediaAutoLoadPolicy =
  (typeof MEDIA_AUTO_LOAD_POLICY)[keyof typeof MEDIA_AUTO_LOAD_POLICY]

export type TNsfwDisplayPolicy = (typeof NSFW_DISPLAY_POLICY)[keyof typeof NSFW_DISPLAY_POLICY]

export type TSyncSettings = {
  language?: string
  themeSetting?: TThemeSetting
  primaryColor?: string
  defaultZapSats?: number
  defaultZapComment?: string
  quickZap?: boolean
  nsfwDisplayPolicy?: TNsfwDisplayPolicy
  showKinds?: number[]
  hideContentMentioningMutedUsers?: boolean
  notificationListStyle?: TNotificationStyle
  mediaAutoLoadPolicy?: TMediaAutoLoadPolicy
  sidebarCollapse?: boolean
  enableSingleColumnLayout?: boolean
  faviconUrlTemplate?: string
  filterOutOnionRelays?: boolean
  quickReaction?: boolean
  quickReactionEmoji?: string | TEmoji
  noteListMode?: TNoteListMode
  preferNip44?: boolean
  nrcOnlyConfigSync?: boolean
  autoInsertNewNotes?: boolean
  addClientTag?: boolean
  enableMarkdown?: boolean
  verboseLogging?: boolean
  fallbackRelayCount?: number
  dmConversationFilter?: 'all' | 'follows'
  graphQueriesEnabled?: boolean
  socialGraphProximity?: number | null
  socialGraphIncludeMode?: boolean
  // Per-pubkey config (keyed by the logged-in pubkey at sync time)
  llmConfig?: TLlmConfig | null
  mediaUploadServiceConfig?: TMediaUploadServiceConfig
  // Non-NIP relay configurations (application-specific)
  searchRelays?: string[]
  nrcRendezvousUrl?: string
  outboxMode?: 'automatic' | 'managed'
  relayStatsData?: string
}

// DM types
export type TDMEncryptionType = 'nip04' | 'nip17'

export interface TConversation {
  partnerPubkey: string
  lastMessageAt: number
  lastMessagePreview: string
  unreadCount: number
  preferredEncryption: TDMEncryptionType | null
}

export interface TDirectMessage {
  id: string
  innerEventId?: string
  senderPubkey: string
  recipientPubkey: string
  content: string
  createdAt: number
  encryptionType: TDMEncryptionType
  event: Event
  decryptedContent?: string
  seenOnRelays?: string[]
}

// Deleted messages state (stored in kind 30078 Application Specific Data)
export interface TDMDeletedState {
  // Specific message IDs to ignore
  deletedIds: string[]
  // Timestamp ranges to ignore per conversation
  deletedRanges: {
    [partnerPubkey: string]: Array<{
      start: number // timestamp
      end: number // timestamp
    }>
  }
}
