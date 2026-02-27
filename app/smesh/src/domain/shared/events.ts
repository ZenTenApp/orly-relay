import { Timestamp } from './value-objects'

/**
 * Base class for all domain events
 *
 * Domain events capture something that happened in the domain that domain experts
 * care about. They are named in past tense (e.g., UserFollowed, NotePinned).
 */
export abstract class DomainEvent {
  readonly occurredAt: Timestamp

  constructor() {
    this.occurredAt = Timestamp.now()
  }

  /**
   * Unique identifier for the event type
   * Format: context.event_name (e.g., 'social.user_followed')
   */
  abstract get eventType(): string
}

/**
 * Handler for domain events
 */
export type EventHandler<T extends DomainEvent = DomainEvent> = (event: T) => void | Promise<void>

/**
 * Event dispatcher interface
 */
export interface EventDispatcher {
  /**
   * Dispatch an event to all registered handlers
   */
  dispatch(event: DomainEvent): Promise<void>

  /**
   * Register a handler for a specific event type
   */
  on<T extends DomainEvent>(eventType: string, handler: EventHandler<T>): void

  /**
   * Remove a handler for a specific event type
   */
  off<T extends DomainEvent>(eventType: string, handler: EventHandler<T>): void

  /**
   * Register a handler for all events
   */
  onAll(handler: EventHandler): void

  /**
   * Remove a handler for all events
   */
  offAll(handler: EventHandler): void
}

/**
 * Simple in-memory event dispatcher
 *
 * Dispatches events synchronously to handlers. Handlers are called in registration order.
 * Errors in handlers are logged but don't prevent other handlers from being called.
 */
export class SimpleEventDispatcher implements EventDispatcher {
  private handlers: Map<string, Set<EventHandler>> = new Map()
  private allHandlers: Set<EventHandler> = new Set()

  async dispatch(event: DomainEvent): Promise<void> {
    const eventType = event.eventType

    // Call type-specific handlers
    const typeHandlers = this.handlers.get(eventType)
    if (typeHandlers) {
      for (const handler of typeHandlers) {
        try {
          await handler(event)
        } catch (error) {
          console.error(`Error in event handler for ${eventType}:`, error)
        }
      }
    }

    // Call all-event handlers
    for (const handler of this.allHandlers) {
      try {
        await handler(event)
      } catch (error) {
        console.error(`Error in all-event handler for ${eventType}:`, error)
      }
    }
  }

  on<T extends DomainEvent>(eventType: string, handler: EventHandler<T>): void {
    let handlers = this.handlers.get(eventType)
    if (!handlers) {
      handlers = new Set()
      this.handlers.set(eventType, handlers)
    }
    handlers.add(handler as EventHandler)
  }

  off<T extends DomainEvent>(eventType: string, handler: EventHandler<T>): void {
    const handlers = this.handlers.get(eventType)
    if (handlers) {
      handlers.delete(handler as EventHandler)
      if (handlers.size === 0) {
        this.handlers.delete(eventType)
      }
    }
  }

  onAll(handler: EventHandler): void {
    this.allHandlers.add(handler)
  }

  offAll(handler: EventHandler): void {
    this.allHandlers.delete(handler)
  }

  /**
   * Clear all handlers (useful for testing)
   */
  clear(): void {
    this.handlers.clear()
    this.allHandlers.clear()
  }
}

/**
 * Global event dispatcher instance
 *
 * This is a singleton that can be used throughout the application.
 * For testing, you can create a new SimpleEventDispatcher instance.
 */
export const eventDispatcher = new SimpleEventDispatcher()
