/**
 * NRC (Nostr Relay Connect) Provider
 *
 * Manages NRC state for both:
 * - Listener mode: Accept connections from other devices
 * - Client mode: Connect to and sync from other devices
 */

import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react'
import { Filter, Event } from 'nostr-tools'
import { useNostr } from './NostrProvider'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import {
  NRCConnection,
  NRCListenerConfig,
  generateConnectionURI,
  getNRCListenerService,
  syncFromRemote,
  testConnection,
  parseConnectionURI,
  requestRemoteIDs,
  sendEventsToRemote,
  EventManifestEntry
} from '@/services/nrc'
import type { SyncProgress, RemoteConnection } from '@/services/nrc'

// Kinds to sync bidirectionally
const SYNC_KINDS = [0, 3, 10000, 10001, 10002, 10003, 10012, 30002]

// Storage keys
const STORAGE_KEY_ENABLED = 'nrc:enabled'
const STORAGE_KEY_CONNECTIONS = 'nrc:connections'
const STORAGE_KEY_REMOTE_CONNECTIONS = 'nrc:remoteConnections'
const STORAGE_KEY_RENDEZVOUS_URL = 'nrc:rendezvousUrl'

// No default rendezvous relay - user must configure to protect privacy
// Using a default would leak NRC connection attempts to a third party
const DEFAULT_RENDEZVOUS_URL = ''

interface NRCContextType {
  // Listener State (this device accepts connections)
  isEnabled: boolean
  isListening: boolean
  isConnected: boolean
  connections: NRCConnection[] // Devices authorized to connect to us
  activeSessions: number
  rendezvousUrl: string

  // Client State (this device connects to others)
  remoteConnections: RemoteConnection[] // Devices we connect to
  isSyncing: boolean
  syncProgress: SyncProgress | null

  // Listener Actions
  enable: () => Promise<void>
  disable: () => void
  addConnection: (label: string, rendezvousUrlOverride?: string) => Promise<{ uri: string; connection: NRCConnection }>
  removeConnection: (id: string) => Promise<void>
  getConnectionURI: (connection: NRCConnection) => string
  setRendezvousUrl: (url: string) => void

  // Client Actions
  addRemoteConnection: (uri: string, label: string) => Promise<RemoteConnection>
  removeRemoteConnection: (id: string) => Promise<void>
  testRemoteConnection: (id: string) => Promise<boolean>
  syncFromDevice: (id: string, filters?: Filter[]) => Promise<Event[]>
  syncAllRemotes: (filters?: Filter[]) => Promise<Event[]>
}

const NRCContext = createContext<NRCContextType | undefined>(undefined)

export const useNRC = () => {
  const context = useContext(NRCContext)
  if (!context) {
    throw new Error('useNRC must be used within an NRCProvider')
  }
  return context
}

interface NRCProviderProps {
  children: ReactNode
}

export function NRCProvider({ children }: NRCProviderProps) {
  const { pubkey } = useNostr()

  // ===== Listener State =====
  const [isEnabled, setIsEnabled] = useState<boolean>(() => {
    const stored = localStorage.getItem(STORAGE_KEY_ENABLED)
    return stored === 'true'
  })

  const [connections, setConnections] = useState<NRCConnection[]>(() => {
    const stored = localStorage.getItem(STORAGE_KEY_CONNECTIONS)
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        return []
      }
    }
    return []
  })

  const [rendezvousUrl, setRendezvousUrlState] = useState<string>(() => {
    return localStorage.getItem(STORAGE_KEY_RENDEZVOUS_URL) || DEFAULT_RENDEZVOUS_URL
  })

  const [isListening, setIsListening] = useState(false)
  const [isConnected, setIsConnected] = useState(false)
  const [activeSessions, setActiveSessions] = useState(0)

  // ===== Client State =====
  const [remoteConnections, setRemoteConnections] = useState<RemoteConnection[]>(() => {
    const stored = localStorage.getItem(STORAGE_KEY_REMOTE_CONNECTIONS)
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        return []
      }
    }
    return []
  })

  const [isSyncing, setIsSyncing] = useState(false)
  const [syncProgress, setSyncProgress] = useState<SyncProgress | null>(null)

  const listenerService = getNRCListenerService()

  // ===== Persist State =====
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_ENABLED, String(isEnabled))
  }, [isEnabled])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_CONNECTIONS, JSON.stringify(connections))
  }, [connections])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_REMOTE_CONNECTIONS, JSON.stringify(remoteConnections))
  }, [remoteConnections])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_RENDEZVOUS_URL, rendezvousUrl)
  }, [rendezvousUrl])

  // ===== Listener Logic =====
  const buildAuthorizedSecrets = useCallback((): Map<string, string> => {
    const map = new Map<string, string>()
    for (const conn of connections) {
      if (conn.secret && conn.clientPubkey) {
        map.set(conn.clientPubkey, conn.label)
      }
    }
    return map
  }, [connections])

  useEffect(() => {
    if (!isEnabled || !client.signer || !pubkey) {
      if (listenerService.isRunning()) {
        listenerService.stop()
        setIsListening(false)
        setIsConnected(false)
        setActiveSessions(0)
      }
      return
    }

    // Stop existing listener before starting with new config
    if (listenerService.isRunning()) {
      listenerService.stop()
    }

    let statusInterval: ReturnType<typeof setInterval> | null = null

    const startListener = async () => {
      try {
        const config: NRCListenerConfig = {
          rendezvousUrl,
          signer: client.signer!,
          authorizedSecrets: buildAuthorizedSecrets()
        }

        console.log('[NRC] Starting listener with', config.authorizedSecrets.size, 'authorized clients')

        listenerService.setOnSessionChange((count) => {
          setActiveSessions(count)
        })

        await listenerService.start(config)
        setIsListening(true)
        setIsConnected(listenerService.isConnected())

        statusInterval = setInterval(() => {
          setIsConnected(listenerService.isConnected())
          setActiveSessions(listenerService.getActiveSessionCount())
        }, 5000)
      } catch (error) {
        console.error('[NRC] Failed to start listener:', error)
        setIsListening(false)
        setIsConnected(false)
      }
    }

    startListener()

    return () => {
      if (statusInterval) {
        clearInterval(statusInterval)
      }
      listenerService.stop()
      setIsListening(false)
      setIsConnected(false)
      setActiveSessions(0)
    }
  }, [isEnabled, pubkey, rendezvousUrl, buildAuthorizedSecrets])

  useEffect(() => {
    if (!isEnabled || !client.signer || !pubkey) return
  }, [connections, isEnabled, pubkey])

  // ===== Auto-sync remote connections (bidirectional) =====
  // Sync interval: 15 minutes
  const AUTO_SYNC_INTERVAL = 15 * 60 * 1000
  // Minimum time between syncs for the same connection: 5 minutes
  const MIN_SYNC_INTERVAL = 5 * 60 * 1000

  /**
   * Get local events for sync kinds and build manifest
   */
  const getLocalEventsAndManifest = async (): Promise<{
    events: Event[]
    manifest: EventManifestEntry[]
  }> => {
    const events = await indexedDb.queryEventsForNRC([{ kinds: SYNC_KINDS, limit: 1000 }])
    const manifest: EventManifestEntry[] = events.map((e) => ({
      kind: e.kind,
      id: e.id,
      created_at: e.created_at,
      d: e.tags.find((t) => t[0] === 'd')?.[1]
    }))
    return { events, manifest }
  }

  /**
   * Diff manifests to find what each side needs
   * For replaceable events: compare by (kind, pubkey, d) and use newer created_at
   */
  const diffManifests = (
    local: EventManifestEntry[],
    remote: EventManifestEntry[],
    localEvents: Event[]
  ): { toSend: Event[]; toFetch: string[] } => {
    // Build maps keyed by (kind, d) for replaceable events
    const localMap = new Map<string, EventManifestEntry>()
    const localEventsMap = new Map<string, Event>()

    for (let i = 0; i < local.length; i++) {
      const entry = local[i]
      const key = `${entry.kind}:${entry.d || ''}`
      const existing = localMap.get(key)
      // Keep the newer one
      if (!existing || entry.created_at > existing.created_at) {
        localMap.set(key, entry)
        localEventsMap.set(entry.id, localEvents[i])
      }
    }

    const remoteMap = new Map<string, EventManifestEntry>()
    for (const entry of remote) {
      const key = `${entry.kind}:${entry.d || ''}`
      const existing = remoteMap.get(key)
      if (!existing || entry.created_at > existing.created_at) {
        remoteMap.set(key, entry)
      }
    }

    const toSend: Event[] = []
    const toFetch: string[] = []

    // Find events we have that are newer than remote's (or remote doesn't have)
    for (const [key, localEntry] of localMap) {
      const remoteEntry = remoteMap.get(key)
      if (!remoteEntry || localEntry.created_at > remoteEntry.created_at) {
        const event = localEventsMap.get(localEntry.id)
        if (event) {
          toSend.push(event)
        }
      }
    }

    // Find events remote has that are newer than ours (or we don't have)
    for (const [key, remoteEntry] of remoteMap) {
      const localEntry = localMap.get(key)
      if (!localEntry || remoteEntry.created_at > localEntry.created_at) {
        toFetch.push(remoteEntry.id)
      }
    }

    return { toSend, toFetch }
  }

  useEffect(() => {
    // Only auto-sync if we have remote connections and a signer
    if (remoteConnections.length === 0 || !client.signer || !pubkey) {
      return
    }

    // Don't auto-sync if already syncing
    if (isSyncing) {
      return
    }

    const bidirectionalSync = async () => {
      const now = Date.now()

      // Find connections that need syncing
      const needsSync = remoteConnections.filter(
        (c) => !c.lastSync || (now - c.lastSync) > MIN_SYNC_INTERVAL
      )

      if (needsSync.length === 0) {
        return
      }

      console.log(`[NRC] Bidirectional sync: ${needsSync.length} connection(s) need syncing`)

      for (const remote of needsSync) {
        if (isSyncing) break

        try {
          console.log(`[NRC] Bidirectional sync with ${remote.label}...`)
          setIsSyncing(true)
          setSyncProgress({ phase: 'connecting', eventsReceived: 0 })

          // Step 1: Get remote's event IDs
          setSyncProgress({ phase: 'requesting', eventsReceived: 0, message: 'Getting remote event list...' })
          const remoteManifest = await requestRemoteIDs(
            remote.uri,
            [{ kinds: SYNC_KINDS, limit: 1000 }]
          )
          console.log(`[NRC] Remote has ${remoteManifest.length} events`)

          // Step 2: Get our local events and manifest
          const { events: localEvents, manifest: localManifest } = await getLocalEventsAndManifest()
          console.log(`[NRC] Local has ${localManifest.length} events`)

          // Step 3: Diff to find what each side needs
          const { toSend, toFetch } = diffManifests(localManifest, remoteManifest, localEvents)
          console.log(`[NRC] Diff: sending ${toSend.length}, fetching ${toFetch.length}`)

          let eventsSent = 0
          let eventsReceived = 0

          // Step 4: Send events remote needs
          if (toSend.length > 0) {
            setSyncProgress({ phase: 'sending', eventsReceived: 0, eventsSent: 0, message: `Sending ${toSend.length} events...` })

            eventsSent = await sendEventsToRemote(
              remote.uri,
              toSend,
              (progress) => setSyncProgress({ ...progress, message: `Sending events... (${progress.eventsSent || 0}/${toSend.length})` })
            )
            console.log(`[NRC] Sent ${eventsSent} events to ${remote.label}`)
          }

          // Step 5: Fetch events we need using regular filter queries
          if (toFetch.length > 0) {
            setSyncProgress({ phase: 'receiving', eventsReceived: 0, eventsSent, message: `Fetching ${toFetch.length} events...` })

            // Fetch by ID in batches (relay may limit number of IDs per filter)
            const BATCH_SIZE = 50
            const fetchedEvents: Event[] = []

            for (let i = 0; i < toFetch.length; i += BATCH_SIZE) {
              const batch = toFetch.slice(i, i + BATCH_SIZE)
              const events = await syncFromRemote(
                remote.uri,
                [{ ids: batch }],
                (progress) => setSyncProgress({
                  ...progress,
                  eventsSent,
                  message: `Fetching events... (${fetchedEvents.length + progress.eventsReceived}/${toFetch.length})`
                })
              )
              fetchedEvents.push(...events)
            }

            // Store fetched events
            for (const event of fetchedEvents) {
              try {
                await indexedDb.putReplaceableEvent(event)
              } catch {
                // Ignore storage errors
              }
            }

            eventsReceived = fetchedEvents.length
            console.log(`[NRC] Received ${eventsReceived} events from ${remote.label}`)
          }

          // Update last sync time
          setRemoteConnections((prev) =>
            prev.map((c) =>
              c.id === remote.id
                ? { ...c, lastSync: Date.now(), eventCount: eventsReceived }
                : c
            )
          )

          console.log(`[NRC] Bidirectional sync complete with ${remote.label}: sent ${eventsSent}, received ${eventsReceived}`)
        } catch (err) {
          console.error(`[NRC] Bidirectional sync failed for ${remote.label}:`, err)
        } finally {
          setIsSyncing(false)
          setSyncProgress(null)
        }
      }
    }

    // Run initial sync after a short delay
    const initialTimer = setTimeout(bidirectionalSync, 3000)

    // Set up periodic sync
    const intervalTimer = setInterval(bidirectionalSync, AUTO_SYNC_INTERVAL)

    return () => {
      clearTimeout(initialTimer)
      clearInterval(intervalTimer)
    }
  }, [remoteConnections.length, pubkey, isSyncing])

  // ===== Listener Actions =====
  const enable = useCallback(async () => {
    if (!client.signer) {
      throw new Error('Signer required to enable NRC')
    }
    if (!rendezvousUrl) {
      throw new Error('Rendezvous relay URL required - configure in NRC settings')
    }
    setIsEnabled(true)
  }, [rendezvousUrl])

  const disable = useCallback(() => {
    setIsEnabled(false)
    listenerService.stop()
    setIsListening(false)
    setIsConnected(false)
    setActiveSessions(0)
  }, [])

  const addConnection = useCallback(
    async (label: string, rendezvousUrlOverride?: string): Promise<{ uri: string; connection: NRCConnection }> => {
      if (!pubkey) {
        throw new Error('Not logged in')
      }

      // Use override if provided, otherwise use global rendezvous URL
      const effectiveRendezvousUrl = rendezvousUrlOverride || rendezvousUrl
      if (!effectiveRendezvousUrl) {
        throw new Error('Rendezvous relay URL required - configure in NRC settings')
      }

      const id = crypto.randomUUID()
      const createdAt = Date.now()

      const result = generateConnectionURI(pubkey, effectiveRendezvousUrl, undefined, label)
      const uri = result.uri
      const connection: NRCConnection = {
        id,
        label,
        secret: result.secret,
        clientPubkey: result.clientPubkey,
        createdAt
      }

      setConnections((prev) => [...prev, connection])

      return { uri, connection }
    },
    [pubkey, rendezvousUrl]
  )

  const removeConnection = useCallback(async (id: string) => {
    setConnections((prev) => prev.filter((c) => c.id !== id))
  }, [])

  const getConnectionURI = useCallback(
    (connection: NRCConnection): string => {
      if (!pubkey) {
        throw new Error('Not logged in')
      }

      if (!connection.secret) {
        throw new Error('Connection has no secret')
      }

      const result = generateConnectionURI(
        pubkey,
        rendezvousUrl,
        connection.secret,
        connection.label
      )
      return result.uri
    },
    [pubkey, rendezvousUrl]
  )

  const setRendezvousUrl = useCallback((url: string) => {
    setRendezvousUrlState(url)
  }, [])

  // ===== Client Actions =====
  const addRemoteConnection = useCallback(
    async (uri: string, label: string): Promise<RemoteConnection> => {
      // Validate and parse the URI
      const parsed = parseConnectionURI(uri)

      const remoteConnection: RemoteConnection = {
        id: crypto.randomUUID(),
        uri,
        label,
        relayPubkey: parsed.relayPubkey,
        rendezvousUrl: parsed.rendezvousUrl
      }

      setRemoteConnections((prev) => [...prev, remoteConnection])

      return remoteConnection
    },
    []
  )

  const removeRemoteConnection = useCallback(async (id: string) => {
    setRemoteConnections((prev) => prev.filter((c) => c.id !== id))
  }, [])

  const syncFromDevice = useCallback(
    async (id: string, filters?: Filter[]): Promise<Event[]> => {
      const remote = remoteConnections.find((c) => c.id === id)
      if (!remote) {
        throw new Error('Remote connection not found')
      }

      setIsSyncing(true)
      setSyncProgress({ phase: 'connecting', eventsReceived: 0 })

      try {
        // Default filters: sync everything
        const syncFilters = filters || [
          { kinds: [0, 3, 10000, 10001, 10002, 10003, 10012, 30002], limit: 1000 }
        ]

        const events = await syncFromRemote(
          remote.uri,
          syncFilters,
          (progress) => setSyncProgress(progress)
        )

        // Store synced events in IndexedDB
        for (const event of events) {
          try {
            await indexedDb.putReplaceableEvent(event)
          } catch (err) {
            console.warn('[NRC] Failed to store event:', err)
          }
        }

        // Update last sync time
        setRemoteConnections((prev) =>
          prev.map((c) =>
            c.id === id ? { ...c, lastSync: Date.now(), eventCount: events.length } : c
          )
        )

        return events
      } finally {
        setIsSyncing(false)
        setSyncProgress(null)
      }
    },
    [remoteConnections]
  )

  const syncAllRemotes = useCallback(
    async (filters?: Filter[]): Promise<Event[]> => {
      const allEvents: Event[] = []

      for (const remote of remoteConnections) {
        try {
          const events = await syncFromDevice(remote.id, filters)
          allEvents.push(...events)
        } catch (error) {
          console.error(`[NRC] Failed to sync from ${remote.label}:`, error)
        }
      }

      return allEvents
    },
    [remoteConnections, syncFromDevice]
  )

  const testRemoteConnection = useCallback(
    async (id: string): Promise<boolean> => {
      const remote = remoteConnections.find((c) => c.id === id)
      if (!remote) {
        throw new Error('Remote connection not found')
      }

      setIsSyncing(true)
      setSyncProgress({ phase: 'connecting', eventsReceived: 0, message: 'Testing connection...' })

      try {
        const result = await testConnection(
          remote.uri,
          (progress) => setSyncProgress(progress)
        )

        // Update connection to mark it as tested
        setRemoteConnections((prev) =>
          prev.map((c) =>
            c.id === id ? { ...c, lastSync: Date.now(), eventCount: 0 } : c
          )
        )

        return result
      } finally {
        setIsSyncing(false)
        setSyncProgress(null)
      }
    },
    [remoteConnections]
  )

  const value: NRCContextType = {
    // Listener
    isEnabled,
    isListening,
    isConnected,
    connections,
    activeSessions,
    rendezvousUrl,
    enable,
    disable,
    addConnection,
    removeConnection,
    getConnectionURI,
    setRendezvousUrl,
    // Client
    remoteConnections,
    isSyncing,
    syncProgress,
    addRemoteConnection,
    removeRemoteConnection,
    testRemoteConnection,
    syncFromDevice,
    syncAllRemotes
  }

  return <NRCContext.Provider value={value}>{children}</NRCContext.Provider>
}
