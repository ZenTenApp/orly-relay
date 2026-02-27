import { Filter } from 'nostr-tools'
import { NRCSession, NRCSubscription } from './nrc-types'

// Default session timeout: 30 minutes
const DEFAULT_SESSION_TIMEOUT = 30 * 60 * 1000

// Default max subscriptions per session
const DEFAULT_MAX_SUBSCRIPTIONS = 100

/**
 * Generate a unique session ID
 */
function generateSessionId(): string {
  return crypto.randomUUID()
}

/**
 * Session manager for tracking NRC client sessions
 */
export class NRCSessionManager {
  private sessions: Map<string, NRCSession> = new Map()
  private sessionTimeout: number
  private maxSubscriptions: number
  private cleanupInterval: ReturnType<typeof setInterval> | null = null

  constructor(
    sessionTimeout: number = DEFAULT_SESSION_TIMEOUT,
    maxSubscriptions: number = DEFAULT_MAX_SUBSCRIPTIONS
  ) {
    this.sessionTimeout = sessionTimeout
    this.maxSubscriptions = maxSubscriptions
  }

  /**
   * Start the cleanup interval for expired sessions
   */
  start(): void {
    if (this.cleanupInterval) return

    // Run cleanup every 5 minutes
    this.cleanupInterval = setInterval(() => {
      this.cleanupExpiredSessions()
    }, 5 * 60 * 1000)
  }

  /**
   * Stop the cleanup interval
   */
  stop(): void {
    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval)
      this.cleanupInterval = null
    }
    this.sessions.clear()
  }

  /**
   * Get or create a session for a client
   */
  getOrCreateSession(
    clientPubkey: string,
    conversationKey: Uint8Array | undefined,
    deviceName?: string
  ): NRCSession {
    // Check if session exists for this client
    for (const session of this.sessions.values()) {
      if (session.clientPubkey === clientPubkey) {
        // Update last activity and return existing session
        session.lastActivity = Date.now()
        return session
      }
    }

    // Create new session
    const session: NRCSession = {
      id: generateSessionId(),
      clientPubkey,
      conversationKey,
      deviceName,
      createdAt: Date.now(),
      lastActivity: Date.now(),
      subscriptions: new Map()
    }

    this.sessions.set(session.id, session)
    return session
  }

  /**
   * Get a session by ID
   */
  getSession(sessionId: string): NRCSession | undefined {
    return this.sessions.get(sessionId)
  }

  /**
   * Get a session by client pubkey
   */
  getSessionByClientPubkey(clientPubkey: string): NRCSession | undefined {
    for (const session of this.sessions.values()) {
      if (session.clientPubkey === clientPubkey) {
        return session
      }
    }
    return undefined
  }

  /**
   * Touch a session to update last activity
   */
  touchSession(sessionId: string): void {
    const session = this.sessions.get(sessionId)
    if (session) {
      session.lastActivity = Date.now()
    }
  }

  /**
   * Add a subscription to a session
   */
  addSubscription(
    sessionId: string,
    subId: string,
    filters: Filter[]
  ): NRCSubscription | null {
    const session = this.sessions.get(sessionId)
    if (!session) return null

    // Check subscription limit
    if (session.subscriptions.size >= this.maxSubscriptions) {
      return null
    }

    const subscription: NRCSubscription = {
      id: subId,
      filters,
      createdAt: Date.now(),
      eventCount: 0,
      eoseSent: false
    }

    session.subscriptions.set(subId, subscription)
    session.lastActivity = Date.now()

    return subscription
  }

  /**
   * Get a subscription from a session
   */
  getSubscription(sessionId: string, subId: string): NRCSubscription | undefined {
    const session = this.sessions.get(sessionId)
    return session?.subscriptions.get(subId)
  }

  /**
   * Remove a subscription from a session
   */
  removeSubscription(sessionId: string, subId: string): boolean {
    const session = this.sessions.get(sessionId)
    if (!session) return false

    const deleted = session.subscriptions.delete(subId)
    if (deleted) {
      session.lastActivity = Date.now()
    }
    return deleted
  }

  /**
   * Mark EOSE sent for a subscription
   */
  markEOSE(sessionId: string, subId: string): void {
    const subscription = this.getSubscription(sessionId, subId)
    if (subscription) {
      subscription.eoseSent = true
    }
  }

  /**
   * Increment event count for a subscription
   */
  incrementEventCount(sessionId: string, subId: string): void {
    const subscription = this.getSubscription(sessionId, subId)
    if (subscription) {
      subscription.eventCount++
    }
  }

  /**
   * Remove a session
   */
  removeSession(sessionId: string): boolean {
    return this.sessions.delete(sessionId)
  }

  /**
   * Get the count of active sessions
   */
  getActiveSessionCount(): number {
    return this.sessions.size
  }

  /**
   * Get all active sessions
   */
  getAllSessions(): NRCSession[] {
    return Array.from(this.sessions.values())
  }

  /**
   * Clean up expired sessions
   */
  private cleanupExpiredSessions(): void {
    const now = Date.now()
    const expiredSessionIds: string[] = []

    for (const [sessionId, session] of this.sessions) {
      if (now - session.lastActivity > this.sessionTimeout) {
        expiredSessionIds.push(sessionId)
      }
    }

    for (const sessionId of expiredSessionIds) {
      this.sessions.delete(sessionId)
      console.log(`[NRC] Cleaned up expired session: ${sessionId}`)
    }
  }

  /**
   * Check if a session is expired
   */
  isSessionExpired(sessionId: string): boolean {
    const session = this.sessions.get(sessionId)
    if (!session) return true
    return Date.now() - session.lastActivity > this.sessionTimeout
  }
}
