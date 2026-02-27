/**
 * Hook for responsive image variant selection
 *
 * Automatically fetches and selects the appropriate image variant
 * based on container width and device pixel ratio.
 */

import { UploadedVariant } from '@/lib/responsive-image-event'
import responsiveImageService from '@/services/responsive-image.service'
import { TImetaInfo } from '@/types'
import { useEffect, useMemo, useState } from 'react'

export type UseResponsiveImageOptions = {
  /** Target container width (defaults to viewport width) */
  containerWidth?: number
  /** Force use of original variant (for lightbox) */
  useOriginal?: boolean
  /** Force use of thumbnail variant */
  useThumbnail?: boolean
}

export type UseResponsiveImageResult = {
  /** The selected image info (may be original or a variant) */
  imageInfo: TImetaInfo
  /** All available variants (empty if not a responsive image) */
  variants: UploadedVariant[]
  /** Whether variants are still loading */
  isLoading: boolean
  /** The original/full size variant URL (for lightbox) */
  originalUrl: string
  /** Whether this image has responsive variants */
  hasVariants: boolean
}

/**
 * Hook to get responsive image variant for display
 *
 * @param image - The original image info
 * @param options - Selection options
 */
export function useResponsiveImage(
  image: TImetaInfo,
  options: UseResponsiveImageOptions = {}
): UseResponsiveImageResult {
  const { containerWidth, useOriginal = false, useThumbnail = false } = options

  const [variants, setVariants] = useState<UploadedVariant[]>([])
  const [isLoading, setIsLoading] = useState(false)

  // Get sha256 from image (from imeta tag or extract from URL)
  const sha256 = useMemo(() => {
    if (image.sha256) return image.sha256
    return responsiveImageService.extractSha256FromUrl(image.url)
  }, [image.sha256, image.url])

  // Fetch variants when sha256 is available
  useEffect(() => {
    if (!sha256) {
      setVariants([])
      return
    }

    let cancelled = false
    setIsLoading(true)

    responsiveImageService.getVariantsForHash(sha256).then((result) => {
      if (cancelled) return
      setVariants(result ?? [])
      setIsLoading(false)
    })

    return () => {
      cancelled = true
    }
  }, [sha256])

  // Get viewport width for responsive selection
  const viewportWidth = useMemo(() => {
    if (typeof window === 'undefined') return 1280
    return window.innerWidth
  }, [])

  // Select the best variant
  const selectedVariant = useMemo(() => {
    if (variants.length === 0) return null

    if (useOriginal) {
      return responsiveImageService.getOriginalVariant(variants)
    }

    if (useThumbnail) {
      return responsiveImageService.getThumbnailVariant(variants)
    }

    const targetWidth = containerWidth ?? viewportWidth
    const pixelRatio = typeof window !== 'undefined' ? window.devicePixelRatio : 1

    return responsiveImageService.selectVariant(variants, targetWidth, pixelRatio)
  }, [variants, containerWidth, viewportWidth, useOriginal, useThumbnail])

  // Build the result image info
  const imageInfo: TImetaInfo = useMemo(() => {
    if (!selectedVariant) return image

    return {
      url: selectedVariant.url,
      sha256: selectedVariant.sha256,
      blurHash: selectedVariant.blurhash ?? image.blurHash,
      thumbHash: image.thumbHash,
      dim: {
        width: selectedVariant.width,
        height: selectedVariant.height
      },
      pubkey: image.pubkey,
      variant: selectedVariant.variant
    }
  }, [selectedVariant, image])

  // Get original URL for lightbox
  const originalUrl = useMemo(() => {
    if (variants.length === 0) return image.url
    const original = responsiveImageService.getOriginalVariant(variants)
    return original?.url ?? image.url
  }, [variants, image.url])

  return {
    imageInfo,
    variants,
    isLoading,
    originalUrl,
    hasVariants: variants.length > 0
  }
}

export default useResponsiveImage
