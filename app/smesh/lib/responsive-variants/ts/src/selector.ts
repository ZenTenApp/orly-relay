/**
 * Responsive Image Variants - Variant Selector
 *
 * Implements the "next-larger" selection algorithm:
 * Pick the smallest variant >= target width for minimal client-side downscaling.
 */

import { UploadedVariant } from './event'

/**
 * Select the best variant for a given viewport width
 *
 * Selection rule: Pick the smallest variant >= target width.
 * This ensures the client only needs to downscale slightly (or not at all),
 * rather than upscaling which would cause blur.
 *
 * @param variants - Available variants
 * @param targetWidth - Desired display width in CSS pixels
 * @param pixelRatio - Device pixel ratio (default 1)
 * @returns The most appropriate variant, or undefined if none available
 */
export function selectVariantForViewport(
  variants: UploadedVariant[],
  targetWidth: number,
  pixelRatio: number = 1
): UploadedVariant | undefined {
  if (variants.length === 0) return undefined

  const effectiveWidth = targetWidth * pixelRatio

  // Sort by width ascending
  const sorted = [...variants].sort((a, b) => a.width - b.width)

  // Find smallest variant >= target width (next-larger selection)
  for (const variant of sorted) {
    if (variant.width >= effectiveWidth) {
      return variant
    }
  }

  // If none large enough, return largest available
  return sorted[sorted.length - 1]
}

/**
 * Calculate the display dimensions for a variant at a target width
 *
 * Given a variant and a target display width, calculates the height
 * maintaining the original aspect ratio. Useful for placeholder sizing.
 *
 * @param variant - The variant to calculate dimensions for
 * @param displayWidth - The CSS pixel width it will be displayed at
 * @returns Object with width and height in CSS pixels
 */
export function calculateDisplayDimensions(
  variant: UploadedVariant,
  displayWidth: number
): { width: number; height: number } {
  const aspectRatio = variant.height / variant.width
  return {
    width: displayWidth,
    height: Math.round(displayWidth * aspectRatio)
  }
}

/**
 * Pre-calculate placeholder dimensions from a binding event
 *
 * When loading an image, use this to determine the display dimensions
 * before the image loads, preventing layout shift.
 *
 * @param variants - All variants from the binding event
 * @param containerWidth - The container width in CSS pixels
 * @param pixelRatio - Device pixel ratio (default 1)
 * @returns Dimensions for the selected variant, or undefined if no variants
 */
export function getPlaceholderDimensions(
  variants: UploadedVariant[],
  containerWidth: number,
  pixelRatio: number = 1
): { width: number; height: number; selectedVariant: UploadedVariant } | undefined {
  const selected = selectVariantForViewport(variants, containerWidth, pixelRatio)
  if (!selected) return undefined

  const dims = calculateDisplayDimensions(selected, containerWidth)
  return {
    ...dims,
    selectedVariant: selected
  }
}

/**
 * Check if a string is a valid 64-character hex hash
 */
export function isValidBlobHash(str: string): boolean {
  return /^[a-f0-9]{64}$/i.test(str)
}

/**
 * Extract a blob hash from a Blossom URL
 *
 * Handles formats like:
 * - https://server.com/abc123...def456
 * - https://server.com/abc123...def456.jpg
 *
 * @param url - A Blossom blob URL
 * @returns The 64-character hash, or null if not found
 */
export function extractHashFromUrl(url: string): string | null {
  try {
    const parsed = new URL(url)
    const pathname = parsed.pathname

    // Remove leading slash and any file extension
    const filename = pathname.split('/').pop() || ''
    const hash = filename.replace(/\.[^.]+$/, '')

    return isValidBlobHash(hash) ? hash.toLowerCase() : null
  } catch {
    return null
  }
}
