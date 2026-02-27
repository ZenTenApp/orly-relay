import { useNostr } from '@/providers/NostrProvider'
import relayDiscoveryService from '@/services/relay-discovery.service'
import { useEffect } from 'react'

/**
 * Background relay discovery component.
 * Automatically runs relay discovery after app initialization completes,
 * if no valid cached result exists. This ensures the discovered relay list
 * is populated for use as fallback relays and progressive event querying.
 */
export default function BackgroundRelayDiscovery() {
  const { isInitialized } = useNostr()

  useEffect(() => {
    if (!isInitialized) return

    // Delay to let initial feed loading and relay connections settle first
    const timer = setTimeout(() => {
      relayDiscoveryService.discoverIfNeeded()
    }, 15000)

    return () => clearTimeout(timer)
  }, [isInitialized])

  return null
}
