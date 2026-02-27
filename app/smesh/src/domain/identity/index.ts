/**
 * Identity Bounded Context
 *
 * Handles accounts, authentication, and signing.
 */

// Entities
export { Account } from './Account'
export type { AccountCredentials } from './Account'

// Value Objects
export { SignerType } from './SignerType'
export type { SignerTypeValue } from './SignerType'

// Errors
export {
  InvalidCredentialsError,
  AccountOperationError,
  NotLoggedInError,
  SignerError,
  AccountNotFoundError
} from './errors'

// Domain Events
export {
  ProfilePublished,
  ProfileUpdated,
  RelayListChanged,
  AccountCreated,
  AccountSwitched,
  AccountRemoved,
  SignerTypeChanged,
  type ProfileFields,
  type ProfileChanges
} from './events'

// Adapters for migration
export {
  // Account adapters
  toAccount,
  fromAccount,
  toAccounts,
  fromAccounts,
  isSameAccount,
  isSamePubkey,
  // SignerType adapters
  toSignerType,
  fromSignerType,
  canSign,
  isViewOnly,
  getSignerTypeDisplayName,
  // Account list helpers
  findAccount,
  findAccountsByPubkey,
  removeAccount,
  upsertAccount
} from './adapters'
