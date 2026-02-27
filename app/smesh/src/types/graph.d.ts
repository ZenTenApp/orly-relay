// Re-export TGraphQueryCapability from index.d.ts
export type { TGraphQueryCapability } from './index'

// Graph query request structure (NIP-XX extension)
export interface GraphQuery {
  method: 'follows' | 'followers' | 'mentions' | 'thread'
  seed: string // 64-char hex pubkey or event ID
  depth?: number // 1-16, default 1
  inbound_refs?: RefSpec[]
  outbound_refs?: RefSpec[]
}

export interface RefSpec {
  kinds: number[]
  from_depth?: number
}

// Graph query response (from relay-signed event content)
export interface GraphResponse {
  pubkeys_by_depth?: string[][]
  events_by_depth?: string[][]
  total_pubkeys?: number
  total_events?: number
  inbound_refs?: RefSummary[]
  outbound_refs?: RefSummary[]
}

export interface RefSummary {
  kind: number
  target: string
  count: number
  refs?: string[]
}

// Graph query filter extension for nostr-tools
export interface GraphFilter {
  _graph: GraphQuery
}
