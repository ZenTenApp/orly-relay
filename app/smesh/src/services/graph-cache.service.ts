import { GraphResponse } from '@/types/graph'
import { TGraphQueryCapability } from '@/types'

const DB_NAME = 'smesh-graph-cache'
const DB_VERSION = 1

// Store names
const STORES = {
  FOLLOW_GRAPH: 'followGraphResults',
  THREAD: 'threadResults',
  RELAY_CAPABILITIES: 'relayCapabilities'
}

// Cache expiry times (in milliseconds)
const CACHE_EXPIRY = {
  FOLLOW_GRAPH: 5 * 60 * 1000, // 5 minutes
  THREAD: 10 * 60 * 1000, // 10 minutes
  RELAY_CAPABILITY: 60 * 60 * 1000 // 1 hour
}

interface CachedEntry<T> {
  data: T
  timestamp: number
}

class GraphCacheService {
  static instance: GraphCacheService
  private db: IDBDatabase | null = null
  private dbPromise: Promise<IDBDatabase> | null = null

  public static getInstance(): GraphCacheService {
    if (!GraphCacheService.instance) {
      GraphCacheService.instance = new GraphCacheService()
    }
    return GraphCacheService.instance
  }

  private async getDB(): Promise<IDBDatabase> {
    if (this.db) return this.db
    if (this.dbPromise) return this.dbPromise

    this.dbPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(DB_NAME, DB_VERSION)

      request.onerror = () => {
        console.error('Failed to open graph cache database:', request.error)
        reject(request.error)
      }

      request.onsuccess = () => {
        this.db = request.result
        resolve(request.result)
      }

      request.onupgradeneeded = (event) => {
        const db = (event.target as IDBOpenDBRequest).result

        // Create stores if they don't exist
        if (!db.objectStoreNames.contains(STORES.FOLLOW_GRAPH)) {
          db.createObjectStore(STORES.FOLLOW_GRAPH)
        }
        if (!db.objectStoreNames.contains(STORES.THREAD)) {
          db.createObjectStore(STORES.THREAD)
        }
        if (!db.objectStoreNames.contains(STORES.RELAY_CAPABILITIES)) {
          db.createObjectStore(STORES.RELAY_CAPABILITIES)
        }
      }
    })

    return this.dbPromise
  }

  /**
   * Cache a follow graph query result
   */
  async cacheFollowGraph(
    pubkey: string,
    depth: number,
    result: GraphResponse
  ): Promise<void> {
    try {
      const db = await this.getDB()
      const key = `${pubkey}:${depth}`
      const entry: CachedEntry<GraphResponse> = {
        data: result,
        timestamp: Date.now()
      }

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.FOLLOW_GRAPH, 'readwrite')
        const store = tx.objectStore(STORES.FOLLOW_GRAPH)
        const request = store.put(entry, key)

        request.onsuccess = () => resolve()
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to cache follow graph:', error)
    }
  }

  /**
   * Get cached follow graph result
   */
  async getCachedFollowGraph(
    pubkey: string,
    depth: number
  ): Promise<GraphResponse | null> {
    try {
      const db = await this.getDB()
      const key = `${pubkey}:${depth}`

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.FOLLOW_GRAPH, 'readonly')
        const store = tx.objectStore(STORES.FOLLOW_GRAPH)
        const request = store.get(key)

        request.onsuccess = () => {
          const entry = request.result as CachedEntry<GraphResponse> | undefined
          if (!entry) {
            resolve(null)
            return
          }

          // Check if cache is expired
          if (Date.now() - entry.timestamp > CACHE_EXPIRY.FOLLOW_GRAPH) {
            resolve(null)
            return
          }

          resolve(entry.data)
        }
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to get cached follow graph:', error)
      return null
    }
  }

  /**
   * Cache a thread query result
   */
  async cacheThread(eventId: string, result: GraphResponse): Promise<void> {
    try {
      const db = await this.getDB()
      const entry: CachedEntry<GraphResponse> = {
        data: result,
        timestamp: Date.now()
      }

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.THREAD, 'readwrite')
        const store = tx.objectStore(STORES.THREAD)
        const request = store.put(entry, eventId)

        request.onsuccess = () => resolve()
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to cache thread:', error)
    }
  }

  /**
   * Get cached thread result
   */
  async getCachedThread(eventId: string): Promise<GraphResponse | null> {
    try {
      const db = await this.getDB()

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.THREAD, 'readonly')
        const store = tx.objectStore(STORES.THREAD)
        const request = store.get(eventId)

        request.onsuccess = () => {
          const entry = request.result as CachedEntry<GraphResponse> | undefined
          if (!entry) {
            resolve(null)
            return
          }

          if (Date.now() - entry.timestamp > CACHE_EXPIRY.THREAD) {
            resolve(null)
            return
          }

          resolve(entry.data)
        }
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to get cached thread:', error)
      return null
    }
  }

  /**
   * Cache relay graph capability
   */
  async cacheRelayCapability(
    url: string,
    capability: TGraphQueryCapability | null
  ): Promise<void> {
    try {
      const db = await this.getDB()
      const entry: CachedEntry<TGraphQueryCapability | null> = {
        data: capability,
        timestamp: Date.now()
      }

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.RELAY_CAPABILITIES, 'readwrite')
        const store = tx.objectStore(STORES.RELAY_CAPABILITIES)
        const request = store.put(entry, url)

        request.onsuccess = () => resolve()
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to cache relay capability:', error)
    }
  }

  /**
   * Get cached relay capability
   */
  async getCachedRelayCapability(
    url: string
  ): Promise<TGraphQueryCapability | null | undefined> {
    try {
      const db = await this.getDB()

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.RELAY_CAPABILITIES, 'readonly')
        const store = tx.objectStore(STORES.RELAY_CAPABILITIES)
        const request = store.get(url)

        request.onsuccess = () => {
          const entry = request.result as
            | CachedEntry<TGraphQueryCapability | null>
            | undefined
          if (!entry) {
            resolve(undefined) // Not in cache
            return
          }

          if (Date.now() - entry.timestamp > CACHE_EXPIRY.RELAY_CAPABILITY) {
            resolve(undefined) // Expired
            return
          }

          resolve(entry.data)
        }
        request.onerror = () => reject(request.error)
      })
    } catch (error) {
      console.error('Failed to get cached relay capability:', error)
      return undefined
    }
  }

  /**
   * Invalidate follow graph cache for a pubkey
   */
  async invalidateFollowGraph(pubkey: string): Promise<void> {
    try {
      const db = await this.getDB()

      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORES.FOLLOW_GRAPH, 'readwrite')
        const store = tx.objectStore(STORES.FOLLOW_GRAPH)

        // Delete entries for all depths
        for (let depth = 1; depth <= 16; depth++) {
          store.delete(`${pubkey}:${depth}`)
        }

        tx.oncomplete = () => resolve()
        tx.onerror = () => reject(tx.error)
      })
    } catch (error) {
      console.error('Failed to invalidate follow graph cache:', error)
    }
  }

  /**
   * Clear all caches
   */
  async clearAll(): Promise<void> {
    try {
      const db = await this.getDB()

      return new Promise((resolve, reject) => {
        const tx = db.transaction(
          [STORES.FOLLOW_GRAPH, STORES.THREAD, STORES.RELAY_CAPABILITIES],
          'readwrite'
        )

        tx.objectStore(STORES.FOLLOW_GRAPH).clear()
        tx.objectStore(STORES.THREAD).clear()
        tx.objectStore(STORES.RELAY_CAPABILITIES).clear()

        tx.oncomplete = () => resolve()
        tx.onerror = () => reject(tx.error)
      })
    } catch (error) {
      console.error('Failed to clear graph cache:', error)
    }
  }
}

const instance = GraphCacheService.getInstance()
export default instance
