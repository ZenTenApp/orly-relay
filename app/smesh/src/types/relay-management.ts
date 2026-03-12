export type TRelayApprovalStatus = 'pending' | 'approved' | 'rejected'
export type TRelayDirection = 'outbox' | 'inbox' | 'both'
export type TOutboxMode = 'automatic' | 'managed'

/** Per-network stats for a single relay, keyed by the client's external IP. */
export interface TNetworkRelayStats {
  publishSuccess: number
  publishFailure: number
  fetchSuccess: number
  fetchFailure: number
  lastSeen: number
}

/** Full relay entry with per-network stats and approval state. */
export interface TRelayEntry {
  url: string
  direction: TRelayDirection
  /** Relay resolved IPv4 as dotted-quad string (e.g. "1.2.3.4"), or null if unresolved */
  relayIp: string | null
  /** Per-network stats: key = client external IP hex (8 hex chars for IPv4) */
  networkStats: Map<string, TNetworkRelayStats>
  /** User manually excluded (discretionary, independent of stats or approval) */
  manualExclude: boolean
  /** Approval status in managed mode */
  status: TRelayApprovalStatus
  /** Why this relay was discovered (e.g. "write relay for npub1abc...") */
  reason: string
  addedAt: number
  updatedAt: number
}

/**
 * Binary sync format for relay stats data.
 *
 * Per-entry layout:
 *   [1 byte]  URL length (max 255)
 *   [N bytes] URL (UTF-8)
 *   [1 byte]  flags: direction(2) | status(2) | manualExclude(1) | reserved(3)
 *   [4 bytes] relay resolved IPv4 (0.0.0.0 if unresolved)
 *   [1 byte]  network stats count (K)
 *   K times (12 bytes each):
 *     [4 bytes] client external IPv4
 *     [2 bytes] publishSuccess (uint16, capped 65535)
 *     [2 bytes] publishFailure (uint16)
 *     [2 bytes] fetchSuccess (uint16)
 *     [2 bytes] fetchFailure (uint16)
 */
export const DIRECTION_BITS: Record<TRelayDirection, number> = {
  outbox: 0b00,
  inbox: 0b01,
  both: 0b10
}

export const STATUS_BITS: Record<TRelayApprovalStatus, number> = {
  pending: 0b00,
  approved: 0b01,
  rejected: 0b10
}

export const DIRECTION_FROM_BITS: TRelayDirection[] = ['outbox', 'inbox', 'both', 'outbox']
export const STATUS_FROM_BITS: TRelayApprovalStatus[] = ['pending', 'approved', 'rejected', 'pending']

/** Auto-disable threshold: 99% failure rate */
export const AUTO_DISABLE_FAILURE_RATE = 0.99
/** Minimum attempts before auto-disable can trigger */
export const AUTO_DISABLE_MIN_ATTEMPTS = 10
