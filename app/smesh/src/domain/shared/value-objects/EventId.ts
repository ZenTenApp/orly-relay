import { nip19 } from 'nostr-tools'
import { InvalidEventIdError } from '../errors'
import { Pubkey } from './Pubkey'
import { RelayUrl } from './RelayUrl'

/**
 * Value object representing a Nostr event ID.
 * Can be created from hex or bech32 (note1/nevent1) formats.
 * Optionally includes relay hints and author information.
 */
export class EventId {
  private constructor(
    private readonly _hex: string,
    private readonly _kind?: number,
    private readonly _author?: Pubkey,
    private readonly _relayHints: RelayUrl[] = []
  ) {}

  /**
   * Create an EventId from a 64-character hex string.
   * @throws InvalidEventIdError if the hex is invalid
   */
  static fromHex(hex: string): EventId {
    if (!/^[0-9a-f]{64}$/.test(hex)) {
      throw new InvalidEventIdError(hex)
    }
    return new EventId(hex)
  }

  /**
   * Create an EventId from a bech32 string (note1 or nevent1).
   * @throws InvalidEventIdError if the bech32 is invalid
   */
  static fromBech32(bech32: string): EventId {
    try {
      const { type, data } = nip19.decode(bech32)

      switch (type) {
        case 'note':
          return new EventId(data)

        case 'nevent': {
          const relayHints = (data.relays || [])
            .map((r) => RelayUrl.tryCreate(r))
            .filter((r): r is RelayUrl => r !== null)

          return new EventId(
            data.id,
            data.kind,
            data.author ? Pubkey.tryFromString(data.author) || undefined : undefined,
            relayHints
          )
        }

        default:
          throw new InvalidEventIdError(bech32)
      }
    } catch (e) {
      if (e instanceof InvalidEventIdError) throw e
      throw new InvalidEventIdError(bech32)
    }
  }

  /**
   * Try to create an EventId from any string format (hex, note1, nevent1).
   * Returns null if the string is invalid.
   */
  static tryFromString(input: string): EventId | null {
    try {
      if (input.startsWith('note1') || input.startsWith('nevent1')) {
        return EventId.fromBech32(input)
      }
      return EventId.fromHex(input)
    } catch {
      return null
    }
  }

  /**
   * Check if a string is a valid event ID (hex format only).
   */
  static isValidHex(value: string): boolean {
    return /^[0-9a-f]{64}$/.test(value)
  }

  /** The raw 64-character hex value */
  get hex(): string {
    return this._hex
  }

  /** The event kind if known (from nevent) */
  get kind(): number | undefined {
    return this._kind
  }

  /** The event author if known (from nevent) */
  get author(): Pubkey | undefined {
    return this._author
  }

  /** Relay hints for finding this event */
  get relayHints(): readonly RelayUrl[] {
    return this._relayHints
  }

  /** A shortened display format */
  get formatted(): string {
    return `${this._hex.slice(0, 8)}...${this._hex.slice(-4)}`
  }

  /**
   * Convert to bech32 format.
   * Returns nevent1 if there's additional metadata, otherwise note1.
   */
  toBech32(): string {
    if (this._kind !== undefined || this._author || this._relayHints.length > 0) {
      return nip19.neventEncode({
        id: this._hex,
        kind: this._kind,
        author: this._author?.hex,
        relays: this._relayHints.map((r) => r.value),
      })
    }
    return nip19.noteEncode(this._hex)
  }

  /** Convert to simple note1 format (no metadata) */
  toNote(): string {
    return nip19.noteEncode(this._hex)
  }

  /** Check equality with another EventId (compares hex only) */
  equals(other: EventId): boolean {
    return this._hex === other._hex
  }

  /** Returns the hex representation */
  toString(): string {
    return this._hex
  }

  /** For JSON serialization */
  toJSON(): string {
    return this._hex
  }
}
