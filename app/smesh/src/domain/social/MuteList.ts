import { Event, kinds } from 'nostr-tools'
import { Pubkey, Timestamp } from '../shared'
import { CannotMuteSelfError } from './errors'

/**
 * Type of mute visibility
 */
export type MuteVisibility = 'public' | 'private'

/**
 * A muted entry (user or other content)
 */
export type MuteEntry = {
  pubkey: Pubkey
  visibility: MuteVisibility
}

/**
 * Result of a mute/unmute operation
 */
export type MuteListChange =
  | { type: 'muted'; pubkey: Pubkey; visibility: MuteVisibility }
  | { type: 'unmuted'; pubkey: Pubkey }
  | { type: 'visibility_changed'; pubkey: Pubkey; from: MuteVisibility; to: MuteVisibility }
  | { type: 'no_change' }

/**
 * MuteList Aggregate
 *
 * Represents a user's mute list (kind 10000 event in Nostr).
 * Supports both public mutes (visible to others) and private mutes (encrypted).
 *
 * Invariants:
 * - Cannot mute self
 * - No duplicate entries (a user can only be muted once, either public or private)
 * - Pubkeys must be valid
 *
 * Note: The encryption/decryption of private mutes is handled at the infrastructure layer.
 * This aggregate works with already-decrypted private tags.
 */
export class MuteList {
  private readonly _publicMutes: Map<string, Pubkey>
  private readonly _privateMutes: Map<string, Pubkey>

  private constructor(
    private readonly _owner: Pubkey,
    publicMutes: Pubkey[],
    privateMutes: Pubkey[]
  ) {
    this._publicMutes = new Map()
    this._privateMutes = new Map()

    for (const pubkey of publicMutes) {
      this._publicMutes.set(pubkey.hex, pubkey)
    }
    for (const pubkey of privateMutes) {
      this._privateMutes.set(pubkey.hex, pubkey)
    }
  }

  /**
   * Create an empty MuteList for a user
   */
  static empty(owner: Pubkey): MuteList {
    return new MuteList(owner, [], [])
  }

  /**
   * Reconstruct a MuteList from a Nostr kind 10000 event
   *
   * @param event The mute list event
   * @param decryptedPrivateTags The decrypted private tags (if any)
   */
  static fromEvent(event: Event, decryptedPrivateTags: string[][] = []): MuteList {
    if (event.kind !== kinds.Mutelist) {
      throw new Error(`Expected kind ${kinds.Mutelist}, got ${event.kind}`)
    }

    const owner = Pubkey.fromHex(event.pubkey)
    const publicMutes: Pubkey[] = []
    const privateMutes: Pubkey[] = []

    // Extract public mutes from event tags
    for (const tag of event.tags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          publicMutes.push(pubkey)
        }
      }
    }

    // Extract private mutes from decrypted content
    for (const tag of decryptedPrivateTags) {
      if (tag[0] === 'p' && tag[1]) {
        const pubkey = Pubkey.tryFromString(tag[1])
        if (pubkey) {
          privateMutes.push(pubkey)
        }
      }
    }

    return new MuteList(owner, publicMutes, privateMutes)
  }

  /**
   * The owner of this mute list
   */
  get owner(): Pubkey {
    return this._owner
  }

  /**
   * Total number of muted users
   */
  get count(): number {
    return this._publicMutes.size + this._privateMutes.size
  }

  /**
   * Number of publicly muted users
   */
  get publicCount(): number {
    return this._publicMutes.size
  }

  /**
   * Number of privately muted users
   */
  get privateCount(): number {
    return this._privateMutes.size
  }

  /**
   * Get all muted pubkeys (both public and private)
   */
  getAllMuted(): Pubkey[] {
    return [...this.getPublicMuted(), ...this.getPrivateMuted()]
  }

  /**
   * Get publicly muted pubkeys
   */
  getPublicMuted(): Pubkey[] {
    return Array.from(this._publicMutes.values())
  }

  /**
   * Get privately muted pubkeys
   */
  getPrivateMuted(): Pubkey[] {
    return Array.from(this._privateMutes.values())
  }

  /**
   * Check if a user is muted (either publicly or privately)
   */
  isMuted(pubkey: Pubkey): boolean {
    return this._publicMutes.has(pubkey.hex) || this._privateMutes.has(pubkey.hex)
  }

  /**
   * Get the mute visibility for a user
   */
  getMuteVisibility(pubkey: Pubkey): MuteVisibility | null {
    if (this._publicMutes.has(pubkey.hex)) return 'public'
    if (this._privateMutes.has(pubkey.hex)) return 'private'
    return null
  }

  /**
   * Mute a user publicly
   *
   * @throws CannotMuteSelfError if attempting to mute self
   * @returns MuteListChange indicating what changed
   */
  mutePublicly(pubkey: Pubkey): MuteListChange {
    if (pubkey.equals(this._owner)) {
      throw new CannotMuteSelfError()
    }

    // Already publicly muted
    if (this._publicMutes.has(pubkey.hex)) {
      return { type: 'no_change' }
    }

    // Was privately muted, switch to public
    if (this._privateMutes.has(pubkey.hex)) {
      this._privateMutes.delete(pubkey.hex)
      this._publicMutes.set(pubkey.hex, pubkey)
      return { type: 'visibility_changed', pubkey, from: 'private', to: 'public' }
    }

    // New public mute
    this._publicMutes.set(pubkey.hex, pubkey)
    return { type: 'muted', pubkey, visibility: 'public' }
  }

  /**
   * Mute a user privately
   *
   * @throws CannotMuteSelfError if attempting to mute self
   * @returns MuteListChange indicating what changed
   */
  mutePrivately(pubkey: Pubkey): MuteListChange {
    if (pubkey.equals(this._owner)) {
      throw new CannotMuteSelfError()
    }

    // Already privately muted
    if (this._privateMutes.has(pubkey.hex)) {
      return { type: 'no_change' }
    }

    // Was publicly muted, switch to private
    if (this._publicMutes.has(pubkey.hex)) {
      this._publicMutes.delete(pubkey.hex)
      this._privateMutes.set(pubkey.hex, pubkey)
      return { type: 'visibility_changed', pubkey, from: 'public', to: 'private' }
    }

    // New private mute
    this._privateMutes.set(pubkey.hex, pubkey)
    return { type: 'muted', pubkey, visibility: 'private' }
  }

  /**
   * Unmute a user (removes from both public and private)
   *
   * @returns MuteListChange indicating what changed
   */
  unmute(pubkey: Pubkey): MuteListChange {
    if (this._publicMutes.has(pubkey.hex)) {
      this._publicMutes.delete(pubkey.hex)
      return { type: 'unmuted', pubkey }
    }

    if (this._privateMutes.has(pubkey.hex)) {
      this._privateMutes.delete(pubkey.hex)
      return { type: 'unmuted', pubkey }
    }

    return { type: 'no_change' }
  }

  /**
   * Switch a public mute to private
   *
   * @returns MuteListChange indicating what changed
   */
  switchToPrivate(pubkey: Pubkey): MuteListChange {
    return this.mutePrivately(pubkey)
  }

  /**
   * Switch a private mute to public
   *
   * @returns MuteListChange indicating what changed
   */
  switchToPublic(pubkey: Pubkey): MuteListChange {
    return this.mutePublicly(pubkey)
  }

  /**
   * Convert public mutes to Nostr event tags format
   */
  toPublicTags(): string[][] {
    return Array.from(this._publicMutes.values()).map((pubkey) => ['p', pubkey.hex])
  }

  /**
   * Convert private mutes to tags format (for encryption)
   */
  toPrivateTags(): string[][] {
    return Array.from(this._privateMutes.values()).map((pubkey) => ['p', pubkey.hex])
  }

  /**
   * Convert to a draft event for publishing
   *
   * Note: The content field should be encrypted by the caller using NIP-04
   * with JSON.stringify(this.toPrivateTags())
   *
   * @param encryptedContent The NIP-04 encrypted private tags
   */
  toDraftEvent(encryptedContent: string = ''): {
    kind: number
    content: string
    created_at: number
    tags: string[][]
  } {
    return {
      kind: kinds.Mutelist,
      content: encryptedContent,
      created_at: Timestamp.now().unix,
      tags: this.toPublicTags()
    }
  }

  /**
   * Check if private mutes need to be encrypted/updated
   * Returns true if there are private mutes that need to be persisted
   */
  hasPrivateMutes(): boolean {
    return this._privateMutes.size > 0
  }
}
