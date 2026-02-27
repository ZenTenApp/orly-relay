import { InvalidTimestampError } from '../errors'

/**
 * Value object representing a Unix timestamp (seconds since epoch).
 * Immutable, self-validating, with formatting utilities.
 */
export class Timestamp {
  private constructor(private readonly _unix: number) {}

  /**
   * Create a Timestamp for the current time.
   */
  static now(): Timestamp {
    return new Timestamp(Math.floor(Date.now() / 1000))
  }

  /**
   * Create a Timestamp from a Unix timestamp (seconds).
   * @throws InvalidTimestampError if the value is negative
   */
  static fromUnix(unix: number): Timestamp {
    if (unix < 0 || !Number.isFinite(unix)) {
      throw new InvalidTimestampError(unix)
    }
    return new Timestamp(Math.floor(unix))
  }

  /**
   * Create a Timestamp from a Date object.
   */
  static fromDate(date: Date): Timestamp {
    const unix = Math.floor(date.getTime() / 1000)
    if (unix < 0) {
      throw new InvalidTimestampError(unix)
    }
    return new Timestamp(unix)
  }

  /**
   * Create a Timestamp from milliseconds since epoch.
   */
  static fromMillis(millis: number): Timestamp {
    return Timestamp.fromUnix(millis / 1000)
  }

  /**
   * Try to create a Timestamp. Returns null if invalid.
   */
  static tryFromUnix(unix: number): Timestamp | null {
    try {
      return Timestamp.fromUnix(unix)
    } catch {
      return null
    }
  }

  /** The Unix timestamp in seconds */
  get unix(): number {
    return this._unix
  }

  /** The timestamp as milliseconds since epoch */
  get millis(): number {
    return this._unix * 1000
  }

  /** Convert to a Date object */
  get date(): Date {
    return new Date(this._unix * 1000)
  }

  /** Check if this timestamp is before another */
  isBefore(other: Timestamp): boolean {
    return this._unix < other._unix
  }

  /** Check if this timestamp is after another */
  isAfter(other: Timestamp): boolean {
    return this._unix > other._unix
  }

  /** Check if this timestamp is in the future */
  isFuture(): boolean {
    return this._unix > Timestamp.now()._unix
  }

  /** Check if this timestamp is in the past */
  isPast(): boolean {
    return this._unix < Timestamp.now()._unix
  }

  /** Get the difference in seconds from another timestamp */
  secondsFrom(other: Timestamp): number {
    return this._unix - other._unix
  }

  /** Create a new Timestamp by adding seconds */
  addSeconds(seconds: number): Timestamp {
    return new Timestamp(this._unix + seconds)
  }

  /** Create a new Timestamp by adding minutes */
  addMinutes(minutes: number): Timestamp {
    return this.addSeconds(minutes * 60)
  }

  /** Create a new Timestamp by adding hours */
  addHours(hours: number): Timestamp {
    return this.addSeconds(hours * 3600)
  }

  /** Create a new Timestamp by adding days */
  addDays(days: number): Timestamp {
    return this.addSeconds(days * 86400)
  }

  /**
   * Format as a relative time string (e.g., "5m", "2h", "3d").
   */
  formatRelative(): string {
    const now = Timestamp.now()
    const diff = now._unix - this._unix

    if (diff < 0) {
      return 'in the future'
    }
    if (diff < 60) {
      return 'just now'
    }
    if (diff < 3600) {
      return `${Math.floor(diff / 60)}m`
    }
    if (diff < 86400) {
      return `${Math.floor(diff / 3600)}h`
    }
    if (diff < 604800) {
      return `${Math.floor(diff / 86400)}d`
    }
    if (diff < 2592000) {
      return `${Math.floor(diff / 604800)}w`
    }
    if (diff < 31536000) {
      return `${Math.floor(diff / 2592000)}mo`
    }
    return `${Math.floor(diff / 31536000)}y`
  }

  /**
   * Format as ISO 8601 string.
   */
  toISOString(): string {
    return this.date.toISOString()
  }

  /**
   * Format as locale date string.
   */
  toLocaleDateString(locales?: string | string[], options?: Intl.DateTimeFormatOptions): string {
    return this.date.toLocaleDateString(locales, options)
  }

  /**
   * Format as locale time string.
   */
  toLocaleTimeString(locales?: string | string[], options?: Intl.DateTimeFormatOptions): string {
    return this.date.toLocaleTimeString(locales, options)
  }

  /** Check equality with another Timestamp */
  equals(other: Timestamp): boolean {
    return this._unix === other._unix
  }

  /** Returns the Unix timestamp as a number */
  valueOf(): number {
    return this._unix
  }

  /** Returns the Unix timestamp as a string */
  toString(): string {
    return String(this._unix)
  }

  /** For JSON serialization */
  toJSON(): number {
    return this._unix
  }
}
