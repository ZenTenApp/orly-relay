/**
 * Domain Layer
 *
 * Core domain logic with no external dependencies.
 * Organized into bounded contexts.
 */

// Shared Kernel - Common value objects and utilities
export * from './shared'

// Social Bounded Context - Following, muting, social graph
export * from './social'

// Relay Bounded Context - Relay management and preferences
export * from './relay'

// Identity Bounded Context - Accounts and authentication
export * from './identity'

// Content Bounded Context - Notes, reactions, reposts
export * from './content'
