/**
 * Responsive Image Variants - Event Creation/Parsing
 *
 * Creates and parses NIP-94 File Metadata events (kind 1063)
 * for responsive image sets per NIP-XX specification.
 */

import { ImageVariant, VARIANT_ORDER, FILE_METADATA_KIND } from './constants'

/** Uploaded variant information from Blossom */
export type UploadedVariant = {
  variant: ImageVariant
  url: string
  sha256: string
  width: number
  height: number
  mimeType: string
  size?: number
  blurhash?: string
}

/** Options for creating a responsive image event */
export type ResponsiveImageEventOptions = {
  /** Description/caption for the image */
  description?: string
  /** Alt text for accessibility */
  alt?: string
}

/** Draft event structure (unsigned) */
export type DraftEvent = {
  kind: number
  content: string
  tags: string[][]
  created_at: number
}

/**
 * Build an imeta tag for a single variant
 *
 * Format per NIP-92/94:
 * ["imeta", "url <url>", "x <sha256>", "m <mime>", "dim <WxH>", "variant <name>", ...]
 */
function buildImetaTag(variant: UploadedVariant): string[] {
  const tag = ['imeta']

  tag.push(`url ${variant.url}`)
  tag.push(`x ${variant.sha256}`)
  tag.push(`m ${variant.mimeType}`)
  tag.push(`dim ${variant.width}x${variant.height}`)
  tag.push(`variant ${variant.variant}`)

  if (variant.size !== undefined) {
    tag.push(`size ${variant.size}`)
  }

  if (variant.blurhash) {
    tag.push(`blurhash ${variant.blurhash}`)
  }

  return tag
}

/**
 * Create a NIP-94 File Metadata draft event for a responsive image set
 *
 * @param variants - Array of uploaded variants (should include original + scaled versions)
 * @param options - Optional description and alt text
 * @returns DraftEvent ready for signing and publishing
 */
export function createResponsiveImageEvent(
  variants: UploadedVariant[],
  options?: ResponsiveImageEventOptions
): DraftEvent {
  if (variants.length === 0) {
    throw new Error('At least one variant is required')
  }

  // Sort variants from smallest to largest
  const sortedVariants = [...variants].sort(
    (a, b) => VARIANT_ORDER.indexOf(a.variant) - VARIANT_ORDER.indexOf(b.variant)
  )

  const tags: string[][] = []

  // Add imeta tag for each variant
  for (const variant of sortedVariants) {
    tags.push(buildImetaTag(variant))
  }

  // Add separate x tags for each variant hash (enables NIP-01 tag queries)
  for (const variant of sortedVariants) {
    tags.push(['x', variant.sha256])
  }

  // Add alt tag if provided
  if (options?.alt) {
    tags.push(['alt', options.alt])
  }

  return {
    kind: FILE_METADATA_KIND,
    content: options?.description ?? '',
    tags,
    created_at: Math.floor(Date.now() / 1000)
  }
}

/**
 * Parse a kind 1063 event to extract variant information
 *
 * @param event - A kind 1063 event with imeta tags
 * @returns Array of parsed variants
 */
export function parseResponsiveImageEvent(event: { kind: number; tags: string[][] }): UploadedVariant[] {
  if (event.kind !== FILE_METADATA_KIND) {
    throw new Error(`Expected kind ${FILE_METADATA_KIND}, got ${event.kind}`)
  }

  const variants: UploadedVariant[] = []

  for (const tag of event.tags) {
    if (tag[0] !== 'imeta') continue

    const fields = new Map<string, string>()
    for (let i = 1; i < tag.length; i++) {
      const part = tag[i]
      const spaceIndex = part.indexOf(' ')
      if (spaceIndex > 0) {
        const key = part.substring(0, spaceIndex)
        const value = part.substring(spaceIndex + 1)
        fields.set(key, value)
      }
    }

    const url = fields.get('url')
    const sha256 = fields.get('x')
    const mimeType = fields.get('m')
    const dim = fields.get('dim')
    const variant = fields.get('variant') as ImageVariant | undefined

    if (!url || !sha256 || !mimeType || !dim) continue

    const dimMatch = dim.match(/^(\d+)x(\d+)$/)
    if (!dimMatch) continue

    const width = parseInt(dimMatch[1], 10)
    const height = parseInt(dimMatch[2], 10)

    const parsed: UploadedVariant = {
      variant: variant ?? 'original',
      url,
      sha256,
      width,
      height,
      mimeType
    }

    const size = fields.get('size')
    if (size) parsed.size = parseInt(size, 10)

    const blurhash = fields.get('blurhash')
    if (blurhash) parsed.blurhash = blurhash

    variants.push(parsed)
  }

  return variants
}

/**
 * Get the thumbnail variant from a set of variants
 */
export function getThumbnailVariant(variants: UploadedVariant[]): UploadedVariant | undefined {
  return variants.find((v) => v.variant === 'thumb')
}

/**
 * Get the original variant from a set of variants
 */
export function getOriginalVariant(variants: UploadedVariant[]): UploadedVariant | undefined {
  return variants.find((v) => v.variant === 'original')
}
