import { DomainEvent } from '../shared/events'
import { Pubkey } from '../shared/value-objects/Pubkey'
import { RelayUrl } from '../shared/value-objects/RelayUrl'
import { SignerType } from './SignerType'

// ============================================================================
// Profile Events
// ============================================================================

/**
 * Profile field values
 */
export interface ProfileFields {
  name?: string | null
  about?: string | null
  picture?: string | null
  banner?: string | null
  nip05?: string | null
  lud16?: string | null
  website?: string | null
}

/**
 * Changes to profile fields
 */
export interface ProfileChanges {
  name?: { from: string | null; to: string | null }
  about?: { from: string | null; to: string | null }
  picture?: { from: string | null; to: string | null }
  banner?: { from: string | null; to: string | null }
  nip05?: { from: string | null; to: string | null }
  lud16?: { from: string | null; to: string | null }
  website?: { from: string | null; to: string | null }
}

/**
 * Raised when a profile is published (new or update)
 */
export class ProfilePublished extends DomainEvent {
  get eventType(): string {
    return 'identity.profile_published'
  }

  constructor(
    readonly pubkey: Pubkey,
    readonly name: string | null,
    readonly about: string | null,
    readonly picture: string | null,
    readonly nip05: string | null
  ) {
    super()
  }
}

/**
 * Raised when specific profile fields are updated
 */
export class ProfileUpdated extends DomainEvent {
  get eventType(): string {
    return 'identity.profile_updated'
  }

  constructor(
    readonly pubkey: Pubkey,
    readonly changes: ProfileChanges
  ) {
    super()
  }

  get changedFields(): string[] {
    return Object.keys(this.changes).filter(
      (key) => this.changes[key as keyof ProfileChanges] !== undefined
    )
  }
}

// ============================================================================
// Relay List Events (User's NIP-65 mailbox relays)
// ============================================================================

/**
 * Raised when a user's relay list changes
 */
export class RelayListChanged extends DomainEvent {
  get eventType(): string {
    return 'identity.relay_list_changed'
  }

  constructor(
    readonly pubkey: Pubkey,
    readonly addedRelays: readonly RelayUrl[],
    readonly removedRelays: readonly RelayUrl[]
  ) {
    super()
  }

  get hasAdditions(): boolean {
    return this.addedRelays.length > 0
  }

  get hasRemovals(): boolean {
    return this.removedRelays.length > 0
  }
}

// ============================================================================
// Account Events
// ============================================================================

/**
 * Raised when a new account is created/added
 */
export class AccountCreated extends DomainEvent {
  get eventType(): string {
    return 'identity.account_created'
  }

  constructor(
    readonly pubkey: Pubkey,
    readonly signerType: SignerType
  ) {
    super()
  }
}

/**
 * Raised when switching between accounts
 */
export class AccountSwitched extends DomainEvent {
  get eventType(): string {
    return 'identity.account_switched'
  }

  constructor(
    readonly fromPubkey: Pubkey | null,
    readonly toPubkey: Pubkey | null
  ) {
    super()
  }

  get isLogin(): boolean {
    return this.fromPubkey === null && this.toPubkey !== null
  }

  get isLogout(): boolean {
    return this.fromPubkey !== null && this.toPubkey === null
  }
}

/**
 * Raised when an account is removed
 */
export class AccountRemoved extends DomainEvent {
  get eventType(): string {
    return 'identity.account_removed'
  }

  constructor(readonly pubkey: Pubkey) {
    super()
  }
}

/**
 * Raised when the signer type for an account changes
 */
export class SignerTypeChanged extends DomainEvent {
  get eventType(): string {
    return 'identity.signer_type_changed'
  }

  constructor(
    readonly pubkey: Pubkey,
    readonly fromType: SignerType,
    readonly toType: SignerType
  ) {
    super()
  }
}
