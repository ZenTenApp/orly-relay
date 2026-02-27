/**
 * Domain errors for Identity bounded context
 */

import { DomainError } from '../shared'

/**
 * Thrown when login credentials are invalid
 */
export class InvalidCredentialsError extends DomainError {
  constructor(reason?: string) {
    super(`Invalid credentials${reason ? `: ${reason}` : ''}`)
  }
}

/**
 * Thrown when an account operation fails
 */
export class AccountOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Account operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when attempting to sign without authentication
 */
export class NotLoggedInError extends DomainError {
  constructor() {
    super('Not logged in - authentication required')
  }
}

/**
 * Thrown when a signer operation fails
 */
export class SignerError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Signer error: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when an account is not found
 */
export class AccountNotFoundError extends DomainError {
  constructor(identifier?: string) {
    super(`Account not found${identifier ? `: ${identifier}` : ''}`)
  }
}
