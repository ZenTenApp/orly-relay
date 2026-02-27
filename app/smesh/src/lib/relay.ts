/**
 * Infrastructure utilities for relay info checking.
 *
 * These are infrastructure-level helpers that check relay capabilities
 * and metadata. They don't contain domain logic and are appropriate
 * for use throughout the codebase.
 *
 * Note: For relay URL handling, use the domain RelayUrl value object:
 *   import { RelayUrl } from '@/domain'
 */

import { TRelayInfo } from '@/types'

export function checkAlgoRelay(relayInfo: TRelayInfo | undefined) {
  return relayInfo?.software === 'https://github.com/bitvora/algo-relay' // hardcode for now
}

export function checkSearchRelay(relayInfo: TRelayInfo | undefined) {
  return relayInfo?.supported_nips?.includes(50)
}

export function checkNip43Support(relayInfo: TRelayInfo | undefined) {
  return relayInfo?.supported_nips?.includes(43) && !!relayInfo.pubkey
}
