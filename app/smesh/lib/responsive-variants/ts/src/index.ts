/**
 * Responsive Image Variants
 *
 * A library for generating and selecting responsive image variants
 * per the NIP-XX Responsive Image Variants specification.
 *
 * @example
 * ```typescript
 * import {
 *   generateImageVariants,
 *   createResponsiveImageEvent,
 *   selectVariantForViewport
 * } from 'responsive-variants'
 *
 * // Generate variants from a file
 * const variants = await generateImageVariants(file)
 *
 * // Upload variants and collect metadata...
 *
 * // Create binding event
 * const event = createResponsiveImageEvent(uploadedVariants)
 *
 * // Later, select variant for display
 * const best = selectVariantForViewport(variants, containerWidth, devicePixelRatio)
 * ```
 */

// Constants
export {
  ImageVariant,
  VariantConfig,
  VARIANT_SIZES,
  VARIANT_WIDTHS,
  VARIANT_QUALITY,
  VARIANT_ORDER,
  ORIGINAL_QUALITY,
  FILE_METADATA_KIND
} from './constants'

// Image scaling
export {
  ScaledImage,
  ScaleOptions,
  generateImageVariants,
  isSupportedImage,
  getExtensionFromMimeType
} from './scaler'

// Event creation/parsing
export {
  UploadedVariant,
  ResponsiveImageEventOptions,
  DraftEvent,
  createResponsiveImageEvent,
  parseResponsiveImageEvent,
  getThumbnailVariant,
  getOriginalVariant
} from './event'

// Variant selection
export {
  selectVariantForViewport,
  calculateDisplayDimensions,
  getPlaceholderDimensions,
  isValidBlobHash,
  extractHashFromUrl
} from './selector'
