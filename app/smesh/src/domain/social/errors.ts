/**
 * Domain errors for Social bounded context
 */

import { DomainError } from '../shared'

/**
 * Thrown when attempting to follow oneself
 */
export class CannotFollowSelfError extends DomainError {
  constructor() {
    super('Cannot follow yourself')
  }
}

/**
 * Thrown when attempting to mute oneself
 */
export class CannotMuteSelfError extends DomainError {
  constructor() {
    super('Cannot mute yourself')
  }
}

/**
 * Thrown when attempting to perform an operation that requires authentication
 */
export class NotAuthenticatedError extends DomainError {
  constructor() {
    super('Authentication required for this operation')
  }
}

/**
 * Thrown when a follow list operation fails
 */
export class FollowListOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Follow list operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when a mute list operation fails
 */
export class MuteListOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Mute list operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}
