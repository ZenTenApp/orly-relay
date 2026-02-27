import { DomainEvent } from '../shared/events'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'

// ============================================================================
// Favorite Relay Events
// ============================================================================

/**
 * Raised when a favorite relay is added
 */
export class FavoriteRelayAdded extends DomainEvent {
  get eventType(): string {
    return 'relay.favorite_added'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayUrl: RelayUrl
  ) {
    super()
  }
}

/**
 * Raised when a favorite relay is removed
 */
export class FavoriteRelayRemoved extends DomainEvent {
  get eventType(): string {
    return 'relay.favorite_removed'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayUrl: RelayUrl
  ) {
    super()
  }
}

/**
 * Raised when favorite relays list is published
 */
export class FavoriteRelaysPublished extends DomainEvent {
  get eventType(): string {
    return 'relay.favorites_published'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayCount: number,
    readonly setCount: number
  ) {
    super()
  }
}

// ============================================================================
// Relay Set Events
// ============================================================================

/**
 * Raised when a new relay set is created
 */
export class RelaySetCreated extends DomainEvent {
  get eventType(): string {
    return 'relay.set_created'
  }

  constructor(
    readonly owner: Pubkey,
    readonly setId: string,
    readonly name: string,
    readonly relays: readonly RelayUrl[]
  ) {
    super()
  }
}

/**
 * Changes that can be made to a relay set
 */
export interface RelaySetChanges {
  name?: { from: string; to: string }
  addedRelays?: RelayUrl[]
  removedRelays?: RelayUrl[]
}

/**
 * Raised when a relay set is updated
 */
export class RelaySetUpdated extends DomainEvent {
  get eventType(): string {
    return 'relay.set_updated'
  }

  constructor(
    readonly owner: Pubkey,
    readonly setId: string,
    readonly changes: RelaySetChanges
  ) {
    super()
  }

  get nameChanged(): boolean {
    return this.changes.name !== undefined
  }

  get relaysChanged(): boolean {
    return (
      (this.changes.addedRelays?.length ?? 0) > 0 ||
      (this.changes.removedRelays?.length ?? 0) > 0
    )
  }
}

/**
 * Raised when a relay set is deleted
 */
export class RelaySetDeleted extends DomainEvent {
  get eventType(): string {
    return 'relay.set_deleted'
  }

  constructor(
    readonly owner: Pubkey,
    readonly setId: string
  ) {
    super()
  }
}

// ============================================================================
// Mailbox Relay (NIP-65) Events
// ============================================================================

/**
 * Raised when a relay is added to the user's relay list
 */
export class MailboxRelayAdded extends DomainEvent {
  get eventType(): string {
    return 'relay.mailbox_added'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayUrl: RelayUrl,
    readonly scope: 'read' | 'write' | 'both'
  ) {
    super()
  }
}

/**
 * Raised when a relay is removed from the user's relay list
 */
export class MailboxRelayRemoved extends DomainEvent {
  get eventType(): string {
    return 'relay.mailbox_removed'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayUrl: RelayUrl
  ) {
    super()
  }
}

/**
 * Raised when a relay's scope is changed
 */
export class MailboxRelayScopeChanged extends DomainEvent {
  get eventType(): string {
    return 'relay.mailbox_scope_changed'
  }

  constructor(
    readonly owner: Pubkey,
    readonly relayUrl: RelayUrl,
    readonly fromScope: 'read' | 'write' | 'both',
    readonly toScope: 'read' | 'write' | 'both'
  ) {
    super()
  }
}

/**
 * Raised when the relay list is published
 */
export class RelayListPublished extends DomainEvent {
  get eventType(): string {
    return 'relay.list_published'
  }

  constructor(
    readonly owner: Pubkey,
    readonly readRelayCount: number,
    readonly writeRelayCount: number
  ) {
    super()
  }
}
