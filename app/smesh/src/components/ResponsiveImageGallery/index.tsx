/**
 * Responsive Image Gallery
 *
 * Wraps ImageGallery with automatic responsive variant selection.
 * Displays appropriately-sized variants in the gallery and shows
 * full-size originals in the lightbox.
 */

import { randomString } from '@/lib/random'
import { cn } from '@/lib/utils'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import blossomService from '@/services/blossom.service'
import responsiveImageService from '@/services/responsive-image.service'
import modalManager from '@/services/modal-manager.service'
import { TImetaInfo } from '@/types'
import { UploadedVariant } from '@/lib/responsive-image-event'
import { ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import Lightbox from 'yet-another-react-lightbox'
import Zoom from 'yet-another-react-lightbox/plugins/zoom'
import Image from '../Image'
import ImageWithLightbox from '../ImageWithLightbox'

type EnhancedImageInfo = TImetaInfo & {
  displayInfo: TImetaInfo
  originalUrl: string
  variants: UploadedVariant[]
}

export default function ResponsiveImageGallery({
  className,
  images,
  start = 0,
  end = images.length,
  mustLoad = false
}: {
  className?: string
  images: TImetaInfo[]
  start?: number
  end?: number
  mustLoad?: boolean
}) {
  const id = useMemo(() => `responsive-image-gallery-${randomString()}`, [])
  const { autoLoadMedia } = useContentPolicy()
  const [index, setIndex] = useState(-1)
  const [enhancedImages, setEnhancedImages] = useState<EnhancedImageInfo[]>([])
  const [slides, setSlides] = useState<{ src: string }[]>([])
  const containerRef = useRef<HTMLDivElement>(null)
  const [containerWidth, setContainerWidth] = useState(0)

  // Measure container width for variant selection
  useEffect(() => {
    if (!containerRef.current) return

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width)
      }
    })

    observer.observe(containerRef.current)
    setContainerWidth(containerRef.current.offsetWidth)

    return () => observer.disconnect()
  }, [])

  // Register/unregister modal
  useEffect(() => {
    if (index >= 0) {
      modalManager.register(id, () => setIndex(-1))
    } else {
      modalManager.unregister(id)
    }
  }, [index])

  // Fetch variants and enhance images
  useEffect(() => {
    const enhanceImages = async () => {
      const enhanced = await Promise.all(
        images.map(async (image): Promise<EnhancedImageInfo> => {
          // Get sha256 from image or extract from URL
          const sha256 = image.sha256 ?? responsiveImageService.extractSha256FromUrl(image.url)

          if (!sha256) {
            return {
              ...image,
              displayInfo: image,
              originalUrl: image.url,
              variants: []
            }
          }

          const variants = await responsiveImageService.getVariantsForHash(sha256)

          if (!variants || variants.length === 0) {
            return {
              ...image,
              displayInfo: image,
              originalUrl: image.url,
              variants: []
            }
          }

          // Select variant for display (based on container or estimated width)
          const targetWidth = containerWidth > 0 ? containerWidth : 400
          const pixelRatio = typeof window !== 'undefined' ? window.devicePixelRatio : 1
          const displayVariant = responsiveImageService.selectVariant(variants, targetWidth, pixelRatio)

          // Get original for lightbox
          const originalVariant = responsiveImageService.getOriginalVariant(variants)

          const displayInfo: TImetaInfo = displayVariant
            ? {
                url: displayVariant.url,
                sha256: displayVariant.sha256,
                blurHash: displayVariant.blurhash ?? image.blurHash,
                thumbHash: image.thumbHash,
                dim: { width: displayVariant.width, height: displayVariant.height },
                pubkey: image.pubkey,
                variant: displayVariant.variant
              }
            : image

          return {
            ...image,
            displayInfo,
            originalUrl: originalVariant?.url ?? image.url,
            variants
          }
        })
      )

      setEnhancedImages(enhanced)
    }

    enhanceImages()
  }, [images, containerWidth])

  // Build lightbox slides with original URLs
  useEffect(() => {
    const loadSlides = async () => {
      const newSlides = await Promise.all(
        enhancedImages.map(({ originalUrl, pubkey }) => {
          return new Promise<{ src: string }>((resolve) => {
            const img = new window.Image()
            let validUrl = originalUrl

            img.onload = () => {
              blossomService.markAsSuccess(originalUrl, validUrl)
              resolve({ src: validUrl })
            }

            img.onerror = () => {
              blossomService.tryNextUrl(originalUrl).then((nextUrl) => {
                if (nextUrl) {
                  validUrl = nextUrl
                  resolve({ src: validUrl })
                } else {
                  resolve({ src: originalUrl })
                }
              })
            }

            if (pubkey) {
              blossomService
                .getValidUrl(originalUrl, pubkey)
                .then((u) => {
                  validUrl = u
                  img.src = validUrl
                })
                .catch(() => resolve({ src: originalUrl }))
            } else {
              img.src = originalUrl
            }
          })
        })
      )
      setSlides(newSlides)
    }

    if (enhancedImages.length > 0) {
      loadSlides()
    }
  }, [enhancedImages])

  const handlePhotoClick = (event: React.MouseEvent, current: number) => {
    event.stopPropagation()
    event.preventDefault()
    setIndex(start + current)
  }

  const displayImages = enhancedImages.slice(start, end)

  // Fall back to non-responsive display if auto-load is disabled
  if (!mustLoad && !autoLoadMedia) {
    return (
      <>
        {displayImages.map((image, i) => (
          <ImageWithLightbox
            key={i}
            image={image.displayInfo}
            className="max-h-[80vh] sm:max-h-[50vh] object-contain"
            classNames={{
              wrapper: cn('w-fit max-w-full border', className)
            }}
          />
        ))}
      </>
    )
  }

  let imageContent: ReactNode | null = null

  if (displayImages.length === 1) {
    imageContent = (
      <Image
        key={0}
        className="max-h-[80vh] sm:max-h-[50vh] object-contain"
        classNames={{
          errorPlaceholder: 'aspect-square h-[30vh]',
          wrapper: 'cursor-zoom-in border'
        }}
        image={displayImages[0].displayInfo}
        originalUrl={displayImages[0].originalUrl}
        onClick={(e) => handlePhotoClick(e, 0)}
      />
    )
  } else if (displayImages.length === 2 || displayImages.length === 4) {
    imageContent = (
      <div className="grid grid-cols-2 gap-2 w-full">
        {displayImages.map((image, i) => (
          <Image
            key={i}
            className="aspect-square w-full"
            classNames={{ wrapper: 'cursor-zoom-in border' }}
            image={image.displayInfo}
            originalUrl={image.originalUrl}
            onClick={(e) => handlePhotoClick(e, i)}
          />
        ))}
      </div>
    )
  } else {
    imageContent = (
      <div className="grid grid-cols-3 gap-2 w-full">
        {displayImages.map((image, i) => (
          <Image
            key={i}
            className="aspect-square w-full"
            classNames={{ wrapper: 'cursor-zoom-in border' }}
            image={image.displayInfo}
            originalUrl={image.originalUrl}
            onClick={(e) => handlePhotoClick(e, i)}
          />
        ))}
      </div>
    )
  }

  return (
    <div
      ref={containerRef}
      className={cn(displayImages.length === 1 ? 'w-fit max-w-full' : 'w-full', className)}
    >
      {imageContent}
      {index >= 0 &&
        createPortal(
          <div onClick={(e) => e.stopPropagation()}>
            <Lightbox
              index={index}
              slides={slides}
              plugins={[Zoom]}
              open={index >= 0}
              close={() => setIndex(-1)}
              controller={{
                closeOnBackdropClick: true,
                closeOnPullUp: true,
                closeOnPullDown: true
              }}
              styles={{
                toolbar: { paddingTop: '2.25rem' }
              }}
            />
          </div>,
          document.body
        )}
    </div>
  )
}
