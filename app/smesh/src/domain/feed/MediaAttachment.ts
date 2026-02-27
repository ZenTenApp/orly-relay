/**
 * Media type for attachments
 */
export type MediaType = 'image' | 'video' | 'audio' | 'file'

/**
 * Upload status for media
 */
export type UploadStatus = 'pending' | 'uploading' | 'completed' | 'failed'

/**
 * Image metadata from imeta tag
 */
export interface ImageMetadata {
  url: string
  mimeType?: string
  width?: number
  height?: number
  size?: number
  blurhash?: string
  sha256?: string
  alt?: string
}

/**
 * MediaAttachment Value Object
 *
 * Represents a media file attached to a note.
 * Handles URL validation, type detection, and imeta tag generation.
 */
export class MediaAttachment {
  private constructor(
    private readonly _url: string,
    private readonly _type: MediaType,
    private readonly _mimeType: string | null,
    private readonly _metadata: ImageMetadata | null,
    private readonly _status: UploadStatus,
    private readonly _alt: string | null
  ) {}

  /**
   * Create from a URL (after upload)
   */
  static fromUrl(url: string, mimeType?: string): MediaAttachment {
    const type = MediaAttachment.detectType(url, mimeType)
    return new MediaAttachment(
      url,
      type,
      mimeType ?? null,
      null,
      'completed',
      null
    )
  }

  /**
   * Create with full metadata (from imeta)
   */
  static fromMetadata(metadata: ImageMetadata): MediaAttachment {
    const type = MediaAttachment.detectType(metadata.url, metadata.mimeType)
    return new MediaAttachment(
      metadata.url,
      type,
      metadata.mimeType ?? null,
      metadata,
      'completed',
      metadata.alt ?? null
    )
  }

  /**
   * Create a pending attachment (before upload)
   */
  static pending(_fileName: string, type: MediaType): MediaAttachment {
    return new MediaAttachment(
      '', // No URL yet
      type,
      null,
      null,
      'pending',
      null
    )
  }

  /**
   * Detect media type from URL or mime type
   */
  private static detectType(url: string, mimeType?: string): MediaType {
    // Check mime type first
    if (mimeType) {
      if (mimeType.startsWith('image/')) return 'image'
      if (mimeType.startsWith('video/')) return 'video'
      if (mimeType.startsWith('audio/')) return 'audio'
    }

    // Fall back to URL extension
    const urlLower = url.toLowerCase()
    if (/\.(jpg|jpeg|png|gif|webp|heic|avif|svg)(\?|$)/.test(urlLower)) {
      return 'image'
    }
    if (/\.(mp4|webm|mov|avi|mkv)(\?|$)/.test(urlLower)) {
      return 'video'
    }
    if (/\.(mp3|wav|ogg|flac|m4a)(\?|$)/.test(urlLower)) {
      return 'audio'
    }

    return 'file'
  }

  // Getters
  get url(): string {
    return this._url
  }

  get type(): MediaType {
    return this._type
  }

  get mimeType(): string | null {
    return this._mimeType
  }

  get metadata(): ImageMetadata | null {
    return this._metadata
  }

  get status(): UploadStatus {
    return this._status
  }

  get alt(): string | null {
    return this._alt
  }

  get isImage(): boolean {
    return this._type === 'image'
  }

  get isVideo(): boolean {
    return this._type === 'video'
  }

  get isAudio(): boolean {
    return this._type === 'audio'
  }

  get isUploaded(): boolean {
    return this._status === 'completed' && this._url !== ''
  }

  /**
   * Generate imeta tag for this attachment
   * Returns null if not an image or missing required data
   */
  toImetaTag(): string[] | null {
    if (!this.isImage || !this._url) return null

    const tag = ['imeta', `url ${this._url}`]

    if (this._mimeType) {
      tag.push(`m ${this._mimeType}`)
    }

    if (this._metadata) {
      if (this._metadata.width && this._metadata.height) {
        tag.push(`dim ${this._metadata.width}x${this._metadata.height}`)
      }
      if (this._metadata.size) {
        tag.push(`size ${this._metadata.size}`)
      }
      if (this._metadata.blurhash) {
        tag.push(`blurhash ${this._metadata.blurhash}`)
      }
      if (this._metadata.sha256) {
        tag.push(`x ${this._metadata.sha256}`)
      }
    }

    if (this._alt) {
      tag.push(`alt ${this._alt}`)
    }

    return tag
  }

  /**
   * Set alt text
   */
  withAlt(alt: string): MediaAttachment {
    return new MediaAttachment(
      this._url,
      this._type,
      this._mimeType,
      this._metadata,
      this._status,
      alt
    )
  }

  /**
   * Update status
   */
  withStatus(status: UploadStatus): MediaAttachment {
    return new MediaAttachment(
      this._url,
      this._type,
      this._mimeType,
      this._metadata,
      status,
      this._alt
    )
  }

  /**
   * Set URL after upload
   */
  withUrl(url: string, metadata?: ImageMetadata): MediaAttachment {
    return new MediaAttachment(
      url,
      this._type,
      metadata?.mimeType ?? this._mimeType,
      metadata ?? this._metadata,
      'completed',
      this._alt
    )
  }

  /**
   * Check equality
   */
  equals(other: MediaAttachment): boolean {
    return this._url === other._url
  }
}
