/**
 * Responsive Image Service
 *
 * Looks up kind 1063 binding events for images and provides variant selection
 * for responsive image display. Caches results to avoid repeated queries.
 */

import { RECOMMENDED_SEARCH_RELAYS } from '@/constants'
import { UploadedVariant, parseResponsiveImageEvent } from '@/lib/responsive-image-event'
import clientService from './client.service'
import { Event as NEvent } from 'nostr-tools'

/** Cache of binding events by sha256 hash */
const variantCache = new Map<string, UploadedVariant[] | null>()

/** Pending lookups to avoid duplicate queries */
const pendingLookups = new Map<string, Promise<UploadedVariant[] | null>>()

/** File metadata event kind (NIP-94) */
const FILE_METADATA_KIND = 1063

/**
 * Extract sha256 hash from a blossom URL
 * Blossom URLs are typically: https://domain/sha256.ext or https://domain/sha256
 */
export function extractSha256FromUrl(url: string): string | null {
  try {
    const urlObj = new URL(url)
    const path = urlObj.pathname
    // Get the last path segment
    const segments = path.split('/').filter(Boolean)
    if (segments.length === 0) return null

    const lastSegment = segments[segments.length - 1]
    // Remove extension if present
    const hashPart = lastSegment.replace(/\.[^.]+$/, '')

    // Validate it looks like a sha256 (64 hex chars)
    if (/^[a-fA-F0-9]{64}$/.test(hashPart)) {
      return hashPart.toLowerCase()
    }
    return null
  } catch {
    return null
  }
}

/**
 * Look up responsive variants for an image by its sha256 hash
 *
 * @param sha256 - The sha256 hash of the image
 * @returns Array of variants sorted by width, or null if no binding found
 */
export async function getVariantsForHash(sha256: string): Promise<UploadedVariant[] | null> {
  // Check cache first
  if (variantCache.has(sha256)) {
    return variantCache.get(sha256) ?? null
  }

  // Check if there's already a pending lookup
  const pending = pendingLookups.get(sha256)
  if (pending) {
    return pending
  }

  // Start new lookup
  const lookupPromise = doLookup(sha256)
  pendingLookups.set(sha256, lookupPromise)

  try {
    const result = await lookupPromise
    variantCache.set(sha256, result)
    return result
  } finally {
    pendingLookups.delete(sha256)
  }
}

/**
 * Look up responsive variants for an image by its URL
 */
export async function getVariantsForUrl(url: string): Promise<UploadedVariant[] | null> {
  const sha256 = extractSha256FromUrl(url)
  if (!sha256) return null
  return getVariantsForHash(sha256)
}

/**
 * Select the best variant for a given display width
 *
 * @param variants - Available variants
 * @param targetWidth - Target display width in pixels
 * @param pixelRatio - Device pixel ratio (default 1)
 * @returns Best matching variant - smallest that covers the target width
 */
export function selectVariant(
  variants: UploadedVariant[],
  targetWidth: number,
  pixelRatio: number = 1
): UploadedVariant | null {
  if (variants.length === 0) return null

  const effectiveWidth = targetWidth * pixelRatio

  // Sort by width ascending
  const sorted = [...variants].sort((a, b) => a.width - b.width)

  // Find smallest variant >= effective width (covers target without waste)
  for (const variant of sorted) {
    if (variant.width >= effectiveWidth) {
      return variant
    }
  }

  // If none large enough, return largest available
  return sorted[sorted.length - 1]
}

/**
 * Get the original (largest) variant
 */
export function getOriginalVariant(variants: UploadedVariant[]): UploadedVariant | null {
  const original = variants.find((v) => v.variant === 'original')
  if (original) return original

  // Fall back to largest by width
  if (variants.length === 0) return null
  return variants.reduce((a, b) => (a.width > b.width ? a : b))
}

/**
 * Get the thumbnail variant
 */
export function getThumbnailVariant(variants: UploadedVariant[]): UploadedVariant | null {
  return variants.find((v) => v.variant === 'thumb') ?? null
}

/**
 * Clear the cache (useful for testing or memory management)
 */
export function clearCache(): void {
  variantCache.clear()
}

/**
 * Prefetch variants for multiple image hashes
 */
export async function prefetchVariants(sha256s: string[]): Promise<void> {
  await Promise.all(sha256s.map((hash) => getVariantsForHash(hash)))
}

// Internal lookup function
async function doLookup(sha256: string): Promise<UploadedVariant[] | null> {
  try {
    // Combine user's relays with recommended search relays for better discovery
    const relaysToQuery = Array.from(new Set([
      ...clientService.currentRelays,
      ...RECOMMENDED_SEARCH_RELAYS
    ]))

    // Query for kind 1063 events with x tag matching this hash
    const events = await clientService.fetchEvents(
      relaysToQuery,
      {
        kinds: [FILE_METADATA_KIND],
        '#x': [sha256],
        limit: 5
      }
    )

    if (!events || events.length === 0) return null

    // Use the most recent event
    const eventsArray = Array.from(events) as NEvent[]
    const latest = eventsArray.reduce((a, b) =>
      a.created_at > b.created_at ? a : b
    )

    try {
      const variants = parseResponsiveImageEvent({
        kind: latest.kind,
        tags: latest.tags
      })
      return variants.length > 0 ? variants : null
    } catch {
      return null
    }
  } catch (err) {
    console.warn('Failed to lookup responsive variants:', err)
    return null
  }
}

// Export as default object for consistency with other services
const responsiveImageService = {
  extractSha256FromUrl,
  getVariantsForHash,
  getVariantsForUrl,
  selectVariant,
  getOriginalVariant,
  getThumbnailVariant,
  clearCache,
  prefetchVariants
}

export default responsiveImageService
