/**
 * Responsive Image Variants - Image Scaler
 *
 * Client-side image scaling for responsive image variants.
 * Generates multiple resolution variants with EXIF stripping.
 */

import { ImageVariant, VARIANT_SIZES, VARIANT_ORDER, ORIGINAL_QUALITY } from './constants'

/** Result of scaling an image to a variant size */
export type ScaledImage = {
  variant: ImageVariant
  blob: Blob
  width: number
  height: number
  mimeType: string
}

/** Options for variant generation */
export type ScaleOptions = {
  /** Callback for progress updates (0-100) */
  onProgress?: (percent: number) => void
}

/**
 * Load an image file and return an ImageBitmap
 * This reads only pixel data, automatically stripping EXIF metadata.
 */
async function loadImage(file: File): Promise<ImageBitmap> {
  return createImageBitmap(file)
}

/**
 * Determine output MIME type based on input type
 * Preserves PNG/WebP for transparency, converts others to JPEG.
 */
function getOutputMimeType(inputType: string): string {
  if (inputType === 'image/png') return 'image/png'
  if (inputType === 'image/webp') return 'image/webp'
  if (inputType === 'image/gif') return 'image/png' // Preserve transparency
  return 'image/jpeg'
}

/**
 * Scale an image to a target width while preserving aspect ratio
 */
function scaleToWidth(
  source: ImageBitmap,
  targetWidth: number,
  mimeType: string,
  quality: number
): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const aspectRatio = source.height / source.width
    const targetHeight = Math.round(targetWidth * aspectRatio)

    const canvas = document.createElement('canvas')
    canvas.width = targetWidth
    canvas.height = targetHeight

    const ctx = canvas.getContext('2d')
    if (!ctx) {
      reject(new Error('Failed to get canvas context'))
      return
    }

    ctx.imageSmoothingEnabled = true
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(source, 0, 0, targetWidth, targetHeight)

    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob)
        else reject(new Error('Failed to create blob from canvas'))
      },
      mimeType,
      quality
    )
  })
}

/**
 * Create the original variant (full size, EXIF stripped)
 */
function createOriginal(
  source: ImageBitmap,
  mimeType: string,
  quality: number
): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const canvas = document.createElement('canvas')
    canvas.width = source.width
    canvas.height = source.height

    const ctx = canvas.getContext('2d')
    if (!ctx) {
      reject(new Error('Failed to get canvas context'))
      return
    }

    ctx.drawImage(source, 0, 0)

    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob)
        else reject(new Error('Failed to create blob from canvas'))
      },
      mimeType,
      quality
    )
  })
}

/**
 * Determine which variants to generate based on original image width
 * Only generates variants smaller than the original (no upscaling).
 */
function getVariantsToGenerate(originalWidth: number): ImageVariant[] {
  const variants: ImageVariant[] = ['original']

  for (const [variant, config] of Object.entries(VARIANT_SIZES)) {
    if (config.width < originalWidth) {
      variants.push(variant as ImageVariant)
    }
  }

  return variants.sort((a, b) => VARIANT_ORDER.indexOf(a) - VARIANT_ORDER.indexOf(b))
}

/**
 * Generate all applicable image variants for a file
 *
 * @param file - The image file to scale
 * @param options - Optional progress callback
 * @returns Array of scaled images, sorted from smallest to largest
 */
export async function generateImageVariants(
  file: File,
  options?: ScaleOptions
): Promise<ScaledImage[]> {
  const { onProgress } = options ?? {}

  onProgress?.(0)

  const bitmap = await loadImage(file)
  const mimeType = getOutputMimeType(file.type)

  onProgress?.(10)

  const variantsToGenerate = getVariantsToGenerate(bitmap.width)
  const totalVariants = variantsToGenerate.length
  const results: ScaledImage[] = []

  for (let i = 0; i < variantsToGenerate.length; i++) {
    const variant = variantsToGenerate[i]

    let blob: Blob
    let width: number
    let height: number

    if (variant === 'original') {
      blob = await createOriginal(bitmap, mimeType, ORIGINAL_QUALITY)
      width = bitmap.width
      height = bitmap.height
    } else {
      const config = VARIANT_SIZES[variant]
      const aspectRatio = bitmap.height / bitmap.width
      width = config.width
      height = Math.round(config.width * aspectRatio)
      blob = await scaleToWidth(bitmap, config.width, mimeType, config.quality)
    }

    results.push({ variant, blob, width, height, mimeType })

    const progress = 10 + Math.round(((i + 1) / totalVariants) * 80)
    onProgress?.(progress)
  }

  onProgress?.(90)

  return results
}

/**
 * Check if a file is a supported image type
 */
export function isSupportedImage(file: File): boolean {
  const supportedTypes = [
    'image/jpeg',
    'image/jpg',
    'image/png',
    'image/webp',
    'image/gif'
  ]
  return supportedTypes.includes(file.type)
}

/**
 * Get file extension from MIME type
 */
export function getExtensionFromMimeType(mimeType: string): string {
  const extensions: Record<string, string> = {
    'image/jpeg': 'jpg',
    'image/png': 'png',
    'image/webp': 'webp',
    'image/gif': 'gif'
  }
  return extensions[mimeType] ?? 'jpg'
}
