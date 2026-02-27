import { Event } from 'nostr-tools'
import { TDraftEvent } from '@/types'

/**
 * Function to publish an event to relays
 * This is injected from the NostrProvider context
 */
export type PublishFn = (draftEvent: TDraftEvent) => Promise<Event>

/**
 * Dependencies for repository implementations
 */
export interface RepositoryDependencies {
  /**
   * Function to publish events to relays
   */
  publish: PublishFn
}
