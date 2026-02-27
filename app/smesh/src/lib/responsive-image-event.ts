/**
 * Creates NIP-94 File Metadata events (kind 1063) for responsive image sets
 *
 * Follows NIP-XX Responsive Image Variants specification, extending NIP-94
 * with multiple imeta tags per the NIP-71 video variants pattern.
 */

import { TDraftEvent } from '@/types'
import dayjs from 'dayjs'
import { ImageVariant } from './image-scaler'

/** Kind 1063 = File Metadata (NIP-94) */
export const FILE_METADATA_KIND = 1063

/**
 * Uploaded variant information from Blossom
 */
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

/**
 * Options for creating a responsive image event
 */
export type ResponsiveImageEventOptions = {
  /** Description/caption for the image */
  description?: string
  /** Alt text for accessibility */
  alt?: string
}

/**
 * Build an imeta tag for a single variant
 *
 * Format per NIP-92/94:
 * ["imeta", "url <url>", "x <sha256>", "m <mime>", "dim <WxH>", "variant <name>", ...]
 */
function buildImetaTag(variant: UploadedVariant): string[] {
  const tag = ['imeta']

  // Required fields
  tag.push(`url ${variant.url}`)
  tag.push(`x ${variant.sha256}`)
  tag.push(`m ${variant.mimeType}`)
  tag.push(`dim ${variant.width}x${variant.height}`)
  tag.push(`variant ${variant.variant}`)

  // Optional fields
  if (variant.size !== undefined) {
    tag.push(`size ${variant.size}`)
  }

  // Blurhash is especially useful for thumbnails
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
 * @returns TDraftEvent ready for signing and publishing
 */
export function createResponsiveImageEvent(
  variants: UploadedVariant[],
  options?: ResponsiveImageEventOptions
): TDraftEvent {
  if (variants.length === 0) {
    throw new Error('At least one variant is required')
  }

  // Sort variants from smallest to largest for consistent ordering per NIP-XX
  const variantOrder: ImageVariant[] = ['thumb', 'mobile-sm', 'mobile-lg', 'desktop-sm', 'desktop-md', 'desktop-lg', 'original']
  const sortedVariants = [...variants].sort(
    (a, b) => variantOrder.indexOf(a.variant) - variantOrder.indexOf(b.variant)
  )

  // Build tags array
  const tags: string[][] = []

  // Add imeta tag for each variant
  for (const variant of sortedVariants) {
    tags.push(buildImetaTag(variant))
  }

  // Add separate x tags for each variant hash (enables NIP-01 tag queries)
  for (const variant of sortedVariants) {
    tags.push(['x', variant.sha256])
  }

  // Add alt tag if provided (for accessibility)
  if (options?.alt) {
    tags.push(['alt', options.alt])
  }

  return {
    kind: FILE_METADATA_KIND,
    content: options?.description ?? '',
    tags,
    created_at: dayjs().unix()
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

    // Required fields
    const url = fields.get('url')
    const sha256 = fields.get('x')
    const mimeType = fields.get('m')
    const dim = fields.get('dim')
    const variant = fields.get('variant') as ImageVariant | undefined

    if (!url || !sha256 || !mimeType || !dim) continue

    // Parse dimensions
    const dimMatch = dim.match(/^(\d+)x(\d+)$/)
    if (!dimMatch) continue

    const width = parseInt(dimMatch[1], 10)
    const height = parseInt(dimMatch[2], 10)

    // Build variant object
    const parsed: UploadedVariant = {
      variant: variant ?? 'original',
      url,
      sha256,
      width,
      height,
      mimeType
    }

    // Optional fields
    const size = fields.get('size')
    if (size) {
      parsed.size = parseInt(size, 10)
    }

    const blurhash = fields.get('blurhash')
    if (blurhash) {
      parsed.blurhash = blurhash
    }

    variants.push(parsed)
  }

  return variants
}

/**
 * Select the best variant for a given viewport width
 *
 * @param variants - Available variants
 * @param viewportWidth - Current viewport width in pixels
 * @param pixelRatio - Device pixel ratio (default 1)
 * @returns The most appropriate variant, or undefined if none available
 */
export function selectVariantForViewport(
  variants: UploadedVariant[],
  viewportWidth: number,
  pixelRatio: number = 1
): UploadedVariant | undefined {
  if (variants.length === 0) return undefined

  const targetWidth = viewportWidth * pixelRatio

  // Sort by width ascending
  const sorted = [...variants].sort((a, b) => a.width - b.width)

  // Find smallest variant >= target width
  for (const variant of sorted) {
    if (variant.width >= targetWidth) {
      return variant
    }
  }

  // If none large enough, return largest available
  return sorted[sorted.length - 1]
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
