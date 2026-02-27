import { normalizeUrl } from '@/lib/url'
import { TGraphQueryCapability, TRelayInfo } from '@/types'
import { GraphQuery, GraphResponse } from '@/types/graph'
import { Event as NEvent, Filter, SimplePool, verifyEvent } from 'nostr-tools'
import relayInfoService from './relay-info.service'
import storage from './local-storage.service'

// Graph query response kinds (relay-signed)
const GRAPH_RESPONSE_KINDS = {
  FOLLOWS: 39000,
  MENTIONS: 39001,
  THREAD: 39002
}

class GraphQueryService {
  static instance: GraphQueryService

  private pool: SimplePool
  private capabilityCache = new Map<string, TGraphQueryCapability | null>()
  private capabilityFetchPromises = new Map<string, Promise<TGraphQueryCapability | null>>()

  constructor() {
    this.pool = new SimplePool()
  }

  public static getInstance(): GraphQueryService {
    if (!GraphQueryService.instance) {
      GraphQueryService.instance = new GraphQueryService()
    }
    return GraphQueryService.instance
  }

  /**
   * Check if graph queries are enabled in settings
   */
  isEnabled(): boolean {
    return storage.getGraphQueriesEnabled()
  }

  /**
   * Get relay's graph query capability via NIP-11
   */
  async getRelayCapability(url: string): Promise<TGraphQueryCapability | null> {
    const normalizedUrl = normalizeUrl(url)

    // Check memory cache first
    if (this.capabilityCache.has(normalizedUrl)) {
      return this.capabilityCache.get(normalizedUrl) ?? null
    }

    // Check if already fetching
    const existingPromise = this.capabilityFetchPromises.get(normalizedUrl)
    if (existingPromise) {
      return existingPromise
    }

    // Fetch capability
    const fetchPromise = this._fetchRelayCapability(normalizedUrl)
    this.capabilityFetchPromises.set(normalizedUrl, fetchPromise)

    try {
      const capability = await fetchPromise
      this.capabilityCache.set(normalizedUrl, capability)
      return capability
    } finally {
      this.capabilityFetchPromises.delete(normalizedUrl)
    }
  }

  private async _fetchRelayCapability(url: string): Promise<TGraphQueryCapability | null> {
    try {
      const relayInfo = (await relayInfoService.getRelayInfo(url)) as TRelayInfo | undefined

      if (!relayInfo?.graph_query?.enabled) {
        return null
      }

      return relayInfo.graph_query
    } catch {
      return null
    }
  }

  /**
   * Check if a relay supports a specific graph query method
   */
  async supportsMethod(url: string, method: GraphQuery['method']): Promise<boolean> {
    const capability = await this.getRelayCapability(url)
    if (!capability?.enabled) return false
    return capability.methods.includes(method)
  }

  /**
   * Find a relay supporting graph queries from a list of URLs
   */
  async findGraphCapableRelay(
    urls: string[],
    method?: GraphQuery['method']
  ): Promise<string | null> {
    if (!this.isEnabled()) return null

    // Check capabilities in parallel
    const results = await Promise.all(
      urls.map(async (url) => {
        const capability = await this.getRelayCapability(url)
        if (!capability?.enabled) return null
        if (method && !capability.methods.includes(method)) return null
        return url
      })
    )

    return results.find((url) => url !== null) ?? null
  }

  /**
   * Execute a graph query against a specific relay
   */
  async executeQuery(relayUrl: string, query: GraphQuery): Promise<GraphResponse | null> {
    if (!this.isEnabled()) return null

    const capability = await this.getRelayCapability(relayUrl)
    if (!capability?.enabled) {
      console.warn(`Relay ${relayUrl} does not support graph queries`)
      return null
    }

    // Validate method support
    if (!capability.methods.includes(query.method)) {
      console.warn(`Relay ${relayUrl} does not support method: ${query.method}`)
      return null
    }

    // Validate depth
    const depth = query.depth ?? 1
    if (depth > capability.max_depth) {
      console.warn(`Requested depth ${depth} exceeds relay max ${capability.max_depth}`)
      query = { ...query, depth: capability.max_depth }
    }

    // Build the filter with graph extension
    // The _graph field is a custom extension not in the standard Filter type
    const filter = {
      _graph: query
    } as Filter

    // Determine expected response kind
    const expectedKind = this.getExpectedResponseKind(query.method)

    return new Promise<GraphResponse | null>(async (resolve) => {
      let resolved = false
      const timeout = setTimeout(() => {
        if (!resolved) {
          resolved = true
          resolve(null)
        }
      }, 30000) // 30s timeout for graph queries

      try {
        const relay = await this.pool.ensureRelay(relayUrl, { connectionTimeout: 5000 })

        const sub = relay.subscribe([filter], {
          onevent: (event: NEvent) => {
            // Verify it's a relay-signed graph response
            if (event.kind !== expectedKind) return

            // Verify event signature
            if (!verifyEvent(event)) {
              console.warn('Invalid signature on graph response')
              return
            }

            try {
              const response = JSON.parse(event.content) as GraphResponse
              if (!resolved) {
                resolved = true
                clearTimeout(timeout)
                sub.close()
                resolve(response)
              }
            } catch (e) {
              console.error('Failed to parse graph response:', e)
            }
          },
          oneose: () => {
            // If we got EOSE without a response, the query may not be supported
            if (!resolved) {
              resolved = true
              clearTimeout(timeout)
              sub.close()
              resolve(null)
            }
          }
        })
      } catch (error) {
        console.error('Failed to connect to relay for graph query:', error)
        if (!resolved) {
          resolved = true
          clearTimeout(timeout)
          resolve(null)
        }
      }
    })
  }

  private getExpectedResponseKind(method: GraphQuery['method']): number {
    switch (method) {
      case 'follows':
      case 'followers':
        return GRAPH_RESPONSE_KINDS.FOLLOWS
      case 'mentions':
        return GRAPH_RESPONSE_KINDS.MENTIONS
      case 'thread':
        return GRAPH_RESPONSE_KINDS.THREAD
      default:
        return GRAPH_RESPONSE_KINDS.FOLLOWS
    }
  }

  /**
   * High-level method: Query follow graph with fallback
   */
  async queryFollowGraph(
    relayUrls: string[],
    seed: string,
    depth: number = 1
  ): Promise<GraphResponse | null> {
    const graphRelay = await this.findGraphCapableRelay(relayUrls, 'follows')
    if (!graphRelay) return null

    return this.executeQuery(graphRelay, {
      method: 'follows',
      seed,
      depth
    })
  }

  /**
   * High-level method: Query follower graph
   */
  async queryFollowerGraph(
    relayUrls: string[],
    seed: string,
    depth: number = 1
  ): Promise<GraphResponse | null> {
    const graphRelay = await this.findGraphCapableRelay(relayUrls, 'followers')
    if (!graphRelay) return null

    return this.executeQuery(graphRelay, {
      method: 'followers',
      seed,
      depth
    })
  }

  /**
   * High-level method: Query thread with optional ref aggregation
   */
  async queryThread(
    relayUrls: string[],
    eventId: string,
    depth: number = 10,
    options?: {
      inboundRefKinds?: number[]
      outboundRefKinds?: number[]
    }
  ): Promise<GraphResponse | null> {
    const graphRelay = await this.findGraphCapableRelay(relayUrls, 'thread')
    if (!graphRelay) return null

    const query: GraphQuery = {
      method: 'thread',
      seed: eventId,
      depth
    }

    if (options?.inboundRefKinds?.length) {
      query.inbound_refs = [{ kinds: options.inboundRefKinds, from_depth: 0 }]
    }

    if (options?.outboundRefKinds?.length) {
      query.outbound_refs = [{ kinds: options.outboundRefKinds, from_depth: 0 }]
    }

    return this.executeQuery(graphRelay, query)
  }

  /**
   * High-level method: Query mentions with aggregation
   */
  async queryMentions(
    relayUrls: string[],
    pubkey: string,
    options?: {
      inboundRefKinds?: number[] // e.g., [7, 9735] for reactions and zaps
    }
  ): Promise<GraphResponse | null> {
    const graphRelay = await this.findGraphCapableRelay(relayUrls, 'mentions')
    if (!graphRelay) return null

    const query: GraphQuery = {
      method: 'mentions',
      seed: pubkey
    }

    if (options?.inboundRefKinds?.length) {
      query.inbound_refs = [{ kinds: options.inboundRefKinds, from_depth: 0 }]
    }

    return this.executeQuery(graphRelay, query)
  }

  /**
   * Clear capability cache for a relay (e.g., when relay info is updated)
   */
  clearCapabilityCache(url?: string): void {
    if (url) {
      const normalizedUrl = normalizeUrl(url)
      this.capabilityCache.delete(normalizedUrl)
    } else {
      this.capabilityCache.clear()
    }
  }
}

const instance = GraphQueryService.getInstance()
export default instance
