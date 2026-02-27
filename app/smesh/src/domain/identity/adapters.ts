/**
 * Adapter functions for gradual migration from legacy code to Identity domain objects.
 */

import { Account } from './Account'
import { SignerType, SignerTypeValue } from './SignerType'

/**
 * Legacy TAccount type for reference
 */
type LegacyAccount = {
  pubkey: string
  signerType: SignerTypeValue
  ncryptsec?: string
  nsec?: string
  npub?: string
}

/**
 * Legacy TAccountPointer type for reference
 */
type LegacyAccountPointer = {
  pubkey: string
  signerType: SignerTypeValue
}

// ============================================================================
// Account Adapters
// ============================================================================

/**
 * Convert a legacy TAccount to an Account domain object
 */
export const toAccount = (legacy: LegacyAccount): Account | null => {
  return Account.fromLegacy(legacy)
}

/**
 * Convert an Account domain object to legacy TAccount format
 */
export const fromAccount = (account: Account): LegacyAccount => {
  return account.toLegacy()
}

/**
 * Convert multiple legacy accounts to domain objects
 */
export const toAccounts = (legacyAccounts: LegacyAccount[]): Account[] => {
  return legacyAccounts.map((a) => toAccount(a)).filter((a): a is Account => a !== null)
}

/**
 * Convert multiple domain accounts to legacy format
 */
export const fromAccounts = (accounts: Account[]): LegacyAccount[] => {
  return accounts.map((a) => fromAccount(a))
}

/**
 * Check if two account pointers are the same
 */
export const isSameAccount = (
  a: LegacyAccountPointer | null,
  b: LegacyAccountPointer | null
): boolean => {
  if (!a || !b) return false
  return a.pubkey === b.pubkey && a.signerType === b.signerType
}

/**
 * Check if two accounts have the same pubkey
 */
export const isSamePubkey = (
  a: LegacyAccountPointer | null,
  b: LegacyAccountPointer | null
): boolean => {
  if (!a || !b) return false
  return a.pubkey === b.pubkey
}

// ============================================================================
// SignerType Adapters
// ============================================================================

/**
 * Convert a string to a SignerType domain object
 */
export const toSignerType = (value: string): SignerType | null => {
  return SignerType.tryFromString(value)
}

/**
 * Convert a SignerType to its string value
 */
export const fromSignerType = (signerType: SignerType): SignerTypeValue => {
  return signerType.value
}

/**
 * Check if a signer type can sign events
 */
export const canSign = (signerType: string): boolean => {
  const type = SignerType.tryFromString(signerType)
  return type ? type.canSign : false
}

/**
 * Check if a signer type is view-only
 */
export const isViewOnly = (signerType: string): boolean => {
  const type = SignerType.tryFromString(signerType)
  return type ? type.isViewOnly : false
}

/**
 * Get the display name for a signer type
 */
export const getSignerTypeDisplayName = (signerType: string): string => {
  const type = SignerType.tryFromString(signerType)
  return type ? type.displayName : 'Unknown'
}

// ============================================================================
// Account List Helpers
// ============================================================================

/**
 * Find an account in a list by pubkey and signer type
 */
export const findAccount = (
  accounts: Account[],
  pubkey: string,
  signerType: string
): Account | undefined => {
  return accounts.find(
    (a) => a.pubkey.hex === pubkey && a.signerType.value === signerType
  )
}

/**
 * Find all accounts with a specific pubkey
 */
export const findAccountsByPubkey = (accounts: Account[], pubkey: string): Account[] => {
  return accounts.filter((a) => a.pubkey.hex === pubkey)
}

/**
 * Remove an account from a list
 */
export const removeAccount = (
  accounts: Account[],
  pubkey: string,
  signerType: string
): Account[] => {
  return accounts.filter(
    (a) => !(a.pubkey.hex === pubkey && a.signerType.value === signerType)
  )
}

/**
 * Add or update an account in a list
 */
export const upsertAccount = (accounts: Account[], account: Account): Account[] => {
  const existing = accounts.findIndex((a) => a.equals(account))
  if (existing >= 0) {
    const result = [...accounts]
    result[existing] = account
    return result
  }
  return [...accounts, account]
}
