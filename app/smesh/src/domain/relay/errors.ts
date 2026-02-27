/**
 * Domain errors for Relay bounded context
 */

import { DomainError } from '../shared'

/**
 * Thrown when a relay set operation fails
 */
export class RelaySetOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Relay set operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when a relay list operation fails
 */
export class RelayListOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Relay list operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when attempting to add a duplicate relay
 */
export class DuplicateRelayError extends DomainError {
  constructor(url: string) {
    super(`Relay already exists: ${url}`)
  }
}

/**
 * Thrown when a relay is not found
 */
export class RelayNotFoundError extends DomainError {
  constructor(url: string) {
    super(`Relay not found: ${url}`)
  }
}
