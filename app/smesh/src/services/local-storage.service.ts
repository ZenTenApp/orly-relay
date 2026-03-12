import {
  ALLOWED_FILTER_KINDS,
  DEFAULT_FAVICON_URL_TEMPLATE,
  ExtendedKind,
  MEDIA_AUTO_LOAD_POLICY,
  NOTIFICATION_LIST_STYLE,
  NSFW_DISPLAY_POLICY,
  StorageKey,
  TPrimaryColor
} from '@/constants'
import { isSameAccount } from '@/lib/account'
import { randomString } from '@/lib/random'
import { isTorBrowser } from '@/lib/utils'
import {
  TAccount,
  TAccountPointer,
  TEmoji,
  TFeedInfo,
  TMediaAutoLoadPolicy,
  TLlmConfig,
  TMediaUploadServiceConfig,
  TNoteListMode,
  TNsfwDisplayPolicy,
  TNotificationStyle,
  TRelaySet,
  TThemeSetting
} from '@/types'
import { kinds } from 'nostr-tools'

class LocalStorageService {
  static instance: LocalStorageService

  private relaySets: TRelaySet[] = []
  private themeSetting: TThemeSetting = 'system'
  private accounts: TAccount[] = []
  private currentAccount: TAccount | null = null
  private noteListMode: TNoteListMode = 'posts'
  private lastReadNotificationTimeMap: Record<string, number> = {}
  private defaultZapSats: number = 21
  private defaultZapComment: string = 'Zap!'
  private quickZap: boolean = false
  private accountFeedInfoMap: Record<string, TFeedInfo | undefined> = {}
  private autoplay: boolean = true
  private hideUntrustedInteractions: boolean = false
  private hideUntrustedNotifications: boolean = false
  private hideUntrustedNotes: boolean = false
  private mediaUploadServiceConfigMap: Record<string, TMediaUploadServiceConfig> = {}
  private dismissedTooManyRelaysAlert: boolean = false
  private showKinds: number[] = []
  private hideContentMentioningMutedUsers: boolean = false
  private notificationListStyle: TNotificationStyle = NOTIFICATION_LIST_STYLE.DETAILED
  private mediaAutoLoadPolicy: TMediaAutoLoadPolicy = MEDIA_AUTO_LOAD_POLICY.ALWAYS
  private shownCreateWalletGuideToastPubkeys: Set<string> = new Set()
  private sidebarCollapse: boolean = false
  private primaryColor: TPrimaryColor = 'DEFAULT'
  private enableSingleColumnLayout: boolean = true
  private faviconUrlTemplate: string = DEFAULT_FAVICON_URL_TEMPLATE
  private filterOutOnionRelays: boolean = !isTorBrowser()
  private autoInsertNewNotes: boolean = false
  private quickReaction: boolean = false
  private quickReactionEmoji: string | TEmoji = '+'
  private nsfwDisplayPolicy: TNsfwDisplayPolicy = NSFW_DISPLAY_POLICY.HIDE_CONTENT
  private preferNip44: boolean = false
  private dmConversationFilter: 'all' | 'follows' = 'all'
  private graphQueriesEnabled: boolean = true
  private socialGraphProximity: number | null = null
  private socialGraphIncludeMode: boolean = true // true = include only, false = exclude
  private nrcOnlyConfigSync: boolean = false
  private verboseLogging: boolean = false
  private enableMarkdown: boolean = true
  private addClientTag: boolean = true
  private searchRelays: string[] | null = null
  private fallbackRelayCount: number = 7
  private llmConfigMap: Record<string, TLlmConfig> = {}
  private outboxMode: string = 'automatic'

  constructor() {
    if (!LocalStorageService.instance) {
      this.init()
      LocalStorageService.instance = this
    }
    return LocalStorageService.instance
  }

  init() {
    this.themeSetting =
      (window.localStorage.getItem(StorageKey.THEME_SETTING) as TThemeSetting) ?? 'system'
    const accountsStr = window.localStorage.getItem(StorageKey.ACCOUNTS)
    try { this.accounts = accountsStr ? JSON.parse(accountsStr) : [] } catch { this.accounts = [] }
    const currentAccountStr = window.localStorage.getItem(StorageKey.CURRENT_ACCOUNT)
    try { this.currentAccount = currentAccountStr ? JSON.parse(currentAccountStr) : null } catch { this.currentAccount = null }
    const noteListModeStr = window.localStorage.getItem(StorageKey.NOTE_LIST_MODE)
    this.noteListMode =
      noteListModeStr && ['postsAndReplies', 'you'].includes(noteListModeStr)
        ? (noteListModeStr as TNoteListMode)
        : 'postsAndReplies'
    const lastReadNotificationTimeMapStr =
      window.localStorage.getItem(StorageKey.LAST_READ_NOTIFICATION_TIME_MAP) ?? '{}'
    try { this.lastReadNotificationTimeMap = JSON.parse(lastReadNotificationTimeMapStr) } catch { this.lastReadNotificationTimeMap = {} }

    const relaySetsStr = window.localStorage.getItem(StorageKey.RELAY_SETS)
    if (!relaySetsStr) {
      let relaySets: TRelaySet[] = []
      const legacyRelayGroupsStr = window.localStorage.getItem('relayGroups')
      if (legacyRelayGroupsStr) {
        let legacyRelayGroups: any[]
        try { legacyRelayGroups = JSON.parse(legacyRelayGroupsStr) } catch { legacyRelayGroups = [] }
        relaySets = legacyRelayGroups.map((group: any) => {
          const id = randomString()
          return {
            id,
            aTag: [],
            name: group.groupName,
            relayUrls: group.relayUrls
          }
        })
      }
      if (!relaySets.length) {
        relaySets = []
      }
      window.localStorage.setItem(StorageKey.RELAY_SETS, JSON.stringify(relaySets))
      this.relaySets = relaySets
    } else {
      try { this.relaySets = JSON.parse(relaySetsStr) } catch { this.relaySets = [] }
    }

    const defaultZapSatsStr = window.localStorage.getItem(StorageKey.DEFAULT_ZAP_SATS)
    if (defaultZapSatsStr) {
      const num = parseInt(defaultZapSatsStr)
      if (!isNaN(num)) {
        this.defaultZapSats = num
      }
    }
    this.defaultZapComment = window.localStorage.getItem(StorageKey.DEFAULT_ZAP_COMMENT) ?? 'Zap!'
    this.quickZap = window.localStorage.getItem(StorageKey.QUICK_ZAP) === 'true'

    const accountFeedInfoMapStr =
      window.localStorage.getItem(StorageKey.ACCOUNT_FEED_INFO_MAP) ?? '{}'
    try { this.accountFeedInfoMap = JSON.parse(accountFeedInfoMapStr) } catch { this.accountFeedInfoMap = {} }

    this.autoplay = window.localStorage.getItem(StorageKey.AUTOPLAY) !== 'false'
    this.enableMarkdown = window.localStorage.getItem(StorageKey.ENABLE_MARKDOWN) !== 'false'

    const hideUntrustedEvents =
      window.localStorage.getItem(StorageKey.HIDE_UNTRUSTED_EVENTS) === 'true'
    const storedHideUntrustedInteractions = window.localStorage.getItem(
      StorageKey.HIDE_UNTRUSTED_INTERACTIONS
    )
    const storedHideUntrustedNotifications = window.localStorage.getItem(
      StorageKey.HIDE_UNTRUSTED_NOTIFICATIONS
    )
    const storedHideUntrustedNotes = window.localStorage.getItem(StorageKey.HIDE_UNTRUSTED_NOTES)
    this.hideUntrustedInteractions = storedHideUntrustedInteractions
      ? storedHideUntrustedInteractions === 'true'
      : hideUntrustedEvents
    this.hideUntrustedNotifications = storedHideUntrustedNotifications
      ? storedHideUntrustedNotifications === 'true'
      : hideUntrustedEvents
    this.hideUntrustedNotes = storedHideUntrustedNotes
      ? storedHideUntrustedNotes === 'true'
      : hideUntrustedEvents

    const mediaUploadServiceConfigMapStr = window.localStorage.getItem(
      StorageKey.MEDIA_UPLOAD_SERVICE_CONFIG_MAP
    )
    if (mediaUploadServiceConfigMapStr) {
      try { this.mediaUploadServiceConfigMap = JSON.parse(mediaUploadServiceConfigMapStr) } catch { /* ignore corrupt data */ }
    }

    const llmConfigMapStr = window.localStorage.getItem(StorageKey.LLM_CONFIG_MAP)
    if (llmConfigMapStr) {
      try { this.llmConfigMap = JSON.parse(llmConfigMapStr) } catch { /* ignore corrupt data */ }
    }

    // Migrate old boolean setting to new policy
    const nsfwDisplayPolicyStr = window.localStorage.getItem(StorageKey.NSFW_DISPLAY_POLICY)
    if (
      nsfwDisplayPolicyStr &&
      Object.values(NSFW_DISPLAY_POLICY).includes(nsfwDisplayPolicyStr as TNsfwDisplayPolicy)
    ) {
      this.nsfwDisplayPolicy = nsfwDisplayPolicyStr as TNsfwDisplayPolicy
    } else {
      // Migration: convert old boolean to new policy
      const defaultShowNsfwStr = window.localStorage.getItem(StorageKey.DEFAULT_SHOW_NSFW)
      this.nsfwDisplayPolicy =
        defaultShowNsfwStr === 'true' ? NSFW_DISPLAY_POLICY.SHOW : NSFW_DISPLAY_POLICY.HIDE_CONTENT
      window.localStorage.setItem(StorageKey.NSFW_DISPLAY_POLICY, this.nsfwDisplayPolicy)
    }

    this.dismissedTooManyRelaysAlert =
      window.localStorage.getItem(StorageKey.DISMISSED_TOO_MANY_RELAYS_ALERT) === 'true'

    const showKindsStr = window.localStorage.getItem(StorageKey.SHOW_KINDS)
    if (!showKindsStr) {
      this.showKinds = ALLOWED_FILTER_KINDS
    } else {
      const showKindsVersionStr = window.localStorage.getItem(StorageKey.SHOW_KINDS_VERSION)
      const showKindsVersion = showKindsVersionStr ? parseInt(showKindsVersionStr) : 0
      let parsedKinds: number[]
      try { parsedKinds = JSON.parse(showKindsStr) as number[] } catch { parsedKinds = ALLOWED_FILTER_KINDS }
      const showKindSet = new Set(parsedKinds)
      if (showKindsVersion < 1) {
        showKindSet.add(ExtendedKind.VIDEO)
        showKindSet.add(ExtendedKind.SHORT_VIDEO)
      }
      if (showKindsVersion < 2 && showKindSet.has(ExtendedKind.VIDEO)) {
        showKindSet.add(ExtendedKind.ADDRESSABLE_NORMAL_VIDEO)
        showKindSet.add(ExtendedKind.ADDRESSABLE_SHORT_VIDEO)
      }
      if (showKindsVersion < 3 && showKindSet.has(24236)) {
        showKindSet.delete(24236) // remove typo kind
        showKindSet.add(ExtendedKind.ADDRESSABLE_SHORT_VIDEO)
      }
      if (showKindsVersion < 4 && showKindSet.has(kinds.Repost)) {
        showKindSet.add(kinds.GenericRepost)
      }
      this.showKinds = Array.from(showKindSet)
    }
    window.localStorage.setItem(StorageKey.SHOW_KINDS, JSON.stringify(this.showKinds))
    window.localStorage.setItem(StorageKey.SHOW_KINDS_VERSION, '4')

    this.hideContentMentioningMutedUsers =
      window.localStorage.getItem(StorageKey.HIDE_CONTENT_MENTIONING_MUTED_USERS) === 'true'

    this.notificationListStyle =
      window.localStorage.getItem(StorageKey.NOTIFICATION_LIST_STYLE) ===
      NOTIFICATION_LIST_STYLE.COMPACT
        ? NOTIFICATION_LIST_STYLE.COMPACT
        : NOTIFICATION_LIST_STYLE.DETAILED

    const mediaAutoLoadPolicy = window.localStorage.getItem(StorageKey.MEDIA_AUTO_LOAD_POLICY)
    if (
      mediaAutoLoadPolicy &&
      Object.values(MEDIA_AUTO_LOAD_POLICY).includes(mediaAutoLoadPolicy as TMediaAutoLoadPolicy)
    ) {
      this.mediaAutoLoadPolicy = mediaAutoLoadPolicy as TMediaAutoLoadPolicy
    }

    const shownCreateWalletGuideToastPubkeysStr = window.localStorage.getItem(
      StorageKey.SHOWN_CREATE_WALLET_GUIDE_TOAST_PUBKEYS
    )
    if (shownCreateWalletGuideToastPubkeysStr) {
      try { this.shownCreateWalletGuideToastPubkeys = new Set(JSON.parse(shownCreateWalletGuideToastPubkeysStr)) } catch { this.shownCreateWalletGuideToastPubkeys = new Set() }
    }

    this.sidebarCollapse = window.localStorage.getItem(StorageKey.SIDEBAR_COLLAPSE) === 'true'

    this.primaryColor =
      (window.localStorage.getItem(StorageKey.PRIMARY_COLOR) as TPrimaryColor) ?? 'DEFAULT'

    this.enableSingleColumnLayout =
      window.localStorage.getItem(StorageKey.ENABLE_SINGLE_COLUMN_LAYOUT) !== 'false'

    this.faviconUrlTemplate =
      window.localStorage.getItem(StorageKey.FAVICON_URL_TEMPLATE) ?? DEFAULT_FAVICON_URL_TEMPLATE

    const filterOutOnionRelaysStr = window.localStorage.getItem(StorageKey.FILTER_OUT_ONION_RELAYS)
    if (filterOutOnionRelaysStr) {
      this.filterOutOnionRelays = filterOutOnionRelaysStr !== 'false'
    }

    this.autoInsertNewNotes =
      window.localStorage.getItem(StorageKey.AUTO_INSERT_NEW_NOTES) === 'true'
    this.quickReaction = window.localStorage.getItem(StorageKey.QUICK_REACTION) === 'true'
    const quickReactionEmojiStr =
      window.localStorage.getItem(StorageKey.QUICK_REACTION_EMOJI) ?? '+'
    if (quickReactionEmojiStr.startsWith('{')) {
      this.quickReactionEmoji = JSON.parse(quickReactionEmojiStr) as TEmoji
    } else {
      this.quickReactionEmoji = quickReactionEmojiStr
    }

    this.preferNip44 = window.localStorage.getItem(StorageKey.PREFER_NIP44) === 'true'
    this.dmConversationFilter =
      (window.localStorage.getItem(StorageKey.DM_CONVERSATION_FILTER) as 'all' | 'follows') || 'all'
    this.graphQueriesEnabled =
      window.localStorage.getItem(StorageKey.GRAPH_QUERIES_ENABLED) !== 'false'

    const socialGraphProximityStr = window.localStorage.getItem(StorageKey.SOCIAL_GRAPH_PROXIMITY)
    if (socialGraphProximityStr) {
      const parsed = parseInt(socialGraphProximityStr)
      if (!isNaN(parsed) && parsed >= 1 && parsed <= 2) {
        this.socialGraphProximity = parsed
      }
    }

    this.socialGraphIncludeMode =
      window.localStorage.getItem(StorageKey.SOCIAL_GRAPH_INCLUDE_MODE) !== 'false'

    this.nrcOnlyConfigSync =
      window.localStorage.getItem(StorageKey.NRC_ONLY_CONFIG_SYNC) === 'true'

    this.verboseLogging =
      window.localStorage.getItem(StorageKey.VERBOSE_LOGGING) === 'true'

    // Default to true if not set (enabled by default)
    this.addClientTag =
      window.localStorage.getItem(StorageKey.ADD_CLIENT_TAG) !== 'false'

    // Search relays - user-configurable, defaults to empty (opt-in for privacy)
    const searchRelaysStr = window.localStorage.getItem(StorageKey.SEARCH_RELAYS)
    if (searchRelaysStr) {
      try {
        this.searchRelays = JSON.parse(searchRelaysStr)
      } catch {
        this.searchRelays = null
      }
    }

    // Fallback relay count - how many top discovered relays to use as fallback
    const fallbackRelayCountStr = window.localStorage.getItem(StorageKey.FALLBACK_RELAY_COUNT)
    if (fallbackRelayCountStr) {
      const num = parseInt(fallbackRelayCountStr)
      if (!isNaN(num) && num >= 3 && num <= 50) {
        this.fallbackRelayCount = num
      }
    }

    this.outboxMode = window.localStorage.getItem(StorageKey.OUTBOX_MODE) ?? 'automatic'

    // Clean up deprecated data
    window.localStorage.removeItem(StorageKey.PINNED_PUBKEYS)
    window.localStorage.removeItem(StorageKey.ACCOUNT_PROFILE_EVENT_MAP)
    window.localStorage.removeItem(StorageKey.ACCOUNT_FOLLOW_LIST_EVENT_MAP)
    window.localStorage.removeItem(StorageKey.ACCOUNT_RELAY_LIST_EVENT_MAP)
    window.localStorage.removeItem(StorageKey.ACCOUNT_MUTE_LIST_EVENT_MAP)
    window.localStorage.removeItem(StorageKey.ACCOUNT_MUTE_DECRYPTED_TAGS_MAP)
    window.localStorage.removeItem(StorageKey.ACTIVE_RELAY_SET_ID)
    window.localStorage.removeItem(StorageKey.FEED_TYPE)
  }

  getRelaySets() {
    return this.relaySets
  }

  setRelaySets(relaySets: TRelaySet[]) {
    this.relaySets = relaySets
    window.localStorage.setItem(StorageKey.RELAY_SETS, JSON.stringify(this.relaySets))
  }

  getThemeSetting() {
    return this.themeSetting
  }

  setThemeSetting(themeSetting: TThemeSetting) {
    window.localStorage.setItem(StorageKey.THEME_SETTING, themeSetting)
    this.themeSetting = themeSetting
  }

  getNoteListMode() {
    return this.noteListMode
  }

  setNoteListMode(mode: TNoteListMode) {
    window.localStorage.setItem(StorageKey.NOTE_LIST_MODE, mode)
    this.noteListMode = mode
  }

  getAccounts() {
    return this.accounts
  }

  findAccount(account: TAccountPointer) {
    return this.accounts.find((act) => isSameAccount(act, account))
  }

  getCurrentAccount() {
    return this.currentAccount
  }

  getAccountNsec(pubkey: string) {
    const account = this.accounts.find((act) => act.pubkey === pubkey && act.signerType === 'nsec')
    return account?.nsec
  }

  getAccountNcryptsec(pubkey: string) {
    const account = this.accounts.find(
      (act) => act.pubkey === pubkey && act.signerType === 'ncryptsec'
    )
    return account?.ncryptsec
  }

  addAccount(account: TAccount) {
    const index = this.accounts.findIndex((act) => isSameAccount(act, account))
    if (index !== -1) {
      this.accounts[index] = account
    } else {
      this.accounts.push(account)
    }
    window.localStorage.setItem(StorageKey.ACCOUNTS, JSON.stringify(this.accounts))
    return this.accounts
  }

  removeAccount(account: TAccount) {
    this.accounts = this.accounts.filter((act) => !isSameAccount(act, account))
    window.localStorage.setItem(StorageKey.ACCOUNTS, JSON.stringify(this.accounts))
    return this.accounts
  }

  switchAccount(account: TAccount | null) {
    if (isSameAccount(this.currentAccount, account)) {
      return
    }
    const act = this.accounts.find((act) => isSameAccount(act, account))
    if (!act) {
      return
    }
    this.currentAccount = act
    window.localStorage.setItem(StorageKey.CURRENT_ACCOUNT, JSON.stringify(act))
  }

  getDefaultZapSats() {
    return this.defaultZapSats
  }

  setDefaultZapSats(sats: number) {
    this.defaultZapSats = sats
    window.localStorage.setItem(StorageKey.DEFAULT_ZAP_SATS, sats.toString())
  }

  getDefaultZapComment() {
    return this.defaultZapComment
  }

  setDefaultZapComment(comment: string) {
    this.defaultZapComment = comment
    window.localStorage.setItem(StorageKey.DEFAULT_ZAP_COMMENT, comment)
  }

  getQuickZap() {
    return this.quickZap
  }

  setQuickZap(quickZap: boolean) {
    this.quickZap = quickZap
    window.localStorage.setItem(StorageKey.QUICK_ZAP, quickZap.toString())
  }

  getLastReadNotificationTime(pubkey: string) {
    return this.lastReadNotificationTimeMap[pubkey] ?? 0
  }

  setLastReadNotificationTime(pubkey: string, time: number) {
    this.lastReadNotificationTimeMap[pubkey] = time
    window.localStorage.setItem(
      StorageKey.LAST_READ_NOTIFICATION_TIME_MAP,
      JSON.stringify(this.lastReadNotificationTimeMap)
    )
  }

  getFeedInfo(pubkey: string) {
    return this.accountFeedInfoMap[pubkey]
  }

  setFeedInfo(info: TFeedInfo, pubkey?: string | null) {
    this.accountFeedInfoMap[pubkey ?? 'default'] = info
    window.localStorage.setItem(
      StorageKey.ACCOUNT_FEED_INFO_MAP,
      JSON.stringify(this.accountFeedInfoMap)
    )
  }

  getAutoplay() {
    return this.autoplay
  }

  setAutoplay(autoplay: boolean) {
    this.autoplay = autoplay
    window.localStorage.setItem(StorageKey.AUTOPLAY, autoplay.toString())
  }

  getHideUntrustedInteractions() {
    return this.hideUntrustedInteractions
  }

  setHideUntrustedInteractions(hideUntrustedInteractions: boolean) {
    this.hideUntrustedInteractions = hideUntrustedInteractions
    window.localStorage.setItem(
      StorageKey.HIDE_UNTRUSTED_INTERACTIONS,
      hideUntrustedInteractions.toString()
    )
  }

  getHideUntrustedNotifications() {
    return this.hideUntrustedNotifications
  }

  setHideUntrustedNotifications(hideUntrustedNotifications: boolean) {
    this.hideUntrustedNotifications = hideUntrustedNotifications
    window.localStorage.setItem(
      StorageKey.HIDE_UNTRUSTED_NOTIFICATIONS,
      hideUntrustedNotifications.toString()
    )
  }

  getHideUntrustedNotes() {
    return this.hideUntrustedNotes
  }

  setHideUntrustedNotes(hideUntrustedNotes: boolean) {
    this.hideUntrustedNotes = hideUntrustedNotes
    window.localStorage.setItem(StorageKey.HIDE_UNTRUSTED_NOTES, hideUntrustedNotes.toString())
  }

  getMediaUploadServiceConfig(pubkey?: string | null): TMediaUploadServiceConfig {
    const defaultConfig = { type: 'blossom' } as const
    if (!pubkey) {
      return defaultConfig
    }
    // Always read from localStorage directly to avoid stale cache issues
    const mapStr = window.localStorage.getItem(StorageKey.MEDIA_UPLOAD_SERVICE_CONFIG_MAP)
    if (mapStr) {
      try {
        const map = JSON.parse(mapStr) as Record<string, TMediaUploadServiceConfig>
        return map[pubkey] ?? defaultConfig
      } catch {
        return defaultConfig
      }
    }
    return defaultConfig
  }

  setMediaUploadServiceConfig(
    pubkey: string,
    config: TMediaUploadServiceConfig
  ): TMediaUploadServiceConfig {
    this.mediaUploadServiceConfigMap[pubkey] = config
    window.localStorage.setItem(
      StorageKey.MEDIA_UPLOAD_SERVICE_CONFIG_MAP,
      JSON.stringify(this.mediaUploadServiceConfigMap)
    )
    return config
  }

  getLlmConfig(pubkey?: string | null): TLlmConfig | null {
    if (!pubkey) return null
    const mapStr = window.localStorage.getItem(StorageKey.LLM_CONFIG_MAP)
    if (mapStr) {
      try {
        const map = JSON.parse(mapStr) as Record<string, TLlmConfig>
        return map[pubkey] ?? null
      } catch {
        return null
      }
    }
    return null
  }

  setLlmConfig(pubkey: string, config: TLlmConfig) {
    this.llmConfigMap[pubkey] = config
    window.localStorage.setItem(StorageKey.LLM_CONFIG_MAP, JSON.stringify(this.llmConfigMap))
  }

  getDismissedTooManyRelaysAlert() {
    return this.dismissedTooManyRelaysAlert
  }

  setDismissedTooManyRelaysAlert(dismissed: boolean) {
    this.dismissedTooManyRelaysAlert = dismissed
    window.localStorage.setItem(StorageKey.DISMISSED_TOO_MANY_RELAYS_ALERT, dismissed.toString())
  }

  getShowKinds() {
    return this.showKinds
  }

  setShowKinds(kinds: number[]) {
    this.showKinds = kinds
    window.localStorage.setItem(StorageKey.SHOW_KINDS, JSON.stringify(kinds))
  }

  getHideContentMentioningMutedUsers() {
    return this.hideContentMentioningMutedUsers
  }

  setHideContentMentioningMutedUsers(hide: boolean) {
    this.hideContentMentioningMutedUsers = hide
    window.localStorage.setItem(StorageKey.HIDE_CONTENT_MENTIONING_MUTED_USERS, hide.toString())
  }

  getNotificationListStyle() {
    return this.notificationListStyle
  }

  setNotificationListStyle(style: TNotificationStyle) {
    this.notificationListStyle = style
    window.localStorage.setItem(StorageKey.NOTIFICATION_LIST_STYLE, style)
  }

  getMediaAutoLoadPolicy() {
    return this.mediaAutoLoadPolicy
  }

  setMediaAutoLoadPolicy(policy: TMediaAutoLoadPolicy) {
    this.mediaAutoLoadPolicy = policy
    window.localStorage.setItem(StorageKey.MEDIA_AUTO_LOAD_POLICY, policy)
  }

  hasShownCreateWalletGuideToast(pubkey: string) {
    return this.shownCreateWalletGuideToastPubkeys.has(pubkey)
  }

  markCreateWalletGuideToastAsShown(pubkey: string) {
    if (this.shownCreateWalletGuideToastPubkeys.has(pubkey)) {
      return
    }
    this.shownCreateWalletGuideToastPubkeys.add(pubkey)
    window.localStorage.setItem(
      StorageKey.SHOWN_CREATE_WALLET_GUIDE_TOAST_PUBKEYS,
      JSON.stringify(Array.from(this.shownCreateWalletGuideToastPubkeys))
    )
  }

  getSidebarCollapse() {
    return this.sidebarCollapse
  }

  setSidebarCollapse(collapse: boolean) {
    this.sidebarCollapse = collapse
    window.localStorage.setItem(StorageKey.SIDEBAR_COLLAPSE, collapse.toString())
  }

  getPrimaryColor() {
    return this.primaryColor
  }

  setPrimaryColor(color: TPrimaryColor) {
    this.primaryColor = color
    window.localStorage.setItem(StorageKey.PRIMARY_COLOR, color)
  }

  getEnableSingleColumnLayout() {
    return this.enableSingleColumnLayout
  }

  setEnableSingleColumnLayout(enable: boolean) {
    this.enableSingleColumnLayout = enable
    window.localStorage.setItem(StorageKey.ENABLE_SINGLE_COLUMN_LAYOUT, enable.toString())
  }

  getFaviconUrlTemplate() {
    return this.faviconUrlTemplate
  }

  setFaviconUrlTemplate(template: string) {
    this.faviconUrlTemplate = template
    window.localStorage.setItem(StorageKey.FAVICON_URL_TEMPLATE, template)
  }

  getFilterOutOnionRelays() {
    return this.filterOutOnionRelays
  }

  setFilterOutOnionRelays(filterOut: boolean) {
    this.filterOutOnionRelays = filterOut
    window.localStorage.setItem(StorageKey.FILTER_OUT_ONION_RELAYS, filterOut.toString())
  }

  getAutoInsertNewNotes() {
    return this.autoInsertNewNotes
  }

  setAutoInsertNewNotes(value: boolean) {
    this.autoInsertNewNotes = value
    window.localStorage.setItem(StorageKey.AUTO_INSERT_NEW_NOTES, value.toString())
  }

  getQuickReaction() {
    return this.quickReaction
  }

  setQuickReaction(quickReaction: boolean) {
    this.quickReaction = quickReaction
    window.localStorage.setItem(StorageKey.QUICK_REACTION, quickReaction.toString())
  }

  getQuickReactionEmoji() {
    return this.quickReactionEmoji
  }

  setQuickReactionEmoji(emoji: string | TEmoji) {
    this.quickReactionEmoji = emoji
    window.localStorage.setItem(
      StorageKey.QUICK_REACTION_EMOJI,
      typeof emoji === 'string' ? emoji : JSON.stringify(emoji)
    )
  }

  getNsfwDisplayPolicy() {
    return this.nsfwDisplayPolicy
  }

  setNsfwDisplayPolicy(policy: TNsfwDisplayPolicy) {
    this.nsfwDisplayPolicy = policy
    window.localStorage.setItem(StorageKey.NSFW_DISPLAY_POLICY, policy)
  }

  getPreferNip44() {
    return this.preferNip44
  }

  setPreferNip44(prefer: boolean) {
    this.preferNip44 = prefer
    window.localStorage.setItem(StorageKey.PREFER_NIP44, prefer.toString())
  }

  getDMConversationFilter() {
    return this.dmConversationFilter
  }

  setDMConversationFilter(filter: 'all' | 'follows') {
    this.dmConversationFilter = filter
    window.localStorage.setItem(StorageKey.DM_CONVERSATION_FILTER, filter)
  }

  getDMLastSeenTimestamp(pubkey: string): number {
    const mapStr = window.localStorage.getItem(StorageKey.DM_LAST_SEEN_TIMESTAMP)
    if (!mapStr) return 0
    try {
      const map = JSON.parse(mapStr) as Record<string, number>
      return map[pubkey] ?? 0
    } catch {
      return 0
    }
  }

  setDMLastSeenTimestamp(pubkey: string, timestamp: number) {
    const mapStr = window.localStorage.getItem(StorageKey.DM_LAST_SEEN_TIMESTAMP)
    let map: Record<string, number> = {}
    if (mapStr) {
      try {
        map = JSON.parse(mapStr)
      } catch {
        // ignore
      }
    }
    map[pubkey] = timestamp
    window.localStorage.setItem(StorageKey.DM_LAST_SEEN_TIMESTAMP, JSON.stringify(map))
  }

  getGraphQueriesEnabled() {
    return this.graphQueriesEnabled
  }

  setGraphQueriesEnabled(enabled: boolean) {
    this.graphQueriesEnabled = enabled
    window.localStorage.setItem(StorageKey.GRAPH_QUERIES_ENABLED, enabled.toString())
  }

  getSocialGraphProximity(): number | null {
    return this.socialGraphProximity
  }

  setSocialGraphProximity(depth: number | null) {
    this.socialGraphProximity = depth
    if (depth === null) {
      window.localStorage.removeItem(StorageKey.SOCIAL_GRAPH_PROXIMITY)
    } else {
      window.localStorage.setItem(StorageKey.SOCIAL_GRAPH_PROXIMITY, depth.toString())
    }
  }

  getSocialGraphIncludeMode(): boolean {
    return this.socialGraphIncludeMode
  }

  setSocialGraphIncludeMode(include: boolean) {
    this.socialGraphIncludeMode = include
    window.localStorage.setItem(StorageKey.SOCIAL_GRAPH_INCLUDE_MODE, include.toString())
  }

  getNrcOnlyConfigSync() {
    return this.nrcOnlyConfigSync
  }

  setNrcOnlyConfigSync(nrcOnly: boolean) {
    this.nrcOnlyConfigSync = nrcOnly
    window.localStorage.setItem(StorageKey.NRC_ONLY_CONFIG_SYNC, nrcOnly.toString())
  }

  getVerboseLogging() {
    return this.verboseLogging
  }

  setVerboseLogging(verbose: boolean) {
    this.verboseLogging = verbose
    window.localStorage.setItem(StorageKey.VERBOSE_LOGGING, verbose.toString())
  }

  getEnableMarkdown() {
    return this.enableMarkdown
  }

  setEnableMarkdown(enable: boolean) {
    this.enableMarkdown = enable
    window.localStorage.setItem(StorageKey.ENABLE_MARKDOWN, enable.toString())
  }

  getAddClientTag() {
    return this.addClientTag
  }

  setAddClientTag(add: boolean) {
    this.addClientTag = add
    window.localStorage.setItem(StorageKey.ADD_CLIENT_TAG, add.toString())
  }

  /**
   * Get user-configured search relays. Returns empty array if not configured.
   * Search is opt-in to protect user privacy - queries are not sent to
   * third-party relays without explicit user configuration.
   */
  getSearchRelays(): string[] {
    return this.searchRelays ?? []
  }

  /**
   * Set custom search relays. Pass null to reset to defaults.
   */
  setSearchRelays(relays: string[] | null) {
    this.searchRelays = relays
    if (relays === null) {
      window.localStorage.removeItem(StorageKey.SEARCH_RELAYS)
    } else {
      window.localStorage.setItem(StorageKey.SEARCH_RELAYS, JSON.stringify(relays))
    }
  }

  /**
   * Check if user has custom search relays configured.
   */
  hasCustomSearchRelays(): boolean {
    return this.searchRelays !== null && this.searchRelays.length > 0
  }

  getFallbackRelayCount(): number {
    return this.fallbackRelayCount
  }

  setFallbackRelayCount(count: number) {
    this.fallbackRelayCount = Math.max(3, Math.min(50, count))
    window.localStorage.setItem(StorageKey.FALLBACK_RELAY_COUNT, this.fallbackRelayCount.toString())
  }

  getOutboxMode(): string {
    return this.outboxMode
  }

  setOutboxMode(mode: string) {
    this.outboxMode = mode
    window.localStorage.setItem(StorageKey.OUTBOX_MODE, mode)
  }

  // NRC rendezvous URL - stored separately by NRCProvider but accessed here for sync
  private static readonly NRC_RENDEZVOUS_KEY = 'nrc:rendezvousUrl'

  /**
   * Get the NRC rendezvous relay URL.
   * Returns empty string if not configured.
   */
  getNrcRendezvousUrl(): string {
    return window.localStorage.getItem(LocalStorageService.NRC_RENDEZVOUS_KEY) || ''
  }

  /**
   * Set the NRC rendezvous relay URL.
   * Pass empty string to clear.
   */
  setNrcRendezvousUrl(url: string) {
    if (url) {
      window.localStorage.setItem(LocalStorageService.NRC_RENDEZVOUS_KEY, url)
    } else {
      window.localStorage.removeItem(LocalStorageService.NRC_RENDEZVOUS_KEY)
    }
  }
}

const instance = new LocalStorageService()
export default instance

// Custom event for settings sync
export const SETTINGS_CHANGED_EVENT = 'smesh-settings-changed'
export function dispatchSettingsChanged() {
  window.dispatchEvent(new CustomEvent(SETTINGS_CHANGED_EVENT))
}
