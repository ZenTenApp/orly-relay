/**
 * Domain errors for Content bounded context
 */

import { DomainError } from '../shared'

/**
 * Thrown when content validation fails
 */
export class InvalidContentError extends DomainError {
  constructor(reason: string) {
    super(`Invalid content: ${reason}`)
  }
}

/**
 * Thrown when a note operation fails
 */
export class NoteOperationError extends DomainError {
  constructor(operation: string, reason?: string) {
    super(`Note operation failed: ${operation}${reason ? ` - ${reason}` : ''}`)
  }
}

/**
 * Thrown when attempting to react to own content
 */
export class CannotReactToOwnContentError extends DomainError {
  constructor() {
    super('Cannot react to your own content')
  }
}

/**
 * Thrown when a note is not found
 */
export class NoteNotFoundError extends DomainError {
  constructor(id: string) {
    super(`Note not found: ${id}`)
  }
}

/**
 * Thrown when content exceeds size limits
 */
export class ContentTooLargeError extends DomainError {
  constructor(maxSize: number) {
    super(`Content exceeds maximum size of ${maxSize} characters`)
  }
}
