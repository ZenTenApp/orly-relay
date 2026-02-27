import client from '@/services/client.service'
import graphQueryService from '@/services/graph-query.service'
import { useEffect, useState } from 'react'

interface FollowGraphResult {
  /** Pubkeys by depth (index 0 = direct follows, index 1 = follows of follows, etc.) */
  pubkeysByDepth: string[][]
  /** Whether graph query was used (vs traditional method) */
  usedGraphQuery: boolean
  /** Loading state */
  isLoading: boolean
  /** Error if any */
  error: Error | null
}

/**
 * Hook for fetching follow graph with automatic graph query optimization.
 * Falls back to traditional method when graph queries are not available.
 *
 * @param pubkey - The seed pubkey to fetch follows for
 * @param depth - How many levels deep to fetch (1 = direct follows, 2 = + follows of follows)
 * @param relayUrls - Optional relay URLs to try for graph queries
 */
export function useFetchFollowGraph(
  pubkey: string | null,
  depth: number = 1,
  relayUrls?: string[]
): FollowGraphResult {
  const [pubkeysByDepth, setPubkeysByDepth] = useState<string[][]>([])
  const [usedGraphQuery, setUsedGraphQuery] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    if (!pubkey) {
      setPubkeysByDepth([])
      setIsLoading(false)
      return
    }

    const fetchFollowGraph = async () => {
      setIsLoading(true)
      setError(null)
      setUsedGraphQuery(false)

      const urls = relayUrls ?? client.currentRelays

      try {
        // Try graph query first
        const graphResult = await graphQueryService.queryFollowGraph(urls, pubkey, depth)

        if (graphResult?.pubkeys_by_depth?.length) {
          setPubkeysByDepth(graphResult.pubkeys_by_depth)
          setUsedGraphQuery(true)
          setIsLoading(false)
          return
        }

        // Fallback to traditional method
        const result = await fetchTraditionalFollowGraph(pubkey, depth)
        setPubkeysByDepth(result)
      } catch (e) {
        console.error('Failed to fetch follow graph:', e)
        setError(e instanceof Error ? e : new Error('Unknown error'))
      } finally {
        setIsLoading(false)
      }
    }

    fetchFollowGraph()
  }, [pubkey, depth, relayUrls?.join(',')])

  return { pubkeysByDepth, usedGraphQuery, isLoading, error }
}

/**
 * Traditional method for fetching follow graph (used as fallback)
 */
async function fetchTraditionalFollowGraph(pubkey: string, depth: number): Promise<string[][]> {
  const result: string[][] = []

  // Depth 1: Direct follows
  const directFollows = await client.fetchFollowings(pubkey)
  result.push(directFollows)

  if (depth < 2 || directFollows.length === 0) {
    return result
  }

  // Depth 2: Follows of follows
  // Note: This is expensive - N queries for N follows
  const followsOfFollows = new Set<string>()
  const directFollowsSet = new Set(directFollows)
  directFollowsSet.add(pubkey) // Exclude self

  // Fetch in batches to avoid overwhelming relays
  const batchSize = 10
  for (let i = 0; i < directFollows.length; i += batchSize) {
    const batch = directFollows.slice(i, i + batchSize)
    const batchResults = await Promise.all(batch.map((pk) => client.fetchFollowings(pk)))

    for (const follows of batchResults) {
      for (const pk of follows) {
        if (!directFollowsSet.has(pk)) {
          followsOfFollows.add(pk)
        }
      }
    }
  }

  result.push(Array.from(followsOfFollows))

  return result
}

/**
 * Get all pubkeys from a follow graph result as a flat array
 */
export function flattenFollowGraph(pubkeysByDepth: string[][]): string[] {
  return pubkeysByDepth.flat()
}

/**
 * Check if a pubkey is in the follow graph at a specific depth
 */
export function isPubkeyAtDepth(
  pubkeysByDepth: string[][],
  pubkey: string,
  depth: number
): boolean {
  const depthIndex = depth - 1
  if (depthIndex < 0 || depthIndex >= pubkeysByDepth.length) {
    return false
  }
  return pubkeysByDepth[depthIndex].includes(pubkey)
}

/**
 * Check if a pubkey is anywhere in the follow graph
 */
export function isPubkeyInGraph(pubkeysByDepth: string[][], pubkey: string): boolean {
  return pubkeysByDepth.some((depth) => depth.includes(pubkey))
}
