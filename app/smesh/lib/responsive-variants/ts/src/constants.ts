/**
 * Responsive Image Variants - Constants
 *
 * Defines the standard variant sizes, quality settings, and ordering
 * for responsive image generation per NIP-XX.
 */

/** Variant name identifiers */
export type ImageVariant = 'thumb' | 'mobile-sm' | 'mobile-lg' | 'desktop-sm' | 'desktop-md' | 'desktop-lg' | 'original'

/** Configuration for a single variant */
export type VariantConfig = {
  width: number
  quality: number
}

/**
 * Standard variant sizes and quality settings
 *
 * Selection rule: Pick the smallest variant >= target width
 * (next-larger, not next-smaller) for minimal downscaling in the client.
 */
export const VARIANT_SIZES: Record<Exclude<ImageVariant, 'original'>, VariantConfig> = {
  'thumb': { width: 128, quality: 0.70 },
  'mobile-sm': { width: 512, quality: 0.75 },
  'mobile-lg': { width: 1024, quality: 0.80 },
  'desktop-sm': { width: 1536, quality: 0.85 },
  'desktop-md': { width: 2048, quality: 0.88 },
  'desktop-lg': { width: 2560, quality: 0.90 },
} as const

/** Quality setting for the original (EXIF-stripped) variant */
export const ORIGINAL_QUALITY = 0.92

/** Variants ordered from smallest to largest */
export const VARIANT_ORDER: ImageVariant[] = [
  'thumb',
  'mobile-sm',
  'mobile-lg',
  'desktop-sm',
  'desktop-md',
  'desktop-lg',
  'original'
]

/** Variant widths as a simple map for quick lookup */
export const VARIANT_WIDTHS: Record<Exclude<ImageVariant, 'original'>, number> = {
  'thumb': 128,
  'mobile-sm': 512,
  'mobile-lg': 1024,
  'desktop-sm': 1536,
  'desktop-md': 2048,
  'desktop-lg': 2560,
}

/** Variant quality settings as a simple map */
export const VARIANT_QUALITY: Record<ImageVariant, number> = {
  'thumb': 0.70,
  'mobile-sm': 0.75,
  'mobile-lg': 0.80,
  'desktop-sm': 0.85,
  'desktop-md': 0.88,
  'desktop-lg': 0.90,
  'original': 0.92,
}

/** File metadata event kind (NIP-94) */
export const FILE_METADATA_KIND = 1063
