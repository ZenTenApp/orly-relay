/**
 * Domain errors for value object validation
 */

export class DomainError extends Error {
  constructor(message: string) {
    super(message)
    this.name = this.constructor.name
  }
}

export class InvalidPubkeyError extends DomainError {
  constructor(value: string) {
    super(`Invalid pubkey: "${value.slice(0, 20)}${value.length > 20 ? '...' : ''}"`)
  }
}

export class InvalidRelayUrlError extends DomainError {
  constructor(value: string) {
    super(`Invalid relay URL: "${value}"`)
  }
}

export class InvalidEventIdError extends DomainError {
  constructor(value: string) {
    super(`Invalid event ID: "${value.slice(0, 20)}${value.length > 20 ? '...' : ''}"`)
  }
}

export class InvalidTimestampError extends DomainError {
  constructor(value: number) {
    super(`Invalid timestamp: ${value}`)
  }
}
