import { Event, kinds } from 'nostr-tools'

/**
 * PropagationQueueService
 *
 * Manages a queue of events to republish to the user's relays.
 * This helps:
 * 1. Cache frequently-accessed profiles locally
 * 2. Spread data across the decentralized network
 * 3. Reduce latency for future fetches
 *
 * Uses IndexedDB for persistence so the queue survives page refreshes.
 */

const DB_NAME = 'smesh-propagation'
const DB_VERSION = 1
const QUEUE_STORE = 'propagation-queue'
const PROPAGATED_STORE = 'propagated-events'

interface PropagationJob {
  id: string
  event: Event
  targetRelays: string[]
  addedAt: number
  attempts: number
}

interface PropagatedEntry {
  id: string
  timestamp: number
}

class PropagationQueueService {
  private db: IDBDatabase | null = null
  private initPromise: Promise<void> | null = null
  private isProcessing = false
  private publishFn: ((event: Event, relays: string[]) => Promise<boolean>) | null = null

  // Kinds that should be propagated
  private propagatableKinds = [kinds.Metadata, kinds.RelayList]

  // How long to remember propagated events (24 hours)
  private propagationCooldownMs = 24 * 60 * 60 * 1000

  // Max retry attempts per job
  private maxAttempts = 3

  /**
   * Initialize the IndexedDB database
   */
  private init(): Promise<void> {
    if (this.initPromise) {
      return this.initPromise
    }

    this.initPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(DB_NAME, DB_VERSION)

      request.onerror = () => {
        console.error('Failed to open propagation queue DB:', request.error)
        reject(request.error)
      }

      request.onsuccess = () => {
        this.db = request.result
        resolve()
      }

      request.onupgradeneeded = () => {
        const db = request.result

        if (!db.objectStoreNames.contains(QUEUE_STORE)) {
          db.createObjectStore(QUEUE_STORE, { keyPath: 'id' })
        }

        if (!db.objectStoreNames.contains(PROPAGATED_STORE)) {
          const store = db.createObjectStore(PROPAGATED_STORE, { keyPath: 'id' })
          store.createIndex('timestamp', 'timestamp', { unique: false })
        }
      }
    })

    return this.initPromise
  }

  /**
   * Set the publish function to use for propagation
   * Must be called before queue processing can work
   */
  setPublishFunction(fn: (event: Event, relays: string[]) => Promise<boolean>): void {
    this.publishFn = fn
  }

  /**
   * Queue an event for propagation to the user's relays
   */
  async queueForPropagation(
    event: Event,
    targetRelays: string[],
    currentUserPubkey: string
  ): Promise<void> {
    // Skip if not a propagatable kind
    if (!this.propagatableKinds.includes(event.kind)) {
      return
    }

    // Skip own events (they're already on user's relays)
    if (event.pubkey === currentUserPubkey) {
      return
    }

    // Skip if no target relays
    if (targetRelays.length === 0) {
      return
    }

    await this.init()

    // Check if recently propagated
    const recentlyPropagated = await this.getPropagatedEntry(event.id)
    if (recentlyPropagated) {
      const age = Date.now() - recentlyPropagated.timestamp
      if (age < this.propagationCooldownMs) {
        return
      }
    }

    // Add to queue
    const job: PropagationJob = {
      id: event.id,
      event,
      targetRelays,
      addedAt: Date.now(),
      attempts: 0
    }

    await this.putJob(job)

    // Trigger processing (don't await - let it run in background)
    this.processQueue().catch(console.error)
  }

  /**
   * Process the propagation queue
   */
  async processQueue(): Promise<void> {
    if (this.isProcessing || !this.publishFn) {
      return
    }

    this.isProcessing = true

    try {
      await this.init()
      const jobs = await this.getAllJobs()

      for (const job of jobs) {
        try {
          // Try to publish
          const success = await this.publishFn(job.event, job.targetRelays)

          if (success) {
            // Mark as propagated and remove from queue
            await this.markAsPropagated(job.id)
            await this.deleteJob(job.id)
          } else {
            // Increment attempts
            job.attempts++
            if (job.attempts >= this.maxAttempts) {
              // Give up after max attempts
              await this.deleteJob(job.id)
            } else {
              await this.putJob(job)
            }
          }
        } catch (error) {
          console.warn('Propagation failed for event:', job.id, error)
          job.attempts++
          if (job.attempts >= this.maxAttempts) {
            await this.deleteJob(job.id)
          } else {
            await this.putJob(job)
          }
        }
      }

      // Clean up old propagated entries
      await this.cleanupOldEntries()
    } finally {
      this.isProcessing = false
    }
  }

  /**
   * Get queue size for debugging/monitoring
   */
  async getQueueSize(): Promise<number> {
    await this.init()
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return resolve(0)
      }

      const transaction = this.db.transaction(QUEUE_STORE, 'readonly')
      const store = transaction.objectStore(QUEUE_STORE)
      const request = store.count()

      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
  }

  // IndexedDB helpers

  private putJob(job: PropagationJob): Promise<void> {
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject(new Error('Database not initialized'))
      }

      const transaction = this.db.transaction(QUEUE_STORE, 'readwrite')
      const store = transaction.objectStore(QUEUE_STORE)
      const request = store.put(job)

      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  private deleteJob(id: string): Promise<void> {
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject(new Error('Database not initialized'))
      }

      const transaction = this.db.transaction(QUEUE_STORE, 'readwrite')
      const store = transaction.objectStore(QUEUE_STORE)
      const request = store.delete(id)

      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  private getAllJobs(): Promise<PropagationJob[]> {
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return resolve([])
      }

      const transaction = this.db.transaction(QUEUE_STORE, 'readonly')
      const store = transaction.objectStore(QUEUE_STORE)
      const request = store.getAll()

      request.onsuccess = () => resolve(request.result || [])
      request.onerror = () => reject(request.error)
    })
  }

  private getPropagatedEntry(id: string): Promise<PropagatedEntry | null> {
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return resolve(null)
      }

      const transaction = this.db.transaction(PROPAGATED_STORE, 'readonly')
      const store = transaction.objectStore(PROPAGATED_STORE)
      const request = store.get(id)

      request.onsuccess = () => resolve(request.result || null)
      request.onerror = () => reject(request.error)
    })
  }

  private markAsPropagated(id: string): Promise<void> {
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject(new Error('Database not initialized'))
      }

      const transaction = this.db.transaction(PROPAGATED_STORE, 'readwrite')
      const store = transaction.objectStore(PROPAGATED_STORE)
      const entry: PropagatedEntry = { id, timestamp: Date.now() }
      const request = store.put(entry)

      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  private async cleanupOldEntries(): Promise<void> {
    if (!this.db) {
      return
    }

    const weekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000

    return new Promise((resolve) => {
      const transaction = this.db!.transaction(PROPAGATED_STORE, 'readwrite')
      const store = transaction.objectStore(PROPAGATED_STORE)
      const index = store.index('timestamp')
      const range = IDBKeyRange.upperBound(weekAgo)
      const request = index.openCursor(range)

      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          cursor.delete()
          cursor.continue()
        } else {
          resolve()
        }
      }

      request.onerror = () => resolve()
    })
  }
}

const propagationQueueService = new PropagationQueueService()
export default propagationQueueService
