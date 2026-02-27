/**
 * Responsive Image Component
 *
 * Displays an image from a NIP-94 kind 1063 event with multiple resolution variants.
 * Automatically selects the appropriate variant based on viewport width and device pixel ratio.
 * Can also detect 64-character hex blob hashes and query for binding events automatically.
 */

import Image from '@/components/Image'
import {
  parseResponsiveImageEvent,
  selectVariantForViewport,
  getThumbnailVariant,
  UploadedVariant
} from '@/lib/responsive-image-event'
import { cn } from '@/lib/utils'
import { TImetaInfo } from '@/types'
import { useEffect, useMemo, useState } from 'react'
import blossomService from '@/services/blossom.service'

type ResponsiveImageProps = {
  /** The kind 1063 event containing imeta tags for all variants */
  event?: { kind: number; tags: string[][] }
  /** Or provide pre-parsed variants directly */
  variants?: UploadedVariant[]
  /** Target container width (defaults to viewport width) */
  containerWidth?: number
  /** Alt text for accessibility */
  alt?: string
  /** Pubkey for Blossom URL validation */
  pubkey?: string
  /** Additional class names */
  className?: string
  /** Class names for sub-elements */
  classNames?: {
    wrapper?: string
    errorPlaceholder?: string
    skeleton?: string
  }
  /** Hide if all variants fail to load */
  hideIfError?: boolean
  /** Custom error placeholder */
  errorPlaceholder?: React.ReactNode
  /** Force a specific variant (bypasses viewport selection) */
  forceVariant?: 'thumb' | 'mobile-sm' | 'mobile-lg' | 'desktop-sm' | 'desktop-md' | 'desktop-lg' | 'original'
  /** Use thumbnail mode (always show thumb variant) */
  thumbnail?: boolean
}

export default function ResponsiveImage({
  event,
  variants: providedVariants,
  containerWidth,
  alt,
  pubkey,
  className,
  classNames,
  hideIfError,
  errorPlaceholder,
  forceVariant,
  thumbnail
}: ResponsiveImageProps) {
  const [viewportWidth, setViewportWidth] = useState(
    typeof window !== 'undefined' ? window.innerWidth : 1280
  )

  // Parse variants from event if provided
  const variants = useMemo(() => {
    if (providedVariants) return providedVariants
    if (event) {
      try {
        return parseResponsiveImageEvent(event)
      } catch {
        return []
      }
    }
    return []
  }, [event, providedVariants])

  // Update viewport width on resize
  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleResize = () => {
      setViewportWidth(window.innerWidth)
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  // Select the best variant
  const selectedVariant = useMemo(() => {
    if (variants.length === 0) return null

    // Thumbnail mode - always use thumb
    if (thumbnail) {
      const thumb = getThumbnailVariant(variants)
      if (thumb) return thumb
    }

    // Force specific variant if requested
    if (forceVariant) {
      const forced = variants.find((v) => v.variant === forceVariant)
      if (forced) return forced
    }

    // Select based on viewport
    const targetWidth = containerWidth ?? viewportWidth
    const pixelRatio = typeof window !== 'undefined' ? window.devicePixelRatio : 1

    return selectVariantForViewport(variants, targetWidth, pixelRatio)
  }, [variants, viewportWidth, containerWidth, forceVariant, thumbnail])

  // Build the image info for the Image component
  const imageInfo: TImetaInfo | null = useMemo(() => {
    if (!selectedVariant) return null

    return {
      url: selectedVariant.url,
      blurHash: selectedVariant.blurhash,
      pubkey,
      dim: {
        width: selectedVariant.width,
        height: selectedVariant.height
      }
    }
  }, [selectedVariant, pubkey])

  if (!imageInfo) {
    if (hideIfError) return null
    return (
      <div
        className={cn(
          'flex items-center justify-center bg-muted rounded-xl',
          className,
          classNames?.errorPlaceholder
        )}
      >
        {errorPlaceholder ?? <span className="text-muted-foreground">No image</span>}
      </div>
    )
  }

  return (
    <Image
      image={imageInfo}
      alt={alt}
      className={className}
      classNames={classNames}
      hideIfError={hideIfError}
      errorPlaceholder={errorPlaceholder}
    />
  )
}

/**
 * Hook to fetch and parse a responsive image event
 */
export function useResponsiveImage(eventId?: string) {
  const [variants, setVariants] = useState<UploadedVariant[]>([])
  const [loading, setLoading] = useState(false)
  const [error, _setError] = useState<Error | null>(null)

  useEffect(() => {
    if (!eventId) {
      setVariants([])
      return
    }

    // TODO: Fetch event from relays by ID
    // For now, this is a placeholder for the fetch logic
    setLoading(true)
    // client.fetchEvent(eventId).then(event => {
    //   if (event?.kind === 1063) {
    //     setVariants(parseResponsiveImageEvent(event))
    //   }
    // }).catch(setError).finally(() => setLoading(false))
    setLoading(false)
  }, [eventId])

  return { variants, loading, error }
}

/**
 * Hook to fetch responsive image variants from a blob hash.
 * Queries relays for kind 1063 binding events with matching x tag.
 *
 * @param blobHash - 64-character SHA256 hash of any variant
 * @returns Parsed variants, loading state, and whether binding event was found
 */
export function useBindingEvent(blobHash?: string) {
  const [variants, setVariants] = useState<UploadedVariant[]>([])
  const [loading, setLoading] = useState(false)
  const [hasBindingEvent, setHasBindingEvent] = useState(false)

  useEffect(() => {
    if (!blobHash || !blossomService.isValidBlobHash(blobHash)) {
      setVariants([])
      setHasBindingEvent(false)
      return
    }

    let cancelled = false
    setLoading(true)

    blossomService.queryBindingEvent(blobHash).then((result) => {
      if (cancelled) return
      if (result && result.length > 0) {
        setVariants(result)
        setHasBindingEvent(true)
      } else {
        setVariants([])
        setHasBindingEvent(false)
      }
      setLoading(false)
    }).catch(() => {
      if (cancelled) return
      setVariants([])
      setHasBindingEvent(false)
      setLoading(false)
    })

    return () => {
      cancelled = true
    }
  }, [blobHash])

  return { variants, loading, hasBindingEvent }
}

/**
 * Props for ResponsiveImageFromHash component
 */
type ResponsiveImageFromHashProps = {
  /** The 64-character SHA256 blob hash */
  hash: string
  /** Base Blossom server URL for fallback */
  baseServerUrl?: string
  /** Target container width in CSS pixels */
  containerWidth?: number
  /** Alt text for accessibility */
  alt?: string
  /** Pubkey for Blossom URL validation */
  pubkey?: string
  /** Additional class names */
  className?: string
  /** Class names for sub-elements */
  classNames?: {
    wrapper?: string
    errorPlaceholder?: string
    skeleton?: string
  }
  /** Hide if all variants fail to load */
  hideIfError?: boolean
  /** Custom error placeholder */
  errorPlaceholder?: React.ReactNode
  /** Use thumbnail mode (always show thumb variant) */
  thumbnail?: boolean
}

/**
 * Responsive image component that loads from a blob hash.
 *
 * When given a 64-character hex hash, queries relays for a kind 1063 binding event.
 * If found, selects the appropriate variant based on container width.
 * Falls back to the original blob URL if no binding event is found.
 */
export function ResponsiveImageFromHash({
  hash,
  baseServerUrl,
  containerWidth,
  alt,
  pubkey,
  className,
  classNames,
  hideIfError,
  errorPlaceholder,
  thumbnail
}: ResponsiveImageFromHashProps) {
  const { variants, loading, hasBindingEvent } = useBindingEvent(hash)
  const [viewportWidth, setViewportWidth] = useState(
    typeof window !== 'undefined' ? window.innerWidth : 1280
  )

  // Update viewport width on resize
  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleResize = () => {
      setViewportWidth(window.innerWidth)
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  // Select the best variant
  const selectedVariant = useMemo(() => {
    if (variants.length === 0) return null

    // Thumbnail mode - always use thumb
    if (thumbnail) {
      const thumb = getThumbnailVariant(variants)
      if (thumb) return thumb
    }

    // Select based on viewport
    const targetWidth = containerWidth ?? viewportWidth
    const pixelRatio = typeof window !== 'undefined' ? window.devicePixelRatio : 1

    return selectVariantForViewport(variants, targetWidth, pixelRatio)
  }, [variants, viewportWidth, containerWidth, thumbnail])

  // Build image info
  const imageInfo: TImetaInfo | null = useMemo(() => {
    if (loading) return null

    // Use selected variant if available
    if (selectedVariant) {
      return {
        url: selectedVariant.url,
        blurHash: selectedVariant.blurhash,
        pubkey,
        dim: {
          width: selectedVariant.width,
          height: selectedVariant.height
        }
      }
    }

    // Fall back to original blob URL
    if (!hasBindingEvent && blossomService.isValidBlobHash(hash)) {
      const fallbackUrl = baseServerUrl
        ? `${baseServerUrl.replace(/\/$/, '')}/${hash}`
        : `https://blossom.band/${hash}`

      return {
        url: fallbackUrl,
        pubkey
      }
    }

    return null
  }, [selectedVariant, loading, hasBindingEvent, hash, baseServerUrl, pubkey])

  if (loading) {
    return (
      <div className={cn('animate-pulse bg-muted rounded-xl', className, classNames?.skeleton)} />
    )
  }

  if (!imageInfo) {
    if (hideIfError) return null
    return (
      <div
        className={cn(
          'flex items-center justify-center bg-muted rounded-xl',
          className,
          classNames?.errorPlaceholder
        )}
      >
        {errorPlaceholder ?? <span className="text-muted-foreground">No image</span>}
      </div>
    )
  }

  return (
    <Image
      image={imageInfo}
      alt={alt}
      className={className}
      classNames={classNames}
      hideIfError={hideIfError}
      errorPlaceholder={errorPlaceholder}
    />
  )
}
