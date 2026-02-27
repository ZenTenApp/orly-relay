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

function getCurrentSettings(): TSyncSettings {
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
    // Non-NIP relay configurations (application-specific)
    searchRelays: storage.getSearchRelays(),
    nrcRendezvousUrl: storage.getNrcRendezvousUrl() || undefined
  }
}

function applySettings(settings: TSyncSettings) {
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
  // Non-NIP relay configurations (application-specific)
  if (settings.searchRelays !== undefined) {
    storage.setSearchRelays(settings.searchRelays.length > 0 ? settings.searchRelays : null)
  }
  if (settings.nrcRendezvousUrl !== undefined) {
    storage.setNrcRendezvousUrl(settings.nrcRendezvousUrl)
  }
}

export function SettingsSyncProvider({ children }: { children: React.ReactNode }) {
  const { pubkey, account, publish } = useNostr()
  const [isLoading, setIsLoading] = useState(false)
  const syncTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const lastSyncedSettingsRef = useRef<string | null>(null)

  const fetchRemoteSettings = useCallback(async (): Promise<TSyncSettings | null> => {
    if (!pubkey) return null

    try {
      const relayList = await client.fetchRelayList(pubkey)
      // Use user's write relays only (no hardcoded fallback)
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
        try {
          return JSON.parse(settingsEvent.content) as TSyncSettings
        } catch {
          return null
        }
      }
    } catch (err) {
      console.error('Failed to fetch remote settings:', err)
    }
    return null
  }, [pubkey])

  const syncSettings = useCallback(async () => {
    if (!pubkey || !account) return

    // Skip relay-based settings sync if NRC-only config sync is enabled
    if (storage.getNrcOnlyConfigSync()) return

    const currentSettings = getCurrentSettings()
    const settingsJson = JSON.stringify(currentSettings)

    // Don't sync if settings haven't changed since last sync
    if (settingsJson === lastSyncedSettingsRef.current) {
      return
    }

    setIsLoading(true)
    try {
      const draftEvent = createSettingsDraftEvent(currentSettings)
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

  // Load settings from network on login
  useEffect(() => {
    if (!pubkey) {
      lastSyncedSettingsRef.current = null
      return
    }

    // Skip relay-based settings sync if NRC-only config sync is enabled
    // (settings will sync via NRC instead)
    if (storage.getNrcOnlyConfigSync()) {
      lastSyncedSettingsRef.current = JSON.stringify(getCurrentSettings())
      return
    }

    const loadRemoteSettings = async () => {
      setIsLoading(true)
      try {
        const currentSettings = getCurrentSettings()
        const currentSettingsJson = JSON.stringify(currentSettings)

        const remoteSettings = await fetchRemoteSettings()
        if (remoteSettings) {
          // Apply remote settings first, then re-read through getCurrentSettings()
          // to normalize key order and types before comparing
          applySettings(remoteSettings)
          const appliedSettingsJson = JSON.stringify(getCurrentSettings())

          if (currentSettingsJson !== appliedSettingsJson) {
            lastSyncedSettingsRef.current = appliedSettingsJson
            // Trigger a page reload to apply the settings
            window.location.reload()
          } else {
            lastSyncedSettingsRef.current = currentSettingsJson
          }
        } else {
          // No remote settings, use current as baseline
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

    // Listen for settings change events
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
