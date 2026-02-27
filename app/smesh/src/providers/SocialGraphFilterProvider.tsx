import { useFetchFollowGraph } from '@/hooks/useFetchFollowGraph'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { useNostr } from './NostrProvider'

type TSocialGraphFilterContext = {
  // Settings
  proximityLevel: number | null // null = disabled, 1 = direct follows, 2 = follows of follows
  includeMode: boolean // true = include only graph members, false = exclude graph members
  updateProximityLevel: (level: number | null) => void
  updateIncludeMode: (include: boolean) => void

  // Cached data
  graphPubkeys: Set<string> // Pre-computed Set for O(1) lookup
  graphPubkeyCount: number
  isLoading: boolean

  // Filter function for use in feeds
  isPubkeyAllowed: (pubkey: string) => boolean
}

const SocialGraphFilterContext = createContext<TSocialGraphFilterContext | undefined>(undefined)

export const useSocialGraphFilter = () => {
  const context = useContext(SocialGraphFilterContext)
  if (!context) {
    throw new Error('useSocialGraphFilter must be used within a SocialGraphFilterProvider')
  }
  return context
}

export function SocialGraphFilterProvider({ children }: { children: React.ReactNode }) {
  const { pubkey } = useNostr()
  const [proximityLevel, setProximityLevel] = useState<number | null>(
    storage.getSocialGraphProximity()
  )
  const [includeMode, setIncludeMode] = useState<boolean>(storage.getSocialGraphIncludeMode())

  // Fetch the follow graph when proximity is enabled
  const { pubkeysByDepth, isLoading } = useFetchFollowGraph(
    proximityLevel !== null ? pubkey : null,
    proximityLevel ?? 1
  )

  // Build the Set of graph pubkeys (always includes self)
  const graphPubkeys = useMemo(() => {
    const set = new Set<string>()

    // Always include self in the graph
    if (pubkey) {
      set.add(pubkey)
    }

    // Add pubkeys up to selected depth
    if (proximityLevel && pubkeysByDepth.length) {
      for (let depth = 0; depth < proximityLevel && depth < pubkeysByDepth.length; depth++) {
        pubkeysByDepth[depth].forEach((pk) => set.add(pk))
      }
    }

    return set
  }, [pubkey, proximityLevel, pubkeysByDepth])

  const graphPubkeyCount = graphPubkeys.size

  const updateProximityLevel = useCallback((level: number | null) => {
    storage.setSocialGraphProximity(level)
    setProximityLevel(level)
    dispatchSettingsChanged()
  }, [])

  const updateIncludeMode = useCallback((include: boolean) => {
    storage.setSocialGraphIncludeMode(include)
    setIncludeMode(include)
    dispatchSettingsChanged()
  }, [])

  const isPubkeyAllowed = useCallback(
    (targetPubkey: string): boolean => {
      // If filter disabled, allow all
      if (proximityLevel === null) return true

      // If loading, allow all (graceful degradation)
      if (isLoading) return true

      // Always allow self
      if (targetPubkey === pubkey) return true

      const isInGraph = graphPubkeys.has(targetPubkey)

      // Include mode: only allow if in graph
      // Exclude mode: only allow if NOT in graph
      return includeMode ? isInGraph : !isInGraph
    },
    [proximityLevel, isLoading, graphPubkeys, includeMode, pubkey]
  )

  return (
    <SocialGraphFilterContext.Provider
      value={{
        proximityLevel,
        includeMode,
        updateProximityLevel,
        updateIncludeMode,
        graphPubkeys,
        graphPubkeyCount,
        isLoading,
        isPubkeyAllowed
      }}
    >
      {children}
    </SocialGraphFilterContext.Provider>
  )
}
