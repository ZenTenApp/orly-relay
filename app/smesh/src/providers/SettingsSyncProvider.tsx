import { ApplicationDataKey } from '@/constants'
import { createSettingsDraftEvent } from '@/lib/draft-event'
import { getReplaceableEventIdentifier } from '@/lib/event'
import client from '@/services/client.service'
import storage, { SETTINGS_CHANGED_EVENT } from '@/services/local-storage.service'
import { TSyncSettings } from '@/types'
import { kinds } from 'nostr-tools'
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { useNostr } from './NostrProvider'

type TSettingsSyncContext = {
  syncSettings: () => Promise<void>
  isLoading: boolean
}

const SettingsSyncContext = createContext<TSettingsSyncContext | undefined>(undefined)

export const useSettingsSync = () => {
  const context = useContext(SettingsSyncContext)
  if (!context) {
    throw new Error('useSettingsSync must be used within a SettingsSyncProvider')
  }
  return context
}

function getCurrentSettings(pubkey: string | null): TSyncSettings {
  return {
    themeSetting: storage.getThemeSetting(),
    primaryColor: storage.getPrimaryColor(),
    defaultZapSats: storage.getDefaultZapSats(),
    defaultZapComment: storage.getDefaultZapComment(),
    quickZap: storage.getQuickZap(),
    autoplay: storage.getAutoplay(),
    hideUntrustedInteractions: storage.getHideUntrustedInteractions(),
    hideUntrustedNotifications: storage.getHideUntrustedNotifications(),
    hideUntrustedNotes: storage.getHideUntrustedNotes(),
    nsfwDisplayPolicy: storage.getNsfwDisplayPolicy(),
    showKinds: storage.getShowKinds(),
    hideContentMentioningMutedUsers: storage.getHideContentMentioningMutedUsers(),
    notificationListStyle: storage.getNotificationListStyle(),
    mediaAutoLoadPolicy: storage.getMediaAutoLoadPolicy(),
    sidebarCollapse: storage.getSidebarCollapse(),
    enableSingleColumnLayout: storage.getEnableSingleColumnLayout(),
    faviconUrlTemplate: storage.getFaviconUrlTemplate(),
    filterOutOnionRelays: storage.getFilterOutOnionRelays(),
    quickReaction: storage.getQuickReaction(),
    quickReactionEmoji: storage.getQuickReactionEmoji(),
    noteListMode: storage.getNoteListMode(),
    nrcOnlyConfigSync: storage.getNrcOnlyConfigSync(),
    autoInsertNewNotes: storage.getAutoInsertNewNotes(),
    addClientTag: storage.getAddClientTag(),
    enableMarkdown: storage.getEnableMarkdown(),
    verboseLogging: storage.getVerboseLogging(),
    fallbackRelayCount: storage.getFallbackRelayCount(),
    preferNip44: storage.getPreferNip44(),
    dmConversationFilter: storage.getDMConversationFilter(),
    graphQueriesEnabled: storage.getGraphQueriesEnabled(),
    socialGraphProximity: storage.getSocialGraphProximity(),
    socialGraphIncludeMode: storage.getSocialGraphIncludeMode(),
    llmConfig: pubkey ? storage.getLlmConfig(pubkey) : null,
    mediaUploadServiceConfig: pubkey ? storage.getMediaUploadServiceConfig(pubkey) : undefined,
    // Non-NIP relay configurations (application-specific)
    searchRelays: storage.getSearchRelays(),
    nrcRendezvousUrl: storage.getNrcRendezvousUrl() || undefined
  }
}

function applySettings(settings: TSyncSettings, pubkey: string | null) {
  if (settings.themeSetting !== undefined) {
    storage.setThemeSetting(settings.themeSetting)
  }
  if (settings.primaryColor !== undefined) {
    storage.setPrimaryColor(settings.primaryColor as any)
  }
  if (settings.defaultZapSats !== undefined) {
    storage.setDefaultZapSats(settings.defaultZapSats)
  }
  if (settings.defaultZapComment !== undefined) {
    storage.setDefaultZapComment(settings.defaultZapComment)
  }
  if (settings.quickZap !== undefined) {
    storage.setQuickZap(settings.quickZap)
  }
  if (settings.autoplay !== undefined) {
    storage.setAutoplay(settings.autoplay)
  }
  if (settings.hideUntrustedInteractions !== undefined) {
    storage.setHideUntrustedInteractions(settings.hideUntrustedInteractions)
  }
  if (settings.hideUntrustedNotifications !== undefined) {
    storage.setHideUntrustedNotifications(settings.hideUntrustedNotifications)
  }
  if (settings.hideUntrustedNotes !== undefined) {
    storage.setHideUntrustedNotes(settings.hideUntrustedNotes)
  }
  if (settings.nsfwDisplayPolicy !== undefined) {
    storage.setNsfwDisplayPolicy(settings.nsfwDisplayPolicy)
  }
  if (settings.showKinds !== undefined) {
    storage.setShowKinds(settings.showKinds)
  }
  if (settings.hideContentMentioningMutedUsers !== undefined) {
    storage.setHideContentMentioningMutedUsers(settings.hideContentMentioningMutedUsers)
  }
  if (settings.notificationListStyle !== undefined) {
    storage.setNotificationListStyle(settings.notificationListStyle)
  }
  if (settings.mediaAutoLoadPolicy !== undefined) {
    storage.setMediaAutoLoadPolicy(settings.mediaAutoLoadPolicy)
  }
  if (settings.sidebarCollapse !== undefined) {
    storage.setSidebarCollapse(settings.sidebarCollapse)
  }
  if (settings.enableSingleColumnLayout !== undefined) {
    storage.setEnableSingleColumnLayout(settings.enableSingleColumnLayout)
  }
  if (settings.faviconUrlTemplate !== undefined) {
    storage.setFaviconUrlTemplate(settings.faviconUrlTemplate)
  }
  if (settings.filterOutOnionRelays !== undefined) {
    storage.setFilterOutOnionRelays(settings.filterOutOnionRelays)
  }
  if (settings.quickReaction !== undefined) {
    storage.setQuickReaction(settings.quickReaction)
  }
  if (settings.quickReactionEmoji !== undefined) {
    storage.setQuickReactionEmoji(settings.quickReactionEmoji)
  }
  if (settings.noteListMode !== undefined) {
    storage.setNoteListMode(settings.noteListMode)
  }
  if (settings.nrcOnlyConfigSync !== undefined) {
    storage.setNrcOnlyConfigSync(settings.nrcOnlyConfigSync)
  }
  if (settings.autoInsertNewNotes !== undefined) {
    storage.setAutoInsertNewNotes(settings.autoInsertNewNotes)
  }
  if (settings.addClientTag !== undefined) {
    storage.setAddClientTag(settings.addClientTag)
  }
  if (settings.enableMarkdown !== undefined) {
    storage.setEnableMarkdown(settings.enableMarkdown)
  }
  if (settings.verboseLogging !== undefined) {
    storage.setVerboseLogging(settings.verboseLogging)
  }
  if (settings.fallbackRelayCount !== undefined) {
    storage.setFallbackRelayCount(settings.fallbackRelayCount)
  }
  if (settings.preferNip44 !== undefined) {
    storage.setPreferNip44(settings.preferNip44)
  }
  if (settings.dmConversationFilter !== undefined) {
    storage.setDMConversationFilter(settings.dmConversationFilter)
  }
  if (settings.graphQueriesEnabled !== undefined) {
    storage.setGraphQueriesEnabled(settings.graphQueriesEnabled)
  }
  if (settings.socialGraphProximity !== undefined) {
    storage.setSocialGraphProximity(settings.socialGraphProximity)
  }
  if (settings.socialGraphIncludeMode !== undefined) {
    storage.setSocialGraphIncludeMode(settings.socialGraphIncludeMode)
  }
  // Non-NIP relay configurations (application-specific)
  if (settings.searchRelays !== undefined) {
    storage.setSearchRelays(settings.searchRelays.length > 0 ? settings.searchRelays : null)
  }
  if (settings.nrcRendezvousUrl !== undefined) {
    storage.setNrcRendezvousUrl(settings.nrcRendezvousUrl)
  }
  // Per-pubkey settings
  if (pubkey) {
    if (settings.llmConfig !== undefined) {
      if (settings.llmConfig) {
        storage.setLlmConfig(pubkey, settings.llmConfig)
      }
    }
    if (settings.mediaUploadServiceConfig !== undefined) {
      storage.setMediaUploadServiceConfig(pubkey, settings.mediaUploadServiceConfig)
    }
  }
}

export function SettingsSyncProvider({ children }: { children: React.ReactNode }) {
  const { pubkey, account, publish, nip44Encrypt, nip44Decrypt, hasNip44Support } = useNostr()
  const [isLoading, setIsLoading] = useState(false)
  const syncTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const lastSyncedSettingsRef = useRef<string | null>(null)
  const hasLoadedRef = useRef(false)

  // Store encryption functions in refs so callbacks don't change identity
  // when the signer initializes asynchronously
  const encryptRef = useRef({ nip44Encrypt, nip44Decrypt, hasNip44Support })
  encryptRef.current = { nip44Encrypt, nip44Decrypt, hasNip44Support }

  /**
   * Decrypt settings content from an event.
   * Tries plain JSON first (backward compat), then NIP-44 decryption.
   */
  const decryptContent = useCallback(async (content: string, authorPubkey: string): Promise<TSyncSettings | null> => {
    // Try plain JSON first (backward compat with old unencrypted events)
    try {
      const parsed = JSON.parse(content)
      if (typeof parsed === 'object' && parsed !== null) {
        return parsed as TSyncSettings
      }
    } catch {
      // Not valid JSON — likely NIP-44 encrypted ciphertext
    }

    // Try NIP-44 decryption (self-encrypted to own pubkey)
    const { hasNip44Support, nip44Decrypt } = encryptRef.current
    if (hasNip44Support) {
      try {
        const decrypted = await nip44Decrypt(authorPubkey, content)
        return JSON.parse(decrypted) as TSyncSettings
      } catch (err) {
        console.error('Failed to decrypt settings:', err)
      }
    }

    return null
  }, []) // stable — reads from encryptRef

  const fetchRemoteSettings = useCallback(async (): Promise<TSyncSettings | null> => {
    if (!pubkey) return null

    try {
      const relayList = await client.fetchRelayList(pubkey)
      const relays = relayList.write.length > 0 ? relayList.write.slice(0, 5) : client.currentRelays.slice(0, 5)

      const events = await client.fetchEvents(relays, {
        kinds: [kinds.Application],
        authors: [pubkey],
        '#d': [ApplicationDataKey.SETTINGS],
        limit: 1
      })

      const settingsEvent = events
        .filter((e) => getReplaceableEventIdentifier(e) === ApplicationDataKey.SETTINGS)
        .sort((a, b) => b.created_at - a.created_at)[0]

      if (settingsEvent) {
        return await decryptContent(settingsEvent.content, settingsEvent.pubkey)
      }
    } catch (err) {
      console.error('Failed to fetch remote settings:', err)
    }
    return null
  }, [pubkey, decryptContent])

  const syncSettings = useCallback(async () => {
    if (!pubkey || !account) return

    // Skip relay-based settings sync if NRC-only config sync is enabled
    if (storage.getNrcOnlyConfigSync()) return

    const currentSettings = getCurrentSettings(pubkey)
    const settingsJson = JSON.stringify(currentSettings)

    // Don't sync if settings haven't changed since last sync
    if (settingsJson === lastSyncedSettingsRef.current) {
      return
    }

    setIsLoading(true)
    try {
      // Encrypt settings with NIP-44 self-encryption if available
      const { hasNip44Support, nip44Encrypt } = encryptRef.current
      let content: string
      if (hasNip44Support) {
        content = await nip44Encrypt(pubkey, settingsJson)
      } else {
        content = settingsJson
      }

      const draftEvent = createSettingsDraftEvent(content)
      await publish(draftEvent)
      lastSyncedSettingsRef.current = settingsJson
    } catch (err) {
      console.error('Failed to sync settings:', err)
    } finally {
      setIsLoading(false)
    }
  }, [pubkey, account, publish])

  // Debounced sync on settings change
  const debouncedSync = useCallback(() => {
    if (syncTimeoutRef.current) {
      clearTimeout(syncTimeoutRef.current)
    }
    syncTimeoutRef.current = setTimeout(() => {
      syncSettings()
    }, 2000)
  }, [syncSettings])

  // Load settings from network on login — runs once per pubkey
  useEffect(() => {
    if (!pubkey) {
      lastSyncedSettingsRef.current = null
      hasLoadedRef.current = false
      return
    }

    // Only load once per pubkey to prevent reload loops
    if (hasLoadedRef.current) return
    hasLoadedRef.current = true

    // Skip relay-based settings sync if NRC-only config sync is enabled
    if (storage.getNrcOnlyConfigSync()) {
      lastSyncedSettingsRef.current = JSON.stringify(getCurrentSettings(pubkey))
      return
    }

    const loadRemoteSettings = async () => {
      // Wait briefly for signer to initialize so we can decrypt
      if (!encryptRef.current.hasNip44Support) {
        await new Promise((r) => setTimeout(r, 500))
      }

      setIsLoading(true)
      try {
        const currentSettings = getCurrentSettings(pubkey)
        const currentSettingsJson = JSON.stringify(currentSettings)

        const remoteSettings = await fetchRemoteSettings()
        if (remoteSettings) {
          applySettings(remoteSettings, pubkey)
          const appliedSettingsJson = JSON.stringify(getCurrentSettings(pubkey))

          if (currentSettingsJson !== appliedSettingsJson) {
            lastSyncedSettingsRef.current = appliedSettingsJson
            // Notify providers to re-render with new values instead of reloading
            window.dispatchEvent(new CustomEvent(SETTINGS_CHANGED_EVENT))
          } else {
            lastSyncedSettingsRef.current = currentSettingsJson
          }
        } else {
          lastSyncedSettingsRef.current = currentSettingsJson
        }
      } catch (err) {
        console.error('Failed to load remote settings:', err)
      } finally {
        setIsLoading(false)
      }
    }

    loadRemoteSettings()
  }, [pubkey, fetchRemoteSettings])

  // Listen for settings changes and sync
  useEffect(() => {
    if (!pubkey || !account) return

    const handleSettingsChange = () => {
      debouncedSync()
    }

    window.addEventListener(SETTINGS_CHANGED_EVENT, handleSettingsChange)

    return () => {
      window.removeEventListener(SETTINGS_CHANGED_EVENT, handleSettingsChange)
      if (syncTimeoutRef.current) {
        clearTimeout(syncTimeoutRef.current)
      }
    }
  }, [pubkey, account, debouncedSync])

  return (
    <SettingsSyncContext.Provider value={{ syncSettings, isLoading }}>
      {children}
    </SettingsSyncContext.Provider>
  )
}
