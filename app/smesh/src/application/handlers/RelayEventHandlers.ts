import {
  FavoriteRelayAdded,
  FavoriteRelayRemoved,
  FavoriteRelaysPublished,
  RelaySetCreated,
  RelaySetUpdated,
  RelaySetDeleted,
  MailboxRelayAdded,
  MailboxRelayRemoved,
  MailboxRelayScopeChanged,
  RelayListPublished
} from '@/domain/relay/events'
import { EventHandler, eventDispatcher } from '@/domain/shared'

/**
 * Handlers for Relay domain events
 *
 * These handlers coordinate cross-context updates when relay configuration changes.
 * They enable coordination between Relay, Feed, and Identity contexts.
 */

/**
 * Handler for favorite relay added events
 * Can be used to:
 * - Update relay picker UI
 * - Add relay to connection pool
 */
export const handleFavoriteRelayAdded: EventHandler<FavoriteRelayAdded> = async (event) => {
  console.debug('[RelayEventHandler] Favorite relay added:', {
    owner: event.owner.formatted,
    relay: event.relayUrl.value
  })

  // Future: Update relay picker options
  // Future: Pre-connect to new favorite relay
}

/**
 * Handler for favorite relay removed events
 * Can be used to:
 * - Update relay picker UI
 * - Close connection if no longer needed
 */
export const handleFavoriteRelayRemoved: EventHandler<FavoriteRelayRemoved> = async (event) => {
  console.debug('[RelayEventHandler] Favorite relay removed:', {
    owner: event.owner.formatted,
    relay: event.relayUrl.value
  })

  // Future: Update relay picker options
}

/**
 * Handler for favorite relays published events
 * Can be used to:
 * - Invalidate relay preference caches
 * - Sync with remote state
 */
export const handleFavoriteRelaysPublished: EventHandler<FavoriteRelaysPublished> = async (event) => {
  console.debug('[RelayEventHandler] Favorite relays published:', {
    owner: event.owner.formatted,
    relayCount: event.relayCount
  })

  // Future: Invalidate caches
}

/**
 * Handler for relay set created events
 * Can be used to:
 * - Update feed type options in UI
 * - Add new relay set to navigation
 */
export const handleRelaySetCreated: EventHandler<RelaySetCreated> = async (event) => {
  console.debug('[RelayEventHandler] Relay set created:', {
    owner: event.owner.formatted,
    setId: event.setId,
    name: event.name,
    relayCount: event.relays.length
  })

  // Future: Update feed selector options
  // Future: Add to relay set navigation
}

/**
 * Handler for relay set updated events
 * Can be used to:
 * - Refresh active feed if using this relay set
 * - Update relay set display
 */
export const handleRelaySetUpdated: EventHandler<RelaySetUpdated> = async (event) => {
  console.debug('[RelayEventHandler] Relay set updated:', {
    owner: event.owner.formatted,
    setId: event.setId,
    nameChanged: event.nameChanged,
    changes: {
      addedCount: event.changes.addedRelays?.length ?? 0,
      removedCount: event.changes.removedRelays?.length ?? 0
    }
  })

  // Future: Refresh feed if currently using this relay set
  // Future: Update relay set display
}

/**
 * Handler for relay set deleted events
 * Can be used to:
 * - Switch to different feed if current feed uses deleted set
 * - Remove from navigation
 */
export const handleRelaySetDeleted: EventHandler<RelaySetDeleted> = async (event) => {
  console.debug('[RelayEventHandler] Relay set deleted:', {
    owner: event.owner.formatted,
    setId: event.setId
  })

  // Future: Switch feed if currently using this relay set
  // Future: Remove from feed selector options
}

/**
 * Handler for mailbox relay added events
 * Can be used to:
 * - Update relay list display
 * - Connect to new mailbox relay
 */
export const handleMailboxRelayAdded: EventHandler<MailboxRelayAdded> = async (event) => {
  console.debug('[RelayEventHandler] Mailbox relay added:', {
    owner: event.owner.formatted,
    relay: event.relayUrl.value,
    scope: event.scope
  })

  // Future: Update relay list in settings
  // Future: Connect to relay based on scope
}

/**
 * Handler for mailbox relay removed events
 * Can be used to:
 * - Update relay list display
 * - Disconnect if no longer needed
 */
export const handleMailboxRelayRemoved: EventHandler<MailboxRelayRemoved> = async (event) => {
  console.debug('[RelayEventHandler] Mailbox relay removed:', {
    owner: event.owner.formatted,
    relay: event.relayUrl.value
  })

  // Future: Update relay list in settings
}

/**
 * Handler for mailbox relay scope changed events
 * Can be used to:
 * - Update relay list display
 * - Adjust connection strategy
 */
export const handleMailboxRelayScopeChanged: EventHandler<MailboxRelayScopeChanged> = async (event) => {
  console.debug('[RelayEventHandler] Mailbox relay scope changed:', {
    owner: event.owner.formatted,
    relay: event.relayUrl.value,
    fromScope: event.fromScope,
    toScope: event.toScope
  })

  // Future: Update relay list in settings
  // Future: Adjust write/read connection strategy
}

/**
 * Handler for relay list published events
 * Can be used to:
 * - Invalidate relay caches
 * - Trigger feed refresh if relay configuration changed
 */
export const handleRelayListPublished: EventHandler<RelayListPublished> = async (event) => {
  console.debug('[RelayEventHandler] Relay list published:', {
    owner: event.owner.formatted,
    readRelayCount: event.readRelayCount,
    writeRelayCount: event.writeRelayCount
  })

  // Future: Invalidate relay caches
  // Future: Trigger feed refresh if needed
}

/**
 * Register all relay event handlers with the event dispatcher
 */
export function registerRelayEventHandlers(): void {
  eventDispatcher.on('relay.favorite_added', handleFavoriteRelayAdded)
  eventDispatcher.on('relay.favorite_removed', handleFavoriteRelayRemoved)
  eventDispatcher.on('relay.favorites_published', handleFavoriteRelaysPublished)
  eventDispatcher.on('relay.set_created', handleRelaySetCreated)
  eventDispatcher.on('relay.set_updated', handleRelaySetUpdated)
  eventDispatcher.on('relay.set_deleted', handleRelaySetDeleted)
  eventDispatcher.on('relay.mailbox_added', handleMailboxRelayAdded)
  eventDispatcher.on('relay.mailbox_removed', handleMailboxRelayRemoved)
  eventDispatcher.on('relay.mailbox_scope_changed', handleMailboxRelayScopeChanged)
  eventDispatcher.on('relay.list_published', handleRelayListPublished)
}

/**
 * Unregister all relay event handlers
 */
export function unregisterRelayEventHandlers(): void {
  eventDispatcher.off('relay.favorite_added', handleFavoriteRelayAdded)
  eventDispatcher.off('relay.favorite_removed', handleFavoriteRelayRemoved)
  eventDispatcher.off('relay.favorites_published', handleFavoriteRelaysPublished)
  eventDispatcher.off('relay.set_created', handleRelaySetCreated)
  eventDispatcher.off('relay.set_updated', handleRelaySetUpdated)
  eventDispatcher.off('relay.set_deleted', handleRelaySetDeleted)
  eventDispatcher.off('relay.mailbox_added', handleMailboxRelayAdded)
  eventDispatcher.off('relay.mailbox_removed', handleMailboxRelayRemoved)
  eventDispatcher.off('relay.mailbox_scope_changed', handleMailboxRelayScopeChanged)
  eventDispatcher.off('relay.list_published', handleRelayListPublished)
}
