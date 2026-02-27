import { useEffect } from 'react'
import {
  registerSocialEventHandlers,
  unregisterSocialEventHandlers,
  clearSocialHandlerCallbacks
} from '@/application/handlers/SocialEventHandlers'
import {
  registerContentEventHandlers,
  unregisterContentEventHandlers,
  clearContentHandlerCallbacks
} from '@/application/handlers/ContentEventHandlers'
import {
  registerFeedEventHandlers,
  unregisterFeedEventHandlers
} from '@/application/handlers/FeedEventHandlers'
import {
  registerRelayEventHandlers,
  unregisterRelayEventHandlers
} from '@/application/handlers/RelayEventHandlers'

/**
 * EventHandlerProvider
 *
 * Initializes domain event handlers when the app starts.
 * This provider should be placed near the root of the component tree.
 *
 * Handlers are organized by domain context:
 * - Social: User follow/mute events
 * - Content: Bookmarks, pins, reactions, reposts
 * - Feed: Timeline, notes, content filtering
 * - Relay: Relay sets, favorites, mailbox configuration
 */
export function EventHandlerProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    // Register all event handlers on mount
    registerSocialEventHandlers()
    registerContentEventHandlers()
    registerFeedEventHandlers()
    registerRelayEventHandlers()

    console.debug('[EventHandlerProvider] Domain event handlers registered')

    // Cleanup on unmount
    return () => {
      unregisterSocialEventHandlers()
      unregisterContentEventHandlers()
      unregisterFeedEventHandlers()
      unregisterRelayEventHandlers()

      // Clear callback registrations
      clearSocialHandlerCallbacks()
      clearContentHandlerCallbacks()

      console.debug('[EventHandlerProvider] Domain event handlers unregistered')
    }
  }, [])

  return <>{children}</>
}
